package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The frozen state after concept 08 (ADR 0016, migration 00026; testdata/db/startseite.sql)
// carries the data the concept adds: a spotlight rhythm other than the default, sort keys on the
// texts, a vote average on a matched title, the indexes of the library page and a pick with a
// note (the page "Cat's recommendations"; discarded by 00041, concept 18 part B). TestFrozenStatesMigrate replays it like every state;
// this test checks that each of those is really in it, so a later migration is measured against
// the data the concept produced.
func TestFrozenStartseiteStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "startseite.sql"))
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
		"spotlight rhythm set to 7 s":                 `SELECT count(*) FROM spotlight_config WHERE rotation_seconds = 7`,
		"text rows with a sort key":                   `SELECT count(*) FROM item_texts WHERE sort_key IS NOT NULL AND sort_key <> ''`,
		"German text whose sort key lost its article": `SELECT count(*) FROM item_texts WHERE lang = 'de' AND sort_key NOT LIKE 'der %' AND sort_key NOT LIKE 'die %' AND sort_key NOT LIKE 'das %'`,
		"matched title with a vote average":           `SELECT count(*) FROM items WHERE metadata_state = 'matched' AND vote_average IS NOT NULL`,
		"provenance row of the vote":                  `SELECT count(*) FROM item_field_sources WHERE field = 'vote_average'`,
		"index of the sort order added":               `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'items_library_kind_added'`,
		"index of the sort key":                       `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'item_texts_sort_key'`,
		"picks discarded by 00041, user picks empty":  `SELECT (SELECT count(*) FROM sqlite_master WHERE name LIKE 'top_picks%') = 0 AND (SELECT count(*) FROM user_picks) = 0`,
		"film and series library":                     `SELECT count(*) FROM library_roots WHERE content IN ('movie', 'series')`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen start page state has no %s", what)
		}
	}
	// The pick with a note is in the state; migration 00041 discards it (concept 18 part B, ADR 0028).
	if !regexp.MustCompile(`INSERT INTO "top_picks" VALUES \(\d+, '[0-9A-Z]{26}', NULL, \d+, '[^']+'`).Match(script) {
		t.Error("frozen start page state has no pick with a note")
	}
}
