package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 20 stage 5 (migration 00055, testdata/db/loeschen.sql) carries
// what the stage added on the state of gesehen.sql: the second profile restricted to two libraries
// through profile_libraries (now with its foreign key to libraries), a film and a whole series
// taken out of Slouchcat (removed_at with deleted_at, their files kept) and the new indexes.
// TestFrozenStatesMigrate replays it like every state; this test checks that each of those is
// really in it.
func TestFrozenLoeschenStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "loeschen.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"grants of a restricted profile":        `SELECT count(*) = 2 FROM profile_libraries AS pl JOIN profiles AS p ON p.id = pl.profile_id WHERE p.all_libraries = 0`,
		"the foreign key of the grants":         `SELECT count(*) FROM pragma_foreign_key_list('profile_libraries') WHERE "table" = 'libraries' AND on_delete = 'CASCADE'`,
		"a removed film with its file":          `SELECT count(*) FROM items AS i WHERE i.kind = 'movie' AND i.removed_at IS NOT NULL AND i.deleted_at IS NOT NULL AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id)`,
		"a removed series with every episode":   `SELECT count(*) FROM items AS s WHERE s.kind = 'series' AND s.removed_at IS NOT NULL AND NOT EXISTS (SELECT 1 FROM items AS e JOIN items AS se ON se.id = e.parent_id WHERE se.parent_id = s.id AND e.removed_at IS NULL)`,
		"the index of the removed titles":       `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'items_removed'`,
		"the index of the progress events":      `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'progress_events_item'`,
		"migration 00055 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 55 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen loeschen state has no %s", what)
		}
	}
}
