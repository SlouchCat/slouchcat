package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen M3 state carries every kind of M3 data (testdata/db/README.md), so later migrations
// are tested against it.
func TestFrozenM3StateCoversM3Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M3.sql"))
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
		"series":            `SELECT count(*) FROM items WHERE kind = 'series' AND available = 1`,
		"specials":          `SELECT count(*) FROM items WHERE kind = 'season' AND season_number = 0`,
		"multi-episode":     `SELECT count(*) FROM items WHERE kind = 'episode' AND episode_end IS NOT NULL`,
		"extras":            `SELECT count(*) FROM items WHERE kind = 'extra' AND extra_kind IS NOT NULL`,
		"versions":          `SELECT count(*) FROM (SELECT item_id FROM media_files GROUP BY item_id HAVING count(*) > 1)`,
		"automatic trailer": `SELECT count(*) FROM trailers WHERE source <> 'manual'`,
		"manual trailer":    `SELECT count(*) FROM trailers WHERE source = 'manual'`,
		"credits":           `SELECT count(*) FROM credits JOIN people ON people.id = credits.person_id`,
		"manual match":      `SELECT count(*) FROM items WHERE locked_fields LIKE '%tmdb_id%' AND metadata_state = 'pending'`,
		"ignored file":      `SELECT count(*) FROM ignored_files`,
		"unplayable file":   `SELECT count(*) FROM media_files WHERE probe_error IS NOT NULL`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M3 state has no %s", what)
		}
	}
}

// Migration 00008 (M3) keeps the frozen states before it: the rows of items, media_files and
// playback_sessions survive, the new columns take their defaults, the new tables start empty and
// the container state follows the new rule (only seasons, episodes and podcast episodes count).
func TestMigration00008KeepsFrozenData(t *testing.T) {
	tables := []string{"items", "media_files", "playback_sessions"}
	for _, name := range []string{"M1b.sql", "M2.sql"} {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", name))
			if err != nil {
				t.Fatal(err)
			}
			cfg := t.TempDir()
			replay(t, filepath.Join(cfg, FileName), string(script))
			raw, err := sql.Open("sqlite", filepath.Join(cfg, FileName))
			if err != nil {
				t.Fatal(err)
			}
			before := map[string]int{}
			for _, table := range tables {
				var n int
				if err := raw.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
					t.Fatal(err)
				}
				before[table] = n
			}
			_ = raw.Close()

			db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, table := range tables {
				var n int
				if err := db.Read.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n != before[table] {
					t.Errorf("%s: %d rows before, %d after migration 00008", table, before[table], n)
				}
			}
			if before["items"] == 0 || before["media_files"] == 0 {
				t.Fatalf("%s carries no titles", name)
			}
			for query, want := range map[string]int{
				`SELECT count(*) FROM items WHERE trailer_state <> 'pending' OR season_number IS NOT NULL OR extra_kind IS NOT NULL`:                             0,
				`SELECT count(*) FROM media_files WHERE probe_error IS NOT NULL`:                                                                                 0,
				`SELECT (SELECT count(*) FROM trailers) + (SELECT count(*) FROM people) + (SELECT count(*) FROM credits) + (SELECT count(*) FROM ignored_files)`: 0,
				// Containers follow the new rule: available and min_child_rating from content children only.
				`SELECT count(*) FROM items AS i WHERE i.kind IN ('series', 'season', 'podcast') AND (
				     i.available <> EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = i.id AND c.deleted_at IS NULL AND c.available = 1
				                            AND c.kind IN ('season', 'episode', 'podcast_episode'))
				  OR i.min_child_rating IS NOT (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
				                                WHERE c.parent_id = i.id AND c.deleted_at IS NULL AND c.available = 1
				                                  AND c.kind IN ('season', 'episode', 'podcast_episode')))`: 0,
				// Every film with a present file is still available.
				`SELECT count(*) FROM items AS i WHERE i.kind = 'movie' AND i.available = 0
				   AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NULL)`: 0,
			} {
				var got int
				if err := db.Read.QueryRow(query).Scan(&got); err != nil {
					t.Fatalf("%s: %v", query, err)
				}
				if got != want {
					t.Errorf("%s = %d, want %d", query, got, want)
				}
			}
		})
	}
}
