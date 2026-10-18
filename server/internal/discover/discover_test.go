package discover

import (
	"context"
	"errors"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func rowKeys(rows []model.HomeRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Key)
	}
	return out
}

func TestHomeRowsInOrderWithoutEmptyRows(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()

	rows, err := s.Home(ctx, f.P6)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowPicks, RowContinue, RowRecent}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P6 rows = %v, want %v", rowKeys(rows), want)
	}
	// P0 has no progress: the continue row is omitted (E-9).
	rows, err = s.Home(ctx, f.P0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowPicks, RowRecent}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P0 rows = %v, want %v", rowKeys(rows), want)
	}
}

func TestItemCarriesMimeType(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	d, err := s.Item(context.Background(), f.P18, f.Unrated().ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.MediaFiles) != 1 || d.MediaFiles[0].MimeType == "" {
		t.Errorf("media files = %+v", d.MediaFiles)
	}
	if _, err := s.Item(context.Background(), f.P6, f.Unrated().ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unrated title for P6: err = %v", err)
	}
}

func TestImageOpensThroughCacheRoot(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	m := f.Rated(6)
	src, err := s.Image(ctx, f.P6, m.ID, "backdrop", "w1280")
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(src.File)
	_ = src.File.Close()
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 16 || b.Dy() != 9 || src.MediaType != "image/jpeg" {
		t.Errorf("image %v, type %s", b, src.MediaType)
	}
	if _, err := s.Image(ctx, f.P6, f.Rated(12).ID, "poster", "w500"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible title: err = %v", err)
	}
	if err := os.Remove(filepath.Join(f.CacheDir, filepath.FromSlash(m.PosterRel))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Image(ctx, f.P6, m.ID, "poster", "w500"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("row without file: err = %v, want ErrNotFound", err)
	}
}
