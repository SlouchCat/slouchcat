package metadata

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Concept 19 part A, 5.2 and P-14: the planner hook hears a new subscription without episode ids
// (its first fetch never counts as new episodes), a fetch with the ids of the episodes it reports
// as new, and nothing of a fetch without new episodes, so 94 podcasts polled 26 times a day queue
// no empty runs.
func TestPodcastFetchedHook(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	type call struct {
		podcast string
		ids     []string
	}
	var mu sync.Mutex
	var calls []call
	hook := func(ctx context.Context, podcastID string, ids []string) error {
		if d, ok := ctx.Deadline(); !ok || time.Until(d) > newEpisodesTimeout {
			t.Errorf("hook context deadline %v (set %v), want at most %v ahead", d, ok, newEpisodesTimeout)
		}
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, call{podcastID, slices.Sorted(slices.Values(ids))})
		return nil
	}
	e.svc = New(e.db, e.queue, staticKey(""), t.TempDir(), transcode.Runner{}, slog.New(slog.DiscardHandler),
		WithFeedClock(e.clock.now), WithFeedUserAgent("Slouchcat/test"), WithPodcastFetched(hook))
	take := func() []call {
		mu.Lock()
		defer mu.Unlock()
		out := calls
		calls = nil
		return out
	}
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got := take(); len(got) != 1 || got[0].podcast != p.ID || len(got[0].ids) != 0 {
		t.Fatalf("subscribe told the planner %+v, want the podcast without episode ids", got)
	}
	e.clock.set(e.clock.now().Add(time.Hour))
	e.feed.set(func(f *fakeFeed) { f.n = 3 })
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got, want := take(), []string{e.episodes(t, p.ID)["ep-3"].ID}; len(got) != 1 || got[0].podcast != p.ID || !slices.Equal(got[0].ids, want) {
		t.Errorf("fetch told the planner %+v, want %s with %v", got, p.ID, want)
	}
	e.clock.set(e.clock.now().Add(time.Hour))
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got := take(); len(got) != 0 {
		t.Errorf("a fetch without new episodes told the planner %+v", got)
	}
}
