package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00073 (concept 25 step 4.3, F-14) on the newest frozen state (ordner-metadaten.sql):
// every channel row of an administrator chooses wish_review at the end of its list, a row that
// has it already keeps one, the rows of other accounts never get it, and the other events and
// enabled stay as they were.
func TestMigration00073WishReviewChannels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	// The frozen state has channel rows of the administrator only; the guest gets the rows
	// ChannelEnsureDefaults wrote before 00072. One administrator row chose wish_review by hand.
	extra := `
INSERT INTO account_channels (account_id, channel, enabled, events, config, updated_at)
SELECT a.id, c.channel, c.channel = 'email', '["announcements"]', '{}', 1
FROM accounts AS a JOIN (SELECT 'email' AS channel UNION ALL SELECT 'push' UNION ALL SELECT 'telegram') AS c
WHERE a.role = 'user';
UPDATE account_channels SET events = '["errors","wish_review"]', enabled = 1 WHERE channel = 'telegram'
  AND account_id IN (SELECT id FROM accounts WHERE role = 'admin');
CREATE TABLE before_00073 AS SELECT account_id, channel, enabled, events FROM account_channels;
`
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql")+extra)
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
	admins := count(`SELECT 3 * count(*) FROM accounts WHERE role = 'admin'`)
	users := count(`SELECT 3 * count(*) FROM accounts WHERE role = 'user'`)
	if admins == 0 || users == 0 {
		t.Fatalf("frozen state with %d administrator and %d guest channel rows, want both", admins, users)
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 73 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		// Administrators: wish_review exactly once, appended after what 00072 left; enabled kept.
		`SELECT count(*) FROM account_channels AS c JOIN accounts AS a ON a.id = c.account_id
		 WHERE a.role = 'admin' AND (SELECT count(*) FROM json_each(c.events) WHERE value = 'wish_review') = 1`: admins,
		`SELECT count(*) FROM account_channels AS c JOIN before_00073 AS b USING (account_id, channel)
		 JOIN accounts AS a ON a.id = c.account_id
		 WHERE a.role = 'admin' AND b.events NOT LIKE '%wish_review%' AND c.enabled = b.enabled
		   AND c.events = json_insert(b.events, '$[#]', 'wish_available', '$[#]', 'wish_review')`: admins - 1,
		`SELECT count(*) FROM account_channels WHERE events = '["errors","wish_review","wish_available"]' AND enabled = 1`: 1,
		// Nobody else gets it; the guests keep their list with wish_available of 00072.
		`SELECT count(*) FROM account_channels AS c JOIN accounts AS a ON a.id = c.account_id
		 WHERE a.role <> 'admin' AND c.events LIKE '%wish_review%'`: 0,
		`SELECT count(*) FROM account_channels AS c JOIN before_00073 AS b USING (account_id, channel)
		 JOIN accounts AS a ON a.id = c.account_id
		 WHERE a.role = 'user' AND c.enabled = b.enabled AND c.events = '["announcements","wish_available"]'`: users,
		`SELECT count(*) FROM account_channels WHERE events LIKE '%wish_status%'`: 0,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
}
