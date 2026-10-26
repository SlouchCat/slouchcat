package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// imageFixture is the vistest library with its edge cases (a title with a missing file, a deleted
// title) and an administration whose cache directory is the fixture's; logs go into a buffer.
func imageFixture(t *testing.T) (*vistest.Fixture, *Service, *bytes.Buffer) {
	t.Helper()
	f := vistest.New(t)
	f.AddEdgeCases(t)
	var logs bytes.Buffer
	log, err := logging.New(&logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(f.DB, auth.New(f.DB, log), &fakeLibraries{}, nil, &fakeTMDB{}, func(string) string { return "" }, log).
		WithSystem(SystemSource{CacheDir: f.CacheDir})
	return f, svc, &logs
}

// cacheFile reads a file of the fixture's image cache.
func cacheFile(t *testing.T, f *vistest.Fixture, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.CacheDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// readImage opens an image through ItemImage and returns its media type and bytes.
func readImage(t *testing.T, svc *Service, itemID, kind, size string, preferWebP bool) (string, []byte, error) {
	t.Helper()
	src, err := svc.ItemImage(context.Background(), itemID, kind, size, preferWebP)
	if err != nil {
		return "", nil, err
	}
	defer src.File.Close()
	data, err := io.ReadAll(src.File)
	if err != nil {
		t.Fatal(err)
	}
	return src.MediaType, data, nil
}

// ItemImage serves a title no profile of the fixture may see (missing file) without asking for a
// profile, WebP when preferred and stored, else JPEG; deleted titles, unknown sizes and rows
// without file are model.ErrNotFound, and without a cache directory nothing is opened.
func TestItemImage(t *testing.T) {
	t.Parallel()
	f, svc, logs := imageFixture(t)
	var hidden vistest.Movie
	for _, m := range f.Movies {
		if m.Missing {
			hidden = m
		}
	}
	if hidden.ID == "" || f.VisibleTo(f.P18, hidden) {
		t.Fatalf("fixture has no title hidden from P18: %+v", hidden)
	}
	posterJPEG := "images/" + hidden.ID + "/poster-w200.jpg"
	posterWebP := "images/" + hidden.ID + "/poster-w200.webp"

	typ, data, err := readImage(t, svc, hidden.ID, "poster", "w200", false)
	if err != nil || typ != "image/jpeg" || !bytes.Equal(data, cacheFile(t, f, posterJPEG)) {
		t.Errorf("JPEG poster of a hidden title: type %q, err %v", typ, err)
	}
	typ, data, err = readImage(t, svc, hidden.ID, "poster", "w200", true)
	if err != nil || typ != "image/webp" || !bytes.Equal(data, cacheFile(t, f, posterWebP)) {
		t.Errorf("WebP poster of a hidden title: type %q, err %v", typ, err)
	}
	typ, data, err = readImage(t, svc, hidden.ID, "backdrop", "w1280", false)
	if err != nil || typ != "image/jpeg" || !bytes.Equal(data, cacheFile(t, f, hidden.BackdropRel)) {
		t.Errorf("backdrop of a hidden title: type %q, err %v", typ, err)
	}

	// Without a WebP row the JPEG row answers a client that prefers WebP.
	if _, err := f.DB.Write.ExecContext(context.Background(),
		`DELETE FROM images WHERE item_id = ? AND kind = 'poster' AND size = 'w800' AND format = 'webp'`, hidden.ID); err != nil {
		t.Fatal(err)
	}
	if typ, _, err := readImage(t, svc, hidden.ID, "poster", "w800", true); err != nil || typ != "image/jpeg" {
		t.Errorf("WebP preferred without WebP row: type %q, err %v", typ, err)
	}

	var deleted vistest.Movie
	for _, m := range f.Movies {
		if m.Deleted {
			deleted = m
		}
	}
	for _, c := range []struct{ name, id, kind, size string }{
		{"deleted title", deleted.ID, "poster", "w200"},
		{"unknown title", "01J0000000000000000000XXXX", "poster", "w200"},
		{"kept original", hidden.ID, "poster", "original"},
		{"size not stored", hidden.ID, "poster", "w500"},
	} {
		if _, _, err := readImage(t, svc, c.id, c.kind, c.size, false); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: err %v, want model.ErrNotFound", c.name, err)
		}
	}

	// A row whose file is gone is 404 with a warning, like the scoped image route.
	if err := os.Remove(filepath.Join(f.CacheDir, filepath.FromSlash(posterJPEG))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readImage(t, svc, hidden.ID, "poster", "w200", false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("row without file: err %v, want model.ErrNotFound", err)
	}
	if !strings.Contains(logs.String(), "image row without file") {
		t.Errorf("no warning for the row without file: %s", logs.String())
	}

	bare := New(f.DB, auth.New(f.DB, vistest.Logger()), &fakeLibraries{}, nil, &fakeTMDB{}, func(string) string { return "" }, vistest.Logger())
	if _, err := bare.ItemImage(context.Background(), hidden.ID, "poster", "w400", false); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("without cache directory: err %v, want ErrNotConfigured", err)
	}
}

// Every pick carries the content hash of its title's w200 JPEG poster; a deleted title has none,
// like the image route answers 404 for it.
func TestTopPicksPosterHash(t *testing.T) {
	t.Parallel()
	f, svc, _ := imageFixture(t)
	picks, err := svc.ListTopPicks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	deleted := map[string]bool{}
	for _, m := range f.Movies {
		deleted[m.ID] = m.Deleted
	}
	seenDeleted := false
	for _, p := range picks {
		if deleted[p.ItemID] {
			seenDeleted = true
			if p.PosterHash != "" {
				t.Errorf("pick %d of a deleted title has poster hash %q", p.ID, p.PosterHash)
			}
			continue
		}
		sum := sha256.Sum256(cacheFile(t, f, "images/"+p.ItemID+"/poster-"+model.AdminPosterSize+".jpg"))
		if want := hex.EncodeToString(sum[:]); p.PosterHash != want {
			t.Errorf("pick %d (%s): poster hash %q, want %q", p.ID, p.ItemID, p.PosterHash, want)
		}
	}
	if len(picks) == 0 || !seenDeleted {
		t.Fatalf("fixture picks %+v lack a deleted title", picks)
	}
}
