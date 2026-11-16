package metadata

import (
	"context"
	"testing"
)

// Feed episodes take the home folder of their podcast (concept 24 3.2, ADR 0036): none for a
// podcast only from its feed, the folder of a podcast a folder adopted.
func TestFeedEpisodesInheritThePodcastsHomeFolder(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml", true)
	if err != nil {
		t.Fatal(err)
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := e.db.Read.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM items WHERE (id = ? OR parent_id = ?) AND root_id IS NOT NULL`, p.ID, p.ID); n != 0 {
		t.Errorf("a feed-only podcast and its episodes: %d rows with a home folder, want none", n)
	}
	mustExec(t, e.db.Write, `INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at, content, providers)
		VALUES (78, 3, '/media/podcasts-home', 'ext4', 'poll', 1, 'podcast', '["local"]')`)
	mustExec(t, e.db.Write, `UPDATE items SET root_id = 78 WHERE id = ?`, p.ID)
	e.feed.mu.Lock()
	e.feed.n = 3
	e.feed.mu.Unlock()
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if n := count(`SELECT count(*) FROM items WHERE parent_id = ? AND guid = 'ep-3' AND root_id = 78`, p.ID); n != 1 {
		t.Errorf("the new feed episode did not take the podcast's home folder")
	}
}
