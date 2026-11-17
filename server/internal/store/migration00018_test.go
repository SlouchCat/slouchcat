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

// Migration 00018 (M9) removes the notification event "acquisition": its producer I-1 fell away
// when M9 was cut down to the webhook receiver. The event disappears from the CHECK constraint of
// deliveries and from the chosen events of every channel, while everything else stays as it was.
//
// The frozen M8 state never chose the event (it was not a default), so the test adds it to the
// state first — that is the case the migration exists for.
func TestMigration00018DropsTheAcquisitionEvent(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M8.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	path := filepath.Join(cfg, FileName)
	// An account that chose the event, and a delivery of it: both existed before M9.
	extra := `
UPDATE account_channels SET events = '["new_content","acquisition","errors","invitations","announcements"]'
WHERE channel = 'email';
INSERT INTO deliveries (account_id, channel, event, subject, body, state, created_at)
SELECT id, 'email', 'acquisition', 'Beschaffung: Dune', 'Neuer Stand: verfügbar', 'sent', 1792616612523
FROM accounts LIMIT 1;
INSERT INTO deliveries (account_id, channel, event, subject, body, state, created_at)
SELECT id, 'email', 'errors', 'Fehler', 'Etwas ging schief', 'queued', 1792616612524
FROM accounts LIMIT 1;
`
	replay(t, path, string(script)+extra)

	db, err := Open(context.Background(), Options{
		ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// No channel chooses it any more, and the other four survive in their order, followed by
	// wish_available, which migration 00072 appends to every list, and for an administrator
	// wish_review, which 00073 appends (concept 25 F-14).
	rows, err := db.Read.Query(`SELECT a.role, c.channel, c.events FROM account_channels AS c
	                            JOIN accounts AS a ON a.id = c.account_id ORDER BY c.account_id, c.channel`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var role, channel, events string
		if err := rows.Scan(&role, &channel, &events); err != nil {
			t.Fatal(err)
		}
		seen++
		if strings.Contains(events, "acquisition") {
			t.Errorf("channel %s still chooses the event: %s", channel, events)
		}
		want := `["new_content","errors","invitations","announcements","wish_available"]`
		if role == "admin" {
			want = `["new_content","errors","invitations","announcements","wish_available","wish_review"]`
		}
		if channel == "email" && events != want {
			t.Errorf("channel email of a %s keeps %s, want %s", role, events, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("no channel rows after the migration")
	}

	// The delivery of the gone event is gone; the other one survived with its columns.
	var deliveries, acquisition int
	if err := db.Read.QueryRow(`SELECT count(*) FROM deliveries`).Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRow(`SELECT count(*) FROM deliveries WHERE event = 'acquisition'`).Scan(&acquisition); err != nil {
		t.Fatal(err)
	}
	if acquisition != 0 {
		t.Errorf("%d acquisition deliveries survived", acquisition)
	}
	var subject, state string
	if err := db.Read.QueryRow(`SELECT subject, state FROM deliveries WHERE event = 'errors'`).Scan(&subject, &state); err != nil {
		t.Fatalf("the other delivery did not survive the rebuild: %v", err)
	}
	if subject != "Fehler" || state != "queued" {
		t.Errorf("delivery after the rebuild: subject %q state %q", subject, state)
	}

	// The rebuilt table refuses the event, and its indexes are back.
	if _, err := db.Write.Exec(`INSERT INTO deliveries (account_id, channel, event, subject, body, created_at)
	                            SELECT id, 'email', 'acquisition', 's', 'b', 1 FROM accounts LIMIT 1`); err == nil {
		t.Error("the rebuilt deliveries table still accepts the event")
	}
	for _, index := range []string{"deliveries_state", "deliveries_account", "deliveries_dedupe"} {
		var n int
		if err := db.Read.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&n); err != nil || n != 1 {
			t.Errorf("index %s missing after the rebuild (%v)", index, err)
		}
	}
}
