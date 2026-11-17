package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// catalogToday is the clock of the catalog tests.
var catalogToday = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

// catalogFixture is externalFixture with a catalog row for the series S (P18 plays season 0 and
// four episodes of season 1, P12 three, P6 does not see the series): specials, season 1 (10
// episodes), season 2 (8, aired 2020), season 3 airing (last episode S03E02, next S03E03), season
// 4 announced without episodes, season 5 announced with a future air date.
func catalogFixture(t *testing.T) (*vistest.Fixture, *Reader) {
	t.Helper()
	f, _ := externalFixture(t)
	setCatalog(t, f, `[{"number":5,"episodeCount":10,"airDate":"2027-03-01"},{"number":0,"episodeCount":2,"airDate":"2009-12-01"},
		{"number":1,"episodeCount":10,"airDate":"2010-04-01"},{"number":2,"episodeCount":8,"airDate":"2020-01-05"},
		{"number":3,"episodeCount":6,"airDate":"2026-11-03"},{"number":4,"episodeCount":0}]`,
		`{"season":3,"episode":3,"airDate":"2026-11-24"}`, `{"season":3,"episode":2,"airDate":"2026-11-10"}`)
	return f, New(f.DB, vistest.Logger(), WithClock(func() time.Time { return catalogToday }))
}

func setCatalog(t *testing.T, f *vistest.Fixture, seasons string, next, last any) {
	t.Helper()
	if _, err := f.DB.Write.ExecContext(context.Background(), `INSERT INTO series_catalog (item_id, tmdb_id, status, in_production, seasons, next_episode, last_episode, fetched_at)
		VALUES (?, 1399, 'Returning Series', 1, ?, ?, ?, 1)
		ON CONFLICT (item_id) DO UPDATE SET seasons = excluded.seasons, next_episode = excluded.next_episode, last_episode = excluded.last_episode`,
		f.Show.Series.ID, seasons, next, last); err != nil {
		t.Fatal(err)
	}
}

func canRequest(media string) bool { return media == MediaShow }

// Missing seasons are the catalog minus the seasons the profile can play: season 0 never appears,
// seasons not aired yet are announced (not offered), a season on air counts as missing; the numbers
// for «n von m Staffeln vorhanden» leave the announced seasons out.
func TestSeriesCatalogMissingSeasons(t *testing.T) {
	t.Parallel()
	f, r := catalogFixture(t)
	ctx := context.Background()
	got, err := r.SeriesCatalog(ctx, Profile(f.P18), canRequest, f.Show.Series.ID)
	if err != nil || got == nil {
		t.Fatalf("catalog = %v, %v", got, err)
	}
	want := []CatalogSeason{
		{Number: 1, EpisodeCount: 10, AirDate: "2010-04-01", Episodes: 4, State: SeasonPresent},
		{Number: 2, EpisodeCount: 8, AirDate: "2020-01-05", State: SeasonMissing},
		{Number: 3, EpisodeCount: 6, AirDate: "2026-11-03", State: SeasonMissing},
		{Number: 4, State: SeasonAnnounced},
		{Number: 5, EpisodeCount: 10, AirDate: "2027-03-01", State: SeasonAnnounced},
	}
	if !slices.Equal(got.Seasons, want) {
		t.Errorf("seasons = %+v, want %+v", got.Seasons, want)
	}
	if !slices.Equal(got.Missing, []int{2, 3}) || got.Present != 1 || got.Total != 3 || !got.CanRequestSeasons {
		t.Errorf("missing %v, %d of %d, can request %v; want [2 3], 1 of 3, true", got.Missing, got.Present, got.Total, got.CanRequestSeasons)
	}
	if got.Status != "Returning Series" || !got.InProduction || got.NextEpisode == nil || *got.NextEpisode != (CatalogEpisode{3, 3, "2026-11-24"}) ||
		got.LastEpisode == nil || *got.LastEpisode != (CatalogEpisode{3, 2, "2026-11-10"}) {
		t.Errorf("status %q, in production %v, next %+v, last %+v", got.Status, got.InProduction, got.NextEpisode, got.LastEpisode)
	}
	// P12 plays fewer episodes of season 1; the season is present all the same.
	if p12, err := r.SeriesCatalog(ctx, Profile(f.P12), canRequest, f.Show.Series.ID); err != nil || p12.Seasons[0].Episodes != 3 || p12.Present != 1 {
		t.Errorf("P12 = %+v, %v", p12, err)
	}
	// A season the profile plays that TMDB does not list is present too; a season aired by the
	// last episode without an air date of its own is missing.
	setCatalog(t, f, `[{"number":2,"episodeCount":8}]`, nil, `{"season":2,"episode":8}`)
	got, err = r.SeriesCatalog(ctx, Profile(f.P18), canRequest, f.Show.Series.ID)
	if err != nil || len(got.Seasons) != 2 || got.Seasons[0] != (CatalogSeason{Number: 1, Episodes: 4, State: SeasonPresent}) ||
		got.Seasons[1].State != SeasonMissing || got.Present != 1 || got.Total != 2 || got.NextEpisode != nil {
		t.Errorf("library season outside the catalog = %+v, %v", got, err)
	}
	// Every aired season present: nothing to request.
	setCatalog(t, f, `[{"number":0,"episodeCount":2},{"number":1,"episodeCount":4,"airDate":"2010-04-01"},{"number":2,"episodeCount":0}]`, nil, nil)
	got, err = r.SeriesCatalog(ctx, Profile(f.P18), canRequest, f.Show.Series.ID)
	if err != nil || got.CanRequestSeasons || len(got.Missing) != 0 || got.Present != 1 || got.Total != 1 {
		t.Errorf("complete series = %+v, %v; want nothing missing", got, err)
	}
	// Only specials in the catalog (season 0 is never offered): no catalog.
	setCatalog(t, f, `[{"number":0,"episodeCount":5,"airDate":"2010-01-01"}]`, nil, nil)
	if got, err := r.SeriesCatalog(ctx, Profile(f.P18), canRequest, f.Show.Series.ID); err != nil || got != nil {
		t.Errorf("catalog of specials only = %+v, %v; want nil", got, err)
	}
}

// A profile without the request right for series gets no catalog (no missing seasons, nothing to
// request, F-9); a hidden series is not found whatever the right; a series without catalog row has
// no missing seasons.
func TestSeriesCatalogRightsAndVisibility(t *testing.T) {
	t.Parallel()
	f, r := catalogFixture(t)
	ctx := context.Background()
	for name, right := range map[string]RequestRight{
		"nil":         nil,
		"movies only": func(media string) bool { return media == MediaMovie },
		"none":        func(string) bool { return false },
	} {
		if got, err := r.SeriesCatalog(ctx, Profile(f.P18), right, f.Show.Series.ID); err != nil || got != nil {
			t.Errorf("right %s: catalog = %+v, %v; want nil (no missing seasons, canRequestSeasons false)", name, got, err)
		}
	}
	if _, err := r.SeriesCatalog(ctx, Profile(f.P6), canRequest, f.Show.Series.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 (series rated 12) = %v, want ErrNotFound", err)
	}
	if _, err := r.SeriesCatalog(ctx, Profile(f.P6), nil, f.Show.Series.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 without right = %v, want ErrNotFound", err)
	}
	if _, err := r.SeriesCatalog(ctx, Profile(f.P18), canRequest, f.Rated(0).ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("a film = %v, want ErrNotFound", err)
	}
	if _, err := r.SeriesCatalog(ctx, System("test"), canRequest, f.Show.Series.ID); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope = %v, want ErrUnscoped", err)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `DELETE FROM series_catalog`); err != nil {
		t.Fatal(err)
	}
	if got, err := r.SeriesCatalog(ctx, Profile(f.P18), canRequest, f.Show.Series.ID); err != nil || got != nil {
		t.Errorf("series without catalog row = %+v, %v; want nil", got, err)
	}
}

// The target of a wished season: the TMDB episode count, for the season on air the episodes aired
// so far; season 0, a season without episodes, an unlisted season and a series without row are not
// found (the caller falls back to external_titles).
func TestSeriesCatalogSeasonTarget(t *testing.T) {
	t.Parallel()
	f, r := catalogFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		season int
		want   SeasonTarget
		found  bool
	}{
		{1, SeasonTarget{Episodes: 10}, true},
		{2, SeasonTarget{Episodes: 8}, true},
		{3, SeasonTarget{Episodes: 2, Airing: true}, true},
		{5, SeasonTarget{Episodes: 10}, true},
		{0, SeasonTarget{}, false},
		{4, SeasonTarget{}, false},
		{9, SeasonTarget{}, false},
	} {
		got, found, err := r.CatalogSeasonTarget(ctx, f.Show.Series.ID, tc.season)
		if err != nil || found != tc.found || got != tc.want {
			t.Errorf("season %d = %+v, %v, %v; want %+v, %v", tc.season, got, found, err, tc.want, tc.found)
		}
	}
	// The next episode opens a season nothing of which has aired yet.
	setCatalog(t, f, `[{"number":1,"episodeCount":10}]`, `{"season":1,"episode":1,"airDate":"2026-12-01"}`, nil)
	if got, found, err := r.CatalogSeasonTarget(ctx, f.Show.Series.ID, 1); err != nil || !found || got != (SeasonTarget{Airing: true}) {
		t.Errorf("season about to start = %+v, %v, %v", got, found, err)
	}
	if _, found, err := r.CatalogSeasonTarget(ctx, "01J9000000000000000000NONE", 1); err != nil || found {
		t.Errorf("series without row: found %v, %v", found, err)
	}
}
