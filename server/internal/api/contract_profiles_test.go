package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/discover"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Child-profile contract test (plan 10.2) for every scoped operation of api/openapi.yaml. It runs the
// real catalog and playback services on the vistest fixture (movies rated 0, 6, 12, 16, 18 and one
// unrated) with the profiles P18, P6 and P0 and checks:
//   - negative: no response of a profile contains the id of a title above its rating, nor of the
//     unrated title (except for P18);
//   - positive: titles exactly on the profile's level appear (off-by-one), for P6 in every home row;
//   - ID endpoints: an invisible title answers 404 not_found, a visible one succeeds.
//
// The operations come from the contract, so a new scoped operation without an entry in
// profileRegistry fails the test.

// profileAuth resolves the tokens "p18", "p6" and "p0" to sessions of the fixture profiles.
type profileAuth struct{ sessions map[string]model.Session }

func (profileAuth) SetupRequired(context.Context) (bool, error) { return false, nil }
func (profileAuth) Login(context.Context, string, string, string) (model.Session, string, error) {
	return model.Session{}, "", model.ErrInvalidCredentials
}
func (profileAuth) Logout(context.Context, int64) error { return nil }
func (a profileAuth) Resolve(_ context.Context, token string) (model.Session, error) {
	if s, ok := a.sessions[token]; ok {
		return s, nil
	}
	return model.Session{}, model.ErrNotFound
}

// profileCall is one request of the registry. Path is relative to BasePath; {id} is replaced by
// the fixture item id when perItem is set.
type profileCall struct {
	contract string // "METHOD /path" exactly as in api/openapi.yaml
	path     string
	body     string
	header   map[string]string
	perItem  bool
	okStatus int // expected status for a visible item
	check    func(t *testing.T, f *vistest.Fixture, h http.Handler, token string, m vistest.Movie, rec *httptest.ResponseRecorder)
}

var profileRegistry = map[string]profileCall{
	"logout":     {contract: "POST /auth/logout", path: "/auth/logout"},
	"getSession": {contract: "GET /auth/session", path: "/auth/session"},
	"getHome":    {contract: "GET /home", path: "/home"},
	"getItem": {contract: "GET /items/{id}", path: "/items/{id}", perItem: true, okStatus: http.StatusOK,
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var d struct {
				ID         string `json:"id"`
				MediaFiles []struct {
					ID string `json:"id"`
				} `json:"mediaFiles"`
				Genres []string `json:"genres"`
			}
			decode(t, rec, &d)
			if d.ID != m.ID || len(d.MediaFiles) != 1 || d.MediaFiles[0].ID != m.FileID || d.Genres == nil {
				t.Errorf("getItem %s = %s", m.Label, rec.Body.String())
			}
		}},
	"getItemImage": {contract: "GET /items/{id}/images/{kind}/{size}", path: "/items/{id}/images/poster/w500", perItem: true, okStatus: http.StatusOK,
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			want, err := os.ReadFile(filepath.Join(f.CacheDir, filepath.FromSlash(m.PosterRel)))
			if err != nil {
				t.Fatal(err)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
				t.Errorf("image content type = %q", ct)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
				t.Errorf("image cache control = %q", cc)
			}
			if !bytes.Equal(rec.Body.Bytes(), want) {
				t.Errorf("image body differs from the stored file")
			}
		}},
	"startPlayback": {contract: "POST /items/{id}/play", path: "/items/{id}/play", perItem: true, okStatus: http.StatusOK,
		body: `{"deviceId":"contract-test"}`,
		check: func(t *testing.T, _ *vistest.Fixture, h http.Handler, token string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var info struct {
				Mode      string `json:"mode"`
				StreamURL string `json:"streamUrl"`
				MediaFile struct {
					ID       string `json:"id"`
					MimeType string `json:"mimeType"`
				} `json:"mediaFile"`
			}
			decode(t, rec, &info)
			if info.Mode != "direct" || info.MediaFile.ID != m.FileID || !strings.HasPrefix(info.MediaFile.MimeType, "video/mp4") {
				t.Errorf("startPlayback %s = %s", m.Label, rec.Body.String())
			}
			// The returned stream URL works as it is.
			stream := send(t, h, http.MethodGet, info.StreamURL, token, "", map[string]string{"Range": "bytes=10-19"})
			if stream.Code != http.StatusPartialContent || stream.Body.Len() != 10 {
				t.Errorf("stream URL %s: status %d, %d bytes", info.StreamURL, stream.Code, stream.Body.Len())
			}
		}},
	"getItemStream": {contract: "GET /items/{id}/stream", path: "/items/{id}/stream", perItem: true, okStatus: http.StatusPartialContent,
		header: map[string]string{"Range": "bytes=0-99"},
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			if cr := rec.Header().Get("Content-Range"); cr != fmt.Sprintf("bytes 0-99/%d", vistest.FileSize) {
				t.Errorf("Content-Range = %q", cr)
			}
			if rec.Header().Get("Accept-Ranges") != "bytes" || rec.Header().Get("Cache-Control") != "private, no-store" || rec.Header().Get("Content-Type") != "video/mp4" {
				t.Errorf("stream headers = %v", rec.Header())
			}
			if !bytes.Equal(rec.Body.Bytes(), m.Content[:100]) {
				t.Errorf("stream body differs from the first 100 bytes of the file")
			}
		}},
	"reportProgress": {contract: "POST /items/{id}/progress", path: "/items/{id}/progress", perItem: true, okStatus: http.StatusOK,
		body: fmt.Sprintf(`{"positionMs":%d,"durationMs":%d,"playedMs":10000,"seq":1,"state":"playing","clientTime":"2026-10-18T20:00:00Z"}`,
			vistest.DurationMs/10, vistest.DurationMs),
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var p struct {
				PositionMs int64 `json:"positionMs"`
				Completed  bool  `json:"completed"`
			}
			decode(t, rec, &p)
			if p.PositionMs != vistest.DurationMs/10 || p.Completed {
				t.Errorf("reportProgress %s = %s", m.Label, rec.Body.String())
			}
		}},
}

func send(t *testing.T, h http.Handler, method, path, token, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(CSRFHeader, CSRFHeaderValue)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("status %d, body %q: %v", rec.Code, rec.Body.String(), err)
	}
}

// itemIDsIn walks a JSON document and returns every fixture item id contained in a string value,
// also inside URLs and nested rows.
func itemIDsIn(t *testing.T, rec *httptest.ResponseRecorder, movies []vistest.Movie) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	if !strings.Contains(rec.Header().Get("Content-Type"), "json") || rec.Body.Len() == 0 {
		return found
	}
	var doc any
	decode(t, rec, &doc)
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			for _, c := range v {
				walk(c)
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		case string:
			for _, m := range movies {
				if strings.Contains(v, m.ID) {
					found[m.ID] = true
				}
			}
		}
	}
	walk(doc)
	return found
}

// scopedOperations reads api/openapi.yaml and returns operationId → "METHOD /path" of every
// scoped operation.
func scopedOperations(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	paths, _ := doc["paths"].(map[string]any)
	out := map[string]string{}
	for path, item := range paths {
		ops, _ := item.(map[string]any)
		for _, method := range httpMethods {
			op, ok := ops[method].(map[string]any)
			if !ok || op["x-visibility"] != "scoped" {
				continue
			}
			id, _ := op["operationId"].(string)
			if id == "" {
				t.Errorf("%s %s: scoped operation without operationId", method, path)
				continue
			}
			out[id] = strings.ToUpper(method) + " " + path
		}
	}
	if len(out) == 0 {
		t.Fatal("no scoped operations in api/openapi.yaml")
	}
	return out
}

type profileFixture struct {
	f      *vistest.Fixture
	h      http.Handler
	tokens map[string]model.Profile
}

func newProfileFixture(t *testing.T) profileFixture {
	t.Helper()
	f := vistest.New(t)
	log := vistest.Logger()
	sessions := map[string]model.Session{}
	tokens := map[string]model.Profile{"p18": f.P18, "p6": f.P6, "p0": f.P0}
	i := int64(0)
	for token, p := range tokens {
		i++
		sessions[token] = model.Session{ID: i, Account: f.Account, Profile: p, ExpiresAt: time.Now().Add(time.Hour)}
	}
	h := NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: log, Web: http.NotFoundHandler(),
		Auth:     profileAuth{sessions: sessions},
		Catalog:  discover.New(f.DB, f.CacheDir, log),
		Playback: playback.New(f.DB, log),
	})
	return profileFixture{f: f, h: h, tokens: tokens}
}

func homeRows(t *testing.T, rec *httptest.ResponseRecorder) (keys []string, rows map[string][]string) {
	t.Helper()
	var page struct {
		Rows []struct {
			Key   string `json:"key"`
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"rows"`
	}
	decode(t, rec, &page)
	rows = map[string][]string{}
	for _, r := range page.Rows {
		keys = append(keys, r.Key)
		for _, it := range r.Items {
			rows[r.Key] = append(rows[r.Key], it.ID)
		}
	}
	return keys, rows
}

func TestContractChildProfiles(t *testing.T) {
	ops := scopedOperations(t)
	for id, call := range profileRegistry {
		if contract, ok := ops[id]; !ok {
			t.Errorf("registry entry %s is not a scoped operation of the contract", id)
		} else if contract != call.contract {
			t.Errorf("registry entry %s targets %q, the contract says %q", id, call.contract, contract)
		}
	}
	names := make([]string, 0, len(ops))
	for id := range ops {
		names = append(names, id)
	}
	sort.Strings(names)

	pf := newProfileFixture(t)
	f, h := pf.f, pf.h

	// Positive checks on the start page come first, before reportProgress changes progress.
	t.Run("home", func(t *testing.T) {
		keys, rows := homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "p6", "", nil))
		if want := []string{"picks", "continue", "recent"}; !slices.Equal(keys, want) {
			t.Errorf("P6 rows = %v, want %v", keys, want)
		}
		on := f.Rated(6).ID
		for _, key := range []string{"picks", "continue", "recent"} {
			if !slices.Contains(rows[key], on) {
				t.Errorf("P6 row %s = %v lacks the 6-rated movie", key, rows[key])
			}
		}
		keys, rows = homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "p0", "", nil))
		if want := []string{"picks", "recent"}; !slices.Equal(keys, want) {
			t.Errorf("P0 rows = %v, want %v (empty continue row omitted)", keys, want)
		}
		if !slices.Contains(rows["recent"], f.Rated(0).ID) {
			t.Errorf("P0 recent = %v lacks the 0-rated movie", rows["recent"])
		}
		keys, rows = homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "p18", "", nil))
		if want := []string{"picks", "continue", "recent"}; !slices.Equal(keys, want) {
			t.Errorf("P18 rows = %v, want %v", keys, want)
		}
		for _, m := range []vistest.Movie{f.Rated(18), f.Unrated()} {
			if !slices.Contains(rows["recent"], m.ID) {
				t.Errorf("P18 recent = %v lacks movie %s", rows["recent"], m.Label)
			}
		}
		if len(rows["picks"]) == 0 || rows["picks"][0] != f.Rated(18).ID {
			t.Errorf("P18 picks = %v, want the 18-rated movie first", rows["picks"])
		}
	})

	for _, token := range []string{"p0", "p6", "p18"} {
		profile := pf.tokens[token]
		t.Run(profile.Name, func(t *testing.T) {
			for _, op := range names {
				call, ok := profileRegistry[op]
				if !ok {
					t.Errorf("scoped operation %s (%s) has no entry in profileRegistry", op, ops[op])
					continue
				}
				method := strings.Fields(call.contract)[0]
				if !call.perItem {
					rec := send(t, h, method, BasePath+call.path, token, call.body, call.header)
					if rec.Code >= 500 {
						t.Errorf("%s: status %d (%s)", op, rec.Code, rec.Body.String())
					}
					assertOnlyVisible(t, op, profile, f.Movies, itemIDsIn(t, rec, f.Movies))
					continue
				}
				for _, m := range f.Movies {
					rec := send(t, h, method, BasePath+strings.ReplaceAll(call.path, "{id}", m.ID), token, call.body, call.header)
					if !vistest.VisibleTo(profile, m) {
						if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
							t.Errorf("%s movie %s: status %d (%s), want 404 not_found", op, m.Label, rec.Code, rec.Body.String())
						}
						continue
					}
					if rec.Code != call.okStatus {
						t.Errorf("%s movie %s: status %d (%s), want %d", op, m.Label, rec.Code, rec.Body.String(), call.okStatus)
						continue
					}
					if call.check != nil {
						call.check(t, f, h, token, m, rec)
					}
					assertOnlyVisible(t, op, profile, f.Movies, itemIDsIn(t, rec, f.Movies))
				}
			}
		})
	}

	// None of the rejected progress reports left a raw event behind.
	for _, p := range []model.Profile{f.P0, f.P6} {
		for _, m := range f.Movies {
			if !vistest.VisibleTo(p, m) && countProgressEvents(t, f, p.ID, m.ID) != 0 {
				t.Errorf("%s stored a progress event for invisible movie %s", p.Name, m.Label)
			}
		}
	}
}

func assertOnlyVisible(t *testing.T, op string, p model.Profile, movies []vistest.Movie, found map[string]bool) {
	t.Helper()
	for _, m := range movies {
		if found[m.ID] && !vistest.VisibleTo(p, m) {
			t.Errorf("%s: response for %s contains movie %s", op, p.Name, m.Label)
		}
	}
}

func countProgressEvents(t *testing.T, f *vistest.Fixture, profileID int64, itemID string) int {
	t.Helper()
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress_events WHERE profile_id = ? AND item_id = ?`, profileID, itemID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestContractProgressCompletion(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m12 := f.Rated(12)
	body := fmt.Sprintf(`{"positionMs":%d,"durationMs":%d,"playedMs":10000,"seq":7,"state":"playing","clientTime":"2026-10-18T20:00:00Z"}`,
		vistest.DurationMs*95/100, vistest.DurationMs)

	rec := send(t, h, http.MethodPost, BasePath+"/items/"+m12.ID+"/progress", "p18", body, nil)
	var p struct {
		Completed bool    `json:"completed"`
		Percent   float64 `json:"percent"`
		UpdatedAt string  `json:"updatedAt"`
	}
	decode(t, rec, &p)
	if rec.Code != http.StatusOK || !p.Completed || p.Percent < 94.9 || p.UpdatedAt == "" {
		t.Errorf("P18 at 95 %%: status %d, %s", rec.Code, rec.Body.String())
	}
	// The completed title leaves the continue row of P18.
	_, rows := homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "p18", "", nil))
	if slices.Contains(rows["continue"], m12.ID) {
		t.Errorf("completed movie still in continue: %v", rows["continue"])
	}

	rec = send(t, h, http.MethodPost, BasePath+"/items/"+m12.ID+"/progress", "p6", body, nil)
	if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
		t.Errorf("P6 on the 12-rated movie: status %d (%s)", rec.Code, rec.Body.String())
	}
	if n := countProgressEvents(t, f, f.P6.ID, m12.ID); n != 0 {
		t.Errorf("P6 stored %d events for an invisible movie", n)
	}

	rec = send(t, h, http.MethodPost, BasePath+"/items/"+m12.ID+"/progress", "p18", `{"positionMs":1,"durationMs":0,"playedMs":0,"seq":1,"state":"stopped","clientTime":"2026-10-18T20:00:00Z"}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
		t.Errorf("invalid event: status %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestContractStreamAndImageEdges(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := f.Rated(6)
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/stream", "p6", "", map[string]string{"Range": "bytes=999999-"})
	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("unsatisfiable range: status %d", rec.Code)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/stream?file="+m.FileID, "p6", "", nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != vistest.FileSize {
		t.Errorf("whole file: status %d, %d bytes", rec.Code, rec.Body.Len())
	}
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/stream?file="+f.Rated(0).FileID, "p6", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("file of another title: status %d", rec.Code)
	}
	for _, path := range []string{"/images/poster/w9999", "/images/logo/w500", "/images/backdrop/w500"} {
		rec = send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+path, "p6", "", nil)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, "p6", "", nil)
	var d struct {
		Images struct {
			Poster struct {
				URL string `json:"url"`
			} `json:"poster"`
		} `json:"images"`
	}
	decode(t, rec, &d)
	if !strings.HasPrefix(d.Images.Poster.URL, BasePath+"/items/"+m.ID+"/images/poster/w500?v=") {
		t.Errorf("poster URL = %q", d.Images.Poster.URL)
	}
	if img := send(t, h, http.MethodGet, d.Images.Poster.URL, "p6", "", nil); img.Code != http.StatusOK {
		t.Errorf("poster URL from the detail: status %d", img.Code)
	}
}
