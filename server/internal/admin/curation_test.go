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
// user picks of P18 (the films rated 18 and 6).
func newCurationFixture(t *testing.T) curationFixture {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddGuest(t)
	f.AddSecondAdmin(t)
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

// Concept 18 part B (14.7, ADR 0028): the administration lists every title with a pick that has
// not expired, newest pick first, with the number of picks, the day of the newest one in the
// server zone and the days left, never a profile; removing takes every pick of the title, and the
// retention is 7 to 365 days and works at once.
func TestUserPicksAdministration(t *testing.T) {
	c := newCurationFixture(t)
	f := c.f
	now := c.svc.now()
	const day = int64(24 * 60 * 60 * 1000)
	c.exec(t, `DELETE FROM user_picks`)
	c.exec(t, `INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?), (?, ?, ?), (?, ?, ?)`,
		f.Rated(16).ID, f.P18.ID, now.UnixMilli()-10*day,
		f.Rated(16).ID, f.P12.ID, now.UnixMilli()-60*60*1000, // newest: one hour ago
		f.Rated(6).ID, f.P6.ID, now.UnixMilli()-59*day-day/2, // half a day left
		f.Rated(0).ID, f.P0.ID, now.UnixMilli()-61*day, // expired
		f.Rated(12).ID, f.PG.ID, now.UnixMilli()-2*day) // a stored row of a guest still counts here
	picks, err := c.svc.ListUserPicks(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id, title, on string
		count, left   int
	}
	var got []row
	for _, p := range picks {
		got = append(got, row{p.ItemID, p.Title, p.PickedOn, p.Count, p.ExpiresInDays})
	}
	want := []row{
		{f.Rated(16).ID, "Film 16", "2026-10-21", 2, 60},
		{f.Rated(12).ID, "Film 12", "2026-10-19", 1, 58},
		{f.Rated(6).ID, "Film 06", "2026-08-23", 1, 1},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("user picks = %+v, want %+v", got, want)
	}

	if err := c.svc.RemoveUserPick(c.ctx, f.Rated(16).ID); err != nil {
		t.Fatal(err)
	}
	if err := c.svc.RemoveUserPick(c.ctx, f.Rated(16).ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second remove: %v, want ErrNotFound", err)
	}
	var left int
	if err := f.DB.Read.QueryRowContext(c.ctx, `SELECT count(*) FROM user_picks WHERE item_id = ?`, f.Rated(16).ID).Scan(&left); err != nil || left != 0 {
		t.Errorf("rows of the removed title: %d (%v)", left, err)
	}

	if days, err := c.svc.PicksRetention(c.ctx); err != nil || days != 60 {
		t.Errorf("default retention %d (%v), want 60", days, err)
	}
	for _, bad := range []int{6, 366, 0, -1} {
		wantField(t, "retention "+strconv.Itoa(bad), errOf(c.svc.SetPicksRetention(c.ctx, bad)), "retentionDays", "invalid_value")
	}
	for _, good := range []int{365, 7} {
		if days, err := c.svc.SetPicksRetention(c.ctx, good); err != nil || days != good {
			t.Errorf("SetPicksRetention(%d) = %d, %v", good, days, err)
		}
	}
	// Seven days: only the pick of two days ago is left, with five days.
	if picks, err = c.svc.ListUserPicks(c.ctx); err != nil || len(picks) != 1 || picks[0].ItemID != f.Rated(12).ID || picks[0].ExpiresInDays != 5 {
		t.Errorf("user picks with 7 days = %+v (%v), want the film rated 12 with 5 days", picks, err)
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

// Merging titles (B-9) moves watchlist, favourites, collection membership, notifications and user
// picks (concept 18 part B, 14.8: a profile that picked both keeps the newer time).
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
	m.exec(t, `DELETE FROM user_picks`)
	m.exec(t, `INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (?, ?, 5), (?, ?, 3), (?, ?, 4)`,
		source.ID, f.P18.ID, target.ID, f.P18.ID, source.ID, f.P16.ID)
	if _, err := m.svc.MergeItems(m.ctx, target.ID, []string{source.ID}); err != nil {
		t.Fatal(err)
	}
	var picks string
	if err := f.DB.Read.QueryRow(`SELECT group_concat(item_id || ':' || profile_id || ':' || created_at, ' ') FROM (SELECT * FROM user_picks ORDER BY profile_id)`).Scan(&picks); err != nil ||
		picks != target.ID+":"+strconv.FormatInt(f.P18.ID, 10)+":5 "+target.ID+":"+strconv.FormatInt(f.P16.ID, 10)+":4" {
		t.Errorf("user picks after the merge = %s (%v), want P18 at 5 and P16 at 4 on the target", picks, err)
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
