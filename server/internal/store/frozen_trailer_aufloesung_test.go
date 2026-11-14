package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after migration 00057 (testdata/db/trailer-aufloesung.sql) carries the
// trailers of concept 21 stage 2: an automatic TMDB trailer with its size, a YouTube hit and a
// manual trailer without one, and a title with manual trailers only. TestFrozenStatesMigrate
// replays it like every state; this test checks that each of those is really in it.
func TestFrozenTrailerAufloesungStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "trailer-aufloesung.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"TMDB trailer with a size":              `SELECT count(*) FROM trailers WHERE source = 'tmdb' AND size = 1080`,
		"YouTube trailer without a size":        `SELECT count(*) FROM trailers WHERE source = 'youtube' AND size IS NULL`,
		"manual trailer without a size":         `SELECT count(*) FROM trailers WHERE source = 'manual' AND size IS NULL`,
		"title with manual trailers only":       `SELECT count(*) FROM items AS i WHERE i.trailer_state = 'found' AND EXISTS (SELECT 1 FROM trailers AS t WHERE t.item_id = i.id) AND NOT EXISTS (SELECT 1 FROM trailers AS t WHERE t.item_id = i.id AND t.source <> 'manual')`,
		"migration 00057 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 57 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen trailer-aufloesung state has no %s", what)
		}
	}
}
