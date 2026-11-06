package metadata

import (
	"context"
	"testing"
)

// Tests of the text locks per language (concept 15 7.5, F-10): title:de, overview:de and
// tagline:de keep the text of one language, the other languages follow the providers; the old
// lock without language still holds every language.

// seedText stores a text row of an item (rank 0 for de, 1 for en, 2 otherwise).
func (e *env) seedText(t *testing.T, id, lang, title, overview, tagline string) {
	t.Helper()
	rank := map[string]int{"de": 0, "en": 1}[lang]
	if lang != "de" && lang != "en" {
		rank = 2
	}
	mustExec(t, e.db.Write, `INSERT INTO item_texts (item_id, lang, title, overview, tagline, sort_key, rank)
		VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), lower(?), ?)`, id, lang, title, overview, tagline, title, rank)
}

func TestMatchKeepsTextLocksPerLanguage(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550, locked: `["title:de","overview:de","tagline:en","title:fr"]`})
	e.seedText(t, idByTMDB, "de", "Mein Titel", "Meine Beschreibung", "")
	e.seedText(t, idByTMDB, "en", "Old title", "Old overview", "My tagline")
	e.seedText(t, idByTMDB, "fr", "Mon titre", "", "") // not in the list, but locked
	e.seedText(t, idByTMDB, "it", "Il titolo", "", "") // not in the list, free: pruned
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	tx := e.texts(t, idByTMDB)
	if tx["de"].Title != "Mein Titel" || tx["de"].Overview.String != "Meine Beschreibung" {
		t.Fatalf("locked de texts rewritten: %+v", tx["de"])
	}
	if tx["de"].Tagline.String != "English tagline" {
		t.Fatalf("free de tagline %+v, want the provider's", tx["de"])
	}
	if tx["en"].Title != "First Film" || tx["en"].Overview.String != "English overview of First Film" {
		t.Fatalf("free en texts kept: %+v, want the provider's", tx["en"])
	}
	if tx["en"].Tagline.String != "My tagline" {
		t.Fatalf("locked en tagline rewritten: %+v", tx["en"])
	}
	if _, ok := tx["fr"]; !ok {
		t.Fatalf("locked fr row pruned: %v", tx)
	}
	if _, ok := tx["it"]; ok {
		t.Fatalf("free it row kept: %v", tx)
	}
}

func TestMatchLegacyTextLockHoldsEveryLanguage(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550, locked: `["overview"]`})
	e.seedText(t, idByTMDB, "de", "Alt", "Von Hand", "")
	e.seedText(t, idByTMDB, "en", "Old", "By hand", "")
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	tx := e.texts(t, idByTMDB)
	if tx["de"].Overview.String != "Von Hand" || tx["en"].Overview.String != "By hand" {
		t.Fatalf("lock without language did not hold both languages: %+v", tx)
	}
	if tx["de"].Title != "Erster Film" || tx["en"].Title != "First Film" {
		t.Fatalf("free titles %+v, want the provider's", tx)
	}
}

func TestFillKeepsTextLocksPerLanguage(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE item_texts SET overview = 'Von Hand' WHERE item_id = ? AND lang = 'de'`, idByTMDB)
	mustExec(t, e.db.Write, `UPDATE item_texts SET overview = 'By hand' WHERE item_id = ? AND lang = 'en'`, idByTMDB)
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["overview:de"]' WHERE id = ?`, idByTMDB)
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	tx := e.texts(t, idByTMDB)
	if tx["de"].Overview.String != "Von Hand" {
		t.Fatalf("locked de overview rewritten: %+v", tx["de"])
	}
	if tx["en"].Overview.String != "English overview of First Film" {
		t.Fatalf("free en overview %+v, want the provider's", tx["en"])
	}
	src := e.sources(t, idByTMDB)
	if _, ok := src["overview/de"]; ok {
		t.Fatalf("locked de overview got a source: %v", src)
	}
	if src["overview/en"] != "tmdb" || src["title/de"] != "tmdb" {
		t.Fatalf("sources %v", src)
	}
}

func TestDecideFieldsTextLocks(t *testing.T) {
	t.Parallel()
	c := providerFields{provider: "tmdb", values: fieldValues{
		{"title", "de"}: "Titel", {"title", "en"}: "Title", {"overview", "de"}: "Text", {"overview", "en"}: "Text en",
		{"original_title", ""}: "Original",
	}}
	got := decideFields([]string{"tmdb"}, []providerFields{c}, nil, map[string]bool{"title:de": true, "overview": true, "original_title:de": true})
	for _, k := range []fieldKey{{"title", "de"}, {"overview", "de"}, {"overview", "en"}} {
		if _, ok := got[k]; ok {
			t.Errorf("%v decided despite its lock", k)
		}
	}
	for _, k := range []fieldKey{{"title", "en"}, {"original_title", ""}} {
		if _, ok := got[k]; !ok {
			t.Errorf("%v not decided, but it is free", k)
		}
	}
}
