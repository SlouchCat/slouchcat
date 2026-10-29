package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Migration 00023 (ADR 0015) over the frozen M6 state, the one with all four former kinds: the
// libraries become families, every root takes content and provider list from its library, every
// title takes its list from its kind, the feed podcast records who chose its feed, and the
// provenance table starts empty. The view and the triggers never read libraries, so the rebuild
// leaves them alone; the guards of the migration abort a state whose pairs do not fit.
func TestMigration00023OverM6(t *testing.T) {
	t.Parallel()
	script := frozenState(t, "M6.sql")
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), script)

	// Before the migration: no view and no trigger names libraries (concept 4.6 point 4).
	raw := openRaw(t, filepath.Join(cfg, FileName))
	var objects int
	// all_libraries and profile_libraries name the table only in their names; the table itself is
	// read with FROM, JOIN, UPDATE, INSERT INTO or in a subquery.
	if err := raw.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type IN ('view', 'trigger')
	    AND (sql LIKE '%FROM libraries%' OR sql LIKE '%JOIN libraries%' OR sql LIKE '%UPDATE libraries%'
	         OR sql LIKE '%INTO libraries%' OR sql LIKE '%(libraries%' OR sql LIKE '%"libraries"%')`).Scan(&objects); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	if objects != 0 {
		t.Fatalf("%d views or triggers read libraries; the rebuild would have to restore them", objects)
	}

	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatalf("migrate M6: %v", err)
	}
	defer db.Close()
	checks := map[string]string{
		`SELECT group_concat(kind, ',') FROM (SELECT kind FROM libraries ORDER BY id)`:                                          "video,video,audiobooks,podcasts",
		`SELECT group_concat(content || '/' || providers, ';') FROM (SELECT content, providers FROM library_roots ORDER BY id)`: `movie/["tmdb"];series/["tmdb","tvdb"];audiobook/["local"]`,
		`SELECT group_concat(DISTINCT providers) FROM items WHERE library_id = 3`:                                               `["local"]`,
		`SELECT providers || '/' || feed_origin FROM items WHERE kind = 'podcast'`:                                              `["local"]/admin`,
		`SELECT group_concat(DISTINCT providers) FROM items WHERE kind IN ('series', 'season', 'episode')`:                      `["tmdb","tvdb"]`,
		`SELECT group_concat(DISTINCT providers) FROM items WHERE kind IN ('movie', 'extra')`:                                   `["tmdb"]`,
		`SELECT count(*) FROM item_field_sources`:                                                                               "0",
		`SELECT count(*) FROM items WHERE anilist_id IS NOT NULL`:                                                               "0",
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                         "0",
	}
	for q, want := range checks {
		var got sql.NullString
		if err := db.Read.QueryRow(q).Scan(&got); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if got.String != want {
			t.Errorf("%s\n  = %q, want %q", q, got.String, want)
		}
	}
}

// TestMigration00023GuardAbortsAWrongPair prepares the M6 state so that a root sits in a podcast
// library, which the pre-ADR-0015 code never allowed, and expects the migration to stop before
// COMMIT (concept 4.6 points 1 and 5: the CASE without ELSE is the first guard, the CHECK table
// the second) and to leave the schema as it was.
func TestMigration00023GuardAbortsAWrongPair(t *testing.T) {
	t.Parallel()
	script := frozenState(t, "M6.sql")
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), script)
	raw := openRaw(t, filepath.Join(cfg, FileName))
	// The film library's root into the podcast library: content movie in family podcasts.
	if _, err := raw.Exec(`UPDATE library_roots SET library_id = 4 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	_, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err == nil || !strings.Contains(err.Error(), "version:23") {
		t.Fatalf("migration over a root in the wrong family: err = %v, want the guard of 00023 to abort", err)
	}
	// Nothing of 00023 was committed: the old kinds are still there.
	raw = openRaw(t, filepath.Join(cfg, FileName))
	defer raw.Close()
	var kinds string
	if err := raw.QueryRow(`SELECT group_concat(kind, ',') FROM (SELECT kind FROM libraries ORDER BY id)`).Scan(&kinds); err != nil {
		t.Fatal(err)
	}
	if kinds != "movies,series,audiobooks,podcasts" {
		t.Fatalf("kinds after the aborted run = %s, want the old ones", kinds)
	}
}

func frozenState(t *testing.T, name string) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(script)
}

func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: url.Values{"_pragma": {"foreign_keys(0)"}}.Encode()}).String()
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
