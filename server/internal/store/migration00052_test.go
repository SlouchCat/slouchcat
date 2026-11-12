package store

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// Migration 00052 makes media_files_missing partial on missing_since IS NOT NULL: from the frozen
// state of concept 19 stage B2 (radio-wikidata.sql) the index is there with its WHERE, SQLite
// still finds it for the missing files and no longer for the present ones (IS NULL).
func TestMigration00052MediaFilesMissingPartial(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "radio-wikidata.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var ddl string
	if err := db.Read.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'media_files_missing'`).Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "WHERE missing_since IS NOT NULL") {
		t.Errorf("media_files_missing = %q, want partial on missing_since IS NOT NULL", ddl)
	}
	var applied int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id = 52 AND is_applied = 1`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Error("migration 00052 not recorded as applied")
	}
	plan := func(query string) string {
		t.Helper()
		rows, err := db.Read.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id, parent, notused int
			var detail string
			if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
				t.Fatal(err)
			}
			out = append(out, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return strings.Join(out, "\n")
	}
	if p := plan(`SELECT id FROM media_files WHERE missing_since < 100`); !strings.Contains(p, "media_files_missing") {
		t.Errorf("missing files do not use media_files_missing:\n%s", p)
	}
	if p := plan(`SELECT id FROM media_files WHERE missing_since IS NULL`); strings.Contains(p, "media_files_missing") {
		t.Errorf("present files use media_files_missing:\n%s", p)
	}
}
