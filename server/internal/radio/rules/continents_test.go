package rules

import (
	"slices"
	"strings"
	"testing"
)

// iso3166 is the list of ISO 3166-1 alpha-2 codes (249 codes, alphabetical), written down apart
// from the continent table.
const iso3166 = `AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ
BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ
CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ
DE DJ DK DM DO DZ
EC EE EG EH ER ES ET
FI FJ FK FM FO FR
GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY
HK HM HN HR HT HU
ID IE IL IM IN IO IQ IR IS IT
JE JM JO JP
KE KG KH KI KM KN KP KR KW KY KZ
LA LB LC LI LK LR LS LT LU LV LY
MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ
NA NC NE NF NG NI NL NO NP NR NU NZ
OM
PA PE PF PG PH PK PL PM PN PR PS PT PW PY
QA
RE RO RS RU RW
SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ
TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ
UA UG UM US UY UZ
VA VC VE VG VI VN VU
WF WS
YE YT
ZA ZM ZW`

// Every ISO 3166-1 code and Kosovo has exactly one of the six groups of B9.2 (UN M49 joined: the
// Caribbean and Central America in North America, the Antarctic in Oceania, Kosovo in Europe), and
// no other code has one.
func TestContinentsCoverEveryCountryOnce(t *testing.T) {
	want := append(strings.Fields(iso3166), "XK")
	if len(want) != 250 {
		t.Fatalf("the test list has %d codes, want 249 and XK", len(want))
	}
	seen := map[string]int{}
	for _, list := range continentCountries {
		for _, c := range strings.Fields(list) {
			seen[c]++
		}
	}
	for _, c := range want {
		if seen[c] != 1 {
			t.Errorf("%s in %d groups, want 1", c, seen[c])
		}
		if !slices.Contains(Continents, Continent(c)) {
			t.Errorf("Continent(%s) = %q", c, Continent(c))
		}
	}
	if len(seen) != len(want) {
		t.Errorf("%d codes in the table, want %d", len(seen), len(want))
	}
	if len(Continents) != 6 || len(continentCountries) != 6 {
		t.Errorf("groups %v, table %d, want 6", Continents, len(continentCountries))
	}
	for c, cont := range map[string]string{
		"CH": ContinentEurope, "RU": ContinentEurope, "XK": ContinentEurope, "US": ContinentNorthAmerica,
		"MX": ContinentNorthAmerica, "JM": ContinentNorthAmerica, "PA": ContinentNorthAmerica, "GL": ContinentNorthAmerica,
		"BR": ContinentSouthAmerica, "TR": ContinentAsia, "CY": ContinentAsia, "KZ": ContinentAsia, "EG": ContinentAfrica,
		"AU": ContinentOceania, "AQ": ContinentOceania, "": "", "ZZ": "", "ch": "",
	} {
		if got := Continent(c); got != cont {
			t.Errorf("Continent(%q) = %q, want %q", c, got, cont)
		}
	}
}
