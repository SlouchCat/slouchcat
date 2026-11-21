package model

import "testing"

func TestCleanDeviceName(t *testing.T) {
	for _, c := range []struct {
		in    string
		limit int
		want  string
	}{
		{"  Pixel 9 ", 80, "Pixel 9"},
		{"Pixel\n9", 80, "Pixel9"},
		{"Pixel\u202e 9\u200b", 80, "Pixel 9"},         // bidi override, zero-width space
		{"\u2066TV\u2069 Stube\u00ad", 80, "TV Stube"}, // bidi isolates, soft hyphen
		{"\u200b\u202e\u0000", 80, ""},                 // nothing but hidden characters
		{"Wohnzimmer-Fernseher", 10, "Wohnzimmer"},     // cut to the limit
		{"Fernseh zimmer", 8, "Fernseh"},               // no trailing space after the cut
		{"Küche 📺", 0, "Küche 📺"},                      // no limit, emoji stays
	} {
		if got := CleanDeviceName(c.in, c.limit); got != c.want {
			t.Errorf("CleanDeviceName(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}
