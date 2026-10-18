package metadata

import "testing"

func rd(countries ...CountryReleases) ReleaseDates { return ReleaseDates{Results: countries} }

func country(code string, releases ...Release) CountryReleases {
	return CountryReleases{Country: code, ReleaseDates: releases}
}

func theatrical(cert string) Release { return Release{Certification: cert, Type: 3} }

// TestFSKTables checks every row of the AN-8 mapping, including the rounding up to the next
// higher FSK step.
func TestFSKTables(t *testing.T) {
	cases := []struct {
		country, cert string
		want          int
	}{
		{"DE", "0", 0}, {"DE", "6", 6}, {"DE", "12", 12}, {"DE", "16", 16}, {"DE", "18", 18},
		{"AT", "0", 0}, {"AT", "6", 6}, {"AT", "10", 12}, {"AT", "12", 12}, {"AT", "14", 16},
		{"AT", "16", 16}, {"AT", "18", 18},
		{"CH", "0", 0}, {"CH", "6", 6}, {"CH", "7", 12}, {"CH", "8", 12}, {"CH", "10", 12},
		{"CH", "12", 12}, {"CH", "14", 16}, {"CH", "16", 16}, {"CH", "18", 18},
		{"GB", "U", 0}, {"GB", "PG", 6}, {"GB", "12", 12}, {"GB", "12A", 12}, {"GB", "15", 16},
		{"GB", "18", 18}, {"GB", "R18", 18},
		{"US", "G", 0}, {"US", "PG", 6}, {"US", "PG-13", 12}, {"US", "R", 16}, {"US", "NC-17", 18},
		{"US", "TV-Y", 0}, {"US", "TV-G", 0}, {"US", "TV-Y7", 6}, {"US", "TV-PG", 6},
		{"US", "TV-14", 16}, {"US", "TV-MA", 18},
	}
	for _, tc := range cases {
		got, src := FSKFromReleaseDates(rd(country(tc.country, theatrical(tc.cert))))
		if got == nil || *got != tc.want || src != "tmdb:"+tc.country {
			t.Errorf("%s %q = %v, %q; want %d, tmdb:%s", tc.country, tc.cert, deref(got), src, tc.want, tc.country)
		}
	}
}

func TestFSKCountryOrderAndFallbacks(t *testing.T) {
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
			want: ptr(16), wantSrc: "tmdb:AT",
		},
		{
			name: "CH 7 rounds up when DE and AT are missing",
			in:   rd(country("CH", theatrical("7")), country("GB", theatrical("U"))),
			want: ptr(12), wantSrc: "tmdb:CH",
		},
		{
			name: "GB before US",
			in:   rd(country("US", theatrical("PG-13")), country("GB", theatrical("15"))),
			want: ptr(16), wantSrc: "tmdb:GB",
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
			name: "countries outside the order are ignored",
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
			want: ptr(12), wantSrc: "tmdb:US",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, src := FSKFromReleaseDates(tc.in)
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
