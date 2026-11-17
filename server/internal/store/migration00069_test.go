package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00069 (concept 25, 08-datenmodell-api.md sections 2.4 to 2.7) on the newest frozen
// state (ordner-metadaten.sql): the four wish tables come empty with their indexes; the partial
// indexes refuse a second open film wish and a second open row for the same season, a closed wish
// frees both; the checks hold (states, partial only for a show, failed with a reason, a request id
// only for a film, season 0, event kinds); the season copy of the TMDB id must match its wish; the
// seasons, requesters and events go with their wish.
func TestMigration00069Wishes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql"))
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
		`SELECT count(*) FROM goose_db_version WHERE version_id = 69 AND is_applied = 1`:                                                   1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                                    0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('wishes', 'wish_seasons', 'wish_requesters', 'wish_events')`: 4,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('wishes_open_movie', 'wishes_request', 'wishes_withdraw', 'wishes_open',
			'wishes_open_tvdb', 'wishes_due', 'wishes_admin', 'wish_seasons_open', 'wish_seasons_request', 'wish_requesters_owner', 'wish_requesters_profile')`: 11,
		`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('wish_seasons_copy_insert', 'wish_seasons_copy_update', 'wishes_title_fixed')`: 3,
		`SELECT count(*) FROM wishes`: 0,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	exec := func(query string) error {
		_, err := db.Write.ExecContext(ctx, query)
		return err
	}
	const movie = `INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES `
	for _, q := range []string{
		movie + `(1, 'movie', 1061474, 'Superman', 'slouchcat', 'queued', 1, 1)`,
		`INSERT INTO wishes (id, media, tmdb_id, tvdb_id, title, origin, state, created_at, updated_at) VALUES (2, 'show', 1399, 121361, 'Game of Thrones', 'slouchcat', 'partial', 1, 1)`,
		`INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (2, 1, 1399, 'available')`,
		`INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (2, 2, 1399, 'processing')`,
		`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) SELECT 1, id, account_id, 'owner', 1 FROM profiles ORDER BY id LIMIT 1`,
		`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) SELECT 2, id, account_id, 'owner', 1 FROM profiles ORDER BY id LIMIT 1`,
		`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) SELECT 2, id, account_id, 'joined', 2 FROM profiles ORDER BY id LIMIT 1 OFFSET 1`,
		`INSERT INTO wish_events (wish_id, at, kind, detail) VALUES (2, 1, 'state_changed', '{"from":"pending","to":"processing"}')`,
		// A closed film wish does not count: the same film may be wished again after a decline.
		movie + `(3, 'movie', 550, 'Fight Club', 'slouchcat', 'declined', 1, 1)`,
		movie + `(4, 'movie', 550, 'Fight Club', 'slouchcat', 'pending', 1, 1)`,
		`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, fail_reason, created_at, updated_at) VALUES (5, 'movie', 551, 'X', 'slouchcat', 'failed', 'quota', 1, 1)`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for name, q := range map[string]string{
		"second open film wish":   movie + `(10, 'movie', 1061474, 'Superman', 'slouchcat', 'processing', 1, 1)`,
		"unknown state":           movie + `(11, 'movie', 12, 'X', 'slouchcat', 'requested', 1, 1)`,
		"unknown origin":          `INSERT INTO wishes (media, tmdb_id, title, origin, state, created_at, updated_at) VALUES ('movie', 12, 'X', 'seerr', 'queued', 1, 1)`,
		"partial film":            movie + `(12, 'movie', 13, 'X', 'slouchcat', 'partial', 1, 1)`,
		"failed without a reason": movie + `(13, 'movie', 14, 'X', 'slouchcat', 'failed', 1, 1)`,
		"reason without failed":   `INSERT INTO wishes (media, tmdb_id, title, origin, state, fail_reason, created_at, updated_at) VALUES ('movie', 15, 'X', 'slouchcat', 'pending', 'quota', 1, 1)`,
		"unknown fail reason":     `INSERT INTO wishes (media, tmdb_id, title, origin, state, fail_reason, created_at, updated_at) VALUES ('movie', 16, 'X', 'slouchcat', 'failed', 'gone', 1, 1)`,
		"request id of a show":    `INSERT INTO wishes (media, tmdb_id, title, origin, state, seerr_request_id, created_at, updated_at) VALUES ('show', 17, 'X', 'slouchcat', 'pending', 7, 1, 1)`,
		"tmdb id 0":               movie + `(14, 'movie', 0, 'X', 'slouchcat', 'queued', 1, 1)`,
		"second open season":      `INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (20, 'show', 1399, 'Game of Thrones', 'slouchcat', 'pending', 1, 1); INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (20, 2, 1399, 'pending')`,
		"season 0":                `INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (2, 0, 1399, 'queued')`,
		"partial season":          `INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (2, 3, 1399, 'partial')`,
		"season copy differs":     `INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (2, 4, 1400, 'queued')`,
		"season of a film wish":   `INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (1, 1, 1061474, 'queued')`,
		"season copy moved":       `UPDATE wish_seasons SET tmdb_id = 1400 WHERE wish_id = 2 AND season_number = 1`,
		"title of a series moved": `UPDATE wishes SET tmdb_id = 1400 WHERE id = 2`,
		"second owner":            `INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) SELECT 1, id, account_id, 'owner', 1 FROM profiles ORDER BY id LIMIT 1 OFFSET 1`,
		"unknown role":            `INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) SELECT 4, id, account_id, 'guest', 1 FROM profiles ORDER BY id LIMIT 1`,
		"event kind with capital": `INSERT INTO wish_events (wish_id, at, kind) VALUES (1, 1, 'Created')`,
		"event kind empty":        `INSERT INTO wish_events (wish_id, at, kind) VALUES (1, 1, '')`,
		"event detail not JSON":   `INSERT INTO wish_events (wish_id, at, kind, detail) VALUES (1, 1, 'created', '{')`,
	} {
		tx, err := db.Write.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.ExecContext(ctx, q)
		_ = tx.Rollback()
		if err == nil {
			t.Errorf("%s: statement succeeded, want a constraint error", name)
		}
	}

	// A season row that is no longer open frees the season for a new wish.
	for _, q := range []string{
		`UPDATE wish_seasons SET state = 'declined' WHERE wish_id = 2 AND season_number = 2`,
		`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (21, 'show', 1399, 'Game of Thrones', 'slouchcat', 'pending', 1, 1)`,
		`INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (21, 2, 1399, 'pending')`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := exec(`DELETE FROM wishes WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT (SELECT count(*) FROM wish_seasons WHERE wish_id = 2) + (SELECT count(*) FROM wish_requesters WHERE wish_id = 2) + (SELECT count(*) FROM wish_events WHERE wish_id = 2)`); got != 0 {
		t.Errorf("seasons, requesters and events after their wish went: %d rows, want 0", got)
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
