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

// releasedNow is the clock of the released tests: today is 2026-10-26 (UTC), so the window of
// twelve months (S-5) runs from 2025-10-26 to 2026-10-26, both included. The fixture films are
// dated 2010 to 2015 and stay outside it.
var releasedNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func releasedReader(f *vistest.Fixture) *Reader {
	return New(f.DB, vistest.Logger(), WithClock(func() time.Time { return releasedNow }))
}

func released(t *testing.T, r *Reader, p model.Profile, limit int) []string {
	t.Helper()
	items, err := r.Released(context.Background(), Profile(p), limit)
	if err != nil {
		t.Fatal(err)
	}
	return ids(items)
}

// Plan step 3, case "Titel ohne release_date" (S-5): only titles with a release_date count; a film
// without one is absent, the dated films inside the window come newest first.
func TestReleasedTitleWithoutReleaseDate(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := releasedReader(f)
	undated := f.AddReleasedFilm(t, 1, 0, "")
	march := f.AddReleasedFilm(t, 2, 0, "2026-03-01")
	september := f.AddReleasedFilm(t, 3, 0, "2026-09-01")

	got := released(t, r, f.P0, MaxRowItems)
	if want := []string{september, march}; !slices.Equal(got, want) {
		t.Errorf("P0 released = %v, want %v (newest first, without %s)", got, want, undated)
	}
	// The row is a profile read: an unscoped or system scope is rejected.
	if _, err := r.Released(context.Background(), System("test"), MaxRowItems); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: err = %v, want ErrUnscoped", err)
	}
}

// Plan step 3, case "genau an der 12-Monats-Grenze" (S-5): with today 2026-10-26, a film dated
// exactly twelve months back (2025-10-26) and one dated today are inside, the day before the
// boundary (2025-10-25) and tomorrow (a future date) are outside.
func TestReleasedTwelveMonthBoundary(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := releasedReader(f)
	boundary := f.AddReleasedFilm(t, 1, 0, "2025-10-26")
	before := f.AddReleasedFilm(t, 2, 0, "2025-10-25")
	today := f.AddReleasedFilm(t, 3, 0, "2026-10-26")
	tomorrow := f.AddReleasedFilm(t, 4, 0, "2026-10-27")

	got := released(t, r, f.P0, MaxRowItems)
	if want := []string{today, boundary}; !slices.Equal(got, want) {
		t.Errorf("P0 released = %v, want %v (without %s before the window and %s in the future)", got, want, before, tomorrow)
	}
}

// Plan step 3, case "Serie mit unsichtbarer neuester Folge" (S-6): a series counts with its newest
// visible episode. Its newest dated episode is rated 18, the older one 0; a film dated between the
// two shows which date the series sorts by: the older one for P0, the newest one for P18.
func TestReleasedSeriesWithInvisibleNewestEpisode(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := releasedReader(f)
	series := f.AddReleasedSeries(t, 1, 0, []vistest.ReleasedEpisode{
		{Rating: 0, Date: "2026-02-01"},
		{Rating: 18, Date: "2026-08-01"},
	})
	film := f.AddReleasedFilm(t, 1, 0, "2026-05-01")

	if got, want := released(t, r, f.P0, MaxRowItems), []string{film, series}; !slices.Equal(got, want) {
		t.Errorf("P0 released = %v, want %v (series dated by its episode of 2026-02-01)", got, want)
	}
	if got, want := released(t, r, f.P18, MaxRowItems), []string{series, film}; !slices.Equal(got, want) {
		t.Errorf("P18 released = %v, want %v (series dated by its episode of 2026-08-01)", got, want)
	}
}

// Plan step 3, case "Serie ohne sichtbare Folge" (S-6): a series without a visible dated episode in
// the window is absent, even when the series itself is visible or carries a date of its own:
// episodes without a date, a dated episode the profile may not see, episodes dated before the
// window, and a series that is invisible as a whole because every episode is rated 16.
func TestReleasedSeriesWithoutVisibleEpisode(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := releasedReader(f)
	undated := f.AddReleasedSeries(t, 1, 0, []vistest.ReleasedEpisode{{Rating: 0}, {Rating: 0}})
	// The series' own date is not its release in the row (S-6: not the series year).
	f.SetReleaseDate(t, undated, "2026-09-01")
	onlyAdult := f.AddReleasedSeries(t, 2, 0, []vistest.ReleasedEpisode{
		{Rating: 0},
		{Rating: 18, Date: "2026-07-01"},
	})
	old := f.AddReleasedSeries(t, 3, 0, []vistest.ReleasedEpisode{{Rating: 0, Date: "2024-01-01"}})
	sixteen := f.AddReleasedSeries(t, 4, 0, []vistest.ReleasedEpisode{
		{Rating: 16, Date: "2026-06-01"},
		{Rating: 16, Date: "2026-07-15"},
	})

	for _, tc := range []struct {
		p    model.Profile
		want []string
	}{
		{f.P0, []string{}},
		{f.P12, []string{}},
		{f.P16, []string{sixteen}},
		{f.P18, []string{sixteen, onlyAdult}},
	} {
		got := released(t, r, tc.p, MaxRowItems)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s released = %v, want %v", tc.p.Name, got, tc.want)
		}
		for _, absent := range []string{undated, old} {
			if slices.Contains(got, absent) {
				t.Errorf("%s released contains %s", tc.p.Name, absent)
			}
		}
	}
	// The series with the episode rated 18 is itself visible to P0 (its other episode is rated 0);
	// only the date is missing.
	if _, err := r.Item(context.Background(), Profile(f.P0), onlyAdult); err != nil {
		t.Errorf("P0 item %s: %v, want the series visible", onlyAdult, err)
	}
}

// S-5: audiobooks count with their release_date like films; a podcast and its episodes never enter
// the row, whatever their date.
func TestReleasedAudiobookButNoPodcast(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddAudio(t)
	r := releasedReader(f)
	book0, book18 := f.Audio.Books[0], f.Audio.Books[4] // rated 0 and 18
	f.SetReleaseDate(t, book0.ID, "2026-04-01")
	f.SetReleaseDate(t, book18.ID, "2026-06-01")
	f.SetReleaseDate(t, f.Audio.Podcast.ID, "2026-09-01")
	f.SetReleaseDate(t, f.Audio.Episodes[0].ID, "2026-09-02")

	if got, want := released(t, r, f.P0, MaxRowItems), []string{book0.ID}; !slices.Equal(got, want) {
		t.Errorf("P0 released = %v, want %v", got, want)
	}
	if got, want := released(t, r, f.P18, MaxRowItems), []string{book18.ID, book0.ID}; !slices.Equal(got, want) {
		t.Errorf("P18 released = %v, want %v (no podcast %s, no episode %s)", got, want, f.Audio.Podcast.ID, f.Audio.Episodes[0].ID)
	}
}

// The limit cuts the newest titles and is clamped to 1..MaxRowItems.
func TestReleasedLimitClampedToTwenty(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := releasedReader(f)
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var newestFirst []string
	for n := 1; n <= MaxRowItems+5; n++ {
		newestFirst = slices.Insert(newestFirst, 0, f.AddReleasedFilm(t, n, 0, day.AddDate(0, 0, n).Format(time.DateOnly)))
	}
	for _, tc := range []struct {
		limit int
		want  []string
	}{
		{5, newestFirst[:5]},
		{MaxRowItems + 30, newestFirst[:MaxRowItems]},
		{0, newestFirst[:1]},
	} {
		if got := released(t, r, f.P0, tc.limit); !slices.Equal(got, tc.want) {
			t.Errorf("limit %d: released = %v, want %v", tc.limit, got, tc.want)
		}
	}
}
