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

// The frozen M9 state carries every kind of M9 data (testdata/db/README.md): two webhook
// integrations, one of them with a path mapping and a last contact, and an event log that holds
// every result a webhook can have. Later migrations are tested against it.
func TestFrozenM9StateCoversM9Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M9.sql"))
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
		"radarr integration with a path mapping": `SELECT count(*) FROM webhook_integrations
		                                           WHERE kind = 'radarr' AND json_array_length(path_map) = 1
		                                             AND last_seen_at IS NOT NULL`,
		"sonarr integration that never reported": `SELECT count(*) FROM webhook_integrations
		                                           WHERE kind = 'sonarr' AND path_map = '[]' AND last_seen_at IS NULL`,
		"queued scan with both paths":            `SELECT count(*) FROM webhook_events WHERE result = 'scan_queued' AND reported_path IS NOT NULL AND resolved_path IS NOT NULL`,
		"acknowledged test":                      `SELECT count(*) FROM webhook_events WHERE result = 'acknowledged' AND event = 'Test'`,
		"ignored event":                          `SELECT count(*) FROM webhook_events WHERE result = 'ignored'`,
		"path outside every root":                `SELECT count(*) FROM webhook_events WHERE result = 'outside_roots'`,
		"acting event without a path":            `SELECT count(*) FROM webhook_events WHERE result = 'invalid_payload'`,
		"scanned film library of the base state": `SELECT count(*) FROM items WHERE kind = 'movie'`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M9 state has no %s", what)
		}
	}
	// The tokens exist only as their SHA-256; neither they nor the account password are in the
	// clear (A-9, 3.8 points 2 and 3). The hash is 64 hex characters.
	var plain int
	if err := db.Read.QueryRow(`SELECT count(*) FROM webhook_integrations
	                            WHERE length(token_hash) <> 64 OR token_hash GLOB '*[^0-9a-f]*'`).Scan(&plain); err != nil {
		t.Fatal(err)
	}
	if plain != 0 {
		t.Errorf("%d integrations store something other than a SHA-256", plain)
	}
	if strings.Contains(string(script), "frozen-password-0123") {
		t.Error("frozen M9 state contains the plain account password")
	}
	// The event of the removed acquisition channel is gone everywhere (M9 step 4).
	if strings.Contains(string(script), "acquisition") {
		t.Error("frozen M9 state still mentions the acquisition event")
	}
}
