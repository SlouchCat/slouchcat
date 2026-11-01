package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00031 (concept 11 step 2, ADR 0019) creates activity_events: STRICT, empty, kind
// without CHECK (a new kind breaks nothing), detail a JSON object, no foreign keys (an entry
// outlives its title and its library), and the indexes on the time and the account.
func TestMigration00031CreatesActivityEvents(t *testing.T) {
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "metadaten.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var n int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM activity_events`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("activity_events after 00031: n=%d err=%v (the migration creates it empty)", n, err)
	}
	for _, index := range []string{"activity_events_at", "activity_events_account"} {
		if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&n); err != nil || n != 1 {
			t.Errorf("index %s: n=%d err=%v", index, n, err)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO activity_events (at, kind, account_id, profile_id, item_id, library_id, subject)
		VALUES (1, 'a_kind_from_the_future', 999999, 999999, 'gone', 999999, 'Titel')`); err != nil {
		t.Errorf("an unknown kind and ids without rows should be accepted: %v", err)
	}
	var detail string
	if err := db.Read.QueryRowContext(ctx, `SELECT detail FROM activity_events WHERE kind = 'a_kind_from_the_future'`).Scan(&detail); err != nil || detail != "{}" {
		t.Errorf("default detail = %q, %v; want {}", detail, err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO activity_events (at, kind, detail) VALUES (1, 'x', 'not json')`); err == nil {
		t.Error("invalid JSON in detail should fail the CHECK")
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO activity_events (at, kind) VALUES ('yesterday', 'x')`); err == nil {
		t.Error("STRICT should refuse text in at")
	}
	var violations int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Errorf("foreign_key_check: %d, %v", violations, err)
	}
}
