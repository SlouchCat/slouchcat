package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

func upsert(key, value string) appq.UpsertSettingParams {
	return appq.UpsertSettingParams{Key: key, Value: value, Now: 1}
}

// manifestServer serves the release manifest body and records the User-Agent of each request.
type manifestServer struct {
	*httptest.Server
	mu     sync.Mutex
	body   string
	status int
	agents []string
}

func newManifestServer(t *testing.T, body string) *manifestServer {
	t.Helper()
	m := &manifestServer{body: body, status: http.StatusOK}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.agents = append(m.agents, r.UserAgent())
		if r.URL.Path != "/manifest.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(m.status)
		_, _ = w.Write([]byte(m.body))
	}))
	t.Cleanup(m.Close)
	return m
}

func (m *manifestServer) set(body string, status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.body, m.status = body, status
}

// A-5 (design 3.5): a newer manifest version is announced, the same or an older one is not, a
// disabled check announces nothing and refuses to check, a broken manifest records its error.
func TestUpdateCheck(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	srv := newManifestServer(t, `{"version":"v0.10.0","url":"https://github.com/SlouchCat/slouchcat/releases/tag/v0.10.0","notes":"Neu: Admin","publishedAt":"2026-10-21T10:00:00Z"}`)
	f.svc.WithSystem(SystemSource{Version: "0.9.0+abc1234"}).WithUpdateCheck(srv.URL+"/manifest.json", srv.Client())
	if f.svc.UpdateAvailable(ctx) != nil {
		t.Fatal("notice before any check")
	}

	st, err := f.svc.CheckForUpdate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || !st.Available || st.LatestVersion != "v0.10.0" || st.CurrentVersion != "0.9.0+abc1234" || st.Notes != "Neu: Admin" ||
		st.CheckedAt == nil || st.Error != "" {
		t.Fatalf("state = %+v, want v0.10.0 available", st)
	}
	if n := f.svc.UpdateAvailable(ctx); n == nil || n.Version != "v0.10.0" || n.URL != st.URL {
		t.Errorf("notice = %+v", n)
	}
	if got := srv.agents[len(srv.agents)-1]; got != "Slouchcat/0.9.0+abc1234" {
		t.Errorf("User-Agent = %q", got)
	}

	srv.set(`{"version":"0.9.0","url":"https://example.org/0.9.0"}`, http.StatusOK)
	if st, err = f.svc.CheckForUpdate(ctx); err != nil || st.Available || st.LatestVersion != "0.9.0" {
		t.Errorf("same version: %+v, %v", st, err)
	}
	if f.svc.UpdateAvailable(ctx) != nil {
		t.Error("notice for the running version")
	}

	srv.set(`{"version":"0.11.0","url":"https://example.org/0.11.0"}`, http.StatusOK)
	if _, err := f.svc.CheckForUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		body   string
		status int
		want   string
	}{
		"bad JSON":        {body: `{"version":`, status: http.StatusOK, want: "not JSON"},
		"no version":      {body: `{"url":"https://example.org"}`, status: http.StatusOK, want: "version"},
		"no url":          {body: `{"version":"1.0.0"}`, status: http.StatusOK, want: "url"},
		"server error":    {body: `oops`, status: http.StatusBadGateway, want: "status 502"},
		"larger than max": {body: `{"version":"1.0.0","notes":"` + strings.Repeat("x", maxManifestBytes) + `"}`, status: http.StatusOK, want: "64 KiB"},
	} {
		t.Run(name, func(t *testing.T) {
			srv.set(tc.body, tc.status)
			st, err := f.svc.CheckForUpdate(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(st.Error, tc.want) {
				t.Errorf("error = %q, want it to mention %q", st.Error, tc.want)
			}
			// The last known release stays.
			if st.LatestVersion != "0.11.0" || !st.Available {
				t.Errorf("state after a failed check = %+v", st)
			}
		})
	}

	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CheckForUpdate(ctx); !errors.Is(err, model.ErrUpdateCheckDisabled) {
		t.Errorf("check while disabled: %v, want ErrUpdateCheckDisabled", err)
	}
	if st, err := f.svc.UpdateState(ctx); err != nil || st.Enabled || st.Available || st.LatestVersion != "" || st.CheckedAt != nil {
		t.Errorf("state while disabled = %+v, %v", st, err)
	}
	if n := f.svc.UpdateAvailable(ctx); n != nil {
		t.Errorf("notice while disabled = %+v", n)
	}
}

// A development build is not comparable with a release: the state names it and offers no update,
// and the header carries no badge.
func TestUpdateCheckDevBuild(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	srv := newManifestServer(t, `{"version":"0.9.0","url":"https://example.org/0.9.0"}`)
	f.svc.WithUpdateCheck(srv.URL+"/manifest.json", srv.Client())
	st, err := f.svc.CheckForUpdate(ctx)
	if err != nil || st.Available || !st.Development || st.CurrentVersion != "dev" || st.LatestVersion != "0.9.0" {
		t.Errorf("dev build: %+v, %v", st, err)
	}
	if n := f.svc.UpdateAvailable(ctx); n != nil {
		t.Errorf("notice of a dev build = %+v", n)
	}
}

func TestIsRelease(t *testing.T) {
	for _, tc := range []struct {
		v    string
		want bool
	}{
		{"1.4.0", true},
		{"v1.4.0", true},
		{"v1.1.0-79-gf92c09f", true},
		{"v1.1.0-79-gf92c09f-dirty", true},
		{"1.4.0-rc.1", true},
		{"dev", false},
		{"", false},
		{"f92c09f", false},
		{"spotlight-marker-f92c09f", false},
		{"1bb1c7e2b9f0c3d4e5f60718293a4b5c6d7e8f90", false},
	} {
		if got := isRelease(tc.v); got != tc.want {
			t.Errorf("isRelease(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// RunUpdateCheck's tick queues the job once a day while the check is on; the job stores the state.
func TestQueueDueUpdateCheck(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return now }
	srv := newManifestServer(t, `{"version":"1.0.0","url":"https://example.org/1.0.0"}`)
	queue := jobs.NewQueue(f.db.Write, func() time.Time { return now })
	f.svc.WithQueue(queue).WithSystem(SystemSource{Version: "0.9.0"}).WithUpdateCheck(srv.URL+"/manifest.json", srv.Client())

	queued, err := f.svc.QueueDueUpdateCheck(ctx)
	if err != nil || !queued {
		t.Fatalf("first tick: %v, %v; want queued", queued, err)
	}
	// A worker with the admin jobs runs it.
	w := jobs.NewWorker(queue, "w1", f.svc.log)
	f.svc.RegisterJobs(w)
	wctx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { w.Run(wctx); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for f.svc.UpdateAvailable(ctx) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	stop()
	<-done
	if n := f.svc.UpdateAvailable(ctx); n == nil || n.Version != "1.0.0" {
		t.Fatalf("notice after the job = %+v", n)
	}
	if queued, err := f.svc.QueueDueUpdateCheck(ctx); err != nil || queued {
		t.Errorf("tick right after the check: %v, %v; want nothing queued", queued, err)
	}
	now = now.Add(UpdateInterval + time.Minute)
	if queued, err := f.svc.QueueDueUpdateCheck(ctx); err != nil || !queued {
		t.Errorf("tick a day later: %v, %v; want queued", queued, err)
	}
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * UpdateInterval)
	if queued, err := f.svc.QueueDueUpdateCheck(ctx); err != nil || queued {
		t.Errorf("tick while disabled: %v, %v; want nothing queued", queued, err)
	}
}

func TestNewerVersion(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		want            bool
	}{
		{"0.10.0", "0.9.0", true},
		{"v0.9.1", "0.9.0+abc1234", true},
		{"0.9.0", "v0.9.0", false},
		{"0.9.0", "0.10.0", false},
		{"1.0.0", "1.0.0-rc.1", true},
		{"1.0.0-rc.2", "1.0.0-rc.1", true},
		{"1.0.0-rc.10", "1.0.0-rc.9", true},
		{"1.0.0-beta", "1.0.0-alpha.1", true},
		{"1.0.0-alpha.1", "1.0.0-alpha", true},
		{"1.0.0-alpha", "1.0.0-1", true},
		{"1.0.0-rc.1", "1.0.0", false},
		{"0.9.0", "dev", false},
		{"0.9.0", "", false},
		{"0.9.0", "spotlight-marker-f92c09f", false},
		// A self-built image of the release (git describe) runs that release (review M-3).
		{"0.9.0", "v0.9.0-3-gabc1234-dirty", false},
		{"0.9.1", "v0.9.0-3-gabc1234", true},
		{"latest", "0.9.0", false},
		{"1.0", "0.9.0", false},
		{"01.0.0", "0.9.0", false},
	} {
		if got := newerVersion(tc.latest, tc.current); got != tc.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}
