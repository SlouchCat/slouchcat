package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 15 stage 3, P-7 (migration 00035; testdata/db/startseitenreihen.sql)
// carries the data of the step: «Meine Medien» shown in second place, the row of library 2 shown,
// the row of library 1 hidden, and library 3 created after the migration with the row of the
// trigger, hidden at the end. TestFrozenStatesMigrate replays it like every state; this test
// checks that each of those is really in it.
func TestFrozenStartseitenreihenStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "startseitenreihen.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"«Meine Medien» shown second":           `SELECT count(*) FROM home_rows WHERE id = 'libraries' AND position = 1 AND hidden = 0`,
		"a shown library row":                   `SELECT count(*) FROM home_rows WHERE id = 'library:2' AND key = 'library' AND library_id = 2 AND hidden = 0`,
		"a hidden library row":                  `SELECT count(*) FROM home_rows WHERE id = 'library:1' AND hidden = 1`,
		"the trigger's row, hidden at the end":  `SELECT count(*) FROM home_rows WHERE id = 'library:3' AND hidden = 1 AND position = (SELECT max(position) FROM home_rows WHERE key <> 'radio') AND updated_at = (SELECT created_at FROM libraries WHERE id = 3)`,
		"a row for every library":               `SELECT count(*) = (SELECT count(*) FROM libraries) FROM home_rows WHERE key = 'library'`,
		"the trigger":                           `SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'home_rows_library_added'`,
		"migration 00035 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 35 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen startseitenreihen state has no %s", what)
		}
	}
}
