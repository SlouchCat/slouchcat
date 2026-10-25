package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// ReadDir reads exactly one level, lists a symlinked directory without following it and refuses
// every rel that Resolve refuses (the folder browser of idea 02 reads directories with it).
func TestReadDirReadsOneLevel(t *testing.T) {
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

	entries, err := ReadDir(root, ".", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]fs.FileMode{}
	for _, e := range entries {
		got[e.Name()] = e.Type()
	}
	if len(got) != 4 {
		t.Errorf("ReadDir(.) = %v, want four entries of the top level only", got)
	}
	if mode, ok := got["aussen"]; !ok || mode&fs.ModeSymlink == 0 {
		t.Errorf("the symlink out of the root is listed as %v, want a symlink entry", mode)
	}
	if mode, ok := got["Filme"]; !ok || !mode.IsDir() {
		t.Errorf("Filme = %v, want a directory", mode)
	}
	if _, ok := got["Action"]; ok {
		t.Error("ReadDir went into a subdirectory; it reads one level")
	}

	// The symlink is listed, but it cannot be read through: that would leave the root.
	if _, err := ReadDir(root, "aussen", 0); err == nil {
		t.Error("reading through a symlink that leaves the root must fail")
	}
	if entries, err := ReadDir(root, "Filme", 0); err != nil || len(entries) != 1 {
		t.Errorf("ReadDir(Filme) = %v, %v; want one entry", entries, err)
	}
	for _, rel := range []string{"", "..", "a/../b", "/etc", "a\x00b"} {
		if _, err := ReadDir(root, rel, 0); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("ReadDir(%q) error = %v, want ErrOutsideRoot", rel, err)
		}
	}
}

// max caps how many entries a huge directory costs; the caller sees the cap from the length.
func TestReadDirMax(t *testing.T) {
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
		entries, err := ReadDir(root, ".", max)
		if err != nil || len(entries) != max {
			t.Errorf("ReadDir(max=%d) = %d entries, %v", max, len(entries), err)
		}
	}
	entries, err := ReadDir(root, ".", 5000)
	if err != nil || len(entries) != 2500 {
		t.Errorf("ReadDir(max=5000) = %d entries, %v; want all 2500", len(entries), err)
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
