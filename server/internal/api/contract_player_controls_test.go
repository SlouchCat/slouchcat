package api

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Concept 23 (contract 1.38.0): the fields of the episode buttons. PlaybackInfo.previousEpisode is
// an optional ItemRef next to nextEpisode, ItemDetail gets no previousEpisode, DownloadState names
// the neighbours by optional ItemIDs, and every operation that answers with one of these schemas
// is scoped (CLAUDE.md rule 7).
func checkPlayerControlFields(t *testing.T, doc map[string]any) {
	t.Helper()
	// properties and required of a schema, through allOf.
	schema := func(name string) (map[string]any, []any) {
		props, required := map[string]any{}, []any{}
		var walk func(node any)
		walk = func(node any) {
			s, _ := node.(map[string]any)
			if ref, ok := s["$ref"].(string); ok && ref != itemRefPointer {
				walk(resolve(doc, ref))
			}
			if p, ok := s["properties"].(map[string]any); ok {
				for k, v := range p {
					props[k] = v
				}
			}
			if r, ok := s["required"].([]any); ok {
				required = append(required, r...)
			}
			if all, ok := s["allOf"].([]any); ok {
				for _, part := range all {
					walk(part)
				}
			}
		}
		walk(resolve(doc, "#/components/schemas/"+name))
		return props, required
	}
	optionalRef := func(name, field, target string) {
		props, required := schema(name)
		p, _ := props[field].(map[string]any)
		desc, _ := p["description"].(string)
		if p == nil || p["$ref"] != target || slices.Contains(required, any(field)) || !strings.Contains(desc, "1.38.0") {
			t.Errorf("%s.%s = %v, want an optional %s described since contract 1.38.0", name, field, p, target)
		}
	}
	optionalRef("PlaybackInfo", "previousEpisode", itemRefPointer)
	optionalRef("DownloadState", "nextEpisodeId", "#/components/schemas/ItemID")
	optionalRef("DownloadState", "previousEpisodeId", "#/components/schemas/ItemID")
	// nextEpisode names the podcast rule (F-5) on both schemas that carry it.
	for _, name := range []string{"PlaybackInfo", "ItemDetail"} {
		props, required := schema(name)
		p, _ := props["nextEpisode"].(map[string]any)
		desc, _ := p["description"].(string)
		if p == nil || p["$ref"] != itemRefPointer || slices.Contains(required, any("nextEpisode")) || !strings.Contains(desc, "podcast") {
			t.Errorf("%s.nextEpisode = %v, want an optional ItemRef that names the podcast rule", name, p)
		}
	}
	if props, _ := schema("ItemDetail"); props["previousEpisode"] != nil {
		t.Error("ItemDetail.previousEpisode exists; the detail page does not need it (concept 23 section 3.3)")
	}
	// Every operation answering with PlaybackInfo or DownloadState is scoped.
	paths, _ := doc["paths"].(map[string]any)
	found := map[string]int{}
	for path, item := range paths {
		ops, _ := item.(map[string]any)
		for _, method := range httpMethods {
			op, ok := ops[method].(map[string]any)
			if !ok {
				continue
			}
			for _, name := range []string{"PlaybackInfo", "DownloadState"} {
				if !references(doc, op["responses"], "#/components/schemas/"+name, map[string]bool{}) {
					continue
				}
				found[name]++
				if op["x-visibility"] != "scoped" {
					t.Errorf("%s %s answers with %s, x-visibility %v, want scoped", strings.ToUpper(method), path, name, op["x-visibility"])
				}
			}
		}
	}
	if found["PlaybackInfo"] == 0 || found["DownloadState"] == 0 {
		t.Errorf("operations with PlaybackInfo or DownloadState: %s", fmt.Sprint(found))
	}
}
