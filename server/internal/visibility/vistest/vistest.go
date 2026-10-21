// Package vistest builds the child-profile fixture of plan section 10.2 for tests: a temporary
// database with one movie library, movies rated FSK 0, 6, 12, 16, 18 and one without rating, each
// with a real media file, texts in de and en, a stored poster and backdrop (w1280 and w780),
// progress rows and top picks, plus the profiles P18, P16, P12, P6 and P0 of one account.
// AddEdgeCases adds a second library with a profile restricted to the first one, a title whose
// only file is missing and a soft-deleted title. AddSeries adds a series tree (series rated 12,
// season 1 with an episode without own rating, one rated 16 and a multi-episode file, the specials
// season with one episode, an extra) to library 1 with the M3 data around the movies (extras,
// versions, trailers, a person, an ignored file), and AddGuest a guest account limited to FSK 12
// and library 1 with the profiles PG (primary) and PG18. AddDiscover adds the M7 data: collections
// with mixed ratings, watchlist and favourite rows and picks per profile and with time windows.
//
// It lives below visibility because seeding writes the title tables directly, which only the
// packages allowed by the itemsaccess analyzer may do. Only tests import it.
package vistest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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

	LibraryID    int64 // the library the title is in (set when it is seeded)
	OtherLibrary bool  // in library X, which PX may not see
	Missing      bool  // its only file has missing_since set
	Unavailable  bool  // a container without available content children (AddLoneExtraSeries)
	Deleted      bool  // soft-deleted

	// Series tree entries and extras (AddSeries): Kind is "series", "season", "episode" or "extra"
	// ("" = movie); a container has no file; Effective and MinChild are the ratings the triggers
	// compute (an extra inherits the effective rating of its parent).
	Kind      string
	ParentID  string
	Container bool
	Effective *int
	MinChild  *int

	// Audio (AddAudio): the enclosure URL of a podcast episode and the file ids of the parts of a
	// multi-part audiobook in their order (FileID is the first; empty for a single file).
	Enclosure string
	PartIDs   []string
}

// Show is the series tree of AddSeries.
type Show struct {
	Series   Movie // rated 12
	Season   Movie // season 1, inherits 12
	E1       Movie // S01E01, no own rating, effective 12
	E2       Movie // S01E02, own rating 16
	E3       Movie // S01E03-E04: one file, episode_end 4, effective 12
	Specials Movie // season 0, inherits 12
	Special  Movie // S00E01, effective 12
	Extra    Movie // behind_the_scenes extra of the series, effective 12
}

// All returns the titles of the tree, series first.
func (s Show) All() []Movie {
	return []Movie{s.Series, s.Season, s.E1, s.E2, s.E3, s.Specials, s.Special, s.Extra}
}

// Person is the fixture person of AddSeries: a cast credit in the film rated 6 and the director
// credit of the film rated 18.
type Person struct {
	ID   int64
	Name string
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

	Show Show // series tree (AddSeries)
	// Extras of the film rated 6 (AddSeries): a trailer and a featurette, effective 6.
	Extras []Movie
	// Version2FileID is the second media file of the film rated 12 (1280x720, AddSeries); the
	// first file (1920x1080) stays the primary one.
	Version2FileID string
	Person         Person // AddSeries
	// Trailers counts the trailer rows per item id (AddSeries).
	Trailers map[string]int
	// IgnoredRelPath is the ignored_files row of library 1 (AddSeries).
	IgnoredRelPath string
	Guest          model.Account // guest account G: max 12, library 1 only (AddGuest)
	PG             model.Profile // primary profile of G, max 12, language en
	PG18           model.Profile // profile of G with max 18, capped to 12 by the account
	// Movies are ordered by rating 0, 6, 12, 16, 18, unrated; AddEdgeCases appends the
	// OtherLibrary, Missing and Deleted titles (all rated 0).
	Movies []Movie
	// Audio holds the audiobooks and podcasts of AddAudio (M6).
	Audio Audio
	// Discover holds the collections and picks of AddDiscover (M7).
	Discover Discover

	libraryID      int64
	rootID         int64
	otherLibraryID int64   // library X (AddEdgeCases)
	loose          []Movie // titles outside All that the oracle looks up (AddLoneExtraSeries)
}

// LibraryID is the id of library 1 (the movies and the series).
func (f *Fixture) LibraryID() int64 { return f.libraryID }

// RootID is the id of the root of library 1.
func (f *Fixture) RootID() int64 { return f.rootID }

// All returns every fixture title: the movies with the edge cases, the extras of the film rated 6,
// the series tree and the audio titles, so a leak of any of them is detected.
func (f *Fixture) All() []Movie {
	out := slices.Concat(f.Movies, f.Extras)
	if f.Show.Series.ID != "" {
		out = append(out, f.Show.All()...)
	}
	return append(out, f.Audio.Titles()...)
}

// Playable returns the titles that are no containers: with a media file (movies, episodes, extras,
// audiobooks, podcast episodes) or, for the enclosure-only podcast episode of AddAudio, an enclosure.
func (f *Fixture) Playable() []Movie {
	var out []Movie
	for _, m := range f.All() {
		if !m.Container {
			out = append(out, m)
		}
	}
	return out
}

// OtherLibraryID is the id of library X (AddEdgeCases).
func (f *Fixture) OtherLibraryID() int64 { return f.otherLibraryID }

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

// VisibleTo reports whether a profile of the owner account may see the movie m by the rules of
// plan 3.3 (unknown counts as 18; missing and deleted titles are invisible; the library of the
// title must be in the profile's rights, as PX only has library 1; a container is visible when its
// own rating and its stored MinChild fit). Series trees, extras and profiles of the guest account
// need Fixture.VisibleTo.
func VisibleTo(p model.Profile, m Movie) bool {
	return visibleTo(p, 18, true, nil, m, m.minChild())
}

// VisibleTo reports whether the profile may see m, with the limits of its account applied: the
// ceiling of the guest account G and its library rights (library 1 only) cap the profile. The
// container rule is derived from the fixture tree (migration 00010): a container's smallest child
// rating is the minimum over its live content children, a leaf counting with its rating and a
// container child with the stricter of its rating and its own smallest child rating. An extra is
// visible only while its parent is.
func (f *Fixture) VisibleTo(p model.Profile, m Movie) bool {
	if f.Guest.ID != 0 && p.AccountID == f.Guest.ID {
		return f.visible(p, f.Guest.MaxRating, f.Guest.AllLibraries, f.Guest.LibraryIDs, m, 0)
	}
	return f.visible(p, 18, true, nil, m, 0)
}

func (f *Fixture) visible(p model.Profile, accountMax int, accountAll bool, accountLibs []int64, m Movie, depth int) bool {
	if !visibleTo(p, accountMax, accountAll, accountLibs, m, f.minChild(m, depth)) {
		return false
	}
	if m.Kind != "extra" {
		return true
	}
	parent, ok := f.lookup(m.ParentID)
	return ok && depth < 4 && f.visible(p, accountMax, accountAll, accountLibs, parent, depth+1)
}

// lookup finds a fixture title by id, including the titles that are not part of All.
func (f *Fixture) lookup(id string) (Movie, bool) {
	for _, m := range slices.Concat(f.All(), f.loose) {
		if m.ID == id {
			return m, true
		}
	}
	return Movie{}, false
}

// minChild derives the smallest child rating of a container from the tree (18 without content
// children); a title without children in the fixture keeps its stored MinChild.
func (f *Fixture) minChild(m Movie, depth int) int {
	if !m.Container {
		return m.minChild()
	}
	out, found := 18, false
	for _, c := range slices.Concat(f.All(), f.loose) {
		if c.ParentID != m.ID || c.ID == "" || c.Kind == "extra" || c.Missing || c.Deleted || c.Unavailable {
			continue
		}
		found = true
		v := c.rating()
		if c.Container && depth < 4 {
			v = max(v, f.minChild(c, depth+1))
		}
		out = min(out, v)
	}
	if !found {
		return m.minChild()
	}
	return out
}

func visibleTo(p model.Profile, accountMax int, accountAll bool, accountLibs []int64, m Movie, minChild int) bool {
	if m.Missing || m.Deleted || m.Unavailable {
		return false
	}
	// Library rights: the profile's own and the account's, both read from the fixture data.
	if !p.AllLibraries && !slices.Contains(p.LibraryIDs, m.LibraryID) {
		return false
	}
	if !accountAll && !slices.Contains(accountLibs, m.LibraryID) {
		return false
	}
	limit := min(p.MaxRating, accountMax)
	if m.rating() > limit {
		return false
	}
	if m.Container && minChild > limit {
		return false
	}
	return true
}

// rating is the effective rating of the title, unknown counting as 18.
func (m Movie) rating() int {
	r := m.Rating
	if m.Kind != "" {
		r = m.Effective
	}
	if r == nil {
		return 18
	}
	return *r
}

func (m Movie) minChild() int {
	if m.MinChild == nil {
		return 18
	}
	return *m.MinChild
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
	f.Account = model.Account{ID: acc.ID, Email: acc.Email, Role: acc.Role, MaxRating: 18, AllLibraries: true, LibraryIDs: []int64{}}
	add := func(name string, maxRating int, lang string, isDefault int64) model.Profile {
		p, err := q.InsertProfile(ctx, appq.InsertProfileParams{AccountID: acc.ID, Name: name, MaxRating: int64(maxRating), Language: lang, AllLibraries: 1, IsDefault: isDefault, Now: 1})
		if err != nil {
			t.Fatal(err)
		}
		return model.Profile{ID: p.ID, AccountID: acc.ID, Name: name, MaxRating: maxRating, Language: lang, AllLibraries: true, LibraryIDs: []int64{}, IsPrimary: isDefault == 1}
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
	f.PX = model.Profile{ID: p.ID, AccountID: f.Account.ID, Name: "PX", MaxRating: 18, Language: "de", LibraryIDs: []int64{f.libraryID}}
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
	f.otherLibraryID = lib.ID
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
	m.LibraryID = libraryID
	if m.RelPath == "" {
		m.RelPath = fmt.Sprintf("Movie %s (20%02d)/Movie %s (20%02d).mp4", m.Label, 10+i, m.Label, 10+i)
		if m.Kind == "episode" {
			m.RelPath = fmt.Sprintf("Show/Season 01/Show - S01%s.mp4", m.Label)
		}
	}
	if m.Kind == "" {
		m.Effective = m.Rating
	}
	if !m.Container {
		m.Content = make([]byte, FileSize)
		if _, err := rand.Read(m.Content); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(mediaDir, filepath.FromSlash(m.RelPath)), m.Content)
	}
	kind := "movie"
	if m.Kind != "" {
		kind = m.Kind
	}
	exec(t, f.DB, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, original_title, year, release_date,
	               runtime_minutes, genres, tmdb_id, own_rating, metadata_state, added_at, created_at, updated_at)
	               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 120, ?, ?, ?, 'matched', ?, 1, 1)`,
		m.ID, kind, sql.NullString{String: m.ParentID, Valid: m.ParentID != ""}, libraryID, kind+" "+m.Label, kind+" "+m.Label,
		"Original "+m.Label, 2010+i, fmt.Sprintf("20%02d-05-01", 10+i),
		`{"de":["Abenteuer"],"en":["Adventure"]}`, 1000+i, m.Rating, int64(1000*(i+1)))
	if !m.Container {
		exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec,
		               audio_codec, width, height, duration_ms, created_at, updated_at)
		               VALUES (?, ?, ?, ?, ?, 1, 'mp4', 'h264', 'aac', 1920, 1080, ?, 1, 1)`,
			m.FileID, m.ID, rootID, m.RelPath, FileSize, DurationMs)
	}
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

// AddSeries adds the series tree to library 1: the series S rated 12, season 1 (no own rating)
// with the episodes E1 (no own rating, effective 12), E2 (own rating 16) and the multi-episode file
// E3 (S01E03-E04), the specials season 0 with S00E01, and a behind-the-scenes extra of the series;
// every episode and extra with a media file, texts and images like the movies, series and seasons
// with texts and images too. The container ratings follow plan 3.2 (effective 12 for the tree
// except E2 with 16; the smallest content child of series and seasons is 12; extras never count).
//
// It also adds the M3 data around the movies: a trailer and a featurette extra of the film rated 6,
// a second version (1280x720) of the film rated 12, trailers in de and en of the film rated 6 and
// of the series plus one of the film rated 18, the person "Pat Example" with a cast credit in the
// film rated 6 and the director credit of the film rated 18, and an ignored file of library 1.
func (f *Fixture) AddSeries(t testing.TB) {
	t.Helper()
	twelve, sixteen := ptr(12), ptr(16)
	series := f.addMovie(t, Movie{Kind: "series", Label: "SS", Rating: twelve, Container: true, Effective: twelve, MinChild: twelve},
		f.libraryID, f.rootID, f.MediaDir, 20)
	season := f.addMovie(t, Movie{Kind: "season", Label: "SN", ParentID: series.ID, Container: true, Effective: twelve, MinChild: twelve},
		f.libraryID, f.rootID, f.MediaDir, 21)
	e1 := f.addMovie(t, Movie{Kind: "episode", Label: "E1", ParentID: season.ID, Effective: twelve},
		f.libraryID, f.rootID, f.MediaDir, 22)
	e2 := f.addMovie(t, Movie{Kind: "episode", Label: "E2", ParentID: season.ID, Rating: sixteen, Effective: sixteen},
		f.libraryID, f.rootID, f.MediaDir, 23)
	e3 := f.addMovie(t, Movie{Kind: "episode", Label: "E3", ParentID: season.ID, Effective: twelve, RelPath: "Show/Season 01/Show - S01E03-E04.mp4"},
		f.libraryID, f.rootID, f.MediaDir, 24)
	specials := f.addMovie(t, Movie{Kind: "season", Label: "SP", ParentID: series.ID, Container: true, Effective: twelve, MinChild: twelve},
		f.libraryID, f.rootID, f.MediaDir, 25)
	special := f.addMovie(t, Movie{Kind: "episode", Label: "P1", ParentID: specials.ID, Effective: twelve, RelPath: "Show/Specials/Show - S00E01.mp4"},
		f.libraryID, f.rootID, f.MediaDir, 26)
	extra := f.addMovie(t, Movie{Kind: "extra", Label: "XB", ParentID: series.ID, Effective: twelve, RelPath: "Show/Behind The Scenes/Making of.mp4"},
		f.libraryID, f.rootID, f.MediaDir, 27)
	f.Show = Show{Series: series, Season: season, E1: e1, E2: e2, E3: e3, Specials: specials, Special: special, Extra: extra}
	exec(t, f.DB, `UPDATE items SET season_number = 1 WHERE id IN (?, ?, ?, ?)`, season.ID, e1.ID, e2.ID, e3.ID)
	exec(t, f.DB, `UPDATE items SET season_number = 0 WHERE id IN (?, ?)`, specials.ID, special.ID)
	exec(t, f.DB, `UPDATE items SET episode_number = 1 WHERE id IN (?, ?)`, e1.ID, special.ID)
	exec(t, f.DB, `UPDATE items SET episode_number = 2 WHERE id = ?`, e2.ID)
	exec(t, f.DB, `UPDATE items SET episode_number = 3, episode_end = 4 WHERE id = ?`, e3.ID)
	exec(t, f.DB, `UPDATE items SET extra_kind = 'behind_the_scenes', metadata_state = 'skipped' WHERE id = ?`, extra.ID)

	// Extras of the film rated 6: they inherit its rating (effective 6).
	m6, six := f.Rated(6), ptr(6)
	for i, x := range []struct{ label, kind, rel string }{
		{"XT", "trailer", "Movie 06 (2011)/Trailers/Teaser.mp4"},
		{"XF", "featurette", "Movie 06 (2011)/Featurettes/On set.mp4"},
	} {
		m := f.addMovie(t, Movie{Kind: "extra", Label: x.label, ParentID: m6.ID, Effective: six, RelPath: x.rel}, f.libraryID, f.rootID, f.MediaDir, 28+i)
		exec(t, f.DB, `UPDATE items SET extra_kind = ?, metadata_state = 'skipped' WHERE id = ?`, x.kind, m.ID)
		f.Extras = append(f.Extras, m)
	}

	// Second version of the film rated 12 (B-5): 720p by its probed height, so the 1080p file
	// stays the primary one.
	m12 := f.Rated(12)
	f.Version2FileID = "01J9000000000000000000F0V2"
	rel := "Movie 12 (2012)/Movie 12 (2012) - 720p.mp4"
	content := make([]byte, FileSize)
	if _, err := rand.Read(content); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.MediaDir, filepath.FromSlash(rel)), content)
	exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec,
	               audio_codec, width, height, duration_ms, probed_at, created_at, updated_at)
	               VALUES (?, ?, ?, ?, ?, 1, 'mp4', 'h264', 'aac', 1280, 720, ?, 2, 2, 2)`,
		f.Version2FileID, m12.ID, f.rootID, rel, FileSize, DurationMs)

	// Trailers (B-8): de and en of the film rated 6 and of the series, one of the film rated 18.
	f.Trailers = map[string]int{}
	for _, tr := range []struct {
		itemID, lang, youtubeID, source string
		official                        int
	}{
		{m6.ID, "en", "EnTrailer06", "tmdb", 1},
		{m6.ID, "de", "DeTrailer06", "youtube", 0},
		{series.ID, "de", "DeTrailerSS", "manual", 0},
		{series.ID, "en", "EnTrailerSS", "tmdb", 1},
		{f.Rated(18).ID, "en", "EnTrailer18", "tmdb", 1},
	} {
		exec(t, f.DB, `INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at) VALUES (?, ?, ?, ?, ?, ?, 1)`,
			tr.itemID, tr.lang, tr.youtubeID, "Trailer "+tr.lang, tr.source, tr.official)
		f.Trailers[tr.itemID]++
	}
	exec(t, f.DB, `UPDATE items SET trailer_state = 'found' WHERE id IN (?, ?, ?)`, m6.ID, series.ID, f.Rated(18).ID)

	// A person with roles in the films rated 6 (cast) and 18 (director).
	f.Person = Person{ID: 1, Name: "Pat Example"}
	exec(t, f.DB, `INSERT INTO people (id, tmdb_id, name, created_at) VALUES (?, 9001, ?, 1)`, f.Person.ID, f.Person.Name)
	exec(t, f.DB, `INSERT INTO credits (item_id, person_id, role, character, ord) VALUES (?, ?, 'cast', 'Hero', 0), (?, ?, 'director', NULL, 0)`,
		m6.ID, f.Person.ID, f.Rated(18).ID, f.Person.ID)

	// A file the administrator removed (B-9); the scanner skips it.
	f.IgnoredRelPath = "Broken (2024)/Broken (2024).mkv"
	exec(t, f.DB, `INSERT INTO ignored_files (root_id, rel_path, reason, created_at) VALUES (?, ?, 'unplayable', 1)`, f.rootID, f.IgnoredRelPath)
}

// AddGuest adds the guest account G (FSK ceiling 12, only library 1) with its primary profile PG
// (max 12, language en) and the profile PG18 (max 18, language de), which the account caps to 12.
func (f *Fixture) AddGuest(t testing.TB) {
	t.Helper()
	ctx := context.Background()
	q := appq.New(f.DB.Write)
	acc, err := q.InsertGuestAccount(ctx, appq.InsertGuestAccountParams{Email: "guest@example.test", PasswordHash: "x", MaxRating: 12, AllLibraries: 0, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.InsertAccountLibrary(ctx, appq.InsertAccountLibraryParams{AccountID: acc.ID, LibraryID: f.libraryID}); err != nil {
		t.Fatal(err)
	}
	f.Guest = model.Account{ID: acc.ID, Email: acc.Email, Role: "guest", MaxRating: 12, LibraryIDs: []int64{f.libraryID}}
	add := func(name string, maxRating int, lang string, isDefault int64) model.Profile {
		p, err := q.InsertProfile(ctx, appq.InsertProfileParams{AccountID: acc.ID, Name: name, MaxRating: int64(maxRating), Language: lang, AllLibraries: 1, IsDefault: isDefault, Now: 1})
		if err != nil {
			t.Fatal(err)
		}
		return model.Profile{ID: p.ID, AccountID: acc.ID, Name: name, MaxRating: maxRating, Language: lang, AllLibraries: true, LibraryIDs: []int64{}, IsPrimary: isDefault == 1}
	}
	f.PG = add("PG", 12, "en", 1)
	f.PG18 = add("PG18", 18, "de", 0)
}
