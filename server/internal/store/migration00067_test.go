package store

import (
	"context"
	"testing"
)

// Migration 00067 (concept 25, data model 2.3) on the frozen state before concept 25
// (ordner-metadaten.sql: owner account 1 with the primary profile 1 and the profiles 2 and 3,
// account 2 with the primary profile 4, two invitations): the owner account and the primary
// profiles may request, every other account, profile and invitation may not; the invitations get
// the default limit, and the CHECKs of the new columns hold. The state is migrated up to 00067
// only: migration 00086 (ADR 0045) replaces this backfill by one that follows the ratings
// (TestMigration00086RequestRightFollowsRating).
func TestMigration00067WishesRights(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql"))
	opts := testOptions(t, cfg)
	opts.migrations = migrationsBefore(t, "00068")
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 67 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		// Backfill.
		`SELECT count(*) FROM accounts WHERE id = 1 AND is_owner = 1 AND can_request = 1`:         1,
		`SELECT count(*) FROM accounts WHERE id <> 1 AND can_request = 0`:                         1,
		`SELECT count(*) FROM accounts WHERE can_request = 1`:                                     1,
		`SELECT count(*) FROM profiles WHERE id IN (1, 4) AND is_default = 1 AND can_request = 1`: 2,
		`SELECT count(*) FROM profiles WHERE id IN (2, 3) AND can_request = 0`:                    2,
		`SELECT count(*) FROM profiles WHERE can_request = 1`:                                     2,
		`SELECT count(*) FROM invitations WHERE can_request = 0 AND wish_limit_mode = 'default'
			AND wish_movie_limit IS NULL AND wish_movie_days IS NULL AND wish_tv_limit IS NULL AND wish_tv_days IS NULL`: 2,
		// The view stays (no rebuild of accounts and profiles).
		`SELECT count(*) FROM sqlite_master WHERE type = 'view' AND name = 'visible_items'`: 1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	for _, bad := range []string{
		`UPDATE accounts SET can_request = 2 WHERE id = 1`,
		`UPDATE profiles SET can_request = -1 WHERE id = 1`,
		`UPDATE invitations SET can_request = 3 WHERE id = 1`,
		`UPDATE invitations SET wish_limit_mode = 'weekly' WHERE id = 1`,
		// custom needs all four numbers, the other modes none
		`UPDATE invitations SET wish_limit_mode = 'custom' WHERE id = 1`,
		`UPDATE invitations SET wish_limit_mode = 'custom', wish_movie_limit = 10, wish_movie_days = 7, wish_tv_limit = 5 WHERE id = 1`,
		`UPDATE invitations SET wish_limit_mode = 'unlimited', wish_movie_limit = 10 WHERE id = 1`,
		`UPDATE invitations SET wish_movie_limit = 10 WHERE id = 1`,
		// ranges
		`UPDATE invitations SET wish_limit_mode = 'custom', wish_movie_limit = 101, wish_movie_days = 7, wish_tv_limit = 5, wish_tv_days = 7 WHERE id = 1`,
		`UPDATE invitations SET wish_limit_mode = 'custom', wish_movie_limit = 10, wish_movie_days = 91, wish_tv_limit = 5, wish_tv_days = 7 WHERE id = 1`,
		`UPDATE invitations SET wish_limit_mode = 'custom', wish_movie_limit = 10, wish_movie_days = 7, wish_tv_limit = 5, wish_tv_days = 0 WHERE id = 1`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: should fail a CHECK", bad)
		}
	}
	for _, good := range []string{
		`UPDATE accounts SET can_request = 1 WHERE id = 2`,
		`UPDATE invitations SET can_request = 1, wish_limit_mode = 'custom', wish_movie_limit = 0, wish_movie_days = 1, wish_tv_limit = 100, wish_tv_days = 90 WHERE id = 1`,
		`UPDATE invitations SET wish_limit_mode = 'unlimited', wish_movie_limit = NULL, wish_movie_days = NULL, wish_tv_limit = NULL, wish_tv_days = NULL WHERE id = 1`,
	} {
		if _, err := db.Write.ExecContext(ctx, good); err != nil {
			t.Errorf("%s: %v", good, err)
		}
	}
}
