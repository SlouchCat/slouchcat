package api

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Concept 25 stage 5 (contract 1.41.0, build step 5.4): the pages of the wishes area with a fake
// Seerr. 404 without the right and with the pages off, an ItemRef only for a library title the
// profile sees, images only through Slouchcat's routes (the TMDB paths never reach the client), the
// genre backdrop taken from the cached genre row, 422 for bad parameters, 503 when Seerr does not
// answer and nothing is cached, stale passed through.

// discoverTitles serves the genre backdrops without TMDB and records the paths it was asked for.
type discoverTitles struct {
	contractTitles
	mu   sync.Mutex
	srcs []string
}

func (d *discoverTitles) OpenGenreBackdrop(_ context.Context, src string) (model.StreamSource, error) {
	d.mu.Lock()
	d.srcs = append(d.srcs, src)
	d.mu.Unlock()
	return model.StreamSource{File: readSeekNopCloser{bytes.NewReader(contractJPEG)}, MediaType: "image/jpeg",
		Size: int64(len(contractJPEG)), ModTime: time.Unix(1, 0), ETag: `"genre"`}, nil
}

func TestContractWishDiscover(t *testing.T) {
	c := newWishContract(t)
	f := c.f
	titles := &discoverTitles{contractTitles: c.titles}
	c.withTitles(titles)
	// Seerr's catalogue: the library film rated 16 (TMDB 1003), a stored missing film rated 12 and a
	// film Slouchcat does not know yet.
	c.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: tmdbLibrary16, Name: "Film 16", Year: 2013, GenreIDs: []int{28}, Popularity: 90})
	c.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: tmdbMissing12, Name: "Wunsch Zwoelf", Year: 2024, GenreIDs: []int{28}, Popularity: 80})
	c.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 9500, Name: "Unbekannt", Year: 2024, GenreIDs: []int{35}, Popularity: 70})
	c.exec(`INSERT INTO external_titles (media, tmdb_id, year, min_age, poster_path, fetched_at) VALUES ('movie', ?, 2013, 16, '/l16.jpg', ?)`,
		tmdbLibrary16, time.Now().UnixMilli())

	get := func(token, path string, status int, code string) map[string]any {
		t.Helper()
		out := c.expect(token, http.MethodGet, path, "", status, code)
		return out
	}
	noRaw := func(path string) {
		t.Helper()
		rec := send(t, c.h, http.MethodGet, BasePath+path, "p18", "", nil)
		for _, raw := range []string{"image.tmdb.org", ".jpg\"", "posterPath", "backdropPath"} {
			if strings.Contains(rec.Body.String(), raw) {
				t.Errorf("%s: a TMDB path in the answer (%s): %s", path, raw, rec.Body.String())
			}
		}
	}

	// Without the right: 404 on every page and the image (F-9).
	for _, path := range []string{"/wishes/discover", "/wishes/discover/trending", "/wishes/titles/movie", "/wishes/genres/movie",
		"/wishes/search?q=Film", "/wishes/genres/movie/28/backdrop"} {
		get("p6", path, http.StatusNotFound, "not_found")
	}

	// «Entdecken»: seven rows in their order; the library title as ItemRef for P18 only.
	disc := get("p18", "/wishes/discover", http.StatusOK, "")
	rows, _ := disc["rows"].([]any)
	keys := []string{}
	for _, r := range rows {
		keys = append(keys, r.(map[string]any)["key"].(string))
	}
	if strings.Join(keys, ",") != "trending,popular_movies,movie_genres,upcoming_movies,popular_shows,show_genres,upcoming_shows" {
		t.Fatalf("rows = %v", keys)
	}
	trending := func(token string) map[float64]map[string]any {
		t.Helper()
		out := map[float64]map[string]any{}
		d := get(token, "/wishes/discover/trending", http.StatusOK, "")
		for _, e := range d["entries"].([]any) {
			em := e.(map[string]any)
			out[em["title"].(map[string]any)["tmdbId"].(float64)] = em
		}
		return out
	}
	t18 := trending("p18")
	if item, _ := t18[tmdbLibrary16]["item"].(map[string]any); item == nil || item["id"] != f.Rated(16).ID {
		t.Errorf("p18 library entry = %v", t18[tmdbLibrary16])
	}
	missing := t18[tmdbMissing12]
	if missing == nil || missing["item"] != nil || missing["title"].(map[string]any)["canRequest"] != true {
		t.Errorf("p18 missing entry = %v", missing)
	}
	// Only the stored kinds have images, through the own route; an unknown title has none.
	poster, _ := missing["title"].(map[string]any)["images"].(map[string]any)["poster"].(map[string]any)
	if u, _ := poster["url"].(string); !strings.HasPrefix(u, BasePath+"/titles/movie/9012/images/poster/w400?v=") {
		t.Errorf("poster of a stored title = %v", poster)
	}
	if imgs := t18[9500]["title"].(map[string]any)["images"].(map[string]any); len(imgs) != 0 {
		t.Errorf("images of an unknown title = %v", imgs)
	}
	if _, ok := trending("p12")[tmdbLibrary16]; ok {
		t.Errorf("p12 sees the library title rated 16 in the discover list")
	}
	noRaw("/wishes/discover")
	noRaw("/wishes/discover/trending")

	// The genre tiles: the image through the own route; the server asks for the path of its cached
	// row, a genre the row does not list is 404.
	var tile map[string]any
	for _, r := range rows {
		if rm := r.(map[string]any); rm["key"] == "movie_genres" {
			for _, g := range rm["genres"].([]any) {
				if gm := g.(map[string]any); gm["id"] == float64(28) {
					tile = gm
				}
			}
		}
	}
	bd, _ := tile["backdrop"].(map[string]any)
	if u, _ := bd["url"].(string); !strings.HasPrefix(u, BasePath+"/wishes/genres/movie/28/backdrop") {
		t.Fatalf("genre tile = %v", tile)
	}
	rec := send(t, c.h, http.MethodGet, BasePath+"/wishes/genres/movie/28/backdrop", "p18", "", nil)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), contractJPEG) {
		t.Errorf("genre backdrop: %d", rec.Code)
	}
	if len(titles.srcs) != 1 || !strings.HasPrefix(titles.srcs[0], "/backdrop") {
		t.Errorf("backdrop paths asked = %v", titles.srcs)
	}
	get("p18", "/wishes/genres/movie/99/backdrop", http.StatusNotFound, "not_found")
	get("p18", "/wishes/genres/book/28/backdrop", http.StatusNotFound, "not_found")

	// «Filme», «Serien», the genres and the search; bad parameters are 422.
	get("p18", "/wishes/titles/movie?sort=rating&genre=28&hideAvailable=true", http.StatusOK, "")
	get("p18", "/wishes/titles/movie?sort=title", http.StatusUnprocessableEntity, "validation_failed")
	get("p18", "/wishes/discover/movie_genres", http.StatusUnprocessableEntity, "validation_failed")
	get("p18", "/wishes/discover/trending?page=501", http.StatusUnprocessableEntity, "validation_failed")
	get("p18", "/wishes/discover/nope", http.StatusNotFound, "not_found")
	if g := get("p18", "/wishes/genres/movie", http.StatusOK, ""); len(g["items"].([]any)) == 0 {
		t.Errorf("genres = %v", g)
	}
	get("p18", "/wishes/search?q=W", http.StatusUnprocessableEntity, "validation_failed")
	get("p18", "/wishes/search?q=Wunsch", http.StatusOK, "")

	// Stale: an hour-old cache while Seerr fails is served with stale; nothing cached is 503.
	c.exec(`UPDATE wish_discover_pages SET fetched_at = 1`)
	c.fake.Close()
	if d := get("p18", "/wishes/discover/trending", http.StatusOK, ""); d["stale"] != true {
		t.Errorf("stale page = %v", d["stale"])
	}
	get("p18", "/wishes/discover/trending?page=4", http.StatusServiceUnavailable, "seerr_unreachable")

	// The session says whether the pages show; the switch off makes them 404.
	if s := get("p18", "/auth/session", http.StatusOK, ""); s["wishes"].(map[string]any)["discover"] != true {
		t.Errorf("session wishes = %v", s["wishes"])
	}
	c.expect("p18", http.MethodPut, "/admin/wishes/settings", `{"discoverPages":false}`, http.StatusOK, "")
	get("p18", "/wishes/discover", http.StatusNotFound, "not_found")
	if s := get("p18", "/auth/session", http.StatusOK, ""); s["wishes"].(map[string]any)["discover"] != false {
		t.Errorf("session wishes with the pages off = %v", s["wishes"])
	}
}
