package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 20 stage 3 (migrations 00053 and 00054; testdata/db/merkliste.sql)
// carries the watchlist that took over from the favourite: an entry that was on the watchlist and a
// favourite (follow_since from the favourite, created_at from the watchlist), a favourite alone
// (both times the favourite's), an entry without favourite (follow_since at the moment of the
// migration) and an entry added by the service afterwards (follow_since = created_at), next to
// the check marks of gesehen.sql. TestFrozenStatesMigrate replays it like every state; this test
// checks that each of those is really in it.
func TestFrozenMerklisteStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "merkliste.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a series on the watchlist, followed since its favourite":  `SELECT count(*) FROM watchlist w JOIN items i ON i.id = w.item_id WHERE i.kind = 'series' AND w.follow_since > w.created_at`,
		"a podcast taken over from a favourite alone":              `SELECT count(*) FROM watchlist w JOIN items i ON i.id = w.item_id WHERE i.kind = 'podcast' AND w.follow_since = w.created_at`,
		"an entry without favourite, followed since the migration": `SELECT count(*) FROM watchlist w JOIN items i ON i.id = w.item_id WHERE i.kind = 'movie' AND w.follow_since > w.created_at`,
		"an entry added after the migration":                       `SELECT count(*) FROM watchlist w JOIN items i ON i.id = w.item_id WHERE i.kind = 'movie' AND w.follow_since = w.created_at`,
		"no table favorites":                                       `SELECT count(*) = 0 FROM sqlite_master WHERE name = 'favorites'`,
		"check marks of stage 2":                                   `SELECT count(*) FROM progress WHERE watched_at IS NOT NULL`,
		"migrations 00053 and 00054 recorded as applied":           `SELECT count(*) = 2 FROM goose_db_version WHERE version_id IN (53, 54) AND is_applied = 1`,
		"foreign keys of the frozen state hold":                    `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen merkliste state has no %s", what)
		}
	}
}
