package visibility

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// continueNow is the Reader's clock in the tests of "Weiterschauen".
var continueNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

// daysAgo is a progress time n days before continueNow, in Unix milliseconds.
func daysAgo(n float64) int64 {
	return continueNow.Add(-time.Duration(n * float64(24*time.Hour))).UnixMilli()
}

// setProgress writes the progress row of p on itemID: started at a tenth of the runtime, or
// completed at the end.
func setProgress(t *testing.T, f *vistest.Fixture, p model.Profile, itemID string, completed bool, at int64) {
	t.Helper()
	pos, done := int64(vistest.DurationMs/10), 0
	if completed {
		pos, done = vistest.DurationMs, 1
	}
	// As the 90 % rule writes it: a completed row is watched at its time, a later open one keeps
	// the date (concept 20 section 6.3).
	exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
	            VALUES (?, ?, ?, ?, ?, ?, CASE WHEN ? = 1 THEN ? END)
	            ON CONFLICT (profile_id, item_id) DO UPDATE SET position_ms = excluded.position_ms, completed = excluded.completed,
	              updated_at = excluded.updated_at, watched_at = COALESCE(excluded.watched_at, progress.watched_at)`,
		p.ID, itemID, pos, vistest.DurationMs, done, at, done, at)
}

// Concept 15 P-1: one entry per series. The newest progress of the series counts, completed or
// not; a completed one leads to the next episode by the rule of GetNextVisibleEpisode within
// NextEpisodeWindow, skipping episodes the profile already completed. Films stay as they were.
func TestContinueWatchingPerSeries(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	ctx := context.Background()
	show := f.Show
	p16 := Profile(f.P16)

	cases := []struct {
		name     string
		progress func()
		want     []string
	}{
		{"two started episodes give one entry with the newer one", func() {
			setProgress(t, f, f.P16, show.E1.ID, false, daysAgo(3))
			setProgress(t, f, f.P16, show.E3.ID, false, daysAgo(1))
		}, []string{show.E3.ID}},
		{"a completed episode leads to the next one", func() {
			setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(2))
		}, []string{show.E2.ID}},
		{"the window includes its last day", func() {
			setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(60))
		}, []string{show.E2.ID}},
		{"after the window the series is gone", func() {
			setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(60.01))
		}, nil},
		{"the last episode completed: the series is gone (specials do not follow)", func() {
			setProgress(t, f, f.P16, show.E3.ID, true, daysAgo(1))
		}, nil},
		{"an older half-watched episode is ignored once another one was finished", func() {
			setProgress(t, f, f.P16, show.E1.ID, false, daysAgo(5))
			setProgress(t, f, f.P16, show.E2.ID, true, daysAgo(1))
		}, []string{show.E3.ID}},
		{"the next episode keeps its older progress", func() {
			setProgress(t, f, f.P16, show.E2.ID, false, daysAgo(5))
			setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(1))
		}, []string{show.E2.ID}},
		{"a completed next episode is skipped (rewatch)", func() {
			setProgress(t, f, f.P16, show.E2.ID, true, daysAgo(10))
			setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(1))
		}, []string{show.E3.ID}},
		{"every next episode completed: the series is gone", func() {
			setProgress(t, f, f.P16, show.E2.ID, true, daysAgo(10))
			setProgress(t, f, f.P16, show.E3.ID, true, daysAgo(9))
			setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(1))
		}, nil},
		{"after a special the first regular episode follows", func() {
			setProgress(t, f, f.P16, show.Special.ID, true, daysAgo(1))
		}, []string{show.E1.ID}},
		{"a progress row at position 0 is no activity", func() {
			setProgress(t, f, f.P16, show.E1.ID, false, daysAgo(3))
			exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 0, ?, 0, ?)`,
				f.P16.ID, show.E2.ID, vistest.DurationMs, daysAgo(1))
		}, []string{show.E1.ID}},
	}
	for _, tc := range cases {
		exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P16.ID)
		tc.progress()
		got, err := r.ContinueWatching(ctx, p16, MaxRowItems)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !slices.Equal(ids(got), tc.want) {
			t.Errorf("%s: continue = %v, want %v", tc.name, ids(got), tc.want)
		}
	}

	// The next episode the card shows as new carries no progress, but its numbers and runtime; the
	// series comes with its title and images.
	exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P16.ID)
	setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(2))
	got, err := r.ContinueWatching(ctx, p16, MaxRowItems)
	if err != nil || len(got) != 1 {
		t.Fatalf("next episode: %v, %v", ids(got), err)
	}
	e := got[0]
	if e.Progress != nil || e.SeasonNumber == nil || *e.SeasonNumber != 1 || e.EpisodeNumber == nil || *e.EpisodeNumber != 2 || e.RuntimeMinutes == nil {
		t.Errorf("next episode = %+v, want S1 E2 with runtime and without progress", e)
	}
	if s := e.SeriesRef; s == nil || s.ID != show.Series.ID || s.Title != "Film SS" || s.Poster == nil || s.PosterSmall == nil || s.Backdrop == nil {
		t.Errorf("next episode series = %+v, want the series with title and images", s)
	}
}

// Concept 15 P-1: the series and the shown episode must be visible to the profile. The next episode
// skips episodes the profile may not see, exactly like NextEpisode (GetNextVisibleEpisode).
func TestContinueWatchingVisibility(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	ctx := context.Background()
	show := f.Show
	p12, p6 := Profile(f.P12), Profile(f.P6)

	// E2 is rated 16: for P12 the episode after E1 is E3.
	setProgress(t, f, f.P12, show.E1.ID, true, daysAgo(1))
	got, err := r.ContinueWatching(ctx, p12, MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	next, err := r.NextEpisode(ctx, p12, show.E1.ID)
	if err != nil || next == nil {
		t.Fatalf("next episode of E1 for P12: %v, %v", next, err)
	}
	if want := []string{show.E3.ID}; !slices.Equal(ids(got), want) || next.ID != show.E3.ID {
		t.Errorf("P12 continue = %v, next episode %s, want %v for both", ids(got), next.ID, want)
	}
	// The newest progress is on an episode P12 may not see (E2): it is no activity for P12, so the
	// older progress on E1 counts.
	exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P12.ID)
	setProgress(t, f, f.P12, show.E1.ID, false, daysAgo(3))
	setProgress(t, f, f.P12, show.E2.ID, false, daysAgo(1))
	if got, err := r.ContinueWatching(ctx, p12, MaxRowItems); err != nil || !slices.Equal(ids(got), []string{show.E1.ID}) {
		t.Errorf("P12 continue with progress on E2 = %v, %v, want E1", ids(got), err)
	}
	// A series the profile may not see removes its entry, even with a visible episode. The test
	// forces that state through the series' smallest child rating.
	exec(t, f, `UPDATE items SET min_child_rating = 16 WHERE id = ?`, show.Series.ID)
	if got, err := r.ContinueWatching(ctx, p12, MaxRowItems); err != nil || len(got) != 0 {
		t.Errorf("P12 continue with the series hidden = %v, %v, want none", ids(got), err)
	}
	exec(t, f, `UPDATE items SET min_child_rating = 12 WHERE id = ?`, show.Series.ID)

	// The series is rated 12: P6 sees neither it nor its episodes, only its own film.
	setProgress(t, f, f.P6, show.E1.ID, false, daysAgo(1))
	if got, err := r.ContinueWatching(ctx, p6, MaxRowItems); err != nil || !slices.Equal(ids(got), []string{f.Rated(6).ID}) {
		t.Errorf("P6 continue = %v, %v, want only the film rated 6", ids(got), err)
	}
}

// Films, audiobooks and podcast episodes stay as they were (started, not completed, no window);
// all entries are ordered by their newest activity. Only episodes of a series carry SeriesRef.
func TestContinueWatchingMixed(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddAudio(t)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	ctx := context.Background()
	show, pods := f.Show, f.Audio.Episodes
	exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P16.ID)
	setProgress(t, f, f.P16, f.Rated(0).ID, false, daysAgo(100)) // a film has no window
	setProgress(t, f, f.P16, show.E1.ID, false, daysAgo(1))
	setProgress(t, f, f.P16, f.Rated(16).ID, false, daysAgo(2))
	setProgress(t, f, f.P16, f.Rated(12).ID, true, daysAgo(0.5)) // a completed film is gone
	setProgress(t, f, f.P16, pods[0].ID, false, daysAgo(4))
	setProgress(t, f, f.P16, pods[1].ID, false, daysAgo(3)) // podcast episodes are not grouped
	got, err := r.ContinueWatching(ctx, Profile(f.P16), MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{show.E1.ID, f.Rated(16).ID, pods[1].ID, pods[0].ID, f.Rated(0).ID}
	if !slices.Equal(ids(got), want) {
		t.Errorf("continue = %v, want %v", ids(got), want)
	}
	for _, it := range got {
		if (it.SeriesRef != nil) != (it.Kind == "episode") {
			t.Errorf("%s (%s): series = %+v, want it on episodes only", it.ID, it.Kind, it.SeriesRef)
		}
	}
}

// ItemSummary.SeriesRef (ItemRef.seriesRef, contract 1.14.0) is filled wherever an episode
// appears, not only in the row "Weiterschauen": in the episodes of a season and on the episode's
// own detail; never on a season or a series.
func TestSeriesRefOnEpisodes(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	show := f.Show
	p18 := Profile(f.P18)

	season, err := r.Item(ctx, p18, show.Season.ID)
	if err != nil {
		t.Fatal(err)
	}
	if season.SeriesRef != nil {
		t.Errorf("season summary series = %+v, want none", season.SeriesRef)
	}
	if len(season.Episodes) != 3 {
		t.Fatalf("episodes = %v, want 3", ids(season.Episodes))
	}
	for _, e := range season.Episodes {
		if e.SeriesRef == nil || e.SeriesRef.ID != show.Series.ID || e.SeriesRef.Title != e.SeriesTitle || e.SeriesRef.Poster == nil {
			t.Errorf("episode %s: series = %+v, want %s with title %q and poster", e.ID, e.SeriesRef, show.Series.ID, e.SeriesTitle)
		}
	}
	series, err := r.Item(ctx, p18, show.Series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if series.SeriesRef != nil {
		t.Errorf("series summary series = %+v, want none", series.SeriesRef)
	}
	for _, s := range series.Seasons {
		if s.SeriesRef != nil {
			t.Errorf("season %s: series = %+v, want none", s.ID, s.SeriesRef)
		}
	}
	ep, err := r.Item(ctx, p18, show.E1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s := ep.SeriesRef; s == nil || s.ID != show.Series.ID {
		t.Errorf("episode detail series = %+v", s)
	}
}

// releasedEpisodeID is the item id of episode k (1-based) of AddReleasedSeries n.
func releasedEpisodeID(n, k int) string { return fmt.Sprintf("01J9RELEASEDE%010d%03d", n, k) }

// addNumberedSeries adds series n by AddReleasedSeries with one season (number 1) and an episode per
// entry of numbers: {first, last} with last 0 for a single episode, so {3, 4} is a double episode.
func addNumberedSeries(t *testing.T, f *vistest.Fixture, n int, numbers [][2]int) string {
	t.Helper()
	eps := make([]vistest.ReleasedEpisode, len(numbers))
	id := f.AddReleasedSeries(t, n, 0, eps)
	exec(t, f, `UPDATE items SET season_number = 1 WHERE parent_id = ? AND kind = 'season'`, id)
	for k, num := range numbers {
		end := sql.NullInt64{Int64: int64(num[1]), Valid: num[1] != 0}
		exec(t, f, `UPDATE items SET season_number = 1, episode_number = ?, episode_end = ? WHERE id = ?`,
			num[0], end, releasedEpisodeID(n, k+1))
	}
	return id
}

// Review L-2: without its season or episode number a completed episode has no next one, in the row
// as in NextEpisode (episodeChain); SQL must not jump to the first episode of a later season.
func TestContinueWatchingWithoutNumbers(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	ctx := context.Background()
	show := f.Show
	p16 := Profile(f.P16)
	// The specials become season 2, so a later season exists.
	exec(t, f, `UPDATE items SET season_number = 2 WHERE id IN (?, ?)`, show.Specials.ID, show.Special.ID)
	exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P16.ID)
	setProgress(t, f, f.P16, show.E1.ID, true, daysAgo(1))
	if got, err := r.ContinueWatching(ctx, p16, MaxRowItems); err != nil || !slices.Equal(ids(got), []string{show.E2.ID}) {
		t.Fatalf("with numbers = %v, %v, want E2", ids(got), err)
	}

	for _, tc := range []struct{ name, update string }{
		{"no episode number", `UPDATE items SET episode_number = NULL WHERE id = ?`},
		{"no season number", `UPDATE items SET season_number = NULL WHERE id IN (?, ?)`},
	} {
		exec(t, f, `UPDATE items SET season_number = 1, episode_number = 1 WHERE id = ?`, show.E1.ID)
		exec(t, f, `UPDATE items SET season_number = 1 WHERE id = ?`, show.Season.ID)
		exec(t, f, tc.update, show.E1.ID, show.Season.ID)
		got, err := r.ContinueWatching(ctx, p16, MaxRowItems)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		next, err := r.NextEpisode(ctx, p16, show.E1.ID)
		if err != nil {
			t.Fatalf("%s: next episode: %v", tc.name, err)
		}
		if len(got) != 0 || next != nil {
			t.Errorf("%s: continue = %v, next episode %+v, want neither", tc.name, ids(got), next)
		}
	}
}

// Review L-8: a completed double episode (E3-E4) leads to E5, also when E5 has the lower id.
func TestContinueWatchingAfterDoubleEpisode(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	ctx := context.Background()
	// Episode ids by insertion: 1 = E5, 2 = E1, 3 = E3-E4, 4 = E2.
	addNumberedSeries(t, f, 1, [][2]int{{5, 0}, {1, 0}, {3, 4}, {2, 0}})
	e5, e34 := releasedEpisodeID(1, 1), releasedEpisodeID(1, 3)
	exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P16.ID)
	setProgress(t, f, f.P16, e34, true, daysAgo(1))
	got, err := r.ContinueWatching(ctx, Profile(f.P16), MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	next, err := r.NextEpisode(ctx, Profile(f.P16), e34)
	if err != nil || next == nil {
		t.Fatalf("next episode of E3-E4: %v, %v", next, err)
	}
	if !slices.Equal(ids(got), []string{e5}) || next.ID != e5 {
		t.Errorf("continue = %v, next episode %s, want E5 (%s) for both", ids(got), next.ID, e5)
	}
}

// Review L-8: series and films mixed over MaxRowItems. Grouping comes before the limit (a series
// with two progress rows takes one place), the order is the newest activity of each group across
// series and films, and the oldest groups fall off.
func TestContinueWatchingLimitAcrossGroups(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	ctx := context.Background()
	exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P16.ID)

	const groups = MaxRowItems + 4
	var want []string
	for g := range groups {
		at := daysAgo(float64(g + 1)) // group 0 is the newest
		n := g/2 + 1
		if g%2 == 1 {
			film := f.AddReleasedFilm(t, n, 0, "")
			setProgress(t, f, f.P16, film, false, at)
			want = append(want, film)
			continue
		}
		addNumberedSeries(t, f, n, [][2]int{{1, 0}, {2, 0}, {3, 0}})
		e1, e2 := releasedEpisodeID(n, 1), releasedEpisodeID(n, 2)
		switch n % 3 {
		case 0: // started: the episode itself
			setProgress(t, f, f.P16, e1, false, at)
			want = append(want, e1)
		case 1: // completed: the next episode
			setProgress(t, f, f.P16, e1, true, at)
			want = append(want, e2)
		case 2: // an older completed row (newer than the next group) and a newer started one
			setProgress(t, f, f.P16, e1, true, at-int64(12*time.Hour/time.Millisecond))
			setProgress(t, f, f.P16, e2, false, at)
			want = append(want, e2)
		}
	}
	// A completed film is newer than all of them and takes no place.
	done := f.AddReleasedFilm(t, groups, 0, "")
	setProgress(t, f, f.P16, done, true, daysAgo(0.5))

	got, err := r.ContinueWatching(ctx, Profile(f.P16), MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	if want = want[:MaxRowItems]; !slices.Equal(ids(got), want) {
		t.Errorf("continue = %v,\nwant %v", ids(got), want)
	}
}

// Two progress rows of a series at the same time: the higher item id is the newest, as with the
// order updated_at DESC, item_id DESC (docs/perf/feinschliff.md: newest keeps it by max()). E2
// carries the higher id than E1 in the fixture.
func TestContinueWatchingSameTime(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	ctx := context.Background()
	show := f.Show
	if show.E2.ID <= show.E1.ID {
		t.Fatalf("fixture ids: E2 %s not above E1 %s", show.E2.ID, show.E1.ID)
	}
	for _, tc := range []struct {
		name   string
		e1, e2 bool // completed
		want   string
	}{
		{"E1 completed, E2 started: E2 is the newest and started", true, false, show.E2.ID},
		{"E1 started, E2 completed: E2 is the newest, the entry is E3", false, true, show.E3.ID},
	} {
		exec(t, f, `DELETE FROM progress WHERE profile_id = ?`, f.P16.ID)
		setProgress(t, f, f.P16, show.E1.ID, tc.e1, daysAgo(1))
		setProgress(t, f, f.P16, show.E2.ID, tc.e2, daysAgo(1))
		got, err := r.ContinueWatching(ctx, Profile(f.P16), MaxRowItems)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), []string{tc.want}) {
			t.Errorf("%s: continue = %v, want %s", tc.name, ids(got), tc.want)
		}
	}
}
