package store

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// Migration 00053 (concept 20 section 6.3, contract 1.31.0) adds progress.watched_at on the frozen
// state of concept 19 stage B2 (radio-wikidata.sql, the newest one before it). The state holds no
// progress, so the test adds three rows before the migration: a completed film gets watched_at =
// updated_at, a started one and an episode at position 0 stay NULL; the partial index is there,
// and a negative watched_at is refused.
func TestMigration00053ProgressWatchedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "radio-wikidata.sql"))
	raw := openRaw(t, path)
	if _, err := raw.ExecContext(ctx, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES
		(1, '01M6585TXK7NNK68XFTPTBESVF', 9500, 10000, 1, 1792000000001),
		(1, '01M6585TXM70G9GSBN41B2G7D0', 4000, 10000, 0, 1792000000002),
		(2, '01M6585TWSZF0B4KYY554YCSGP', 0, 8000, 0, 1792000000003)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
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
	for query, want := range map[string]string{
		`SELECT group_concat(item_id || '=' || COALESCE(watched_at, '-'), ',') FROM (SELECT * FROM progress ORDER BY updated_at)`: "01M6585TXK7NNK68XFTPTBESVF=1792000000001,01M6585TXM70G9GSBN41B2G7D0=-,01M6585TWSZF0B4KYY554YCSGP=-",
		`SELECT group_concat(completed, ',') FROM (SELECT * FROM progress ORDER BY updated_at)`:                                   "1,0,0",
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                           "0",
		`SELECT count(*) FROM goose_db_version WHERE version_id = 53 AND is_applied = 1`:                                          "1",
	} {
		if got := text(query); got != want {
			t.Errorf("%s = %s, want %s", query, got, want)
		}
	}
	if ddl := text(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'progress_watched'`); !strings.Contains(ddl, "(profile_id, watched_at DESC) WHERE watched_at IS NOT NULL") {
		t.Errorf("progress_watched = %q, want partial on watched_at IS NOT NULL", ddl)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE progress SET watched_at = -1`); err == nil {
		t.Error("a negative watched_at was accepted")
	}
}
