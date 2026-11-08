package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

// Migration 00038 (concept 16 stage 1) creates the webradio tables with one radio_config row holding
// the defaults of F-13, and its CHECKs keep the values the server relies on.
func TestMigration00038RadioSchema(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	ctx := context.Background()
	var buffer, budget, relays, grace, hide, updated int64
	var countries sql.NullString
	if err := db.Read.QueryRowContext(ctx, `SELECT buffer_minutes, budget_mb, max_relays, grace_seconds, hide_broken, countries, updated_at
		FROM radio_config`).Scan(&buffer, &budget, &relays, &grace, &hide, &countries, &updated); err != nil {
		t.Fatal(err)
	}
	if buffer != 30 || budget != 512 || relays != 10 || grace != 120 || hide != 1 || countries.Valid || updated != 0 {
		t.Errorf("radio_config = %d min, %d MB, %d relays, %d s, hide %d, countries %v, updated %d; want the defaults of F-13",
			buffer, budget, relays, grace, hide, countries, updated)
	}
	var rows int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM radio_config`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("radio_config has %d rows, want 1", rows)
	}
	mustExec(t, db.Write, `INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`)
	mustExec(t, db.Write, `INSERT INTO profiles (id, account_id, name, created_at, updated_at) VALUES (1, 1, 'P', 0, 0)`)
	for _, bad := range []string{
		`UPDATE radio_config SET buffer_minutes = 20`,
		`INSERT INTO radio_config (id, updated_at) VALUES (2, 0)`,
		`INSERT INTO radio_stations (source, group_key, name, sort_key, search_text, created_at, updated_at) VALUES ('x', 'k', 'n', 'n', 'n', 0, 0)`,
		`INSERT INTO radio_stations (group_key, name, sort_key, search_text, created_at, updated_at) VALUES ('k', '', '', '', 0, 0)`,
		`INSERT INTO radio_listen_time (profile_id, quarter, played_ms) VALUES (1, 1, 900001)`,
		`INSERT INTO radio_profile_state (profile_id, last_sort, updated_at) VALUES (1, 'votes', 0)`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil || !strings.Contains(err.Error(), "constraint") {
			t.Errorf("%s: err = %v, want a constraint violation", bad, err)
		}
	}
}

// Deleting a station removes everything that hangs on it, favourites included (F-18); deleting a
// profile removes its favourites, profile state, listen sessions and listen time.
func TestMigration00038RadioCascades(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	w := db.Write
	mustExec(t, w, `INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`)
	mustExec(t, w, `INSERT INTO profiles (id, account_id, name, created_at, updated_at) VALUES (1, 1, 'A', 0, 0), (2, 1, 'B', 0, 0)`)
	mustExec(t, w, `INSERT INTO sessions (id, token_hash, account_id, profile_id, created_at, last_seen_at, expires_at) VALUES (1, 'h', 1, 1, 0, 0, 1)`)
	for _, id := range []int64{1, 2} {
		mustExec(t, w, `INSERT INTO radio_stations (id, group_key, name, sort_key, search_text, created_at, updated_at) VALUES (?, 'k', 'S', 's', 's', 0, 0)`, id)
		uuid := []string{"", "uuid-1", "uuid-2"}[id]
		mustExec(t, w, `INSERT INTO radio_station_aliases (stationuuid, station_id, seen_at) VALUES (?, ?, 0)`, uuid, id)
		mustExec(t, w, `INSERT INTO radio_streams (station_id, stationuuid, url, updated_at) VALUES (?, ?, 'https://example.org/s', 0)`, id, uuid)
		mustExec(t, w, `INSERT INTO radio_station_tags (station_id, tag) VALUES (?, 'jazz')`, id)
		mustExec(t, w, `INSERT INTO radio_station_facets (station_id, facet, value) VALUES (?, 'genre', 'jazz')`, id)
		mustExec(t, w, `INSERT INTO radio_station_locks (station_id, field) VALUES (?, 'name')`, id)
		mustExec(t, w, `INSERT INTO radio_favorites (profile_id, station_id, position, created_at) VALUES (1, ?, ?, 0), (2, ?, ?, 0)`, id, id, id, id)
	}
	mustExec(t, w, `INSERT INTO radio_listen_sessions (id, session_id, account_id, profile_id, station_id, created_at, last_seen_at, expires_at)
		VALUES ('01LISTEN000000000000000001', 1, 1, 1, 1, 0, 0, 1), ('01LISTEN000000000000000002', 1, 1, 1, 2, 0, 0, 1)`)
	mustExec(t, w, `INSERT INTO radio_profile_state (profile_id, default_country, updated_at) VALUES (1, 'CH', 0), (2, 'DE', 0)`)
	mustExec(t, w, `INSERT INTO radio_listen_time (profile_id, quarter, played_ms) VALUES (1, 1, 1000), (2, 1, 1000)`)

	count := func(table, where string) int64 {
		t.Helper()
		return queryInt(t, db.Read, `SELECT count(*) FROM `+table+` WHERE `+where).Int64
	}
	mustExec(t, w, `DELETE FROM radio_stations WHERE id = 1`)
	for _, table := range []string{"radio_station_aliases", "radio_streams", "radio_station_tags", "radio_station_facets",
		"radio_station_locks", "radio_favorites", "radio_listen_sessions"} {
		if n := count(table, "station_id = 1"); n != 0 {
			t.Errorf("%s keeps %d rows of the deleted station", table, n)
		}
		if n := count(table, "station_id = 2"); n == 0 {
			t.Errorf("%s lost the rows of the other station", table)
		}
	}

	mustExec(t, w, `DELETE FROM profiles WHERE id = 1`)
	for _, table := range []string{"radio_favorites", "radio_profile_state", "radio_listen_sessions", "radio_listen_time"} {
		if n := count(table, "profile_id = 1"); n != 0 {
			t.Errorf("%s keeps %d rows of the deleted profile", table, n)
		}
	}
	for _, table := range []string{"radio_favorites", "radio_profile_state", "radio_listen_time"} {
		if n := count(table, "profile_id = 2"); n == 0 {
			t.Errorf("%s lost the rows of the other profile", table)
		}
	}
}
