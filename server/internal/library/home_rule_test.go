package library

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The home-folder rule in Go (ListTitlePresentRoots, which rehome reads) and in migration 00065
// agree (ADR 0036): oldest present file first, ties by path.

// Two copies with the same created_at: the path decides, whichever folder is first.
func TestHomeFolderTieBreakByPath(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, a, b, film, _, _ := e.twoMovieFolders(nil)
	e.exec(`UPDATE media_files SET created_at = 1 WHERE item_id = ?`, film)
	q := systemq.New(e.svc.db.Read)
	for _, c := range []struct {
		pathA string
		want  int64
	}{{"/aa/first", a}, {"/zz/first", b}} {
		e.exec(`UPDATE library_roots SET path = ? WHERE id = ?`, c.pathA, a)
		roots, err := q.ListTitlePresentRoots(e.ctx, film)
		if err != nil {
			t.Fatal(err)
		}
		if len(roots) != 2 || roots[0] != c.want {
			t.Errorf("first folder A at %s: roots = %v, want %d first", c.pathA, roots, c.want)
		}
	}
}

// On the frozen state before migration 00065 the migration writes, for every top-level title
// with a present file, the folder the Go rule puts first.
func TestHomeFolderOfTheMigrationAgreesWithTheRule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "ordner-metadaten.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	// synchronous(OFF): one fsync-bound transaction per line of the dump otherwise (replay in
	// internal/store/frozen_test.go has the measurement).
	dsn := (&url.URL{Scheme: "file", Path: filepath.Join(cfg, store.FileName), RawQuery: url.Values{"_pragma": {"foreign_keys(0)", "synchronous(OFF)"}}.Encode()}).String()
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(string(script)); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, store.Options{ConfigDir: cfg, Version: "home-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	type title struct {
		id   string
		root int64
	}
	var titles []title
	func() {
		rows, err := db.Read.QueryContext(ctx, `SELECT id, coalesce(root_id, 0) FROM items WHERE parent_id IS NULL ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var x title
			if err := rows.Scan(&x.id, &x.root); err != nil {
				t.Fatal(err)
			}
			titles = append(titles, x)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}()
	q := systemq.New(db.Read)
	checked := 0
	for _, x := range titles {
		roots, err := q.ListTitlePresentRoots(ctx, x.id)
		if err != nil {
			t.Fatal(err)
		}
		if len(roots) == 0 {
			continue
		}
		checked++
		if roots[0] != x.root {
			t.Errorf("title %s: migration wrote folder %d, the rule puts %d first", x.id, x.root, roots[0])
		}
	}
	if checked < 5 {
		t.Fatalf("only %d titles with a present file in the frozen state", checked)
	}
}
