package visibility

import (
	"context"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// The text chain of concept 10 (4.2, ADR 0018): metadata language of the profile, then its
// interface language, then the rows by rank, field by field; the genres and their filter follow
// the same chain. The scoped reads read no setting: the rank in the row carries the server's
// order (rule 2 and 12).

func rankedText(lang, title, overview string, rank int64) scopedq.ItemText {
	t := text(lang, title, overview)
	t.Rank = rank
	return t
}

func TestPickTextsChainWithThirdLanguage(t *testing.T) {
	t.Parallel()
	rows := []scopedq.ItemText{
		rankedText("de", "Chihiros Reise", "Deutsche Handlung", 0),
		rankedText("en", "Spirited Away", "English plot", 1),
		rankedText("ja", "千と千尋の神隠し", "", 2),
	}
	// A profile reading Japanese with a German interface: the Japanese title, the overview falls
	// back field by field to German (the interface language), not to English.
	got := pickTexts(rows, langPref{meta: "ja", ui: "de"}, "spirited away (2001)")
	if got.title != "千と千尋の神隠し" || got.overview != "Deutsche Handlung" {
		t.Errorf("ja/de = %+v", got)
	}
	// A profile reading Spanish (no row): the interface language English, then the ranks.
	got = pickTexts(rows, langPref{meta: "es", ui: "en"}, "x")
	if got.title != "Spirited Away" {
		t.Errorf("es/en = %+v", got)
	}
	// Neither language has a row: the lowest rank wins, whatever the code order.
	onlyRanks := []scopedq.ItemText{rankedText("ja", "JA", "", 1), rankedText("fr", "FR", "", 0)}
	if got := pickTexts(onlyRanks, langPref{meta: "es", ui: "de"}, "x"); got.title != "FR" {
		t.Errorf("rank order = %+v, want the rank-0 row", got)
	}
	// Genres: the metadata language, the interface language, the title's languages by rank, en.
	raw := `{"de":["Abenteuer"],"en":["Adventure"],"ja":[]}`
	if g := pickGenres(raw, langPref{meta: "ja", ui: "de"}, []string{"ja", "de", "en"}); !slices.Equal(g, []string{"Abenteuer"}) {
		t.Errorf("ja/de genres = %v", g)
	}
	if g := pickGenres(`{"de":[],"ja":["冒険"]}`, langPref{meta: "es", ui: "en"}, []string{"ja", "de"}); !slices.Equal(g, []string{"冒険"}) {
		t.Errorf("es/en genres by rank = %v", g)
	}
	if g := pickGenres(`["Drama"]`, langPref{meta: "es", ui: "de"}, nil); !slices.Equal(g, []string{"Drama"}) {
		t.Errorf("plain list = %v", g)
	}
}

// A profile with a third metadata language reads the third language where it exists, the
// interface language where it does not, and filters genres in it (the library page and the
// discover genres, both from item_genres which the trigger fills from the genres object).
func TestThirdLanguageThroughTheReader(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	movie := f.Rated(12)
	exec(t, f, `INSERT INTO item_texts (item_id, lang, title, overview, rank, sort_key) VALUES (?, 'es', 'Película 12', '', 2, 'pelicula 12')`, movie.ID)
	exec(t, f, `UPDATE items SET genres = '{"de":["Abenteuer"],"en":["Adventure"],"es":["Aventura"]}' WHERE id = ?`, movie.ID)

	es := model.Profile{ID: f.P18.ID, Language: "de", MetadataLanguage: "es"}
	d, err := r.Item(ctx, Profile(es), movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Película 12" {
		t.Errorf("title = %q, want the Spanish text", d.Title)
	}
	if d.Overview != "Handlung 12" {
		t.Errorf("overview = %q, want the German one (field-wise fallback to the interface language)", d.Overview)
	}
	if !slices.Equal(d.Genres, []string{"Aventura"}) {
		t.Errorf("genres = %v", d.Genres)
	}
	// Another title without Spanish texts: German, the interface language.
	other, err := r.Item(ctx, Profile(es), f.Rated(6).ID)
	if err != nil {
		t.Fatal(err)
	}
	if other.Title != "Film 06" || !slices.Equal(other.Genres, []string{"Abenteuer"}) {
		t.Errorf("title without es row = %q %v, want the German text and genres", other.Title, other.Genres)
	}

	// The library page: the Spanish genre filters and appears in the facets; the German genre
	// still filters (the chain lets the interface language through).
	page := libraryItems(t, r, es, f.LibraryID(), model.LibraryQuery{Genre: "Aventura"})
	if !slices.Equal(ids(page.Items), []string{movie.ID}) {
		t.Errorf("genre Aventura = %v, want the 12-rated film", ids(page.Items))
	}
	var names []string
	for _, g := range page.Facets.Genres {
		names = append(names, g.Name)
	}
	if !slices.Contains(names, "Aventura") {
		t.Errorf("facets genres = %v, want Aventura", names)
	}
	all := libraryItems(t, r, es, f.LibraryID(), model.LibraryQuery{Genre: "Abenteuer"})
	if !slices.Contains(ids(all.Items), f.Rated(6).ID) {
		t.Errorf("genre Abenteuer for the es profile = %v, want the German-only titles too", ids(all.Items))
	}
	// The sort key follows the chain too: the Spanish key sorts the title under P.
	sorted := libraryItems(t, r, es, f.LibraryID(), model.LibraryQuery{Letter: "P"})
	if !slices.Equal(ids(sorted.Items), []string{movie.ID}) {
		t.Errorf("letter P = %v, want the film with the Spanish key", ids(sorted.Items))
	}

	// Discover genres: Spanish has one genre, so the list is Spanish; a profile reading a language
	// without any genre gets the interface language.
	genres, err := r.Genres(ctx, Profile(es), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(genres) != 1 || genres[0].Name != "Aventura" {
		t.Errorf("discover genres es = %+v", genres)
	}
	fr := model.Profile{ID: f.P18.ID, Language: "de", MetadataLanguage: "fr"}
	genres, err = r.Genres(ctx, Profile(fr), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(genres) == 0 || genres[0].Name != "Abenteuer" {
		t.Errorf("discover genres fr (none) = %+v, want the German list", genres)
	}
}
