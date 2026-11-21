package store

import (
	"context"
	"testing"
)

// Migration 00082 (concept 27 stage 1, ADR 0040): every session that exists before it is a browser
// session, with its expiry untouched (code review m-10).
func TestMigration00082SessionsAreBrowser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	opts := testOptions(t, dir)
	opts.migrations = migrationsBefore(t, "00082")
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db.Write, `INSERT INTO accounts (id, email, password_hash, role, created_at, updated_at) VALUES (1, 'a@example.test', 'x', 'admin', 1, 1)`)
	mustExec(t, db.Write, `INSERT INTO sessions (id, token_hash, account_id, created_at, last_seen_at, expires_at)
	                       VALUES (1, 'h1', 1, 1, 1, 4102444800000), (2, 'h2', 1, 1, 1, 2)`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for query, want := range map[string]int64{
		`SELECT count(*) FROM sessions WHERE kind = 'browser'`:              2,
		`SELECT count(*) FROM sessions WHERE kind <> 'browser'`:             0,
		`SELECT expires_at FROM sessions WHERE id = 1`:                      4102444800000,
		`SELECT count(*) FROM sqlite_master WHERE name = 'sessions_device'`: 1,
	} {
		var got int64
		if err := db.Read.QueryRowContext(ctx, query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
}
