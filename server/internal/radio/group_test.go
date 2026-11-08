package radio

import "testing"

// The group key folds bitrate and format suffixes away and falls back from the homepage to the
// stream host (m-7); without any host the entry never groups.
func TestGroupKeyWithoutHomepage(t *testing.T) {
	for _, tc := range []struct {
		e    rbStation
		want string
	}{
		{rbStation{Name: "SRF 3 (AAC) 128k", CountryCode: "ch", Homepage: "https://www.srf.ch/radio-srf-3"}, "srf 3|CH|srf.ch"},
		{rbStation{Name: "SRF 3 - MP3", CountryCode: "CH", Homepage: "https://srf.ch/"}, "srf 3|CH|srf.ch"},
		{rbStation{Name: "Antenne Bayern [64]", CountryCode: "DE", Homepage: "http://www.antenne.de"}, "antenne bayern|DE|antenne.de"},
		{rbStation{Name: "Radio X", CountryCode: "CH", URLResolved: "https://Stream.Example.org:8000/x.mp3", URL: "https://other.example/x.pls"}, "radio x|CH|stream.example.org"},
		{rbStation{Name: "Radio X", CountryCode: "CH", URL: "http://www.icecast.example/x"}, "radio x|CH|icecast.example"},
		{rbStation{StationUUID: "u-1", Name: "Radio X", CountryCode: "CH", URL: "not a url"}, "uuid:u-1"},
		{rbStation{StationUUID: "u-2", Name: "128k", CountryCode: "CH", Homepage: "https://a.example"}, "uuid:u-2"},
	} {
		if got := groupKey(&tc.e); got != tc.want {
			t.Errorf("groupKey(%q, %q) = %q, want %q", tc.e.Name, tc.e.Homepage, got, tc.want)
		}
	}
}

// The display name of a station prefers an entry without a suffix.
func TestStationNameWithoutSuffix(t *testing.T) {
	got := stationName([]entryMeta{{name: "Bayern 3 HD"}, {name: "Bayern 3"}})
	if got != "Bayern 3" {
		t.Errorf("stationName = %q, want Bayern 3", got)
	}
	if got := stationName([]entryMeta{{name: "Radio Paradise 320k"}, {name: "Radio Paradise FLAC"}}); got != "Radio Paradise 320k" {
		t.Errorf("stationName without a clean entry = %q, want the best entry's", got)
	}
}
