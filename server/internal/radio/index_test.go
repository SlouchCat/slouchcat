package radio

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// facetCount returns the count of value in list (0 when absent).
func facetCount(list []model.RadioFacetCount, value string) int {
	for _, c := range list {
		if c.Value == value {
			return c.Count
		}
	}
	return 0
}

// A facet's counts ignore its own selection and honour every other one.
func TestIndexFacetCountsIgnoreOwnSelection(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	all, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{RadioFilter: model.RadioFilter{Countries: []string{"CH"}}})
	if err != nil {
		t.Fatal(err)
	}
	if facetCount(ch.Countries, "DE") != facetCount(all.Countries, "DE") || facetCount(ch.Countries, "DE") == 0 {
		t.Errorf("country counts with CH selected: DE %d, want %d", facetCount(ch.Countries, "DE"), facetCount(all.Countries, "DE"))
	}
	listed := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Countries: []string{"CH"}, Genres: []string{"jazz"}}})
	if got := facetCount(ch.Genres, "jazz"); got != len(listed) || got == 0 {
		t.Errorf("jazz in CH: facet %d, list %v", got, listed)
	}
	if facetCount(ch.Genres, "jazz") >= facetCount(all.Genres, "jazz") {
		t.Errorf("the genre counts ignore the country selection: %d of %d", facetCount(ch.Genres, "jazz"), facetCount(all.Genres, "jazz"))
	}
	both, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{RadioFilter: model.RadioFilter{Countries: []string{"CH"}, Genres: []string{"jazz"}}})
	if err != nil {
		t.Fatal(err)
	}
	if facetCount(both.Countries, "CH") != len(listed) {
		t.Errorf("CH count with jazz selected = %d, want %d", facetCount(both.Countries, "CH"), len(listed))
	}
	if facetCount(both.Genres, "pop") != facetCount(ch.Genres, "pop") {
		t.Errorf("genre counts with jazz selected changed: pop %d vs %d", facetCount(both.Genres, "pop"), facetCount(ch.Genres, "pop"))
	}
	if _, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{RadioFilter: model.RadioFilter{Qualities: []string{"best"}}}); err == nil {
		t.Error("an unknown quality was accepted")
	}
}

// A hidden station is never listed, counted or found, and its page answers 404.
func TestIndexHiddenNeverListed(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	id := e.stationID("FIP")
	yes := true
	if _, err := e.s.UpdateStation(e.ctx, id, model.AdminRadioStationPatch{Hidden: &yes}); err != nil {
		t.Fatal(err)
	}
	if got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Q: "fip"}}); len(got) != 0 {
		t.Errorf("search finds the hidden station: %v", got)
	}
	f, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{RadioFilter: model.RadioFilter{Q: "fip"}})
	if err != nil || len(f.Countries) != 0 {
		t.Errorf("facets of the hidden station: %+v %v", f.Countries, err)
	}
	if _, err := e.s.Station(e.ctx, catSession(1), id); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("hidden station: %v", err)
	}
	page, err := e.s.AdminStations(e.ctx, model.AdminRadioQuery{State: "hidden", Exceptions: true, Limit: 50})
	if err != nil || page.Total != 1 || page.Items[0].ID != id || !page.Items[0].Hidden {
		t.Errorf("admin hidden list = %+v %v", page, err)
	}
}

// The search folds case and diacritics and reads name, region and tags.
func TestIndexSearchFolded(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	for q, want := range map[string]string{
		"zurich":      "Energy Zürich",
		"ZÜRICH":      "Energy Zürich",
		"zürisee":     "Radio Zürisee",
		"smooth jazz": "Radio Swiss Jazz",
		"ch-vd":       "Couleur 3",
	} {
		if got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Q: q}}); !slices.Contains(got, want) {
			t.Errorf("search %q = %v, want %s among them", q, got, want)
		}
	}
	if got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Q: "keinsender"}}); len(got) != 0 {
		t.Errorf("search without a hit = %v", got)
	}
}

// The sorts: popular (recommended of the selected country first), name, quality, new; paging.
func TestIndexSortAndPage(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	ch := model.RadioFilter{Countries: []string{"CH"}}
	if got := e.list(model.RadioQuery{RadioFilter: ch}); got[0] != "Testradio Eins" {
		t.Errorf("popular in CH starts with %q", got[0])
	}
	yes := true
	if _, err := e.s.UpdateStation(e.ctx, e.stationID("Radio Munot"), model.AdminRadioStationPatch{Recommended: &yes}); err != nil {
		t.Fatal(err)
	}
	if got := e.list(model.RadioQuery{RadioFilter: ch}); got[0] != "Radio Munot" {
		t.Errorf("popular in CH with Radio Munot recommended starts with %q", got[0])
	}
	if got := e.list(model.RadioQuery{}); got[0] == "Radio Munot" {
		t.Error("a recommended station leads without a selected country")
	}
	names := e.list(model.RadioQuery{RadioFilter: ch, Sort: model.RadioSortName})
	if !slices.IsSortedFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }) {
		t.Errorf("name sort = %v", names)
	}
	if got := e.list(model.RadioQuery{Sort: model.RadioSortQuality}); got[0] != "Radio Paradise 320k" {
		t.Errorf("quality sort starts with %q", got[0])
	}
	page, err := e.s.ListStations(e.ctx, catSession(1), model.RadioQuery{RadioFilter: ch, Offset: 5, Limit: 5, Sort: model.RadioSortName})
	if err != nil || page.Total != len(names) || len(page.Items) != 5 || page.Items[0].Name != names[5] {
		t.Errorf("page = %d of %d, first %v; %v", len(page.Items), page.Total, page.Items, err)
	}
	if _, err := e.s.ListStations(e.ctx, catSession(1), model.RadioQuery{Sort: "votes", Limit: 5}); err == nil {
		t.Error("an unknown sort was accepted")
	}
	if got := fmt.Sprint(e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Genres: []string{"nope"}}})); got != "[]" {
		t.Errorf("unknown genre lists %s", got)
	}
}

// BenchmarkIndexQuery measures one list page and one facet request over 40 000 stations (the
// synthetic catalog of BenchmarkSyncLargeCatalog), with a search, a country and a genre selected.
// go test ./internal/radio -run '^$' -bench BenchmarkIndexQuery -benchmem
func BenchmarkIndexQuery(b *testing.B) {
	e := newCatEnv(b)
	raw := synthCatalog(50000)
	e.body.Store(&raw)
	e.sync()
	q := model.RadioQuery{RadioFilter: model.RadioFilter{Q: "station 1", Countries: []string{"DE"}, Genres: []string{"pop"}}, Limit: 60}
	for b.Loop() {
		if _, err := e.s.ListStations(e.ctx, catSession(1), q); err != nil {
			b.Fatal(err)
		}
		if _, err := e.s.Facets(e.ctx, catSession(1), q); err != nil {
			b.Fatal(err)
		}
	}
}
