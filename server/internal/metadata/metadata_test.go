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
	poster []byte
	backdr []byte
	backdS []byte // backdrop in w780

	mu        sync.Mutex
	hits      map[string]int
	rateCalls int
	authSeen  []string
}

func jpegBytes(t *testing.T, w, h int) []byte {
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
	f := &fakeTMDB{hits: map[string]int{}, poster: jpegBytes(t, 20, 30), backdr: jpegBytes(t, 32, 18), backdS: jpegBytes(t, 16, 9)}
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
	mux.HandleFunc("GET /t/p/w500/poster550.jpg", image(f.poster))
	mux.HandleFunc("GET /t/p/w1280/backdrop550.jpg", image(f.backdr))
	mux.HandleFunc("GET /t/p/w780/backdrop550.jpg", image(f.backdS))
	mux.HandleFunc("GET /t/p/w500/poster601.jpg", image(f.poster))
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
	svc := New(db, q, staticKey(key), cache, log,
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

type imageRow struct {
	Kind, Size, RelPath, Hash, MediaType, Source string
	Width, Height, Bytes                         int64
}

func (e *env) images(t *testing.T, id string) map[string]imageRow {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT kind, size, rel_path, content_hash, media_type, source_url, width, height, bytes
	                              FROM images WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]imageRow{}
	for rows.Next() {
		var r imageRow
		if err := rows.Scan(&r.Kind, &r.Size, &r.RelPath, &r.Hash, &r.MediaType, &r.Source, &r.Width, &r.Height, &r.Bytes); err != nil {
			t.Fatal(err)
		}
		out[r.Kind+"/"+r.Size] = r
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

func TestImageFetchStoresFilesAndIsIdempotent(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	imgs := e.images(t, idByTMDB)
	want := map[string]struct {
		rel  string
		data []byte
		w, h int64
	}{
		"poster/w500":    {"images/" + idByTMDB + "/poster-w500.jpg", e.tmdb.poster, 20, 30},
		"backdrop/w1280": {"images/" + idByTMDB + "/backdrop-w1280.jpg", e.tmdb.backdr, 32, 18},
		"backdrop/w780":  {"images/" + idByTMDB + "/backdrop-w780.jpg", e.tmdb.backdS, 16, 9},
	}
	if len(imgs) != 3 {
		t.Fatalf("image rows %+v", imgs)
	}
	for kind, w := range want {
		r := imgs[kind]
		if r.RelPath != w.rel || r.Hash != sha(w.data) || r.Bytes != int64(len(w.data)) ||
			r.Width != w.w || r.Height != w.h || r.MediaType != "image/jpeg" {
			t.Fatalf("%s row %+v", kind, r)
		}
		if !strings.HasPrefix(r.Source, e.tmdb.srv.URL+"/t/p/") {
			t.Fatalf("%s source_url %s", kind, r.Source)
		}
		got, err := os.ReadFile(filepath.Join(e.cache, filepath.FromSlash(w.rel)))
		if err != nil || !bytes.Equal(got, w.data) {
			t.Fatalf("%s file: %v", kind, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(e.cache, "images", idByTMDB))
	if len(entries) != 3 {
		t.Fatalf("image dir holds %d entries, want 3 (no temp files left)", len(entries))
	}

	// Second run: same sources and existing files, no download.
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/t/p/w500/poster550.jpg"); n != 1 {
		t.Fatalf("poster downloaded %d times, want 1", n)
	}
	// A missing file is fetched again.
	if err := os.Remove(filepath.Join(e.cache, filepath.FromSlash(want["poster/w500"].rel))); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/t/p/w500/poster550.jpg"); n != 2 {
		t.Fatalf("poster downloaded %d times, want 2 after the file vanished", n)
	}
	if n := e.tmdb.count("/t/p/w1280/backdrop550.jpg"); n != 1 {
		t.Fatalf("backdrop downloaded %d times, want 1", n)
	}

	// A size added later (w780 next to w1280): the next run fetches only the missing size.
	if _, err := e.db.Write.Exec(`DELETE FROM images WHERE item_id = ? AND size = 'w780'`, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(e.cache, filepath.FromSlash(want["backdrop/w780"].rel))); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if len(e.images(t, idByTMDB)) != 3 || e.tmdb.count("/t/p/w780/backdrop550.jpg") != 2 ||
		e.tmdb.count("/t/p/w1280/backdrop550.jpg") != 1 || e.tmdb.count("/t/p/w500/poster550.jpg") != 2 {
		t.Fatalf("after removing w780: rows %d, %d TMDB requests", len(e.images(t, idByTMDB)), e.tmdb.total())
	}
}

func TestQueueIncompleteImages(t *testing.T) {
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

	// An older store without the w780 backdrop: queued, and image.fetch fills in that size.
	mustExec(t, e.db.Write, `DELETE FROM images WHERE item_id = ? AND size = 'w780'`, idByTMDB)
	check("w780 missing", 1, job)
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	check("w780 fetched", 0)

	// A file deleted from the cache.
	if err := os.Remove(filepath.Join(e.cache, "images", idByTMDB, "poster-w500.jpg")); err != nil {
		t.Fatal(err)
	}
	check("poster file deleted", 1, job)
	// A locked kind is never fetched, so it is not checked either.
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["poster"]' WHERE id = ?`, idByTMDB)
	check("poster locked", 0)
}

func TestImageFetchWithoutPathsStoresNothing(t *testing.T) {
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
	// Only a poster: one row.
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2002})
	if err := e.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if imgs := e.images(t, idBySearch); len(imgs) != 1 || imgs["poster/w500"].Width != 20 {
		t.Fatalf("rows %+v, want the poster only", imgs)
	}
}

func TestJobChainWithWorker(t *testing.T) {
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
	for len(e.images(t, idByIMDB)) < 3 {
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
