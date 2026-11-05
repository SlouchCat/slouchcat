package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen state after concept 11 step 2 (ADR 0019, migration 00031; testdata/db/verwaltung.sql)
// carries the activity log: at least one row of every known kind, the rows of a real server run
// (server_started with its version, scan_started with the trigger of the schedule, scan_done with
// newTitles) and the rows written by SQL (playback of the owner's profile with the title as
// subject, a stop by the administration, a failed and a cancelled scan with their fixed codes, an
// aggregated job_failed with its count, a backup with its size, a restart and a restore), the two
// indexes and the two delete triggers. TestFrozenStatesMigrate replays it like every state; this test checks that each of
// those is really in it and that no row carries a path.
func TestFrozenVerwaltungStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "verwaltung.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, kind := range []string{
		"playback_started", "playback_completed", "playback_stopped", "scan_started", "scan_done", "scan_failed",
		"scan_aborted", "job_failed", "backup_created", "backup_restored", "server_started", "server_restart",
	} {
		var n int
		if err := db.Read.QueryRow(`SELECT count(*) FROM activity_events WHERE kind = ?`, kind).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("frozen state has no %s row", kind)
		}
	}
	for what, query := range map[string]string{
		"server start with its version":         `SELECT count(*) FROM activity_events WHERE kind = 'server_started' AND json_extract(detail, '$.version') <> ''`,
		"scheduled full scan with its library":  `SELECT count(*) FROM activity_events WHERE kind = 'scan_started' AND json_extract(detail, '$.trigger') = 'schedule' AND library_id IN (SELECT id FROM libraries) AND subject IN (SELECT name FROM libraries)`,
		"finished scan with new titles":         `SELECT count(*) FROM activity_events WHERE kind = 'scan_done' AND json_type(detail, '$.newTitles') = 'integer'`,
		"playback of a profile and a title":     `SELECT count(*) FROM activity_events WHERE kind = 'playback_started' AND profile_id IN (SELECT id FROM profiles) AND account_id IN (SELECT id FROM accounts) AND subject = (SELECT title FROM items WHERE id = item_id)`,
		"stop by the administration":            `SELECT count(*) FROM activity_events WHERE kind = 'playback_stopped' AND json_extract(detail, '$.by') = 'admin'`,
		"failed and cancelled scan with a code": `SELECT count(*) FROM activity_events WHERE kind IN ('scan_failed', 'scan_aborted') AND json_extract(detail, '$.reason') IN ('error', 'cancelled')`,
		"aggregated job failure":                `SELECT count(*) FROM activity_events WHERE kind = 'job_failed' AND json_extract(detail, '$.count') > 1 AND json_extract(detail, '$.jobType') <> ''`,
		"backup with its size":                  `SELECT count(*) FROM activity_events WHERE kind = 'backup_created' AND json_extract(detail, '$.sizeBytes') > 0`,
		"index on the time":                     `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'activity_events_at'`,
		"index on the account":                  `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'activity_events_account'`,
		"trigger on deleted accounts":           `SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'activity_events_account_deleted'`,
		"trigger on deleted profiles":           `SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'activity_events_profile_deleted'`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen verwaltung state has no %s", what)
		}
	}
	var paths int
	if err := db.Read.QueryRow(`SELECT count(*) FROM activity_events WHERE detail LIKE '%/%' OR subject LIKE '%/%'`).Scan(&paths); err != nil || paths != 0 {
		t.Errorf("rows with a path: %d, %v", paths, err)
	}
}
