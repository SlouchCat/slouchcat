package library

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

// Measurement M-1 of concept 20 (section 9.3, docs/perf/20-bedienung.md): how long the delete of
// a library with 10 000 titles takes, when the foreign keys clear everything that hangs on its
// titles and the search triggers clear their documents.
//
//	go test -run '^$' -bench DeleteLibrary -benchtime 1x -timeout 30m -v ./internal/library/
//
// Each run seeds two libraries of the same shape, so the tables without an index on the
// child column (progress, playback_sessions, downloads.media_file_id) hold rows of the other
// library as well, and then deletes one of them in one transaction, as deleteLibrary does.
func BenchmarkDeleteLibrary(b *testing.B) {
	for _, shape := range []string{"films", "series"} {
		b.Run(shape, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				db := seedDeleteBench(b, shape)
				b.StartTimer()
				start := time.Now()
				deleteBenchLibrary(b, db, 1)
				b.ReportMetric(float64(time.Since(start).Milliseconds()), "ms/delete")
				b.StopTimer()
				_ = db.Close()
				b.StartTimer()
			}
		})
	}
}

// deleteBenchLibrary is the transaction of deleteLibrary in its plain form: the grants of the
// profiles, the library row and with it, through the foreign keys, every title and file.
func deleteBenchLibrary(b *testing.B, db *store.DB, id int64) {
	b.Helper()
	ctx := context.Background()
	tx, err := db.Write.BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range []string{
		`DELETE FROM profile_libraries WHERE library_id = ?`,
		`UPDATE activity_events SET library_id = NULL WHERE library_id = ?`,
		`DELETE FROM libraries WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			b.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	var left int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM items WHERE library_id = ?`, id).Scan(&left); err != nil || left != 0 {
		b.Fatalf("items left = %d, %v", left, err)
	}
}

// seedDeleteBench builds two libraries of 10 000 titles each: films with one file, three tracks,
// two texts, four image rows, six credits and a trailer each; or 400 series with 25 episodes
// (the episodes are the titles). Three profiles have progress on every third title with ten
// progress events and two playback sessions each, a watchlist entry on every tenth, and 500
// TMDB collections hold two films each.
func seedDeleteBench(b *testing.B, shape string) *store.DB {
	b.Helper()
	ctx := context.Background()
	db, err := storetest.Open(ctx, store.Options{ConfigDir: b.TempDir(), Version: "bench", Logger: discard})
	if err != nil {
		b.Fatal(err)
	}
	tx, err := db.Write.BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	// One prepared statement per query text: about 1.5 million inserts per seed.
	stmts := map[string]*sql.Stmt{}
	exec := func(query string, args ...any) {
		st := stmts[query]
		if st == nil {
			var err error
			if st, err = tx.PrepareContext(ctx, query); err != nil {
				b.Fatalf("%s: %v", query, err)
			}
			stmts[query] = st
		}
		if _, err := st.ExecContext(ctx, args...); err != nil {
			b.Fatalf("%s: %v", query, err)
		}
	}
	const now = 1792000000000
	exec(`INSERT INTO accounts (id, email, password_hash, role, is_owner, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 'admin', 1, ?, ?)`, now, now)
	for p := 1; p <= 3; p++ {
		exec(`INSERT INTO profiles (id, account_id, name, is_default, created_at, updated_at) VALUES (?, 1, ?, ?, ?, ?)`, p, fmt.Sprintf("P%d", p), boolInt(p == 1), now, now)
	}
	exec(`INSERT INTO sessions (id, token_hash, account_id, profile_id, created_at, last_seen_at, expires_at) VALUES (1, 'h', 1, 1, ?, ?, ?)`, now, now, now)
	for p := 1; p <= 20000; p++ {
		exec(`INSERT INTO people (id, tmdb_id, name, created_at) VALUES (?, ?, ?, ?)`, p, p, fmt.Sprintf("Person %d", p), now)
	}
	n := 0
	id := func() string { n++; return fmt.Sprintf("01M%023d", n) }
	for lib := int64(1); lib <= 2; lib++ {
		exec(`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (?, ?, 'video', ?, ?)`, lib, fmt.Sprintf("Bibliothek %d", lib), now, now)
		exec(`INSERT INTO library_roots (id, library_id, path, created_at, content) VALUES (?, ?, ?, ?, ?)`, lib, lib, fmt.Sprintf("/media/bench%d", lib), now, map[string]string{"films": "movie", "series": "series"}[shape])
		exec(`INSERT INTO profile_libraries (profile_id, library_id) VALUES (3, ?)`, lib)
		title := func(kind string, parent sql.NullString, k int) string {
			it := id()
			exec(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, tmdb_id, available, added_at, created_at, updated_at)
			      VALUES (?, ?, ?, ?, ?, ?, '["Drama","Krimi"]', ?, 1, ?, ?, ?)`, it, kind, parent, lib, fmt.Sprintf("Titel %d %d", lib, k), fmt.Sprintf("titel %06d", k), int64(lib)*1000000+int64(k), now, now, now)
			for _, lang := range []string{"de", "en"} {
				exec(`INSERT INTO item_texts (item_id, lang, title, overview) VALUES (?, ?, ?, 'Eine Beschreibung')`, it, lang, fmt.Sprintf("Titel %s %d", lang, k))
			}
			return it
		}
		leaf := func(it string, k int) {
			mf := id()
			exec(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at) VALUES (?, ?, ?, ?, 1000, 1, 'mkv', ?, ?)`, mf, it, lib, fmt.Sprintf("t%06d/t.mkv", k), now, now)
			for s := range 3 {
				exec(`INSERT INTO tracks (media_file_id, stream_index, kind) VALUES (?, ?, ?)`, mf, s, []string{"video", "audio", "subtitle"}[s])
			}
			if k%3 == 0 {
				for p := 1; p <= 3; p++ {
					exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, 5000, 0, ?)`, p, it, now)
					for e := range 10 {
						exec(`INSERT INTO progress_events (profile_id, item_id, media_file_id, position_ms, duration_ms, played_ms, seq, state, client_time, server_time) VALUES (?, ?, ?, 1, 5000, 1, ?, 'playing', ?, ?)`, p, it, mf, e, now, now)
					}
					for range 2 {
						exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at, revoked_at) VALUES (?, 1, 1, ?, ?, ?, 'direct', ?, ?, ?, ?)`, id(), p, it, mf, now, now, now, now)
					}
				}
			}
			if k%10 == 0 {
				exec(`INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (1, ?, ?)`, it, now)
			}
		}
		decorate := func(it string, k int) {
			for i, kind := range []string{"poster", "backdrop"} {
				exec(`INSERT INTO images (item_id, kind, size, provider, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at) VALUES (?, ?, 'w500', 'tmdb', 'u', 'p', 'h', 'image/jpeg', 1, 1, 1, ?), (?, ?, 'original', 'tmdb', 'u', 'p', 'h', 'image/jpeg', 1, 1, 1, ?)`, it, kind, now, it, kind, now)
				exec(`INSERT INTO item_images (item_id, kind, provider, source, chosen, updated_at) VALUES (?, ?, 'tmdb', ?, 1, ?)`, it, kind, fmt.Sprint(i), now)
			}
			for c := range 6 {
				exec(`INSERT INTO credits (item_id, person_id, role, ord) VALUES (?, ?, 'cast', ?)`, it, (k*6+c)%20000+1, c)
			}
			exec(`INSERT INTO trailers (item_id, lang, youtube_id, source, created_at) VALUES (?, 'de', 'abcdefghijk', 'tmdb', ?)`, it, now)
		}
		switch shape {
		case "films":
			var prev string
			for k := range 10000 {
				it := title("movie", sql.NullString{}, k)
				decorate(it, k)
				leaf(it, k)
				if lib == 1 && k%20 == 1 {
					col := id()
					exec(`INSERT INTO collections (id, source, tmdb_id, name, created_at, updated_at) VALUES (?, 'tmdb', ?, ?, ?, ?)`, col, k, fmt.Sprintf("Reihe %d", k), now, now)
					exec(`INSERT INTO collection_items (collection_id, item_id, created_at) VALUES (?, ?, ?), (?, ?, ?)`, col, prev, now, col, it, now)
				}
				prev = it
			}
		case "series":
			k := 0
			for s := range 400 {
				series := title("series", sql.NullString{}, 100000+s)
				decorate(series, s)
				season := title("season", sql.NullString{String: series, Valid: true}, 200000+s)
				for range 25 {
					ep := title("episode", sql.NullString{String: season, Valid: true}, k)
					leaf(ep, k)
					k++
				}
			}
		}
	}
	for _, st := range stmts {
		_ = st.Close()
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `ANALYZE`); err != nil {
		b.Fatal(err)
	}
	return db
}
