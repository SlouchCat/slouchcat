package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	root, err := NewRoot("/media/movies")
	if err != nil {
		t.Fatal(err)
	}
	ok := map[string]string{
		"Alien (1979)/Alien.mkv": "/media/movies/Alien (1979)/Alien.mkv",
		"a/./b.mkv":              "/media/movies/a/b.mkv",
		"..hidden/file..mkv":     "/media/movies/..hidden/file..mkv",
	}
	for rel, want := range ok {
		got, err := Resolve(root, rel)
		if err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", rel, got, err, want)
		}
	}
	for _, rel := range []string{"", "..", "../etc/passwd", "a/../b", "a/..", "/etc/passwd", "a\x00b"} {
		if _, err := Resolve(root, rel); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("Resolve(%q) error = %v, want ErrOutsideRoot", rel, err)
		}
	}
}

func TestNewRootRequiresAbsolutePath(t *testing.T) {
	if _, err := NewRoot("media"); err == nil {
		t.Fatal("expected an error for a relative root")
	}
	if _, err := Resolve(Root{}, "a"); err == nil {
		t.Fatal("expected an error for the zero Root")
	}
}

func TestClassifyFS(t *testing.T) {
	cases := map[uint32]FSType{
		0xEF53: FSExt4, 0x58465342: FSXFS, 0x9123683E: FSBtrfs, 0x2FC12FC1: FSZFS, 0x65735546: FSFUSE,
		0x6969: FSNFS, 0x517B: FSSMB, 0xFF534D42: FSSMB, 0xFE534D42: FSSMB,
		0x01021994: FSUnknown, // tmpfs
		0x794C7630: FSUnknown, // overlayfs
	}
	for magic, want := range cases {
		if got := ClassifyFS(magic); got != want {
			t.Errorf("ClassifyFS(%#x) = %s, want %s", magic, got, want)
		}
	}
	for typ, want := range map[FSType]bool{FSNFS: true, FSSMB: true, FSExt4: false, FSFUSE: false, FSUnknown: false} {
		if got := typ.IsNetwork(); got != want {
			t.Errorf("%s.IsNetwork() = %v, want %v", typ, got, want)
		}
	}
}

func TestDetectFS(t *testing.T) {
	typ, err := DetectFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("temp dir file system: %s", typ)
	if _, err := DetectFS("/does/not/exist"); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func TestOpenAndWalkStayInsideRoot(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Movies", "A (2020)"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Movies", "A (2020)", "A (2020).mkv"), []byte("v"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.Stat("Movies/A (2020)/A (2020).mkv"); err != nil {
		t.Errorf("stat inside root: %v", err)
	}
	if _, err := r.Open("link/secret.txt"); err == nil {
		t.Error("symlink leaving the root must be refused")
	}
	if _, err := r.Open("../secret.txt"); err == nil {
		t.Error(".. must be refused")
	}
	var seen []string
	err = Walk(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".", "Movies", "Movies/A (2020)", "Movies/A (2020)/A (2020).mkv", "link"}
	if strings.Join(seen, "|") != strings.Join(want, "|") {
		t.Errorf("walk = %v, want %v (symlinked directory listed but not entered)", seen, want)
	}
}

// ReadDirNames reads exactly one level and refuses every rel that Resolve refuses; LstatAll
// types the few names a page shows, without following a symlink out of the root (idea 02).
func TestReadDirNamesAndLstatAll(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"Filme", "Serien", "Filme/Action"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "aussen")); err != nil {
		t.Fatal(err)
	}
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	names, err := ReadDirNames(root, ".", 0)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	if strings.Join(names, "|") != "Filme|Serien|aussen|note.txt" {
		t.Errorf("ReadDirNames(.) = %v, want the four entries of the top level only", names)
	}

	entries, err := LstatAll(root, ".", names)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]fs.FileMode{}
	for _, e := range entries {
		got[e.Name()] = e.Type()
	}
	if mode, ok := got["aussen"]; !ok || mode&fs.ModeSymlink == 0 {
		t.Errorf("the symlink out of the root is typed as %v, want a symlink entry", mode)
	}
	if mode, ok := got["Filme"]; !ok || !mode.IsDir() {
		t.Errorf("Filme = %v, want a directory", mode)
	}
	if mode, ok := got["note.txt"]; !ok || mode.IsDir() {
		t.Errorf("note.txt = %v, want a file", mode)
	}
	// A name that vanished between the listing and the stat costs its row, not the page.
	entries, err = LstatAll(root, ".", []string{"Filme", "weg", "Serien"})
	if err != nil || len(entries) != 2 {
		t.Errorf("LstatAll with a vanished name = %d entries, %v; want the other two", len(entries), err)
	}

	// The symlink is listed, but it cannot be read through: that would leave the root.
	if _, err := ReadDirNames(root, "aussen", 0); err == nil {
		t.Error("reading through a symlink that leaves the root must fail")
	}
	if _, err := LstatAll(root, "aussen", []string{"config"}); err != nil {
		t.Errorf("LstatAll below a symlink out of the root = %v, want no error and no entry", err)
	}
	// Reading something that is no directory fails, capped or not; an empty listing would let a
	// caller mistake a file for an empty folder.
	for _, max := range []int{0, 10} {
		if _, err := ReadDirNames(root, "note.txt", max); err == nil {
			t.Errorf("ReadDirNames of a file (max=%d) = nil, want an error", max)
		}
	}
	if names, err := ReadDirNames(root, "Filme", 0); err != nil || strings.Join(names, "|") != "Action" {
		t.Errorf("ReadDirNames(Filme) = %v, %v; want one entry", names, err)
	}
	for _, rel := range []string{"", "..", "a/../b", "/etc", "a\x00b"} {
		if _, err := ReadDirNames(root, rel, 0); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("ReadDirNames(%q) error = %v, want ErrOutsideRoot", rel, err)
		}
		if _, err := LstatAll(root, rel, []string{"x"}); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("LstatAll(%q) error = %v, want ErrOutsideRoot", rel, err)
		}
	}
}

// max caps how many names a huge directory costs; the caller sees the cap from the length.
func TestReadDirNamesMax(t *testing.T) {
	dir := t.TempDir()
	for i := range 2500 {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprintf("d%04d", i)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, max := range []int{1, 200, 2000} {
		names, err := ReadDirNames(root, ".", max)
		if err != nil || len(names) != max {
			t.Errorf("ReadDirNames(max=%d) = %d names, %v", max, len(names), err)
		}
	}
	names, err := ReadDirNames(root, ".", 5000)
	if err != nil || len(names) != 2500 {
		t.Errorf("ReadDirNames(max=5000) = %d names, %v; want all 2500", len(names), err)
	}
}

// FSTypeAt classifies through the descriptor of the directory, so it agrees with DetectFS on the
// same directory and refuses a rel that leaves the root.
func TestFSTypeAt(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Filme"), 0o750); err != nil {
		t.Fatal(err)
	}
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := DetectFS(filepath.Join(dir, "Filme"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := FSTypeAt(root, "Filme")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("FSTypeAt = %s, DetectFS = %s", got, want)
	}
	if _, err := FSTypeAt(root, "../.."); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("FSTypeAt(../..) error = %v, want ErrOutsideRoot", err)
	}
	if _, err := FSTypeAt(root, "missing"); err == nil {
		t.Error("expected an error for a directory that does not exist")
	}
}
