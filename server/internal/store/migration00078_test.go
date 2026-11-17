package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00078 (concept 25 step 5.1, 08-datenmodell-api.md section 2.15) on the newest frozen
// state (ordner-metadaten.sql): wish_discover_pages comes empty with its index, the checks hold
// (key, a JSON array of items, page count) and a second row of the same key replaces the first.
func TestMigration00078WishDiscover(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 78 AND is_applied = 1`:                                                  1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                                   0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'wish_discover_pages'`:                                        1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'wish_discover_pages_fetched'`:                                1,
		`SELECT count(*) FROM wish_discover_pages`:                                                                                        0,
		`SELECT count(*) FROM pragma_table_info('wish_discover_pages') WHERE name IN ('cache_key', 'items', 'total_pages', 'fetched_at')`: 4,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	exec := func(query string) error {
		_, err := db.Write.ExecContext(ctx, query)
		return err
	}
	for _, q := range []string{
		`INSERT INTO wish_discover_pages (cache_key, items, total_pages, fetched_at) VALUES ('trending|p=1|de|12', '[{"media":"movie","tmdbId":603}]', 500, 1)`,
		`INSERT INTO wish_discover_pages (cache_key, items, fetched_at) VALUES ('movie_genres|de|18', '[]', 1)`,
		`INSERT INTO wish_discover_pages (cache_key, items, total_pages, fetched_at) VALUES ('trending|p=1|de|12', '[]', 1, 2)
		 ON CONFLICT (cache_key) DO UPDATE SET items = excluded.items, total_pages = excluded.total_pages, fetched_at = excluded.fetched_at`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if got := count(`SELECT count(*) FROM wish_discover_pages WHERE cache_key = 'trending|p=1|de|12' AND items = '[]' AND fetched_at = 2`); got != 1 {
		t.Errorf("replaced row: %d, want 1", got)
	}
	for name, q := range map[string]string{
		"same key":          `INSERT INTO wish_discover_pages (cache_key, items, fetched_at) VALUES ('movie_genres|de|18', '[]', 3)`,
		"empty key":         `INSERT INTO wish_discover_pages (cache_key, items, fetched_at) VALUES ('', '[]', 3)`,
		"key too long":      `INSERT INTO wish_discover_pages (cache_key, items, fetched_at) VALUES (printf('%.501c', 'k'), '[]', 3)`,
		"items not JSON":    `INSERT INTO wish_discover_pages (cache_key, items, fetched_at) VALUES ('a', 'nope', 3)`,
		"items an object":   `INSERT INTO wish_discover_pages (cache_key, items, fetched_at) VALUES ('b', '{}', 3)`,
		"negative pages":    `INSERT INTO wish_discover_pages (cache_key, items, total_pages, fetched_at) VALUES ('c', '[]', -1, 3)`,
		"without fetch":     `INSERT INTO wish_discover_pages (cache_key, items) VALUES ('d', '[]')`,
		"text in an int":    `INSERT INTO wish_discover_pages (cache_key, items, total_pages, fetched_at) VALUES ('e', '[]', 'x', 3)`,
		"without the items": `INSERT INTO wish_discover_pages (cache_key, fetched_at) VALUES ('f', 3)`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: statement succeeded, want a constraint error", name)
		}
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
