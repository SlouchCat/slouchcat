package admin

import (
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// myMediaIDs returns the ids of entries in their order.
func myMediaIDs(entries []model.MyMediaSetting) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}

// Concept 21 P-11: radio, collections and picks (migration 00059, on an empty database), then the
// library the fixture created afterwards; every entry in the side navigation, collections and picks
// not on the start page; a library carries its name, kind and top-level titles. The order is set by id, the slice order is
// the display order, and each place keeps its own flag (O-21).
func TestMyMedia(t *testing.T) {
	c := newCurationFixture(t)
	entries, err := c.svc.MyMedia(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	lib := fmt.Sprintf("library:%d", c.f.LibraryID())
	// The fixture's library comes after migration 00059 on an empty database: behind the last library,
	// that is in front of Radio, Sammlungen and User-Empfehlungen (O-20).
	if want := []string{lib, model.MyMediaRadio, model.MyMediaCollections, model.MyMediaPicks}; !slices.Equal(myMediaIDs(entries), want) {
		t.Fatalf("default order %v, want %v", myMediaIDs(entries), want)
	}
	var titles int
	if err := c.f.DB.Read.QueryRowContext(c.ctx, `SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NULL
		AND kind IN ('movie', 'series', 'audiobook', 'podcast')`, c.f.LibraryID()).Scan(&titles); err != nil {
		t.Fatal(err)
	}
	if e := entries[0]; e.Key != model.MyMediaLibrary || e.LibraryID != c.f.LibraryID() || e.LibraryName != "Filme" ||
		e.LibraryKind != "video" || e.Count != titles || titles == 0 || e.HideSidebar || e.HideHome {
		t.Errorf("library entry %+v, want Filme with its %d titles, shown", e, titles)
	}
	for _, e := range entries[1:] {
		if e.HideSidebar || e.HideHome != (e.Key != model.MyMediaRadio) || e.LibraryID != 0 {
			t.Errorf("entry %+v, want in the side navigation, on the start page only radio", e)
		}
	}

	order := []string{model.MyMediaPicks, model.MyMediaRadio, lib, model.MyMediaCollections}
	in := make([]model.MyMediaSetting, 0, len(order))
	for i, id := range order {
		key := id
		if id == lib {
			key = model.MyMediaLibrary
		}
		// Positions and counts in the request do not matter.
		in = append(in, model.MyMediaSetting{ID: id, Key: key, Position: 9 - i, Count: 99,
			HideSidebar: id == model.MyMediaPicks, HideHome: id == lib})
	}
	out, err := c.svc.SetMyMedia(c.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(myMediaIDs(out), order) {
		t.Errorf("stored order %v, want %v", myMediaIDs(out), order)
	}
	for i, e := range out {
		if e.Position != i || e.HideSidebar != (e.ID == model.MyMediaPicks) || e.HideHome != (e.ID == lib) || e.Count == 99 {
			t.Errorf("entry %+v at %d", e, i)
		}
	}
	if again, err := c.svc.MyMedia(c.ctx); err != nil || !slices.Equal(myMediaIDs(again), order) || !again[0].HideSidebar || !again[2].HideHome {
		t.Errorf("read back %+v (%v)", again, err)
	}

	bad := map[string][]model.MyMediaSetting{
		"empty":         {},
		"duplicate":     append(slices.Clone(in[1:]), in[1]),
		"unknown":       {{ID: "genres", Key: "genres"}},
		"key mismatch":  {{ID: model.MyMediaPicks, Key: model.MyMediaRadio}},
		"other library": {{ID: "library:999", Key: model.MyMediaLibrary}},
	}
	for name, entries := range bad {
		wantField(t, name, errOf(c.svc.SetMyMedia(c.ctx, entries)), "rows", "invalid_value")
	}
	if again, err := c.svc.MyMedia(c.ctx); err != nil || !slices.Equal(myMediaIDs(again), order) {
		t.Errorf("a refused change touched the order: %v (%v)", myMediaIDs(again), err)
	}
}

// O-20: a library created later gets its entry at the end, visible in the side navigation and on
// the start page (trigger of migration 00059); a list that leaves it out (a page read before the
// library came) keeps it there with its flags.
func TestMyMediaNewLibrary(t *testing.T) {
	c := newCurationFixture(t)
	before, err := c.svc.MyMedia(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := c.f.DB.Write.QueryRowContext(c.ctx, `INSERT INTO libraries (name, kind, created_at, updated_at) VALUES ('Anime', 'video', 5, 5) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	added := fmt.Sprintf("library:%d", id)
	entries, err := c.svc.MyMedia(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Behind the last library (O-20, «hinter Bibliotheken»): before is the old library, radio,
	// collections, picks.
	if want := []string{myMediaIDs(before)[0], added, before[1].ID, before[2].ID, before[3].ID}; !slices.Equal(myMediaIDs(entries), want) {
		t.Fatalf("entries after a new library %v, want %v", myMediaIDs(entries), want)
	}
	if e := entries[1]; e.HideSidebar || e.HideHome || e.LibraryName != "Anime" || e.Count != 0 {
		t.Errorf("new library %+v, want visible in both places, without titles", e)
	}

	// The page knew only the four entries from before: it moves radio to the front and hides picks in
	// the side navigation (before: the library, radio, collections, picks).
	in := []model.MyMediaSetting{before[1], before[0], before[2], before[3]}
	in[3].HideSidebar = true
	out, err := c.svc.SetMyMedia(c.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	// The left-out new library keeps its place (index 1).
	if want := []string{in[0].ID, added, in[1].ID, in[2].ID, in[3].ID}; !slices.Equal(myMediaIDs(out), want) {
		t.Errorf("stored %v, want the left-out library at its place %v", myMediaIDs(out), want)
	}
	if e := out[1]; e.HideSidebar || e.HideHome || e.Position != 1 {
		t.Errorf("left-out library %+v lost its place or flags", e)
	}
}
