package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 20 stage 2 (migration 00053, testdata/db/gesehen.sql) carries the
// progress the real playback service wrote on the state of radio-wikidata.sql: a film finished by
// the 90 % rule, a film finished and started again (completed 0, watched_at kept), season 1 of a
// series marked by hand with staggered times (the specials not, so the series is half watched),
// a check mark set and removed again, and a whole podcast and a film marked for the second
// profile. TestFrozenStatesMigrate replays it like every state; this test checks that each of
// those is really in it.
func TestFrozenGesehenStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "gesehen.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a finished film":                       `SELECT count(*) FROM progress AS p JOIN items AS i ON i.id = p.item_id WHERE i.kind = 'movie' AND p.completed = 1 AND p.watched_at = p.updated_at`,
		"a film in its second run":              `SELECT count(*) FROM progress WHERE completed = 0 AND position_ms > 0 AND watched_at IS NOT NULL AND watched_at < updated_at`,
		"a removed check mark":                  `SELECT count(*) FROM progress WHERE completed = 0 AND position_ms = 0 AND watched_at IS NULL`,
		"three staggered episodes":              `SELECT count(DISTINCT p.updated_at) = 3 AND max(p.updated_at) - min(p.updated_at) = 2 FROM progress AS p JOIN items AS i ON i.id = p.item_id WHERE i.kind = 'episode' AND p.watched_at IS NOT NULL`,
		"a series with an unwatched special":    `SELECT count(*) FROM items AS e WHERE e.kind = 'episode' AND e.season_number = 0 AND NOT EXISTS (SELECT 1 FROM progress AS p WHERE p.item_id = e.id)`,
		"watched podcast episodes of profile 2": `SELECT count(*) FROM progress AS p JOIN items AS i ON i.id = p.item_id WHERE p.profile_id = 2 AND i.kind = 'podcast_episode' AND p.watched_at IS NOT NULL`,
		"the partial index":                     `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'progress_watched'`,
		"migration 00053 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 53 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen gesehen state has no %s", what)
		}
	}
}
