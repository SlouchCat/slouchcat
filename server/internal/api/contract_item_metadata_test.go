package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Contract 1.15.0 (concept 15 7.5, P-6): the metadata editor. updateItemMetadata writes the texts
// of the server's metadata language (de in the fixture) and the title columns and locks every field
// it sets; unlock lifts the locks; the sort title orders the library; listAdminGenres collects the
// genres of that language.
func TestItemMetadataOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	path := BasePath + "/admin/items/" + fixtureFilm6 + "/metadata"
	patch := func(body string) gen.AdminItem {
		t.Helper()
		rec := send(t, pf.h, http.MethodPatch, path, "p18", body, nil)
		wantStatus(t, rec, http.StatusOK)
		var it gen.AdminItem
		decode(t, rec, &it)
		if it.Metadata == nil {
			t.Fatalf("%s: no metadata in %s", body, rec.Body.String())
		}
		return it
	}
	locked := func(m *gen.AdminItemMetadata) []string {
		out := []string{}
		for _, f := range m.Locked {
			out = append(out, string(f))
		}
		return out
	}

	// Before: the fixture's de texts, nothing locked; the field sources name nobody for the film.
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var before gen.AdminItem
	decode(t, rec, &before)
	if m := before.Metadata; m == nil || m.Lang != "de" || !strings.HasPrefix(m.Title, "Film ") || m.Overview == nil ||
		len(m.Locked) != 0 || !slices.Equal(m.Genres, []string{"Abenteuer"}) || m.Year == nil || m.ReleaseDate == nil {
		t.Fatalf("metadata before the edit = %+v", before.Metadata)
	}

	it := patch(`{"title":" Zebra ","sortTitle":"Aaa Érster","overview":"Neu","tagline":"","year":1999,"releaseDate":"1999-12-31",
		"genres":["Drama"," Drama ","Krimi"],"originalTitle":""}`)
	m := it.Metadata
	if m.Title != "Zebra" || m.SortTitle == nil || *m.SortTitle != "aaa erster" || m.Overview == nil || *m.Overview != "Neu" ||
		m.Tagline != nil || m.OriginalTitle != nil || m.Year == nil || *m.Year != 1999 || m.ReleaseDate == nil ||
		*m.ReleaseDate != "1999-12-31" || !slices.Equal(m.Genres, []string{"Drama", "Krimi"}) {
		t.Fatalf("metadata after the edit = %+v", m)
	}
	if got, want := locked(m), []string{"title", "originalTitle", "sortTitle", "year", "releaseDate", "tagline", "overview", "genres"}; !slices.Equal(got, want) {
		t.Errorf("locked = %v, want %v", got, want)
	}
	for _, l := range []string{"title:de", "sort_title:de", "overview:de", "tagline:de", "original_title", "year", "release_date", "genres"} {
		if !slices.Contains(it.LockedFields, l) {
			t.Errorf("lockedFields %v without %s", it.LockedFields, l)
		}
	}
	// The other language stays the providers': P6 reads English, P18 German.
	detail := func(token string) gen.ItemDetail {
		t.Helper()
		rec := send(t, pf.h, http.MethodGet, BasePath+"/items/"+fixtureFilm6, token, "", nil)
		wantStatus(t, rec, http.StatusOK)
		var d gen.ItemDetail
		decode(t, rec, &d)
		return d
	}
	if en := detail("p6"); !strings.HasPrefix(en.Title, "Movie ") || en.Overview == nil || !strings.HasPrefix(*en.Overview, "Plot ") ||
		!slices.Equal(en.Genres, []string{"Adventure"}) {
		t.Errorf("en detail changed: %q %v %v", en.Title, en.Overview, en.Genres)
	}
	if de := detail("p18"); de.Title != "Zebra" || !slices.Equal(de.Genres, []string{"Drama", "Krimi"}) {
		t.Errorf("de detail = %q %v", de.Title, de.Genres)
	}

	// The sort title orders the library: "aaa erster" comes before every "movie …".
	rec = send(t, pf.h, http.MethodGet, fmt.Sprintf("%s/libraries/%d/items?limit=100", BasePath, fixtureLibrary), "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var page libraryPageJSON
	decode(t, rec, &page)
	if len(page.Items) == 0 || page.Items[0].ID != fixtureFilm6 {
		t.Errorf("library order = %v, want %s first", page.ids(), fixtureFilm6)
	}

	// The genres of the server language, without regard to case.
	rec = send(t, pf.h, http.MethodGet, BasePath+"/admin/genres", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var list gen.AdminGenreList
	decode(t, rec, &list)
	if list.Lang != "de" || !slices.Equal(list.Genres, []string{"Abenteuer", "Drama", "Krimi"}) {
		t.Errorf("genres = %+v", list)
	}

	// Unlock: the locks go, the values stay; the sort key follows the title again.
	it = patch(`{"unlock":["title","sortTitle","genres"]}`)
	if got := locked(it.Metadata); slices.Contains(got, "title") || slices.Contains(got, "sortTitle") || slices.Contains(got, "genres") || !slices.Contains(got, "overview") {
		t.Errorf("locked after unlock = %v", got)
	}
	if it.Metadata.Title != "Zebra" || it.Metadata.SortTitle == nil || *it.Metadata.SortTitle != "zebra" {
		t.Errorf("after unlock: title %q sort %v", it.Metadata.Title, it.Metadata.SortTitle)
	}
	if slices.Contains(it.LockedFields, "title:de") || slices.Contains(it.LockedFields, "sort_title:de") || slices.Contains(it.LockedFields, "genres") {
		t.Errorf("lockedFields after unlock = %v", it.LockedFields)
	}
	// An empty body changes nothing.
	if again := patch(`{}`); !slices.Equal(again.LockedFields, it.LockedFields) {
		t.Errorf("empty body changed the locks: %v, want %v", again.LockedFields, it.LockedFields)
	}

	// Validation, unknown title, kinds.
	for _, tc := range []struct{ id, body, want string }{
		{fixtureFilm6, `{"title":"  "}`, "title:required"},
		{fixtureFilm6, `{"sortTitle":""}`, "sortTitle:required"},
		{fixtureFilm6, `{"title":"` + strings.Repeat("x", 501) + `"}`, "title:too_long"},
		{fixtureFilm6, `{"overview":"` + strings.Repeat("ü", 10001) + `"}`, "overview:too_long"},
		{fixtureFilm6, `{"year":1700}`, "year:invalid_value"},
		{fixtureFilm6, `{"releaseDate":"1999-13-01"}`, "releaseDate:invalid_value"},
		{fixtureFilm6, `{"genres":["Drama",""]}`, "genres:invalid_value"},
		{fixtureFilm6, `{"title":"X","unlock":["title"]}`, "unlock:invalid_value"},
		{fixtureFilm6, `{"unlock":["rating"]}`, "unlock:invalid_value"},
		{pf.f.Show.E1.ID, `{"tagline":"Nein"}`, "tagline:invalid_value"},
		{pf.f.Audio.Books[0].ID, `{"title":"Buch"}`, "kind:invalid_value"},
	} {
		rec := send(t, pf.h, http.MethodPatch, BasePath+"/admin/items/"+tc.id+"/metadata", "p18", tc.body, nil)
		p := decodeProblem(t, rec)
		if rec.Code != http.StatusUnprocessableEntity || p.Errors == nil || len(*p.Errors) != 1 ||
			(*p.Errors)[0].Field+":"+(*p.Errors)[0].Code != tc.want {
			t.Errorf("%s %.60s: %d %s, want 422 %s", tc.id, tc.body, rec.Code, rec.Body.String(), tc.want)
		}
	}
	// An episode takes its title, overview and dates.
	rec = send(t, pf.h, http.MethodPatch, BasePath+"/admin/items/"+pf.f.Show.E1.ID+"/metadata", "p18", `{"title":"Pilot","releaseDate":""}`, nil)
	wantStatus(t, rec, http.StatusOK)
	var ep gen.AdminItem
	decode(t, rec, &ep)
	if ep.Metadata == nil || ep.Metadata.Title != "Pilot" || ep.Metadata.ReleaseDate != nil {
		t.Errorf("episode after the edit = %+v", ep.Metadata)
	}
	rec = send(t, pf.h, http.MethodPatch, BasePath+"/admin/items/"+unknownItemID+"/metadata", "p18", `{"title":"X"}`, nil)
	if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
		t.Errorf("unknown title: %d %s", rec.Code, rec.Body.String())
	}
}

// refreshItem takes an optional body since contract 1.15.0: none, {} and {"unlockAll": …} are 202,
// a body that is no JSON object of that shape is 400.
func TestRefreshItemBodyOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	path := BasePath + "/admin/items/" + fixtureFilm6 + "/refresh"
	for body, want := range map[string]int{
		"": http.StatusAccepted, `{}`: http.StatusAccepted, `{"unlockAll":true}`: http.StatusAccepted, `{"unlockAll":false}`: http.StatusAccepted,
		`{"unlockAll":"yes"}`: http.StatusBadRequest,
	} {
		if rec := send(t, pf.h, http.MethodPost, path, "p18", body, nil); rec.Code != want {
			t.Errorf("refresh %q: %d %s, want %d", body, rec.Code, rec.Body.String(), want)
		}
	}
	if rec := send(t, pf.h, http.MethodPost, BasePath+"/admin/items/"+unknownItemID+"/refresh", "p18", `{"unlockAll":true}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("refresh of an unknown title: %d %s", rec.Code, rec.Body.String())
	}
}
