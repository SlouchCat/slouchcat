package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen M5 state carries every kind of M5 data (testdata/db/README.md), so later migrations
// are tested against downloads, tombstones, rules, offline PIN material and offline events.
func TestFrozenM5StateCoversM5Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M5.sql"))
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
		"ready download":         `SELECT count(*) FROM downloads WHERE state = 'ready' AND size_bytes > 0`,
		"fetched download":       `SELECT count(*) FROM downloads WHERE fetched_at IS NOT NULL`,
		"season download":        `SELECT count(*) FROM (SELECT d.device_id FROM downloads d JOIN items i ON i.id = d.item_id WHERE i.kind = 'episode' GROUP BY d.device_id HAVING count(*) > 1)`,
		"allowed profiles":       `SELECT count(*) FROM downloads WHERE json_array_length(allowed_profile_ids) > 0`,
		"tombstone":              `SELECT count(*) FROM download_tombstones`,
		"auto-download rule":     `SELECT count(*) FROM download_rules WHERE enabled = 1`,
		"pin salt":               `SELECT count(*) FROM profiles WHERE pin_hash IS NOT NULL AND length(pin_salt) = 32`,
		"offline pin":            `SELECT count(*) FROM profiles WHERE json_valid(offline_pin) AND json_extract(offline_pin, '$.iterations') > 0`,
		"offline progress event": `SELECT count(*) FROM progress_events WHERE offline = 1 AND uploaded_at IS NOT NULL AND download_id IS NOT NULL`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M5 state has no %s", what)
		}
	}
}
