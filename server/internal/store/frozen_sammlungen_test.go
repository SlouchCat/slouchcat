package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after migration 00060 (testdata/db/sammlungen.sql) carries the descriptions of
// concept 21 stage 6 (O-18) as the job people.details leaves them: one collection with a German
// description asked in German, one with the English fallback asked in German, and the older
// overview of the match beside them. TestFrozenStatesMigrate replays it like every state; this
// test checks that each of those is really in it.
func TestFrozenSammlungenStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "sammlungen.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a description in the main language": `SELECT count(*) FROM collections WHERE description IS NOT NULL AND description_lang = details_lang
			AND details_fetched_at IS NOT NULL`,
		"an English fallback": `SELECT count(*) FROM collections WHERE description IS NOT NULL AND description_lang = 'en'
			AND details_lang <> 'en'`,
		"the older overview beside a description": `SELECT count(*) FROM collections WHERE overview IS NOT NULL AND description IS NOT NULL`,
		"a collection with members":               `SELECT count(*) FROM collection_items`,
		"migration 00060 recorded as applied":     `SELECT count(*) FROM goose_db_version WHERE version_id = 60 AND is_applied = 1`,
		"foreign keys of the frozen state hold":   `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen sammlungen state has no %s", what)
		}
	}
}
