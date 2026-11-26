package store

import (
	"context"
	"testing"
)

// Migration 00086 (question 36, ADR 0045): the request right follows the age limit. The backfill
// switches it on for every account, profile and open invitation without an age limit and off for
// every one with an age limit (also one given by hand under F-5), turns «Neue Konten dürfen
// anfordern» on and sends the Seerr users Slouchcat opened for a changed account back to the sync.
// Afterwards the triggers move the right with the rating, and the guards refuse a right below 18.
func TestMigration00086RequestRightFollowsRating(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	opts := testOptions(t, dir)
	opts.migrations = migrationsBefore(t, "00086")
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO accounts (id, email, password_hash, role, is_owner, max_rating, can_request, created_at, updated_at) VALUES
			(1, 'owner@example.test', 'x', 'admin', 1, 18, 1, 1, 1),
			(2, 'adult@example.test', 'x', 'user', 0, 18, 0, 1, 1),
			(3, 'teen@example.test', 'x', 'user', 0, 12, 1, 1, 1),
			(4, 'kept@example.test', 'x', 'user', 0, 18, 1, 1, 1)`,
		`INSERT INTO profiles (id, account_id, name, max_rating, is_default, can_request, created_at, updated_at) VALUES
			(1, 1, 'Owner', 18, 1, 1, 1, 1),
			(2, 1, 'Adult', 18, 0, 0, 1, 1),
			(3, 1, 'Teen', 12, 0, 0, 1, 1),
			(4, 1, 'Child by hand', 6, 0, 1, 1, 1),
			(5, 3, 'Guest', 12, 1, 1, 1, 1),
			(6, 2, 'Adult guest', 18, 1, 1, 1, 1),
			(7, 3, 'Above the ceiling', 18, 0, 0, 1, 1)`,
		`INSERT INTO invitations (id, token_hash, max_rating, all_libraries, can_request, created_at, expires_at, used_at, revoked_at) VALUES
			(1, 'a', 18, 1, 0, 1, 9999999999999, NULL, NULL),
			(2, 'b', 12, 1, 1, 1, 9999999999999, NULL, NULL),
			(3, 'c', 18, 1, 0, 1, 9999999999999, 5, NULL),
			(4, 'd', 12, 1, 1, 1, 9999999999999, NULL, 5)`,
		`INSERT INTO settings (key, value, updated_at) VALUES ('wishes.new_accounts_can_request', '0', 1)`,
		`INSERT INTO seerr_users (seerr_id, display_name, user_type, permissions, fetched_at) VALUES (2, 'a', 2, 32, 1), (3, 'b', 2, 32, 1), (4, 'c', 2, 32, 1)`,
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, limit_written_at, set_at) VALUES
			(2, 2, 'created', 'ready', 5, 1), (3, 3, 'created', 'ready', 5, 1), (4, 4, 'created', 'ready', 5, 1)`,
	} {
		mustExec(t, db.Write, q)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, testOptions(t, dir))
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
		`SELECT count(*) FROM goose_db_version WHERE version_id = 86 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		// Accounts by their own rating.
		`SELECT group_concat(id, '') FROM (SELECT id FROM accounts WHERE can_request = 1 ORDER BY id)`: 124,
		// Profiles by the lower of their rating and the account's.
		`SELECT group_concat(id, '') FROM (SELECT id FROM profiles WHERE can_request = 1 ORDER BY id)`: 126,
		// Open invitations by their rating; every invitation with an age limit off.
		`SELECT group_concat(id, '') FROM (SELECT id FROM invitations WHERE can_request = 1 ORDER BY id)`: 1,
		`SELECT count(*) FROM invitations WHERE id = 3 AND can_request = 0`:                               1,
		`SELECT count(*) FROM settings WHERE key = 'wishes.new_accounts_can_request' AND value = '1'`:     1,
		// The opened users of the changed accounts 2 and 3 go back to the sync, 4 keeps its mark.
		`SELECT group_concat(account_id, '') FROM (SELECT account_id FROM wish_account_map WHERE limit_written_at IS NULL ORDER BY account_id)`: 23,
		`SELECT count(*) FROM wish_account_map WHERE account_id = 4 AND limit_written_at = 5`:                                                   1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	// The right follows the rating both ways, also when the account's ceiling lowers its profiles.
	for _, step := range []struct {
		exec, check string
		want        int
	}{
		{`UPDATE profiles SET max_rating = 16 WHERE id = 2`, `SELECT can_request FROM profiles WHERE id = 2`, 0},
		{`UPDATE profiles SET max_rating = 18 WHERE id = 2`, `SELECT can_request FROM profiles WHERE id = 2`, 1},
		{`UPDATE profiles SET can_request = 0 WHERE id = 2`, `SELECT can_request FROM profiles WHERE id = 2`, 0},
		{`UPDATE profiles SET max_rating = 12 WHERE id = 2`, `SELECT can_request FROM profiles WHERE id = 2`, 0},
		// A right switched off by hand before the limit comes back with its end (ADR 0045, point 5).
		{`UPDATE profiles SET max_rating = 18 WHERE id = 2`, `SELECT can_request FROM profiles WHERE id = 2`, 1},
		// A step that stays on one side of 18 leaves the switch alone.
		{`UPDATE profiles SET max_rating = 6 WHERE id = 3`, `SELECT can_request FROM profiles WHERE id = 3`, 0},
		{`UPDATE profiles SET can_request = 0 WHERE id = 6`, `SELECT can_request FROM profiles WHERE id = 6`, 0},
		{`UPDATE profiles SET name = 'Renamed' WHERE id = 6`, `SELECT can_request FROM profiles WHERE id = 6`, 0},
		{`UPDATE accounts SET max_rating = 16 WHERE id = 2`, `SELECT can_request FROM accounts WHERE id = 2`, 0},
		{`UPDATE profiles SET max_rating = 16 WHERE account_id = 2 AND max_rating > 16`, `SELECT count(*) FROM profiles WHERE account_id = 2 AND can_request = 1`, 0},
		{`UPDATE accounts SET max_rating = 18 WHERE id = 2`, `SELECT can_request FROM accounts WHERE id = 2`, 1},
		{`UPDATE accounts SET role = 'admin', max_rating = 18, all_libraries = 1 WHERE id = 3`, `SELECT can_request FROM accounts WHERE id = 3`, 1},
	} {
		mustExec(t, db.Write, step.exec)
		if got := count(step.check); got != step.want {
			t.Errorf("after %s: %s = %d, want %d", step.exec, step.check, got, step.want)
		}
	}

	// The guards: no right below 18, whoever writes it.
	for _, bad := range []string{
		`UPDATE profiles SET can_request = 1 WHERE id = 3`,
		`UPDATE profiles SET max_rating = 12, can_request = 1 WHERE id = 1`,
		`INSERT INTO profiles (account_id, name, max_rating, is_default, can_request, created_at, updated_at) VALUES (1, 'Kid', 12, 0, 1, 1, 1)`,
		`INSERT INTO accounts (email, password_hash, role, is_owner, max_rating, can_request, created_at, updated_at) VALUES ('kid@example.test', 'x', 'user', 0, 12, 1, 1, 1)`,
		`UPDATE invitations SET can_request = 1 WHERE id = 2`,
		`INSERT INTO invitations (token_hash, max_rating, all_libraries, can_request, created_at, expires_at) VALUES ('e', 16, 1, 1, 1, 2)`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: should be refused", bad)
		}
	}
	mustExec(t, db.Write, `UPDATE accounts SET max_rating = 12 WHERE id = 4`)
	if _, err := db.Write.ExecContext(ctx, `UPDATE accounts SET can_request = 1 WHERE id = 4`); err == nil {
		t.Error("an account with an age limit got the right")
	}
	for _, good := range []string{
		`INSERT INTO profiles (account_id, name, max_rating, is_default, can_request, created_at, updated_at) VALUES (1, 'Adult 2', 18, 0, 1, 1, 1)`,
		`INSERT INTO invitations (token_hash, max_rating, all_libraries, can_request, created_at, expires_at) VALUES ('f', 18, 1, 1, 1, 2)`,
		`UPDATE profiles SET can_request = 0 WHERE id = 1`,
	} {
		mustExec(t, db.Write, good)
	}
}
