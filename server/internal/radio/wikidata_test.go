package radio

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// The Wikidata tests (concept 19 part B stage B2, PB-14): the match of the offer against the
// fixture of the SPARQL answer (testdata/radio/wikidata.csv), the owner as provider, the sources of
// the administration, the setting «Wikidata verwenden», the stations that leave the offer, the
// queue after an apply and the network rules.

// wdFake is the SPARQL endpoint of the tests: the station query answers wikidata.csv, a label
// query the labels of wikidata-labels.csv for the items it names, in its language chain.
type wdFake struct {
	srv      *httptest.Server
	stations atomic.Int32
	labels   atomic.Int32
	mu       sync.Mutex
	agents   []string
	langs    []string
	// body, when set, answers the station query instead of the fixture.
	body atomic.Pointer[[]byte]
}

var (
	wdValuesRe = regexp.MustCompile(`wd:(Q[0-9]+)`)
	wdLangsRe  = regexp.MustCompile(`wikibase:language "([^"]+)"`)
)

func newWDFake(t *testing.T) *wdFake {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "testdata", "radio")
	stations, err := os.ReadFile(filepath.Join(dir, "wikidata.csv"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "wikidata-labels.csv"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]map[string]string{} // item: language: label
	for _, r := range rows[1:] {
		q := wdQID(r[0])
		if labels[q] == nil {
			labels[q] = map[string]string{}
		}
		labels[q][r[1]] = r[2]
	}
	f := &wdFake{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sparql" || r.Header.Get("Accept") != "text/csv" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "form", http.StatusBadRequest)
			return
		}
		query := r.PostForm.Get("query")
		f.mu.Lock()
		f.agents = append(f.agents, r.Header.Get("User-Agent"))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/csv")
		if !strings.Contains(query, "rdfs:label") {
			f.stations.Add(1)
			if b := f.body.Load(); b != nil {
				_, _ = w.Write(*b)
				return
			}
			_, _ = w.Write(stations)
			return
		}
		f.labels.Add(1)
		chain := strings.Split(wdLangsRe.FindStringSubmatch(query)[1], ",")
		f.mu.Lock()
		f.langs = append(f.langs, strings.Join(chain, ","))
		f.mu.Unlock()
		out := "o,label\r\n"
		for _, m := range wdValuesRe.FindAllStringSubmatch(query, -1) {
			label := m[1] // the label service answers the id of an item without a label
			for _, l := range chain {
				if v := labels[m[1]][l]; v != "" {
					label = v
					break
				}
			}
			out += "http://www.wikidata.org/entity/" + m[1] + "," + label + "\r\n"
		}
		_, _ = w.Write([]byte(out))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// wdEnv is a catalog environment with the fake endpoint, the loopback allowed and German as the
// server's language; the offer is empty.
func wdEnv(t *testing.T) (*catEnv, *wdFake) {
	t.Helper()
	e := newCatEnv(t, WithLanguage(func(context.Context) string { return "de-CH" }))
	withDialControl(e.s, loopbackOnly)
	f := newWDFake(t)
	e.s.wd.endpoint = f.srv.URL + "/sparql"
	e.s.wd.limiter = ratelimit.New("radio-wikidata-test", 1000, 1000)
	e.s.wd.minItems = 5 // the fixture lists eleven stations
	e.emptyOffer()
	e.sync()
	return e, f
}

// wdRun runs the job.
func (e *catEnv) wdRun() {
	e.t.Helper()
	if err := e.s.handleWikidataSync(e.ctx, nil); err != nil {
		e.t.Fatal(err)
	}
}

// wdRow returns the stored match of the station named name; nil without one.
func (e *catEnv) wdRow(name string) *model.RadioWikidata {
	e.t.Helper()
	a, err := e.s.AdminStation(e.ctx, e.stationID(name))
	if err != nil {
		e.t.Fatal(err)
	}
	return a.Wikidata
}

// PB-14: a station of the offer with a unique match gets its item, owner and founding date, and
// the owner becomes its provider in the app and the administration; an ambiguous address stays
// empty; stations outside the offer are not stored; the owners' labels come in the server's
// language, else English.
func TestWikidataMatchesTheOffer(t *testing.T) {
	e, f := wdEnv(t)
	e.apply(choose("CH"))
	// The streams of the test catalog run on 127.0.0.1, which has no registrable domain.
	e.exec(`UPDATE radio_stations SET provider = 'srg-ssr.ch' WHERE name IN ('SRF 3', 'Couleur 3')`)
	e.s.reloadStations(e.ctx, []int64{e.stationID("SRF 3"), e.stationID("Couleur 3")})
	streamProvider := e.station("SRF 3").Provider
	e.wdRun()
	if f.stations.Load() != 1 || f.labels.Load() != 1 {
		t.Fatalf("requests: %d station queries, %d label queries, want one each", f.stations.Load(), f.labels.Load())
	}
	if f.langs[0] != "de,en" {
		t.Errorf("label languages %q, want de,en (de-CH of the server, then English)", f.langs[0])
	}
	for _, a := range f.agents {
		if a != "Slouchcat/test (+https://github.com/SlouchCat/slouchcat)" {
			t.Errorf("User-Agent %q", a)
		}
	}
	for name, want := range map[string]*model.RadioWikidata{
		"SRF 3":            {QID: "Q1157782", OwnerQID: "Q1441462", Owner: "Schweizer Radio und Fernsehen", Founded: "1983-11-01"},
		"Radio SRF 1":      {QID: "Q577689", OwnerQID: "Q1441462", Owner: "Schweizer Radio und Fernsehen", Founded: "1946"},
		"Energy Zürich":    {QID: "Q900010", OwnerQID: "Q900011", Owner: "Energy Schweiz", Founded: "2003"},
		"Radio Pilatus":    {QID: "Q900020", Founded: "1983-08"},
		"Couleur 3":        {QID: "Q900030", OwnerQID: "Q900031", Founded: "1980"},
		"Radio Swiss Jazz": {QID: "Q900040", OwnerQID: "Q900041"},
		"Radio 24":         nil, // two items with its address
		"Radio Basilisk":   nil, // no item
		"Testradio Eins":   nil, // no homepage
	} {
		got := e.wdRow(name)
		if got != nil {
			got.FetchedAt = time.Time{}
		}
		if (got == nil) != (want == nil) || got != nil && *got != *want {
			t.Errorf("%s: wikidata %+v, want %+v", name, got, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM radio_station_wikidata w JOIN radio_stations s ON s.id = w.station_id WHERE s.country <> 'CH'`); n != 0 {
		t.Errorf("%d stations outside the offer stored", n)
	}
	if got := e.station("SRF 3").Provider; got != "Schweizer Radio und Fernsehen" {
		t.Errorf("provider of SRF 3 in the app %q, want the owner", got)
	}
	a, err := e.s.AdminStation(e.ctx, e.stationID("SRF 3"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Provider != "Schweizer Radio und Fernsehen" || a.Sources.Provider != model.RadioSourceWikidata ||
		a.Sources.Name != model.RadioSourceCatalog || a.Sources.Country != model.RadioSourceCatalog || a.Sources.Logo != "" {
		t.Errorf("admin SRF 3: provider %q, sources %+v", a.Provider, a.Sources)
	}
	if a, err := e.s.AdminStation(e.ctx, e.stationID("Couleur 3")); err != nil || a.Provider != "srg-ssr.ch" ||
		a.Sources.Provider != model.RadioSourceStream {
		t.Errorf("an owner without label keeps the stream's provider: %q %+v %v", a.Provider, a.Sources, err)
	}
	page, err := e.s.AdminStations(e.ctx, model.AdminRadioQuery{Offered: true, Provider: "Schweizer Radio und Fernsehen", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 { // SRF 3, Radio SRF 1
		t.Errorf("filter by the owner: %d stations, want 2", page.Total)
	}
	if streamProvider != "srg-ssr.ch" {
		t.Fatalf("provider before the run %q", streamProvider)
	}

	// Austria comes in: the next run adds its station and keeps the Swiss ones.
	e.apply(choose("AT"))
	e.wdRun()
	if got := e.wdRow("Hitradio Ö3"); got == nil || got.Owner != "Österreichischer Rundfunk" {
		t.Errorf("Ö3 after AT chosen: %+v", got)
	}
	if e.wdRow("SRF 3") == nil {
		t.Error("SRF 3 lost its match")
	}
	if n := e.count(`SELECT count(*) FROM radio_config WHERE wikidata_synced_at = ?`, e.clock.Now().UnixMilli()); n != 1 {
		t.Error("wikidata_synced_at not set")
	}
}

// PB-14: with the setting off the job fetches nothing; switching it off forgets every match at
// once (the providers are the streams' domains again), switching it on queues a run.
func TestWikidataSettingOff(t *testing.T) {
	e, f := wdEnv(t)
	e.apply(choose("CH"))
	e.wdRun()
	e.exec(`UPDATE radio_stations SET provider = 'srg-ssr.ch' WHERE name = 'SRF 3'`)
	e.s.reloadStation(e.ctx, e.stationID("SRF 3"))
	if got := e.station("SRF 3").Provider; got != "Schweizer Radio und Fernsehen" {
		t.Fatalf("provider before switching off %q", got)
	}
	off := false
	cfg, err := e.s.UpdateConfig(e.ctx, model.RadioConfigUpdate{WikidataEnabled: &off})
	if err != nil || cfg.WikidataEnabled {
		t.Fatalf("switch off: %+v %v", cfg, err)
	}
	if n := e.count(`SELECT count(*) FROM radio_station_wikidata`); n != 0 {
		t.Errorf("%d matches kept after switching off", n)
	}
	if got := e.station("SRF 3").Provider; got != "srg-ssr.ch" {
		t.Errorf("provider after switching off %q, want the stream's", got)
	}
	before := f.stations.Load() + f.labels.Load()
	e.wdRun()
	if after := f.stations.Load() + f.labels.Load(); after != before {
		t.Errorf("the job asked Wikidata %d times with the setting off", after-before)
	}
	if n := e.count(`SELECT count(*) FROM radio_station_wikidata`); n != 0 {
		t.Errorf("%d matches stored with the setting off", n)
	}
}

// Switching the setting on queues a run; an apply that adds stations queues one only when the
// last run is more than 24 hours ago; an apply that adds none queues nothing.
func TestWikidataQueue(t *testing.T) {
	e, _, _ := catJobs(t)
	withDialControl(e.s, loopbackOnly)
	f := newWDFake(t)
	e.s.wd.endpoint = f.srv.URL + "/sparql"
	e.s.wd.minItems = 5
	e.emptyOffer()
	e.sync()
	queued := func() int {
		t.Helper()
		return e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobWikidataSync)
	}
	e.apply(choose("CH"))
	if queued() != 1 {
		t.Fatalf("an apply that added stations queued %d runs, want 1 (never ran)", queued())
	}
	e.exec(`DELETE FROM jobs WHERE type = ?`, JobWikidataSync)
	e.wdRun()
	e.apply(choose("AT"))
	if queued() != 0 {
		t.Error("an apply within 24 hours of the last run queued a run")
	}
	e.clock.Advance(25 * time.Hour)
	e.apply(demark(e.stationID("FM4")))
	if queued() != 0 {
		t.Error("an apply that added no station queued a run")
	}
	e.apply(choose("DE"))
	if queued() != 1 {
		t.Errorf("an apply 25 hours after the last run queued %d runs, want 1", queued())
	}
	e.exec(`DELETE FROM jobs WHERE type = ?`, JobWikidataSync)
	off, on := false, true
	if _, err := e.s.UpdateConfig(e.ctx, model.RadioConfigUpdate{WikidataEnabled: &off}); err != nil {
		t.Fatal(err)
	}
	e.apply(choose("FR"))
	if queued() != 0 {
		t.Error("an apply with the setting off queued a run")
	}
	if _, err := e.s.UpdateConfig(e.ctx, model.RadioConfigUpdate{WikidataEnabled: &on}); err != nil {
		t.Fatal(err)
	}
	if queued() != 1 {
		t.Errorf("switching on queued %d runs, want 1", queued())
	}
	if _, err := e.s.UpdateConfig(e.ctx, model.RadioConfigUpdate{WikidataEnabled: &on}); err != nil || queued() != 1 {
		t.Errorf("on again: %d runs, %v", queued(), err)
	}
}

// A station that leaves the offer loses its match at once (B7.1); a station of a country that is
// removed loses it too.
func TestWikidataLeavesWithTheOffer(t *testing.T) {
	e, _ := wdEnv(t)
	e.apply(choose("CH"))
	e.wdRun()
	e.apply(demark(e.stationID("Energy Zürich")))
	if e.wdRow("Energy Zürich") != nil || e.wdRow("SRF 3") == nil {
		t.Error("the demarked station kept its match, or another station lost it")
	}
	if idx, _ := e.s.index(e.ctx); idx.byID[e.stationID("Energy Zürich")].st.Provider == "Energy Schweiz" {
		t.Error("the index kept the owner of the demarked station")
	}
	e.apply(model.RadioOfferStep{Op: model.RadioOfferRemoveCountry, Country: "CH"})
	if n := e.count(`SELECT count(*) FROM radio_station_wikidata`); n != 0 {
		t.Errorf("%d matches kept after CH left the offer", n)
	}
	// An empty offer asks nothing and records no run: the next apply still queues one (B7.3).
	e.exec(`UPDATE radio_config SET wikidata_synced_at = NULL`)
	e.wdRun()
	if n := e.count(`SELECT count(*) FROM radio_config WHERE wikidata_synced_at IS NULL`); n != 1 {
		t.Error("a run without a station to ask for recorded itself as the last run")
	}
}

// The guarded client refuses a local endpoint (the production control, the test build is not
// radiotest); an answer over the cap and an error status fail the run without storing anything;
// errors name the host only.
func TestWikidataNetworkRules(t *testing.T) {
	e, f := wdEnv(t)
	e.apply(choose("CH"))
	e.s.rel.control = controlFunc
	e.s.rel.net = nil
	e.s.rel.netOnce = sync.Once{}
	err := e.s.handleWikidataSync(e.ctx, nil)
	if !errors.Is(err, netguard.ErrRefused) || f.stations.Load() != 0 {
		t.Fatalf("local endpoint: %v, %d requests", err, f.stations.Load())
	}
	if strings.Contains(err.Error(), "/sparql") {
		t.Errorf("error names the path: %v", err)
	}
	withDialControl(e.s, loopbackOnly)
	e.s.rel.net = nil
	e.s.rel.netOnce = sync.Once{}
	if _, err := e.s.wikidataPost(e.ctx, wikidataStationQuery, 100); err == nil || !strings.Contains(err.Error(), "more than 100 bytes") {
		t.Errorf("answer over the cap: %v", err)
	}
	e.s.wd.endpoint = f.srv.URL + "/other"
	if err := e.s.handleWikidataSync(e.ctx, nil); err == nil || !strings.Contains(err.Error(), "answered 400") {
		t.Errorf("error status: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM radio_station_wikidata`); n != 0 {
		t.Errorf("%d matches stored by failed runs", n)
	}
	e.s.wd.endpoint = f.srv.URL + "/sparql"
	// A run that stored matches, then an answer with status 200 but only its head (a hiccup of the
	// service): the attempt fails, the matches and the time of the last run stay.
	e.wdRun()
	stored, synced := e.count(`SELECT count(*) FROM radio_station_wikidata`), e.count(`SELECT wikidata_synced_at FROM radio_config`)
	head := []byte("s,site,owner,founded,precision\r\n")
	f.body.Store(&head)
	e.clock.Advance(time.Hour)
	if err := e.s.handleWikidataSync(e.ctx, nil); !errors.Is(err, errWikidataShort) {
		t.Errorf("an answer without stations: %v", err)
	}
	if n, at := e.count(`SELECT count(*) FROM radio_station_wikidata`), e.count(`SELECT wikidata_synced_at FROM radio_config`); n != stored || n == 0 || at != synced {
		t.Errorf("after a short answer: %d matches (were %d), synced %d (was %d)", n, stored, at, synced)
	}
	f.body.Store(nil)
	// The client waits longer for the answer's head than the relay client; dial control and
	// redirect checks stay the guarded ones.
	c := e.s.wikidataClient()
	if tr, ok := c.Transport.(*http.Transport); !ok || tr.ResponseHeaderTimeout != wikidataHeaderTimeout ||
		c.CheckRedirect == nil || e.s.netClient().http.Transport.(*http.Transport).ResponseHeaderTimeout == wikidataHeaderTimeout {
		t.Errorf("wikidata client: %+v", c)
	}
	e.s.wd.timeout = time.Nanosecond
	if err := e.s.handleWikidataSync(e.ctx, nil); err == nil {
		t.Error("a request past its deadline succeeded")
	}
}

// The parts of the match: addresses, dates, the platform guard and an answer without a column.
func TestWikidataMatchRules(t *testing.T) {
	for raw, want := range map[string][2]string{
		"https://WWW.Radio.Example/Live/":    {"radio.example", "radio.example/live"},
		"http://radio.example?utm=1#top":     {"radio.example", "radio.example"},
		"https://www.radio.example./de":      {"radio.example", "radio.example/de"},
		"ftp://radio.example/":               {"", ""},
		"radio.example":                      {"", ""},
		"https://www.zeno.fm/radio/abc-123/": {"zeno.fm", "zeno.fm/radio/abc-123"},
	} {
		if h, a := wdAddress(raw); h != want[0] || a != want[1] {
			t.Errorf("wdAddress(%q) = %q %q, want %q", raw, h, a, want)
		}
	}
	for in, want := range map[[2]string]string{
		{"1983-11-01T00:00:00Z", "11"}: "1983-11-01",
		{"1983-11-01T00:00:00Z", "14"}: "1983-11-01",
		{"1983-11-01T00:00:00Z", "10"}: "1983-11",
		{"1983-01-01T00:00:00Z", "9"}:  "1983",
		{"1900-01-01T00:00:00Z", "8"}:  "",
		{"0950-01-01T00:00:00Z", "9"}:  "",
		{"-0500-01-01T00:00:00Z", "9"}: "",
		{"1983-00-00T00:00:00Z", "11"}: "1983",
		{"1983-05-00T00:00:00Z", "11"}: "1983-05",
		{"", ""}:                       "",
	} {
		if got := wdFounded(in[0], in[1]); got != want {
			t.Errorf("wdFounded(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
	// A host with more than wikidataPlatformURLs station addresses matches through its address only.
	idx := &stationIndex{byID: map[int64]*indexEntry{}}
	for i := range wikidataPlatformURLs + 1 {
		idx.entries = append(idx.entries, &indexEntry{st: model.RadioStation{ID: int64(i + 1),
			Homepage: fmt.Sprintf("https://zeno.fm/radio/s%d/", i)}})
	}
	idx.entries = append(idx.entries, &indexEntry{st: model.RadioStation{ID: 999, Homepage: "https://somafm.example/x"}})
	cat, err := parseWikidataStations(strings.NewReader("s,site,owner,founded,precision\n" +
		"http://www.wikidata.org/entity/Q1,https://zeno.fm/,,,\n" +
		"http://www.wikidata.org/entity/Q2,https://zeno.fm/radio/s7,,,\n" +
		"http://www.wikidata.org/entity/Q3,https://somafm.example/,,,\n" +
		"not an entity,https://other.example/,,,\n"))
	if err != nil {
		t.Fatal(err)
	}
	platforms := platformHosts(idx)
	if !platforms["zeno.fm"] || platforms["somafm.example"] {
		t.Fatalf("platforms %v", platforms)
	}
	for home, want := range map[string]string{
		"https://zeno.fm/radio/s7/": "Q2", // its own address
		"https://zeno.fm/radio/s8/": "",   // the platform's item would match through the host
		"https://somafm.example/x":  "Q3",
		"https://other.example/":    "",
	} {
		if got := cat.match(home, platforms); got != want {
			t.Errorf("match(%q) = %q, want %q", home, got, want)
		}
	}
	if _, err := parseWikidataStations(strings.NewReader("s,site,owner\n")); err == nil {
		t.Error("an answer without the column founded parsed")
	}
	if !slices.Equal(addDistinct([]string{"a"}, "a"), []string{"a"}) {
		t.Error("addDistinct repeated a value")
	}
}

// A language outside the pattern falls back to English; without a source the labels are English.
func TestWikidataLanguage(t *testing.T) {
	s := New(nil, catDiscard)
	if got := s.wikidataLanguage(context.Background()); got != "en" {
		t.Errorf("without a source %q", got)
	}
	for in, want := range map[string]string{"de": "de", "fr-CH": "fr", "EN_gb": "en", "": "en", "deutsch": "en", "x\"y": "en"} {
		s := New(nil, catDiscard, WithLanguage(func(context.Context) string { return in }))
		if got := s.wikidataLanguage(context.Background()); got != want {
			t.Errorf("language %q = %q, want %q", in, got, want)
		}
	}
}
