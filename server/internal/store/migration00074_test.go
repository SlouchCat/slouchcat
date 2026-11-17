package store

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/slouchcat/slouchcat/server/migrations"
)

// migrationsBefore are the migrations whose names sort before name (the state a migration test
// builds its rows on).
func migrationsBefore(t *testing.T, name string) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() >= name {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = &fstest.MapFile{Data: data}
	}
	return out
}

// Migration 00074 (concept 25, 08-datenmodell-api.md section 2.9, «Aufräumen»): external_titles
// gets used_at with its index; a title stored before the run starts with its fetch time, a new one
// may leave it empty.
func TestMigration00074ExternalTitlesUsed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	opts := testOptions(t, dir)
	opts.migrations = migrationsBefore(t, "00074")
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db.Write, `INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('movie', 550, 1234), ('show', 1399, 99)`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for query, want := range map[string]int64{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 74 AND is_applied = 1`:            1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                             0,
		`SELECT count(*) FROM pragma_table_info('external_titles') WHERE name = 'used_at'`:          1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'external_titles_used'`: 1,
		`SELECT used_at FROM external_titles WHERE media = 'movie' AND tmdb_id = 550`:               1234,
		`SELECT used_at FROM external_titles WHERE media = 'show' AND tmdb_id = 1399`:               99,
	} {
		var got int64
		if err := db.Read.QueryRowContext(ctx, query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	mustExec(t, db.Write, `INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('movie', 603, 5)`)
}
