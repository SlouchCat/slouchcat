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
		if _, err := f.libs.Create(ctx, model.LibraryRequest{Name: filepath.Base(path), Kind: model.LibraryVideo, Roots: movieRoots(path)}); err != nil {
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
	// The roots come from ListRoots: List counts the items and files of every library, 1.8 s on
	// CT 190, and the browser asks for the roots on every click.
	if f.libs.listed != 0 {
		t.Errorf("the folder browser called List %d times, want ListRoots only", f.libs.listed)
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
