package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00042 (concept 18 part A, 8.3) adds the trickplay settings to libraries on the frozen
// state of migration 00041 (empfehlungen.sql): every existing library stands at «Im Voraus» for
// films and series with «Speichern» and the scope «ganze Serie» (F-11), and the CHECK constraints
// refuse every other value. Open runs 00043 as well, which switches «Im Voraus» to «Aus» (1.23.1);
// TestMigration00043TrickplayOff checks that step on its own.
func TestMigration00042Trickplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "empfehlungen.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var violations int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign_key_check after 00042: %d violations (%v)", violations, err)
	}
	var libraries, defaults int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*), coalesce(sum(trickplay_movies = 'off' AND trickplay_movies_keep = 1
		AND trickplay_series = 'off' AND trickplay_series_keep = 1 AND trickplay_series_scope = 'series'), 0) FROM libraries`).Scan(&libraries, &defaults); err != nil {
		t.Fatal(err)
	}
	if libraries == 0 || defaults != libraries {
		t.Errorf("%d of %d libraries at the defaults", defaults, libraries)
	}
	var id int64
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM libraries ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, good := range []string{
		`UPDATE libraries SET trickplay_movies = 'on_start', trickplay_movies_keep = 0 WHERE id = ?`,
		`UPDATE libraries SET trickplay_movies = 'off' WHERE id = ?`,
		`UPDATE libraries SET trickplay_series = 'series_start', trickplay_series_scope = 'season_rest' WHERE id = ?`,
		`UPDATE libraries SET trickplay_series = 'on_start', trickplay_series_keep = 0, trickplay_series_scope = 'next_episode' WHERE id = ?`,
	} {
		if _, err := db.Write.ExecContext(ctx, good, id); err != nil {
			t.Errorf("%s: %v", good, err)
		}
	}
	for _, bad := range []string{
		`UPDATE libraries SET trickplay_movies = 'series_start' WHERE id = ?`, // films have no look-ahead
		`UPDATE libraries SET trickplay_movies = 'never' WHERE id = ?`,
		`UPDATE libraries SET trickplay_movies_keep = 2 WHERE id = ?`,
		`UPDATE libraries SET trickplay_series = 'later' WHERE id = ?`,
		`UPDATE libraries SET trickplay_series_keep = -1 WHERE id = ?`,
		`UPDATE libraries SET trickplay_series_scope = 'season' WHERE id = ?`,
		`UPDATE libraries SET trickplay_movies = NULL WHERE id = ?`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad, id); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
