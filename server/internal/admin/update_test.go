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

	// An older bulk job of a library import (a scan's keyframe index, priority 1) is waiting.
	bulk := "probe.keyframes"
	if _, err := queue.Enqueue(ctx, jobs.NewJob{Type: bulk, Priority: 1, IdempotencyKey: "keyframes:01J"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)

	// The scheduled run (the Run of the type update.check) queues one job under its key.
	ids, err := f.svc.runUpdateCheck(ctx, queue, updatePriority)
	if err != nil || len(ids) != 1 {
		t.Fatalf("scheduled run: %v, %v; want one job", ids, err)
	}
	// The check carries a priority above every bulk job (the ladder in package jobs), so a worker
	// claims it before the older index job: at priority 1 it waited behind the backlog of an
	// import for days and the administration showed a stale release.
	if updatePriority <= 6 {
		t.Fatalf("updatePriority = %d, want above the bulk rungs (6 and below)", updatePriority)
	}
	first, err := queue.Claim(ctx, "w0", []string{bulk, JobUpdateCheck})
	if err != nil || first == nil {
		t.Fatalf("claim: %v, %v", first, err)
	}
	if first.Type != JobUpdateCheck || first.Priority != updatePriority {
		t.Fatalf("first claimed job = %s at priority %d, want %s at %d before the older %s", first.Type, first.Priority, JobUpdateCheck, updatePriority, bulk)
	}
	if err := queue.Release(ctx, "w0", first.ID); err != nil {
		t.Fatal(err)
	}
	// A worker with the admin jobs runs it.
	w := jobs.NewWorker(queue, "w1", f.svc.log)
	registry := jobs.NewRegistry()
	f.svc.RegisterTypes(registry)
	registry.Attach(w, jobs.PoolGeneral)
	if types := registry.Types(); len(types) != 1 || !types[0].Scheduled() || types[0].Every != UpdateInterval || types[0].StartDelay == nil || *types[0].StartDelay != updateStartDelay {
		t.Fatalf("registered types = %+v, want update.check as a schedule type every %v, %v after the start", types, UpdateInterval, updateStartDelay)
	}
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
	// The seed of the first schedule row continues the daily rhythm from the stored check.
	seed, err := f.svc.updateSeed(ctx)
	if err != nil || !seed.Enabled || seed.Every != UpdateInterval || !seed.LastRun.Equal(now) {
		t.Errorf("seed after the check = %+v, %v; want enabled, %v, last run %v", seed, err, UpdateInterval, now)
	}
	// A second run under the same key raises the waiting job instead of being swallowed by it,
	// and never lowers it (a job queued low, the scheduled run at 8, a manual run at 7).
	if _, err := f.svc.runUpdateCheck(ctx, queue, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.runUpdateCheck(ctx, queue, updatePriority); err != nil {
		t.Fatal(err)
	}
	waiting, err := queue.Claim(ctx, "w2", []string{JobUpdateCheck})
	if err != nil || waiting == nil || waiting.Priority != updatePriority {
		t.Fatalf("claimed after a run at priority 1: %+v, %v; want the job at %d", waiting, err, updatePriority)
	}
	if err := queue.Release(ctx, "w2", waiting.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.runUpdateCheck(ctx, queue, jobs.ManualPriority); err != nil {
		t.Fatal(err)
	}
	if j, err := queue.Get(ctx, waiting.ID); err != nil || j.State != jobs.StateQueued || j.Priority != updatePriority {
		t.Errorf("waiting job after a manual run = %+v, %v; want still queued at %d (never lowered, 8 > 7)", j, err, updatePriority)
	}
	// Switched off, the run queues nothing and the setting is written into the schedule row.
	sched := &fakeSchedules{}
	f.svc.WithSchedules(sched)
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if len(sched.updates) != 1 || sched.updates[0].typ != JobUpdateCheck || sched.updates[0].u.Enabled == nil || *sched.updates[0].u.Enabled {
		t.Errorf("schedule updates after switching the check off = %+v, want update.check disabled", sched.updates)
	}
	if _, err := queue.Cancel(ctx, waiting.ID); err != nil {
		t.Fatal(err)
	}
	if ids, err := f.svc.runUpdateCheck(ctx, queue, updatePriority); err != nil || len(ids) != 0 {
		t.Errorf("run while disabled: %v, %v; want nothing queued", ids, err)
	}
	if seed, err := f.svc.updateSeed(ctx); err != nil || seed.Enabled {
		t.Errorf("seed while disabled = %+v, %v", seed, err)
	}
}

// fakeSchedules records the schedule updates of the settings pages (admin.Schedules).
type fakeSchedules struct {
	updates []struct {
		typ string
		u   jobs.ScheduleUpdate
	}
}

func (f *fakeSchedules) UpdateSchedule(_ context.Context, typ string, u jobs.ScheduleUpdate) (jobs.Schedule, error) {
	f.updates = append(f.updates, struct {
		typ string
		u   jobs.ScheduleUpdate
	}{typ, u})
	return jobs.Schedule{Type: typ}, nil
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
