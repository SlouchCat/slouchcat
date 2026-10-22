package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The frozen M8 state carries every kind of M8 data (testdata/db/README.md), so later migrations
// are tested against the channel settings of an account, a push subscription, an open Telegram
// link code, a delivery of the queue, a published announcement with an acknowledgement next to a
// draft, the encrypted mail server settings and a progress event that names its device.
func TestFrozenM8StateCoversM8Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M8.sql"))
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
	for what, query := range map[string]string{
		"e-mail channel switched on with events": `SELECT count(*) FROM account_channels WHERE channel = 'email' AND enabled = 1 AND json_array_length(events) >= 2`,
		"default rows of the other channels":     `SELECT count(*) FROM account_channels WHERE channel IN ('push', 'telegram') AND enabled = 0`,
		"push subscription with keys":            `SELECT count(*) FROM push_subscriptions WHERE endpoint LIKE 'https://%' AND length(p256dh) > 80 AND length(auth) > 20`,
		"open telegram link code":                `SELECT count(*) FROM telegram_links WHERE length(code) = 6 AND expires_at > created_at`,
		"delivery of an announcement":            `SELECT count(*) FROM deliveries WHERE channel = 'email' AND event = 'announcements' AND state IN ('queued', 'sent', 'failed')`,
		"published announcement":                 `SELECT count(*) FROM announcements WHERE published_at IS NOT NULL`,
		"draft announcement":                     `SELECT count(*) FROM announcements WHERE published_at IS NULL`,
		"acknowledgement of a profile":           `SELECT count(*) FROM announcement_acks a JOIN announcements n ON n.id = a.announcement_id WHERE n.published_at IS NOT NULL`,
		"encrypted mail server password":         `SELECT count(*) FROM settings WHERE key = 'smtp_password' AND value LIKE 'v1:%'`,
		"public url and update check setting":    `SELECT count(*) FROM settings s WHERE s.key = 'public_url' AND EXISTS (SELECT 1 FROM settings WHERE key = 'update_check_enabled')`,
		"progress event with its device":         `SELECT count(*) FROM progress_events p JOIN devices d ON d.id = p.device_id`,
		"pick and favourite of M7":               `SELECT count(*) FROM top_picks WHERE EXISTS (SELECT 1 FROM favorites)`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M8 state has no %s", what)
		}
	}
	// No secret ever reaches the frozen state in the clear (A-9, 3.8.3).
	for _, plain := range []string{"FAKE-frozen-smtp-pass", "frozen-owner-pass"} {
		if strings.Contains(string(script), plain) {
			t.Errorf("frozen M8 state contains the plain secret %q", plain)
		}
	}
}
