package trackpick

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"de": "de", "ger": "de", "deu": "de", "German": "de", "deutsch": "de", "gsw": "de", "gsw-CH": "de",
		"eng": "en", "en_US": "en", "pt-BR": "pt", "por": "pt", "cn": "zh", "zh-Hans": "zh", "chi": "zh",
		"nob": "no", "nb": "no", "iw": "he", "in": "id", "ji": "yi", "jw": "jv", "mo": "ro", "fil": "tl", " FRE ": "fr", "yue": "yue",
		"": "", "und": "", "zxx": "", "xx": "", "mul": "", "und-Latn": "",
		"mal": "ml", "kan": "kn", "mar": "mr", "pan": "pa", "guj": "gu", "bel": "be", "aze": "az",
		"kaz": "kk", "uzb": "uz", "mlt": "mt", "ltz": "lb", "epo": "eo", "khm": "km", "lao": "lo",
		"mon": "mn", "nep": "ne", "sin": "si", "scc": "sr", "scr": "hr", "bur": "my", "mao": "mi",
		"tib": "bo", "Français": "fr", "español": "es", "Türkçe": "tr", "francais": "fr",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValid(t *testing.T) {
	for code, want := range map[string]bool{
		"de": true, "en": true, "he": true, "id": true, "no": true, "tl": true, "zh": true, "ja": true,
		"iw": false, "in": false, "ji": false, "jw": false, "mo": false, "sh": false,
		"nb": false, "nn": false, "cn": false, "xx": false, "zz": false, "DE": false, "ger": false,
		"gsw": false, "pt-br": false, "": false, "d": false,
	} {
		if got := Valid(code); got != want {
			t.Errorf("Valid(%q) = %v, want %v", code, got, want)
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
	for _, m := range regexp.MustCompile(`'?(\p{L}+)'?: '([a-z]+)'`).FindAllStringSubmatch(block[1], -1) {
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

// TestLanguageCodesComplete checks the table against golang.org/x/text for every ISO 639-1 code of
// the metadata languages: the ISO 639-2/T code and the one-word English and native names map to
// the code (review step 9, m-6).
func TestLanguageCodesComplete(t *testing.T) {
	raw, err := os.ReadFile("../metadata/languages.go")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile("(?s)var iso6391 = toSet\\(strings.Fields\\(`(.*?)`").FindSubmatch(raw)
	if block == nil {
		t.Fatal("iso6391 not found in metadata/languages.go")
	}
	// cn and xx are TMDB codes, mo, sh and bh withdrawn ISO 639-1 codes; "akan" names ak, not tw.
	skip := map[string]bool{"cn": true, "xx": true, "mo": true, "sh": true, "bh": true}
	word := regexp.MustCompile(`^\p{L}+$`)
	codes := strings.Fields(string(block[1]))
	if len(codes) < 150 {
		t.Fatalf("iso6391 has %d codes", len(codes))
	}
	for _, code := range codes {
		if skip[code] {
			continue
		}
		want := code
		if code == "nb" || code == "nn" {
			want = "no"
		}
		if got := Normalize(code); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", code, got, want)
		}
		iso3 := language.MustParseBase(code).ISO3()
		if got := languageCodes[iso3]; got != want {
			t.Errorf("languageCodes[%q] (ISO 639-2/T of %s) = %q, want %q", iso3, code, got, want)
		}
		tag := language.Make(code)
		for _, name := range []string{display.English.Languages().Name(tag), display.Self.Name(tag)} {
			name = strings.ToLower(name)
			if !word.MatchString(name) || name == code || name == "akan" {
				continue
			}
			if got := Normalize(name); got != want {
				t.Errorf("Normalize(%q) (name of %s) = %q, want %q", name, code, got, want)
			}
		}
	}
}
