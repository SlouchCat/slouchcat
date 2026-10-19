package metadata

import "strings"

// ratingCountries is the order in which release certifications are consulted (AN-8): the
// household is in Switzerland and TMDB often lists only AT or CH certifications for German
// language releases.
var ratingCountries = []string{"DE", "AT", "CH", "GB", "US"}

// fskTables maps the certification of each country to the FSK step, rounding up to the next
// higher step (AN-8). Values that are missing here are unknown and skip to the next country.
var fskTables = map[string]map[string]int{
	"DE": {"0": 0, "6": 6, "12": 12, "16": 16, "18": 18},
	"AT": {"0": 0, "6": 6, "10": 12, "12": 12, "14": 16, "16": 16, "18": 18},
	"CH": {"0": 0, "6": 6, "7": 12, "8": 12, "10": 12, "12": 12, "14": 16, "16": 16, "18": 18},
	"GB": {"U": 0, "PG": 6, "12": 12, "12A": 12, "15": 16, "18": 18, "R18": 18},
	"US": {
		"G": 0, "PG": 6, "PG-13": 12, "R": 16, "NC-17": 18, // cinema (MPA)
		"TV-Y": 0, "TV-G": 0, "TV-Y7": 6, "TV-PG": 6, "TV-14": 16, "TV-MA": 18, // TV parental guidelines
	},
}

// TMDB release types that carry the certification of the main release: theatrical, digital and
// physical. Premieres, limited and TV releases are used only when none of these is certified.
var mainReleaseTypes = map[int]bool{3: true, 4: true, 5: true}

// FSKFromReleaseDates maps TMDB release dates to an FSK step (0, 6, 12, 16, 18) after AN-8. It
// returns the rating and its source ("tmdb:DE", "tmdb:AT", …) or nil and "" when no country in
// the order DE → AT → CH → GB → US has a known certification (unknown = NULL, AN-1).
func FSKFromReleaseDates(rd ReleaseDates) (rating *int, source string) {
	byCountry := make(map[string][]Release, len(rd.Results))
	for _, c := range rd.Results {
		code := strings.ToUpper(strings.TrimSpace(c.Country))
		byCountry[code] = append(byCountry[code], c.ReleaseDates...)
	}
	for _, country := range ratingCountries {
		cert := certification(byCountry[country])
		if cert == "" {
			continue
		}
		if fsk, ok := fskTables[country][cert]; ok {
			return &fsk, "tmdb:" + country
		}
	}
	return nil, ""
}

// certification returns the first non-empty certification of a main release, else the first
// non-empty one of any release, normalised to upper case without surrounding blanks.
func certification(releases []Release) string {
	first := ""
	for _, r := range releases {
		c := strings.ToUpper(strings.TrimSpace(r.Certification))
		if c == "" {
			continue
		}
		if mainReleaseTypes[r.Type] {
			return c
		}
		if first == "" {
			first = c
		}
	}
	return first
}
