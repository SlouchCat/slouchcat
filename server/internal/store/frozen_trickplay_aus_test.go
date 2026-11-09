package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after the patch 1.23.1 (migration 00043; testdata/db/trickplay-aus.sql) carries
// the audiobook library switched from «Im Voraus» to «Aus» by the migration, films set back to «Im
// Voraus» by hand afterwards, and the other settings of trickplay.sql. TestFrozenStatesMigrate
// replays it like every state; this test checks that each of those is really in it.
func TestFrozenTrickplayAusStateCoversThePatch(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "trickplay-aus.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a library switched to off by 00043":    `SELECT count(*) FROM libraries WHERE trickplay_movies = 'off' AND trickplay_series = 'off' AND trickplay_series_scope = 'series'`,
		"films ahead set after the switch":      `SELECT count(*) FROM libraries WHERE kind = 'video' AND trickplay_movies = 'ahead'`,
		"series look-ahead over the season":     `SELECT count(*) FROM libraries WHERE trickplay_series = 'series_start' AND trickplay_series_scope = 'season_rest'`,
		"series on start without keeping":       `SELECT count(*) FROM libraries WHERE trickplay_series = 'on_start' AND trickplay_series_keep = 0`,
		"migration 00043 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 43 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen trickplay-aus state has no %s", what)
		}
	}
}
