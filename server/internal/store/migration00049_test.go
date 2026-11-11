package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// episodeCoverFixture adds a podcast library with a podcast and its cover, an episode with a feed
// cover of its own (cover_url, the poster lock, image rows, a decided source) and an episode read
// from a folder with a cover_path to a frozen state before migration 00049.
const episodeCoverFixture = `
INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (4, 'Podcasts', 'podcasts', 0, 1, 1);
INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at, content, providers) VALUES
  (4, 4, '/media/frozen/Podcasts', 'nfs', 'poll', 1, 'podcast', '["local"]');
INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, cover_url, added_at, created_at, updated_at) VALUES
  ('01M6585TZB00P0DCAST000000B', 'podcast', 4, 'Frozen Podcast', 'frozen podcast', 'https://feeds.example.test/frozen.xml', 'admin',
   'https://img.example.test/podcast.jpg', 1, 1, 1);
INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, guid, cover_url, cover_path, locked_fields, added_at, created_at, updated_at) VALUES
  ('01M6585TZB00P0DCASTEP0001B', 'podcast_episode', '01M6585TZB00P0DCAST000000B', 4, 'Folge 1', 'folge 1', 'g1',
   'https://img.example.test/folge-1.jpg', NULL, '["title","poster"]', 1, 1, 1),
  ('01M6585TZB00P0DCASTEP0002B', 'podcast_episode', '01M6585TZB00P0DCAST000000B', 4, 'Folge 2', 'folge 2', NULL,
   NULL, 'Frozen Podcast/Folge 2.jpg', '[]', 1, 1, 1);
INSERT INTO images (item_id, kind, size, format, provider, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at) VALUES
  ('01M6585TZB00P0DCAST000000B', 'poster', 'original', 'jpeg', 'cover', 'https://img.example.test/podcast.jpg', 'images/01M6585TZB00P0DCAST000000B/poster-original.jpg', 'a', 'image/jpeg', 800, 800, 1, 1),
  ('01M6585TZB00P0DCAST000000B', 'poster', 'w400', 'jpeg', 'cover', 'https://img.example.test/podcast.jpg', 'images/01M6585TZB00P0DCAST000000B/poster-w400.jpg', 'b', 'image/jpeg', 400, 400, 1, 1),
  ('01M6585TZB00P0DCASTEP0001B', 'poster', 'original', 'jpeg', 'cover', 'https://img.example.test/folge-1.jpg', 'images/01M6585TZB00P0DCASTEP0001B/poster-original.jpg', 'c', 'image/jpeg', 800, 800, 1, 1),
  ('01M6585TZB00P0DCASTEP0001B', 'poster', 'w400', 'webp', 'cover', 'https://img.example.test/folge-1.jpg', 'images/01M6585TZB00P0DCASTEP0001B/poster-w400.webp', 'd', 'image/webp', 400, 400, 1, 1);
INSERT INTO item_images (item_id, kind, provider, source, updated_at) VALUES
  ('01M6585TZB00P0DCASTEP0001B', 'poster', 'cover', 'https://img.example.test/folge-1.jpg', 1);
INSERT INTO item_local_images (item_id, kind, rel_path, root_id, width, height, size, mtime) VALUES
  ('01M6585TZB00P0DCASTEP0002B', 'poster', 'Frozen Podcast/Folge 2.jpg', 4, 400, 400, 1, 1);
`

// Migration 00049 (decision of 11 November 2026) on the frozen state of migration 00046
// (hoerbuch-quellen.sql) with a podcast library: no podcast episode keeps a cover of its own (its
// cover_url and cover_path are empty, its image rows, decided source and local images are gone,
// the poster lock leaves), the podcast keeps its cover and its rows.
func TestMigration00049PodcastEpisodeCovers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-quellen.sql")+episodeCoverFixture)
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for query, want := range map[string]string{
		`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND (cover_url IS NOT NULL OR cover_path IS NOT NULL)`: "0",
		`SELECT count(*) FROM items WHERE kind = 'podcast_episode'`:                                                       "2",
		`SELECT locked_fields FROM items WHERE id = '01M6585TZB00P0DCASTEP0001B'`:                                         `["title"]`,
		`SELECT locked_fields FROM items WHERE id = '01M6585TZB00P0DCASTEP0002B'`:                                         `[]`,
		`SELECT count(*) FROM images WHERE item_id IN (SELECT id FROM items WHERE kind = 'podcast_episode')`:              "0",
		`SELECT count(*) FROM item_images WHERE item_id IN (SELECT id FROM items WHERE kind = 'podcast_episode')`:         "0",
		`SELECT count(*) FROM item_local_images WHERE item_id IN (SELECT id FROM items WHERE kind = 'podcast_episode')`:   "0",
		`SELECT cover_url FROM items WHERE id = '01M6585TZB00P0DCAST000000B'`:                                             "https://img.example.test/podcast.jpg",
		`SELECT count(*) FROM images WHERE item_id = '01M6585TZB00P0DCAST000000B'`:                                        "2",
		`SELECT count(*) FROM goose_db_version WHERE version_id = 49 AND is_applied = 1`:                                  "1",
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                   "0",
	} {
		var got string
		if err := db.Read.QueryRowContext(ctx, query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if got != want {
			t.Errorf("%s = %s, want %s", query, got, want)
		}
	}
}
