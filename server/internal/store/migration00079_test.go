package store

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// Migration 00079 (concept 25 step 3.4, 08-datenmodell-api.md section 2.11, 07-administration.md
// section 10): home_rows takes the key wishes; the new row «Erfüllte Wünsche» comes right after
// «Weiterschauen», visible, and every other row keeps its order, its hidden flag and its time; on
// a start page that already shows 15 rows it comes hidden. The trigger for a new library row still
// works.
func TestMigration00079HomeRowsWishes(t *testing.T) {
	t.Parallel()
	type row struct {
		id     string
		hidden int
	}
	order := func(t *testing.T, db *DB) []row {
		t.Helper()
		rows, err := db.Read.QueryContext(context.Background(), `SELECT id, hidden FROM home_rows ORDER BY position, id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.hidden); err != nil {
				t.Fatal(err)
			}
			out = append(out, r)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, c := range []struct {
		name   string
		before string // SQL on the state before 00079
		hidden int    // the new row
	}{
		{"default order", `UPDATE home_rows SET position = position * 10, updated_at = 77 WHERE id = 'recent'`, 0},
		{"fifteen rows shown", `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES
			(1, 'A', 'video', 1, 1), (2, 'B', 'video', 1, 1), (3, 'C', 'video', 1, 1), (4, 'D', 'video', 1, 1), (5, 'E', 'video', 1, 1);
			UPDATE home_rows SET hidden = 0;`, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dir := t.TempDir()
			opts := testOptions(t, dir)
			opts.migrations = migrationsBefore(t, "00079")
			db, err := Open(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}
			mustExec(t, db.Write, c.before)
			// «Weiterschauen» in the middle, so rows on both sides show the shift.
			mustExec(t, db.Write, `UPDATE home_rows SET position = (SELECT count(*) FROM home_rows) WHERE id = 'continue'`)
			before := order(t, db)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = Open(ctx, testOptions(t, dir))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			after := order(t, db)
			i := slices.IndexFunc(before, func(r row) bool { return r.id == "continue" })
			want := slices.Concat(before[:i+1], []row{{"wishes", c.hidden}}, before[i+1:])
			if !slices.Equal(after, want) {
				t.Errorf("rows after = %v\nwant %v", after, want)
			}
			for query, want := range map[string]int64{
				`SELECT count(*) FROM goose_db_version WHERE version_id = 79 AND is_applied = 1`:                 1,
				`SELECT count(*) FROM pragma_foreign_key_check`:                                                  0,
				`SELECT count(*) FROM sqlite_master WHERE name IN ('home_rows_new', 'home_rows_anchor')`:         0,
				`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'home_rows_library_added'`: 1,
				`SELECT updated_at FROM home_rows WHERE id = 'wishes'`:                                           0,
			} {
				var got int64
				if err := db.Read.QueryRowContext(ctx, query).Scan(&got); err != nil {
					t.Fatalf("%s: %v", query, err)
				}
				if got != want {
					t.Errorf("%s = %d, want %d", query, got, want)
				}
			}
			if c.name == "default order" {
				var at int64
				if err := db.Read.QueryRowContext(ctx, `SELECT updated_at FROM home_rows WHERE id = 'recent'`).Scan(&at); err != nil || at != 77 {
					t.Errorf("updated_at of recent = %d, %v", at, err)
				}
			}
			// The key is accepted now, an unknown one still refused; a new library still gets its row.
			if _, err := db.Write.ExecContext(ctx, `UPDATE home_rows SET key = 'nope', id = 'nope' WHERE id = 'wishes'`); err == nil {
				t.Error("an unknown key was accepted")
			}
			mustExec(t, db.Write, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (9, 'Neu', 'video', 5, 5)`)
			var n int
			if err := db.Read.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM home_rows WHERE id = 'library:%d' AND hidden = 1`, 9)).Scan(&n); err != nil || n != 1 {
				t.Errorf("row of the new library: %d, %v", n, err)
			}
		})
	}
}
