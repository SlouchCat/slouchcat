package store

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
)

// Migration 00080 (concept 25 step 4.5, data model 2.11) on the newest frozen state
// (ordner-metadaten.sql) with a Radarr integration and its log written before: the integration,
// its log and the trigger that keeps 20 entries stay; the kind seerr and the result sync_queued
// are accepted, an unknown kind and an unknown result are not.
func TestMigration00080WebhookSeerr(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	state := frozenState(t, "ordner-metadaten.sql")
	replay(t, path, state)
	raw := openRaw(t, path)
	for _, q := range []string{
		`INSERT INTO webhook_integrations (id, name, kind, token_hash, token_tail, path_map, last_seen_at, created_at, updated_at)
		 VALUES (7, 'Radarr', 'radarr', printf('%.64c', 'a'), 'abcd', '[{"from":"/movies","to":"/media/Filme"}]', 5, 1, 2)`,
		`INSERT INTO webhook_events (id, integration_id, received_at, event, reported_path, resolved_path, result)
		 VALUES (3, 7, 5, 'Download', '/movies/A', '/media/Filme/A', 'scan_queued')`,
	} {
		if _, err := raw.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	_ = raw.Close()
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 80 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM webhook_integrations WHERE id = 7 AND name = 'Radarr' AND kind = 'radarr' AND token_tail = 'abcd'
		 AND path_map = '[{"from":"/movies","to":"/media/Filme"}]' AND last_seen_at = 5 AND created_at = 1 AND updated_at = 2`: 1,
		`SELECT count(*) FROM webhook_events WHERE id = 3 AND integration_id = 7 AND result = 'scan_queued'`: 1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'webhook_events_integration'`:    1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'webhook_events_trim'`:         1,
		`SELECT count(*) FROM sqlite_master WHERE name LIKE 'webhook_%_new'`:                                 0,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	exec := func(query string) error {
		_, err := db.Write.ExecContext(ctx, query)
		return err
	}
	if err := exec(`INSERT INTO webhook_integrations (id, name, kind, token_hash, token_tail, created_at, updated_at)
	                VALUES (8, 'Seerr', 'seerr', printf('%.64c', 'b'), 'efgh', 1, 1)`); err != nil {
		t.Fatalf("seerr kind: %v", err)
	}
	for i := range 25 {
		if err := exec(fmt.Sprintf(`INSERT INTO webhook_events (integration_id, received_at, event, result)
		                            VALUES (8, %d, 'MEDIA_PENDING', 'sync_queued')`, i)); err != nil {
			t.Fatalf("sync_queued: %v", err)
		}
	}
	if got := count(`SELECT count(*) FROM webhook_events WHERE integration_id = 8`); got != 20 {
		t.Errorf("seerr log rows = %d, want 20 (trigger)", got)
	}
	if got := count(`SELECT count(*) FROM webhook_events WHERE integration_id = 7`); got != 1 {
		t.Errorf("radarr log rows = %d, want 1", got)
	}
	for name, q := range map[string]string{
		"unknown kind":   `INSERT INTO webhook_integrations (name, kind, token_hash, token_tail, created_at, updated_at) VALUES ('X', 'jellyseerr', printf('%.64c', 'c'), 'ijkl', 1, 1)`,
		"unknown result": `INSERT INTO webhook_events (integration_id, received_at, event, result) VALUES (8, 1, 'x', 'queued')`,
		"no integration": `INSERT INTO webhook_events (integration_id, received_at, event, result) VALUES (99, 1, 'x', 'ignored')`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: statement succeeded, want a constraint error", name)
		}
	}
	if err := exec(`DELETE FROM webhook_integrations WHERE id = 8`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM webhook_events WHERE integration_id = 8`); got != 0 {
		t.Errorf("log of a deleted integration = %d rows, want 0 (cascade)", got)
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
