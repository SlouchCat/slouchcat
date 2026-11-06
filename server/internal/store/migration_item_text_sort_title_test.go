package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The migration item_text_sort_title (concept 15 7.5, P-6) adds the sort title as the
// administrator typed it to item_texts: every text row of the frozen state after concept 15 stage 2
// (testdata/db/feinschliff.sql) stays as it was and gets NULL, and the column takes a text.
func TestMigrationItemTextSortTitle(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "feinschliff.sql"))
	const fingerprint = `SELECT count(*), coalesce(sum(length(title) + length(coalesce(sort_key, '')) + rank), 0) FROM item_texts`
	raw := openRaw(t, path)
	var rowsBefore, sumBefore int
	if err := raw.QueryRow(fingerprint).Scan(&rowsBefore, &sumBefore); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if rowsBefore == 0 {
		t.Fatal("frozen feinschliff state has no text rows")
	}
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var rows, sum, typed int
	if err := db.Read.QueryRow(fingerprint).Scan(&rows, &sum); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRow(`SELECT count(*) FROM item_texts WHERE sort_title IS NOT NULL`).Scan(&typed); err != nil {
		t.Fatal(err)
	}
	if rows != rowsBefore || sum != sumBefore || typed != 0 {
		t.Errorf("text rows %d (was %d), fingerprint %d (was %d), typed sort titles %d; want nothing changed and NULL",
			rows, rowsBefore, sum, sumBefore, typed)
	}
	ctx := context.Background()
	if _, err := db.Write.ExecContext(ctx, `UPDATE item_texts SET sort_title = 'Herr der Ringe, Der'`); err != nil {
		t.Errorf("write a sort title: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE item_texts SET sort_title = x'00'`); err == nil {
		t.Error("a blob sort title was accepted, want the STRICT type error")
	}
}
