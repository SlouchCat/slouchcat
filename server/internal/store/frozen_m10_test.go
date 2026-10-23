package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen M10 state carries every kind of M10 data (testdata/db/README.md): the M9 webhook
// integrations and event log, plus playback history in both of its forms -- a raw row in
// progress_events and a condensed day in progress_days, written by the server's own condensation
// run (migration 00019). A later migration that touches the playback history has to migrate both.
func TestFrozenM10StateCoversM10Data(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M10.sql"))
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
		"condensed day with played time and plays": `SELECT count(*) FROM progress_days WHERE played_ms > 0 AND plays > 0`,
		"raw event inside the retention window":    `SELECT count(*) FROM progress_events`,
		"resume position":                          `SELECT count(*) FROM progress WHERE position_ms > 0`,
		"webhook event log of M9":                  `SELECT count(*) FROM webhook_events`,
		"scanned film library of the base state":   `SELECT count(*) FROM items WHERE kind = 'movie'`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen M10 state has no %s", what)
		}
	}
}
