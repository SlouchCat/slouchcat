package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 19 stage D1 (migration 00044; testdata/db/hoerbuch-bleistift.sql)
// carries a German book edited with the pencil (both text rows locked per language, the author and
// the volume locked, publisher, ISBN and ASIN, a chosen folder cover) and an English book from the
// files with an embedded cover and provenance local. TestFrozenStatesMigrate replays it like every
// state; this test checks that each of those is really in it.
func TestFrozenHoerbuchBleistiftStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-bleistift.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a book with publisher, ISBN and ASIN":  `SELECT count(*) FROM items WHERE kind = 'audiobook' AND publisher <> '' AND isbn <> '' AND asin <> ''`,
		"text locks per language on both rows":  `SELECT count(*) FROM items WHERE kind = 'audiobook' AND instr(locked_fields, '"title:de"') > 0 AND instr(locked_fields, '"title:en"') > 0`,
		"a locked book field":                   `SELECT count(*) FROM items WHERE kind = 'audiobook' AND instr(locked_fields, '"author"') > 0`,
		"original languages de and en":          `SELECT count(DISTINCT original_language) = 2 FROM items WHERE kind = 'audiobook'`,
		"a book with only an English text row":  `SELECT count(*) FROM items AS i WHERE kind = 'audiobook' AND NOT EXISTS (SELECT 1 FROM item_texts WHERE item_id = i.id AND lang <> 'en')`,
		"provenance local of book fields":       `SELECT count(*) FROM item_field_sources AS s JOIN items AS i ON i.id = s.item_id WHERE i.kind = 'audiobook' AND s.provider = 'local'`,
		"a chosen cover of a book":              `SELECT count(*) FROM item_images AS m JOIN items AS i ON i.id = m.item_id WHERE i.kind = 'audiobook' AND m.chosen = 1`,
		"an embedded cover":                     `SELECT count(*) FROM items WHERE kind = 'audiobook' AND cover_path LIKE 'embedded:%'`,
		"migration 00044 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 44 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen hoerbuch-bleistift state has no %s", what)
		}
	}
}
