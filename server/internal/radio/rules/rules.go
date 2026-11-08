// Package rules holds the embedded rules of the webradio catalog (concept 16 sections 5.4 and 5.5,
// ADR 0025): the tag rules that derive the facets genre, kind and mood from the raw Radio Browser
// tags (default.json), the names of the Swiss cantons as Radio Browser writes them into the field
// state (regions_ch.json), the language names it writes into the field language
// (languages.json) and the IANA time zones with their country (zones.json, from zone1970.tab and
// zone.tab), which give a new profile its default country. Lookups fold case and diacritics.
package rules

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Facet names (radio_station_facets.facet).
const (
	FacetGenre = "genre"
	FacetKind  = "kind"
	FacetMood  = "mood"
)

// Stable facet values; the texts live in the web locales.
var (
	Genres = []string{"pop", "rock", "jazz", "classical", "electronic", "hiphop", "schlager", "folk", "alternative",
		"oldies", "chillout", "world", "country", "metal", "mixed", "rnb", "latin", "reggae", "blues", "soul", "dance",
		"indie", "religious", "kids", "soundtrack"}
	Kinds = []string{"music", "news", "talk", "sport", "culture"}
	Moods = []string{"chill", "party", "focus", "sleep", "workout"}
)

// KindDefault is the kind of a station without a kind hit.
const KindDefault = "music"

// Values returns the known values of a facet (nil for an unknown facet).
func Values(facet string) []string {
	switch facet {
	case FacetGenre:
		return Genres
	case FacetKind:
		return Kinds
	case FacetMood:
		return Moods
	}
	return nil
}

var (
	//go:embed default.json
	defaultJSON []byte
	//go:embed regions_ch.json
	regionsJSON []byte
	//go:embed languages.json
	languagesJSON []byte
	//go:embed zones.json
	zonesJSON []byte
)

// TagRules maps a folded tag (or tag word) to a facet value, per facet.
type TagRules struct {
	Genre map[string]string `json:"genre"`
	Kind  map[string]string `json:"kind"`
	Mood  map[string]string `json:"mood"`
}

// ByFacet returns the map of one facet.
func (r TagRules) ByFacet(facet string) map[string]string {
	switch facet {
	case FacetGenre:
		return r.Genre
	case FacetKind:
		return r.Kind
	case FacetMood:
		return r.Mood
	}
	return nil
}

type tables struct {
	tags      TagRules
	regions   map[string]string // folded canton name → ISO 3166-2
	languages map[string]string // folded language name → ISO 639-1
	zones     map[string]string // IANA zone → ISO 3166-1
}

var load = sync.OnceValues(func() (*tables, error) {
	t := &tables{regions: map[string]string{}, languages: map[string]string{}}
	var raw TagRules
	if err := json.Unmarshal(defaultJSON, &raw); err != nil {
		return nil, fmt.Errorf("default.json: %w", err)
	}
	fold := func(m map[string]string) map[string]string {
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[Fold(k)] = v
		}
		return out
	}
	t.tags = TagRules{Genre: fold(raw.Genre), Kind: fold(raw.Kind), Mood: fold(raw.Mood)}
	var regions map[string][]string
	if err := json.Unmarshal(regionsJSON, &regions); err != nil {
		return nil, fmt.Errorf("regions_ch.json: %w", err)
	}
	for code, names := range regions {
		t.regions[Fold(strings.TrimPrefix(code, "CH-"))] = code
		for _, n := range names {
			t.regions[Fold(n)] = code
		}
	}
	var langs map[string]string
	if err := json.Unmarshal(languagesJSON, &langs); err != nil {
		return nil, fmt.Errorf("languages.json: %w", err)
	}
	t.languages = fold(langs)
	if err := json.Unmarshal(zonesJSON, &t.zones); err != nil {
		return nil, fmt.Errorf("zones.json: %w", err)
	}
	return t, nil
})

// mustLoad returns the parsed tables; the files are embedded and checked by the tests, so a parse
// error is a build defect.
func mustLoad() *tables {
	t, err := load()
	if err != nil {
		panic("radio rules: " + err.Error())
	}
	return t
}

// Fold lower-cases s, folds diacritics and collapses white space (model.SortKey without articles).
func Fold(s string) string { return model.SortKey("", s) }

// Default returns the embedded tag rules with folded keys.
func Default() TagRules { return mustLoad().tags }

// cantonPrefixes are the words Radio Browser entries put before a canton's name.
var cantonPrefixes = []string{"canton of ", "canton de ", "canton du ", "kanton ", "cantone del ", "cantone ", "canton ", "chantun "}

// CHRegion returns the ISO 3166-2 code of a Swiss canton from the free text of the field state
// («Zurich», «Zürich», «ZH», «Canton of Zurich» → CH-ZH); "" when it names none.
func CHRegion(state string) string {
	s := Fold(state)
	for _, p := range cantonPrefixes {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimSuffix(s, " canton")
	return mustLoad().regions[strings.TrimSpace(s)]
}

// Language returns the ISO 639-1 code of a language name («german», «Swiss German» → de); "" when
// the name is unknown.
func Language(name string) string { return mustLoad().languages[Fold(name)] }

// ZoneCountry returns the ISO 3166-1 country of an IANA time zone (Europe/Zurich → CH); "" when
// the zone is unknown.
func ZoneCountry(zone string) string { return mustLoad().zones[zone] }

// Raw returns the tag rules as embedded (for the validity test).
func Raw() (TagRules, error) {
	var r TagRules
	err := json.Unmarshal(defaultJSON, &r)
	return r, err
}
