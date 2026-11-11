package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 19 part B stage B2 (migration 00051; testdata/db/radio-wikidata.sql)
// carries the matches of one run of radio.wikidata.sync on the offer of radio-angebot.sql: an
// owner with its label, an owner without label, an item without owner, founding dates of the
// three precisions, a marked station of a country not chosen, and the time of the run.
// TestFrozenStatesMigrate replays it like every state; this test checks that each of those is
// really in it.
func TestFrozenRadioWikidataStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "radio-wikidata.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"an owner with its label":                 `SELECT count(*) FROM radio_station_wikidata WHERE owner_qid IS NOT NULL AND owner IS NOT NULL`,
		"an owner without label":                  `SELECT count(*) FROM radio_station_wikidata WHERE owner_qid IS NOT NULL AND owner IS NULL`,
		"an item without owner":                   `SELECT count(*) FROM radio_station_wikidata WHERE owner_qid IS NULL`,
		"a founding year":                         `SELECT count(*) FROM radio_station_wikidata WHERE length(founded) = 4`,
		"a founding month":                        `SELECT count(*) FROM radio_station_wikidata WHERE length(founded) = 7`,
		"a founding day":                          `SELECT count(*) FROM radio_station_wikidata WHERE length(founded) = 10`,
		"a match of a marked station elsewhere":   `SELECT count(*) FROM radio_station_wikidata w JOIN radio_stations s ON s.id = w.station_id JOIN radio_offer_exceptions e ON e.station_id = s.id WHERE e.offered = 1 AND s.country NOT IN (SELECT country FROM radio_offer_countries)`,
		"no match of a station outside the offer": `SELECT count(*) = 0 FROM radio_station_wikidata w JOIN radio_offer_exceptions e ON e.station_id = w.station_id WHERE e.offered = 0`,
		"the setting on and the run recorded":     `SELECT count(*) FROM radio_config WHERE wikidata_enabled = 1 AND wikidata_synced_at IS NOT NULL`,
		"migration 00051 recorded as applied":     `SELECT count(*) FROM goose_db_version WHERE version_id = 51 AND is_applied = 1`,
		"foreign keys of the frozen state hold":   `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen radio wikidata state has no %s", what)
		}
	}
}
