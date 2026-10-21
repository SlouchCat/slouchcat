package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen M7 state carries every kind of M7 data (testdata/db/README.md), so later migrations
// are tested against picks (global and per profile with note and window), a manual collection
// with members, a watchlist entry, favourites, a changed row order with a hidden row, the Trakt
// lists of a run and an in-app notification; the search index and the genres come from the
// triggers.
func TestFrozenM7StateCoversM7Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M7.sql"))
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
	for what, query := range map[string]string{
		"global pick with a note":                `SELECT count(*) FROM top_picks WHERE profile_id IS NULL AND note <> ''`,
		"pick of one profile with a time window": `SELECT count(*) FROM top_picks WHERE profile_id IS NOT NULL AND starts_at IS NOT NULL AND ends_at IS NOT NULL`,
		"manual collection with ordered members": `SELECT count(*) FROM collections c WHERE c.source = 'manual' AND (SELECT count(*) FROM collection_items m WHERE m.collection_id = c.id) = 2`,
		"collection in the search index":         `SELECT count(*) FROM search_index WHERE kind = 'collection'`,
		"watchlist entry":                        `SELECT count(*) FROM watchlist`,
		"favourite series":                       `SELECT count(*) FROM favorites f JOIN items i ON i.id = f.item_id WHERE i.kind = 'series'`,
		"favourite podcast":                      `SELECT count(*) FROM favorites f JOIN items i ON i.id = f.item_id WHERE i.kind = 'podcast'`,
		"changed row order with a hidden row":    `SELECT count(*) FROM home_rows WHERE key = 'genres' AND hidden = 1`,
		"eight home rows":                        `SELECT count(*) FROM home_rows WHERE (SELECT count(*) FROM home_rows) = 8`,
		"trakt rows of both lists":               `SELECT count(*) FROM trakt_lists t WHERE t.list = 'popular' AND EXISTS (SELECT 1 FROM trakt_lists u WHERE u.list = 'trending')`,
		"trakt movies and shows":                 `SELECT count(*) FROM trakt_lists t WHERE t.media = 'show' AND EXISTS (SELECT 1 FROM trakt_lists u WHERE u.media = 'movie')`,
		"trakt rows matching library titles":     `SELECT count(*) FROM trakt_lists t JOIN items i ON i.tmdb_id = t.tmdb_id AND i.kind = CASE t.media WHEN 'movie' THEN 'movie' ELSE 'series' END`,
		"notification for a new episode":         `SELECT count(*) FROM notifications n JOIN items i ON i.id = n.item_id WHERE n.kind = 'new_episode' AND i.kind = 'podcast_episode' AND n.parent_id IS NOT NULL`,
		"item rows in the search index":          `SELECT count(*) FROM search_index WHERE kind = 'item'`,
		"genres per language":                    `SELECT count(*) FROM item_genres WHERE lang = 'de' AND EXISTS (SELECT 1 FROM item_genres WHERE lang = 'en')`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M7 state has no %s", what)
		}
	}
}
