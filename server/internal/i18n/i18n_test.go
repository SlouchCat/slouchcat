package i18n

import (
	"slices"
	"testing"
	"testing/fstest"
)

func TestChain(t *testing.T) {
	t.Parallel()
	for lang, want := range map[string][]string{
		"de":    {"de", "en"},
		"de-DE": {"de-DE", "de", "en"},
		"en":    {"en"},
		"fr":    {"fr", "en"},
		"pt-BR": {"pt-BR", "en"},
	} {
		if got := Chain(lang); !slices.Equal(got, want) {
			t.Errorf("Chain(%q) = %v, want %v", lang, got, want)
		}
	}
}

func TestBundleTextFollowsTheChain(t *testing.T) {
	t.Parallel()
	b, err := Load(fstest.MapFS{
		"en.json":    {Data: []byte(`{"hello": "Hello {{name}}", "only_en": "English", "nested": {"key": "Nested"}}`)},
		"de.json":    {Data: []byte(`{"hello": "Hallo {{name}}", "street": "Strasse"}`)},
		"de-DE.json": {Data: []byte(`{"street": "Straße"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Languages(); !slices.Equal(got, []string{"de", "de-DE", "en"}) {
		t.Errorf("Languages = %v", got)
	}
	for _, tc := range []struct{ lang, key, want string }{
		{"de", "hello", "Hallo Marco"},
		{"de-DE", "hello", "Hallo Marco"}, // from de
		{"de-DE", "street", "Straße"},
		{"de", "street", "Strasse"},
		{"de-DE", "only_en", "English"}, // from en
		{"fr", "hello", "Hello Marco"},  // unknown language: English
		{"en", "nested.key", "Nested"},
	} {
		got, ok := b.Text(tc.lang, tc.key, map[string]any{"name": "Marco"})
		if !ok || got != tc.want {
			t.Errorf("Text(%q, %q) = %q, %v, want %q", tc.lang, tc.key, got, ok, tc.want)
		}
	}
	if _, ok := b.Text("de", "missing", nil); ok {
		t.Error("a missing key was found")
	}
}

func TestLoadRefusesBrokenFiles(t *testing.T) {
	t.Parallel()
	for name, fsys := range map[string]fstest.MapFS{
		"no English":     {"de.json": {Data: []byte(`{}`)}},
		"not JSON":       {"en.json": {Data: []byte(`{`)}},
		"a number value": {"en.json": {Data: []byte(`{"n": 1}`)}},
	} {
		if _, err := Load(fsys); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

func TestInterpolateAndPlaceholders(t *testing.T) {
	t.Parallel()
	text := "{{count}} von {{ total }} in {{count}}, {{unknown}}"
	if got := Interpolate(text, map[string]any{"count": 3, "total": int64(5)}); got != "3 von 5 in 3, {{unknown}}" {
		t.Errorf("Interpolate = %q", got)
	}
	if got := Placeholders(text); !slices.Equal(got, []string{"count", "total", "unknown"}) {
		t.Errorf("Placeholders = %v", got)
	}
}
