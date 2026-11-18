package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00075 (concept 25 step 4.1, 08-datenmodell-api.md section 2.10) on the newest frozen
// state (ordner-metadaten.sql): series_catalog comes empty with its index, the checks hold (TMDB
// id, in_production 0/1, seasons a JSON array, JSON episodes, one row per item, an existing item),
// and the row goes with its item.
func TestMigration00075SeriesCatalog(t *testing.T) {
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
		`SELECT count(*) FROM goose_db_version WHERE version_id = 75 AND is_applied = 1`:                1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                 0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'series_catalog'`:           1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'series_catalog_fetched'`:   1,
		`SELECT count(*) FROM series_catalog`:                                                           0,
		`SELECT count(*) FROM items WHERE id = '01M6585TWQDRBNT5QQW1HXW2JH' AND kind = 'series'`:        1,
		`SELECT count(*) FROM items WHERE id = '01M6585TWQDRBNT5QQW1HXW2JH' AND tmdb_id = 7101`:         1,
		`SELECT count(*) FROM items WHERE parent_id = '01M6585TWQDRBNT5QQW1HXW2JH' AND kind = 'season'`: 1,
		`SELECT count(*) FROM items WHERE id = '01M6585TWQDRBNT5QQW1HXW2JX'`:                            0,
		`SELECT count(*) FROM libraries WHERE id = 2`:                                                   1,
		`SELECT count(*) FROM pragma_table_info('series_catalog') WHERE name IN ('item_id', 'tmdb_id', 'status', 'in_production', 'seasons', 'next_episode', 'last_episode', 'fetched_at')`: 8,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	exec := func(query string) error {
		_, err := db.Write.ExecContext(ctx, query)
		return err
	}
	// A series of its own, so the delete below touches nothing of the frozen state.
	for _, q := range []string{
		`INSERT INTO items (id, kind, library_id, title, sort_title, tmdb_id, added_at, created_at, updated_at)
			VALUES ('01M6585TWQDRBNT5QQW1HXW2JX', 'series', 2, 'Catalog Show', 'catalog show', 1399, 1, 1, 1)`,
		`INSERT INTO series_catalog (item_id, tmdb_id, status, in_production, seasons, next_episode, last_episode, fetched_at)
			VALUES ('01M6585TWQDRBNT5QQW1HXW2JX', 1399, 'Returning Series', 1,
			'[{"number":0,"episodeCount":2},{"number":1,"episodeCount":10,"airDate":"2011-04-17"}]',
			'{"season":2,"episode":1,"airDate":"2026-12-01"}', '{"season":1,"episode":10,"airDate":"2011-06-19"}', 1)`,
		`INSERT INTO series_catalog (item_id, tmdb_id, fetched_at) VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 7101, 1)`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for name, q := range map[string]string{
		"tmdb id 0":              `INSERT INTO series_catalog (item_id, tmdb_id, fetched_at) VALUES ('01M6585TWSZF0B4KYY554V0591', 0, 1)`,
		"in_production 2":        `INSERT INTO series_catalog (item_id, tmdb_id, in_production, fetched_at) VALUES ('01M6585TWSZF0B4KYY554V0591', 1, 2, 1)`,
		"seasons not an array":   `INSERT INTO series_catalog (item_id, tmdb_id, seasons, fetched_at) VALUES ('01M6585TWSZF0B4KYY554V0591', 1, '{}', 1)`,
		"next episode not JSON":  `INSERT INTO series_catalog (item_id, tmdb_id, next_episode, fetched_at) VALUES ('01M6585TWSZF0B4KYY554V0591', 1, 'x', 1)`,
		"last episode not JSON":  `INSERT INTO series_catalog (item_id, tmdb_id, last_episode, fetched_at) VALUES ('01M6585TWSZF0B4KYY554V0591', 1, 'x', 1)`,
		"without fetched_at":     `INSERT INTO series_catalog (item_id, tmdb_id) VALUES ('01M6585TWSZF0B4KYY554V0591', 1)`,
		"duplicate item":         `INSERT INTO series_catalog (item_id, tmdb_id, fetched_at) VALUES ('01M6585TWQDRBNT5QQW1HXW2JX', 1399, 1)`,
		"unknown item":           `INSERT INTO series_catalog (item_id, tmdb_id, fetched_at) VALUES ('01M6585TWQDRBNT5QQW1HXNONE', 1, 1)`,
		"text in an integer col": `INSERT INTO series_catalog (item_id, tmdb_id, fetched_at) VALUES ('01M6585TWSZF0B4KYY554V0591', 'x', 1)`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: insert succeeded, want a constraint error", name)
		}
	}
	if got := count(`SELECT count(*) FROM series_catalog WHERE item_id = '01M6585TWQDRBNT5QQW1HXW2JH' AND status IS NULL AND in_production = 0 AND seasons = '[]' AND next_episode IS NULL AND last_episode IS NULL`); got != 1 {
		t.Errorf("defaults of a new row: %d rows, want 1", got)
	}
	if err := exec(`DELETE FROM items WHERE id = '01M6585TWQDRBNT5QQW1HXW2JX'`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM series_catalog WHERE item_id = '01M6585TWQDRBNT5QQW1HXW2JX'`); got != 0 {
		t.Errorf("catalog rows after their item went: %d, want 0", got)
	}
	if got := count(`SELECT count(*) FROM series_catalog`); got != 1 {
		t.Errorf("catalog rows of other items after the delete: %d, want 1", got)
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
