package store

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/slouchcat/slouchcat/server/internal/updater"
)

// schemaCall is one call of the schema hooks.
type schemaCall struct {
	after  bool
	schema int64
	backup string
}

func recordingHooks(calls *[]schemaCall) SchemaHooks {
	return SchemaHooks{
		Before: func(schema int64, backup string) { *calls = append(*calls, schemaCall{schema: schema, backup: backup}) },
		After:  func(schema int64) { *calls = append(*calls, schemaCall{after: true, schema: schema}) },
	}
}

// The schema hooks run at every Open (ADR 0024, point 12): a new database from 0 to the newest
// version without a backup, a reopen without a pending migration with the same version twice, a
// pending migration with the name of its backup before and the new version after.
func TestSchemaHooks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	var calls []schemaCall
	opts := testOptions(t, dir)
	opts.Schema = recordingHooks(&calls)
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	var newest int64
	if err := db.Read.QueryRow("SELECT max(version_id) FROM goose_db_version").Scan(&newest); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if want := []schemaCall{{schema: 0}, {after: true, schema: newest}}; !equalCalls(calls, want) {
		t.Fatalf("new database: %+v, want %+v", calls, want)
	}

	calls = nil
	db, err = Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if want := []schemaCall{{schema: newest}, {after: true, schema: newest}}; !equalCalls(calls, want) {
		t.Fatalf("nothing pending: %+v, want %+v", calls, want)
	}

	calls = nil
	opts.migrations = withSecondMigration(t)
	db, err = Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got := backups(t, dir)
	if len(got) != 1 || len(calls) != 2 {
		t.Fatalf("pending migration: backups %v, calls %+v", got, calls)
	}
	if want := []schemaCall{{schema: newest, backup: filepath.Base(got[0])}, {after: true, schema: 99999}}; !equalCalls(calls, want) {
		t.Fatalf("pending migration: %+v, want %+v", calls, want)
	}
}

// A migration that fails is heard before, never after: the updater then sees no schemaAfter.
func TestSchemaHooksSkipAfterOnFailure(t *testing.T) {
	t.Parallel()
	var calls []schemaCall
	opts := testOptions(t, t.TempDir())
	opts.Schema = recordingHooks(&calls)
	opts.migrations = fstest.MapFS{"00001_broken.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT * FROM missing_table;\n")}}
	if _, err := Open(context.Background(), opts); err == nil {
		t.Fatal("want the broken migration to fail")
	}
	if want := []schemaCall{{schema: 0}}; !equalCalls(calls, want) {
		t.Fatalf("calls %+v, want %+v", calls, want)
	}
}

// The server's record for the updater (updater.Recorder as the schema hooks) on a frozen state:
// server.json carries the version found and the pre-migration backup while the migrations are
// still to run, and the newest version once they are through.
func TestSchemaHooksWriteServerRecord(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "verwaltung.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, shared := t.TempDir(), t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	var frozen int64
	raw, err := sql.Open("sqlite", "file:"+filepath.Join(cfg, FileName)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	err = raw.QueryRow("SELECT max(version_id) FROM goose_db_version").Scan(&frozen)
	if err := errors.Join(err, raw.Close()); err != nil {
		t.Fatal(err)
	}
	rec := updater.NewRecorder(shared, "1.16.0", slog.New(slog.NewTextHandler(io.Discard, nil)))
	var during updater.ServerRecord
	opts := Options{ConfigDir: cfg, Version: "1.16.0", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	opts.Schema = SchemaHooks{
		Before: func(schema int64, backup string) {
			rec.Before(schema, backup)
			if err := updater.ReadJSON(filepath.Join(shared, updater.ServerFile), &during); err != nil {
				t.Errorf("server.json before the migrations: %v", err)
			}
		},
		After: rec.After,
	}
	db, err := Open(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var newest int64
	if err := db.Read.QueryRow("SELECT max(version_id) FROM goose_db_version").Scan(&newest); err != nil {
		t.Fatal(err)
	}
	if frozen >= newest {
		t.Fatalf("frozen state at %d, newest %d: nothing to migrate", frozen, newest)
	}
	got := backups(t, cfg)
	if len(got) != 1 {
		t.Fatalf("backups %v, want one", got)
	}
	if during.Version != "1.16.0" || during.SchemaBefore != frozen || during.Backup != filepath.Base(got[0]) ||
		during.StartedAt.IsZero() || during.SchemaAfter != 0 || !during.MigratedAt.IsZero() {
		t.Errorf("server.json before = %+v", during)
	}
	var after updater.ServerRecord
	if err := updater.ReadJSON(filepath.Join(shared, updater.ServerFile), &after); err != nil {
		t.Fatal(err)
	}
	if after.SchemaBefore != frozen || after.SchemaAfter != newest || after.Backup != during.Backup ||
		!after.StartedAt.Equal(during.StartedAt) || after.MigratedAt.IsZero() {
		t.Errorf("server.json after = %+v", after)
	}
}

func equalCalls(a, b []schemaCall) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
