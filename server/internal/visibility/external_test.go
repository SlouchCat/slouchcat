package visibility

import (
	"context"
	"errors"
	"maps"
	"strconv"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// externalFixture is the library with the series tree, the guest account and stored titles
// outside the library: film 5000 rated 12, film 5001 with an unknown age, film 5002 rated 16 and
// show 4242 with TVDB id 7777. The series S carries TVDB id 7777 and no TMDB id (a series matched
// TVDB first); a second series O carries TMDB id 4242. The film rated 0 carries IMDb id tt0000001.
func externalFixture(t *testing.T) (*vistest.Fixture, *Reader) {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddGuest(t)
	for _, q := range []string{
		`UPDATE items SET tmdb_id = NULL, tvdb_id = 7777 WHERE id = '` + f.Show.Series.ID + `'`,
		`UPDATE items SET imdb_id = 'tt0000001' WHERE id = '` + f.Rated(0).ID + `'`,
		`INSERT INTO items (id, kind, library_id, title, sort_title, tmdb_id, metadata_state, added_at, created_at, updated_at)
			VALUES ('01J9000000000000000000R0OS', 'series', ` + strconv.FormatInt(f.LibraryID(), 10) + `, 'series OS', 'series OS', 4242, 'matched', 1, 1, 1)`,
		`INSERT INTO external_titles (media, tmdb_id, min_age, fetched_at) VALUES ('movie', 5000, 12, 1)`,
		`INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('movie', 5001, 1)`,
		`INSERT INTO external_titles (media, tmdb_id, min_age, fetched_at) VALUES ('movie', 5002, 16, 1)`,
		`INSERT INTO external_titles (media, tmdb_id, tvdb_id, min_age, fetched_at) VALUES ('show', 4242, 7777, 12, 1)`,
		`INSERT INTO external_titles (media, tmdb_id, imdb_id, min_age, fetched_at) VALUES ('movie', 9998, 'tt0000001', 0, 1)`,
	} {
		if _, err := f.DB.Write.ExecContext(context.Background(), q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return f, New(f.DB, vistest.Logger())
}

func allowAll(string) bool { return true }

func movie(tmdb int64) ExternalRef { return ExternalRef{Media: MediaMovie, TMDBID: tmdb} }

// A title in the library is never missing, also not for a profile it is rated too high for: for
// that profile it does not exist (404), so a child cannot request it (AK-26).
func TestMissingTitleInTheLibraryRatedTooHigh(t *testing.T) {
	t.Parallel()
	f, r := externalFixture(t)
	ctx := context.Background()
	m16 := f.Rated(16)
	for _, p := range []model.Profile{f.P12, f.P6, f.P0, f.PG18} {
		if err := r.MissingTitle(ctx, Profile(p), allowAll, movie(1003)); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: MissingTitle(film rated 16 in the library) = %v, want ErrNotFound", p.Name, err)
		}
		got, err := r.MatchExternalTitles(ctx, Profile(p), allowAll, []ExternalRef{movie(1003)})
		if err != nil {
			t.Fatal(err)
		}
		if g := got[0]; !g.InLibrary || g.Missing || g.NeedsDetails || g.ItemID != "" {
			t.Errorf("%s: match = %+v, want in the library, not visible, not missing", p.Name, g)
		}
	}
	got, err := r.MatchExternalTitles(ctx, Profile(f.P18), allowAll, []ExternalRef{movie(1003)})
	if err != nil {
		t.Fatal(err)
	}
	if g := got[0]; !g.InLibrary || g.Missing || g.ItemID != m16.ID {
		t.Errorf("P18: match = %+v, want the library title %s", g, m16.ID)
	}
}

// The age of a title outside the library against min(profile, account); unknown counts as 18;
// a title without a stored row needs its details for a profile under 18; a profile without the
// request right sees no missing title (F-9).
func TestMissingTitleAgeAndRight(t *testing.T) {
	t.Parallel()
	f, r := externalFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		p       model.Profile
		ref     ExternalRef
		right   RequestRight
		missing bool
		details bool
	}{
		{"rated 12 for 12", f.P12, movie(5000), allowAll, true, false},
		{"rated 12 for 6", f.P6, movie(5000), allowAll, false, false},
		{"rated 16 for the guest profile 18 capped to 12 by the account", f.PG18, movie(5002), allowAll, false, false},
		{"rated 12 for the guest profile 18 capped to 12", f.PG18, movie(5000), allowAll, true, false},
		{"unknown age for 18", f.P18, movie(5001), allowAll, true, false},
		{"unknown age for 16", f.P16, movie(5001), allowAll, false, false},
		{"not stored for 18", f.P18, movie(5003), allowAll, true, false},
		{"not stored for 16", f.P16, movie(5003), allowAll, false, true},
		{"without the right", f.P18, movie(5000), nil, false, false},
		{"right for shows only", f.P18, movie(5000), func(m string) bool { return m == MediaShow }, false, false},
		{"a film id asked as a show", f.P18, ExternalRef{Media: MediaShow, TMDBID: 1000}, allowAll, true, false},
	} {
		got, err := r.MatchExternalTitles(ctx, Profile(tc.p), tc.right, []ExternalRef{tc.ref})
		if err != nil {
			t.Fatal(err)
		}
		if g := got[0]; g.InLibrary || g.Missing != tc.missing || g.NeedsDetails != tc.details {
			t.Errorf("%s: match = %+v, want missing %v, details %v", tc.name, g, tc.missing, tc.details)
		}
		err = r.MissingTitle(ctx, Profile(tc.p), tc.right, tc.ref)
		if tc.missing && err != nil || !tc.missing && !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: MissingTitle = %v, want missing %v", tc.name, err, tc.missing)
		}
	}
	if err := r.MissingTitle(ctx, System("test"), allowAll, movie(5000)); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: %v, want ErrUnscoped", err)
	}
	if err := r.MissingTitle(ctx, Profile(f.P18), allowAll, ExternalRef{Media: "series", TMDBID: 5000}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown medium: %v, want ErrNotFound", err)
	}
	// The batch keeps the order and answers invalid refs with nothing.
	got, err := r.MatchExternalTitles(ctx, Profile(f.P12), allowAll, []ExternalRef{movie(5002), {}, movie(5000), movie(1002)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].Missing || got[1].Missing || got[1].InLibrary || !got[2].Missing || got[3].ItemID != f.Rated(12).ID {
		t.Errorf("batch = %+v", got)
	}
}

// Matching by external ids: a film by TMDB id, then IMDb id (also an IMDb id stored in
// external_titles); a series matched TVDB first without a TMDB id is found by its TVDB id, also
// when only external_titles knows it; with a TVDB match and a different TMDB match the TVDB one
// wins.
func TestTitleByExternalIDs(t *testing.T) {
	t.Parallel()
	f, r := externalFixture(t)
	ctx := context.Background()
	series := f.Show.Series.ID
	for _, tc := range []struct {
		name string
		ref  ExternalRef
		want string
	}{
		{"film by TMDB", movie(1002), f.Rated(12).ID},
		{"film by IMDb", ExternalRef{Media: MediaMovie, TMDBID: 9999, IMDBID: "tt0000001"}, f.Rated(0).ID},
		{"film by the IMDb id of external_titles", movie(9998), f.Rated(0).ID},
		{"series without TMDB id by TVDB", ExternalRef{Media: MediaShow, TMDBID: 31337, TVDBID: 7777}, series},
		{"TVDB wins over TMDB", ExternalRef{Media: MediaShow, TMDBID: 4242, TVDBID: 7777}, series},
		{"TVDB from external_titles wins", ExternalRef{Media: MediaShow, TMDBID: 4242}, series},
		{"series by TMDB", ExternalRef{Media: MediaShow, TMDBID: 4242, TVDBID: 1}, "01J9000000000000000000R0OS"},
		{"film ids never match a series", ExternalRef{Media: MediaMovie, TMDBID: 4242}, ""},
		{"unknown", movie(77), ""},
		{"invalid", ExternalRef{Media: MediaShow}, ""},
	} {
		id, found, err := r.TitleByExternalIDs(ctx, tc.ref)
		if err != nil {
			t.Fatal(err)
		}
		if id != tc.want || found != (tc.want != "") {
			t.Errorf("%s: %q, %v; want %q", tc.name, id, found, tc.want)
		}
	}
	// The same ladder for a profile: the series S is in the library for everyone, P12 sees it, P6
	// does not (rated 12) and it is not missing for P6 either.
	show := ExternalRef{Media: MediaShow, TMDBID: 4242}
	for _, tc := range []struct {
		p    model.Profile
		want string
	}{{f.P12, series}, {f.P6, ""}} {
		got, err := r.MatchExternalTitles(ctx, Profile(tc.p), allowAll, []ExternalRef{show})
		if err != nil {
			t.Fatal(err)
		}
		if g := got[0]; !g.InLibrary || g.Missing || g.ItemID != tc.want {
			t.Errorf("%s: match = %+v, want item %q", tc.p.Name, g, tc.want)
		}
	}
}

// The playable seasons of a series per profile with their episode counts: the multi-episode file
// S01E03-E04 counts twice, E2 (16) only from 16 on, the specials count as season 0.
func TestVisibleSeasons(t *testing.T) {
	t.Parallel()
	f, r := externalFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		p    model.Profile
		want map[int]int
	}{
		{f.P18, map[int]int{0: 1, 1: 4}},
		{f.P16, map[int]int{0: 1, 1: 4}},
		{f.P12, map[int]int{0: 1, 1: 3}},
	} {
		got, err := r.VisibleSeasons(ctx, Profile(tc.p), f.Show.Series.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !maps.Equal(got, tc.want) {
			t.Errorf("%s: seasons = %v, want %v", tc.p.Name, got, tc.want)
		}
	}
	// A playable episode goes missing: its season counts one less.
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE media_files SET missing_since = 2 WHERE item_id = ?`, f.Show.E1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE items SET available = 0 WHERE id = ?`, f.Show.E1.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := r.VisibleSeasons(ctx, Profile(f.P12), f.Show.Series.ID); err != nil || !maps.Equal(got, map[int]int{0: 1, 1: 2}) {
		t.Errorf("P12 after E1 went missing: %v, %v", got, err)
	}
	for name, id := range map[string]string{"not a series": f.Rated(0).ID, "unknown": "01J9000000000000000000NONE"} {
		if _, err := r.VisibleSeasons(ctx, Profile(f.P18), id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: %v, want ErrNotFound", name, err)
		}
	}
	if _, err := r.VisibleSeasons(ctx, Profile(f.P6), f.Show.Series.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 (series rated 12): %v, want ErrNotFound", err)
	}
	if _, err := r.VisibleSeasons(ctx, System("test"), f.Show.Series.ID); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: %v, want ErrUnscoped", err)
	}
}
