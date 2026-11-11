package radio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
)

// The offer tests (concept 19 part B, B13): the steps of a draft, the sync with the modes, the
// logo queue of an apply, «Nicht erreichbare entfernen», the preferences outside the offer and two
// drafts on one version. They run on the catalog extract of the catalog tests, from an empty offer.

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

// reachable reports whether profile 1 reaches the station named name.
func (e *catEnv) reachable(name string) bool {
	e.t.Helper()
	_, err := e.s.Station(e.ctx, catSession(1), e.stationID(name))
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		e.t.Fatal(err)
	}
	return err == nil
}

// offerCountry returns the entry of country code of the stored offer.
func (e *catEnv) offerCountry(code string) model.RadioOfferCountry {
	e.t.Helper()
	o, err := e.s.Offer(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, c := range o.Countries {
		if c.Code == code {
			return c
		}
	}
	return model.RadioOfferCountry{}
}

// step builders.
func choose(c string) model.RadioOfferStep {
	return model.RadioOfferStep{Op: model.RadioOfferChooseCountry, Country: c}
}
func mark(ids ...int64) model.RadioOfferStep {
	return model.RadioOfferStep{Op: model.RadioOfferMark, StationIDs: ids}
}
func demark(ids ...int64) model.RadioOfferStep {
	return model.RadioOfferStep{Op: model.RadioOfferDemark, StationIDs: ids}
}

// The rule of B4.1 and the steps on a country: «Land wählen» brings every station of it, «Alle
// demarkieren» none, a mark brings one back whatever its country, own stations are offered unless
// demarked, «Land entfernen» takes the exceptions of the country's stations with it. The offer's
// numbers count the catalog stations of a country (m) and those offered (n), unreachable included.
func TestOfferRuleAndCountrySteps(t *testing.T) {
	e := newCatEnv(t)
	e.emptyOffer()
	e.sync()
	if e.reachable("SRF 3") || e.offerCountry("CH").Mode != "" {
		t.Fatal("a station is offered by an empty offer")
	}
	f, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{})
	if err != nil || !f.OfferEmpty {
		t.Errorf("facets of the empty offer: offerEmpty %v, %v", f.OfferEmpty, err)
	}
	e.apply(choose("CH"))
	ch := e.offerCountry("CH")
	if ch.Mode != model.RadioOfferAll || ch.Continent != rules.ContinentEurope || ch.Total != ch.Offered || ch.Total < 20 || ch.Unreachable != 1 {
		t.Errorf("CH after choosing it = %+v", ch)
	}
	if de := e.offerCountry("DE"); de.Mode != "" || de.Offered != 0 || de.Total == 0 {
		t.Errorf("DE not chosen = %+v", de)
	}
	if !e.reachable("SRF 3") || e.reachable("Bayern 3") {
		t.Fatal("CH chosen: SRF 3 offered, Bayern 3 not")
	}
	fm4 := e.stationID("FM4")
	e.apply(model.RadioOfferStep{Op: model.RadioOfferDemarkAll, Country: "CH"}, mark(e.stationID("SRF 3"), fm4))
	if ch := e.offerCountry("CH"); ch.Mode != model.RadioOfferMarked || ch.Offered != 1 {
		t.Errorf("CH after «Alle demarkieren» and one mark = %+v", ch)
	}
	if !e.reachable("FM4") || e.reachable("Radio 24") {
		t.Error("a mark brings FM4 of a country not chosen; Radio 24 stays out")
	}
	own, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Mein Sender", StreamURL: "http://203.0.113.10/live", Country: "IT"})
	if err != nil {
		t.Fatal(err)
	}
	if !e.reachable("Mein Sender") {
		t.Error("an own station of a country not chosen is not offered")
	}
	a := e.apply(demark(own.ID))
	if e.reachable("Mein Sender") || a.Offer.Totals.Own != 0 || a.Offer.Totals.Demarked != 1 || a.Offer.Totals.Marked != 2 {
		t.Errorf("demarked own station: totals %+v", a.Offer.Totals)
	}
	// Removing CH takes the exception of SRF 3 with it, not that of FM4 (AT) or the own station.
	a = e.apply(model.RadioOfferStep{Op: model.RadioOfferRemoveCountry, Country: "CH"})
	if e.reachable("SRF 3") || !e.reachable("FM4") || a.Offer.Totals.Marked != 1 || a.Offer.Totals.Demarked != 1 || a.Offer.Totals.Countries != 0 {
		t.Errorf("after removing CH: totals %+v", a.Offer.Totals)
	}
	// A mark that equals the default is no exception.
	a = e.apply(choose("AT"), mark(fm4))
	if a.Offer.Totals.Marked != 0 {
		t.Errorf("a mark in a country with «all» stays an exception: %+v", a.Offer.Totals)
	}
	for _, bad := range []model.RadioOfferStep{{Op: "hide"}, {Op: model.RadioOfferChooseCountry, Country: "ch"},
		{Op: model.RadioOfferMarkMatching, Country: "CH", Q: strings.Repeat("x", 101)},
		{Op: model.RadioOfferMark, StationIDs: make([]int64, 5001)}} {
		if _, err := e.s.PreviewOffer(e.ctx, []model.RadioOfferStep{bad}, nil); fieldCodeOf(err, "steps") != "invalid_value" {
			t.Errorf("step %+v: %v, want invalid_value on steps", bad.Op, err)
		}
	}
	if _, err := e.s.PreviewOffer(e.ctx, make([]model.RadioOfferStep, 1001), nil); fieldCodeOf(err, "steps") != "invalid_value" {
		t.Errorf("1001 steps: %v", err)
	}
	if _, err := e.s.PreviewOffer(e.ctx, nil, &model.RadioOfferListQuery{Country: "CH", Limit: 101}); fieldCodeOf(err, "list") != "invalid_value" {
		t.Errorf("a list of 101: %v", err)
	}
}

// fieldCodeOf returns the code of field in a *model.ValidationError, "" otherwise.
func fieldCodeOf(err error, field string) string {
	var verr *model.ValidationError
	if !errors.As(err, &verr) {
		return ""
	}
	for _, f := range verr.Fields {
		if f.Field == field {
			return f.Code
		}
	}
	return ""
}

// PB-2: after «Alle demarkieren» and three marks exactly these three Swiss stations are offered,
// and a new Swiss station of the next sync stays out; with «Alle» it comes in, and so does the next
// new one. A demarked station that changes its country stays out, a marked one stays in (the
// exception hangs at the station, B4.1).
func TestOfferSyncFollowsTheModes(t *testing.T) {
	e := newCatEnv(t)
	e.emptyOffer()
	e.sync()
	three := []int64{e.stationID("SRF 3"), e.stationID("Radio 24"), e.stationID("Couleur 3")}
	e.apply(choose("CH"), model.RadioOfferStep{Op: model.RadioOfferDemarkAll, Country: "CH"}, mark(three...))
	o, err := e.s.Offer(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if o.Totals.Offered != 3 || o.Totals.Marked != 3 {
		t.Fatalf("offer after three marks = %+v", o.Totals)
	}
	list := catEntries(t)
	newEntry := func(uuid, name string) rbStation {
		return rbStation{StationUUID: uuid, Name: name, URL: "https://" + uuid + ".example/live", CountryCode: "CH",
			Codec: "MP3", Bitrate: 128, LastCheckOK: 1, LastChangeTime: "2026-11-08 08:00:00"}
	}
	list = append(list, newEntry("neu-1", "Neuradio Eins"))
	e.setEntries(list)
	e.clock.Advance(time.Hour)
	e.sync()
	if e.reachable("Neuradio Eins") {
		t.Error("a new Swiss station came in under «Nur markierte»")
	}
	e.apply(model.RadioOfferStep{Op: model.RadioOfferMarkAll, Country: "CH"})
	if !e.reachable("Neuradio Eins") {
		t.Error("the new station is not offered under «Alle»")
	}
	list = append(list, newEntry("neu-2", "Neuradio Zwei"))
	// Radio Pilatus (CH) is demarked and moves to Germany, chosen with «all»; FM4 (AT, not chosen)
	// is marked and moves to France, not chosen either.
	pilatus, fm4 := e.stationID("Radio Pilatus"), e.stationID("FM4")
	e.apply(demark(pilatus), mark(fm4), choose("DE"))
	for _, move := range []struct{ name, country string }{{"Radio Pilatus", "DE"}, {"FM4", "FR"}} {
		i := entryNamed(list, move.name)
		list[i].CountryCode, list[i].LastChangeTime = move.country, "2026-11-08 09:00:00"
	}
	e.setEntries(list)
	e.clock.Advance(time.Hour)
	e.sync()
	if !e.reachable("Neuradio Zwei") {
		t.Error("the next new Swiss station is not offered under «Alle»")
	}
	if st, _ := e.s.AdminStation(e.ctx, pilatus); st.Country != "DE" || st.Offered {
		t.Errorf("the demarked station after its move = %s offered %v, want DE and out", st.Country, st.Offered)
	}
	if st, _ := e.s.AdminStation(e.ctx, fm4); st.Country != "FR" || !st.Offered {
		t.Errorf("the marked station after its move = %s offered %v, want FR and in", st.Country, st.Offered)
	}
}

// PB-3: «Gezeigte demarkieren» and «Gezeigte markieren» with over 100 hits change exactly the hits
// and keep the mode; the preview shows the second page of the hits right.
func TestOfferMatchingSteps(t *testing.T) {
	e := newCatEnv(t)
	e.emptyOffer()
	e.setEntries(rulesCatalog(rulesGroup{prefix: "Eins FM", n: 150, tags: "pop"}, rulesGroup{prefix: "Andere", n: 20, tags: "rock"}))
	e.sync()
	e.apply(choose("DE"))
	q := model.RadioOfferStep{Op: model.RadioOfferDemarkMatching, Country: "DE", Q: "EINS fm"}
	p, err := e.s.PreviewOffer(e.ctx, []model.RadioOfferStep{q}, &model.RadioOfferListQuery{Country: "DE", Q: "eins", Offset: 100, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if p.Removed != 150 || p.Added != 0 || p.Totals.Offered != 20 || p.Totals.Demarked != 150 {
		t.Errorf("preview of the demark = removed %d added %d totals %+v", p.Removed, p.Added, p.Totals)
	}
	if p.List == nil || p.List.Total != 150 || p.List.Offset != 100 || len(p.List.Items) != 50 {
		t.Fatalf("page 2 = %+v", p.List)
	}
	first, err := e.s.PreviewOffer(e.ctx, []model.RadioOfferStep{q}, &model.RadioOfferListQuery{Country: "DE", Q: "eins", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, it := range slices.Concat(first.List.Items, p.List.Items) {
		if seen[it.ID] || it.Offered || !strings.HasPrefix(it.Name, "Eins FM ") || len(it.Tags) != 1 || it.Tags[0] != "pop" {
			t.Errorf("list item %+v (twice: %v)", it, seen[it.ID])
		}
		seen[it.ID] = true
	}
	if len(seen) != 150 || rules.Fold(first.List.Items[99].Name) > rules.Fold(p.List.Items[0].Name) {
		t.Errorf("pages hold %d stations; last of page 1 %q, first of page 2 %q", len(seen), first.List.Items[99].Name, p.List.Items[0].Name)
	}
	a := e.apply(q)
	if de := e.offerCountry("DE"); de.Mode != model.RadioOfferAll || de.Offered != 20 || a.Offer.Totals.Demarked != 150 {
		t.Errorf("after the demark: DE %+v, totals %+v", de, a.Offer.Totals)
	}
	a = e.apply(model.RadioOfferStep{Op: model.RadioOfferMarkMatching, Country: "DE", Q: "eins fm 1"})
	// «eins fm 1», «eins fm 10» … «eins fm 19», «eins fm 100» … «eins fm 150»: 1 + 10 + 51.
	if a.Offer.Totals.Demarked != 150-62 || e.offerCountry("DE").Offered != 20+62 || a.Offer.Totals.Marked != 0 {
		t.Errorf("after marking the hits of «eins fm 1»: totals %+v", a.Offer.Totals)
	}
	if e.offerCountry("DE").Mode != model.RadioOfferAll {
		t.Error("a matching step changed the mode")
	}
}

// PB-4: an apply queues the logos of the stations that came in without a logo, favourites first
// and then by popularity; the preview, the administration's list and its image route queue
// nothing, and a station already in the offer is not queued again.
func TestOfferApplyQueuesLogosOfNewStations(t *testing.T) {
	e, _, _ := catJobs(t)
	e.emptyOffer()
	e.sync()
	jobsOf := func() []string {
		t.Helper()
		rows, err := e.db.Read.QueryContext(e.ctx, `SELECT idempotency_key FROM jobs WHERE type = ? ORDER BY id`, JobLogoFetch)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				t.Fatal(err)
			}
			out = append(out, k)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	withLogo, fav := e.stationID("Radio Swiss Jazz"), e.stationID("Radio Chablais")
	e.exec(`UPDATE radio_stations SET logo_hash = 'abc', logo_state = 'catalog' WHERE id = ?`, withLogo)
	e.s.reloadStation(e.ctx, withLogo)
	if err := e.s.AddFavorite(e.ctx, catSession(1), fav); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("favourite outside the offer: %v", err)
	}
	e.exec(`INSERT INTO radio_favorites (profile_id, station_id, position, created_at) VALUES (1, ?, 0, 1)`, fav)
	if _, err := e.s.PreviewOffer(e.ctx, []model.RadioOfferStep{choose("CH")}, &model.RadioOfferListQuery{Country: "CH", Limit: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.AdminStations(e.ctx, model.AdminRadioQuery{Limit: 200}); err != nil {
		t.Fatal(err)
	}
	if src, err := e.s.Image(e.ctx, withLogo, "logo", "w96", false, true); err == nil {
		_ = src.File.Close()
	}
	if got := jobsOf(); len(got) != 0 {
		t.Fatalf("preview, admin list and admin image queued %v", got)
	}
	a := e.apply(choose("CH"))
	ch := e.offerCountry("CH")
	got := jobsOf()
	if a.QueuedLogos != ch.Offered-1 || len(got) != a.QueuedLogos {
		t.Fatalf("queued %d (%d jobs), want the %d CH stations but the one with a logo", a.QueuedLogos, len(got), ch.Offered-1)
	}
	if got[0] != fmt.Sprintf("radio.logo:%d", fav) || got[1] != fmt.Sprintf("radio.logo:%d", e.stationID("Testradio Eins")) {
		t.Errorf("order %v, want the favourite, then the most popular", got[:2])
	}
	if slices.Contains(got, fmt.Sprintf("radio.logo:%d", withLogo)) {
		t.Error("the station with a logo was queued")
	}
	if a := e.apply(mark(e.stationID("SRF 3")), choose("CH")); a.QueuedLogos != 0 || len(jobsOf()) != len(got) {
		t.Errorf("an apply without new stations queued %d", a.QueuedLogos)
	}
	if a := e.apply(choose("AT")); a.QueuedLogos != e.offerCountry("AT").Offered {
		t.Errorf("AT chosen: queued %d, want its %d stations", a.QueuedLogos, e.offerCountry("AT").Offered)
	}
}

// PB-8: «Nicht erreichbare entfernen» demarks exactly the unreachable stations of the offer but
// the favourites and the own stations, and answers their ids; marking them again («Rückgängig»)
// restores the offer as it was.
func TestOfferDemarkUnreachable(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	stille, kaputt := e.stationID("Radio Stille"), e.stationID("Kaputtradio")
	broken := []int64{e.stationID("Radio Munot"), e.stationID("Radio Lac"), e.stationID("FIP")}
	for _, id := range broken {
		e.exec(`UPDATE radio_stations SET broken_at = 1 WHERE id = ?`, id)
		e.s.reloadStation(e.ctx, id)
	}
	if err := e.s.AddFavorite(e.ctx, catSession(2), broken[2]); err != nil {
		t.Fatal(err)
	}
	own, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Mein Sender", StreamURL: "http://203.0.113.10/live"})
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE radio_stations SET reachable = 0 WHERE id = ?`, own.ID)
	e.s.reloadStation(e.ctx, own.ID)
	// A broken station outside the offer stays as it is.
	outside := e.stationID("Radio Fribourg")
	e.exec(`UPDATE radio_stations SET reachable = 0 WHERE id = ?`, outside)
	e.s.reloadStation(e.ctx, outside)
	before := e.apply(demark(outside))
	if before.Offer.Totals.RemovableUnreachable != 4 {
		t.Fatalf("removable = %d, want Radio Stille, Kaputtradio, Radio Munot and Radio Lac", before.Offer.Totals.RemovableUnreachable)
	}
	a := e.apply(model.RadioOfferStep{Op: model.RadioOfferDemarkUnreachable})
	got := slices.Sorted(slices.Values(a.DemarkedStationIDs))
	want := slices.Sorted(slices.Values([]int64{stille, kaputt, broken[0], broken[1]}))
	if !slices.Equal(got, want) || a.Offer.Totals.RemovableUnreachable != 0 {
		t.Fatalf("demarked %v, want %v; removable after %d", got, want, a.Offer.Totals.RemovableUnreachable)
	}
	if e.reachable("Radio Stille") || !e.reachable("FIP") || !e.reachable("Mein Sender") {
		t.Error("after the demark: Radio Stille out, the favourite FIP and the own station in")
	}
	undo := e.apply(mark(a.DemarkedStationIDs...))
	if undo.Offer.Totals != before.Offer.Totals || !e.reachable("Radio Stille") {
		t.Errorf("after the undo: totals %+v, want %+v", undo.Offer.Totals, before.Offer.Totals)
	}
}

// PB-9: a default country outside the offer gives an empty effectiveCountry and keeps
// defaultCountry; a stored filter loses the countries outside the offer when it is read, while the
// stored row keeps them. Once the country is offered, it is the effective one again.
func TestPreferencesOutsideTheOffer(t *testing.T) {
	e := newCatEnv(t)
	e.emptyOffer()
	e.sync()
	e.apply(choose("DE"))
	p := catSession(1)
	got, err := e.s.SetPreferences(e.ctx, p, model.RadioPreferences{DefaultCountry: "CH", EffectiveCountry: "CH",
		LastFilter: &model.RadioFilter{Countries: []string{"CH", "DE"}, Genres: []string{"pop"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultCountry != "CH" || got.EffectiveCountry != "" || !slices.Equal(got.LastFilter.Countries, []string{"DE"}) ||
		!slices.Equal(got.LastFilter.Genres, []string{"pop"}) {
		t.Errorf("preferences with CH outside the offer = %+v %+v", got, got.LastFilter)
	}
	var stored string
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT last_filter FROM radio_profile_state WHERE profile_id = 1`).Scan(&stored); err != nil ||
		!strings.Contains(stored, `"CH"`) {
		t.Errorf("stored filter %s (%v), want CH kept", stored, err)
	}
	// Without a stored country the default of the time zone (CH) is not offered either.
	if got, _ := e.s.Preferences(e.ctx, catSession(2)); got.DefaultCountry != "CH" || got.EffectiveCountry != "" {
		t.Errorf("default preferences = %+v", got)
	}
	e.apply(choose("CH"))
	if got, _ := e.s.Preferences(e.ctx, p); got.EffectiveCountry != "CH" || !slices.Equal(got.LastFilter.Countries, []string{"CH", "DE"}) {
		t.Errorf("preferences with CH offered = %+v %+v", got, got.LastFilter)
	}
	// A country whose only stations are unreachable while hide_broken is on lists none.
	e.apply(model.RadioOfferStep{Op: model.RadioOfferRemoveCountry, Country: "CH"}, mark(e.stationID("Radio Stille")))
	if got, _ := e.s.Preferences(e.ctx, p); got.EffectiveCountry != "" {
		t.Errorf("effective country with only a broken CH station = %q", got.EffectiveCountry)
	}
	f, err := e.s.Facets(e.ctx, p, model.RadioQuery{})
	if err != nil || f.OfferEmpty {
		t.Errorf("facets of a non-empty offer: offerEmpty %v, %v", f.OfferEmpty, err)
	}
}

// PB-11: two drafts on one version: the second apply is 409; computed again on the new version,
// the same steps keep both changes.
func TestOfferConflictKeepsBothDrafts(t *testing.T) {
	e := newCatEnv(t)
	e.emptyOffer()
	e.sync()
	o, err := e.s.offer(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	v := o.version
	bayern := e.stationID("Bayern 3")
	first := []model.RadioOfferStep{choose("CH")}
	second := []model.RadioOfferStep{choose("DE"), demark(bayern)}
	if _, err := e.s.ApplyOffer(e.ctx, v, first); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.ApplyOffer(e.ctx, v, second); !errors.Is(err, model.ErrRadioOfferConflict) {
		t.Fatalf("second apply on the old version: %v, want ErrRadioOfferConflict", err)
	}
	p, err := e.s.PreviewOffer(e.ctx, second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != v+1 || p.Totals.Countries != 2 || p.Totals.Demarked != 1 {
		t.Errorf("preview on the new version = version %d totals %+v", p.Version, p.Totals)
	}
	a, err := e.s.ApplyOffer(e.ctx, p.Version, second)
	if err != nil {
		t.Fatal(err)
	}
	if a.Offer.Version != v+2 || !e.reachable("SRF 3") || !e.reachable("SWR3") || e.reachable("Bayern 3") {
		t.Errorf("after both drafts: version %d", a.Offer.Version)
	}
	// The stored offer survives a restart of the service.
	e.s.cat.offer.Store(nil)
	if o, err := e.s.offer(e.ctx); err != nil || o.version != v+2 || o.countries["DE"] != model.RadioOfferAll || o.exceptions[bayern] {
		t.Errorf("reloaded offer = %+v %v", o, err)
	}
}

// BenchmarkOfferPreview computes a preview (steps, totals, countries, difference and one page of
// the list) over a synthetic index of 55,000 stations in 200 countries with 30 chosen countries
// and 2,000 exceptions: the computation B0 compares against its target of 100 ms (B8), without
// the reads of the favourites and the tags of the page.
// go test ./internal/radio -run '^$' -bench BenchmarkOfferPreview -benchtime 20x
func BenchmarkOfferPreview(b *testing.B) {
	var codes []string
	for a := 'A'; a <= 'Z'; a++ {
		for z := 'A'; z <= 'Z'; z++ {
			if c := string([]rune{a, z}); rules.Continent(c) != "" {
				codes = append(codes, c)
			}
		}
	}
	codes = codes[:200]
	idx := &stationIndex{byID: map[int64]*indexEntry{}}
	for i := range 55_000 {
		e := &indexEntry{st: model.RadioStation{ID: int64(i + 1), Name: fmt.Sprintf("Station %d", i), Country: codes[i%len(codes)]},
			sortKey: fmt.Sprintf("station %d", i), search: fmt.Sprintf("station %d pop rock fm", i), broken: i%7 == 0,
			own: i%5000 == 0, popularity: int64(i % 997)}
		if i%3 == 0 {
			e.st.LogoHash = "h"
		}
		idx.entries = append(idx.entries, e)
		idx.byID[e.st.ID] = e
	}
	cur := &offerState{countries: map[string]string{}, exceptions: map[int64]bool{}}
	for _, c := range codes[:30] {
		cur.countries[c] = model.RadioOfferAll
	}
	for i := range 2000 {
		cur.exceptions[int64(i*20+1)] = i%2 == 0
	}
	favs := map[int64]bool{7: true, 14: true}
	steps := []model.RadioOfferStep{choose(codes[40]), {Op: model.RadioOfferDemarkMatching, Country: codes[3], Q: "station 1"},
		mark(1, 2, 3, 4, 5), {Op: model.RadioOfferDemarkUnreachable}}
	list := model.RadioOfferListQuery{Country: codes[40], Q: "fm", Limit: 100}
	for b.Loop() {
		calc := newOfferCalc(idx, favs)
		draft := cur.clone()
		calc.apply(draft, steps)
		_, added, _ := calc.summary(draft, cur)
		if page := calc.page(draft, list); added == 0 || len(page.Items) != 100 {
			b.Fatalf("added %d, page %d", added, len(page.Items))
		}
	}
}

// A country step names a country of the country list: an ISO code with a continent (OB-7). A code
// without one (ZZ) or in lower case is invalid_value on steps, in the steps and in the list.
func TestOfferStepsNeedACountryWithAContinent(t *testing.T) {
	for _, c := range []struct {
		country string
		ok      bool
	}{{"CH", true}, {"XK", true}, {"ZZ", false}, {"ch", false}, {"", false}} {
		for _, op := range []string{model.RadioOfferChooseCountry, model.RadioOfferRemoveCountry, model.RadioOfferMarkAll,
			model.RadioOfferDemarkAll, model.RadioOfferMarkMatching, model.RadioOfferDemarkMatching} {
			err := validateOfferSteps([]model.RadioOfferStep{{Op: op, Country: c.country}})
			if (err == nil) != c.ok {
				t.Errorf("%s %q: %v, want ok %v", op, c.country, err, c.ok)
			}
		}
	}
	e := newCatEnv(t)
	e.sync()
	if _, err := e.s.PreviewOffer(e.ctx, nil, &model.RadioOfferListQuery{Country: "ZZ", Limit: 100}); err == nil {
		t.Error("preview list of ZZ accepted")
	}
}

// Marco's decision of 11 November 2026: a station has a fetched logo only while it is in the
// offer. When Germany leaves the offer, the logo files and the logo state of its stations go, an
// own upload stays; a Swiss station in the offer keeps its logo.
func TestLogosLeaveWithTheOffer(t *testing.T) {
	cache := t.TempDir()
	e := newCatEnv(t, WithCacheDir(cache))
	e.sync()
	srf, live, bayern := e.stationID("SRF 3"), e.stationID("1LIVE"), e.stationID("Bayern 3")
	for _, c := range []struct {
		id    int64
		state string
	}{{srf, "catalog"}, {live, "website"}, {bayern, "own"}} {
		e.exec(`UPDATE radio_stations SET logo_state = ?, logo_hash = 'h' WHERE id = ?`, c.state, c.id)
		e.s.reloadStation(e.ctx, c.id)
		dir := filepath.Join(cache, logoDir(c.id))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "logo-w96.webp"), []byte("webp"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.apply(model.RadioOfferStep{Op: model.RadioOfferRemoveCountry, Country: "DE"})
	has := func(id int64) bool {
		_, err := os.Stat(filepath.Join(cache, logoDir(id), "logo-w96.webp"))
		return err == nil
	}
	if has(live) || e.count(`SELECT count(*) FROM radio_stations WHERE id = ? AND logo_state = 'none' AND logo_hash IS NULL`, live) != 1 {
		t.Error("the fetched logo of a station outside the offer stayed")
	}
	if !has(bayern) || e.count(`SELECT count(*) FROM radio_stations WHERE id = ? AND logo_state = 'own'`, bayern) != 1 {
		t.Error("an own upload of a station outside the offer was removed")
	}
	if !has(srf) {
		t.Error("the logo of a station in the offer was removed")
	}
}
