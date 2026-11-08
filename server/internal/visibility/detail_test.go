package visibility

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Tests of the detail page of concept 17 stage 2 (contract 1.20.0) against the database: the season
// shelf (year, watched episodes), the overview of the episode list, the collection members and next
// up, each through visible_items.

// seasonsOf returns the seasons of the series detail by id.
func seasonsOf(t *testing.T, r *Reader, p model.Profile, seriesID string) map[string]model.ItemSummary {
	t.Helper()
	d, err := r.Item(context.Background(), Profile(p), seriesID)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]model.ItemSummary, len(d.Seasons))
	for _, s := range d.Seasons {
		out[s.ID] = s
	}
	return out
}

// Concept 17 section 2.3: a season without a year of its own takes the year of its release date;
// watchedCount counts the completed episodes the profile may see, per profile, on seasons only.
func TestSeasonYearAndWatchedCount(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	show := f.Show
	exec(t, f, `UPDATE items SET year = NULL, release_date = '2004-09-22' WHERE id = ?`, show.Season.ID)
	exec(t, f, `UPDATE items SET year = NULL, release_date = 'unknown' WHERE id = ?`, show.Specials.ID)
	exec(t, f, `UPDATE items SET year = NULL WHERE id = ?`, show.Series.ID)
	// P12 and P16 complete E1 and E2; P12 may not see E2 (own rating 16). P16 has E3 only started.
	for _, p := range []model.Profile{f.P12, f.P16} {
		setProgress(t, f, p, show.E1.ID, true, 1)
		setProgress(t, f, p, show.E2.ID, true, 2)
	}
	setProgress(t, f, f.P16, show.E3.ID, false, 3)

	for _, tc := range []struct {
		p       model.Profile
		watched int
	}{{f.P12, 1}, {f.P16, 2}, {f.P18, 0}} {
		seasons := seasonsOf(t, r, tc.p, show.Series.ID)
		s1, sp := seasons[show.Season.ID], seasons[show.Specials.ID]
		if s1.Year == nil || *s1.Year != 2004 {
			t.Errorf("%s season 1 year = %v, want 2004 from the release date", tc.p.Name, s1.Year)
		}
		if sp.Year != nil {
			t.Errorf("%s specials year = %d, want none (no valid date)", tc.p.Name, *sp.Year)
		}
		if s1.WatchedCount == nil || *s1.WatchedCount != tc.watched {
			t.Errorf("%s season 1 watchedCount = %v, want %d", tc.p.Name, s1.WatchedCount, tc.watched)
		}
		if sp.WatchedCount == nil || *sp.WatchedCount != 0 {
			t.Errorf("%s specials watchedCount = %v, want 0", tc.p.Name, sp.WatchedCount)
		}
		d, err := r.Item(context.Background(), Profile(tc.p), show.Series.ID)
		if err != nil {
			t.Fatal(err)
		}
		if d.Year != nil || d.WatchedCount != nil {
			t.Errorf("%s series: year %v watchedCount %v, want neither", tc.p.Name, d.Year, d.WatchedCount)
		}
	}
	// A season with a year of its own keeps it.
	exec(t, f, `UPDATE items SET year = 2005 WHERE id = ?`, show.Season.ID)
	if s1 := seasonsOf(t, r, f.P18, show.Series.ID)[show.Season.ID]; s1.Year == nil || *s1.Year != 2005 {
		t.Errorf("season 1 with its own year = %v, want 2005", s1.Year)
	}
}

// Concept 17 section 2.3: summaries carry the overview in the profile language (the API passes it
// on only for the episodes of a season).
func TestSummaryOverviewInProfileLanguage(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	en := f.P18
	en.Language, en.MetadataLanguage = "en", "en"
	for _, tc := range []struct {
		p               model.Profile
		episode, season string
	}{{f.P18, "Handlung E1", "Handlung SN"}, {en, "Plot E1", "Plot SN"}} {
		d, err := r.Item(context.Background(), Profile(tc.p), f.Show.Season.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Episodes) == 0 || d.Episodes[0].Overview != tc.episode || d.Overview != tc.season || d.ItemSummary.Overview != tc.season {
			t.Fatalf("%s (%s): episodes %+v, season overview %q / %q, want %q and %q",
				tc.p.Language, tc.p.Name, d.Episodes, d.Overview, d.ItemSummary.Overview, tc.episode, tc.season)
		}
	}
}

// Concept 17 section 4.2 (E-7): the collections of a detail carry their first MaxRowItems visible
// members in collection order with progress and watchlist marks; other collection lists do not.
func TestCollectionMembersOnDetail(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	members := func(p model.Profile, itemID string) model.CollectionRef {
		t.Helper()
		d, err := r.Item(ctx, Profile(p), itemID)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range d.Collections {
			if c.ID == vistest.MixedCollectionID {
				return c
			}
		}
		t.Fatalf("%s: detail of %s without the mixed collection: %+v", p.Name, itemID, d.Collections)
		return model.CollectionRef{}
	}
	// Visibility: P12 does not see the film rated 18, P18 does; order by position.
	for _, tc := range []struct {
		p    model.Profile
		want []string
	}{
		{f.P12, []string{f.Rated(6).ID, f.Rated(12).ID}},
		{f.P18, []string{f.Rated(6).ID, f.Rated(12).ID, f.Rated(18).ID}},
	} {
		c := members(tc.p, f.Rated(6).ID)
		if !slices.Equal(ids(c.Members), tc.want) || c.Count != len(tc.want) {
			t.Errorf("%s members = %v (count %d), want %v", tc.p.Name, ids(c.Members), c.Count, tc.want)
		}
	}
	// Progress and the watchlist mark of the profile come with the members.
	setProgress(t, f, f.P18, f.Rated(12).ID, false, 5)
	c := members(f.P18, f.Rated(6).ID)
	if c.Members[1].Progress == nil || c.Members[1].Progress.Completed || !c.Members[2].InWatchlist || c.Members[0].InWatchlist {
		t.Errorf("P18 members: progress %+v, marks %v %v %v", c.Members[1].Progress,
			c.Members[0].InWatchlist, c.Members[1].InWatchlist, c.Members[2].InWatchlist)
	}
	// The limit: 25 more films after the three (same position, then by id); the first 20 in order.
	f.AddSpotlightFilms(t, 25)
	want := []string{f.Rated(6).ID, f.Rated(12).ID, f.Rated(18).ID}
	for k := 1; k <= 25; k++ {
		id := fmt.Sprintf("01J9SPOTLIGHT00000000%05d", k)
		exec(t, f, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES (?, ?, 3, 1)`, vistest.MixedCollectionID, id)
		want = append(want, id)
	}
	c = members(f.P18, f.Rated(6).ID)
	if !slices.Equal(ids(c.Members), want[:MaxRowItems]) || c.Count != 28 {
		t.Errorf("P18 members of 28 = %v (count %d), want the first %d", ids(c.Members), c.Count, MaxRowItems)
	}
	// Only the detail sets members.
	refs, err := r.Collections(ctx, Profile(f.P18), MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		if ref.Members != nil {
			t.Errorf("collection list %s carries members %v", ref.ID, ids(ref.Members))
		}
	}
}

// Concept 17 section 3.3 against the database: next up of a series over every season and of a
// season over that season only; an episode the profile may not see neither counts nor is chosen,
// and a series it may not see has no detail.
func TestNextUpOnDetail(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	show := f.Show
	next := func(p model.Profile, itemID string) (string, string) {
		t.Helper()
		d, err := r.Item(ctx, Profile(p), itemID)
		if err != nil {
			t.Fatal(err)
		}
		if d.NextUp == nil {
			return "", ""
		}
		return d.NextUp.Reason, d.NextUp.Episode.ID
	}
	check := func(what string, p model.Profile, itemID, reason, episode string) {
		t.Helper()
		if gotReason, gotEpisode := next(p, itemID); gotReason != reason || gotEpisode != episode {
			t.Errorf("%s: %s next up = %s %s, want %s %s", what, p.Name, gotReason, gotEpisode, reason, episode)
		}
	}
	check("no progress", f.P12, show.Series.ID, NextUpStart, show.E1.ID)
	check("no progress, specials page", f.P12, show.Specials.ID, NextUpStart, show.Special.ID)

	// P12 completed E1; its newer progress on E2 (own rating 16, hidden from P12) does not count,
	// and E2 is never its next episode. P16 sees E2.
	setProgress(t, f, f.P12, show.E1.ID, true, 1)
	setProgress(t, f, f.P12, show.E2.ID, false, 5)
	setProgress(t, f, f.P16, show.E1.ID, true, 1)
	check("hidden episode", f.P12, show.Series.ID, NextUpNext, show.E3.ID)
	check("hidden episode, season page", f.P12, show.Season.ID, NextUpNext, show.E3.ID)
	check("visible next", f.P16, show.Series.ID, NextUpNext, show.E2.ID)
	check("another profile's progress", f.P18, show.Series.ID, NextUpStart, show.E1.ID)

	// P16 finishes the special last: the series page goes on with the first regular episode,
	// watched or not (S01E01, Marco's answer of 8 November 2026); the season page of season 1 does
	// not see the special; the specials page, all of whose episodes are completed, offers the
	// rewatch.
	setProgress(t, f, f.P16, show.Special.ID, true, 9)
	check("newest is a special", f.P16, show.Series.ID, NextUpNext, show.E1.ID)
	check("season 1 ignores the special", f.P16, show.Season.ID, NextUpNext, show.E2.ID)
	check("specials page", f.P16, show.Specials.ID, NextUpRewatch, show.Special.ID)

	// Every episode completed: rewatch from the first regular one; the episode is a full summary.
	setProgress(t, f, f.P16, show.E2.ID, true, 10)
	setProgress(t, f, f.P16, show.E3.ID, true, 11)
	check("all watched", f.P16, show.Series.ID, NextUpRewatch, show.E1.ID)
	d, err := r.Item(ctx, Profile(f.P16), show.Series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if e := d.NextUp.Episode; e.Progress == nil || !e.Progress.Completed || e.SeasonNumber == nil || *e.SeasonNumber != 1 || e.SeriesTitle == "" {
		t.Errorf("next up episode summary = %+v", e)
	}
	// Films and episodes have no next up; a series the profile may not see has no detail.
	for _, id := range []string{f.Rated(12).ID, show.E1.ID} {
		if reason, _ := next(f.P16, id); reason != "" {
			t.Errorf("%s carries next up %s", id, reason)
		}
	}
	if _, err := r.Item(ctx, Profile(f.P6), show.Series.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 detail of the series rated 12: err = %v, want not found", err)
	}
}
