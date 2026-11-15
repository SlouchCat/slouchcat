package metadata

import "strings"

// ratingFallback is the order in which the certifications of further countries are read when
// the rating country has none (AN-8, decision 6 as amended by ADR 0034): Germany first (F-12),
// then the neighbours and the two big English-speaking markets. The country read first is the
// setting metadata.rating_country (any country of model.RatingSystems); ratingOrder puts it in
// front, the rest keeps this order.
var ratingFallback = []string{"DE", "AT", "CH", "GB", "US"}

// ratingOrder returns the country order of the certifications with country first; an unknown or
// empty country gives the fallback order.
func ratingOrder(country string) []string {
	if _, ok := certificationAges[country]; !ok || country == ratingFallback[0] {
		return ratingFallback
	}
	out := make([]string, 0, len(ratingFallback)+1)
	out = append(out, country)
	for _, c := range ratingFallback {
		if c != country {
			out = append(out, c)
		}
	}
	return out
}

// certificationAges maps the certificates of every rating country, as TMDB and TVDB deliver them
// (normalised by normalizeCertification), to the minimum age (F-17): the real age everywhere,
// never rounded to an FSK step (PG-13 = 13, R = 17, TV-MA = 17, AT 14 = 14). A certificate
// without a fixed age gets the value of table 4.11 (US PG and TV-PG 6, GB PG 6, GB 12A 12, IT BA
// 6); NC-17, R18 and the like are 18, Korea's TV 19 is 18 (the ceiling, model.MaxRatingAge).
// Values missing here are unknown and skip to the next country (TMDB "NR", "Unrated").
var certificationAges = map[string]map[string]int{
	"DE": {"0": 0, "6": 6, "12": 12, "16": 16, "18": 18},
	"AT": {"0": 0, "6": 6, "10": 10, "12": 12, "14": 14, "16": 16, "18": 18},
	"CH": {"0": 0, "6": 6, "7": 7, "8": 8, "10": 10, "12": 12, "14": 14, "16": 16, "18": 18},
	"GB": {"U": 0, "UC": 0, "PG": 6, "12": 12, "12A": 12, "15": 15, "18": 18, "R18": 18},
	"US": {
		"G": 0, "PG": 6, "PG-13": 13, "R": 17, "NC-17": 18, // cinema (MPA)
		"TV-Y": 0, "TV-G": 0, "TV-Y7": 7, "TV-Y7-FV": 7, "TV-PG": 6, "TV-14": 14, "TV-MA": 17, // TV parental guidelines
	},
	"FR": {"U": 0, "TP": 0, "TOUS PUBLICS": 0, "10": 10, "12": 12, "16": 16, "18": 18},
	"ES": {"A": 0, "AI": 0, "APTA": 0, "TP": 0, "I": 0, "7": 7, "10": 10, "12": 12, "13": 13, "16": 16, "18": 18, "X": 18},
	"IT": {"T": 0, "BA": 6, "6": 6, "12": 12, "14": 14, "VM14": 14, "18": 18, "VM18": 18},
	"NL": {"AL": 0, "6": 6, "9": 9, "12": 12, "14": 14, "16": 16, "18": 18},
	"BE": {
		"AL": 0, "KT": 0, "EA": 0, "KT/EA": 0, "6": 6, "9": 9, "12": 12, "14": 14, "16": 16, "18": 18,
		"KNT": 16, "ENA": 16, "KNT/ENA": 16,
	},
	"PT": {
		"PÚBLICOS": 0, "T": 0, "M/3": 3, "M/4": 4, "M/6": 6, "M/12": 12, "M/14": 14, "M/16": 16, "M/18": 18, "P": 18,
		"10AP": 10, "12AP": 12, "16": 16, "18": 18,
	},
	"BR": {"L": 0, "AL": 0, "10": 10, "12": 12, "14": 14, "16": 16, "18": 18},
	"PL": {"0": 0, "AL": 0, "BO": 0, "7": 7, "12": 12, "15": 15, "16": 16, "18": 18, "21": 18},
	"SE": {"BTL": 0, "7": 7, "11": 11, "15": 15},
	"DK": {"A": 0, "7": 7, "11": 11, "15": 15, "F": 15},
	"NO": {"A": 0, "6": 6, "7": 7, "9": 9, "11": 11, "12": 12, "15": 15, "18": 18},
	"FI": {
		"S": 0, "T": 0, "K-7": 7, "K7": 7, "K-12": 12, "K12": 12, "K-16": 16, "K16": 16, "K-18": 18, "K18": 18, "KK": 18,
	},
	"CZ": {"U": 0, "0": 0, "12": 12, "15": 15, "18": 18},
	"HU": {"KN": 0, "KORHATÁR NÉLKÜL": 0, "6": 6, "12": 12, "16": 16, "18": 18, "X": 18},
	"TR": {
		"GENEL İZLEYICI KITLESI": 0, "GENEL İZLEYICI": 0, "GENEL": 0, "6": 6, "6A": 6, "7": 7, "10": 10, "10A": 10,
		"13A": 13, "13": 13, "15": 15, "15A": 15, "16": 16, "18": 18,
	},
	"RU": {"0": 0, "6": 6, "12": 12, "16": 16, "18": 18},
	"UA": {"0": 0, "ЗА": 0, "12": 12, "16": 16, "18": 18},
	"JP": {"G": 0, "PG12": 12, "R15": 15, "R18": 18},
	"KR": {"ALL": 0, "7": 7, "12": 12, "15": 15, "18": 18, "19": 18, "RESTRICTED SCREENING": 18},
	"TW": {"0": 0, "6": 6, "12": 12, "15": 15, "18": 18},
}

// TMDB release types that carry the certification of the main release: theatrical, digital and
// physical. Premieres, limited and TV releases are used only when none of these is certified.
var mainReleaseTypes = map[int]bool{3: true, 4: true, 5: true}

// AgeFromReleaseDates maps TMDB release dates to a minimum age (F-17). It returns the age and its
// source ("tmdb:DE", "tmdb:US", …) or nil and "" when no country in the order DE → AT → CH → GB →
// US has a known certificate (unknown = NULL, AN-1).
func AgeFromReleaseDates(rd ReleaseDates) (rating *int, source string) {
	rating, country := ageFromCertifications(releaseCertifications(rd), ratingFallback[0])
	if rating == nil {
		return nil, ""
	}
	return rating, ProviderTMDB + ":" + country
}

// releaseCertifications returns the certification of the main release per country.
func releaseCertifications(rd ReleaseDates) map[string]string {
	byCountry := make(map[string][]Release, len(rd.Results))
	for _, c := range rd.Results {
		code := strings.ToUpper(strings.TrimSpace(c.Country))
		byCountry[code] = append(byCountry[code], c.ReleaseDates...)
	}
	out := make(map[string]string, len(byCountry))
	for code, releases := range byCountry {
		if cert := certification(releases); cert != "" {
			out[code] = cert
		}
	}
	return out
}

// ageFromCertifications maps one certificate per country (ISO 3166-1 alpha-2) to the minimum age
// and returns it with the country it came from, reading first (the setting
// metadata.rating_country), then DE → AT → CH → GB → US; nil and "" when none of them has a known
// value. Values are compared in upper case; a "FSK" prefix, a leading "-" and a trailing "+" are
// ignored ("FSK 16", "-12", "16+", "R15+"). Whether the age may replace a stored one is the
// caller's rule (match.go, fill.go, ratings_reread.go).
func ageFromCertifications(certs map[string]string, first string) (rating *int, country string) {
	for _, c := range ratingOrder(first) {
		cert := normalizeCertification(certs[c])
		if cert == "" {
			continue
		}
		if age, ok := certificationAges[c][cert]; ok {
			return &age, c
		}
	}
	return nil, ""
}

// normalizeCertification upper-cases a certification and strips "FSK", blanks, a leading "-" and a
// leading or trailing "+".
func normalizeCertification(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(v, "FSK"), "-"))
	// "+6" and "6+" are the same certificate (TMDB lists both spellings in some countries).
	return strings.TrimSuffix(strings.TrimPrefix(v, "+"), "+")
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
