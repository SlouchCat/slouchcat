package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// podcastFixture04 adds a podcast library with a podcast folder, a subscription with a feed and
// two feed episodes (one with a file in the folder) to a frozen state before migration 00047.
const podcastFixture04 = `
INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (4, 'Podcasts', 'podcasts', 0, 1, 1);
INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at, content, providers) VALUES
  (4, 4, '/media/frozen/Podcasts', 'nfs', 'poll', 1, 'podcast', '["local"]');
INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, added_at, created_at, updated_at) VALUES
  ('01M6585TZB00P0DCAST000000A', 'podcast', 4, 'Frozen Podcast', 'frozen podcast', 'https://feeds.example.test/frozen.xml', 'admin', 1, 1, 1);
INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, guid, published_at, enclosure_url, enclosure_length, added_at, created_at, updated_at) VALUES
  ('01M6585TZB00P0DCASTEP0001A', 'podcast_episode', '01M6585TZB00P0DCAST000000A', 4, 'Folge 1', 'folge 1', 'g1', 1794830400000, 'https://cdn.example.test/1.mp3', 1000, 1, 1, 1),
  ('01M6585TZB00P0DCASTEP0002A', 'podcast_episode', '01M6585TZB00P0DCAST000000A', 4, 'Folge 2', 'folge 2', 'g2', 1794916800000, 'https://cdn.example.test/2.mp3', 2000, 1, 1, 1);
INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at) VALUES
  ('01M6585TZB00P0DCASTF1LE01A', '01M6585TZB00P0DCASTEP0001A', 4, 'Frozen Podcast/Folge 1.mp3', 1000, 1, 'mp3', 1, 1);
`

// Migration 00047 (1.27.0, concept 19 part A) on the frozen state of migration 00046
// (hoerbuch-quellen.sql) with a podcast library: every library starts at «Nur Link», «Nur neue
// Folgen», N 10, two at once, no folder chosen; the checks refuse values outside the setting, the
// folder reference empties when the folder stops holding podcasts or goes, and the setting and the
// failure of a podcast and an episode go with their item.
func TestMigration00047PodcastDownloads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-quellen.sql")+podcastFixture04)
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	text := func(query string) string {
		t.Helper()
		var s string
		if err := db.Read.QueryRowContext(ctx, query).Scan(&s); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return s
	}
	exec := func(stmt string) error {
		_, err := db.Write.ExecContext(ctx, stmt)
		return err
	}
	for query, want := range map[string]string{
		`SELECT count(*) FROM libraries WHERE podcast_mode <> 'link' OR podcast_scope <> 'new' OR podcast_last_n <> 10 OR podcast_parallel <> 2 OR podcast_root_id IS NOT NULL`: "0",
		`SELECT count(*) FROM libraries`:                     "4",
		`SELECT count(*) FROM podcast_settings`:              "0",
		`SELECT count(*) FROM podcast_download_failures`:     "0",
		`SELECT count(*) FROM pragma_foreign_key_check`:      "0",
		`SELECT count(*) FROM media_files WHERE root_id = 4`: "1",
	} {
		if got := text(query); got != want {
			t.Errorf("%s = %s, want %s", query, got, want)
		}
	}
	for _, stmt := range []string{
		`UPDATE libraries SET podcast_mode = 'keep' WHERE id = 4`,
		`UPDATE libraries SET podcast_scope = 'some' WHERE id = 4`,
		`UPDATE libraries SET podcast_last_n = 0 WHERE id = 4`,
		`UPDATE libraries SET podcast_last_n = 101 WHERE id = 4`,
		`UPDATE libraries SET podcast_parallel = 6 WHERE id = 4`,
		`UPDATE libraries SET podcast_root_id = 99 WHERE id = 4`,
		`INSERT INTO podcast_settings (item_id, mode, updated_at) VALUES ('01M6585TZB00P0DCAST000000A', 'keep', 1)`,
		`INSERT INTO podcast_settings (item_id, last_n, updated_at) VALUES ('01M6585TZB00P0DCAST000000A', 0, 1)`,
		`INSERT INTO podcast_download_failures (item_id, error, attempts, first_failed_at, last_failed_at) VALUES ('01M6585TZB00P0DCASTEP0002A', 'not_audio', 0, 1, 1)`,
	} {
		if err := exec(stmt); err == nil {
			t.Errorf("accepted: %s", stmt)
		}
	}
	for _, stmt := range []string{
		`UPDATE libraries SET podcast_mode = 'save', podcast_scope = 'last_n', podcast_last_n = 3, podcast_parallel = 5, podcast_root_id = 4 WHERE id = 4`,
		`INSERT INTO podcast_settings (item_id, mode, scope, last_n, pending_root_id, pending_folder, updated_at) VALUES ('01M6585TZB00P0DCAST000000A', 'link', NULL, NULL, 4, 'Frozen Podcast', 1)`,
		`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES ('01M6585TZB00P0DCASTEP0002A', 'not_audio', 1, 1)`,
	} {
		if err := exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// The folder stops holding podcasts: no target for new subscriptions, no pending folder.
	if err := exec(`UPDATE library_roots SET content = 'audiobook' WHERE id = 4`); err != nil {
		t.Fatal(err)
	}
	if got := text(`SELECT count(*) FROM libraries WHERE podcast_root_id IS NOT NULL`); got != "0" {
		t.Errorf("libraries still pointing at a folder without podcasts: %s", got)
	}
	if got := text(`SELECT count(*) FROM podcast_settings WHERE pending_root_id IS NOT NULL OR pending_folder IS NOT NULL`); got != "0" {
		t.Errorf("pending folders in a folder without podcasts: %s", got)
	}
	// A removed folder empties the reference as well.
	if err := exec(`UPDATE library_roots SET content = 'podcast' WHERE id = 4`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`UPDATE libraries SET podcast_root_id = 4 WHERE id = 4`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`DELETE FROM library_roots WHERE id = 4`); err != nil {
		t.Fatal(err)
	}
	if got := text(`SELECT count(*) FROM libraries WHERE podcast_root_id IS NOT NULL`); got != "0" {
		t.Errorf("libraries still pointing at a removed folder: %s", got)
	}
	// The setting and the failure go with their items.
	if err := exec(`DELETE FROM items WHERE id = '01M6585TZB00P0DCASTEP0002A'`); err != nil {
		t.Fatal(err)
	}
	if got := text(`SELECT count(*) FROM podcast_download_failures`); got != "0" {
		t.Errorf("failures after deleting their episode: %s", got)
	}
	if err := exec(`DELETE FROM items WHERE kind = 'podcast_episode' AND parent_id = '01M6585TZB00P0DCAST000000A'`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`DELETE FROM items WHERE id = '01M6585TZB00P0DCAST000000A'`); err != nil {
		t.Fatal(err)
	}
	if got := text(`SELECT count(*) FROM podcast_settings`); got != "0" {
		t.Errorf("settings after deleting their podcast: %s", got)
	}
}
