package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Library views of concept 08 (ADR 0016, 5.3): the side navigation (listVisibleLibraries), the
// library page with its K-3 negative test (listLibraryItems), all of Cat's recommendations
// (listPicks) and the stage's first title of getHome (heroStart, start).

// fixtureLibrary is the id of library 1 of the vistest fixture (the first library it creates).
const fixtureLibrary = 1

type libraryNavJSON struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Contents  []string `json:"contents"`
	ItemCount int      `json:"itemCount"`
}

type libraryPageJSON struct {
	Items []struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		Rating   *int   `json:"rating"`
		Progress *struct {
			Completed bool `json:"completed"`
		} `json:"progress"`
		HasTrailer *bool `json:"hasTrailer"`
	} `json:"items"`
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Facets struct {
		Letters []string `json:"letters"`
		Genres  []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"genres"`
		Ratings []int `json:"ratings"`
	} `json:"facets"`
}

func (p libraryPageJSON) ids() []string {
	out := make([]string, 0, len(p.Items))
	for _, it := range p.Items {
		out = append(out, it.ID)
	}
	return out
}

// topLevelTitles returns the ids of the top-level titles of a library that p may see (films,
// series, audiobooks, podcasts), sorted.
func topLevelTitles(f *vistest.Fixture, p model.Profile, libraryID int64) []string {
	var out []string
	for _, m := range f.All() {
		top := m.Kind == "" || m.Kind == "series" || m.Kind == "audiobook" || m.Kind == "podcast"
		if m.LibraryID == libraryID && top && f.VisibleTo(p, m) {
			out = append(out, m.ID)
		}
	}
	slices.Sort(out)
	return out
}

// checkVisibleLibraries checks an answer of listVisibleLibraries for p: exactly the libraries with
// a visible top-level title, in id order, each counting those titles.
func checkVisibleLibraries(t *testing.T, f *vistest.Fixture, p model.Profile, rec *httptest.ResponseRecorder) {
	t.Helper()
	var libs []libraryNavJSON
	decode(t, rec, &libs)
	var got, want []int64
	for _, l := range libs {
		got = append(got, l.ID)
		if n := len(topLevelTitles(f, p, l.ID)); l.ItemCount != n || len(l.Contents) == 0 {
			t.Errorf("listVisibleLibraries for %s: library %d = %+v, want %d titles and its contents", p.Name, l.ID, l, n)
		}
	}
	for _, id := range []int64{f.LibraryID(), f.OtherLibraryID(), f.Audio.AudiobookLibraryID, f.Audio.PodcastLibraryID} {
		if len(topLevelTitles(f, p, id)) > 0 {
			want = append(want, id)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("listVisibleLibraries for %s = %v, want %v", p.Name, got, want)
	}
}

// checkLibraryPage checks a page of library 1 (limit 100) for p: exactly its visible top-level
// titles, no rating facet above the ceiling.
func checkLibraryPage(t *testing.T, f *vistest.Fixture, p model.Profile, rec *httptest.ResponseRecorder) {
	t.Helper()
	var page libraryPageJSON
	decode(t, rec, &page)
	got := page.ids()
	slices.Sort(got)
	if want := topLevelTitles(f, p, f.LibraryID()); !slices.Equal(got, want) || page.Total != len(want) {
		t.Errorf("listLibraryItems for %s = %v (total %d), want %v", p.Name, got, page.Total, want)
	}
	ceiling := p.MaxRating
	if p.AccountID == f.Guest.ID {
		ceiling = min(ceiling, f.Guest.MaxRating)
	}
	for _, r := range page.Facets.Ratings {
		if r > ceiling {
			t.Errorf("listLibraryItems for %s: ratings facet %v above %d", p.Name, page.Facets.Ratings, ceiling)
		}
	}
}

// libraryPage fetches library 1 for token with query ("&…"), limit 100 unless query sets one.
func libraryPage(t *testing.T, h http.Handler, token, query string) libraryPageJSON {
	t.Helper()
	if !strings.Contains(query, "limit=") {
		query = "&limit=100" + query
	}
	rec := send(t, h, http.MethodGet, fmt.Sprintf("%s/libraries/%d/items?%s", BasePath, fixtureLibrary, strings.TrimPrefix(query, "&")), token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("library page %s for %s: status %d (%s)", query, token, rec.Code, rec.Body.String())
	}
	var page libraryPageJSON
	decode(t, rec, &page)
	return page
}

// K-3 (concept 08, 5.3): a child profile sees no title above its ceiling in a permitted library,
// maxRating 18 is clamped to the ceiling, the ratings facet stays below it; a library the profile
// may not see and one without a visible title are 404; letter without sort title and author
// outside an audiobook library are 422.
func TestContractLibraryItemsChildProfile(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	if f.LibraryID() != fixtureLibrary {
		t.Fatalf("library 1 of the fixture has id %d, want %d", f.LibraryID(), fixtureLibrary)
	}
	for _, token := range []string{"p0", "p6", "p12"} {
		p := pf.tokens[token]
		page := libraryPage(t, h, token, "")
		for _, it := range page.Items {
			if it.Rating == nil || *it.Rating > p.MaxRating {
				t.Errorf("%s sees %s rated %v above its ceiling %d", token, it.ID, it.Rating, p.MaxRating)
			}
			if it.HasTrailer != nil {
				t.Errorf("%s: %s carries hasTrailer on the library page", token, it.ID)
			}
		}
		clamped := libraryPage(t, h, token, "&maxRating=18")
		if !slices.Equal(clamped.ids(), page.ids()) || clamped.Total != page.Total {
			t.Errorf("%s maxRating=18 = %v, want the same as without %v", token, clamped.ids(), page.ids())
		}
		for _, r := range clamped.Facets.Ratings {
			if r > p.MaxRating {
				t.Errorf("%s ratings facet %v above its ceiling %d", token, clamped.Facets.Ratings, p.MaxRating)
			}
		}
		if p.MaxRating > 0 {
			lower := libraryPage(t, h, token, "&maxRating=0")
			if !slices.Equal(lower.ids(), []string{f.Rated(0).ID}) {
				t.Errorf("%s maxRating=0 = %v, want only the film rated 0", token, lower.ids())
			}
		}
	}
	if got := libraryPage(t, h, "p6", "").Facets.Ratings; !slices.Equal(got, []int{0, 6}) {
		t.Errorf("p6 ratings facet = %v, want 0 6", got)
	}
	if got := libraryPage(t, h, "p18", "").Facets.Ratings; !slices.Equal(got, []int{0, 6, 12, 16, 18}) {
		t.Errorf("p18 ratings facet = %v, want 0 6 12 16 18", got)
	}

	// 404 not_found, never 403: library X for px, a library without a title, an unknown one.
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (99, 'Leer', 'video', 1, 1)`)
	for _, tc := range []struct {
		token string
		lib   int64
	}{{"px", f.OtherLibraryID()}, {"p18", 99}, {"p18", 12345}} {
		rec := send(t, h, http.MethodGet, fmt.Sprintf("%s/libraries/%d/items", BasePath, tc.lib), tc.token, "", nil)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s library %d: status %d (%s), want 404 not_found", tc.token, tc.lib, rec.Code, rec.Body.String())
		}
	}
	if rec := send(t, h, http.MethodGet, fmt.Sprintf("%s/libraries/%d/items", BasePath, f.OtherLibraryID()), "p18", "", nil); rec.Code != http.StatusOK {
		t.Errorf("p18 library X: status %d, want 200", rec.Code)
	}

	// 422 validation_failed with the field and code of the contract.
	for query, want := range map[string]string{
		"letter=A&sort=added":                 "letter letter_requires_title_sort",
		"letter=a":                            "letter invalid_value",
		"sort=author":                         "sort invalid_value",
		"content=series":                      "content invalid_value",
		"yearFrom=1869":                       "yearFrom invalid_value",
		"yearTo=2101":                         "yearTo invalid_value",
		"maxRating=7":                         "maxRating invalid_value",
		"genre=" + strings.Repeat("g", 81):    "genre too_long",
		"sort=bogus":                          "sort invalid_value",
		"order=up":                            "order invalid_value",
		"status=seen":                         "status invalid_value",
		"letter=%23&sort=released&order=desc": "letter letter_requires_title_sort",
	} {
		rec := send(t, h, http.MethodGet, fmt.Sprintf("%s/libraries/%d/items?%s", BasePath, fixtureLibrary, query), "p18", "", nil)
		if got := fieldProblem(t, rec); got != want {
			t.Errorf("%s: %s, want %s", query, got, want)
		}
	}
	// Valid edges: letter # with the title sort, author in the audiobook library, limit clamped.
	if page := libraryPage(t, h, "p18", "&letter=%23"); page.Total != 0 || len(page.Facets.Letters) == 0 {
		t.Errorf("p18 letter # = %+v, want no title (every key starts with a letter) and the letters facet", page)
	}
	rec := send(t, h, http.MethodGet, fmt.Sprintf("%s/libraries/%d/items?sort=author", BasePath, f.Audio.AudiobookLibraryID), "p18", "", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("author in the audiobook library: status %d (%s)", rec.Code, rec.Body.String())
	}
	if page := libraryPage(t, h, "p18", "&limit=500&offset=-3"); page.Limit != 100 || page.Offset != 0 {
		t.Errorf("limit 500, offset -3: limit %d, offset %d; want 100 and 0", page.Limit, page.Offset)
	}
	if page := libraryPage(t, h, "p18", "&limit=2&offset=1"); len(page.Items) != 2 || page.Limit != 2 || page.Offset != 1 || page.Total != len(topLevelTitles(f, f.P18, f.LibraryID())) {
		t.Errorf("limit 2, offset 1 = %+v", page)
	}
	// The series is watched once every visible episode is completed (concept 08, 4.2).
	for _, e := range []vistest.Movie{f.Show.E1, f.Show.E3, f.Show.Special} {
		exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 0, 100, 1, 9000)`, f.P12.ID, e.ID)
	}
	watched := libraryPage(t, h, "p12", "&status=watched")
	if !slices.Equal(watched.ids(), []string{f.Show.Series.ID}) || watched.Items[0].Progress == nil || !watched.Items[0].Progress.Completed {
		t.Errorf("p12 watched = %+v, want the series with progress completed", watched.Items)
	}
}

// listVisibleLibraries: px lists library 1 only, with its top-level titles (the series once,
// never its episodes); listPicks equals the row picks in Cat's order, without the cut.
func TestContractVisibleLibrariesAndPicks(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	rec := send(t, h, http.MethodGet, BasePath+"/libraries/visible", "px", "", nil)
	var libs []libraryNavJSON
	decode(t, rec, &libs)
	if len(libs) != 1 || libs[0].ID != f.LibraryID() || libs[0].Name != "Filme" || libs[0].Kind != "video" ||
		!slices.Equal(libs[0].Contents, []string{"movie"}) || libs[0].ItemCount != 8 {
		t.Errorf("px libraries = %+v, want Filme (video, movie) with six films, the filler and the series", libs)
	}
	for _, token := range []string{"p0", "p18", "pg"} {
		checkVisibleLibraries(t, f, pf.tokens[token], send(t, h, http.MethodGet, BasePath+"/libraries/visible", token, "", nil))
	}

	for _, token := range []string{"p0", "p12", "p18"} {
		home := homePage(t, send(t, h, http.MethodGet, BasePath+"/home", token, "", nil))
		row := home.row("picks").Items
		var page struct {
			Items []struct {
				ID   string `json:"id"`
				Note string `json:"note"`
			} `json:"items"`
			Total, Offset, Limit int
		}
		decode(t, send(t, h, http.MethodGet, BasePath+"/picks", token, "", nil), &page)
		if len(page.Items) != len(row) || page.Total != len(row) || page.Limit != 50 || page.Offset != 0 {
			t.Errorf("%s picks page = %+v, want the row %v", token, page, row)
			continue
		}
		for i := range row {
			if page.Items[i].ID != row[i].ID || page.Items[i].Note != row[i].Note {
				t.Errorf("%s pick %d = %+v, want %+v", token, i, page.Items[i], row[i])
			}
		}
	}
}

// getHome (concept 08, F-2, F-10): with a stage, heroStart repeats a start inside the stage row
// and draws for one outside it; heroIntervalSeconds is the configured rhythm; without a stage both
// are absent.
func TestContractHomeHeroStart(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	// The trending films 1000..1004 (rated 0 to 18): five hits for p18, the default min_hits.
	for i := range 5 {
		if _, err := f.DB.Write.Exec(`INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`,
			i+1, 1000+i, time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	home := func(token, query string) (heroRow *string, start, interval *int) {
		t.Helper()
		var page struct {
			HeroRow             *string `json:"heroRow"`
			HeroStart           *int    `json:"heroStart"`
			HeroIntervalSeconds *int    `json:"heroIntervalSeconds"`
		}
		decode(t, send(t, h, http.MethodGet, BasePath+"/home"+query, token, "", nil), &page)
		return page.HeroRow, page.HeroStart, page.HeroIntervalSeconds
	}
	for k := range 5 {
		row, start, interval := home("p18", fmt.Sprintf("?start=%d", k))
		if row == nil || *row != "spotlight" || start == nil || *start != k || interval == nil || *interval != 5 {
			t.Errorf("start=%d: heroRow %v, heroStart %v, interval %v; want spotlight, %d, 5", k, row, start, interval, k)
		}
	}
	for _, query := range []string{"", "?start=5", "?start=99", "?start=-1"} {
		if _, start, _ := home("p18", query); start == nil || *start < 0 || *start >= 5 {
			t.Errorf("%q: heroStart %v, want a draw in [0, 5)", query, start)
		}
	}
	// p0 sees one hit, below min_hits: no stage, no heroStart, whatever start says.
	if row, start, interval := home("p0", "?start=0"); row != nil || start != nil || interval != nil {
		t.Errorf("p0 without stage: heroRow %v, heroStart %v, interval %v; want none", row, start, interval)
	}
}
