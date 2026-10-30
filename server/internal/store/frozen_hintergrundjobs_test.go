package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen state after concept 09 step 2 (ADR 0017, migration 00027;
// testdata/db/hintergrundjobs.sql) carries the data the concept adds: a row per job type in
// job_types, the schedule rows with their next run, done runs with start and end, a run that was
// skipped (jobs.prune with nothing to prune), a running run with progress and a note, and the
// index jobs_history. TestFrozenStatesMigrate replays it like every state; this test checks that
// each of those is really in it, so a later migration is measured against the data the concept
// produced.
func TestFrozenHintergrundjobsStateCoversTheConcept(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "hintergrundjobs.sql"))
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
	for what, query := range map[string]string{
		"job type rows":                          `SELECT count(*) FROM job_types`,
		"schedule rows with a next run":          `SELECT count(*) FROM job_schedules WHERE next_run_at > 0`,
		"schedule of the full scan":              `SELECT count(*) FROM job_schedules WHERE type = 'scan.library' AND every_ms = 21600000`,
		"schedule that is switched off":          `SELECT count(*) FROM job_schedules WHERE enabled = 0`,
		"done run with start and end":            `SELECT count(*) FROM jobs WHERE state = 'done' AND started_at IS NOT NULL AND finished_at >= started_at`,
		"skipped run":                            `SELECT count(*) FROM jobs WHERE skipped = 1`,
		"running run with progress and a note":   `SELECT count(*) FROM jobs WHERE state = 'running' AND progress_total > 0 AND progress_note <> ''`,
		"scan run with a progress note":          `SELECT count(*) FROM jobs WHERE type = 'scan.library' AND progress_note LIKE '% · %'`,
		"index of the history":                   `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'jobs_history'`,
		"no absolute path in a progress note":    `SELECT count(*) = (SELECT count(*) FROM jobs WHERE progress_note IS NOT NULL) FROM jobs WHERE progress_note IS NOT NULL AND progress_note NOT LIKE '/%'`,
		"film and series library of the concept": `SELECT count(*) FROM library_roots WHERE content IN ('movie', 'series')`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen background jobs state has no %s", what)
		}
	}
}
