package metadata

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Metadata languages as a list (concept 10 step 1, ADR 0018): the settings with their defaults
// and validation, the static language list, and the match that fetches every language of the
// list (the critic's blocker: a folder with one provider gets a third language only through a new
// match) and prunes the rows of a language that left the list.

func TestKnownLanguages(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"de", "en", "es", "ja", "pt-BR", "zh-TW", "rm"} {
		if !KnownLanguage(ok) {
			t.Errorf("%s should be a known language", ok)
		}
	}
	for _, bad := range []string{"", "DE", "de-xx", "de-XX", "german", "d", "es-AR", "en_US"} {
		if KnownLanguage(bad) {
			t.Errorf("%q should not be a known language", bad)
		}
	}
	if PrimaryLanguage("pt-BR") != "pt" || PrimaryLanguage("de") != "de" {
		t.Error("PrimaryLanguage")
	}
	if got := primaryLanguages([]string{"de", "pt-BR", "pt-PT", "en"}); !slices.Equal(got, []string{"de", "pt", "en"}) {
		t.Errorf("primaryLanguages = %v", got)
	}
}

func TestSettingsDefaultsParseAndValidate(t *testing.T) {
	t.Parallel()
	d := DefaultSettings()
	if !slices.Equal(d.TextLangs, []string{"de", "en"}) || !slices.Equal(d.ImageLangs["poster"], []string{"de", "en", ""}) ||
		d.ImageMinWidth["poster"] != 500 || d.RatingCountry != "DE" || !d.LocalImages || d.FanartEnabled {
		t.Fatalf("defaults: %+v", d)
	}
	if verr := d.Validate(); verr != nil {
		t.Fatalf("defaults do not validate: %v", verr)
	}
	// Stored rows win; a row that does not parse keeps the default and is reported.
	s, err := parseSettings(settingRows{
		SettingTextLangs:                   `["de","en","es"]`,
		SettingImageLangsPrefix + "poster": `["","de"]`,
		SettingImageMinPrefix + "backdrop": "1920",
		SettingImageByVote:                 "0",
		SettingRatingCountry:               "CH",
		SettingFanartEnabled:               "1",
		SettingLocalImages:                 "maybe",
	})
	if err == nil {
		t.Error("a value that does not parse should be reported")
	}
	if !slices.Equal(s.TextLangs, []string{"de", "en", "es"}) || !slices.Equal(s.ImageLangs["poster"], []string{"", "de"}) ||
		s.ImageMinWidth["backdrop"] != 1920 || s.ImageByVote || s.RatingCountry != "CH" || !s.FanartEnabled || !s.LocalImages {
		t.Errorf("parsed: %+v", s)
	}
	if s.Rank("es") != 2 || s.Rank("fr") != 3 {
		t.Errorf("rank es=%d fr=%d", s.Rank("es"), s.Rank("fr"))
	}
	if got := s.ImageLanguages(); !slices.Equal(got, []string{"de", "en"}) {
		t.Errorf("image languages = %v", got)
	}
	// rows() round-trips.
	back, err := parseSettings(s.rows())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(back.TextLangs, s.TextLangs) || back.RatingCountry != s.RatingCountry || back.ImageMinWidth["backdrop"] != 1920 {
		t.Errorf("round trip: %+v", back)
	}

	// Validation (9.5): the server's list needs de (F-9), a library's list de or en (E-9).
	cases := []struct {
		field  string
		langs  []string
		server bool
		want   string
	}{
		{"textLangs", nil, true, "langs_empty"},
		{"textLangs", []string{"en", "xx-YY"}, true, "lang_unknown"},
		{"textLangs", []string{"de", "de"}, true, "chain_duplicate"},
		{"textLangs", []string{"en", "es"}, true, "ui_lang_required"},
		{"textLangs", []string{"ja", "es"}, false, "ui_lang_required"},
		{"textLangs", []string{"ja", "en"}, false, ""},
	}
	for _, c := range cases {
		verr := ValidateTextLangs(c.field, c.langs, c.server)
		if c.want == "" {
			if verr != nil {
				t.Errorf("%v server=%v: unexpected %v", c.langs, c.server, verr)
			}
			continue
		}
		if verr == nil || !hasFieldError(verr.Fields, c.field, c.want) {
			t.Errorf("%v server=%v: got %v, want %s", c.langs, c.server, verr, c.want)
		}
	}
	bad := DefaultSettings()
	bad.ImageLangs["logo"] = []string{"de", "de"}
	bad.RatingCountry = "FR"
	verr := bad.Validate()
	if verr == nil || !hasFieldError(verr.Fields, "imageLangs.logo", "chain_duplicate") || !hasFieldError(verr.Fields, "ratingCountry", "country_unknown") {
		t.Errorf("validate: %v", verr)
	}
}

func hasFieldError(fields []model.FieldError, field, code string) bool {
	for _, f := range fields {
		if f.Field == field && f.Code == code {
			return true
		}
	}
	return false
}

// setTextLangs stores the server's text language list.
func (e *env) setTextLangs(t *testing.T, langs ...string) {
	t.Helper()
	raw, _ := json.Marshal(langs)
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, 0) ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		SettingTextLangs, string(raw))
}

// textRanks returns lang -> rank of the stored texts of an item.
func (e *env) textRanks(t *testing.T, id string) map[string]int {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT lang, rank FROM item_texts WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var lang string
		var rank int
		if err := rows.Scan(&lang, &rank); err != nil {
			t.Fatal(err)
		}
		out[lang] = rank
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// The blocker case of the critic (concept 13): a folder with one provider ([tmdb]) gets a third
// language through a new match, not through fill. The match asks TMDB once per language of the
// list, writes one text row per language with its rank, the genres object carries the third key
// (and the trigger fills item_genres), and a later match with a shorter list deletes the rows of
// the language that left (6.1).
func TestMatchFetchesEveryLanguageOfTheListAndPrunesTheRest(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	e.tmdb.setExtraLanguage("550", "es", "Primera Película", "Descripción en español", []string{"Drama", "Comedia"})
	e.setTextLangs(t, "de", "en", "es")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	ctx := context.Background()
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	texts := e.texts(t, idByTMDB)
	if texts["es"].Title != "Primera Película" || texts["es"].Overview.String != "Descripción en español" {
		t.Fatalf("es texts after the match: %+v", texts["es"])
	}
	if texts["de"].Title != "Erster Film" || texts["en"].Title != "First Film" {
		t.Fatalf("de/en texts changed: %+v %+v", texts["de"], texts["en"])
	}
	if ranks := e.textRanks(t, idByTMDB); ranks["de"] != 0 || ranks["en"] != 1 || ranks["es"] != 2 {
		t.Errorf("ranks = %v, want de 0, en 1, es 2", ranks)
	}
	if n := e.tmdb.count("/3/movie/550"); n != 3 {
		t.Errorf("TMDB movie requests = %d, want one per language", n)
	}
	var genres map[string][]string
	if err := json.Unmarshal([]byte(e.item(t, idByTMDB).Genres), &genres); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(genres["es"], []string{"Drama", "Comedia"}) || len(genres) != 3 {
		t.Errorf("genres = %v", genres)
	}
	var esGenres int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM item_genres WHERE item_id = ? AND lang = 'es'`, idByTMDB).Scan(&esGenres); err != nil || esGenres != 2 {
		t.Errorf("item_genres es rows = %d (%v), want 2", esGenres, err)
	}

	// The list shrinks: the next match deletes the es row and the genres object loses the key.
	e.setTextLangs(t, "de", "en")
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if ranks := e.textRanks(t, idByTMDB); len(ranks) != 2 || ranks["de"] != 0 || ranks["en"] != 1 {
		t.Errorf("ranks after the shorter list = %v, want only de 0, en 1", ranks)
	}
	genres = nil
	if err := json.Unmarshal([]byte(e.item(t, idByTMDB).Genres), &genres); err != nil {
		t.Fatal(err)
	}
	if _, ok := genres["es"]; ok || len(genres) != 2 {
		t.Errorf("genres after the shorter list = %v", genres)
	}

	// A locked text keeps every row: the administrator's text may live in any of them.
	e.setTextLangs(t, "de", "en", "es")
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["overview"]', metadata_state = 'pending' WHERE id = ?`, idByTMDB)
	e.setTextLangs(t, "de", "en")
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if ranks := e.textRanks(t, idByTMDB); len(ranks) != 3 {
		t.Errorf("ranks with a locked overview = %v, want the es row kept", ranks)
	}
}

// A profile's choice: AvailableMetadataLanguages is the server's list (the libraries' lists join
// it in step 3), and ProviderLanguages names what each provider delivers of it.
func TestAvailableAndProviderLanguages(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	got, err := e.svc.AvailableMetadataLanguages(ctx)
	if err != nil || !slices.Equal(got, []string{"de", "en"}) {
		t.Fatalf("default available = %v, %v", got, err)
	}
	e.setTextLangs(t, "de", "en", "rm")
	got, err = e.svc.AvailableMetadataLanguages(ctx)
	if err != nil || !slices.Equal(got, []string{"de", "en", "rm"}) {
		t.Fatalf("available = %v, %v", got, err)
	}
	pl, err := e.svc.ProviderLanguages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pl["tmdb"], []string{"de", "en", "rm"}) {
		t.Errorf("tmdb languages = %v", pl["tmdb"])
	}
	// The fake TVDB has no key here: the static table knows de and en, not Romansh.
	if !slices.Equal(pl["tvdb"], []string{"de", "en"}) {
		t.Errorf("tvdb languages = %v, want de en (no Romansh)", pl["tvdb"])
	}
	if !slices.Equal(pl["anilist"], []string{"en"}) || len(pl["local"]) != 0 {
		t.Errorf("anilist %v local %v", pl["anilist"], pl["local"])
	}
}
