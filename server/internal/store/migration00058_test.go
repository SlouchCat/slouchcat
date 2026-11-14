package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00058 (concept 21 P-8) adds the facts of the person page to people on the frozen state
// after 00057 (trailer-aufloesung.sql, three people). Every person keeps its row with NULL facts.
// Afterwards a person takes a biography with its language, the dates, the place, the department
// and the count; a biography without a language (and the other way round), an empty text, a date
// that is no YYYY-MM-DD and a negative count are refused.
func TestMigration00058PersonDetails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "trailer-aufloesung.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM people`: 3,
		`SELECT count(*) FROM people WHERE biography IS NULL AND biography_lang IS NULL AND birthday IS NULL
			AND deathday IS NULL AND place_of_birth IS NULL AND known_for_department IS NULL AND total_credits IS NULL
			AND details_lang IS NULL AND details_fetched_at IS NULL`: 3,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM goose_db_version WHERE version_id = 58 AND is_applied = 1`: 1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE people SET biography = 'Bio', biography_lang = 'en', details_lang = 'de',
		birthday = '1964-04-25', deathday = '2020-01-02', place_of_birth = 'Queens', known_for_department = 'Acting',
		total_credits = 0, details_fetched_at = 1 WHERE id = 1`); err != nil {
		t.Fatalf("the facts of a person were refused: %v", err)
	}
	for what, set := range map[string]string{
		"a biography without its language": `biography = 'Bio', biography_lang = NULL`,
		"a language without a biography":   `biography = NULL, biography_lang = 'en'`,
		"an empty biography":               `biography = '', biography_lang = 'en'`,
		"an empty language":                `biography = 'Bio', biography_lang = ''`,
		"a birthday that is no day":        `birthday = '25.04.1964'`,
		"a deathday that is no day":        `deathday = '2020'`,
		"an empty place of birth":          `place_of_birth = ''`,
		"an empty department":              `known_for_department = ''`,
		"a negative count":                 `total_credits = -1`,
	} {
		if _, err := db.Write.ExecContext(ctx, `UPDATE people SET `+set+` WHERE id = 2`); err == nil {
			t.Errorf("%s was accepted", what)
		}
	}
}
