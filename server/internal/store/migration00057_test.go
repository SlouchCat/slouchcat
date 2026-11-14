package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00057 (concept 21 P-1, O-2) adds trailers.size on the frozen state after 00056
// (loeschen-job.sql, the newest one before it). The state carries no trailer, so the test stores
// two before the migration: both keep their row and get NULL (unknown). Afterwards a trailer takes
// a TMDB size, NULL stays allowed, and zero or a negative size is refused.
func TestMigration00057TrailerSize(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "loeschen-job.sql"))
	raw := openRaw(t, path)
	var itemID string
	if err := raw.QueryRowContext(ctx, `SELECT id FROM items WHERE kind = 'movie' AND deleted_at IS NULL ORDER BY id LIMIT 1`).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at)
		VALUES (?, 'de', 'tmdbTrl0001', 'Trailer', 'tmdb', 1, 1794571200000), (?, 'en', 'manualT0001', NULL, 'manual', 0, 1794571200000)`,
		itemID, itemID); err != nil {
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
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM trailers`:                                                  2,
		`SELECT count(*) FROM trailers WHERE size IS NULL`:                               2,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM goose_db_version WHERE version_id = 57 AND is_applied = 1`: 1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE trailers SET size = 1080 WHERE youtube_id = 'tmdbTrl0001'`); err != nil {
		t.Fatalf("a TMDB size was refused: %v", err)
	}
	if n := count(`SELECT count(*) FROM trailers WHERE size = 1080`); n != 1 {
		t.Errorf("trailers with size 1080 = %d, want 1", n)
	}
	for _, size := range []int{0, -360} {
		if _, err := db.Write.ExecContext(ctx, `UPDATE trailers SET size = ? WHERE youtube_id = 'manualT0001'`, size); err == nil {
			t.Errorf("size %d was accepted", size)
		}
	}
}
