package library

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// recordingDB passes the generated queries through to the database and keeps the last statement
// with its arguments, so the test can ask SQLite for the plan of exactly the statement sqlc sends.
type recordingDB struct {
	systemq.DBTX
	query string
	args  []any
}

func (r *recordingDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	r.query, r.args = query, args
	return r.DBTX.QueryContext(ctx, query, args...)
}

func (r *recordingDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	r.query, r.args = query, args
	return r.DBTX.QueryRowContext(ctx, query, args...)
}

// The relink lookups find twins by provider id through items_tmdb, items_tvdb and items_imdb
// whatever the planner's statistics say (queries/system/relink.sql). With an OR over the three ids,
// ANALYZE statistics of a library without TVDB ids led SQLite to a skip-scan of items_library_kind
// or a full scan of items for every title: 11 s instead of 10 ms for 2000 fresh copies among 72k
// titles, inside the transaction of a folder removal. The test fills a library of that shape at a
// tenth of the size (the plans flip already), runs ANALYZE and checks that every access to the
// twin side goes through a provider id index or the primary key, before and after ANALYZE.
func TestRelinkLookupsUseProviderIndexes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	const (
		now     = int64(1_000_000_000_000)
		pairs   = 100  // titles deleted at now, each with a fresh copy
		deleted = 1500 // older soft-deleted items
		live    = 2000 // other live items
	)
	e.exec(`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'A', 'video', 0, 0), (2, 'B', 'video', 0, 0), (3, 'C', 'video', 0, 0)`)
	tx, err := e.db.Write.BeginTx(e.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	ins, err := tx.PrepareContext(e.ctx, `INSERT INTO items (id, kind, library_id, title, sort_title, tmdb_id, imdb_id,
		added_at, created_at, updated_at, deleted_at) VALUES (?, ?, ?, 't', 't', ?, ?, 0, ?, 0, ?)`)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	// add inserts an item with a TMDB and an IMDb id and no TVDB id, like the films of a library.
	add := func(kind string, lib, tmdb, created int64, deletedAt sql.NullInt64) string {
		n++
		id := fmt.Sprintf("%026d", n)
		if _, err := ins.ExecContext(e.ctx, id, kind, lib, tmdb, fmt.Sprintf("tt%d", tmdb), created, deletedAt); err != nil {
			t.Fatal(err)
		}
		return id
	}
	kinds := []string{"movie", "series", "season", "episode", "episode"}
	for i := range deleted {
		add(kinds[i%len(kinds)], int64(i%3+1), int64(1+i*7%1_000_000), 1, sql.NullInt64{Int64: int64(1 + i), Valid: true})
	}
	for i := range live {
		add(kinds[i%len(kinds)], int64(i%3+1), int64(1+(i*13+5)%1_000_000), 1, sql.NullInt64{})
	}
	var copyID string
	for i := range pairs {
		tmdb := int64(2_000_000 + i)
		add("movie", 1, tmdb, 1, sql.NullInt64{Int64: now, Valid: true})
		copyID = add("movie", 1, tmdb, now-1000, sql.NullInt64{})
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	rec := &recordingDB{DBTX: e.db.Read}
	q := systemq.New(rec)
	grace := relinkGrace.Milliseconds()
	lookups := []struct {
		name string
		twin string // the alias of the side looked up by provider id
		run  func() (hits int, want int, err error)
	}{
		{"ListFreshCopiesOfDeleted", "l", func() (int, int, error) {
			ids, err := q.ListFreshCopiesOfDeleted(e.ctx, systemq.ListFreshCopiesOfDeletedParams{Now: sql.NullInt64{Int64: now, Valid: true}, GraceMs: grace})
			return len(ids), pairs, err
		}},
		{"FindDeletedTwin", "d", func() (int, int, error) {
			_, err := q.FindDeletedTwin(e.ctx, systemq.FindDeletedTwinParams{LiveID: copyID, GraceMs: sql.NullInt64{Int64: grace, Valid: true}})
			return 1, 1, err
		}},
		{"ListRelinkPairs", "d", func() (int, int, error) {
			rows, err := q.ListRelinkPairs(e.ctx, 1)
			return min(len(rows), pairs), pairs, err
		}},
		{"ListRelinkChainTwins", "d", func() (int, int, error) {
			ids, err := q.ListRelinkChainTwins(e.ctx, systemq.ListRelinkChainTwinsParams{
				LibraryID: 1, Kind: "movie", TmdbID: sql.NullInt64{Int64: 2_000_000, Valid: true},
				ImdbID: sql.NullString{String: "tt2000000", Valid: true},
			})
			return len(ids), 1, err
		}},
	}
	byKey := regexp.MustCompile(`USING (COVERING )?INDEX (items_tmdb|items_tvdb|items_imdb|sqlite_autoindex_items_1) \(`)
	for _, analyzed := range []bool{false, true} {
		if analyzed {
			e.exec(`ANALYZE`)
		}
		for _, l := range lookups {
			hits, want, err := l.run()
			if err != nil {
				t.Fatalf("%s: %v", l.name, err)
			}
			if hits != want {
				t.Errorf("%s (analyzed %v): %d hits, want %d", l.name, analyzed, hits, want)
			}
			plan := e.plan(rec.query, rec.args)
			twin := regexp.MustCompile(`^(SCAN|SEARCH) ` + l.twin + `\b`)
			for _, line := range plan {
				if twin.MatchString(line) && !byKey.MatchString(line) {
					t.Errorf("%s (analyzed %v) walks the twin side: %q\n%s", l.name, analyzed, line, strings.Join(plan, "\n"))
				}
			}
			for _, idx := range []string{"items_tmdb", "items_tvdb", "items_imdb"} {
				if !slices.ContainsFunc(plan, func(line string) bool { return strings.Contains(line, "INDEX "+idx+" (") }) {
					t.Errorf("%s (analyzed %v) does not use %s:\n%s", l.name, analyzed, idx, strings.Join(plan, "\n"))
				}
			}
		}
	}
}

// plan returns the detail lines of EXPLAIN QUERY PLAN for query.
func (e *env) plan(query string, args []any) []string {
	e.t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, detail)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}
