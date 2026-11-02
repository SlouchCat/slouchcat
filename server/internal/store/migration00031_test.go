package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00031 (concept 11 step 2, ADR 0019) creates activity_events: STRICT, empty, kind
// without CHECK (a new kind breaks nothing), detail a JSON object, no foreign keys (an entry
// outlives its title and its library), the indexes on the time and the account, and the triggers
// that delete the rows of a deleted account or profile in the same statement.
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
	// Deleting an account or a profile deletes its rows in the same statement (the triggers).
	var accountID, profileID, otherProfile int64
	if err := db.Read.QueryRowContext(ctx, `SELECT account_id, id FROM profiles ORDER BY id LIMIT 1`).Scan(&accountID, &profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO accounts (email, password_hash, role, is_owner, created_at, updated_at, max_rating, all_libraries)
		VALUES ('trigger@example.test', 'x', 'user', 0, 1, 1, 18, 1)`); err != nil {
		t.Fatal(err)
	}
	var guest int64
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM accounts WHERE email = 'trigger@example.test'`).Scan(&guest); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO profiles (account_id, name, max_rating, language, all_libraries, is_default, created_at, updated_at)
		VALUES (?, 'Gast', 12, 'de', 1, 1, 1, 1)`, guest); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM profiles WHERE account_id = ?`, guest).Scan(&otherProfile); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO activity_events (at, kind, account_id, profile_id) VALUES (1, 'playback_started', ?, ?), (1, 'playback_started', ?, ?), (1, 'server_restart', ?, NULL)`,
		accountID, profileID, guest, otherProfile, guest); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, guest); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM activity_events WHERE account_id = ? OR profile_id = ?`, guest, otherProfile).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows of the deleted account: n=%d err=%v", n, err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM activity_events WHERE profile_id = ?`, profileID).Scan(&n); err != nil || n != 1 {
		t.Errorf("rows of another account: n=%d err=%v, want 1", n, err)
	}

	var violations int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Errorf("foreign_key_check: %d, %v", violations, err)
	}
}
