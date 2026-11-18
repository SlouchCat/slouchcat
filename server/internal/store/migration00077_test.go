package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00077 (concept 25 step 4.6d, 08-datenmodell-api.md section 2.16) on the newest frozen
// state (ordner-metadaten.sql): wish_progress comes empty with its index and trigger, the checks
// hold (stages, percent only with download, episodes only for Sonarr, errors, message length) and
// the row goes when the wish leaves procurement or is deleted.
func TestMigration00077WishProgress(t *testing.T) {
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
		`SELECT count(*) FROM goose_db_version WHERE version_id = 77 AND is_applied = 1`:              1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                               0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'wish_progress'`:          1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'wish_progress_service'`:  1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'wish_progress_closed'`: 1,
		`SELECT count(*) FROM wish_progress`:                                                          0,
		`SELECT count(*) FROM pragma_table_info('wish_progress') WHERE name IN ('wish_id', 'service', 'arr_title_id', 'stage', 'percent', 'eta_seconds', 'episodes_done', 'episodes_total', 'admin_message', 'error', 'has_file', 'title_read_at', 'read_at')`: 13,
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
		`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (1, 'movie', 603, 'The Matrix', 'slouchcat', 'processing', 1, 1)`,
		`INSERT INTO wishes (id, media, tmdb_id, tvdb_id, title, origin, state, created_at, updated_at) VALUES (2, 'show', 1399, 121361, 'Game of Thrones', 'slouchcat', 'partial', 1, 1)`,
		`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (3, 'movie', 604, 'X', 'slouchcat', 'processing', 1, 1)`,
		`INSERT INTO wish_progress (wish_id, service, arr_title_id, stage, percent, eta_seconds, admin_message, read_at) VALUES (1, 'radarr', 11, 'download', 45, 720, '(SABnzbd)', 5)`,
		`INSERT INTO wish_progress (wish_id, service, arr_title_id, stage, episodes_done, episodes_total, error, read_at) VALUES (2, 'sonarr', 21, 'search', 3, 10, 'incomplete', 5)`,
		`INSERT INTO wish_progress (wish_id, service, error, read_at) VALUES (3, 'radarr', 'unreachable', 5)`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for name, q := range map[string]string{
		"second row":              `INSERT INTO wish_progress (wish_id, service, read_at) VALUES (1, 'radarr', 1)`,
		"unknown wish":            `INSERT INTO wish_progress (wish_id, service, read_at) VALUES (99, 'radarr', 1)`,
		"unknown service":         `UPDATE wish_progress SET service = 'lidarr' WHERE wish_id = 3`,
		"unknown stage":           `UPDATE wish_progress SET stage = 'done' WHERE wish_id = 3`,
		"percent with search":     `UPDATE wish_progress SET stage = 'search', percent = 10 WHERE wish_id = 3`,
		"percent without stage":   `UPDATE wish_progress SET percent = 10 WHERE wish_id = 3`,
		"eta with import":         `UPDATE wish_progress SET stage = 'import', eta_seconds = 10 WHERE wish_id = 3`,
		"percent 101":             `UPDATE wish_progress SET percent = 101 WHERE wish_id = 1`,
		"episodes for radarr":     `UPDATE wish_progress SET episodes_done = 1, episodes_total = 2 WHERE wish_id = 1`,
		"done without total":      `UPDATE wish_progress SET episodes_total = NULL WHERE wish_id = 2`,
		"unknown error":           `UPDATE wish_progress SET error = 'gone' WHERE wish_id = 3`,
		"message too long":        `UPDATE wish_progress SET admin_message = printf('%.501c', 'a') WHERE wish_id = 1`,
		"empty message":           `UPDATE wish_progress SET admin_message = '' WHERE wish_id = 1`,
		"without read_at":         `UPDATE wish_progress SET read_at = NULL WHERE wish_id = 1`,
		"has_file 2":              `UPDATE wish_progress SET has_file = 2 WHERE wish_id = 1`,
		"title id 0":              `UPDATE wish_progress SET arr_title_id = 0 WHERE wish_id = 1`,
		"text in an integer col.": `UPDATE wish_progress SET percent = 'x' WHERE wish_id = 1`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: statement succeeded, want a constraint error", name)
		}
	}
	// A state change inside procurement keeps the row; leaving it removes the row.
	if err := exec(`UPDATE wishes SET state = 'partial', updated_at = 2 WHERE id = 1`); err == nil {
		t.Error("a film wish became partial")
	}
	if err := exec(`UPDATE wishes SET state = 'processing', updated_at = 2 WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM wish_progress WHERE wish_id = 2`); got != 1 {
		t.Errorf("row after partial to processing: %d, want 1", got)
	}
	if err := exec(`UPDATE wishes SET state = 'available', updated_at = 3 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM wish_progress WHERE wish_id = 1`); got != 0 {
		t.Errorf("row of an available wish: %d, want 0", got)
	}
	if err := exec(`DELETE FROM wishes WHERE id = 3`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM wish_progress`); got != 1 {
		t.Errorf("rows after the delete: %d, want 1", got)
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
