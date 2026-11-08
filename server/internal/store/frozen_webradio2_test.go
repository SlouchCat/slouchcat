package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 16 stage 2 (migration 00039; testdata/db/webradio2.sql) carries the
// data of the stage: the administration's tag rules (a value, a null value that takes a default
// away and an ignored tag), an own station with its stream, languages, manual facets and locks and a
// favourite, the cover cache with a hit and a miss, the cover switches (iTunes off) and the start
// page row «Meine Sender» shown. TestFrozenStatesMigrate replays it like every state; this test
// checks that each of those is really in it.
func TestFrozenWebradio2StateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "webradio2.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	own := `(SELECT id FROM radio_stations WHERE source = 'own')`
	for what, query := range map[string]string{
		"a tag rule with a value":               `SELECT count(*) FROM radio_tag_rules WHERE facet = 'genre' AND value = 'pop'`,
		"a tag rule without a value":            `SELECT count(*) FROM radio_tag_rules WHERE facet = 'kind' AND value IS NULL`,
		"an ignored tag":                        `SELECT count(*) FROM radio_tag_rules WHERE facet = 'ignore'`,
		"one own station":                       `SELECT count(*) = 1 FROM radio_stations WHERE source = 'own' AND group_key LIKE 'own:%' AND languages = '["de"]'`,
		"the stream of the own station":         `SELECT count(*) FROM radio_streams WHERE station_id = ` + own + ` AND stationuuid IS NULL AND preferred = 1`,
		"manual facets of the own station":      `SELECT count(*) FROM radio_station_facets WHERE station_id = ` + own + ` AND manual = 1`,
		"locks of the own station":              `SELECT count(*) FROM radio_station_locks WHERE station_id = ` + own + ` AND field = 'genre'`,
		"the own station as a favourite":        `SELECT count(*) FROM radio_favorites WHERE station_id = ` + own,
		"a found cover":                         `SELECT count(*) FROM radio_tracks WHERE state = 'found' AND source = 'musicbrainz' AND image_hash IS NOT NULL`,
		"a missing cover":                       `SELECT count(*) FROM radio_tracks WHERE state = 'none' AND image_hash IS NULL`,
		"the iTunes search switched off":        `SELECT count(*) FROM radio_config WHERE cover_musicbrainz = 1 AND cover_itunes = 0`,
		"the row «Meine Sender» shown":          `SELECT count(*) FROM home_rows WHERE id = 'radio' AND key = 'radio' AND hidden = 0`,
		"the stage 1 data kept":                 `SELECT count(*) = 58 FROM radio_stations WHERE source = 'catalog'`,
		"migration 00039 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 39 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen webradio2 state has no %s", what)
		}
	}
}
