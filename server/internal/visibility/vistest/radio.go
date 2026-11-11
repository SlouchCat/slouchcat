package vistest

import (
	"fmt"
	"testing"
)

// Webradio stations of the contract fixture (concept 16 stage 1): the rows of the S2 visibility
// table with fixed ids, so the registries can name them in their paths.
const (
	RadioCH       int64 = 9001 // visible, Switzerland
	RadioDE       int64 = 9002 // visible, Germany; a favourite of P6
	RadioHidden   int64 = 9003 // Switzerland, demarked: outside the offer (404 everywhere)
	RadioExcluded int64 = 9004 // France, a country the offer does not choose (404 everywhere)
	RadioBroken   int64 = 9005 // Switzerland, unreachable: listed only with hide_broken off
)

// RadioFixture names the stations AddRadio adds.
type RadioFixture struct {
	CH, DE, Hidden, Excluded, Broken int64
}

// AddRadio adds the five radio stations with one stream and the facets genre pop and kind music
// each, chooses CH and DE with «all» for the offer (radio_offer_countries, concept 19 part B),
// demarks RadioHidden (radio_offer_exceptions) and makes RadioDE a favourite of P6. Every station
// exists in the index and the radio service loads the offer at its first use, so it answers
// without a sync and without Start.
func (f *Fixture) AddRadio(t testing.TB) RadioFixture {
	t.Helper()
	for _, st := range []struct {
		id            int64
		name, country string
		reachable     int
	}{
		{RadioCH, "Fixture Radio CH", "CH", 1},
		{RadioDE, "Fixture Radio DE", "DE", 1},
		{RadioHidden, "Fixture Radio Hidden", "CH", 1},
		{RadioExcluded, "Fixture Radio FR", "FR", 1},
		{RadioBroken, "Fixture Radio Broken", "CH", 0},
	} {
		key := fmt.Sprintf("fixture radio %d", st.id)
		exec(t, f.DB, `INSERT INTO radio_stations (id, group_key, name, sort_key, search_text, country, languages, votes,
		               quality, reachable, created_at, updated_at)
		               VALUES (?, ?, ?, ?, ?, ?, '["de"]', 10, 'good', ?, 1, 1)`,
			st.id, key, st.name, key, key+" pop", st.country, st.reachable)
		uuid := fmt.Sprintf("fixture-radio-%d", st.id)
		exec(t, f.DB, `INSERT INTO radio_station_aliases (stationuuid, station_id, seen_at) VALUES (?, ?, 1)`, uuid, st.id)
		exec(t, f.DB, `INSERT INTO radio_streams (station_id, stationuuid, url, codec, bitrate, lastcheckok, updated_at)
		               VALUES (?, ?, ?, 'mp3', 128, ?, 1)`, st.id, uuid, fmt.Sprintf("https://radio.example/%d.mp3", st.id), st.reachable)
		exec(t, f.DB, `INSERT INTO radio_station_facets (station_id, facet, value) VALUES (?, 'genre', 'pop'), (?, 'kind', 'music')`, st.id, st.id)
	}
	exec(t, f.DB, `INSERT INTO radio_offer_countries (country, mode, updated_at) VALUES ('CH', 'all', 1), ('DE', 'all', 1)`)
	exec(t, f.DB, `INSERT INTO radio_offer_exceptions (station_id, offered, updated_at) VALUES (?, 0, 1)`, RadioHidden)
	exec(t, f.DB, `INSERT INTO radio_favorites (profile_id, station_id, position, created_at) VALUES (?, ?, 0, 1)`, f.P6.ID, RadioDE)
	return RadioFixture{CH: RadioCH, DE: RadioDE, Hidden: RadioHidden, Excluded: RadioExcluded, Broken: RadioBroken}
}
