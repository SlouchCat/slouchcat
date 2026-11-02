package admin

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

type fakeChannels struct {
	avail map[string]model.ChannelAvail
}

func (f fakeChannels) ChannelAvailability(context.Context) (map[string]model.ChannelAvail, error) {
	return f.avail, nil
}

// A-3: a job that failed finally appears in recentFailed with its error redacted, its retry queues
// it again, an error of the log appears in errors, and the storage and integrations are filled.
func TestDashboardJobsErrorsAndStorage(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	queue := jobs.NewQueue(f.db.Write, nil)
	var logs bytes.Buffer
	log, ring, err := logging.NewWithErrors(&logs, logging.FormatJSON, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	src := fakeSystem(t, false, false, "", "524288")
	src.ConfigDir, src.CacheDir = t.TempDir(), t.TempDir()
	f.svc.WithJobs(queue).WithErrors(ring).WithSystem(src)

	if _, err := f.svc.Dashboard(ctx); err != nil {
		t.Fatalf("empty dashboard: %v", err)
	}
	id, err := queue.Enqueue(ctx, jobs.NewJob{Type: "metadata.match", MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	j, err := queue.Claim(ctx, "w1", []string{"metadata.match"})
	if err != nil || j == nil {
		t.Fatalf("claim: %v, %v", j, err)
	}
	if err := queue.Fail(ctx, "w1", j, errors.New(`Get "https://api.themoviedb.org/3/movie/1?api_key=dash-FAKE-key": timeout`)); err != nil {
		t.Fatal(err)
	}
	log.With("component", "metadata").Error("tmdb unreachable", "err", errors.New("dial tcp: timeout"), "token", "dash-FAKE-token")

	d, err := f.svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Jobs.Failed != 1 || d.Jobs.Queued != 0 || len(d.Jobs.RecentFailed) != 1 {
		t.Fatalf("jobs = %+v, want one failed", d.Jobs)
	}
	failed := d.Jobs.RecentFailed[0]
	if failed.ID != id || failed.Type != "metadata.match" || failed.State != "failed" || failed.Attempts != 1 ||
		!strings.Contains(failed.LastError, "api_key=[redacted]") || failed.UpdatedAt.IsZero() {
		t.Errorf("failed job = %+v", failed)
	}
	if len(d.Errors) != 1 || d.Errors[0].Source != "metadata" || d.Errors[0].Message != "tmdb unreachable: dial tcp: timeout" {
		t.Errorf("errors = %+v, want the logged error", d.Errors)
	}
	if d.Config.Path != src.ConfigDir || d.Config.TotalBytes <= 0 || d.Config.FreeBytes <= 0 || d.Cache.Path != src.CacheDir {
		t.Errorf("storage = %+v / %+v", d.Config, d.Cache)
	}
	if !d.Update.Enabled || d.Update.CurrentVersion != "dev" {
		t.Errorf("update = %+v", d.Update)
	}
	for _, secret := range []string{"dash-FAKE-key", "dash-FAKE-token"} {
		if strings.Contains(failed.LastError+d.Errors[0].Message, secret) {
			t.Errorf("dashboard carries %q", secret)
		}
	}

	retried, err := f.svc.RetryJob(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if retried.ID != id || retried.State != "queued" || retried.Attempts != 0 {
		t.Errorf("retried = %+v, want queued with attempts 0", retried)
	}
	if d, err = f.svc.Dashboard(ctx); err != nil || d.Jobs.Queued != 1 || d.Jobs.Failed != 0 || len(d.Jobs.RecentFailed) != 0 {
		t.Errorf("after the retry: %+v, %v", d.Jobs, err)
	}
	if _, err := f.svc.RetryJob(ctx, id); !errors.Is(err, model.ErrJobNotFailed) {
		t.Errorf("retry of a queued job: %v, want ErrJobNotFailed", err)
	}
	if err := f.svc.CancelJob(ctx, id); err != nil {
		t.Errorf("cancel: %v", err)
	}
	if err := f.svc.CancelJob(ctx, id); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("cancel of a cancelled job: %v, want ErrNotFound", err)
	}
	// "Abbrechen" on a failed job dismisses it (review I-5).
	failedID, err := queue.Enqueue(ctx, jobs.NewJob{Type: "metadata.match", MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	if j, err = queue.Claim(ctx, "w1", []string{"metadata.match"}); err != nil || j == nil || j.ID != failedID {
		t.Fatalf("claim: %v, %v", j, err)
	}
	if err := queue.Fail(ctx, "w1", j, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CancelJob(ctx, failedID); err != nil {
		t.Errorf("cancel of a failed job: %v", err)
	}
	if d, err = f.svc.Dashboard(ctx); err != nil || d.Jobs.Failed != 0 || len(d.Jobs.RecentFailed) != 0 {
		t.Errorf("after dismissing the failed job: %+v, %v", d.Jobs, err)
	}
	if _, err := f.svc.RetryJob(ctx, 9999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("retry of an unknown job: %v, want ErrNotFound", err)
	}
	if _, err := newFixture(t).svc.Dashboard(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("dashboard without jobs: %v, want ErrNotConfigured", err)
	}
}

// The integrations: keys by source, channels from notify or, without it, from the configuration,
// the update check by its switch.
func TestDashboardIntegrations(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.env[EnvYouTubeKey] = "yt-FAKE-env"
	f.svc.WithJobs(jobs.NewQueue(f.db.Write, nil))
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{Keys: model.AdminKeysUpdate{TMDB: ptr(fakeKey)}}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetVAPIDKeys(ctx, "BPub-FAKE", "priv-FAKE-vapid"); err != nil {
		t.Fatal(err)
	}
	states := func() map[string]model.IntegrationStatus {
		t.Helper()
		d, err := f.svc.Dashboard(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]model.IntegrationStatus{}
		var keys []string
		for _, i := range d.Integrations {
			out[i.Key] = i
			keys = append(keys, i.Key)
		}
		if got := strings.Join(keys, ","); got != "tmdb,tvdb,youtube,trakt,email,push,telegram,updateCheck" {
			t.Fatalf("integration keys = %s", got)
		}
		return out
	}
	st := states()
	for key, want := range map[string]string{
		"tmdb": "configured", "tvdb": "missing", "youtube": "configured", "trakt": "missing",
		"email": "missing", "push": "configured", "telegram": "missing", "updateCheck": "configured",
	} {
		if st[key].State != want {
			t.Errorf("%s = %+v, want %s", key, st[key], want)
		}
	}

	attempt := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	f.svc.WithTrakt(&fakeTrakt{status: model.TraktStatus{Configured: true, LastAttempt: &attempt, LastError: "trending movie: status 503"}})
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{Keys: model.AdminKeysUpdate{Trakt: ptr("FAKE-trakt-dash-id")}}); err != nil {
		t.Fatal(err)
	}
	f.svc.WithChannels(fakeChannels{avail: map[string]model.ChannelAvail{
		model.ChannelEmail:    {Configured: true, Detail: "535 authentication failed"},
		model.ChannelTelegram: {Configured: true},
	}})
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	st = states()
	if e := st["email"]; e.State != "error" || e.Detail != "535 authentication failed" {
		t.Errorf("email = %+v, want error with the detail", e)
	}
	if st["telegram"].State != "configured" || st["push"].State != "missing" || st["updateCheck"].State != "disabled" {
		t.Errorf("telegram %+v, push %+v, update %+v", st["telegram"], st["push"], st["updateCheck"])
	}
	if tr := st["trakt"]; tr.State != "error" || tr.Detail != "trending movie: status 503" || tr.CheckedAt == nil || !tr.CheckedAt.Equal(attempt) {
		t.Errorf("trakt = %+v, want the error of the last attempt", tr)
	}
}

// Concept 11: the running jobs with subject and estimate (at most three, newest start first), the
// earliest next run over switched-on schedules of unpaused types, and the failures by type with
// the newest error redacted.
func TestDashboardJobOverview(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	jf.svc.WithSystem(SystemSource{ConfigDir: t.TempDir(), CacheDir: t.TempDir()})

	d, err := jf.svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Jobs.Active == nil || len(d.Jobs.Active) != 0 || d.Jobs.FailedByType == nil || len(d.Jobs.FailedByType) != 0 {
		t.Fatalf("empty jobs = %+v, want empty lists", d.Jobs)
	}

	// Five runs of 30 s give the event type a median; then four jobs start one second apart.
	for range 5 {
		jf.finish(t, fakeEvent, 30*time.Second)
	}
	var started []int64
	for range 4 {
		id, err := jf.queue.Enqueue(ctx, jobs.NewJob{Type: fakeEvent, Payload: []byte(`{"libraryId":1}`)})
		if err != nil {
			t.Fatal(err)
		}
		jf.now = jf.now.Add(time.Second)
		if j, err := jf.queue.Claim(ctx, "w", []string{fakeEvent}); err != nil || j == nil || j.ID != id {
			t.Fatalf("claim: %v, %v", j, err)
		}
		started = append(started, id)
	}
	jf.exec(t, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'Filme', 'video', 0, 0)`)

	// Failures: fake.event once, then the update check twice with a secret in the newest error.
	failJob := func(typ, msg string) {
		t.Helper()
		if _, err := jf.queue.Enqueue(ctx, jobs.NewJob{Type: typ, MaxAttempts: 1}); err != nil {
			t.Fatal(err)
		}
		j, err := jf.queue.Claim(ctx, "f", []string{typ})
		if err != nil || j == nil {
			t.Fatalf("claim %s: %v, %v", typ, j, err)
		}
		jf.now = jf.now.Add(time.Second)
		if err := jf.queue.Fail(ctx, "f", j, errors.New(msg)); err != nil {
			t.Fatal(err)
		}
	}
	failJob(fakeSchedule, "schedule broke")
	failJob(JobUpdateCheck, "first")
	failJob(JobUpdateCheck, `Get "https://example.org/?api_key=dash-FAKE-key": timeout`)

	// Every schedule far ahead, fake.schedule in 1 h and trakt.refresh in 2 h.
	jf.exec(t, `UPDATE job_schedules SET next_run_at = ?`, jf.now.Add(10*time.Hour).UnixMilli())
	jf.exec(t, `UPDATE job_schedules SET next_run_at = ? WHERE type = ?`, jf.now.Add(time.Hour).UnixMilli(), fakeSchedule)
	jf.exec(t, `UPDATE job_schedules SET next_run_at = ? WHERE type = ?`, jf.now.Add(2*time.Hour).UnixMilli(), jobTraktRefresh)

	d, err = jf.svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Jobs.Active) != 3 {
		t.Fatalf("active = %+v, want three", d.Jobs.Active)
	}
	for i, want := range []int64{started[3], started[2], started[1]} {
		run := d.Jobs.Active[i]
		if run.ID != want || run.Type != fakeEvent || run.State != "running" || run.Subject != "Filme" ||
			run.Estimate != model.JobEstimateMedian || run.EstimatedSecondsLeft == nil {
			t.Errorf("active %d = %+v, want job %d of %s with subject and median estimate", i, run, want, fakeEvent)
		}
	}
	if n := d.Jobs.NextRun; n == nil || n.Type != fakeSchedule || !n.At.Equal(jf.now.Add(time.Hour)) {
		t.Errorf("next run = %+v, want %s in 1 h", n, fakeSchedule)
	}
	if len(d.Jobs.FailedByType) != 2 {
		t.Fatalf("failed by type = %+v, want two types", d.Jobs.FailedByType)
	}
	up, sch := d.Jobs.FailedByType[0], d.Jobs.FailedByType[1]
	if up.Type != JobUpdateCheck || up.Count != 2 || !strings.Contains(up.LastError, "api_key=[redacted]") ||
		strings.Contains(up.LastError, "dash-FAKE-key") || !up.LastFailedAt.Equal(jf.now) {
		t.Errorf("newest failure = %+v, want %s twice with the redacted newest error at %v", up, JobUpdateCheck, jf.now)
	}
	if sch.Type != fakeSchedule || sch.Count != 1 || sch.LastError != "schedule broke" || !sch.LastFailedAt.Before(up.LastFailedAt) {
		t.Errorf("older failure = %+v", sch)
	}
	if len(d.Jobs.RecentFailed) != 3 {
		t.Errorf("recentFailed = %d jobs, want 3 (kept for other clients)", len(d.Jobs.RecentFailed))
	}

	// A paused type and a switched-off schedule do not count.
	if _, err := jf.svc.PauseJobType(ctx, fakeSchedule); err != nil {
		t.Fatal(err)
	}
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Jobs.NextRun == nil || d.Jobs.NextRun.Type != jobTraktRefresh {
		t.Errorf("next run with fake.schedule paused = %+v, %v, want %s", d.Jobs.NextRun, err, jobTraktRefresh)
	}
	jf.exec(t, `UPDATE job_schedules SET enabled = 0 WHERE type = ?`, jobTraktRefresh)
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Jobs.NextRun == nil || !d.Jobs.NextRun.At.Equal(jf.now.Add(10*time.Hour)) {
		t.Errorf("next run with trakt.refresh switched off = %+v, %v, want one in 10 h", d.Jobs.NextRun, err)
	}
	jf.exec(t, `UPDATE job_schedules SET enabled = 0`)
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Jobs.NextRun != nil {
		t.Errorf("next run without schedules = %+v, %v, want none", d.Jobs.NextRun, err)
	}
}

// Concept 11: attention lists only scans the loss protection aborted whose loss nobody accepted;
// the counts of the sidebar are kept for 60 s and recomputed at once after a change on the
// maintenance, libraries or accounts pages.
func TestDashboardAttentionAndCounts(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	jf.svc.WithSystem(SystemSource{ConfigDir: t.TempDir(), CacheDir: t.TempDir()})
	for _, q := range []string{
		`INSERT INTO libraries (id, name, kind, created_at, updated_at, last_scan_state, last_scan_error) VALUES (1, 'Filme', 'video', 0, 0, 'aborted', 'root_missing')`,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at, last_scan_state, last_scan_error) VALUES (2, 'Serien', 'video', 0, 0, 'aborted', 'cancelled')`,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at, last_scan_state, last_scan_error, loss_accepted_at) VALUES (3, 'Kinder', 'video', 0, 0, 'aborted', 'loss_over_threshold', 5)`,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at, last_scan_state, last_scan_error) VALUES (4, 'Doku', 'video', 0, 0, 'done', 'root_empty')`,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at, last_scan_state, last_scan_error) VALUES (5, 'Hörbücher', 'audiobooks', 0, 0, 'aborted', 'loss_over_threshold')`,
	} {
		jf.exec(t, q)
	}
	// A film rated 12 that is still pending since long and has no file: unmatched and orphaned.
	movie := func(id string, lib int) {
		t.Helper()
		jf.exec(t, `INSERT INTO items (id, kind, library_id, title, sort_title, own_rating, added_at, created_at, updated_at)
		            VALUES (?, 'movie', ?, 'Film', 'film', 12, 0, 0, 0)`, id, lib)
	}
	movie("01J000000000000000000000A1", 1)
	movie("01J000000000000000000000A2", 4)
	// CountAllLibraryItems equals the sum of LibraryScan.itemCount (CountLibraryItems) over every
	// library: seasons count like films, a soft-deleted title does not.
	jf.exec(t, `INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at, deleted_at)
	            VALUES ('01J000000000000000000000D1', 'movie', 2, 'Weg', 'weg', 0, 0, 0, 1)`)
	jf.exec(t, `INSERT INTO items (id, kind, library_id, title, sort_title, own_rating, added_at, created_at, updated_at)
	            VALUES ('01J000000000000000000000S1', 'series', 3, 'Serie', 'serie', 12, 0, 0, 0),
	                   ('01J000000000000000000000S2', 'season', 3, 'Staffel 1', 'staffel 1', 12, 0, 0, 0)`)
	jf.exec(t, `UPDATE items SET parent_id = '01J000000000000000000000S1', season_number = 1 WHERE id = '01J000000000000000000000S2'`)
	sum := int64(0)
	for id := int64(1); id <= 5; id++ {
		n, err := jf.svc.sys.CountLibraryItems(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		sum += n
	}
	if all, err := jf.svc.sys.CountAllLibraryItems(ctx); err != nil || all != sum || all != 4 {
		t.Fatalf("CountAllLibraryItems = %d, %v; the sum over the libraries is %d, want 4", all, err, sum)
	}
	jf.exec(t, `DELETE FROM items WHERE id IN ('01J000000000000000000000S2', '01J000000000000000000000S1', '01J000000000000000000000D1')`)
	if _, _, err := jf.auth.CreateOwner(ctx, "owner@example.ch", "FAKE-owner-password-0815", "de"); err != nil {
		t.Fatal(err)
	}

	d, err := jf.svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.AbortedScan{{LibraryID: 1, Name: "Filme", Reason: "root_missing"}, {LibraryID: 5, Name: "Hörbücher", Reason: "loss_over_threshold"}}
	if len(d.Attention.AbortedScans) != len(want) || d.Attention.AbortedScans[0] != want[0] || d.Attention.AbortedScans[1] != want[1] {
		t.Errorf("aborted scans = %+v, want %+v", d.Attention.AbortedScans, want)
	}
	// The five cheap counters: two unmatched and two orphaned films, nothing else.
	open := 0
	for _, kind := range []string{IssueUnmatched, IssueNoImages, IssueUnrated, IssueOrphaned, IssueUnplayable} {
		page, err := jf.svc.ListIssues(ctx, kind, nil, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		open += page.Total
	}
	first := model.DashboardCounts{Titles: 2, Accounts: 1, MaintenanceOpen: 4}
	if d.Counts != first || open != first.MaintenanceOpen {
		t.Fatalf("counts = %+v (issues %d), want %+v", d.Counts, open, first)
	}

	// Changes behind the service's back stay unseen for 60 s.
	movie("01J000000000000000000000A3", 1)
	jf.exec(t, `INSERT INTO accounts (email, password_hash, role, is_owner, created_at, updated_at) VALUES ('second@example.ch', 'x', 'user', 0, 0, 0)`)
	jf.now = jf.now.Add(59 * time.Second)
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Counts != first {
		t.Errorf("counts after 59 s = %+v, %v, want the kept %+v", d.Counts, err, first)
	}
	jf.now = jf.now.Add(time.Second)
	second := model.DashboardCounts{Titles: 3, Accounts: 2, MaintenanceOpen: 6}
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Counts != second {
		t.Errorf("counts after 60 s = %+v, %v, want %+v", d.Counts, err, second)
	}

	// A maintenance change recomputes them at once: the cleanup removes the orphaned third film.
	if items, _, err := jf.svc.CleanupItems(ctx, []string{"01J000000000000000000000A3"}, nil); err != nil || items != 1 {
		t.Fatalf("cleanup = %d, %v", items, err)
	}
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Counts != (model.DashboardCounts{Titles: 2, Accounts: 2, MaintenanceOpen: 4}) {
		t.Errorf("counts after the cleanup = %+v, %v", d.Counts, err)
	}
	// So does a change on the libraries page, and ForgetDashboardCounts (the accounts page).
	movie("01J000000000000000000000A4", 1)
	jf.libs.created = append(jf.libs.created, model.LibraryRequest{Name: "Filme", Kind: "video"}) // the fake's library 1
	if _, err := jf.svc.AcceptLibraryLoss(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Counts.Titles != 3 {
		t.Errorf("titles after a libraries change = %+v, %v, want 3", d.Counts, err)
	}
	jf.exec(t, `DELETE FROM accounts WHERE email = 'second@example.ch'`)
	jf.svc.ForgetDashboardCounts()
	if d, err = jf.svc.Dashboard(ctx); err != nil || d.Counts.Accounts != 1 {
		t.Errorf("accounts after ForgetDashboardCounts = %+v, %v, want 1", d.Counts, err)
	}
}

// Concept 11: one computation of the counts at a time. Without kept values the second caller
// waits for the running computation and gets its result; with expired values it gets those at
// once while the computation runs.
func TestDashboardCountsSingleFlight(t *testing.T) {
	f := newFixture(t)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return now }
	entered, release := make(chan struct{}, 2), make(chan struct{})
	var calls atomic.Int32
	f.svc.counts.compute = func(context.Context, time.Time) (model.DashboardCounts, error) {
		n := calls.Add(1)
		entered <- struct{}{}
		<-release
		return model.DashboardCounts{Titles: int(n)}, nil
	}
	ctx := context.Background()
	results := make(chan model.DashboardCounts, 2)
	call := func() {
		c, err := f.svc.dashboardCounts(ctx)
		if err != nil {
			t.Error(err)
		}
		results <- c
	}
	go call()
	<-entered
	go call()
	// The second caller cannot finish before the release: it waits for the first computation.
	select {
	case c := <-results:
		t.Fatalf("a caller finished before the computation: %+v", c)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	for range 2 {
		if c := <-results; c.Titles != 1 {
			t.Errorf("counts = %+v, want the one computation's", c)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("computations = %d, want 1", n)
	}

	// Expired: the first caller recomputes, a second one meanwhile gets the old values at once.
	now = now.Add(dashboardCountsTTL)
	release = make(chan struct{})
	go call()
	<-entered
	if c, err := f.svc.dashboardCounts(ctx); err != nil || c.Titles != 1 {
		t.Errorf("during the recomputation = %+v, %v, want the expired values", c, err)
	}
	close(release)
	if c := <-results; c.Titles != 2 {
		t.Errorf("recomputed counts = %+v, want the second computation's", c)
	}
	if c, err := f.svc.dashboardCounts(ctx); err != nil || c.Titles != 2 || calls.Load() != 2 {
		t.Errorf("after the recomputation = %+v, %v (%d computations), want the kept second values", c, err, calls.Load())
	}
}

// Concept 11: the first-start wizard creates an account; the counts follow at once.
func TestDashboardCountsAfterSetup(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	jf.svc.WithSystem(SystemSource{ConfigDir: t.TempDir(), CacheDir: t.TempDir()})
	if d, err := jf.svc.Dashboard(ctx); err != nil || d.Counts.Accounts != 0 {
		t.Fatalf("before the setup: %+v, %v", d.Counts, err)
	}
	if _, _, err := jf.svc.RunSetup(ctx, setupRequest(), "test"); err != nil {
		t.Fatal(err)
	}
	if d, err := jf.svc.Dashboard(ctx); err != nil || d.Counts.Accounts != 1 {
		t.Errorf("after the setup: %+v, %v, want one account at once", d.Counts, err)
	}
}
