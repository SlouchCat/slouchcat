package library

import (
	"fmt"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The list of the page «Podcasts» of the administration reaches the media files of a podcast
// through its episodes (items_published or items_episode) and an index on item_id
// (media_files_item, media_files_root_item), never through media_files_missing. Before migration 00052 the index held every file; with missing_since IS NULL
// SQLite walked it and read the table row of every present file of the server once per podcast:
// PodcastList 47 s, PodcastDownloadStats 47 s and PodcastNewestRoots 93 s on CT 190 (61 749 files,
// 106 podcasts, docs/perf/podcasts-liste.md). The test fills a podcast library next to a larger
// video library and checks the plans of the three queries before and after ANALYZE.
func TestPodcastListQueriesReachFilesThroughEpisodes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	const (
		podcasts = 6
		episodes = 40 // per podcast
		movies   = 1500
	)
	e.exec(`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'P', 'podcasts', 0, 0), (2, 'V', 'video', 0, 0)`)
	e.exec(`INSERT INTO library_roots (id, library_id, path, fs_type, watch, content, created_at) VALUES
		(1, 1, '/m/p', 'ext4', 'inotify', 'podcast', 0), (2, 2, '/m/v', 'ext4', 'inotify', 'movie', 0)`)
	tx, err := e.db.Write.BeginTx(e.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	n := 0
	item := func(kind string, lib int64, parent any, feed any, published any) string {
		n++
		id := fmt.Sprintf("01KW%022d", n)
		if _, err := tx.ExecContext(e.ctx, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, feed_url,
			published_at, added_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, 1)`,
			id, kind, parent, lib, id, id, feed, published, n); err != nil {
			t.Fatal(err)
		}
		return id
	}
	file := func(itemID string, root int64, rel string, missing any) {
		n++
		if _, err := tx.ExecContext(e.ctx, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container,
			missing_since, created_at, updated_at) VALUES (?, ?, ?, ?, 10, ?, 'mp3', ?, 1, 1)`,
			fmt.Sprintf("01KF%022d", n), itemID, root, rel, n, missing); err != nil {
			t.Fatal(err)
		}
	}
	for p := range podcasts {
		pod := item("podcast", 1, nil, fmt.Sprintf("https://feed.example/%d.xml", p), nil)
		for ep := range episodes {
			id := item("podcast_episode", 1, pod, nil, int64(1000+ep))
			var missing any
			if ep%10 == 0 {
				missing = int64(5)
			}
			file(id, 1, fmt.Sprintf("pod%d/%02d.mp3", p, ep), missing)
		}
	}
	for m := range movies {
		file(item("movie", 2, nil, nil, nil), 2, fmt.Sprintf("m%04d.mkv", m), nil)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	rec := &recordingDB{DBTX: e.db.Read}
	q := systemq.New(rec)
	queries := map[string]func() (int, error){
		"PodcastList": func() (int, error) {
			rows, err := q.PodcastList(e.ctx, nil)
			for _, r := range rows {
				if r.EpisodeCount != episodes || r.LocalEpisodeCount != episodes-episodes/10 || r.RootID != 1 {
					t.Errorf("PodcastList row %s: %d episodes, %d local, root %d", r.ID, r.EpisodeCount, r.LocalEpisodeCount, r.RootID)
				}
			}
			return len(rows), err
		},
		"PodcastDownloadStats": func() (int, error) {
			rows, err := q.PodcastDownloadStats(e.ctx)
			for _, r := range rows {
				if r.StoredBytes != 10*(episodes-episodes/10) || r.LatestEpisodeAt != 1000+episodes-1 {
					t.Errorf("PodcastDownloadStats row %s: %d bytes, latest %d", r.PodcastID, r.StoredBytes, r.LatestEpisodeAt)
				}
			}
			return len(rows), err
		},
		"PodcastNewestRoots": func() (int, error) {
			rows, err := q.PodcastNewestRoots(e.ctx)
			for _, r := range rows {
				if r.RootID != 1 || !strings.HasPrefix(r.Folder, "pod") {
					t.Errorf("PodcastNewestRoots row %s: root %d, folder %q", r.PodcastID, r.RootID, r.Folder)
				}
			}
			return len(rows), err
		},
	}
	for _, analyzed := range []bool{false, true} {
		if analyzed {
			e.exec(`ANALYZE`)
		}
		for name, run := range queries {
			got, err := run()
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if got != podcasts {
				t.Errorf("%s (analyzed %v): %d rows, want %d", name, analyzed, got, podcasts)
			}
			plan := e.plan(rec.query, rec.args)
			for _, line := range plan {
				onFiles := strings.Contains(line, " m ") || strings.HasSuffix(line, " m")
				if strings.Contains(line, "media_files_missing") || (onFiles && !strings.Contains(line, "item_id=?)")) {
					t.Errorf("%s (analyzed %v) reaches media_files other than by item_id: %q\n%s",
						name, analyzed, line, strings.Join(plan, "\n"))
				}
			}
		}
	}
}
