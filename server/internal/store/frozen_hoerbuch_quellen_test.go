package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 19 stage D3 (migration 00046; testdata/db/hoerbuch-quellen.sql)
// carries an audiobook folder with the book sources of D6.2, a book matched by hand at Audible, a
// book in review with two candidates and a unified author name, a book without hit, a book in
// error with a candidate and a book not tried yet. TestFrozenStatesMigrate replays it like every
// state; this test checks that each of those is really in it.
func TestFrozenHoerbuchQuellenStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-quellen.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"an audiobook folder with the book sources": `SELECT count(*) FROM library_roots WHERE content = 'audiobook' AND providers = '["local","audible","dnb","applebooks"]'`,
		"a book matched by hand":                    `SELECT count(*) FROM items WHERE book_match_state = 'matched' AND book_match_manual = 1 AND book_match_provider = 'audible' AND book_match_region IS NOT NULL`,
		"a book in review with two candidates":      `SELECT count(*) FROM items AS i WHERE i.book_match_state = 'review' AND (SELECT count(*) FROM book_match_candidates AS c WHERE c.item_id = i.id) = 2`,
		"a unified author name":                     `SELECT count(*) FROM items WHERE author_display IS NOT NULL AND author_display <> author`,
		"a book without hit":                        `SELECT count(*) FROM items WHERE book_match_state = 'none'`,
		"a book in error with its reason":           `SELECT count(*) FROM items WHERE book_match_state = 'error' AND book_match_error IS NOT NULL`,
		"a book not tried yet":                      `SELECT count(*) FROM items WHERE kind = 'audiobook' AND book_match_state IS NULL`,
		"the unified name in the search index":      `SELECT count(*) FROM search_index WHERE search_index MATCH '"F. Author"'`,
		"migration 00046 recorded as applied":       `SELECT count(*) FROM goose_db_version WHERE version_id = 46 AND is_applied = 1`,
		"foreign keys of the frozen state hold":     `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen hoerbuch-quellen state has no %s", what)
		}
	}
}
