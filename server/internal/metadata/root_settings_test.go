package metadata

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Concept 24 section 3 (ADR 0036): a folder's own text languages and images, resolved per title
// through its home folder (items.root_id), live; the languages a profile may choose over every
// folder; the provider hint per list; TMDB's image languages as the union of every chain.

// ownFolder registers folder id in library lib with content and the own settings d, and makes it
// the home folder of the given titles.
func ownFolder(t *testing.T, db interface {
	Exec(string, ...any) (sql.Result, error)
}, id, lib int64, content string, d model.RootMetadata, items ...string) {
	t.Helper()
	tl, im := model.RootMetadataColumns(d)
	if _, err := db.Exec(`INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at, content, providers, text_langs, images)
		VALUES (?, ?, '/media/folder-' || ?, 'ext4', 'poll', 0, ?, '["tmdb"]', ?, ?)`, id, lib, id, content, tl, im); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if _, err := db.Exec(`UPDATE items SET root_id = ? WHERE id = ?`, id, it); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEffectivePerTitle(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	// A series in library 2 with a season and an episode; its home folder 2 reads ja, en and textless
	// posters first, logos only. Folder 3 of the same library has nothing of its own.
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at)
		VALUES ('01K24SER1ES000000000000001', 'series', 2, 'S', 's', 0, 0, 0)`)
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, added_at, created_at, updated_at)
		VALUES ('01K24SEAS0N000000000000001', 'season', '01K24SER1ES000000000000001', 2, 'S1', 's1', 0, 0, 0)`)
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, added_at, created_at, updated_at)
		VALUES ('01K24EP1S0DE00000000000001', 'episode', '01K24SEAS0N000000000000001', 2, 'E1', 'e1', 0, 0, 0)`)
	ja := []string{"ja", "en"}
	ownFolder(t, e.db.Write, 2, 2, model.RootSeries, model.RootMetadata{TextLangs: &ja,
		Images: &model.RootImages{Langs: map[string][]string{"poster": {"", "ja"}}, Kinds: []string{ImageKindLogo}}},
		"01K24SER1ES000000000000001", "01K24SEAS0N000000000000001", "01K24EP1S0DE00000000000001")
	ownFolder(t, e.db.Write, 3, 2, model.RootSeries, model.RootMetadata{})
	e.addItem(t, itemSeed{id: idByTMDB, title: "Film"})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Other"})
	ownFolder(t, e.db.Write, 1, 1, model.RootMovie, model.RootMetadata{}, idByIMDB)

	item := func(id string) systemq.Item {
		t.Helper()
		it, err := e.svc.read.GetItemSystem(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
	def := DefaultSettings()
	for _, id := range []string{"01K24SER1ES000000000000001", "01K24SEAS0N000000000000001", "01K24EP1S0DE00000000000001"} {
		st, err := e.svc.Effective(ctx, item(id))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(st.TextLangs, ja) || !slices.Equal(st.ImageLangs["poster"], []string{"", "ja"}) ||
			!slices.Equal(st.ImageLangs["backdrop"], def.ImageLangs["backdrop"]) || !slices.Equal(st.ImageKinds, []string{ImageKindLogo}) {
			t.Errorf("%s in its own folder: %+v", id, st)
		}
	}
	// A title in a folder without own settings, and one without home folder: the default.
	for _, id := range []string{idByIMDB, idByTMDB} {
		st, err := e.svc.Effective(ctx, item(id))
		if err != nil || !slices.Equal(st.TextLangs, def.TextLangs) || !slices.Equal(st.ImageKinds, def.ImageKinds) {
			t.Errorf("%s: %+v %v, want the default", id, st, err)
		}
	}
	// A series with files in two folders reads its home folder only: moving the home folder to
	// folder 3 gives the default, without rewriting anything else.
	mustExec(t, e.db.Write, `UPDATE items SET root_id = 3 WHERE id = '01K24SER1ES000000000000001'`)
	if st, _ := e.svc.Effective(ctx, item("01K24SER1ES000000000000001")); !slices.Equal(st.TextLangs, def.TextLangs) {
		t.Errorf("series moved to folder 3: %v", st.TextLangs)
	}
	// Live: a changed folder applies at once to the titles that live there.
	fr := []string{"fr"}
	tl, _ := model.RootMetadataColumns(model.RootMetadata{TextLangs: &fr})
	mustExec(t, e.db.Write, `UPDATE library_roots SET text_langs = ? WHERE id = 3`, tl)
	if st, _ := e.svc.Effective(ctx, item("01K24SER1ES000000000000001")); !slices.Equal(st.TextLangs, fr) {
		t.Errorf("after folder 3 got fr: %v", st.TextLangs)
	}
	// An own image setting with no kind on is kept as no kind, not as the default.
	_, im := model.RootMetadataColumns(model.RootMetadata{Images: &model.RootImages{}})
	mustExec(t, e.db.Write, `UPDATE library_roots SET images = ? WHERE id = 1`, im)
	if st, _ := e.svc.Effective(ctx, item(idByIMDB)); len(st.ImageKinds) != 0 || !slices.Equal(st.ImageLangs["poster"], def.ImageLangs["poster"]) {
		t.Errorf("empty kinds: %v %v", st.ImageKinds, st.ImageLangs)
	}
	// An audiobook or podcast folder has no own settings, even if a row carried some.
	mustExec(t, e.db.Write, `UPDATE library_roots SET content = 'podcast' WHERE id = 3`)
	if st, _ := e.svc.Effective(ctx, item("01K24SER1ES000000000000001")); !slices.Equal(st.TextLangs, def.TextLangs) {
		t.Errorf("podcast folder: %v", st.TextLangs)
	}
}

func TestLanguagesOverFolders(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	ja := []string{"ja", "en"}
	it := []string{"it", "de"}
	ownFolder(t, e.db.Write, 1, 1, model.RootMovie, model.RootMetadata{TextLangs: &it,
		Images: &model.RootImages{Langs: map[string][]string{"logo": {"it", "fr"}}, Kinds: []string{ImageKindLogo}}})
	ownFolder(t, e.db.Write, 2, 2, model.RootSeries, model.RootMetadata{TextLangs: &ja,
		Images: &model.RootImages{Langs: map[string][]string{"poster": {"", "ja"}}}})
	// A podcast folder's row does not count (concept 24 3.1).
	rm := []string{"rm"}
	ownFolder(t, e.db.Write, 4, 2, model.RootPodcast, model.RootMetadata{TextLangs: &rm})

	got, err := e.svc.AvailableMetadataLanguages(ctx)
	if err != nil || !slices.Equal(got, []string{"de", "en", "it", "ja"}) {
		t.Errorf("available = %v, %v; want the default first, then the folders' languages", got, err)
	}
	// TMDB's image languages: the default's chains (de, en), then the logo chain of folder 1 and the
	// poster chain of folder 2; folder 2 keeps the default's logo chain, which adds nothing.
	img, err := e.svc.imageLanguages(ctx)
	if err != nil || !slices.Equal(img, []string{"de", "en", "it", "fr", "ja"}) {
		t.Errorf("image languages = %v, %v", img, err)
	}
	// The provider hint for the list being edited: the default's, or a folder's.
	pl, err := e.svc.ProviderLanguages(ctx, nil)
	if err != nil || !slices.Equal(pl["tmdb"], []string{"de", "en"}) {
		t.Errorf("default list: %v %v", pl, err)
	}
	pl, err = e.svc.ProviderLanguages(ctx, []string{"ja", "en"})
	if err != nil || !slices.Equal(pl["tmdb"], []string{"ja", "en"}) || !slices.Equal(pl["anilist"], []string{"en"}) {
		t.Errorf("folder list: %v %v", pl, err)
	}
}

func TestValidateRootMetadata(t *testing.T) {
	t.Parallel()
	empty := []string{}
	unknown := []string{"klingon"}
	for name, c := range map[string]struct {
		d     model.RootMetadata
		field string
	}{
		"empty list":         {model.RootMetadata{TextLangs: &empty}, "textLangs"},
		"unknown language":   {model.RootMetadata{TextLangs: &unknown}, "textLangs"},
		"unknown kind":       {model.RootMetadata{Images: &model.RootImages{Kinds: []string{"thumb"}}}, "images.kinds"},
		"unknown chain kind": {model.RootMetadata{Images: &model.RootImages{Langs: map[string][]string{"still": {"de"}}}}, "images.langs"},
		"duplicate in chain": {model.RootMetadata{Images: &model.RootImages{Langs: map[string][]string{"poster": {"de", "de"}}}}, "images.langs.poster"},
	} {
		verr := ValidateRootMetadata(c.d)
		if verr == nil || !slices.ContainsFunc(verr.Fields, func(f model.FieldError) bool { return f.Field == c.field }) {
			t.Errorf("%s: %+v, want a field error on %s", name, verr, c.field)
		}
	}
	jaOnly := []string{"ja"}
	if verr := ValidateRootMetadata(model.RootMetadata{TextLangs: &jaOnly, Images: &model.RootImages{}}); verr != nil {
		t.Errorf("ja only with no image kind: %v", verr)
	}
}
