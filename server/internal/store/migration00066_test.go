package store

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// openFrozenMigrated replays the newest frozen state before concept 25 (ordner-metadaten.sql) and
// opens it, which runs every pending migration.
func openFrozenMigrated(t *testing.T) *DB {
	t.Helper()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Migration 00066 (concept 25, data model 2.1, 2.2, 2.8, 2.13): four new tables on the frozen state.
// The checks of the mapping states hold, a Seerr user belongs to one account at most, the shared
// user to none (in both directions), the call log keeps 20 rows, and deleting an account takes its
// mapping and limit with it.
func TestMigration00066WishesMapping(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openFrozenMigrated(t)
	exec := func(query string, args ...any) error {
		_, err := db.Write.ExecContext(ctx, query, args...)
		return err
	}
	must := func(query string, args ...any) {
		t.Helper()
		if err := exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 66 AND is_applied = 1`:                                                                                                            1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                                                                                             0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('seerr_users', 'wish_account_map', 'wish_account_limits', 'seerr_log')`:                                               4,
		`SELECT count(*) FROM sqlite_master WHERE name = 'wish_profile_map'`:                                                                                                                        0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('wish_account_map_shared_insert', 'wish_account_map_shared_update', 'seerr_users_shared_mapped', 'seerr_log_trim')`: 4,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	user := `INSERT INTO seerr_users (seerr_id, display_name, email, user_type, permissions, fetched_at) VALUES (?, ?, ?, 2, 32, 1)`
	must(user, 3, "kinder", "kinder@example.test")
	must(user, 4, "grosseltern", "Grosseltern@Example.test")
	must(user, 5, "slouchcat-gaeste", "slouchcat-gaeste@slouchcat.invalid")

	// The email compares without case (linking by the same email, N-2).
	if got := count(`SELECT count(*) FROM seerr_users WHERE email = 'grosseltern@example.test'`); got != 1 {
		t.Errorf("email without case: %d rows, want 1", got)
	}
	// Bad users.
	for _, bad := range []string{
		`INSERT INTO seerr_users (seerr_id, display_name, user_type, permissions, fetched_at) VALUES (0, 'x', 2, 0, 1)`,
		`INSERT INTO seerr_users (seerr_id, display_name, user_type, permissions, fetched_at) VALUES (9, '', 2, 0, 1)`,
		`INSERT INTO seerr_users (seerr_id, display_name, user_type, permissions, fetched_at) VALUES (9, 'x', 5, 0, 1)`,
		`INSERT INTO seerr_users (seerr_id, display_name, user_type, permissions, fetched_at) VALUES (9, 'x', 2, -1, 1)`,
	} {
		if err := exec(bad); err == nil {
			t.Errorf("%s: should fail a CHECK", bad)
		}
	}

	// At most one shared user; it never belongs to an account.
	must(`UPDATE seerr_users SET is_shared = 1 WHERE seerr_id = 5`)
	if err := exec(`UPDATE seerr_users SET is_shared = 1 WHERE seerr_id = 4`); err == nil {
		t.Error("a second shared user should fail the unique index")
	}
	mapping := `INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, password_state, set_at) VALUES (?, ?, ?, 'ready', 'off', 1)`
	if err := exec(mapping, 2, 5, "manual"); err == nil || !strings.Contains(err.Error(), "seerr_user_taken") {
		t.Errorf("mapping the shared user: %v, want seerr_user_taken", err)
	}
	must(mapping, 2, 3, "manual")
	if err := exec(`UPDATE wish_account_map SET seerr_user_id = 5 WHERE account_id = 2`); err == nil || !strings.Contains(err.Error(), "seerr_user_taken") {
		t.Errorf("remapping to the shared user: %v, want seerr_user_taken", err)
	}
	if err := exec(`UPDATE seerr_users SET is_shared = 0 WHERE seerr_id = 5`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`UPDATE seerr_users SET is_shared = 1 WHERE seerr_id = 3`); err == nil || !strings.Contains(err.Error(), "seerr_user_taken") {
		t.Errorf("marking a mapped user shared: %v, want seerr_user_taken", err)
	}
	// One account per user.
	if err := exec(mapping, 1, 3, "manual"); err == nil {
		t.Error("a user of two accounts should fail UNIQUE")
	}

	// The states of the mapping.
	for _, bad := range []string{
		// waiting needs no user, ready needs one
		`INSERT INTO wish_account_map (account_id, seerr_user_id, state, set_at) VALUES (1, 4, 'waiting', 1)`,
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, set_at) VALUES (1, NULL, 'created', 'ready', 1)`,
		// ready needs an origin
		`INSERT INTO wish_account_map (account_id, seerr_user_id, state, set_at) VALUES (1, 4, 'ready', 1)`,
		// linked needs linked_by and linked_at, the others neither
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, set_at) VALUES (1, 4, 'linked', 'ready', 1)`,
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, linked_by, set_at) VALUES (1, 4, 'linked', 'ready', 'invite', 1)`,
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, linked_by, linked_at, set_at) VALUES (1, 4, 'created', 'ready', 'admin', 1, 1)`,
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, linked_by, set_at) VALUES (1, 4, 'linked', 'ready', 'robot', 1)`,
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, password_state, set_at) VALUES (1, 4, 'created', 'ready', 'plain', 1)`,
		// no user that does not exist
		`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, set_at) VALUES (1, 99, 'manual', 'ready', 1)`,
	} {
		if err := exec(bad); err == nil {
			t.Errorf("%s: should fail", bad)
		}
	}
	must(`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, linked_by, linked_at, password_state, set_at)
	      VALUES (1, 4, 'linked', 'ready', 'invite', 7, 'synced', 1)`)
	must(`DELETE FROM wish_account_map WHERE account_id = 1`)
	// A waiting row may carry the trigger of the later link.
	must(`INSERT INTO wish_account_map (account_id, state, linked_by, set_at) VALUES (1, 'waiting', 'invite', 1)`)
	if got := count(`SELECT count(*) FROM wish_account_map WHERE account_id = 1 AND password_state = 'next_login'`); got != 1 {
		t.Errorf("default password state: %d rows with next_login, want 1", got)
	}

	// Limits per account.
	for _, bad := range []string{
		`INSERT INTO wish_account_limits (account_id, mode, set_at) VALUES (1, 'custom', 1)`,
		`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at) VALUES (1, 'unlimited', 1, 7, 1, 7, 1)`,
		`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at) VALUES (1, 'custom', 101, 7, 1, 7, 1)`,
		`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at) VALUES (1, 'custom', 1, 0, 1, 7, 1)`,
		`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at) VALUES (1, 'custom', 1, 7, 1, 91, 1)`,
		`INSERT INTO wish_account_limits (account_id, mode, set_at) VALUES (1, 'default', 1)`,
	} {
		if err := exec(bad); err == nil {
			t.Errorf("%s: should fail a CHECK", bad)
		}
	}
	must(`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at) VALUES (2, 'custom', 0, 7, 5, 90, 1)`)
	must(`INSERT INTO wish_account_limits (account_id, mode, set_at) VALUES (1, 'unlimited', 1)`)

	// The call log keeps the newest 20 rows.
	for i := range 25 {
		must(`INSERT INTO seerr_log (at, method, path, status, duration_ms) VALUES (?, 'GET', '/user', 200, 3)`, i)
	}
	if got := count(`SELECT count(*) FROM seerr_log`); got != 20 {
		t.Errorf("seerr_log rows: %d, want 20", got)
	}
	if got := count(`SELECT min(at) FROM seerr_log`); got != 5 {
		t.Errorf("oldest kept call: at %d, want 5", got)
	}

	// Deleting an account takes its mapping and limit, the Seerr user stays.
	must(`PRAGMA foreign_keys = ON`)
	must(`DELETE FROM accounts WHERE id = 2`)
	for query, want := range map[string]int{
		`SELECT count(*) FROM wish_account_map WHERE account_id = 2`:    0,
		`SELECT count(*) FROM wish_account_limits WHERE account_id = 2`: 0,
		`SELECT count(*) FROM seerr_users WHERE seerr_id = 3`:           1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                 0,
	} {
		if got := count(query); got != want {
			t.Errorf("after deleting account 2: %s = %d, want %d", query, got, want)
		}
	}
}
