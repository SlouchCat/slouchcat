package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00048 (1.28.0, concept 19 part B stage B1) on the newest frozen state with radio data
// (hoerbuch-quellen.sql, which carries the tag rules, manual facets and facet locks of
// webradio2.sql): the offer tables start empty with version 0 and not prepared, the rules, the
// manual facet values and the locks genre, kind and mood are gone, the other locks, the countries
// setting and the hidden flag stay. The checks of the new tables hold and an exception goes with
// its station.
func TestMigration00048RadioOffer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-quellen.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	text := func(query string) string {
		t.Helper()
		var s string
		if err := db.Read.QueryRowContext(ctx, query).Scan(&s); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return s
	}
	for query, want := range map[string]string{
		`SELECT count(*) FROM radio_offer_countries`:                                         "0",
		`SELECT count(*) FROM radio_offer_exceptions`:                                        "0",
		`SELECT offer_version || '/' || offer_seeded FROM radio_config`:                      "0/0",
		`SELECT countries FROM radio_config`:                                                 `["AT","CH","DE"]`,
		`SELECT count(*) FROM radio_stations WHERE hidden = 1`:                               "1",
		`SELECT count(*) FROM radio_stations WHERE provider IS NOT NULL`:                     "0",
		`SELECT count(*) FROM radio_tag_rules`:                                               "0",
		`SELECT count(*) FROM radio_station_facets WHERE manual = 1`:                         "0",
		`SELECT count(*) FROM radio_station_locks WHERE field IN ('genre', 'kind', 'mood')`:  "0",
		`SELECT group_concat(station_id || ':' || field) FROM radio_station_locks`:           "5:name,59:region",
		`SELECT count(*) FROM radio_station_facets WHERE station_id = 49 AND facet = 'mood'`: "0",
		`SELECT count(*) FROM radio_favorites`:                                               "6",
		`SELECT count(*) FROM pragma_foreign_key_check`:                                      "0",
		`SELECT count(*) FROM goose_db_version WHERE version_id = 48 AND is_applied = 1`:     "1",
	} {
		if got := text(query); got != want {
			t.Errorf("%s = %s, want %s", query, got, want)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO radio_offer_countries (country, mode, updated_at) VALUES ('ch', 'all', 1)`,
		`INSERT INTO radio_offer_countries (country, mode, updated_at) VALUES ('CHE', 'all', 1)`,
		`INSERT INTO radio_offer_countries (country, mode, updated_at) VALUES ('CH', 'some', 1)`,
		`INSERT INTO radio_offer_exceptions (station_id, offered, updated_at) VALUES (1, 2, 1)`,
		`INSERT INTO radio_offer_exceptions (station_id, offered, updated_at) VALUES (9999, 1, 1)`,
		`UPDATE radio_config SET offer_seeded = 2`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err == nil {
			t.Errorf("accepted: %s", stmt)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO radio_offer_countries (country, mode, updated_at) VALUES ('CH', 'all', 1), ('DE', 'marked', 1)`,
		`INSERT INTO radio_offer_exceptions (station_id, offered, updated_at) VALUES (1, 0, 1), (25, 1, 1)`,
		`DELETE FROM radio_stations WHERE id = 25`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if got := text(`SELECT group_concat(station_id) FROM radio_offer_exceptions`); got != "1" {
		t.Errorf("exceptions after the delete of station 25 = %s, want 1", got)
	}
}
