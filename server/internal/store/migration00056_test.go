package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00056 (concept 20 section 9.3 after the acceptance on CT 190) adds
// libraries.deleting_at on the frozen state of stage 5 (loeschen.sql, the newest one before it).
// No library is marked after the migration and every title stays visible; a marked library takes
// its titles out of visible_items and gives them back when the mark goes; a negative mark is
// refused.
func TestMigration00056LibraryDeleting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "loeschen.sql"))
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
	const films = `SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id WHERE v.profile_id = 1 AND i.library_id = 1`
	before := count(films)
	if before == 0 {
		t.Fatal("the frozen state shows no film to profile 1")
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM libraries WHERE deleting_at IS NOT NULL`:                   0,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM goose_db_version WHERE version_id = 56 AND is_applied = 1`: 1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE libraries SET deleting_at = 1794571200000 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if n := count(films); n != 0 {
		t.Errorf("titles of a deleting library visible: %d", n)
	}
	if n := count(`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id WHERE v.profile_id = 1 AND i.library_id = 2`); n == 0 {
		t.Error("the other library went with it")
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE libraries SET deleting_at = NULL WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if n := count(films); n != before {
		t.Errorf("titles after the mark went = %d, want %d", n, before)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE libraries SET deleting_at = -1 WHERE id = 1`); err == nil {
		t.Error("a negative deleting_at was accepted")
	}
}
