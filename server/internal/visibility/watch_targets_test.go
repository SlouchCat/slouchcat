package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 20 section 6.4: the titles the check mark writes. A series gives its visible episodes
// of every season in episode order, specials (season 0) first, never an extra and never an
// episode the profile may not see (P12: no E2); a season its visible episodes; a single title
// itself. The duration is the probed one of the primary file, else the runtime, else 0; an
// audiobook takes its book-wide duration first. An invisible title is model.ErrNotFound.
func TestWatchTargets(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddAudio(t)
	s2 := f.AddSecondSeason(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	show := f.Show
	targets := func(p model.Profile, id string) []model.WatchTarget {
		t.Helper()
		out, err := r.WatchTargets(ctx, Profile(p), id)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	ids := func(ts []model.WatchTarget) []string {
		out := make([]string, 0, len(ts))
		for _, x := range ts {
			out = append(out, x.ItemID)
		}
		return out
	}
	for _, tc := range []struct {
		name string
		p    model.Profile
		id   string
		want []string
	}{
		{"series for P18", f.P18, show.Series.ID, []string{show.Special.ID, show.E1.ID, show.E2.ID, show.E3.ID, s2.E1.ID, s2.E2.ID}},
		{"series for P12 (E2 is 16)", f.P12, show.Series.ID, []string{show.Special.ID, show.E1.ID, show.E3.ID, s2.E1.ID, s2.E2.ID}},
		{"season for P12", f.P12, show.Season.ID, []string{show.E1.ID, show.E3.ID}},
		{"episode", f.P18, show.E2.ID, []string{show.E2.ID}},
		{"extra", f.P18, show.Extra.ID, []string{show.Extra.ID}},
		{"film", f.P18, f.Rated(12).ID, []string{f.Rated(12).ID}},
	} {
		if got := ids(targets(tc.p, tc.id)); !slices.Equal(got, tc.want) {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
	var episodes []string
	for _, m := range f.Audio.Titles() {
		if m.ParentID == f.Audio.Podcast.ID {
			episodes = append(episodes, m.ID)
		}
	}
	podcast := ids(targets(f.P18, f.Audio.Podcast.ID))
	if len(episodes) == 0 || !slices.Equal(slices.Sorted(slices.Values(podcast)), slices.Sorted(slices.Values(episodes))) {
		t.Errorf("podcast targets = %v, want its episodes %v", podcast, episodes)
	}
	for _, tc := range []struct {
		p  model.Profile
		id string
	}{{f.P6, show.Series.ID}, {f.P12, show.E2.ID}, {f.P18, "01J9000000000000000000XXXX"}} {
		if _, err := r.WatchTargets(ctx, Profile(tc.p), tc.id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("invisible %s for %s: %v, want ErrNotFound", tc.id, tc.p.Name, err)
		}
	}

	// Durations: the file's, else the runtime, else 0; a book its own first.
	r6, r16, book := f.Rated(6), f.Rated(16), f.Audio.Books[0]
	exec(t, f, `UPDATE media_files SET duration_ms = NULL WHERE item_id IN (?, ?)`, r6.ID, r16.ID)
	exec(t, f, `UPDATE items SET runtime_minutes = 90 WHERE id = ?`, r6.ID)
	exec(t, f, `UPDATE items SET runtime_minutes = NULL WHERE id = ?`, r16.ID)
	exec(t, f, `UPDATE items SET duration_ms = 3600000 WHERE id = ?`, book.ID)
	for id, want := range map[string]int64{f.Rated(12).ID: vistest.DurationMs, r6.ID: 5_400_000, r16.ID: 0, book.ID: 3_600_000} {
		if got := targets(f.P18, id); len(got) != 1 || got[0].DurationMs != want {
			t.Errorf("duration of %s = %+v, want %d", id, got, want)
		}
	}
}
