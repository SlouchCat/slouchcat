package store

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Every frozen milestone state in testdata/db/*.sql must load into an empty database and
// migrate to the current schema without foreign key violations (plan 6.2 point 4, risk
// "Datenmodell-Migrationen"). The file is replayed with the plain driver, then Open runs the
// pending migrations (with the pre-migration backup) exactly as the server does at startup.
// Whatever row order a state stored, migration 00021 (M11, ADR 0011) leaves exactly the eight
// keys of today on the positions 0 to 7, spotlight first and released directly behind recent.
func TestFrozenStatesMigrate(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "db")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var states []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			states = append(states, e.Name())
		}
	}
	if len(states) == 0 {
		t.Fatal("no frozen states in testdata/db; every milestone from M1a on adds one")
	}
	for _, name := range states {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			cfg := t.TempDir()
			replay(t, filepath.Join(cfg, FileName), string(script))
			db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if err != nil {
				t.Fatalf("migrate frozen state: %v", err)
			}
			defer db.Close()
			var n int
			if err := db.Read.QueryRow(`SELECT count(*) FROM items`).Scan(&n); err != nil {
				t.Fatalf("items table after migration: %v", err)
			}
			if n == 0 {
				t.Error("frozen state should carry sample items")
			}
			var violations int
			if err := db.Read.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
				t.Fatalf("foreign_key_check: %v", err)
			}
			if violations != 0 {
				t.Errorf("foreign_key_check after migration: %d violations", violations)
			}
			order := homeRowOrder(t, db)
			keys := slices.Sorted(slices.Values(order))
			want := []string{"collections", "continue", "genres", "picks", "recent", "released", "spotlight", "watchlist"}
			if !slices.Equal(keys, want) {
				t.Errorf("home_rows keys = %v, want %v", keys, want)
			}
			if len(order) > 0 && order[0] != "spotlight" {
				t.Errorf("home_rows order %v, want spotlight first", order)
			}
			if r := slices.Index(order, "recent"); r < 0 || slices.Index(order, "released") != r+1 {
				t.Errorf("home_rows order %v, want released directly behind recent", order)
			}
		})
	}
}

// homeRowOrder returns the keys of home_rows by position and fails the test unless the positions
// are exactly 0 to n-1.
func homeRowOrder(t *testing.T, db *DB) []string {
	t.Helper()
	rows, err := db.Read.Query(`SELECT key, position FROM home_rows ORDER BY position, key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		var pos int
		if err := rows.Scan(&key, &pos); err != nil {
			t.Fatal(err)
		}
		if pos != len(keys) {
			t.Errorf("home_rows: %s on position %d, want %d (positions 0 to n-1)", key, pos, len(keys))
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}

// Dump followed by replay yields the same tables, rows and schema objects.
func TestDumpRoundTrip(t *testing.T) {
	db := openTestDB(t)
	seedTree(t, db.Write)
	addFile(t, db.Write, "01FILE0000000000000000000A", idEp1, "Show/Season 01/E1.mkv")
	mustExec(t, db.Write, `UPDATE media_files SET keyframes = X'0001ff' WHERE id = '01FILE0000000000000000000A'`)

	var buf bytes.Buffer
	if err := Dump(context.Background(), db.Read, &buf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CREATE TABLE items", "CREATE VIEW visible_items", "CREATE TRIGGER items_rating_update",
		`INSERT INTO "goose_db_version"`, "X'0001ff'", "'Show'"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("dump lacks %q", want)
		}
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), buf.String())
	again, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "t", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	for _, q := range []string{`SELECT count(*) FROM items`, `SELECT count(*) FROM visible_items`, `SELECT count(*) FROM sqlite_master`,
		`SELECT count(*) FROM search_index`, `SELECT count(*) FROM search_index WHERE search_index MATCH '"sh"*'`} {
		var a, b int
		if err := db.Read.QueryRow(q).Scan(&a); err != nil {
			t.Fatal(err)
		}
		if err := again.Read.QueryRow(q).Scan(&b); err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Errorf("%s: original %d, replayed %d", q, a, b)
		}
	}
}

func replay(t *testing.T, path, script string) {
	t.Helper()
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: url.Values{"_pragma": {"foreign_keys(0)"}}.Encode()}).String()
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(script); err != nil {
		t.Fatalf("replay: %v", err)
	}
}
