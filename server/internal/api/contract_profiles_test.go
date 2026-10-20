package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/discover"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Child-profile contract test (plan 10.2) for every scoped operation of api/openapi.yaml. It runs the
// real catalog and playback services on the vistest fixture (movies rated 0, 6, 12, 16, 18 and one
// unrated, plus the edge cases: a 0-rated movie in library X, one whose only file is missing and a
// soft-deleted one) with the profiles P18, P16, P12, P6, P0 and PX (only library 1) and checks:
//   - negative: no response of a profile contains the id of a title above its rating, nor of the
//     unrated title (except for P18), nor of the missing or deleted title, nor (for PX) of the
//     title in library X;
//   - positive: titles exactly on the profile's level appear (off-by-one), for P6 in every home row;
//   - ID endpoints: an invisible title answers 404 not_found, a visible one succeeds.
//
// The operations come from the contract, so a new scoped operation without an entry in
// profileRegistry fails the test.

// profileAuth is the real auth service with logical session names: the tokens "p18", "p16", "p12",
// "p6", "p0", "px" (profiles of the owner account), "none" (owner, no profile chosen), "pg" and
// "pg18" (profiles of the guest account) resolve to the plain tokens of sessions the fixture
// started; any other token is resolved as it is.
type profileAuth struct {
	*auth.Service
	tokens map[string]string
}

func (a profileAuth) Resolve(ctx context.Context, token string) (model.Session, bool, error) {
	if real, ok := a.tokens[token]; ok {
		token = real
	}
	return a.Service.Resolve(ctx, token)
}

// profileCall is one request of the registry. Path is relative to BasePath; {id} is replaced by
// the fixture item id when perItem is set. With mint set, the test first creates a playback session
// of that mode for the movie and the profile's browser session (bypassing startPlayback, so it
// exists for invisible titles too) and replaces {ps} with its id and {st} with its stream token;
// streamOnly requests then go without the cookie, authorised by the token alone.
type profileCall struct {
	contract        string // "METHOD /path" exactly as in api/openapi.yaml
	path            string
	body            string
	header          map[string]string
	perItem         bool
	okStatus        int // expected status for a visible item
	invisibleStatus int // expected status for an invisible item; 0 = 404 not_found
	mint            string
	streamOnly      bool
	// subtitle mints the playback with the text track {track} of the file selected for WebVTT.
	subtitle bool
	// want is the expected status of a call without {id} for a profile; nil = any status below 500.
	want  func(f *vistest.Fixture, p model.Profile) int
	check func(t *testing.T, f *vistest.Fixture, h http.Handler, token string, m vistest.Movie, rec *httptest.ResponseRecorder)
}

// fixtureProfile returns the profile behind a logical session name of profileAuth.
func fixtureProfile(f *vistest.Fixture, token string) model.Profile {
	return map[string]model.Profile{
		"p18": f.P18, "p16": f.P16, "p12": f.P12, "p6": f.P6, "p0": f.P0, "px": f.PX, "pg": f.PG, "pg18": f.PG18,
	}[token]
}

// personCredits returns the credits of the fixture person that p may see: the cast credit in the
// film rated 6 and the director credit of the film rated 18.
func personCredits(f *vistest.Fixture, p model.Profile) []string {
	var out []string
	for _, c := range []struct {
		m    vistest.Movie
		role string
	}{{f.Rated(18), "director"}, {f.Rated(6), "cast"}} {
		if f.VisibleTo(p, c.m) {
			out = append(out, c.m.ID+"/"+c.role)
		}
	}
	return out
}

var profileRegistry = map[string]profileCall{
	"getHome": {contract: "GET /home", path: "/home"},
	"getItem": {contract: "GET /items/{id}", path: "/items/{id}", perItem: true, okStatus: http.StatusOK,
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var d struct {
				ID         string `json:"id"`
				MediaFiles []struct {
					ID           string `json:"id"`
					VersionLabel string `json:"versionLabel"`
				} `json:"mediaFiles"`
				Genres       []string `json:"genres"`
				TrailerCount *int     `json:"trailerCount"`
			}
			decode(t, rec, &d)
			files := 1
			if m.ID == f.Rated(12).ID {
				files = 2 // the 720p version (B-5) after the 1080p primary file
			}
			if d.ID != m.ID || len(d.MediaFiles) != files || d.MediaFiles[0].ID != m.FileID || d.Genres == nil ||
				d.TrailerCount == nil || *d.TrailerCount != f.Trailers[m.ID] {
				t.Errorf("getItem %s = %s", m.Label, rec.Body.String())
			}
			for _, mf := range d.MediaFiles {
				if mf.VersionLabel == "" {
					t.Errorf("getItem %s: file %s without version label", m.Label, mf.ID)
				}
			}
		}},
	"getItemTrailers": {contract: "GET /items/{id}/trailers", path: "/items/{id}/trailers", perItem: true, okStatus: http.StatusOK,
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var list []struct {
				Lang      string `json:"lang"`
				YoutubeID string `json:"youtubeId"`
				EmbedURL  string `json:"embedUrl"`
			}
			decode(t, rec, &list)
			if len(list) != f.Trailers[m.ID] {
				t.Errorf("getItemTrailers %s: %d trailers, want %d", m.Label, len(list), f.Trailers[m.ID])
			}
			for _, tr := range list {
				if tr.EmbedURL != "https://www.youtube-nocookie.com/embed/"+tr.YoutubeID {
					t.Errorf("getItemTrailers %s: embedUrl %q", m.Label, tr.EmbedURL)
				}
			}
			if m.ID == f.Rated(6).ID && (len(list) != 2 || list[0].Lang != fixtureProfile(f, token).Language) {
				t.Errorf("getItemTrailers %s for %s: %s, want de and en with the profile language first", m.Label, token, rec.Body.String())
			}
		}},
	"getPerson": {contract: "GET /people/{id}", path: "/people/1",
		want: func(f *vistest.Fixture, p model.Profile) int {
			if len(personCredits(f, p)) == 0 {
				return http.StatusNotFound
			}
			return http.StatusOK
		},
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			var person struct {
				ID      int64  `json:"id"`
				Name    string `json:"name"`
				Credits []struct {
					Item struct {
						ID string `json:"id"`
					} `json:"item"`
					Role string `json:"role"`
				} `json:"credits"`
			}
			decode(t, rec, &person)
			var got []string
			for _, c := range person.Credits {
				got = append(got, c.Item.ID+"/"+c.Role)
			}
			if want := personCredits(f, fixtureProfile(f, token)); person.ID != f.Person.ID || person.Name != f.Person.Name || !slices.Equal(got, want) {
				t.Errorf("getPerson for %s = %s, want credits %v", token, rec.Body.String(), want)
			}
		}},
	"getItemImage": {contract: "GET /items/{id}/images/{kind}/{size}", path: "/items/{id}/images/poster/w400", perItem: true, okStatus: http.StatusOK,
		check: func(t *testing.T, f *vistest.Fixture, h http.Handler, token string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			// Both backdrop sizes answer for the same visible titles.
			for _, b := range []struct{ size, rel string }{{"w1280", m.BackdropRel}, {"w780", m.BackdropSmallRel}} {
				want, err := os.ReadFile(filepath.Join(f.CacheDir, filepath.FromSlash(b.rel)))
				if err != nil {
					t.Fatal(err)
				}
				got := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/images/backdrop/"+b.size, token, "", nil)
				if got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), want) {
					t.Errorf("backdrop %s of %s: status %d", b.size, m.Label, got.Code)
				}
			}
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
		body: `{"deviceId":"contract-test","capabilities":{"video":["h264"],"audio":["aac"],"hls":"mse"}}`,
		check: func(t *testing.T, _ *vistest.Fixture, h http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var info struct {
				Mode              string `json:"mode"`
				StreamURL         string `json:"streamUrl"`
				PlaybackSessionID string `json:"playbackSessionId"`
				MediaFile         struct {
					ID       string `json:"id"`
					MimeType string `json:"mimeType"`
				} `json:"mediaFile"`
			}
			decode(t, rec, &info)
			if info.Mode != "direct" || info.MediaFile.ID != m.FileID || !strings.HasPrefix(info.MediaFile.MimeType, "video/mp4") ||
				!strings.HasPrefix(info.StreamURL, BasePath+"/items/"+m.ID+"/stream?st="+info.PlaybackSessionID+".") {
				t.Errorf("startPlayback %s = %s", m.Label, rec.Body.String())
			}
			// The returned stream URL works as it is, without the cookie.
			stream := send(t, h, http.MethodGet, info.StreamURL, "", "", map[string]string{"Range": "bytes=10-19"})
			if stream.Code != http.StatusPartialContent || stream.Body.Len() != 10 {
				t.Errorf("stream URL %s: status %d, %d bytes", info.StreamURL, stream.Code, stream.Body.Len())
			}
		}},
	"getItemStream": {contract: "GET /items/{id}/stream", path: "/items/{id}/stream?st={st}", perItem: true, okStatus: http.StatusPartialContent,
		mint: "direct", streamOnly: true, header: map[string]string{"Range": "bytes=0-99"},
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
	"stopPlayback": {contract: "DELETE /playback/sessions/{id}", path: "/playback/sessions/{ps}", perItem: true,
		okStatus: http.StatusNoContent, invisibleStatus: http.StatusNoContent, mint: "remux"},
	"getPlaybackMaster": {contract: "GET /playback/sessions/{id}/master.m3u8", path: "/playback/sessions/{ps}/master.m3u8?st={st}",
		perItem: true, okStatus: http.StatusOK, mint: "transcode", streamOnly: true,
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.apple.mpegurl" || !strings.Contains(rec.Body.String(), "media.m3u8?st=") {
				t.Errorf("master: %s %q", ct, rec.Body.String())
			}
		}},
	"getPlaybackMedia": {contract: "GET /playback/sessions/{id}/media.m3u8", path: "/playback/sessions/{ps}/media.m3u8?st={st}",
		perItem: true, okStatus: http.StatusOK, mint: "remux", streamOnly: true,
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			if body := rec.Body.String(); !strings.Contains(body, "init.mp4?st=") || !strings.Contains(body, "seg/0?st=") {
				t.Errorf("media playlist %q", body)
			}
		}},
	"getPlaybackInit": {contract: "GET /playback/sessions/{id}/init.mp4", path: "/playback/sessions/{ps}/init.mp4?st={st}",
		perItem: true, okStatus: http.StatusOK, mint: "remux", streamOnly: true},
	"getPlaybackSegment": {contract: "GET /playback/sessions/{id}/seg/{n}", path: "/playback/sessions/{ps}/seg/3?st={st}",
		perItem: true, okStatus: http.StatusOK, mint: "transcode", streamOnly: true,
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			if rec.Header().Get("Content-Type") != "video/mp4" || !bytes.Equal(rec.Body.Bytes(), []byte{3}) {
				t.Errorf("segment 3: %v %x", rec.Header(), rec.Body.Bytes())
			}
		}},
	"getItemTracks": {contract: "GET /items/{id}/tracks", path: "/items/{id}/tracks", perItem: true, okStatus: http.StatusOK,
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var list struct {
				MediaFileID string `json:"mediaFileId"`
				Audio       []struct {
					ID      int64 `json:"id"`
					Index   int   `json:"index"`
					Default bool  `json:"default"`
				} `json:"audio"`
				Subtitles []struct {
					ID     int64  `json:"id"`
					Index  int    `json:"index"`
					Source string `json:"source"`
					Text   bool   `json:"text"`
				} `json:"subtitles"`
			}
			decode(t, rec, &list)
			if list.MediaFileID != m.FileID || len(list.Audio) != 2 || !list.Audio[0].Default || len(list.Subtitles) != 3 {
				t.Errorf("getItemTracks %s = %s", m.Label, rec.Body.String())
			}
			for _, s := range list.Subtitles {
				if s.Source == "sidecar" && (s.Index != -1 || !s.Text) {
					t.Errorf("getItemTracks %s sidecar = %+v", m.Label, s)
				}
			}
			// The sidecar's path never leaves the server.
			if strings.Contains(rec.Body.String(), ".srt") || strings.Contains(rec.Body.String(), "relPath") {
				t.Errorf("getItemTracks %s leaks the sidecar path: %s", m.Label, rec.Body.String())
			}
		}},
	"getPlaybackSubtitle": {contract: "GET /playback/sessions/{id}/subtitles/{track}.vtt", path: "/playback/sessions/{ps}/subtitles/{track}.vtt?st={st}",
		perItem: true, okStatus: http.StatusOK, mint: "direct", subtitle: true, streamOnly: true,
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			if ct := rec.Header().Get("Content-Type"); ct != "text/vtt; charset=utf-8" || rec.Header().Get("Cache-Control") != "private, no-store" ||
				!strings.HasPrefix(rec.Body.String(), "WEBVTT") {
				t.Errorf("subtitle of %s: %s %q", m.Label, ct, rec.Body.String())
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
	f        *vistest.Fixture
	h        http.Handler
	tokens   map[string]model.Profile
	sessions map[string]model.Session
	tc       *fakeTranscoder
	tracks   map[string]vistest.FileTracks // track ids by media file id (vistest.AddTracks)
	a        *auth.Service
	pa       profileAuth
	admin    *admin.Service
	pb       *playback.Service
	opts     Options
}

// testTokenKey is the stream-token key of the contract fixtures.
var testTokenKey = []byte("contract-test-stream-token-key!!")

func newProfileFixture(t *testing.T) profileFixture {
	t.Helper()
	return newProfileFixtureWithWeb(t, http.NotFoundHandler())
}

// newProfileFixtureWithWeb is newProfileFixture with web serving every path outside the API.
func newProfileFixtureWithWeb(t *testing.T, web http.Handler) profileFixture {
	t.Helper()
	f := vistest.New(t)
	f.AddEdgeCases(t)
	f.AddSeries(t)
	f.AddGuest(t)
	tracks := f.AddTracks(t)
	f.AddSegments(t, f.Show.E1.ID)
	log := vistest.Logger()
	a := auth.New(f.DB, log)
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	adm := admin.New(f.DB, a, &authTestLibraries{}, ring, &authTestTMDB{}, func(string) string { return "" }, log).
		WithMetadata(contractMetadata{}).WithLibraryMaintenance(contractLibraryMaintenance{})

	pa := profileAuth{Service: a, tokens: map[string]string{}}
	sessions := map[string]model.Session{}
	tokens := map[string]model.Profile{
		"p18": f.P18, "p16": f.P16, "p12": f.P12, "p6": f.P6, "p0": f.P0, "px": f.PX, "pg": f.PG, "pg18": f.PG18,
	}
	pf := profileFixture{f: f, tokens: tokens, sessions: sessions, tc: newFakeTranscoder(), a: a, pa: pa, admin: adm, tracks: tracks}
	for name, p := range tokens {
		pf.newSession(t, name, p)
	}
	pf.newSession(t, "none", model.Profile{})
	pf.pb = playback.New(f.DB, log, playback.WithTokenKey(testTokenKey), playback.WithTranscoder(pf.tc),
		playback.WithSubtitleConverter(fakeEmbeddedVTT, fakeSidecarVTT), playback.WithCacheDir(t.TempDir()))
	adm.WithStreams(pf.pb)
	pf.opts = Options{
		Version: "test", Started: time.Now(), Logger: log, Web: web,
		Auth: pa, Profiles: a, Access: a, Admin: adm,
		Catalog:  discover.New(f.DB, f.CacheDir, log),
		Playback: pf.pb,
	}
	pf.h = NewHandler(pf.opts)
	return pf
}

// handlerWithPlayback is the handler of the fixture with another playback service (a wrapper of
// pf.pb that observes or interleaves calls).
func (pf profileFixture) handlerWithPlayback(pb PlaybackService) http.Handler {
	o := pf.opts
	o.Playback = pb
	return NewHandler(o)
}

// newSession starts a browser session for the account of profile p (owner or guest account; a
// zero profile starts it without an active profile) and registers it under the logical name.
func (pf profileFixture) newSession(t *testing.T, name string, p model.Profile) model.Session {
	t.Helper()
	account := pf.f.Account
	if pf.f.Guest.ID != 0 && p.AccountID == pf.f.Guest.ID {
		account = pf.f.Guest
	}
	session, token, err := pf.a.StartSession(context.Background(), account, p, "contract-test")
	if err != nil {
		t.Fatal(err)
	}
	pf.pa.tokens[name] = token
	pf.sessions[name] = session
	return session
}

// mint creates a playback session of the given mode for movie m and the browser session of token,
// directly in the store, and returns its id and stream token.
func (pf profileFixture) mint(t *testing.T, token string, m vistest.Movie, mode string) (id, st string) {
	t.Helper()
	return pf.mintWith(t, token, m, mode, 0)
}

// mintWith is mint with a text subtitle track selected for WebVTT delivery (0 = none).
func (pf profileFixture) mintWith(t *testing.T, token string, m vistest.Movie, mode string, subtitle int64) (id, st string) {
	t.Helper()
	session := pf.sessions[token]
	id = ulid.Make().String()
	p := appq.InsertPlaybackSessionParams{
		ID: id, SessionID: session.ID, AccountID: session.Account.ID, ProfileID: session.Profile.ID,
		ItemID: m.ID, MediaFileID: m.FileID, Mode: mode, Now: time.Now().UnixMilli(), ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}
	if subtitle > 0 {
		p.SubtitleTrack = sql.NullInt64{Int64: subtitle, Valid: true}
		p.SubtitleDelivery = sql.NullString{String: model.SubtitleText, Valid: true}
	}
	err := appq.New(pf.f.DB.Write).InsertPlaybackSession(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return id, playback.SignToken(testTokenKey, id, m.ID)
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
	titles := f.All()        // everything a response must not leak
	playable := f.Playable() // titles with a file

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
		// Each level sees its own rating and not the next one; PX sees library 1 only. Titles that
		// are missing or deleted show up nowhere, although they carry progress and a top pick.
		for _, tc := range []struct {
			token     string
			see, hide vistest.Movie
		}{
			{"p12", f.Rated(12), f.Rated(16)},
			{"p16", f.Rated(16), f.Rated(18)},
			{"px", f.Rated(18), edgeCase(f, func(m vistest.Movie) bool { return m.OtherLibrary })},
			{"p18", edgeCase(f, func(m vistest.Movie) bool { return m.OtherLibrary }), edgeCase(f, func(m vistest.Movie) bool { return m.Missing })},
			{"p0", f.Rated(0), edgeCase(f, func(m vistest.Movie) bool { return m.Deleted })},
		} {
			_, rows := homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", tc.token, "", nil))
			if !slices.Contains(rows["recent"], tc.see.ID) {
				t.Errorf("%s recent = %v lacks movie %s", tc.token, rows["recent"], tc.see.Label)
			}
			for key, ids := range rows {
				if slices.Contains(ids, tc.hide.ID) {
					t.Errorf("%s row %s contains movie %s", tc.token, key, tc.hide.Label)
				}
			}
		}
		if keys, _ := homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "px", "", nil)); slices.Contains(keys, "continue") {
			t.Errorf("PX rows = %v: progress on invisible titles only, continue must be omitted", keys)
		}
	})

	for _, token := range []string{"p0", "p6", "p12", "p16", "p18", "px", "pg", "pg18"} {
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
					if call.want != nil {
						if want := call.want(f, profile); rec.Code != want {
							t.Errorf("%s: status %d (%s), want %d", op, rec.Code, rec.Body.String(), want)
						} else if want == http.StatusNotFound && problemCode(t, rec) != "not_found" {
							t.Errorf("%s: code %s, want not_found", op, problemCode(t, rec))
						}
					}
					if call.check != nil && rec.Code == http.StatusOK {
						call.check(t, f, h, token, vistest.Movie{}, rec)
					}
					assertOnlyVisible(t, f, op, profile, titles, itemIDsIn(t, rec, titles))
					continue
				}
				for _, m := range playable {
					path := strings.ReplaceAll(call.path, "{id}", m.ID)
					if call.mint != "" {
						var sub int64
						if call.subtitle {
							sub = pf.tracks[m.FileID].Text
						}
						ps, st := pf.mintWith(t, token, m, call.mint, sub)
						path = strings.NewReplacer("{ps}", ps, "{st}", url.QueryEscape(st), "{track}", fmt.Sprint(sub)).Replace(path)
					}
					cookie := token
					if call.streamOnly {
						cookie = ""
					}
					rec := send(t, h, method, BasePath+path, cookie, call.body, call.header)
					if !f.VisibleTo(profile, m) {
						switch call.invisibleStatus {
						case 0:
							if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
								t.Errorf("%s movie %s: status %d (%s), want 404 not_found", op, m.Label, rec.Code, rec.Body.String())
							}
						default:
							if rec.Code != call.invisibleStatus {
								t.Errorf("%s movie %s: status %d (%s), want %d", op, m.Label, rec.Code, rec.Body.String(), call.invisibleStatus)
							}
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
					assertOnlyVisible(t, f, op, profile, titles, itemIDsIn(t, rec, titles))
				}
			}
		})
	}

	// The series tree (AN-9): getItem decides per title; the containers have no file, so the
	// file-based operations above run on the episodes only.
	t.Run("series tree", func(t *testing.T) {
		for _, token := range []string{"p0", "p6", "p12", "p16", "p18", "px", "pg", "pg18"} {
			profile := pf.tokens[token]
			for _, m := range f.Show.All() {
				rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, token, "", nil)
				if want := f.VisibleTo(profile, m); want != (rec.Code == http.StatusOK) {
					t.Errorf("%s getItem %s %s: status %d (%s), visible want %v", profile.Name, m.Kind, m.Label, rec.Code, rec.Body.String(), want)
				}
				if rec.Code == http.StatusNotFound && problemCode(t, rec) != "not_found" {
					t.Errorf("%s getItem %s: code %s, want not_found", profile.Name, m.Label, problemCode(t, rec))
				}
				if rec.Code == http.StatusOK { // a 404 names the requested id in its instance only
					// seasons, episodes, extras, series, season and nextEpisode name visible titles only
					assertOnlyVisible(t, f, "getItem "+m.Label, profile, titles, itemIDsIn(t, rec, titles))
				}
				trailers := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/trailers", token, "", nil)
				if want := f.VisibleTo(profile, m); want != (trailers.Code == http.StatusOK) {
					t.Errorf("%s getItemTrailers %s %s: status %d, visible want %v", profile.Name, m.Kind, m.Label, trailers.Code, want)
				}
				img := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/images/poster/w400", token, "", nil)
				if want := f.VisibleTo(profile, m); want != (img.Code == http.StatusOK) {
					t.Errorf("%s poster of %s %s: status %d, visible want %v", profile.Name, m.Kind, m.Label, img.Code, want)
				}
			}
			home := send(t, h, http.MethodGet, BasePath+"/home", token, "", nil)
			assertOnlyVisible(t, f, "getHome", profile, titles, itemIDsIn(t, home, titles))
			if _, rows := homeRows(t, home); f.VisibleTo(profile, f.Show.Series) != slices.Contains(rows["recent"], f.Show.Series.ID) {
				t.Errorf("%s: series in recent = %v, visible want %v", profile.Name, slices.Contains(rows["recent"], f.Show.Series.ID), f.VisibleTo(profile, f.Show.Series))
			}
		}
	})

	// None of the rejected progress reports left a raw event behind.
	for _, p := range pf.tokens {
		for _, m := range titles {
			if !f.VisibleTo(p, m) && countProgressEvents(t, f, p.ID, m.ID) != 0 {
				t.Errorf("%s stored a progress event for invisible movie %s", p.Name, m.Label)
			}
		}
	}
}

// edgeCase returns the first fixture movie matching pick.
func edgeCase(f *vistest.Fixture, pick func(vistest.Movie) bool) vistest.Movie {
	for _, m := range f.Movies {
		if pick(m) {
			return m
		}
	}
	panic("contract test: edge case missing from the fixture")
}

func assertOnlyVisible(t *testing.T, f *vistest.Fixture, op string, p model.Profile, movies []vistest.Movie, found map[string]bool) {
	t.Helper()
	for _, m := range movies {
		if found[m.ID] && !f.VisibleTo(p, m) {
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
	_, st := pf.mint(t, "p6", m, "direct")
	stream := BasePath + "/items/" + m.ID + "/stream?st=" + url.QueryEscape(st)
	rec := send(t, h, http.MethodGet, stream, "", "", map[string]string{"Range": "bytes=999999-"})
	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("unsatisfiable range: status %d", rec.Code)
	}
	rec = send(t, h, http.MethodGet, stream, "", "", nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != vistest.FileSize {
		t.Errorf("whole file: status %d, %d bytes", rec.Code, rec.Body.Len())
	}
	// A token of another title does not open this one.
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+f.Rated(0).ID+"/stream?st="+url.QueryEscape(st), "", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("token of another title: status %d", rec.Code)
	}
	for _, path := range []string{"/images/poster/w9999", "/images/logo/w400", "/images/backdrop/w500", "/images/poster/w500", "/images/backdrop/w342", "/images/poster/w780", "/images/backdrop/w400", "/images/backdrop/original", "/images/poster/original"} {
		rec = send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+path, "p6", "", nil)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
	// The small backdrop of a title above the profile limit is as invisible as the title.
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+f.Rated(12).ID+"/images/backdrop/w780", "p6", "", nil)
	if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
		t.Errorf("w780 backdrop of a hidden title: status %d", rec.Code)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, "p6", "", nil)
	var d struct {
		Images struct {
			Poster struct {
				URL string `json:"url"`
			} `json:"poster"`
			BackdropSmall struct {
				URL   string `json:"url"`
				Width int    `json:"width"`
			} `json:"backdropSmall"`
		} `json:"images"`
	}
	decode(t, rec, &d)
	if !strings.HasPrefix(d.Images.BackdropSmall.URL, BasePath+"/items/"+m.ID+"/images/backdrop/w780?v=") || d.Images.BackdropSmall.Width != 8 {
		t.Errorf("backdropSmall = %+v", d.Images.BackdropSmall)
	}
	if img := send(t, h, http.MethodGet, d.Images.BackdropSmall.URL, "p6", "", nil); img.Code != http.StatusOK {
		t.Errorf("backdropSmall URL from the detail: status %d", img.Code)
	}
	if !strings.HasPrefix(d.Images.Poster.URL, BasePath+"/items/"+m.ID+"/images/poster/w400?v=") {
		t.Errorf("poster URL = %q", d.Images.Poster.URL)
	}
	if img := send(t, h, http.MethodGet, d.Images.Poster.URL, "p6", "", nil); img.Code != http.StatusOK {
		t.Errorf("poster URL from the detail: status %d", img.Code)
	}
}

// fakeEmbeddedVTT and fakeSidecarVTT stand for transcode.Runner's WebVTT conversion.
func fakeEmbeddedVTT(_ context.Context, path string, idx int) ([]byte, error) {
	return []byte(fmt.Sprintf("WEBVTT\n\nNOTE stream %d of %s\n", idx, filepath.Base(path))), nil
}

func fakeSidecarVTT(_ context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return append([]byte("WEBVTT\n\n"), data...), nil
}
