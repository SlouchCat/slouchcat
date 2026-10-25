package visibility

import (
	"context"
	"database/sql"
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
		got, hits, err := r.SpotlightRow(ctx, Profile(tc.p), 20)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), tc.want) || hits != len(tc.want) {
			t.Errorf("%s trending = %v (%d hits), want %v", tc.p.Name, ids(got), hits, tc.want)
		}
	}
	// The configured list key selects the entries.
	exec(t, f, `UPDATE spotlight_config SET list_key = 'popular'`)
	got, _, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(0).ID}; !slices.Equal(ids(got), want) {
		t.Errorf("P18 popular = %v, want %v", ids(got), want)
	}
	exec(t, f, `UPDATE spotlight_config SET list_key = 'trending'`)
	// The limit keeps the list order; the hits are counted before the cut.
	got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(0).ID, series}; !slices.Equal(ids(got), want) || hits != 6 {
		t.Errorf("P18 trending limit 2 = %v (%d hits), want %v of 6 hits", ids(got), hits, want)
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
			got, hits, err := stale.SpotlightRow(ctx, Profile(f.P18), 20)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want || hits != tc.want {
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
	got, hits, err := r.SpotlightRow(ctx, Profile(f.P0), 50)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(got), want) || hits != 25 {
		t.Errorf("spotlight row = %d titles %v (%d hits), want the first 20 of the list %v of 25 hits", len(got), ids(got), hits, want)
	}
}

// spotlightFilms adds n films rated 0 (visible to every profile) with TMDB ids 8001..8000+n and
// lists them as trending movies in natural order: film k (1-based) at position k. It returns the
// item ids, film k at index k-1.
func spotlightFilms(t *testing.T, f *vistest.Fixture, n int, fetched time.Time) []string {
	t.Helper()
	out := make([]string, 0, n)
	for k := 1; k <= n; k++ {
		id := fmt.Sprintf("01J9SPOTLIGHT00000000%05d", k)
		exec(t, f, `INSERT INTO items (id, kind, library_id, title, sort_title, genres, tmdb_id, own_rating, metadata_state, added_at, created_at, updated_at)
		            VALUES (?, 'movie', ?, ?, ?, '[]', ?, 0, 'matched', 1, 1, 1)`, id, f.LibraryID(), fmt.Sprintf("Spotlight %02d", k), fmt.Sprintf("spotlight %02d", k), 8000+k)
		exec(t, f, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec, width, height, duration_ms, created_at, updated_at)
		            VALUES (?, ?, ?, ?, 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, 1000, 1, 1)`, fmt.Sprintf("01J9SPOTFILE000000000%05d", k), id, f.RootID(), fmt.Sprintf("spotlight-%02d.mp4", k))
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, k, 8000+k, fetched.UnixMilli())
		out = append(out, id)
	}
	return out
}

// override writes the curation of the trending movie with TMDB id 8000+k (film k of spotlightFilms);
// rank 0 means not pinned.
func override(t *testing.T, f *vistest.Fixture, k int, hidden bool, rank int, note string) {
	t.Helper()
	exec(t, f, `INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, pinned_rank, note, updated_at)
	            VALUES ('trending', 'movie', ?, ?, ?, ?, 1)`,
		8000+k, hidden, sql.NullInt64{Int64: int64(rank), Valid: rank > 0}, sql.NullString{String: note, Valid: note != ""})
}

// films maps 1-based film numbers to the item ids of spotlightFilms.
func films(all []string, ks ...int) []string {
	out := make([]string, 0, len(ks))
	for _, k := range ks {
		out = append(out, all[k-1])
	}
	return out
}

// Plan 5.3, "pinned_rank, angewandt als Einfügen — nicht als Sortierschlüssel": the row is the
// natural list order with the pinned entries INSERTED at their rank, in the sequence dedup, insert,
// cut. Every case starts from six library hits in natural order 1..6.
func TestSpotlightRowPinnedRankIsAnInsertion(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	all := spotlightFilms(t, f, 6, fetched)
	type pin struct{ film, rank int }
	for _, tc := range []struct {
		name string
		pins []pin
		want []int
	}{
		// The failure of the first version: position + offset made "entry 5 to place 1" tie with
		// entry 1, which won, so the moved entry stuck on place 2.
		{"entry 5 to place 1 (failure of the first version)", []pin{{5, 1}}, []int{5, 1, 2, 3, 4, 6}},
		// The failure of the second version: sorting pinned before free put "entry 1 to place 5"
		// on place 1, exactly where it was moved away from.
		{"entry 1 to place 5 (failure of the second version)", []pin{{1, 5}}, []int{2, 3, 4, 5, 1, 6}},
		{"a single entry with rank 3 lands on place 3, not place 1", []pin{{6, 3}}, []int{1, 2, 6, 3, 4, 5}},
		// [A, x, B, y, z]: two pinned in their rank order, the free ones keep theirs around them.
		{"two pinned in their rank order beside free entries", []pin{{6, 1}, {1, 3}}, []int{6, 2, 1, 3, 4, 5}},
		{"a rank beyond the list appends at the end", []pin{{2, 10}}, []int{1, 3, 4, 5, 6, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec(t, f, `DELETE FROM spotlight_overrides`)
			for _, p := range tc.pins {
				override(t, f, p.film, false, p.rank, "")
			}
			got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
			if err != nil {
				t.Fatal(err)
			}
			if want := films(all, tc.want...); !slices.Equal(ids(got), want) || hits != 6 {
				t.Errorf("row = %v (%d hits), want %v (6 hits)", ids(got), hits, want)
			}
		})
	}

	// "Reihenfolge der Schritte": dedup runs before the insertion. A movie and a show with the same
	// TMDB id are two different items, so the duplicate that reaches dedup is the same title listed
	// twice; both rows join the same override and are pinned. It appears once, on its rank.
	t.Run("a title listed twice and pinned appears once (dedup before insert)", func(t *testing.T) {
		exec(t, f, `DELETE FROM spotlight_overrides`)
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', 7, 8004, ?)`, fetched.UnixMilli())
		t.Cleanup(func() { exec(t, f, `DELETE FROM spotlight_entries WHERE position = 7`) })
		override(t, f, 4, false, 2, "")
		got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
		if err != nil {
			t.Fatal(err)
		}
		if want := films(all, 1, 4, 2, 3, 5, 6); !slices.Equal(ids(got), want) || hits != 6 {
			t.Errorf("row = %v (%d hits), want %v (6 hits)", ids(got), hits, want)
		}
	})

	// The note travels as ItemSummary.Note (ItemRef.note) on its title, and only there.
	t.Run("the note travels with its title", func(t *testing.T) {
		exec(t, f, `DELETE FROM spotlight_overrides`)
		override(t, f, 2, false, 0, "Marcos Tipp")
		got, _, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), all) {
			t.Fatalf("row = %v, want the natural order %v", ids(got), all)
		}
		for i, it := range got {
			want := ""
			if i == 1 {
				want = "Marcos Tipp"
			}
			if it.Note != want {
				t.Errorf("note of film %d = %q, want %q", i+1, it.Note, want)
			}
		}
	})

	// An orphaned override (its title is not in the list) changes nothing, its rank included.
	t.Run("an orphaned override changes nothing", func(t *testing.T) {
		exec(t, f, `DELETE FROM spotlight_overrides`)
		override(t, f, 50, false, 1, "verwaist")
		got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), all) || hits != 6 {
			t.Errorf("row = %v (%d hits), want the natural order %v", ids(got), hits, all)
		}
	})
}

// Plan 5.3: the cut to item_count comes last, and the hits are counted after dedup and before the
// cut (min_hits counts hits, not shown titles).
func TestSpotlightRowCutComesLast(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	all := spotlightFilms(t, f, 12, fetched)

	t.Run("hits are counted before the cut", func(t *testing.T) {
		got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 5)
		if err != nil {
			t.Fatal(err)
		}
		if want := films(all, 1, 2, 3, 4, 5); !slices.Equal(ids(got), want) || hits != 12 {
			t.Errorf("row = %v (%d hits), want %v of 12 hits", ids(got), hits, want)
		}
	})
	// Item count 5 and natural position 10 pinned to rank 5: cutting first would drop it.
	t.Run("a pinned entry takes its place before the cut", func(t *testing.T) {
		override(t, f, 10, false, 5, "")
		got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 5)
		if err != nil {
			t.Fatal(err)
		}
		if want := films(all, 1, 2, 3, 4, 10); !slices.Equal(ids(got), want) || hits != 12 {
			t.Errorf("row = %v (%d hits), want %v of 12 hits", ids(got), hits, want)
		}
	})
}

// DoD step 2, "Anheftung jenseits des alten Fensters": a library hit at natural position 55 pinned
// to rank 1 is on place 1 with item_count 20. The former 2 × n window fetched 40 rows and would have
// swallowed it silently; this case is the proof that the window is gone.
func TestSpotlightRowPinBeyondTheOldWindow(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	all := spotlightFilms(t, f, 60, fetched)
	override(t, f, 55, false, 1, "")
	got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Concat(films(all, 55), all[:19])
	if !slices.Equal(ids(got), want) || hits != 60 {
		t.Errorf("row = %v (%d hits), want %v of 60 hits", ids(got), hits, want)
	}
}

// DoD step 2, "hidden verbraucht nichts": 30 hidden entries at the head of the list do not shorten
// the row, because hidden filters in SQL and not in Go (plan 5.3).
func TestSpotlightRowHiddenConsumesNothing(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	all := spotlightFilms(t, f, 50, fetched)
	for k := 1; k <= 30; k++ {
		override(t, f, k, true, 0, "")
	}
	got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := all[30:]; !slices.Equal(ids(got), want) || hits != 20 {
		t.Errorf("row = %d titles %v (%d hits), want the 20 visible hits behind the hidden ones %v", len(got), ids(got), hits, want)
	}
}

// DoD step 2, "Kuratierung überlebt" (P4): an override hangs on the title (list, medium, TMDB id),
// never on a position, so a run that delivers the same titles in a completely different order keeps
// every override on its title.
func TestSpotlightOverrideSurvivesReorder(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	all := spotlightFilms(t, f, 6, fetched)
	override(t, f, 2, true, 0, "")         // hidden
	override(t, f, 3, false, 1, "")        // pinned to rank 1
	override(t, f, 5, false, 0, "Notiz 5") // note
	check := func(stage string, want []string) {
		t.Helper()
		got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), want) || hits != 5 {
			t.Errorf("%s: row = %v (%d hits), want %v (5 hits)", stage, ids(got), hits, want)
		}
		for _, it := range got {
			wantNote := ""
			if it.ID == all[4] {
				wantNote = "Notiz 5"
			}
			if it.Note != wantNote {
				t.Errorf("%s: note of %s = %q, want %q", stage, it.ID, it.Note, wantNote)
			}
		}
	}
	check("before the run", films(all, 3, 1, 4, 5, 6))

	// A run replaces the cache as a whole: the same titles, positions reversed (film k at 7-k).
	exec(t, f, `DELETE FROM spotlight_entries`)
	for k := 1; k <= 6; k++ {
		exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, 7-k, 8000+k, fetched.UnixMilli())
	}
	// Natural order now 6,5,4,3,2,1; film 2 stays hidden, film 3 stays on rank 1, the note on film 5.
	check("after the reordering run", films(all, 3, 6, 5, 4, 1))
}

// DoD step 2, "Listenwechsel": after a change of list_key no override of the old list applies, so
// the notes do not tip onto the titles of another list.
func TestSpotlightOverridesBelongToTheirList(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	all := spotlightFilms(t, f, 6, fetched)
	// The same titles in the same order under popular.
	exec(t, f, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at)
	            SELECT 'popular', media, position, tmdb_id, fetched_at FROM spotlight_entries WHERE list_key = 'trending'`)
	override(t, f, 1, true, 0, "")
	override(t, f, 6, false, 1, "")
	override(t, f, 2, false, 0, "nur trending")

	got, _, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := films(all, 6, 2, 3, 4, 5); !slices.Equal(ids(got), want) || got[1].Note != "nur trending" {
		t.Fatalf("trending row = %v (%+v), want %v with the note on film 2", ids(got), got, want)
	}
	exec(t, f, `UPDATE spotlight_config SET list_key = 'popular'`)
	got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(got), all) || hits != 6 {
		t.Errorf("popular row = %v (%d hits), want the natural order %v without the trending curation", ids(got), hits, all)
	}
	for _, it := range got {
		if it.Note != "" {
			t.Errorf("popular row: note %q on %s, want none", it.Note, it.ID)
		}
	}
}

// A disabled spotlight (spotlight_config.enabled = 0) gives no row and no hits, and no error.
func TestSpotlightRowDisabled(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	spotlightFilms(t, f, 6, fetched)
	exec(t, f, `UPDATE spotlight_config SET enabled = 0`)
	got, hits, err := r.SpotlightRow(ctx, Profile(f.P18), 20)
	if err != nil || got != nil || hits != 0 {
		t.Errorf("disabled spotlight = %v, %d hits, %v; want nil, 0, nil", ids(got), hits, err)
	}
}
