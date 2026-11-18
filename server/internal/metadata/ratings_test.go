package metadata

import (
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func rd(countries ...CountryReleases) ReleaseDates { return ReleaseDates{Results: countries} }

func country(code string, releases ...Release) CountryReleases {
	return CountryReleases{Country: code, ReleaseDates: releases}
}

func theatrical(cert string) Release { return Release{Certification: cert, Type: 3} }

// TestCertificationAges checks every certificate string of every rating country (concept 22
// section 4.11, F-17): the real minimum age, never rounded to an FSK step, with the spellings
// TMDB and TVDB deliver ("FSK 16", "-12", "16+", "R15+", lower case, blanks).
func TestCertificationAges(t *testing.T) {
	t.Parallel()
	cases := map[string]map[string]int{
		"DE": {"0": 0, "6": 6, "12": 12, "16": 16, "18": 18, "FSK 16": 16, "FSK-12": 12, "fsk 6": 6},
		"AT": {"0": 0, "6": 6, "10": 10, "12": 12, "14": 14, "16": 16, "18": 18},
		"CH": {"0": 0, "6": 6, "7": 7, "8": 8, "10": 10, "12": 12, "14": 14, "16": 16, "18": 18},
		"GB": {"U": 0, "UC": 0, "PG": 6, "12": 12, "12A": 12, "15": 15, "18": 18, "R18": 18},
		"US": {
			"G": 0, "PG": 6, "PG-13": 13, "R": 17, "NC-17": 18,
			"TV-Y": 0, "TV-G": 0, "TV-Y7": 7, "TV-Y7-FV": 7, "TV-PG": 6, "TV-14": 14, "TV-MA": 17, " pg-13 ": 13,
		},
		"FR": {"U": 0, "TP": 0, "Tous publics": 0, "10": 10, "12": 12, "16": 16, "18": 18, "-10": 10, "-12": 12, "-16": 16, "-18": 18},
		"ES": {"A": 0, "AI": 0, "APTA": 0, "TP": 0, "I": 0, "7": 7, "10": 10, "12": 12, "13": 13, "16": 16, "18": 18, "X": 18},
		"IT": {"T": 0, "BA": 6, "6+": 6, "12": 12, "14+": 14, "VM14": 14, "18+": 18, "VM18": 18},
		"NL": {"AL": 0, "6": 6, "9": 9, "12": 12, "14": 14, "16": 16, "18": 18},
		"BE": {"AL": 0, "KT": 0, "EA": 0, "KT/EA": 0, "6": 6, "9": 9, "12": 12, "14": 14, "16": 16, "18": 18, "KNT": 16, "ENA": 16, "KNT/ENA": 16},
		"PT": {
			"Públicos": 0, "T": 0, "M/3": 3, "M/4": 4, "M/6": 6, "M/12": 12, "M/14": 14, "M/16": 16, "M/18": 18, "P": 18,
			"10AP": 10, "12AP": 12, "16": 16, "18": 18,
		},
		"BR": {"L": 0, "AL": 0, "10": 10, "12": 12, "14": 14, "16": 16, "18": 18},
		"PL": {"0": 0, "AL": 0, "BO": 0, "7": 7, "12": 12, "15": 15, "16": 16, "18": 18, "21": 18},
		"SE": {"Btl": 0, "7": 7, "11": 11, "15": 15},
		"DK": {"A": 0, "7": 7, "11": 11, "15": 15, "F": 15},
		"NO": {"A": 0, "6": 6, "7": 7, "9": 9, "11": 11, "12": 12, "15": 15, "18": 18},
		"FI": {"S": 0, "T": 0, "K-7": 7, "K7": 7, "K-12": 12, "K12": 12, "K-16": 16, "K16": 16, "K-18": 18, "K18": 18, "KK": 18},
		"CZ": {"U": 0, "0+": 0, "12": 12, "12+": 12, "15": 15, "15+": 15, "18": 18, "18+": 18},
		"HU": {"KN": 0, "Korhatár nélkül": 0, "6": 6, "12": 12, "16": 16, "18": 18, "X": 18},
		"TR": {
			"Genel İzleyici Kitlesi": 0, "Genel İzleyici": 0, "6": 6, "6+": 6, "6A": 6, "7+": 7, "+7": 7, "10": 10,
			"10A": 10, "13A": 13, "13+": 13, "15+": 15, "15A": 15, "16+": 16, "18+": 18,
		},
		"RU": {"0+": 0, "6+": 6, "12+": 12, "16+": 16, "18+": 18, "+16": 16},
		"UA": {"0": 0, "ЗА": 0, "12": 12, "16": 16, "18": 18},
		"JP": {"G": 0, "PG12": 12, "R15+": 15, "R18+": 18},
		"KR": {"All": 0, "ALL": 0, "7": 7, "12": 12, "15": 15, "18": 18, "19": 18, "Restricted Screening": 18},
		"TW": {"0+": 0, "6+": 6, "12+": 12, "15+": 15, "18+": 18},
	}
	if len(cases) != len(model.RatingCountries()) {
		t.Fatalf("the test covers %d countries, the table has %d", len(cases), len(model.RatingCountries()))
	}
	for c, certs := range cases {
		for cert, want := range certs {
			got, from := ageFromCertifications(map[string]string{c: cert}, c)
			if got == nil || *got != want || from != c {
				t.Errorf("%s %q = %v from %q; want %d", c, cert, deref(got), from, want)
			}
		}
	}
	// TMDB's release dates of one country give its age and the source.
	if got, src := AgeFromReleaseDates(rd(country("US", theatrical("PG-13")))); got == nil || *got != 13 || src != "tmdb:US" {
		t.Errorf("US PG-13 from the release dates = %v %q, want 13 (F-17: no longer 12)", deref(got), src)
	}
}

// Every rating country has a mapping, every mapped age lies within 0 to 18, every step of a
// country but the ceiling 18 is reached by one of its certificates, and the steps rise.
func TestRatingTableMatchesTheSystems(t *testing.T) {
	t.Parallel()
	if len(certificationAges) != len(model.RatingCountries()) {
		t.Errorf("mapping for %d countries, %d rating systems", len(certificationAges), len(model.RatingCountries()))
	}
	for _, rs := range model.RatingSystems() {
		ages, ok := certificationAges[rs.Country]
		if !ok {
			t.Errorf("%s has no certificate mapping", rs.Country)
			continue
		}
		reached := map[int]bool{}
		for cert, age := range ages {
			if age < 0 || age > model.MaxRatingAge {
				t.Errorf("%s %q = %d, outside 0..18", rs.Country, cert, age)
			}
			if cert != normalizeCertification(cert) {
				t.Errorf("%s key %q is not normalised", rs.Country, cert)
			}
			reached[age] = true
		}
		for i, st := range rs.Steps {
			if i > 0 && st.Age <= rs.Steps[i-1].Age {
				t.Errorf("%s steps do not rise at %d", rs.Country, st.Age)
			}
			if st.Age != model.MaxRatingAge && !reached[st.Age] {
				t.Errorf("%s step %d is reached by no certificate", rs.Country, st.Age)
			}
		}
		if last := rs.Steps[len(rs.Steps)-1].Age; last != model.MaxRatingAge {
			t.Errorf("%s ends with %d, not 18", rs.Country, last)
		}
	}
	for alpha3, c := range tvdbCountries {
		if !model.IsRatingCountry(c) {
			t.Errorf("TVDB country %s maps to %s, no rating country", alpha3, c)
		}
	}
	if len(tvdbCountries) != len(model.RatingCountries()) {
		t.Errorf("TVDB knows %d countries, the table has %d", len(tvdbCountries), len(model.RatingCountries()))
	}
}

func TestRatingCountryOrderAndFallbacks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      ReleaseDates
		want    *int
		wantSrc string
	}{
		{
			name: "DE wins over US",
			in:   rd(country("US", theatrical("R")), country("DE", theatrical("12"))),
			want: ptr(12), wantSrc: "tmdb:DE",
		},
		{
			name: "AT before CH, GB and US",
			in:   rd(country("US", theatrical("G")), country("CH", theatrical("16")), country("AT", theatrical("14"))),
			want: ptr(14), wantSrc: "tmdb:AT",
		},
		{
			name: "CH 7 stays 7 when DE and AT are missing (no rounding, F-17)",
			in:   rd(country("CH", theatrical("7")), country("GB", theatrical("U"))),
			want: ptr(7), wantSrc: "tmdb:CH",
		},
		{
			name: "GB before US",
			in:   rd(country("US", theatrical("PG-13")), country("GB", theatrical("15"))),
			want: ptr(15), wantSrc: "tmdb:GB",
		},
		{
			name: "unknown DE value skips to the next country",
			in:   rd(country("DE", theatrical("BPjM")), country("US", theatrical("PG"))),
			want: ptr(6), wantSrc: "tmdb:US",
		},
		{
			name: "empty certifications are ignored",
			in:   rd(country("DE", theatrical(""), Release{Certification: " ", Type: 4}), country("US", theatrical("NC-17"))),
			want: ptr(18), wantSrc: "tmdb:US",
		},
		{
			name: "main release types win over a premiere",
			in:   rd(country("DE", Release{Certification: "16", Type: 1}, Release{Certification: "12", Type: 4})),
			want: ptr(12), wantSrc: "tmdb:DE",
		},
		{
			name: "any release type when no main release is certified",
			in:   rd(country("DE", Release{Certification: "6", Type: 6}, Release{Type: 3})),
			want: ptr(6), wantSrc: "tmdb:DE",
		},
		{
			name: "countries outside the fallback order are ignored when not chosen",
			in:   rd(country("FR", theatrical("12")), country("NL", theatrical("6"))),
			want: nil, wantSrc: "",
		},
		{
			name: "unknown values everywhere",
			in:   rd(country("DE", theatrical("x")), country("US", theatrical("Unrated"))),
			want: nil, wantSrc: "",
		},
		{name: "empty", in: ReleaseDates{}, want: nil, wantSrc: ""},
		{
			name: "case and blanks are normalised",
			in:   rd(country("us", theatrical(" pg-13 "))),
			want: ptr(13), wantSrc: "tmdb:US",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, src := AgeFromReleaseDates(tc.in)
			if deref(got) != deref(tc.want) || (got == nil) != (tc.want == nil) || src != tc.wantSrc {
				t.Fatalf("got %v, %q; want %v, %q", deref(got), src, deref(tc.want), tc.wantSrc)
			}
		})
	}
}

func ptr(v int) *int { return &v }

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestCertificationAtMost(t *testing.T) {
	for _, tc := range []struct {
		country string
		age     int
		tv      bool
		want    string
		ok      bool
	}{
		{"DE", 12, false, "12", true}, {"DE", 15, false, "12", true}, {"DE", 0, true, "0", true},
		{"US", 12, false, "PG", true}, {"US", 13, false, "PG-13", true}, {"US", 16, true, "TV-14", true},
		{"US", 0, true, "TV-Y", true}, {"GB", 12, false, "12", true}, {"GB", 14, false, "12", true},
		{"JP", 11, false, "G", true}, {"PT", -1, false, "", false}, {"XX", 12, false, "", false}, {"SE", 18, false, "15", true},
	} {
		got, ok := CertificationAtMost(tc.country, tc.age, tc.tv)
		if got != tc.want || ok != tc.ok {
			t.Errorf("CertificationAtMost(%s, %d, %v) = %q %v, want %q %v", tc.country, tc.age, tc.tv, got, ok, tc.want, tc.ok)
		}
	}
}
