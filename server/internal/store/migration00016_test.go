package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// Migration 00016 (M8) adds the notification channels, Telegram links, push subscriptions,
// deliveries and announcements, and gives every existing account its three channel rows with the
// defaults of plan 5.2: e-mail on for guests with the event announcements, the main account's e-mail
// off with new content, errors, invitations and announcements chosen; push and Telegram off with the
// same event lists. The frozen states M2 to M7 must come out that way (M2 carries a guest account).
func TestMigration00016DefaultChannels(t *testing.T) {
	const ownerEvents = `["new_content","errors","invitations","announcements"]`
	const guestEvents = `["announcements"]`
	guests := 0
	for _, state := range []string{"M2", "M3", "M4", "M5", "M6", "M7"} {
		t.Run(state, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", state+".sql"))
			if err != nil {
				t.Fatal(err)
			}
			cfg := t.TempDir()
			replay(t, filepath.Join(cfg, FileName), string(script))
			db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, table := range []string{"account_channels", "telegram_links", "push_subscriptions", "deliveries", "announcements", "announcement_acks"} {
				var n int
				if err := db.Read.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil || n != 1 {
					t.Errorf("table %s missing after 00016 (%v)", table, err)
				}
			}
			var accounts, rows int
			if err := db.Read.QueryRow(`SELECT count(*) FROM accounts`).Scan(&accounts); err != nil {
				t.Fatal(err)
			}
			if err := db.Read.QueryRow(`SELECT count(*) FROM account_channels`).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			if accounts == 0 || rows != 3*accounts {
				t.Fatalf("account_channels rows = %d for %d accounts, want three per account", rows, accounts)
			}
			list, err := db.Read.Query(`SELECT a.role, c.channel, c.enabled, c.events, c.config
			                            FROM account_channels AS c JOIN accounts AS a ON a.id = c.account_id
			                            ORDER BY a.id, c.channel`)
			if err != nil {
				t.Fatal(err)
			}
			defer list.Close()
			for list.Next() {
				var role, channel, events, config string
				var enabled int
				if err := list.Scan(&role, &channel, &enabled, &events, &config); err != nil {
					t.Fatal(err)
				}
				wantEnabled, wantEvents := 0, ownerEvents
				if role == "guest" {
					wantEvents = guestEvents
					if channel == "email" {
						wantEnabled = 1
						guests++
					}
				}
				if enabled != wantEnabled || events != wantEvents || config != "{}" {
					t.Errorf("%s %s: enabled %d, events %s, config %s; want enabled %d, events %s, config {}",
						role, channel, enabled, events, config, wantEnabled, wantEvents)
				}
			}
			if err := list.Err(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if guests == 0 {
		t.Error("no frozen state carried a guest account; the guest default went unchecked")
	}
}
