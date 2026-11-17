package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00072 (concept 25 step 2.6, F-14) on the newest frozen state (ordner-metadaten.sql):
// every channel row of every account chooses wish_available at the end of its list, a row whose
// owner removed every event gets it too, a row that has it already keeps one, the other events and
// enabled stay as they were.
func TestMigration00072WishAvailableChannels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	// Before: one row without any event, one that chose wish_available already (both changed by
	// hand in the frozen state), the others with the defaults of their account.
	// The frozen state has channel rows of the administrator only; the guest gets the rows
	// ChannelEnsureDefaults wrote before 00072.
	extra := `
INSERT INTO account_channels (account_id, channel, enabled, events, config, updated_at)
SELECT a.id, c.channel, c.channel = 'email', '["announcements"]', '{}', 1
FROM accounts AS a JOIN (SELECT 'email' AS channel UNION ALL SELECT 'push' UNION ALL SELECT 'telegram') AS c
WHERE a.role = 'user';
UPDATE account_channels SET events = '[]', enabled = 1 WHERE channel = 'push'
  AND account_id = (SELECT id FROM accounts ORDER BY id LIMIT 1);
UPDATE account_channels SET events = '["wish_available","announcements"]' WHERE channel = 'telegram'
  AND account_id = (SELECT id FROM accounts ORDER BY id LIMIT 1);
CREATE TABLE before_00072 AS SELECT account_id, channel, enabled, events FROM account_channels;
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
	rows := count(`SELECT count(*) FROM before_00072`)
	if rows < 6 {
		t.Fatalf("%d channel rows in the frozen state, want those of at least two accounts", rows)
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 72 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		// Every row has the event exactly once.
		`SELECT count(*) FROM account_channels AS c
		 WHERE (SELECT count(*) FROM json_each(c.events) WHERE value = 'wish_available') = 1`: rows,
		// Appended to the old list; enabled unchanged. The test runs every later migration too:
		// 00073 appends wish_review to the rows of administrators.
		`SELECT count(*) FROM account_channels AS c JOIN before_00072 AS b USING (account_id, channel)
		 WHERE b.events NOT LIKE '%wish_available%' AND c.enabled = b.enabled
		   AND c.events IN (json_insert(b.events, '$[#]', 'wish_available'),
		                    json_insert(b.events, '$[#]', 'wish_available', '$[#]', 'wish_review'))`: rows - 1,
		`SELECT count(*) FROM account_channels AS c JOIN before_00072 AS b USING (account_id, channel)
		 WHERE b.events LIKE '%wish_available%' AND c.enabled = b.enabled
		   AND c.events IN (b.events, json_insert(b.events, '$[#]', 'wish_review'))`: 1,
		`SELECT count(*) FROM account_channels WHERE events IN ('["wish_available"]', '["wish_available","wish_review"]')
		 AND enabled = 1`: 1,
		// Guests keep e-mail on with announcements, administrators their four events.
		`SELECT count(*) FROM account_channels AS c JOIN accounts AS a ON a.id = c.account_id
		 WHERE a.role = 'user' AND c.events <> '["announcements","wish_available"]'`: 0,
		`SELECT count(*) FROM account_channels AS c JOIN accounts AS a ON a.id = c.account_id
		 WHERE a.role = 'user'`: count(`SELECT 3 * count(*) FROM accounts WHERE role = 'user'`),
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	if count(`SELECT count(*) FROM accounts WHERE role = 'user'`) == 0 {
		t.Error("the frozen state carries no guest account; the guest list went unchecked")
	}
}
