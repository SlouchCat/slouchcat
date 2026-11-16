package metadata

import (
	"context"
	"testing"
	"time"
)

// The feed side of a podcast folder (ADR 0015, concept 4.5): a new feed episode that matches
// exactly one file episode of the podcast takes it over instead of becoming a second row; two
// candidates leave both apart.
func TestFetchAdoptsAFileEpisode(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml", true)
	if err != nil {
		t.Fatal(err)
	}
	// The fake feed dates episode i at 2025-01-01 06:00 UTC + i days; episode 3 is not in the
	// feed yet. Two file episodes read from a folder: one at episode 3's minute, one at episode
	// 4's minute twice (ambiguous later).
	third := time.Date(2025, 1, 4, 6, 0, 20, 0, time.UTC).UnixMilli()
	fourth := time.Date(2025, 1, 5, 6, 0, 0, 0, time.UTC).UnixMilli()
	for _, f := range []struct {
		id, title string
		published int64
	}{{"01FILE000000000000000000F3", "Datei drei", third}, {"01FILE000000000000000000F4", "Datei vier a", fourth}, {"01FILE000000000000000000F5", "Datei vier b", fourth}} {
		mustExec(t, e.db.Write, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, published_at, providers, metadata_state,
		          added_at, created_at, updated_at) VALUES (?, 'podcast_episode', ?, 3, ?, ?, ?, '["local"]', 'matched', 0, 0, 0)`,
			f.id, p.ID, f.title, sortTitle(f.title), f.published)
	}
	e.feed.mu.Lock()
	e.feed.n = 3
	e.feed.mu.Unlock()
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	eps := e.episodes(t, p.ID)
	if got := eps["ep-3"]; got.ID != "01FILE000000000000000000F3" || got.Title != "Episode 3" || got.Enclosure == "" {
		t.Fatalf("episode 3 = %+v, want the file episode taken over with the feed's title and enclosure", got)
	}
	delete(eps, "") // the file episodes without guid share one key of the helper
	if n := len(eps); n != 3 {
		t.Fatalf("episodes with guid = %d, want 3", n)
	}
	var rows int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM items WHERE parent_id = ? AND kind = 'podcast_episode' AND deleted_at IS NULL`, p.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 5 {
		t.Fatalf("episode rows = %d, want 5 (3 feed, of which one took a file, plus 2 file episodes)", rows)
	}
	// Episode 4 arrives: two file episodes share its minute, so neither is taken.
	e.feed.mu.Lock()
	e.feed.n = 4
	e.feed.mu.Unlock()
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM items WHERE parent_id = ? AND kind = 'podcast_episode' AND deleted_at IS NULL`, p.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 6 {
		t.Fatalf("episode rows after the ambiguous episode = %d, want 6", rows)
	}
	if got := e.episodes(t, p.ID)["ep-4"]; got.ID == "01FILE000000000000000000F4" || got.ID == "01FILE000000000000000000F5" {
		t.Fatalf("episode 4 took one of two fitting files: %+v", got)
	}
}

// A feed whose address came from a file on the media share (feed_origin file) is fetched through
// the guarded client even when it names a host in the local network: the LAN exception is the
// administrator's, not the NAS's (concept 9.1).
func TestFetchOfAFeedFromAFileIsGuarded(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	ctx := context.Background()
	// The same loopback feed twice: subscribed by hand it is fetched, read from a file it is refused.
	byHand, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml", true)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, providers, metadata_state,
	          added_at, created_at, updated_at) VALUES ('01FILEFEED00000000000000FF', 'podcast', 1, 'Aus Datei', 'aus datei', ?, 'file', '["local"]', 'matched', 0, 0, 0)`,
		e.feed.srv.URL+"/feed.xml")
	if err := e.svc.FetchPodcast(ctx, byHand.ID); err != nil {
		t.Fatalf("fetch of the subscription by hand: %v", err)
	}
	err = e.svc.FetchPodcast(ctx, "01FILEFEED00000000000000FF")
	if err == nil {
		t.Fatal("fetch of a loopback feed from a file succeeded, want refused")
	}
	_, _, feedErr, _, _ := e.podcast(t, "01FILEFEED00000000000000FF")
	if !feedErr.Valid || feedErr.String == "" {
		t.Fatalf("feed_error after the refused fetch = %v", feedErr)
	}
}

// The pairing rule since 16 November 2026 (addendum to ADR 0029): the same minute and the same
// title are the same episode whatever the durations say (the feed states 0:01:03, the file of the
// provider runs 55 s); the same title alone, on another day and with another length, is not.
func TestFetchAdoptsAFileEpisodeWhateverItsDuration(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml", true)
	if err != nil {
		t.Fatal(err)
	}
	third := time.Date(2025, 1, 4, 6, 0, 45, 0, time.UTC).UnixMilli()
	later := time.Date(2025, 2, 1, 6, 0, 0, 0, time.UTC).UnixMilli()
	for _, f := range []struct {
		id, title string
		published int64
		duration  int64
	}{{"01FILE000000000000000000F3", "Episode 3", third, 55_000}, {"01FILE000000000000000000F4", "Episode 4", later, 55_000}} {
		mustExec(t, e.db.Write, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, published_at, duration_ms, providers,
		          metadata_state, added_at, created_at, updated_at) VALUES (?, 'podcast_episode', ?, 3, ?, ?, ?, ?, '["local"]', 'matched', 0, 0, 0)`,
			f.id, p.ID, f.title, sortTitle(f.title), f.published, f.duration)
	}
	e.feed.set(func(f *fakeFeed) { f.n = 4 })
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	eps := e.episodes(t, p.ID)
	if got := eps["ep-3"]; got.ID != "01FILE000000000000000000F3" || got.Enclosure == "" {
		t.Errorf("episode 3 = %+v, want the file episode of its minute and title taken over", got)
	}
	if got := eps["ep-4"]; got.ID == "01FILE000000000000000000F4" {
		t.Errorf("episode 4 took a file that shares only its title and runs 14 percent shorter: %+v", got)
	}
}
