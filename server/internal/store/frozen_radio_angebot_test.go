package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 19 part B stage B1 (migration 00048; testdata/db/radio-angebot.sql)
// carries the prepared offer: Switzerland with «all» and a demarked station, Austria with «marked»
// and a marked station, marks of two German stations and an American one without their country,
// the hidden station demarked by the first start, the switch for unreachable stations off, the
// version 2 and the provider of the stations. TestFrozenStatesMigrate replays it like every state;
// this test checks that each of those is really in it.
func TestFrozenRadioAngebotStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "radio-angebot.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a country with all":                             `SELECT count(*) FROM radio_offer_countries WHERE country = 'CH' AND mode = 'all'`,
		"a country with marked":                          `SELECT count(*) FROM radio_offer_countries WHERE country = 'AT' AND mode = 'marked'`,
		"a demarked station of a country with all":       `SELECT count(*) FROM radio_offer_exceptions e JOIN radio_stations s ON s.id = e.station_id WHERE e.offered = 0 AND s.country = 'CH'`,
		"a marked station of a country with marked":      `SELECT count(*) FROM radio_offer_exceptions e JOIN radio_stations s ON s.id = e.station_id WHERE e.offered = 1 AND s.country = 'AT'`,
		"marks of a country not chosen":                  `SELECT count(*) >= 3 FROM radio_offer_exceptions e JOIN radio_stations s ON s.id = e.station_id WHERE e.offered = 1 AND s.country IN ('DE', 'US')`,
		"the hidden station demarked":                    `SELECT count(*) FROM radio_offer_exceptions e JOIN radio_stations s ON s.id = e.station_id WHERE e.offered = 0 AND s.hidden = 1`,
		"the offer prepared, version 2, hide_broken off": `SELECT count(*) FROM radio_config WHERE offer_seeded = 1 AND offer_version = 2 AND hide_broken = 0`,
		"the countries setting kept unread":              `SELECT count(*) FROM radio_config WHERE countries = '["AT","CH","DE"]'`,
		"providers of the catalog stations":              `SELECT count(*) = 58 FROM radio_stations WHERE source = 'catalog' AND provider IS NOT NULL`,
		"no provider for an IP address":                  `SELECT count(*) FROM radio_stations WHERE source = 'own' AND provider IS NULL`,
		"no tag rules, manual facets or facet locks":     `SELECT (SELECT count(*) FROM radio_tag_rules) + (SELECT count(*) FROM radio_station_facets WHERE manual = 1) + (SELECT count(*) FROM radio_station_locks WHERE field IN ('genre', 'kind', 'mood')) = 0`,
		"migration 00048 recorded as applied":            `SELECT count(*) FROM goose_db_version WHERE version_id = 48 AND is_applied = 1`,
		"foreign keys of the frozen state hold":          `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen radio offer state has no %s", what)
		}
	}
}
