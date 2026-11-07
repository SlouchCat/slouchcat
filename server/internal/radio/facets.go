package radio

import (
	"slices"
	"strings"
	"unicode"

	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
)

// Facets of a station (concept 16 section 5.4): genre, kind and mood derived from its raw tags
// through the embedded tag rules (rules/default.json). A tag matches as a whole; a tag without a
// whole match in a facet matches by its words («80s hits» → oldies and pop). A station without a
// kind hit is music, unless its name says news, info or sport; mood is only set on a hit. A facet
// the admin locked keeps its manual values (sync.go).

// facetNames are the facets in their stored order.
var facetNames = []string{rules.FacetGenre, rules.FacetKind, rules.FacetMood}

// nameKinds are the name words that give a station its kind when no tag does.
var nameKinds = map[string]string{"news": "news", "info": "news", "nachrichten": "news", "sport": "sport", "sports": "sport"}

// tagWords splits a folded tag into its words.
func tagWords(t string) []string {
	return strings.FieldsFunc(t, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '&' && r != '\''
	})
}

// deriveFacets returns the facet values of a station with name and (lower-cased) tags, each list
// in the order of rules.Values.
func deriveFacets(r rules.TagRules, name string, tags []string) map[string][]string {
	out := make(map[string][]string, len(facetNames))
	for _, facet := range facetNames {
		m := r.ByFacet(facet)
		hits := map[string]bool{}
		for _, tag := range tags {
			t := rules.Fold(tag)
			if v, ok := m[t]; ok {
				hits[v] = true
				continue
			}
			for _, w := range tagWords(t) {
				if v, ok := m[w]; ok {
					hits[v] = true
				}
			}
		}
		if facet == rules.FacetKind && len(hits) == 0 {
			for _, w := range tagWords(rules.Fold(name)) {
				if v, ok := nameKinds[w]; ok {
					hits[v] = true
				}
			}
			if len(hits) == 0 {
				hits[rules.KindDefault] = true
			}
		}
		for _, v := range rules.Values(facet) {
			if hits[v] {
				out[facet] = append(out[facet], v)
			}
		}
	}
	return out
}

// knownFacetValue reports whether v is a value of facet.
func knownFacetValue(facet, v string) bool { return slices.Contains(rules.Values(facet), v) }
