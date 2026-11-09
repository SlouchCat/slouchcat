package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 18 part B (migration 00041; testdata/db/empfehlungen.sql) carries
// the user picks of the part: one film picked by both profiles, a series picked by one, an old
// pick past the retention and a retention other than the default. TestFrozenStatesMigrate replays
// it like every state; this test checks that each of those is really in it.
func TestFrozenEmpfehlungenStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "empfehlungen.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a film picked by two profiles": `SELECT count(*) FROM (SELECT u.item_id FROM user_picks u JOIN items i ON i.id = u.item_id
			WHERE i.kind = 'movie' GROUP BY u.item_id HAVING count(DISTINCT u.profile_id) = 2)`,
		"a picked series":                       `SELECT count(*) FROM user_picks u JOIN items i ON i.id = u.item_id WHERE i.kind = 'series'`,
		"a pick past the retention":             `SELECT count(*) FROM user_picks u, picks_config c WHERE u.created_at <= c.updated_at - c.retention_days * 86400000`,
		"a retention other than 60 days":        `SELECT count(*) FROM picks_config WHERE id = 1 AND retention_days <> 60`,
		"no table of the curated picks":         `SELECT count(*) = 0 FROM sqlite_master WHERE name LIKE 'top_picks%'`,
		"the stage 3 images of concept 17":      `SELECT count(*) FROM person_images WHERE status = 'ok'`,
		"migration 00041 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 41 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen empfehlungen state has no %s", what)
		}
	}
}
