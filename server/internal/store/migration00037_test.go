package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Migration 00037 adds the partial index items_top_added on the frozen state after concept 15
// stage 3 (testdata/db/startseitenreihen.sql): the items stay as they were, and the start page
// queries over the top level read it instead of scanning every item (docs/perf/feinschliff.md
// section 3). The plans hold for any data, because the server runs without ANALYZE statistics.
func TestMigration00037TopAddedIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "startseitenreihen.sql"))
	const fingerprint = `SELECT count(*), coalesce(sum(added_at), 0) FROM items`
	raw := openRaw(t, path)
	var rowsBefore, sumBefore int64
	if err := raw.QueryRowContext(ctx, fingerprint).Scan(&rowsBefore, &sumBefore); err != nil {
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
	var rows, sum int64
	if err := db.Read.QueryRowContext(ctx, fingerprint).Scan(&rows, &sum); err != nil {
		t.Fatal(err)
	}
	if rows == 0 || rows != rowsBefore || sum != sumBefore {
		t.Errorf("items %d (was %d), sum of added_at %d (was %d); want them unchanged", rows, rowsBefore, sum, sumBefore)
	}
	var def string
	if err := db.Read.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'items_top_added'`).Scan(&def); err != nil {
		t.Fatalf("index items_top_added: %v", err)
	}
	if !strings.Contains(def, "WHERE kind IN ('movie', 'series', 'audiobook', 'podcast')") {
		t.Errorf("index items_top_added = %q, want it partial on the top-level kinds", def)
	}
	var stats int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name LIKE 'sqlite_stat%'`).Scan(&stats); err != nil {
		t.Fatal(err)
	}
	if stats != 0 {
		t.Errorf("%d statistics tables, want none (the plans below assume the planner's defaults)", stats)
	}

	for _, tc := range []struct {
		file, query string
		noSort      bool // the index gives the whole ORDER BY
	}{
		{"items.sql", "ListRecentlyAdded", true},
		{"library.sql", "ListVisibleLibraries", false},
	} {
		plan := queryPlan(t, db, scopedQuery(t, tc.file, tc.query))
		if !strings.Contains(plan, "INDEX items_top_added") {
			t.Errorf("%s does not read items_top_added:\n%s", tc.query, plan)
		}
		if tc.noSort && strings.Contains(plan, "TEMP B-TREE FOR ORDER BY") {
			t.Errorf("%s sorts although the index is in its order:\n%s", tc.query, plan)
		}
	}
}

// scopedQuery returns the statement of the named query in server/queries/scoped/<file>, from its
// "-- name:" line to the semicolon. Its parameters (@name) are SQLite's named parameters.
func scopedQuery(t *testing.T, file, name string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "queries", "scoped", file))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)-- name: ` + name + ` :\w+\n(.*?);`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("query %s not found in %s", name, file)
	}
	return string(m[1])
}

// queryPlan returns the lines of EXPLAIN QUERY PLAN for query, each named parameter bound to 1
// (the plan does not depend on the values).
func queryPlan(t *testing.T, db *DB, query string) string {
	t.Helper()
	var args []any
	for _, m := range regexp.MustCompile(`@(\w+)`).FindAllStringSubmatch(query, -1) {
		args = append(args, sql.Named(m[1], 1))
	}
	rows, err := db.Read.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		b.WriteString(detail + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
