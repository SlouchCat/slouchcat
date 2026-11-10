package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00045 (1.25.0, concept 19 stage D2) adds audio_series and items.audio_series_id on the
// frozen state of migration 00044 (hoerbuch-bleistift.sql): the books keep their rows without a
// series (the first start links them, library.RelinkAudioSeriesOnNewKeys), a series is unique per
// library and key, and deleting a series sets the link of its books NULL.
func TestMigration00045AudioSeries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-bleistift.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var violations, linked, books int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign_key_check after 00045: %d violations (%v)", violations, err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*), count(audio_series_id) FROM items WHERE kind = 'audiobook'`).Scan(&books, &linked); err != nil {
		t.Fatal(err)
	}
	if books == 0 || linked != 0 {
		t.Errorf("after 00045: %d books, %d linked; want books without series", books, linked)
	}
	for _, stmt := range []string{
		`INSERT INTO audio_series (id, library_id, series_key, name, created_at, updated_at) VALUES ('01M6585TZB00SER1E50000000A', 3, 'k', 'K', 1, 1)`,
		`UPDATE items SET audio_series_id = '01M6585TZB00SER1E50000000A' WHERE kind = 'audiobook'`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO audio_series (id, library_id, series_key, name, created_at, updated_at) VALUES ('01M6585TZB00SER1E50000000B', 3, 'k', 'K', 1, 1)`); err == nil {
		t.Error("a second series with the same key in one library was accepted")
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM audio_series WHERE id = '01M6585TZB00SER1E50000000A'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(audio_series_id) FROM items`).Scan(&linked); err != nil || linked != 0 {
		t.Errorf("links after deleting the series: %d (%v), want 0", linked, err)
	}
}
