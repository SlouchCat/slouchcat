package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after migration 00058 (testdata/db/personenseite.sql) carries the people of
// concept 21 stage 4: one with a biography in the main language, one with the English fallback,
// one not fetched yet, and the portrait rows still in w185 that the job replaces with w300
// (O-12). TestFrozenStatesMigrate replays it like every state; this test checks that each of
// those is really in it.
func TestFrozenPersonenseiteStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "personenseite.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"biography in the main language": `SELECT count(*) FROM people WHERE biography IS NOT NULL AND biography_lang = details_lang
			AND birthday IS NOT NULL AND place_of_birth IS NOT NULL AND known_for_department IS NOT NULL AND total_credits > 0`,
		"English biography as fallback":         `SELECT count(*) FROM people WHERE biography_lang = 'en' AND details_lang = 'de'`,
		"person not fetched yet":                `SELECT count(*) FROM people WHERE details_fetched_at IS NULL`,
		"portrait still in w185":                `SELECT count(*) FROM person_images WHERE status = 'ok' AND size = 'w185'`,
		"migration 00058 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 58 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen personenseite state has no %s", what)
		}
	}
}
