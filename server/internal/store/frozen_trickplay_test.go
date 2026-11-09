package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 18 part A (migration 00042; testdata/db/trickplay.sql) carries
// the trickplay settings of the libraries: a library away from every default, one with films off and
// the series look-ahead «next episode», and one at the defaults. TestFrozenStatesMigrate replays it
// like every state; this test checks that each of those is really in it.
func TestFrozenTrickplayStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "trickplay.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"films on start without keeping":        `SELECT count(*) FROM libraries WHERE trickplay_movies = 'on_start' AND trickplay_movies_keep = 0`,
		"films off":                             `SELECT count(*) FROM libraries WHERE trickplay_movies = 'off'`,
		"series look-ahead over the season":     `SELECT count(*) FROM libraries WHERE trickplay_series = 'series_start' AND trickplay_series_scope = 'season_rest'`,
		"series look-ahead to the next episode": `SELECT count(*) FROM libraries WHERE trickplay_series_scope = 'next_episode'`,
		"a library at the defaults":             `SELECT count(*) FROM libraries WHERE trickplay_movies = 'ahead' AND trickplay_series = 'ahead' AND trickplay_series_scope = 'series'`,
		"the part B user picks":                 `SELECT count(*) FROM user_picks`,
		"migration 00042 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 42 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen trickplay state has no %s", what)
		}
	}
}
