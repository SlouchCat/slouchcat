package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

const (
	tvdbKey   = "tvdb-test-api-key-0001"
	tvdbPin   = "4711-subscriber"
	tvdbToken = "tvdb-bearer-token-secret"
)

// testKeys is a KeySource with every provider key.
type testKeys struct {
	tmdb, tvdb, pin, youtube string
}

func (k testKeys) TMDBKey(context.Context) (string, error) { return k.tmdb, nil }

func (k testKeys) TVDBKey(context.Context) (string, string, error) { return k.tvdb, k.pin, nil }

func (k testKeys) YouTubeKey(context.Context) (string, error) { return k.youtube, nil }

// tid returns a 26 character item ID for tag and n.
func tid(tag string, n int) string {
	return fmt.Sprintf("01J%s%0*d", tag, 23-len(tag), n)
}

// node is an item row of a test series tree (library 2).
type node struct {
	id, kind, title string
	parent          any
	year            any
	tmdbID, tvdbID  any
	imdbID          any
	season, episode any
	episodeEnd      any
	extraKind       any
	locked          string
	state           string
}

func (e *env) addNode(t *testing.T, n node) {
	t.Helper()
	if n.locked == "" {
		n.locked = "[]"
	}
	if n.state == "" {
		n.state = "pending"
	}
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, year, tmdb_id, tvdb_id, imdb_id,
	                         season_number, episode_number, episode_end, extra_kind, locked_fields, metadata_state,
	                         added_at, created_at, updated_at)
	                         VALUES (?, ?, ?, 2, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0)`,
		n.id, n.kind, n.parent, n.title, strings.ToLower(n.title), n.year, n.tmdbID, n.tvdbID, n.imdbID,
		n.season, n.episode, n.episodeEnd, n.extraKind, n.locked, n.state)
}

// Items of the standard tree below a series.
var (
	idSeries  = tid("SER", 1)
	idSeason1 = tid("SEA", 1)
	idSeason0 = tid("SEA", 0)
	idSeason2 = tid("SEA", 2)
	idE0101   = tid("EPI", 101)
	idE0102   = tid("EPI", 102) // S01E02-E03 in one file
	idE0109   = tid("EPI", 109) // unknown to TMDB
	idE0001   = tid("EPI", 1)
	idE0201   = tid("EPI", 201) // season 2 is unknown to TMDB
	idExtra   = tid("EXT", 1)
)

// addTree adds a series with seasons 0, 1 and 2, five episodes and an extra.
func (e *env) addTree(t *testing.T, series node) {
	t.Helper()
	series.id, series.kind = idSeries, "series"
	e.addNode(t, series)
	for n, id := range map[int]string{0: idSeason0, 1: idSeason1, 2: idSeason2} {
		e.addNode(t, node{id: id, kind: "season", parent: idSeries, title: fmt.Sprintf("Staffel %d", n), season: n})
	}
	e.addNode(t, node{id: idE0101, kind: "episode", parent: idSeason1, title: "Folge 1", season: 1, episode: 1})
	e.addNode(t, node{id: idE0102, kind: "episode", parent: idSeason1, title: "Folge 2", season: 1, episode: 2, episodeEnd: 3})
	e.addNode(t, node{id: idE0109, kind: "episode", parent: idSeason1, title: "Folge 9", season: 1, episode: 9})
	e.addNode(t, node{id: idE0001, kind: "episode", parent: idSeason0, title: "Folge 1", season: 0, episode: 1})
	e.addNode(t, node{id: idE0201, kind: "episode", parent: idSeason2, title: "Folge 1", season: 2, episode: 1})
	e.addNode(t, node{id: idExtra, kind: "extra", parent: idSeries, title: "Trailer", extraKind: "trailer"})
}

// childRow is the part of a season or episode row the tests check.
type childRow struct {
	State                  string
	Err, Date              sql.NullString
	Poster, Backdrop       sql.NullString
	Runtime, TMDB, TVDB    sql.NullInt64
	Rating                 sql.NullInt64
	Source, IMDB, Original sql.NullString
}

func (e *env) child(t *testing.T, id string) childRow {
	t.Helper()
	var r childRow
	err := e.db.Read.QueryRow(`SELECT metadata_state, metadata_error, release_date, tmdb_poster_path, tmdb_backdrop_path,
	                                  runtime_minutes, tmdb_id, tvdb_id, own_rating, rating_source, imdb_id, original_title
	                           FROM items WHERE id = ?`, id).
		Scan(&r.State, &r.Err, &r.Date, &r.Poster, &r.Backdrop, &r.Runtime, &r.TMDB, &r.TVDB, &r.Rating, &r.Source, &r.IMDB, &r.Original)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// jobList returns "<key> p<priority>" of the queued jobs of type, in insertion order.
func (e *env) jobList(t *testing.T, jobType string) []string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT idempotency_key || ' p' || priority FROM jobs WHERE type = ? ORDER BY id`, jobType)
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

type creditRow struct {
	TMDB      int64
	Name      string
	Role      string
	Character sql.NullString
	Ord       int64
}

func (e *env) credits(t *testing.T, id string) []creditRow {
	t.Helper()
	rows, err := e.svc.read.ListItemCreditsSystem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]creditRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, creditRow{TMDB: r.TmdbID.Int64, Name: r.Name, Role: r.Role, Character: r.Character, Ord: r.Ord})
	}
	return out
}

// addTV adds the TMDB series endpoints of series 900 "Test Show" to the fake.
func (f *fakeTMDB) addTV(t *testing.T, mux *http.ServeMux, auth func(http.HandlerFunc) http.HandlerFunc, writeJSON func(http.ResponseWriter, any)) {
	lang := func(r *http.Request) string {
		l := r.URL.Query().Get("language")
		if l != langDE && l != langEN {
			t.Errorf("unexpected language in %s", r.URL)
		}
		return l
	}
	mux.HandleFunc("GET /3/tv/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "900" {
			http.Error(w, `{"status_code":34}`, http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("append_to_response"); got != "content_ratings,external_ids,credits" {
			t.Errorf("tv request appends %q", got)
		}
		cast := []any{}
		for i := range 17 {
			cast = append(cast, map[string]any{"id": 8000 + i, "name": fmt.Sprintf("Actor %02d", i), "character": fmt.Sprintf("Role %02d", i), "order": i})
		}
		tv := map[string]any{
			"id": 900, "original_name": "Test Show", "first_air_date": "2010-04-01", "episode_run_time": []int{0, 42},
			"poster_path": "/tvposter900.jpg", "backdrop_path": "/tvbackdrop900.jpg",
			"seasons": []any{
				map[string]any{"season_number": 0, "episode_count": 1, "poster_path": "/s0.jpg"},
				map[string]any{"season_number": 1, "episode_count": 3, "poster_path": "/s1-list.jpg"},
				map[string]any{"season_number": 2, "episode_count": 10},
			},
			"created_by":      []any{map[string]any{"id": 7001, "name": "Creator One", "profile_path": "/c1.jpg"}},
			"content_ratings": map[string]any{"results": []any{map[string]any{"iso_3166_1": "US", "rating": "TV-14"}, map[string]any{"iso_3166_1": "DE", "rating": "12"}}},
			"external_ids":    map[string]any{"imdb_id": "tt0000900", "tvdb_id": 81189},
			"credits": map[string]any{"cast": cast, "crew": []any{
				map[string]any{"id": 8100, "name": "Episode Director", "job": "Director", "department": "Directing"},
			}},
		}
		if lang(r) == langDE {
			tv["name"], tv["overview"], tv["genres"] = "Testserie", "Deutsche Serienbeschreibung", []any{map[string]any{"id": 18, "name": "Drama"}}
		} else {
			tv["name"], tv["overview"], tv["genres"] = "Test Show", "English series overview", []any{map[string]any{"id": 18, "name": "Drama"}}
		}
		writeJSON(w, tv)
	}))
	episode := func(s, e int, l string) (map[string]any, bool) {
		names := map[string][2]string{"1/1": {"Der Anfang", "Pilot"}, "1/2": {"Zweite", "Second"}, "1/3": {"", "Third"}, "0/1": {"", "Special One"}}
		n, ok := names[fmt.Sprintf("%d/%d", s, e)]
		if !ok {
			return nil, false
		}
		name := n[1]
		if l == langDE {
			name = n[0]
		}
		return map[string]any{"season_number": s, "episode_number": e, "name": name, "overview": "Overview " + l,
			"air_date": fmt.Sprintf("2010-04-%02d", e), "runtime": 40 + e, "still_path": fmt.Sprintf("/still%d-%d.jpg", s, e)}, true
	}
	mux.HandleFunc("GET /3/tv/{id}/season/{n}", auth(func(w http.ResponseWriter, r *http.Request) {
		l := lang(r)
		var s map[string]any
		switch r.PathValue("n") {
		case "1":
			s = map[string]any{"season_number": 1, "name": map[string]string{langDE: "Staffel 1", langEN: "Season 1"}[l],
				"overview": "Season overview " + l, "air_date": "2010-04-01", "poster_path": "/s1.jpg"}
			eps := []any{}
			for e := 1; e <= 3; e++ {
				ep, _ := episode(1, e, l)
				eps = append(eps, ep)
			}
			s["episodes"] = eps
		case "0":
			ep, _ := episode(0, 1, l)
			s = map[string]any{"season_number": 0, "name": "", "overview": "", "episodes": []any{ep}}
		default:
			http.Error(w, `{"status_code":34}`, http.StatusNotFound)
			return
		}
		writeJSON(w, s)
	}))
	mux.HandleFunc("GET /3/tv/{id}/season/{n}/episode/{e}", auth(func(w http.ResponseWriter, r *http.Request) {
		var s, e int
		_, _ = fmt.Sscan(r.PathValue("n"), &s)
		_, _ = fmt.Sscan(r.PathValue("e"), &e)
		ep, ok := episode(s, e, lang(r))
		if !ok {
			http.Error(w, `{"status_code":34}`, http.StatusNotFound)
			return
		}
		writeJSON(w, ep)
	}))
	mux.HandleFunc("GET /3/search/tv", auth(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		results := []any{}
		if q.Get("query") == "Test Show" && q.Get("first_air_date_year") != "2011" {
			results = append(results, map[string]any{"id": 900, "name": "Test Show", "original_name": "Test Show",
				"first_air_date": "2010-04-01", "overview": "English series overview", "poster_path": "/tvposter900.jpg"})
		}
		writeJSON(w, map[string]any{"results": results})
	}))
	tvImage := func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits[r.URL.Path]++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "image/jpeg")
		if strings.HasPrefix(r.PathValue("name"), "still") || strings.HasPrefix(r.PathValue("name"), "tvbackdrop") {
			_, _ = w.Write(f.backdr)
			return
		}
		_, _ = w.Write(f.poster)
	}
	mux.HandleFunc("GET /t/p/original/{name}", tvImage)

	// Videos (trailer chain): the same list for every request language, as TMDB answers with
	// include_video_language.
	videos := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("include_video_language") != "de,en,null" {
			t.Errorf("videos without include_video_language: %s", r.URL)
		}
		lang(r)
		v := func(lang any, key, typ, site string, official bool, published, name string) map[string]any {
			return map[string]any{"iso_639_1": lang, "iso_3166_1": "US", "key": key, "type": typ, "site": site,
				"official": official, "published_at": published, "name": name, "size": 1080, "id": key}
		}
		results := []any{}
		switch r.URL.Path {
		case "/3/movie/550/videos":
			results = []any{
				v("de", "deTrailer01", "Trailer", "YouTube", false, "2021-01-01T10:00:00.000Z", "Trailer (neu)"),
				v("de", "deOfficial1", "Trailer", "YouTube", true, "2019-01-01T10:00:00.000Z", "Offizieller Trailer"),
				v("en", "enOlder0001", "Trailer", "YouTube", true, "2018-01-01T10:00:00.000Z", "Official Trailer 1"),
				v("en", "enOfficial1", "Trailer", "YouTube", true, "2019-06-01T10:00:00.000Z", "Official Trailer 2"),
				v("en", "enTeaser001", "Teaser", "YouTube", true, "2022-01-01T10:00:00.000Z", "Teaser"),
				v("en", "vimeo123456", "Trailer", "Vimeo", true, "2022-01-01T10:00:00.000Z", "Vimeo trailer"),
				v("fr", "frTrailer01", "Trailer", "YouTube", true, "2022-01-01T10:00:00.000Z", "Bande-annonce"),
				v(nil, "noLang00001", "Trailer", "YouTube", true, "2022-01-01T10:00:00.000Z", "Without language"),
			}
		case "/3/tv/900/videos":
			results = []any{v("en", "tvTrailer01", "Trailer", "YouTube", true, "2010-01-01T10:00:00.000Z", "Season 1 Trailer")}
		}
		writeJSON(w, map[string]any{"id": 1, "results": results})
	}
	mux.HandleFunc("GET /3/movie/{id}/videos", auth(videos))
	mux.HandleFunc("GET /3/tv/{id}/videos", auth(videos))
}

// fakeTVDB serves the TVDB v4 endpoints the provider uses, with the answer shapes of the real API.
type fakeTVDB struct {
	srv *httptest.Server

	mu     sync.Mutex
	logins int
	token  string
	hits   map[string]int
}

func newFakeTVDB(t *testing.T) *fakeTVDB {
	t.Helper()
	f := &fakeTVDB{hits: map[string]int{}}
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("POST /v4/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			APIKey string `json:"apikey"`
			PIN    string `json:"pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIKey != tvdbKey || body.PIN != tvdbPin {
			w.WriteHeader(http.StatusUnauthorized)
			writeJSON(w, map[string]any{"status": "failure", "message": "Unauthorized", "data": nil})
			return
		}
		f.mu.Lock()
		f.logins++
		f.token = fmt.Sprintf("%s-%d", tvdbToken, f.logins)
		token := f.token
		f.mu.Unlock()
		writeJSON(w, map[string]any{"status": "success", "data": map[string]any{"token": token}})
	})
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.hits[r.URL.Path]++
			ok := f.token != "" && r.Header.Get("Authorization") == "Bearer "+f.token
			f.mu.Unlock()
			for _, secret := range []string{tvdbKey, tvdbPin, tvdbToken} {
				if strings.Contains(r.URL.String(), secret) {
					t.Errorf("secret leaked into the URL %s", r.URL)
				}
			}
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				writeJSON(w, map[string]any{"status": "failure", "message": "Unauthorized"})
				return
			}
			next(w, r)
		}
	}
	art := func(p string) string { return f.srv.URL + "/art" + p }
	mux.HandleFunc("GET /v4/search", auth(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("type") != "series" {
			t.Errorf("search without type=series: %s", r.URL)
		}
		data := []any{}
		if q.Get("query") == "Only On TVDB" {
			data = append(data, map[string]any{
				"objectID": "series-424242", "id": "series-424242", "tvdb_id": "424242", "type": "series",
				"name": "Nur bei TVDB", "year": "2015", "first_air_time": "2015-02-03", "overview": "Originalbeschreibung",
				"image_url": art("/banners/posters/424242-1.jpg"), "thumbnail": art("/banners/posters/424242-1_t.jpg"),
				"translations":     map[string]string{"eng": "Only on TVDB", "deu": "Nur bei TVDB"},
				"overviews":        map[string]string{"eng": "English overview"},
				"primary_language": "deu", "remote_ids": []any{map[string]any{"id": "tt4242", "sourceName": "IMDB"}},
			})
		}
		writeJSON(w, map[string]any{"status": "success", "data": data, "links": map[string]any{"prev": nil, "self": r.URL.String(), "next": nil}})
	}))
	mux.HandleFunc("GET /v4/series/{id}/extended", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "424242" {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"status": "failure", "message": "NotFoundException"})
			return
		}
		if r.URL.Query().Get("meta") != "translations" {
			t.Errorf("extended without meta=translations: %s", r.URL)
		}
		writeJSON(w, map[string]any{"status": "success", "data": map[string]any{
			"id": 424242, "name": "Nur bei TVDB", "slug": "nur-bei-tvdb", "image": art("/banners/posters/424242-1.jpg"),
			"firstAired": "2015-02-03", "year": "2015", "averageRuntime": 25, "originalLanguage": "deu",
			"overview":       "Originalbeschreibung",
			"genres":         []any{map[string]any{"id": 4, "name": "Comedy", "slug": "comedy"}},
			"contentRatings": []any{map[string]any{"id": 1, "name": "TV-PG", "country": "usa", "contentType": ""}, map[string]any{"id": 2, "name": "12", "country": "deu", "contentType": ""}},
			"remoteIds":      []any{map[string]any{"id": "tt4242", "type": 2, "sourceName": "IMDB"}, map[string]any{"id": "999", "type": 12, "sourceName": "TheMovieDB.com"}},
			"artworks": []any{
				map[string]any{"id": 1, "image": art("/banners/posters/424242-2.jpg"), "type": 2, "score": 100000, "language": "deu"},
				map[string]any{"id": 2, "image": art("/banners/posters/424242-1.jpg"), "type": 2, "score": 10, "language": "eng"},
				map[string]any{"id": 3, "image": art("/banners/fanart/424242-1.jpg"), "type": 3, "score": 5, "language": nil},
			},
			"seasons": []any{
				map[string]any{"id": 11, "number": 1, "image": art("/banners/seasons/424242-1.jpg"), "type": map[string]any{"id": 1, "type": "official", "name": "Aired Order"}},
				map[string]any{"id": 12, "number": 1, "image": "", "type": map[string]any{"id": 2, "type": "dvd", "name": "DVD Order"}},
			},
			"characters": []any{
				map[string]any{"id": 1, "name": "Hauptfigur", "personName": "Max Muster", "peopleType": "Actor", "sort": 1},
				map[string]any{"id": 2, "name": "", "personName": "Regie Person", "peopleType": "Director", "sort": 0},
			},
			"translations": map[string]any{
				"nameTranslations":     []any{map[string]any{"language": "deu", "name": "Nur bei TVDB"}, map[string]any{"language": "eng", "name": "Only on TVDB"}},
				"overviewTranslations": []any{map[string]any{"language": "deu", "overview": "Deutsche Beschreibung"}, map[string]any{"language": "eng", "overview": "English overview"}},
			},
		}})
	}))
	mux.HandleFunc("GET /v4/series/{id}/episodes/default/{lang}", auth(func(w http.ResponseWriter, r *http.Request) {
		lang := r.PathValue("lang")
		names := map[string][2]string{"deu": {"Folge eins", "Folge zwei"}, "eng": {"Episode one", "Episode two"}}[lang]
		page := r.URL.Query().Get("page")
		var eps []any
		var next any
		switch page {
		case "0":
			eps = []any{map[string]any{"id": 1, "seasonNumber": 1, "number": 1, "name": names[0], "overview": "Ov 1 " + lang,
				"aired": "2015-02-03", "runtime": 25, "image": "/banners/episodes/424242/1.jpg"}}
			next = f.srv.URL + r.URL.Path + "?page=1"
		case "1":
			eps = []any{map[string]any{"id": 2, "seasonNumber": 1, "number": 2, "name": names[1], "overview": nil,
				"aired": "2015-02-10", "runtime": nil, "image": nil}}
		default:
			t.Errorf("unexpected page %q", page)
		}
		writeJSON(w, map[string]any{"status": "success",
			"data":  map[string]any{"series": map[string]any{"id": 424242}, "episodes": eps},
			"links": map[string]any{"prev": nil, "self": r.URL.String(), "next": next, "total_items": 2, "page_size": 1}})
	}))
	artwork := jpegBytes(t, 300, 450)
	mux.HandleFunc("GET /art/{path...}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("credentials sent to the artwork host")
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(artwork)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTVDB) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func (f *fakeTVDB) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := f.logins
	for _, v := range f.hits {
		n += v
	}
	return n
}

// expire invalidates the issued token, as TVDB does after a month.
func (f *fakeTVDB) expire() {
	f.mu.Lock()
	f.token = "expired"
	f.mu.Unlock()
}

func TestMatchMovieStoresCreditsAndImagePaths(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	r := e.child(t, idByTMDB)
	if r.Poster.String != "/poster550.jpg" || r.Backdrop.String != "/backdrop550.jpg" {
		t.Fatalf("image paths %v %v", r.Poster, r.Backdrop)
	}
	want := []creditRow{
		{9001, "Lead Actor", "cast", sql.NullString{String: "Hero", Valid: true}, 0},
		{9002, "Second Actor", "cast", sql.NullString{String: "Sidekick", Valid: true}, 1},
		{9101, "The Director", "director", sql.NullString{}, 0},
		{9102, "The Writer", "writer", sql.NullString{}, 1},
	}
	if got := e.credits(t, idByTMDB); !slices.Equal(got, want) {
		t.Fatalf("credits %+v, want %+v", got, want)
	}
	// A second match replaces the credits and reuses the people rows.
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	var people int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM people`).Scan(&people); err != nil || people != 4 {
		t.Fatalf("%d people, %v; want 4", people, err)
	}
	// Locked cast keeps the credits as they are.
	mustExec(t, e.db.Write, `DELETE FROM credits WHERE role = 'cast'`)
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["cast"]' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.credits(t, idByTMDB); len(got) != 2 {
		t.Fatalf("locked cast rewritten: %+v", got)
	}
}

func TestMatchSeriesTMDB(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	// The folder year is one off: the search with the year finds nothing, the one without does.
	e.addTree(t, node{title: "Test Show", year: 2011})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/3/search/tv"); n != 2 {
		t.Fatalf("%d searches, want 2 (with and without year)", n)
	}
	s := e.child(t, idSeries)
	if s.State != "matched" || s.TMDB.Int64 != 900 || s.TVDB.Int64 != 81189 || s.IMDB.String != "tt0000900" ||
		s.Date.String != "2010-04-01" || s.Runtime.Int64 != 42 || s.Original.String != "Test Show" {
		t.Fatalf("series row %+v", s)
	}
	if s.Rating.Int64 != 12 || s.Source.String != "tmdb:DE" {
		t.Fatalf("series rating %v %v, want 12 tmdb:DE", s.Rating, s.Source)
	}
	if s.Poster.String != "/tvposter900.jpg" || s.Backdrop.String != "/tvbackdrop900.jpg" {
		t.Fatalf("series images %v %v", s.Poster, s.Backdrop)
	}
	if r := e.item(t, idSeries); r.Year.Int64 != 2010 || r.Genres != `{"de":["Drama"],"en":["Drama"]}` {
		t.Fatalf("year %v genres %s", r.Year, r.Genres)
	}
	tx := e.texts(t, idSeries)
	if tx["de"].Title != "Testserie" || tx["de"].Overview.String != "Deutsche Serienbeschreibung" || tx["en"].Title != "Test Show" {
		t.Fatalf("series texts %+v", tx)
	}
	creds := e.credits(t, idSeries)
	roles := map[string]int{}
	for _, c := range creds {
		roles[c.Role]++
	}
	if roles["cast"] != 15 || roles["creator"] != 1 || roles["director"] != 1 {
		t.Fatalf("credit roles %v, want 15 cast, 1 creator, 1 director", roles)
	}

	// Seasons: texts from the provider, "Specials" when it has none, poster from the season detail
	// or the series list.
	if r := e.child(t, idSeason1); r.State != "matched" || r.Poster.String != "/s1.jpg" || r.Date.String != "2010-04-01" {
		t.Fatalf("season 1 %+v", r)
	}
	if tx := e.texts(t, idSeason1); tx["de"].Title != "Staffel 1" || tx["en"].Title != "Season 1" || tx["en"].Overview.String != "Season overview en-US" {
		t.Fatalf("season 1 texts %+v", tx)
	}
	if r := e.child(t, idSeason0); r.State != "matched" || r.Poster.String != "/s0.jpg" {
		t.Fatalf("specials %+v", r)
	}
	if tx := e.texts(t, idSeason0); tx["de"].Title != "Specials" || tx["en"].Title != "Specials" {
		t.Fatalf("specials texts %+v", tx)
	}
	// Episodes.
	if r := e.child(t, idE0101); r.State != "matched" || r.Date.String != "2010-04-01" || r.Runtime.Int64 != 41 || r.Backdrop.String != "/still1-1.jpg" {
		t.Fatalf("S01E01 %+v", r)
	}
	if tx := e.texts(t, idE0101); tx["de"].Title != "Der Anfang" || tx["en"].Title != "Pilot" || tx["de"].Overview.String != "Overview de-DE" {
		t.Fatalf("S01E01 texts %+v", tx)
	}
	if tx := e.texts(t, idE0102); tx["de"].Title != "E02–E03: Zweite / Third" || tx["en"].Title != "E02–E03: Second / Third" {
		t.Fatalf("multi-episode texts %+v", tx)
	}
	if r := e.child(t, idE0102); r.Runtime.Int64 != 42 || r.Backdrop.String != "/still1-2.jpg" {
		t.Fatalf("multi-episode row %+v", r)
	}
	if tx := e.texts(t, idE0001); tx["de"].Title != "Special One" {
		t.Fatalf("special texts %+v", tx)
	}
	for _, id := range []string{idE0109, idE0201, idSeason2, idExtra} {
		if r := e.child(t, id); r.State != "pending" {
			t.Fatalf("%s state %s, want pending", id, r.State)
		}
	}
	wantImages := []string{"image:" + idSeries + " p2", "image:" + idSeason0 + " p2", "image:" + idE0001 + " p1",
		"image:" + idSeason1 + " p2", "image:" + idE0101 + " p1", "image:" + idE0102 + " p1"}
	if got := e.jobList(t, JobImageFetch); !slices.Equal(got, wantImages) {
		t.Fatalf("image jobs %q, want %q", got, wantImages)
	}
	wantMatch := []string{"match:" + idE0109 + " p3", "match:" + idE0201 + " p3"}
	if got := e.jobList(t, JobMatch); !slices.Equal(got, wantMatch) {
		t.Fatalf("match jobs %q, want %q", got, wantMatch)
	}

	// The episodes TMDB does not know end unmatched on their own job.
	for _, id := range []string{idE0109, idE0201} {
		if err := e.svc.match(ctx, id); err != nil {
			t.Fatal(err)
		}
		if r := e.child(t, id); r.State != "unmatched" {
			t.Fatalf("%s state %s, want unmatched", id, r.State)
		}
	}
	// A matched episode makes no request; a pending one fetches its episode (and the second one of
	// a multi-episode file).
	before := e.tmdb.total()
	if err := e.svc.match(ctx, idE0101); err != nil {
		t.Fatal(err)
	}
	if e.tmdb.total() != before {
		t.Fatal("matched episode asked TMDB again")
	}
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idE0102)
	mustExec(t, e.db.Write, `DELETE FROM item_texts WHERE item_id = ?`, idE0102)
	if err := e.svc.match(ctx, idE0102); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/3/tv/900/season/1/episode/3"); n != 2 {
		t.Fatalf("second episode fetched %d times, want 2 (de, en)", n)
	}
	if tx := e.texts(t, idE0102); tx["en"].Title != "E02–E03: Second / Third" || e.child(t, idE0102).State != "matched" {
		t.Fatalf("episode match texts %+v", tx)
	}
	// Season and extra jobs never ask the provider.
	before = e.tmdb.total()
	for _, id := range []string{idSeason1, idSeason2, idExtra} {
		if err := e.svc.match(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if e.tmdb.total() != before {
		t.Fatal("season or extra job asked TMDB")
	}
	if r := e.child(t, idSeason1); r.State != "matched" {
		t.Fatalf("matched season reset to %s", r.State)
	}
	if r := e.child(t, idSeason2); r.State != "skipped" || r.Err.String != "season_via_series" {
		t.Fatalf("season 2 %+v", r)
	}
	if r := e.child(t, idExtra); r.State != "skipped" || r.Err.String != "unsupported_kind" {
		t.Fatalf("extra %+v", r)
	}
}

func TestMatchSeriesByTVDBIDThroughFind(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addTree(t, node{title: "Whatever", tvdbID: 81189})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idSeries); r.State != "matched" || r.TMDB.Int64 != 900 {
		t.Fatalf("series %+v", r)
	}
	if e.tmdb.count("/3/find/81189") != 1 || e.tmdb.count("/3/search/tv") != 0 {
		t.Fatal("expected one find by tvdb_id and no search")
	}
}

func TestMatchEpisodeNeedsMatchedSeries(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(context.Background(), idE0101); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idE0101); r.State != "skipped" || r.Err.String != "series_unmatched" {
		t.Fatalf("episode of a pending series: %+v", r)
	}
	if n := e.tmdb.total(); n != 0 {
		t.Fatalf("%d TMDB requests for an episode of an unmatched series", n)
	}
	// The series job fills it in after all.
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idE0101); r.State != "matched" {
		t.Fatalf("episode after the series job: %+v", r)
	}
}

func TestMatchSeriesTVDBFallback(t *testing.T) {
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addNode(t, node{id: idSeries, kind: "series", title: "Only On TVDB", year: 2015})
	e.addNode(t, node{id: idSeason1, kind: "season", parent: idSeries, title: "Staffel 1", season: 1})
	e.addNode(t, node{id: idE0101, kind: "episode", parent: idSeason1, title: "Folge 1", season: 1, episode: 1})
	e.addNode(t, node{id: idE0102, kind: "episode", parent: idSeason1, title: "Folge 2", season: 1, episode: 2})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/3/search/tv"); n != 2 {
		t.Fatalf("%d TMDB searches, want 2 before the fallback", n)
	}
	s := e.child(t, idSeries)
	if s.State != "matched" || s.TVDB.Int64 != 424242 || s.TMDB.Valid || s.IMDB.String != "tt4242" || s.Runtime.Int64 != 25 {
		t.Fatalf("series %+v", s)
	}
	if s.Rating.Int64 != 12 || s.Source.String != "tvdb:DE" {
		t.Fatalf("rating %v %v, want 12 tvdb:DE", s.Rating, s.Source)
	}
	art := e.tvdb.srv.URL + "/art/banners/"
	if s.Poster.String != art+"posters/424242-2.jpg" || s.Backdrop.String != art+"fanart/424242-1.jpg" {
		t.Fatalf("images %v %v", s.Poster, s.Backdrop)
	}
	if tx := e.texts(t, idSeries); tx["de"].Title != "Nur bei TVDB" || tx["de"].Overview.String != "Deutsche Beschreibung" ||
		tx["en"].Title != "Only on TVDB" || tx["en"].Overview.String != "English overview" {
		t.Fatalf("texts %+v", tx)
	}
	if r := e.item(t, idSeries); r.Genres != `{"de":["Comedy"],"en":["Comedy"]}` {
		t.Fatalf("genres %s", r.Genres)
	}
	if c := e.credits(t, idSeries); len(c) != 0 {
		t.Fatalf("TVDB people without TMDB ID stored: %+v", c)
	}
	if r := e.child(t, idSeason1); r.State != "matched" || r.Poster.String != art+"seasons/424242-1.jpg" {
		t.Fatalf("season %+v", r)
	}
	if tx := e.texts(t, idSeason1); tx["de"].Title != "Staffel 1" || tx["en"].Title != "Season 1" {
		t.Fatalf("season texts %+v", tx)
	}
	if r := e.child(t, idE0101); r.State != "matched" || r.Date.String != "2015-02-03" || r.Runtime.Int64 != 25 ||
		r.Backdrop.String != art+"episodes/424242/1.jpg" {
		t.Fatalf("episode 1 %+v", r)
	}
	if tx := e.texts(t, idE0102); tx["de"].Title != "Folge zwei" || tx["en"].Title != "Episode two" {
		t.Fatalf("episode 2 (second page) texts %+v", tx)
	}
	if e.tvdb.logins != 1 {
		t.Fatalf("%d logins, want 1 (the token is cached)", e.tvdb.logins)
	}
	// An expired token is renewed once.
	e.tvdb.expire()
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idE0102)
	e.svc.tvdb.mu.Lock()
	clear(e.svc.tvdb.episodes)
	e.svc.tvdb.mu.Unlock()
	if err := e.svc.match(ctx, idE0102); err != nil {
		t.Fatal(err)
	}
	if e.tvdb.logins != 2 || e.child(t, idE0102).State != "matched" {
		t.Fatalf("logins %d, episode %+v; want a second login", e.tvdb.logins, e.child(t, idE0102))
	}
}

func TestMatchSeriesManualTVDBSkipsTMDB(t *testing.T) {
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	e.addNode(t, node{id: idSeries, kind: "series", title: "Anything", tvdbID: 424242, locked: `["tmdb_id","tvdb_id","imdb_id"]`})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	s := e.child(t, idSeries)
	if s.State != "matched" || s.TVDB.Int64 != 424242 || s.TMDB.Valid || s.IMDB.Valid {
		t.Fatalf("series %+v", s)
	}
	if n := e.tmdb.total(); n != 0 {
		t.Fatalf("%d TMDB requests for a manual TVDB match", n)
	}
	if e.tvdb.count("/v4/search") != 0 || e.tvdb.count("/v4/series/424242/extended") != 1 {
		t.Fatal("expected the TVDB detail without a search")
	}
}

func TestMatchSeriesWithoutTVDBKeyIsUnmatched(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addNode(t, node{id: idSeries, kind: "series", title: "Only On TVDB", year: 2015})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idSeries); r.State != "unmatched" {
		t.Fatalf("series %+v", r)
	}
	if n := e.tvdb.total(); n != 0 {
		t.Fatalf("%d TVDB requests without key", n)
	}
}

func TestMatchSeriesRejectedTVDBKeyFails(t *testing.T) {
	e := newEnvWith(t, testKeys{tvdb: "wrong-key"})
	e.addNode(t, node{id: idSeries, kind: "series", title: "Only On TVDB", year: 2015})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatalf("rejected key must not be retried: %v", err)
	}
	if r := e.child(t, idSeries); r.State != "failed" || r.Err.String != "tvdb_key_invalid" {
		t.Fatalf("series %+v", r)
	}
	// Without any key the series is skipped.
	e2 := newEnv(t, "")
	e2.addNode(t, node{id: idSeries, kind: "series", title: "Test Show"})
	if err := e2.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if r := e2.child(t, idSeries); r.State != "skipped" || r.Err.String != "no_tmdb_key" {
		t.Fatalf("series without keys %+v", r)
	}
}

func TestQueuePendingMetadata(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "Pending movie"})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Movie with job"})
	e.addItem(t, itemSeed{id: idNoMatch, title: "Matched movie"})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id = ?`, idNoMatch)
	if err := e.svc.enqueueMatch(ctx, idByIMDB, matchPriority); err != nil {
		t.Fatal(err)
	}
	// Series 1 is pending with an episode; series 2 is matched with a pending episode.
	e.addNode(t, node{id: idSeries, kind: "series", title: "Pending show"})
	e.addNode(t, node{id: idSeason1, kind: "season", parent: idSeries, title: "Staffel 1", season: 1})
	e.addNode(t, node{id: idE0101, kind: "episode", parent: idSeason1, title: "Folge 1", season: 1, episode: 1})
	other := tid("SER", 2)
	e.addNode(t, node{id: other, kind: "series", title: "Matched show", state: "matched"})
	e.addNode(t, node{id: idE0201, kind: "episode", parent: other, title: "Folge 1", season: 1, episode: 1})

	n1, err := e.svc.QueuePendingMetadata(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := e.svc.QueuePendingMetadata(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 1 || n2 != 2 {
		t.Fatalf("queued %d and %d, want 1 and 2", n1, n2)
	}
	want := []string{"match:" + idByIMDB + " p4", "match:" + idByTMDB + " p4", "match:" + idE0201 + " p3", "match:" + idSeries + " p4"}
	if got := e.jobList(t, JobMatch); !slices.Equal(got, want) {
		t.Fatalf("match jobs %q, want %q", got, want)
	}
	// Active jobs are not queued twice.
	if n, err := e.svc.QueuePendingMetadata(ctx, 2); err != nil || n != 0 {
		t.Fatalf("second run queued %d, %v", n, err)
	}
}

func TestQueueIncompleteImagesCoversSeriesTree(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["backdrop"]' WHERE id = ?`, idE0102)
	n, err := e.svc.QueueIncompleteImages(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"image:" + idE0001 + " p1", "image:" + idE0101 + " p1", "image:" + idSeason0 + " p2",
		"image:" + idSeason1 + " p2", "image:" + idSeries + " p2"}
	if got := e.jobList(t, JobImageFetch); n != len(want) || !slices.Equal(got, want) {
		t.Fatalf("queued %d: %q, want %q", n, got, want)
	}
}

func TestImageFetchSeriesTree(t *testing.T) {
	requireFFmpeg(t)
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{idSeries, idSeason1, idE0101} {
		if err := e.svc.fetchImages(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	kinds := func(id string) (poster, backdrop int) {
		for key := range e.images(t, id) {
			switch {
			case strings.HasPrefix(key, "poster/"):
				poster++
			case strings.HasPrefix(key, "backdrop/"):
				backdrop++
			}
		}
		return poster, backdrop
	}
	for id, want := range map[string][2]int{idSeries: {7, 7}, idSeason1: {7, 0}, idE0101: {0, 7}} {
		if p, b := kinds(id); p != want[0] || b != want[1] {
			t.Errorf("%s: %d poster and %d backdrop rows, want %v", id, p, b, want)
		}
	}
	if src := e.images(t, idE0101)["backdrop/w780/webp"].Source; src != e.tmdb.srv.URL+"/t/p/original/still1-1.jpg" {
		t.Errorf("still source %q", src)
	}
	// TVDB artwork URLs are fetched as they are; another host is ignored.
	tvdbSeries := tid("SER", 3)
	art := e.tvdb.srv.URL + "/art/banners/posters/424242-2.jpg"
	e.addNode(t, node{id: tvdbSeries, kind: "series", title: "TVDB", state: "matched"})
	mustExec(t, e.db.Write, `UPDATE items SET tmdb_poster_path = ?, tmdb_backdrop_path = 'https://evil.example/x.jpg' WHERE id = ?`, art, tvdbSeries)
	if err := e.svc.fetchImages(ctx, tvdbSeries); err != nil {
		t.Fatal(err)
	}
	if p, b := kinds(tvdbSeries); p != 7 || b != 0 {
		t.Fatalf("TVDB series: %d poster and %d backdrop rows, want 7 and 0", p, b)
	}
}

func TestTVDBSearchCandidates(t *testing.T) {
	e := newEnvWith(t, testKeys{tvdb: tvdbKey, pin: tvdbPin})
	cs, err := e.svc.tvdb.SearchSeries(context.Background(), SeriesQuery{Title: "Only On TVDB"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].ID != "424242" || cs[0].Title != "Only on TVDB" || cs[0].OriginalTitle != "Nur bei TVDB" ||
		*cs[0].Year != 2015 || cs[0].PosterURL != e.tvdb.srv.URL+"/art/banners/posters/424242-1_t.jpg" {
		t.Fatalf("candidates %+v", cs)
	}
	if ms, err := e.svc.tvdb.SearchMovie(context.Background(), MovieQuery{Title: "Only On TVDB"}); err != nil || len(ms) != 0 {
		t.Fatalf("TVDB movie search %v, %v; want nothing", ms, err)
	}
}
