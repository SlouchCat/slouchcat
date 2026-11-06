package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 15 stage 2, step 1 (migration 00034; testdata/db/feinschliff.sql)
// carries the data the step adds: a time window on probe.keyframes, one across midnight on
// segments.detect, chapters.images without one, a playback job with both overrides, a job after a
// playback with override_window alone and a plain waiting job. TestFrozenStatesMigrate replays it
// like every state; this test checks that each of those is really in it and that the constraints of
// the migration hold.
func TestFrozenFeinschliffStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "feinschliff.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"window within a day":                   `SELECT count(*) FROM job_types WHERE type = 'probe.keyframes' AND window_start = 60 AND window_end = 360`,
		"window across midnight":                `SELECT count(*) FROM job_types WHERE type = 'segments.detect' AND window_start > window_end`,
		"windowable type without a window":      `SELECT count(*) FROM job_types WHERE type = 'chapters.images' AND window_start IS NULL AND window_end IS NULL`,
		"playback job with both overrides":      `SELECT count(*) FROM jobs WHERE type = 'probe.keyframes' AND state = 'queued' AND override_pause = 1 AND override_window = 1`,
		"job after a playback, window only":     `SELECT count(*) FROM jobs WHERE type = 'chapters.images' AND state = 'queued' AND override_pause = 0 AND override_window = 1`,
		"plain waiting job of a windowed type":  `SELECT count(*) FROM jobs WHERE type = 'segments.detect' AND state = 'queued' AND override_window = 0`,
		"finished jobs without override_window": `SELECT count(*) FROM jobs WHERE state = 'done' AND override_window = 0`,
		"migration 00034 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 34 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen feinschliff state has no %s", what)
		}
	}
	assertWindowConstraints(t, db)
}

// assertWindowConstraints writes what migration 00034 forbids: half a window, an empty one, minutes
// outside the day, and override_window other than 0 and 1.
func assertWindowConstraints(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	for _, bad := range []string{
		`UPDATE job_types SET window_start = 60, window_end = NULL WHERE type = 'scan.path'`,
		`UPDATE job_types SET window_start = NULL, window_end = 60 WHERE type = 'scan.path'`,
		`UPDATE job_types SET window_start = 60, window_end = 60 WHERE type = 'scan.path'`,
		`UPDATE job_types SET window_start = 1440, window_end = 60 WHERE type = 'scan.path'`,
		`UPDATE job_types SET window_start = 60, window_end = 1440 WHERE type = 'scan.path'`,
		`UPDATE job_types SET window_start = -1, window_end = 60 WHERE type = 'scan.path'`,
		`UPDATE jobs SET override_window = 2`,
		`UPDATE jobs SET override_window = NULL`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: accepted, want a constraint error", bad)
		}
	}
	for _, good := range []string{
		`UPDATE job_types SET window_start = 1380, window_end = 0 WHERE type = 'scan.path'`,
		`UPDATE job_types SET window_start = NULL, window_end = NULL WHERE type = 'scan.path'`,
	} {
		if _, err := db.Write.ExecContext(ctx, good); err != nil {
			t.Errorf("%s: %v", good, err)
		}
	}
}

// Migration 00034 adds the window columns empty and override_window as 0 to every row of the frozen
// state of migration 00033 and keeps every job, type and pause override as it was.
func TestMigration00034JobWindows(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "wiedergabestart.sql"))
	raw := openRaw(t, path)
	var jobsBefore, typesBefore, pausedBefore int
	if err := raw.QueryRow(`SELECT (SELECT count(*) FROM jobs), (SELECT count(*) FROM job_types),
		(SELECT count(*) FROM jobs WHERE override_pause = 1)`).Scan(&jobsBefore, &typesBefore, &pausedBefore); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var jobs, types, paused, windows, overrides int
	if err := db.Read.QueryRow(`SELECT (SELECT count(*) FROM jobs), (SELECT count(*) FROM job_types),
		(SELECT count(*) FROM jobs WHERE override_pause = 1),
		(SELECT count(*) FROM job_types WHERE window_start IS NOT NULL OR window_end IS NOT NULL),
		(SELECT count(*) FROM jobs WHERE override_window <> 0)`).Scan(&jobs, &types, &paused, &windows, &overrides); err != nil {
		t.Fatal(err)
	}
	if jobs != jobsBefore || types != typesBefore || paused != pausedBefore || windows != 0 || overrides != 0 {
		t.Errorf("jobs %d (was %d), types %d (was %d), override_pause %d (was %d), windows %d, override_window %d; want nothing lost and defaults",
			jobs, jobsBefore, types, typesBefore, paused, pausedBefore, windows, overrides)
	}
	assertWindowConstraints(t, db)
}
