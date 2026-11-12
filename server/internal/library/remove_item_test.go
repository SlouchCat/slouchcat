package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// «Nur aus Slouchcat entfernen» on a film (concept 20 section 10.2): it leaves every list, keeps
// its file, its media_files row, progress and watchlist; its download goes (the tombstone tells
// the device); the activity log names who, the title and the choice.
func TestRemoveFilmKeepsFileAndUserData(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, append(append([]string{}, twentyMovies[4:16]...), takenFilm))
	lib := e.create(dir)
	e.scan(lib.ID)
	e.profiles(1)
	film := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, takenFilm)
	e.progress(1, film, 5000, false)
	e.exec(`INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (1, ?, 1, 1)`, film)
	e.exec(`INSERT INTO devices (id, created_at, last_seen_at) VALUES ('dddddddddddddddddddddddddddddddd', 1, 1)`)
	e.exec(`INSERT INTO downloads (id, account_id, device_id, item_id, media_file_id, quality, audio_stream, estimated_bytes, created_at, updated_at)
	        VALUES ('01DOWNLOAD0000000000000001', 1, 'dddddddddddddddddddddddddddddddd', ?, (SELECT id FROM media_files WHERE item_id = ?), 'high', 0, 1, 1, 1)`, film, film)
	var events []model.ActivityEvent
	e.svc.WithActivity(func(_ context.Context, ev model.ActivityEvent) { events = append(events, ev) })

	preview, err := e.svc.ItemDeletion(e.ctx, film, "de")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FileCount != 1 || preview.TypeTitle || len(preview.Files) != 1 || preview.Files[0].Path != takenFilm ||
		preview.Files[0].RootPath != dir || preview.Folder == nil || preview.Folder.Path != takenFolder || preview.ConfirmTitle != "Collateral" {
		t.Errorf("preview = %+v", preview)
	}
	// One file: no title to type.
	out, err := e.deleteItem(film, "de", model.DeleteModeLibrary, "", model.Actor{AccountID: 1, ProfileID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.RemovedItems != 1 || out.DeletedFiles != 0 {
		t.Errorf("result = %+v", out)
	}
	for what, query := range map[string]string{
		"taken out":       `SELECT count(*) FROM items WHERE id = ? AND removed_at IS NOT NULL AND deleted_at IS NOT NULL`,
		"its file row":    `SELECT count(*) FROM media_files WHERE item_id = ?`,
		"its progress":    `SELECT count(*) FROM progress WHERE item_id = ?`,
		"its watchlist":   `SELECT count(*) FROM watchlist WHERE item_id = ?`,
		"a tombstone":     `SELECT count(*) FROM download_tombstones WHERE id = '01DOWNLOAD0000000000000001' AND ? <> ''`,
		"the removed row": `SELECT count(*) FROM items WHERE id = ?`,
	} {
		if n := e.count(query, film); n != 1 {
			t.Errorf("%s: %d", what, n)
		}
	}
	if n := e.count(`SELECT count(*) FROM downloads WHERE item_id = ?`, film); n != 0 {
		t.Errorf("downloads left: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM visible_items WHERE item_id = ?`, film); n != 0 {
		t.Errorf("still visible for %d profiles", n)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(takenFilm))); err != nil {
		t.Errorf("the file went: %v", err)
	}
	if len(events) != 1 || events[0].Kind != model.ActivityItemDeleted || events[0].ItemID != film ||
		events[0].Detail.Mode != model.DeleteModeLibrary || events[0].Subject != "Collateral" || events[0].AccountID != 1 {
		t.Errorf("activity = %+v", events)
	}
	// Gone from the administration as well: a second delete and a preview find nothing.
	if _, err := e.svc.ItemDeletion(e.ctx, film, "de"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("preview of a removed title: %v", err)
	}
	// The file comes and goes: a missing file that reappears does not bring it back.
	rename(t, dir, takenFilm, "elsewhere.tmp")
	e.scan(lib.ID)
	rename(t, dir, "elsewhere.tmp", takenFilm)
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL`, film); n != 0 {
		t.Error("the reappearing file brought the removed film back")
	}
}

// A series goes with every season and episode; with more than one file the title is typed, any
// case, surrounding spaces forgiven. A playing profile refuses it (O-13).
func TestRemoveSeriesTypedAndRefusedWhilePlaying(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{takenE01, takenShow + "/Season 01/Night Shift - S01E02.mkv", takenShow + "/Season 02/Night Shift - S02E01.mkv"}
	for _, ep := range []string{"S01E01", "S01E02", "S01E03", "S01E04"} {
		files = append(files, "Keep (2001)/Season 01/Keep - "+ep+".mkv")
	}
	dir := tree(t, files)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	e.profiles(1)
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Night Shift'`)
	preview, err := e.svc.ItemDeletion(e.ctx, show, "de")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FileCount != 3 || !preview.TypeTitle || preview.Folder == nil || preview.Folder.Path != takenShow {
		t.Errorf("preview = %+v", preview)
	}
	if _, err := e.deleteItem(show, "de", model.DeleteModeLibrary, "Night", model.Actor{}); !errors.Is(err, model.ErrConfirmMismatch) {
		t.Errorf("wrong title: %v", err)
	}
	if _, err := e.deleteItem(show, "de", "trash", "Night Shift", model.Actor{}); err == nil {
		t.Error("an unknown mode was accepted")
	}
	now := e.clock.Now().UnixMilli()
	e.exec(`UPDATE sessions SET expires_at = ? WHERE id = 1`, now+3600000)
	episode := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, takenE01)
	e.exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at)
	        VALUES ('01PLAYING00000000000000001', 1, 1, 1, ?, 'f', 'direct', ?, ?, ?)`, episode, now, now, now+60000)
	if p, err := e.svc.ItemDeletion(e.ctx, show, "de"); err != nil || !slices.Equal(p.PlayingProfiles, []string{"P1"}) {
		t.Errorf("preview while playing = %v, %v", p.PlayingProfiles, err)
	}
	var playing *model.PlayingError
	if _, err := e.deleteItem(show, "de", model.DeleteModeLibrary, " night shift ", model.Actor{}); !errors.As(err, &playing) {
		t.Errorf("while playing: %v", err)
	}
	e.exec(`UPDATE playback_sessions SET revoked_at = ?`, now)
	out, err := e.deleteItem(show, "de", model.DeleteModeLibrary, " night shift ", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	// Series, two seasons, three episodes.
	if out.RemovedItems != 6 {
		t.Errorf("removed %d titles, want 6", out.RemovedItems)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND removed_at IS NOT NULL`, lib.ID); n != 6 {
		t.Errorf("titles taken out: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'series' AND deleted_at IS NULL`); n != 1 {
		t.Errorf("live series: %d, want the other one", n)
	}
}

// Extras and podcast episodes that are only a reference from the feed have no delete (O-5).
func TestRemoveRefusesExtrasAndFeedEpisodes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	extra := e.str(`SELECT id FROM items WHERE kind = 'extra' LIMIT 1`)
	if _, err := e.svc.ItemDeletion(e.ctx, extra, "de"); !errors.Is(err, model.ErrNotDeletable) {
		t.Errorf("extra: %v", err)
	}
	e.exec(`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (50, 'Podcasts', 'podcasts', 1, 1)`)
	e.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at) VALUES ('01PODCAST00000000000000001', 'podcast', 50, 'Pod', 'pod', 1, 1, 1)`)
	e.exec(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, enclosure_url, added_at, created_at, updated_at)
	        VALUES ('01PODCASTEPISODE0000000001', 'podcast_episode', '01PODCAST00000000000000001', 50, 'Ep', 'ep', 'https://example.test/ep.mp3', 1, 1, 1)`)
	if _, err := e.deleteItem("01PODCASTEPISODE0000000001", "de", model.DeleteModeLibrary, "", model.Actor{}); !errors.Is(err, model.ErrNotDeletable) {
		t.Errorf("feed episode: %v", err)
	}
	if _, err := e.svc.ItemDeletion(e.ctx, "01UNKNOWN00000000000000001", "de"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

// «Entfernte Titel» lists the titles that can come back on their own; «Wieder aufnehmen» brings
// the same title back with what the scanner put below it meanwhile, and a season taken out on its
// own before stays out and shows up in the list once its series is back. A live copy of a removed
// film elsewhere in the library is no twin: it never merges into the removed one.
func TestRestoreRemovedTitles(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{takenE01, takenShow + "/Season 02/Night Shift - S02E01.mkv"}
	for _, ep := range []string{"S01E01", "S01E02", "S01E03", "S01E04"} {
		files = append(files, "Keep (2001)/Season 01/Keep - "+ep+".mkv")
	}
	dir := tree(t, files)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Night Shift'`)
	season2 := e.str(`SELECT id FROM items WHERE kind = 'season' AND parent_id = ? AND season_number = 2`, show)
	if _, err := e.deleteItem(season2, "de", model.DeleteModeLibrary, "", model.Actor{}); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Minute)
	if _, err := e.deleteItem(show, "de", model.DeleteModeLibrary, "Night Shift", model.Actor{}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, takenShow+"/Season 01/Night Shift - S01E02.mkv", "new episode while out")
	e.scan(lib.ID)
	newEpisode := e.str(`SELECT item_id FROM media_files WHERE rel_path LIKE '%S01E02.mkv' AND rel_path LIKE 'Night%'`)

	page, err := e.svc.ListRemovedItems(e.ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != show || page.Items[0].Folder == nil ||
		page.Items[0].Folder.Path != takenShow || page.Items[0].LibraryName != "Serien" {
		t.Fatalf("removed list = %+v", page)
	}
	// The season comes back only with its series.
	if _, err := e.svc.RestoreRemovedItem(e.ctx, season2); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("restore below a removed series: %v", err)
	}
	n, err := e.svc.RestoreRemovedItem(e.ctx, show)
	if err != nil {
		t.Fatal(err)
	}
	// Series, season 1, its two episodes (one new); season 2 and its episode stay out.
	if n != 4 {
		t.Errorf("restored %d titles, want 4", n)
	}
	for id, live := range map[string]bool{show: true, newEpisode: true, season2: false} {
		if got := e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL AND removed_at IS NULL`, id) == 1; got != live {
			t.Errorf("%s live = %v, want %v", id, got, live)
		}
	}
	page, err = e.svc.ListRemovedItems(e.ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].ID != season2 || page.Items[0].ParentTitle != "Night Shift" {
		t.Errorf("removed list after the restore = %+v", page)
	}
	if _, err := e.svc.RestoreRemovedItem(e.ctx, show); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("restore of a live title: %v", err)
	}
}

// A removed film is no twin: a live copy elsewhere in the library under another name (the scanner
// would put one of the same name and year under the removed film) that gets the same TMDB id keeps
// its own title, and neither the automatic relink nor the maintenance list offers the pair.
func TestRemovedFilmIsNoTwin(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	film := e.addFilm(dir, lib.ID, "Heat (1995)/Heat (1995).mkv", 949)
	if _, err := e.deleteItem(film, "de", model.DeleteModeLibrary, "", model.Actor{}); err != nil {
		t.Fatal(err)
	}
	// A new release of the same name and year in another folder joins the removed film and stays
	// out with it (F-13).
	writeFile(t, dir, "Heat (1995) Remux/Heat (1995).mkv", "remux of the removed film")
	e.scan(lib.ID)
	if owner := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat (1995) Remux/Heat (1995).mkv'`); owner != film {
		t.Errorf("the release of the same name joined %s, want the removed film", owner)
	}
	copyID := e.addFilm(dir, lib.ID, "Copies/Heat Directors Cut.mkv", 949)
	if copyID == film {
		t.Fatal("the copy joined the removed film")
	}
	if twin, err := e.relinkJob(copyID); err != nil || twin {
		t.Errorf("relink of the copy = %v, %v; want no twin", twin, err)
	}
	pairs, err := e.svc.ListRelinkPairs(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 0 {
		t.Errorf("relink pairs = %+v", pairs)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ? AND removed_at IS NOT NULL`, film); n != 1 {
		t.Error("the removed film came back")
	}
}

// deleteItem deletes as the dialog does: it reads the preview first and repeats its stamp of the
// files (expectedFiles). A title without preview (not deletable, unknown) gets the zero stamp; the
// delete answers the same error before it compares.
func (e *env) deleteItem(itemID, lang, mode, confirmTitle string, by model.Actor) (model.ItemDeletionResult, error) {
	e.t.Helper()
	var expected model.ExpectedFiles
	if p, err := e.svc.ItemDeletion(e.ctx, itemID, lang); err == nil {
		expected = p.Expected
	}
	return e.svc.DeleteItem(e.ctx, itemID, lang, mode, confirmTitle, expected, by)
}

// Review finding 2: the delete takes exactly what the preview showed. A file that joined the title
// after the preview (a new episode) changes the stamp: 409 changed and nothing is deleted; a
// preview read again lets it through.
func TestDeleteItemTiedToItsPreview(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{takenE01, "Keep (2001)/Season 01/Keep - S01E01.mkv", "Keep (2001)/Season 01/Keep - S01E02.mkv"}
	dir := tree(t, files)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Night Shift'`)
	preview, err := e.svc.ItemDeletion(e.ctx, show, "de")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Expected.Count != 1 || len(preview.Expected.Hash) != 32 {
		t.Fatalf("stamp = %+v", preview.Expected)
	}
	writeFile(t, dir, takenShow+"/Season 01/Night Shift - S01E02.mkv", "new episode after the preview")
	e.scan(lib.ID)
	if _, err := e.svc.DeleteItem(e.ctx, show, "de", model.DeleteModeFiles, "Night Shift", preview.Expected, model.Actor{}); !errors.Is(err, model.ErrChanged) {
		t.Fatalf("delete with the old stamp = %v, want changed", err)
	}
	if !exists(t, dir, takenE01) || e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL`, show) != 1 {
		t.Fatal("something went although the files changed")
	}
	// Review N4: a file that only changed (path, size) changes the stamp as well.
	preview, err = e.svc.ItemDeletion(e.ctx, show, "de")
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE media_files SET size = size + 1 WHERE rel_path = ?`, takenE01)
	if _, err := e.svc.DeleteItem(e.ctx, show, "de", model.DeleteModeFiles, "Night Shift", preview.Expected, model.Actor{}); !errors.Is(err, model.ErrChanged) {
		t.Fatalf("delete after a size change = %v, want changed", err)
	}
	preview, err = e.svc.ItemDeletion(e.ctx, show, "de")
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE media_files SET rel_path = ? WHERE rel_path = ?`, takenShow+"/Season 01/renamed.mkv", takenE01)
	if _, err := e.svc.DeleteItem(e.ctx, show, "de", model.DeleteModeFiles, "Night Shift", preview.Expected, model.Actor{}); !errors.Is(err, model.ErrChanged) {
		t.Fatalf("delete after a rename = %v, want changed", err)
	}
	e.exec(`UPDATE media_files SET rel_path = ? WHERE rel_path = ?`, takenE01, takenShow+"/Season 01/renamed.mkv")
	if _, err := e.deleteItem(show, "de", model.DeleteModeFiles, "Night Shift", model.Actor{}); err != nil {
		t.Errorf("delete with the new preview: %v", err)
	}
}
