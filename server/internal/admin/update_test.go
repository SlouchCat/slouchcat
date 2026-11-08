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
	"github.com/slouchcat/slouchcat/server/internal/updater"
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
	srv := newManifestServer(t, `{"version":"0.10.0","url":"https://github.com/SlouchCat/slouchcat/releases/tag/v0.10.0","notes":"Neu: Admin","publishedAt":"2026-10-21T10:00:00Z"}`)
	f.svc.WithSystem(SystemSource{Version: "0.9.0+abc1234"}).WithUpdateCheck(srv.URL+"/manifest.json", srv.Client())
	if f.svc.UpdateAvailable(ctx) != nil {
		t.Fatal("notice before any check")
	}

	st, err := f.svc.CheckForUpdate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || !st.Available || st.LatestVersion != "0.10.0" || st.CurrentVersion != "0.9.0+abc1234" || st.Notes != "Neu: Admin" ||
		st.CheckedAt == nil || st.Error != "" || st.LastSuccessAt == nil || !st.LastSuccessAt.Equal(*st.CheckedAt) {
		t.Fatalf("state = %+v, want 0.10.0 available", st)
	}
	if n := f.svc.UpdateAvailable(ctx); n == nil || n.Version != "0.10.0" || n.URL != st.URL {
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

	// Only a plain release X.Y.Z is offered (ADR 0024, addendum of 8 Nov 2026); a pre-release or a
	// "v" version is stored and shown, never offered.
	for _, v := range []string{"0.12.0-rc.1", "v0.12.0"} {
		srv.set(`{"version":"`+v+`","url":"https://example.org/`+v+`"}`, http.StatusOK)
		if st, err = f.svc.CheckForUpdate(ctx); err != nil || st.Available || st.LatestVersion != v || st.Error != "" {
			t.Errorf("manifest %s: %+v, %v", v, st, err)
		}
		if n := f.svc.UpdateAvailable(ctx); n != nil {
			t.Errorf("notice for %s: %+v", v, n)
		}
	}

	f.svc.now = func() time.Time { return time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC) }
	srv.set(`{"version":"0.11.0","url":"https://example.org/0.11.0"}`, http.StatusOK)
	if _, err := f.svc.CheckForUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	success := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return success.Add(time.Hour) }
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
			// The last known release and the last success stay.
			if st.LatestVersion != "0.11.0" || !st.Available || st.LastSuccessAt == nil || !st.LastSuccessAt.Equal(success) ||
				st.CheckedAt == nil || !st.CheckedAt.Equal(success.Add(time.Hour)) {
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
	registry := jobs.NewRegistry(queue, f.svc.log)
	f.svc.RegisterTypes(registry)
	registry.Attach(w, jobs.PoolGeneral)
	if ty, ok := registry.Lookup(JobUpdateCheck); !ok || !ty.Scheduled() || ty.Every != UpdateInterval || ty.StartDelay == nil || *ty.StartDelay != updateStartDelay {
		t.Fatalf("update.check = %+v, want a schedule type every %v, %v after the start", ty, UpdateInterval, updateStartDelay)
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

// fakeSchedules records the schedule updates of the settings pages (admin.Schedules) and keeps the
// one-off date per type, as the scheduler does.
type fakeSchedules struct {
	loc     *time.Location // nil: UTC
	once    map[string]time.Time
	updates []struct {
		typ string
		u   jobs.ScheduleUpdate
	}
}

func (f *fakeSchedules) Location() *time.Location {
	if f.loc == nil {
		return time.UTC
	}
	return f.loc
}

func (f *fakeSchedules) UpdateSchedule(_ context.Context, typ string, u jobs.ScheduleUpdate) (jobs.Schedule, error) {
	f.updates = append(f.updates, struct {
		typ string
		u   jobs.ScheduleUpdate
	}{typ, u})
	if u.ClearOnce {
		delete(f.once, typ)
	} else if u.OnceLocal != nil {
		at, err := time.ParseInLocation(jobs.OnceLayout, *u.OnceLocal, f.Location())
		if err != nil {
			return jobs.Schedule{}, err
		}
		f.setOnce(typ, at)
	}
	return jobs.Schedule{Type: typ, OnceAt: f.once[typ]}, nil
}

func (f *fakeSchedules) Schedule(_ context.Context, typ string) (jobs.Schedule, error) {
	return jobs.Schedule{Type: typ, OnceAt: f.once[typ]}, nil
}

// setOnce sets the one-off date of typ, as an administrator does on the jobs page.
func (f *fakeSchedules) setOnce(typ string, at time.Time) {
	if f.once == nil {
		f.once = map[string]time.Time{}
	}
	f.once[typ] = at
}

// fireDue clears the one-off dates that are due at now: the scheduler has fired them.
func (f *fakeSchedules) fireDue(now time.Time) {
	for typ, at := range f.once {
		if !at.After(now) {
			delete(f.once, typ)
		}
	}
}

// The offer rule of the check (updater.Newer): only a plain release newer than the running
// version; a self-built image counts as its release.
func TestNewerVersion(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		want            bool
	}{
		{"0.10.0", "0.9.0", true},
		{"0.9.1", "0.9.0+abc1234", true},
		{"v0.9.1", "0.9.0", false},
		{"0.9.0", "v0.9.0", false},
		{"0.9.0", "0.10.0", false},
		{"1.0.0", "1.0.0-rc.1", true},
		// A pre-release is never offered (ADR 0024, addendum of 8 Nov 2026).
		{"1.0.0-rc.2", "1.0.0-rc.1", false},
		{"1.1.0-rc.1", "1.0.0", false},
		{"1.0.0+build", "0.9.0", false},
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
		if got := updater.Newer(tc.latest, tc.current); got != tc.want {
			t.Errorf("updater.Newer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

// A failed check of the job is tried again after 15 min, 1 h and 6 h through the one-off date of
// the schedule (in the scheduler's zone); the fourth failure in a row leaves the daily rhythm alone,
// and a success starts the sequence anew.
func TestUpdateCheckRetryBackoff(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	now := time.Date(2026, 11, 5, 16, 33, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return now }
	srv := newManifestServer(t, `{"version":"1.13.1","url":"https://example.org/1.13.1"}`)
	sched := &fakeSchedules{loc: time.FixedZone("CET", 3600)}
	f.svc.WithSchedules(sched).WithSystem(SystemSource{Version: "1.12.0"}).WithUpdateCheck(srv.URL+"/manifest.json", srv.Client())

	check := func(wantOnce string, wantFailures int) {
		t.Helper()
		sched.fireDue(now)
		before := len(sched.updates)
		if err := f.svc.handleUpdateCheck(ctx, nil); err != nil {
			t.Fatal(err)
		}
		st, err := f.svc.storedUpdate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.Failures != wantFailures {
			t.Errorf("failures = %d, want %d", st.Failures, wantFailures)
		}
		got := sched.updates[before:]
		if wantOnce == "" {
			if len(got) != 0 {
				t.Errorf("schedule updates = %+v, want none (daily rhythm)", got)
			}
			return
		}
		if len(got) != 1 || got[0].typ != JobUpdateCheck || got[0].u.OnceLocal == nil || *got[0].u.OnceLocal != wantOnce || got[0].u.Every != nil || got[0].u.Enabled != nil {
			t.Fatalf("schedule updates = %+v, want one one-off date %s", got, wantOnce)
		}
	}

	srv.set(`oops`, http.StatusBadGateway)
	check("2026-11-05T17:48", 1) // +15 min, 16:33 UTC = 17:33 CET
	now = now.Add(15 * time.Minute)
	check("2026-11-05T18:48", 2) // +1 h from 16:48 UTC
	now = now.Add(time.Hour)
	check("2026-11-06T00:48", 3) // +6 h from 17:48 UTC
	now = now.Add(6 * time.Hour)
	check("", 4) // back to the daily rhythm
	now = now.Add(24 * time.Hour)
	check("", 4) // stays there, the counter does not grow without bound

	srv.set(`{"version":"1.13.1","url":"https://example.org/1.13.1"}`, http.StatusOK)
	check("", 0) // a success resets
	if st, err := f.svc.UpdateState(ctx); err != nil || st.Error != "" || !st.Available {
		t.Fatalf("state after the success = %+v, %v", st, err)
	}
	srv.set(`oops`, http.StatusBadGateway)
	check("2026-11-07T01:03", 1) // 15 min again, from 2026-11-06 23:48 UTC

	// The failed state keeps what the card shows.
	if st, err := f.svc.UpdateState(ctx); err != nil || !strings.Contains(st.Error, "status 502") || st.LatestVersion != "1.13.1" {
		t.Errorf("state after a failed check = %+v, %v", st, err)
	}
}

// updateRetryFixture is a service with a failing manifest and a recording scheduler at now.
func updateRetryFixture(t *testing.T, now time.Time) (*Service, *manifestServer, *fakeSchedules) {
	t.Helper()
	f := newFixture(t)
	f.svc.now = func() time.Time { return now }
	srv := newManifestServer(t, `oops`)
	srv.set(`oops`, http.StatusBadGateway)
	sched := &fakeSchedules{}
	f.svc.WithSchedules(sched).WithSystem(SystemSource{Version: "1.12.0"}).WithUpdateCheck(srv.URL+"/manifest.json", srv.Client())
	return f.svc, srv, sched
}

// A failed check by hand («Jetzt prüfen») records its error but neither counts toward the
// retries of the job nor sets or moves one (review of stage 1).
func TestUpdateCheckByHandDoesNotCountTowardRetries(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	svc, _, sched := updateRetryFixture(t, now)

	st, err := svc.CheckForUpdate(ctx)
	if err != nil || !strings.Contains(st.Error, "status 502") {
		t.Fatalf("state after a failed check by hand = %+v, %v", st, err)
	}
	if stored, err := svc.storedUpdate(ctx); err != nil || stored.Failures != 0 || stored.RetryAt != 0 {
		t.Errorf("stored after a failed check by hand = %+v, %v; want no failure counted", stored, err)
	}
	if len(sched.updates) != 0 {
		t.Errorf("schedule updates = %+v, want none", sched.updates)
	}

	// The job's failure counts and sets the retry; one by hand after it leaves both alone.
	if err := svc.handleUpdateCheck(ctx, nil); err != nil {
		t.Fatal(err)
	}
	retry := now.Add(15 * time.Minute)
	if _, err := svc.CheckForUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.storedUpdate(ctx)
	if err != nil || stored.Failures != 1 || stored.RetryAt != retry.UnixMilli() {
		t.Errorf("stored = %+v, %v; want one failure and the retry at %v", stored, err, retry)
	}
	if len(sched.updates) != 1 || !sched.once[JobUpdateCheck].Equal(retry) {
		t.Errorf("schedule updates = %+v, date %v; want the one retry at %v", sched.updates, sched.once[JobUpdateCheck], retry)
	}
}

// A one-off date an administrator set before the retry stays (it checks earlier anyway); one after
// it gives way to the retry (review of stage 1).
func TestUpdateRetryKeepsAnEarlierAdminDate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	svc, _, sched := updateRetryFixture(t, now)

	earlier := now.Add(10 * time.Minute)
	sched.setOnce(JobUpdateCheck, earlier)
	if err := svc.handleUpdateCheck(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if len(sched.updates) != 0 || !sched.once[JobUpdateCheck].Equal(earlier) {
		t.Errorf("schedule updates = %+v, date %v; want the administrator's date %v untouched", sched.updates, sched.once[JobUpdateCheck], earlier)
	}
	if stored, err := svc.storedUpdate(ctx); err != nil || stored.Failures != 1 || stored.RetryAt != 0 {
		t.Errorf("stored = %+v, %v; want one failure and no retry of ours", stored, err)
	}

	// A later date is replaced by the retry (+1 h after the second failure).
	sched.setOnce(JobUpdateCheck, now.Add(3*time.Hour))
	if err := svc.handleUpdateCheck(ctx, nil); err != nil {
		t.Fatal(err)
	}
	retry := now.Add(time.Hour)
	if len(sched.updates) != 1 || !sched.once[JobUpdateCheck].Equal(retry) {
		t.Errorf("schedule updates = %+v, date %v; want the retry at %v", sched.updates, sched.once[JobUpdateCheck], retry)
	}
	if stored, err := svc.storedUpdate(ctx); err != nil || stored.Failures != 2 || stored.RetryAt != retry.UnixMilli() {
		t.Errorf("stored = %+v, %v; want two failures and the retry at %v", stored, err, retry)
	}
}

// A success clears the retry still pending, but not a date an administrator set in its place
// (review of stage 1).
func TestUpdateSuccessClearsOnlyThePendingRetry(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	svc, srv, sched := updateRetryFixture(t, now)
	const good = `{"version":"1.13.1","url":"https://example.org/1.13.1"}`

	// Failure, then a success by hand before the retry: the retry goes.
	if err := svc.handleUpdateCheck(ctx, nil); err != nil {
		t.Fatal(err)
	}
	srv.set(good, http.StatusOK)
	if _, err := svc.CheckForUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sched.updates) != 2 || !sched.updates[1].u.ClearOnce || !sched.once[JobUpdateCheck].IsZero() {
		t.Errorf("schedule updates = %+v, date %v; want the retry set and cleared", sched.updates, sched.once[JobUpdateCheck])
	}
	if stored, err := svc.storedUpdate(ctx); err != nil || stored.Failures != 0 || stored.RetryAt != 0 {
		t.Errorf("stored after the success = %+v, %v; want no failure and no retry", stored, err)
	}

	// Failure, then the administrator moves the date: the success leaves it.
	srv.set(`oops`, http.StatusBadGateway)
	if err := svc.handleUpdateCheck(ctx, nil); err != nil {
		t.Fatal(err)
	}
	admin := now.Add(2 * time.Hour)
	sched.setOnce(JobUpdateCheck, admin)
	before := len(sched.updates)
	srv.set(good, http.StatusOK)
	if err := svc.handleUpdateCheck(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if len(sched.updates) != before || !sched.once[JobUpdateCheck].Equal(admin) {
		t.Errorf("schedule updates = %+v, date %v; want the administrator's date %v untouched", sched.updates[before:], sched.once[JobUpdateCheck], admin)
	}
}

func TestUpdateRetryDelay(t *testing.T) {
	for failures, want := range map[int]time.Duration{1: 15 * time.Minute, 2: time.Hour, 3: 6 * time.Hour} {
		if d, ok := updateRetryDelay(failures); !ok || d != want {
			t.Errorf("updateRetryDelay(%d) = %v, %v; want %v", failures, d, ok, want)
		}
	}
	for _, failures := range []int{-1, 0, 4, 100} {
		if d, ok := updateRetryDelay(failures); ok {
			t.Errorf("updateRetryDelay(%d) = %v, true; want the daily rhythm", failures, d)
		}
	}
}

func TestUpdateTimeout(t *testing.T) {
	if updateTimeout != 20*time.Second {
		t.Errorf("updateTimeout = %v, want 20s", updateTimeout)
	}
	f := newFixture(t)
	f.svc.WithUpdateCheck("https://example.org/manifest.json", nil)
	if got := f.svc.ops.update.client.Timeout; got != updateTimeout {
		t.Errorf("default client timeout = %v, want %v", got, updateTimeout)
	}
}
