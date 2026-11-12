package visibility

import (
	"context"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract 1.22.0: ItemRef.watched marks a series (and a podcast, below) whose visible episodes
// the profile has all watched (since 1.31.0 by watched_at, concept 20 section 6.3), by the rule of
// the library status watched (specials count, a hidden episode does not); false while one visible
// episode is not watched, absent on a series without visible episodes and on every other kind. The
// detail and the lists read it from the same batched child counts.
func TestSeriesWatched(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	show := f.Show
	// P18 completes every episode but the special; P12 completes all it may see (not E2, rated
	// 16); P16 has nothing.
	for i, id := range []string{show.E1.ID, show.E2.ID, show.E3.ID} {
		setProgress(t, f, f.P18, id, true, int64(i+1))
	}
	for i, id := range []string{show.E1.ID, show.E3.ID, show.Special.ID} {
		setProgress(t, f, f.P12, id, true, int64(i+1))
	}
	// watchedOf returns ItemRef.watched of the series on the detail and in the library list.
	watchedOf := func(p model.Profile) (detail, list *bool) {
		t.Helper()
		d, err := r.Item(ctx, Profile(p), show.Series.ID)
		if err != nil {
			t.Fatal(err)
		}
		page, err := r.LibraryItems(ctx, Profile(p), f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortTitle, MaxRating: -1, Limit: MaxLibraryPage})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, it := range page.Items {
			switch {
			case it.ID == show.Series.ID:
				list, found = it.Watched, true
			case it.Watched != nil:
				t.Errorf("%s: %s %s carries watched %v, want it on series only", p.Name, it.Kind, it.ID, *it.Watched)
			}
		}
		if !found {
			t.Fatalf("%s: series missing from the library page", p.Name)
		}
		return d.Watched, list
	}
	is := func(b *bool, want bool) bool { return b != nil && *b == want }

	if d, l := watchedOf(f.P18); !is(d, false) || !is(l, false) {
		t.Errorf("P18 with the special open: detail %v list %v, want false (specials count)", d, l)
	}
	if d, l := watchedOf(f.P12); !is(d, true) || !is(l, true) {
		t.Errorf("P12 with every visible episode completed: detail %v list %v, want true (E2 is hidden)", d, l)
	}
	if d, l := watchedOf(f.P16); !is(d, false) || !is(l, false) {
		t.Errorf("P16 without progress: detail %v list %v, want false", d, l)
	}
	setProgress(t, f, f.P18, show.Special.ID, true, 4)
	if d, l := watchedOf(f.P18); !is(d, true) || !is(l, true) {
		t.Errorf("P18 with every episode completed: detail %v list %v, want true", d, l)
	}
	// An episode watched once and started again stays watched (watched_at, concept 20 section
	// 6.3, contract 1.31.0): the series keeps its check mark.
	setProgress(t, f, f.P18, show.E2.ID, false, 5)
	if d, l := watchedOf(f.P18); !is(d, true) || !is(l, true) {
		t.Errorf("P18 with E2 started again: detail %v list %v, want true (watched_at stays)", d, l)
	}
	// An episode that is deleted or unavailable is not visible, so its open state does not count.
	exec(t, f, `UPDATE items SET deleted_at = 1 WHERE id = ?`, show.E2.ID)
	if d, l := watchedOf(f.P18); !is(d, true) || !is(l, true) {
		t.Errorf("P18 with only the deleted E2 open: detail %v list %v, want true", d, l)
	}
	exec(t, f, `UPDATE items SET deleted_at = NULL, available = 0 WHERE id = ?`, show.E2.ID)
	if d, l := watchedOf(f.P18); !is(d, true) || !is(l, true) {
		t.Errorf("P18 with only the unavailable E2 open: detail %v list %v, want true", d, l)
	}
	exec(t, f, `UPDATE items SET available = 1 WHERE id = ?`, show.E2.ID)
	// Seasons and episodes never carry it.
	for _, id := range []string{show.Season.ID, show.E1.ID} {
		d, err := r.Item(ctx, Profile(f.P12), id)
		if err != nil {
			t.Fatal(err)
		}
		if d.Watched != nil || slices.ContainsFunc(d.Episodes, func(e model.ItemSummary) bool { return e.Watched != nil }) {
			t.Errorf("%s %s carries watched %v, want none", d.Kind, id, d.Watched)
		}
	}
}

// Contract 1.22.0: a series the profile may see without a visible episode (here a series whose
// only child is an extra, made available by hand) has no ItemRef.watched.
func TestSeriesWatchedAbsentWithoutEpisodes(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger())
	series, _ := f.AddLoneExtraSeries(t)
	exec(t, f, `UPDATE items SET available = 1, min_child_rating = 0 WHERE id = ?`, series.ID)
	d, err := r.Item(context.Background(), Profile(f.P18), series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Watched != nil {
		t.Errorf("series without visible episodes: watched %v, want absent", *d.Watched)
	}
}

// Contract 1.22.0: a podcast carries ItemRef.watched by the same rule (every visible episode
// heard); its episodes and an audiobook never do.
func TestPodcastWatched(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddAudio(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	a := f.Audio
	// Both profiles complete the episodes rated 0 to 12 and the enclosure-only one (rated 0): all
	// that P12 may see, not those rated 16 and 18 that P18 sees too.
	for i, id := range []string{a.Episodes[0].ID, a.Episodes[1].ID, a.Episodes[2].ID, a.EnclosureOnly.ID} {
		setProgress(t, f, f.P12, id, true, int64(i+1))
		setProgress(t, f, f.P18, id, true, int64(i+1))
	}
	watchedOf := func(p model.Profile) (detail, list *bool) {
		t.Helper()
		d, err := r.Item(ctx, Profile(p), a.Podcast.ID)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(d.Episodes, func(e model.ItemSummary) bool { return e.Watched != nil }) {
			t.Errorf("%s: a podcast episode carries watched", p.Name)
		}
		page, err := r.LibraryItems(ctx, Profile(p), a.PodcastLibraryID, model.LibraryQuery{Sort: model.LibrarySortTitle, MaxRating: -1, Limit: MaxLibraryPage})
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(page.Items, func(it model.ItemSummary) bool { return it.ID == a.Podcast.ID })
		if i < 0 {
			t.Fatalf("%s: podcast missing from the library page", p.Name)
		}
		return d.Watched, page.Items[i].Watched
	}
	is := func(b *bool, want bool) bool { return b != nil && *b == want }
	if d, l := watchedOf(f.P12); !is(d, true) || !is(l, true) {
		t.Errorf("P12 with every visible episode heard: detail %v list %v, want true", d, l)
	}
	if d, l := watchedOf(f.P18); !is(d, false) || !is(l, false) {
		t.Errorf("P18 with the episodes rated 16 and 18 open: detail %v list %v, want false", d, l)
	}
	// watchedCount (contract 1.31.0, concept 20 F-12): the heard episodes the profile may see.
	for _, p := range []model.Profile{f.P12, f.P18} {
		d, err := r.Item(ctx, Profile(p), a.Podcast.ID)
		if err != nil {
			t.Fatal(err)
		}
		if d.WatchedCount == nil || *d.WatchedCount != 4 {
			t.Errorf("%s podcast watchedCount = %v, want 4", p.Name, d.WatchedCount)
		}
	}
	book, err := r.Item(ctx, Profile(f.P18), a.Books[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if book.Watched != nil {
		t.Errorf("audiobook carries watched %v, want none", *book.Watched)
	}
	// The podcast without episodes, made available by hand: no watched.
	exec(t, f, `UPDATE items SET available = 1, min_child_rating = 0 WHERE id = ?`, a.Empty.ID)
	d, err := r.Item(ctx, Profile(f.P18), a.Empty.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Watched != nil {
		t.Errorf("podcast without visible episodes: watched %v, want absent", *d.Watched)
	}
}
