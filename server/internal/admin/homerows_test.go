package admin

import (
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
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
	if want := append(slices.Clone(model.HomeRowKeys[:8]), model.RowLibraries, model.RowRadio, lib); !slices.Equal(homeRowIDs(rows), want) {
		t.Fatalf("default order %v, want %v", homeRowIDs(rows), want)
	}
	if r := rows[10]; r.Key != model.RowLibrary || r.LibraryID != c.f.LibraryID() || r.LibraryName != "Filme" || !r.Hidden || !rows[8].Hidden || !rows[9].Hidden {
		t.Errorf("rows of P-7 %+v / %+v, want hidden, the library row with its library", rows[8], r)
	}

	order := []string{model.RowRecent, lib, model.RowPicks, model.RowGenres, model.RowContinue, model.RowWatchlist,
		model.RowCollections, model.RowReleased, model.RowSpotlight, model.RowLibraries, model.RowRadio}
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

// A library that is being deleted (deleting_at, library.delete) leaves the start page rows of the
// administration at once, like ListLibraries. SetHomeRows still merges against its stored row: a
// list that names it and one that leaves it out are both valid, it keeps its place and hidden flag,
// and the answer leaves it out. The row does not count toward the fifteen visible places (F-12), so
// undoing the deletion can bring back more visible rows than the limit, one per library whose
// deletion was undone and whose row was visible (here one); the next save of that list is refused.
func TestHomeRowsDeletingLibrary(t *testing.T) {
	c := newCurationFixture(t)
	newLibrary := func(name string) string {
		t.Helper()
		var id int64
		if err := c.f.DB.Write.QueryRowContext(c.ctx, `INSERT INTO libraries (name, kind, created_at, updated_at) VALUES (?, 'video', 5, 5) RETURNING id`,
			name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("library:%d", id)
	}
	gone := newLibrary("Anime")
	// Five more libraries: 11 + 6 = 17 rows, sixteen of them not being deleted.
	for i := range 5 {
		newLibrary(fmt.Sprintf("Neu %d", i))
	}
	rows, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ids := homeRowIDs(rows); len(ids) != 17 || ids[11] != gone {
		t.Fatalf("rows %v, want Anime at index 11 of 17", ids)
	}
	// Anime shown second: nine visible rows, the eight fixed ones and Anime.
	in := slices.Clone(rows)
	anime := in[11]
	anime.Hidden = false
	in = slices.Insert(slices.Delete(in, 11, 12), 1, anime)
	before, err := c.svc.SetHomeRows(c.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if before[1].ID != gone || before[1].Hidden {
		t.Fatalf("row %+v, want Anime shown second", before[1])
	}

	q := appq.New(c.f.DB.Write)
	id := before[1].LibraryID
	if n, err := q.MarkLibraryDeleting(c.ctx, appq.MarkLibraryDeletingParams{Now: sql.NullInt64{Int64: 7, Valid: true}, ID: id}); err != nil || n != 1 {
		t.Fatalf("mark deleting: %d (%v)", n, err)
	}
	without := slices.Delete(slices.Clone(before), 1, 2)
	read, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(homeRowIDs(read), homeRowIDs(without)) {
		t.Fatalf("rows while deleting %v, want %v", homeRowIDs(read), homeRowIDs(without))
	}

	type storedRow struct {
		id     string
		hidden bool
	}
	stored := func() []storedRow {
		t.Helper()
		rows, err := q.ListHomeRows(c.ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]storedRow, 0, len(rows))
		for i, r := range rows {
			if r.Position != int64(i) {
				t.Errorf("stored row %s at position %d, want %d", r.ID, r.Position, i)
			}
			out = append(out, storedRow{r.ID, r.Hidden == 1})
		}
		return out
	}
	storedIDs := func() []string {
		t.Helper()
		var out []string
		for _, r := range stored() {
			out = append(out, r.id)
		}
		return out
	}
	// A page read before the mark still sends the row: valid, it moves «Meine Sender» to the front.
	radio := slices.IndexFunc(before, func(r model.HomeRowSetting) bool { return r.ID == model.RowRadio })
	stale := append([]model.HomeRowSetting{before[radio]}, slices.Delete(slices.Clone(before), radio, radio+1)...)
	out, err := c.svc.SetHomeRows(c.ctx, stale)
	if err != nil {
		t.Fatal(err)
	}
	if want := slices.DeleteFunc(homeRowIDs(stale), func(s string) bool { return s == gone }); !slices.Equal(homeRowIDs(out), want) {
		t.Errorf("answer %v, want without the library being deleted %v", homeRowIDs(out), want)
	}
	if want := homeRowIDs(stale); !slices.Equal(storedIDs(), want) {
		t.Errorf("stored %v, want %v", storedIDs(), want)
	}

	// A page read after the mark leaves it out and swaps the first two rows: the row keeps its place
	// (index 2) and its flag (shown).
	fresh := slices.Clone(out)
	fresh[0], fresh[1] = fresh[1], fresh[0]
	out, err = c.svc.SetHomeRows(c.ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if want := homeRowIDs(fresh); !slices.Equal(homeRowIDs(out), want) {
		t.Errorf("answer %v, want %v", homeRowIDs(out), want)
	}
	place := slices.Insert(homeRowIDs(fresh), 2, gone)
	if got := stored(); len(got) != len(place) || got[2] != (storedRow{gone, false}) {
		t.Errorf("stored %+v, want the left-out library shown at index 2", got)
	}
	if !slices.Equal(storedIDs(), place) {
		t.Errorf("stored %v, want %v", storedIDs(), place)
	}

	// Eight rows visible apart from Anime: seven more reach fifteen, eight are sixteen. Anime, shown
	// and named in the list, does not count.
	show := func(n int) []model.HomeRowSetting {
		next := slices.Clone(out)
		for i := range next {
			if n > 0 && next[i].Hidden {
				next[i].Hidden = false
				n--
			}
		}
		return next
	}
	wantField(t, "sixteen visible", errOf(c.svc.SetHomeRows(c.ctx, show(8))), "rows", "too_many_visible")
	with := slices.Insert(show(7), 2, anime)
	if _, err := c.svc.SetHomeRows(c.ctx, with); err != nil {
		t.Errorf("fifteen visible and the library being deleted: %v", err)
	}
	if !slices.Equal(storedIDs(), place) {
		t.Errorf("stored %v, want %v", storedIDs(), place)
	}

	if err := q.ClearLibraryDeleting(c.ctx, id); err != nil {
		t.Fatal(err)
	}
	back, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(homeRowIDs(back), place) {
		t.Fatalf("rows after undoing %v, want %v", homeRowIDs(back), place)
	}
	if r := back[2]; r.Position != 2 || r.Hidden || r.LibraryName != "Anime" {
		t.Errorf("row after undoing %+v, want shown at position 2", r)
	}
	visible := 0
	for _, r := range back {
		if !r.Hidden {
			visible++
		}
	}
	if visible != model.MaxVisibleHomeRows+1 {
		t.Errorf("%d visible rows after undoing, want one more than the limit", visible)
	}
	wantField(t, "saved after undoing", errOf(c.svc.SetHomeRows(c.ctx, back)), "rows", "too_many_visible")
}

// The row of a library that is being deleted keeps its stored hidden flag even when a page read
// before the deletion names it as shown; the request decides only its place. Two libraries being
// deleted at once, both rows shown and left out of the request, do not count toward the limit
// (TestHomeRowsDeletingLibrary probes the limit with such a row named); undoing both brings back
// two visible rows more than the limit, and the next save of that list is refused.
func TestHomeRowsDeletingLibraryFlagAndLimit(t *testing.T) {
	c := newCurationFixture(t)
	q := appq.New(c.f.DB.Write)
	libs := map[string]int64{}
	newLibrary := func(name string) string {
		t.Helper()
		var id int64
		if err := c.f.DB.Write.QueryRowContext(c.ctx, `INSERT INTO libraries (name, kind, created_at, updated_at) VALUES (?, 'video', 5, 5) RETURNING id`,
			name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		row := fmt.Sprintf("library:%d", id)
		libs[row] = id
		return row
	}
	shownA, shownB, hidden := newLibrary("A"), newLibrary("B"), newLibrary("H")
	// Five more libraries: 11 + 3 + 5 = 19 rows, sixteen of them not being deleted.
	for i := range 5 {
		newLibrary(fmt.Sprintf("Neu %d", i))
	}
	rows, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 19 {
		t.Fatalf("rows %v, want 19", homeRowIDs(rows))
	}
	// A and B shown, H hidden: ten visible rows, the eight fixed ones, A and B.
	for i := range rows {
		if rows[i].ID == shownA || rows[i].ID == shownB {
			rows[i].Hidden = false
		}
	}
	before, err := c.svc.SetHomeRows(c.ctx, rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{shownA, shownB, hidden} {
		if n, err := q.MarkLibraryDeleting(c.ctx, appq.MarkLibraryDeletingParams{Now: sql.NullInt64{Int64: 7, Valid: true}, ID: libs[row]}); err != nil || n != 1 {
			t.Fatalf("mark deleting %s: %d (%v)", row, n, err)
		}
	}
	storedHidden := func() map[string]bool {
		t.Helper()
		rows, err := q.ListHomeRows(c.ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := make(map[string]bool, len(rows))
		for _, r := range rows {
			out[r.ID] = r.Hidden == 1
		}
		return out
	}

	// A page read before the marks names H as shown: valid, H stays hidden.
	stale := slices.Clone(before)
	for i := range stale {
		if stale[i].ID == hidden {
			stale[i].Hidden = false
		}
	}
	if _, err := c.svc.SetHomeRows(c.ctx, stale); err != nil {
		t.Fatalf("stale list naming H: %v", err)
	}
	if h := storedHidden(); !h[hidden] || h[shownA] || h[shownB] {
		t.Errorf("stored flags A %v, B %v, H %v, want A and B shown, H hidden", h[shownA], h[shownB], h[hidden])
	}

	// A page read after the marks leaves A, B and H out: fifteen other rows visible are accepted,
	// sixteen are too many.
	read, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 16 {
		t.Fatalf("rows while deleting %v, want 16", homeRowIDs(read))
	}
	show := func(n int) []model.HomeRowSetting {
		next := slices.Clone(read)
		for i := range next {
			if n > 0 && next[i].Hidden {
				next[i].Hidden = false
				n--
			}
		}
		return next
	}
	wantField(t, "sixteen visible", errOf(c.svc.SetHomeRows(c.ctx, show(8))), "rows", "too_many_visible")
	if _, err := c.svc.SetHomeRows(c.ctx, show(7)); err != nil {
		t.Fatalf("fifteen visible with three libraries being deleted left out, two of them shown: %v", err)
	}
	if h := storedHidden(); !h[hidden] || h[shownA] || h[shownB] {
		t.Errorf("stored flags A %v, B %v, H %v after the left-out save", h[shownA], h[shownB], h[hidden])
	}

	for _, row := range []string{shownA, shownB, hidden} {
		if err := q.ClearLibraryDeleting(c.ctx, libs[row]); err != nil {
			t.Fatal(err)
		}
	}
	back, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	visible := 0
	for _, r := range back {
		if !r.Hidden {
			visible++
		}
		if r.ID == hidden && !r.Hidden {
			t.Errorf("H after undoing %+v, want hidden", r)
		}
	}
	if len(back) != 19 || visible != model.MaxVisibleHomeRows+2 {
		t.Errorf("%d rows, %d visible after undoing, want 19 and seventeen visible", len(back), visible)
	}
	wantField(t, "saved after undoing", errOf(c.svc.SetHomeRows(c.ctx, back)), "rows", "too_many_visible")
}
