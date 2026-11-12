package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after migration 00056 (testdata/db/loeschen-job.sql) carries a library that is
// being deleted: «Hörbücher» with deleting_at and its queued library.delete job, as a server leaves
// them when it stops in the middle of a delete. TestFrozenStatesMigrate replays it like every
// state; this test checks that each of those is really in it.
func TestFrozenLoeschenJobStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "loeschen-job.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a library being deleted":               `SELECT count(*) FROM libraries WHERE deleting_at IS NOT NULL`,
		"its queued delete job":                 `SELECT count(*) FROM jobs WHERE type = 'library.delete' AND state = 'queued' AND json_extract(payload, '$.libraryId') = (SELECT id FROM libraries WHERE deleting_at IS NOT NULL)`,
		"no visible title of it":                `SELECT count(*) = 0 FROM visible_items AS v JOIN items AS i ON i.id = v.item_id JOIN libraries AS l ON l.id = i.library_id WHERE l.deleting_at IS NOT NULL`,
		"the titles of the other libraries":     `SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id JOIN libraries AS l ON l.id = i.library_id WHERE l.deleting_at IS NULL`,
		"migration 00056 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 56 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen loeschen-job state has no %s", what)
		}
	}
}
