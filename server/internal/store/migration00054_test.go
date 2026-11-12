package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

// Migration 00054 (concept 20 P-2, contract 1.31.0) moves the favourites onto the watchlist and
// drops the table favorites. From the frozen state of concept 19 stage B2 (radio-wikidata.sql) with
// favourites and watchlist entries added in the three shapes of the paper (3.3):
//   - a favourite whose title is on the watchlist already: one entry, created_at of the watchlist,
//     follow_since of the favourite;
//   - a favourite alone: a new entry with created_at = follow_since = the favourite's date;
//   - a watchlist entry without favourite (a podcast on the list for a year): follow_since is the
//     moment of the migration, so none of its old episodes counts as new for the offline rule.
func TestMigration00054WatchlistTakesOverFavorites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "radio-wikidata.sql"))
	raw := openRaw(t, path)
	ids := func(query string) []string {
		t.Helper()
		rows, err := raw.Query(query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			out = append(out, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	series := ids(`SELECT id FROM items WHERE kind = 'series' ORDER BY id`)
	// The podcast with the most episodes (the frozen state has two podcasts, one with episodes).
	podcasts := ids(`SELECT p.id FROM items AS p WHERE p.kind = 'podcast'
	                 ORDER BY (SELECT count(*) FROM items AS e WHERE e.parent_id = p.id) DESC, p.id`)
	movies := ids(`SELECT id FROM items WHERE kind = 'movie' ORDER BY id`)
	if len(series) < 2 || len(podcasts) < 1 || len(movies) < 1 {
		t.Fatalf("frozen state lacks titles: series %v, podcasts %v, movies %v", series, podcasts, movies)
	}
	yearAgo := time.Date(2025, 11, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM watchlist`, nil},
		{`DELETE FROM favorites`, nil},
		// Profile 1: series 0 on the watchlist (100) and favourite (200); series 1 favourite only (300);
		// a film on the watchlist only (400).
		{`INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (1, ?, 100), (1, ?, 400)`, []any{series[0], movies[0]}},
		{`INSERT INTO favorites (profile_id, item_id, created_at) VALUES (1, ?, 200), (1, ?, 300)`, []any{series[0], series[1]}},
		// Profile 2: the podcast on the watchlist for a year, no favourite.
		{`INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (2, ?, ?)`, []any{podcasts[0], yearAgo}},
		{`UPDATE items SET added_at = ? WHERE parent_id = ? AND kind = 'podcast_episode'`, []any{yearAgo + 1000, podcasts[0]}},
	} {
		if _, err := raw.Exec(q.sql, q.args...); err != nil {
			t.Fatalf("%s: %v", q.sql, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UnixMilli()
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	after := time.Now().UnixMilli()

	var tables int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name IN ('favorites', 'favorites_item')`).Scan(&tables); err != nil || tables != 0 {
		t.Errorf("favorites and its index after 00054: %d (%v), want gone", tables, err)
	}
	var index int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'watchlist_item'`).Scan(&index); err != nil || index != 1 {
		t.Errorf("index watchlist_item after 00054: %d (%v)", index, err)
	}
	type entry struct{ created, since int64 }
	got := map[string]entry{}
	rows, err := db.Read.QueryContext(ctx, `SELECT profile_id || ':' || item_id, created_at, follow_since FROM watchlist`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var e entry
		if err := rows.Scan(&key, &e.created, &e.since); err != nil {
			t.Fatal(err)
		}
		got[key] = e
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Errorf("watchlist after 00054 = %v, want 4 entries without duplicates", got)
	}
	for key, want := range map[string]entry{
		"1:" + series[0]: {100, 200}, // on the list before: created_at stays, follow_since of the favourite
		"1:" + series[1]: {300, 300}, // favourite alone
	} {
		if got[key] != want {
			t.Errorf("entry %s = %+v, want %+v", key, got[key], want)
		}
	}
	for _, key := range []string{"1:" + movies[0], "2:" + podcasts[0]} {
		e, ok := got[key]
		if !ok || e.since < before || e.since > after {
			t.Errorf("entry %s without favourite = %+v (%v), want follow_since in [%d, %d]", key, e, ok, before, after)
		}
	}
	if e := got["2:"+podcasts[0]]; e.created != yearAgo {
		t.Errorf("podcast entry created_at = %d, want %d", e.created, yearAgo)
	}
	// No backfill: every episode of the podcast is older than follow_since, so the offline rule
	// (ListFollowedPodcasts: added_at <= follow_since) counts none of them as new.
	var episodes, newer int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*), coalesce(sum(e.added_at > w.follow_since), 0)
	                                        FROM items AS e JOIN watchlist AS w ON w.item_id = e.parent_id AND w.profile_id = 2
	                                        WHERE e.kind = 'podcast_episode'`).Scan(&episodes, &newer); err != nil || episodes == 0 || newer != 0 {
		t.Errorf("podcast episodes newer than follow_since = %d of %d (%v), want 0 of at least 1", newer, episodes, err)
	}
	var applied int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id = 54 AND is_applied = 1`).Scan(&applied); err != nil || applied != 1 {
		t.Errorf("migration 00054 recorded %d times (%v)", applied, err)
	}
}
