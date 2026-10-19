// Package vistest builds the child-profile fixture of plan section 10.2 for tests: a temporary
// database with one movie library, movies rated FSK 0, 6, 12, 16, 18 and one without rating, each
// with a real media file, texts in de and en, a stored poster and backdrop (w1280 and w780),
// progress rows and top picks, plus the profiles P18, P16, P12, P6 and P0 of one account.
// AddEdgeCases adds a second library with a profile restricted to the first one, a title whose
// only file is missing and a soft-deleted title.
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
	RelPath          string // relative to Fixture.MediaDir (OtherMediaDir for the OtherLibrary title)
	Content          []byte
	PosterRel        string // poster w400 JPEG, relative to Fixture.CacheDir
	PosterWebPRel    string // poster w400 WebP
	BackdropRel      string // backdrop w1280 JPEG
	BackdropSmallRel string // backdrop w780 JPEG

	OtherLibrary bool // in library X, which PX may not see
	Missing      bool // its only file has missing_since set
	Deleted      bool // soft-deleted
}

// Fixture is the seeded database with its directories.
type Fixture struct {
	DB            *store.DB
	MediaDir      string
	OtherMediaDir string // root of library X (AddEdgeCases)
	CacheDir      string
	Account       model.Account
	P18           model.Profile // owner, max 18, language de
	P16           model.Profile // max 16, language de
	P12           model.Profile // max 12, language de
	P6            model.Profile // max 6, language en
	P0            model.Profile // max 0, language de, no progress
	PX            model.Profile // max 18, only library 1 (AddEdgeCases)
	// Movies are ordered by rating 0, 6, 12, 16, 18, unrated; AddEdgeCases appends the
	// OtherLibrary, Missing and Deleted titles (all rated 0).
	Movies []Movie

	libraryID int64
	rootID    int64
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

// VisibleTo reports whether the profile may see m by the rules of plan 3.3 (unknown counts as 18;
// missing and deleted titles are invisible; library X needs all libraries, as PX only has
// library 1).
func VisibleTo(p model.Profile, m Movie) bool {
	if m.Missing || m.Deleted || (m.OtherLibrary && !p.AllLibraries) {
		return false
	}
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
	f.P16 = add("P16", 16, "de", 0)
	f.P12 = add("P12", 12, "de", 0)
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
	f.libraryID, f.rootID = lib.ID, root.ID
	ratings := []*int{ptr(0), ptr(6), ptr(12), ptr(16), ptr(18), nil}
	for i, rating := range ratings {
		label := "NN"
		if rating != nil {
			label = fmt.Sprintf("%02d", *rating)
		}
		m := Movie{Rating: rating, Label: label}
		f.Movies = append(f.Movies, f.addMovie(t, m, lib.ID, root.ID, f.MediaDir, i))
	}
	// Progress: P18 started the 12-rated movie, P6 the 6-rated one (both at 1/6 of the runtime).
	exec(t, f.DB, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, ?, ?, 0, 5000)`,
		f.P18.ID, f.Rated(12).ID, DurationMs/6, DurationMs)
	exec(t, f.DB, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, ?, ?, 0, 5000)`,
		f.P6.ID, f.Rated(6).ID, DurationMs/6, DurationMs)
	// Top picks for every profile: the 18-rated movie first, then the 6-rated one.
	exec(t, f.DB, `INSERT INTO top_picks (item_id, position, created_at) VALUES (?, 0, 1), (?, 1, 1)`, f.Rated(18).ID, f.Rated(6).ID)
}

// AddEdgeCases seeds library X with a movie rated 0, the profile PX (max 18, only library 1), a
// movie rated 0 whose only file is missing and a soft-deleted movie rated 0, each with file,
// texts, images, progress of P18 and PX and a top pick, so only the visibility rules hide them.
func (f *Fixture) AddEdgeCases(t testing.TB) {
	t.Helper()
	ctx := context.Background()
	q := appq.New(f.DB.Write)
	p, err := q.InsertProfile(ctx, appq.InsertProfileParams{AccountID: f.Account.ID, Name: "PX", MaxRating: 18, Language: "de", AllLibraries: 0, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	f.PX = model.Profile{ID: p.ID, AccountID: f.Account.ID, Name: "PX", MaxRating: 18, Language: "de"}
	exec(t, f.DB, `INSERT INTO profile_libraries (profile_id, library_id) VALUES (?, ?)`, f.PX.ID, f.libraryID)

	f.OtherMediaDir = t.TempDir()
	lib, err := q.InsertLibrary(ctx, appq.InsertLibraryParams{Name: "Filme X", Kind: "movies", Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	root, err := q.InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{LibraryID: lib.ID, Path: f.OtherMediaDir, FsType: "ext4", Watch: "inotify", Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	other := f.addMovie(t, Movie{Rating: ptr(0), Label: "X0", OtherLibrary: true}, lib.ID, root.ID, f.OtherMediaDir, 6)
	missing := f.addMovie(t, Movie{Rating: ptr(0), Label: "M0", Missing: true}, f.libraryID, f.rootID, f.MediaDir, 7)
	deleted := f.addMovie(t, Movie{Rating: ptr(0), Label: "D0", Deleted: true}, f.libraryID, f.rootID, f.MediaDir, 8)
	exec(t, f.DB, `UPDATE media_files SET missing_since = 2 WHERE id = ?`, missing.FileID)
	exec(t, f.DB, `UPDATE items SET deleted_at = 2 WHERE id = ?`, deleted.ID)
	for i, m := range []Movie{other, missing, deleted} {
		for _, pid := range []int64{f.P18.ID, f.PX.ID} {
			exec(t, f.DB, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, ?, ?, 0, 6000)`,
				pid, m.ID, DurationMs/6, DurationMs)
		}
		exec(t, f.DB, `INSERT INTO top_picks (item_id, position, created_at) VALUES (?, ?, 1)`, m.ID, 2+i)
		f.Movies = append(f.Movies, m)
	}
}

// addMovie writes the media file of m below mediaDir and inserts the item (number i sets ids,
// year and order) with its file, texts and images; it returns m with the generated fields.
func (f *Fixture) addMovie(t testing.TB, m Movie, libraryID, rootID int64, mediaDir string, i int) Movie {
	t.Helper()
	m.ID = "01J9000000000000000000R0" + m.Label
	m.FileID = "01J9000000000000000000F0" + m.Label
	m.RelPath = fmt.Sprintf("Movie %s (20%02d)/Movie %s (20%02d).mp4", m.Label, 10+i, m.Label, 10+i)
	m.Content = make([]byte, FileSize)
	if _, err := rand.Read(m.Content); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(mediaDir, filepath.FromSlash(m.RelPath)), m.Content)
	exec(t, f.DB, `INSERT INTO items (id, kind, library_id, title, sort_title, original_title, year, release_date,
	               runtime_minutes, genres, tmdb_id, own_rating, metadata_state, added_at, created_at, updated_at)
	               VALUES (?, 'movie', ?, ?, ?, ?, ?, ?, 120, ?, ?, ?, 'matched', ?, 1, 1)`,
		m.ID, libraryID, "movie "+m.Label, "movie "+m.Label, "Original "+m.Label, 2010+i, fmt.Sprintf("20%02d-05-01", 10+i),
		`{"de":["Abenteuer"],"en":["Adventure"]}`, 1000+i, m.Rating, int64(1000*(i+1)))
	exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec,
	               audio_codec, width, height, duration_ms, created_at, updated_at)
	               VALUES (?, ?, ?, ?, ?, 1, 'mp4', 'h264', 'aac', 1920, 1080, ?, 1, 1)`,
		m.FileID, m.ID, rootID, m.RelPath, FileSize, DurationMs)
	exec(t, f.DB, `INSERT INTO item_texts (item_id, lang, title, overview, tagline) VALUES (?, 'de', ?, ?, ?), (?, 'en', ?, ?, ?)`,
		m.ID, "Film "+m.Label, "Handlung "+m.Label, "Leitsatz "+m.Label,
		m.ID, "Movie "+m.Label, "Plot "+m.Label, "Tagline "+m.Label)
	// The image pipeline's layout (plan 3.1): every size as JPEG and WebP, the accent on the
	// poster rows, the kept original. Tiny pixel sizes keep the fixture fast; the rows record them.
	for _, im := range []struct {
		kind, size string
		w, h       int
	}{
		{"poster", "w200", 4, 6}, {"poster", "w400", 10, 15}, {"poster", "w800", 20, 30},
		{"backdrop", "w780", 8, 5}, {"backdrop", "w1280", 16, 9}, {"backdrop", "w1920", 24, 14},
	} {
		jpg := f.addImage(t, m.ID, im.kind, im.size, "jpeg", im.w, im.h)
		webp := f.addImage(t, m.ID, im.kind, im.size, "webp", im.w, im.h)
		switch im.kind + "/" + im.size {
		case "poster/w400":
			m.PosterRel, m.PosterWebPRel = jpg, webp
		case "backdrop/w1280":
			m.BackdropRel = jpg
		case "backdrop/w780":
			m.BackdropSmallRel = jpg
		}
	}
	f.addImage(t, m.ID, "poster", "original", "jpeg", 40, 60)
	return m
}

// ItemAccent is the accent colour every fixture title carries on its poster rows.
const ItemAccent = "#c81e28"

// addImage writes a small image under the cache directory and records it; it returns the rel_path.
// A JPEG is a real image; a WebP is a RIFF/WEBP header with a marker (the standard library cannot
// encode WebP; the tests compare bytes, not pixels).
func (f *Fixture) addImage(t testing.TB, itemID, kind, size, format string, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	ext, mediaType := "jpg", "image/jpeg"
	if format == "webp" {
		ext, mediaType = "webp", "image/webp"
		buf.WriteString("RIFF\x00\x00\x00\x00WEBP" + kind + "-" + size + "-" + itemID) // size field left 0
	} else {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for x := range w {
			for y := range h {
				img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: 128, A: 255})
			}
		}
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			t.Fatal(err)
		}
	}
	var accent any
	if kind == "poster" && size != "original" {
		accent = ItemAccent
	}
	rel := fmt.Sprintf("images/%s/%s-%s.%s", itemID, kind, size, ext)
	writeFile(t, filepath.Join(f.CacheDir, filepath.FromSlash(rel)), buf.Bytes())
	sum := sha256.Sum256(buf.Bytes())
	exec(t, f.DB, `INSERT INTO images (item_id, kind, size, format, lang, source_url, rel_path, content_hash, media_type, width, height, bytes, accent, created_at)
	               VALUES (?, ?, ?, ?, NULL, 'https://image.tmdb.org/t/p/original/x.jpg', ?, ?, ?, ?, ?, ?, ?, 1)`,
		itemID, kind, size, format, rel, hex.EncodeToString(sum[:]), mediaType, w, h, buf.Len(), accent)
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
