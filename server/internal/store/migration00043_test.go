package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00043 (1.23.1, decision of 9 November 2026) switches every library still at «Im
// Voraus» to «Aus», per kind, on the frozen state of migration 00042 (trickplay.sql): «Beim Start»,
// «Beim Start alle Folgen» and «Aus» stay, and so do «Speichern» and the scope of the look-ahead,
// also on a switched row. The state holds no video library at «Im Voraus», so the test adds one
// before the migration with «Speichern» off and the scope «nächste Folge».
func TestMigration00043TrickplayOff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "trickplay.sql"))
	raw := openRaw(t, path)
	if _, err := raw.ExecContext(ctx, `INSERT INTO libraries (id, name, kind, created_at, updated_at, trickplay_movies,
		trickplay_movies_keep, trickplay_series, trickplay_series_keep, trickplay_series_scope)
		VALUES (9, 'Vorschau', 'video', 1, 1, 'ahead', 0, 'ahead', 0, 'next_episode')`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var violations int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign_key_check after 00043: %d violations (%v)", violations, err)
	}
	type setting struct {
		movies      string
		moviesKeep  int
		series      string
		seriesKeep  int
		seriesScope string
	}
	want := map[int64]setting{
		1: {"on_start", 0, "series_start", 1, "season_rest"}, // Filme: untouched
		2: {"off", 1, "on_start", 0, "next_episode"},         // Serien: untouched
		3: {"off", 1, "off", 1, "series"},                    // Hörbücher: ahead → off for both
		9: {"off", 0, "off", 0, "next_episode"},              // Vorschau: ahead → off, keep and scope stay
	}
	rows, err := db.Read.QueryContext(ctx, `SELECT id, trickplay_movies, trickplay_movies_keep, trickplay_series,
		trickplay_series_keep, trickplay_series_scope FROM libraries`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id int64
		var got setting
		if err := rows.Scan(&id, &got.movies, &got.moviesKeep, &got.series, &got.seriesKeep, &got.seriesScope); err != nil {
			t.Fatal(err)
		}
		if w, ok := want[id]; !ok {
			t.Errorf("unexpected library %d", id)
		} else if got != w {
			t.Errorf("library %d after 00043: %+v, want %+v", id, got, w)
		}
		seen++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen != len(want) {
		t.Errorf("%d libraries after 00043, want %d", seen, len(want))
	}
	// «Im Voraus» stays a valid choice after the switch.
	if _, err := db.Write.ExecContext(ctx, `UPDATE libraries SET trickplay_movies = 'ahead', trickplay_series = 'ahead' WHERE id = 9`); err != nil {
		t.Errorf("ahead after 00043: %v", err)
	}
}
