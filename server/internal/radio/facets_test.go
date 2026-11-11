package radio

import (
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
)

// The default rules derive genre, kind and mood from whole tags and from tag words; a whole match
// in a facet wins over its words; kind falls back to the name, then to music.
func TestFacetsFromDefaultRules(t *testing.T) {
	r := rules.Default()
	for _, tc := range []struct {
		name        string
		tags        []string
		want        string
		kindDefault bool
	}{
		{"Hits FM", []string{"80s hits"}, "map[genre:[pop oldies] kind:[music]]", true},
		{"Rock FM", []string{"classic rock"}, "map[genre:[rock] kind:[music]]", true},
		{"Info Radio", []string{"pop"}, "map[genre:[pop] kind:[news]]", false},
		{"Talk One", []string{"talk", "comedy"}, "map[kind:[talk]]", false},
		{"Chill Zone", []string{"Chillout", "Lounge"}, "map[genre:[chillout] kind:[music] mood:[chill]]", true},
		{"Sport Eins", nil, "map[kind:[sport]]", false},
		{"Kirche FM", []string{"Gospel", "Worship"}, "map[genre:[religious] kind:[music]]", true},
		{"Plain", []string{"unknowntag"}, "map[kind:[music]]", true},
	} {
		got, kindDefault := deriveFacets(r, tc.name, tc.tags)
		if fmt.Sprint(got) != tc.want || kindDefault != tc.kindDefault {
			t.Errorf("%s %v: %v (kind default %v), want %s (%v)", tc.name, tc.tags, got, kindDefault, tc.want, tc.kindDefault)
		}
	}
}

// The rules file names known values only, holds at least 250 tags, and no two spellings of a tag
// fold to different values.
func TestDefaultRulesFileValid(t *testing.T) {
	raw, err := rules.Raw()
	if err != nil {
		t.Fatal(err)
	}
	tags := map[string]bool{}
	for _, facet := range facetNames {
		folded := map[string]string{}
		m := raw.ByFacet(facet)
		if len(m) == 0 {
			t.Errorf("facet %s has no rules", facet)
		}
		for tag, v := range m {
			if !slices.Contains(rules.Values(facet), v) {
				t.Errorf("%s: tag %q maps to unknown value %q", facet, tag, v)
			}
			f := rules.Fold(tag)
			if prev, ok := folded[f]; ok && prev != v {
				t.Errorf("%s: %q folds to %q, which already maps to %q, not %q", facet, tag, f, prev, v)
			}
			folded[f] = v
			tags[f] = true
		}
		for _, v := range rules.Values(facet) {
			found := false
			for _, got := range m {
				found = found || got == v
			}
			if !found && (facet != rules.FacetKind || v != rules.KindDefault) {
				t.Errorf("%s: value %q has no tag", facet, v)
			}
		}
	}
	if len(tags) < 250 {
		t.Errorf("%d distinct tags, want at least 250", len(tags))
	}
}

// The canton table reads the spellings Radio Browser carries.
func TestRegionsCHNames(t *testing.T) {
	for in, want := range map[string]string{
		"Zurich": "CH-ZH", "Zürich": "CH-ZH", "ZH": "CH-ZH", "Canton of Zurich": "CH-ZH", "Kanton Zürich": "CH-ZH",
		"Genève": "CH-GE", "Canton de Genève": "CH-GE", "St. Gallen": "CH-SG", "Basel-Landschaft": "CH-BL",
		"Ticino": "CH-TI", "Tessin": "CH-TI", "Graubünden": "CH-GR", "Bayern": "", "": "",
	} {
		if got := rules.CHRegion(in); got != want {
			t.Errorf("CHRegion(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"German": "de", "swiss german": "de", "Français": "fr", "romansh": "rm", "klingon": ""} {
		if got := rules.Language(in); got != want {
			t.Errorf("Language(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"Europe/Zurich": "CH", "Europe/Berlin": "DE", "America/New_York": "US", "UTC": ""} {
		if got := rules.ZoneCountry(in); got != want {
			t.Errorf("ZoneCountry(%q) = %q, want %q", in, got, want)
		}
	}
}
