package rules

import "strings"

// The continent of every country (concept 19 part B, B9.2): the subregions of UN M49 joined into six
// groups for the tab «Angebot». The Caribbean and Central America belong to North America, the
// Antarctic (AQ) to Oceania; Kosovo (XK, not in M49) to Europe. The names of the groups live in the
// web locales.

// Continent keys (contract RadioOfferCountry.continent).
const (
	ContinentEurope       = "europe"
	ContinentNorthAmerica = "northAmerica"
	ContinentSouthAmerica = "southAmerica"
	ContinentAsia         = "asia"
	ContinentAfrica       = "africa"
	ContinentOceania      = "oceania"
)

// Continents are the six groups in their display order.
var Continents = []string{ContinentEurope, ContinentNorthAmerica, ContinentSouthAmerica, ContinentAsia,
	ContinentAfrica, ContinentOceania}

// continentCountries lists the ISO 3166-1 alpha-2 codes of each group.
var continentCountries = map[string]string{
	ContinentEurope: "AD AL AT AX BA BE BG BY CH CZ DE DK EE ES FI FO FR GB GG GI GR HR HU IE IM IS IT JE LI LT " +
		"LU LV MC MD ME MK MT NL NO PL PT RO RS RU SE SI SJ SK SM UA VA XK",
	ContinentAsia: "AE AF AM AZ BD BH BN BT CN CY GE HK ID IL IN IQ IR JO JP KG KH KP KR KW KZ LA LB LK MM MN " +
		"MO MV MY NP OM PH PK PS QA SA SG SY TH TJ TL TM TR TW UZ VN YE",
	ContinentAfrica: "AO BF BI BJ BW CD CF CG CI CM CV DJ DZ EG EH ER ET GA GH GM GN GQ GW IO KE KM LR LS LY MA " +
		"MG ML MR MU MW MZ NA NE NG RE RW SC SD SH SL SN SO SS ST SZ TD TF TG TN TZ UG YT ZA ZM ZW",
	ContinentNorthAmerica: "AG AI AW BB BL BM BQ BS BZ CA CR CU CW DM DO GD GL GP GT HN HT JM KN KY LC MF MQ MS MX " +
		"NI PA PM PR SV SX TC TT US VC VG VI",
	ContinentSouthAmerica: "AR BO BR BV CL CO EC FK GF GS GY PE PY SR UY VE",
	ContinentOceania: "AQ AS AU CC CK CX FJ FM GU HM KI MH MP NC NF NR NU NZ PF PG PN PW SB TK TO TV UM VU WF " +
		"WS",
}

// continentOf maps a country to its group.
var continentOf = func() map[string]string {
	m := make(map[string]string, 250)
	for continent, list := range continentCountries {
		for _, c := range strings.Fields(list) {
			if prev, ok := m[c]; ok {
				panic("rules: country " + c + " in " + prev + " and " + continent)
			}
			m[c] = continent
		}
	}
	return m
}()

// Continent returns the continent key of an ISO 3166-1 alpha-2 code, "" for an unknown code.
func Continent(country string) string { return continentOf[country] }
