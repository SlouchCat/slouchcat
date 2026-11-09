package discover

import (
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func rowKeys(rows []model.HomeRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Key)
	}
	return out
}

// homeRows returns the rows of the start page of p.
func homeRows(ctx context.Context, s *Service, p model.Profile) ([]model.HomeRow, error) {
	page, err := s.Home(ctx, p, nil)
	return page.Rows, err
}

func TestHomeRowsInOrderWithoutEmptyRows(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()

	rows, err := homeRows(ctx, s, f.P6)
	if err != nil {
		t.Fatal(err)
	}
	// Without watchlist, collections and Trakt lists those rows are omitted (E-9).
	if want := []string{RowPicks, RowContinue, RowRecent, RowGenres}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P6 rows = %v, want %v", rowKeys(rows), want)
	}
	// P0 has no progress and sees no pick: continue and picks are omitted.
	rows, err = homeRows(ctx, s, f.P0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowRecent, RowGenres}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P0 rows = %v, want %v", rowKeys(rows), want)
	}
}

// Plan 3.5: without a spotlight row the page has no stage, even with picks on top; picks is never
// the stage, so FirstHero answers model.ErrNotFound and the shell paints nothing ahead.
func TestFirstHeroWithoutStage(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	for _, p := range []model.Profile{f.P18, f.P6, f.P0, {ID: 9999, Language: "de"}} {
		page, err := s.Home(ctx, p, nil)
		if err != nil {
			t.Fatal(err)
		}
		if page.HeroRow != "" {
			t.Errorf("profile %d: hero row = %q, want none", p.ID, page.HeroRow)
		}
		if _, _, _, err := s.FirstHero(ctx, p); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("profile %d without a stage: err = %v, want ErrNotFound", p.ID, err)
		}
	}
	if rows, err := homeRows(ctx, s, f.P18); err != nil || len(rows) == 0 || rows[0].Key != RowPicks {
		t.Fatalf("P18 rows = %v, %v, want picks on top", rowKeys(rows), err)
	}
}

func TestItemCarriesMimeType(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	d, err := s.Item(context.Background(), f.P18, f.Unrated().ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.MediaFiles) != 1 || d.MediaFiles[0].MimeType == "" {
		t.Errorf("media files = %+v", d.MediaFiles)
	}
	if _, err := s.Item(context.Background(), f.P6, f.Unrated().ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unrated title for P6: err = %v", err)
	}
}

func TestImageOpensThroughCacheRoot(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	m := f.Rated(6)
	src, err := s.Image(ctx, f.P6, m.ID, "backdrop", "w1280", false)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(src.File)
	_ = src.File.Close()
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 16 || b.Dy() != 9 || src.MediaType != "image/jpeg" {
		t.Errorf("image %v, type %s", b, src.MediaType)
	}
	webp, err := s.Image(ctx, f.P6, m.ID, "poster", "w400", true)
	if err != nil {
		t.Fatal(err)
	}
	head := make([]byte, 12)
	_, err = io.ReadFull(webp.File, head)
	_ = webp.File.Close()
	if err != nil || string(head[:4]) != "RIFF" || string(head[8:]) != "WEBP" || webp.MediaType != "image/webp" {
		t.Errorf("webp poster: %q, type %s, %v", head, webp.MediaType, err)
	}
	if _, err := s.Image(ctx, f.P6, f.Rated(12).ID, "poster", "w400", false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible title: err = %v", err)
	}
	if err := os.Remove(filepath.Join(f.CacheDir, filepath.FromSlash(m.PosterRel))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Image(ctx, f.P6, m.ID, "poster", "w400", false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("row without file: err = %v, want ErrNotFound", err)
	}
}

func discoverFixture(t *testing.T) *vistest.Fixture {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddAudio(t)
	f.AddDiscover(t)
	return f
}

func exec(t *testing.T, f *vistest.Fixture, query string, args ...any) {
	t.Helper()
	if _, err := f.DB.Write.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func rowOf(rows []model.HomeRow, key string) model.HomeRow {
	for _, r := range rows {
		if r.Key == key {
			return r
		}
	}
	return model.HomeRow{}
}

func itemIDs(items []model.ItemSummary) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

// E-1, E-9, ADR 0011: all rows in the stored order; hidden and empty rows are omitted, the spotlight
// row also below spotlight_config.min_hits hits (5 after the migration); never more than eight rows.
func TestHomeStoredOrderAndSpotlightRow(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()

	rows, err := homeRows(ctx, s, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowPicks, RowContinue, RowRecent, RowWatchlist, RowCollections, RowGenres}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P18 rows = %v, want %v", rowKeys(rows), want)
	}
	if got := rowOf(rows, RowWatchlist).Items; !slices.Equal(itemIDs(got), []string{f.Rated(18).ID}) {
		t.Errorf("P18 watchlist row = %v", itemIDs(got))
	}
	if got := rowOf(rows, RowCollections); len(got.Items) != 0 || len(got.Collections) != 2 {
		t.Errorf("P18 collections row = %+v", got)
	}
	if got := rowOf(rows, RowGenres); len(got.Items) != 0 || len(got.Genres) != 1 || got.Genres[0].Name != "Abenteuer" {
		t.Errorf("P18 genres row = %+v", got)
	}

	// Spotlight: the trending movies are the films 1000..1004 (rated 0 to 18): five visible hits
	// for P18, four for P16, three for P12.
	for i := range 5 {
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, time.Now().UnixMilli())
	}
	for _, tc := range []struct {
		p     model.Profile
		shown bool
	}{{f.P18, true}, {f.P16, false}, {f.P12, false}} {
		rows, err := homeRows(ctx, s, tc.p)
		if err != nil {
			t.Fatal(err)
		}
		spotlight := rowOf(rows, RowSpotlight)
		if tc.shown != (len(spotlight.Items) >= 5) || (!tc.shown && spotlight.Key != "") {
			t.Errorf("%s spotlight row = %v, want shown %v", tc.p.Name, itemIDs(spotlight.Items), tc.shown)
		}
	}

	// The stored order applies, hidden rows are left out (the administration writes home_rows).
	// Reversed, released sits between watchlist and recent; no fixture title is dated inside its
	// window yet, so it is empty and omitted like any empty row.
	exec(t, f, `UPDATE home_rows SET position = 7 - position`)
	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key IN ('continue', 'genres')`)
	rows, err = homeRows(ctx, s, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowCollections, RowWatchlist, RowRecent, RowPicks, RowSpotlight}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P18 rows reordered = %v, want %v", rowKeys(rows), want)
	}
	// The eight fixed rows shown (E-9; the rows of concept 15 P-7 stay hidden): every row of every
	// profile fixture fits. With the film rated 0 released a month ago, P18 gets all eight rows in
	// the reversed order.
	exec(t, f, `UPDATE home_rows SET hidden = 0 WHERE key NOT IN ('libraries', 'library')`)
	f.SetReleaseDate(t, f.Rated(0).ID, time.Now().UTC().AddDate(0, -1, 0).Format(time.DateOnly))
	for _, p := range []model.Profile{f.P0, f.P6, f.P12, f.P16, f.P18} {
		rows, err := homeRows(ctx, s, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) > MaxRows {
			t.Errorf("%s: %d rows", p.Name, len(rows))
		}
		want := []string{RowGenres, RowCollections, RowWatchlist, RowReleased, RowRecent, RowContinue, RowPicks, RowSpotlight}
		if p.ID == f.P18.ID && !slices.Equal(rowKeys(rows), want) {
			t.Errorf("P18 with every row filled: rows = %v, want %v", rowKeys(rows), want)
		}
	}
}

// E-2 through the service clock (concept 18 part B): a pick stays in the row until the retention
// after it has passed; E-3: the pick of a title above the profile's ceiling never reaches it.
func TestHomePicksClockAndRated(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	picked := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	exec(t, f, `INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (?, ?, ?), (?, ?, ?)`,
		f.Rated(0).ID, f.P0.ID, picked.UnixMilli(), f.Rated(18).ID, f.P12.ID, vistest.PickedAt+1)
	for _, tc := range []struct {
		at   time.Time
		want []string
	}{
		{picked.Add(60*24*time.Hour - time.Millisecond), []string{f.Rated(6).ID, f.Rated(12).ID, f.Rated(0).ID}},
		{picked.Add(60 * 24 * time.Hour), []string{f.Rated(6).ID, f.Rated(12).ID}},
	} {
		s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return tc.at }))
		rows, err := homeRows(ctx, s, f.P12)
		if err != nil {
			t.Fatal(err)
		}
		if got := itemIDs(rowOf(rows, RowPicks).Items); !slices.Equal(got, tc.want) {
			t.Errorf("P12 picks at %s = %v, want %v", tc.at, got, tc.want)
		}
	}
}

// Concept 18 part B (14.4, 14.9, U-F7, U-F10): recommending stores or renews the own pick and
// answers its expiry, an expired own pick is renewed (not taken back), taking back removes only the
// own pick, and an invisible or unknown title, a season, an extra and a profile outside the owner's
// account all answer model.ErrNotFound without writing.
func TestRecommendAndUnrecommend(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return now }))
	m0 := f.Rated(0).ID
	own := func(p model.Profile, id string) bool {
		t.Helper()
		d, err := s.Item(ctx, p, id)
		if err != nil {
			t.Fatal(err)
		}
		return d.RecommendedByMe
	}
	count := func(id string) (n int) {
		t.Helper()
		if err := f.DB.Read.QueryRowContext(ctx, `SELECT count(*) FROM user_picks WHERE item_id = ?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// An expired own pick (the fixture's pick of P18 on the film rated 0) is not a recommendation;
	// recommending renews it.
	if own(f.P18, m0) {
		t.Fatal("P18 recommendedByMe for an expired pick")
	}
	pick, err := s.Recommend(ctx, f.P18, true, m0)
	if err != nil || pick.RetentionDays != 60 || !pick.ExpiresAt.Equal(now.Add(60*24*time.Hour)) {
		t.Fatalf("Recommend = %+v, %v; want 60 days from now", pick, err)
	}
	if !own(f.P18, m0) || count(m0) != 1 {
		t.Errorf("after renewing: recommendedByMe %v, %d rows; want true and one row", own(f.P18, m0), count(m0))
	}
	// Twice is idempotent; the retention of the settings travels.
	exec(t, f, `UPDATE picks_config SET retention_days = 30`)
	if pick, err = s.Recommend(ctx, f.P18, true, m0); err != nil || pick.RetentionDays != 30 || count(m0) != 1 {
		t.Errorf("second Recommend = %+v, %v, %d rows", pick, err, count(m0))
	}
	// P0 recommends the same title; P18 takes its own back: the title stays for P0's pick.
	if _, err := s.Recommend(ctx, f.P0, true, m0); err != nil {
		t.Fatal(err)
	}
	if err := s.Unrecommend(ctx, f.P18, true, m0); err != nil {
		t.Fatal(err)
	}
	if err := s.Unrecommend(ctx, f.P18, true, m0); err != nil { // idempotent
		t.Fatal(err)
	}
	if own(f.P18, m0) || !own(f.P0, m0) || count(m0) != 1 {
		t.Errorf("after taking back: P18 %v, P0 %v, %d rows; want false, true, one row", own(f.P18, m0), own(f.P0, m0), count(m0))
	}
	picks, _, err := s.PicksPage(ctx, f.P6, 0, 100)
	if err != nil || !slices.Contains(itemIDs(picks), m0) {
		t.Errorf("P6 picks = %v (%v), want the film rated 0 picked by P0", itemIDs(picks), err)
	}

	before := count(f.Rated(18).ID) + count(f.Show.Season.ID) + count(f.Show.Extra.ID)
	for name, tc := range map[string]struct {
		p     model.Profile
		owner bool
		id    string
	}{
		"invisible":     {f.P6, true, f.Rated(18).ID},
		"unknown":       {f.P18, true, "01J9000000000000000000ZZZZ"},
		"season":        {f.P18, true, f.Show.Season.ID},
		"extra":         {f.P18, true, f.Show.Extra.ID},
		"other account": {f.P18, false, f.Rated(18).ID},
	} {
		if _, err := s.Recommend(ctx, tc.p, tc.owner, tc.id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("Recommend %s = %v, want not found", name, err)
		}
		if err := s.Unrecommend(ctx, tc.p, tc.owner, tc.id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("Unrecommend %s = %v, want not found", name, err)
		}
	}
	if after := count(f.Rated(18).ID) + count(f.Show.Season.ID) + count(f.Show.Extra.ID); after != before {
		t.Errorf("refused recommendations wrote rows: %d before, %d after", before, after)
	}
}

// E-5, E-8: watchlist and favourites belong to the profile; invisible titles answer not found,
// seasons and extras are not listable, only series and podcasts are favourites.
func TestWatchlistAndFavoriteWrites(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	now := time.UnixMilli(50_000)
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()

	if err := s.AddToWatchlist(ctx, f.P6, f.Rated(0).ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AddToWatchlist(ctx, f.P6, f.Rated(0).ID); err != nil { // idempotent
		t.Fatal(err)
	}
	items, total, err := s.Watchlist(ctx, f.P6, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(0).ID, f.Rated(6).ID}; !slices.Equal(itemIDs(items), want) || total != 2 {
		t.Errorf("P6 watchlist = %v (%d), want %v newest first", itemIDs(items), total, want)
	}
	items, _, err = s.Watchlist(ctx, f.P18, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(18).ID}; !slices.Equal(itemIDs(items), want) {
		t.Errorf("P18 watchlist = %v, want %v (separate from P6)", itemIDs(items), want)
	}
	for _, tc := range []struct {
		name string
		p    model.Profile
		id   string
		want error
	}{
		{"invisible", f.P6, f.Rated(12).ID, model.ErrNotFound},
		{"unknown", f.P18, "01J0000000000000000000NONE", model.ErrNotFound},
		{"season", f.P18, f.Show.Season.ID, model.ErrNotListable},
		{"extra", f.P18, f.Show.Extra.ID, model.ErrNotListable},
		{"episode", f.P18, f.Show.E1.ID, nil},
		{"podcast", f.P18, f.Audio.Podcast.ID, nil},
	} {
		if err := s.AddToWatchlist(ctx, tc.p, tc.id); !errors.Is(err, tc.want) {
			t.Errorf("add %s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	if err := s.RemoveFromWatchlist(ctx, f.P6, f.Rated(12).ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("remove invisible: err = %v", err)
	}
	if err := s.RemoveFromWatchlist(ctx, f.P6, f.Rated(0).ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveFromWatchlist(ctx, f.P6, f.Rated(0).ID); err != nil { // idempotent
		t.Fatal(err)
	}
	if items, _, err := s.Watchlist(ctx, f.P6, 0, 0); err != nil || !slices.Equal(itemIDs(items), []string{f.Rated(6).ID}) {
		t.Errorf("P6 watchlist after remove = %v, %v", itemIDs(items), err)
	}

	for _, tc := range []struct {
		name string
		p    model.Profile
		id   string
		want error
	}{
		{"movie", f.P18, f.Rated(6).ID, model.ErrNotFavoritable},
		{"episode", f.P18, f.Show.E1.ID, model.ErrNotFavoritable},
		{"audiobook", f.P18, f.Audio.Books[0].ID, model.ErrNotFavoritable},
		{"invisible series", f.P6, f.Show.Series.ID, model.ErrNotFound},
		{"invisible podcast", f.P12, f.Audio.ExplicitPodcast.ID, model.ErrNotFound},
		{"series", f.P12, f.Show.Series.ID, nil},
		{"podcast", f.P6, f.Audio.Podcast.ID, nil},
	} {
		if err := s.AddFavorite(ctx, tc.p, tc.id); !errors.Is(err, tc.want) {
			t.Errorf("favourite %s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct {
		p    model.Profile
		want []string
	}{{f.P12, []string{f.Show.Series.ID}}, {f.P6, []string{f.Audio.Podcast.ID}}, {f.P18, []string{f.Show.Series.ID}}} {
		favs, err := s.Favorites(ctx, tc.p)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(itemIDs(favs), tc.want) {
			t.Errorf("%s favourites = %v, want %v", tc.p.Name, itemIDs(favs), tc.want)
		}
	}
	if err := s.RemoveFavorite(ctx, f.P12, f.Show.Series.ID); err != nil {
		t.Fatal(err)
	}
	if favs, err := s.Favorites(ctx, f.P12); err != nil || len(favs) != 0 {
		t.Errorf("P12 favourites after remove = %v, %v", itemIDs(favs), err)
	}
	if err := s.RemoveFavorite(ctx, f.P6, f.Show.Series.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("remove invisible favourite: err = %v", err)
	}
}

// Plan 5.3 and 8.1, "Mindesttreffer": min_hits counts the hits after dedup and before the cut to
// item_count, not the shown titles. The row appears at exactly min_hits hits, is absent at one
// fewer, and min_hits = 0 shows it with a single hit. The key case of the plan: item_count 5 and
// min_hits 10 mean "show 5 if there are at least 10", so 12 hits show 5 titles and 9 hits none;
// counting after the cut, the row would never appear.
func TestSpotlightMinHits(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	now := time.Date(2026, 10, 21, 4, 0, 0, 0, time.UTC)
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	f.AddSpotlightFilms(t, 12)
	for _, tc := range []struct {
		name                 string
		minHits, count, hits int
		want                 int // titles shown, 0 = row absent
	}{
		{"exactly min_hits", 5, 20, 5, 5},
		{"one fewer than min_hits", 5, 20, 4, 0},
		{"min_hits 0 with a single hit", 0, 20, 1, 1},
		{"exactly min_hits 10 above item_count 5", 10, 5, 10, 5},
		{"min_hits 10, item_count 5, 12 hits show 5 titles", 10, 5, 12, 5},
		{"min_hits 10, item_count 5, 9 hits show none", 10, 5, 9, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec(t, f, `UPDATE spotlight_config SET min_hits = ?, item_count = ?`, tc.minHits, tc.count)
			exec(t, f, `DELETE FROM spotlight_entries`)
			for k := 1; k <= tc.hits; k++ {
				exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, k, 8000+k, now.Add(-time.Hour).UnixMilli())
			}
			rows, err := homeRows(ctx, s, f.P18)
			if err != nil {
				t.Fatal(err)
			}
			got := rowOf(rows, RowSpotlight)
			if (got.Key != "") != (tc.want > 0) || len(got.Items) != tc.want {
				t.Errorf("spotlight row = %v (key %q), want %d titles", itemIDs(got.Items), got.Key, tc.want)
			}
		})
	}
}

// DoD step 2, "Ausfall kostet nur die Reihe": a failed run empties spotlight_entries; the spotlight
// row disappears and every other row of the answer is equal field by field to the answer before.
// (P18 has seven other rows here, released included: the film rated 0 is dated inside its window.)
func TestSpotlightOutageLeavesEverythingElse(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	now := time.Date(2026, 10, 21, 4, 0, 0, 0, time.UTC)
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	// The trending movies are the fixture films 1000..1004: five hits for P18, min_hits is 5.
	f.SetReleaseDate(t, f.Rated(0).ID, "2026-09-01")
	for i := range 5 {
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, now.Add(-time.Hour).UnixMilli())
	}
	before, err := homeRows(ctx, s, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowOf(before, RowSpotlight); len(got.Items) != 5 {
		t.Fatalf("spotlight row before the outage = %v, want 5 titles", itemIDs(got.Items))
	}
	exec(t, f, `DELETE FROM spotlight_entries`) // what a failed run leaves behind
	after, err := homeRows(ctx, s, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowOf(after, RowSpotlight); got.Key != "" {
		t.Errorf("spotlight row after the outage = %+v, want it absent", got)
	}
	others := slices.DeleteFunc(slices.Clone(before), func(r model.HomeRow) bool { return r.Key == RowSpotlight })
	if len(others) != 7 {
		t.Fatalf("rows besides spotlight before the outage = %v, want 7", rowKeys(others))
	}
	if !reflect.DeepEqual(after, others) {
		t.Errorf("rows after the outage differ from the run before:\n got  %+v\n want %+v", after, others)
	}
}

// M11 step 3, S-5, E-9: the row released shows the visible titles dated in the last twelve months
// by the service clock, in its stored place (default order: spotlight, picks, continue, recent,
// released, watchlist, collections, genres), and is omitted while no visible title is dated.
func TestHomeReleasedRow(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()

	// The fixture titles are dated 2010 to 2015 (films) and after 2030 (series, audio): no row.
	for _, p := range []model.Profile{f.P0, f.P12, f.P18} {
		rows, err := homeRows(ctx, s, p)
		if err != nil {
			t.Fatal(err)
		}
		if got := rowOf(rows, RowReleased); got.Key != "" {
			t.Errorf("%s released row without dated titles = %v, want it absent", p.Name, itemIDs(got.Items))
		}
	}

	// The film rated 12 released in July: P12 sees the row behind recent, P6 does not see the film.
	f.SetReleaseDate(t, f.Rated(12).ID, "2026-07-01")
	rows, err := homeRows(ctx, s, f.P12)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowPicks, RowRecent, RowReleased, RowCollections, RowGenres}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P12 rows = %v, want %v", rowKeys(rows), want)
	}
	if got := itemIDs(rowOf(rows, RowReleased).Items); !slices.Equal(got, []string{f.Rated(12).ID}) {
		t.Errorf("P12 released row = %v, want the film rated 12", got)
	}
	rows, err = homeRows(ctx, s, f.P6)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowOf(rows, RowReleased); got.Key != "" {
		t.Errorf("P6 released row = %v, want it absent (the film rated 12 is invisible)", itemIDs(got.Items))
	}
}

// spotlightFixture is discoverFixture with the trending movies 1000..1004 as spotlight entries:
// five hits for P18, enough for the default min_hits of 5.
func spotlightFixture(t *testing.T) *vistest.Fixture {
	t.Helper()
	f := discoverFixture(t)
	for i := range 5 {
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, time.Now().UnixMilli())
	}
	return f
}

// Plan step 4 DoD "Bühnenregel" and 8.1 "heroRow" (3.5): a filled spotlight row is the stage; an
// empty, hidden or disabled one gives no stage, whatever picks holds (picks is never the stage); a
// reordered page keeps spotlight as the stage at its stored place. Step 4 DoD "FirstHero im leeren
// Fall": FirstHero answers model.ErrNotFound without a stage, else the first spotlight title and
// whether the row comes first, after hidden and empty rows are left out.
func TestHeroRowStageRule(t *testing.T) {
	t.Parallel()
	f := spotlightFixture(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()

	page, err := s.Home(ctx, f.P18, nil)
	if err != nil {
		t.Fatal(err)
	}
	spotlight := rowOf(page.Rows, RowSpotlight).Items
	if page.HeroRow != RowSpotlight || len(spotlight) != 5 || page.Rows[0].Key != RowSpotlight {
		t.Fatalf("filled spotlight: hero row %q, rows %v", page.HeroRow, rowKeys(page.Rows))
	}
	// FirstHero draws like Home (concept 08, 6): the title at the drawn index, any of the row.
	hero, k, first, err := s.FirstHero(ctx, f.P18)
	if err != nil || k < 0 || k >= len(spotlight) || hero.ID != spotlight[k].ID || !first {
		t.Errorf("FirstHero = %s at %d, first %v, %v; want a spotlight title first", hero.ID, k, first, err)
	}
	fixed := New(f.DB, f.CacheDir, vistest.Logger(), WithHeroStart(3))
	if hero, k, _, err := fixed.FirstHero(ctx, f.P18); err != nil || k != 3 || hero.ID != spotlight[3].ID {
		t.Errorf("WithHeroStart(3): FirstHero = %s at %d, %v; want %s at 3", hero.ID, k, err, spotlight[3].ID)
	}

	noStage := func(name string) {
		t.Helper()
		page, err := s.Home(ctx, f.P18, nil)
		if err != nil {
			t.Fatal(err)
		}
		if page.HeroRow != "" || rowOf(page.Rows, RowSpotlight).Key != "" || len(rowOf(page.Rows, RowPicks).Items) == 0 {
			t.Errorf("%s: hero row %q, rows %v; want no stage and picks as a row", name, page.HeroRow, rowKeys(page.Rows))
		}
		if _, _, _, err := s.FirstHero(ctx, f.P18); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: FirstHero err = %v, want ErrNotFound", name, err)
		}
	}
	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key = 'spotlight'`)
	noStage("hidden spotlight")
	exec(t, f, `UPDATE home_rows SET hidden = 0 WHERE key NOT IN ('libraries', 'library')`)
	exec(t, f, `UPDATE spotlight_config SET enabled = 0`)
	noStage("disabled spotlight")
	exec(t, f, `UPDATE spotlight_config SET enabled = 1, min_hits = 6`)
	noStage("spotlight below min_hits")
	exec(t, f, `UPDATE spotlight_config SET min_hits = 5`)
	exec(t, f, `DELETE FROM spotlight_entries`)
	noStage("empty spotlight")
	for i := range 5 {
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, time.Now().UnixMilli())
	}

	// Spotlight and recent swap places (positions 0 and 3): the stage moves to index 3 of the rows
	// (spotlight, picks, continue, recent were the first four; released is empty for P18).
	exec(t, f, `UPDATE home_rows SET position = CASE key WHEN 'spotlight' THEN 3 WHEN 'recent' THEN 0 ELSE position END`)
	page, err = s.Home(ctx, f.P18, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowRecent, RowPicks, RowContinue, RowSpotlight}; page.HeroRow != RowSpotlight || len(page.Rows) < 4 || !slices.Equal(rowKeys(page.Rows)[:4], want) {
		t.Errorf("reordered: hero row %q, rows %v, want spotlight behind %v", page.HeroRow, rowKeys(page.Rows), want[:3])
	}
	hero, k, first, err = s.FirstHero(ctx, f.P18)
	if err != nil || hero.ID != spotlight[k].ID || first {
		t.Errorf("reordered FirstHero = %s at %d, first %v, %v; want a spotlight title not first", hero.ID, k, first, err)
	}
	// Hidden rows in front of the stage are left out: with recent, picks and continue hidden, the
	// stage is the first row again.
	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key IN ('recent', 'picks', 'continue')`)
	if page, err = s.Home(ctx, f.P18, nil); err != nil || page.HeroRow != RowSpotlight || page.Rows[0].Key != RowSpotlight {
		t.Errorf("rows in front hidden: hero row %q, rows %v, %v", page.HeroRow, rowKeys(page.Rows), err)
	}
	hero, k, first, err = s.FirstHero(ctx, f.P18)
	if err != nil || hero.ID != spotlight[k].ID || !first {
		t.Errorf("rows in front hidden: FirstHero = %s at %d, first %v, %v; want a spotlight title first", hero.ID, k, first, err)
	}
	// Empty rows in front are left out the same way: P0 sees no pick and has no progress, and with
	// min_hits 0 its single visible spotlight hit (the film rated 0) still makes a stage.
	exec(t, f, `UPDATE home_rows SET hidden = 0 WHERE key IN ('picks', 'continue')`)
	exec(t, f, `UPDATE spotlight_config SET min_hits = 0`)
	page, err = s.Home(ctx, f.P0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if page.HeroRow != RowSpotlight || page.Rows[0].Key != RowSpotlight {
		t.Fatalf("P0 with picks and continue empty: hero row %q, rows %v", page.HeroRow, rowKeys(page.Rows))
	}
	if hero, k, first, err = s.FirstHero(ctx, f.P0); err != nil || k >= len(page.Rows[0].Items) || hero.ID != page.Rows[0].Items[k].ID || !first {
		t.Errorf("P0 FirstHero = %s at %d, first %v, %v; want a title of %v first", hero.ID, k, first, err, page.Rows[0].Items)
	}
}

// P-10: without a stage FirstRow answers the titles of the first row, the cards the start page
// loads first; with a stage (on top or further down), without rows and with a first row of genres
// it answers model.ErrNotFound.
func TestFirstRow(t *testing.T) {
	t.Parallel()
	f := spotlightFixture(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()

	notFound := func(name string) {
		t.Helper()
		if row, err := s.FirstRow(ctx, f.P18); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: FirstRow = %d titles, %v; want ErrNotFound", name, len(row.Items), err)
		}
	}
	notFound("stage on top")
	exec(t, f, `UPDATE home_rows SET position = CASE key WHEN 'spotlight' THEN 3 WHEN 'recent' THEN 0 ELSE position END`)
	notFound("stage further down")
	// An empty spotlight row further down is no stage: FirstRow stops at the first row and still
	// answers what Home shows first.
	exec(t, f, `DELETE FROM spotlight_entries`)
	if page, err := s.Home(ctx, f.P18, nil); err != nil || page.HeroRow != "" || len(page.Rows) == 0 {
		t.Fatalf("spotlight empty: hero row %q, rows %v, %v", page.HeroRow, rowKeys(page.Rows), err)
	} else if first, err := s.FirstRow(ctx, f.P18); err != nil || first.Key != page.Rows[0].Key ||
		!slices.Equal(itemIDs(first.Items), itemIDs(page.Rows[0].Items)) {
		t.Errorf("spotlight empty: FirstRow = %s %v (%v), want %s %v", first.Key, itemIDs(first.Items), err,
			page.Rows[0].Key, itemIDs(page.Rows[0].Items))
	}

	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key = 'spotlight'`)
	page, err := s.Home(ctx, f.P18, nil)
	if err != nil {
		t.Fatal(err)
	}
	if page.HeroRow != "" || len(page.Rows) == 0 || page.Rows[0].Key != RowRecent || len(page.Rows[0].Items) < 2 {
		t.Fatalf("no stage: hero row %q, rows %v; want recent first with several titles", page.HeroRow, rowKeys(page.Rows))
	}
	first, err := s.FirstRow(ctx, f.P18)
	if err != nil {
		t.Fatalf("no stage: FirstRow: %v", err)
	}
	if got, want := itemIDs(first.Items), itemIDs(page.Rows[0].Items); !slices.Equal(got, want) {
		t.Errorf("no stage: FirstRow = %v, want %v", got, want)
	}

	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key <> 'genres'`)
	if page, err := s.Home(ctx, f.P18, nil); err != nil || len(page.Rows) != 1 || page.Rows[0].Key != RowGenres {
		t.Fatalf("genres only: rows %v, %v", rowKeys(page.Rows), err)
	}
	notFound("genres first")
	exec(t, f, `UPDATE home_rows SET hidden = 1`)
	notFound("no rows")
}

// Plan 3.6 (D3) and step 4: HasTrailer is set on every title of the rows spotlight and picks, true
// exactly for the titles with a stored trailer, and stays nil in every other row.
func TestHomeHasTrailerOnlyInSpotlightAndPicks(t *testing.T) {
	t.Parallel()
	f := spotlightFixture(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	// The film rated 0 released a month ago fills the row released.
	f.SetReleaseDate(t, f.Rated(0).ID, time.Now().UTC().AddDate(0, -1, 0).Format(time.DateOnly))
	page, err := s.Home(ctx, f.P18, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Only the first title of each flagged row keeps a trailer (AddSeries stores some of its own).
	exec(t, f, `DELETE FROM trailers`)
	withTrailer := map[string]bool{}
	for _, key := range []string{RowSpotlight, RowPicks} {
		items := rowOf(page.Rows, key).Items
		if len(items) < 2 {
			t.Fatalf("row %s = %v, want at least two titles", key, itemIDs(items))
		}
		exec(t, f, `INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at) VALUES (?, 'en', ?, 'Trailer', 'tmdb', 1, 1)`, items[0].ID, "Flag"+key)
		withTrailer[items[0].ID] = true
	}
	if page, err = s.Home(ctx, f.P18, nil); err != nil {
		t.Fatal(err)
	}
	if want := []string{RowSpotlight, RowPicks, RowContinue, RowRecent, RowReleased, RowWatchlist, RowCollections, RowGenres}; !slices.Equal(rowKeys(page.Rows), want) {
		t.Fatalf("rows = %v, want %v", rowKeys(page.Rows), want)
	}
	for _, row := range page.Rows {
		flagged := row.Key == RowSpotlight || row.Key == RowPicks
		with, without := false, false
		for _, it := range row.Items {
			switch {
			case !flagged && it.HasTrailer != nil:
				t.Errorf("row %s, %s: HasTrailer = %v, want nil", row.Key, it.ID, *it.HasTrailer)
			case flagged && it.HasTrailer == nil:
				t.Errorf("row %s, %s: HasTrailer nil", row.Key, it.ID)
			case flagged && *it.HasTrailer != withTrailer[it.ID]:
				t.Errorf("row %s, %s: HasTrailer = %v, want %v", row.Key, it.ID, *it.HasTrailer, withTrailer[it.ID])
			case flagged && *it.HasTrailer:
				with = true
			case flagged:
				without = true
			}
		}
		if flagged && (!with || !without) {
			t.Errorf("row %s = %v: want titles with and without a trailer", row.Key, itemIDs(row.Items))
		}
	}
}

// Concept 08, F-2 and F-10 (ADR 0016): with a stage, Home names the title it begins with, drawn
// once per page view from the Service's random source; a start inside the stage row repeats an
// earlier draw, one outside it draws again; WithHeroStart fixes the index. The rhythm comes from
// spotlight_config.rotation_seconds. Without a stage both stay unset.
func TestHomeHeroStart(t *testing.T) {
	t.Parallel()
	f := spotlightFixture(t)
	ctx := context.Background()
	exec(t, f, `UPDATE spotlight_config SET rotation_seconds = 7`)
	draws := func(seed uint64) []int {
		s := New(f.DB, f.CacheDir, vistest.Logger(), WithRand(rand.New(rand.NewPCG(seed, seed))))
		var out []int
		for range 8 {
			page, err := s.Home(ctx, f.P18, nil)
			if err != nil {
				t.Fatal(err)
			}
			if page.HeroStart == nil || *page.HeroStart < 0 || *page.HeroStart >= 5 || page.HeroIntervalSeconds != 7 {
				t.Fatalf("hero start %v, interval %d; want an index in [0, 5) and 7 s", page.HeroStart, page.HeroIntervalSeconds)
			}
			out = append(out, *page.HeroStart)
		}
		return out
	}
	first := draws(42)
	if again := draws(42); !slices.Equal(first, again) {
		t.Errorf("same seed: draws %v and %v, want the same", first, again)
	}
	if slices.Equal(first, []int{0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Errorf("draws %v always start at place 1", first)
	}

	s := New(f.DB, f.CacheDir, vistest.Logger(), WithRand(rand.New(rand.NewPCG(1, 2))))
	for _, start := range []int{0, 3, 4} {
		page, err := s.Home(ctx, f.P18, &start)
		if err != nil || page.HeroStart == nil || *page.HeroStart != start {
			t.Errorf("start %d: hero start %v, %v; want it repeated", start, page.HeroStart, err)
		}
	}
	for _, start := range []int{-1, 5, 99} {
		page, err := s.Home(ctx, f.P18, &start)
		if err != nil || page.HeroStart == nil || *page.HeroStart < 0 || *page.HeroStart >= 5 {
			t.Errorf("start %d outside the row: hero start %v, %v; want a draw in [0, 5)", start, page.HeroStart, err)
		}
	}
	// A fixed start (e2e): always that index, the last title beyond the row, a valid start wins.
	for _, tc := range []struct {
		fixed int
		start *int
		want  int
	}{{0, nil, 0}, {2, nil, 2}, {9, nil, 4}, {0, func() *int { k := 3; return &k }(), 3}} {
		fixed := New(f.DB, f.CacheDir, vistest.Logger(), WithHeroStart(tc.fixed))
		for range 3 {
			page, err := fixed.Home(ctx, f.P18, tc.start)
			if err != nil || page.HeroStart == nil || *page.HeroStart != tc.want {
				t.Errorf("fixed %d, start %v: hero start %v, %v; want %d", tc.fixed, tc.start, page.HeroStart, err, tc.want)
			}
		}
	}
	// FirstHero is the shell's draw (concept 08, 6): the title at its index, which Home repeats.
	for range 4 {
		hero, k, _, err := s.FirstHero(ctx, f.P18)
		if err != nil || k < 0 || k >= 5 {
			t.Fatalf("FirstHero = %s at %d, %v; want an index in [0, 5)", hero.ID, k, err)
		}
		page, err := s.Home(ctx, f.P18, &k)
		if err != nil || page.HeroStart == nil || *page.HeroStart != k || rowOf(page.Rows, RowSpotlight).Items[k].ID != hero.ID {
			t.Errorf("Home(start %d) = %v, %v; want the shell's title %s", k, page.HeroStart, err, hero.ID)
		}
	}
	if hero, k, _, err := New(f.DB, f.CacheDir, vistest.Logger(), WithHeroStart(9)).FirstHero(ctx, f.P18); err != nil || k != 4 ||
		hero.ID != rowOf(mustHome(t, s, f.P18).Rows, RowSpotlight).Items[4].ID {
		t.Errorf("WithHeroStart(9): FirstHero = %s at %d, %v; want the last title", hero.ID, k, err)
	}
	// Without a stage: no start, no rhythm, whatever start says.
	exec(t, f, `UPDATE spotlight_config SET enabled = 0`)
	start := 0
	page, err := s.Home(ctx, f.P18, &start)
	if err != nil || page.HeroRow != "" || page.HeroStart != nil || page.HeroIntervalSeconds != 0 {
		t.Errorf("no stage: hero row %q, start %v, interval %d, %v; want none", page.HeroRow, page.HeroStart, page.HeroIntervalSeconds, err)
	}
}

func mustHome(t *testing.T, s *Service, p model.Profile) model.HomePage {
	t.Helper()
	page, err := s.Home(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

// The library views pass through to visibility without hasTrailer (only the start page rows
// spotlight and picks carry it).
func TestLibraryViewsWithoutTrailerFlags(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	libs, err := s.VisibleLibraries(ctx, f.P18)
	if err != nil || len(libs) == 0 || libs[0].ID != f.LibraryID() {
		t.Fatalf("P18 libraries = %+v, %v; want library 1 first", libs, err)
	}
	page, err := s.LibraryItems(ctx, f.P18, f.LibraryID(), model.LibraryQuery{MaxRating: -1})
	if err != nil || page.Total != libs[0].ItemCount || len(page.Items) != page.Total {
		t.Fatalf("P18 library 1 = %d of %d, %v; want %d", len(page.Items), page.Total, err, libs[0].ItemCount)
	}
	picks, total, err := s.PicksPage(ctx, f.P18, 0, 0)
	if err != nil || total == 0 || len(picks) != total {
		t.Fatalf("P18 picks = %d of %d, %v", len(picks), total, err)
	}
	for _, it := range slices.Concat(page.Items, picks) {
		if it.HasTrailer != nil {
			t.Errorf("%s carries hasTrailer on a library view", it.ID)
		}
	}
}

// Concept 15 P-1: the row continue of the start page holds one entry per series. Two started
// episodes give the newer one; once it is completed the series stays with the next episode (no
// progress yet), and a completed last episode removes the series. With continue on top, FirstRow
// hands the shell those entries with the series and its poster.
func TestHomeContinueOneEntryPerSeries(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	show, now := f.Show, time.Now().UnixMilli()
	progress := func(itemID string, completed int, at int64) {
		exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, ?, ?, ?)
		            ON CONFLICT (profile_id, item_id) DO UPDATE SET completed = excluded.completed, updated_at = excluded.updated_at`,
			f.P18.ID, itemID, vistest.DurationMs, completed, at)
	}
	continueRow := func(name string, want ...string) {
		t.Helper()
		rows, err := homeRows(ctx, s, f.P18)
		if err != nil {
			t.Fatal(err)
		}
		if got := itemIDs(rowOf(rows, RowContinue).Items); !slices.Equal(got, want) {
			t.Errorf("%s: continue = %v, want %v", name, got, want)
		}
	}
	film := f.Rated(12).ID // started by the seed, long ago
	progress(show.E1.ID, 0, now-2000)
	progress(show.E2.ID, 0, now-1000)
	continueRow("two started episodes", show.E2.ID, film)
	progress(show.E2.ID, 1, now-500)
	continueRow("newest episode completed", show.E3.ID, film)
	progress(show.E3.ID, 1, now-100)
	continueRow("last episode completed", film)

	progress(show.E1.ID, 1, now) // rewatched E1: E2 and E3 are completed, so the series is gone
	continueRow("every next episode completed", film)
	exec(t, f, `DELETE FROM progress WHERE profile_id = ? AND item_id IN (?, ?)`, f.P18.ID, show.E2.ID, show.E3.ID)
	continueRow("next episode not started", show.E2.ID, film)

	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key = 'picks'`)
	first, err := s.FirstRow(ctx, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	items := first.Items
	if got := itemIDs(items); !slices.Equal(got, []string{show.E2.ID, film}) {
		t.Fatalf("FirstRow = %v, want the continue row", got)
	}
	if e := items[0]; e.Progress != nil || e.SeriesRef == nil || e.SeriesRef.ID != show.Series.ID || e.SeriesRef.Poster == nil || items[1].SeriesRef != nil {
		t.Errorf("FirstRow entries = %+v / %+v, want the next episode without progress with the series poster, the film without series", e, items[1].SeriesRef)
	}
}

// Concept 15 P-1: only the row continue shows an episode with its series, so FirstRow keeps
// SeriesRef there and drops it in every other row (the prepaint follows the card).
func TestFirstRowCardsSeriesOnlyInContinue(t *testing.T) {
	t.Parallel()
	series := &model.SeriesRef{ID: "s1", Title: "Show"}
	items := []model.ItemSummary{{ID: "e1", Kind: "episode", SeriesRef: series}, {ID: "f1", Kind: "movie"}}
	if got := firstRowCards(model.HomeRow{Key: RowContinue, Items: items}); got[0].SeriesRef != series {
		t.Errorf("continue: SeriesRef = %v, want the series", got[0].SeriesRef)
	}
	got := firstRowCards(model.HomeRow{Key: RowRecent, Items: items})
	if got[0].SeriesRef != nil {
		t.Errorf("recent: SeriesRef = %v, want none", got[0].SeriesRef)
	}
	if items[0].SeriesRef != series {
		t.Error("recent: the row's own titles were changed")
	}
}

// Concept 15 P-7: «Meine Medien» holds one tile per library the profile sees, in the order of the
// side navigation, with the number of its visible titles and its four newest; «Neu in …» holds the
// newest visible titles of its library like the library page sorted by added, at most 20. A library
// the profile does not see has neither tile nor row; a library row carries its name; FirstRow hands
// the tiles to the prepaint when «Meine Medien» comes first.
func TestHomeLibraryRows(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddEdgeCases(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	lib, other := f.LibraryID(), f.OtherLibraryID()
	exec(t, f, `UPDATE home_rows SET hidden = 1`)
	exec(t, f, `UPDATE home_rows SET hidden = 0, position = CASE WHEN id = 'libraries' THEN -1 ELSE position END WHERE key IN ('libraries', 'library')`)

	for _, p := range []model.Profile{f.P18, f.PX, f.P6} {
		page, err := s.Home(ctx, p, nil)
		if err != nil {
			t.Fatal(err)
		}
		navs, err := s.VisibleLibraries(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		var wantIDs []string
		for _, n := range navs {
			wantIDs = append(wantIDs, fmt.Sprintf("library:%d", n.ID))
		}
		var ids []string
		for _, r := range page.Rows {
			ids = append(ids, r.ID)
		}
		if want := append([]string{RowLibraries}, wantIDs...); !slices.Equal(ids, want) {
			t.Fatalf("%s rows %v, want %v", p.Name, ids, want)
		}
		tiles := page.Rows[0].Libraries
		if len(tiles) != len(navs) || len(page.Rows[0].Items) != 0 {
			t.Fatalf("%s tiles %+v, want one per visible library %+v", p.Name, tiles, navs)
		}
		for i, tile := range tiles {
			newest, err := s.LibraryItems(ctx, p, tile.ID, model.LibraryQuery{Sort: model.LibrarySortAdded, MaxRating: -1, Limit: 20})
			if err != nil {
				t.Fatal(err)
			}
			if tile.ID != navs[i].ID || tile.Name != navs[i].Name || tile.Kind != navs[i].Kind || tile.ItemCount != navs[i].ItemCount ||
				!slices.Equal(itemIDs(tile.Items), itemIDs(newest.Items[:min(4, len(newest.Items))])) {
				t.Errorf("%s tile %+v, want %+v with the four newest %v", p.Name, tile, navs[i], itemIDs(newest.Items))
			}
			row := page.Rows[1+i]
			if row.Key != RowLibrary || row.LibraryID != tile.ID || row.LibraryName != tile.Name ||
				!slices.Equal(itemIDs(row.Items), itemIDs(newest.Items)) {
				t.Errorf("%s row %s = %v (%d %q), want the library page by added %v", p.Name, row.ID, itemIDs(row.Items), row.LibraryID, row.LibraryName, itemIDs(newest.Items))
			}
		}
		if p.ID == f.PX.ID && slices.Contains(ids, fmt.Sprintf("library:%d", other)) {
			t.Errorf("PX sees the row of library X: %v", ids)
		}
		if p.ID == f.P18.ID && !slices.Contains(ids, fmt.Sprintf("library:%d", other)) {
			t.Errorf("P18 misses the row of library X: %v", ids)
		}

		first, err := s.FirstRow(ctx, p)
		if err != nil || first.Key != RowLibraries || len(first.Libraries) != len(tiles) {
			t.Errorf("%s FirstRow = %+v (%v), want «Meine Medien» with its tiles", p.Name, first, err)
		}
	}
	if lib == other {
		t.Fatal("fixture libraries collide")
	}
}

// A row loads in a goroutine of loadRows, out of reach of the recoverer of the HTTP handler: a
// panic in it becomes the row's error with its stack, so the page fails (500) and the process lives.
func TestHomeRowPanicFailsThePage(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	row := s.loadRow
	s.loadRow = func(ctx context.Context, profile model.Profile, setting model.HomeRowSetting) (model.HomeRow, error) {
		if setting.Key == RowRecent {
			panic("row exploded")
		}
		return row(ctx, profile, setting)
	}
	_, err := s.Home(context.Background(), f.P6, nil)
	if err == nil || !strings.Contains(err.Error(), "home row recent: panic: row exploded") || !strings.Contains(err.Error(), "goroutine") {
		t.Errorf("Home with a panicking row: err = %v, want the panic with its stack", err)
	}
}

// rowsFixture returns a service over 30 shown rows (spotlight sixth, behind five other rows) whose
// rows come from load instead of the database, the order of those rows and the fixture.
func rowsFixture(t *testing.T, load func(ctx context.Context, i int, setting model.HomeRowSetting) (model.HomeRow, error)) (*Service, []model.HomeRowSetting, *vistest.Fixture) {
	t.Helper()
	f := vistest.New(t)
	for i := range 30 {
		exec(t, f, `INSERT INTO libraries (name, kind, created_at, updated_at) VALUES (?, 'video', 0, 0)`, fmt.Sprintf("Library %d", i))
	}
	exec(t, f, `UPDATE home_rows SET hidden = 0, position = position * 10`)
	exec(t, f, `UPDATE home_rows SET position = (SELECT position FROM home_rows WHERE id = 'watchlist') + 5 WHERE id = 'spotlight'`)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	order, err := s.rowOrder(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(order) < 30 || order[5].Key != RowSpotlight {
		t.Fatalf("order %v, want at least 30 rows with spotlight sixth", order)
	}
	index := make(map[string]int, len(order))
	for i, r := range order {
		index[r.ID] = i
	}
	s.loadRow = func(ctx context.Context, _ model.Profile, setting model.HomeRowSetting) (model.HomeRow, error) {
		return load(ctx, index[setting.ID], setting)
	}
	return s, order, f
}

// rowIDs returns the ids of rows.
func rowIDs(rows []model.HomeRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// settingIDs returns the ids of the rows of order at the indexes idx.
func settingIDs(order []model.HomeRowSetting, idx ...int) []string {
	out := make([]string, 0, len(idx))
	for _, i := range idx {
		out = append(out, order[i].ID)
	}
	return out
}

// span returns the indexes from..to-1 without skip.
func span(from, to int, skip ...int) []int {
	var out []int
	for i := from; i < to; i++ {
		if !slices.Contains(skip, i) {
			out = append(out, i)
		}
	}
	return out
}

// The rows of the start page load side by side but count as one after the other (loadRows, page):
// the error or panic of a row fails the page only when the page reaches the row, ahead of the cut
// to MaxRows filled rows or, with untilHero, up to the spotlight row; empty rows are skipped and
// do not count towards the cut.
func TestHomeRowErrorsAroundTheCut(t *testing.T) {
	t.Parallel()
	errRow := errors.New("row failed")
	for _, tc := range []struct {
		name      string
		untilHero bool
		fail      []int // rows that fail
		panics    []int // rows that panic
		empty     []int // rows without titles
		want      []int // the rows of the page; nil with wantErr
		wantHero  bool
		wantErr   string
	}{
		{name: "error ahead of the cut", fail: []int{3}, wantErr: "row failed"},
		{name: "panic ahead of the cut", panics: []int{14}, wantErr: "panic: row 14"},
		{name: "errors behind the cut", fail: []int{15, 29}, panics: []int{20}, want: span(0, 15), wantHero: true},
		{name: "empty rows move the cut", empty: []int{2, 7}, fail: []int{17}, want: span(0, 17, 2, 7), wantHero: true},
		{name: "error at the moved cut", empty: []int{2, 7}, fail: []int{16}, wantErr: "row failed"},
		{name: "empty rows only", empty: span(0, 100), want: []int{}},
		{name: "until the stage", untilHero: true, fail: []int{6, 20}, panics: []int{7}, want: span(0, 6), wantHero: true},
		{name: "error ahead of the stage", untilHero: true, fail: []int{4}, wantErr: "row failed"},
		{name: "empty stage", untilHero: true, empty: []int{5}, fail: []int{6}, want: span(0, 5)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, order, _ := rowsFixture(t, func(_ context.Context, i int, setting model.HomeRowSetting) (model.HomeRow, error) {
				switch {
				case slices.Contains(tc.fail, i):
					return model.HomeRow{}, errRow
				case slices.Contains(tc.panics, i):
					panic(fmt.Sprintf("row %d", i))
				case slices.Contains(tc.empty, i):
					return model.HomeRow{ID: setting.ID, Key: setting.Key}, nil
				}
				return model.HomeRow{ID: setting.ID, Key: setting.Key, Items: []model.ItemSummary{{ID: setting.ID}}}, nil
			})
			page, err := s.page(context.Background(), model.Profile{}, tc.untilHero)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := settingIDs(order, tc.want...); !slices.Equal(rowIDs(page.Rows), want) {
				t.Errorf("rows %v, want %v", rowIDs(page.Rows), want)
			}
			if hero := page.HeroRow == RowSpotlight; hero != tc.wantHero {
				t.Errorf("HeroRow = %q, want stage %v", page.HeroRow, tc.wantHero)
			}
		})
	}
}

// Once the page holds MaxRows rows (or reached the stage, with untilHero) the rows still loading
// are cancelled: here every row behind the cut waits for its context, so the page returns only
// through that cancellation, well before the deadline of the request. Without a spotlight row a
// page until the stage loads no row at all.
func TestHomeStopsLoadingBehindThePage(t *testing.T) {
	t.Parallel()
	var loaded atomic.Int32
	s, order, f := rowsFixture(t, func(ctx context.Context, i int, setting model.HomeRowSetting) (model.HomeRow, error) {
		loaded.Add(1)
		if i >= MaxRows {
			<-ctx.Done()
			return model.HomeRow{}, ctx.Err()
		}
		return model.HomeRow{ID: setting.ID, Key: setting.Key, Items: []model.ItemSummary{{ID: setting.ID}}}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, err := s.page(ctx, model.Profile{}, false)
	if err != nil || ctx.Err() != nil {
		t.Fatalf("page: err %v, request %v, want the rows behind the cut cancelled", err, ctx.Err())
	}
	if want := settingIDs(order, span(0, MaxRows)...); !slices.Equal(rowIDs(page.Rows), want) {
		t.Errorf("rows %v, want %v", rowIDs(page.Rows), want)
	}
	if n := int(loaded.Load()); n >= len(order) {
		t.Errorf("loaded %d of %d rows, want the rows not yet started skipped", n, len(order))
	}

	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE id = 'spotlight'`)
	loaded.Store(0)
	page, err = s.page(ctx, model.Profile{}, true)
	if err != nil || len(page.Rows) != 0 || page.HeroRow != "" || loaded.Load() != 0 {
		t.Errorf("until the stage without spotlight: page %+v, err %v, %d rows loaded, want an empty page and no row loaded", page, err, loaded.Load())
	}
}
