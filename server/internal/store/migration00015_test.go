package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Migration 00015 (M7) adds the search index, the genre table, the start page rows and the
// curation tables: the frozen M6 state must come out with every title searchable by a two-letter
// prefix, its genres split per language, the eight rows in the E-1 order and the picks kept; the
// triggers must keep the index and the genres in step afterwards.
func TestMigration00015IndexesFrozenData(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M6.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string, args ...any) int {
		var n int
		if err := db.Read.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	items := count(`SELECT count(*) FROM items`)
	if items == 0 {
		t.Fatal("frozen M6 state has no items")
	}
	if got := count(`SELECT count(*) FROM search_index WHERE kind = 'item'`); got != items {
		t.Errorf("search_index item rows = %d, want %d", got, items)
	}
	if got, want := count(`SELECT count(*) FROM search_index`), count(`SELECT count(*) FROM search_docs`); got != want {
		t.Errorf("search_index rows = %d, search_docs rows = %d (index out of step with its content table)", got, want)
	}
	// The FTS integrity check compares the index with the content table.
	if _, err := db.Write.Exec(`INSERT INTO search_index(search_index) VALUES ('integrity-check')`); err != nil {
		t.Errorf("integrity-check after the backfill: %v", err)
	}
	if got, want := count(`SELECT count(*) FROM search_index WHERE kind = 'text'`), count(`SELECT count(*) FROM item_texts`); got != want {
		t.Errorf("search_index text rows = %d, want %d", got, want)
	}
	// The two-letter prefix finds every title that starts with "Sl" (film names, series, books).
	want := count(`SELECT count(*) FROM items AS i WHERE i.title LIKE 'sl%' OR i.original_title LIKE 'sl%'
	               OR EXISTS (SELECT 1 FROM item_texts AS x WHERE x.item_id = i.id AND x.title LIKE 'sl%')`)
	if want == 0 {
		t.Fatal("frozen M6 state has no title starting with Sl")
	}
	if got := count(`SELECT count(DISTINCT item_id) FROM search_index WHERE search_index MATCH '"sl"*'`); got != want {
		t.Errorf(`prefix "sl"* finds %d titles, want %d`, got, want)
	}
	if got := count(`SELECT count(*) FROM home_rows`); got != 8 {
		t.Errorf("home_rows = %d, want 8", got)
	}
	if got := count(`SELECT count(*) FROM home_rows WHERE key = 'spotlight' AND position = 0`); got != 1 {
		t.Error("spotlight is not the first row")
	}
	// After the full run (00021, ADR 0011) the eight positions are exactly 0 to 7, in the default
	// order: released directly behind recent.
	defaultOrder := []string{"spotlight", "picks", "continue", "recent", "released", "watchlist", "collections", "genres"}
	if got := homeRowOrder(t, db); !slices.Equal(got, defaultOrder) {
		t.Errorf("home_rows order = %v, want %v", got, defaultOrder)
	}
	for _, col := range []string{"watchlist", "favorites", "collections", "collection_items", "item_genres",
		"spotlight_config", "spotlight_entries", "spotlight_overrides", "notifications"} {
		if got := count(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, col); got != 1 {
			t.Errorf("table %s missing after the full migration run", col)
		}
	}
	if got := count(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'trakt%'`); got != 0 {
		t.Error("the Trakt cache table of 00015 is still present after 00021")
	}

	// Triggers: a new title with genres in both forms is searchable and has its genres split.
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Write.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	lib := count(`SELECT min(id) FROM libraries`)
	exec(`INSERT INTO items (id, kind, library_id, title, sort_title, genres, added_at, created_at, updated_at)
	      VALUES ('01JM7000000000000000000001', 'movie', ?, 'Zebra Quest', 'zebra quest', '{"de":["Abenteuer"],"en":["Adventure","Drama"]}', 1, 1, 1)`, lib)
	exec(`INSERT INTO items (id, kind, library_id, title, sort_title, genres, added_at, created_at, updated_at)
	      VALUES ('01JM7000000000000000000002', 'movie', ?, 'Yak Story', 'yak story', '["Comedy"]', 1, 1, 1)`, lib)
	if got := count(`SELECT count(*) FROM item_genres WHERE item_id = '01JM7000000000000000000001'`); got != 3 {
		t.Errorf("item_genres of the object form = %d, want 3", got)
	}
	if got := count(`SELECT count(*) FROM item_genres WHERE item_id = '01JM7000000000000000000002'`); got != 2 {
		t.Errorf("item_genres of the plain list = %d, want 2 (both languages)", got)
	}
	if got := count(`SELECT count(*) FROM search_index WHERE search_index MATCH '"ze"*' AND item_id = '01JM7000000000000000000001'`); got != 1 {
		t.Errorf("new title not found by prefix (%d rows)", got)
	}
	exec(`UPDATE items SET title = 'Xylophone Nights', genres = '{"de":["Musik"],"en":["Music"]}' WHERE id = '01JM7000000000000000000001'`)
	if got := count(`SELECT count(*) FROM search_index WHERE search_index MATCH '"xy"*' AND item_id = '01JM7000000000000000000001'`); got != 1 {
		t.Errorf("renamed title not found by prefix (%d rows)", got)
	}
	if got := count(`SELECT count(*) FROM search_index WHERE search_index MATCH '"ze"*' AND item_id = '01JM7000000000000000000001'`); got != 0 {
		t.Errorf("old title still indexed (%d rows)", got)
	}
	if got := count(`SELECT count(*) FROM item_genres WHERE item_id = '01JM7000000000000000000001' AND name = 'Musik'`); got != 1 {
		t.Error("genre update not applied")
	}
	// Texts, people and collections.
	exec(`INSERT INTO item_texts (item_id, lang, title) VALUES ('01JM7000000000000000000001', 'de', 'Nächte am Xylofon')`)
	if got := count(`SELECT count(*) FROM search_index WHERE search_index MATCH '"nach"*' AND item_id = '01JM7000000000000000000001'`); got != 1 {
		t.Errorf("text title (diacritics folded) not found (%d rows)", got)
	}
	exec(`INSERT INTO people (id, tmdb_id, name, created_at) VALUES (777001, 777001, 'Quentin Example', 1)`)
	exec(`INSERT INTO credits (item_id, person_id, role, ord) VALUES ('01JM7000000000000000000001', 777001, 'cast', 0)`)
	if got := count(`SELECT count(*) FROM search_index WHERE kind = 'person' AND search_index MATCH '"quen"*'`); got != 1 {
		t.Errorf("person not indexed (%d rows)", got)
	}
	exec(`UPDATE people SET name = 'Quincy Example' WHERE id = 777001`)
	if got := count(`SELECT count(*) FROM search_index WHERE kind = 'person' AND search_index MATCH '"quin"*'`); got != 1 {
		t.Errorf("renamed person not indexed (%d rows)", got)
	}
	exec(`INSERT INTO collections (id, source, name, name_de, created_at, updated_at) VALUES ('01JM7C00000000000000000001', 'manual', 'Winter Set', NULL, 1, 1)`)
	exec(`INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES ('01JM7C00000000000000000001', '01JM7000000000000000000001', 0, 1)`)
	if got := count(`SELECT count(*) FROM search_index WHERE kind = 'collection' AND search_index MATCH '"wint"*'`); got != 1 {
		t.Errorf("collection not indexed (%d rows)", got)
	}
	exec(`UPDATE collections SET name_de = 'Winterreihe' WHERE id = '01JM7C00000000000000000001'`)
	if got := count(`SELECT count(*) FROM search_index WHERE kind = 'collection' AND search_index MATCH '"winterr"*'`); got != 1 {
		t.Errorf("renamed collection not indexed (%d rows)", got)
	}
	// An update that changes nothing leaves the index untouched (no trigger work).
	exec(`UPDATE items SET title = title WHERE id = '01JM7000000000000000000001'`)
	exec(`DELETE FROM items WHERE id = '01JM7000000000000000000001'`)
	if got := count(`SELECT count(*) FROM search_index WHERE item_id = '01JM7000000000000000000001'`); got != 0 {
		t.Errorf("deleted title left %d index rows", got)
	}
	if got := count(`SELECT count(*) FROM search_docs WHERE item_id = '01JM7000000000000000000001'`); got != 0 {
		t.Errorf("deleted title left %d docs rows", got)
	}
	if _, err := db.Write.Exec(`INSERT INTO search_index(search_index) VALUES ('integrity-check')`); err != nil {
		t.Errorf("integrity-check after the trigger run: %v", err)
	}
	if got := count(`SELECT count(*) FROM item_genres WHERE item_id = '01JM7000000000000000000001'`); got != 0 {
		t.Errorf("deleted title left %d genre rows", got)
	}
}
