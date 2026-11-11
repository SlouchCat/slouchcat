package radio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

// The catalog tests (lane B) run the sync against an httptest server that serves the frozen
// extract testdata/radio/catalog.json (68 entries, 58 stations), with {{BASE}} replaced by the
// server's URL. The catalog client gets the test server's unguarded client, so no request leaves
// the machine.

var catDiscard = slog.New(slog.NewTextHandler(io.Discard, nil))

// catClock is the fake clock of the catalog tests.
type catClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *catClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *catClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// catEnv is a radio service on a fresh database with the extract served by srv.
type catEnv struct {
	t     testing.TB
	ctx   context.Context
	db    *store.DB
	s     *Service
	clock *catClock
	srv   *httptest.Server
	body  atomic.Pointer[[]byte] // the station list srv answers with
	// override, when set, answers every request of srv instead of the station list.
	override atomic.Pointer[http.HandlerFunc]
	hits     atomic.Int32 // requests to /json/stations
}

// catEntries reads the extract.
func catEntries(t testing.TB) []rbStation {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "radio", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list []rbStation
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	return list
}

func newCatEnv(t testing.TB, opts ...Option) *catEnv {
	t.Helper()
	ctx := context.Background()
	db, err := storetest.Open(ctx, store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: catDiscard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &catEnv{t: t, ctx: ctx, db: db, clock: &catClock{now: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)}}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := e.override.Load(); h != nil {
			(*h)(w, r)
			return
		}
		if r.URL.Path != "/json/stations" || r.URL.Query().Get("hidebroken") != "false" {
			http.NotFound(w, r)
			return
		}
		// The real API cuts a list without limit to 1000 entries: the client must ask for all.
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err != nil || n < 100_000 {
			http.Error(w, "limit", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Slouchcat/") {
			http.Error(w, "user agent", http.StatusForbidden)
			return
		}
		e.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(*e.body.Load())
	}))
	t.Cleanup(e.srv.Close)
	e.setEntries(catEntries(t))
	all := append([]Option{WithClock(e.clock.Now), WithUserAgent("Slouchcat/test")}, opts...)
	e.s = New(db, catDiscard, all...)
	e.s.cat.fetch.client = e.srv.Client()
	e.s.cat.fetch.limiter = ratelimit.New("radio-browser-test", 1000, 1000)
	e.s.cat.fetch.base, e.s.cat.fetch.mirrors = e.srv.URL, false
	e.s.cat.zone = func() string { return "Europe/Zurich" }
	e.exec(`INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`)
	e.exec(`INSERT INTO profiles (id, account_id, name, created_at, updated_at) VALUES (1, 1, 'P1', 0, 0), (2, 1, 'P2', 0, 0)`)
	// The catalog tests list the catalog: every country is in the offer with «all» (B4.1), so a
	// station with a country is offered; the offer tests start from an empty offer (emptyOffer).
	for _, c := range []string{"AT", "BE", "CH", "DE", "ES", "FR", "GB", "IT", "NL", "US"} {
		e.exec(`INSERT INTO radio_offer_countries (country, mode, updated_at) VALUES (?, 'all', 0)`, c)
	}
	return e
}

// emptyOffer clears the offer of the environment (tables and snapshot).
func (e *catEnv) emptyOffer() {
	e.t.Helper()
	e.exec(`DELETE FROM radio_offer_countries`)
	e.exec(`DELETE FROM radio_offer_exceptions`)
	e.s.cat.offer.Store(nil)
}

// apply stores steps on the current offer and returns the result.
func (e *catEnv) apply(steps ...model.RadioOfferStep) model.RadioOfferApplied {
	e.t.Helper()
	o, err := e.s.offer(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	a, err := e.s.ApplyOffer(e.ctx, o.version, steps)
	if err != nil {
		e.t.Fatal(err)
	}
	return a
}

// answer makes srv answer every request with h.
func (e *catEnv) answer(h http.HandlerFunc) { e.override.Store(&h) }

// setEntries replaces the station list the server answers with.
func (e *catEnv) setEntries(list []rbStation) {
	e.t.Helper()
	raw, err := json.Marshal(list)
	if err != nil {
		e.t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte("{{BASE}}"), []byte(e.srv.URL))
	e.body.Store(&raw)
}

func (e *catEnv) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Write.ExecContext(e.ctx, query, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *catEnv) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.db.Read.QueryRowContext(e.ctx, query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// sync runs the job handler once.
func (e *catEnv) sync() {
	e.t.Helper()
	if err := e.s.handleCatalogSync(e.ctx, nil); err != nil {
		e.t.Fatal(err)
	}
}

// session is a session of profile id.
func catSession(profileID int64) model.Session {
	return model.Session{ID: profileID, Account: model.Account{ID: 1}, Profile: model.Profile{ID: profileID, AccountID: 1}}
}

// stationID returns the id of the station named name.
func (e *catEnv) stationID(name string) int64 {
	e.t.Helper()
	var id int64
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT id FROM radio_stations WHERE name = ?`, name).Scan(&id); err != nil {
		e.t.Fatalf("station %q: %v", name, err)
	}
	return id
}

// station returns the index view of the station named name for profile 1.
func (e *catEnv) station(name string) model.RadioStation {
	e.t.Helper()
	st, err := e.s.Station(e.ctx, catSession(1), e.stationID(name))
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}

// list returns the names of the listed stations for q (profile 1).
func (e *catEnv) list(q model.RadioQuery) []string {
	e.t.Helper()
	if q.Limit == 0 {
		q.Limit = 120
	}
	page, err := e.s.ListStations(e.ctx, catSession(1), q)
	if err != nil {
		e.t.Fatal(err)
	}
	out := make([]string, 0, len(page.Items))
	for _, st := range page.Items {
		out = append(out, st.Name)
	}
	return out
}

func entryNamed(list []rbStation, name string) int {
	for i := range list {
		if list[i].Name == name {
			return i
		}
	}
	panic("no entry " + name)
}

// Duplicates by bitrate and format become one station with one stream per entry; the stats count
// entries and stations.
func TestSyncCreatesAndGroupsStations(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	if n := e.count(`SELECT count(*) FROM radio_stations`); n != 58 {
		t.Errorf("stations = %d, want 58 (68 entries, 10 duplicates)", n)
	}
	if n := e.count(`SELECT count(*) FROM radio_station_aliases`); n != 68 {
		t.Errorf("aliases = %d, want 68", n)
	}
	for name, streams := range map[string]int{"SRF 3": 3, "Radio Paradise 320k": 3, "Radio Swiss Jazz": 2, "Antenne Bayern": 2, "Radio 24": 1} {
		if n := e.count(`SELECT count(*) FROM radio_streams WHERE station_id = ?`, e.stationID(name)); n != streams {
			t.Errorf("%s: %d streams, want %d", name, n, streams)
		}
	}
	cs, err := e.s.CatalogState(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.EntriesTotal != 68 || cs.EntriesReachable != 66 || cs.StationsTotal != 58 || cs.LastAdded != 58 || cs.LastRemoved != 0 ||
		cs.LastSyncAt.IsZero() || cs.SyncFailures != 0 || cs.Source != "radio-browser" {
		t.Errorf("catalog state = %+v", cs)
	}
	// A second run adds nothing.
	e.sync()
	if cs, _ := e.s.CatalogState(e.ctx); cs.LastAdded != 0 || cs.StationsTotal != 58 {
		t.Errorf("second run: %+v", cs)
	}
}

// A renamed entry stays with its station (an alias never moves); the name follows the catalog.
func TestSyncKeepsAliasOnRename(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	id := e.stationID("Radio 24")
	list := catEntries(t)
	i := entryNamed(list, "Radio 24")
	list[i].Name, list[i].LastChangeTime = "Radio 24 Zürich", "2026-11-06 08:00:00"
	e.setEntries(list)
	e.sync()
	if got := e.stationID("Radio 24 Zürich"); got != id {
		t.Errorf("renamed entry moved from station %d to %d", id, got)
	}
	if n := e.count(`SELECT count(*) FROM radio_stations`); n != 58 {
		t.Errorf("stations after the rename = %d, want 58", n)
	}
}

// The best stream (5.3) gives codec, bitrate and quality; the admin's preferred stream wins once
// the field stream is locked.
func TestSyncBestStream(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	for name, want := range map[string]struct {
		codec   string
		kbps    int
		quality string
	}{
		"SRF 3":               {"mp3", 128, model.RadioQualityGood},
		"Bayern 3":            {"aac", 256, model.RadioQualityHigh},
		"Radio Paradise 320k": {"mp3", 320, model.RadioQualityHigh},
		"talkSPORT":           {"mp3", 64, model.RadioQualityBasic},
	} {
		st := e.station(name)
		if st.Codec != want.codec || st.BitrateKbps != want.kbps || st.Quality != want.quality {
			t.Errorf("%s: %s %d %s, want %+v", name, st.Codec, st.BitrateKbps, st.Quality, want)
		}
	}
	// Equal bitrate: AAC before MP3.
	id := e.stationID("Radio Swiss Jazz")
	e.exec(`UPDATE radio_streams SET bitrate = 128 WHERE station_id = ?`, id)
	e.s.reloadStation(e.ctx, id)
	if st := e.station("Radio Swiss Jazz"); st.Codec != "aac" {
		t.Errorf("equal bitrate: codec %q, want aac", st.Codec)
	}
	srf := e.stationID("SRF 3")
	a, err := e.s.AdminStation(e.ctx, srf)
	if err != nil {
		t.Fatal(err)
	}
	low := a.Streams[len(a.Streams)-1]
	if low.BitrateKbps != 32 {
		t.Fatalf("lowest ranked stream = %+v, want the 32 kbit/s one", low)
	}
	if _, err := e.s.UpdateStation(e.ctx, srf, model.AdminRadioStationPatch{PreferredStreamID: &low.ID}); err != nil {
		t.Fatal(err)
	}
	if st := e.station("SRF 3"); st.BitrateKbps != 32 {
		t.Errorf("preferred and locked stream: bitrate %d, want 32", st.BitrateKbps)
	}
}

// A locked field keeps the admin's value through a sync; an unlocked one and the facets follow
// the catalog (no facet is set by hand since concept 19 part B, B5).
func TestSyncRespectsLocks(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	id := e.stationID("Radio 24")
	name := "Radio 24 (Admin)"
	if _, err := e.s.UpdateStation(e.ctx, id, model.AdminRadioStationPatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	list := catEntries(t)
	i := entryNamed(list, "Radio 24")
	list[i].Name, list[i].Tags, list[i].LastChangeTime = "Radio 24 Neu", "rock,sport", "2026-11-06 08:00:00"
	e.setEntries(list)
	e.sync()
	st, err := e.s.Station(e.ctx, catSession(1), id)
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != name || fmt.Sprint(st.Genres) != "[rock]" || fmt.Sprint(st.Kinds) != "[sport]" {
		t.Errorf("locked name and facets after the sync: %q %v %v, want the name and rock, sport from the tags", st.Name, st.Genres, st.Kinds)
	}
	// Lifting the locks lets the next sync overwrite the fields.
	none := []string{}
	if _, err := e.s.UpdateStation(e.ctx, id, model.AdminRadioStationPatch{Locks: &none}); err != nil {
		t.Fatal(err)
	}
	list[i].LastChangeTime = "2026-11-07 08:00:00"
	e.setEntries(list)
	e.sync()
	if st, _ := e.s.Station(e.ctx, catSession(1), id); st.Name != "Radio 24 Neu" || fmt.Sprint(st.Genres) != "[rock]" {
		t.Errorf("after lifting the locks: %q %v", st.Name, st.Genres)
	}
}

// An entry that left the catalog marks its station missing (still a favourite, state missing);
// seven days later the sync deletes the station and the favourite with it (F-18).
func TestSyncMarksMissingAndDeletesAfterSevenDays(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	id := e.stationID("Radio 24")
	if err := e.s.AddFavorite(e.ctx, catSession(1), id); err != nil {
		t.Fatal(err)
	}
	list := catEntries(t)
	i := entryNamed(list, "Radio 24")
	e.setEntries(append(list[:i:i], list[i+1:]...))
	e.clock.Advance(24 * time.Hour)
	e.sync()
	st, err := e.s.Station(e.ctx, catSession(1), id)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != model.RadioStateMissing || !st.MissingSince.Equal(e.clock.Now()) || !st.RemovalAt.Equal(e.clock.Now().Add(7*24*time.Hour)) {
		t.Errorf("missing station = %s since %v removal %v", st.State, st.MissingSince, st.RemovalAt)
	}
	favs, err := e.s.Favorites(e.ctx, catSession(1))
	if err != nil || len(favs) != 1 || favs[0].State != model.RadioStateMissing {
		t.Fatalf("favourites of a missing station = %+v %v", favs, err)
	}
	e.clock.Advance(6 * 24 * time.Hour)
	e.sync()
	if _, err := e.s.Station(e.ctx, catSession(1), id); err != nil {
		t.Errorf("deleted before day 7: %v", err)
	}
	e.clock.Advance(24*time.Hour + time.Minute)
	e.sync()
	if _, err := e.s.Station(e.ctx, catSession(1), id); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("station after day 7: %v, want not found", err)
	}
	if n := e.count(`SELECT count(*) FROM radio_favorites`); n != 0 {
		t.Errorf("favourites after the deletion = %d, want 0", n)
	}
	if cs, _ := e.s.CatalogState(e.ctx); cs.LastRemoved != 1 {
		t.Errorf("last removed = %d, want 1", cs.LastRemoved)
	}
}

// synthCatalog is a synthetic catalog of n entries: every fourth station has a second entry in
// another bitrate, so n entries form n - n/5 stations (n a multiple of 5).
func synthCatalog(n int) []byte {
	var b bytes.Buffer
	b.WriteString("[")
	station := 0
	for i := 0; i < n; i++ {
		second := i%5 == 4
		if !second {
			station++
		}
		if i > 0 {
			b.WriteString(",")
		}
		br := 128
		if second {
			br = 64
		}
		fmt.Fprintf(&b, `{"stationuuid":"synth-%d","name":"Synth Station %d %dk","url":"https://s%d.example/live","url_resolved":"",`+
			`"homepage":"https://s%d.example/","favicon":"","tags":"pop,rock,80s hits","countrycode":"DE","iso_3166_2":"",`+
			`"state":"","language":"german","languagecodes":"de","codec":"MP3","bitrate":%d,"hls":0,"lastcheckok":1,`+
			`"votes":%d,"clickcount":%d,"lastchangetime":"2026-11-01 10:00:00"}`,
			i, station, br, station, station, br, i%100, i%50)
	}
	b.WriteString("]")
	return b.Bytes()
}

// The sync streams a catalog of 50 000 entries (correctness only; time and heap are measured by
// BenchmarkSyncLargeCatalog without the race detector).
func TestSyncStreamsLargeCatalog(t *testing.T) {
	e := newCatEnv(t)
	raw := synthCatalog(50000)
	e.body.Store(&raw)
	e.sync()
	if n := e.count(`SELECT count(*) FROM radio_station_aliases`); n != 50000 {
		t.Errorf("aliases = %d, want 50000", n)
	}
	if n := e.count(`SELECT count(*) FROM radio_stations`); n != 40000 {
		t.Errorf("stations = %d, want 40000", n)
	}
	if n, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{}); err != nil || len(n.Genres) == 0 || n.Genres[0].Count != 40000 {
		t.Errorf("facets over the large catalog = %+v %v", n.Genres, err)
	}
}

// BenchmarkSyncLargeCatalog measures one sync of 50 000 entries into an empty database: time per
// run and the peak heap in use (sampled every 5 ms). Run without -race:
// go test ./internal/radio -run '^$' -bench BenchmarkSyncLargeCatalog -benchmem -benchtime 1x
func BenchmarkSyncLargeCatalog(b *testing.B) {
	raw := synthCatalog(50000)
	for b.Loop() {
		b.StopTimer()
		e := newCatEnv(b)
		e.body.Store(&raw)
		runtime.GC()
		var base runtime.MemStats
		runtime.ReadMemStats(&base)
		var peak atomic.Uint64
		done := make(chan struct{})
		go func() {
			var m runtime.MemStats
			for {
				select {
				case <-done:
					return
				case <-time.After(5 * time.Millisecond):
				}
				runtime.ReadMemStats(&m)
				if m.HeapInuse > peak.Load() {
					peak.Store(m.HeapInuse)
				}
			}
		}()
		b.StartTimer()
		e.sync()
		b.StopTimer()
		close(done)
		b.ReportMetric(float64(peak.Load()-min(peak.Load(), base.HeapInuse))/(1<<20), "peak-heap-MiB")
		b.StartTimer()
	}
}

// A second run with the same lastchangetime writes no tag, facet, stream or station row: only
// seen_at of the aliases and the stats.
func TestSyncSkipsUnchangedEntries(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	conn, err := e.db.Write.Conn(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The writer has one connection; temp triggers on it count the writes of the second run.
	for _, stmt := range []string{
		`CREATE TEMP TABLE writes (tbl TEXT)`,
		`CREATE TEMP TRIGGER w_tags AFTER INSERT ON radio_station_tags BEGIN INSERT INTO writes VALUES ('tags'); END`,
		`CREATE TEMP TRIGGER w_tags_d AFTER DELETE ON radio_station_tags BEGIN INSERT INTO writes VALUES ('tags'); END`,
		`CREATE TEMP TRIGGER w_facets AFTER INSERT ON radio_station_facets BEGIN INSERT INTO writes VALUES ('facets'); END`,
		`CREATE TEMP TRIGGER w_facets_d AFTER DELETE ON radio_station_facets BEGIN INSERT INTO writes VALUES ('facets'); END`,
		`CREATE TEMP TRIGGER w_streams AFTER UPDATE ON radio_streams BEGIN INSERT INTO writes VALUES ('streams'); END`,
		`CREATE TEMP TRIGGER w_stations AFTER UPDATE ON radio_stations BEGIN INSERT INTO writes VALUES ('stations'); END`,
		`CREATE TEMP TRIGGER w_aliases AFTER UPDATE ON radio_station_aliases BEGIN INSERT INTO writes VALUES ('aliases'); END`,
	} {
		if _, err := conn.ExecContext(e.ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(24 * time.Hour)
	e.sync()
	got := map[string]int{}
	rows, err := e.db.Write.QueryContext(e.ctx, `SELECT tbl, count(*) FROM writes GROUP BY tbl`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var tbl string
		var n int
		if err := rows.Scan(&tbl, &n); err != nil {
			t.Fatal(err)
		}
		got[tbl] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["aliases"] != 68 {
		t.Errorf("writes of an unchanged run = %v, want only the 68 alias touches", got)
	}
}

// A cancelled run stops between two batches and marks nothing missing.
func TestSyncCancel(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	release := make(chan struct{})
	e.answer(func(w http.ResponseWriter, r *http.Request) {
		// 1 500 entries, then the stream stalls until the client gives up.
		raw := synthCatalog(1500)
		_, _ = w.Write(raw[:len(raw)-1])
		_, _ = w.Write([]byte(","))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	defer close(release)
	errc := make(chan error, 1)
	go func() {
		e.clock.Advance(time.Hour)
		_, err := e.s.syncCatalog(ctx)
		errc <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for e.count(`SELECT count(*) FROM radio_station_aliases WHERE stationuuid LIKE 'synth-%'`) < syncBatch {
		if time.Now().After(deadline) {
			t.Fatal("the first batch never arrived")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled sync: %v, want context.Canceled", err)
	}
	if n := e.count(`SELECT count(*) FROM radio_station_aliases WHERE stationuuid LIKE 'synth-%'`); n != syncBatch {
		t.Errorf("aliases of the cancelled run = %d, want one batch (%d)", n, syncBatch)
	}
	if n := e.count(`SELECT count(*) FROM radio_station_aliases WHERE missing_since IS NOT NULL`); n != 0 {
		t.Errorf("a cancelled run marked %d aliases missing", n)
	}
}

// Swiss regions come from iso_3166_2 or, for CH, from the free text of state.
func TestRegionsCH(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	for name, want := range map[string]string{
		"Radio 24": "CH-ZH", "Radio Zürisee": "CH-ZH", "Energy Zürich": "CH-ZH", "Couleur 3": "CH-VD", "Rete Tre": "CH-TI",
		"Radio Bern1": "CH-BE", "Radio Top": "CH-SG", "Radio Basilisk": "CH-BS", "Radio Fribourg": "CH-FR", "Radio Lac": "CH-GE",
		"Radio Rumantsch": "CH-GR", "Bayern 3": "DE-BY", "Deutschlandfunk": "",
	} {
		if got := e.station(name).Region; got != want {
			t.Errorf("%s: region %q, want %q", name, got, want)
		}
	}
	if got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Regions: []string{"CH-ZH"}}}); len(got) != 5 {
		t.Errorf("stations in CH-ZH = %v, want Testradio Eins, Energy Zürich, Radio SRF 1, Radio 24, Radio Zürisee", got)
	}
}

// Languages are ISO 639-1: languagecodes first, else the names of language.
func TestLanguageNormalisation(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	for name, want := range map[string]string{
		"Couleur 3": "[fr]", "Energy Zürich": "[de]", "Radio Swiss Jazz": "[en de]", "Radio Rumantsch": "[rm]",
		"Radio Swiss Classic": "[de fr it]", "Radio 538": "[nl]",
	} {
		if got := fmt.Sprint(e.station(name).Languages); got != want {
			t.Errorf("%s: languages %s, want %s", name, got, want)
		}
	}
}

// The index reads the station once the hook says its row changed.
func TestReloadStationOnHook(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	id := e.stationID("Radio 24")
	if got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Titles: true}}); len(got) != 0 {
		t.Fatalf("stations with titles before = %v", got)
	}
	e.exec(`UPDATE radio_stations SET shows_titles = 1 WHERE id = ?`, id)
	e.s.onStationChanged(e.ctx, id)
	if got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Titles: true}}); fmt.Sprint(got) != "[Radio 24]" {
		t.Errorf("stations with titles after the hook = %v", got)
	}
	e.exec(`DELETE FROM radio_stations WHERE id = ?`, id)
	e.s.onStationChanged(e.ctx, id)
	if _, err := e.s.Station(e.ctx, catSession(1), id); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted station after the hook: %v", err)
	}
}

// Before the first sync the lists answer model.ErrRadioCatalogLoading.
func TestCatalogLoadingBeforeFirstSync(t *testing.T) {
	e := newCatEnv(t)
	if _, err := e.s.ListStations(e.ctx, catSession(1), model.RadioQuery{Limit: 60}); !errors.Is(err, model.ErrRadioCatalogLoading) {
		t.Errorf("list before the first sync: %v", err)
	}
	if _, err := e.s.Facets(e.ctx, catSession(1), model.RadioQuery{}); !errors.Is(err, model.ErrRadioCatalogLoading) {
		t.Errorf("facets before the first sync: %v", err)
	}
	e.sync()
	if got := e.list(model.RadioQuery{}); len(got) == 0 {
		t.Error("no station after the first sync")
	}
}

// A failed sync stores the error and counts the failures; the stations stay.
func TestCatalogSyncFailureRecorded(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	e.answer(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	})
	e.clock.Advance(time.Hour)
	if err := e.s.handleCatalogSync(e.ctx, nil); err == nil {
		t.Fatal("sync against a 503 succeeded")
	}
	cs, err := e.s.CatalogState(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.SyncFailures != 1 || !strings.Contains(cs.LastSyncError, "503") || cs.StationsTotal != 58 || !cs.LastSyncAt.Equal(e.clock.Now()) {
		t.Errorf("state after a failed sync = %+v", cs)
	}
	if strings.Contains(cs.LastSyncError, "/json/") {
		t.Errorf("stored error names a path: %q", cs.LastSyncError)
	}
}

// After a connection failure of the base the client reads the mirrors from /json/servers (by name,
// only below .api.radio-browser.info), else from DNS SRV.
func TestCatalogMirrorFallback(t *testing.T) {
	for _, viaSRV := range []bool{false, true} {
		t.Run(fmt.Sprintf("srv=%v", viaSRV), func(t *testing.T) {
			e := newCatEnv(t)
			dead := httptest.NewServer(http.NotFoundHandler())
			dead.Close() // refuses connections
			var asked []string
			servers := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if viaSRV {
					http.Error(w, "gone", http.StatusBadGateway)
					return
				}
				_, _ = w.Write([]byte(`[{"ip":"1.2.3.4","name":"evil.example.com"},{"ip":"5.6.7.8","name":"de1.api.radio-browser.info"}]`))
			}))
			defer servers.Close()
			f := e.s.cat.fetch
			f.base, f.mirrors, f.serversURL = dead.URL, true, servers.URL
			f.client = &http.Client{}
			f.lookupSRV = func(context.Context) ([]string, error) {
				return []string{"nl1.api.radio-browser.info."}, nil
			}
			f.mirrorBase = func(name string) string {
				asked = append(asked, name)
				return e.srv.URL
			}
			e.sync()
			want := "de1.api.radio-browser.info"
			if viaSRV {
				want = "nl1.api.radio-browser.info"
			}
			if fmt.Sprint(asked) != "["+want+"]" {
				t.Errorf("mirrors asked = %v, want [%s]", asked, want)
			}
			if n := e.count(`SELECT count(*) FROM radio_stations`); n != 58 {
				t.Errorf("stations from the mirror = %d", n)
			}
		})
	}
}

// The visibility rules (S2 table, B4.2): a demarked station and one of a country outside the
// offer are 404 everywhere; a broken one is listed only while hide_broken is off, and stays
// reachable.
func TestVisibilityRules(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	p := catSession(1)
	hidden, de, broken := e.stationID("Radio 24"), e.stationID("Bayern 3"), e.stationID("Radio Stille")
	for _, id := range []int64{hidden, de, broken} {
		if err := e.s.AddFavorite(e.ctx, p, id); err != nil {
			t.Fatal(err)
		}
	}
	e.emptyOffer()
	e.apply(model.RadioOfferStep{Op: model.RadioOfferChooseCountry, Country: "CH"},
		model.RadioOfferStep{Op: model.RadioOfferDemark, StationIDs: []int64{hidden}})
	for _, id := range []int64{hidden, de} {
		if _, err := e.s.Station(e.ctx, p, id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("station %d: %v, want not found", id, err)
		}
		if err := e.s.AddFavorite(e.ctx, p, id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("add favourite %d: %v, want not found", id, err)
		}
	}
	names := e.list(model.RadioQuery{})
	for _, n := range []string{"Radio 24", "Bayern 3", "Radio Stille"} {
		if strings.Contains(fmt.Sprint(names), n) {
			t.Errorf("%s listed: %v", n, names)
		}
	}
	favs, err := e.s.Favorites(e.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(favs) != 1 || favs[0].ID != broken || favs[0].State != model.RadioStateBroken {
		t.Errorf("favourites = %+v, want only the broken station", favs)
	}
	if st, err := e.s.Station(e.ctx, p, broken); err != nil || st.State != model.RadioStateBroken || !st.Favorite {
		t.Errorf("broken station = %+v %v", st, err)
	}
	if err := e.s.AddFavorite(e.ctx, p, broken); err != nil {
		t.Errorf("add a broken favourite again: %v", err)
	}
	e.apply(model.RadioOfferStep{Op: model.RadioOfferSetHideBroken, Value: false})
	if !strings.Contains(fmt.Sprint(e.list(model.RadioQuery{})), "Radio Stille") {
		t.Error("broken station not listed with hide_broken off")
	}
	// The offer is reversible: Germany chosen, the favourite is back.
	e.apply(model.RadioOfferStep{Op: model.RadioOfferChooseCountry, Country: "DE"})
	if _, err := e.s.Station(e.ctx, p, de); err != nil {
		t.Errorf("Bayern 3 with Germany chosen: %v", err)
	}
	if favs, _ := e.s.Favorites(e.ctx, p); len(favs) != 2 {
		t.Errorf("favourites with Germany chosen = %+v, want Bayern 3 and the broken one", favs)
	}
}

// Favourites keep their order; a new order must name exactly the visible favourites.
func TestFavoritesOrder(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	p := catSession(1)
	a, b, c := e.stationID("SRF 3"), e.stationID("FIP"), e.stationID("FM4")
	for _, id := range []int64{a, b, c} {
		if err := e.s.AddFavorite(e.ctx, p, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.s.SetFavoriteOrder(e.ctx, p, []int64{c, a}); err == nil {
		t.Error("an order without one favourite was accepted")
	}
	if err := e.s.SetFavoriteOrder(e.ctx, p, []int64{c, a, b}); err != nil {
		t.Fatal(err)
	}
	favs, err := e.s.Favorites(e.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(favs) != 3 || favs[0].ID != c || favs[1].ID != a || favs[2].ID != b {
		t.Errorf("order = %+v", favs)
	}
	if other, _ := e.s.Favorites(e.ctx, catSession(2)); len(other) != 0 {
		t.Errorf("profile 2 sees %d favourites", len(other))
	}
	if err := e.s.RemoveFavorite(e.ctx, p, a); err != nil {
		t.Fatal(err)
	}
	if page, _ := e.s.ListStations(e.ctx, p, model.RadioQuery{RadioFilter: model.RadioFilter{Favorites: true}, Limit: 60}); page.Total != 2 {
		t.Errorf("favourite filter: %d stations, want 2", page.Total)
	}
}

// The default country comes from the server's time zone, else the metadata rating country;
// preferences round-trip.
func TestPreferencesDefaultCountry(t *testing.T) {
	e := newCatEnv(t)
	p := catSession(1)
	got, err := e.s.Preferences(e.ctx, p)
	if err != nil || got.DefaultCountry != "CH" {
		t.Errorf("preferences in Europe/Zurich = %+v %v", got, err)
	}
	e.s.cat.zone = func() string { return "" }
	e.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('metadata.rating_country', 'AT', 0)`)
	if got, _ := e.s.Preferences(e.ctx, p); got.DefaultCountry != "AT" {
		t.Errorf("default from the rating country = %q, want AT", got.DefaultCountry)
	}
	want := model.RadioPreferences{DefaultCountry: "DE", LastSort: model.RadioSortName,
		LastFilter: &model.RadioFilter{Genres: []string{"jazz"}, Q: "fip"}}
	if got, err := e.s.SetPreferences(e.ctx, p, want); err != nil || got.DefaultCountry != "DE" || got.LastSort != "name" ||
		got.LastFilter == nil || got.LastFilter.Q != "fip" || fmt.Sprint(got.LastFilter.Genres) != "[jazz]" {
		t.Errorf("set preferences = %+v %v", got, err)
	}
	var verr *model.ValidationError
	if _, err := e.s.SetPreferences(e.ctx, p, model.RadioPreferences{DefaultCountry: "che", LastSort: "votes"}); !errors.As(err, &verr) || len(verr.Fields) != 2 {
		t.Errorf("bad preferences: %v", err)
	}
}

// The settings check their bounds; maxRelaysByBudget follows budget and buffer.
func TestConfigUpdate(t *testing.T) {
	e := newCatEnv(t)
	c, err := e.s.Config(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c.BufferMinutes != 30 || c.BudgetMB != 512 || c.MaxRelays != 10 || c.BufferMode != model.RadioBufferMemory ||
		c.MaxRelaysByBudget != 12 {
		t.Errorf("defaults = %+v", c)
	}
	ten, bad, many := 10, 20, 99
	if c, err := e.s.UpdateConfig(e.ctx, model.RadioConfigUpdate{BufferMinutes: &ten}); err != nil || c.MaxRelaysByBudget != 36 {
		t.Errorf("10 minutes: %+v %v", c, err)
	}
	var verr *model.ValidationError
	if _, err := e.s.UpdateConfig(e.ctx, model.RadioConfigUpdate{BufferMinutes: &bad, MaxRelays: &many}); !errors.As(err, &verr) || len(verr.Fields) != 2 {
		t.Errorf("bad settings: %v", err)
	}
}

// A station list with less than half of the known entries is a broken answer: the sync fails
// like any other, stores the failure and marks nothing missing.
func TestSyncRefusesShrunkList(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	list := catEntries(t)
	e.setEntries(list[:len(list)/3])
	e.clock.Advance(24 * time.Hour)
	if err := e.s.handleCatalogSync(e.ctx, nil); !errors.Is(err, errCatalogShrunk) {
		t.Fatalf("sync of a third of the list: %v, want errCatalogShrunk", err)
	}
	if n := e.count(`SELECT count(*) FROM radio_station_aliases WHERE missing_since IS NOT NULL`); n != 0 {
		t.Errorf("%d aliases marked missing", n)
	}
	if n := e.count(`SELECT count(*) FROM radio_stations WHERE missing_since IS NOT NULL`); n != 0 {
		t.Errorf("%d stations marked missing", n)
	}
	cs, err := e.s.CatalogState(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.SyncFailures != 1 || !strings.Contains(cs.LastSyncError, "shorter than half") {
		t.Errorf("state after the refused sync = %+v", cs)
	}
	// Half of the list or more is a real change.
	e.setEntries(list[:(len(list)+1)/2])
	e.sync()
	if n := e.count(`SELECT count(*) FROM radio_station_aliases WHERE missing_since IS NOT NULL`); n == 0 {
		t.Error("a sync of half the list marked nothing missing")
	}
}

// onStationsChanged reloads many stations with one copy of the index.
func TestReloadStationsBatch(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	a, b, gone := e.stationID("Radio 24"), e.stationID("Radio Pilatus"), e.stationID("Radio Top")
	e.exec(`UPDATE radio_stations SET shows_titles = 1 WHERE id IN (?, ?)`, a, b)
	e.exec(`DELETE FROM radio_stations WHERE id = ?`, gone)
	e.s.cat.mu.RLock()
	before := e.s.cat.idx
	e.s.cat.mu.RUnlock()
	e.s.onStationsChanged(e.ctx, []int64{a, b, gone})
	got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Titles: true}})
	if len(got) != 2 {
		t.Errorf("stations with titles after the batch = %v", got)
	}
	if _, err := e.s.Station(e.ctx, catSession(1), gone); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted station after the batch: %v", err)
	}
	if len(before.entries) != len(before.byID) || before.byID[gone] == nil {
		t.Error("the batch changed the index a reader holds")
	}
}
