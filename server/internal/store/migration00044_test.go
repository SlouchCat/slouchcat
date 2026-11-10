package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
)

// Migration 00044 (1.24.0, concept 19 stage D1) adds publisher, ISBN and ASIN to items, sets books
// that a refresh had left skipped with unsupported_kind back to matched and gives every book
// without original language the language of its text rows, on the frozen state of migration 00043
// (trickplay-aus.sql). The state holds no audiobook, so the test adds five before the migration.
func TestMigration00044AudiobookFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "trickplay-aus.sql"))
	raw := openRaw(t, path)
	for _, stmt := range []string{
		`INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at, metadata_state, metadata_error)
		 VALUES ('BOOKSKIPPED000000000000000', 'audiobook', 3, 'Deutsch', 'deutsch', 1, 1, 1, 'skipped', 'unsupported_kind'),
		        ('BOOKENGLISH000000000000000', 'audiobook', 3, 'English', 'english', 1, 1, 1, 'matched', NULL),
		        ('BOOKNOTEXT0000000000000000', 'audiobook', 3, 'Ohne Text', 'ohne text', 1, 1, 1, 'matched', NULL)`,
		`INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at, metadata_state,
		   original_language, locked_fields)
		 VALUES ('BOOKLOCKED0000000000000000', 'audiobook', 3, 'Gesperrt', 'gesperrt', 1, 1, 1, 'matched', 'fr', '["original_language"]')`,
		`INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at, metadata_state, metadata_error)
		 VALUES ('FILMSKIPPED000000000000000', 'movie', 1, 'Film', 'film', 1, 1, 1, 'skipped', 'unsupported_kind')`,
		`INSERT INTO item_texts (item_id, lang, title, sort_key) VALUES ('BOOKSKIPPED000000000000000', 'de', 'Deutsch', 'deutsch'),
		   ('BOOKSKIPPED000000000000000', 'en', 'Deutsch', 'deutsch'), ('BOOKENGLISH000000000000000', 'en', 'English', 'english'),
		   ('BOOKLOCKED0000000000000000', 'de', 'Gesperrt', 'gesperrt')`,
	} {
		if _, err := raw.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var violations int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign_key_check after 00044: %d violations (%v)", violations, err)
	}
	want := map[string]string{
		"BOOKSKIPPED000000000000000": "matched||de",
		"BOOKENGLISH000000000000000": "matched||en",
		"BOOKNOTEXT0000000000000000": "matched||-",
		"BOOKLOCKED0000000000000000": "matched||fr",
		"FILMSKIPPED000000000000000": "skipped|unsupported_kind|-", // only books need no match
	}
	for id, w := range want {
		var got string
		if err := db.Read.QueryRowContext(ctx, `SELECT metadata_state || '|' || coalesce(metadata_error, '') || '|' ||
			coalesce(original_language, '-') FROM items WHERE id = ?`, id).Scan(&got); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if got != w {
			t.Errorf("%s after 00044: %q, want %q", id, got, w)
		}
	}
	var publisher, isbn, asin sql.NullString
	if err := db.Read.QueryRowContext(ctx, `SELECT publisher, isbn, asin FROM items WHERE id = 'BOOKENGLISH000000000000000'`).Scan(&publisher, &isbn, &asin); err != nil {
		t.Fatal(err)
	}
	if publisher.Valid || isbn.Valid || asin.Valid {
		t.Errorf("new columns of an existing book: %v %v %v, want NULL", publisher, isbn, asin)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE items SET publisher = 'Verlag', isbn = '9783000000000', asin = 'B0EXAMPLE1' WHERE id = 'BOOKENGLISH000000000000000'`); err != nil {
		t.Errorf("write the new columns: %v", err)
	}
}
