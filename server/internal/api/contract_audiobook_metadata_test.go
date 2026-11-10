package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Contract 1.24.0 (concept 19 D5.3, PD-1, PD-15): the editor takes audiobooks. A text goes into
// every text row of the book (de and en in the fixture) and is locked in each; the book fields are
// columns with their own locks; an empty series takes the volume along; ISBN and ASIN are checked
// and stored in one form. The book's language, cover and refresh are package metadata's
// (TestBookLanguageAndRefresh, TestBookCoverChoice).
func TestAudiobookMetadataOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	book := pf.f.Audio.Books[0].ID
	path := BasePath + "/admin/items/" + book + "/metadata"
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

	it := patch(`{"title":"Neuer Titel","tagline":"Teil eins","overview":"Neu","genres":["Fantasy"],"author":" Hand Autorin ",
		"narrator":"Hand Sprecher","seriesName":"Hand Reihe","seriesIndex":2.5,"publisher":"Hand Verlag","isbn":"978-3-00-000000-0",
		"asin":"b0example1","year":2002}`)
	m := it.Metadata
	if m.Lang != "de" || m.Title != "Neuer Titel" || deref(m.Tagline) != "Teil eins" || deref(m.Author) != "Hand Autorin" ||
		deref(m.Narrator) != "Hand Sprecher" || deref(m.SeriesName) != "Hand Reihe" || m.SeriesIndex == nil || *m.SeriesIndex != 2.5 ||
		deref(m.Publisher) != "Hand Verlag" || deref(m.Isbn) != "9783000000000" || deref(m.Asin) != "B0EXAMPLE1" ||
		!slices.Equal(m.Genres, []string{"Fantasy"}) {
		t.Fatalf("metadata after the edit = %+v", m)
	}
	for _, lock := range []string{"title:de", "title:en", "tagline:de", "tagline:en", "overview:de", "overview:en", "genres", "author",
		"narrator", "series_name", "series_index", "publisher", "isbn", "asin", "year"} {
		if !slices.Contains(it.LockedFields, lock) {
			t.Errorf("lockedFields %v without %s", it.LockedFields, lock)
		}
	}
	rows, err := pf.f.DB.Read.Query(`SELECT lang || '=' || title || '/' || overview || '/' || tagline FROM item_texts WHERE item_id = ? ORDER BY lang`, book)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		texts = append(texts, s)
	}
	_ = rows.Close()
	if want := []string{"de=Neuer Titel/Neu/Teil eins", "en=Neuer Titel/Neu/Teil eins"}; !slices.Equal(texts, want) {
		t.Errorf("text rows %v, want %v", texts, want)
	}
	var genres string
	if err := pf.f.DB.Read.QueryRow(`SELECT genres FROM items WHERE id = ?`, book).Scan(&genres); err != nil || genres != `{"de":["Fantasy"],"en":["Fantasy"]}` {
		t.Errorf("genres %s (%v)", genres, err)
	}

	// Unlocking a text frees it in every row; an empty series takes the volume along.
	it = patch(`{"seriesName":"","unlock":["title"]}`)
	if slices.Contains(it.LockedFields, "title:de") || slices.Contains(it.LockedFields, "title:en") ||
		!slices.Contains(it.LockedFields, "series_index") || it.Metadata.SeriesName != nil || it.Metadata.SeriesIndex != nil {
		t.Errorf("after clearing the series: locks %v, metadata %+v", it.LockedFields, it.Metadata)
	}

	// Validation of the book fields, and of book fields on other kinds.
	for _, tc := range []struct{ id, body, want string }{
		{book, `{"isbn":"978-3"}`, "isbn:invalid_value"},
		{book, `{"isbn":"30000000X0"}`, "isbn:invalid_value"},
		{book, `{"asin":"B0SHORT"}`, "asin:invalid_value"},
		{book, `{"seriesIndex":-1}`, "seriesIndex:invalid_value"},
		{book, `{"seriesName":"","seriesIndex":1}`, "seriesIndex:invalid_value"},
		{book, `{"originalTitle":"Nein"}`, "originalTitle:invalid_value"},
		{book, `{"releaseDate":"2002-01-01"}`, "releaseDate:invalid_value"},
		{book, `{"author":"` + strings.Repeat("x", 501) + `"}`, "author:too_long"},
		{fixtureFilm6, `{"author":"Nein"}`, "author:invalid_value"},
		{fixtureFilm6, `{"unlock":["asin"]}`, "unlock:invalid_value"},
	} {
		rec := send(t, pf.h, http.MethodPatch, BasePath+"/admin/items/"+tc.id+"/metadata", "p18", tc.body, nil)
		p := decodeProblem(t, rec)
		if rec.Code != http.StatusUnprocessableEntity || p.Errors == nil || len(*p.Errors) != 1 ||
			(*p.Errors)[0].Field+":"+(*p.Errors)[0].Code != tc.want {
			t.Errorf("%.12s %.60s: %d %s, want 422 %s", tc.id, tc.body, rec.Code, rec.Body.String(), tc.want)
		}
	}
	if it := patch(`{"isbn":"3-00-000000-x","asin":""}`); deref(it.Metadata.Isbn) != "300000000X" || it.Metadata.Asin != nil {
		t.Errorf("ISBN-10 and an empty ASIN: %+v", it.Metadata)
	}

}
