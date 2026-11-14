package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// myMediaEntryJSON is one entry of listMyMedia.
type myMediaEntryJSON struct {
	ID      string          `json:"id"`
	Key     string          `json:"key"`
	Library *libraryNavJSON `json:"library"`
	Count   int             `json:"count"`
}

// myMediaTileJSON is one tile of the start page row «Meine Medien» (HomeRow.media).
type myMediaTileJSON struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Count   int    `json:"count"`
	Library *struct {
		ID        int64 `json:"id"`
		ItemCount int   `json:"itemCount"`
	} `json:"library"`
	Stations []struct {
		ID int64 `json:"id"`
	} `json:"stations"`
}

// checkMyMedia checks an answer of listMyMedia in the stored default order of the fixture: radio,
// collections and picks (migration 00059 on an empty database), then the libraries the fixture
// created afterwards, those with a visible top-level title for the profile, in id order, each
// counting those titles; radio counts the profile's favourites of listRadioFavorites.
func checkMyMedia(t *testing.T, f *vistest.Fixture, h http.Handler, token string, rec *httptest.ResponseRecorder) {
	t.Helper()
	p := fixtureProfile(f, token)
	var entries []myMediaEntryJSON
	decode(t, rec, &entries)
	// Libraries created after migration 00059 come behind the last library, in front of radio,
	// collections and picks (O-20).
	var want []string
	for _, id := range []int64{f.LibraryID(), f.OtherLibraryID(), f.Audio.AudiobookLibraryID, f.Audio.PodcastLibraryID} {
		if len(topLevelTitles(f, p, id)) > 0 {
			want = append(want, fmt.Sprintf("library:%d", id))
		}
	}
	want = append(want, "radio", "collections", "picks")
	var got []string
	for _, e := range entries {
		got = append(got, e.ID)
		if e.Key == "library" {
			if e.Library == nil || e.ID != fmt.Sprintf("library:%d", e.Library.ID) || e.Count != len(topLevelTitles(f, p, e.Library.ID)) ||
				e.Library.ItemCount != e.Count {
				t.Errorf("listMyMedia for %s: library entry %+v, want its visible titles", token, e)
			}
		} else if e.Library != nil {
			t.Errorf("listMyMedia for %s: entry %+v carries a library", token, e)
		}
		if e.Key == "radio" {
			var favorites []struct{}
			if fav := send(t, h, http.MethodGet, BasePath+"/radio/favorites", token, "", nil); fav.Code == http.StatusOK {
				decode(t, fav, &favorites)
			}
			if e.Count != len(favorites) {
				t.Errorf("listMyMedia for %s: radio counts %d, want the %d favourites", token, e.Count, len(favorites))
			}
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("listMyMedia for %s = %v, want %v", token, got, want)
	}
}

// Concept 21 P-9, P-11 over HTTP: getMyMedia lists every entry with its flags and counts,
// setMyMedia stores the order by id with the hiding per place (O-21), and the side navigation
// (listMyMedia) and the start page row «Meine Medien» (HomeRow.media and libraries) follow it. A
// library created afterwards is appended visible (O-20) and keeps its place when a list leaves it
// out; a wrong list is 422 rows invalid_value.
func TestContractMyMedia(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	get := func() []gen.MyMediaSetting {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/admin/my-media", "p18", "", nil)
		adminCurationAnswer(t, rec, http.StatusOK)
		var out []gen.MyMediaSetting
		decode(t, rec, &out)
		return out
	}
	ids := func(entries []gen.MyMediaSetting) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Id)
		}
		return out
	}
	stored := get()
	lib := fmt.Sprintf("library:%d", pf.f.LibraryID())
	if n := len(stored); n < 4 || !slices.Equal(ids(stored)[n-3:], []string{"radio", "collections", "picks"}) || !slices.Contains(ids(stored), lib) {
		t.Fatalf("default entries %v", ids(stored))
	}
	for i, e := range stored {
		if e.Position != i || e.HideSidebar || e.HideHome != (e.Key == "collections" || e.Key == "picks") {
			t.Errorf("default entry %+v at %d", e, i)
		}
		if e.Key == "library" && (e.LibraryId == nil || e.LibraryName == nil || e.LibraryKind == nil || *e.LibraryName == "") {
			t.Errorf("library entry %+v without its library", e)
		}
	}
	if films := stored[slices.Index(ids(stored), lib)]; films.Count == 0 {
		t.Errorf("library 1 counts %d titles", films.Count)
	}

	// The library first, then radio, collections, picks; the library not in the side navigation,
	// radio not on the start page, collections on the start page too.
	order := []string{lib, "radio", "collections", "picks"}
	parts := make([]string, 0, len(order))
	for i, id := range order {
		key := id
		if id == lib {
			key = "library"
		}
		parts = append(parts, fmt.Sprintf(`{"id":%q,"key":%q,"position":%d,"hideSidebar":%t,"hideHome":%t,"count":0}`,
			id, key, 9-i, id == lib, id == "radio" || id == "picks"))
	}
	// The other libraries follow in their stored order (since O-20 they stand in front of radio).
	for _, e := range stored {
		if !slices.Contains(order, e.Id) {
			parts = append(parts, fmt.Sprintf(`{"id":%q,"key":"library","position":0,"hideSidebar":false,"hideHome":false,"count":0}`, e.Id))
		}
	}
	rec := send(t, h, http.MethodPut, BasePath+"/admin/my-media", "p18", "["+strings.Join(parts, ",")+"]", nil)
	adminCurationAnswer(t, rec, http.StatusOK)
	var saved []gen.MyMediaSetting
	decode(t, rec, &saved)
	if got := ids(saved); !slices.Equal(got[:4], order) {
		t.Fatalf("stored %v, want %v first", got, order)
	}
	if !saved[0].HideSidebar || saved[0].HideHome || !saved[1].HideHome || saved[2].HideHome || !saved[3].HideHome {
		t.Errorf("stored flags %+v", saved[:4])
	}

	// The side navigation of P18: library 1 is gone, the other libraries keep their place.
	rec = send(t, h, http.MethodGet, BasePath+"/my-media", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var nav []myMediaEntryJSON
	decode(t, rec, &nav)
	if len(nav) < 3 || nav[0].ID != "radio" || nav[1].ID != "collections" || nav[2].ID != "picks" ||
		slices.ContainsFunc(nav, func(e myMediaEntryJSON) bool { return e.ID == lib }) {
		t.Errorf("P18 side navigation %+v, want radio, collections, picks first and library 1 hidden", nav)
	}

	// The start page of P18: library 1 first, collections, then the other libraries; no radio, no picks.
	if _, err := pf.f.DB.Write.Exec(`UPDATE home_rows SET hidden = (id <> 'libraries')`); err != nil {
		t.Fatal(err)
	}
	tiles := func() ([]myMediaTileJSON, []int64) {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/home", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var page struct {
			Rows []struct {
				Key       string            `json:"key"`
				Media     []myMediaTileJSON `json:"media"`
				Libraries []struct {
					ID int64 `json:"id"`
				} `json:"libraries"`
			} `json:"rows"`
		}
		decode(t, rec, &page)
		if len(page.Rows) != 1 || page.Rows[0].Key != "libraries" {
			t.Fatalf("start page rows %+v, want «Meine Medien» alone", page.Rows)
		}
		var libs []int64
		for _, l := range page.Rows[0].Libraries {
			libs = append(libs, l.ID)
		}
		return page.Rows[0].Media, libs
	}
	media, libs := tiles()
	if len(media) < 2 || media[0].ID != lib || media[0].Library == nil || media[0].Library.ID != pf.f.LibraryID() ||
		media[0].Count != media[0].Library.ItemCount || media[1].ID != "collections" || media[1].Count == 0 ||
		slices.ContainsFunc(media, func(m myMediaTileJSON) bool { return m.Key == "radio" || m.Key == "picks" }) {
		t.Errorf("P18 tiles %+v, want library 1, collections, the other libraries", media)
	}
	if len(libs) == 0 || libs[0] != pf.f.LibraryID() || len(libs) != len(media)-1 {
		t.Errorf("P18 row libraries %v, want the library tiles in the order of media", libs)
	}

	// O-20: a new library comes last, visible in both places; a list that leaves it out keeps it there.
	var added int64
	if err := pf.f.DB.Write.QueryRow(`INSERT INTO libraries (name, kind, created_at, updated_at) VALUES ('Anime', 'video', 1, 1) RETURNING id`).Scan(&added); err != nil {
		t.Fatal(err)
	}
	newID := fmt.Sprintf("library:%d", added)
	after := get()
	if e := after[len(after)-1]; e.Id != newID || e.HideSidebar || e.HideHome || e.Position != len(after)-1 || e.Count != 0 {
		t.Errorf("new library %+v, want last and visible", e)
	}
	rec = send(t, h, http.MethodPut, BasePath+"/admin/my-media", "p18", `[{"id":"picks","key":"picks","position":0,"hideSidebar":true,"hideHome":true,"count":0}]`, nil)
	adminCurationAnswer(t, rec, http.StatusOK)
	decode(t, rec, &saved)
	picks := slices.IndexFunc(saved, func(e gen.MyMediaSetting) bool { return e.Id == "picks" })
	if e := saved[len(saved)-1]; e.Id != newID || e.HideSidebar || e.HideHome || picks != 3 || !saved[picks].HideSidebar {
		t.Errorf("short list stored %+v, want picks hidden in its place and the new library last, visible", saved)
	}

	for name, body := range map[string]string{
		"unknown id":   `[{"id":"genres","key":"radio","position":0,"hideSidebar":false,"hideHome":false,"count":0}]`,
		"unknown lib":  `[{"id":"library:999","key":"library","position":0,"hideSidebar":false,"hideHome":false,"count":0}]`,
		"key mismatch": `[{"id":"picks","key":"radio","position":0,"hideSidebar":false,"hideHome":false,"count":0}]`,
		"twice radio": `[{"id":"radio","key":"radio","position":0,"hideSidebar":false,"hideHome":false,"count":0},` +
			`{"id":"radio","key":"radio","position":1,"hideSidebar":false,"hideHome":false,"count":0}]`,
		"empty": `[]`,
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPut, BasePath+"/admin/my-media", "p18", body, nil)); got != "rows invalid_value" {
			t.Errorf("%s: %s", name, got)
		}
	}
	// Only the administration reaches the settings.
	wantStatus(t, send(t, h, http.MethodGet, BasePath+"/admin/my-media", "p6", "", nil), http.StatusForbidden)
}
