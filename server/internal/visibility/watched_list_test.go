package visibility

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 20 section 15.4 (listWatched): P12 finished a film, a book, an episode of the series and
// one of the podcast; the series and the podcast appear once with the date of their newest
// visible watched episode, their visible watched episodes and their visible episodes (E2, rated
// 16, counts nowhere although it carries watched_at). A started film, an extra and a season never
// appear; a film in its second run stays; the filter, the title order, a page beyond the end and
// the invalid values follow the contract. P6 does not see the series, so its watched episode
// gives no entry.
func TestWatchedList(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddAudio(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	show, a := f.Show, f.Audio
	film, started, book := f.Rated(12), f.Rated(6), a.Books[0]
	exec(t, f, `DELETE FROM progress`)
	watched := func(p model.Profile, id string, at int64) {
		t.Helper()
		exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at) VALUES (?, ?, 100, 100, 1, ?, ?)`,
			p.ID, id, at, at)
	}
	watched(f.P12, film.ID, 100)
	watched(f.P12, book.ID, 50)
	watched(f.P12, show.E1.ID, 200)
	watched(f.P12, show.E2.ID, 300) // hidden from P12
	watched(f.P12, show.Extra.ID, 400)
	watched(f.P12, a.Episodes[0].ID, 150)
	watched(f.P6, show.E1.ID, 200)
	exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 50, 100, 0, 500)`, f.P12.ID, started.ID)

	list := func(p model.Profile, q model.WatchedQuery) model.WatchedPage {
		t.Helper()
		page, err := r.Watched(ctx, Profile(p), q)
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	entryIDs := func(page model.WatchedPage) []string {
		out := make([]string, 0, len(page.Items))
		for _, e := range page.Items {
			out = append(out, e.Item.ID)
		}
		return out
	}
	page := list(f.P12, model.WatchedQuery{})
	if want := []string{show.Series.ID, a.Podcast.ID, film.ID, book.ID}; !slices.Equal(entryIDs(page), want) || page.Total != 4 {
		t.Fatalf("P12 watched = %v (total %d), want %v", entryIDs(page), page.Total, want)
	}
	for _, tc := range []struct {
		id       string
		at       int64
		watched  int
		episodes int // -1: none (a single title)
	}{{show.Series.ID, 200, 1, 3}, {a.Podcast.ID, 150, 1, 4}, {film.ID, 100, 0, -1}, {book.ID, 50, 0, -1}} {
		i := slices.IndexFunc(page.Items, func(e model.WatchedEntry) bool { return e.Item.ID == tc.id })
		e := page.Items[i]
		if !e.LastWatchedAt.Equal(time.UnixMilli(tc.at)) {
			t.Errorf("%s lastWatchedAt = %v, want %d", tc.id, e.LastWatchedAt, tc.at)
		}
		switch {
		case tc.episodes < 0 && e.EpisodeCount != nil:
			t.Errorf("%s carries episodeCount %d", tc.id, *e.EpisodeCount)
		case tc.episodes >= 0 && (e.EpisodeCount == nil || *e.EpisodeCount != tc.episodes):
			t.Errorf("%s episodeCount = %v, want %d", tc.id, e.EpisodeCount, tc.episodes)
		case tc.episodes >= 0 && (e.Item.WatchedCount == nil || *e.Item.WatchedCount != tc.watched):
			t.Errorf("%s watchedCount = %v, want %d", tc.id, e.Item.WatchedCount, tc.watched)
		}
	}
	if got := entryIDs(list(f.P12, model.WatchedQuery{Content: "series"})); !slices.Equal(got, []string{show.Series.ID}) {
		t.Errorf("P12 series only = %v", got)
	}
	if got := list(f.P12, model.WatchedQuery{Content: "movie"}); !slices.Equal(entryIDs(got), []string{film.ID}) || got.Total != 1 {
		t.Errorf("P12 films only = %v (total %d)", entryIDs(got), got.Total)
	}
	// By title: the fixture's texts carry no sort key, so the order is that of items.sort_title.
	var keyed int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM item_texts WHERE sort_key IS NOT NULL AND item_id IN (?, ?, ?, ?)`,
		show.Series.ID, a.Podcast.ID, film.ID, book.ID).Scan(&keyed); err != nil || keyed != 0 {
		t.Fatalf("fixture texts with a sort key: %d (%v)", keyed, err)
	}
	sortTitle := map[string]string{}
	for _, id := range entryIDs(page) {
		var st string
		if err := f.DB.Read.QueryRow(`SELECT sort_title FROM items WHERE id = ?`, id).Scan(&st); err != nil {
			t.Fatal(err)
		}
		sortTitle[id] = st
	}
	byTitle := slices.SortedFunc(slices.Values(entryIDs(page)), func(x, y string) int { return cmp.Compare(sortTitle[x], sortTitle[y]) })
	if got := entryIDs(list(f.P12, model.WatchedQuery{Sort: model.WatchedSortTitle})); !slices.Equal(got, byTitle) {
		t.Errorf("P12 by title = %v, want %v", got, byTitle)
	}
	// Paging: one per page, a page beyond the end keeps the total.
	if got := list(f.P12, model.WatchedQuery{Offset: 1, Limit: 1}); !slices.Equal(entryIDs(got), []string{a.Podcast.ID}) || got.Total != 4 {
		t.Errorf("P12 page 2 of 1 = %v (total %d)", entryIDs(got), got.Total)
	}
	if got := list(f.P12, model.WatchedQuery{Offset: 10}); len(got.Items) != 0 || got.Total != 4 {
		t.Errorf("P12 beyond the end = %v (total %d), want none of 4", entryIDs(got), got.Total)
	}
	// A film in its second run stays; a removed check mark takes it out.
	exec(t, f, `UPDATE progress SET completed = 0, position_ms = 10, updated_at = 600 WHERE profile_id = ? AND item_id = ?`, f.P12.ID, film.ID)
	if got := entryIDs(list(f.P12, model.WatchedQuery{Content: "movie"})); !slices.Equal(got, []string{film.ID}) {
		t.Errorf("P12 films in the second run = %v, want the film", got)
	}
	exec(t, f, `UPDATE progress SET watched_at = NULL WHERE profile_id = ? AND item_id = ?`, f.P12.ID, film.ID)
	if got := list(f.P12, model.WatchedQuery{Content: "movie"}); len(got.Items) != 0 || got.Total != 0 {
		t.Errorf("P12 films without the check mark = %v (total %d)", entryIDs(got), got.Total)
	}
	if got := list(f.P6, model.WatchedQuery{}); len(got.Items) != 0 || got.Total != 0 {
		t.Errorf("P6 watched = %v (total %d), want none (the series is rated 12)", entryIDs(got), got.Total)
	}
	for _, q := range []model.WatchedQuery{{Content: "radio"}, {Content: "episode"}, {Sort: "added"}} {
		var verr *model.ValidationError
		if _, err := r.Watched(ctx, Profile(f.P12), q); !errors.As(err, &verr) {
			t.Errorf("query %+v: %v, want a validation error", q, err)
		}
	}
}
