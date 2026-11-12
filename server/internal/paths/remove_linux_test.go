package paths

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Remove deletes a regular file below the root and nothing else: not a directory, not a symlink
// (least of all the file outside the root it points at), not through "..", and a missing file
// says so. RemoveEmptyDir deletes an empty directory only, never the root.
func TestRemoveStaysInsideTheRoot(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.mkv")
	if err := os.WriteFile(secret, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, rel := range []string{"Film/Film.mkv", "Film/Film.de.srt", "Other/Other.mkv"} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(secret, filepath.Join(dir, "Film", "link.mkv")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "away")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Empty"), 0o750); err != nil {
		t.Fatal(err)
	}
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := Remove(root, "Film/Film.mkv", FileID{}); err != nil {
		t.Fatalf("remove a file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Film", "Film.mkv")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the file is still there: %v", err)
	}
	if err := Remove(root, "Film/Film.mkv", FileID{}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("remove a missing file = %v, want not exist", err)
	}
	if err := Remove(root, "Film/link.mkv", FileID{}); !errors.Is(err, ErrNotRegular) {
		t.Errorf("remove a symlink = %v, want not regular", err)
	}
	for _, rel := range []string{"away/secret.mkv", "../" + filepath.Base(outside) + "/secret.mkv", secret, "Film/../../x"} {
		if err := Remove(root, rel, FileID{}); err == nil {
			t.Errorf("remove %s went through", rel)
		}
	}
	if _, err := os.Stat(secret); err != nil {
		t.Errorf("the file outside the root went: %v", err)
	}
	if err := Remove(root, "Other", FileID{}); !errors.Is(err, ErrNotRegular) {
		t.Errorf("remove a directory as a file = %v", err)
	}

	if err := RemoveEmptyDir(root, "Film"); err == nil {
		t.Error("a directory with files in it went")
	}
	if err := RemoveEmptyDir(root, "Empty"); err != nil {
		t.Errorf("remove an empty directory: %v", err)
	}
	if err := RemoveEmptyDir(root, "away"); !errors.Is(err, ErrNotDir) {
		t.Errorf("remove a symlinked directory = %v, want not a directory", err)
	}
	for _, rel := range []string{".", "", "Film/.."} {
		if err := RemoveEmptyDir(root, rel); err == nil {
			t.Errorf("remove the root as %q went through", rel)
		}
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the root went: %v", err)
	}
}

// The checks before a delete write nothing: a temporary directory is writable and not mounted
// read-only; a directory without write permission refuses (root may write anywhere, so for root
// the same directory is allowed).
func TestMayDeleteInChecksWithoutWriting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "open"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "closed"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "closed"), 0o550); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "closed"), 0o750) })
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ro, err := ReadOnlyAt(root, "open"); err != nil || ro {
		t.Errorf("ReadOnlyAt(open) = %v, %v", ro, err)
	}
	if err := MayDeleteIn(root, "open"); err != nil {
		t.Errorf("MayDeleteIn(open) = %v", err)
	}
	want := ErrNoPermission
	if os.Geteuid() == 0 {
		want = nil
	}
	if err := MayDeleteIn(root, "closed"); !errors.Is(err, want) {
		t.Errorf("MayDeleteIn(closed) = %v, want %v", err, want)
	}
	if err := MayDeleteIn(root, "../x"); err == nil {
		t.Error("a check outside the root went through")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "open"))
	if err != nil || len(entries) != 0 {
		t.Errorf("the check wrote into the folder: %v, %v", entries, err)
	}
}

// Review finding 3: with an expected device and inode, Remove deletes only that file; another file
// at the same path (replaced since the scan) stays and answers ErrChanged.
func TestRemoveChecksTheExpectedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "Film.mkv")
	if err := os.WriteFile(p, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	old := fileIDOf(fi)
	// Replace it: a new inode at the same path (the old one is kept alive by a link).
	if err := os.Link(p, filepath.Join(dir, "keep")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Remove(root, "Film.mkv", old); !errors.Is(err, ErrChanged) {
		t.Errorf("remove a replaced file = %v, want changed", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("the replacing file went: %v", err)
	}
	fi, err = os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Remove(root, "Film.mkv", fileIDOf(fi)); err != nil {
		t.Errorf("remove the expected file: %v", err)
	}
}
