package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// Migration 00014 rebuilds libraries and downloads (M6): the frozen M5 state must keep every
// library (with no default rating), every download and every tombstone; the rebuild must not leave
// tombstones of the moved rows behind, and the tombstone trigger must still fire afterwards.
func TestMigration00014KeepsFrozenData(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M5.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string) int {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	if got := count(`SELECT count(*) FROM libraries`); got != 2 {
		t.Fatalf("libraries after 00014 = %d, want 2", got)
	}
	if got := count(`SELECT count(*) FROM libraries WHERE default_rating IS NOT NULL`); got != 0 {
		t.Errorf("video libraries got a default rating (%d rows)", got)
	}
	downloads := count(`SELECT count(*) FROM downloads`)
	if downloads != 6 {
		t.Fatalf("downloads after 00014 = %d, want 6", downloads)
	}
	if got := count(`SELECT count(*) FROM downloads WHERE media_file_id IS NULL`); got != 0 {
		t.Errorf("downloads lost their media file (%d rows)", got)
	}
	tombstones := count(`SELECT count(*) FROM download_tombstones`)
	if tombstones != 1 {
		t.Fatalf("tombstones after 00014 = %d, want 1 (the rebuild must not leave tombstones)", tombstones)
	}
	if _, err := db.Write.Exec(`DELETE FROM downloads WHERE id = (SELECT id FROM downloads ORDER BY id LIMIT 1)`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM download_tombstones`); got != tombstones+1 {
		t.Errorf("tombstone trigger after the rebuild: %d tombstones, want %d", got, tombstones+1)
	}
	for _, col := range []string{"author", "narrator", "series_name", "series_index", "explicit", "duration_ms", "cover_url",
		"cover_path", "feed_url", "feed_checked_at", "guid", "published_at", "enclosure_type", "enclosure_length"} {
		if got := count(`SELECT count(*) FROM pragma_table_info('items') WHERE name = '` + col + `'`); got != 1 {
			t.Errorf("items.%s missing after 00014", col)
		}
	}
	if got := count(`SELECT count(*) FROM pragma_table_info('media_files') WHERE name = 'part_index'`); got != 1 {
		t.Errorf("media_files.part_index missing after 00014")
	}
	if got := count(`SELECT count(*) FROM pragma_table_info('progress') WHERE name = 'playback_rate'`); got != 1 {
		t.Errorf("progress.playback_rate missing after 00014")
	}
	if got := count(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'chapters'`); got != 1 {
		t.Errorf("chapters table missing after 00014")
	}
	// A podcast episode with a local file is available through the media file triggers (M6).
	if _, err := db.Write.Exec(`INSERT INTO items (id, kind, library_id, title, sort_title, available, created_at, updated_at, added_at)
		SELECT '01M6EPISODE00000000000000X', 'podcast_episode', id, 'Folge', 'folge', 0, 1, 1, 1 FROM libraries ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO media_files (id, item_id, root_id, rel_path, dev, inode, size, mtime, partial_hash, container, created_at, updated_at)
		SELECT '01M6EPISODEFILE0000000000X', '01M6EPISODE00000000000000X', id, 'episode.mp3', 1, 99999, 10, 1, 'hash', 'mp3', 1, 1 FROM library_roots ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT available FROM items WHERE id = '01M6EPISODE00000000000000X'`); got != 1 {
		t.Errorf("podcast episode with a file is not available (available = %d)", got)
	}
}

// M6 review m-7 and m-2: an episode with an enclosure stays available when its local file goes
// missing (as when it is deleted), and a library has one live subscription per feed URL.
func TestMigration00014EpisodeAvailabilityAndFeedIndex(t *testing.T) {
	db := openTestDB(t)
	w := db.Write
	mustExec(t, w, `INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (1, 'Podcasts', 'podcasts', 0, 0, 0)`)
	mustExec(t, w, `INSERT INTO library_roots (id, library_id, path, created_at) VALUES (1, 1, '/media/podcasts', 0)`)
	item := `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, own_rating, feed_url, enclosure_url, added_at, created_at, updated_at)
	         VALUES (?, ?, ?, 1, 'T', 't', ?, ?, ?, 0, 0, 0)`
	const podcast, episode, local = "01PODCAST0000000000000000A", "01EPISODE00000000000000ENC", "01EPISODE0000000000000FILE"
	mustExec(t, w, item, podcast, "podcast", nil, 0, "https://feeds.example.org/a.xml", nil)
	mustExec(t, w, item, episode, "podcast_episode", podcast, nil, nil, "https://cdn.example.org/1.mp3")
	mustExec(t, w, item, local, "podcast_episode", podcast, nil, nil, nil)
	addFile(t, w, "01FILE0000000000000000ENC1", episode, "a/1.mp3")
	addFile(t, w, "01FILE0000000000000000LOC1", local, "a/2.mp3")
	mustExec(t, w, `UPDATE media_files SET missing_since = 1`)
	if available(t, w, episode) != 1 || available(t, w, local) != 0 {
		t.Errorf("missing files: episode with enclosure available %d, without %d; want 1 and 0", available(t, w, episode), available(t, w, local))
	}
	mustExec(t, w, `UPDATE media_files SET missing_since = NULL`)
	if available(t, w, episode) != 1 || available(t, w, local) != 1 {
		t.Error("files back: both episodes must be available")
	}
	if _, err := w.Exec(item, "01PODCAST0000000000000000B", "podcast", nil, 0, "https://feeds.example.org/a.xml", nil); err == nil {
		t.Error("a second live subscription of the same feed was stored")
	}
	mustExec(t, w, `UPDATE items SET deleted_at = 1 WHERE id = ?`, podcast)
	mustExec(t, w, item, "01PODCAST0000000000000000B", "podcast", nil, 0, "https://feeds.example.org/a.xml", nil)
}
