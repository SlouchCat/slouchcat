package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen M11 state carries every kind of M11 data (testdata/db/README.md): the spotlight
// configuration away from its defaults, the fetched list of a run against the Trakt mock, the
// three forms of a curation (pinned with a note, hidden, note only) and the eight start page keys
// of migration 00021 with a changed order. A later migration that touches the spotlight has to
// migrate all of it.
func TestFrozenM11StateCoversM11Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M11.sql"))
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
		"configuration away from the migration's defaults": `SELECT count(*) FROM spotlight_config WHERE id = 1 AND item_count <> 20 AND min_hits <> 5 AND interval_hours <> 24 AND last_manual_refresh IS NOT NULL`,
		"fetched entries of both media":                    `SELECT min(n) FROM (SELECT count(*) AS n FROM spotlight_entries GROUP BY media)`,
		"entry with a library hit":                         `SELECT count(*) FROM spotlight_entries e JOIN items i ON i.tmdb_id = e.tmdb_id WHERE e.media = 'movie' AND i.kind = 'movie'`,
		"entry without a library hit":                      `SELECT count(*) FROM spotlight_entries e WHERE NOT EXISTS (SELECT 1 FROM items i WHERE i.tmdb_id = e.tmdb_id)`,
		"pinned override with a note":                      `SELECT count(*) FROM spotlight_overrides WHERE pinned_rank = 1 AND note IS NOT NULL`,
		"hidden override":                                  `SELECT count(*) FROM spotlight_overrides WHERE hidden = 1`,
		"note-only override":                               `SELECT count(*) FROM spotlight_overrides WHERE hidden = 0 AND pinned_rank IS NULL AND note IS NOT NULL`,
		"hidden start page row":                            `SELECT count(*) FROM home_rows WHERE hidden = 1`,
		"scanned film library of the base state":           `SELECT count(*) FROM items WHERE kind = 'movie'`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M11 state has no %s", what)
		}
	}
}
