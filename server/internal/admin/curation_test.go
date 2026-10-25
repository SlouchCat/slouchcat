package admin

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

const unknownID = "01J0000000000000000000XXXX"

type curationFixture struct {
	f   *vistest.Fixture
	svc *Service
	ctx context.Context
}

// newCurationFixture seeds the films, the series tree and the guest account; vistest adds two
// global picks (the films rated 18 and 6).
func newCurationFixture(t *testing.T) curationFixture {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddGuest(t)
	log := vistest.Logger()
	svc := New(f.DB, auth.New(f.DB, log), &fakeLibraries{}, nil, &fakeTMDB{}, func(string) string { return "" }, log)
	svc.now = func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) }
	return curationFixture{f: f, svc: svc, ctx: context.Background()}
}

func (c curationFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := c.f.DB.Write.ExecContext(c.ctx, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// wantField asserts a validation error with exactly this field and code.
func wantField(t *testing.T, what string, err error, field, code string) {
	t.Helper()
	var verr *model.ValidationError
	if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != field || verr.Fields[0].Code != code {
		t.Errorf("%s: %v, want %s %s", what, err, field, code)
	}
}

func pickIDs(picks []model.AdminPick) []int64 {
	out := make([]int64, 0, len(picks))
	for _, p := range picks {
		out = append(out, p.ID)
	}
	return out
}

func TestTopPicks(t *testing.T) {
	c := newCurationFixture(t)
	f := c.f
	// vistest names every title "Film X" (de) and "Movie X" (en); the film rated 12 has no German text.
	c.exec(t, `UPDATE item_texts SET title = 'Sechzehn' WHERE item_id = ? AND lang = 'de'`, f.Rated(16).ID)
	c.exec(t, `DELETE FROM item_texts WHERE item_id = ? AND lang = 'de'`, f.Rated(12).ID)

	seeded, err := c.svc.ListTopPicks(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeded) != 2 || seeded[0].ItemID != f.Rated(18).ID || seeded[0].ProfileID != nil || seeded[0].Title != "Film 18" || seeded[0].Kind != "movie" {
		t.Fatalf("seeded picks %+v", seeded)
	}

	// Global pick with note and window; the German text names it.
	starts, ends := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	global, err := c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(16).ID, Note: "Cat mag ihn", StartsAt: &starts, EndsAt: &ends})
	if err != nil {
		t.Fatal(err)
	}
	if len(global) != 1 || global[0].ProfileID != nil || global[0].Title != "Sechzehn" || global[0].Note != "Cat mag ihn" ||
		global[0].Position != 2 || !global[0].StartsAt.Equal(starts) || !global[0].EndsAt.Equal(ends) || global[0].Year == nil {
		t.Fatalf("global pick %+v", global)
	}
	// One pick per profile, at the end; the English text when there is no German one.
	per, err := c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(12).ID, ProfileIDs: []int64{f.P12.ID, f.P6.ID, f.P12.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(per) != 2 || *per[0].ProfileID != f.P12.ID || per[0].ProfileName != "P12" || *per[1].ProfileID != f.P6.ID ||
		per[0].Position != 3 || per[1].Position != 4 || per[0].Title != "Movie 12" || per[0].Note != "" || per[0].StartsAt != nil {
		t.Fatalf("per-profile picks %+v", per)
	}
	// Episodes may be picked.
	if _, err := c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Show.E1.ID}); err != nil {
		t.Errorf("episode pick: %v", err)
	}

	wantField(t, "unknown item", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: unknownID})), "itemId", "unknown_item")
	wantField(t, "missing item", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{})), "itemId", "required")
	wantField(t, "season", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Show.Season.ID})), "itemId", "invalid_value")
	wantField(t, "extra", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Show.Extra.ID})), "itemId", "invalid_value")
	wantField(t, "guest profile", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(0).ID, ProfileIDs: []int64{f.P18.ID, f.PG.ID}})), "profileIds", "invalid_value")
	wantField(t, "unknown profile", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(0).ID, ProfileIDs: []int64{999}})), "profileIds", "invalid_value")
	wantField(t, "window", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(0).ID, StartsAt: &ends, EndsAt: &starts})), "endsAt", "invalid_value")
	wantField(t, "empty window", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(0).ID, StartsAt: &starts, EndsAt: &starts})), "endsAt", "invalid_value")
	wantField(t, "long note", errOf(c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(0).ID, Note: strings.Repeat("ä", 281)})), "note", "too_long")
	if _, err := c.svc.CreateTopPicks(c.ctx, model.PickRequest{ItemID: f.Rated(0).ID, Note: strings.Repeat("ä", 280)}); err != nil {
		t.Errorf("note of 280 characters: %v", err)
	}

	// Update: absent keeps, a Clear flag clears, the window is checked on the result.
	note := "neu"
	upd, err := c.svc.UpdateTopPick(c.ctx, global[0].ID, model.PickUpdate{Note: &note, ClearStartsAt: true})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Note != "neu" || upd.StartsAt != nil || upd.EndsAt == nil || !upd.EndsAt.Equal(ends) {
		t.Errorf("updated pick %+v", upd)
	}
	late := ends.Add(time.Hour)
	wantField(t, "update window", errOf(c.svc.UpdateTopPick(c.ctx, global[0].ID, model.PickUpdate{StartsAt: &late})), "endsAt", "invalid_value")
	if upd, err = c.svc.UpdateTopPick(c.ctx, global[0].ID, model.PickUpdate{ClearNote: true, ClearEndsAt: true}); err != nil || upd.Note != "" || upd.EndsAt != nil {
		t.Errorf("cleared pick %+v (%v)", upd, err)
	}
	if _, err := c.svc.UpdateTopPick(c.ctx, 9999, model.PickUpdate{}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("update of an unknown pick: %v", err)
	}

	// Reorder: every pick exactly once.
	all, err := c.svc.ListTopPicks(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	ids := pickIDs(all)
	reversed := slices.Clone(ids)
	slices.Reverse(reversed)
	if err := c.svc.ReorderTopPicks(c.ctx, reversed); err != nil {
		t.Fatal(err)
	}
	after, err := c.svc.ListTopPicks(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := pickIDs(after); !slices.Equal(got, reversed) || after[0].Position != 0 || after[len(after)-1].Position != len(after)-1 {
		t.Errorf("order after reorder %v, want %v", got, reversed)
	}
	for name, bad := range map[string][]int64{
		"missing one": reversed[1:], "duplicate": append(slices.Clone(reversed[1:]), reversed[1]),
		"unknown": append(slices.Clone(reversed[1:]), 9999), "empty": nil,
	} {
		wantField(t, "reorder "+name, c.svc.ReorderTopPicks(c.ctx, bad), "ids", "invalid_value")
	}

	// Delete.
	if err := c.svc.DeleteTopPick(c.ctx, global[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := c.svc.DeleteTopPick(c.ctx, global[0].ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second delete: %v, want ErrNotFound", err)
	}
	if final, err := c.svc.ListTopPicks(c.ctx); err != nil || len(final) != len(all)-1 {
		t.Errorf("%d picks after the delete (%v), want %d", len(final), err, len(all)-1)
	}
}

func errOf[T any](_ T, err error) error { return err }

func TestCollections(t *testing.T) {
	c := newCurationFixture(t)
	f := c.f

	wantField(t, "empty name", errOf(c.svc.CreateCollection(c.ctx, "  ")), "name", "required")
	wantField(t, "long name", errOf(c.svc.CreateCollection(c.ctx, strings.Repeat("x", 121))), "name", "too_long")
	coll, err := c.svc.CreateCollection(c.ctx, "  Lieblinge ")
	if err != nil {
		t.Fatal(err)
	}
	if len(coll.ID) != 26 || coll.Source != "manual" || coll.Name != "Lieblinge" || coll.ItemCount != 0 || coll.Items == nil || coll.TMDBID != nil {
		t.Fatalf("new collection %+v", coll)
	}

	members := []string{f.Rated(18).ID, f.Show.Series.ID, f.Rated(6).ID}
	coll, err = c.svc.SetCollectionItems(c.ctx, coll.ID, members)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i, it := range coll.Items {
		got = append(got, it.ItemID)
		if it.Position != i {
			t.Errorf("member %s at position %d, want %d", it.ItemID, it.Position, i)
		}
	}
	if !slices.Equal(got, members) || coll.ItemCount != 3 || coll.Items[1].Kind != "series" || coll.Items[1].Title != "Film SS" {
		t.Fatalf("members %+v", coll)
	}
	// The members are searchable under the collection's name.
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM search_index WHERE kind = 'collection' AND ref_id = ?`, coll.ID).Scan(&n); err != nil || n != 3 {
		t.Errorf("%d search rows (%v), want 3", n, err)
	}

	wantField(t, "unknown member", errOf(c.svc.SetCollectionItems(c.ctx, coll.ID, []string{f.Rated(0).ID, unknownID})), "itemIds", "unknown_item")
	for name, id := range map[string]string{"season": f.Show.Season.ID, "episode": f.Show.E1.ID, "extra": f.Show.Extra.ID} {
		wantField(t, name+" member", errOf(c.svc.SetCollectionItems(c.ctx, coll.ID, []string{id})), "itemIds", "invalid_value")
	}
	wantField(t, "duplicate member", errOf(c.svc.SetCollectionItems(c.ctx, coll.ID, []string{f.Rated(0).ID, f.Rated(0).ID})), "itemIds", "invalid_value")
	// n4: one read for the whole list; the first refused id in list order decides, a deleted title
	// is unknown.
	wantField(t, "season before unknown", errOf(c.svc.SetCollectionItems(c.ctx, coll.ID, []string{f.Show.Season.ID, unknownID})), "itemIds", "invalid_value")
	wantField(t, "empty id", errOf(c.svc.SetCollectionItems(c.ctx, coll.ID, []string{f.Rated(0).ID, ""})), "itemIds", "required")
	if _, err := f.DB.Write.ExecContext(c.ctx, `UPDATE items SET deleted_at = 1 WHERE id = ?`, f.Rated(16).ID); err != nil {
		t.Fatal(err)
	}
	wantField(t, "deleted member", errOf(c.svc.SetCollectionItems(c.ctx, coll.ID, []string{f.Rated(0).ID, f.Rated(16).ID})), "itemIds", "unknown_item")
	if _, err := f.DB.Write.ExecContext(c.ctx, `UPDATE items SET deleted_at = NULL WHERE id = ?`, f.Rated(16).ID); err != nil {
		t.Fatal(err)
	}
	if cur, err := c.svc.GetCollection(c.ctx, coll.ID); err != nil || len(cur.Items) != 3 {
		t.Errorf("a refused change touched the members: %+v (%v)", cur, err)
	}

	// Replace in a new order, rename.
	if coll, err = c.svc.SetCollectionItems(c.ctx, coll.ID, []string{f.Rated(6).ID, f.Rated(18).ID}); err != nil || len(coll.Items) != 2 || coll.Items[0].ItemID != f.Rated(6).ID {
		t.Fatalf("replaced members %+v (%v)", coll, err)
	}
	if coll, err = c.svc.UpdateCollection(c.ctx, coll.ID, "Beste"); err != nil || coll.Name != "Beste" || len(coll.Items) != 2 {
		t.Fatalf("renamed %+v (%v)", coll, err)
	}
	wantField(t, "rename empty", errOf(c.svc.UpdateCollection(c.ctx, coll.ID, "")), "name", "required")

	// TMDB collections are read-only; the admin sees them unfiltered.
	c.exec(t, `INSERT INTO collections (id, source, tmdb_id, name, name_de, created_at, updated_at) VALUES ('01J9000000000000000000TMDB', 'tmdb', 10, 'Saga', 'Saga DE', 1, 1)`)
	c.exec(t, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES ('01J9000000000000000000TMDB', ?, 2, 1), ('01J9000000000000000000TMDB', ?, 1, 1)`, f.Rated(16).ID, f.Rated(0).ID)
	tmdb, err := c.svc.GetCollection(c.ctx, "01J9000000000000000000TMDB")
	if err != nil {
		t.Fatal(err)
	}
	if tmdb.Source != "tmdb" || *tmdb.TMDBID != 10 || tmdb.NameDe != "Saga DE" || len(tmdb.Items) != 2 || tmdb.Items[0].ItemID != f.Rated(0).ID {
		t.Errorf("tmdb collection %+v", tmdb)
	}
	if _, err := c.svc.UpdateCollection(c.ctx, tmdb.ID, "Neu"); !errors.Is(err, model.ErrNotEditable) {
		t.Errorf("rename of a TMDB collection: %v", err)
	}
	if _, err := c.svc.SetCollectionItems(c.ctx, tmdb.ID, nil); !errors.Is(err, model.ErrNotEditable) {
		t.Errorf("members of a TMDB collection: %v", err)
	}
	if err := c.svc.DeleteCollection(c.ctx, tmdb.ID); !errors.Is(err, model.ErrNotEditable) {
		t.Errorf("delete of a TMDB collection: %v", err)
	}

	list, err := c.svc.ListCollections(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Beste" || list[0].ItemCount != 2 || len(list[0].Items) != 0 || list[1].Name != "Saga" {
		t.Errorf("list %+v", list)
	}

	for name, err := range map[string]error{
		"get":    errOf(c.svc.GetCollection(c.ctx, unknownID)),
		"rename": errOf(c.svc.UpdateCollection(c.ctx, unknownID, "x")),
		"items":  errOf(c.svc.SetCollectionItems(c.ctx, unknownID, nil)),
		"delete": c.svc.DeleteCollection(c.ctx, unknownID),
	} {
		if !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s of an unknown collection: %v", name, err)
		}
	}
	if err := c.svc.DeleteCollection(c.ctx, coll.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.svc.GetCollection(c.ctx, coll.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted collection: %v", err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM items WHERE id = ?`, f.Rated(6).ID).Scan(&n); err != nil || n != 1 {
		t.Errorf("a member went with its collection")
	}
}

func TestHomeRows(t *testing.T) {
	c := newCurationFixture(t)
	rows, err := c.svc.HomeRows(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	keys := func(rows []model.HomeRowSetting) []string {
		var out []string
		for _, r := range rows {
			out = append(out, r.Key)
		}
		return out
	}
	if !slices.Equal(keys(rows), model.HomeRowKeys) {
		t.Fatalf("default order %v", keys(rows))
	}

	order := []string{model.RowRecent, model.RowPicks, model.RowGenres, model.RowContinue, model.RowWatchlist,
		model.RowCollections, model.RowReleased, model.RowSpotlight}
	in := make([]model.HomeRowSetting, 0, len(order))
	for i, k := range order {
		// Positions in the request do not matter: the slice order is the display order.
		in = append(in, model.HomeRowSetting{Key: k, Position: 7 - i, Hidden: k == model.RowGenres})
	}
	out, err := c.svc.SetHomeRows(c.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(keys(out), order) {
		t.Errorf("stored order %v, want %v", keys(out), order)
	}
	for i, r := range out {
		if r.Position != i || r.Hidden != (r.Key == model.RowGenres) {
			t.Errorf("row %+v at %d", r, i)
		}
	}
	if again, err := c.svc.HomeRows(c.ctx); err != nil || !slices.Equal(keys(again), order) || !again[2].Hidden {
		t.Errorf("read back %+v (%v)", again, err)
	}

	bad := map[string][]model.HomeRowSetting{
		"seven":     in[1:],
		"duplicate": append(slices.Clone(in[1:]), in[1]),
		"unknown":   append(slices.Clone(in[1:]), model.HomeRowSetting{Key: "trailers"}),
		"nine":      append(slices.Clone(in), in[0]),
	}
	for name, rows := range bad {
		wantField(t, name, errOf(c.svc.SetHomeRows(c.ctx, rows)), "rows", "invalid_value")
	}
	if again, err := c.svc.HomeRows(c.ctx); err != nil || !slices.Equal(keys(again), order) {
		t.Errorf("a refused change touched the order: %v (%v)", keys(again), err)
	}
}

func TestSearchItems(t *testing.T) {
	c := newCurationFixture(t)
	f := c.f
	hits, err := c.svc.SearchItems(c.ctx, "series", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != f.Show.Series.ID || hits[0].Kind != "series" || hits[0].LibraryID != f.LibraryID() {
		t.Errorf("series hits %+v", hits)
	}
	hits, err = c.svc.SearchItems(c.ctx, "episode E1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != f.Show.E1.ID || hits[0].ParentTitle != "Film SS" {
		t.Errorf("episode hits %+v", hits)
	}
	// Seasons and extras are never hits; the search is unfiltered (FSK 18 included).
	if hits, err = c.svc.SearchItems(c.ctx, "season", 0); err != nil || len(hits) != 0 {
		t.Errorf("season hits %+v (%v)", hits, err)
	}
	if hits, err = c.svc.SearchItems(c.ctx, "movie", 2); err != nil || len(hits) != 2 {
		t.Errorf("limited hits %+v (%v)", hits, err)
	}
	if hits, err = c.svc.SearchItems(c.ctx, "movie 18", 0); err != nil || len(hits) != 1 || hits[0].ID != f.Rated(18).ID {
		t.Errorf("FSK 18 hits %+v (%v)", hits, err)
	}
	wantField(t, "short query", errOf(c.svc.SearchItems(c.ctx, " a ", 0)), "q", "too_short")
	wantField(t, "long query", errOf(c.svc.SearchItems(c.ctx, strings.Repeat("a", 101), 0)), "q", "too_long")
	wantField(t, "limit", errOf(c.svc.SearchItems(c.ctx, "movie", 51)), "limit", "invalid_value")
}

// Merging titles (B-9) moves watchlist, favourites, collection membership and notifications.
func TestMergeMovesDiscoverRows(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	target, source := f.Rated(16), f.Rated(18)
	m.exec(t, `INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (?, ?, 1), (?, ?, 2), (?, ?, 3)`,
		f.P18.ID, source.ID, f.P16.ID, source.ID, f.P16.ID, target.ID)
	m.exec(t, `INSERT INTO favorites (profile_id, item_id, created_at) VALUES (?, ?, 1)`, f.P18.ID, source.ID)
	m.exec(t, `INSERT INTO collections (id, source, name, created_at, updated_at) VALUES ('01J9000000000000000000COLL', 'manual', 'Mix', 1, 1)`)
	m.exec(t, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES ('01J9000000000000000000COLL', ?, 5, 1)`, source.ID)
	m.exec(t, `INSERT INTO notifications (profile_id, kind, item_id, parent_id, created_at) VALUES (?, 'new_episode', ?, ?, 1)`,
		f.P18.ID, source.ID, source.ID)
	if _, err := m.svc.MergeItems(m.ctx, target.ID, []string{source.ID}); err != nil {
		t.Fatal(err)
	}
	for what, q := range map[string]string{
		"watchlist P18":     `SELECT count(*) FROM watchlist WHERE profile_id = ` + strconv.FormatInt(f.P18.ID, 10) + ` AND item_id = ?`,
		"watchlist P16":     `SELECT count(*) FROM watchlist WHERE profile_id = ` + strconv.FormatInt(f.P16.ID, 10) + ` AND item_id = ?`,
		"favorite":          `SELECT count(*) FROM favorites WHERE item_id = ?`,
		"collection member": `SELECT count(*) FROM collection_items WHERE collection_id = '01J9000000000000000000COLL' AND position = 5 AND item_id = ?`,
		"search row":        `SELECT count(*) FROM search_index WHERE kind = 'collection' AND item_id = ?`,
		"notification":      `SELECT count(*) FROM notifications WHERE item_id = ?1 AND parent_id = ?1`,
	} {
		var n int
		if err := f.DB.Read.QueryRow(q, target.ID).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s on the target: %d (%v), want 1", what, n, err)
		}
	}
	var n int
	if err := f.DB.Read.QueryRow(`SELECT (SELECT count(*) FROM watchlist) + (SELECT count(*) FROM collection_items)`).Scan(&n); err != nil || n != 3 {
		t.Errorf("%d watchlist and member rows (%v), want 3 (the source's rows are gone)", n, err)
	}
}
