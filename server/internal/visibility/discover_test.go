package visibility

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func discoverFixture(t *testing.T) *vistest.Fixture {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddDiscover(t)
	return f
}

func exec(t *testing.T, f *vistest.Fixture, query string, args ...any) {
	t.Helper()
	if _, err := f.DB.Write.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// E-2: a pick is visible only inside its time window, by the Reader's clock.
func TestPicksTimeWindowClock(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	m0 := f.Rated(0).ID
	for _, tc := range []struct {
		name string
		at   int64
		want bool
	}{
		{"inside the expired window", vistest.ExpiredPickEnd - 1, true},
		{"at the end of the expired window", vistest.ExpiredPickEnd, false},
		{"between both windows", time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC).UnixMilli(), false},
		{"before the future window", vistest.FuturePickStart - 1, false},
		{"at the start of the future window", vistest.FuturePickStart, true},
	} {
		at := time.UnixMilli(tc.at)
		r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return at }))
		picks, err := r.Picks(ctx, Profile(f.P0), 20)
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(ids(picks), m0); got != tc.want {
			t.Errorf("%s: P0 picks = %v, want the film rated 0 %v", tc.name, ids(picks), tc.want)
		}
	}
}

// E-1, E-2, E-3: a pick for one profile carries its note and reaches only that profile; a pick of
// a title above the profile's ceiling never appears, not even when it targets the profile.
func TestPicksPerProfileWithNote(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	// A pick of the film rated 18 only for P12 (E-3 negative test) and a global pick of the film
	// rated 12 with another note at the front: the own pick of P12 (place and note) wins over it.
	exec(t, f, `INSERT INTO top_picks (item_id, profile_id, position, note, created_at) VALUES (?, ?, 0, 'zu alt', 1)`, f.Rated(18).ID, f.P12.ID)
	exec(t, f, `INSERT INTO top_picks (item_id, position, note, created_at) VALUES (?, 0, 'global', 1)`, f.Rated(12).ID)

	picks, err := r.Picks(ctx, Profile(f.P12), 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(6).ID, f.Rated(12).ID}; !slices.Equal(ids(picks), want) {
		t.Fatalf("P12 picks = %v, want %v", ids(picks), want)
	}
	if picks[0].Note != "" || picks[1].Note != f.Discover.PickNote {
		t.Errorf("P12 notes = %q, %q, want none and %q", picks[0].Note, picks[1].Note, f.Discover.PickNote)
	}
	picks, err = r.Picks(ctx, Profile(f.P16), 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(12).ID, f.Rated(6).ID}; !slices.Equal(ids(picks), want) || picks[0].Note != "global" {
		t.Errorf("P16 picks = %v (%+v), want %v with the global note", ids(picks), picks, want)
	}
	picks, err = r.Picks(ctx, Profile(f.P18), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) == 0 || picks[0].ID != f.Rated(18).ID || picks[0].Note != "" {
		t.Errorf("P18 picks = %v, want the global pick of the film rated 18 first, without the note of P12", ids(picks))
	}
}

func TestWatchlistFavoritesAndDetail(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	for _, tc := range []struct {
		p    model.Profile
		want []string
	}{{f.P6, []string{f.Rated(6).ID}}, {f.P18, []string{f.Rated(18).ID}}, {f.P12, nil}} {
		items, total, err := r.Watchlist(ctx, Profile(tc.p), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(items), tc.want) || total != len(tc.want) {
			t.Errorf("%s watchlist = %v (%d), want %v", tc.p.Name, ids(items), total, tc.want)
		}
	}
	// An entry whose title is above the profile's ceiling is left out (P6 with the film rated 18).
	exec(t, f, `INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (?, ?, 20)`, f.P6.ID, f.Rated(18).ID)
	items, total, err := r.Watchlist(ctx, Profile(f.P6), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(6).ID}; !slices.Equal(ids(items), want) || total != 1 {
		t.Errorf("P6 watchlist with an invisible entry = %v (%d), want %v", ids(items), total, want)
	}
	// The start page row reads the same entries without counting (k2), newest first, cut at limit.
	exec(t, f, `INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (?, ?, 30)`, f.P6.ID, f.Rated(0).ID)
	row, err := r.WatchlistRow(ctx, Profile(f.P6), 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(0).ID, f.Rated(6).ID}; !slices.Equal(ids(row), want) {
		t.Errorf("P6 watchlist row = %v, want %v", ids(row), want)
	}
	if row, err = r.WatchlistRow(ctx, Profile(f.P6), 1); err != nil || !slices.Equal(ids(row), []string{f.Rated(0).ID}) {
		t.Errorf("P6 watchlist row limit 1 = %v, %v", ids(row), err)
	}

	favs, err := r.Favorites(ctx, Profile(f.P18))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Show.Series.ID}; !slices.Equal(ids(favs), want) {
		t.Errorf("P18 favourites = %v, want %v", ids(favs), want)
	}

	d, err := r.Item(ctx, Profile(f.P18), f.Rated(18).ID)
	if err != nil {
		t.Fatal(err)
	}
	if !d.InWatchlist || d.Favorite || len(d.Collections) != 2 {
		t.Errorf("P18 detail of the film rated 18: inWatchlist %v favorite %v collections %+v", d.InWatchlist, d.Favorite, d.Collections)
	}
	d, err = r.Item(ctx, Profile(f.P18), f.Show.Series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.InWatchlist || !d.Favorite || d.Collections == nil || len(d.Collections) != 0 {
		t.Errorf("P18 detail of the series: inWatchlist %v favorite %v collections %+v", d.InWatchlist, d.Favorite, d.Collections)
	}
	// P12 sees the mixed collection with two members on the detail of the film rated 6, by its German name.
	d, err = r.Item(ctx, Profile(f.P12), f.Rated(6).ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Collections) != 1 || d.Collections[0].Name != vistest.MixedCollectionDe || d.Collections[0].Count != 2 ||
		d.Collections[0].Cover == nil || d.Collections[0].Cover.ID != f.Rated(6).ID {
		t.Errorf("P12 collections of the film rated 6 = %+v", d.Collections)
	}
}

// E-6, 10.2: a collection counts only visible members and exists only through one.
func TestCollectionsCountVisibleMembers(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	for _, tc := range []struct {
		p       model.Profile
		members []string
		manual  bool
		name    string
	}{
		{f.P0, nil, false, ""},
		{f.P6, []string{f.Rated(6).ID}, false, vistest.MixedCollectionName},
		{f.P12, []string{f.Rated(6).ID, f.Rated(12).ID}, false, vistest.MixedCollectionDe},
		{f.P18, []string{f.Rated(6).ID, f.Rated(12).ID, f.Rated(18).ID}, true, vistest.MixedCollectionDe},
	} {
		c, err := r.Collection(ctx, Profile(tc.p), vistest.MixedCollectionID, 0, 0)
		if len(tc.members) == 0 {
			if !errors.Is(err, model.ErrNotFound) {
				t.Errorf("%s: collection without a visible member: err = %v", tc.p.Name, err)
			}
		} else if err != nil {
			t.Fatal(err)
		} else if !slices.Equal(ids(c.Items), tc.members) || c.Total != len(tc.members) || c.Name != tc.name {
			t.Errorf("%s collection = %+v, want %v named %q", tc.p.Name, c, tc.members, tc.name)
		}
		_, err = r.Collection(ctx, Profile(tc.p), vistest.ManualCollectionID, 0, 0)
		if tc.manual != (err == nil) {
			t.Errorf("%s manual collection: err = %v, want visible %v", tc.p.Name, err, tc.manual)
		}
		refs, err := r.Collections(ctx, Profile(tc.p), 20)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if len(tc.members) > 0 {
			want++
		}
		if tc.manual {
			want++
		}
		if len(refs) != want {
			t.Errorf("%s collections = %+v, want %d", tc.p.Name, refs, want)
		}
		for _, ref := range refs {
			if ref.ID == vistest.MixedCollectionID && (ref.Count != len(tc.members) || ref.Cover == nil || ref.Cover.ID != f.Rated(6).ID) {
				t.Errorf("%s mixed collection ref = %+v", tc.p.Name, ref)
			}
		}
	}
	// Paging: the second member of P18 alone.
	c, err := r.Collection(ctx, Profile(f.P18), vistest.MixedCollectionID, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(12).ID}; !slices.Equal(ids(c.Items), want) || c.Total != 3 {
		t.Errorf("P18 page 2 = %v (%d), want %v of 3", ids(c.Items), c.Total, want)
	}
}

func TestGenresOfVisibleTitles(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	exec(t, f, `UPDATE items SET genres = '{"de":["Abenteuer","Drama"],"en":["Adventure","Drama"]}' WHERE id = ?`, f.Rated(18).ID)
	for _, tc := range []struct {
		p    model.Profile
		want []model.GenreRef
	}{
		// P6 (en) sees the films rated 0 and 6; the series is rated 12.
		{f.P6, []model.GenreRef{{Name: "Adventure", Count: 2}}},
		// P18 (de) sees six films and the series; only the film rated 18 is a drama.
		{f.P18, []model.GenreRef{{Name: "Abenteuer", Count: 7}, {Name: "Drama", Count: 1}}},
	} {
		got, err := r.Genres(ctx, Profile(tc.p), 20)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s genres = %+v, want %+v", tc.p.Name, got, tc.want)
		}
	}
}

// AN-3, ADR 0011: the spotlight row shows the visible library titles of the configured list with
// their TMDB ids in list order, movies as films and shows as series only; entries of another list
// key do not show.
func TestSpotlightRowIntersection(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	now := fetched.Add(time.Hour)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	// Positions 1..5 of the trending movies are the fixture films 1000..1004 (ratings 0..18); the
	// show at position 1 is the series (tmdb 1020); a movie entry with the series' id, a show entry
	// with a film's id and an id outside the library match nothing.
	for i := range 5 {
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, fetched.UnixMilli())
	}
	exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES
	            ('trending', 'show', 1, 1020, ?1), ('trending', 'movie', 6, 1020, ?1), ('trending', 'show', 2, 1001, ?1),
	            ('trending', 'movie', 7, 424242, ?1), ('popular', 'movie', 1, 1000, ?1)`, fetched.UnixMilli())
	series := f.Show.Series.ID
	for _, tc := range []struct {
		p    model.Profile
		want []string
	}{
		{f.P18, []string{f.Rated(0).ID, series, f.Rated(6).ID, f.Rated(12).ID, f.Rated(16).ID, f.Rated(18).ID}},
		{f.P12, []string{f.Rated(0).ID, series, f.Rated(6).ID, f.Rated(12).ID}},
		{f.P0, []string{f.Rated(0).ID}},
	} {
		got, err := r.SpotlightRow(ctx, Profile(tc.p), 20)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), tc.want) {
			t.Errorf("%s trending = %v, want %v", tc.p.Name, ids(got), tc.want)
		}
	}
	// The configured list key selects the entries.
	exec(t, f, `UPDATE spotlight_config SET list_key = 'popular'`)
	got, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(0).ID}; !slices.Equal(ids(got), want) {
		t.Errorf("P18 popular = %v, want %v", ids(got), want)
	}
	exec(t, f, `UPDATE spotlight_config SET list_key = 'trending'`)
	// The limit keeps the list order.
	got, err = r.SpotlightRow(ctx, Profile(f.P18), 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(0).ID, series}; !slices.Equal(ids(got), want) {
		t.Errorf("P18 trending limit 2 = %v, want %v", ids(got), want)
	}
	// W1, S-8: entries fetched more than twice spotlight_config.interval_hours ago are stale (the
	// refresh stopped) and show nothing; the boundary is inclusive and follows the configuration.
	for _, hours := range []int{6, 48} {
		exec(t, f, `UPDATE spotlight_config SET interval_hours = ?`, hours)
		limit := 2 * time.Duration(hours) * time.Hour
		for _, tc := range []struct {
			age  time.Duration
			want int
		}{{limit, 6}, {limit + time.Millisecond, 0}} {
			at := fetched.Add(tc.age)
			stale := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return at }))
			got, err := stale.SpotlightRow(ctx, Profile(f.P18), 20)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Errorf("interval %dh: P18 trending %v after the fetch = %v, want %d titles", hours, tc.age, ids(got), tc.want)
			}
		}
	}
}

// E-7, AN-3: the spotlight row shows at most the first 20 library hits in list order, even when
// more of the list is in the library and the caller asks for more.
func TestSpotlightRowAtMostTwenty(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	var want []string
	for i := range 25 {
		id := fmt.Sprintf("01J9TRAKT0000000000000%04d", i)
		exec(t, f, `INSERT INTO items (id, kind, library_id, title, sort_title, genres, tmdb_id, own_rating, metadata_state, added_at, created_at, updated_at)
		            VALUES (?, 'movie', ?, ?, ?, '[]', ?, 0, 'matched', 1, 1, 1)`, id, f.LibraryID(), fmt.Sprintf("Trakt %02d", i), fmt.Sprintf("trakt %02d", i), 8000+i)
		exec(t, f, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec, width, height, duration_ms, created_at, updated_at)
		            VALUES (?, ?, ?, ?, 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, 1000, 1, 1)`, fmt.Sprintf("01J9TRAKTF00000000000%05d", i), id, f.RootID(), fmt.Sprintf("trakt-%02d.mp4", i))
		// Positions run backwards, so the row order must come from the list, not from the ids.
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, 100-i, 8000+i, fetched.UnixMilli())
	}
	for i := 24; i >= 5; i-- {
		want = append(want, fmt.Sprintf("01J9TRAKT0000000000000%04d", i))
	}
	got, err := r.SpotlightRow(ctx, Profile(f.P0), 50)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(got), want) {
		t.Errorf("spotlight row = %d titles %v, want the first 20 of the list %v", len(got), ids(got), want)
	}
}
