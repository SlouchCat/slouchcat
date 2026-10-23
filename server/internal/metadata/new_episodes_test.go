package metadata

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// M7 E-8: a feed fetch tells the new-episode hook about the episodes it inserted, after their
// transaction; updated and unchanged episodes are not announced.
func TestPodcastNewEpisodesHook(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	var mu sync.Mutex
	var heard []string
	hook := func(ctx context.Context, ids []string) error {
		// n8: the hook runs detached from the fetch but with its own timeout.
		if d, ok := ctx.Deadline(); !ok || time.Until(d) > newEpisodesTimeout {
			t.Errorf("hook context deadline %v (set %v), want at most %v ahead", d, ok, newEpisodesTimeout)
		}
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

	// k4: an episode published before the newest known one (a back catalogue the feed adds, or a
	// renewed guid) is stored but not announced; a later one is.
	e.clock.set(e.clock.now().Add(time.Hour))
	old := time.Date(2024, 6, 1, 6, 0, 0, 0, time.UTC)
	later := time.Date(2025, 3, 1, 6, 0, 0, 0, time.UTC)
	e.feed.set(func(f *fakeFeed) {
		var extra strings.Builder
		for _, ep := range []struct {
			guid string
			at   time.Time
		}{{"ep-old", old}, {"ep-renewed-1", time.Date(2025, 1, 2, 6, 0, 0, 0, time.UTC)}, {"ep-later", later}} {
			fmt.Fprintf(&extra, `<item><title>%s</title><guid>%s</guid><pubDate>%s</pubDate><enclosure url="%s/%s.mp3" length="1" type="audio/mpeg"/></item>`,
				ep.guid, ep.guid, ep.at.Format(time.RFC1123Z), f.srv.URL, ep.guid)
		}
		f.body = strings.Replace(f.xml(), "</channel>", extra.String()+"</channel>", 1)
	})
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got, want := take(), ids(p.ID, "ep-later"); !slices.Equal(got, want) {
		t.Errorf("fetch with old episodes announced %v, want only the later one %v", got, want)
	}
	if eps := e.episodes(t, p.ID); eps["ep-old"].ID == "" || eps["ep-renewed-1"].ID == "" {
		t.Errorf("old episodes were not stored: %v", eps)
	}
}
