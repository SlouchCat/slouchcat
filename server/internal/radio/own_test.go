package radio

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// ownHosts is the fake resolver of the own-station tests: public hosts on TEST-NET addresses,
// local ones in the household's ranges, everything else unknown.
var ownHosts = map[string][]string{
	"radio.example":  {"203.0.113.10"},
	"pls.example":    {"203.0.113.20"},
	"lan.example":    {"192.168.1.10"},
	"mixed.example":  {"203.0.113.11", "10.0.0.1"},
	"localhost":      {"127.0.0.1", "::1"},
	"public.example": {"2001:db8::10"},
	// The homepage of the catalog entry Radio 24.
	"www.radio24.example": {"203.0.113.24"},
}

// ownEnv is a catalog env with the fake resolver, a playlist server answering every host (beside
// the catalog) and a fake probe that counts its calls.
func ownEnv(t *testing.T) (*catEnv, *atomic.Int32) {
	t.Helper()
	e := newCatEnv(t, WithCacheDir(t.TempDir()))
	e.s.cat.resolve = func(_ context.Context, host string) ([]netip.Addr, error) {
		list, ok := ownHosts[host]
		if !ok {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		var out []netip.Addr
		for _, a := range list {
			out = append(out, netip.MustParseAddr(a))
		}
		return out, nil
	}
	// Every playlist host reaches the test server, whatever its name.
	addr := strings.TrimPrefix(e.srv.URL, "http://")
	e.s.cat.fetch.client = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}}}
	e.answer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/local.pls":
			_, _ = w.Write([]byte("[playlist]\nFile1=http://192.168.1.20:8000/live\nNumberOfEntries=1\n"))
		case "/public.m3u":
			_, _ = w.Write([]byte("#EXTM3U\nhttp://radio.example/live\n"))
		case "/json/stations": // the catalog, as newCatEnv serves it
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(*e.body.Load())
		default:
			http.NotFound(w, r)
		}
	})
	probes := &atomic.Int32{}
	e.s.cat.probe = func(_ context.Context, rawURL string) (transcode.RadioInput, bool, error) {
		probes.Add(1)
		if strings.Contains(rawURL, "dead") {
			return transcode.RadioInput{}, false, model.ErrRadioUnreachable
		}
		return transcode.RadioInput{Format: "mp3", Codec: "mp3", BitrateKbps: 192}, false, nil
	}
	t.Cleanup(e.s.cat.bg.wait)
	return e, probes
}

// fieldCode returns the code of field in a validation error, "" when it names none.
func fieldCode(err error, field string) string {
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

// An own station's stream address and homepage must be http or https on public addresses only;
// a host that resolves to a local address, a local IP literal and a playlist with a local entry
// are local_address (F-15, D-8), an unknown host or another scheme invalid_value. Nothing is
// stored for a refused create.
func TestOwnStationAddressChecks(t *testing.T) {
	e, probes := ownEnv(t)
	base := strings.Replace(e.srv.URL, "127.0.0.1", "pls.example", 1)
	for _, tc := range []struct {
		stream, homepage string
		field, code      string
	}{
		{"http://192.168.1.10:8000/stream", "", "streamUrl", codeLocalAddress},
		{"http://lan.example/s", "", "streamUrl", codeLocalAddress},
		{"http://mixed.example/s", "", "streamUrl", codeLocalAddress},
		{"http://localhost:8000/s", "", "streamUrl", codeLocalAddress},
		{"http://[::1]/s", "", "streamUrl", codeLocalAddress},
		{"http://169.254.1.1/s", "", "streamUrl", codeLocalAddress},
		{base + "/local.pls", "", "streamUrl", codeLocalAddress},
		{"http://nowhere.example/s", "", "streamUrl", codeInvalidValue},
		{"ftp://radio.example/s", "", "streamUrl", codeInvalidValue},
		{"radio.example/s", "", "streamUrl", codeInvalidValue},
		{"http://radio.example/s", "http://10.1.2.3/", "homepage", codeLocalAddress},
		{"http://radio.example/s", "javascript:alert(1)", "homepage", codeInvalidValue},
	} {
		_, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Lokal", StreamURL: tc.stream, Homepage: tc.homepage})
		if got := fieldCode(err, tc.field); got != tc.code {
			t.Errorf("%s / %s: %v, want %s on %s", tc.stream, tc.homepage, err, tc.code, tc.field)
		}
	}
	if n := e.count(`SELECT count(*) FROM radio_stations`); n != 0 {
		t.Errorf("%d stations stored", n)
	}
	for _, c := range []model.AdminRadioStationCreate{
		{Name: "Liste", StreamURL: base + "/public.m3u"},
		{Name: "IPv6", StreamURL: "http://public.example/live"},
		{Name: "Literal", StreamURL: "http://203.0.113.10/live"},
	} {
		if _, err := e.s.CreateStation(e.ctx, c); err != nil {
			t.Errorf("%s: %v", c.StreamURL, err)
		}
	}
	e.s.cat.bg.wait()
	if probes.Load() != 3 {
		t.Errorf("%d probes, want 3", probes.Load())
	}
	for field, c := range map[string]model.AdminRadioStationCreate{
		"name":      {Name: " ", StreamURL: "http://radio.example/s"},
		"country":   {Name: "X", StreamURL: "http://radio.example/s", Country: "che"},
		"region":    {Name: "X", StreamURL: "http://radio.example/s", Region: "Zurich"},
		"languages": {Name: "X", StreamURL: "http://radio.example/s", Languages: []string{"deu"}},
		"genres":    {Name: "X", StreamURL: "http://radio.example/s", Genres: []string{"polka-techno"}},
	} {
		if _, err := e.s.CreateStation(e.ctx, c); fieldCode(err, field) != codeInvalidValue {
			t.Errorf("%s: %v, want invalid_value", field, err)
		}
	}
}

// A created own station has source own, its group key own:<ULID>, one preferred stream, the given
// facets manual and locked with the region, the others derived from its name; the probe fills
// codec, bitrate and quality. It is an exception of the admin list, found by source own only,
// and the app lists it with its languages.
func TestOwnStationCreate(t *testing.T) {
	e, _ := ownEnv(t)
	e.sync()
	a, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: " Mein Sender ", StreamURL: "http://radio.example/live",
		Homepage: "https://radio.example/", Country: "CH", Region: "CH-ZH", Languages: []string{"de", "fr", "de"},
		Genres: []string{"jazz"}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Source != sourceOwn || a.Name != "Mein Sender" || !slices.Equal(a.Locks, []string{"genre", "region"}) ||
		len(a.Streams) != 1 || a.Streams[0].URL != "http://radio.example/live" || !a.Streams[0].Preferred || a.AliasCount != 0 {
		t.Fatalf("created %+v", a)
	}
	if !slices.Equal(a.Languages, []string{"de", "fr"}) || a.Region != "CH-ZH" || a.Homepage != "https://radio.example/" {
		t.Errorf("languages %v region %q homepage %q", a.Languages, a.Region, a.Homepage)
	}
	wantFacets := []model.RadioStationFacet{{Facet: "genre", Value: "jazz", Manual: true}, {Facet: "kind", Value: "music"}}
	if !slices.Equal(a.Facets, wantFacets) {
		t.Errorf("facets %+v, want %+v", a.Facets, wantFacets)
	}
	var key string
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT group_key FROM radio_stations WHERE id = ?`, a.ID).Scan(&key); err != nil || !strings.HasPrefix(key, "own:") || len(key) != 30 {
		t.Errorf("group key %q, %v", key, err)
	}
	e.s.cat.bg.wait()
	st := e.station("Mein Sender")
	if st.Codec != "mp3" || st.BitrateKbps != 192 || st.Quality != model.RadioQualityGood {
		t.Errorf("after the probe: codec %q bitrate %d quality %q", st.Codec, st.BitrateKbps, st.Quality)
	}
	if got := e.list(model.RadioQuery{RadioFilter: model.RadioFilter{Languages: []string{"fr"}}}); !slices.Contains(got, "Mein Sender") {
		t.Errorf("language filter fr lists %v", got)
	}
	names := func(q model.AdminRadioQuery) []string {
		t.Helper()
		q.Limit = 200
		page, err := e.s.AdminStations(e.ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, it := range page.Items {
			out = append(out, it.Name)
		}
		return out
	}
	if got := names(model.AdminRadioQuery{Source: sourceOwn}); !slices.Equal(got, []string{"Mein Sender"}) {
		t.Errorf("source own lists %v", got)
	}
	if got := names(model.AdminRadioQuery{Source: sourceCatalog}); slices.Contains(got, "Mein Sender") || len(got) == 0 {
		t.Errorf("source catalog lists %v", got)
	}
	if got := names(model.AdminRadioQuery{Exceptions: true}); !slices.Contains(got, "Mein Sender") {
		t.Errorf("exceptions %v lack the own station", got)
	}
	if _, err := e.s.AdminStations(e.ctx, model.AdminRadioQuery{Source: "other"}); fieldCode(err, "source") != codeInvalidValue {
		t.Errorf("source other: %v", err)
	}
	// A failed probe does not refuse the create.
	if _, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Stumm", StreamURL: "http://radio.example/dead"}); err != nil {
		t.Errorf("create with a dead stream: %v", err)
	}
}

// The own-station fields of a PATCH are checked like a create, refused on a catalog station
// (not_own); a new stream address replaces the stream and is probed again.
func TestOwnStationUpdate(t *testing.T) {
	e, probes := ownEnv(t)
	e.sync()
	a, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Mein Sender", StreamURL: "http://radio.example/live", Country: "CH"})
	if err != nil {
		t.Fatal(err)
	}
	e.s.cat.bg.wait()
	local, other := "http://lan.example/live", "http://radio.example/other"
	if _, err := e.s.UpdateStation(e.ctx, a.ID, model.AdminRadioStationPatch{StreamURL: &local}); fieldCode(err, "streamUrl") != codeLocalAddress {
		t.Errorf("local stream address: %v", err)
	}
	catalog := e.stationID("Radio 24")
	de := "DE"
	if _, err := e.s.UpdateStation(e.ctx, catalog, model.AdminRadioStationPatch{StreamURL: &other, Country: &de}); fieldCode(err, "streamUrl") != codeNotOwn ||
		fieldCode(err, "country") != codeNotOwn {
		t.Errorf("own fields on a catalog station: %v", err)
	}
	langs := []string{"it"}
	got, err := e.s.UpdateStation(e.ctx, a.ID, model.AdminRadioStationPatch{StreamURL: &other, ClearCountry: true, Languages: &langs})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Streams) != 1 || got.Streams[0].URL != other || got.Country != "" || !slices.Equal(got.Languages, []string{"it"}) {
		t.Errorf("after the patch: %+v", got)
	}
	e.s.cat.bg.wait()
	if probes.Load() != 2 {
		t.Errorf("%d probes, want 2 (create and the new address)", probes.Load())
	}
	if _, err := e.s.UpdateStation(e.ctx, a.ID, model.AdminRadioStationPatch{StreamURL: &other}); err != nil {
		t.Fatal(err)
	}
	e.s.cat.bg.wait()
	if probes.Load() != 2 {
		t.Errorf("an unchanged address was probed again (%d)", probes.Load())
	}
}

// Deleting an own station removes it with its favourites and logo files; a catalog station is
// not_own, an unknown one not found.
func TestOwnStationDelete(t *testing.T) {
	e, _ := ownEnv(t)
	e.sync()
	a, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Mein Sender", StreamURL: "http://radio.example/live"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.AddFavorite(e.ctx, catSession(1), a.ID); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(e.s.o.cacheDir, logoDir(a.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.s.DeleteStation(e.ctx, e.stationID("Radio 24")); fieldCode(err, "stationId") != codeNotOwn {
		t.Errorf("delete a catalog station: %v", err)
	}
	if err := e.s.DeleteStation(e.ctx, 1<<40); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete an unknown station: %v", err)
	}
	if err := e.s.DeleteStation(e.ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Station(e.ctx, catSession(1), a.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("station after the delete: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM radio_favorites WHERE station_id = ?`, a.ID); n != 0 {
		t.Errorf("%d favourites left", n)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("logo directory: %v", err)
	}
}

// The sync never marks, deletes or regroups an own station: a catalog entry with the same name,
// country and homepage founds its own station, and after the seven days of F-18 the own station
// is still there.
func TestSyncNeverTouchesOwnStations(t *testing.T) {
	e, _ := ownEnv(t)
	e.sync()
	a, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Radio 24", StreamURL: "http://radio.example/live",
		Homepage: "https://www.radio24.example/", Country: "CH"})
	if err != nil {
		t.Fatal(err)
	}
	list := catEntries(t)
	twin := list[entryNamed(list, "Radio 24")]
	twin.StationUUID, twin.LastChangeTime = "11111111-2222-3333-4444-555555555555", "2026-11-05 10:00:00"
	list = append(list, twin)
	e.setEntries(list)
	for range 3 {
		e.clock.Advance(4 * 24 * time.Hour)
		e.sync()
	}
	var source string
	var missing, aliases int
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT source, missing_since IS NOT NULL,
		(SELECT count(*) FROM radio_station_aliases WHERE station_id = radio_stations.id) FROM radio_stations WHERE id = ?`, a.ID).
		Scan(&source, &missing, &aliases); err != nil {
		t.Fatalf("own station after the syncs: %v", err)
	}
	if source != sourceOwn || missing != 0 || aliases != 0 {
		t.Errorf("own station: source %s missing %d aliases %d", source, missing, aliases)
	}
	if n := e.count(`SELECT count(*) FROM radio_stations WHERE name = 'Radio 24' AND source = 'catalog'`); n != 1 {
		t.Errorf("%d catalog stations Radio 24, want 1 (the twin joins the catalog one)", n)
	}
	if n := e.count(`SELECT count(*) FROM radio_streams WHERE station_id = ?`, a.ID); n != 1 {
		t.Errorf("own station has %d streams, want 1", n)
	}
}

// The probe of the own stations reads codec and bitrate through the guarded client.
func TestProbeStreamReadsInput(t *testing.T) {
	st := startStation(t)
	st.Set("/own", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}})
	s := withDialControl(New(nil, testLogger()), loopbackOnly)
	in, hls, err := s.probeStream(context.Background(), st.URL("/own"))
	if err != nil || in.Codec != "mp3" || in.BitrateKbps != 128 || hls {
		t.Errorf("probe = %+v hls %v, %v", in, hls, err)
	}
}

// A deleted own station is gone for a Listen at once (404): the row and the index entry go before
// the relay stops, so no Listen in between starts a new one.
func TestOwnStationDeleteThenListen(t *testing.T) {
	e, _ := ownEnv(t)
	e.sync()
	a, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Mein Sender", StreamURL: "http://radio.example/live"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.DeleteStation(e.ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Listen(e.ctx, catSession(1), a.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("Listen after the delete: %v, want not found", err)
	}
}

// No sync derives an own station again, so a new name or a lifted facet lock derives its open
// facets at once; a locked facet keeps the admin's values.
func TestOwnStationRederivesOpenFacets(t *testing.T) {
	e, _ := ownEnv(t)
	e.sync()
	a, err := e.s.CreateStation(e.ctx, model.AdminRadioStationCreate{Name: "Mein Sender", StreamURL: "http://radio.example/live",
		Kinds: []string{"talk"}, Genres: []string{"jazz"}})
	if err != nil {
		t.Fatal(err)
	}
	name := "Sport Eins"
	if _, err := e.s.UpdateStation(e.ctx, a.ID, model.AdminRadioStationPatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if v := e.facetValues("Sport Eins", "kind"); !slices.Equal(v, []string{"talk"}) {
		t.Errorf("locked kind after the rename = %v, want [talk]", v)
	}
	keep := []string{"name", "genre"}
	if _, err := e.s.UpdateStation(e.ctx, a.ID, model.AdminRadioStationPatch{Locks: &keep}); err != nil {
		t.Fatal(err)
	}
	if v := e.facetValues("Sport Eins", "kind"); !slices.Equal(v, []string{"sport"}) {
		t.Errorf("kind after lifting its lock = %v, want [sport] from the name", v)
	}
	if v := e.facetValues("Sport Eins", "genre"); !slices.Equal(v, []string{"jazz"}) {
		t.Errorf("locked genre = %v, want [jazz]", v)
	}
	news := "Info Eins"
	if _, err := e.s.UpdateStation(e.ctx, a.ID, model.AdminRadioStationPatch{Name: &news}); err != nil {
		t.Fatal(err)
	}
	if st := e.station("Info Eins"); !slices.Equal(st.Kinds, []string{"news"}) {
		t.Errorf("index kinds after the rename = %v, want [news]", st.Kinds)
	}
}
