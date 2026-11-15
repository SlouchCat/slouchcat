package model

import "slices"

// Rating countries and their age steps (concept 22 section 4.11, ADR 0034). A stored rating is
// the minimum age of the title (F-17: PG-13 = 13, R = 17), not a rounded FSK step, and a profile
// picks its ceiling from the steps of the rating country (F-12). The table is data, not text
// (CLAUDE.md rule 9, named exception): a label is the short name a country prints on the
// certificate («PG-13», «-12», «R15+»); a step whose certificate is the bare number carries no
// label, the interface then shows its own words («ab 12»). Name is the body that rates, a name
// as well and never translated. Which certificate strings of TMDB and TVDB map to which age is
// package metadata's table (metadata/ratings.go).

// SettingRatingCountry is the settings key of the rating country (package metadata writes it).
const SettingRatingCountry = "metadata.rating_country"

// SettingMinimumAgeNotice is set by migration 00064 when the database held ratings of a provider:
// the administration shows once that ratings are minimum ages now, until it is closed.
const SettingMinimumAgeNotice = "metadata.minimum_age_notice"

// JobRatingsReread is the job «Freigaben neu lesen» of package metadata (ratings_reread.go).
const JobRatingsReread = "metadata.ratings_reread"

// MaxRatingAge is the highest minimum age: unknown counts as it (AN-1), administrators have it
// (migration 00022), and every country's steps end with it.
const MaxRatingAge = 18

// DefaultRatingCountry is the rating country of a server without the setting and the fallback
// of a title without a certificate of the chosen country (F-12).
const DefaultRatingCountry = "DE"

// RatingStep is one age step of a rating country.
type RatingStep struct {
	Age   int
	Label string // the certificate name, "" when it is the bare number
}

// RatingSystem is the rating country with its body and its age steps, lowest first.
type RatingSystem struct {
	Country string // ISO 3166-1 alpha-2
	Name    string // the rating body ("FSK", "BBFC"), "" when there is none to name
	Steps   []RatingStep
}

func steps(ages []int, labels ...string) []RatingStep {
	out := make([]RatingStep, len(ages))
	for i, a := range ages {
		out[i].Age = a
		if i < len(labels) {
			out[i].Label = labels[i]
		}
	}
	return out
}

// ratingSystems are the rating countries of the 23 interface languages (table 4.1) plus BE, in
// the order of table 4.11. A country without a certificate of 18 still ends with the step 18,
// the ceiling that shows everything (unknown included).
var ratingSystems = []RatingSystem{
	{"DE", "FSK", steps([]int{0, 6, 12, 16, 18}, "FSK 0", "FSK 6", "FSK 12", "FSK 16", "FSK 18")},
	{"AT", "JMK", steps([]int{0, 6, 10, 12, 14, 16, 18})},
	{"CH", "", steps([]int{0, 6, 7, 8, 10, 12, 14, 16, 18})},
	{"GB", "BBFC", steps([]int{0, 6, 12, 15, 18}, "U", "PG", "12A")},
	{"US", "MPA", steps([]int{0, 6, 7, 13, 14, 17, 18}, "G / TV-G", "PG / TV-PG", "TV-Y7", "PG-13", "TV-14", "R / TV-MA", "NC-17")},
	{"FR", "CNC", steps([]int{0, 10, 12, 16, 18}, "Tous publics", "-10", "-12", "-16", "-18")},
	{"ES", "ICAA", steps([]int{0, 7, 12, 16, 18}, "A")},
	{"IT", "", steps([]int{0, 6, 14, 18}, "T", "6+", "14+", "18+")},
	{"NL", "Kijkwijzer", steps([]int{0, 6, 9, 12, 14, 16, 18}, "AL")},
	{"BE", "Kijkwijzer", steps([]int{0, 6, 9, 12, 14, 16, 18}, "AL")},
	{"PT", "CCE", steps([]int{3, 6, 12, 14, 16, 18}, "M/3", "M/6", "M/12", "M/14", "M/16", "M/18")},
	{"BR", "ClassInd", steps([]int{0, 10, 12, 14, 16, 18}, "L")},
	{"PL", "", steps([]int{0, 7, 12, 16, 18})},
	{"SE", "Statens medieråd", steps([]int{0, 7, 11, 15, 18}, "Btl")},
	{"DK", "Medierådet", steps([]int{0, 7, 11, 15, 18}, "A")},
	{"NO", "Medietilsynet", steps([]int{0, 6, 9, 12, 15, 18}, "A")},
	{"FI", "KAVI", steps([]int{0, 7, 12, 16, 18}, "S", "K-7", "K-12", "K-16", "K-18")},
	{"CZ", "", steps([]int{0, 12, 15, 18}, "U")},
	{"HU", "NMHH", steps([]int{0, 6, 12, 16, 18}, "KN")},
	{"TR", "", steps([]int{0, 7, 13, 15, 18}, "Genel İzleyici", "7+", "13+", "15+", "18+")},
	{"RU", "", steps([]int{0, 6, 12, 16, 18}, "0+", "6+", "12+", "16+", "18+")},
	{"UA", "", steps([]int{0, 12, 16, 18})},
	{"JP", "Eirin", steps([]int{0, 12, 15, 18}, "G", "PG12", "R15+", "R18+")},
	{"KR", "KMRB", steps([]int{0, 7, 12, 15, 18}, "ALL")},
	{"TW", "", steps([]int{0, 6, 12, 15, 18}, "0+", "6+", "12+", "15+", "18+")},
}

// RatingSystems returns every rating country in the order of the table.
func RatingSystems() []RatingSystem {
	out := make([]RatingSystem, len(ratingSystems))
	for i, rs := range ratingSystems {
		out[i] = RatingSystem{Country: rs.Country, Name: rs.Name, Steps: slices.Clone(rs.Steps)}
	}
	return out
}

// RatingCountries returns the codes of every rating country in the order of the table.
func RatingCountries() []string {
	out := make([]string, len(ratingSystems))
	for i, rs := range ratingSystems {
		out[i] = rs.Country
	}
	return out
}

// IsRatingCountry reports whether country is a rating country of the table.
func IsRatingCountry(country string) bool {
	return slices.ContainsFunc(ratingSystems, func(rs RatingSystem) bool { return rs.Country == country })
}

// RatingSystemOf returns the system of country; an unknown country gives the default (DE).
func RatingSystemOf(country string) RatingSystem {
	for _, rs := range ratingSystems {
		if rs.Country == country {
			return RatingSystem{Country: rs.Country, Name: rs.Name, Steps: slices.Clone(rs.Steps)}
		}
	}
	return RatingSystemOf(DefaultRatingCountry)
}

// HasAge reports whether age is a step of the system.
func (rs RatingSystem) HasAge(age int) bool {
	return slices.ContainsFunc(rs.Steps, func(s RatingStep) bool { return s.Age == age })
}

// Allows reports whether a ceiling or a rating may be stored: a step of the system, or a value a
// row already has (keep), so a profile keeps its «ab 12» after the household moved from DE to
// US (section 4.11, «Prüfung»).
func (rs RatingSystem) Allows(age int, keep ...int) bool {
	return rs.HasAge(age) || (age >= 0 && age <= MaxRatingAge && slices.Contains(keep, age))
}
