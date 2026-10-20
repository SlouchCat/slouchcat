package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen M6 state carries every kind of M6 data (testdata/db/README.md), so later migrations
// are tested against audio libraries with a default rating, audiobooks with parts and chapters,
// a podcast subscription with enclosure-only episodes (one explicit), audio downloads, a playback
// rate on a progress row and a rule for new podcast episodes.
func TestFrozenM6StateCoversM6Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M6.sql"))
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
	for what, query := range map[string]string{
		"audiobook library with default rating": `SELECT count(*) FROM libraries WHERE kind = 'audiobooks' AND default_rating IS NOT NULL`,
		"podcast library without a root":        `SELECT count(*) FROM libraries l WHERE l.kind = 'podcasts' AND NOT EXISTS (SELECT 1 FROM library_roots r WHERE r.library_id = l.id)`,
		"audiobook with author and narrator":    `SELECT count(*) FROM items WHERE kind = 'audiobook' AND author <> '' AND narrator <> '' AND rating_source = 'library'`,
		"multi-part audiobook":                  `SELECT count(*) FROM (SELECT item_id FROM media_files WHERE part_index IS NOT NULL GROUP BY item_id HAVING count(*) > 1)`,
		"book-wide chapters":                    `SELECT count(*) FROM chapters WHERE start_ms > 0 AND media_file_id IS NOT NULL`,
		"embedded or folder cover":              `SELECT count(*) FROM items WHERE kind = 'audiobook' AND cover_path IS NOT NULL`,
		"podcast with feed state":               `SELECT count(*) FROM items WHERE kind = 'podcast' AND feed_url IS NOT NULL AND feed_checked_at IS NOT NULL AND cover_url IS NOT NULL`,
		"enclosure-only episode":                `SELECT count(*) FROM items i WHERE i.kind = 'podcast_episode' AND i.enclosure_url IS NOT NULL AND i.guid IS NOT NULL AND i.published_at IS NOT NULL AND i.available = 1 AND NOT EXISTS (SELECT 1 FROM media_files f WHERE f.item_id = i.id)`,
		"explicit episode rated 18 by the feed": `SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND explicit = 1 AND own_rating = 18 AND rating_source = 'feed'`,
		"audiobook download":                    `SELECT count(*) FROM downloads d JOIN items i ON i.id = d.item_id WHERE i.kind = 'audiobook' AND d.state = 'ready' AND d.audio_bitrate > 0`,
		"episode download from the enclosure":   `SELECT count(*) FROM downloads d JOIN items i ON i.id = d.item_id WHERE i.kind = 'podcast_episode' AND d.media_file_id IS NULL AND d.source_url IS NOT NULL AND d.state = 'ready'`,
		"playback rate":                         `SELECT count(*) FROM progress WHERE playback_rate IS NOT NULL`,
		"rule for new podcast episodes":         `SELECT count(*) FROM download_rules WHERE new_podcast_episodes = 1`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M6 state has no %s", what)
		}
	}
}
