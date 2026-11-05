package storetest_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

func options(dir string) store.Options {
	return store.Options{ConfigDir: dir, Version: "test", Logger: slog.New(slog.DiscardHandler)}
}

// The copy of the template must be indistinguishable from a fresh store.Open: same schema and
// rows, same applied migrations, same pragmas on the writer and on the readers, no backup.
func TestOpenMatchesFreshOpen(t *testing.T) {
	ctx := context.Background()
	freshDir, copyDir := t.TempDir(), t.TempDir()
	fresh, err := store.Open(ctx, options(freshDir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fresh.Close() })
	copied, err := storetest.Open(ctx, options(copyDir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = copied.Close() })

	if want, got := dump(t, fresh.Read), dump(t, copied.Read); want != got {
		t.Errorf("schema or rows differ from a fresh database:\nfresh:\n%s\ncopy:\n%s", want, got)
	}
	if want, got := migrations(t, fresh.Read), migrations(t, copied.Read); want != got {
		t.Errorf("applied migrations: fresh %s, copy %s", want, got)
	}
	for _, p := range []string{"foreign_keys", "journal_mode", "busy_timeout", "synchronous", "query_only", "user_version"} {
		if want, got := pragma(t, fresh.Write, p), pragma(t, copied.Write, p); want != got {
			t.Errorf("writer PRAGMA %s: fresh %s, copy %s", p, want, got)
		}
		if want, got := pragma(t, fresh.Read, p), pragma(t, copied.Read, p); want != got {
			t.Errorf("reader PRAGMA %s: fresh %s, copy %s", p, want, got)
		}
	}
	if got := pragma(t, copied.Write, "foreign_keys"); got != "1" {
		t.Errorf("foreign_keys = %s, want 1", got)
	}
	if copied.Restored {
		t.Error("the copy reports a restore")
	}
	for _, dir := range []string{freshDir, copyDir} {
		if _, err := os.Stat(filepath.Join(dir, store.BackupDir)); !os.IsNotExist(err) {
			t.Errorf("%s: backup directory exists (err %v)", dir, err)
		}
	}
}

// Every Open gets its own file: a write through one is invisible to the next.
func TestOpenCopiesAreIndependent(t *testing.T) {
	ctx := context.Background()
	first, err := storetest.Open(ctx, options(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if _, err := first.Write.ExecContext(ctx, "CREATE TABLE storetest_marker (x INTEGER)"); err != nil {
		t.Fatal(err)
	}
	second, err := storetest.Open(ctx, options(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	var n int
	if err := second.Read.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name = 'storetest_marker'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("the second copy sees a table created in the first")
	}
}

// A directory that already holds a database is refused instead of overwritten.
func TestOpenRefusesExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, store.FileName), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if db, err := storetest.Open(context.Background(), options(dir)); err == nil {
		_ = db.Close()
		t.Fatal("Open overwrote an existing database")
	}
	got, err := os.ReadFile(filepath.Join(dir, store.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Errorf("existing database changed to %d bytes", len(got))
	}
}

// dump is store.Dump without the rows of goose_db_version, whose timestamps differ by nature;
// migrations compares that table without them.
func dump(t *testing.T, db *sql.DB) string {
	t.Helper()
	var b bytes.Buffer
	if err := store.Dump(context.Background(), db, &b); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for line := range strings.Lines(b.String()) {
		if !strings.Contains(line, "INSERT INTO") || !strings.Contains(line, "goose_db_version") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "")
}

func migrations(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "SELECT version_id, is_applied FROM goose_db_version ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var b strings.Builder
	for rows.Next() {
		var version int64
		var applied bool
		if err := rows.Scan(&version, &applied); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%d:%t ", version, applied)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func pragma(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var v string
	if err := db.QueryRowContext(context.Background(), "PRAGMA "+name).Scan(&v); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return v
}
