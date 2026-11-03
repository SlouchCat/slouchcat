package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// TestUpdateRootPathKeepsTitlesAndUserData covers ADR 0023 point 4: a folder copied to a new
// place gets the new path. The titles keep their ids and user data, the media files their ids,
// because the scan finds them under the same names; the old watcher stops and one on the new path
// starts, and the scanner's state of the folder is dropped.
func TestUpdateRootPathKeepsTitlesAndUserData(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, firstFolder)
	lib := e.create(dir)
	e.scan(lib.ID)
	e.profiles(1)
	rootID := rootIDOf(t, e, lib.ID, dir)
	before := e.str(`SELECT group_concat(item_id || '@' || id || ':' || rel_path, ' ') FROM
	                 (SELECT * FROM media_files WHERE root_id = ? ORDER BY rel_path)`, rootID)
	alien := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, firstFolder[0])
	e.progress(1, alien, 4200, false)

	// The folder moved to another disk: the same names, every file with a new inode.
	moved := filepath.Join(t.TempDir(), "Filme-neu")
	copyTree(t, dir, moved)

	ctx, cancel := context.WithCancel(e.ctx)
	done := make(chan error, 1)
	go func() { done <- e.svc.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Start = %v", err)
		}
	})
	waitFor(t, 5*time.Second, "the watcher of the folder", func() bool {
		_, ok := e.svc.WatcherStats()[rootID]
		return ok
	})
	e.svc.mu.Lock()
	oldWatcher := e.svc.watchers[rootID]
	e.svc.mu.Unlock()
	e.svc.orphanMu.Lock()
	e.svc.orphans[rootID] = map[string]fileStamp{"old.mkv": {1, 1}}
	e.svc.orphanMu.Unlock()
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		t.Fatal(err)
	}

	after, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Path: &moved})
	if err != nil {
		t.Fatalf("path change: %v", err)
	}
	if r := after.Roots[0]; r.ID != rootID || r.Path != moved || r.Content != model.RootMovie {
		t.Fatalf("root after the change = %+v", r)
	}
	e.svc.mu.Lock()
	newWatcher := e.svc.watchers[rootID]
	e.svc.mu.Unlock()
	if newWatcher == nil || newWatcher == oldWatcher {
		t.Fatal("the watcher was not replaced")
	}
	e.svc.orphanMu.Lock()
	orphans := len(e.svc.orphans[rootID])
	e.svc.orphanMu.Unlock()
	if orphans != 0 {
		t.Fatalf("scanner state of the old place: %d orphans", orphans)
	}
	if n := e.jobCount(JobScanLibrary); n != 1 {
		t.Fatalf("scan jobs after the change = %d, want 1", n)
	}
	// Nothing was deleted before the scan, and nothing after it: the files are found again.
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NOT NULL`, lib.ID); n != 0 {
		t.Fatalf("deleted titles after the change = %d", n)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("scan at the new place = %+v", got.Scan)
	}
	after2 := e.str(`SELECT group_concat(item_id || '@' || id || ':' || rel_path, ' ') FROM
	                 (SELECT * FROM media_files WHERE root_id = ? ORDER BY rel_path)`, rootID)
	if after2 != before {
		t.Fatalf("files after the move = %s, want %s", after2, before)
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE root_id = ? AND missing_since IS NOT NULL`, rootID); n != 0 {
		t.Fatalf("missing files after the move = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NOT NULL`, lib.ID); n != 0 {
		t.Fatalf("deleted titles after the scan = %d", n)
	}
	if v := e.str(`SELECT position_ms FROM progress WHERE profile_id = 1 AND item_id = ?`, alien); v != "4200" {
		t.Fatalf("progress after the move = %q, want 4200", v)
	}
}

// TestUpdateRootPathRules: the new path passes the rules of a new folder, the folder itself not
// counting as an overlap; the same path changes nothing; a path goes with a provider list but not
// with a content change; a running scan refuses it and leaves the row as it was.
func TestUpdateRootPathRules(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	media := t.TempDir()
	dir := filepath.Join(media, "Filme")
	for i, f := range firstFolder {
		writeFile(t, dir, f, "video "+string(rune('a'+i)))
	}
	writeFile(t, media, "Andere/Dune (2021)/Dune (2021).mkv", "video d")
	writeFile(t, media, "Neu/Alien (1979)/Alien (1979).mkv", "video a")
	WithBrowseRoots([]string{media})(e.svc)
	lib := e.create(dir)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, dir)
	other, err := e.svc.Create(e.ctx, testRequest("Andere", filepath.Join(media, "Andere")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		t.Fatal(err)
	}
	update := func(upd model.LibraryRootUpdate) error {
		_, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, upd)
		return err
	}
	path := func(p string) model.LibraryRootUpdate { return model.LibraryRootUpdate{Path: &p} }

	// The same path, written with a trailing slash and spaces: no change, no scan.
	if err := update(path(" " + dir + "/ ")); err != nil {
		t.Fatalf("same path: %v", err)
	}
	if n := e.jobCount(JobScanLibrary); n != 0 {
		t.Fatalf("scan jobs after the same path = %d, want none", n)
	}
	for _, c := range []struct {
		name, path, code string
	}{
		{"another library's folder", other.Roots[0].Path, model.CodeRootOverlap},
		{"below another library's folder", filepath.Join(other.Roots[0].Path, "Dune (2021)"), model.CodeRootOverlap},
		{"holding another library's folder", media, model.CodeRootOverlap},
		{"relative", "Neu", "library_path_invalid"},
		{"missing", filepath.Join(media, "Fehlt"), "library_path_invalid"},
		{"a file", filepath.Join(media, "Neu/Alien (1979)/Alien (1979).mkv"), "library_path_invalid"},
		{"outside the browsable roots", t.TempDir(), "library_path_invalid"},
	} {
		if err := update(path(c.path)); !hasFieldError(err, "path", c.code) {
			t.Errorf("%s: %v, want path %s", c.name, err, c.code)
		}
	}
	series := model.RootSeries
	newPath := filepath.Join(media, "Neu")
	if err := update(model.LibraryRootUpdate{Path: &newPath, Content: &series, ConfirmPath: dir}); !hasFieldError(err, "path", model.CodePathWithContent) {
		t.Fatalf("path with content: %v", err)
	}
	unlock, ok := e.svc.tryLockScan(lib.ID)
	if !ok {
		t.Fatal("scan lock busy")
	}
	err = update(path(newPath))
	unlock()
	if !errors.Is(err, model.ErrScanRunning) {
		t.Fatalf("path change under a running scan: %v", err)
	}
	if v := e.str(`SELECT path FROM library_roots WHERE id = ?`, rootID); v != dir {
		t.Fatalf("path after the refused changes = %s, want %s", v, dir)
	}

	// A folder below its own old path is no overlap, and a provider list comes along.
	after, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Path: &newPath, Providers: []string{"tmdb", "anilist"}})
	if err != nil {
		t.Fatalf("path and providers: %v", err)
	}
	if r := after.Roots[0]; r.Path != newPath || len(r.Providers) != 2 || r.Providers[1] != "anilist" {
		t.Fatalf("root after path and providers = %+v", r)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND providers = '["tmdb","anilist"]'`, lib.ID); n != len(firstFolder) {
		t.Fatalf("titles with the new list = %d, want %d", n, len(firstFolder))
	}
	if n := e.jobCount(JobScanLibrary); n != 1 {
		t.Fatalf("scan jobs after the change = %d, want 1", n)
	}
	inside := filepath.Join(newPath, "Alien (1979)")
	if err := update(path(inside)); err != nil {
		t.Fatalf("a folder below the folder's own path: %v", err)
	}
}
