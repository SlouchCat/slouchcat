package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// filmLibrary creates a movie library with twelve films, so the loss of one stays below the
// threshold, and scans it.
func (e *env) filmLibrary() (string, model.Library) {
	e.t.Helper()
	dir := tree(e.t, twentyMovies[4:16])
	lib := e.create(dir)
	e.scan(lib.ID)
	return dir, lib
}

// addFilm writes the file rel, scans, gives its new film the TMDB id tmdb and returns the film.
// Each file gets its own size: the file system may give a new file the inode of one removed
// before, and with the same size the scanner would take it for that file moved (dev, inode, size).
func (e *env) addFilm(dir string, libID int64, rel string, tmdb int64) string {
	e.t.Helper()
	e.films++
	writeFile(e.t, dir, rel, fmt.Sprintf("release %-100s", rel)+strings.Repeat("+", e.films))
	e.scan(libID)
	id := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, rel)
	e.exec(`UPDATE items SET tmdb_id = ? WHERE id = ?`, tmdb, id)
	return id
}

// dropFilm removes the file rel; the scan marks it missing and the sweep a day later soft-deletes
// its film. The clock moves on a minute after the sweep.
func (e *env) dropFilm(dir string, libID int64, rel string) {
	e.t.Helper()
	remove(e.t, dir, rel)
	e.scan(libID)
	e.clock.Advance(25 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 1 {
		e.t.Fatalf("sweep = %d, %v; want the film", n, err)
	}
	e.clock.Advance(time.Minute)
}

func (e *env) fileOf(rel string) string {
	e.t.Helper()
	return e.str(`SELECT id FROM media_files WHERE rel_path = ?`, rel)
}

// Three generations of a film: A was deleted first, T later, and C is the live copy. C merges into
// T, the most recently deleted twin, and A folds into T in the same transaction, so the match of T
// afterwards finds no older twin. T keeps its id and holds A's user data.
func TestRelinkFoldsOlderTwins(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	e.profiles(2)
	a := e.addFilm(dir, lib.ID, "Heat A/Heat.mkv", 949)
	e.progress(1, a, 100, true)
	e.exec(`INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (2, ?, 1, 1)`, a)
	e.dropFilm(dir, lib.ID, "Heat A/Heat.mkv")
	e.clock.Advance(24 * time.Hour)
	tt := e.addFilm(dir, lib.ID, "Heat T/Heat.mkv", 949)
	e.exec(`UPDATE items SET imdb_id = 'tt0113277' WHERE id = ?`, tt)
	e.progress(2, tt, 300, false)
	e.dropFilm(dir, lib.ID, "Heat T/Heat.mkv")
	e.clock.Advance(24 * time.Hour)
	c := e.addFilm(dir, lib.ID, "Heat C/Heat.mkv", 949)
	e.progress(1, c, 50, false)

	if twin, err := e.relinkJob(c); err != nil || !twin {
		t.Fatalf("relink = %v, %v", twin, err)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id IN (?, ?)`, a, c); n != 0 {
		t.Errorf("older twin or copy still exist: %d rows", n)
	}
	if v := e.str(`SELECT (deleted_at IS NULL) || '/' || available FROM items WHERE id = ?`, tt); v != "1/1" {
		t.Errorf("survivor live/available = %s, want 1/1", v)
	}
	progress := e.str(`SELECT group_concat(profile_id || ':' || item_id || ':' || position_ms || ':' || completed, ' ')
	                   FROM (SELECT * FROM progress ORDER BY profile_id)`)
	if want := "1:" + tt + ":100:1 2:" + tt + ":300:0"; progress != want {
		t.Errorf("progress = %s, want %s", progress, want)
	}
	if n := e.count(`SELECT count(*) FROM watchlist WHERE profile_id = 2 AND item_id = ?`, tt); n != 1 {
		t.Error("watchlist entry of the older twin lost")
	}
	// The survivor's own match runs the hook again: no older twin is left.
	if twin, err := e.svc.QueueRelink(e.ctx, tt); err != nil || twin {
		t.Errorf("twin of the survivor = %v, %v; want none", twin, err)
	}
	if pairs, err := e.svc.ListRelinkPairs(e.ctx, 0); err != nil || len(pairs) != 0 {
		t.Errorf("pairs = %+v, %v", pairs, err)
	}
}

// The automatic relink takes a copy created up to relinkGrace before its twin was deleted (a new
// folder scanned before the old one is removed, decision 21); a copy that lived beside the twin
// for longer stays apart. The maintenance action knows no freshness rule: it lists that pair and
// merges it.
func TestRelinkFreshnessGrace(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	heat := e.addFilm(dir, lib.ID, "Heat/Heat.mkv", 949)
	heatCopy := e.addFilm(dir, lib.ID, "Heat Copy/Heat.mkv", 949)
	alien := e.addFilm(dir, lib.ID, "Alien/Alien.mkv", 348)
	alienCopy := e.addFilm(dir, lib.ID, "Alien Copy/Alien.mkv", 348)
	e.dropFilm(dir, lib.ID, "Heat/Heat.mkv")
	e.dropFilm(dir, lib.ID, "Alien/Alien.mkv")
	createdBefore := func(copyID, deadID string, d time.Duration) {
		e.exec(`UPDATE items SET created_at = (SELECT deleted_at FROM items WHERE id = ?) - ? WHERE id = ?`, deadID, d.Milliseconds(), copyID)
	}
	createdBefore(heatCopy, heat, time.Hour)
	createdBefore(alienCopy, alien, 8*24*time.Hour)

	if twin, err := e.relinkJob(alienCopy); err != nil || twin {
		t.Errorf("twin of a copy created 8 days before = %v, %v; want none", twin, err)
	}
	pairs, err := e.svc.ListRelinkPairs(e.ctx, 0)
	if err != nil || len(pairs) != 2 {
		t.Fatalf("pairs = %+v, %v; want both copies", pairs, err)
	}
	if twin, err := e.relinkJob(heatCopy); err != nil || !twin {
		t.Fatalf("twin of a copy created an hour before = %v, %v", twin, err)
	}
	if v := e.str(`SELECT (deleted_at IS NULL) || '/' || (SELECT count(*) FROM items WHERE id = ?) FROM items WHERE id = ?`, heatCopy, heat); v != "1/0" {
		t.Errorf("film live / copy rows = %s, want 1/0", v)
	}
	if v := e.str(`SELECT (deleted_at IS NULL) FROM items WHERE id = ?`, alien); v != "0" {
		t.Error("the automatic relink restored the film with the old copy")
	}

	if libID, err := e.svc.CheckRelinkPair(e.ctx, alien, alienCopy); err != nil || libID != lib.ID {
		t.Errorf("check of the old copy = %v, %v; want library %d", libID, err, lib.ID)
	}
	if _, err := e.svc.Relink(e.ctx, alien, alienCopy); err != nil {
		t.Fatalf("maintenance relink of the old copy = %v", err)
	}
	if v := e.str(`SELECT (deleted_at IS NULL) || '/' || (SELECT count(*) FROM items WHERE id = ?) FROM items WHERE id = ?`, alienCopy, alien); v != "1/0" {
		t.Errorf("film live / copy rows = %s, want 1/0", v)
	}
}

// Two copies of one deleted title: the maintenance list shows both pairs; the first merges, the
// second is no pair any more, since its deleted title is back, and stays as a live duplicate. An
// unknown id names no film or series.
func TestRelinkTwoCopiesOfOneTitle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	heat := e.addFilm(dir, lib.ID, "Heat/Heat.mkv", 949)
	e.dropFilm(dir, lib.ID, "Heat/Heat.mkv")
	first := e.addFilm(dir, lib.ID, "Heat A/Heat.mkv", 949)
	second := e.addFilm(dir, lib.ID, "Heat B/Heat.mkv", 949)

	pairs, err := e.svc.ListRelinkPairs(e.ctx, lib.ID)
	if err != nil || len(pairs) != 2 || pairs[0].DeletedID != heat || pairs[1].DeletedID != heat {
		t.Fatalf("pairs = %+v, %v; want both copies of the deleted film", pairs, err)
	}
	if pairs[0].DeletedTitle != "Heat" {
		t.Errorf("deleted title = %q", pairs[0].DeletedTitle)
	}
	for _, c := range []string{first, second} {
		if _, err := e.svc.CheckRelinkPair(e.ctx, heat, c); err != nil {
			t.Fatalf("check of %s = %v", c, err)
		}
	}
	if _, err := e.svc.Relink(e.ctx, heat, first); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CheckRelinkPair(e.ctx, heat, second); !errors.Is(err, ErrNotTwins) {
		t.Errorf("check of the second copy = %v, want ErrNotTwins", err)
	}
	if _, err := e.svc.Relink(e.ctx, heat, second); !errors.Is(err, ErrNotTwins) {
		t.Errorf("relink of the second copy = %v, want ErrNotTwins", err)
	}
	if v := e.str(`SELECT (deleted_at IS NULL) FROM items WHERE id = ?`, second); v != "1" {
		t.Error("the second copy is gone")
	}
	if pairs, err := e.svc.ListRelinkPairs(e.ctx, lib.ID); err != nil || len(pairs) != 0 {
		t.Errorf("pairs after the merge = %+v, %v", pairs, err)
	}
	if _, err := e.svc.CheckRelinkPair(e.ctx, heat, "01ZZZZZZZZZZZZZZZZZZZZZZZZ"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("check of an unknown id = %v, want ErrNotFound", err)
	}
}

// The copy's curated state survives the merge: its stricter age rating with its source, its
// locked fields with their values, the administrator's image choice over the survivor's automatic
// one, and image choices of a kind the survivor has none of.
func TestRelinkKeepsCuratedStateOfCopy(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	dead := e.addFilm(dir, lib.ID, "Heat/Heat.mkv", 949)
	e.exec(`UPDATE items SET own_rating = 12, rating_source = 'tmdb:DE' WHERE id = ?`, dead)
	e.exec(`INSERT INTO item_images (item_id, kind, provider, source, chosen, updated_at) VALUES
	        (?, 'poster', 'tmdb', '/old.jpg', 0, 1), (?, 'backdrop', 'tmdb', '/old-b.jpg', 0, 1)`, dead, dead)
	e.dropFilm(dir, lib.ID, "Heat/Heat.mkv")
	c := e.addFilm(dir, lib.ID, "Heat Copy/Heat.mkv", 949)
	e.exec(`UPDATE items SET own_rating = 16, rating_source = 'tmdb:US', imdb_id = 'tt0113277',
	        locked_fields = '["poster","imdb_id","title"]' WHERE id = ?`, c)
	e.exec(`INSERT INTO item_images (item_id, kind, provider, source, chosen, updated_at) VALUES
	        (?, 'poster', 'manual', 'upload.jpg', 1, 1), (?, 'backdrop', 'tmdb', '/new-b.jpg', 0, 1),
	        (?, 'logo', 'tmdb', '/logo.png', 0, 1)`, c, c, c)

	if twin, err := e.relinkJob(c); err != nil || !twin {
		t.Fatalf("relink = %v, %v", twin, err)
	}
	if v := e.str(`SELECT own_rating || '/' || rating_source || '/' || effective_rating || '/' || imdb_id || '/' || locked_fields
	               FROM items WHERE id = ?`, dead); v != `16/tmdb:US/16/tt0113277/["poster","imdb_id"]` {
		t.Errorf("rating/source/effective/imdb/locks = %s", v)
	}
	images := e.str(`SELECT group_concat(kind || ':' || source || ':' || chosen, ' ') FROM (SELECT * FROM item_images WHERE item_id = ? ORDER BY kind)`, dead)
	if want := "backdrop:/old-b.jpg:0 logo:/logo.png:0 poster:upload.jpg:1"; images != want {
		t.Errorf("images = %s, want %s", images, want)
	}
}

// The merged title is never milder than the copy, unless the administrator locked the copy's
// rating (decision 21): a locked rating of the copy wins, even over a locked one of the survivor;
// otherwise the stricter wins, even over a locked rating of the survivor, whose lock then holds
// the stricter value. A copy without a rating leaves the survivor's alone.
func TestRelinkRatingNeverMilderThanCopy(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	rating := func(id string, v any, locks string) {
		e.exec(`UPDATE items SET own_rating = ?, rating_source = CASE WHEN ? IS NULL THEN NULL ELSE 'tmdb:' || ? END,
		        locked_fields = ? WHERE id = ?`, v, v, id, locks, id)
	}
	for i, tc := range []struct {
		name                     string
		dead, copy               any // own_rating, nil for none
		deadLocks, copyLocks     string
		wantRating, wantSourceOf string // wantSourceOf: "dead" or "copy"
		wantLocks                string
	}{
		{"survivor locked, copy stricter", 6, 16, `["own_rating"]`, `[]`, "16", "copy", `["own_rating"]`},
		{"both locked", 6, 16, `["own_rating"]`, `["own_rating"]`, "16", "copy", `["own_rating"]`},
		{"copy locked milder", 16, 6, `[]`, `["own_rating"]`, "6", "copy", `["own_rating"]`},
		{"copy without rating", 12, nil, `[]`, `[]`, "12", "dead", `[]`},
		{"copy milder", 16, 12, `[]`, `[]`, "16", "dead", `[]`},
	} {
		rel := fmt.Sprintf("Film %d/Film.mkv", i)
		dead := e.addFilm(dir, lib.ID, rel, int64(9000+i))
		rating(dead, tc.dead, tc.deadLocks)
		e.dropFilm(dir, lib.ID, rel)
		c := e.addFilm(dir, lib.ID, fmt.Sprintf("Film %d Copy/Film.mkv", i), int64(9000+i))
		rating(c, tc.copy, tc.copyLocks)
		if twin, err := e.relinkJob(c); err != nil || !twin {
			t.Fatalf("%s: relink = %v, %v", tc.name, twin, err)
		}
		source := map[string]string{"dead": "tmdb:" + dead, "copy": "tmdb:" + c}[tc.wantSourceOf]
		want := tc.wantRating + "/" + source + "/" + tc.wantRating + "/" + tc.wantLocks
		if v := e.str(`SELECT own_rating || '/' || rating_source || '/' || effective_rating || '/' || locked_fields
		               FROM items WHERE id = ?`, dead); v != want {
			t.Errorf("%s: rating/source/effective/locks = %s, want %s", tc.name, v, want)
		}
	}
}

// The order "add the new folder, scan, remove the old one" (ADR 0023): the copy in the new folder
// is matched while the film still lives, so its match finds no twin. Removing the old folder
// soft-deletes the film and queues library.relink for the fresh copy in the same transaction;
// the job merges the copy into the film, whose id survives.
func TestRelinkQueuedWhenOriginalFolderRemoved(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.profiles(1)
	oldDir := tree(t, []string{"Heat/Heat.mkv"})
	lib := e.create(oldDir)
	e.scan(lib.ID)
	film := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat/Heat.mkv'`)
	e.exec(`UPDATE items SET tmdb_id = 949 WHERE id = ?`, film)
	e.progress(1, film, 4200, false)

	newDir := t.TempDir()
	writeFile(t, newDir, "Heat/Heat.mkv", "the same film in a new folder")
	if _, err := e.svc.AddRoots(e.ctx, lib.ID, rootReq(newDir, model.RootMovie)); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	newRoot := rootIDOf(t, e, lib.ID, newDir)
	c := e.str(`SELECT item_id FROM media_files WHERE root_id = ?`, newRoot)
	if c == "" || c == film {
		t.Fatalf("copy = %q, film = %q; want a new title", c, film)
	}
	e.exec(`UPDATE items SET tmdb_id = 949 WHERE id = ?`, c)
	if twin, err := e.svc.QueueRelink(e.ctx, c); err != nil || twin {
		t.Fatalf("twin while the film lives = %v, %v; want none", twin, err)
	}

	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootIDOf(t, e, lib.ID, oldDir), oldDir); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND state = 'queued'`, JobRelink, relinkKey(c)); n != 1 {
		t.Fatalf("library.relink jobs of the copy = %d, want 1", n)
	}
	payload, err := json.Marshal(itemPayload{c})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleRelink(e.ctx, &jobs.Job{Type: JobRelink, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT (deleted_at IS NULL) || '/' || (SELECT count(*) FROM items WHERE id = ?) || '/' ||
	               (SELECT item_id FROM media_files WHERE root_id = ?) FROM items WHERE id = ?`, c, newRoot, film); v != "1/0/"+film {
		t.Errorf("film live / copy rows / owner of the new file = %s, want 1/0/%s", v, film)
	}
	if v := e.str(`SELECT position_ms FROM progress WHERE profile_id = 1 AND item_id = ?`, film); v != "4200" {
		t.Errorf("progress of the film = %q, want 4200", v)
	}
}

// A merge by library.relink rechecks the open streams (WithVisibilityRecheck). A relink whose last
// attempt fails, or that the administration cancels, leaves the copy unmerged: its match runs again
// with noRelink, so it completes the title instead of handing it back; a queued match of the title
// takes noRelink over, a running one is left alone. An earlier attempt that fails is only retried,
// and a stopped context (pause, shutdown) is no failure: the job runs again.
func TestRelinkJobRecheckAndFallbackMatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	rechecks := 0
	e.svc.recheck = func(context.Context) error { rechecks++; return nil }
	pair := func(i int) string {
		rel := fmt.Sprintf("Film %d/Film.mkv", i)
		e.addFilm(dir, lib.ID, rel, int64(9100+i))
		e.dropFilm(dir, lib.ID, rel)
		c := e.addFilm(dir, lib.ID, fmt.Sprintf("Film %d Copy/Film.mkv", i), int64(9100+i))
		e.exec(`DELETE FROM jobs`)
		return c
	}
	job := func(id string, attempts int) *jobs.Job {
		payload, err := json.Marshal(itemPayload{id})
		if err != nil {
			t.Fatal(err)
		}
		return &jobs.Job{Type: JobRelink, Payload: payload, Attempts: attempts, MaxAttempts: jobs.DefaultMaxAttempts}
	}
	matchPayload := func(id string) string {
		return e.str(`SELECT (SELECT payload FROM jobs WHERE type = ? AND idempotency_key = ?)`, JobMetadataMatch, "match:"+id)
	}

	merged := pair(0)
	if err := e.svc.handleRelink(e.ctx, job(merged, 1)); err != nil {
		t.Fatal(err)
	}
	if rechecks != 1 {
		t.Errorf("rechecks after a merge = %d, want 1", rechecks)
	}

	failing, running := pair(1), pair(3)
	// The merge fails in its transaction while the context lives.
	e.exec(`CREATE TRIGGER relink_fails BEFORE UPDATE OF locked_fields ON items BEGIN SELECT RAISE(ABORT, 'relink fails'); END`)
	if err := e.svc.handleRelink(e.ctx, job(failing, 1)); err == nil {
		t.Fatal("failing relink succeeded")
	}
	if p := matchPayload(failing); p != "" {
		t.Errorf("match after a failed first attempt = %s, want none", p)
	}
	stopped, cancel := context.WithCancel(e.ctx)
	cancel()
	if err := e.svc.handleRelink(stopped, job(failing, jobs.DefaultMaxAttempts)); err == nil {
		t.Fatal("relink with a stopped context succeeded")
	}
	if p := matchPayload(failing); p != "" {
		t.Errorf("match after a stopped last attempt = %s, want none", p)
	}
	queueMatch := func(id string) int64 {
		m := matchJob(id)
		payload, err := json.Marshal(m.payload)
		if err != nil {
			t.Fatal(err)
		}
		jobID, err := e.svc.queue.Enqueue(e.ctx, jobs.NewJob{Type: m.typ, Priority: m.priority, IdempotencyKey: m.key, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		return jobID
	}
	queueMatch(failing)
	if err := e.svc.handleRelink(e.ctx, job(failing, jobs.DefaultMaxAttempts)); err == nil {
		t.Fatal("failing relink succeeded")
	}
	if p, want := matchPayload(failing), `{"itemId":"`+failing+`","noRelink":true}`; p != want {
		t.Errorf("queued match after the last attempt = %s, want %s", p, want)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE idempotency_key = ?`, "match:"+failing); n != 1 {
		t.Errorf("match jobs of the failing title = %d, want 1", n)
	}
	e.exec(`UPDATE jobs SET state = 'running' WHERE id = ?`, queueMatch(running))
	if err := e.svc.handleRelink(e.ctx, job(running, jobs.DefaultMaxAttempts)); err == nil {
		t.Fatal("failing relink succeeded")
	}
	if p, want := matchPayload(running), `{"itemId":"`+running+`"}`; p != want {
		t.Errorf("running match after the last attempt = %s, want %s", p, want)
	}
	e.exec(`DROP TRIGGER relink_fails`)

	cancelled := pair(2)
	if err := e.svc.cancelRelink(e.ctx, *job(cancelled, 0)); err != nil {
		t.Fatal(err)
	}
	if p, want := matchPayload(cancelled), `{"itemId":"`+cancelled+`","noRelink":true}`; p != want {
		t.Errorf("match after a cancel = %s, want %s", p, want)
	}
	if rechecks != 1 {
		t.Errorf("rechecks = %d, want 1 (no merge since)", rechecks)
	}
}

// User data that both titles hold for the same key: one row survives per unique key (watchlist,
// notifications, downloads per device), condensed days add up, the further progress wins with its
// playback rate while the later watched_at wins on its own (concept 20 section 6.3: the dead
// title was finished once and started again), and a dead download of the survivor gives way to the copy's working one for the
// same device, whose package the device keeps.
func TestRelinkMergesConflictingUserData(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	e.profiles(2)
	dev1, dev2 := strings.Repeat("a", 32), strings.Repeat("b", 32)
	e.exec(`INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, 1), (?, 1, 1)`, dev1, dev2)
	download := func(id, device, item string, file any, url any) {
		e.t.Helper()
		e.exec(`INSERT INTO downloads (id, account_id, profile_id, device_id, item_id, media_file_id, source_url, quality,
		        audio_stream, state, estimated_bytes, created_at, updated_at) VALUES (?, 1, 1, ?, ?, ?, ?, 'medium', 0, 'ready', 1, 1, 1)`,
			id, device, item, file, url)
	}
	deadD1, deadD2 := "01DEADDEV1AAAAAAAAAAAAAAAA", "01DEADDEV2AAAAAAAAAAAAAAAA"
	copyD1, copyD2 := "01COPYDEV1AAAAAAAAAAAAAAAA", "01COPYDEV2AAAAAAAAAAAAAAAA"

	dead := e.addFilm(dir, lib.ID, "Heat/Heat.mkv", 949)
	e.exec(`INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (1, ?, 1, 1)`, dead)
	e.exec(`INSERT INTO notifications (profile_id, kind, item_id, created_at) VALUES (1, 'new_episode', ?, 1)`, dead)
	e.exec(`INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays) VALUES (1, ?, '', 10, 100, 1)`, dead)
	e.progress(1, dead, 500, false)
	e.exec(`UPDATE progress SET watched_at = 7 WHERE item_id = ?`, dead)
	download(deadD1, dev1, dead, e.fileOf("Heat/Heat.mkv"), nil) // its file goes missing below
	download(deadD2, dev2, dead, nil, "https://example.org/heat.mkv")
	e.dropFilm(dir, lib.ID, "Heat/Heat.mkv")

	c := e.addFilm(dir, lib.ID, "Heat Copy/Heat.mkv", 949)
	e.exec(`INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (1, ?, 2, 2), (2, ?, 2, 2)`, c, c)
	e.exec(`INSERT INTO notifications (profile_id, kind, item_id, created_at) VALUES (1, 'new_episode', ?, 2), (2, 'new_episode', ?, 2)`, c, c)
	e.exec(`INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays) VALUES (1, ?, '', 10, 50, 2), (1, ?, '', 11, 7, 1)`, c, c)
	e.exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, playback_rate, updated_at)
	        VALUES (1, ?, 900, 1000000, 0, 1.5, 2)`, c)
	copyFile := e.fileOf("Heat Copy/Heat.mkv")
	download(copyD1, dev1, c, copyFile, nil)
	download(copyD2, dev2, c, copyFile, nil)
	e.exec(`DELETE FROM download_tombstones`)

	if twin, err := e.relinkJob(c); err != nil || !twin {
		t.Fatalf("relink = %v, %v", twin, err)
	}
	for _, tc := range []struct{ what, query, want string }{
		{"watchlist", `SELECT group_concat(profile_id || ':' || created_at, ' ') FROM (SELECT * FROM watchlist WHERE item_id = ? ORDER BY profile_id)`, "1:1 2:2"},
		{"notifications", `SELECT group_concat(profile_id || ':' || created_at, ' ') FROM (SELECT * FROM notifications WHERE item_id = ? ORDER BY profile_id)`, "1:1 2:2"},
		{"progress days", `SELECT group_concat(day || ':' || played_ms || ':' || plays, ' ') FROM (SELECT * FROM progress_days WHERE item_id = ? ORDER BY day)`, "10:150:3 11:7:1"},
		{"progress", `SELECT position_ms || ':' || playback_rate || ':' || watched_at FROM progress WHERE item_id = ?`, "900:1.5:7"},
		{"downloads", `SELECT group_concat(device_id || ':' || id, ' ') FROM (SELECT * FROM downloads WHERE item_id = ? ORDER BY device_id)`,
			dev1 + ":" + copyD1 + " " + dev2 + ":" + deadD2},
	} {
		if got := e.str(tc.query, dead); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.what, got, tc.want)
		}
	}
	// The dead package and the copy's surplus one are tombstoned; the moved one is not.
	if got := e.str(`SELECT group_concat(id, ' ') FROM (SELECT id FROM download_tombstones ORDER BY id)`); got != copyD2+" "+deadD1 {
		t.Errorf("tombstones = %s, want %s %s", got, copyD2, deadD1)
	}
}

// Season 0 merges by its number like any season; an extra of the copy moves below the survivor;
// an episode of the copy that is deleted itself merges into its counterpart without restoring it,
// or moves below the season and stays deleted. Segments of a merged episode replace the survivor's
// detected ones (manual ones stay); a merged episode without segments sets its season to pending.
func TestRelinkSeriesSpecialsExtrasAndSegments(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	oldDir := tree(t, []string{
		"Show/Season 00/Show - S00E01.mkv", "Show/Season 01/Show - S01E01.mkv", "Show/Season 01/Show - S01E02.mkv",
		"Show/Season 01/Show - S01E03.mkv", "Show/Trailers/Old Trailer.mkv",
	})
	lib := e.createSeries(oldDir)
	e.scan(lib.ID)
	e.exec(`UPDATE items SET tvdb_id = 300 WHERE kind = 'series'`)
	dead := e.str(`SELECT id FROM items WHERE kind = 'series'`)
	deadS0 := e.str(`SELECT id FROM items WHERE kind = 'season' AND season_number = 0`)
	deadS1 := e.str(`SELECT id FROM items WHERE kind = 'season' AND season_number = 1`)
	deadE01, deadE11, deadE12, deadE13 := e.episode("Show", 0, 1), e.episode("Show", 1, 1), e.episode("Show", 1, 2), e.episode("Show", 1, 3)
	deadExtra := e.str(`SELECT id FROM items WHERE parent_id = ? AND kind NOT IN ('season')`, dead)
	e.exec(`INSERT INTO media_segments (item_id, kind, start_ms, end_ms, source, created_at) VALUES
	        (?, 'intro', 1000, 2000, 'fingerprint', 1), (?, 'outro', 9000, 9500, 'manual', 1)`, deadE11, deadE11)
	e.exec(`UPDATE items SET segments_state = 'done' WHERE id = ?`, deadS1)

	newDir := tree(t, []string{
		"Show/Season 00/Show - S00E01.mkv", "Show/Season 01/Show - S01E01.mkv", "Show/Season 01/Show - S01E02.mkv",
		"Show/Season 01/Show - S01E03.mkv", "Show/Season 01/Show - S01E04.mkv", "Show/Featurettes/Making Of.mkv",
	})
	got, err := e.svc.AddRoots(e.ctx, lib.ID, rootReq(newDir, model.RootSeries))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, e.rootID(got, oldDir), filepath.Clean(oldDir)); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Minute)
	e.scan(lib.ID)
	live := e.str(`SELECT id FROM items WHERE kind = 'series' AND deleted_at IS NULL`)
	liveE11, liveE13, liveE14 := e.episode("Show", 1, 1), e.episode("Show", 1, 3), e.episode("Show", 1, 4)
	liveExtra := e.str(`SELECT id FROM items WHERE parent_id = ? AND kind NOT IN ('season')`, live)
	if deadExtra == "" || liveExtra == "" {
		t.Fatalf("extras: old %q, copy %q", deadExtra, liveExtra)
	}
	e.exec(`INSERT INTO media_segments (item_id, kind, start_ms, end_ms, source, created_at) VALUES
	        (?, 'intro', 3000, 4000, 'fingerprint', 2), (?, 'outro', 8000, 8500, 'fingerprint', 2)`, liveE11, liveE11)
	e.exec(`UPDATE items SET deleted_at = ? WHERE id IN (?, ?)`, e.clock.Now().UnixMilli(), liveE13, liveE14)
	e.profiles(1)
	e.progress(1, liveE13, 400, false)
	e.exec(`UPDATE items SET tvdb_id = 300 WHERE id = ?`, live)
	e.clock.Advance(time.Minute)

	if twin, err := e.relinkJob(live); err != nil || !twin {
		t.Fatalf("relink = %v, %v", twin, err)
	}
	state := func(id string) string {
		return e.str(`SELECT (deleted_at IS NULL) || '/' || parent_id FROM items WHERE id = ?`, id)
	}
	for id, want := range map[string]string{
		deadS0: "1/" + dead, deadE01: "1/" + deadS0, // season 0 and its episode merged
		deadE11: "1/" + deadS1, deadE12: "1/" + deadS1,
		deadE13:   "0/" + deadS1, // the copy's deleted episode does not restore its counterpart
		liveE14:   "0/" + deadS1, // and without counterpart it moves, still deleted
		liveExtra: "1/" + dead, deadExtra: "0/" + dead,
	} {
		if v := state(id); v != want {
			t.Errorf("item %s = %s, want %s", id, v, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id IN (?, ?, ?)`, live, liveE11, liveE13); n != 0 {
		t.Errorf("merged nodes of the copy left: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM progress WHERE item_id = ? AND position_ms = 400`, deadE13); n != 1 {
		t.Error("progress of the copy's deleted episode lost")
	}
	segments := e.str(`SELECT group_concat(kind || ':' || start_ms || ':' || source, ' ') FROM (SELECT * FROM media_segments WHERE item_id = ? ORDER BY kind)`, deadE11)
	if want := "intro:3000:fingerprint outro:9000:manual"; segments != want {
		t.Errorf("segments = %s, want %s", segments, want)
	}
	if v := e.str(`SELECT segments_state FROM items WHERE id = ?`, deadS1); v != "pending" {
		t.Errorf("season 1 segments state = %s, want pending (S01E02 moved in without segments)", v)
	}
}

// A film with a year comes back by (sort_title, year) within 30 days when no live film has the
// key; a live film with the same key wins over the deleted one.
func TestMovieRelinkByTitleAndYear(t *testing.T) {
	t.Parallel()
	for _, liveTwin := range []bool{false, true} {
		t.Run(map[bool]string{false: "deleted film", true: "live film wins"}[liveTwin], func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			dir, lib := e.filmLibrary()
			writeFile(t, dir, "Heat (1995)/Heat (1995).mkv", "first release")
			e.scan(lib.ID)
			heat := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat (1995)/Heat (1995).mkv'`)
			e.dropFilm(dir, lib.ID, "Heat (1995)/Heat (1995).mkv")
			want := heat
			if liveTwin {
				want = e.addFilm(dir, lib.ID, "Other (1995)/Other (1995).mkv", 1)
				e.exec(`UPDATE items SET title = 'Heat', sort_title = (SELECT sort_title FROM items WHERE id = ?) WHERE id = ?`, heat, want)
			}
			e.clock.Advance(10 * 24 * time.Hour)
			writeFile(t, dir, "Heat (1995) Remux/Heat (1995).mkv", "second release")
			e.scan(lib.ID)
			if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat (1995) Remux/Heat (1995).mkv'`); v != want {
				t.Errorf("new file joined %q, want %q", v, want)
			}
			if v := e.str(`SELECT (deleted_at IS NULL) FROM items WHERE id = ?`, heat); (v == "1") != !liveTwin {
				t.Errorf("deleted film live = %s, want %v", v, !liveTwin)
			}
		})
	}
}

// Audiobooks and podcasts come back by provider id only, never by (sort_title, year).
func TestDeletedByTitleOnlyFilmsAndSeries(t *testing.T) {
	t.Parallel()
	idx := newItemIndex()
	year := sql.NullInt64{Int64: 2001, Valid: true}
	for _, kind := range []string{"audiobook", "podcast", "movie"} {
		idx.addDeleted(systemq.Item{ID: kind, Kind: kind, SortTitle: kind, Year: year}, true)
	}
	for kind, want := range map[string]string{"audiobook": "", "podcast": "", "movie": "movie"} {
		if got := idx.find(titleInfo{sortTitle: kind, year: year}, ""); got != want {
			t.Errorf("%s by title = %q, want %q", kind, got, want)
		}
	}
}
