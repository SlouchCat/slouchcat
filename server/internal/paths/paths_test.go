package paths

import (
	"errors"
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
