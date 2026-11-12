package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
)

// deleteFixture is a film library with user data on its titles, a second library beside it, a
// restricted profile 2 that sees only the film library, and collections: one TMDB collection only
// in the film library, one shared with the other library, one made by hand.
type deleteFixture struct {
	e            *env
	dir          string
	films, other model.Library
	film         string // a title of the film library
}

func newDeleteFixture(t *testing.T) *deleteFixture {
	t.Helper()
	e := newEnv(t)
	otherDir := tree(t, []string{"Keep (2001)/Keep (2001).mkv"})
	other := e.create(otherDir)
	e.scan(other.ID)
	dir := tree(t, twentyMovies[4:10])
	films, err := e.svc.Create(e.ctx, testRequest("Filme", dir))
	if err != nil {
		t.Fatal(err)
	}
	e.scan(films.ID)
	if films.ID <= other.ID {
		t.Fatalf("film library %d must have the highest id (other %d)", films.ID, other.ID)
	}
	e.profiles(2)
	e.exec(`UPDATE profiles SET all_libraries = 0 WHERE id = 2`)
	e.exec(`INSERT INTO profile_libraries (profile_id, library_id) VALUES (2, ?)`, films.ID)
	film := e.str(`SELECT id FROM items WHERE library_id = ? AND kind = 'movie' ORDER BY id LIMIT 1`, films.ID)
	keep := e.str(`SELECT id FROM items WHERE library_id = ? AND kind = 'movie'`, other.ID)
	e.progress(1, film, 5000, false)
	e.progressEvent(film)
	e.exec(`INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (2, ?, 1, 1)`, film)
	e.exec(`INSERT INTO activity_events (at, kind, library_id, subject) VALUES (1, 'scan_done', ?, 'Filme')`, films.ID)
	e.exec(`INSERT INTO collections (id, source, tmdb_id, name, created_at, updated_at) VALUES
	        ('01COLLECTIONONLYFILMS00001', 'tmdb', 1, 'Only', 1, 1),
	        ('01COLLECTIONSHARED00000001', 'tmdb', 2, 'Shared', 1, 1),
	        ('01COLLECTIONMANUAL00000001', 'manual', NULL, 'Manual', 1, 1)`)
	e.exec(`INSERT INTO collection_items (collection_id, item_id, created_at) VALUES
	        ('01COLLECTIONONLYFILMS00001', ?, 1), ('01COLLECTIONSHARED00000001', ?, 1),
	        ('01COLLECTIONSHARED00000001', ?, 1), ('01COLLECTIONMANUAL00000001', ?, 1)`, film, film, keep, film)
	return &deleteFixture{e: e, dir: dir, films: films, other: other, film: film}
}

// Deleting the library with the highest id takes its titles and everything on them, never a file;
// a new library then gets the same id, and the restricted profile does not see it (the blocker of
// the review, section 9.3).
func TestDeleteLibraryTakesEverythingButTheFiles(t *testing.T) {
	t.Parallel()
	f := newDeleteFixture(t)
	e := f.e
	cfg := t.TempDir()
	WithBackups(cfg, "1.31.0")(e.svc)
	var events []model.ActivityEvent
	e.svc.WithActivity(func(_ context.Context, ev model.ActivityEvent) { events = append(events, ev) })
	queued := e.count(`SELECT count(*) FROM jobs WHERE state = 'queued' AND (json_extract(payload, '$.itemId') IN (SELECT id FROM items WHERE library_id = ?)
	                   OR json_extract(payload, '$.mediaFileId') IN (SELECT m.id FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id WHERE r.library_id = ?))`,
		f.films.ID, f.films.ID)
	if queued == 0 {
		t.Fatal("the scan queued no job for the titles")
	}
	items := e.count(`SELECT count(*) FROM items WHERE library_id = ?`, f.films.ID)

	asked, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "  Filme ", model.Actor{AccountID: 1, ProfileID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if asked.Name != "Filme" || asked.JobID == 0 {
		t.Errorf("asked = %+v", asked)
	}
	e.runLibraryDelete(asked.JobID)
	backups, err := filepath.Glob(filepath.Join(cfg, store.BackupDir, "pre-library-delete-1.31.0-*.db"))
	if err != nil || len(backups) != 1 {
		t.Errorf("backups = %v, %v; want one", backups, err)
	}
	for what, query := range map[string]string{
		"titles":            `SELECT count(*) FROM items WHERE library_id = ?`,
		"folders":           `SELECT count(*) FROM library_roots WHERE library_id = ?`,
		"grants":            `SELECT count(*) FROM profile_libraries WHERE library_id = ?`,
		"home row":          `SELECT count(*) FROM home_rows WHERE library_id = ?`,
		"activity of it":    `SELECT count(*) FROM activity_events WHERE library_id = ?`,
		"jobs of its title": `SELECT count(*) FROM jobs WHERE state IN ('queued', 'running') AND json_extract(payload, '$.itemId') IN (SELECT id FROM items WHERE library_id = ?)`,
	} {
		if n := e.count(query, f.films.ID); n != 0 {
			t.Errorf("%s left: %d", what, n)
		}
	}
	for what, query := range map[string]string{
		"progress":          `SELECT count(*) FROM progress WHERE item_id = ?`,
		"progress events":   `SELECT count(*) FROM progress_events WHERE item_id = ?`,
		"watchlist":         `SELECT count(*) FROM watchlist WHERE item_id = ?`,
		"collection member": `SELECT count(*) FROM collection_items WHERE item_id = ?`,
	} {
		if n := e.count(query, f.film); n != 0 {
			t.Errorf("%s left: %d", what, n)
		}
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE state = 'cancelled'`); n < queued {
		t.Errorf("cancelled jobs = %d, want at least the %d of the titles", n, queued)
	}
	if got := e.str(`SELECT group_concat(name, ',') FROM (SELECT name FROM collections ORDER BY name)`); got != "Manual,Shared" {
		t.Errorf("collections = %s, want the manual one (empty) and the shared one", got)
	}
	if n := e.count(`SELECT count(*) FROM activity_events WHERE kind = 'scan_done' AND library_id IS NULL AND subject = 'Filme'`); n != 1 {
		t.Errorf("the old event keeps its name without the library: %d", n)
	}
	if len(events) != 1 || events[0].Kind != model.ActivityLibraryDeleted || events[0].Subject != "Filme" ||
		events[0].AccountID != 1 || events[0].Detail.Count != items {
		t.Errorf("activity = %+v", events)
	}
	// Not a single file went.
	for _, rel := range twentyMovies[4:10] {
		if _, err := os.Stat(filepath.Join(f.dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("file %s: %v", rel, err)
		}
	}
	// The other library stays as it was.
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NULL`, f.other.ID); n == 0 {
		t.Error("the other library lost its titles")
	}

	// A new library on the same folder takes the free id; the restricted profile was granted only
	// the deleted one and sees nothing of it.
	again, err := e.svc.Create(e.ctx, testRequest("Neu", f.dir))
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != f.films.ID {
		t.Fatalf("new library id %d, want the reused %d", again.ID, f.films.ID)
	}
	e.scan(again.ID)
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NULL`, again.ID); n == 0 {
		t.Fatal("the scan of the new library found nothing")
	}
	if n := e.count(`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id WHERE v.profile_id = 2 AND i.library_id = ?`, again.ID); n != 0 {
		t.Errorf("the restricted profile sees %d titles of the new library", n)
	}
	if n := e.count(`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id WHERE v.profile_id = 1 AND i.library_id = ?`, again.ID); n == 0 {
		t.Error("the profile with every library sees nothing of the new one")
	}
}

// A wrong name, an unknown library and a running playback leave everything as it was (O-13: the
// delete is refused, the playback goes on and the error names the profile).
func TestDeleteLibraryRefuses(t *testing.T) {
	t.Parallel()
	f := newDeleteFixture(t)
	e := f.e
	before := e.count(`SELECT count(*) FROM items WHERE library_id = ?`, f.films.ID)
	if _, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "filme", model.Actor{}); !errors.Is(err, model.ErrConfirmMismatch) {
		t.Errorf("wrong case: %v, want confirm mismatch", err)
	}
	if _, err := e.svc.DeleteLibrary(e.ctx, 999, "Filme", model.Actor{}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown library: %v", err)
	}
	now := e.clock.Now().UnixMilli()
	e.exec(`UPDATE sessions SET expires_at = ? WHERE id = 1`, now+int64(time.Hour/time.Millisecond))
	e.exec(`UPDATE profiles SET name = 'Anna' WHERE id = 1`)
	e.exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at)
	        VALUES ('01PLAYING00000000000000001', 1, 1, 1, ?, 'f', 'direct', ?, ?, ?)`, f.film, now, now, now+60000)
	_, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "Filme", model.Actor{})
	var playing *model.PlayingError
	if !errors.As(err, &playing) || !errors.Is(err, model.ErrPlaying) || !slices.Equal(playing.Profiles, []string{"Anna"}) {
		t.Errorf("while playing: %v, want the profile Anna", err)
	}
	if n := e.count(`SELECT count(*) FROM playback_sessions WHERE revoked_at IS NULL`); n != 1 {
		t.Errorf("the playback was stopped: %d open", n)
	}
	// A stale playback (no request for longer than the window) no longer counts.
	e.clock.Advance(3 * time.Minute)
	e.exec(`UPDATE playback_sessions SET expires_at = ?`, e.clock.Now().UnixMilli()+60000)
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ?`, f.films.ID); n != before {
		t.Errorf("titles after the refusals = %d, want %d", n, before)
	}
	if _, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "Filme", model.Actor{}); err != nil {
		t.Errorf("after the playback went quiet: %v", err)
	}
	// A second request finds the library being deleted.
	if _, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "Filme", model.Actor{}); !errors.Is(err, model.ErrDeleting) {
		t.Errorf("second request: %v, want deleting", err)
	}
}

// runLibraryDelete runs the queued job library.delete as a worker would and checks that it ends
// without error.
func (e *env) runLibraryDelete(jobID int64) {
	e.t.Helper()
	j, err := e.queue.Get(e.ctx, jobID)
	if err != nil {
		e.t.Fatal(err)
	}
	if j.Type != JobLibraryDelete || j.State != jobs.StateQueued {
		e.t.Fatalf("job = %s %s", j.Type, j.State)
	}
	if err := e.svc.handleLibraryDelete(e.ctx, &j); err != nil {
		e.t.Fatalf("library.delete: %v", err)
	}
}

// Concept 20 section 9.3 (acceptance on CT 190): the request marks the library and queues the job;
// from that moment the library and its titles are gone for every view, no scan of it runs, and a
// failure of the job before its transaction gives the library back. A run that repeats after a
// stop of the server (the library already gone) ends without error.
func TestDeleteLibraryInTheBackground(t *testing.T) {
	t.Parallel()
	f := newDeleteFixture(t)
	e := f.e
	queued, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "Filme", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(e.ctx, f.films.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("Get of a deleting library = %v, want not found", err)
	}
	libs, err := e.svc.List(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(libs, func(l model.Library) bool { return l.ID == f.films.ID }) {
		t.Error("the list still shows the library")
	}
	if n := e.count(`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id WHERE i.library_id = ?`, f.films.ID); n != 0 {
		t.Errorf("visible titles of a deleting library: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id WHERE i.library_id = ?`, f.other.ID); n == 0 {
		t.Error("the other library is gone from view")
	}
	// A scan asked for now does nothing to it.
	if _, err := e.svc.QueueScan(e.ctx, f.films.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("scan of a deleting library = %v", err)
	}

	// A failure before the transaction: the playback check of the job (a playback that began
	// before the mark) gives the library back, the run ends final.
	now := e.clock.Now().UnixMilli()
	e.exec(`UPDATE sessions SET expires_at = ? WHERE id = 1`, now+3600000)
	e.exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at)
	        VALUES ('01PLAYING00000000000000002', 1, 1, 1, ?, 'f', 'direct', ?, ?, ?)`, f.film, now, now, now+60000)
	j, err := e.queue.Get(e.ctx, queued.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleLibraryDelete(e.ctx, &j); err == nil {
		t.Fatal("the job went through while a profile played")
	}
	if _, err := e.svc.Get(e.ctx, f.films.ID); err != nil {
		t.Errorf("the library is not back after the failed job: %v", err)
	}

	// Again, without the playback: the job deletes it; a second run finds nothing to do.
	e.exec(`UPDATE playback_sessions SET revoked_at = ?`, now)
	e.exec(`DELETE FROM jobs WHERE id = ?`, queued.JobID)
	queued, err = e.svc.DeleteLibrary(e.ctx, f.films.ID, "Filme", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	e.runLibraryDelete(queued.JobID)
	if n := e.count(`SELECT count(*) FROM libraries WHERE id = ?`, f.films.ID); n != 0 {
		t.Error("the library is still there")
	}
	j, err = e.queue.Get(e.ctx, queued.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleLibraryDelete(e.ctx, &j); err != nil {
		t.Errorf("a repeated run: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE id = ? AND state = 'cancelled'`, queued.JobID); n != 0 {
		t.Error("the job cancelled itself")
	}
}

// A library marked for deletion whose job is gone (dismissed on the jobs page) gets a new job at
// the start of the server; a cancelled job gives the library back.
func TestDeleteLibraryRequeuedAndCancelled(t *testing.T) {
	t.Parallel()
	f := newDeleteFixture(t)
	e := f.e
	queued, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "Filme", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE jobs SET state = 'failed' WHERE id = ?`, queued.JobID)
	e.svc.requeueLibraryDeletes(e.ctx)
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND idempotency_key = ?`, JobLibraryDelete, libraryDeleteKey(f.films.ID)); n != 1 {
		t.Fatalf("queued deletes after the start = %d, want 1", n)
	}
	id := e.str(`SELECT id FROM jobs WHERE type = ? AND state = 'queued'`, JobLibraryDelete)
	j, err := e.queue.Get(e.ctx, mustInt(t, id))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.cancelLibraryDelete(e.ctx, j); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(e.ctx, f.films.ID); err != nil {
		t.Errorf("the library is not back after the cancel: %v", err)
	}
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
