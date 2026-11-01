package admin

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// activityFixture is the vistest catalogue (owner with P18 … P0, the guest account G with PG and
// PG18) with an admin service on a settable clock, logging into logs.
type activityFixture struct {
	f    *vistest.Fixture
	svc  *Service
	auth *auth.Service
	logs *bytes.Buffer
	now  time.Time
	ctx  context.Context
}

func newActivityFixture(t *testing.T) *activityFixture {
	t.Helper()
	f := vistest.New(t)
	f.AddGuest(t)
	var logs bytes.Buffer
	log, err := logging.New(&logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	a := auth.New(f.DB, log)
	af := &activityFixture{f: f, auth: a, logs: &logs, now: time.Date(2026, 11, 2, 10, 15, 0, 0, time.UTC), ctx: context.Background()}
	af.svc = New(f.DB, a, &dbLibraries{q: appq.New(f.DB.Read)}, ring, &fakeTMDB{}, func(string) string { return "" }, log)
	af.svc.now = func() time.Time { return af.now }
	return af
}

func (a *activityFixture) title(t *testing.T, itemID string) string {
	t.Helper()
	var title string
	if err := a.f.DB.Read.QueryRowContext(a.ctx, `SELECT title FROM items WHERE id = ?`, itemID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	return title
}

func (a *activityFixture) count(t *testing.T, where string, args ...any) int {
	t.Helper()
	var n int
	if err := a.f.DB.Read.QueryRowContext(a.ctx, `SELECT count(*) FROM activity_events WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Concept 11 (11.4), the flood rules: a profile's second start of a title within 30 minutes
// writes nothing (another profile or a later start does), scan_failed and scan_aborted once per
// library and hour, job_failed one row per job type and clock hour whose count grows. Unknown
// kinds and free error texts never reach the log.
func TestRecordActivityFloodRules(t *testing.T) {
	a := newActivityFixture(t)
	movie := a.f.Rated(12)
	start := func(p model.Profile) {
		a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: model.ActivityPlaybackStarted, AccountID: p.AccountID, ProfileID: p.ID, ItemID: movie.ID})
	}
	start(a.f.P18)
	a.now = a.now.Add(10 * time.Minute)
	start(a.f.P18)
	if n := a.count(t, `kind = 'playback_started'`); n != 1 {
		t.Fatalf("second start within 30 min: %d rows, want 1", n)
	}
	start(a.f.P16)
	a.now = a.now.Add(21 * time.Minute)
	start(a.f.P18)
	if n := a.count(t, `kind = 'playback_started'`); n != 3 {
		t.Errorf("another profile and a start after 31 min: %d rows, want 3", n)
	}
	if n := a.count(t, `kind = 'playback_started' AND subject = ? AND item_id = ?`, a.title(t, movie.ID), movie.ID); n != 3 {
		t.Errorf("subject read from the item: %d rows with %q", n, a.title(t, movie.ID))
	}

	lib := a.f.LibraryID()
	abort := func(reason string) {
		a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: model.ActivityScanAborted, LibraryID: lib, Subject: "Filme", Detail: model.ActivityDetail{Reason: reason}})
	}
	abort("root_missing")
	a.now = a.now.Add(30 * time.Minute)
	abort("root_empty")
	if n := a.count(t, `kind = 'scan_aborted'`); n != 1 {
		t.Errorf("second abort within the hour: %d rows, want 1", n)
	}
	a.now = a.now.Add(31 * time.Minute)
	abort("open /media/Filme: permission denied")
	if n := a.count(t, `kind = 'scan_aborted'`); n != 2 {
		t.Errorf("abort after an hour: %d rows, want 2", n)
	}
	if n := a.count(t, `kind = 'scan_aborted' AND json_extract(detail, '$.reason') = 'error'`); n != 1 {
		t.Error("a free error text was not replaced by the code error")
	}
	if n := a.count(t, `detail LIKE '%permission%' OR detail LIKE '%/media%'`); n != 0 {
		t.Error("the free error text reached the log")
	}

	a.now = time.Date(2026, 11, 2, 14, 5, 0, 0, time.UTC)
	fail := func(typ string) { a.svc.RecordJobFailure(a.ctx, jobs.Job{Type: typ}, nil) }
	fail("metadata.match")
	a.now = a.now.Add(20 * time.Minute)
	fail("metadata.match")
	fail("image.fetch")
	a.now = a.now.Add(30 * time.Minute)
	fail("metadata.match")
	var count int
	var at int64
	if err := a.f.DB.Read.QueryRowContext(a.ctx, `SELECT json_extract(detail, '$.count'), at FROM activity_events
		WHERE kind = 'job_failed' AND json_extract(detail, '$.jobType') = 'metadata.match'`).Scan(&count, &at); err != nil {
		t.Fatal(err)
	}
	if count != 3 || at != a.now.UnixMilli() {
		t.Errorf("job_failed of metadata.match: count %d at %d, want 3 at the newest failure %d", count, at, a.now.UnixMilli())
	}
	a.now = a.now.Add(10 * time.Minute) // 15:05, the next clock hour
	fail("metadata.match")
	if n := a.count(t, `kind = 'job_failed'`); n != 3 {
		t.Errorf("job_failed rows = %d, want metadata.match twice (two hours) and image.fetch once", n)
	}
	if n := a.count(t, `kind = 'job_failed' AND json_extract(detail, '$.count') = 1`); n != 2 {
		t.Errorf("job_failed rows with count 1 = %d, want 2", n)
	}

	before := a.count(t, `1`)
	a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: "something_new"})
	if a.count(t, `1`) != before || !strings.Contains(a.logs.String(), "activity not recorded") {
		t.Error("an unknown kind was written or not logged")
	}
}

// The dashboard reads the six newest events, with the e-mail address and the profile name read
// now, the item title for playback and the library name for scans.
func TestRecentActivity(t *testing.T) {
	a := newActivityFixture(t)
	movie := a.f.Rated(6)
	for i := range 8 {
		a.now = a.now.Add(time.Minute)
		a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: model.ActivityScanDone, LibraryID: a.f.LibraryID(), Subject: "Filme",
			Detail: model.ActivityDetail{NewTitles: &i}})
	}
	a.now = a.now.Add(time.Minute)
	a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: model.ActivityPlaybackStarted, AccountID: a.f.Account.ID, ProfileID: a.f.P6.ID, ItemID: movie.ID})
	got, err := a.svc.recentActivity(a.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != dashboardActivityLimit {
		t.Fatalf("entries = %d, want %d", len(got), dashboardActivityLimit)
	}
	p := got[0]
	if p.Kind != model.ActivityPlaybackStarted || p.ProfileName != "P6" || p.AccountEmail != a.f.Account.Email || p.ItemTitle != a.title(t, movie.ID) ||
		p.LibraryName != "" || !p.At.Equal(a.now) || p.At.Location() != time.UTC {
		t.Errorf("newest = %+v", p)
	}
	s := got[1]
	if s.Kind != model.ActivityScanDone || s.LibraryName != "Filme" || s.ItemTitle != "" || s.ProfileName != "" || s.Detail.NewTitles == nil || *s.Detail.NewTitles != 7 {
		t.Errorf("second = %+v", s)
	}
}

// The hooks in playback (concept 11): a start is written without delaying Start, a track switch
// (a new start of the same title within 30 minutes) writes nothing, a title newly completed by
// the 90 % rule is written once, and the administration's stops write one row per session.
func TestPlaybackActivity(t *testing.T) {
	a := newActivityFixture(t)
	var wg sync.WaitGroup
	release := make(chan struct{})
	hook := func(ctx context.Context, e model.ActivityEvent) {
		defer wg.Done()
		if e.Kind == model.ActivityPlaybackStarted {
			<-release
		}
		a.svc.RecordActivity(ctx, e)
	}
	pb := playback.New(a.f.DB, vistest.Logger()).WithActivity(hook)
	session := a.f.Session(t, a.f.P18)
	movie := a.f.Rated(12)

	wg.Add(1)
	done := make(chan error, 1)
	var first model.PlaybackInfo
	go func() {
		var err error
		first, err = pb.Start(a.ctx, session, movie.ID, model.PlaybackRequest{DeviceID: "tv"})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Start waited for the activity log")
	}
	close(release)
	wg.Wait()
	if n := a.count(t, `kind = 'playback_started' AND profile_id = ? AND item_id = ?`, a.f.P18.ID, movie.ID); n != 1 {
		t.Fatalf("start rows = %d, want 1", n)
	}
	// A track switch is a new start of the same title in the same browser session.
	wg.Add(1)
	if _, err := pb.Start(a.ctx, session, movie.ID, model.PlaybackRequest{DeviceID: "tv"}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if n := a.count(t, `kind = 'playback_started'`); n != 1 {
		t.Errorf("start rows after a track switch = %d, want 1", n)
	}

	report := func(pos int64) {
		t.Helper()
		if _, err := pb.Report(a.ctx, a.f.P18, movie.ID, model.ProgressEvent{
			PositionMs: pos, DurationMs: 100_000, State: "playing", ClientTime: time.Now(), Seq: pos,
		}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Add(1) // only the report that completes the title calls the hook (synchronously)
	report(50_000)
	report(95_000)
	report(97_000)
	wg.Wait()
	if n := a.count(t, `kind = 'playback_completed' AND profile_id = ? AND account_id = ? AND item_id = ?`, a.f.P18.ID, a.f.Account.ID, movie.ID); n != 1 {
		t.Errorf("completed rows = %d, want 1", n)
	}

	// The first playback session was replaced by the switch: stopping it finds nothing.
	if err := pb.StopStream(a.ctx, first.PlaybackSessionID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("stop of the replaced session = %v", err)
	}
	wg.Add(1) // the start of P6
	other, err := pb.Start(a.ctx, a.f.Session(t, a.f.P6), a.f.Rated(6).ID, model.PlaybackRequest{DeviceID: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	wg.Add(1) // its stop
	if err := pb.StopStream(a.ctx, other.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	wg.Add(1) // the running playback of P18 ends with its device
	if err := pb.EndDevice(a.ctx, "tv"); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if n := a.count(t, `kind = 'playback_stopped' AND profile_id = ? AND json_extract(detail, '$.by') = 'admin'`, a.f.P6.ID); n != 1 {
		t.Errorf("stopped rows of P6 = %d, want 1 by admin", n)
	}
	if n := a.count(t, `kind = 'playback_stopped' AND profile_id = ? AND item_id = ? AND json_extract(detail, '$.by') = 'revoked'`, a.f.P18.ID, movie.ID); n != 1 {
		t.Errorf("stopped rows of P18 = %d, want 1 by revoked", n)
	}
	if n := a.count(t, `kind = 'playback_stopped'`); n != 2 {
		t.Errorf("stopped rows = %d, want 2", n)
	}
}

// A write error of the activity log is logged as WARN and never aborts playback.Start.
func TestActivityWriteErrorDoesNotAbortStart(t *testing.T) {
	a := newActivityFixture(t)
	if _, err := a.f.DB.Write.ExecContext(a.ctx, `DROP TABLE activity_events`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	pb := playback.New(a.f.DB, vistest.Logger()).WithActivity(func(ctx context.Context, e model.ActivityEvent) {
		defer wg.Done()
		a.svc.RecordActivity(ctx, e)
	})
	wg.Add(1)
	info, err := pb.Start(a.ctx, a.f.Session(t, a.f.P18), a.f.Rated(12).ID, model.PlaybackRequest{})
	if err != nil || info.PlaybackSessionID == "" {
		t.Fatalf("Start with a broken activity log = %+v, %v", info, err)
	}
	wg.Wait()
	logs := a.logs.String()
	if !strings.Contains(logs, `"level":"WARN"`) || !strings.Contains(logs, "activity not recorded") || !strings.Contains(logs, "playback_started") {
		t.Errorf("no WARN for the failed write: %s", logs)
	}
}

// Deleting an account or a profile deletes its events (ADR 0019, privacy); an event of a deleted
// profile that arrives later is dropped.
func TestActivityDeletedWithAccountAndProfile(t *testing.T) {
	a := newActivityFixture(t)
	movie := a.f.Rated(6)
	for _, p := range []model.Profile{a.f.P18, a.f.P6, a.f.PG, a.f.PG18} {
		a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: model.ActivityPlaybackStarted, AccountID: p.AccountID, ProfileID: p.ID, ItemID: movie.ID})
	}
	a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: model.ActivityScanDone, LibraryID: a.f.LibraryID(), Subject: "Filme"})
	if n := a.count(t, `1`); n != 5 {
		t.Fatalf("rows = %d, want 5", n)
	}
	if err := a.auth.DeleteAccount(a.ctx, a.f.Guest.ID, nil); err != nil {
		t.Fatal(err)
	}
	if n := a.count(t, `account_id = ?`, a.f.Guest.ID); n != 0 {
		t.Errorf("rows of the deleted account = %d", n)
	}
	if err := a.auth.DeleteProfile(a.ctx, a.f.Session(t, a.f.P18), a.f.P6.ID, nil); err != nil {
		t.Fatal(err)
	}
	if n := a.count(t, `profile_id = ?`, a.f.P6.ID); n != 0 {
		t.Errorf("rows of the deleted profile = %d", n)
	}
	if n := a.count(t, `1`); n != 2 {
		t.Errorf("rows left = %d, want P18's start and the scan", n)
	}
	a.now = a.now.Add(time.Hour)
	a.svc.RecordActivity(a.ctx, model.ActivityEvent{Kind: model.ActivityPlaybackCompleted, AccountID: a.f.Guest.ID, ProfileID: a.f.PG.ID, ItemID: movie.ID})
	if n := a.count(t, `1`); n != 2 {
		t.Errorf("an event of a deleted profile was written")
	}
}

// The cleanup in jobs.prune keeps 30 days and at most 2'000 rows, in batches.
func TestPruneActivity(t *testing.T) {
	a := newActivityFixture(t)
	now := a.now.UnixMilli()
	day := int64(24 * time.Hour / time.Millisecond)
	// 2'100 events of the last hours and 600 events older than 30 days: two batches.
	if _, err := a.f.DB.Write.ExecContext(a.ctx, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 2100)
		INSERT INTO activity_events (at, kind) SELECT ? - i * 1000, 'server_started' FROM n`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := a.f.DB.Write.ExecContext(a.ctx, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 600)
		INSERT INTO activity_events (at, kind) SELECT ? - 31 * ? - i, 'server_started' FROM n`, now, day); err != nil {
		t.Fatal(err)
	}
	newest := now - 1000
	queue := jobs.NewQueue(a.f.DB.Write, func() time.Time { return a.now })
	queue.AddPruneTask(a.svc.PruneActivity)
	n, err := queue.Prune(a.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 700 {
		t.Errorf("pruned %d rows, want 700 (600 old ones and 100 beyond 2'000)", n)
	}
	if left := a.count(t, `1`); left != activityKeep {
		t.Errorf("rows left = %d, want %d", left, activityKeep)
	}
	if old := a.count(t, `at < ?`, now-30*day); old != 0 {
		t.Errorf("rows older than 30 days = %d", old)
	}
	if kept := a.count(t, `at = ?`, newest); kept != 1 {
		t.Error("the newest row went")
	}
	if n, err := queue.Prune(a.ctx); err != nil || n != 0 {
		t.Errorf("second prune = %d, %v; want nothing to do", n, err)
	}
}

// A manual backup is recorded with its size.
func TestBackupCreatedActivity(t *testing.T) {
	a := newActivityFixture(t)
	a.svc.WithBackups(t.TempDir(), "test")
	b, err := a.svc.CreateBackup(a.ctx, model.BackupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if n := a.count(t, `kind = 'backup_created' AND json_extract(detail, '$.sizeBytes') = ?`, b.SizeBytes); n != 1 {
		t.Errorf("backup_created rows with size %d = %d", b.SizeBytes, n)
	}
}
