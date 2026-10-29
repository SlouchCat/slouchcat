package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func TestUnderAnyRoot(t *testing.T) {
	t.Parallel()
	roots := []string{"/media", "/mnt/nas/"}
	for path, want := range map[string]bool{
		"/media":              true,
		"/media/":             true,
		"/media/Filme":        true,
		"/media/Filme/A":      true,
		"/mnt/nas/Serien":     true,
		"/mediaX":             false, // a prefix on the raw string, not a path below the root
		"/media-archiv":       false,
		"/mnt":                false,
		"/config":             false,
		"/media/../config":    false, // cleaned first, so it does not sneak past
		"/media/Filme/../../": false,
	} {
		if got := UnderAnyRoot(path, roots); got != want {
			t.Errorf("UnderAnyRoot(%q) = %v, want %v", path, got, want)
		}
	}
	if UnderAnyRoot("/media/Filme", nil) {
		t.Error("without roots nothing is below a root; the caller decides what that means")
	}
}

// E14: a new library folder must lie below a browsable root, and the rule survives a symlinked
// root such as /media/ablage -> /config, which passes every lexical check.
func TestValidateNewRoot(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	media := t.TempDir()
	secret := t.TempDir()
	filme := filepath.Join(media, "Filme")
	if err := os.Mkdir(filme, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(media, "ablage")); err != nil {
		t.Fatal(err)
	}
	WithBrowseRoots([]string{media})(e.svc)

	if err := e.svc.ValidateNewRoot(e.ctx, filme); err != nil {
		t.Errorf("a folder below the root = %v, want nil", err)
	}
	if err := e.svc.ValidateNewRoot(e.ctx, media); err != nil {
		t.Errorf("the root itself = %v, want nil", err)
	}
	// A file outside the roots is refused as not browsable, not as "not a directory": the root
	// check runs before anything looks at the path, so the answer tells nothing about it.
	outsideFile := filepath.Join(secret, "film.mkv")
	if err := os.WriteFile(outsideFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{secret, outsideFile, filepath.Join(secret, "x"), media + "X", "/"} {
		err := e.svc.ValidateNewRoot(e.ctx, path)
		if !errors.Is(err, ErrNotBrowsable) {
			t.Errorf("ValidateNewRoot(%q) = %v, want ErrNotBrowsable", path, err)
		}
		if !errors.Is(err, model.ErrLibraryPathInvalid) {
			t.Errorf("ValidateNewRoot(%q) must wrap ErrLibraryPathInvalid, got %v", path, err)
		}
	}
	// The symlink passes the lexical check and is caught by the resolved one.
	if err := e.svc.ValidateNewRoot(e.ctx, filepath.Join(media, "ablage")); !errors.Is(err, ErrNotBrowsable) {
		t.Errorf("a symlinked folder that leaves the root = %v, want ErrNotBrowsable", err)
	}
	// Below the root but not a usable directory: the normal path errors, not ErrNotBrowsable.
	missing := filepath.Join(media, "fehlt")
	err := e.svc.ValidateNewRoot(e.ctx, missing)
	if !errors.Is(err, model.ErrLibraryPathInvalid) || errors.Is(err, ErrNotBrowsable) {
		t.Errorf("a missing folder below the root = %v, want a plain ErrLibraryPathInvalid", err)
	}
	for _, path := range []string{"", "relative/path"} {
		if err := e.svc.ValidateNewRoot(e.ctx, path); !errors.Is(err, model.ErrLibraryPathInvalid) {
			t.Errorf("ValidateNewRoot(%q) = %v, want ErrLibraryPathInvalid", path, err)
		}
	}
}

// A library root that passes the lexical check but resolves outside keeps its right: it has been
// a root for months, and an update must not stop the installation (E14, grandfathering last).
func TestValidateNewRootGrandfathersASymlinkedRoot(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	media := t.TempDir()
	elsewhere := t.TempDir()
	link := filepath.Join(media, "ablage")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	e.create(link) // created before the rule existed
	WithBrowseRoots([]string{media})(e.svc)

	if err := e.svc.ValidateNewRoot(e.ctx, link); err != nil {
		t.Errorf("the existing root behind a symlink = %v, want nil", err)
	}
	// A new folder on the same symlink is still refused: only the existing root is protected.
	if err := os.Mkdir(filepath.Join(elsewhere, "neu"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ValidateNewRoot(e.ctx, filepath.Join(link, "neu")); !errors.Is(err, ErrNotBrowsable) {
		t.Errorf("a new folder behind the symlink = %v, want ErrNotBrowsable", err)
	}
}

// Without a configured root every valid folder passes: an installation whose media are somewhere
// else must still be able to finish its setup (E14).
func TestValidateNewRootWithoutRoots(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	if err := e.svc.ValidateNewRoot(e.ctx, dir); err != nil {
		t.Errorf("without browse roots = %v, want nil", err)
	}
}

// A folder that is already a library root keeps its right when the roots change under it, by
// exact path equality: an update must not stop a running installation (E14).
func TestValidateNewRootGrandfathersExistingRoots(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	old := t.TempDir()
	sub := filepath.Join(old, "Unterordner")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	e.create(old) // created before the rule existed: no browse roots yet
	WithBrowseRoots([]string{t.TempDir()})(e.svc)

	if err := e.svc.ValidateNewRoot(e.ctx, old); err != nil {
		t.Errorf("the existing root = %v, want nil", err)
	}
	// Grandfathering is equality, never a prefix: the right does not spread downwards.
	if err := e.svc.ValidateNewRoot(e.ctx, sub); !errors.Is(err, ErrNotBrowsable) {
		t.Errorf("a folder below an existing root = %v, want ErrNotBrowsable", err)
	}
	// And Create refuses a library on a path below a configured root rule it does not satisfy.
	if _, err := e.svc.Create(e.ctx, testRequest("Neu", sub)); !hasFieldError(err, "roots[0].path", "library_path_invalid") {
		t.Errorf("Create below no browsable root = %v, want roots[0].path library_path_invalid", err)
	}
}
