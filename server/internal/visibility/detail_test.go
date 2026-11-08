package visibility

import (
	"context"
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
