package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Tests of the original language (concept 13 11.7): the normalisation onto ISO 639-1, the field
// in the fill order (TMDB, TVDB, AniList) with provenance and lock, the match carrying the
// identifier's value, metadata.original_language_backfill taking only NULL titles and its single
// automatic queueing, and the administrator's value.

// idLangKnown is a film whose original language was looked up already (backfill test).
const idLangKnown = "01JTESTMOVIE0000000000000K"

// setTMDBLanguage sets original_language of a fixture film in both languages ("" removes it).
func (e *env) setTMDBLanguage(id, lang string) {
	e.tmdb.mu.Lock()
	defer e.tmdb.mu.Unlock()
	fx := e.tmdb.movies[id]
	for _, m := range []map[string]any{fx.de, fx.en} {
		if lang == "" {
			delete(m, "original_language")
		} else {
			m["original_language"] = lang
		}
	}
}

func (e *env) setAniListCountry(id int64, country string) {
	e.al.mu.Lock()
	defer e.al.mu.Unlock()
	e.al.country[id] = country
}

// origLang returns items.original_language of an item, "<null>" for NULL.
func (e *env) origLang(t *testing.T, id string) string {
	t.Helper()
	var v sql.NullString
	if err := e.db.Read.QueryRow(`SELECT original_language FROM items WHERE id = ?`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if !v.Valid {
		return "<null>"
	}
	return v.String
}

func validationField(err error) string {
	var v *model.ValidationError
	if !errors.As(err, &v) || len(v.Fields) != 1 {
		return ""
	}
	return v.Fields[0].Field + "/" + v.Fields[0].Code
}

func TestNormalizeOriginalLanguage(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"en": "en", "EN": "en", " ja ": "ja", "pt-BR": "pt", "cn": "zh", "zh": "zh", "xx": "", "": "",
		"zz": "", "eng": "en", "deu": "de", "jpn": "ja", "iw": "he", "nb": "no", "und": "", "e": "",
	} {
		if got := normalizeOriginalLanguage(in); got != want {
			t.Errorf("normalizeOriginalLanguage(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"JP": "ja", "jp": "ja", "KR": "ko", "CN": "zh", "TW": "", "US": "", "": ""} {
		if got := countryLanguage(in); got != want {
			t.Errorf("countryLanguage(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]bool{"": true, "ja": true, "de": true, "cn": false, "xx": false, "EN": false, "jpn": false, "pt-BR": false} {
		if got := ValidOriginalLanguage(in); got != want {
			t.Errorf("ValidOriginalLanguage(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestTVDBLanguageFollowsTheTrackTable: TVDB's ISO 639-3 id comes back through the shared language
// table of the track choice (trackpick.Normalize), so Norwegian is no whether the table names it
// nb or no (review step 8, S8-m-4), and the original language compares with the tracks.
func TestTVDBLanguageFollowsTheTrackTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, table := range map[string]map[string]string{
		"static":  tvdbStaticLangs,
		"only nb": {"nb": "nor", "de": "deu", "ja": "jpn"},
		"only no": {"no": "nor", "de": "deu", "ja": "jpn"},
	} {
		p := &tvdbProvider{languages: table}
		for id, want := range map[string]string{"nor": "no", "NOR": "no", "deu": "de", "jpn": "ja", "xyz": "", "": ""} {
			if got := p.shortLanguage(ctx, id); got != want {
				t.Errorf("%s: shortLanguage(%q) = %q, want %q", name, id, got, want)
			}
		}
	}
}

// TestMatchCarriesOriginalLanguage: a list with one provider gets the identifier's language from
// the match, and the fill records the identifier as its source; a match without a language keeps
// a known value of the same title; a title TMDB names none for is "looked up, unknown".
func TestMatchCarriesOriginalLanguage(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.setTMDBLanguage("550", "en")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Searched", tmdbID: 601})
	for _, id := range []string{idByTMDB, idByIMDB} {
		if err := e.svc.match(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := e.svc.fill(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.origLang(t, idByTMDB); got != "en" {
		t.Fatalf("film 550 original language %q, want en", got)
	}
	if src := e.sources(t, idByTMDB); src["original_language/"] != "tmdb" {
		t.Fatalf("film 550 sources %v, want original_language from tmdb", src)
	}
	if got := e.origLang(t, idByIMDB); got != "" {
		t.Fatalf("film 601 original language %q, want '' (looked up, unknown)", got)
	}
	if _, ok := e.sources(t, idByIMDB)["original_language/"]; ok {
		t.Fatalf("an unknown language got a source: %v", e.sources(t, idByIMDB))
	}
	e.setTMDBLanguage("550", "")
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.origLang(t, idByTMDB); got != "en" {
		t.Fatalf("a match without language cleared the known one: %q", got)
	}
}

// TestFillOriginalLanguageInListOrder: with ["tmdb","anilist"] TMDB's language wins, AniList's
// country steps in where TMDB names none (or xx), the source says who; "" defends nothing, so a
// later TMDB value (cn, normalised to zh) replaces it; a locked language stays.
func TestFillOriginalLanguageInListOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.setTMDBLanguage("550", "en")
	e.setTMDBLanguage("602", "xx")
	e.setAniListCountry(1001, "JP")
	e.setAniListCountry(1002, "KR")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Searched", tmdbID: 601}) // AniList 1002 through tt0000601
	e.addItem(t, itemSeed{id: idNoMatch, title: "Unrated", tmdbID: 602}) // no AniList entry
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE library_id = 1`)
	run := func() {
		t.Helper()
		for _, id := range []string{idByTMDB, idByIMDB, idNoMatch} {
			if err := e.svc.match(ctx, id); err != nil {
				t.Fatal(id, err)
			}
			if err := e.svc.fill(ctx, id); err != nil {
				t.Fatal(id, err)
			}
		}
	}
	run()
	for id, want := range map[string][2]string{idByTMDB: {"en", "tmdb"}, idByIMDB: {"ko", "anilist"}, idNoMatch: {"", ""}} {
		if got, src := e.origLang(t, id), e.sources(t, id)["original_language/"]; got != want[0] || src != want[1] {
			t.Errorf("%s: original language %q from %q, want %q from %q", id, got, src, want[0], want[1])
		}
	}
	// TMDB learns the language of 602 (Cantonese, cn): the unknown value gives way.
	e.setTMDBLanguage("602", "cn")
	run()
	if got, src := e.origLang(t, idNoMatch), e.sources(t, idNoMatch)["original_language/"]; got != "zh" || src != "tmdb" {
		t.Fatalf("602 after TMDB named cn: %q from %q, want zh from tmdb", got, src)
	}
	// A locked language stays through match and fill.
	mustExec(t, e.db.Write, `UPDATE items SET original_language = 'de', locked_fields = '["original_language"]' WHERE id = ?`, idByTMDB)
	mustExec(t, e.db.Write, `DELETE FROM item_field_sources WHERE item_id = ? AND field = 'original_language'`, idByTMDB)
	e.setTMDBLanguage("550", "fr")
	run()
	if got := e.origLang(t, idByTMDB); got != "de" {
		t.Fatalf("locked original language changed to %q", got)
	}
	if src, ok := e.sources(t, idByTMDB)["original_language/"]; ok {
		t.Fatalf("locked original language got the source %q", src)
	}
}

// TestFillOriginalLanguageFromTVDB: TVDB's ISO 639-3 originalLanguage (deu) arrives as de; with
// TMDB in front its language wins once it has one.
func TestFillOriginalLanguageFromTVDB(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addNode(t, node{id: idSeries, kind: "series", title: "Only On TVDB", year: 2015})
	e.addNode(t, node{id: idSeason1, kind: "season", parent: idSeries, title: "Staffel 1", season: 1})
	e.setProviders(t, `["tvdb","tmdb"]`)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if got, src := e.origLang(t, idSeries), e.sources(t, idSeries)["original_language/"]; got != "de" || src != "tvdb" {
		t.Fatalf("series: %q from %q, want de from tvdb", got, src)
	}
	e.tmdb.mu.Lock()
	e.tmdb.tvOriginalLanguage = "en"
	e.tmdb.mu.Unlock()
	e.setProviders(t, `["tmdb","tvdb"]`)
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if got, src := e.origLang(t, idSeries), e.sources(t, idSeries)["original_language/"]; got != "en" || src != "tmdb" {
		t.Fatalf("series with TMDB first: %q from %q, want en from tmdb", got, src)
	}
}

// TestOriginalLanguageBackfillTakesOnlyNull: the backfill asks the providers of the NULL titles
// only (not a locked one, not one looked up already), writes the first value in list order with
// its source and "" without one; a second run asks nobody.
func TestOriginalLanguageBackfillTakesOnlyNull(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.setTMDBLanguage("550", "en")
	e.setAniListCountry(1002, "JP")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Searched", tmdbID: 601, imdbID: "tt0000601"}) // AniList 1002 by its IMDb id
	e.addItem(t, itemSeed{id: idNoMatch, title: "Unrated", tmdbID: 602})
	e.addItem(t, itemSeed{id: idLocked, title: "Locked", tmdbID: 550, locked: `["original_language"]`})
	e.addItem(t, itemSeed{id: idLangKnown, title: "Known", tmdbID: 550})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE library_id = 1`)
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE id = ?`, idByIMDB)
	mustExec(t, e.db.Write, `UPDATE items SET original_language = '' WHERE id = ?`, idLangKnown)
	last, err := e.svc.originalLanguageBackfill(ctx, "")
	if err != nil || last != "" {
		t.Fatalf("backfill: last %q, %v; want the only batch", last, err)
	}
	for id, want := range map[string][2]string{
		idByTMDB: {"en", "tmdb"}, idByIMDB: {"ja", "anilist"}, idNoMatch: {"", ""},
		idLocked: {"<null>", ""}, idLangKnown: {"", ""},
	} {
		if got, src := e.origLang(t, id), e.sources(t, id)["original_language/"]; got != want[0] || src != want[1] {
			t.Errorf("%s: original language %q from %q, want %q from %q", id, got, src, want[0], want[1])
		}
	}
	// 550 is asked for the one NULL title (two text languages), not for the locked or known one.
	if n := e.tmdb.count("/3/movie/550"); n != 2 {
		t.Fatalf("film 550 fetched %d times, want 2 (one title, de and en)", n)
	}
	calls := e.tmdb.total()
	if _, err := e.svc.originalLanguageBackfill(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if e.tmdb.total() != calls {
		t.Fatalf("a second run made %d requests", e.tmdb.total()-calls)
	}
	// The handler runs a batch from its payload; a broken cursor is refused.
	if err := e.svc.handleOriginalLanguageBackfill(ctx, &jobs.Job{Type: JobOriginalLanguageBackfill, Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleOriginalLanguageBackfill(ctx, &jobs.Job{Type: JobOriginalLanguageBackfill, Payload: json.RawMessage(`{"after":"x"}`)}); err == nil {
		t.Fatal("an invalid cursor was accepted")
	}
	if got := e.activeJobs(t, JobOriginalLanguageBackfill); len(got) != 0 {
		t.Fatalf("a short batch queued a successor: %v", got)
	}
}

// TestQueueOriginalLanguageBackfillOnce: the first start with a waiting title queues the chain at
// the fill priority and remembers it; later starts queue nothing, even after the job is gone; a
// first start without any waiting title queues nothing and remembers too. A run by hand queues
// at its priority and raises a waiting batch.
func TestQueueOriginalLanguageBackfillOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id = ?`, idByTMDB)
	if queued, err := e.svc.QueueOriginalLanguageBackfillOnce(ctx); err != nil || !queued {
		t.Fatalf("first start: queued %v, %v", queued, err)
	}
	if got := e.activeJobs(t, JobOriginalLanguageBackfill); !slices.Equal(got, []string{"original_language_backfill p2"}) {
		t.Fatalf("jobs after the first start %v", got)
	}
	// A run by hand raises the waiting batch instead of doubling it.
	ids, err := e.svc.runOriginalLanguageBackfill(ctx, e.queue, jobs.ManualPriority)
	if err != nil || len(ids) != 1 {
		t.Fatalf("run by hand: %v, %v", ids, err)
	}
	if got := e.activeJobs(t, JobOriginalLanguageBackfill); !slices.Equal(got, []string{"original_language_backfill p7"}) {
		t.Fatalf("jobs after the run by hand %v", got)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	for range 2 {
		if queued, err := e.svc.QueueOriginalLanguageBackfillOnce(ctx); err != nil || queued {
			t.Fatalf("later start: queued %v, %v", queued, err)
		}
	}
	if got := e.activeJobs(t, JobOriginalLanguageBackfill); len(got) != 0 {
		t.Fatalf("a later start queued %v", got)
	}

	empty := newEnv(t, goodToken)
	if queued, err := empty.svc.QueueOriginalLanguageBackfillOnce(ctx); err != nil || queued {
		t.Fatalf("first start without titles: queued %v, %v", queued, err)
	}
	empty.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	mustExec(t, empty.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id = ?`, idByTMDB)
	if queued, err := empty.svc.QueueOriginalLanguageBackfillOnce(ctx); err != nil || queued {
		t.Fatalf("second start after a start without titles: queued %v, %v", queued, err)
	}
}

// TestRunOriginalLanguageBackfillJoinsAnActiveChain: a run by hand while a successor of the chain
// waits raises that successor instead of starting a second chain (review step 8, S8-m-1); without
// an active chain it starts one.
func TestRunOriginalLanguageBackfillJoinsAnActiveChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, goodToken)
	successor, err := e.svc.enqueueOriginalLanguageBackfill(ctx, idByTMDB, fillPriority)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := e.svc.runOriginalLanguageBackfill(ctx, e.queue, jobs.ManualPriority)
	if err != nil || !slices.Equal(ids, []int64{successor}) {
		t.Fatalf("run by hand: %v, %v; want the waiting successor %d", ids, err, successor)
	}
	if got := e.activeJobs(t, JobOriginalLanguageBackfill); !slices.Equal(got, []string{"original_language_backfill:" + idByTMDB + " p7"}) {
		t.Fatalf("jobs after the run by hand %v, want only the raised successor", got)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	ids, err = e.svc.runOriginalLanguageBackfill(ctx, e.queue, jobs.ManualPriority)
	if err != nil || len(ids) != 1 {
		t.Fatalf("run without a chain: %v, %v", ids, err)
	}
	if got := e.activeJobs(t, JobOriginalLanguageBackfill); !slices.Equal(got, []string{"original_language_backfill p7"}) {
		t.Fatalf("jobs after a run without a chain %v", got)
	}
}

// TestSetItemOriginalLanguage: the administrator's code or "" is stored and locked without a
// source, a match keeps it; invalid codes and other kinds are refused; nil lifts the lock,
// clears the value and queues a manual match that looks it up again.
func TestSetItemOriginalLanguage(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.setTMDBLanguage("550", "en")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	ja := "ja"
	if err := e.svc.SetItemOriginalLanguage(ctx, idByTMDB, &ja); err != nil {
		t.Fatal(err)
	}
	if got := e.origLang(t, idByTMDB); got != "ja" || !strings.Contains(e.item(t, idByTMDB).Locked, `"original_language"`) {
		t.Fatalf("after the choice: %q, locked %s", got, e.item(t, idByTMDB).Locked)
	}
	if src, ok := e.sources(t, idByTMDB)["original_language/"]; ok {
		t.Fatalf("the administrator's value kept the source %q", src)
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.origLang(t, idByTMDB); got != "ja" {
		t.Fatalf("a match replaced the locked language with %q", got)
	}
	if src, ok := e.sources(t, idByTMDB)["original_language/"]; ok {
		t.Fatalf("the fill gave the locked language the source %q", src)
	}
	unknown := ""
	if err := e.svc.SetItemOriginalLanguage(ctx, idByTMDB, &unknown); err != nil || e.origLang(t, idByTMDB) != "" {
		t.Fatalf("unknown by hand: %v, %q", err, e.origLang(t, idByTMDB))
	}
	for _, bad := range []string{"jpn", "xx", "cn", "EN", "pt-BR"} {
		if err := e.svc.SetItemOriginalLanguage(ctx, idByTMDB, &bad); validationField(err) != "language/invalid_value" {
			t.Errorf("language %q: %v, want language/invalid_value", bad, err)
		}
	}
	e.addNode(t, node{id: idSeries, kind: "series", title: "Test Show", year: 2010})
	e.addNode(t, node{id: idSeason1, kind: "season", parent: idSeries, title: "Staffel 1", season: 1})
	if err := e.svc.SetItemOriginalLanguage(ctx, idSeason1, &ja); validationField(err) != "kind/invalid_value" {
		t.Errorf("a season: %v, want kind/invalid_value", err)
	}
	if err := e.svc.SetItemOriginalLanguage(ctx, idSeries, &ja); err != nil || e.origLang(t, idSeries) != "ja" {
		t.Errorf("a series: %v, %q", err, e.origLang(t, idSeries))
	}
	if err := e.svc.SetItemOriginalLanguage(ctx, "01JTESTMOVIE0000000000000Z", &ja); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("an unknown title: %v, want not found", err)
	}
	if err := e.svc.SetItemOriginalLanguage(ctx, idByTMDB, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.origLang(t, idByTMDB); got != "<null>" || strings.Contains(e.item(t, idByTMDB).Locked, "original_language") {
		t.Fatalf("after lifting: %q, locked %s", got, e.item(t, idByTMDB).Locked)
	}
	if got := e.activeJobs(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p7"}) {
		t.Fatalf("match jobs after lifting %v, want the manual match", got)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.origLang(t, idByTMDB); got != "en" {
		t.Fatalf("the match after lifting wrote %q, want en", got)
	}
}
