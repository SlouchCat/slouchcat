package radio

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// rulesCatalog is a catalog with known tag counts: per group, n stations named prefix 1..n with
// tags; reachable is false for the group "Broken".
func rulesCatalog(groups ...rulesGroup) []rbStation {
	var out []rbStation
	for _, g := range groups {
		for i := 1; i <= g.n; i++ {
			ok := int64(1)
			if g.broken {
				ok = 0
			}
			out = append(out, rbStation{StationUUID: fmt.Sprintf("%s-%d", g.prefix, i), Name: fmt.Sprintf("%s %d", g.prefix, i),
				URL: fmt.Sprintf("https://%s%d.example/live", strings.ToLower(g.prefix), i), Tags: g.tags, CountryCode: "DE",
				LanguageCodes: g.lang, Codec: "MP3", Bitrate: 128, LastCheckOK: ok, LastChangeTime: "2026-11-01 10:00:00"})
		}
	}
	return out
}

type rulesGroup struct {
	prefix string
	n      int
	tags   string
	lang   string
	broken bool
}

// facetValues returns the stored values of a facet of the station named name.
func (e *catEnv) facetValues(name, facet string) []string {
	e.t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT value FROM radio_station_facets WHERE station_id = ? AND facet = ? ORDER BY value`,
		e.stationID(name), facet)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func coverageOf(r model.RadioTagRules, facet string) model.RadioCoverage {
	for _, c := range r.Coverage {
		if c.Facet == facet {
			return c
		}
	}
	return model.RadioCoverage{}
}

// The worklist holds the tags of at least 20 stations that no rule maps, merged by their folded
// form (m-11), most stations first; openTags counts those above 50; an ignore row and a rule take
// a tag off; a rule applies at once to the stored facets and the index, and the coverage counts
// the reachable stations only, without the default kind (D-3, D-4).
func TestTagRulesWorklistAndCoverage(t *testing.T) {
	e := newCatEnv(t)
	e.setEntries(rulesCatalog(
		rulesGroup{prefix: "Weird", n: 60, tags: "zzweird", lang: "de"},
		rulesGroup{prefix: "Fm", n: 30, tags: "fm,pop"},
		rulesGroup{prefix: "Merge", n: 12, tags: "zzmérge"},
		rulesGroup{prefix: "Merga", n: 12, tags: "zzmerge"},
		rulesGroup{prefix: "Rare", n: 10, tags: "zzrare"},
		rulesGroup{prefix: "News", n: 4, tags: "zzweird"},
		rulesGroup{prefix: "Broken", n: 5, tags: "pop,talk", broken: true},
	))
	e.sync()
	got, err := e.s.TagRules(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.RadioTagCount{{Tag: "zzweird", Stations: 64}, {Tag: "fm", Stations: 30}, {Tag: "zzmerge", Stations: 24}}
	if !slices.Equal(got.Worklist, want) || got.OpenTags != 1 || got.WorklistMinStations != 20 {
		t.Errorf("worklist %+v open %d min %d, want %+v open 1 min 20", got.Worklist, got.OpenTags, got.WorklistMinStations, want)
	}
	// 128 reachable stations: 30 with pop, 4 with kind news by their name, 60 with a language.
	for facet, w := range map[string]int{"genre": 30, "kind": 4, "mood": 0, "region": 0, "language": 60} {
		if c := coverageOf(got, facet); c.Assigned != w || c.Total != 128 {
			t.Errorf("coverage %s = %+v, want %d of 128", facet, c, w)
		}
	}
	if i := slices.IndexFunc(got.Rules, func(v model.RadioTagRuleView) bool { return v.Tag == "pop" && v.Facet == "genre" }); i < 0 ||
		got.Rules[i].Source != model.RadioRuleDefault || got.Rules[i].Value != "pop" || got.Rules[i].Stations != 35 {
		t.Errorf("default rule pop missing or wrong: %d", i)
	}

	got, err = e.s.SetTagRules(e.ctx, []model.RadioTagRule{{Tag: "FM", Facet: "ignore"}, {Tag: "ZZWeird", Facet: "genre", Value: "rock"},
		{Tag: "zzweird", Facet: "mood", Value: "party"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []model.RadioTagCount{{Tag: "zzmerge", Stations: 24}}; !slices.Equal(got.Worklist, want) || got.OpenTags != 0 {
		t.Errorf("worklist after the rules %+v open %d, want %+v", got.Worklist, got.OpenTags, want)
	}
	if c := coverageOf(got, "genre"); c.Assigned != 94 {
		t.Errorf("genre coverage after the rules = %+v, want 94", c)
	}
	if v := e.facetValues("Weird 7", "genre"); !slices.Equal(v, []string{"rock"}) {
		t.Errorf("stored genres of Weird 7 = %v, want [rock]", v)
	}
	if st := e.station("Weird 7"); !slices.Equal(st.Genres, []string{"rock"}) || !slices.Equal(st.Moods, []string{"party"}) {
		t.Errorf("index of Weird 7: genres %v moods %v", st.Genres, st.Moods)
	}
	if e.s.cat.rulesDirty.Load() {
		t.Error("rules still pending after an apply without a sync")
	}
	admin := 0
	for _, v := range got.Rules {
		if v.Source == model.RadioRuleAdmin {
			admin++
			if v.Tag == "zzweird" && v.Stations != 64 {
				t.Errorf("rule zzweird counts %d stations, want 64", v.Stations)
			}
		}
	}
	if admin != 3 {
		t.Errorf("%d admin rules, want 3", admin)
	}
	// A new service on the same database applies the stored rules (sync after a restart).
	again := New(e.db, catDiscard)
	if r, err := again.TagRules(e.ctx); err != nil || len(r.Worklist) != 1 {
		t.Errorf("rules after a restart: worklist %+v, %v", r.Worklist, err)
	}
}

// A rule without a value takes the default mapping of its tag in that facet away and leaves the
// other facets and a locked facet alone; putting the rules back restores the default (D-2).
func TestTagRulesNullValueAndLocks(t *testing.T) {
	e := newCatEnv(t)
	e.setEntries(rulesCatalog(rulesGroup{prefix: "Hits", n: 3, tags: "80s,party"}))
	e.sync()
	if v := e.facetValues("Hits 1", "genre"); !slices.Equal(v, []string{"oldies"}) {
		t.Fatalf("genres before = %v", v)
	}
	jazz := []string{"jazz"}
	if _, err := e.s.UpdateStation(e.ctx, e.stationID("Hits 2"), model.AdminRadioStationPatch{Genres: &jazz}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.SetTagRules(e.ctx, []model.RadioTagRule{{Tag: "80s", Facet: "genre"}}); err != nil {
		t.Fatal(err)
	}
	if v := e.facetValues("Hits 1", "genre"); len(v) != 0 {
		t.Errorf("genres with 80s → none = %v, want none", v)
	}
	if v := e.facetValues("Hits 1", "mood"); !slices.Equal(v, []string{"party"}) {
		t.Errorf("moods = %v, want [party] (another facet keeps its default)", v)
	}
	if v := e.facetValues("Hits 2", "genre"); !slices.Equal(v, jazz) {
		t.Errorf("locked genres = %v, want [jazz]", v)
	}
	if _, err := e.s.SetTagRules(e.ctx, nil); err != nil {
		t.Fatal(err)
	}
	if v := e.facetValues("Hits 1", "genre"); !slices.Equal(v, []string{"oldies"}) {
		t.Errorf("genres after removing the rule = %v, want [oldies]", v)
	}
	if st := e.station("Hits 1"); !slices.Equal(st.Genres, []string{"oldies"}) {
		t.Errorf("index genres = %v, want [oldies]", st.Genres)
	}
}

// Invalid rules are refused on the field rules and store nothing.
func TestTagRulesValidation(t *testing.T) {
	e := newCatEnv(t)
	for _, list := range [][]model.RadioTagRule{
		{{Tag: "80s", Facet: "genre", Value: "no-such-genre"}},
		{{Tag: "fm", Facet: "ignore", Value: "pop"}},
		{{Tag: "80s", Facet: "genre", Value: "pop"}, {Tag: " 80S ", Facet: "genre", Value: "rock"}},
		{{Tag: "  ", Facet: "genre", Value: "pop"}},
		{{Tag: strings.Repeat("x", 101), Facet: "genre", Value: "pop"}},
		{{Tag: "80s", Facet: "region", Value: "CH-ZH"}},
		{{Tag: "news", Facet: "kind", Value: "pop"}},
	} {
		_, err := e.s.SetTagRules(e.ctx, list)
		var verr *model.ValidationError
		if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != "rules" {
			t.Errorf("rules %+v: %v, want a validation error on rules", list, err)
		}
	}
	if n := e.count(`SELECT count(*) FROM radio_tag_rules`); n != 0 {
		t.Errorf("%d rules stored", n)
	}
}

// An apply that fails midway keeps the rules pending, and the next round converges (critic M-3).
func TestTagRulesApplyFailureConverges(t *testing.T) {
	e := newCatEnv(t)
	raw := synthCatalog(3125) // 2500 stations with "80s hits": three batches
	e.body.Store(&raw)
	e.sync()
	var last int64
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT max(id) FROM radio_stations`).Scan(&last); err != nil {
		t.Fatal(err)
	}
	e.exec(fmt.Sprintf(`CREATE TRIGGER fail_apply BEFORE DELETE ON radio_station_facets WHEN OLD.station_id = %d
		BEGIN SELECT RAISE(ABORT, 'apply fails'); END`, last))
	if _, err := e.s.SetTagRules(e.ctx, []model.RadioTagRule{{Tag: "80s", Facet: "genre"}}); err != nil {
		t.Fatal(err)
	}
	oldies := `SELECT count(*) FROM radio_station_facets WHERE facet = 'genre' AND value = 'oldies'`
	if n := e.count(oldies); n == 0 || n == 2500 || !e.s.cat.rulesDirty.Load() {
		t.Fatalf("after the failed apply: %d stations keep oldies, pending %v; want some, not all, and pending", n, e.s.cat.rulesDirty.Load())
	}
	e.exec(`DROP TRIGGER fail_apply`)
	e.s.retryTagRules(e.ctx)
	if n := e.count(oldies); n != 0 || e.s.cat.rulesDirty.Load() {
		t.Errorf("after the retry: %d stations keep oldies, pending %v", n, e.s.cat.rulesDirty.Load())
	}
	if f, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{}); err != nil || facetCount(f.Genres, "oldies") != 0 {
		t.Errorf("index still counts oldies: %+v %v", f.Genres, err)
	}
}

// Rules stored by a process that stopped before it applied them reach the stations after a
// restart: Start marks the stored rules pending and the next sweep applies them.
func TestTagRulesAppliedAfterRestart(t *testing.T) {
	e := newCatEnv(t)
	e.setEntries(rulesCatalog(rulesGroup{prefix: "Hits", n: 3, tags: "80s"}))
	e.sync()
	// The rule is stored, the facets are not: the state a stop between both leaves behind.
	e.exec(`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('80s', 'genre', 'rock', 1)`)
	if v := e.facetValues("Hits 1", "genre"); !slices.Equal(v, []string{"oldies"}) {
		t.Fatalf("before the restart: genres %v, want [oldies]", v)
	}
	restarted := New(e.db, e.s.log, WithClock(e.s.now))
	if err := restarted.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	if !restarted.cat.rulesDirty.Load() {
		t.Fatal("Start left the stored rules unmarked")
	}
	restarted.retryTagRules(e.ctx) // what the first sweep does
	if v := e.facetValues("Hits 1", "genre"); !slices.Equal(v, []string{"rock"}) || restarted.cat.rulesDirty.Load() {
		t.Errorf("after the restart: genres %v, pending %v; want [rock], none", v, restarted.cat.rulesDirty.Load())
	}
}

// Rules stored while a sync holds syncMu are applied by that sync before it ends.
func TestTagRulesDuringSyncAppliedBySync(t *testing.T) {
	e := newCatEnv(t)
	e.setEntries(rulesCatalog(rulesGroup{prefix: "Hits", n: 3, tags: "80s"}))
	e.sync()
	e.s.cat.syncMu.Lock()
	if _, err := e.s.SetTagRules(e.ctx, []model.RadioTagRule{{Tag: "80s", Facet: "genre", Value: "rock"}}); err != nil {
		t.Fatal(err)
	}
	if v := e.facetValues("Hits 1", "genre"); !slices.Equal(v, []string{"oldies"}) || !e.s.cat.rulesDirty.Load() {
		t.Fatalf("during the sync: genres %v, pending %v; want [oldies], pending", v, e.s.cat.rulesDirty.Load())
	}
	e.s.cat.syncMu.Unlock()
	e.sync() // the entries are unchanged: only the pending rules change the station
	if v := e.facetValues("Hits 1", "genre"); !slices.Equal(v, []string{"rock"}) || e.s.cat.rulesDirty.Load() {
		t.Errorf("after the sync: genres %v, pending %v", v, e.s.cat.rulesDirty.Load())
	}
	if st := e.station("Hits 1"); !slices.Equal(st.Genres, []string{"rock"}) {
		t.Errorf("index genres = %v", st.Genres)
	}
}

// The answer is cached per generation: a station write and a change of the settings count again.
func TestTagRulesCacheBumps(t *testing.T) {
	e := newCatEnv(t)
	e.setEntries(rulesCatalog(rulesGroup{prefix: "Pop", n: 3, tags: "pop"}, rulesGroup{prefix: "Ch", n: 2, tags: "pop"}))
	e.sync()
	total := func() int {
		t.Helper()
		r, err := e.s.TagRules(e.ctx)
		if err != nil {
			t.Fatal(err)
		}
		return coverageOf(r, "genre").Total
	}
	if n := total(); n != 5 {
		t.Fatalf("total = %d, want 5", n)
	}
	hidden := true
	if _, err := e.s.UpdateStation(e.ctx, e.stationID("Pop 1"), model.AdminRadioStationPatch{Hidden: &hidden}); err != nil {
		t.Fatal(err)
	}
	if n := total(); n != 4 {
		t.Errorf("total after hiding one = %d, want 4", n)
	}
	e.exec(`UPDATE radio_stations SET country = 'CH' WHERE name LIKE 'Ch %'`)
	e.s.reloadStations(e.ctx, []int64{e.stationID("Ch 1"), e.stationID("Ch 2")})
	countries := []string{"DE"}
	if _, err := e.s.UpdateConfig(e.ctx, model.RadioConfigUpdate{Countries: &countries}); err != nil {
		t.Fatal(err)
	}
	if n := total(); n != 2 {
		t.Errorf("total with country DE only = %d, want 2", n)
	}
}

// BenchmarkTagRulesLarge measures, over the synthetic catalog of 40,000 stations with the tag
// "80s hits", one SetTagRules whose apply rewrites the genres of every station (the rule toggles)
// and one uncached TagRules. Run without -race:
// go test ./internal/radio -run '^$' -bench BenchmarkTagRulesLarge -benchtime 3x
func BenchmarkTagRulesLarge(b *testing.B) {
	e := newCatEnv(b)
	raw := synthCatalog(50000)
	e.body.Store(&raw)
	e.sync()
	toggles := [][]model.RadioTagRule{{{Tag: "80s", Facet: "genre"}}, nil}
	var apply, read time.Duration
	i := 0
	for b.Loop() {
		start := time.Now()
		if _, err := e.s.SetTagRules(e.ctx, toggles[i%2]); err != nil {
			b.Fatal(err)
		}
		apply += time.Since(start)
		e.s.cat.stationsChanged()
		start = time.Now()
		if _, err := e.s.TagRules(e.ctx); err != nil {
			b.Fatal(err)
		}
		read += time.Since(start)
		i++
	}
	b.ReportMetric(float64(apply.Milliseconds())/float64(i), "settagrules-ms/op")
	b.ReportMetric(float64(read.Milliseconds())/float64(i), "tagrules-ms/op")
}
