package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests of metadata.fill (ADR 0015, concept 4.4): the provider list of a title is walked front to
// back, the first provider with a value wins a field, item_field_sources records who, a stored
// source further front keeps the field, ratings are only raised, and AniList is a filler found
// through the mapping list of Fribb.

// fakeAniList serves the GraphQL endpoint, the mapping list and the image host from fixtures.
type fakeAniList struct {
	srv *httptest.Server

	mu       sync.Mutex
	hits     map[string]int
	ids      []int64 // the ids of every Media request, in order
	rateOnce bool    // answer the next Media request with 429 and Retry-After: 1
	tooLarge bool    // serve a list larger than maxAnimeListBytes
	adult    map[int64]bool
	list     []map[string]any
}

// animeList is the mapping list of the tests: the film 550 and the series 81189 share the
// number 550 at TMDB (a film and a tv entry); the series has two seasons, season 2 listed first;
// the film 777 is known by its IMDb id only.
func animeList() []map[string]any {
	return []map[string]any{
		{"type": "TV", "anidb_id": 2, "anilist_id": 2002, "tvdb_id": 81189, "themoviedb_id": map[string]any{"tv": 550},
			"season": map[string]any{"tvdb": 2, "tmdb": 2}, "imdb_id": []string{"tt0000900"}},
		{"type": "TV", "anidb_id": 1, "anilist_id": 2001, "tvdb_id": 81189, "themoviedb_id": map[string]any{"tv": 550},
			"season": map[string]any{"tvdb": 1, "tmdb": 1}, "imdb_id": []string{"tt0000900"}},
		{"type": "MOVIE", "anidb_id": 3, "anilist_id": 1001, "themoviedb_id": map[string]any{"movie": []int{550, 551}}, "imdb_id": []string{"tt0000550"}},
		{"type": "MOVIE", "anidb_id": 4, "anilist_id": 1002, "imdb_id": "tt0000601"},
		{"type": "TV", "anidb_id": 5, "anilist_id": 2003, "tvdb_id": 424242, "season": map[string]any{"tvdb": 1}},
		{"anidb_id": 6, "tvdb_id": 555}, // no anilist id: nothing for this package
		{"type": "MOVIE", "anidb_id": 7, "anilist_id": 1003, "themoviedb_id": map[string]any{"movie": "not a number"}, "tvdb_id": "77"}, // another shape: skipped
	}
}

func newFakeAniList(t *testing.T) *fakeAniList {
	t.Helper()
	f := &fakeAniList{hits: map[string]int{}, adult: map[int64]bool{2001: true}, list: animeList()}
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query     string           `json:"query"`
			Variables map[string]int64 `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !strings.Contains(body.Query, "Media(id: $id") {
			t.Errorf("anilist request: %v %q", err, body.Query)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id := body.Variables["id"]
		f.mu.Lock()
		f.hits["/graphql"]++
		f.ids = append(f.ids, id)
		rate := f.rateOnce
		f.rateOnce = false
		adult := f.adult[id]
		f.mu.Unlock()
		if rate {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"errors": []any{map[string]any{"message": "Too Many Requests.", "status": 429}}})
			return
		}
		media := map[int64]map[string]any{
			1001: {"id": 1001, "title": map[string]any{"romaji": "Anime Eiga", "english": "Anime Film", "native": "アニメ映画"},
				"description": "<p>An <i>anime</i> film.<br>Second line &amp; more.</p>", "genres": []string{"Action", "Fantasy"},
				"coverImage": map[string]any{"extraLarge": f.srv.URL + "/img/cover1001.jpg"}, "bannerImage": f.srv.URL + "/img/banner1001.jpg"},
			1002: {"id": 1002, "title": map[string]any{"romaji": "Only Romaji", "english": nil, "native": "ローマ字"},
				"description": "By IMDb id.", "genres": []string{"Drama"}, "coverImage": map[string]any{"extraLarge": nil}, "bannerImage": nil},
			2001: {"id": 2001, "title": map[string]any{"romaji": "Anime Bangumi", "english": "Anime Show", "native": "アニメ番組"},
				"description": "Season one.", "genres": []string{"Action"},
				"coverImage": map[string]any{"extraLarge": f.srv.URL + "/img/cover2001.jpg"}, "bannerImage": "https://elsewhere.test/banner.jpg"},
			2002: {"id": 2002, "title": map[string]any{"romaji": "Anime Bangumi 2", "english": "Anime Show 2", "native": "アニメ番組2"},
				"description": "Season two.", "genres": []string{}, "coverImage": map[string]any{}, "bannerImage": nil},
		}
		m, ok := media[id]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"errors": []any{map[string]any{"message": "Not Found.", "status": 404}}, "data": map[string]any{"Media": nil}})
			return
		}
		m["isAdult"] = adult
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"Media": m}})
	})
	mux.HandleFunc("GET /lists/anime-list-full.json", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits["/lists"]++
		tooLarge := f.tooLarge
		list := f.list
		f.mu.Unlock()
		if tooLarge {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			chunk := make([]byte, 1<<20)
			for i := range chunk {
				chunk[i] = ' '
			}
			for written := 0; written <= maxAnimeListBytes; written += len(chunk) {
				if _, err := w.Write(chunk); err != nil {
					return
				}
			}
			return
		}
		if r.Header.Get("If-None-Match") == `"list-v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"list-v1"`)
		writeJSON(w, http.StatusOK, list)
	})
	image := jpegBytes(t, 300, 450)
	mux.HandleFunc("GET /img/{name}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits[r.URL.Path]++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(image)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAniList) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

// sources returns the item_field_sources of an item as "field/lang" → provider.
func (e *env) sources(t *testing.T, id string) map[string]string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT field, lang, provider FROM item_field_sources WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var field, lang, provider string
		if err := rows.Scan(&field, &lang, &provider); err != nil {
			t.Fatal(err)
		}
		out[field+"/"+lang] = provider
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// snapshot renders the texts, filled columns and sources of an item for equality checks (not
// the provider ids: those belong to the identifier, and a match rewrites them).
func (e *env) snapshot(t *testing.T, id string) string {
	t.Helper()
	r := e.child(t, id)
	return fmt.Sprintf("%+v %v %v %v %v %v %v %v %v %v", e.texts(t, id), r.Date, r.Poster, r.Backdrop, r.Runtime, r.Rating, r.Source,
		r.Original, e.item(t, id).Genres, e.sources(t, id))
}

func (e *env) fillJobs(t *testing.T) []string { return e.activeJobs(t, JobFill) }

// TestFillSeriesTMDBThenTVDB: with ["tmdb","tvdb"] the German overview TMDB lacks comes from
// TVDB, the English one TMDB has stays TMDB's, the sources say who; the match queued the fill
// in its transaction; two runs give the same result; the seasons follow.
func TestFillSeriesTMDBThenTVDB(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.tmdb.mu.Lock()
	e.tmdb.tvNoGermanOverview = true
	e.tmdb.mu.Unlock()
	e.addTree(t, node{title: "Test Show", year: 2010})
	e.setProviders(t, `["tmdb","tvdb"]`)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if got := e.fillJobs(t); !slices.Equal(got, []string{"fill:" + idSeries + " p2"}) {
		t.Fatalf("fill jobs after the match = %v", got)
	}
	// The match stored TVDB's id from TMDB's external ids (81189); the fake TVDB knows 424242.
	mustExec(t, e.db.Write, `UPDATE items SET tvdb_id = 424242 WHERE id = ?`, idSeries)
	before := e.texts(t, idSeries)
	if before["de"].Overview.String != "English series overview" {
		t.Fatalf("match wrote de overview %q, want the English fallback", before["de"].Overview.String)
	}
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	tx := e.texts(t, idSeries)
	if tx["de"].Overview.String != "Deutsche Beschreibung" || tx["en"].Overview.String != "English series overview" {
		t.Fatalf("texts after the fill %+v", tx)
	}
	if tx["de"].Title != "Testserie" || tx["en"].Title != "Test Show" {
		t.Fatalf("titles changed: %+v", tx)
	}
	src := e.sources(t, idSeries)
	want := map[string]string{"overview/de": "tvdb", "overview/en": "tmdb", "title/de": "tmdb", "title/en": "tmdb",
		"original_title/": "tmdb", "release_date/": "tmdb", "runtime_minutes/": "tmdb", "poster/": "tmdb", "backdrop/": "tmdb",
		"genres/": "union", "own_rating/": "tmdb"}
	for k, v := range want {
		if src[k] != v {
			t.Errorf("source %s = %q, want %q (all: %v)", k, src[k], v, src)
		}
	}
	r := e.child(t, idSeries)
	if r.Rating.Int64 != 12 || r.Source.String != "tmdb:DE" {
		t.Fatalf("rating %+v, want 12 from tmdb:DE", r)
	}
	if g := e.item(t, idSeries).Genres; g != `{"de":["Drama","Comedy"],"en":["Drama","Comedy"]}` {
		t.Fatalf("genres %s, want the union of TMDB and TVDB", g)
	}
	// Seasons: TMDB described episode 2 and 3 as one file (title locked by the match), TVDB has
	// no texts of its own for the season; episode 1 keeps TMDB's title and overview.
	if ep := e.texts(t, idE0101); ep["de"].Title != "Der Anfang" || ep["de"].Overview.String != "Overview de-DE" {
		t.Fatalf("episode 1 texts %+v", ep)
	}
	if ep := e.texts(t, idE0102); !strings.HasPrefix(ep["de"].Title, "E02–E03:") {
		t.Fatalf("multi-episode title rewritten: %+v", ep)
	}
	if s := e.sources(t, idE0101); s["title/de"] != "tmdb" || s["backdrop/"] != "tmdb" {
		t.Fatalf("episode 1 sources %v", s)
	}
	snap := e.snapshot(t, idSeries)
	tmdbCalls, tvdbCalls := e.tmdb.total(), e.tvdb.total()
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if again := e.snapshot(t, idSeries); again != snap {
		t.Fatalf("second run differs:\n%s\n%s", snap, again)
	}
	if e.tmdb.total() <= tmdbCalls || e.tvdb.total() <= tvdbCalls {
		t.Fatalf("second run made no requests (%d/%d → %d/%d)", tmdbCalls, tvdbCalls, e.tmdb.total(), e.tvdb.total())
	}
}

// TestFillTVDBFirstKeepsTVDBTextsUntilTheListChanges: with ["tvdb","tmdb"] TMDB does not replace
// a TVDB overview; after the list becomes ["tmdb","tvdb"] the next run replaces it, and a run
// after another match gives the same fields again.
func TestFillTVDBFirstKeepsTVDBTextsUntilTheListChanges(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addNode(t, node{id: idSeries, kind: "series", title: "Only On TVDB", year: 2015})
	e.addNode(t, node{id: idSeason1, kind: "season", parent: idSeries, title: "Staffel 1", season: 1})
	e.addNode(t, node{id: idE0101, kind: "episode", parent: idSeason1, title: "Folge 1", season: 1, episode: 1})
	e.addNode(t, node{id: idE0102, kind: "episode", parent: idSeason1, title: "Folge 2", season: 1, episode: 2})
	e.setProviders(t, `["tvdb","tmdb"]`)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	tx := e.texts(t, idSeries)
	if tx["de"].Overview.String != "Deutsche Beschreibung" || tx["de"].Title != "Nur bei TVDB" {
		t.Fatalf("TVDB first: texts %+v", tx)
	}
	src := e.sources(t, idSeries)
	if src["overview/de"] != "tvdb" || src["title/de"] != "tvdb" || src["genres/"] != "union" {
		t.Fatalf("sources %v", src)
	}
	// TVDB has no overview for episode 2: TMDB fills it, the title stays TVDB's.
	if ep := e.texts(t, idE0102); ep["en"].Title != "Episode two" || ep["en"].Overview.String != "Overview en-US" {
		t.Fatalf("episode 2 texts %+v", ep)
	}
	if s := e.sources(t, idE0102); s["title/en"] != "tvdb" || s["overview/en"] != "tmdb" {
		t.Fatalf("episode 2 sources %v", s)
	}
	// A list change: TMDB moves to the front, its texts replace TVDB's on the next run, twice.
	e.setProviders(t, `["tmdb","tvdb"]`)
	for range 2 {
		if err := e.svc.fill(ctx, idSeries); err != nil {
			t.Fatal(err)
		}
		tx = e.texts(t, idSeries)
		if tx["de"].Overview.String != "Deutsche Serienbeschreibung" || tx["de"].Title != "Testserie" {
			t.Fatalf("after the list change: texts %+v", tx)
		}
		if src = e.sources(t, idSeries); src["overview/de"] != "tmdb" {
			t.Fatalf("after the list change: sources %v", src)
		}
	}
	// Another match (the identifier TMDB writes its values) and another fill: the same fields.
	snap := e.snapshot(t, idSeries)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE items SET tvdb_id = 424242 WHERE id = ?`, idSeries) // the match took 81189 from TMDB
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if again := e.snapshot(t, idSeries); again != snap {
		t.Fatalf("after match and fill the fields differ:\n%s\n%s", snap, again)
	}
}

// TestFillKeepsLockedFieldsAndNeverLowersTheRating: a locked overview stays; AniList's isAdult
// raises the rating to 18 over TMDB's 12; a rating already higher than what the providers say
// is never lowered, whatever their order (K-3).
func TestFillKeepsLockedFieldsAndNeverLowersTheRating(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550, locked: `["overview"]`})
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idByTMDB); r.Rating.Int64 != 12 {
		t.Fatalf("match rating %+v, want 12", r)
	}
	mustExec(t, e.db.Write, `UPDATE item_texts SET overview = 'Von Hand' WHERE item_id = ? AND lang = 'de'`, idByTMDB)
	e.al.mu.Lock()
	e.al.adult[1001] = true
	e.al.mu.Unlock()
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	r := e.child(t, idByTMDB)
	if r.Rating.Int64 != 18 || r.Source.String != "anilist" {
		t.Fatalf("rating %+v, want 18 from anilist", r)
	}
	if r.Original.String != "First Film (Original)" {
		t.Fatalf("original title %q: TMDB stands first and has one", r.Original.String)
	}
	tx := e.texts(t, idByTMDB)
	if tx["de"].Overview.String != "Von Hand" {
		t.Fatalf("locked overview rewritten: %+v", tx)
	}
	src := e.sources(t, idByTMDB)
	if _, ok := src["overview/de"]; ok {
		t.Fatalf("locked field got a source: %v", src)
	}
	if src["own_rating/"] != "anilist" || src["original_title/"] != "tmdb" || src["title/en"] != "tmdb" {
		t.Fatalf("sources %v", src)
	}
	if it := e.item(t, idByTMDB); it.Genres != `{"de":["Drama","Komödie"],"en":["Drama","Comedy","Action","Fantasy"]}` {
		t.Fatalf("genres %s, want the union with AniList's", it.Genres)
	}
	// AniList says adult no longer, TMDB still says 12: the 18 stays, the source stays.
	e.al.mu.Lock()
	e.al.adult[1001] = false
	e.al.mu.Unlock()
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idByTMDB); r.Rating.Int64 != 18 || r.Source.String != "anilist" {
		t.Fatalf("rating lowered: %+v", r)
	}
	// A rating from a stricter provider than the one in front: the order does not matter either.
	mustExec(t, e.db.Write, `UPDATE items SET own_rating = 16, rating_source = 'manual' WHERE id = ?`, idByTMDB)
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idByTMDB); r.Rating.Int64 != 16 || r.Source.String != "manual" {
		t.Fatalf("manual rating touched: %+v", r)
	}
}

// TestFillAniListThroughTheMappingList: a film finds its AniList entry by its TMDB id, a series
// by its TVDB id with season 1 preferred, a film and a series with the same TMDB number land at
// different entries, a film without TMDB entry by its IMDb id; no entry writes anilist_id 0; the
// AniList image becomes the poster and image.fetch downloads it; the list is fetched once and
// renewed with its ETag.
func TestFillAniListThroughTheMappingList(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Searched", tmdbID: 601}) // TMDB names its IMDb id tt0000601
	e.addItem(t, itemSeed{id: idNoMatch, title: "Unrated", tmdbID: 602})
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE library_id = 1`)
	e.addTree(t, node{title: "Test Show", year: 2010})
	e.setProviders(t, `["tmdb","tvdb","anilist"]`)
	for _, id := range []string{idByTMDB, idByIMDB, idNoMatch, idSeries} {
		if err := e.svc.match(ctx, id); err != nil {
			t.Fatal(id, err)
		}
		if err := e.svc.fill(ctx, id); err != nil {
			t.Fatal(id, err)
		}
	}
	for id, want := range map[string]int64{idByTMDB: 1001, idByIMDB: 1002, idNoMatch: 0, idSeries: 2001} {
		var got int64
		if err := e.db.Read.QueryRow(`SELECT anilist_id FROM items WHERE id = ?`, id).Scan(&got); err != nil || got != want {
			t.Errorf("anilist_id of %s = %d (%v), want %d", id, got, err, want)
		}
	}
	e.al.mu.Lock()
	ids := slices.Clone(e.al.ids)
	e.al.mu.Unlock()
	if !slices.Equal(ids, []int64{1001, 1002, 2001}) {
		t.Fatalf("AniList asked for %v, want 1001, 1002 and 2001 (season 1, not 2002)", ids)
	}
	// The film: TMDB has every text, AniList adds the native title? No: TMDB's original title
	// stands first. The series: the German fields stay TMDB's, the native title is AniList's
	// only where TMDB has none.
	r := e.child(t, idByTMDB)
	if r.Original.String != "First Film (Original)" || r.Poster.String != "/poster550.jpg" {
		t.Fatalf("film %+v: TMDB stands first", r)
	}
	src := e.sources(t, idByTMDB)
	if src["original_title/"] != "tmdb" || src["poster/"] != "tmdb" {
		t.Fatalf("film sources %v", src)
	}
	// The film 601 has no backdrop at TMDB and no German overview: AniList fills neither (no
	// banner, no German), but its English texts stand behind TMDB's.
	if tx := e.texts(t, idByIMDB); tx["en"].Overview.String != "English overview of Searched Film" || tx["de"].Overview.String != "English overview of Searched Film" {
		t.Fatalf("film 601 texts %+v", tx)
	}
	// The film 602 has no images at TMDB and no AniList entry: nothing to fill.
	if r := e.child(t, idNoMatch); r.Poster.Valid {
		t.Fatalf("film 602 got a poster %+v", r)
	}
	// The series: TMDB has no backdrop for AniList to beat, but film 550's AniList cover is a
	// poster only where TMDB has none. Give the series a provider without poster to see AniList's.
	mustExec(t, e.db.Write, `UPDATE items SET tmdb_poster_path = NULL, tmdb_backdrop_path = NULL WHERE id = ?`, idSeries)
	mustExec(t, e.db.Write, `DELETE FROM item_field_sources WHERE item_id = ? AND field IN ('poster', 'backdrop')`, idSeries)
	e.tmdb.mu.Lock()
	e.tmdb.tvNoGermanOverview = true
	e.tmdb.mu.Unlock()
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["poster","backdrop"]' WHERE id = ?`, idSeries)
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idSeries); r.Poster.Valid {
		t.Fatalf("locked poster filled: %+v", r)
	}
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '[]' WHERE id = ?`, idSeries)
	// Without TMDB's poster the fill would take TMDB's again (it stands first); make the series
	// one TMDB does not know an image for by pointing it at the film's AniList entry instead.
	mustExec(t, e.db.Write, `UPDATE items SET anilist_id = 1001, tmdb_id = 602 WHERE id = ?`, idSeries)
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	r = e.child(t, idSeries)
	wantPoster := e.al.srv.URL + "/img/cover1001.jpg"
	if r.Poster.String != wantPoster || r.Backdrop.String != e.al.srv.URL+"/img/banner1001.jpg" {
		t.Fatalf("series images %+v, want AniList's cover and banner", r)
	}
	if src := e.sources(t, idSeries); src["poster/"] != "anilist" || src["backdrop/"] != "anilist" {
		t.Fatalf("series image sources %v", src)
	}
	if jobs := e.imageJobs(t); !slices.Contains(jobs, "image:"+idSeries+" p2 "+`{"itemId":"`+idSeries+`"}`) {
		t.Fatalf("image.fetch not queued after the poster changed: %v", jobs)
	}
	if lookFFmpeg() != "" {
		if err := e.svc.fetchImages(ctx, idSeries); err != nil {
			t.Fatal(err)
		}
		if n := e.al.count("/img/cover1001.jpg"); n != 1 {
			t.Fatalf("AniList cover downloaded %d times, want 1", n)
		}
	}
	// The list was fetched once for every lookup of this test and is renewed with its ETag.
	if n := e.al.count("/lists"); n != 1 {
		t.Fatalf("list fetched %d times, want 1", n)
	}
	// Workers that ask at once after a scan share one load (a fresh service, eight lookups).
	e2 := newEnv(t, goodToken)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e2.svc.animeLists.MovieAniListID(ctx, 550, "")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := e2.al.count("/lists"); n != 1 {
		t.Fatalf("eight workers fetched the list %d times, want 1", n)
	}
	e.svc.animeLists.mu.Lock()
	e.svc.animeLists.loadedAt = testNow.Add(-animeListsTTL)
	e.svc.animeLists.mu.Unlock()
	e.svc.animeLists.now = func() time.Time { return time.Now().Add(animeListsTTL + time.Hour) }
	if id, err := e.svc.animeLists.MovieAniListID(ctx, 550, ""); err != nil || id != 1001 {
		t.Fatalf("after the TTL: %d, %v", id, err)
	}
	if n := e.al.count("/lists"); n != 2 {
		t.Fatalf("list not renewed after the TTL: %d requests", n)
	}
	if !strings.Contains(e.logs.String(), "anime list loaded") {
		t.Fatalf("no log line for the list:\n%s", e.logs.String())
	}
}

// TestFillSingleProviderRecordsTheIdentifier: a list with one entry makes no request and marks
// every field the title carries as the identifier's, so a later longer list knows what it may
// replace; QueuePendingMetadata leaves such titles alone.
func TestFillSingleProviderRecordsTheIdentifier(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	calls := e.tmdb.total()
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if e.tmdb.total() != calls {
		t.Fatalf("a single-provider fill made %d requests", e.tmdb.total()-calls)
	}
	src := e.sources(t, idByTMDB)
	for _, k := range []string{"title/de", "title/en", "overview/de", "overview/en", "tagline/en", "original_title/", "release_date/",
		"runtime_minutes/", "genres/", "poster/", "backdrop/", "own_rating/"} {
		if src[k] != "tmdb" {
			t.Errorf("source %s = %q, want tmdb (%v)", k, src[k], src)
		}
	}
	if _, ok := src["tagline/de"]; !ok {
		// The German text fell back to the English tagline: it is a value the title carries.
		t.Errorf("tagline/de without source: %v", src)
	}
	if n, err := e.svc.QueuePendingMetadata(ctx, 1); err != nil || n != 0 {
		t.Fatalf("queued %d jobs (%v) for a title with one provider", n, err)
	}
}

// TestQueuePendingMetadataQueuesStaleFills: after a scan, matched titles with more than one
// provider and no provenance, or with a provenance naming a provider that left the list, get a
// fill; titles whose provenance fits do not; an active fill job keeps the key.
func TestQueuePendingMetadataQueuesStaleFills(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	for _, id := range []string{idByTMDB, idByIMDB, idNoMatch} {
		e.addItem(t, itemSeed{id: id, title: "First", tmdbID: 550})
		mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched', providers = '["tmdb","anilist"]' WHERE id = ?`, id)
	}
	mustExec(t, e.db.Write, `INSERT INTO item_field_sources (item_id, field, lang, provider) VALUES (?, 'title', 'de', 'tmdb'), (?, 'genres', '', 'union')`, idByIMDB, idByIMDB)
	mustExec(t, e.db.Write, `INSERT INTO item_field_sources (item_id, field, lang, provider) VALUES (?, 'overview', 'de', 'tvdb')`, idNoMatch)
	n, err := e.svc.QueuePendingMetadata(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.fillJobs(t); !slices.Equal(got, []string{"fill:" + idByTMDB + " p2", "fill:" + idNoMatch + " p2"}) {
		t.Fatalf("fill jobs %v (queued %d)", got, n)
	}
	if n2, err := e.svc.QueuePendingMetadata(ctx, 1); err != nil || n2 != 0 {
		t.Fatalf("second run queued %d (%v), want 0: the active jobs keep their key", n2, err)
	}
	if got := e.fillJobs(t); len(got) != 2 {
		t.Fatalf("fill jobs after the second run %v", got)
	}
}

// TestFillSkipsAProviderWithoutKeyAndWaitsForTheMatch: a TVDB entry without key is skipped and
// logged; a fill that finds the match job of its title still active ends with an error the
// queue retries.
func TestFillSkipsAProviderWithoutKeyAndWaitsForTheMatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken) // no TVDB key
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", year: 2010})
	e.setProviders(t, `["tmdb","tvdb"]`)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.logs.String(), "not configured, skipped") || !strings.Contains(e.logs.String(), "provider=tvdb") {
		t.Fatalf("no warning for the TVDB entry without key:\n%s", e.logs.String())
	}
	if src := e.sources(t, idSeries); src["overview/de"] != "tmdb" {
		t.Fatalf("sources %v", src)
	}
	if n := e.tvdb.total(); n != 0 {
		t.Fatalf("%d TVDB requests without key", n)
	}
	if err := e.svc.enqueueMatch(ctx, idSeries, matchPriority); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fill(ctx, idSeries); !errors.Is(err, errFillWaitsForMatch) {
		t.Fatalf("fill with an active match job: %v", err)
	}
}

// TestFillAniListRateLimitAndListSize: a 429 with Retry-After pauses the limiter and the request
// is repeated after the pause; a mapping list larger than the brake ends the run with an error.
func TestFillAniListRateLimitAndListSize(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.al.mu.Lock()
	e.al.rateOnce = true
	e.al.mu.Unlock()
	start := time.Now()
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if n := e.al.count("/graphql"); n != 2 {
		t.Fatalf("%d AniList requests, want the refused one and its retry", n)
	}
	if d := time.Since(start); d < time.Second {
		t.Fatalf("retry after %v, want the second of Retry-After", d)
	}
	if tx := e.texts(t, idByTMDB); tx["en"].Overview.String != "English overview of First Film" {
		t.Fatalf("texts %+v", tx)
	}

	// The list: a fresh service with a list beyond the brake.
	e2 := newEnv(t, goodToken)
	e2.al.mu.Lock()
	e2.al.tooLarge = true
	e2.al.mu.Unlock()
	e2.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	mustExec(t, e2.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE id = ?`, idByTMDB)
	if err := e2.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e2.svc.fill(ctx, idByTMDB); !errors.Is(err, errAnimeListTooLarge) {
		t.Fatalf("fill with a list beyond the brake: %v", err)
	}
	if n := e2.al.count("/graphql"); n != 0 {
		t.Fatalf("%d AniList requests without a list", n)
	}
}

func TestParseAnimeListAndStripHTML(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(animeList())
	if err != nil {
		t.Fatal(err)
	}
	movies, imdb, series, err := parseAnimeList(strings.NewReader(string(raw)))
	var skipped errAnimeListEntriesSkipped
	if !errors.As(err, &skipped) || skipped != 1 {
		t.Fatalf("parse: %v, want one skipped entry of an unexpected shape", err)
	}
	if movies[550] != 1001 || movies[551] != 1001 || len(movies) != 2 {
		t.Fatalf("movies by tmdb %v", movies)
	}
	if imdb["tt0000550"] != 1001 || imdb["tt0000601"] != 1002 || imdb["tt0000900"] != 0 {
		t.Fatalf("movies by imdb %v: series ids must not be listed", imdb)
	}
	if got := series[81189]; len(got) != 2 || got[0].AniListID != 2002 || got[1].Season != 1 {
		t.Fatalf("series 81189 %v", got)
	}
	if _, ok := series[555]; ok {
		t.Fatalf("an entry without anilist id was kept: %v", series[555])
	}
	if _, _, _, err := parseAnimeList(strings.NewReader(`{"not": "a list"}`)); err == nil {
		t.Fatal("an object parsed as a list")
	}
	if _, _, _, err := parseAnimeList(io.LimitReader(strings.NewReader(string(raw)), 40)); err == nil {
		t.Fatal("a truncated list parsed")
	}
	if got := stripHTML("<p>An <i>anime</i> film.<br>Second line &amp; more.<br/><br />Third.</p>"); got != "An anime film.\nSecond line & more.\n\nThird." {
		t.Fatalf("stripHTML = %q", got)
	}
}
