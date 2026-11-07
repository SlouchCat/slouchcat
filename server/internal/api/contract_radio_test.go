package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Webradio contract (concept 16 stage 1, contract 1.17.0): no radio schema and no radio operation
// refers to ItemRef, directly or through a chain of $ref (12.3: "Antworten enthalten kein ItemRef").
// A station is not a title; an ItemRef would carry scoped title image URLs into the radio views.
func TestRadioContractHasNoItemRef(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	var radioSchemas int
	for name, schema := range schemas {
		if !strings.HasPrefix(name, "Radio") && !strings.HasPrefix(name, "AdminRadio") {
			continue
		}
		radioSchemas++
		if references(doc, schema, itemRefPointer, map[string]bool{}) {
			t.Errorf("schema %s refers to ItemRef", name)
		}
	}
	if radioSchemas < 20 {
		t.Errorf("%d radio schemas in the contract, want the full set of contract 1.17.0", radioSchemas)
	}
	paths, _ := doc["paths"].(map[string]any)
	var radioOps int
	for path, item := range paths {
		if !strings.HasPrefix(path, "/radio/") && !strings.HasPrefix(path, "/admin/radio/") {
			continue
		}
		ops, _ := item.(map[string]any)
		for _, method := range httpMethods {
			op, ok := ops[method].(map[string]any)
			if !ok {
				continue
			}
			radioOps++
			if references(doc, op, itemRefPointer, map[string]bool{}) {
				t.Errorf("%s %s refers to ItemRef", strings.ToUpper(method), path)
			}
		}
	}
	if radioOps != 30 {
		t.Errorf("%d radio operations, want the 30 of contract 1.17.0", radioOps)
	}
}

// The radio errors carry what the client needs: radio_capacity the numbers of the relays (running,
// max), radio_catalog_loading the pause in Retry-After.
func TestRadioErrorExtensions(t *testing.T) {
	t.Parallel()
	p := problems{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	p.mapError(rec, httptest.NewRequest(http.MethodPost, "/api/v1/radio/stations/1/listen", nil), &model.RadioCapacityError{Running: 7, Max: 7})
	var body struct {
		Code       string         `json:"code"`
		Extensions map[string]int `json:"extensions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict || body.Code != "radio_capacity" || body.Extensions["running"] != 7 || body.Extensions["max"] != 7 {
		t.Errorf("capacity: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	p.mapError(rec, httptest.NewRequest(http.MethodGet, "/api/v1/radio/stations", nil), model.ErrRadioCatalogLoading)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "30" {
		t.Errorf("catalog loading: status %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

// radioListen starts a listen session through the API for the session name token and returns its
// listen id and the path of its playlist (with rt).
func radioListen(t *testing.T, h http.Handler, token string, station int64) (id, playlist string) {
	t.Helper()
	rec := send(t, h, http.MethodPost, fmt.Sprintf("%s/radio/stations/%d/listen", BasePath, station), token, "", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("listen %d: status %d (%s)", station, rec.Code, rec.Body.String())
	}
	var l struct {
		ListenID    string `json:"listenId"`
		PlaylistURL string `json:"playlistUrl"`
	}
	decode(t, rec, &l)
	return l.ListenID, l.PlaylistURL
}

// sendLive requests a live route with the listen token only (no cookie), like hls.js.
func sendLive(h http.Handler, path string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The S2 visibility table through the API: the hidden and the excluded-country station answer
// 404 not_found on station, favourite, listen, now and image for every profile; the broken one is
// reachable (state broken) but not listed while hide_broken is on.
func TestRadioHiddenStations404(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	for _, token := range []string{"p0", "p6", "p12", "p16", "p18", "px", "pg", "pg18"} {
		for _, id := range []string{radioHidden, radioExcluded} {
			for _, c := range []struct{ method, path string }{
				{http.MethodGet, "/radio/stations/" + id},
				{http.MethodPut, "/radio/favorites/" + id},
				{http.MethodPost, "/radio/stations/" + id + "/listen"},
				{http.MethodGet, "/radio/stations/" + id + "/now?since=0"},
				{http.MethodGet, "/radio/images/" + id + "/logo/w256"},
			} {
				rec := send(t, h, c.method, BasePath+c.path, token, "", nil)
				if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
					t.Errorf("%s %s %s: status %d (%s), want 404 not_found", token, c.method, c.path, rec.Code, rec.Body.String())
				}
			}
		}
		rec := send(t, h, http.MethodGet, fmt.Sprintf("%s/radio/stations/%d", BasePath, vistest.RadioBroken), token, "", nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"broken"`) {
			t.Errorf("%s broken station: %d %s", token, rec.Code, rec.Body.String())
		}
	}
	// P6's favourite list holds RadioDE; the favourite flag is the profile's own.
	rec := send(t, h, http.MethodGet, BasePath+"/radio/favorites", "p6", "", nil)
	if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"id":%d`, vistest.RadioDE)) {
		t.Errorf("P6 favourites = %s", rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, fmt.Sprintf("%s/radio/stations/%d", BasePath, vistest.RadioDE), "p12", "", nil)
	if !strings.Contains(rec.Body.String(), `"favorite":false`) {
		t.Errorf("P12 sees P6's favourite flag: %s", rec.Body.String())
	}
	// The administration sees the hidden station.
	rec = send(t, h, http.MethodGet, BasePath+"/admin/radio/stations/"+radioHidden, "p18", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hidden":true`) {
		t.Errorf("admin hidden station: %d %s", rec.Code, rec.Body.String())
	}
}

// The live routes need the listen token of their own listen session; an ended session answers
// 401 from the next request on.
func TestRadioLiveNeedsToken(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	a, playlistA := radioListen(t, h, "p6", vistest.RadioCH)
	b, playlistB := radioListen(t, h, "p6", vistest.RadioDE)
	if rec := sendLive(h, playlistA, nil); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != mediaTypeHLS {
		t.Fatalf("own playlist: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rtB := playlistB[strings.Index(playlistB, "rt="):]
	for name, path := range map[string]string{
		"no rt":          BasePath + "/radio/live/" + a + "/index.m3u8",
		"foreign rt":     BasePath + "/radio/live/" + a + "/index.m3u8?" + rtB,
		"forged rt":      BasePath + "/radio/live/" + a + "/index.m3u8?rt=" + radioDummyToken,
		"segment no rt":  BasePath + "/radio/live/" + b + "/seg/0",
		"unknown listen": BasePath + "/radio/live/" + radioDummyListen + "/init/0?rt=" + radioDummyToken,
	} {
		if rec := sendLive(h, path, nil); rec.Code != http.StatusUnauthorized || problemCode(t, rec) != "unauthenticated" {
			t.Errorf("%s: status %d (%s), want 401 unauthenticated", name, rec.Code, rec.Body.String())
		}
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/radio/listen/"+a, "p12", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("another profile ends the listen: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/radio/listen/"+a, "p6", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("end listen: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendLive(h, playlistA, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("playlist after the end: %d, want 401", rec.Code)
	}
	rec := send(t, h, http.MethodPost, BasePath+"/radio/listen/"+a+"/heartbeat", "p6", `{"playing":true}`, nil)
	if rec.Code != http.StatusGone || problemCode(t, rec) != "radio_unreachable" {
		t.Errorf("heartbeat of an ended relay: %d %s, want 410 radio_unreachable", rec.Code, rec.Body.String())
	}
	if rec := sendLive(h, playlistB, nil); rec.Code != http.StatusOK {
		t.Errorf("the other listen session ended too: %d", rec.Code)
	}
}

// The playlist is gzipped when the client takes it, the delta query reaches the service, and
// segments are immutable audio/mp4.
func TestRadioPlaylistGzipAndDelta(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	_, playlist := radioListen(t, h, "p0", vistest.RadioCH)
	rec := sendLive(h, playlist+"&_HLS_skip=YES", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("gzip playlist: %d %v", rec.Code, rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(body, fakeRadioPlaylist) {
		t.Errorf("unzipped playlist differs: %v", err)
	}
	if rec := sendLive(h, playlist, nil); rec.Header().Get("Content-Encoding") != "" {
		t.Error("gzip without Accept-Encoding")
	}
	pf.radio.live.mu.Lock()
	skips := slices.Clone(pf.radio.live.skips)
	pf.radio.live.mu.Unlock()
	if !slices.Equal(skips, []string{"YES", ""}) {
		t.Errorf("_HLS_skip values = %q, want [YES ]", skips)
	}
	seg := strings.Replace(playlist, "/index.m3u8?", "/seg/0?", 1)
	rec = sendLive(h, seg, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "audio/mp4" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("segment: %d %v", rec.Code, rec.Header())
	}
	if rec := sendLive(h, strings.Replace(playlist, "/index.m3u8?", "/seg/7?", 1), nil); rec.Code != http.StatusNotFound {
		t.Errorf("segment outside the window: %d", rec.Code)
	}
}

// A profile switch and a logout end the listen sessions of the browser session (3.7).
func TestRadioSelectProfileEndsListen(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	_, playlist := radioListen(t, h, "p18", vistest.RadioCH)
	rec := send(t, h, http.MethodPost, fmt.Sprintf("%s/auth/profiles/%d/select", BasePath, pf.f.P12.ID), "p18", `{}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("select profile: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendLive(h, playlist, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("playlist after the profile switch: %d, want 401", rec.Code)
	}
}

func TestRadioLogoutEndsListen(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	_, playlist := radioListen(t, h, "p16", vistest.RadioCH)
	if rec := send(t, h, http.MethodPost, BasePath+"/auth/logout", "p16", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendLive(h, playlist, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("playlist after the logout: %d, want 401", rec.Code)
	}
}

// Deleting a profile ends its listen sessions through the playback hook OnProfileEnd, without a
// radio call in the handler (m-3).
func TestRadioDeleteProfileEndsListen(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	row, err := appq.New(pf.f.DB.Write).InsertProfile(context.Background(), appq.InsertProfileParams{
		AccountID: pf.f.Account.ID, Name: "Radio Kind", MaxRating: 6, Language: "de", AllLibraries: 1, AudioLanguages: "[]",
		SubtitleMode: "forced", SubtitleLanguages: "[]", Now: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	pf.newSession(t, "radio-kind", model.Profile{ID: row.ID, AccountID: pf.f.Account.ID, Name: "Radio Kind", MaxRating: 6})
	_, playlist := radioListen(t, h, "radio-kind", vistest.RadioCH)
	if rec := sendLive(h, playlist, nil); rec.Code != http.StatusOK {
		t.Fatalf("playlist before: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, fmt.Sprintf("%s/profiles/%d", BasePath, row.ID), "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete profile: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendLive(h, playlist, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("playlist after the profile was deleted: %d, want 401", rec.Code)
	}
}

// The error codes of the radio contract through mapError: 403 watch_limit_reached with the
// minutes, 502 radio_unreachable, 422 radio_not_audio.
func TestRadioErrorCodes(t *testing.T) {
	t.Parallel()
	p := problems{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, tc := range []struct {
		err    error
		status int
		code   string
		ext    string
	}{
		{&model.WatchLimitError{LimitMinutes: 60, UsedMinutes: 61}, http.StatusForbidden, "watch_limit_reached", `"limitMinutes":60`},
		{model.ErrRadioUnreachable, http.StatusBadGateway, "radio_unreachable", ""},
		{model.ErrRadioNotAudio, http.StatusUnprocessableEntity, "radio_not_audio", ""},
		{&model.RadioCapacityError{Running: 3, Max: 3}, http.StatusConflict, "radio_capacity", `"running":3`},
	} {
		rec := httptest.NewRecorder()
		p.mapError(rec, httptest.NewRequest(http.MethodPost, "/api/v1/radio/stations/1/listen", nil), tc.err)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), `"code":"`+tc.code+`"`) || !strings.Contains(rec.Body.String(), tc.ext) {
			t.Errorf("%v: %d %s", tc.err, rec.Code, rec.Body.String())
		}
	}
}
