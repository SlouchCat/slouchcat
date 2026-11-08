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
	RadioHidden   int64 = 9003 // hidden by the administration (404 everywhere)
	RadioExcluded int64 = 9004 // France, which the countries setting excludes (404 everywhere)
	RadioBroken   int64 = 9005 // Switzerland, unreachable: listed only with hide_broken off
)

// RadioFixture names the stations AddRadio adds.
type RadioFixture struct {
	CH, DE, Hidden, Excluded, Broken int64
}

// AddRadio adds the five radio stations with one stream and the facets genre pop and kind music
// each, limits the catalog to CH and DE (radio_config.countries) and makes RadioDE a favourite of
// P6. Every station exists in the index, so the radio service answers without a sync.
func (f *Fixture) AddRadio(t testing.TB) RadioFixture {
	t.Helper()
	for _, st := range []struct {
		id                int64
		name, country     string
		hidden, reachable int
	}{
		{RadioCH, "Fixture Radio CH", "CH", 0, 1},
		{RadioDE, "Fixture Radio DE", "DE", 0, 1},
		{RadioHidden, "Fixture Radio Hidden", "CH", 1, 1},
		{RadioExcluded, "Fixture Radio FR", "FR", 0, 1},
		{RadioBroken, "Fixture Radio Broken", "CH", 0, 0},
	} {
		key := fmt.Sprintf("fixture radio %d", st.id)
		exec(t, f.DB, `INSERT INTO radio_stations (id, group_key, name, sort_key, search_text, country, languages, votes,
		               quality, reachable, hidden, created_at, updated_at)
		               VALUES (?, ?, ?, ?, ?, ?, '["de"]', 10, 'good', ?, ?, 1, 1)`,
			st.id, key, st.name, key, key+" pop", st.country, st.reachable, st.hidden)
		uuid := fmt.Sprintf("fixture-radio-%d", st.id)
		exec(t, f.DB, `INSERT INTO radio_station_aliases (stationuuid, station_id, seen_at) VALUES (?, ?, 1)`, uuid, st.id)
		exec(t, f.DB, `INSERT INTO radio_streams (station_id, stationuuid, url, codec, bitrate, lastcheckok, updated_at)
		               VALUES (?, ?, ?, 'mp3', 128, ?, 1)`, st.id, uuid, fmt.Sprintf("https://radio.example/%d.mp3", st.id), st.reachable)
		exec(t, f.DB, `INSERT INTO radio_station_facets (station_id, facet, value) VALUES (?, 'genre', 'pop'), (?, 'kind', 'music')`, st.id, st.id)
	}
	exec(t, f.DB, `UPDATE radio_config SET countries = '["CH","DE"]'`)
	exec(t, f.DB, `INSERT INTO radio_favorites (profile_id, station_id, position, created_at) VALUES (?, ?, 0, 1)`, f.P6.ID, RadioDE)
	return RadioFixture{CH: RadioCH, DE: RadioDE, Hidden: RadioHidden, Excluded: RadioExcluded, Broken: RadioBroken}
}
