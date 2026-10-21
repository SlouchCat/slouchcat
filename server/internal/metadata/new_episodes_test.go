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

// M7 E-8: a feed fetch tells the new-episode hook about the episodes it inserted, after their
// transaction; updated and unchanged episodes are not announced.
func TestPodcastNewEpisodesHook(t *testing.T) {
	e := newPodcastEnv(t, 2)
	var mu sync.Mutex
	var heard []string
	hook := func(_ context.Context, ids []string) error {
		for _, id := range ids {
			var n int
			if err := e.db.Read.QueryRow(`SELECT count(*) FROM items WHERE id = ? AND kind = 'podcast_episode'`, id).Scan(&n); err != nil || n != 1 {
				t.Errorf("announced %s is no committed episode (%d, %v)", id, n, err)
			}
		}
		mu.Lock()
		defer mu.Unlock()
		heard = append(heard, ids...)
		return nil
	}
	e.svc = New(e.db, e.queue, staticKey(""), t.TempDir(), transcode.Runner{}, slog.New(slog.DiscardHandler),
		WithFeedClock(e.clock.now), WithFeedUserAgent("Slouchcat/test"), WithNewEpisodes(hook))
	take := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := heard
		heard = nil
		slices.Sort(out)
		return out
	}
	ids := func(podcastID string, guids ...string) []string {
		eps := e.episodes(t, podcastID)
		var out []string
		for _, g := range guids {
			out = append(out, eps[g].ID)
		}
		slices.Sort(out)
		return out
	}

	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := take(), ids(p.ID, "ep-1", "ep-2"); !slices.Equal(got, want) {
		t.Errorf("subscribe announced %v, want %v", got, want)
	}
	e.clock.set(e.clock.now().Add(time.Hour))
	e.feed.set(func(f *fakeFeed) { f.n, f.title[1] = 3, "Episode 1 (neu)" })
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got, want := take(), ids(p.ID, "ep-3"); !slices.Equal(got, want) {
		t.Errorf("fetch announced %v, want only the new episode %v", got, want)
	}
	e.clock.set(e.clock.now().Add(time.Hour))
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got := take(); len(got) != 0 {
		t.Errorf("unchanged feed announced %v", got)
	}
}
