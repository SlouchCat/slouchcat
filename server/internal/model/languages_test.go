package model

import "testing"

func TestFallbackLanguageAndMetadataBase(t *testing.T) {
	t.Parallel()
	for ui, want := range map[string][2]string{
		"de":    {"de", "de"},
		"de-DE": {"de", "de"},
		"en":    {"en", "en"},
		"fr":    {"en", "fr"},
		"pt-BR": {"en", "pt-BR"},
		"pt-PT": {"en", "pt-PT"},
		"zh-CN": {"en", "zh-CN"},
		"zh-TW": {"en", "zh-TW"},
		"nb":    {"en", "no"},
		"ja":    {"en", "ja"},
	} {
		if got := FallbackLanguage(ui); got != want[0] {
			t.Errorf("FallbackLanguage(%q) = %q, want %q", ui, got, want[0])
		}
		if got := MetadataBase(ui); got != want[1] {
			t.Errorf("MetadataBase(%q) = %q, want %q", ui, got, want[1])
		}
	}
}

func TestIsUILanguage(t *testing.T) {
	t.Parallel()
	if len(UILanguages) != 23 {
		t.Errorf("%d interface languages, want the 23 of concept 22 table 4.1", len(UILanguages))
	}
	for code, want := range map[string]bool{
		"de": true, "en": true, "de-DE": true, "fr": true, "nb": true, "pt-BR": true, "zh-TW": true,
		"de-de": false, "no": false, "pt": false, "zh": false, "ar": false, "en-XA": false, "": false, "DE": false,
	} {
		if got := IsUILanguage(code); got != want {
			t.Errorf("IsUILanguage(%q) = %v, want %v", code, got, want)
		}
	}
}
