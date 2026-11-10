package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract 1.25.0 (concept 19 stage D2): the row «Reihen und Sammlungen» (listAudioGroups), the
// page of a series (getAudioSeries), «Reihe bearbeiten» (updateAudioSeries) and
// ItemDetail.seriesId/libraryId, on the series and the audiobook collection of
// vistest.AddAudioSeries.

// fixtureAudiobookLibrary is the library «Hörbücher» of AddAudio in the profile fixture (the third
// library it creates); TestAudioSeriesOverHTTP checks it.
const fixtureAudiobookLibrary = 3

// audioLibraryStatus is the answer of the audiobook library for a profile: 404 when it sees no
// book of it (PX and the guest see library 1 only), else 200; the book rated 0 is visible to all
// others.
func audioLibraryStatus(f *vistest.Fixture, p model.Profile) int {
	if !f.VisibleTo(p, f.Audio.Books[0]) {
		return http.StatusNotFound
	}
	return http.StatusOK
}

// seriesVolumesFor returns the volumes of AudioSeriesID the profile sees, in series order (0.5 =
// the book rated 18, 1 = rated 0, 2 = rated 12).
func seriesVolumesFor(f *vistest.Fixture, p model.Profile) []string {
	var out []string
	for _, m := range []vistest.Movie{f.Audio.Books[4], f.Audio.Books[0], f.Audio.Books[2]} {
		if f.VisibleTo(p, m) {
			out = append(out, m.ID)
		}
	}
	return out
}

func checkAudioGroups(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
	t.Helper()
	p := fixtureProfile(f, token)
	var l gen.AudioGroupList
	decode(t, rec, &l)
	var series, collection *gen.AudioGroup
	for i, g := range l.Groups {
		switch g.Id {
		case vistest.AudioSeriesID:
			series = &l.Groups[i]
		case vistest.AudioCollectionID:
			collection = &l.Groups[i]
		}
	}
	vols := seriesVolumesFor(f, p)
	switch {
	case len(vols) < 2 && series != nil:
		t.Errorf("listAudioGroups for %s shows the series with %d visible volumes", token, len(vols))
	case len(vols) >= 2 && (series == nil || series.Count != len(vols) || series.Kind != gen.AudioGroupKindSeries ||
		series.Name != vistest.AudioSeriesName || series.Cover == nil || series.Cover.Id != vols[0]):
		t.Errorf("listAudioGroups for %s: series %+v, want %d volumes with the cover %s", token, series, len(vols), vols[0])
	}
	if collection == nil || collection.Kind != gen.AudioGroupKindCollection || collection.Count != 2 || l.CollectionCount != 1 {
		t.Errorf("listAudioGroups for %s: audiobook collection %+v (%d collections)", token, collection, l.CollectionCount)
	}
}

func checkAudioSeries(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
	t.Helper()
	var a gen.AudioSeries
	decode(t, rec, &a)
	var got []string
	for _, v := range a.Volumes {
		got = append(got, v.Item.Id)
	}
	if want := seriesVolumesFor(f, fixtureProfile(f, token)); !slices.Equal(got, want) || a.Cover == nil || a.Cover.Id != want[0] ||
		a.Next == nil || a.LibraryId != fixtureAudiobookLibrary || a.Name != vistest.AudioSeriesName {
		t.Errorf("getAudioSeries for %s: volumes %v, want %v (%s)", token, got, want, rec.Body.String())
	}
}

// The three operations over HTTP: 200 with the shape of the contract, 404 for what the profile may
// not see, 422 for invalid filters and names; the detail of a book carries seriesId only with two
// visible volumes, libraryId always.
func TestAudioSeriesOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	if f.Audio.AudiobookLibraryID != fixtureAudiobookLibrary {
		t.Fatalf("audiobook library is %d, the registry assumes %d", f.Audio.AudiobookLibraryID, fixtureAudiobookLibrary)
	}
	groups := fmt.Sprintf("%s/libraries/%d/audio-groups", BasePath, fixtureAudiobookLibrary)
	page := fmt.Sprintf("%s/libraries/%d/series/%s", BasePath, fixtureAudiobookLibrary, vistest.AudioSeriesID)

	rec := send(t, h, http.MethodGet, page, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var a gen.AudioSeries
	decode(t, rec, &a)
	if len(a.Volumes) != 3 || a.Volumes[0].SeriesIndex == nil || *a.Volumes[0].SeriesIndex != 0.5 || a.Volumes[0].Narrator == nil ||
		len(a.Spellings) != 2 || a.Spellings[0].Name != "Reihe" || a.Spellings[0].Count != 2 || a.DurationMs != 3*vistest.DurationMs ||
		a.NameLocked == nil || *a.NameLocked || a.Narrator != nil {
		t.Errorf("series page %s", rec.Body.String())
	}
	for path, token := range map[string]string{
		page: "p6", // one visible volume
		fmt.Sprintf("%s/libraries/%d/series/%s", BasePath, fixtureLibrary, vistest.AudioSeriesID):  "p18", // another library
		fmt.Sprintf("%s/libraries/%d/series/%s", BasePath, fixtureAudiobookLibrary, unknownItemID): "p18",
		fmt.Sprintf("%s/libraries/999/audio-groups", BasePath):                                     "p18",
		groups: "px",
	} {
		if rec := send(t, h, http.MethodGet, path, token, "", nil); rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s for %s: %d %s, want 404", path, token, rec.Code, rec.Body.String())
		}
	}

	// The filters: bounds like listLibraryItems, status by the catalog; a year nobody has empties the row.
	for query, want := range map[string]string{
		"?yearFrom=1869":                    "yearFrom invalid_value",
		"?yearTo=2101":                      "yearTo invalid_value",
		"?maxRating=7":                      "maxRating invalid_value",
		"?genre=" + strings.Repeat("g", 81): "genre too_long",
		"?status=nope":                      "status invalid_value",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodGet, groups+query, "p18", "", nil)); got != want {
			t.Errorf("listAudioGroups%s: %s, want %s", query, got, want)
		}
	}
	rec = send(t, h, http.MethodGet, groups+"?yearFrom=1870&yearTo=1870", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var l gen.AudioGroupList
	decode(t, rec, &l)
	if len(l.Groups) != 0 || l.SeriesCount != 0 || l.CollectionCount != 0 {
		t.Errorf("groups of 1870: %s", rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, groups+"?maxRating=12", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &l)
	for _, g := range l.Groups {
		if g.Id == vistest.AudioSeriesID && (g.Count != 3 || g.Matching != 2) {
			t.Errorf("series with maxRating 12: %+v, want 2 of 3", g)
		}
	}

	// The detail: seriesId with two visible volumes, libraryId always.
	detail := func(token, id string) gen.ItemDetail {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/items/"+id, token, "", nil)
		wantStatus(t, rec, http.StatusOK)
		var d gen.ItemDetail
		decode(t, rec, &d)
		return d
	}
	book := f.Audio.Books[0].ID
	if d := detail("p12", book); d.SeriesId == nil || *d.SeriesId != vistest.AudioSeriesID || d.LibraryId == nil || *d.LibraryId != fixtureAudiobookLibrary {
		t.Errorf("P12 detail seriesId %v libraryId %v", d.SeriesId, d.LibraryId)
	}
	if d := detail("p6", book); d.SeriesId != nil || d.LibraryId == nil {
		t.Errorf("P6 detail seriesId %v libraryId %v, want no series", d.SeriesId, d.LibraryId)
	}
	if d := detail("p18", fixtureFilm6); d.SeriesId != nil || d.LibraryId == nil || *d.LibraryId != fixtureLibrary {
		t.Errorf("film detail seriesId %v libraryId %v", d.SeriesId, d.LibraryId)
	}

	// «Reihe bearbeiten»: 422, 404, 200 with the locked name; with writeToVolumes the volumes the
	// admin's profile sees carry the name.
	admin := BasePath + "/admin/audio-series/" + vistest.AudioSeriesID
	for body, want := range map[string]string{`{"name":"  "}`: "name required", `{"name":"` + strings.Repeat("x", 201) + `"}`: "name too_long"} {
		if got := fieldProblem(t, send(t, h, http.MethodPatch, admin, "p18", body, nil)); got != want {
			t.Errorf("updateAudioSeries %.20s: %s, want %s", body, got, want)
		}
	}
	if rec := send(t, h, http.MethodPatch, BasePath+"/admin/audio-series/"+unknownItemID, "p18", `{"name":"X"}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown series: %d", rec.Code)
	}
	rec = send(t, h, http.MethodPatch, admin, "p18", `{"name":" Die Reihe ","writeToVolumes":true}`, nil)
	wantStatus(t, rec, http.StatusOK)
	var adm gen.AdminAudioSeries
	decode(t, rec, &adm)
	if adm.Id != vistest.AudioSeriesID || adm.Name != "Die Reihe" || !adm.NameLocked || adm.VolumeCount != 3 || adm.LibraryId != fixtureAudiobookLibrary {
		t.Errorf("updateAudioSeries = %s", rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, page, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &a)
	if a.Name != "Die Reihe" || a.NameLocked == nil || !*a.NameLocked || len(a.Spellings) != 1 || a.Spellings[0].Name != "Die Reihe" {
		t.Errorf("page after the rename: %s", rec.Body.String())
	}
}
