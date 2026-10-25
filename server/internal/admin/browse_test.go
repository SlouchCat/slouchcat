package admin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/browse"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// E5: the wizard's root list answers from the list filled at start-up. The proof is a root that
// is deleted after the classification: a list that stats per request would drop it.
func TestSetupRootsAnswersFromTheStartupList(t *testing.T) {
	t.Parallel()
	ctx := model.WithClientIP(context.Background(), "192.0.2.30")
	f := newFixture(t)
	dir := filepath.Join(t.TempDir(), "medien")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	f.svc.WithBrowse([]string{dir, "/does/not/exist"}, browse.ClassifyRoots([]string{dir, "/does/not/exist"}, quiet), browse.New(quiet))

	roots, err := f.svc.SetupRoots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].Path != dir {
		t.Fatalf("roots = %+v, want only the one that exists", roots)
	}
	// Before an account exists nobody learns which mount is a network share.
	if roots[0].FSType != "" || roots[0].Warning != "" {
		t.Errorf("the wizard's list describes the file system: %+v", roots[0])
	}

	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	roots, err = f.svc.SetupRoots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].Path != dir {
		t.Errorf("roots = %+v after the directory was removed: the answer reaches the file system per request", roots)
	}
}

// E5: the list has a pot of its own. Emptying it must leave the attempt limit of login, setup,
// PIN and invitations untouched, because the setup page fetches the list on every load.
func TestSetupRootsHasItsOwnLimit(t *testing.T) {
	t.Parallel()
	ctx := model.WithClientIP(context.Background(), "192.0.2.31")
	f := newFixture(t)
	f.svc.WithBrowse([]string{t.TempDir()}, browse.ClassifyRoots([]string{t.TempDir()}, quiet), browse.New(quiet))

	var err error
	for i := range setupRootsPerMin + 1 {
		if _, err = f.svc.SetupRoots(ctx); err != nil {
			if i < setupRootsPerMin {
				t.Fatalf("request %d of its own pot: %v", i, err)
			}
			break
		}
	}
	if !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("request %d = %v, want ErrTooManyAttempts", setupRootsPerMin+1, err)
	}
	// The shared pot is untouched: the wizard still runs.
	if _, err := f.svc.CheckSetupPath(ctx, t.TempDir()); err != nil {
		t.Errorf("the wizard's path check after %d root lists: %v", setupRootsPerMin+1, err)
	}
	if _, _, err := f.svc.RunSetup(ctx, setupRequest(), "test"); err != nil {
		t.Errorf("the wizard itself after %d root lists: %v", setupRootsPerMin+1, err)
	}
}

// The dialog's roots are the configured ones plus the roots of the libraries that exist; a
// library root below no configured root is listed but cannot be entered (E14).
func TestBrowseRoots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixture(t)
	media := t.TempDir()
	inside := filepath.Join(media, "Filme")
	if err := os.Mkdir(inside, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	f.svc.WithBrowse([]string{media}, browse.ClassifyRoots([]string{media}, quiet), browse.New(quiet))
	for _, path := range []string{inside, outside, media} {
		if _, err := f.libs.Create(ctx, model.LibraryRequest{Name: filepath.Base(path), Kind: "movies", Path: path}); err != nil {
			t.Fatal(err)
		}
	}

	roots, err := f.svc.BrowseRoots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]model.BrowseRoot{}
	for _, r := range roots {
		if _, twice := byPath[r.Path]; twice {
			t.Errorf("%s is listed twice", r.Path)
		}
		byPath[r.Path] = r
	}
	if got := byPath[media]; got.Source != model.BrowseRootConfigured || !got.Browsable || got.LibraryName == "" {
		t.Errorf("the configured root that is also a library root = %+v, want one row carrying the library", got)
	}
	if got := byPath[inside]; got.Source != model.BrowseRootLibrary || !got.Browsable || got.LibraryName != "Filme" {
		t.Errorf("a library root below the configured one = %+v, want it browsable", got)
	}
	if got := byPath[outside]; got.Source != model.BrowseRootLibrary || got.Browsable {
		t.Errorf("a grandfathered library root = %+v, want it listed but not browsable", got)
	}
	// Listed, but not a way out: entering it answers like an unknown root.
	if _, err := f.svc.BrowsePaths(ctx, model.BrowseRequest{RootID: byPath[outside].ID}); !errors.Is(err, model.ErrPathNotBrowsable) {
		t.Errorf("entering a grandfathered root = %v, want ErrPathNotBrowsable", err)
	}
	if _, err := f.svc.BrowsePaths(ctx, model.BrowseRequest{RootID: byPath[media].ID}); err != nil {
		t.Errorf("entering the configured root = %v", err)
	}
}

// Without the browser wired nothing is browsable, and the dashboard says nothing is wrong.
func TestBrowseNotConfigured(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if _, err := f.svc.BrowsePaths(context.Background(), model.BrowseRequest{RootID: "c0"}); !errors.Is(err, model.ErrPathNotBrowsable) {
		t.Errorf("without the browser = %v, want ErrPathNotBrowsable", err)
	}
	if got := f.svc.browseHealth(); got != (model.BrowseHealth{}) {
		t.Errorf("browse health = %+v, want the zero value", got)
	}
	f.svc.WithBrowse(nil, nil, browse.New(quiet))
	if got := f.svc.browseHealth(); got.LostSlots != 0 || got.Slots != browse.DefaultSlots {
		t.Errorf("browse health = %+v, want no lost slot and the default number", got)
	}
}
