package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/i18n"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// localeText reads key of lang straight from locales/<lang>.json and fills in v, so the tests
// check the messages against the files and not against the code that loads them.
func localeText(t *testing.T, lang, key string, v vars) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("locales", lang+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var texts map[string]string
	if err := json.Unmarshal(data, &texts); err != nil {
		t.Fatal(err)
	}
	text, ok := texts[key]
	if !ok {
		t.Fatalf("locales/%s.json has no %q", lang, key)
	}
	return i18n.Interpolate(text, v)
}

// strictTexts are the languages whose server texts must be complete in every commit (concept 22
// section 4.8): de and en in each pull request (de-DE is an overlay). The other languages are
// completed before each release («Übersetzungen nachziehen»); until then a missing text is a log
// line here and English along the chain at run time. `task i18n:check`, the release gate, sets
// SLOUCHCAT_I18N_STRICT and makes every language strict.
var strictTexts = map[string]bool{"de": true, "en": true}

// The server texts (concept 22 sections 4.5 and 4.9): one i18next v4 file per interface language,
// flat keys with text values only; every language has the keys of English (de and en strictly,
// the others warn), no key English lacks, the same placeholders, no empty text, German writes ss,
// never ß.
func TestServerTexts(t *testing.T) {
	t.Parallel()
	if got, want := texts.Languages(), slices.Sorted(slices.Values(model.UILanguages)); !slices.Equal(got, want) {
		t.Errorf("server text files %v, want one per interface language %v", got, want)
	}
	reference := texts.Keys(i18n.Fallback)
	if len(reference) == 0 {
		t.Fatal("no English server texts")
	}
	for _, lang := range texts.Languages() {
		// de-DE is an overlay (ADR 0033): only the texts that read differently in Germany (ß,
		// „…“), each one different from de; the rest comes from de along the chain.
		if lang != "de-DE" {
			var missing []string
			for _, key := range reference {
				if _, ok := texts.Raw(lang, key); !ok {
					missing = append(missing, key)
				}
			}
			switch {
			case len(missing) > 0 && (strictTexts[lang] || os.Getenv("SLOUCHCAT_I18N_STRICT") != ""):
				t.Errorf("%s: missing %v", lang, missing)
			case len(missing) > 0:
				t.Logf("warning: %s misses %d texts (before the release: «Übersetzungen nachziehen»): %v", lang, len(missing), missing)
			}
		}
		for _, key := range texts.Keys(lang) {
			text, _ := texts.Raw(lang, key)
			en, ok := texts.Raw(i18n.Fallback, key)
			if !ok {
				t.Errorf("%s %s: a key English does not have", lang, key)
			}
			if de, _ := texts.Raw("de", key); lang == "de-DE" && text == de {
				t.Errorf("de-DE %s: the same as de; the overlay holds only texts that differ", key)
			}
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s %s: empty", lang, key)
			}
			if got, want := i18n.Placeholders(text), i18n.Placeholders(en); !slices.Equal(got, want) {
				t.Errorf("%s %s: placeholders %v, en has %v", lang, key, got, want)
			}
			if strings.Contains(text, "%s") || strings.Contains(text, "%d") {
				t.Errorf("%s %s: a printf verb instead of {{name}}", lang, key)
			}
			if lang == "de" && strings.Contains(text, "ß") {
				t.Errorf("de %s: ß in Swiss German", key)
			}
		}
	}
}

// text walks the chain: an unknown language and a language without the key take English, de-DE
// takes German, a key nobody has returns the key.
func TestTextFallsBackAlongTheChain(t *testing.T) {
	t.Parallel()
	want := localeText(t, "en", "invitation_body", vars{"email": "a@b.test"})
	for _, lang := range []string{"ar", "", "xx"} {
		if got := text(lang, "invitation_body", vars{"email": "a@b.test"}); got != want {
			t.Errorf("text(%q) = %q, want English %q", lang, got, want)
		}
	}
	if got, want := text("de-DE", "invitation_subject"), localeText(t, "de", "invitation_subject", nil); got != want {
		t.Errorf("text(de-DE) = %q, want German %q", got, want)
	}
	if got := text("de", "no_such_reason"); got != "no_such_reason" {
		t.Errorf("missing key = %q", got)
	}
}

// The HTML part of a mail names its language (concept 22 section 4.5); without one it has none.
func TestEmailBodiesCarryTheLanguage(t *testing.T) {
	t.Parallel()
	_, withLang := emailBodies(message{Subject: "s", Body: "b", Lang: "en"}, "")
	if !strings.HasPrefix(withLang, `<!doctype html><html lang="en"><body>`) {
		t.Errorf("html = %q", withLang)
	}
	_, without := emailBodies(message{Subject: "s", Body: "b"}, "")
	if !strings.HasPrefix(without, `<!doctype html><html><body>`) {
		t.Errorf("html = %q", without)
	}
}
