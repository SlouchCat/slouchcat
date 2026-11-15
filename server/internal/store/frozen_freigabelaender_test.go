package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state before the migrations of concept 22 stage 3 (testdata/db/freigabelaender.sql)
// is sprachgeruest.sql after 00061 with ratings of every source a title can carry (TMDB from DE,
// US and GB, TVDB from US, by hand with the lock, NFO, a lock without a manual value, unrated), a
// library default rating, a guest account with ceiling 12 and its own libraries, a used and an
// open invitation and the rating country CH. TestFrozenStatesMigrate replays it like every state;
// this test checks that each of those is really in it.
func TestFrozenFreigabelaenderStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "freigabelaender.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"rating from TMDB in Germany":           `SELECT count(*) FROM items WHERE rating_source = 'tmdb:DE' AND own_rating = 12`,
		"rating from TMDB in the US":            `SELECT count(*) FROM items WHERE rating_source = 'tmdb:US' AND own_rating = 16`,
		"rating from TMDB in Great Britain":     `SELECT count(*) FROM items WHERE rating_source = 'tmdb:GB'`,
		"rating from TVDB":                      `SELECT count(*) FROM items WHERE rating_source = 'tvdb:US'`,
		"rating set by hand with the lock":      `SELECT count(*) FROM items WHERE rating_source = 'manual' AND locked_fields LIKE '%own_rating%'`,
		"rating from an NFO":                    `SELECT count(*) FROM items WHERE rating_source = 'nfo'`,
		"provider rating under a lock":          `SELECT count(*) FROM items WHERE rating_source LIKE 'tmdb:%' AND locked_fields LIKE '%own_rating%'`,
		"unrated film":                          `SELECT count(*) FROM items WHERE kind = 'movie' AND own_rating IS NULL`,
		"derived ratings of a series":           `SELECT count(*) FROM items WHERE kind = 'season' AND effective_rating = 16`,
		"library default rating":                `SELECT count(*) FROM libraries WHERE default_rating = 6`,
		"guest account below 18":                `SELECT count(*) FROM accounts WHERE role = 'user' AND max_rating = 12 AND all_libraries = 0`,
		"guest account libraries":               `SELECT count(*) FROM account_libraries`,
		"used invitation":                       `SELECT count(*) FROM invitations WHERE used_by IS NOT NULL`,
		"open invitation":                       `SELECT count(*) FROM invitations WHERE used_at IS NULL AND max_rating = 6`,
		"invitation libraries":                  `SELECT count(*) FROM invitation_libraries`,
		"activity event of the guest":           `SELECT count(*) FROM activity_events WHERE account_id = 2`,
		"rating country CH":                     `SELECT count(*) FROM settings WHERE key = 'metadata.rating_country' AND value = 'CH'`,
		"migration 00061 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 61 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen freigabelaender state has no %s", what)
		}
	}
}
