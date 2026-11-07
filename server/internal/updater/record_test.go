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
