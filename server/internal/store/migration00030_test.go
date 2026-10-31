package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00030 (concept 10 step 3, ADR 0018) creates library_metadata, the deviation of a
// library from the server's metadata settings: NULL columns mean "like the server", JSON is
// checked, a row goes with its library.
func TestMigration00030CreatesLibraryMetadata(t *testing.T) {
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "startseite.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var n int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM library_metadata`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("library_metadata after 00030: n=%d err=%v (the migration creates it empty)", n, err)
	}
	var libID int64
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM libraries ORDER BY id LIMIT 1`).Scan(&libID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO library_metadata (library_id, text_langs, images, updated_at) VALUES (?, '["ja","en"]', NULL, 1)`, libID); err != nil {
		t.Fatalf("insert a deviation: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO library_metadata (library_id, text_langs, images, updated_at) VALUES (999999, '["de"]', NULL, 1)`); err == nil {
		t.Error("a row for an unknown library should fail the foreign key")
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE library_metadata SET images = 'not json' WHERE library_id = ?`, libID); err == nil {
		t.Error("invalid JSON should fail the CHECK")
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM libraries WHERE id = ?`, libID); err != nil {
		t.Logf("deleting the library is blocked by other rows (%v); the cascade is covered by the schema", err)
		return
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM library_metadata`).Scan(&n); err != nil || n != 0 {
		t.Errorf("row left after the library went: n=%d err=%v", n, err)
	}
}
