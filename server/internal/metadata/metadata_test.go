package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const (
	goodToken = "test-read-token"
	rateToken = "rate-limited-token"

	idByTMDB   = "01JTESTMOVIE0000000000000A"
	idByIMDB   = "01JTESTMOVIE0000000000000B"
	idBySearch = "01JTESTMOVIE0000000000000C"
	idNoMatch  = "01JTESTMOVIE0000000000000D"
	idNoRating = "01JTESTMOVIE0000000000000E"
	idLocked   = "01JTESTMOVIE0000000000000F"
	idBroken   = "01JTESTMOVIE0000000000000G"
)

var testNow = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

// fakeTMDB serves the TMDB endpoints the package uses, from fixtures.
type fakeTMDB struct {
	srv    *httptest.Server
	poster []byte // TMDB original of the poster
	backdr []byte // TMDB original of the backdrop

	mu        sync.Mutex
	hits      map[string]int
	rateCalls int
	authSeen  []string
}

// jpegBytes returns a w×h JPEG: a strong red block in the upper half (the poster's accent) over a
// greyish gradient of low saturation.
func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			c := color.RGBA{R: uint8(90 + x%40), G: uint8(90 + y%40), B: 110, A: 255}
			if y < h/2 {
				c = color.RGBA{R: 200, G: 30, B: 40, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type fixture struct {
	de, en map[string]any
}

func movieFixture(id int64, imdb, title, deTitle, deOverview string, genresDE, genresEN []string, releases []any, poster, backdrop string) fixture {
	gen := func(names []string) []map[string]any {
		out := make([]map[string]any, 0, len(names))
		for i, n := range names {
			out = append(out, map[string]any{"id": i + 1, "name": n})
		}
		return out
	}
	base := func(lang string) map[string]any {
		m := map[string]any{
			"id": id, "imdb_id": imdb, "original_title": title + " (Original)", "release_date": "2001-03-04",
			"runtime": 117, "poster_path": poster, "backdrop_path": backdrop,
			"release_dates": map[string]any{"results": releases},
		}
		if lang == langDE {
			m["title"], m["overview"], m["tagline"], m["genres"] = deTitle, deOverview, "", gen(genresDE)
		} else {
			m["title"], m["overview"], m["tagline"], m["genres"] = title, "English overview of "+title, "English tagline", gen(genresEN)
		}
		return m
	}
	return fixture{de: base(langDE), en: base(langEN)}
}

func countryJSON(code, cert string) map[string]any {
	return map[string]any{"iso_3166_1": code, "release_dates": []any{map[string]any{"certification": cert, "type": 3}}}
}

func newFakeTMDB(t *testing.T) *fakeTMDB {
	t.Helper()
	f := &fakeTMDB{hits: map[string]int{}, poster: jpegBytes(t, 300, 450), backdr: jpegBytes(t, 640, 360)}
	movies := map[string]fixture{
		// DE FSK 12 next to US R: DE wins.
		"550": movieFixture(550, "tt0000550", "First Film", "Erster Film", "Deutsche Beschreibung",
			[]string{"Drama", "Komödie"}, []string{"Drama", "Comedy"},
			[]any{countryJSON("US", "R"), countryJSON("DE", "12")}, "/poster550.jpg", "/backdrop550.jpg"),
		// Only US PG-13, no German overview.
		"601": movieFixture(601, "tt0000601", "Searched Film", "", "",
			[]string{"Action"}, []string{"Action"},
			[]any{countryJSON("US", "PG-13")}, "/poster601.jpg", ""),
		// No certification at all, no images.
		"602": movieFixture(602, "", "Unrated Film", "Film ohne Freigabe", "Beschreibung",
			nil, nil, []any{}, "", ""),
	}
	mux := http.NewServeMux()
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.hits[r.URL.Path]++
			f.authSeen = append(f.authSeen, r.Header.Get("Authorization"))
			f.mu.Unlock()
			if strings.Contains(r.URL.RawQuery, goodToken) || strings.Contains(r.URL.RawQuery, rateToken) {
				t.Errorf("token leaked into the URL %s", r.URL)
			}
			switch r.Header.Get("Authorization") {
			case "Bearer " + goodToken:
			case "Bearer " + rateToken:
				f.mu.Lock()
				f.rateCalls++
				first := f.rateCalls == 1
				f.mu.Unlock()
				if first {
					w.Header().Set("Retry-After", "1")
					http.Error(w, `{"status_code":25}`, http.StatusTooManyRequests)
					return
				}
			default:
				http.Error(w, `{"status_code":7}`, http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("GET /3/configuration", auth(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"images": map[string]any{"secure_base_url": "https://image.tmdb.org/t/p/"}})
	}))
	mux.HandleFunc("GET /3/movie/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "503" {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		fx, ok := movies[r.PathValue("id")]
		if !ok {
			http.Error(w, `{"status_code":34}`, http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("append_to_response") != "release_dates" {
			t.Errorf("movie request without release_dates: %s", r.URL)
		}
		switch r.URL.Query().Get("language") {
		case langDE:
			writeJSON(w, fx.de)
		case langEN:
			writeJSON(w, fx.en)
		default:
			t.Errorf("unexpected language in %s", r.URL)
			http.Error(w, "bad language", http.StatusBadRequest)
		}
	}))
	mux.HandleFunc("GET /3/find/{imdb}", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("external_source") != "imdb_id" {
			t.Errorf("find without external_source: %s", r.URL)
		}
		results := []any{}
		if r.PathValue("imdb") == "tt0000550" {
			results = append(results, map[string]any{"id": 550})
		}
		writeJSON(w, map[string]any{"movie_results": results})
	}))
	mux.HandleFunc("GET /3/search/movie", auth(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		results := []any{}
		if q.Get("query") == "Searched Film" && q.Get("language") == langEN {
			results = append(results,
				map[string]any{"id": 999, "release_date": "1990-01-01"},
				map[string]any{"id": 601, "release_date": "2002-05-01"})
		}
		writeJSON(w, map[string]any{"results": results})
	}))
	image := func(data []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.hits[r.URL.Path]++
			f.mu.Unlock()
			if r.Header.Get("Authorization") != "" {
				t.Errorf("token sent to the image host")
			}
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(data)
		}
	}
	mux.HandleFunc("GET /t/p/original/poster550.jpg", image(f.poster))
	mux.HandleFunc("GET /t/p/original/backdrop550.jpg", image(f.backdr))
	mux.HandleFunc("GET /t/p/original/poster601.jpg", image(f.poster))
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTMDB) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func (f *fakeTMDB) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, v := range f.hits {
		n += v
	}
	return n
}

type staticKey string

func (k staticKey) TMDBKey(context.Context) (string, error) { return string(k), nil }

type env struct {
	db    *store.DB
	queue *jobs.Queue
	svc   *Service
	tmdb  *fakeTMDB
	cache string
	logs  *bytes.Buffer
}

func newEnv(t *testing.T, key string) *env {
	t.Helper()
	logs := &bytes.Buffer{}
	log := slog.New(logging.NewRedactingHandler(slog.NewTextHandler(&lockedWriter{w: logs}, nil)))
	db, err := store.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f := newFakeTMDB(t)
	cache := t.TempDir()
	q := jobs.NewQueue(db.Write, nil)
	svc := New(db, q, staticKey(key), cache, transcode.Runner{FFmpegPath: lookFFmpeg()}, log,
		WithBaseURL(f.srv.URL+"/3", f.srv.URL+"/t/p"), WithHTTPClient(f.srv.Client()),
		WithClock(func() time.Time { return testNow }))
	mustExec(t, db.Write, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'Filme', 'movies', 0, 0)`)
	t.Cleanup(func() {
		if strings.Contains(logs.String(), goodToken) || strings.Contains(logs.String(), rateToken) {
			t.Errorf("token in logs:\n%s", logs.String())
		}
	})
	return &env{db: db, queue: q, svc: svc, tmdb: f, cache: cache, logs: logs}
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

type itemSeed struct {
	id, title    string
	year, tmdbID any
	imdbID       any
	ownRating    any
	ratingSource any
	locked       string
}

func (e *env) addItem(t *testing.T, s itemSeed) {
	t.Helper()
	if s.locked == "" {
		s.locked = "[]"
	}
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, year, tmdb_id, imdb_id, own_rating,
	                         rating_source, locked_fields, added_at, created_at, updated_at)
	                         VALUES (?, 'movie', 1, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0)`,
		s.id, s.title, strings.ToLower(s.title), s.year, s.tmdbID, s.imdbID, s.ownRating, s.ratingSource, s.locked)
}

type itemRow struct {
	State, Genres, Locked                         string
	Err, OriginalTitle, ReleaseDate, IMDB, Source sql.NullString
	Year, Runtime, TMDB, Rating, MatchedAt        sql.NullInt64
}

func (e *env) item(t *testing.T, id string) itemRow {
	t.Helper()
	var r itemRow
	err := e.db.Read.QueryRow(`SELECT metadata_state, genres, locked_fields, metadata_error, original_title, release_date,
	                                  imdb_id, rating_source, year, runtime_minutes, tmdb_id, own_rating, matched_at
	                           FROM items WHERE id = ?`, id).
		Scan(&r.State, &r.Genres, &r.Locked, &r.Err, &r.OriginalTitle, &r.ReleaseDate, &r.IMDB, &r.Source,
			&r.Year, &r.Runtime, &r.TMDB, &r.Rating, &r.MatchedAt)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type textRow struct {
	Title             string
	Overview, Tagline sql.NullString
}

func (e *env) texts(t *testing.T, id string) map[string]textRow {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT lang, title, overview, tagline FROM item_texts WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]textRow{}
	for rows.Next() {
		var lang string
		var r textRow
		if err := rows.Scan(&lang, &r.Title, &r.Overview, &r.Tagline); err != nil {
			t.Fatal(err)
		}
		out[lang] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *env) imageJobs(t *testing.T) []string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT idempotency_key || ' p' || priority || ' ' || payload FROM jobs WHERE type = 'image.fetch' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCheckKey(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	if err := e.svc.CheckKey(ctx, goodToken); err != nil {
		t.Fatalf("valid key: %v", err)
	}
	if err := e.svc.CheckKey(ctx, "wrong"); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Fatalf("invalid key: %v, want ErrTMDBKeyInvalid", err)
	}
	if err := e.svc.CheckKey(ctx, ""); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Fatalf("empty key: %v, want ErrTMDBKeyInvalid", err)
	}
	e.tmdb.mu.Lock()
	seen := strings.Join(e.tmdb.authSeen, ",")
	e.tmdb.mu.Unlock()
	if !strings.Contains(seen, "Bearer "+goodToken) {
		t.Fatalf("Authorization header not sent as bearer token: %q", seen)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "maintenance", http.StatusServiceUnavailable)
	}))
	defer down.Close()
	c := NewClient(down.URL, down.URL, down.Client())
	if err := c.CheckKey(ctx, goodToken); !errors.Is(err, model.ErrTMDBUnreachable) {
		t.Fatalf("5xx: %v, want ErrTMDBUnreachable", err)
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()
	c = NewClient(closedURL, closedURL, &http.Client{Timeout: time.Second})
	if err := c.CheckKey(ctx, goodToken); !errors.Is(err, model.ErrTMDBUnreachable) {
		t.Fatalf("network error: %v, want ErrTMDBUnreachable", err)
	}
}

func TestCheckKeyRetriesAfter429(t *testing.T) {
	e := newEnv(t, goodToken)
	start := time.Now()
	if err := e.svc.CheckKey(context.Background(), rateToken); err != nil {
		t.Fatalf("CheckKey after 429: %v", err)
	}
	e.tmdb.mu.Lock()
	calls := e.tmdb.rateCalls
	e.tmdb.mu.Unlock()
	if calls != 2 {
		t.Fatalf("%d calls, want 2 (429, then 200)", calls)
	}
	if d := time.Since(start); d < 900*time.Millisecond {
		t.Fatalf("retry after %v, want the Retry-After of 1s", d)
	}
}

func TestMatchByTMDBID(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByTMDB, title: "first film", year: 2001, tmdbID: 550})
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	r := e.item(t, idByTMDB)
	if r.State != "matched" || r.Err.Valid || r.MatchedAt.Int64 != testNow.UnixMilli() {
		t.Fatalf("state %s err %v matched_at %v", r.State, r.Err, r.MatchedAt)
	}
	if r.TMDB.Int64 != 550 || r.IMDB.String != "tt0000550" || r.Year.Int64 != 2001 ||
		r.ReleaseDate.String != "2001-03-04" || r.Runtime.Int64 != 117 || r.OriginalTitle.String != "First Film (Original)" {
		t.Fatalf("item columns: %+v", r)
	}
	if !r.Rating.Valid || r.Rating.Int64 != 12 || r.Source.String != "tmdb:DE" {
		t.Fatalf("rating %v source %v, want 12 tmdb:DE", r.Rating, r.Source)
	}
	var g map[string][]string
	if err := json.Unmarshal([]byte(r.Genres), &g); err != nil {
		t.Fatal(err)
	}
	if strings.Join(g["de"], ",") != "Drama,Komödie" || strings.Join(g["en"], ",") != "Drama,Comedy" || len(g) != 2 {
		t.Fatalf("genres %s", r.Genres)
	}
	tx := e.texts(t, idByTMDB)
	if tx["de"].Title != "Erster Film" || tx["de"].Overview.String != "Deutsche Beschreibung" ||
		tx["de"].Tagline.String != "English tagline" {
		t.Fatalf("de texts %+v", tx["de"])
	}
	if tx["en"].Title != "First Film" || tx["en"].Overview.String != "English overview of First Film" {
		t.Fatalf("en texts %+v", tx["en"])
	}
	jobs := e.imageJobs(t)
	if len(jobs) != 1 || jobs[0] != `image:`+idByTMDB+` p2 {"itemId":"`+idByTMDB+`"}` {
		t.Fatalf("image jobs %q", jobs)
	}
	// Running the job again changes nothing and does not duplicate the active image job.
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	if n := len(e.imageJobs(t)); n != 1 {
		t.Fatalf("%d image jobs after a second match, want 1", n)
	}
}

func TestMatchByIMDB(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByIMDB, title: "Whatever", imdbID: "tt0000550"})
	if err := e.svc.match(context.Background(), idByIMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByIMDB); r.State != "matched" || r.TMDB.Int64 != 550 {
		t.Fatalf("state %s tmdb %v", r.State, r.TMDB)
	}
	if e.tmdb.count("/3/find/tt0000550") != 1 || e.tmdb.count("/3/search/movie") != 0 {
		t.Fatal("expected exactly one find and no search")
	}
}

func TestMatchBySearchWithEnglishFallback(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2001})
	if err := e.svc.match(context.Background(), idBySearch); err != nil {
		t.Fatal(err)
	}
	r := e.item(t, idBySearch)
	if r.State != "matched" || r.TMDB.Int64 != 601 {
		t.Fatalf("state %s tmdb %v, want matched 601 (first result within ±1 year)", r.State, r.TMDB)
	}
	if r.Rating.Int64 != 12 || r.Source.String != "tmdb:US" {
		t.Fatalf("rating %v %v, want 12 tmdb:US", r.Rating, r.Source)
	}
	de := e.texts(t, idBySearch)["de"]
	if de.Title != "Searched Film" || de.Overview.String != "English overview of Searched Film" {
		t.Fatalf("de texts did not fall back to English: %+v", de)
	}
}

func TestMatchUnmatched(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idNoMatch, title: "Nothing", year: 1999})
	e.addItem(t, itemSeed{id: idBroken, title: "Gone", tmdbID: 123456})
	for _, id := range []string{idNoMatch, idBroken} {
		if err := e.svc.match(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		r := e.item(t, id)
		if r.State != "unmatched" || r.MatchedAt.Int64 != testNow.UnixMilli() {
			t.Fatalf("%s: state %s matched_at %v", id, r.State, r.MatchedAt)
		}
	}
	if len(e.imageJobs(t)) != 0 {
		t.Fatal("image job enqueued for an unmatched item")
	}
}

func TestMatchWithoutCertification(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idNoRating, title: "Unrated", tmdbID: 602, ownRating: 6, ratingSource: "tmdb:DE"})
	if err := e.svc.match(context.Background(), idNoRating); err != nil {
		t.Fatal(err)
	}
	r := e.item(t, idNoRating)
	if r.State != "matched" || r.Rating.Valid || r.Source.Valid {
		t.Fatalf("state %s rating %v source %v, want matched with unknown rating", r.State, r.Rating, r.Source)
	}
	if r.IMDB.Valid {
		t.Fatalf("imdb_id %v, want NULL when TMDB has none", r.IMDB)
	}
}

func TestMatchRespectsLockedFieldsAndNFORating(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idLocked, title: "My Title", year: 1999, tmdbID: 550, ownRating: 16, ratingSource: "nfo",
		locked: `["year","genres","title","imdb_id"]`})
	if err := e.svc.match(context.Background(), idLocked); err != nil {
		t.Fatal(err)
	}
	r := e.item(t, idLocked)
	if r.State != "matched" || r.Year.Int64 != 1999 || r.Genres != "[]" || r.IMDB.Valid {
		t.Fatalf("locked fields overwritten: %+v", r)
	}
	if r.Rating.Int64 != 16 || r.Source.String != "nfo" {
		t.Fatalf("nfo rating overwritten: %v %v", r.Rating, r.Source)
	}
	if r.Runtime.Int64 != 117 {
		t.Fatalf("unlocked runtime not written: %v", r.Runtime)
	}
	for lang, tx := range e.texts(t, idLocked) {
		if tx.Title != "My Title" || !tx.Overview.Valid {
			t.Fatalf("%s texts %+v, want locked title and TMDB overview", lang, tx)
		}
	}

	// A locked own_rating survives as well, also when its source is TMDB.
	mustExec(t, e.db.Write, `UPDATE items SET rating_source = 'tmdb:US', own_rating = 18, locked_fields = '["own_rating"]' WHERE id = ?`, idLocked)
	if err := e.svc.match(context.Background(), idLocked); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idLocked); r.Rating.Int64 != 18 || r.Source.String != "tmdb:US" {
		t.Fatalf("locked own_rating overwritten: %v %v", r.Rating, r.Source)
	}

	// The administrator's override (admin.SetItemRating, M2) is source "manual" with the rating
	// field locked: either mark alone keeps it.
	for name, set := range map[string]string{
		"manual source": `rating_source = 'manual', locked_fields = '[]'`,
		"locked field":  `rating_source = NULL, locked_fields = '["own_rating"]'`,
	} {
		mustExec(t, e.db.Write, `UPDATE items SET own_rating = 6, `+set+` WHERE id = ?`, idLocked)
		if err := e.svc.match(context.Background(), idLocked); err != nil {
			t.Fatal(err)
		}
		if r := e.item(t, idLocked); r.Rating.Int64 != 6 {
			t.Fatalf("%s: override overwritten by the match: %v %v", name, r.Rating, r.Source)
		}
	}
}

func TestMatchWithoutKeySkipsTMDB(t *testing.T) {
	e := newEnv(t, "")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.State != "skipped" || r.Err.String != "no_tmdb_key" {
		t.Fatalf("state %s err %v", r.State, r.Err)
	}
	if n := e.tmdb.total(); n != 0 {
		t.Fatalf("%d TMDB requests without a key", n)
	}
}

func TestMatchInvalidKeyFails(t *testing.T) {
	e := newEnv(t, "revoked")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatalf("invalid key must not be retried, got %v", err)
	}
	if r := e.item(t, idByTMDB); r.State != "failed" || r.Err.String != "tmdb_key_invalid" {
		t.Fatalf("state %s err %v", r.State, r.Err)
	}
}

func TestMatchTransientErrorIsRetried(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idBroken, title: "Down", tmdbID: 503})
	err := e.svc.match(context.Background(), idBroken)
	if !errors.Is(err, model.ErrTMDBUnreachable) {
		t.Fatalf("got %v, want ErrTMDBUnreachable for the queue to retry", err)
	}
	if r := e.item(t, idBroken); r.State != "pending" {
		t.Fatalf("state %s, want pending", r.State)
	}
}

func TestMatchIgnoresDeletedAndUnknownItems(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	mustExec(t, e.db.Write, `UPDATE items SET deleted_at = 1 WHERE id = ?`, idByTMDB)
	for _, id := range []string{idByTMDB, idNoMatch} {
		if err := e.svc.match(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.tmdb.total(); n != 0 {
		t.Fatalf("%d TMDB requests for deleted or unknown items", n)
	}
}

// lookFFmpeg returns the ffmpeg on PATH or "" (package metadata may not import os/exec; the
// runner resolves the name itself, this only decides whether the image tests can run).
func lookFFmpeg() string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, "ffmpeg")
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return path
		}
	}
	return ""
}

// requireFFmpeg skips image pipeline tests on machines without ffmpeg.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if lookFFmpeg() == "" {
		t.Skip("ffmpeg not installed")
	}
}

type imageRow struct {
	Kind, Size, Format, RelPath, Hash, MediaType, Source string
	Accent                                               sql.NullString
	Width, Height, Bytes                                 int64
}

// images returns the image rows of a title keyed "<kind>/<size>/<format>".
func (e *env) images(t *testing.T, id string) map[string]imageRow {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT kind, size, format, rel_path, content_hash, media_type, source_url, accent, width, height, bytes
	                              FROM images WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]imageRow{}
	for rows.Next() {
		var r imageRow
		if err := rows.Scan(&r.Kind, &r.Size, &r.Format, &r.RelPath, &r.Hash, &r.MediaType, &r.Source, &r.Accent, &r.Width, &r.Height, &r.Bytes); err != nil {
			t.Fatal(err)
		}
		out[r.Kind+"/"+r.Size+"/"+r.Format] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// scaledHeight is the height ffmpeg's scale=W:-2 gives a w×h input: rounded, then even.
func scaledHeight(width, w, h int) int64 {
	return int64((width*h+w)/(2*w)) * 2
}

// checkPipeline verifies the rows and files of one title after image.fetch: the original and six
// derived rows per kind in kinds, with matching hashes, sizes, dimensions and formats, the accent
// on every poster row, and no temporary files.
func (e *env) checkPipeline(t *testing.T, id string, kinds ...string) map[string]imageRow {
	t.Helper()
	imgs := e.images(t, id)
	originals := map[string]struct {
		data []byte
		path string
	}{"poster": {e.tmdb.poster, "/poster550.jpg"}, "backdrop": {e.tmdb.backdr, "/backdrop550.jpg"}}
	if id == idBySearch {
		originals["poster"] = struct {
			data []byte
			path string
		}{e.tmdb.poster, "/poster601.jpg"}
	}
	want := 0
	accent := ""
	for _, kind := range kinds {
		o := originals[kind]
		cfg, _, err := image.DecodeConfig(bytes.NewReader(o.data))
		if err != nil {
			t.Fatal(err)
		}
		source := e.tmdb.srv.URL + "/t/p/original" + o.path
		orig := imgs[kind+"/original/jpeg"]
		if orig.RelPath != "images/"+id+"/"+kind+"-original.jpg" || orig.Hash != sha(o.data) || orig.Source != source ||
			orig.MediaType != "image/jpeg" || orig.Width != int64(cfg.Width) || orig.Height != int64(cfg.Height) || orig.Accent.Valid {
			t.Fatalf("%s original row %+v", kind, orig)
		}
		want++
		for _, w := range imageWidths[kind] {
			for _, f := range imageFormats {
				key := kind + "/" + sizeName(w) + "/" + string(f.format)
				r, ok := imgs[key]
				if !ok {
					t.Fatalf("no row %s in %v", key, imgs)
				}
				want++
				data, err := os.ReadFile(filepath.Join(e.cache, filepath.FromSlash(r.RelPath)))
				if err != nil {
					t.Fatalf("%s file: %v", key, err)
				}
				if r.RelPath != "images/"+id+"/"+kind+"-"+sizeName(w)+"."+f.ext || r.MediaType != f.mediaType ||
					r.Hash != sha(data) || r.Bytes != int64(len(data)) || r.Source != source ||
					r.Width != int64(w) || r.Height != scaledHeight(w, cfg.Width, cfg.Height) {
					t.Fatalf("%s row %+v (%d bytes on disk)", key, r, len(data))
				}
				switch f.format {
				case transcode.ImageWebP:
					if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
						t.Fatalf("%s is not WebP", key)
					}
				case transcode.ImageJPEG:
					c, err := jpeg.DecodeConfig(bytes.NewReader(data))
					if err != nil || int64(c.Width) != r.Width || int64(c.Height) != r.Height {
						t.Fatalf("%s JPEG header %+v: %v", key, c, err)
					}
				}
				switch {
				case kind == "backdrop" && r.Accent.Valid:
					t.Fatalf("%s carries an accent", key)
				case kind == "poster" && (!r.Accent.Valid || (accent != "" && r.Accent.String != accent)):
					t.Fatalf("%s accent %v, others %q", key, r.Accent, accent)
				case kind == "poster":
					accent = r.Accent.String
				}
			}
		}
	}
	if len(imgs) != want {
		t.Fatalf("%d image rows, want %d: %v", len(imgs), want, imgs)
	}
	entries, err := os.ReadDir(filepath.Join(e.cache, "images", id))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != want {
		t.Fatalf("image dir holds %d entries, want %d (no temp files left)", len(entries), want)
	}
	return imgs
}

func TestImageFetchStoresFilesAndIsIdempotent(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	t.Logf("image.fetch of one title (2 originals, 12 derived images, palette): %v", time.Since(start))
	imgs := e.checkPipeline(t, idByTMDB, "poster", "backdrop")
	// The red upper half of the poster is the most saturated colour at medium lightness.
	if a := imgs["poster/w400/webp"].Accent.String; len(a) != 7 || a[1:3] < "a0" || a[3:5] > "60" || a[5:7] > "60" {
		t.Fatalf("accent %q, want the poster's red", a)
	}
	poster, backdrop := "/t/p/original/poster550.jpg", "/t/p/original/backdrop550.jpg"

	// Second run: same sources and existing files, no download and no ffmpeg run (a broken runner
	// would fail the job).
	runner := e.svc.runner
	e.svc.runner = transcode.Runner{FFmpegPath: filepath.Join(t.TempDir(), "missing-ffmpeg")}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.svc.runner = runner
	if e.tmdb.count(poster) != 1 || e.tmdb.count(backdrop) != 1 {
		t.Fatalf("originals downloaded %d/%d times, want 1", e.tmdb.count(poster), e.tmdb.count(backdrop))
	}

	// A missing derived file is rebuilt from the kept original, without a download.
	if err := os.Remove(filepath.Join(e.cache, "images", idByTMDB, "poster-w800.webp")); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.checkPipeline(t, idByTMDB, "poster", "backdrop")
	if e.tmdb.count(poster) != 1 {
		t.Fatalf("poster downloaded %d times, want 1 (the original is kept)", e.tmdb.count(poster))
	}

	// A missing original is downloaded again.
	for _, name := range []string{"poster-original.jpg", "poster-w200.jpg"} {
		if err := os.Remove(filepath.Join(e.cache, "images", idByTMDB, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.checkPipeline(t, idByTMDB, "poster", "backdrop")
	if e.tmdb.count(poster) != 2 || e.tmdb.count(backdrop) != 1 {
		t.Fatalf("downloads poster %d, backdrop %d; want 2 and 1", e.tmdb.count(poster), e.tmdb.count(backdrop))
	}

	// A changed TMDB image (another source URL) replaces original and derived rows.
	mustExec(t, e.db.Write, `UPDATE images SET source_url = 'https://image.tmdb.org/t/p/original/old.jpg' WHERE item_id = ? AND kind = 'backdrop'`, idByTMDB)
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.checkPipeline(t, idByTMDB, "poster", "backdrop")
	if e.tmdb.count(backdrop) != 2 {
		t.Fatalf("backdrop downloaded %d times, want 2 after a source change", e.tmdb.count(backdrop))
	}
}

// insertM1aImage stores an image the way M1a did (TMDB size, unchanged JPEG, format column at its
// migration default) and returns its file path.
func (e *env) insertM1aImage(t *testing.T, id, kind, size string, data []byte) string {
	t.Helper()
	rel := "images/" + id + "/" + kind + "-" + size + ".jpg"
	path := filepath.Join(e.cache, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `INSERT INTO images (item_id, kind, size, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
	                         VALUES (?, ?, ?, ?, ?, ?, 'image/jpeg', 20, 30, ?, 1)`,
		id, kind, size, e.tmdb.srv.URL+"/t/p/"+size+"/x.jpg", rel, sha(data), len(data))
	return path
}

func TestImageFetchReplacesM1aImages(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	old := []byte("m1a image")
	w500 := e.insertM1aImage(t, idByTMDB, "poster", "w500", old)
	w1280 := e.insertM1aImage(t, idByTMDB, "backdrop", "w1280", old)
	e.insertM1aImage(t, idByTMDB, "backdrop", "w780", old)
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueueIncompleteImages(ctx, 1); err != nil || n != 1 {
		t.Fatalf("M1a title queued %d, %v; want 1", n, err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.checkPipeline(t, idByTMDB, "poster", "backdrop")
	if _, err := os.Stat(w500); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("M1a poster file still there: %v", err)
	}
	if data, err := os.ReadFile(w1280); err != nil || bytes.Equal(data, old) {
		t.Fatalf("M1a backdrop file not replaced: %v", err)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueueIncompleteImages(ctx, 1); err != nil || n != 0 {
		t.Fatalf("after the pipeline queued %d, %v; want 0", n, err)
	}
}

func TestQueueIncompleteImages(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idNoRating, title: "Unrated", tmdbID: 602}) // TMDB has no images
	for _, id := range []string{idByTMDB, idNoRating} {
		if err := e.svc.match(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	check := func(step string, wantQueued int, wantJobs ...string) {
		t.Helper()
		mustExec(t, e.db.Write, `DELETE FROM jobs`)
		n, err := e.svc.QueueIncompleteImages(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := e.imageJobs(t); n != wantQueued || !slices.Equal(got, wantJobs) {
			t.Fatalf("%s: queued %d, jobs %q; want %d, %q", step, n, got, wantQueued, wantJobs)
		}
	}
	job := "image:" + idByTMDB + ` p2 {"itemId":"` + idByTMDB + `"}`
	check("complete", 0)

	// A store without one format of a size: queued, and image.fetch fills it in.
	mustExec(t, e.db.Write, `DELETE FROM images WHERE item_id = ? AND size = 'w780' AND format = 'webp'`, idByTMDB)
	check("w780 webp missing", 1, job)
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	check("w780 webp rebuilt", 0)

	// The original alone may go missing: the delivered images are complete.
	if err := os.Remove(filepath.Join(e.cache, "images", idByTMDB, "backdrop-original.jpg")); err != nil {
		t.Fatal(err)
	}
	check("original file deleted", 0)

	// A derived file deleted from the cache.
	if err := os.Remove(filepath.Join(e.cache, "images", idByTMDB, "poster-w400.jpg")); err != nil {
		t.Fatal(err)
	}
	check("poster file deleted", 1, job)
	// A locked kind is not fetched from TMDB but scaled from the stored original, so it is checked
	// as well; image.fetch rebuilds the file without a download.
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["poster"]' WHERE id = ?`, idByTMDB)
	check("poster locked", 1, job)
	posterDownloads := e.tmdb.count("/t/p/original/poster550.jpg")
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.cache, "images", idByTMDB, "poster-w400.jpg")); err != nil {
		t.Fatalf("locked poster file not rebuilt: %v", err)
	}
	if got := e.tmdb.count("/t/p/original/poster550.jpg"); got != posterDownloads {
		t.Fatalf("locked poster downloaded again (%d → %d)", posterDownloads, got)
	}
	check("locked poster rebuilt", 0)
	// A locked kind without any file left cannot be scaled: not queued.
	for _, name := range []string{"original.jpg", "w200.jpg", "w200.webp", "w400.jpg", "w400.webp", "w800.jpg", "w800.webp"} {
		if err := os.Remove(filepath.Join(e.cache, "images", idByTMDB, "poster-"+name)); err != nil {
			t.Fatal(err)
		}
	}
	check("locked poster without files", 0)
}

// M11: a locked M1a poster (w500 only, no original) is scaled into the new sizes without TMDB.
func TestImageFetchScalesLockedM1aPoster(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550, locked: `["poster"]`})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	w500 := e.insertM1aImage(t, idByTMDB, "poster", "w500", jpegBytes(t, 500, 750))
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueueIncompleteImages(ctx, 1); err != nil || n != 1 {
		t.Fatalf("locked M1a poster queued %d, %v; want 1", n, err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/t/p/original/poster550.jpg"); n != 0 {
		t.Fatalf("locked poster downloaded %d times", n)
	}
	imgs := e.images(t, idByTMDB)
	for _, size := range []string{"w200", "w400", "w800"} {
		for _, format := range []string{"jpeg", "webp"} {
			r, ok := imgs["poster/"+size+"/"+format]
			if !ok || r.Source != e.tmdb.srv.URL+"/t/p/w500/x.jpg" || !r.Accent.Valid {
				t.Errorf("poster %s %s = %+v (present %v)", size, format, r, ok)
			}
		}
	}
	if o := imgs["poster/original/jpeg"]; o.Width != 500 || o.Height != 750 {
		t.Errorf("promoted original = %+v", o)
	}
	if _, ok := imgs["poster/w500/jpeg"]; ok {
		t.Error("w500 row kept")
	}
	if _, err := os.Stat(w500); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("w500 file kept: %v", err)
	}
	// The unlocked backdrop still came from TMDB.
	if _, ok := imgs["backdrop/w1280/webp"]; !ok {
		t.Error("backdrop not fetched")
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueueIncompleteImages(ctx, 1); err != nil || n != 0 {
		t.Fatalf("after scaling queued %d, %v; want 0", n, err)
	}
}

// failingPalette returns an ffmpeg wrapper that fails every palettegen run and runs the real
// ffmpeg otherwise.
func failingPalette(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\ncase \"$*\" in *palettegen*) echo palette broken >&2; exit 1;; esac\nexec " + lookFFmpeg() + " \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// M10: a palette failure keeps the previous accent; a poster without accent is retried a bounded
// number of times and image.fetch fills it in.
func TestAccentSurvivesPaletteFailure(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	good := e.images(t, idByTMDB)["poster/w400/jpeg"].Accent
	if !good.Valid {
		t.Fatal("no accent after the first run")
	}

	// The poster is derived again (a file went missing) while the palette fails: accent kept.
	runner := e.svc.runner
	e.svc.runner = transcode.Runner{FFmpegPath: failingPalette(t), FFprobePath: runner.FFprobePath}
	if err := os.Remove(filepath.Join(e.cache, "images", idByTMDB, "poster-w800.jpg")); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	for key, r := range e.images(t, idByTMDB) {
		if strings.HasPrefix(key, "poster/w") && r.Accent != good {
			t.Errorf("%s accent %v after a palette failure, want %v", key, r.Accent, good)
		}
	}

	// Rows without accent (an earlier failure): queued at most maxAccentRetries times.
	mustExec(t, e.db.Write, `UPDATE images SET accent = NULL WHERE item_id = ?`, idByTMDB)
	for i := range maxAccentRetries + 2 {
		mustExec(t, e.db.Write, `DELETE FROM jobs`)
		n, err := e.svc.QueueIncompleteImages(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if i < maxAccentRetries {
			want = 1
		}
		if n != want {
			t.Fatalf("check %d queued %d, want %d", i+1, n, want)
		}
		if i == 0 {
			// Still failing: nothing changes.
			if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
				t.Fatal(err)
			}
			if e.images(t, idByTMDB)["poster/w400/jpeg"].Accent.Valid {
				t.Fatal("accent appeared with a failing palette")
			}
		}
	}
	// With a working palette the next image.fetch fills in the accent without re-deriving.
	e.svc.runner = runner
	before := e.images(t, idByTMDB)["poster/w400/webp"].Hash
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	imgs := e.checkPipeline(t, idByTMDB, "poster", "backdrop")
	if imgs["poster/w400/webp"].Accent != good || imgs["poster/w400/webp"].Hash != before {
		t.Errorf("filled accent %v (want %v), hash changed %v", imgs["poster/w400/webp"].Accent, good, imgs["poster/w400/webp"].Hash != before)
	}
}

func TestImageFetchWithoutPathsStoresNothing(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idNoRating, title: "Unrated", tmdbID: 602})
	if err := e.svc.match(ctx, idNoRating); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idNoRating); err != nil {
		t.Fatal(err)
	}
	if imgs := e.images(t, idNoRating); len(imgs) != 0 {
		t.Fatalf("rows %+v for a movie without images", imgs)
	}
	if _, err := os.Stat(filepath.Join(e.cache, "images", idNoRating)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("image dir created without images: %v", err)
	}
	// Only a poster: the original and its six derived rows.
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2002})
	if err := e.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	e.checkPipeline(t, idBySearch, "poster")
}

func TestImageFetchRetryReusesOriginal(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	// Scaling fails (ffmpeg missing): the job fails, the downloaded original stays.
	runner := e.svc.runner
	e.svc.runner = transcode.Runner{FFmpegPath: filepath.Join(t.TempDir(), "missing-ffmpeg")}
	if err := e.svc.fetchImages(ctx, idByTMDB); err == nil {
		t.Fatal("image.fetch succeeded without ffmpeg")
	}
	imgs := e.images(t, idByTMDB)
	if len(imgs) != 1 || imgs["poster/original/jpeg"].RelPath == "" {
		t.Fatalf("rows after the failed run: %v, want the poster original only", imgs)
	}
	entries, err := os.ReadDir(filepath.Join(e.cache, "images", idByTMDB))
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), ".tmp-") {
			t.Fatalf("temporary file %s left behind", en.Name())
		}
	}
	// The retry scales from the kept original.
	e.svc.runner = runner
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.checkPipeline(t, idByTMDB, "poster", "backdrop")
	if n := e.tmdb.count("/t/p/original/poster550.jpg"); n != 1 {
		t.Fatalf("poster downloaded %d times, want 1", n)
	}
}

func TestJobChainWithWorker(t *testing.T) {
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByIMDB, title: "Whatever", imdbID: "tt0000550"})
	w := jobs.NewWorker(e.queue, "metadata-test", slog.New(slog.DiscardHandler))
	e.svc.RegisterJobs(w)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	payload, _ := json.Marshal(map[string]string{"itemId": idByIMDB})
	if _, err := e.queue.Enqueue(context.Background(), jobs.NewJob{Type: JobMatch, IdempotencyKey: "match:" + idByIMDB, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for len(e.images(t, idByIMDB)) < 14 {
		if time.Now().After(deadline) {
			t.Fatalf("images not stored; item %+v, image jobs %q", e.item(t, idByIMDB), e.imageJobs(t))
		}
		time.Sleep(20 * time.Millisecond)
	}
	var states string
	if err := e.db.Read.QueryRow(`SELECT group_concat(type || '=' || state, ',') FROM (SELECT type, state FROM jobs ORDER BY id)`).Scan(&states); err != nil {
		t.Fatal(err)
	}
	for !strings.Contains(states, "image.fetch=done") {
		if time.Now().After(deadline) {
			t.Fatalf("job states %s", states)
		}
		time.Sleep(20 * time.Millisecond)
		_ = e.db.Read.QueryRow(`SELECT group_concat(type || '=' || state, ',') FROM (SELECT type, state FROM jobs ORDER BY id)`).Scan(&states)
	}
	if states != "metadata.match=done,image.fetch=done" {
		t.Fatalf("job states %s", states)
	}
	if r := e.item(t, idByIMDB); r.State != "matched" || r.Source.String != "tmdb:DE" {
		t.Fatalf("item %+v", r)
	}
}
