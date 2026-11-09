package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00040 (concept 17 stage 3) adds person_images and collection_images on the frozen state
// of migration 00039: both tables STRICT and WITHOUT ROWID, empty after the migration, the
// constraints refuse every row that does not fit the rule «ok carries the file, missing is one bare
// jpeg row», and deleting a person or a collection cascades into its image rows.
func TestMigration00040PersonCollectionImages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "webradio2.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var violations int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign_key_check after 00040: %d violations (%v)", violations, err)
	}
	for _, table := range []string{"person_images", "collection_images"} {
		var strict, withoutRowid, rows int
		if err := db.Read.QueryRowContext(ctx, `SELECT strict, wr FROM pragma_table_list WHERE schema = 'main' AND name = ?`, table).Scan(&strict, &withoutRowid); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if strict != 1 || withoutRowid != 1 {
			t.Errorf("%s: strict %d, without rowid %d; want both", table, strict, withoutRowid)
		}
		if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&rows); err != nil || rows != 0 {
			t.Errorf("%s holds %d rows after the migration (%v), want none", table, rows, err)
		}
	}

	const coll, empty = `'01M6586A0000000000000C0001'`, `'01M6586A0000000000000C0002'`
	mustExec(t, db.Write, `INSERT INTO collections (id, source, tmdb_id, name, poster_path, created_at, updated_at)
		VALUES (`+coll+`, 'tmdb', 990001, 'Test', '/c.jpg', 1, 1), (`+empty+`, 'tmdb', 990002, 'Empty', '/e.jpg', 1, 1)`)
	const ok = `'ok', '/p.jpg', 'w185', 'people/1/w185.jpg', 'abc', 'image/jpeg', 185, 278, 1000, 1)`
	for _, good := range []string{
		`INSERT INTO person_images VALUES (1, 'jpeg', ` + ok,
		`INSERT INTO person_images VALUES (1, 'webp', 'ok', '/p.jpg', 'w185', 'people/1/w185.webp', 'def', 'image/webp', 185, 278, 800, 1)`,
		`INSERT INTO person_images VALUES (2, 'jpeg', 'missing', '/q.jpg', 'w185', NULL, NULL, NULL, NULL, NULL, NULL, 1)`,
		`INSERT INTO collection_images VALUES (` + coll + `, 'jpeg', 'ok', '/c.jpg', 'w400', 'collections/x/w400.jpg', 'abc', 'image/jpeg', 400, 600, 4000, 1)`,
		`INSERT INTO collection_images VALUES (` + coll + `, 'webp', 'ok', '/c.jpg', 'w400', 'collections/x/w400.webp', 'def', 'image/webp', 400, 600, 3000, 1)`,
	} {
		if _, err := db.Write.ExecContext(ctx, good); err != nil {
			t.Errorf("%s: %v", good, err)
		}
	}
	for _, table := range []struct{ name, id string }{{"person_images", "3"}, {"collection_images", empty}} {
		ins := `INSERT INTO ` + table.name + ` VALUES (` + table.id + `, `
		for _, bad := range []string{
			`'png', ` + ok, // unknown format
			`'jpeg', 'found', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', 1, 1, 1, 1)`,         // unknown status
			`'jpeg', 'ok', '/p.jpg', 'w185', NULL, 'h', 'image/jpeg', 1, 1, 1, 1)`,           // ok without rel_path
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', NULL, 'image/jpeg', 1, 1, 1, 1)`,           // ok without content_hash
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', NULL, 1, 1, 1, 1)`,                    // ok without media_type
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', NULL, 1, 1, 1)`,         // ok without width
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', 1, NULL, 1, 1)`,         // ok without height
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', 1, 1, NULL, 1)`,         // ok without bytes
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', 0, 1, 1, 1)`,            // width 0
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', 1, 0, 1, 1)`,            // height 0
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', 1, 1, 0, 1)`,            // bytes 0
			`'webp', 'missing', '/p.jpg', 'w185', NULL, NULL, NULL, NULL, NULL, NULL, 1)`,    // missing as webp
			`'jpeg', 'missing', '/p.jpg', 'w185', 'r', NULL, NULL, NULL, NULL, NULL, 1)`,     // missing with rel_path
			`'jpeg', 'missing', '/p.jpg', 'w185', NULL, NULL, NULL, 185, 278, NULL, 1)`,      // missing with dimensions
			`'jpeg', 'missing', NULL, 'w185', NULL, NULL, NULL, NULL, NULL, NULL, 1)`,        // without source
			`'jpeg', 'missing', '/p.jpg', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1)`,      // without size
			`'jpeg', 'missing', '/p.jpg', 'w185', NULL, NULL, NULL, NULL, NULL, NULL, NULL)`, // without fetched_at
			`'jpeg', 'ok', '/p.jpg', 'w185', 'r', 'h', 'image/jpeg', 'wide', 1, 1, 1)`,       // STRICT: text in an integer column
		} {
			if _, err := db.Write.ExecContext(ctx, ins+bad); err == nil {
				t.Errorf("%s%s: accepted, want a constraint error", ins, bad)
			}
		}
	}
	for _, bad := range []string{
		`INSERT INTO person_images VALUES (1, 'jpeg', ` + ok,   // (person, format) twice
		`INSERT INTO person_images VALUES (999, 'jpeg', ` + ok, // unknown person
		`INSERT INTO collection_images VALUES ('01M6586A0000000000000C0009', 'jpeg', 'missing', '/c.jpg', 'w400', NULL, NULL, NULL, NULL, NULL, NULL, 1)`, // unknown collection
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: accepted, want a constraint error", bad)
		}
	}

	// Deleting a person or a collection takes its image rows along.
	mustExec(t, db.Write, `DELETE FROM people WHERE id = 1`)
	mustExec(t, db.Write, `DELETE FROM collections WHERE id = `+coll)
	for query, want := range map[string]int{
		`SELECT count(*) FROM person_images WHERE person_id = 1`:               0,
		`SELECT count(*) FROM person_images WHERE person_id = 2`:               1,
		`SELECT count(*) FROM collection_images WHERE collection_id = ` + coll: 0,
	} {
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil || n != want {
			t.Errorf("%s = %d (%v), want %d", query, n, err, want)
		}
	}
}
