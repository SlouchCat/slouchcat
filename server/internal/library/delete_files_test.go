package library

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// «Auch die Dateien löschen» (concept 20 section 11, ADR 0032) on temporary roots.

func exists(t *testing.T, dir, rel string) bool {
	t.Helper()
	_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// A flat movie folder: «Alien» takes its file, its subtitle and its poster by the whole stem, never
// «Alien-Resurrection.mkv», which is the file of another title although its name fits the rule. A
// symlink of a fitting name that points out of the root is no regular file: it stays, and so does
// what it points at. The root stays. No file is left, so the film is deleted for good with its
// progress (O-14).
func TestDeleteFilesFlatMovieFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	outside := t.TempDir()
	writeFile(t, outside, "Alien.en.srt", "outside the root")
	dir := tree(t, []string{"Alien.mkv", "Alien-Resurrection.mkv", "Heat (1995).mkv", "Ronin (1998).mkv"})
	writeFile(t, dir, "Alien.de.srt", "subtitle")
	writeFile(t, dir, "Alien-poster.jpg", "poster")
	writeFile(t, dir, "Aliens.txt", "not the stem")
	if err := os.Symlink(filepath.Join(outside, "Alien.en.srt"), filepath.Join(dir, "Alien.en.srt")); err != nil {
		t.Fatal(err)
	}
	lib := e.create(dir)
	e.scan(lib.ID)
	e.profiles(1)
	alien := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Alien.mkv'`)
	resurrection := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Alien-Resurrection.mkv'`)
	if alien == "" || resurrection == "" || alien == resurrection {
		t.Fatalf("titles %q and %q", alien, resurrection)
	}
	e.progress(1, alien, 5000, false)

	preview, err := e.svc.ItemDeletion(e.ctx, alien, "de")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.FilesDeletable || len(preview.Files) != 1 || !preview.Files[0].Deletable || preview.Files[0].Reason != "" {
		t.Fatalf("preview = %+v", preview)
	}
	out, err := e.deleteItem(alien, "de", model.DeleteModeFiles, "", model.Actor{AccountID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.DeletedFiles != 1 || !out.TitleDeleted || len(out.Leftover) != 0 {
		t.Errorf("result = %+v", out)
	}
	for rel, want := range map[string]bool{
		"Alien.mkv": false, "Alien.de.srt": false, "Alien-poster.jpg": false,
		"Alien-Resurrection.mkv": true, "Aliens.txt": true, "Alien.en.srt": true, "Heat (1995).mkv": true,
	} {
		if got := exists(t, dir, rel); got != want {
			t.Errorf("%s exists = %v, want %v", rel, got, want)
		}
	}
	if !exists(t, outside, "Alien.en.srt") {
		t.Error("the file outside the root went")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the root went: %v", err)
	}
	for what, query := range map[string]string{
		"the title":  `SELECT count(*) FROM items WHERE id = ?`,
		"its file":   `SELECT count(*) FROM media_files WHERE item_id = ?`,
		"progress":   `SELECT count(*) FROM progress WHERE item_id = ?`,
		"other file": `SELECT count(*) = 0 FROM media_files WHERE rel_path = 'Alien-Resurrection.mkv' AND ? <> ''`,
	} {
		if n := e.count(query, alien); n != 0 {
			t.Errorf("%s left: %d", what, n)
		}
	}
}

// A series folder with an extra, a subtitle, the recorded poster, tvshow.nfo and a file Slouchcat
// does not know: everything Slouchcat knows goes, the season and extras folders go empty, and the
// series folder stays with the unknown file, which the result counts.
func TestDeleteFilesSeriesLeavesUnknownFiles(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{
		"Show (2020)/Season 01/Show - S01E01.mkv", "Show (2020)/Season 01/Show - S01E02.mkv",
		"Show (2020)/Trailers/Show Trailer.mkv",
		"Keep (2001)/Season 01/Keep - S01E01.mkv", "Keep (2001)/Season 01/Keep - S01E02.mkv",
	}
	dir := tree(t, files)
	writeFile(t, dir, "Show (2020)/Season 01/Show - S01E01.de.srt", "subtitle")
	writeFile(t, dir, "Show (2020)/tvshow.nfo", tvshowNFOFixture)
	writeFile(t, dir, "Show (2020)/poster.jpg", "poster")
	writeFile(t, dir, "Show (2020)/notes.txt", "unknown")
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Show'`)
	e.exec(`INSERT INTO item_local_images (item_id, kind, rel_path, root_id, width, height, size, mtime)
	        VALUES (?, 'poster', 'Show (2020)/poster.jpg', (SELECT id FROM library_roots WHERE library_id = ?), 1, 1, 1, 1)`, show, lib.ID)
	preview, err := e.svc.ItemDeletion(e.ctx, show, "de")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FileCount != 3 || !preview.TypeTitle || !preview.FilesDeletable {
		t.Fatalf("preview = %+v", preview)
	}
	out, err := e.deleteItem(show, "de", model.DeleteModeFiles, "show", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if out.DeletedFiles != 3 || !out.TitleDeleted || out.LeftInFolder != 1 {
		t.Errorf("result = %+v", out)
	}
	for rel, want := range map[string]bool{
		"Show (2020)/Season 01": false, "Show (2020)/Trailers": false, "Show (2020)/tvshow.nfo": false,
		"Show (2020)/poster.jpg": false, "Show (2020)/notes.txt": true, "Keep (2001)/Season 01/Keep - S01E01.mkv": true,
	} {
		if got := exists(t, dir, rel); got != want {
			t.Errorf("%s exists = %v, want %v", rel, got, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND title = 'Show'`, lib.ID); n != 0 {
		t.Errorf("series rows left: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'episode' AND parent_id IN (SELECT id FROM items WHERE kind = 'season')`); n != 2 {
		t.Errorf("episodes left: %d, want the two of Keep", n)
	}
}

// A folder mounted read-only (the check, faked here as a mount is not possible in a test) makes
// every file not deletable: the preview says why, the delete is refused and nothing goes.
func TestDeleteFilesRefusedOnReadOnlyMount(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, append(append([]string{}, twentyMovies[4:10]...), takenFilm))
	lib := e.create(dir)
	e.scan(lib.ID)
	e.svc.deleteCheckFn = func(paths.Root, string) error { return paths.ErrReadOnly }
	film := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, takenFilm)
	preview, err := e.svc.ItemDeletion(e.ctx, film, "de")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FilesDeletable || preview.FilesReason != model.DeleteReasonReadOnly || preview.Files[0].Deletable ||
		preview.Files[0].Reason != model.DeleteReasonReadOnly {
		t.Errorf("preview = %+v", preview)
	}
	_, err = e.deleteItem(film, "de", model.DeleteModeFiles, "", model.Actor{})
	var fe *model.FilesNotDeletableError
	if !errors.As(err, &fe) || fe.Reason != model.DeleteReasonReadOnly || !errors.Is(err, model.ErrFilesNotDeletable) {
		t.Errorf("delete = %v", err)
	}
	if !exists(t, dir, takenFilm) || e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL`, film) != 1 {
		t.Error("something went although the folder is read-only")
	}
	// «Nur aus Slouchcat entfernen» stays possible.
	if _, err := e.deleteItem(film, "de", model.DeleteModeLibrary, "", model.Actor{}); err != nil {
		t.Errorf("library mode on a read-only folder: %v", err)
	}
}

// One file of a series fails to go: the title stays with the remaining file, the episode whose
// file went is deleted for good, the result names the file that stayed. A missing file counts as
// deleted.
func TestDeleteFilesPartialFailureKeepsTheTitle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{
		"Show (2020)/Season 01/Show - S01E01.mkv", "Show (2020)/Season 01/Show - S01E02.mkv",
		"Show (2020)/Season 02/Show - S02E01.mkv",
		"Keep (2001)/Season 01/Keep - S01E01.mkv", "Keep (2001)/Season 01/Keep - S01E02.mkv",
	}
	dir := tree(t, files)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Show'`)
	// Review finding 6: the series keeps a file, so it keeps its recorded poster.
	writeFile(t, dir, "Show (2020)/poster.jpg", "poster")
	e.exec(`INSERT INTO item_local_images (item_id, kind, rel_path, root_id, width, height, size, mtime)
	        VALUES (?, 'poster', 'Show (2020)/poster.jpg', (SELECT id FROM library_roots WHERE library_id = ?), 1, 1, 1, 1)`, show, lib.ID)
	stuck := "Show (2020)/Season 02/Show - S02E01.mkv"
	e.svc.removeFileFn = func(root paths.Root, rel string) error {
		if rel == stuck {
			return fs.ErrPermission
		}
		return paths.Remove(root, rel)
	}
	remove(t, dir, "Show (2020)/Season 01/Show - S01E02.mkv") // gone before: counts as deleted
	out, err := e.deleteItem(show, "de", model.DeleteModeFiles, "Show", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if out.DeletedFiles != 2 || out.TitleDeleted || len(out.Leftover) != 1 || out.Leftover[0].Path != stuck ||
		out.Leftover[0].Reason != model.DeleteReasonNoPermission {
		t.Errorf("result = %+v", out)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL`, show); n != 1 {
		t.Error("the series went although a file stayed")
	}
	if got := e.str(`SELECT group_concat(rel_path) FROM media_files WHERE item_id IN (
	                   SELECT e.id FROM items AS e JOIN items AS s ON s.id = e.parent_id WHERE s.parent_id = ?)`, show); got != stuck {
		t.Errorf("files left = %q, want the stuck one", got)
	}
	// Season 1 lost both episodes and goes; season 2 stays with its episode.
	if n := e.count(`SELECT count(*) FROM items WHERE parent_id = ? AND kind = 'season'`, show); n != 1 {
		t.Errorf("seasons left: %d, want 1", n)
	}
	if exists(t, dir, "Show (2020)/Season 01") || !exists(t, dir, stuck) {
		t.Error("folders after the partial delete are wrong")
	}
	if !exists(t, dir, "Show (2020)/poster.jpg") || e.count(`SELECT count(*) FROM item_local_images WHERE item_id = ?`, show) != 1 {
		t.Error("the poster of the series that stays went")
	}
}

// A media file that is a hard link of another title's file (another library) stays and is named;
// the title stays with it.
func TestDeleteFilesKeepsAHardLinkOfAnotherTitle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, append(append([]string{}, twentyMovies[4:10]...), takenFilm))
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, takenFolder), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, filepath.FromSlash(takenFilm)), filepath.Join(other, filepath.FromSlash(takenFilm))); err != nil {
		t.Fatal(err)
	}
	lib := e.create(dir)
	e.scan(lib.ID)
	second, err := e.svc.Create(e.ctx, testRequest("Zweite", other))
	if err != nil {
		t.Fatal(err)
	}
	e.scan(second.ID)
	film := e.str(`SELECT m.item_id FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id WHERE r.library_id = ? AND m.rel_path = ?`, lib.ID, takenFilm)
	preview, err := e.svc.ItemDeletion(e.ctx, film, "de")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FilesDeletable || preview.FilesReason != model.DeleteReasonShared || preview.Files[0].Reason != model.DeleteReasonShared {
		t.Errorf("preview = %+v", preview)
	}
	if _, err := e.deleteItem(film, "de", model.DeleteModeFiles, "", model.Actor{}); !errors.Is(err, model.ErrFilesNotDeletable) {
		t.Errorf("delete of a title whose only file is shared: %v", err)
	}
	if !exists(t, dir, takenFilm) || !exists(t, other, takenFilm) {
		t.Error("a shared file went")
	}
	// The kinds of the result stay apart: a deletable verdict is not shared.
	if slices.ContainsFunc(preview.Files, func(f model.DeletionFile) bool { return f.Deletable }) {
		t.Error("the shared file counts as deletable")
	}
}

// Review finding 1: a file with a media extension is never a companion, even when no scan has seen
// it yet (the next film of a flat folder), and neither is a file whose device and inode are those
// of a media file of any title (a hard link under a fitting name). Other companions still go.
func TestDeleteFilesNeverTakesMediaAsCompanion(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Alien.mkv", "Heat (1995).mkv", "Ronin (1998).mkv"})
	lib := e.create(dir)
	e.scan(lib.ID)
	writeFile(t, dir, "Alien-Resurrection.mkv", "new film, not scanned yet")
	writeFile(t, dir, "Alien-commentary.mp3", "audio, not scanned")
	writeFile(t, dir, "Alien.de.srt", "subtitle")
	if err := os.Link(filepath.Join(dir, "Heat (1995).mkv"), filepath.Join(dir, "Alien.heat-backup")); err != nil {
		t.Fatal(err)
	}
	alien := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Alien.mkv'`)
	if _, err := e.deleteItem(alien, "de", model.DeleteModeFiles, "", model.Actor{}); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]bool{
		"Alien.mkv": false, "Alien.de.srt": false,
		"Alien-Resurrection.mkv": true, "Alien-commentary.mp3": true, "Alien.heat-backup": true, "Heat (1995).mkv": true,
	} {
		if got := exists(t, dir, rel); got != want {
			t.Errorf("%s exists = %v, want %v", rel, got, want)
		}
	}
}

// Review finding 3: a file replaced at the path of a scanned media file (another inode) stays,
// the result names it with reason changed, and the title stays with it.
func TestDeleteFilesKeepsAReplacedFile(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, append(append([]string{}, twentyMovies[4:10]...), takenFilm))
	lib := e.create(dir)
	e.scan(lib.ID)
	film := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, takenFilm)
	// The old inode stays alive through a link outside the root, so the new file gets another one.
	if err := os.Link(filepath.Join(dir, filepath.FromSlash(takenFilm)), filepath.Join(t.TempDir(), "old")); err != nil {
		t.Fatal(err)
	}
	remove(t, dir, takenFilm)
	writeFile(t, dir, takenFilm, "another file under the same name, not scanned")
	out, err := e.deleteItem(film, "de", model.DeleteModeFiles, "", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if out.DeletedFiles != 0 || out.TitleDeleted || len(out.Leftover) != 1 || out.Leftover[0].Reason != model.DeleteReasonChanged {
		t.Errorf("result = %+v", out)
	}
	if !exists(t, dir, takenFilm) || e.count(`SELECT count(*) FROM items WHERE id = ?`, film) != 1 {
		t.Error("the replaced file or the title went")
	}
}

// Review finding 7: deleting the last episode of a season with its file takes the empty season,
// and the last season of a series the empty series; a series with another season stays.
func TestDeleteFilesTakesEmptyContainersAbove(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{
		"Solo (2020)/Season 01/Solo - S01E01.mkv",
		"Duo (2021)/Season 01/Duo - S01E01.mkv", "Duo (2021)/Season 02/Duo - S02E01.mkv",
		"Keep (2001)/Season 01/Keep - S01E01.mkv", "Keep (2001)/Season 01/Keep - S01E02.mkv",
	}
	dir := tree(t, files)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	episode := func(rel string) string { return e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, rel) }
	solo := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Solo'`)
	duo := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Duo'`)
	for _, rel := range []string{"Solo (2020)/Season 01/Solo - S01E01.mkv", "Duo (2021)/Season 02/Duo - S02E01.mkv"} {
		if _, err := e.deleteItem(episode(rel), "de", model.DeleteModeFiles, "", model.Actor{}); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ? OR parent_id = ?`, solo, solo); n != 0 {
		t.Errorf("rows of the emptied series Solo left: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE parent_id = ? AND kind = 'season'`, duo); n != 1 {
		t.Errorf("seasons of Duo = %d, want the first one", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ?`, duo); n != 1 {
		t.Error("the series Duo with a season left went")
	}
}

// Review finding 8: a check that hangs (a share that does not answer) gives up after its deadline:
// the file is not deletable with reason unreachable, the preview answers at once, and the next
// file of the same root does not start another check while the first one hangs.
func TestDeleteFilesCheckGivesUpOnAHangingShare(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{
		"Show (2020)/Season 01/Show - S01E01.mkv", "Show (2020)/Season 02/Show - S02E01.mkv",
		"Keep (2001)/Season 01/Keep - S01E01.mkv", "Keep (2001)/Season 01/Keep - S01E02.mkv",
	}
	dir := tree(t, files)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Show'`)
	release := make(chan struct{})
	var calls atomic.Int32
	e.svc.fileCheckTimeout = 50 * time.Millisecond
	e.svc.deleteCheckFn = func(paths.Root, string) error {
		calls.Add(1)
		<-release
		return nil
	}
	start := time.Now()
	preview, err := e.svc.ItemDeletion(e.ctx, show, "de")
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("preview took %v", elapsed)
	}
	if preview.FilesDeletable || preview.FilesReason != model.DeleteReasonUnreachable {
		t.Errorf("preview = %+v", preview)
	}
	for _, f := range preview.Files {
		if f.Deletable || f.Reason != model.DeleteReasonUnreachable {
			t.Errorf("file %+v", f)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("checks started = %d, want 1 while it hangs", n)
	}
	close(release)
}

// Review finding 5: while a delete works on a title (between its last check for playbacks and its
// commit) the title is in the set playback refuses to start; after the delete it is free again.
func TestDeleteHoldsTheTitlesAgainstPlayback(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, append(append([]string{}, twentyMovies[4:10]...), takenFilm))
	lib := e.create(dir)
	e.scan(lib.ID)
	deleting := model.NewDeletingSet()
	WithDeleting(deleting)(e.svc)
	film := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, takenFilm)
	held := false
	e.svc.removeFileFn = func(root paths.Root, rel string) error {
		held = held || deleting.Has(film)
		return paths.Remove(root, rel)
	}
	if _, err := e.deleteItem(film, "de", model.DeleteModeFiles, "", model.Actor{}); err != nil {
		t.Fatal(err)
	}
	if !held || deleting.Has(film) {
		t.Errorf("held during the delete %v, held after %v", held, deleting.Has(film))
	}
}

// Review N1: «-» after the stem also starts a longer neighbour. Deleting «Alien.mkv» leaves the
// files of «Alien-Resurrection.mkv» (scanned) and of «Alien-Covenant.mkv» (on the disk, not yet
// scanned) alone, and a file recorded as the sidecar subtitle of another title; its own subtitle
// still goes.
func TestDeleteFilesLeavesTheCompanionsOfALongerNeighbour(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Alien.mkv", "Alien-Resurrection.mkv", "Heat (1995).mkv", "Ronin (1998).mkv"})
	lib := e.create(dir)
	e.scan(lib.ID)
	for _, rel := range []string{
		"Alien.de.srt", "Alien-Resurrection.de.srt", "Alien-Resurrection-poster.jpg", "Alien-Resurrection.MKV.part",
		"Alien-Covenant.mkv", "Alien-Covenant.en.srt", "Alien.notes.srt",
	} {
		writeFile(t, dir, rel, rel)
	}
	heat := e.str(`SELECT id FROM media_files WHERE rel_path = 'Heat (1995).mkv'`)
	e.exec(`INSERT INTO tracks (media_file_id, stream_index, kind, source, rel_path) VALUES (?, 0, 'subtitle', 'sidecar', 'Alien.notes.srt')`, heat)
	alien := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Alien.mkv'`)
	if _, err := e.deleteItem(alien, "de", model.DeleteModeFiles, "", model.Actor{}); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]bool{
		"Alien.mkv": false, "Alien.de.srt": false,
		"Alien-Resurrection.mkv": true, "Alien-Resurrection.de.srt": true, "Alien-Resurrection-poster.jpg": true,
		"Alien-Resurrection.MKV.part": true, "Alien-Covenant.mkv": true, "Alien-Covenant.en.srt": true, "Alien.notes.srt": true,
	} {
		if got := exists(t, dir, rel); got != want {
			t.Errorf("%s exists = %v, want %v", rel, got, want)
		}
	}
}

// Review N3: a media file without recorded device and inode is not deleted: it stays with reason
// changed, and the title stays with it.
func TestDeleteFilesKeepsAFileWithoutIdentity(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, append(append([]string{}, twentyMovies[4:10]...), takenFilm))
	lib := e.create(dir)
	e.scan(lib.ID)
	film := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, takenFilm)
	e.exec(`UPDATE media_files SET dev = 0, inode = 0 WHERE rel_path = ?`, takenFilm)
	out, err := e.deleteItem(film, "de", model.DeleteModeFiles, "", model.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if out.DeletedFiles != 0 || out.TitleDeleted || len(out.Leftover) != 1 || out.Leftover[0].Reason != model.DeleteReasonChanged {
		t.Errorf("result = %+v", out)
	}
	if !exists(t, dir, takenFilm) {
		t.Error("the file without identity went")
	}
}
