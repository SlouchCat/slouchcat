package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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

	out, err := e.svc.DeleteLibrary(e.ctx, f.films.ID, "  Filme ", model.Actor{AccountID: 1, ProfileID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Filme" || out.Roots != 1 || out.Items != items || !strings.HasPrefix(out.Backup, "pre-library-delete-1.31.0-") {
		t.Errorf("deletion = %+v, want Filme, 1 root, %d items, a backup", out, items)
	}
	if _, err := os.Stat(filepath.Join(cfg, store.BackupDir, out.Backup)); err != nil {
		t.Errorf("backup: %v", err)
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
}
