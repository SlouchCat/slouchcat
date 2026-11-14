package discover

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func entryIDs(entries []model.MyMediaEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}

func tileIDs(tiles []model.MyMediaTile) []string {
	out := make([]string, 0, len(tiles))
	for _, e := range tiles {
		out = append(out, e.ID)
	}
	return out
}

// Concept 21 P-9, P-11: the side navigation's group and the start page row «Meine Medien» follow
// the one stored order, each place without the entries hidden there (O-21); a library appears only
// while the profile sees a title of it (PX not library X); radio counts the profile's favourites
// and the tile carries the first four for the logos, none without favourites (O-15); collections
// and picks carry the visible counts. The row's libraries are the library tiles in that order.
func TestMyMediaOrderAndHiding(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	f.AddEdgeCases(t)
	r := &fakeRadio{stations: map[int64][]model.RadioHomeStation{}}
	for id := range int64(5) {
		r.stations[f.P18.ID] = append(r.stations[f.P18.ID], model.RadioHomeStation{Station: model.RadioStation{ID: 9100 + id, Name: fmt.Sprint("Radio ", id)}})
	}
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithRadio(r))
	ctx := context.Background()
	lib, other := fmt.Sprintf("library:%d", f.LibraryID()), fmt.Sprintf("library:%d", f.OtherLibraryID())
	// radio, library X, collections, library 1, picks; library 1 not in the side navigation, library
	// X not on the start page, picks on the start page too; the fixture's other libraries hidden in
	// both places, behind them.
	exec(t, f, `UPDATE my_media SET position = CASE id WHEN 'radio' THEN 0 WHEN ? THEN 1 WHEN 'collections' THEN 2 WHEN ? THEN 3
		WHEN 'picks' THEN 4 ELSE 5 END, hide_sidebar = (id = ?), hide_home = (id IN (?, 'collections'))`, other, lib, lib, other)
	exec(t, f, `UPDATE my_media SET hide_sidebar = 1, hide_home = 1 WHERE key = 'library' AND id NOT IN (?, ?)`, lib, other)
	exec(t, f, `UPDATE home_rows SET hidden = 1`)
	exec(t, f, `UPDATE home_rows SET hidden = 0 WHERE id = 'libraries'`)

	scope := visibility.Profile(f.P18)
	collections, err := s.vis.CollectionCount(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	picks, err := s.vis.PicksCount(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if collections == 0 || picks == 0 {
		t.Fatalf("fixture: %d collections, %d picks for P18, want some", collections, picks)
	}
	navs, err := s.VisibleLibraries(ctx, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	nav := func(id int64) model.LibraryNav {
		i := slices.IndexFunc(navs, func(n model.LibraryNav) bool { return n.ID == id })
		if i < 0 {
			t.Fatalf("P18 does not see library %d", id)
		}
		return navs[i]
	}

	entries, err := s.MyMedia(ctx, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{model.MyMediaRadio, other, model.MyMediaCollections, model.MyMediaPicks}; !slices.Equal(entryIDs(entries), want) {
		t.Fatalf("P18 side navigation %v, want %v", entryIDs(entries), want)
	}
	x := nav(f.OtherLibraryID())
	if e := entries[1]; e.Key != model.MyMediaLibrary || e.Library == nil || e.Library.ID != x.ID || e.Count != x.ItemCount {
		t.Errorf("library entry %+v, want %+v", e, x)
	}
	if entries[0].Count != 5 || entries[2].Count != collections || entries[3].Count != picks {
		t.Errorf("counts %+v, want 5 stations, %d collections, %d picks", entries, collections, picks)
	}
	// PX does not see library X: its entry is absent, the rest stands.
	if px, err := s.MyMedia(ctx, f.PX); err != nil || !slices.Equal(entryIDs(px), []string{model.MyMediaRadio, model.MyMediaCollections, model.MyMediaPicks}) {
		t.Errorf("PX side navigation %v (%v)", entryIDs(px), err)
	}

	page, err := s.Home(ctx, f.P18, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].Key != RowLibraries {
		t.Fatalf("rows %v, want «Meine Medien» alone", rowIDs(page.Rows))
	}
	row := page.Rows[0]
	if want := []string{model.MyMediaRadio, lib, model.MyMediaPicks}; !slices.Equal(tileIDs(row.Media), want) {
		t.Fatalf("P18 tiles %v, want %v", tileIDs(row.Media), want)
	}
	one := nav(f.LibraryID())
	radio, tile := row.Media[0], row.Media[1]
	if radio.Count != 5 || len(radio.Stations) != model.MyMediaStations || radio.Stations[0].ID != 9100 || radio.Library != nil {
		t.Errorf("radio tile %+v, want 5 stations and the first four", radio)
	}
	if tile.Library == nil || tile.Library.ID != one.ID || tile.Count != one.ItemCount || len(tile.Library.Items) == 0 {
		t.Errorf("library tile %+v, want library 1 with its collage", tile)
	}
	if row.Media[2].Count != picks {
		t.Errorf("picks tile %+v, want %d", row.Media[2], picks)
	}
	if len(row.Libraries) != 1 || row.Libraries[0].ID != one.ID {
		t.Errorf("row libraries %+v, want library 1 only", row.Libraries)
	}

	// O-15: without favourites the radio tile stays, without stations.
	page, err = s.Home(ctx, f.P6, nil)
	if err != nil || len(page.Rows) != 1 || page.Rows[0].Media[0].Key != model.MyMediaRadio ||
		page.Rows[0].Media[0].Count != 0 || len(page.Rows[0].Media[0].Stations) != 0 {
		t.Errorf("P6 without favourites: rows %+v (%v), want the radio tile without stations", page.Rows, err)
	}

	// Everything hidden on the start page: the row is empty and left out.
	exec(t, f, `UPDATE my_media SET hide_home = 1`)
	if page, err := s.Home(ctx, f.P18, nil); err != nil || len(page.Rows) != 0 {
		t.Errorf("all tiles hidden: rows %v (%v), want none", rowIDs(page.Rows), err)
	}
}
