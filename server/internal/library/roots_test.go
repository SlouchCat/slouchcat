package library

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// twoFolders is one movie per folder, so each root carries titles of its own.
var (
	firstFolder  = []string{"Alien (1979)/Alien (1979).mkv", "Heat (1995)/Heat (1995).mkv"}
	secondFolder = []string{"Dune (2021)/Dune (2021).mkv"}
)

// addRoot adds dir to the library and scans it; it returns the id of the new root.
func (e *env) addRoot(libraryID int64, dir string) int64 {
	e.t.Helper()
	lib, err := e.svc.AddRoot(e.ctx, libraryID, dir)
	if err != nil {
		e.t.Fatalf("add root %s: %v", dir, err)
	}
	e.scan(libraryID)
	for _, r := range lib.Roots {
		if r.Path == filepath.Clean(dir) {
			return r.ID
		}
	}
	e.t.Fatalf("root %s is not in the library", dir)
	return 0
}

func TestAddRootScansTheNewFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first, second := tree(t, firstFolder), tree(t, secondFolder)
	lib := e.create(first)
	e.scan(lib.ID)

	rootID := e.addRoot(lib.ID, second)
	if rootID == 0 {
		t.Fatal("the new root has no id")
	}
	after, err := e.svc.Get(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Roots) != 2 {
		t.Fatalf("roots = %d, want 2", len(after.Roots))
	}
	if got := e.count(`SELECT count(*) FROM media_files WHERE root_id = ?`, rootID); got != len(secondFolder) {
		t.Fatalf("files below the new root = %d, want %d", got, len(secondFolder))
	}
}

func TestAddRootRefusesOverlapAndUnknownLibrary(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, firstFolder)
	lib := e.create(dir)

	if _, err := e.svc.AddRoot(e.ctx, lib.ID, dir); !errors.Is(err, model.ErrLibraryPathInvalid) {
		t.Fatalf("adding the same folder again: err = %v, want ErrLibraryPathInvalid", err)
	}
	if _, err := e.svc.AddRoot(e.ctx, lib.ID, filepath.Join(dir, "Alien (1979)")); !errors.Is(err, model.ErrLibraryPathInvalid) {
		t.Fatalf("adding a folder below an existing root: err = %v, want ErrLibraryPathInvalid", err)
	}
	if _, err := e.svc.AddRoot(e.ctx, lib.ID+99, t.TempDir()); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("adding to an unknown library: err = %v, want ErrNotFound", err)
	}
}

// TestAddRootKeepsTheBrowsableRootRule pins E14 down for the second folder as well: the rule is
// not only about the first one, or an administrator could walk out of the browsable roots by
// creating a library inside them and then adding a folder anywhere.
func TestAddRootKeepsTheBrowsableRootRule(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	allowed := t.TempDir()
	inside := filepath.Join(allowed, "Filme")
	writeFile(t, allowed, "Filme/Alien (1979)/Alien (1979).mkv", "video")
	WithBrowseRoots([]string{allowed})(e.svc)

	lib := e.create(inside)
	outside := tree(t, secondFolder)
	if _, err := e.svc.AddRoot(e.ctx, lib.ID, outside); !errors.Is(err, ErrNotBrowsable) {
		t.Fatalf("adding a folder outside the browsable roots: err = %v, want ErrNotBrowsable", err)
	}
}

// TestRemoveRootLeavesNoTitleWithoutFiles is the test section 6.7 asks for by name: after the
// removal the library holds no undeleted title with zero media files. Such a title would be
// invisible but not deleted, and SoftDeleteItemsMissingSince would never look at it again,
// because it only ever considers titles that still have a file.
func TestRemoveRootLeavesNoTitleWithoutFiles(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first, second := tree(t, firstFolder), tree(t, secondFolder)
	lib := e.create(first)
	e.scan(lib.ID)
	secondID := e.addRoot(lib.ID, second)

	firstID := rootIDOf(t, e, lib.ID, first)
	before := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NULL`, lib.ID)
	if before != len(firstFolder)+len(secondFolder) {
		t.Fatalf("titles before = %d, want %d", before, len(firstFolder)+len(secondFolder))
	}

	after, err := e.svc.RemoveRoot(e.ctx, lib.ID, firstID, first)
	if err != nil {
		t.Fatalf("remove root: %v", err)
	}
	if len(after.Roots) != 1 || after.Roots[0].ID != secondID {
		t.Fatalf("roots after the removal = %+v, want only %d", after.Roots, secondID)
	}
	orphans := e.count(`SELECT count(*) FROM items AS i WHERE i.library_id = ? AND i.deleted_at IS NULL
	    AND i.kind IN ('movie', 'episode', 'extra', 'audiobook')
	    AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id)`, lib.ID)
	if orphans != 0 {
		t.Fatalf("undeleted titles without a file = %d, want 0", orphans)
	}
	left := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NULL`, lib.ID)
	if left != len(secondFolder) {
		t.Fatalf("titles left = %d, want %d", left, len(secondFolder))
	}
	if got := e.count(`SELECT count(*) FROM media_files WHERE root_id = ?`, firstID); got != 0 {
		t.Fatalf("files of the removed root = %d, want 0", got)
	}
}

// TestRemoveRootKeepsTitleUnderAnotherRootWithItsDownload covers the second half of the order in
// 6.7: a title that still has a file elsewhere stays, and its download moves to that file
// instead of being taken by the cascade -- a device must not drop a package whose title is still
// on screen.
func TestRemoveRootKeepsTitleUnderAnotherRootWithItsDownload(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	shared := []string{"Alien (1979)/Alien (1979).mkv"}
	first, second := tree(t, shared), tree(t, shared)
	// Distinct content, so the two copies are two files of the same title and not one file
	// that moved.
	writeFile(t, second, shared[0], "a second copy of the same film")
	lib := e.create(first)
	e.scan(lib.ID)
	secondID := e.addRoot(lib.ID, second)
	firstID := rootIDOf(t, e, lib.ID, first)

	itemID := e.str(`SELECT item_id FROM media_files WHERE root_id = ?`, firstID)
	if itemID == "" {
		t.Fatal("the first root holds no file")
	}
	if got := e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, itemID); got != 2 {
		t.Fatalf("files of the shared title = %d, want 2 (one per root)", got)
	}
	fileID := e.str(`SELECT id FROM media_files WHERE root_id = ? AND item_id = ?`, firstID, itemID)
	downloadID := e.addDownload(itemID, fileID)

	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, firstID, first); err != nil {
		t.Fatalf("remove root: %v", err)
	}
	if got := e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL`, itemID); got != 1 {
		t.Fatalf("the title with a file under the second root was deleted")
	}
	if got := e.count(`SELECT count(*) FROM downloads WHERE id = ?`, downloadID); got != 1 {
		t.Fatalf("downloads of the surviving title = %d, want 1", got)
	}
	if got := e.count(`SELECT count(*) FROM downloads AS d JOIN media_files AS m ON m.id = d.media_file_id
	    WHERE d.id = ? AND m.root_id = ?`, downloadID, secondID); got != 1 {
		t.Fatal("the download did not move to the file under the second root")
	}
	if got := e.count(`SELECT count(*) FROM download_tombstones WHERE id = ?`, downloadID); got != 0 {
		t.Fatal("the surviving download left a tombstone, so the device would delete its package")
	}
}

// TestRemoveRootDropsTheDownloadOfALeavingTitle is the other side of it: a title that leaves
// takes its download with it, and the tombstone tells the device to free the space.
func TestRemoveRootDropsTheDownloadOfALeavingTitle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first := tree(t, firstFolder)
	lib := e.create(first)
	e.scan(lib.ID)
	firstID := rootIDOf(t, e, lib.ID, first)
	itemID := e.str(`SELECT item_id FROM media_files WHERE root_id = ? ORDER BY rel_path LIMIT 1`, firstID)
	fileID := e.str(`SELECT id FROM media_files WHERE item_id = ?`, itemID)
	downloadID := e.addDownload(itemID, fileID)

	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, firstID, first); err != nil {
		t.Fatalf("remove root: %v", err)
	}
	if got := e.count(`SELECT count(*) FROM downloads WHERE id = ?`, downloadID); got != 0 {
		t.Fatalf("downloads of the leaving title = %d, want 0", got)
	}
	if got := e.count(`SELECT count(*) FROM download_tombstones WHERE id = ?`, downloadID); got != 1 {
		t.Fatal("the dropped download left no tombstone, so the device would keep its package for good")
	}
}

// TestRemoveRootTakesAudiobooksWithIt pins the one difference to SoftDeleteItemWithoutFiles: that
// query knows movie, episode and extra only, so an audiobook library would keep empty rows.
func TestRemoveRootTakesAudiobooksWithIt(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, "Der Schwarm/01 Kapitel.mp3", "audio one")
	writeFile(t, dir, "Der Schwarm/02 Kapitel.mp3", "audio two")
	lib, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Hörbücher", Kind: "audiobooks", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	if got := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND kind = 'audiobook' AND deleted_at IS NULL`, lib.ID); got == 0 {
		t.Fatal("the scan created no audiobook, so the removal would prove nothing")
	}
	rootID := rootIDOf(t, e, lib.ID, dir)
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootID, dir); err != nil {
		t.Fatalf("remove root: %v", err)
	}
	if got := e.count(`SELECT count(*) FROM items AS i WHERE i.library_id = ? AND i.deleted_at IS NULL
	    AND i.kind = 'audiobook'
	    AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id)`, lib.ID); got != 0 {
		t.Fatalf("audiobooks without a file = %d, want 0", got)
	}
}

func TestRemoveRootRefusesAWrongConfirmPath(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, firstFolder)
	lib := e.create(dir)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, dir)

	for _, confirm := range []string{"", dir + "x", strings.ToUpper(dir), filepath.Dir(dir)} {
		if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootID, confirm); !errors.Is(err, model.ErrConfirmMismatch) {
			t.Fatalf("confirmPath %q: err = %v, want ErrConfirmMismatch", confirm, err)
		}
	}
	if got := e.count(`SELECT count(*) FROM library_roots WHERE id = ?`, rootID); got != 1 {
		t.Fatal("the root was removed although the confirmation did not match")
	}
	// Surrounding spaces are forgiven: a path copied out of the dialog carries them, and the
	// value is still the folder the administrator read.
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootID, "  "+dir+"\n"); err != nil {
		t.Fatalf("confirmPath with surrounding spaces: %v", err)
	}
}

func TestRemoveRootRefusesWhileAScanRuns(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, firstFolder)
	lib := e.create(dir)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, dir)

	unlock, err := e.svc.lockScan(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.RemoveRoot(e.ctx, lib.ID, rootID, dir)
	unlock()
	if !errors.Is(err, model.ErrScanRunning) {
		t.Fatalf("err = %v, want ErrScanRunning", err)
	}
	if got := e.count(`SELECT count(*) FROM library_roots WHERE id = ?`, rootID); got != 1 {
		t.Fatal("the root was removed while a scan held the lock")
	}
	// Once the scan let go, the removal works.
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootID, dir); err != nil {
		t.Fatalf("remove root after the scan: %v", err)
	}
}

func TestRemoveRootRefusesAnUnknownRootOrLibrary(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first, second := tree(t, firstFolder), tree(t, secondFolder)
	one := e.create(first)
	two, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Serien", Kind: "series", Path: second})
	if err != nil {
		t.Fatal(err)
	}
	rootOfOne := rootIDOf(t, e, one.ID, first)

	if _, err := e.svc.RemoveRoot(e.ctx, one.ID+99, rootOfOne, first); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown library: err = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.RemoveRoot(e.ctx, one.ID, rootOfOne+999, first); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown root: err = %v, want ErrNotFound", err)
	}
	// A root of another library is not found either, and the mismatch is never the reason it
	// answers -- the id decides first, so nothing leaks about the other library's folder.
	if _, err := e.svc.RemoveRoot(e.ctx, two.ID, rootOfOne, first); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("root of another library: err = %v, want ErrNotFound", err)
	}
}

// TestRemoveLastRootLeavesTheLibraryWithoutAFolder is E13: the library stays, keeps its settings
// and survives a scan, which has nothing to walk (step 14).
func TestRemoveLastRootLeavesTheLibraryWithoutAFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, firstFolder)
	lib := e.create(dir)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, dir)

	after, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootID, dir)
	if err != nil {
		t.Fatalf("remove the last root: %v", err)
	}
	if len(after.Roots) != 0 {
		t.Fatalf("roots = %d, want 0", len(after.Roots))
	}
	if after.Name != lib.Name || after.Kind != lib.Kind {
		t.Fatalf("the library changed: %+v", after)
	}
	// The scan of a library without a folder runs through without touching anything.
	state := e.scan(lib.ID)
	if state.Scan.State == "failed" {
		t.Fatalf("scan state = %q, last error %q", state.Scan.State, state.Scan.LastError)
	}
	if got := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NULL`, lib.ID); got != 0 {
		t.Fatalf("titles left = %d, want 0", got)
	}
}

// TestRemoveRootReportsWhatLeft covers the only trace the operation has: the notification with
// the numbers (there is no audit table).
func TestRemoveRootReportsWhatLeft(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, firstFolder)
	type report struct {
		library, path string
		items, files  int64
	}
	var got []report
	WithRootRemoved(func(_ context.Context, libraryName, path string, items, files int64) error {
		got = append(got, report{libraryName, path, items, files})
		return nil
	})(e.svc)

	lib := e.create(dir)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, dir)
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootID, dir); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reports = %d, want 1", len(got))
	}
	if got[0].library != lib.Name || got[0].path != dir {
		t.Fatalf("report = %+v, want library %q on %q", got[0], lib.Name, dir)
	}
	if got[0].items != int64(len(firstFolder)) || got[0].files != int64(len(firstFolder)) {
		t.Fatalf("report counts = %d titles, %d files; want %d and %d",
			got[0].items, got[0].files, len(firstFolder), len(firstFolder))
	}
}

// rootIDOf returns the id of the library's root on dir.
func rootIDOf(t *testing.T, e *env, libraryID int64, dir string) int64 {
	t.Helper()
	roots, err := appq.New(e.db.Read).ListLibraryRoots(e.ctx, libraryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roots {
		if r.Path == filepath.Clean(dir) {
			return r.ID
		}
	}
	t.Fatalf("library %d has no root on %s", libraryID, dir)
	return 0
}

// addDownload gives a title a ready download on a device, so the removal path can be watched
// from the offline side. The library package has no fixture for accounts and devices, so the
// three rows are written directly.
func (e *env) addDownload(itemID, mediaFileID string) string {
	e.t.Helper()
	now := e.clock.Now().UnixMilli()
	const deviceID = "0123456789abcdef0123456789abcdef"
	downloadID := "01K0000000000000000000DOWN"
	exec := func(query string, args ...any) {
		if _, err := e.db.Write.ExecContext(e.ctx, query, args...); err != nil {
			e.t.Fatalf("%s: %v", query, err)
		}
	}
	exec(`INSERT OR IGNORE INTO accounts (id, email, password_hash, role, created_at, updated_at)
	      VALUES (1, 'owner@example.test', 'x', 'owner', ?, ?)`, now, now)
	exec(`INSERT OR IGNORE INTO devices (id, user_agent, created_at, last_seen_at)
	      VALUES (?, 'test', ?, ?)`, deviceID, now, now)
	exec(`INSERT INTO downloads (id, account_id, device_id, item_id, media_file_id, quality,
	          audio_stream, state, estimated_bytes, created_at, updated_at)
	      VALUES (?, 1, ?, ?, ?, 'medium', 0, 'ready', 1024, ?, ?)`,
		downloadID, deviceID, itemID, mediaFileID, now, now)
	return downloadID
}
