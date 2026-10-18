// Package vistest builds the child-profile fixture of plan section 10.2 for tests: a temporary
// database with one movie library, movies rated FSK 0, 6, 12, 16, 18 and one without rating, each
// with a real media file, texts in de and en, a stored poster and backdrop (w1280 and w780),
// progress rows and top picks, plus the profiles P18, P6 and P0 of one account.
//
// It lives below visibility because seeding writes the title tables directly, which only the
// packages allowed by the itemsaccess analyzer may do. Only tests import it.
package vistest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Duration of every fixture movie in milliseconds (2 h).
const DurationMs = 7_200_000

// FileSize is the size of every fixture media file (64 KiB of random bytes).
const FileSize = 64 << 10

// Movie is one fixture title.
type Movie struct {
	ID               string
	Rating           *int // nil = unrated
	Label            string
	FileID           string
	RelPath          string // relative to Fixture.MediaDir
	Content          []byte
	PosterRel        string // relative to Fixture.CacheDir
	BackdropRel      string
	BackdropSmallRel string // backdrop in w780
}

// Fixture is the seeded database with its directories.
type Fixture struct {
	DB       *store.DB
	MediaDir string
	CacheDir string
	Account  model.Account
	P18      model.Profile // owner, max 18, language de
	P6       model.Profile // max 6, language en
	P0       model.Profile // max 0, language de, no progress
	Movies   []Movie       // ordered by rating 0, 6, 12, 16, 18, unrated
}

// Rated returns the movie with the given rating; it panics for a rating the fixture lacks.
func (f *Fixture) Rated(rating int) Movie {
	for _, m := range f.Movies {
		if m.Rating != nil && *m.Rating == rating {
			return m
		}
	}
	panic(fmt.Sprintf("vistest: no movie rated %d", rating))
}

// Unrated returns the movie without rating.
func (f *Fixture) Unrated() Movie {
	for _, m := range f.Movies {
		if m.Rating == nil {
			return m
		}
	}
	panic("vistest: no unrated movie")
}

// VisibleTo reports whether the profile may see m by the rules of plan 3.3 (unknown counts as 18).
func VisibleTo(p model.Profile, m Movie) bool {
	r := 18
	if m.Rating != nil {
		r = *m.Rating
	}
	return r <= p.MaxRating
}

// Logger discards everything.
func Logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// New opens a fresh database in a temporary directory and seeds it.
func New(t testing.TB) *Fixture {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: Logger()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f := &Fixture{DB: db, MediaDir: t.TempDir(), CacheDir: t.TempDir()}
	f.seedProfiles(t, ctx)
	f.seedMovies(t, ctx)
	return f
}

func (f *Fixture) seedProfiles(t testing.TB, ctx context.Context) {
	t.Helper()
	q := appq.New(f.DB.Write)
	acc, err := q.InsertAccount(ctx, appq.InsertAccountParams{Email: "owner@example.test", PasswordHash: "x", Role: "owner", Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	f.Account = model.Account{ID: acc.ID, Email: acc.Email, Role: acc.Role}
	add := func(name string, maxRating int, lang string, isDefault int64) model.Profile {
		p, err := q.InsertProfile(ctx, appq.InsertProfileParams{AccountID: acc.ID, Name: name, MaxRating: int64(maxRating), Language: lang, AllLibraries: 1, IsDefault: isDefault, Now: 1})
		if err != nil {
			t.Fatal(err)
		}
		return model.Profile{ID: p.ID, AccountID: acc.ID, Name: name, MaxRating: maxRating, Language: lang, AllLibraries: true}
	}
	f.P18 = add("P18", 18, "de", 1)
	f.P6 = add("P6", 6, "en", 0)
	f.P0 = add("P0", 0, "de", 0)
}

func (f *Fixture) seedMovies(t testing.TB, ctx context.Context) {
	t.Helper()
	q := appq.New(f.DB.Write)
	lib, err := q.InsertLibrary(ctx, appq.InsertLibraryParams{Name: "Filme", Kind: "movies", Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	root, err := q.InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{LibraryID: lib.ID, Path: f.MediaDir, FsType: "ext4", Watch: "inotify", Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	ratings := []*int{ptr(0), ptr(6), ptr(12), ptr(16), ptr(18), nil}
	for i, rating := range ratings {
		label := "NN"
		if rating != nil {
			label = fmt.Sprintf("%02d", *rating)
		}
		m := Movie{
			ID:      "01J9000000000000000000R0" + label,
			Rating:  rating,
			Label:   label,
			FileID:  "01J9000000000000000000F0" + label,
			RelPath: fmt.Sprintf("Movie %s (20%02d)/Movie %s (20%02d).mp4", label, 10+i, label, 10+i),
		}
		m.Content = make([]byte, FileSize)
		if _, err := rand.Read(m.Content); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(f.MediaDir, filepath.FromSlash(m.RelPath)), m.Content)
		exec(t, f.DB, `INSERT INTO items (id, kind, library_id, title, sort_title, original_title, year, release_date,
		               runtime_minutes, genres, tmdb_id, own_rating, metadata_state, added_at, created_at, updated_at)
		               VALUES (?, 'movie', ?, ?, ?, ?, ?, ?, 120, ?, ?, ?, 'matched', ?, 1, 1)`,
			m.ID, lib.ID, "movie "+label, "movie "+label, "Original "+label, 2010+i, fmt.Sprintf("20%02d-05-01", 10+i),
			`{"de":["Abenteuer"],"en":["Adventure"]}`, 1000+i, rating, int64(1000*(i+1)))
		exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec,
		               audio_codec, width, height, duration_ms, created_at, updated_at)
		               VALUES (?, ?, ?, ?, ?, 1, 'mp4', 'h264', 'aac', 1920, 1080, ?, 1, 1)`,
			m.FileID, m.ID, root.ID, m.RelPath, FileSize, DurationMs)
		exec(t, f.DB, `INSERT INTO item_texts (item_id, lang, title, overview, tagline) VALUES (?, 'de', ?, ?, ?), (?, 'en', ?, ?, ?)`,
			m.ID, "Film "+label, "Handlung "+label, "Leitsatz "+label,
			m.ID, "Movie "+label, "Plot "+label, "Tagline "+label)
		m.PosterRel = f.addImage(t, m.ID, "poster", "w500", 10, 15)
		m.BackdropRel = f.addImage(t, m.ID, "backdrop", "w1280", 16, 9)
		m.BackdropSmallRel = f.addImage(t, m.ID, "backdrop", "w780", 8, 5)
		f.Movies = append(f.Movies, m)
	}
	// Progress: P18 started the 12-rated movie, P6 the 6-rated one (both at 1/6 of the runtime).
	exec(t, f.DB, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, ?, ?, 0, 5000)`,
		f.P18.ID, f.Rated(12).ID, DurationMs/6, DurationMs)
	exec(t, f.DB, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, ?, ?, 0, 5000)`,
		f.P6.ID, f.Rated(6).ID, DurationMs/6, DurationMs)
	// Top picks for every profile: the 18-rated movie first, then the 6-rated one.
	exec(t, f.DB, `INSERT INTO top_picks (item_id, position, created_at) VALUES (?, 0, 1), (?, 1, 1)`, f.Rated(18).ID, f.Rated(6).ID)
}

// addImage writes a small JPEG under the cache directory and records it; it returns the rel_path.
func (f *Fixture) addImage(t testing.TB, itemID, kind, size string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	rel := fmt.Sprintf("images/%s/%s-%s.jpg", itemID, kind, size)
	writeFile(t, filepath.Join(f.CacheDir, filepath.FromSlash(rel)), buf.Bytes())
	sum := sha256.Sum256(buf.Bytes())
	exec(t, f.DB, `INSERT INTO images (item_id, kind, size, lang, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
	               VALUES (?, ?, ?, 'de', 'https://image.tmdb.org/t/p/x.jpg', ?, ?, 'image/jpeg', ?, ?, ?, 1)`,
		itemID, kind, size, rel, hex.EncodeToString(sum[:]), w, h, buf.Len())
	return rel
}

func writeFile(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func exec(t testing.TB, db *store.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Write.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func ptr(v int) *int { return &v }
