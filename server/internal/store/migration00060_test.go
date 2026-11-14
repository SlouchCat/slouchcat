package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00060 (concept 21 O-18) adds the TMDB description of a collection on the frozen state
// after 00059 (meine-medien.sql, two TMDB collections). Every collection keeps its row and its
// older overview with NULL in the new columns. Afterwards a collection takes a description with
// its language and the fetch; a description without a language (and the other way round) and an
// empty text or language are refused.
func TestMigration00060CollectionDetails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "meine-medien.sql"))
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
		`SELECT count(*) FROM collections`: 2,
		`SELECT count(*) FROM collections WHERE description IS NULL AND description_lang IS NULL
			AND details_lang IS NULL AND details_fetched_at IS NULL`: 2,
		`SELECT count(*) FROM collections WHERE overview = 'Two test films.'`:            1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM goose_db_version WHERE version_id = 60 AND is_applied = 1`: 1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE collections SET description = 'Zwei Filme.', description_lang = 'de',
		details_lang = 'de', details_fetched_at = 1 WHERE id = '01M6586A0000000000000C0001'`); err != nil {
		t.Fatalf("the description of a collection was refused: %v", err)
	}
	for what, set := range map[string]string{
		"a description without its language": `description = 'Text', description_lang = NULL`,
		"a language without a description":   `description = NULL, description_lang = 'en'`,
		"an empty description":               `description = '', description_lang = 'en'`,
		"an empty language":                  `description = 'Text', description_lang = ''`,
	} {
		if _, err := db.Write.ExecContext(ctx, `UPDATE collections SET `+set+` WHERE id = '01M6586A0000000000000C0002'`); err == nil {
			t.Errorf("%s was accepted", what)
		}
	}
}
