package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00028 (concept 10 step 1, ADR 0018) rebuilds item_texts and item_genres without the
// de/en CHECK, adds item_texts.rank and profiles.metadata_language and creates the search and genre
// triggers again. Over the frozen startseite state (matched series with texts and sort keys):
// the triggers and indexes are back, the ranks of the existing rows follow today's order, a text
// in a third language reaches the search index, and a genres object with a third key fills
// item_genres for it.
func TestMigration00028KeepsTriggersAndOpensTheLanguages(t *testing.T) {
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "startseite.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	for _, name := range []string{"search_texts_insert", "search_texts_update", "search_texts_delete", "items_genres_insert", "items_genres_update"} {
		var n int
		if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("trigger %s after 00028: n=%d err=%v", name, n, err)
		}
	}
	for _, name := range []string{"item_texts_sort_key", "item_texts_rank", "item_genres_name"} {
		var n int
		if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("index %s after 00028: n=%d err=%v", name, n, err)
		}
	}
	var deRank, enRank, texts int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_texts`).Scan(&texts); err != nil || texts == 0 {
		t.Fatalf("item_texts after 00028: n=%d err=%v", texts, err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_texts WHERE lang = 'de' AND rank <> 0`).Scan(&deRank); err != nil || deRank != 0 {
		t.Errorf("de rows without rank 0: %d (%v)", deRank, err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_texts WHERE lang = 'en' AND rank <> 1`).Scan(&enRank); err != nil || enRank != 0 {
		t.Errorf("en rows without rank 1: %d (%v)", enRank, err)
	}
	var sortKeys int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_texts WHERE sort_key IS NOT NULL AND sort_key <> ''`).Scan(&sortKeys); err != nil || sortKeys == 0 {
		t.Errorf("sort keys of 00026 lost in the rebuild: %d (%v)", sortKeys, err)
	}

	// A third language in the texts reaches the search index through the recreated trigger.
	var itemID string
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM items WHERE kind = 'series' AND deleted_at IS NULL LIMIT 1`).Scan(&itemID); err != nil {
		t.Fatalf("no series in the frozen state: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO item_texts (item_id, lang, title, rank, sort_key) VALUES (?, 'es', 'Serie de prueba única', 2, 'serie de prueba unica')`, itemID); err != nil {
		t.Fatalf("insert es text (the CHECK should be gone): %v", err)
	}
	var hits int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM search_index WHERE search_index MATCH 'prueba' AND item_id = ?`, itemID).Scan(&hits); err != nil || hits == 0 {
		t.Errorf("search after an es text: hits=%d err=%v", hits, err)
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM item_texts WHERE item_id = ? AND lang = 'es'`, itemID); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM search_index WHERE search_index MATCH 'prueba' AND item_id = ?`, itemID).Scan(&hits); err != nil || hits != 0 {
		t.Errorf("search after deleting the es text: hits=%d err=%v", hits, err)
	}

	// A genres object with a third key fills item_genres for that language; a plain array stands
	// for de and en as before.
	if _, err := db.Write.ExecContext(ctx, `UPDATE items SET genres = '{"de":["Drama"],"en":["Drama"],"es":["Aventura"]}' WHERE id = ?`, itemID); err != nil {
		t.Fatal(err)
	}
	var es int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_genres WHERE item_id = ? AND lang = 'es' AND name = 'Aventura'`, itemID).Scan(&es); err != nil || es != 1 {
		t.Errorf("item_genres es row after the genres update: n=%d err=%v", es, err)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE items SET genres = '["Krimi"]' WHERE id = ?`, itemID); err != nil {
		t.Fatal(err)
	}
	var plain int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_genres WHERE item_id = ? AND name = 'Krimi'`, itemID).Scan(&plain); err != nil || plain != 2 {
		t.Errorf("item_genres rows for a plain array: n=%d, want 2 (de and en); err=%v", plain, err)
	}

	// profiles.metadata_language exists and is NULL for every existing profile.
	var withMeta int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM profiles WHERE metadata_language IS NOT NULL`).Scan(&withMeta); err != nil || withMeta != 0 {
		t.Errorf("profiles.metadata_language: n=%d err=%v", withMeta, err)
	}
}
