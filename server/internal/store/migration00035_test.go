package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"slices"
	"testing"
)

// Migration 00035 (concept 15 P-7) rebuilds home_rows with an id per row on the frozen state of
// migration 00034: the eight rows keep position, hidden flag and time (id = key), «Meine Medien»
// and one row per library follow hidden in id order, a new library gets its row hidden at the
// end, and the constraints refuse a row that does not fit its key.
func TestMigration00035HomeRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "feinschliff.sql"))
	type row struct {
		id        string
		key       string
		library   int64
		position  int
		hidden    int
		updatedAt int64
	}
	read := func(q *sql.DB) []row {
		t.Helper()
		cols := "key, key, 0, position, hidden, updated_at"
		var hasID int
		if err := q.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info('home_rows') WHERE name = 'id'`).Scan(&hasID); err != nil {
			t.Fatal(err)
		}
		if hasID == 1 {
			cols = "id, key, COALESCE(library_id, 0), position, hidden, updated_at"
		}
		rows, err := q.QueryContext(ctx, `SELECT `+cols+` FROM home_rows ORDER BY position, key`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.key, &r.library, &r.position, &r.hidden, &r.updatedAt); err != nil {
				t.Fatal(err)
			}
			out = append(out, r)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	raw := openRaw(t, path)
	before := map[string]row{}
	for _, r := range read(raw) {
		before[r.key] = r
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if len(before) != 8 {
		t.Fatalf("frozen state holds %d home rows, want 8", len(before))
	}

	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var ids []string
	for _, r := range read(db.Read) {
		ids = append(ids, r.id)
		if old, ok := before[r.key]; ok {
			if r != old {
				t.Errorf("row %+v, want %+v carried over with id = key", r, old)
			}
			continue
		}
		if r.hidden != 1 || r.position < 8 {
			t.Errorf("new row %+v, want hidden behind the eight", r)
		}
	}
	// feinschliff.sql has the libraries 1 «Filme» and 2 «Serien».
	if want := []string{"libraries", "library:1", "library:2"}; !slices.Equal(ids[8:], want) {
		t.Errorf("rows behind the eight %v, want %v", ids[8:], want)
	}

	// A new library gets its row, hidden, at the end.
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (7, 'Hörbücher', 'audiobooks', 42, 42)`); err != nil {
		t.Fatal(err)
	}
	var id string
	var position, hidden int
	var updatedAt int64
	if err := db.Read.QueryRowContext(ctx, `SELECT id, position, hidden, updated_at FROM home_rows WHERE library_id = 7`).Scan(&id, &position, &hidden, &updatedAt); err != nil ||
		id != "library:7" || position != 11 || hidden != 1 || updatedAt != 42 {
		t.Errorf("row of the new library = %s at %d, hidden %d, time %d (%v); want library:7 at 11, hidden, 42", id, position, hidden, updatedAt, err)
	}

	for _, bad := range []string{
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('picks', 'picks', NULL, 20, 0, 0)`,       // twice
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('trailers', 'trailers', NULL, 20, 0, 0)`, // unknown key
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('library', 'library', NULL, 20, 0, 0)`,   // no library
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('library:9', 'library', 1, 20, 0, 0)`,    // id of another library
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('library:1', 'library', 1, 20, 0, 0)`,    // second row of a library
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('recent:1', 'recent', 1, 20, 0, 0)`,      // library on a fixed row
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('library:99', 'library', 99, 20, 0, 0)`,  // unknown library
		`UPDATE home_rows SET hidden = 2 WHERE id = 'picks'`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: accepted, want a constraint error", bad)
		}
	}
	// Deleting a library takes its row along (ON DELETE CASCADE, a precaution).
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM libraries WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM home_rows WHERE id = 'library:7'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("row of the deleted library: %d (%v), want gone", n, err)
	}
}
