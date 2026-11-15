// Package i18n holds the texts the server writes itself (concept 22 section 4.5, ADR 0033). They
// live in embedded JSON files, one per language, in the format of the web app: i18next v4 with
// {{name}} placeholders, so Weblate treats both alike. A text is looked up along the same chain as
// in the web app: the language, German for de-DE, then English.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Fallback is the last language of every chain; its file must hold every key.
const Fallback = "en"

// Bundle is the texts of one component, by language and key. Nested JSON objects become keys
// joined with a dot, as i18next reads them.
type Bundle struct {
	texts map[string]map[string]string
}

// Load reads every <code>.json at the top of fsys. The file of Fallback must be there.
func Load(fsys fs.FS) (*Bundle, error) {
	files, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, err
	}
	b := &Bundle{texts: map[string]map[string]string{}}
	for _, name := range files {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("i18n %s: %w", name, err)
		}
		flat := map[string]string{}
		if err := flatten("", raw, flat); err != nil {
			return nil, fmt.Errorf("i18n %s: %w", name, err)
		}
		b.texts[strings.TrimSuffix(path.Base(name), ".json")] = flat
	}
	if _, ok := b.texts[Fallback]; !ok {
		return nil, fmt.Errorf("i18n: no %s.json", Fallback)
	}
	return b, nil
}

func flatten(prefix string, in map[string]any, out map[string]string) error {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch v := v.(type) {
		case string:
			out[key] = v
		case map[string]any:
			if err := flatten(key, v, out); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: want a text or an object, got %T", key, v)
		}
	}
	return nil
}

// Chain is the order the texts of lang are looked up in: lang itself, German for de-DE (the
// German of Germany is an overlay on the Swiss one), then English. A code appears once.
func Chain(lang string) []string {
	chain := []string{lang}
	if lang == "de-DE" {
		chain = append(chain, "de")
	}
	chain = append(chain, Fallback)
	return slices.Compact(chain)
}

// Languages returns the codes the bundle has a file for, sorted.
func (b *Bundle) Languages() []string { return slices.Sorted(maps.Keys(b.texts)) }

// Keys returns the keys of lang's own file, sorted (nil for an unknown language).
func (b *Bundle) Keys(lang string) []string { return slices.Sorted(maps.Keys(b.texts[lang])) }

// Raw returns the text of key in lang's own file without the chain and without placeholders
// filled in.
func (b *Bundle) Raw(lang, key string) (string, bool) {
	t, ok := b.texts[lang][key]
	return t, ok
}

// Text returns the text of key along Chain(lang) with the placeholders filled in from vars. ok
// is false when no language of the chain has the key.
func (b *Bundle) Text(lang, key string, vars map[string]any) (string, bool) {
	for _, l := range Chain(lang) {
		if t, ok := b.texts[l][key]; ok {
			return Interpolate(t, vars), true
		}
	}
	return "", false
}

var placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// Placeholders returns the names of the {{name}} placeholders of a text, sorted, each once.
func Placeholders(text string) []string {
	var names []string
	for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// Interpolate fills the {{name}} placeholders of text from vars; a name vars lacks stays as it
// is, so a missing value shows instead of vanishing.
func Interpolate(text string, vars map[string]any) string {
	return placeholder.ReplaceAllStringFunc(text, func(m string) string {
		name := placeholder.FindStringSubmatch(m)[1]
		if v, ok := vars[name]; ok {
			return fmt.Sprint(v)
		}
		return m
	})
}
