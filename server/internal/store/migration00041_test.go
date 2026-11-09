package store

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// Migration 00041 (concept 18 part B, ADR 0028) on the frozen state of migration 00040, which still
// carries a pick of the administration: top_picks and its indexes are gone, user_picks is empty,
// STRICT and WITHOUT ROWID, picks_config holds the one row with 60 days, its CHECKs refuse every
// value outside 7..365 and a second row, and deleting a title or a profile takes its picks along.
func TestMigration00041UserPicks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	script := frozenState(t, "detailseiten3.sql")
	if !strings.Contains(script, `INSERT INTO "top_picks"`) {
		t.Fatal("the frozen state of 00040 holds no pick of the administration to discard")
	}
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, script)
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for query, want := range map[string]int{
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                    0,
		`SELECT count(*) FROM sqlite_master WHERE name LIKE 'top_picks%'`:                                                  0,
		`SELECT count(*) FROM user_picks`:                                                                                  0,
		`SELECT count(*) FROM picks_config`:                                                                                1,
		`SELECT count(*) FROM picks_config WHERE id = 1 AND retention_days = 60`:                                           1,
		`SELECT count(*) FROM pragma_table_list WHERE name = 'user_picks' AND strict = 1 AND wr = 1`:                       1,
		`SELECT count(*) FROM pragma_table_list WHERE name = 'picks_config' AND strict = 1`:                                1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('user_picks_created', 'user_picks_profile')`: 2,
	} {
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil || n != want {
			t.Errorf("%s = %d (%v), want %d", query, n, err, want)
		}
	}

	var movie, other string
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM items WHERE kind = 'movie' AND deleted_at IS NULL ORDER BY id LIMIT 1`).Scan(&movie); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM items WHERE kind = 'movie' AND deleted_at IS NULL AND id <> ? ORDER BY id LIMIT 1`, movie).Scan(&other); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db.Write, `INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (?, 1, 10), (?, 2, 20), (?, 2, 30)`, movie, movie, other)
	for _, bad := range []string{
		`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES ('` + movie + `', 1, 11)`,             // (title, profile) twice
		`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES ('01M6586A0000000000000X0001', 1, 1)`, // unknown title
		`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES ('` + movie + `', 999, 1)`,            // unknown profile
		`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES ('` + other + `', 1, NULL)`,           // without a time
		`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES ('` + other + `', 1, 'now')`,          // STRICT: text in an integer column
		`INSERT INTO picks_config (id, retention_days, updated_at) VALUES (2, 60, 0)`,                          // a second row
		`UPDATE picks_config SET retention_days = 6`,
		`UPDATE picks_config SET retention_days = 366`,
		`UPDATE picks_config SET retention_days = 'sixty'`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: accepted, want a constraint error", bad)
		}
	}
	for _, good := range []string{`UPDATE picks_config SET retention_days = 7`, `UPDATE picks_config SET retention_days = 365`} {
		if _, err := db.Write.ExecContext(ctx, good); err != nil {
			t.Errorf("%s: %v", good, err)
		}
	}

	// Deleting a title or a profile takes its picks along.
	mustExec(t, db.Write, `DELETE FROM items WHERE id = ?`, other)
	mustExec(t, db.Write, `DELETE FROM profiles WHERE id = 2`)
	var left int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM user_picks`).Scan(&left); err != nil || left != 1 {
		t.Errorf("user_picks after the deletes = %d (%v), want the one row of profile 1", left, err)
	}
}
