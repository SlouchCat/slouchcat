package updater

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The recorder writes server.json before and after the migrations; the second write keeps what
// the first recorded.
func TestRecorderWritesBeforeAndAfter(t *testing.T) {
	dir := t.TempDir()
	r := NewRecorder(dir, "1.16.0", slog.New(slog.DiscardHandler))
	clock := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return clock }
	r.Before(37, "pre-migration-1.16.0-20261107T100000.000Z.db")
	var got ServerRecord
	if err := ReadJSON(filepath.Join(dir, ServerFile), &got); err != nil {
		t.Fatal(err)
	}
	want := ServerRecord{Version: "1.16.0", SchemaBefore: 37, Backup: "pre-migration-1.16.0-20261107T100000.000Z.db", StartedAt: clock}
	if got != want {
		t.Fatalf("before: %+v, want %+v", got, want)
	}
	clock = clock.Add(3 * time.Second)
	r.After(38)
	if err := ReadJSON(filepath.Join(dir, ServerFile), &got); err != nil {
		t.Fatal(err)
	}
	want.SchemaAfter, want.MigratedAt = 38, clock
	if got != want {
		t.Fatalf("after: %+v, want %+v", got, want)
	}
	if info, err := os.Stat(filepath.Join(dir, ServerFile)); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("server.json mode: %v, %v", info, err)
	}
}

// migratedByUpdaterRule is the updater's reading of server.json (server/cmd/updater migrated): a
// record without migratedAt or with schemaAfter != schemaBefore counts as migrated.
func migratedByUpdaterRule(rec ServerRecord) bool {
	return rec.MigratedAt.IsZero() || rec.SchemaAfter != rec.SchemaBefore
}

// A second start of the same version (restart: unless-stopped after a crash) keeps what the first
// recorded: the schema found before the first migration and its backup, so the updater still sees
// the migration; a start that took a new backup after a half-failed migration still names the
// first one. Another version starts a fresh record.
func TestRecorderKeepsTheFirstStartOfAVersion(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	clock := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	start := func(v string) *Recorder {
		r := NewRecorder(dir, v, log)
		r.now = func() time.Time { return clock }
		return r
	}
	read := func() ServerRecord {
		t.Helper()
		var got ServerRecord
		if err := ReadJSON(filepath.Join(dir, ServerFile), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	first := start("1.16.0")
	if b := first.KeptBackup(); b != "" {
		t.Fatalf("kept backup without a record: %q", b)
	}
	first.Before(37, "pre-migration-1.16.0-A.db")
	first.After(38)

	// Migrated, then crashed: the second start has nothing pending.
	clock = clock.Add(time.Minute)
	second := start("1.16.0")
	if b := second.KeptBackup(); b != "pre-migration-1.16.0-A.db" {
		t.Fatalf("kept backup = %q", b)
	}
	second.Before(38, "")
	if got := read(); got.SchemaBefore != 37 || got.Backup != "pre-migration-1.16.0-A.db" ||
		!got.StartedAt.Equal(clock) || got.SchemaAfter != 0 || !got.MigratedAt.IsZero() || !migratedByUpdaterRule(got) {
		t.Fatalf("second start before the migrations: %+v", got)
	}
	second.After(38)
	if got := read(); got.SchemaBefore != 37 || got.SchemaAfter != 38 || !got.MigratedAt.Equal(clock) || !migratedByUpdaterRule(got) {
		t.Fatalf("second start after the migrations: %+v, want counted as migrated", got)
	}

	// A half-failed migration: the next start takes a new backup and still names the first one.
	third := start("1.16.0")
	third.Before(38, "pre-migration-1.16.0-B.db")
	if got := read(); got.Backup != "pre-migration-1.16.0-A.db" || got.SchemaBefore != 37 {
		t.Fatalf("third start: %+v, want the first backup", got)
	}

	// The first start of a version without a backup keeps the one a later start took.
	dir2 := t.TempDir()
	r := NewRecorder(dir2, "1.16.0", log)
	r.Before(37, "")
	r = NewRecorder(dir2, "1.16.0", log)
	r.Before(37, "pre-migration-1.16.0-C.db")
	var got ServerRecord
	if err := ReadJSON(filepath.Join(dir2, ServerFile), &got); err != nil || got.Backup != "pre-migration-1.16.0-C.db" {
		t.Fatalf("first non-empty backup: %+v, %v", got, err)
	}

	// Another version (the rollback to the old one, or the next release) starts fresh.
	other := start("1.15.0")
	if b := other.KeptBackup(); b != "" {
		t.Fatalf("kept backup of another version: %q", b)
	}
	other.Before(37, "")
	other.After(37)
	if got := read(); got.Version != "1.15.0" || got.SchemaBefore != 37 || got.Backup != "" || migratedByUpdaterRule(got) {
		t.Fatalf("other version: %+v", got)
	}
}

// Without a companion the folder is missing: nothing is written, one debug line says so. A folder
// the server cannot write to is one warning, not one per write; neither panics nor stops.
func TestRecorderWithoutFolder(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	missing := filepath.Join(t.TempDir(), "none")
	r := NewRecorder(missing, "1.16.0", log)
	r.Before(1, "")
	r.After(1)
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("folder created: %v", err)
	}
	if n := strings.Count(logs.String(), "no updater folder"); n != 1 || strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("logs for a missing folder:\n%s", logs.String())
	}

	logs.Reset()
	file := filepath.Join(t.TempDir(), "plain-file") // a file where the folder should be
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r = NewRecorder(file, "1.16.0", log)
	r.Before(1, "")
	r.After(1)
	if n := strings.Count(logs.String(), "level=WARN"); n != 1 {
		t.Errorf("want one warning for an unwritable folder:\n%s", logs.String())
	}
}
