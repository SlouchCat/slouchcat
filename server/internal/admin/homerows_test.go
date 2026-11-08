package admin

import (
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// homeRowIDs returns the ids of rows in their order.
func homeRowIDs(rows []model.HomeRowSetting) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// E-9, concept 15 P-7: the eight fixed rows in the default order, then «Meine Medien», «Meine
// Sender» (migration 00039, concept 16 F-17: hidden at the end of the rows the migration found) and
// one row per library created afterwards, hidden; the order is set by id, the slice order is the
// display order.
func TestHomeRows(t *testing.T) {
	c := newCurationFixture(t)
	rows, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	lib := fmt.Sprintf("library:%d", c.f.LibraryID())
	if want := append(slices.Clone(model.HomeRowKeys[:8]), model.RowLibraries, "radio", lib); !slices.Equal(homeRowIDs(rows), want) {
		t.Fatalf("default order %v, want %v", homeRowIDs(rows), want)
	}
	if r := rows[10]; r.Key != model.RowLibrary || r.LibraryID != c.f.LibraryID() || r.LibraryName != "Filme" || !r.Hidden || !rows[8].Hidden || !rows[9].Hidden {
		t.Errorf("rows of P-7 %+v / %+v, want hidden, the library row with its library", rows[8], r)
	}

	order := []string{model.RowRecent, lib, model.RowPicks, model.RowGenres, model.RowContinue, model.RowWatchlist,
		model.RowCollections, model.RowReleased, model.RowSpotlight, model.RowLibraries, "radio"}
	in := make([]model.HomeRowSetting, 0, len(order))
	for i, id := range order {
		key := id
		if id == lib {
			key = model.RowLibrary
		}
		// Positions in the request do not matter: the slice order is the display order.
		in = append(in, model.HomeRowSetting{ID: id, Key: key, Position: 10 - i, Hidden: id == model.RowGenres})
	}
	out, err := c.svc.SetHomeRows(c.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(homeRowIDs(out), order) {
		t.Errorf("stored order %v, want %v", homeRowIDs(out), order)
	}
	for i, r := range out {
		if r.Position != i || r.Hidden != (r.ID == model.RowGenres) {
			t.Errorf("row %+v at %d", r, i)
		}
	}
	if again, err := c.svc.HomeRows(c.ctx); err != nil || !slices.Equal(homeRowIDs(again), order) || !again[3].Hidden {
		t.Errorf("read back %+v (%v)", again, err)
	}

	bad := map[string][]model.HomeRowSetting{
		"empty":         {},
		"duplicate":     append(slices.Clone(in[1:]), in[1]),
		"unknown":       append(slices.Clone(in[1:]), model.HomeRowSetting{ID: "trailers", Key: "trailers"}),
		"key mismatch":  {{ID: model.RowPicks, Key: model.RowRecent}},
		"other library": {{ID: "library:999", Key: model.RowLibrary}},
	}
	for name, rows := range bad {
		wantField(t, name, errOf(c.svc.SetHomeRows(c.ctx, rows)), "rows", "invalid_value")
	}
	if again, err := c.svc.HomeRows(c.ctx); err != nil || !slices.Equal(homeRowIDs(again), order) {
		t.Errorf("a refused change touched the order: %v (%v)", homeRowIDs(again), err)
	}
}

// Concept 15 P-7: a library created later gets its row hidden at the end (trigger of migration
// 00035); a list that leaves out stored rows keeps them in their place with their flag, so a page
// read before the library came does not lose it; at most fifteen rows are visible (F-12).
func TestHomeRowsNewLibraryAndLimit(t *testing.T) {
	c := newCurationFixture(t)
	before, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Seven more libraries: 10 + 7 = 17 rows, more than the fifteen visible places.
	var ids []string
	for i := range 7 {
		var id int64
		if err := c.f.DB.Write.QueryRowContext(c.ctx, `INSERT INTO libraries (name, kind, created_at, updated_at) VALUES (?, 'video', 5, 5) RETURNING id`,
			fmt.Sprintf("Neu %d", i)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, fmt.Sprintf("library:%d", id))
	}
	rows, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := append(homeRowIDs(before), ids...); !slices.Equal(homeRowIDs(rows), want) {
		t.Fatalf("rows after new libraries %v, want %v", homeRowIDs(rows), want)
	}
	for _, r := range rows[len(before):] {
		if !r.Hidden || r.Key != model.RowLibrary || r.LibraryName == "" {
			t.Errorf("new library row %+v, want hidden with its name", r)
		}
	}

	// The page knew only the first ten rows: it moves recent to the front and shows «Meine Medien».
	in := make([]model.HomeRowSetting, 0, len(before))
	recent := slices.IndexFunc(before, func(r model.HomeRowSetting) bool { return r.ID == model.RowRecent })
	in = append(in, before[recent])
	for _, r := range before {
		if r.ID != model.RowRecent {
			r.Hidden = r.Hidden && r.ID != model.RowLibraries
			in = append(in, r)
		}
	}
	out, err := c.svc.SetHomeRows(c.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if want := append(homeRowIDs(in), ids...); !slices.Equal(homeRowIDs(out), want) {
		t.Errorf("stored %v, want the new rows behind the list %v", homeRowIDs(out), want)
	}
	if r := out[slices.IndexFunc(out, func(r model.HomeRowSetting) bool { return r.ID == model.RowLibraries })]; r.Hidden {
		t.Errorf("«Meine Medien» %+v, want shown", r)
	}
	for _, r := range out[len(in):] {
		if !r.Hidden {
			t.Errorf("left-out row %+v lost its hidden flag", r)
		}
	}

	// Nine rows visible now (the eight and «Meine Medien»): six more reach fifteen, seven are too many.
	show := func(n int) []model.HomeRowSetting {
		next := slices.Clone(out)
		for i := range next {
			if next[i].Key == model.RowLibrary && n > 0 && next[i].Hidden {
				next[i].Hidden = false
				n--
			}
		}
		return next
	}
	wantField(t, "sixteen visible", errOf(c.svc.SetHomeRows(c.ctx, show(7))), "rows", "too_many_visible")
	if _, err := c.svc.SetHomeRows(c.ctx, show(6)); err != nil {
		t.Errorf("fifteen visible: %v", err)
	}
}
