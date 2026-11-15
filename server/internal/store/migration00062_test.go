package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migrations 00062 to 00064 (concept 22 stage 3, ADR 0034) on the frozen state before them
// (freigabelaender.sql): the rating columns of items and libraries lose the CHECK on the FSK steps
// without a table rebuild, accounts and invitations are rebuilt, item_certifications is new and
// the one-time hint of the administration is set because provider ratings exist. Every value is
// kept, the derived columns still equal what the triggers compute, the four rating triggers, the
// account trigger, the index and the view are back, the foreign keys hold, ages such as 13, 14
// and 17 are stored and anything above 18 is refused.
func TestMigration00062MinimumAges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "freigabelaender.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id IN (62, 63, 64) AND is_applied = 1`: 3,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                             0,
		// Values kept, the frozen state's own (see testdata/db/README.md).
		`SELECT count(*) FROM items WHERE id = '01M6585TXK7NNK68XFTPTBESVF' AND own_rating = 12 AND effective_rating = 12 AND rating_source = 'tmdb:DE'`:                                                  1,
		`SELECT count(*) FROM items WHERE id = '01M6585TXM70G9GSBN41B2G7D0' AND own_rating = 16 AND rating_source = 'tmdb:US'`:                                                                            1,
		`SELECT count(*) FROM items WHERE id = '01M6585TXPNMFEW1D01E2HFA8X' AND own_rating = 6 AND rating_source = 'manual' AND locked_fields = '["own_rating"]'`:                                         1,
		`SELECT count(*) FROM items WHERE id = '01M6585TXFV7FMJPYSCG96AQRK' AND own_rating = 16 AND effective_rating = 16 AND rating_source = 'tvdb:US'`:                                                  1,
		`SELECT count(*) FROM items WHERE id = '01M6585TXFV7FMJPYSCH66A7DW' AND own_rating IS NULL AND effective_rating = 16`:                                                                             1,
		`SELECT count(*) FROM items WHERE id = '01M6585TYE1RWHX6C8CR9YJD94' AND own_rating IS NULL AND effective_rating IS NULL`:                                                                          1,
		`SELECT count(*) FROM items WHERE kind = 'series' AND min_child_rating IS NOT NULL`:                                                                                                               2,
		`SELECT count(*) FROM libraries WHERE id = 3 AND default_rating = 6`:                                                                                                                              1,
		`SELECT count(*) FROM libraries WHERE id = 4 AND default_rating = 0`:                                                                                                                              1,
		`SELECT count(*) FROM libraries WHERE default_rating IS NULL`:                                                                                                                                     3,
		`SELECT count(*) FROM accounts WHERE id = 1 AND role = 'admin' AND is_owner = 1 AND max_rating = 18 AND all_libraries = 1`:                                                                        1,
		`SELECT count(*) FROM accounts WHERE id = 2 AND role = 'user' AND max_rating = 12 AND all_libraries = 0 AND invited_by = 1 AND max_bitrate_kbps = 8000 AND max_height = 1080 AND max_streams = 2`: 1,
		`SELECT count(*) FROM account_libraries WHERE account_id = 2`:                                                                                                                                     1,
		`SELECT count(*) FROM profiles WHERE account_id = 2 AND is_default = 1 AND max_rating = 12`:                                                                                                       1,
		`SELECT count(*) FROM invitations WHERE id = 1 AND used_by = 2 AND max_rating = 12 AND note = 'Gast'`:                                                                                             1,
		`SELECT count(*) FROM invitations WHERE id = 2 AND used_at IS NULL AND max_rating = 6 AND created_by = 1`:                                                                                         1,
		`SELECT count(*) FROM invitation_libraries`:                                                                                                                                                       3,
		`SELECT count(*) FROM settings WHERE key = 'metadata.rating_country' AND value = 'CH'`:                                                                                                            1,
		`SELECT count(*) FROM settings WHERE key = 'metadata.minimum_age_notice' AND value = '1'`:                                                                                                         1,
		`SELECT count(*) FROM item_certifications`:                                                                                                                                                        0,
		// The schema: no CHECK on the FSK steps left, the triggers, index and view back, no temporary name.
		`SELECT count(*) FROM sqlite_master WHERE sql LIKE '%IN (0, 6, 12, 16, 18)%'`:                                                                                                                                 0,
		`SELECT count(*) FROM sqlite_master WHERE sql LIKE '%_rating_age%' OR sql LIKE '%accounts_new%' OR sql LIKE '%invitations_new%'`:                                                                              0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('items_rating_insert', 'items_rating_update', 'items_container_update', 'items_container_delete', 'activity_events_account_deleted')`: 5,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'accounts_owner'`:                                                                                                                         1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'view' AND name = 'visible_items'`:                                                                                                                           1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'accounts' AND sql LIKE '%CHECK (role <> ''admin'' OR (max_rating = 18 AND all_libraries = 1))%'`:                                         1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	// The derived columns equal what the triggers compute: touching own_rating recomputes them.
	derived := func() map[string][2]any {
		rows, err := db.Read.QueryContext(ctx, `SELECT id, effective_rating, min_child_rating FROM items ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string][2]any{}
		for rows.Next() {
			var id string
			var eff, child any
			if err := rows.Scan(&id, &eff, &child); err != nil {
				t.Fatal(err)
			}
			out[id] = [2]any{eff, child}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	was := derived()
	mustExec(t, db.Write, `UPDATE items SET own_rating = own_rating`)
	mustExec(t, db.Write, `UPDATE items SET available = available WHERE parent_id IS NOT NULL`)
	for id, now := range derived() {
		if was[id] != now {
			t.Errorf("item %s: derived %v after the migration, %v recomputed by the triggers", id, was[id], now)
		}
	}
	// The guest (ceiling 12, library 1) sees what the frozen state's rules say.
	if n := count(`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id JOIN profiles AS p ON p.id = v.profile_id
		WHERE p.account_id = 2 AND (i.library_id <> 1 OR COALESCE(i.effective_rating, 18) > 12)`); n != 0 {
		t.Errorf("visible_items shows the guest %d titles outside library 1 or above 12", n)
	}
	if count(`SELECT count(*) FROM visible_items AS v JOIN profiles AS p ON p.id = v.profile_id WHERE p.account_id = 2`) == 0 {
		t.Error("visible_items shows the guest nothing")
	}

	// Minimum ages are stored now; above 18 and below 0 are refused everywhere.
	for _, q := range []string{
		`UPDATE items SET own_rating = 13 WHERE id = '01M6585TXK7NNK68XFTPTBESVF'`,
		`UPDATE libraries SET default_rating = 14 WHERE id = 3`,
		`UPDATE accounts SET max_rating = 17 WHERE id = 2`,
		`UPDATE invitations SET max_rating = 15 WHERE id = 2`,
	} {
		if _, err := db.Write.ExecContext(ctx, q); err != nil {
			t.Errorf("%s: %v", q, err)
		}
	}
	for _, q := range []string{
		`UPDATE items SET own_rating = 19 WHERE id = '01M6585TXK7NNK68XFTPTBESVF'`,
		`UPDATE items SET own_rating = -1 WHERE id = '01M6585TXK7NNK68XFTPTBESVF'`,
		`UPDATE libraries SET default_rating = 21 WHERE id = 3`,
		`UPDATE accounts SET max_rating = 21 WHERE id = 2`,
		`UPDATE invitations SET max_rating = 19 WHERE id = 2`,
		// The table CHECK of 00022 holds: an administrator has 18 and every library.
		`UPDATE accounts SET max_rating = 17 WHERE id = 1`,
		`UPDATE accounts SET role = 'admin' WHERE id = 2`,
		`UPDATE accounts SET is_owner = 1 WHERE id = 2`,
	} {
		if _, err := db.Write.ExecContext(ctx, q); err == nil {
			t.Errorf("%s was accepted", q)
		}
	}
	if got := queryInt(t, db.Read, `SELECT effective_rating FROM items WHERE id = '01M6585TXK7NNK68XFTPTBESVF'`); got != val(13) {
		t.Errorf("effective rating after own 13 = %v", got)
	}
	// The account trigger is back: deleting the guest deletes its activity events; the cascades run.
	mustExec(t, db.Write, `DELETE FROM accounts WHERE id = 2`)
	for query, want := range map[string]int{
		`SELECT count(*) FROM activity_events WHERE account_id = 2`:         0,
		`SELECT count(*) FROM profiles WHERE account_id = 2`:                0,
		`SELECT count(*) FROM account_libraries WHERE account_id = 2`:       0,
		`SELECT count(*) FROM invitations WHERE id = 1 AND used_by IS NULL`: 1,
	} {
		if got := count(query); got != want {
			t.Errorf("after deleting the guest: %s = %d, want %d", query, got, want)
		}
	}
	// item_certifications goes with its title and refuses what is no certificate row.
	mustExec(t, db.Write, `INSERT INTO item_certifications (item_id, provider, country, certification, fetched_at)
		VALUES ('01M6585TXK7NNK68XFTPTBESVF', 'tmdb', 'US', 'PG-13', 1), ('01M6585TXK7NNK68XFTPTBESVF', 'tvdb', '', '', 1)`)
	for _, q := range []string{
		`INSERT INTO item_certifications VALUES ('01M6585TXK7NNK68XFTPTBESVF', 'anilist', 'JP', 'G', 1)`,
		`INSERT INTO item_certifications VALUES ('01M6585TXK7NNK68XFTPTBESVF', 'tmdb', 'us', 'R', 1)`,
		`INSERT INTO item_certifications VALUES ('01M6585TXK7NNK68XFTPTBESVF', 'tmdb', '', 'R', 1)`,
		`INSERT INTO item_certifications VALUES ('01M6585TXK7NNK68XFTPTBESVF', 'tmdb', 'FR', '', 1)`,
		`INSERT INTO item_certifications VALUES ('01NOSUCHITEM00000000000000', 'tmdb', 'FR', '12', 1)`,
	} {
		if _, err := db.Write.ExecContext(ctx, q); err == nil {
			t.Errorf("%s was accepted", q)
		}
	}
	mustExec(t, db.Write, `DELETE FROM items WHERE id = '01M6585TXK7NNK68XFTPTBESVF'`)
	if n := count(`SELECT count(*) FROM item_certifications`); n != 0 {
		t.Errorf("%d certificates outlived their title", n)
	}
}

// A fresh database gets no hint: without provider ratings there is nothing that became stricter.
func TestMigration00064NoNoticeOnAFreshInstall(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	if n := queryInt(t, db.Read, `SELECT count(*) FROM settings WHERE key = 'metadata.minimum_age_notice'`); n != val(0) {
		t.Errorf("a fresh install has the minimum age notice (%v)", n)
	}
}
