package store

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// The frozen state after concept 16 stage 1 (migration 00038; testdata/db/webradio.sql) carries the
// data of the webradio: the synced catalog (68 entries as 58 stations with aliases, streams, tags
// and facets), favourites of two profiles with their positions, locks (a renamed station, a manual
// mood), a hidden and a recommended station, the profile state, an ended listen session, listen
// time per quarter hour and the radio config with the state of the sync. TestFrozenStatesMigrate
// replays it like every state; this test checks that each of those is really in it. Migration 00048
// (concept 19 part B) discards the manual mood and its lock, so those are checked in the script.
func TestFrozenWebradioStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	script := frozenState(t, "webradio.sql")
	replay(t, cfg+"/"+FileName, script)
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"catalog stations":                      `SELECT count(*) = 58 FROM radio_stations WHERE source = 'catalog'`,
		"a station with two aliases":            `SELECT count(*) FROM (SELECT station_id FROM radio_station_aliases GROUP BY station_id HAVING count(*) = 2)`,
		"an alias for every catalog entry":      `SELECT count(*) = 68 FROM radio_station_aliases WHERE catalog_changed IS NOT NULL`,
		"a stream for every alias":              `SELECT count(*) = (SELECT count(*) FROM radio_station_aliases) FROM radio_streams WHERE stationuuid IS NOT NULL`,
		"a station with several streams":        `SELECT count(*) FROM (SELECT station_id FROM radio_streams GROUP BY station_id HAVING count(*) > 1)`,
		"raw tags":                              `SELECT count(*) FROM radio_station_tags`,
		"derived genres":                        `SELECT count(*) FROM radio_station_facets WHERE facet = 'genre' AND manual = 0`,
		"derived kinds":                         `SELECT count(*) FROM radio_station_facets WHERE facet = 'kind' AND manual = 0`,
		"derived moods":                         `SELECT count(*) FROM radio_station_facets WHERE facet = 'mood' AND manual = 0`,
		"no manual facet after 00048":           `SELECT count(*) = 0 FROM radio_station_facets WHERE manual = 1`,
		"a renamed station with its lock":       `SELECT count(*) FROM radio_stations s JOIN radio_station_locks l ON l.station_id = s.id AND l.field = 'name' WHERE s.name = 'Radio Swiss Jazz (Marco)'`,
		"a hidden station":                      `SELECT count(*) FROM radio_stations WHERE hidden = 1`,
		"a recommended station":                 `SELECT count(*) FROM radio_stations WHERE recommended = 1`,
		"a station that shows titles":           `SELECT count(*) FROM radio_stations WHERE shows_titles = 1`,
		"an unreachable station":                `SELECT count(*) FROM radio_stations WHERE reachable = 0`,
		"favourites with a position":            `SELECT count(*) FROM radio_favorites WHERE profile_id = 1 AND position = 3`,
		"favourites of a second profile":        `SELECT count(*) FROM radio_favorites WHERE profile_id = 2`,
		"profile state with a last filter":      `SELECT count(*) FROM radio_profile_state WHERE profile_id = 1 AND default_country = 'CH' AND json_valid(last_filter) AND last_sort = 'popular'`,
		"profile state without a last filter":   `SELECT count(*) FROM radio_profile_state WHERE profile_id = 2 AND last_filter IS NULL AND last_sort = 'name'`,
		"an ended listen session":               `SELECT count(*) FROM radio_listen_sessions WHERE ended_at IS NOT NULL AND ended_by = 'client' AND length(id) = 26`,
		"no open listen session":                `SELECT count(*) = 0 FROM radio_listen_sessions WHERE ended_at IS NULL`,
		"listen time over several quarters":     `SELECT count(*) >= 3 FROM radio_listen_time WHERE profile_id = 1`,
		"a full quarter hour of listen time":    `SELECT count(*) FROM radio_listen_time WHERE played_ms = 900000`,
		"radio config with a country subset":    `SELECT count(*) FROM radio_config WHERE id = 1 AND json_valid(countries) AND buffer_minutes = 60 AND max_relays = 6`,
		"radio config with the state of a sync": `SELECT count(*) FROM radio_config WHERE last_sync_at IS NOT NULL AND last_sync_error IS NULL AND entries_total = 68 AND stations_total = 58`,
		"migration 00038 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 38 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen webradio state has no %s", what)
		}
	}
	for _, row := range []string{`INSERT INTO "radio_station_facets" VALUES (49, 'mood', 'focus', 1);`,
		`INSERT INTO "radio_station_locks" VALUES (49, 'mood');`} {
		if !strings.Contains(script, row) {
			t.Errorf("frozen webradio script has no manual facet with its lock: %s", row)
		}
	}
}
