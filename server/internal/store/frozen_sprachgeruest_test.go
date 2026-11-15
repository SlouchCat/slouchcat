package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state before migration 00061 (testdata/db/sprachgeruest.sql) is sammlungen.sql (after
// 00060) with a third profile «Kind» that uses every column of profiles: PIN, offline PIN, limits,
// accent, its own metadata language, playback languages and one library, with progress, watchlist,
// two downloads, a download rule, a device and an activity event. TestFrozenStatesMigrate replays
// it like every state; this test checks that each of those is really in it.
func TestFrozenSprachgeruestStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "sprachgeruest.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"profile with PIN, offline PIN and limits": `SELECT count(*) FROM profiles WHERE pin_hash IS NOT NULL AND offline_pin IS NOT NULL AND watch_limit_minutes IS NOT NULL AND max_height IS NOT NULL`,
		"profile in English":                       `SELECT count(*) FROM profiles WHERE language = 'en'`,
		"profile with its own metadata language":   `SELECT count(*) FROM profiles WHERE metadata_language IS NOT NULL`,
		"profile with its own libraries":           `SELECT count(*) FROM profile_libraries`,
		"progress":                                 `SELECT count(*) FROM progress`,
		"watchlist entry":                          `SELECT count(*) FROM watchlist`,
		"download of a profile":                    `SELECT count(*) FROM downloads WHERE profile_id IS NOT NULL`,
		"download rule":                            `SELECT count(*) FROM download_rules`,
		"activity event of a profile":              `SELECT count(*) FROM activity_events WHERE profile_id IS NOT NULL`,
		"migration 00060 recorded as applied":      `SELECT count(*) FROM goose_db_version WHERE version_id = 60 AND is_applied = 1`,
		"foreign keys of the frozen state hold":    `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen sprachgeruest state has no %s", what)
		}
	}
}
