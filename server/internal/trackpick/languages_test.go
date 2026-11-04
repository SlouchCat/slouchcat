package trackpick

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"de": "de", "ger": "de", "deu": "de", "German": "de", "deutsch": "de", "gsw": "de", "gsw-CH": "de",
		"eng": "en", "en_US": "en", "pt-BR": "pt", "por": "pt", "cn": "zh", "zh-Hans": "zh", "chi": "zh",
		"nob": "no", "nb": "no", "iw": "he", "fil": "tl", " FRE ": "fr", "yue": "yue",
		"": "", "und": "", "zxx": "", "xx": "", "mul": "", "und-Latn": "",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestLanguageParity holds the language table of the web client (LANGUAGE_CODES and NO_LANGUAGE
// in web/src/features/player/labels.ts) in step with this one: the client orders its panels by
// the same languages the server chooses tracks by (concept 13 11.3).
func TestLanguageParity(t *testing.T) {
	raw, err := os.ReadFile("../../../web/src/features/player/labels.ts")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	block := regexp.MustCompile(`(?s)export const LANGUAGE_CODES[^=]*= \{(.*?)\n\};`).FindStringSubmatch(src)
	if block == nil {
		t.Fatal("LANGUAGE_CODES not found in labels.ts")
	}
	web := map[string]string{}
	for _, m := range regexp.MustCompile(`'?([a-z]+)'?: '([a-z]+)'`).FindAllStringSubmatch(block[1], -1) {
		web[m[1]] = m[2]
	}
	if len(web) == 0 {
		t.Fatal("LANGUAGE_CODES has no entries")
	}
	for code, want := range languageCodes {
		if got, ok := web[code]; !ok || got != want {
			t.Errorf("web LANGUAGE_CODES[%q] = %q, want %q", code, got, want)
		}
	}
	for code := range web {
		if _, ok := languageCodes[code]; !ok {
			t.Errorf("web LANGUAGE_CODES has %q, the server table does not", code)
		}
	}

	set := regexp.MustCompile(`NO_LANGUAGE[^=]*= new Set\(\[([^\]]*)\]\)`).FindStringSubmatch(src)
	if set == nil {
		t.Fatal("NO_LANGUAGE not found in labels.ts")
	}
	var none []string
	for _, v := range strings.Split(set[1], ",") {
		if v = strings.Trim(strings.TrimSpace(v), "'"); v != "" {
			none = append(none, v)
		}
	}
	slices.Sort(none)
	if want := slices.Sorted(maps.Keys(noLanguage)); !slices.Equal(none, want) {
		t.Errorf("web NO_LANGUAGE = %v, want %v", none, want)
	}
}
