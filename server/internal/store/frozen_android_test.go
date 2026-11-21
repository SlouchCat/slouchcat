package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 27 stage 1 (migrations 00082 to 00085; testdata/db/android.sql) is
// wuensche.sql migrated to 00085 with the rows of the Android apps: sessions of all three kinds (the
// browser sessions of before, an app session of a phone and a device session of a TV, both without
// expiry, with named devices), an open pairing voucher of the phone's session, the phone's push
// subscription through the push relay and the message device_linked by e-mail and by push with
// pushed_to. TestFrozenStatesMigrate replays it like every state; this test checks that each of
// those is really in it after the later migrations.
func TestFrozenAndroidStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "android.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"migration 00085 recorded as applied":       `SELECT count(*) FROM goose_db_version WHERE version_id = 85 AND is_applied = 1`,
		"sessions of all three kinds":               `SELECT count(DISTINCT kind) = 3 FROM sessions`,
		"app and device sessions without expiry":    `SELECT count(*) = 2 FROM sessions WHERE kind <> 'browser' AND expires_at = 253402214400000`,
		"a named phone and a named TV":              `SELECT count(*) = 2 FROM devices WHERE (type = 'phone' AND name = 'Pixel 9') OR (type = 'tv' AND name = 'Wohnzimmer')`,
		"an open voucher of the phone's session":    `SELECT count(*) FROM pairing_vouchers WHERE session_id IN (SELECT id FROM sessions WHERE kind = 'app')`,
		"a push subscription of the push relay":     `SELECT count(*) FROM push_subscriptions WHERE endpoint LIKE 'https://push.slouchcat.com/%' AND device_id IS NOT NULL`,
		"device_linked by e-mail and by push":       `SELECT count(DISTINCT channel) = 2 FROM deliveries WHERE event = 'device_linked'`,
		"a delivery with the subscriptions reached": `SELECT count(*) FROM deliveries WHERE event = 'device_linked' AND json_array_length(pushed_to) = 1`,
		"foreign keys of the frozen state hold":     `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Errorf("%s: %v", what, err)
			continue
		}
		if n == 0 {
			t.Errorf("frozen android state has no %s", what)
		}
	}
}
