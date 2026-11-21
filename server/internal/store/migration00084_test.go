package store

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Migrations 00084 and 00085 (concept 27 stage 1, ADR 0040): 00084 rebuilds deliveries to take
// the event device_linked, 00085 adds deliveries.pushed_to and rebuilds pairing_vouchers with the
// issuing session. Every delivery that exists before keeps every column, the three indexes come
// back, and the new values work.
func TestMigrations00084And00085KeepTheDeliveries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	opts := testOptions(t, dir)
	opts.migrations = migrationsBefore(t, "00084")
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db.Write, `INSERT INTO accounts (id, email, password_hash, role, created_at, updated_at) VALUES (1, 'a@example.test', 'x', 'admin', 1, 1)`)
	mustExec(t, db.Write, `INSERT INTO profiles (id, account_id, name, max_rating, all_libraries, is_default, created_at, updated_at) VALUES (5, 1, 'P', 18, 1, 1, 1, 1)`)
	mustExec(t, db.Write, `INSERT INTO deliveries (id, account_id, profile_id, channel, event, source, subject, body, link, state, attempts, last_error, created_at, sent_at) VALUES
		(1, 1, 5, 'email', 'new_content', NULL, 'Neue Folge', 'S01E02', '/items/x', 'sent', 1, NULL, 10, 11),
		(2, 1, NULL, 'push', 'errors', 'job:scan', 'Fehler', 'Ein Job', NULL, 'failed', 5, 'push service answered 500', 20, NULL),
		(3, 1, 5, 'telegram', 'wish_review', 'wish:1:review', 'Freigabe', 'Wartet', '/admin/wishes', 'queued', 0, NULL, 30, NULL),
		(4, 1, NULL, 'email', 'announcements', NULL, 'Hinweis', 'Text', NULL, 'skipped', 0, 'channel off', 40, NULL)`)
	const columns = `id, account_id, profile_id, channel, event, source, subject, body, link, state, attempts, last_error, created_at, sent_at`
	before := rowsAsText(t, db, `SELECT `+columns+` FROM deliveries ORDER BY id`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if after := rowsAsText(t, db, `SELECT `+columns+` FROM deliveries ORDER BY id`); !slices.Equal(before, after) {
		t.Fatalf("deliveries changed by the migrations:\nbefore %v\nafter  %v", before, after)
	}
	for query, want := range map[string]int64{
		`SELECT count(*) FROM goose_db_version WHERE version_id IN (84, 85) AND is_applied = 1`:                                               2,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                                       0,
		`SELECT count(*) FROM sqlite_master WHERE name = 'deliveries_new'`:                                                                    0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('deliveries_state', 'deliveries_account', 'deliveries_dedupe')`: 3,
		`SELECT count(*) FROM deliveries WHERE pushed_to IS NULL`:                                                                             4,
		`SELECT count(*) FROM pragma_table_info('pairing_vouchers') WHERE name = 'session_id' AND "notnull" = 1`:                              1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('pairing_vouchers_expiry', 'pairing_vouchers_session')`:         2,
	} {
		var got int64
		if err := db.Read.QueryRowContext(ctx, query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	// The new values: an event device_linked with the subscriptions it reached; an unknown event and
	// pushed_to that is no JSON stay refused.
	mustExec(t, db.Write, `INSERT INTO deliveries (account_id, channel, event, subject, body, created_at, pushed_to) VALUES (1, 'push', 'device_linked', 'Neues Gerät', 'TV', 50, '[3,7]')`)
	for name, q := range map[string]string{
		"unknown event":          `INSERT INTO deliveries (account_id, channel, event, subject, body, created_at) VALUES (1, 'push', 'no_such_event', 's', 'b', 60)`,
		"pushed_to not JSON":     `INSERT INTO deliveries (account_id, channel, event, subject, body, created_at, pushed_to) VALUES (1, 'push', 'device_linked', 's', 'b', 60, 'not json')`,
		"voucher without issuer": `INSERT INTO pairing_vouchers (account_id, voucher_hash, tv_key, device_kind, created_at, expires_at) VALUES (1, '` + strings.Repeat("a", 64) + `', '` + strings.Repeat("b", 43) + `', 'tv', 1, 2)`,
	} {
		if _, err := db.Write.ExecContext(ctx, q); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// rowsAsText returns every row of query as text, one string per row.
func rowsAsText(t *testing.T, db *DB, query string) []string {
	t.Helper()
	rows, err := db.Read.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprint(vals...))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
