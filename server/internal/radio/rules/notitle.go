package rules

// The patterns of stream metadata that is no title (concept 16 section 8.3), owned by the relay
// (lane A of the webradio build); the catalog rules of the same package live in default.go.

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sync"
)

//go:embed notitle.json
var noTitleJSON []byte

var noTitle = sync.OnceValue(func() []*regexp.Regexp {
	var doc struct {
		Patterns []string `json:"patterns"`
	}
	if err := json.Unmarshal(noTitleJSON, &doc); err != nil {
		panic("rules/notitle.json: " + err.Error())
	}
	res := make([]*regexp.Regexp, 0, len(doc.Patterns))
	for _, p := range doc.Patterns {
		res = append(res, regexp.MustCompile("(?i)"+p))
	}
	return res
})

// NoTitlePatterns returns the compiled patterns of notitle.json. They match the folded text (lower
// case, diacritics removed, spaces collapsed); a match means the text announces no title.
func NoTitlePatterns() []*regexp.Regexp { return noTitle() }
