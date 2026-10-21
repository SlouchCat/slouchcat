package store

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/slouchcat/slouchcat/server/migrations"
)

func testOptions(t *testing.T, dir string) Options {
	t.Helper()
	return Options{ConfigDir: dir, Version: "0.1.0+test/1", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// withSecondMigration returns the embedded migrations plus a test-only migration 99999.
func withSecondMigration(t *testing.T) fs.FS {
	t.Helper()
	files := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = &fstest.MapFile{Data: data}
	}
	files["99999_test_only.sql"] = &fstest.MapFile{Data: []byte(`-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
CREATE TABLE test_only (id INTEGER PRIMARY KEY) STRICT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
`)}
	return files
}

func backups(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, BackupDir, preMigrationPrefix+"*"+backupSuffix))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestFreshDatabaseHasNoBackup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "odd name #1 %20 ?")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatalf("database file not created in the config dir: %v", err)
	}
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("fresh database must not be backed up, found %v", got)
	}
	var mode string
	if err := db.Read.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v; want wal", mode, err)
	}
	var fk int
	if err := db.Write.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d, %v; want 1", fk, err)
	}
	if _, err := db.Read.Exec("INSERT INTO job_schedules (type, cron_expr, next_run) VALUES ('x', '* * * * *', 0)"); err == nil {
		t.Fatal("reader pool must be query-only")
	}
}

func TestBackupBeforePendingMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO jobs (type, run_after, created_at, updated_at, payload)
		VALUES ('backup.marker', 0, 0, 0, '{"row":"kept"}')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopening without pending migrations must not back up.
	db, err = Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("no pending migration, but found backups %v", got)
	}

	opts := testOptions(t, dir)
	opts.migrations = withSecondMigration(t)
	opts.now = func() time.Time { return time.Date(2026, 10, 18, 17, 30, 0, 0, time.UTC) }
	db, err = Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Read.Exec("SELECT count(*) FROM test_only"); err != nil {
		t.Fatalf("second migration not applied: %v", err)
	}

	got := backups(t, dir)
	if len(got) != 1 {
		t.Fatalf("want exactly one backup, found %v", got)
	}
	wantName := "pre-migration-0.1.0_test_1-20261018T173000.000Z.db"
	if filepath.Base(got[0]) != wantName {
		t.Fatalf("backup name = %s, want %s", filepath.Base(got[0]), wantName)
	}
	t.Logf("backup written: %s", got[0])

	bak, err := sql.Open("sqlite", "file:"+got[0]+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer bak.Close()
	var payload string
	if err := bak.QueryRow("SELECT payload FROM jobs WHERE type = 'backup.marker'").Scan(&payload); err != nil {
		t.Fatalf("backup does not contain the row: %v", err)
	}
	if payload != `{"row":"kept"}` {
		t.Fatalf("payload = %s", payload)
	}
	if err := bak.QueryRow("SELECT name FROM sqlite_master WHERE name = 'test_only'").Scan(new(string)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("backup must hold the state before the migration, got err = %v", err)
	}
	t.Logf("backup contains row with payload %s and no table test_only", payload)
}

// Pre-migration backups of releases before M8 (slouchcat-*.db) and of now (pre-migration-*.db)
// are pruned together; manual archives and pre-restore backups are not touched.
func TestPruneBackupsKeepsNewestFive(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"slouchcat-0.1.0-20261001T100000.000Z.db",
		"slouchcat-dev-20261002T100000.000Z.db",
		"slouchcat-0.1.0-20261003T100000.000Z.db",
		"pre-migration-0.2.0-rc-1-20261004T100000.000Z.db",
		"pre-migration-0.2.0-20261005T100000.000Z.db",
		"pre-migration-0.2.0-20261006T100000.000Z.db",
		"unrelated.db",
		"slouchcat-backup-0.1.0-20260901T100000Z.tar.gz",
		"pre-restore-0.1.0-20260901T100000.000Z.sqlite",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneBackups(dir, KeepBackups, isPreMigrationBackup); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if len(left) != 8 || strings.Contains(strings.Join(left, " "), "20261001T") {
		t.Fatalf("want the five newest backups plus the three other files, got %v", left)
	}
}

func TestMigrationFailsOnForeignKeyViolation(t *testing.T) {
	files := fstest.MapFS{"00001_broken.sql": &fstest.MapFile{Data: []byte(`-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
CREATE TABLE parent (id INTEGER PRIMARY KEY) STRICT;
CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parent (id)) STRICT;
INSERT INTO child (id, parent_id) VALUES (1, 42);
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
`)}}
	opts := testOptions(t, t.TempDir())
	opts.migrations = files
	_, err := Open(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "foreign key violations") {
		t.Fatalf("want a foreign key violation error, got %v", err)
	}
}
