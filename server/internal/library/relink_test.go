package library

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// exec runs a statement on the write connection.
func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Write.ExecContext(e.ctx, query, args...); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
}

// profiles creates an account with n profiles (ids 1..n) and a session of profile 1 (id 1).
func (e *env) profiles(n int) {
	e.t.Helper()
	e.exec(`INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`)
	for i := 1; i <= n; i++ {
		e.exec(`INSERT INTO profiles (id, account_id, name, created_at, updated_at) VALUES (?, 1, ?, 0, 0)`, i, "P"+string(rune('0'+i)))
	}
	e.exec(`INSERT INTO sessions (id, token_hash, account_id, profile_id, created_at, last_seen_at, expires_at) VALUES (1, 'h', 1, 1, 0, 0, 1)`)
}

func (e *env) progress(profile int, itemID string, pos int, completed bool) {
	e.t.Helper()
	e.exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, ?, 1000000, ?, 1)`,
		profile, itemID, pos, completed)
}

func (e *env) progressEvent(itemID string) {
	e.t.Helper()
	e.exec(`INSERT INTO progress_events (profile_id, item_id, position_ms, duration_ms, played_ms, seq, state, client_time, server_time)
	        VALUES (1, ?, 1000, 1000000, 1000, 1, 'playing', 1, 1)`, itemID)
}

func (e *env) playbackSession(id, itemID string) {
	e.t.Helper()
	e.exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at)
	        VALUES (?, 1, 1, 1, ?, 'f', 'direct', 1, 1, 2)`, id, itemID)
}

// relinkJob runs the relink hook (QueueRelink) for the live title and, when it queued
// library.relink, runs that job; a finished job leaves the queue. It reports whether the hook found
// a twin.
func (e *env) relinkJob(live string) (bool, error) {
	e.t.Helper()
	twin, err := e.svc.QueueRelink(e.ctx, live)
	if err != nil || !twin {
		return twin, err
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND state = 'queued'`, JobRelink, relinkKey(live)); n != 1 {
		e.t.Fatalf("library.relink jobs of %s = %d, want 1", live, n)
	}
	payload, err := json.Marshal(itemPayload{live})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.svc.handleRelink(e.ctx, &jobs.Job{Type: JobRelink, Payload: payload}); err != nil {
		return true, err
	}
	e.exec(`DELETE FROM jobs WHERE idempotency_key = ?`, relinkKey(live))
	return true, nil
}

// rootID returns the id of the folder dir of the library.
func (e *env) rootID(lib model.Library, dir string) int64 {
	e.t.Helper()
	for _, r := range lib.Roots {
		if r.Path == filepath.Clean(dir) {
			return r.ID
		}
	}
	e.t.Fatalf("root %s is not in the library", dir)
	return 0
}

// A series folder is replaced (CT 190, idea 04): the new folder holds part of the old episodes
// and new ones, the old folder is removed before the scan, and the scan creates a copy with new
// ids. Once the match gives the copy the TVDB id of the deleted series, the copy merges into it.
func TestRelinkSeriesMergesTreeAndUserData(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	oldDir := tree(t, []string{
		"Show/Season 01/Show - S01E01.mkv", "Show/Season 01/Show - S01E02.mkv", "Show/Season 02/Show - S02E01.mkv",
	})
	lib := e.createSeries(oldDir)
	e.scan(lib.ID)
	e.exec(`UPDATE items SET tvdb_id = 300, year = 2003, metadata_state = 'matched' WHERE kind = 'series'`)
	dead := e.str(`SELECT id FROM items WHERE kind = 'series'`)
	deadS1 := e.str(`SELECT id FROM items WHERE kind = 'season' AND season_number = 1`)
	deadS2 := e.str(`SELECT id FROM items WHERE kind = 'season' AND season_number = 2`)
	deadE11, deadE12, deadE21 := e.episode("Show", 1, 1), e.episode("Show", 1, 2), e.episode("Show", 2, 1)

	e.profiles(2)
	e.progress(1, deadE11, 100, false)
	e.progress(1, deadE12, 50, true) // completed wins over the copy's larger position
	e.progress(2, deadE12, 100, false)
	e.progressEvent(deadE11)
	e.playbackSession("01AAAAAAAAAAAAAAAAAAAAAAAA", deadE11)
	e.exec(`INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (1, ?, 1)`, dead)
	e.exec(`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (?, 1, 5)`, dead)

	newDir := tree(t, []string{
		"Show/Season 01/Show - S01E02 - copy.mkv", "Show/Season 02/Show - S02E02.mkv", "Show/Season 03/Show - S03E01.mkv",
	})
	got, err := e.svc.AddRoots(e.ctx, lib.ID, rootReq(newDir, model.RootSeries))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, e.rootID(got, oldDir), filepath.Clean(oldDir)); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Minute) // the copy appears after the old series was deleted
	e.scan(lib.ID)
	if v := e.str(`SELECT deleted_at IS NOT NULL FROM items WHERE id = ?`, dead); v != "1" {
		t.Fatalf("old series not deleted: %s", v)
	}
	live := e.str(`SELECT id FROM items WHERE kind = 'series' AND deleted_at IS NULL`)
	liveE12, liveE22 := e.episode("Show", 1, 2), e.episode("Show", 2, 2)
	liveS3 := e.str(`SELECT id FROM items WHERE kind = 'season' AND season_number = 3`)
	e.progress(1, liveE12, 900, false)
	e.progress(2, liveE12, 900, false) // the larger position wins
	e.progress(1, liveE22, 300, false)
	e.progressEvent(liveE12)
	e.playbackSession("01BBBBBBBBBBBBBBBBBBBBBBBB", liveE12)
	e.exec(`INSERT INTO activity_events (at, kind, item_id, subject) VALUES (1, 'played', ?, 'Show')`, live)
	// Profile 1 picked both copies, the copy later (concept 18 part B, 14.8): the newer time wins.
	e.exec(`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (?, 1, 9), (?, 2, 7)`, live, live)

	// metadata.match gives the copy the ids of the deleted series.
	e.exec(`UPDATE items SET tvdb_id = 300, tmdb_id = 2691 WHERE id = ?`, live)
	pairs, err := e.svc.ListRelinkPairs(e.ctx, 0)
	if err != nil || len(pairs) != 1 || pairs[0].LiveID != live || pairs[0].DeletedID != dead || pairs[0].Kind != "series" {
		t.Fatalf("pairs = %+v, %v", pairs, err)
	}
	byTitle, err := e.svc.CountRelinkUserData(e.ctx, []string{dead, live})
	if counts := byTitle[dead]; err != nil || counts.Progress != 3 || counts.ProgressEvents != 1 || counts.PlaybackSessions != 1 || counts.Watchlist != 1 || counts.UserPicks != 1 {
		t.Fatalf("user data of the deleted series = %+v, %v", counts, err)
	}
	// The copy's own rows count apart: progress on two episodes, an event, a session, an activity,
	// two user picks.
	if counts := byTitle[live]; counts != (RelinkUserData{Progress: 3, ProgressEvents: 1, PlaybackSessions: 1, ActivityEvents: 1, UserPicks: 2}) {
		t.Fatalf("user data of the copy = %+v", counts)
	}

	// The scans queued a match of every title; only the relink may queue one for the kept series.
	e.exec(`DELETE FROM jobs WHERE type = ?`, JobMetadataMatch)
	if twin, err := e.relinkJob(live); err != nil || !twin {
		t.Fatalf("relink = %v, %v", twin, err)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ?`, live); n != 0 {
		t.Error("copy still exists")
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'series'`); n != 1 {
		t.Errorf("series = %d, want 1", n)
	}
	state := func(id string) string {
		return e.str(`SELECT (deleted_at IS NULL) || '/' || available || '/' || parent_id FROM items WHERE id = ?`, id)
	}
	if v := e.str(`SELECT (deleted_at IS NULL) || '/' || available FROM items WHERE id = ?`, dead); v != "1/1" {
		t.Errorf("series live/available = %s, want 1/1", v)
	}
	for id, want := range map[string]string{
		deadS1: "1/1/" + dead, deadS2: "1/1/" + dead, liveS3: "1/1/" + dead, // merged, merged, moved
		deadE11: "0/0/" + deadS1, deadE21: "0/0/" + deadS2, // no counterpart: stay deleted
		deadE12: "1/1/" + deadS1, liveE22: "1/1/" + deadS2, // merged, moved below the old season
	} {
		if v := state(id); v != want {
			t.Errorf("item %s = %s, want %s", id, v, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ?`, liveE12); n != 0 {
		t.Error("merged episode of the copy still exists")
	}
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Show/Season 01/Show - S01E02 - copy.mkv'`); v != deadE12 {
		t.Errorf("file of S01E02 belongs to %q, want %q", v, deadE12)
	}
	progress := e.str(`SELECT group_concat(profile_id || ':' || item_id || ':' || position_ms || ':' || completed, ' ')
	                   FROM (SELECT * FROM progress ORDER BY profile_id, position_ms)`)
	if want := "1:" + deadE12 + ":50:1 1:" + deadE11 + ":100:0 1:" + liveE22 + ":300:0 2:" + deadE12 + ":900:0"; progress != want {
		t.Errorf("progress = %s\nwant       %s", progress, want)
	}
	if n := e.count(`SELECT count(*) FROM progress_events WHERE item_id IN (?, ?)`, deadE11, deadE12); n != 2 {
		t.Errorf("progress events on the old episodes = %d, want 2", n)
	}
	if n := e.count(`SELECT count(*) FROM playback_sessions WHERE item_id IN (?, ?)`, deadE11, deadE12); n != 2 {
		t.Errorf("playback sessions on the old episodes = %d, want 2", n)
	}
	if v := e.str(`SELECT item_id FROM activity_events`); v != dead {
		t.Errorf("activity event on %q, want %q", v, dead)
	}
	if n := e.count(`SELECT count(*) FROM watchlist WHERE item_id = ?`, dead); n != 1 {
		t.Error("watchlist entry lost")
	}
	if v := e.str(`SELECT group_concat(item_id || ':' || profile_id || ':' || created_at, ' ') FROM (SELECT * FROM user_picks ORDER BY profile_id)`); v != dead+":1:9 "+dead+":2:7" {
		t.Errorf("user picks = %s, want both profiles on the kept series with the newer time", v)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = 'metadata.match' AND idempotency_key = ? AND state = 'queued'`, "match:"+dead); n != 1 {
		t.Error("no metadata.match for the kept series")
	}
	if pairs, err := e.svc.ListRelinkPairs(e.ctx, lib.ID); err != nil || len(pairs) != 0 {
		t.Errorf("pairs after the relink = %+v, %v", pairs, err)
	}
	// A rescan keeps the merged tree: the files are known.
	if got := e.scan(lib.ID); got.Scan.State != "done" || e.count(`SELECT count(*) FROM items WHERE kind = 'series'`) != 1 {
		t.Errorf("rescan = %+v", got.Scan)
	}
}

func TestRelinkMovieAndContradictingID(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	// Twelve other films keep the loss of two below the threshold.
	dir := tree(t, append([]string{"Heat/Heat.mkv", "Alien/Alien.mkv"}, twentyMovies[4:16]...))
	lib := e.create(dir)
	e.scan(lib.ID)
	heat := e.str(`SELECT id FROM items WHERE title = 'Heat'`)
	alien := e.str(`SELECT id FROM items WHERE title = 'Alien'`)
	e.exec(`UPDATE items SET tmdb_id = 949 WHERE id = ?`, heat)
	e.exec(`UPDATE items SET tmdb_id = 348, imdb_id = 'tt0078748' WHERE id = ?`, alien)
	e.profiles(1)
	e.progress(1, heat, 600, false)

	remove(t, dir, "Heat/Heat.mkv")
	remove(t, dir, "Alien/Alien.mkv")
	e.scan(lib.ID)
	e.clock.Advance(25 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 2 {
		t.Fatalf("sweep = %d, %v", n, err)
	}
	e.clock.Advance(90 * 24 * time.Hour) // no time limit for the relink by provider id
	writeFile(t, dir, "Heat Copy/Heat.mkv", "another release")
	writeFile(t, dir, "Alien Copy/Alien.mkv", "another release of a remake")
	e.scan(lib.ID)
	heatCopy := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat Copy/Heat.mkv'`)
	alienCopy := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Alien Copy/Alien.mkv'`)
	if heatCopy == heat || alienCopy == alien {
		t.Fatal("the scan found the deleted films without provider ids")
	}
	e.exec(`UPDATE items SET tmdb_id = 949, imdb_id = 'tt0113277' WHERE id = ?`, heatCopy)
	e.exec(`UPDATE items SET tmdb_id = 348, imdb_id = 'tt9999999' WHERE id = ?`, alienCopy)

	// The other id contradicts: no relink.
	if twin, err := e.svc.QueueRelink(e.ctx, alienCopy); err != nil || twin {
		t.Errorf("contradicting twin = %v, %v", twin, err)
	}
	if _, err := e.svc.Relink(e.ctx, alien, alienCopy); !errors.Is(err, ErrNotTwins) {
		t.Errorf("Relink of a contradicting pair = %v, want ErrNotTwins", err)
	}

	// While a scan holds the lock, the job waits without using an attempt and merges afterwards.
	unlock, ok := e.svc.tryLockScan(lib.ID)
	if !ok {
		t.Fatal("scan lock busy")
	}
	var later *jobs.RetryLaterError
	if twin, err := e.relinkJob(heatCopy); !twin || !errors.As(err, &later) || later.After != relinkRetry {
		t.Fatalf("relink under the lock = %v, %v; want a retry after %v", twin, err, relinkRetry)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ?`, heatCopy); n != 1 {
		t.Fatal("copy merged while the lock was held")
	}
	unlock()
	if twin, err := e.relinkJob(heatCopy); err != nil || !twin {
		t.Fatalf("relink after the lock = %v, %v", twin, err)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ?`, heatCopy); n != 0 {
		t.Error("copy not merged after the lock was released")
	}
	if v := e.str(`SELECT (deleted_at IS NULL) || '/' || available FROM items WHERE id = ?`, heat); v != "1/1" {
		t.Errorf("film live/available = %s, want 1/1", v)
	}
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat Copy/Heat.mkv'`); v != heat {
		t.Errorf("file belongs to %q, want %q", v, heat)
	}
	if n := e.count(`SELECT count(*) FROM progress WHERE item_id = ? AND position_ms = 600`, heat); n != 1 {
		t.Error("progress of the film lost")
	}
	if v := e.str(`SELECT (deleted_at IS NULL) FROM items WHERE id = ?`, alien); v != "0" {
		t.Error("contradicting film restored")
	}
}
