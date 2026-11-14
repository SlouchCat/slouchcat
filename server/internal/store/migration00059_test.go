package store

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"
)

// Migration 00059 (concept 21 P-11) adds «Meine Medien» on the frozen state after 00058
// (personenseite.sql, four libraries): the libraries by id, then radio, collections and picks, all
// in the side navigation, collections and picks not on the start page. A library created later
// gets its entry at the end, visible in both places (O-20); deleting a library takes its entry
// along. An entry whose id does not match its key or library and a flag that is no 0 or 1 are
// refused.
func TestMigration00059MyMedia(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "personenseite.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	type entry struct {
		id                string
		position          int
		sidebar, homePage int
	}
	entries := func() []entry {
		t.Helper()
		rows, err := db.Read.QueryContext(ctx, `SELECT id, position, hide_sidebar, hide_home FROM my_media ORDER BY position, id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []entry
		for rows.Next() {
			var e entry
			if err := rows.Scan(&e.id, &e.position, &e.sidebar, &e.homePage); err != nil {
				t.Fatal(err)
			}
			out = append(out, e)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	want := []entry{{"library:1", 0, 0, 0}, {"library:2", 1, 0, 0}, {"library:3", 2, 0, 0}, {"library:4", 3, 0, 0},
		{"radio", 4, 0, 0}, {"collections", 5, 0, 1}, {"picks", 6, 0, 1}}
	if got := entries(); !slices.Equal(got, want) {
		t.Fatalf("my_media = %v, want %v", got, want)
	}
	var applied int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id = 59 AND is_applied = 1`).Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("migration 00059 applied %d times (%v)", applied, err)
	}

	if _, err := db.Write.ExecContext(ctx, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (9, 'Neu', 'video', 5, 5)`); err != nil {
		t.Fatal(err)
	}
	added := []entry{{"library:1", 0, 0, 0}, {"library:2", 1, 0, 0}, {"library:3", 2, 0, 0}, {"library:4", 3, 0, 0},
		{"library:9", 4, 0, 0}, {"radio", 5, 0, 0}, {"collections", 6, 0, 1}, {"picks", 7, 0, 1}}
	if got := entries(); !slices.Equal(got, added) {
		t.Errorf("after a new library my_media = %v, want library:9 behind the last library and visible: %v", got, added)
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM libraries WHERE id = 9`); err != nil {
		t.Fatal(err)
	}
	ids := func(es []entry) []string {
		out := make([]string, len(es))
		for i, e := range es {
			out[i] = e.id
		}
		return out
	}
	if got := entries(); !slices.Equal(ids(got), ids(want)) {
		t.Errorf("after deleting the library my_media = %v, want the order %v", got, ids(want))
	}

	for what, query := range map[string]string{
		"a library entry with another id": `INSERT INTO my_media (id, key, library_id, position, updated_at) VALUES ('library:2', 'library', 1, 9, 0)`,
		"a library entry without library": `INSERT INTO my_media (id, key, library_id, position, updated_at) VALUES ('library:', 'library', NULL, 9, 0)`,
		"radio with a library":            `UPDATE my_media SET library_id = 1 WHERE id = 'radio'`,
		"an unknown key":                  `INSERT INTO my_media (id, key, position, updated_at) VALUES ('genres', 'genres', 9, 0)`,
		"a sidebar flag of 2":             `UPDATE my_media SET hide_sidebar = 2 WHERE id = 'radio'`,
		"a home flag of 2":                `UPDATE my_media SET hide_home = 2 WHERE id = 'radio'`,
		"a negative position":             `UPDATE my_media SET position = -1 WHERE id = 'radio'`,
	} {
		if _, err := db.Write.ExecContext(ctx, query); err == nil {
			t.Errorf("%s was accepted", what)
		}
	}
}
