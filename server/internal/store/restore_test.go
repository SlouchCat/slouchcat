package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// newDatabaseFile creates a migrated database in its own directory with a marker setting and
// returns its path (closed, no WAL left).
func newDatabaseFile(t *testing.T, marker string) string {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(context.Background(), testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec("INSERT INTO settings (key, value, updated_at) VALUES ('marker', ?, 1)", marker); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, FileName)
}

func marker(t *testing.T, db *sql.DB) string {
	t.Helper()
	var v string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'marker'").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// A staged restore with a key replaces database and key; the current ones are kept as the newest
// pre-restore backup (older ones beyond five are pruned with their keys), the restore directory
// is removed.
func TestApplyRestoreSwapsDatabaseAndKey(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	live := newDatabaseFile(t, "live")
	if err := os.Rename(live, filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}
	oldKey, newKey := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	writeFile(t, filepath.Join(dir, "secret.key"), oldKey)
	backups := filepath.Join(dir, BackupDir)
	if err := os.MkdirAll(filepath.Join(dir, RestoreDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backups, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, stamp := range []string{"20261001", "20261002", "20261003", "20261004", "20261005"} {
		name := "pre-restore-0.8.0-" + stamp + "T100000.000Z.sqlite"
		writeFile(t, filepath.Join(backups, name), []byte("old"))
		writeFile(t, filepath.Join(backups, BackupKeyFile(name)), oldKey)
	}
	// The oldest was moved instead of copied (I-4): its WAL goes with it.
	writeFile(t, filepath.Join(backups, "pre-restore-0.8.0-20261001T100000.000Z.sqlite-wal"), []byte("wal"))
	// Staged the way package admin does it: unpacked in restore/, then StageRestore.
	staged := filepath.Join(dir, RestoreDir, FileName)
	if err := os.Rename(newDatabaseFile(t, "restored"), staged); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, RestoreDir, "secret.key"), newKey)
	if err := StageRestore(dir, staged, filepath.Join(dir, RestoreDir, "secret.key")); err != nil {
		t.Fatal(err)
	}

	opts := testOptions(t, dir)
	opts.now = func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) }
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := marker(t, db.Read); got != "restored" {
		t.Errorf("marker = %q, want the restored database", got)
	}
	if !bytes.Equal(readFile(t, filepath.Join(dir, "secret.key")), newKey) {
		t.Error("the restored key is not in place")
	}
	for _, p := range []string{RestoreDBFile, RestoreKeyFile, RestoreDir} {
		if !missing(filepath.Join(dir, p)) {
			t.Errorf("%s left after the restore", p)
		}
	}
	kept := filepath.Join(backups, "pre-restore-0.1.0_test_1-20261021T120000.000Z.sqlite")
	old, err := OpenFile(ctx, kept)
	if err != nil {
		t.Fatalf("pre-restore backup: %v", err)
	}
	defer old.Close()
	if got := marker(t, old); got != "live" {
		t.Errorf("pre-restore backup marker = %q, want live", got)
	}
	if !bytes.Equal(readFile(t, filepath.Join(backups, BackupKeyFile(filepath.Base(kept)))), oldKey) {
		t.Error("the replaced key is not kept next to the pre-restore backup")
	}
	entries, err := os.ReadDir(backups)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 10 || slices.Contains(names, "pre-restore-0.8.0-20261001T100000.000Z.sqlite") ||
		slices.Contains(names, "pre-restore-0.8.0-20261001T100000.000Z.secret.key") {
		t.Errorf("backups %v, want the five newest pre-restore copies with their keys", names)
	}
}

// Without a staged key the current key stays and no key is kept with the pre-restore backup.
func TestApplyRestoreWithoutKeyKeepsKey(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.Rename(newDatabaseFile(t, "live"), filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{3}, 32)
	writeFile(t, filepath.Join(dir, "secret.key"), key)
	if err := os.Rename(newDatabaseFile(t, "restored"), filepath.Join(dir, RestoreDBFile)); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := marker(t, db.Read); got != "restored" {
		t.Errorf("marker = %q", got)
	}
	if !bytes.Equal(readFile(t, filepath.Join(dir, "secret.key")), key) {
		t.Error("the key changed without a staged key")
	}
	keys, _ := filepath.Glob(filepath.Join(dir, BackupDir, "*.secret.key"))
	copies, _ := filepath.Glob(filepath.Join(dir, BackupDir, "pre-restore-*.sqlite"))
	if len(keys) != 0 || len(copies) != 1 {
		t.Errorf("backups: copies %v, keys %v; want one copy without a key", copies, keys)
	}
}

// A staged key without a staged database is left over from an interrupted staging: it is removed
// and nothing else changes.
func TestApplyRestoreRemovesStaleKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.Rename(newDatabaseFile(t, "live"), filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{4}, 32)
	writeFile(t, filepath.Join(dir, "secret.key"), key)
	writeFile(t, filepath.Join(dir, RestoreKeyFile), bytes.Repeat([]byte{5}, 32))
	db, err := Open(context.Background(), testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if marker(t, db.Read) != "live" || !bytes.Equal(readFile(t, filepath.Join(dir, "secret.key")), key) {
		t.Error("database or key changed")
	}
	if !missing(filepath.Join(dir, RestoreKeyFile)) || !missing(filepath.Join(dir, BackupDir)) {
		t.Error("stale key left or a backup written")
	}
}

// A start interrupted after the key was swapped and the live database removed finishes the swap.
func TestApplyRestoreResumesInterruptedSwap(t *testing.T) {
	dir := t.TempDir()
	key := bytes.Repeat([]byte{6}, 32)
	writeFile(t, filepath.Join(dir, "secret.key"), key) // already the restored key
	if err := os.Rename(newDatabaseFile(t, "restored"), filepath.Join(dir, RestoreDBFile)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, FileName+"-wal"), []byte("stale wal of the removed database"))
	db, err := Open(context.Background(), testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if marker(t, db.Read) != "restored" || !bytes.Equal(readFile(t, filepath.Join(dir, "secret.key")), key) {
		t.Error("swap not finished")
	}
	if !missing(filepath.Join(dir, BackupDir)) {
		t.Error("no live database, so no pre-restore backup")
	}
}

// Review I-4: when the pre-restore copy cannot be written (a damaged live database, or a target the
// copy cannot write, here a file already in its place), the start does not fail: the live files
// are moved into backups/ as they are, an error is logged and the restore is applied. The key file
// goes next to them as usual. (SQLite itself removes a WAL it cannot use while the copy is tried;
// TestMoveDatabase covers the WAL.)
func TestApplyRestoreMovesLiveFilesWhenTheCopyFails(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	keptName := "pre-restore-0.1.0_test_1-20261021T120000.000Z.sqlite"
	damaged := append([]byte("SQLite format 3\x00"), bytes.Repeat([]byte{0xee}, 8192)...)
	for name, tc := range map[string]struct {
		live    func(t *testing.T, dir string) // writes the live database (and its WAL)
		blocked bool                           // a file takes the copy's place
	}{
		"damaged live database": {live: func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, FileName), damaged)
		}},
		"copy target not writable": {live: func(t *testing.T, dir string) {
			if err := os.Rename(newDatabaseFile(t, "live"), filepath.Join(dir, FileName)); err != nil {
				t.Fatal(err)
			}
		}, blocked: true},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			tc.live(t, dir)
			oldKey, newKey := bytes.Repeat([]byte{7}, 32), bytes.Repeat([]byte{8}, 32)
			writeFile(t, filepath.Join(dir, "secret.key"), oldKey)
			backups := filepath.Join(dir, BackupDir)
			if err := os.MkdirAll(backups, 0o750); err != nil {
				t.Fatal(err)
			}
			if tc.blocked {
				writeFile(t, filepath.Join(backups, keptName), []byte("in the way"))
			}
			if err := os.Rename(newDatabaseFile(t, "restored"), filepath.Join(dir, RestoreDBFile)); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(dir, RestoreKeyFile), newKey)
			var logs bytes.Buffer
			opts := testOptions(t, dir)
			opts.Logger = slog.New(slog.NewTextHandler(&logs, nil))
			opts.now = func() time.Time { return at }

			db, err := Open(ctx, opts)
			if err != nil {
				t.Fatalf("start with a failing pre-restore copy: %v", err)
			}
			defer db.Close()
			if got := marker(t, db.Read); got != "restored" {
				t.Errorf("marker = %q, want the restored database", got)
			}
			if !strings.Contains(logs.String(), "level=ERROR") || !strings.Contains(logs.String(), "pre-restore copy failed") {
				t.Errorf("no error logged:\n%s", logs.String())
			}
			kept := filepath.Join(backups, keptName)
			if tc.blocked {
				old, err := OpenFile(ctx, kept)
				if err != nil {
					t.Fatalf("moved database: %v", err)
				}
				defer old.Close()
				if got := marker(t, old); got != "live" {
					t.Errorf("moved database marker = %q, want live", got)
				}
			} else if !bytes.Equal(readFile(t, kept), damaged) {
				t.Error("the damaged database is not kept as it was")
			}
			if !bytes.Equal(readFile(t, filepath.Join(backups, BackupKeyFile(keptName))), oldKey) ||
				!bytes.Equal(readFile(t, filepath.Join(dir, "secret.key")), newKey) {
				t.Error("keys not swapped")
			}
		})
	}
}

// moveDatabase takes the WAL and the index along when they exist; the database file must exist.
func TestMoveDatabase(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "live.db"), filepath.Join(dir, "kept.sqlite")
	writeFile(t, src, []byte("db"))
	writeFile(t, src+"-wal", []byte("wal"))
	if err := moveDatabase(src, dst); err != nil {
		t.Fatal(err)
	}
	if string(readFile(t, dst)) != "db" || string(readFile(t, dst+"-wal")) != "wal" || !missing(dst+"-shm") {
		t.Error("database and WAL not moved together")
	}
	if !missing(src) || !missing(src+"-wal") {
		t.Error("source files left")
	}
	if err := moveDatabase(src, dst); err == nil {
		t.Error("moving a missing database succeeded")
	}
}

func TestCheckRestorable(t *testing.T) {
	ctx := context.Background()
	good := newDatabaseFile(t, "x")
	if err := CheckRestorable(ctx, good); err != nil {
		t.Fatalf("intact database: %v", err)
	}

	newer := newDatabaseFile(t, "x")
	db, err := OpenFile(ctx, newer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO goose_db_version (version_id, is_applied) VALUES (99999, 1)"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	violated := newDatabaseFile(t, "x")
	db, err = OpenFile(ctx, violated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO profiles (account_id, name, created_at, updated_at) VALUES (42, 'Waise', 1, 1)"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	garbage := filepath.Join(t.TempDir(), "garbage.db")
	writeFile(t, garbage, append([]byte("SQLite format 3\x00"), bytes.Repeat([]byte{0xee}, 8192)...))
	foreign := filepath.Join(t.TempDir(), "foreign.db")
	writeFile(t, foreign, nil)
	db, err = OpenFile(ctx, foreign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE notes (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	for name, tc := range map[string]struct {
		path string
		want error
	}{
		"newer schema":    {newer, ErrSchemaNewer},
		"foreign key":     {violated, ErrRestoreInvalid},
		"damaged":         {garbage, ErrRestoreInvalid},
		"not slouchcat":   {foreign, ErrRestoreInvalid},
		"no file at path": {filepath.Join(t.TempDir(), "none.db"), ErrRestoreInvalid},
	} {
		if err := CheckRestorable(ctx, tc.path); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

// Snapshot reads through its own read-only connection: it neither waits for an open write
// transaction nor sees its rows.
func TestSnapshotDoesNotWaitForWriters(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Write.Exec("INSERT INTO settings (key, value, updated_at) VALUES ('committed', '1', 1)"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Write.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("INSERT INTO settings (key, value, updated_at) VALUES ('uncommitted', '1', 1)"); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "copy.db")
	snapCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := Snapshot(snapCtx, db.Read, dst); err != nil {
		t.Fatalf("snapshot while a write transaction is open: %v", err)
	}
	cp, err := OpenFile(ctx, dst)
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	var keys []string
	var key string
	if err := cp.QueryRow("SELECT group_concat(key) FROM settings").Scan(&key); err != nil {
		t.Fatal(err)
	}
	keys = strings.Split(key, ",")
	if !slices.Equal(keys, []string{"committed"}) {
		t.Errorf("copy has settings %v, want only the committed one", keys)
	}
	var mode string
	if err := cp.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "delete" {
		t.Errorf("journal mode of the copy = %q, %v; want delete (a single file)", mode, err)
	}
}
