package discover

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
	"github.com/slouchcat/slouchcat/server/internal/wishes"
)

// The start page row «Erfüllte Wünsche» (concept 25 step 3.4, E-10, N-97): the profile's wishes,
// own and joined, fulfilled within the last 14 days, newest first, as library titles the profile
// sees; a wish fulfilled 15 days ago, an open wish, a left co-wish and a title above the profile's
// rating stay out; a profile without such wishes gets no row (like other empty rows).
func TestHomeWishesRow(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	now := time.Now()
	day := func(n int) int64 { return now.Add(-time.Duration(n) * 24 * time.Hour).UnixMilli() }
	type w struct {
		id      int64
		state   string
		item    string
		availAt any
		profile model.Profile
		role    string
		closed  any
	}
	for _, x := range []w{
		{1, "available", f.Rated(6).ID, day(1), f.P6, "owner", nil},     // in the row, newest
		{2, "available", f.Rated(0).ID, day(3), f.P6, "joined", nil},    // joined: in the row
		{3, "available", f.Rated(18).ID, day(2), f.P6, "joined", nil},   // above P6's rating: out
		{4, "available", f.Rated(12).ID, day(15), f.P6, "owner", nil},   // too old: out
		{5, "processing", f.Rated(0).ID, nil, f.P6, "owner", nil},       // open: out
		{6, "available", f.Rated(0).ID, day(4), f.P6, "joined", day(4)}, // left: out
		{7, "available", f.Rated(16).ID, day(1), f.P18, "owner", nil},   // another profile's wish
	} {
		exec(t, f, `INSERT INTO wishes (id, media, tmdb_id, title, origin, state, item_id, created_at, updated_at, available_at, closed_at)
		            VALUES (?, 'movie', ?, 'W', 'slouchcat', ?, ?, 1, 1, ?, ?)`, x.id, 9000+x.id, x.state, x.item, x.availAt, x.availAt)
		exec(t, f, `INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at, closed_at) VALUES (?, ?, ?, ?, 1, ?)`,
			x.id, x.profile.ID, x.profile.AccountID, x.role, x.closed)
	}
	exec(t, f, `UPDATE home_rows SET hidden = 0, position = -1 WHERE id = 'wishes'`)
	ws := wishes.New(f.DB, nil, nil, vistest.Logger())
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithFulfilledWishes(ws.FulfilledItems))
	page, err := s.Home(ctx, f.P6, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) == 0 || page.Rows[0].Key != RowWishes {
		t.Fatalf("rows %v, want wishes first", rowIDs(page.Rows))
	}
	var ids []string
	for _, it := range page.Rows[0].Items {
		ids = append(ids, it.ID)
	}
	if want := []string{f.Rated(6).ID, f.Rated(0).ID}; !slices.Equal(ids, want) {
		t.Errorf("row wishes of P6 = %v, want %v", ids, want)
	}
	if page, err := s.Home(ctx, f.P18, nil); err != nil || page.Rows[0].Key != RowWishes || len(page.Rows[0].Items) != 1 ||
		page.Rows[0].Items[0].ID != f.Rated(16).ID {
		t.Errorf("row wishes of P18: %v, %v", rowIDs(page.Rows), err)
	}
	if page, err := s.Home(ctx, f.P0, nil); err != nil || slices.Contains(rowIDs(page.Rows), RowWishes) {
		t.Errorf("P0 without wishes: rows %v, %v", rowIDs(page.Rows), err)
	}
	// Without a source the row never appears.
	if page, err := New(f.DB, f.CacheDir, vistest.Logger()).Home(ctx, f.P6, nil); err != nil || slices.Contains(rowIDs(page.Rows), RowWishes) {
		t.Errorf("without a source: rows %v, %v", rowIDs(page.Rows), err)
	}
}
