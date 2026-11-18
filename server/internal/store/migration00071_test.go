package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00071 (concept 25 step 3.1, 08-datenmodell-api.md sections 2.11 and 3.2) on the newest
// frozen state (ordner-metadaten.sql) with two spotlight entries added before the run: the rebuilt
// spotlight_entries keeps both rows with NULL for the new TVDB and IMDb ids, carries the three
// indexes, accepts a show with a TVDB id alone (unmatched, tmdb_id NULL) and refuses a film without
// a TMDB id, an entry without any id, ids of 0 and an empty IMDb id.
func TestMigration00071SpotlightIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql")+`
INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at)
VALUES ('trending', 'movie', 1, 550, 'Fight Club', 1999, 7), ('trending', 'show', 1, 1399, NULL, NULL, 7);`)
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
		`SELECT count(*) FROM goose_db_version WHERE version_id = 71 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = 'spotlight_entries'
			AND name IN ('spotlight_entries_tmdb', 'spotlight_entries_tvdb', 'spotlight_entries_imdb')`: 3,
		`SELECT count(*) FROM sqlite_master WHERE name = 'spotlight_entries_new'`: 0,
		`SELECT count(*) FROM spotlight_entries WHERE list_key = 'trending' AND fetched_at = 7
			AND tvdb_id IS NULL AND imdb_id IS NULL
			AND ((media = 'movie' AND position = 1 AND tmdb_id = 550 AND title = 'Fight Club' AND year = 1999)
			  OR (media = 'show' AND position = 1 AND tmdb_id = 1399 AND title IS NULL AND year IS NULL))`: 2,
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
		`INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, tvdb_id, imdb_id, fetched_at)
			VALUES ('trending', 'show', 2, 1396, 81189, 'tt0903747', 8)`,
		`INSERT INTO spotlight_entries (list_key, media, position, tvdb_id, fetched_at) VALUES ('trending', 'show', 3, 777, 8)`,
		`INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, imdb_id, fetched_at) VALUES ('trending', 'movie', 2, 603, 'tt0133093', 8)`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for name, q := range map[string]string{
		"film without tmdb id": `INSERT INTO spotlight_entries (list_key, media, position, tvdb_id, imdb_id, fetched_at) VALUES ('trending', 'movie', 3, 5, 'tt1', 8)`,
		"show without any id":  `INSERT INTO spotlight_entries (list_key, media, position, imdb_id, fetched_at) VALUES ('trending', 'show', 4, 'tt2', 8)`,
		"tmdb id 0":            `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', 4, 0, 8)`,
		"tvdb id 0":            `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, tvdb_id, fetched_at) VALUES ('trending', 'show', 5, 9, 0, 8)`,
		"empty imdb id":        `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, imdb_id, fetched_at) VALUES ('trending', 'movie', 5, 9, '', 8)`,
		"unknown media":        `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'series', 1, 9, 8)`,
		"duplicate position":   `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'show', 2, 9, 8)`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: insert succeeded, want a constraint error", name)
		}
	}
}
