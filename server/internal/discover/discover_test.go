package discover

import (
	"context"
	"errors"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

func TestHomeRowsInOrderWithoutEmptyRows(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()

	rows, err := s.Home(ctx, f.P6)
	if err != nil {
		t.Fatal(err)
	}
	// Without watchlist, collections and Trakt lists those rows are omitted (E-9).
	if want := []string{RowPicks, RowContinue, RowRecent, RowGenres}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P6 rows = %v, want %v", rowKeys(rows), want)
	}
	// P0 has no progress and sees no pick: continue and picks are omitted.
	rows, err = s.Home(ctx, f.P0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowRecent, RowGenres}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P0 rows = %v, want %v", rowKeys(rows), want)
	}
}

func TestFirstPickIsFirstHeroSlide(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	for _, p := range []model.Profile{f.P18, f.P6} {
		rows, err := s.Home(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 || rows[0].Key != RowPicks {
			t.Fatalf("%s: rows = %v", p.Name, rowKeys(rows))
		}
		first, err := s.FirstPick(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if want := rows[0].Items[0]; first.ID != want.ID || first.BackdropSmall == nil || first.Backdrop == nil {
			t.Errorf("%s: first pick = %+v, want %s with both backdrops", p.Name, first, want.ID)
		}
	}
	// A profile without a visible pick (P0) or without visible titles has no carousel.
	for _, p := range []model.Profile{f.P0, {ID: 9999, Language: "de"}} {
		if _, err := s.FirstPick(ctx, p); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("profile %d without picks: err = %v", p.ID, err)
		}
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

	rows, err := s.Home(ctx, f.P18)
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
		rows, err := s.Home(ctx, tc.p)
		if err != nil {
			t.Fatal(err)
		}
		spotlight := rowOf(rows, RowSpotlight)
		if tc.shown != (len(spotlight.Items) >= 5) || (!tc.shown && spotlight.Key != "") {
			t.Errorf("%s spotlight row = %v, want shown %v", tc.p.Name, itemIDs(spotlight.Items), tc.shown)
		}
	}

	// The stored order applies, hidden rows are left out (the administration writes home_rows).
	// Reversed, released sits between watchlist and recent; it has no reader in this package yet,
	// so it is empty and omitted like any empty row.
	exec(t, f, `UPDATE home_rows SET position = 7 - position`)
	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key IN ('continue', 'genres')`)
	rows, err = s.Home(ctx, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{RowCollections, RowWatchlist, RowRecent, RowPicks, RowSpotlight}; !slices.Equal(rowKeys(rows), want) {
		t.Errorf("P18 rows reordered = %v, want %v", rowKeys(rows), want)
	}
	// A hidden picks row also drops the hero preload.
	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE key = 'picks'`)
	if _, err := s.FirstPick(ctx, f.P18); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("first pick of a hidden picks row: err = %v", err)
	}
	// Eight non-empty rows at most (E-9): every row of every profile fixture fits. P18 gets every
	// row but the empty released row, in the reversed order.
	exec(t, f, `UPDATE home_rows SET hidden = 0`)
	for _, p := range []model.Profile{f.P0, f.P6, f.P12, f.P16, f.P18} {
		rows, err := s.Home(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) > MaxRows {
			t.Errorf("%s: %d rows", p.Name, len(rows))
		}
		want := []string{RowGenres, RowCollections, RowWatchlist, RowRecent, RowContinue, RowPicks, RowSpotlight}
		if p.ID == f.P18.ID && !slices.Equal(rowKeys(rows), want) {
			t.Errorf("P18 with every row filled: rows = %v, want %v", rowKeys(rows), want)
		}
	}
}

// E-2 through the service clock; E-3: the FSK-18 pick never reaches P12; the note travels.
func TestHomePicksClockAndNote(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	exec(t, f, `INSERT INTO top_picks (item_id, profile_id, position, note, created_at) VALUES (?, ?, 0, 'zu alt', 1)`, f.Rated(18).ID, f.P12.ID)
	inWindow := time.UnixMilli(vistest.FuturePickStart + 1)
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return inWindow }))
	rows, err := s.Home(ctx, f.P12)
	if err != nil {
		t.Fatal(err)
	}
	picks := rowOf(rows, RowPicks).Items
	if want := []string{f.Rated(6).ID, f.Rated(12).ID, f.Rated(0).ID}; !slices.Equal(itemIDs(picks), want) {
		t.Fatalf("P12 picks in the future window = %v, want %v", itemIDs(picks), want)
	}
	if picks[1].Note != f.Discover.PickNote {
		t.Errorf("P12 pick note = %q", picks[1].Note)
	}
	s = New(f.DB, f.CacheDir, vistest.Logger())
	rows, err = s.Home(ctx, f.P12)
	if err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(rowOf(rows, RowPicks).Items); slices.Contains(got, f.Rated(0).ID) || slices.Contains(got, f.Rated(18).ID) {
		t.Errorf("P12 picks now = %v, want neither the future pick nor the FSK-18 pick", got)
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

// k1 (M7 review): a global pick of a title at the front and the profile's own pick of the same
// title further back: the own pick wins in Home, and FirstPick (limit 1) agrees with the first
// hero slide instead of returning the global row.
func TestFirstPickAfterDeduplication(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	s := New(f.DB, f.CacheDir, vistest.Logger())
	ctx := context.Background()
	// The fixture holds the own pick of P12 on the film rated 12 at position 5; a global pick of the
	// same film goes in front of every other pick.
	exec(t, f, `INSERT INTO top_picks (item_id, position, note, created_at) VALUES (?, -1, 'global', 1)`, f.Rated(12).ID)
	rows, err := s.Home(ctx, f.P12)
	if err != nil {
		t.Fatal(err)
	}
	picks := rowOf(rows, RowPicks).Items
	if want := []string{f.Rated(6).ID, f.Rated(12).ID}; !slices.Equal(itemIDs(picks), want) || picks[1].Note != f.Discover.PickNote {
		t.Fatalf("P12 picks = %v (%+v), want %v with the own note last", itemIDs(picks), picks, want)
	}
	first, err := s.FirstPick(ctx, f.P12)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != picks[0].ID {
		t.Errorf("FirstPick(P12) = %s, want the first hero slide %s", first.ID, picks[0].ID)
	}
	// A profile without an own pick of the film still gets the global one first.
	first, err = s.FirstPick(ctx, f.P16)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err = s.Home(ctx, f.P16); err != nil {
		t.Fatal(err)
	}
	if picks := rowOf(rows, RowPicks).Items; first.ID != f.Rated(12).ID || len(picks) == 0 || picks[0].ID != first.ID {
		t.Errorf("FirstPick(P16) = %s, Home picks = %v, want the global pick of the film rated 12 first", first.ID, itemIDs(picks))
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
			rows, err := s.Home(ctx, f.P18)
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
// (P18 has six other rows here: released has no reader in this package and stays empty.)
func TestSpotlightOutageLeavesEverythingElse(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	now := time.Date(2026, 10, 21, 4, 0, 0, 0, time.UTC)
	s := New(f.DB, f.CacheDir, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	// The trending movies are the fixture films 1000..1004: five hits for P18, min_hits is 5.
	for i := range 5 {
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, now.Add(-time.Hour).UnixMilli())
	}
	before, err := s.Home(ctx, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowOf(before, RowSpotlight); len(got.Items) != 5 {
		t.Fatalf("spotlight row before the outage = %v, want 5 titles", itemIDs(got.Items))
	}
	exec(t, f, `DELETE FROM spotlight_entries`) // what a failed run leaves behind
	after, err := s.Home(ctx, f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowOf(after, RowSpotlight); got.Key != "" {
		t.Errorf("spotlight row after the outage = %+v, want it absent", got)
	}
	others := slices.DeleteFunc(slices.Clone(before), func(r model.HomeRow) bool { return r.Key == RowSpotlight })
	if len(others) != 6 {
		t.Fatalf("rows besides spotlight before the outage = %v, want 6", rowKeys(others))
	}
	if !reflect.DeepEqual(after, others) {
		t.Errorf("rows after the outage differ from the run before:\n got  %+v\n want %+v", after, others)
	}
}
