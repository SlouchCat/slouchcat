package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00070 (concept 25, 08-datenmodell-api.md section 2.11) on the newest frozen state
// (ordner-metadaten.sql) with an inbox entry and two deliveries from before: the entry survives
// with the dedupe key of its episode, the deliveries survive unchanged; the new kinds and events
// are accepted, an entry of a wish needs no item, a wish kind needs its wish, the dedupe key is
// unique per profile, and the entries of a wish go with it.
func TestMigration00070WishesNotifications(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	extra := `
INSERT INTO notifications (id, profile_id, kind, item_id, parent_id, created_at, read_at)
SELECT 7, (SELECT id FROM profiles ORDER BY id LIMIT 1), 'new_episode', e.id, NULL, 1793325197706, 1793325197800
FROM items AS e WHERE e.kind = 'episode' ORDER BY e.id LIMIT 1;
INSERT INTO deliveries (id, account_id, channel, event, source, subject, body, state, created_at)
SELECT 3, id, 'email', 'new_content', 'library:1', 'Neu: Folge 1', 'Eine neue Folge', 'sent', 1793325197706
FROM accounts ORDER BY id LIMIT 1;
INSERT INTO deliveries (id, account_id, channel, event, subject, body, state, created_at)
SELECT 4, id, 'push', 'errors', 'Fehler', 'Etwas ging schief', 'queued', 1793325197707
FROM accounts ORDER BY id LIMIT 1;
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
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 70 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM notifications WHERE id = 7 AND kind = 'new_episode' AND dedupe_key = 'episode:' || item_id
			AND wish_id IS NULL AND created_at = 1793325197706 AND read_at = 1793325197800`: 1,
		`SELECT count(*) FROM deliveries WHERE id = 3 AND event = 'new_content' AND source = 'library:1' AND state = 'sent'`: 1,
		`SELECT count(*) FROM deliveries WHERE id = 4 AND event = 'errors' AND state = 'queued'`:                             1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('notifications_inbox', 'notifications_unread', 'notifications_item',
			'notifications_parent', 'notifications_wish', 'deliveries_state', 'deliveries_account', 'deliveries_dedupe')`: 8,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	exec := func(query string) error {
		_, err := db.Write.ExecContext(ctx, query)
		return err
	}
	const profile = `(SELECT id FROM profiles ORDER BY id LIMIT 1)`
	for _, q := range []string{
		`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (1, 'movie', 1061474, 'Superman', 'slouchcat', 'pending', 1, 1)`,
		`INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (` + profile + `, 'wish_approved', 1, 'wish:1:approved', 1)`,
		`INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (` + profile + `, 'wish_declined', 1, 'wish:1:declined', 1)`,
		`INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (` + profile + `, 'wish_failed', 1, 'wish:1:failed', 1)`,
		`INSERT INTO notifications (profile_id, kind, wish_id, item_id, dedupe_key, created_at)
			SELECT ` + profile + `, 'wish_available', 1, id, 'wish:1:available', 1 FROM items WHERE kind = 'movie' ORDER BY id LIMIT 1`,
		`INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (` + profile + `, 'wish_season_available', 1, 'wish:1:season:1', 1)`,
		`INSERT INTO deliveries (account_id, channel, event, subject, body, created_at) SELECT id, 'email', 'wish_available', 's', 'b', 1 FROM accounts ORDER BY id LIMIT 1`,
		`INSERT INTO deliveries (account_id, channel, event, subject, body, created_at) SELECT id, 'email', 'wish_status', 's', 'b', 1 FROM accounts ORDER BY id LIMIT 1`,
		`INSERT INTO deliveries (account_id, channel, event, subject, body, created_at) SELECT id, 'email', 'wish_review', 's', 'b', 1 FROM accounts ORDER BY id LIMIT 1`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for name, q := range map[string]string{
		"unknown kind":           `INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (` + profile + `, 'wish_review', 1, 'wish:1:review', 1)`,
		"wish kind without wish": `INSERT INTO notifications (profile_id, kind, item_id, dedupe_key, created_at) SELECT ` + profile + `, 'wish_available', id, 'x', 1 FROM items ORDER BY id LIMIT 1`,
		"episode without item":   `INSERT INTO notifications (profile_id, kind, dedupe_key, created_at) VALUES (` + profile + `, 'new_episode', 'episode:', 1)`,
		"episode with a wish": `INSERT INTO notifications (profile_id, kind, item_id, wish_id, dedupe_key, created_at)
			SELECT ` + profile + `, 'new_episode', id, 1, 'episode:x', 1 FROM items ORDER BY id LIMIT 1`,
		"duplicate dedupe key":   `INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (` + profile + `, 'wish_failed', 1, 'wish:1:failed', 2)`,
		"empty dedupe key":       `INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (` + profile + `, 'wish_failed', 1, '', 2)`,
		"unknown delivery event": `INSERT INTO deliveries (account_id, channel, event, subject, body, created_at) SELECT id, 'email', 'acquisition', 's', 'b', 1 FROM accounts ORDER BY id LIMIT 1`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: insert succeeded, want a constraint error", name)
		}
	}
	// The same key in another profile is a separate entry.
	if err := exec(`INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at)
		VALUES ((SELECT id FROM profiles ORDER BY id LIMIT 1 OFFSET 1), 'wish_failed', 1, 'wish:1:failed', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`DELETE FROM wishes WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM notifications`); got != 1 {
		t.Errorf("entries after their wish went: %d, want 1 (the episode)", got)
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
