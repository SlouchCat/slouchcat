package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// The backup before a library delete keeps its last two and never touches the five backups
// before a migration; the backup page reads its name like every automatic backup.
func TestBackupBeforeLibraryDeleteKeepsItsOwnRetention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir := filepath.Join(cfg, BackupDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	var migrations []string
	for i := range KeepBackups {
		name := autoBackupName(preMigrationPrefix, "1.30.0", base.Add(time.Duration(i)*time.Minute), backupSuffix)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
		migrations = append(migrations, name)
	}
	var made []string
	for i := range 3 {
		name, err := BackupBeforeLibraryDelete(ctx, db.Read, cfg, "1.31.0+dev", base.Add(time.Hour+time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, name)
		if v, at, ok := ParseAutoBackupName(name); !ok || v != "1.31.0_dev" || !at.Equal(base.Add(time.Hour+time.Duration(i)*time.Minute)) {
			t.Errorf("ParseAutoBackupName(%s) = %q, %v, %v", name, v, at, ok)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := slices.Sorted(slices.Values(append(append([]string{}, migrations...), made[1:]...)))
	if !slices.Equal(slices.Sorted(slices.Values(names)), want) {
		t.Errorf("backups = %v, want %v", names, want)
	}
	if err := CheckRestorable(ctx, filepath.Join(dir, made[2])); err != nil {
		t.Errorf("the backup is no restorable database: %v", err)
	}
}
