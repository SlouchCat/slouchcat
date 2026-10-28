package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
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
	"github.com/slouchcat/slouchcat/server/internal/integrations"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify"
	"github.com/slouchcat/slouchcat/server/internal/offline"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
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
	// download mints a prepared download of the movie on the device of the profile's browser
	// session (directly in the store, so it exists for invisible titles too) and replaces {dl}
	// with its id, {dt} with a download token of the profile and {track} with the text track
	// that travels along as WebVTT (M5).
	download bool
	// okFor overrides okStatus per title (createDownloads and getDownloadOptions refuse extras).
	okFor func(m vistest.Movie) int
	// invisibleFor overrides invisibleStatus per profile (deleteDownload: the primary profile).
	invisibleFor func(p model.Profile) int
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

// fixtureGenre is the genre every fixture title carries, in the profile language.
func fixtureGenre(p model.Profile) string {
	if p.Language == "en" {
		return "Adventure"
	}
	return "Abenteuer"
}

// mixedMembers returns the members of the mixed collection of vistest.AddDiscover that p may see,
// in collection order.
func mixedMembers(f *vistest.Fixture, p model.Profile) []string {
	var out []string
	for _, m := range []vistest.Movie{f.Rated(6), f.Rated(12), f.Rated(18)} {
		if f.VisibleTo(p, m) {
			out = append(out, m.ID)
		}
	}
	return out
}

// listableStatus is status for titles the watchlist takes and 422 not_listable for extras (seasons
// are containers and checked in the subtest "favourites").
func listableStatus(m vistest.Movie) int {
	if m.Kind == "extra" {
		return http.StatusUnprocessableEntity
	}
	return http.StatusNoContent
}

// searchResult is the part of SearchResult the checks read.
type searchResult struct {
	Total int `json:"total"`
	Items []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"items"`
	People []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"people"`
	Collections []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"collections"`
}

func (r searchResult) ids() []string {
	out := make([]string, 0, len(r.Items))
	for _, it := range r.Items {
		out = append(out, it.ID)
	}
	return out
}

// checkSearch (E-4, 10.2): "Fi" finds every visible film (their German titles "Film NN"), so each
// level finds the film exactly on its rating; "Pat" finds the person only for a profile that sees a
// film of hers; "Samm" finds the mixed collection only with a visible member, counting visible
// members only; a filter alone browses; one character, nothing at all and over 100 characters are
// refused.
func checkSearch(t *testing.T, f *vistest.Fixture, h http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
	p := fixtureProfile(f, token)
	var res searchResult
	decode(t, rec, &res)
	for _, m := range f.Movies {
		if f.VisibleTo(p, m) != slices.Contains(res.ids(), m.ID) {
			t.Errorf("search \"Fi\" for %s: film %s found %v, visible %v", token, m.Label, !f.VisibleTo(p, m), f.VisibleTo(p, m))
		}
	}
	if res.Total != len(res.Items) || len(res.People) != 0 || len(res.Collections) != 0 {
		t.Errorf("search \"Fi\" for %s = %s", token, rec.Body.String())
	}
	run := func(body string) searchResult {
		rec := send(t, h, http.MethodPost, BasePath+"/search", token, body, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("search %s for %s: status %d (%s)", body, token, rec.Code, rec.Body.String())
		}
		assertOnlyVisible(t, f, "search "+body, p, f.All(), itemIDsIn(t, rec, f.All()))
		var out searchResult
		decode(t, rec, &out)
		return out
	}
	people := run(`{"query":"Pat"}`).People
	if want := len(personCredits(f, p)) > 0; want != (len(people) == 1 && people[0].ID == f.Person.ID && people[0].Name == f.Person.Name) || (!want && len(people) != 0) {
		t.Errorf("search \"Pat\" for %s: people %+v, want the person %v", token, people, want)
	}
	collections := run(`{"query":"Samm"}`).Collections
	switch members := len(mixedMembers(f, p)); {
	case members == 0 && len(collections) != 0:
		t.Errorf("search \"Samm\" for %s: collections %+v, want none", token, collections)
	case members > 0 && (len(collections) != 1 || collections[0].ID != vistest.MixedCollectionID || collections[0].Count != members):
		t.Errorf("search \"Samm\" for %s: collections %+v, want the mixed one with %d members", token, collections, members)
	}
	// Browse by a filter alone: the genre of every fixture title finds every visible title of the
	// six searchable kinds.
	browse := run(`{"genre":"` + fixtureGenre(p) + `","limit":100}`)
	for _, m := range f.All() {
		if m.Kind == "season" || m.Kind == "extra" {
			continue
		}
		if f.VisibleTo(p, m) != slices.Contains(browse.ids(), m.ID) {
			t.Errorf("browse %s for %s: title %s found %v", fixtureGenre(p), token, m.Label, !f.VisibleTo(p, m))
		}
	}
	if len(browse.People) != 0 || len(browse.Collections) != 0 || browse.Total != len(browse.Items) {
		t.Errorf("browse for %s: %+v", token, browse)
	}
	// Combined filters: films up to FSK 12 from 2011 to 2014.
	combined := run(`{"query":"Fi","mediaTypes":["movie"],"maxRating":12,"yearFrom":2011,"yearTo":2014}`)
	var want []string
	for _, n := range []int{6, 12} {
		if f.VisibleTo(p, f.Rated(n)) {
			want = append(want, f.Rated(n).ID)
		}
	}
	if !slices.Equal(combined.ids(), want) {
		t.Errorf("combined filters for %s = %v, want %v", token, combined.ids(), want)
	}
	for body, code := range map[string]string{
		`{"query":"F"}`: "too_short", `{}`: "required", `{"query":"  "}`: "required",
		`{"query":"` + strings.Repeat("x", 101) + `"}`: "too_long",
	} {
		rec := send(t, h, http.MethodPost, BasePath+"/search", token, body, nil)
		var prob struct {
			Code   string `json:"code"`
			Errors []struct {
				Field string `json:"field"`
				Code  string `json:"code"`
			} `json:"errors"`
		}
		decode(t, rec, &prob)
		if rec.Code != http.StatusUnprocessableEntity || prob.Code != "validation_failed" || len(prob.Errors) != 1 ||
			prob.Errors[0].Field != "query" || prob.Errors[0].Code != code {
			t.Errorf("search %.30s for %s: status %d (%s), want 422 query %s", body, token, rec.Code, rec.Body.String(), code)
		}
	}
	// k7: the bounds of the filters (the strict server checks no schema bounds); yearFrom 0 is no
	// filter the reader would ignore but a 422.
	for body, want := range map[string]string{
		`{"maxRating":-5}`:                            "maxRating invalid_value",
		`{"maxRating":99}`:                            "maxRating invalid_value",
		`{"query":"Fi","maxRating":7}`:                "maxRating invalid_value",
		`{"yearFrom":0}`:                              "yearFrom invalid_value",
		`{"yearFrom":1869}`:                           "yearFrom invalid_value",
		`{"yearTo":2101}`:                             "yearTo invalid_value",
		`{"genre":"` + strings.Repeat("g", 81) + `"}`: "genre too_long",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPost, BasePath+"/search", token, body, nil)); got != want {
			t.Errorf("search %.30s for %s: %s, want %s", body, token, got, want)
		}
	}
	for _, body := range []string{`{"maxRating":0}`, `{"yearFrom":1870,"yearTo":2100}`, `{"genre":"` + strings.Repeat("g", 80) + `"}`} {
		if rec := send(t, h, http.MethodPost, BasePath+"/search", token, body, nil); rec.Code != http.StatusOK {
			t.Errorf("search %.30s for %s: status %d (%s), want 200", body, token, rec.Code, rec.Body.String())
		}
	}
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
			switch {
			case m.ID == f.Rated(12).ID:
				files = 2 // the 720p version (B-5) after the 1080p primary file
			case len(m.PartIDs) > 0:
				files = len(m.PartIDs) // the parts of an audiobook (M6)
			case m.FileID == "":
				files = 0 // a podcast episode with only its enclosure
			}
			if d.ID != m.ID || len(d.MediaFiles) != files || (files > 0 && d.MediaFiles[0].ID != m.FileID) || d.Genres == nil ||
				d.TrailerCount == nil || *d.TrailerCount != f.Trailers[m.ID] {
				t.Errorf("getItem %s = %s", m.Label, rec.Body.String())
			}
			for _, mf := range d.MediaFiles {
				if mf.VersionLabel == "" && !audioKind(m) { // audio files have no picture height
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
			if audioKind(m) && info.Mode != "direct" {
				return // a book of several parts plays as HLS (M6 stream A); the HLS operations cover it
			}
			mime, file := "video/mp4", m.FileID
			if audioKind(m) {
				mime = "" // m4b, mp3 and the enclosure's type
			}
			if m.Kind == "podcast_episode" {
				file = "" // an episode plays from its enclosure through the server
			}
			if info.Mode != "direct" || info.MediaFile.ID != file || !strings.HasPrefix(info.MediaFile.MimeType, mime) ||
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
			mime := rec.Header().Get("Content-Type")
			if audioKind(m) && (strings.HasPrefix(mime, "audio/") || mime == "application/octet-stream") {
				mime = "video/mp4" // the type of an audio file or the enclosure's
			}
			if rec.Header().Get("Accept-Ranges") != "bytes" || rec.Header().Get("Cache-Control") != "private, no-store" || mime != "video/mp4" {
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
	// M7 discover (design 3.1, plan 10.2): search, genres, collections, watchlist and favourites.
	// Positive checks name what each profile must find; the walk of every response proves that
	// nothing above the profile leaks, also through people and collections.
	"search": {contract: "POST /search", path: "/search", body: `{"query":"Fi"}`,
		want:  func(*vistest.Fixture, model.Profile) int { return http.StatusOK },
		check: checkSearch},
	"listGenres": {contract: "GET /genres", path: "/genres",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK },
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			p := fixtureProfile(f, token)
			var genres []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			}
			decode(t, rec, &genres)
			// Every fixture title carries Abenteuer/Adventure; the films, series, audiobooks and
			// podcasts the profile sees count.
			want := 0
			for _, m := range f.All() {
				if (m.Kind == "" || m.Kind == "series" || m.Kind == "audiobook" || m.Kind == "podcast") && f.VisibleTo(p, m) {
					want++
				}
			}
			if len(genres) != 1 || genres[0].Name != fixtureGenre(p) || genres[0].Count != want {
				t.Errorf("listGenres for %s = %s, want %s with %d titles", token, rec.Body.String(), fixtureGenre(p), want)
			}
		}},
	"getCollection": {contract: "GET /collections/{id}", path: "/collections/" + vistest.MixedCollectionID,
		want: func(f *vistest.Fixture, p model.Profile) int {
			if len(mixedMembers(f, p)) == 0 {
				return http.StatusNotFound // without a visible member the collection does not exist (10.2)
			}
			return http.StatusOK
		},
		check: func(t *testing.T, f *vistest.Fixture, h http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			p := fixtureProfile(f, token)
			var c struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Total int    `json:"total"`
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			}
			decode(t, rec, &c)
			var got []string
			for _, it := range c.Items {
				got = append(got, it.ID)
			}
			name := vistest.MixedCollectionDe
			if p.Language == "en" {
				name = vistest.MixedCollectionName
			}
			if want := mixedMembers(f, p); c.ID != vistest.MixedCollectionID || c.Name != name || c.Total != len(want) || !slices.Equal(got, want) {
				t.Errorf("getCollection for %s = %s, want members %v", token, rec.Body.String(), want)
			}
			// The manual collection holds only the film rated 18; an unknown id answers 404.
			manual := send(t, h, http.MethodGet, BasePath+"/collections/"+vistest.ManualCollectionID, token, "", nil)
			if want := f.VisibleTo(p, f.Rated(18)); want != (manual.Code == http.StatusOK) || (!want && manual.Code != http.StatusNotFound) {
				t.Errorf("manual collection for %s: status %d, visible want %v", token, manual.Code, want)
			}
			assertOnlyVisible(t, f, "getCollection manual", p, f.All(), itemIDsIn(t, manual, f.All()))
			if unknown := send(t, h, http.MethodGet, BasePath+"/collections/01J9000000000000000000CM00", token, "", nil); unknown.Code != http.StatusNotFound {
				t.Errorf("unknown collection for %s: status %d", token, unknown.Code)
			}
		}},
	"listWatchlist": {contract: "GET /watchlist", path: "/watchlist?limit=200",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK },
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			var page struct {
				Total int `json:"total"`
				Limit int `json:"limit"`
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			}
			decode(t, rec, &page)
			var got []string
			for _, it := range page.Items {
				got = append(got, it.ID)
			}
			// The fixture's entries (P6: the film rated 6, P18: the film rated 18) are there; the
			// operations above (addToWatchlist runs first) added the other visible titles.
			own := map[string]string{"p6": f.Rated(6).ID, "p18": f.Rated(18).ID}[token]
			if page.Total != len(got) || page.Limit != 200 || (own != "" && !slices.Contains(got, own)) {
				t.Errorf("listWatchlist for %s = %s", token, rec.Body.String())
			}
		}},
	"addToWatchlist": {contract: "PUT /items/{id}/watchlist", path: "/items/{id}/watchlist", perItem: true,
		okStatus: http.StatusNoContent, okFor: listableStatus},
	"removeFromWatchlist": {contract: "DELETE /items/{id}/watchlist", path: "/items/{id}/watchlist", perItem: true,
		okStatus: http.StatusNoContent},
	"listFavorites": {contract: "GET /favorites", path: "/favorites",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK },
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			var list []struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
			}
			decode(t, rec, &list)
			var got []string
			for _, it := range list {
				got = append(got, it.ID)
				if it.Kind != "series" && it.Kind != "podcast" {
					t.Errorf("listFavorites for %s: favourite of kind %s", token, it.Kind)
				}
			}
			if token == "p18" && !slices.Equal(got, []string{f.Show.Series.ID}) {
				t.Errorf("listFavorites for P18 = %v, want the series of the fixture", got)
			}
		}},
	// Playable titles are never favourites (422 not_favoritable); series and podcasts are checked
	// in the subtest "favourites".
	"addFavorite": {contract: "PUT /items/{id}/favorite", path: "/items/{id}/favorite", perItem: true,
		okStatus: http.StatusUnprocessableEntity,
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			if code := problemCode(t, rec); code != "not_favoritable" {
				t.Errorf("addFavorite %s: code %s, want not_favoritable", m.Label, code)
			}
		}},
	"removeFavorite": {contract: "DELETE /items/{id}/favorite", path: "/items/{id}/favorite", perItem: true,
		okStatus: http.StatusNoContent},
	// M7 inbox (E-8): the fixture has no entries; contract_notify_test.go covers the visibility of
	// the entries and the foreign-entry 404.
	"listNotifications": {contract: "GET /notifications", path: "/notifications",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK }},
	"markNotificationRead": {contract: "PUT /notifications/{id}/read", path: "/notifications/1/read",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusNotFound }},
	"markAllNotificationsRead": {contract: "POST /notifications/read-all", path: "/notifications/read-all",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusNoContent }},
	// M8 announcements of the active profile (TestContractAnnouncements checks them per profile).
	"listMyAnnouncements": {contract: "GET /announcements", path: "/announcements",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK }},
	"ackAnnouncement": {contract: "POST /announcements/{id}/ack", path: "/announcements/9999/ack",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusNotFound }},
	// M6: the paged episode list of a podcast. Every title of the loop is no podcast (404); the
	// podcasts themselves are containers and checked in the subtest "audio".
	"listPodcastEpisodes": {contract: "GET /items/{id}/episodes", path: "/items/{id}/episodes", perItem: true,
		okStatus: http.StatusOK, okFor: func(vistest.Movie) int { return http.StatusNotFound }},
	"getItemTracks": {contract: "GET /items/{id}/tracks", path: "/items/{id}/tracks", perItem: true, okStatus: http.StatusOK,
		okFor: fileStatus(http.StatusOK),
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
			if audioKind(m) { // the audio fixture files have no track rows
				if list.MediaFileID != m.FileID || len(list.Audio) != 0 || len(list.Subtitles) != 0 {
					t.Errorf("getItemTracks %s = %s", m.Label, rec.Body.String())
				}
				return
			}
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
		perItem: true, okStatus: http.StatusOK, mint: "direct", subtitle: true, streamOnly: true, okFor: videoStatus(http.StatusOK),
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			if ct := rec.Header().Get("Content-Type"); ct != "text/vtt; charset=utf-8" || rec.Header().Get("Cache-Control") != "private, no-store" ||
				!strings.HasPrefix(rec.Body.String(), "WEBVTT") {
				t.Errorf("subtitle of %s: %s %q", m.Label, ct, rec.Body.String())
			}
		}},
	// M5 offline (design section 2): options and creation per title, the prepared download through
	// its manifest and the download token, the device sync and the auto-download rule.
	"getDownloadOptions": {contract: "GET /items/{id}/download-options", path: "/items/{id}/download-options", perItem: true,
		okStatus: http.StatusOK, okFor: downloadableStatus(http.StatusOK),
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var o struct {
				MediaFileID string `json:"mediaFileId"`
				Qualities   []struct {
					Quality        string `json:"quality"`
					EstimatedBytes int64  `json:"estimatedBytes"`
					AudioBitrate   int    `json:"audioBitrate"`
					Width          int    `json:"width"`
				} `json:"qualities"`
				AudioTracks []struct{} `json:"audioTracks"`
				Subtitles   []struct {
					Text bool `json:"text"`
				} `json:"subtitles"`
			}
			decode(t, rec, &o)
			if audioKind(m) { // AAC 128/96/64 kbit/s, no track choice (M6)
				if o.MediaFileID != m.FileID || len(o.Qualities) != 3 || o.Qualities[0].AudioBitrate != 128 || o.Qualities[2].AudioBitrate != 64 ||
					o.Qualities[0].EstimatedBytes <= 0 || o.Qualities[0].Width != 0 || len(o.AudioTracks) != 0 || len(o.Subtitles) != 0 {
					t.Errorf("getDownloadOptions %s = %s", m.Label, rec.Body.String())
				}
				return
			}
			if o.MediaFileID != m.FileID || len(o.Qualities) != 3 || o.Qualities[0].Quality != "high" || o.Qualities[0].EstimatedBytes <= 0 ||
				len(o.AudioTracks) != 2 || len(o.Subtitles) != 2 || !o.Subtitles[0].Text {
				t.Errorf("getDownloadOptions %s = %s", m.Label, rec.Body.String())
			}
		}},
	"createDownloads": {contract: "POST /items/{id}/downloads", path: "/items/{id}/downloads", perItem: true, body: `{"quality":"low"}`,
		okStatus: http.StatusCreated, okFor: downloadableStatus(http.StatusCreated),
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var list struct {
				Downloads []struct {
					ID    string `json:"id"`
					State string `json:"state"`
					Item  struct {
						ID string `json:"id"`
					} `json:"item"`
				} `json:"downloads"`
			}
			decode(t, rec, &list)
			if len(list.Downloads) != 1 || list.Downloads[0].Item.ID != m.ID || list.Downloads[0].State != "queued" {
				t.Errorf("createDownloads %s = %s", m.Label, rec.Body.String())
			}
		}},
	// A profile deletes downloads it may see or requested (the minted download of an invisible
	// title has no requesting profile); the primary profile looks after every download of the
	// account. Anything else is 404 like an invisible title (M5 review K1).
	"deleteDownload": {contract: "DELETE /downloads/{id}", path: "/downloads/{dl}", perItem: true, download: true,
		okStatus: http.StatusNoContent, invisibleFor: func(p model.Profile) int {
			if p.IsPrimary {
				return http.StatusNoContent
			}
			return http.StatusNotFound
		}},
	"getDownloadManifest": {contract: "GET /downloads/{id}/manifest", path: "/downloads/{dl}/manifest", perItem: true, download: true,
		okStatus: http.StatusOK,
		check: func(t *testing.T, _ *vistest.Fixture, h http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			var man struct {
				ItemID     string     `json:"itemId"`
				MimeType   string     `json:"mimeType"`
				Chapters   []struct{} `json:"chapters"`
				SizeBytes  int64      `json:"sizeBytes"`
				InitLength int64      `json:"initLength"`
				FileURL    string     `json:"fileUrl"`
				Segments   []struct {
					Offset int64 `json:"offset"`
					Length int64 `json:"length"`
				} `json:"segments"`
				Subtitles []struct {
					URL string `json:"url"`
				} `json:"subtitles"`
			}
			decode(t, rec, &man)
			media := mintedMedia(m)
			subs, mime := 1, "video/mp4" // audio titles carry no subtitles (M6)
			if audioKind(m) {
				subs, mime = 0, "audio/mp4"
			}
			if man.ItemID != m.ID || man.SizeBytes != int64(len(media)) || man.InitLength != int64(len(mintedInit)) || len(man.Segments) != 2 ||
				len(man.Subtitles) != subs || man.MimeType != mime || man.Chapters == nil || !strings.Contains(man.FileURL, "/file?dt=") {
				t.Errorf("getDownloadManifest %s = %s", m.Label, rec.Body.String())
				return
			}
			// The URLs work as they are, without the cookie.
			if file := send(t, h, http.MethodGet, man.FileURL, "", "", nil); file.Code != http.StatusOK || !bytes.Equal(file.Body.Bytes(), media) {
				t.Errorf("file URL of %s: status %d", m.Label, file.Code)
			}
			if subs == 0 {
				return
			}
			if sub := send(t, h, http.MethodGet, man.Subtitles[0].URL, "", "", nil); sub.Code != http.StatusOK || !strings.HasPrefix(sub.Body.String(), "WEBVTT") {
				t.Errorf("subtitle URL of %s: status %d", m.Label, sub.Code)
			}
		}},
	"getDownloadFile": {contract: "GET /downloads/{id}/file", path: "/downloads/{dl}/file?dt={dt}", perItem: true, download: true,
		streamOnly: true, okStatus: http.StatusPartialContent, header: map[string]string{"Range": "bytes=0-3"},
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			media := mintedMedia(m)
			if cr := rec.Header().Get("Content-Range"); cr != fmt.Sprintf("bytes 0-3/%d", len(media)) {
				t.Errorf("Content-Range = %q", cr)
			}
			mime := "video/mp4"
			if audioKind(m) {
				mime = "audio/mp4"
			}
			if rec.Header().Get("Accept-Ranges") != "bytes" || rec.Header().Get("Cache-Control") != "private, no-store" ||
				rec.Header().Get("Content-Type") != mime || !bytes.Equal(rec.Body.Bytes(), media[:4]) {
				t.Errorf("download file of %s: %v %q", m.Label, rec.Header(), rec.Body.Bytes())
			}
		}},
	"getDownloadSubtitle": {contract: "GET /downloads/{id}/subtitles/{track}.vtt", path: "/downloads/{dl}/subtitles/{track}.vtt?dt={dt}",
		perItem: true, download: true, streamOnly: true, okStatus: http.StatusOK, okFor: videoStatus(http.StatusOK),
		check: func(t *testing.T, _ *vistest.Fixture, _ http.Handler, _ string, m vistest.Movie, rec *httptest.ResponseRecorder) {
			if ct := rec.Header().Get("Content-Type"); ct != "text/vtt; charset=utf-8" || rec.Header().Get("Cache-Control") != "private, no-store" ||
				rec.Body.String() != mintedVTT {
				t.Errorf("download subtitle of %s: %s %q", m.Label, ct, rec.Body.String())
			}
		}},
	"syncOffline": {contract: "POST /offline/sync", path: "/offline/sync", body: `{"held":[],"network":"wifi"}`,
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK },
		check: func(t *testing.T, f *vistest.Fixture, _ http.Handler, token string, _ vistest.Movie, rec *httptest.ResponseRecorder) {
			var s struct {
				Downloads []struct {
					AllowedProfileIDs []int64 `json:"allowedProfileIds"`
					Item              *struct {
						ID string `json:"id"`
					} `json:"item"`
				} `json:"downloads"`
				Revoked  []string `json:"revoked"`
				Profiles []struct {
					ID int64 `json:"id"`
				} `json:"profiles"`
				Rules struct {
					MaxCount int `json:"maxCount"`
				} `json:"rules"`
			}
			decode(t, rec, &s)
			p := fixtureProfile(f, token)
			active := false
			for _, op := range s.Profiles {
				active = active || op.ID == p.ID
			}
			if s.Revoked == nil || s.Rules.MaxCount == 0 || !active {
				t.Errorf("syncOffline for %s = %s", p.Name, rec.Body.String())
			}
			// The title travels only for the downloads the active profile may play.
			for _, d := range s.Downloads {
				if (d.Item != nil) != slices.Contains(d.AllowedProfileIDs, p.ID) {
					t.Errorf("syncOffline for %s: item %v with allowed %v", p.Name, d.Item, d.AllowedProfileIDs)
				}
			}
		}},
	"getDownloadRules": {contract: "GET /offline/rules", path: "/offline/rules",
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK }},
	"setDownloadRules": {contract: "PUT /offline/rules", path: "/offline/rules",
		body: `{"enabled":true,"nextEpisodes":true,"newPodcastEpisodes":false,"wifiOnly":true,"maxCount":2,"quality":"low"}`,
		want: func(*vistest.Fixture, model.Profile) int { return http.StatusOK }},
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
	off      *offline.Service // M5 offline service (fake encoder, no worker)
	dlDir    string           // its download directory
	queue    *jobs.Queue      // its job queue
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

	notifySvc *notify.Service  // M8 channels and announcements (fake senders)
	senders   *contractSenders // what they sent

	integrations *integrations.Service // M9 webhook receiver
}

// contractPathScanner stands in for package library: the contract never queues a real scan.
type contractPathScanner struct{}

func (contractPathScanner) RequestPathScan(context.Context, int64, int64, []string) error { return nil }

// testTokenKey is the stream-token key of the contract fixtures.
var testTokenKey = []byte("contract-test-stream-token-key!!")

// testDownloadKey is the download-token key of the contract fixtures (M5).
var testDownloadKey = []byte("contract-test-download-token-key")

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
	f.AddSecondAdmin(t)
	tracks := f.AddTracks(t)
	f.AddSegments(t, f.Show.E1.ID)
	f.AddAudio(t)
	f.AddDiscover(t)
	log := vistest.Logger()
	a := auth.New(f.DB, log)
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	pods := metadata.New(f.DB, jobs.NewQueue(f.DB.Write, nil), nil, t.TempDir(), transcode.Runner{}, log)
	adm := admin.New(f.DB, a, &authTestLibraries{}, ring, &authTestTMDB{}, func(string) string { return "" }, log).
		WithMetadata(contractMetadata{}).WithLibraryMaintenance(contractLibraryMaintenance{}).WithPodcasts(pods).WithTrakt(pods).
		WithBackups(t.TempDir(), "test")

	pa := profileAuth{Service: a, tokens: map[string]string{}}
	sessions := map[string]model.Session{}
	tokens := map[string]model.Profile{
		"p18": f.P18, "p16": f.P16, "p12": f.P12, "p6": f.P6, "p0": f.P0, "px": f.PX, "pg": f.PG, "pg18": f.PG18,
		"pa2": f.PA2, "pa2k": f.PA2K,
	}
	pf := profileFixture{f: f, tokens: tokens, sessions: sessions, tc: newFakeTranscoder(), a: a, pa: pa, admin: adm, tracks: tracks}
	for name, p := range tokens {
		pf.newSession(t, name, p)
	}
	pf.newSession(t, "none", model.Profile{})
	pf.pb = playback.New(f.DB, log, playback.WithTokenKey(testTokenKey), playback.WithTranscoder(pf.tc),
		playback.WithSubtitleConverter(fakeEmbeddedVTT, fakeSidecarVTT), playback.WithCacheDir(t.TempDir()))
	adm.WithStreams(pf.pb)
	pf.dlDir, pf.queue = filepath.Join(t.TempDir(), "downloads"), jobs.NewQueue(f.DB.Write, nil)
	// M8 administration: the queue of the fixture, a mock release manifest and a fake VAPID rotation.
	adm.WithJobs(pf.queue).WithPushRotator(contractPushRotator{adm}).WithUpdateCheck(contractManifest(t), nil).
		WithSystem(admin.SystemSource{Version: "test", StartedAt: time.Now(), ConfigDir: t.TempDir(), CacheDir: f.CacheDir})
	pf.off = offline.New(f.DB, visibility.New(f.DB, log), pf.queue, transcode.Runner{}, log, offline.WithDir(pf.dlDir),
		offline.WithTokenKey(testDownloadKey), offline.WithEncoder(contractEncoder{}), offline.WithPins(a), offline.WithPlayback(pf.pb))
	pf.notifySvc, pf.senders = newContractNotify(f, log, pf.queue, a.AllowAttempt)
	adm.WithAnnouncements(pf.notifySvc)
	// M9: the webhook receiver; the fixture has no integration, so every token is unknown (401).
	pf.integrations = integrations.New(f.DB, contractPathScanner{}, log)
	adm.WithIntegrations(pf.integrations)
	pf.opts = Options{
		Version: "test", Started: time.Now(), Logger: log, Web: web,
		Auth: pa, Profiles: a, Access: a, Admin: adm,
		Catalog:  discover.New(f.DB, f.CacheDir, log),
		Playback: pf.pb,
		Offline:  pf.off,
		Notify:   pf.notifySvc,

		Integrations: pf.integrations,
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

// newSession starts a browser session for the account of profile p (the owner, the second
// administrator or the account without administration; a zero profile starts it without an active
// profile) and registers it under the logical name.
func (pf profileFixture) newSession(t *testing.T, name string, p model.Profile) model.Session {
	t.Helper()
	account := pf.f.Account
	switch {
	case pf.f.Guest.ID != 0 && p.AccountID == pf.f.Guest.ID:
		account = pf.f.Guest
	case pf.f.Admin2.ID != 0 && p.AccountID == pf.f.Admin2.ID:
		account = pf.f.Admin2
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

// audioKind reports whether a fixture title is an audiobook or a podcast episode (AddAudio, M6).
func audioKind(m vistest.Movie) bool { return m.Kind == "audiobook" || m.Kind == "podcast_episode" }

// fileStatus is status for titles with a media file and 404 for the podcast episode that has only
// an enclosure (M6): operations on the file of a title find none.
func fileStatus(status int) func(m vistest.Movie) int {
	return func(m vistest.Movie) int {
		if m.FileID == "" {
			return http.StatusNotFound
		}
		return status
	}
}

// videoStatus is status for video titles and 404 for audio titles, whose fixture files carry no
// subtitle track (M6).
func videoStatus(status int) func(m vistest.Movie) int {
	return func(m vistest.Movie) int {
		if audioKind(m) {
			return http.StatusNotFound
		}
		return status
	}
}

// downloadableStatus is status for movies and episodes and 422 not_downloadable for extras.
func downloadableStatus(status int) func(m vistest.Movie) int {
	return func(m vistest.Movie) int {
		if m.Kind == "extra" {
			return http.StatusUnprocessableEntity
		}
		return status
	}
}

// Content of the prepared downloads mintDownload writes: an init segment, two media segments with
// the title's id and a WebVTT file of the text track.
const (
	mintedInit = "ftyp-moov"
	mintedVTT  = "WEBVTT\n\nNOTE minted download\n"
)

func mintedMedia(m vistest.Movie) []byte {
	return []byte(mintedInit + "segment-0-" + m.Label + "segment-1-" + m.Label)
}

// mintDownload stores a prepared download of movie m on the device of the browser session of
// token, directly in the store and the download directory, and returns its id and a download
// token of the session's profile. A download of the device for the title is reused. The session's
// profile is the requesting profile only of a title it sees (it could not request another one).
func (pf profileFixture) mintDownload(t *testing.T, token string, m vistest.Movie) (id, dt string) {
	t.Helper()
	session := pf.sessions[token]
	media := mintedMedia(m)
	seg0 := int64(len("segment-0-" + m.Label))
	requester := sql.NullInt64{Int64: session.Profile.ID, Valid: pf.f.VisibleTo(session.Profile, m)}
	err := pf.f.DB.Write.QueryRow(`INSERT INTO downloads (id, account_id, profile_id, device_id, item_id, media_file_id, source_url, quality,
	            audio_stream, state, mode, duration_ms, estimated_bytes, size_bytes, segments_done, segments_total, created_at, updated_at, ready_at)
	        VALUES (?, ?, ?, ?, ?, ?, ?, 'low', 1, 'ready', 'transcode', 8000, ?, ?, 2, 2, 1, 1, 1)
	        ON CONFLICT (device_id, item_id) DO UPDATE SET state = 'ready', files_removed_at = NULL, size_bytes = excluded.size_bytes,
	            segments_done = 2, segments_total = 2, audio_stream = 1, ready_at = 1, profile_id = excluded.profile_id
	        RETURNING id`,
		ulid.Make().String(), session.Account.ID, requester, session.DeviceID, m.ID, sql.NullString{String: m.FileID, Valid: m.FileID != ""},
		sql.NullString{String: m.Enclosure, Valid: m.FileID == ""}, len(media), len(media)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	index := fmt.Sprintf(`{"init":%d,"codecs":"avc1.640029,mp4a.40.2","width":852,"height":480}`+"\n"+
		`{"n":0,"startMs":0,"durationMs":4000,"offset":%d,"length":%d}`+"\n"+
		`{"n":1,"startMs":4000,"durationMs":4000,"offset":%d,"length":%d}`+"\n",
		len(mintedInit), len(mintedInit), seg0, int64(len(mintedInit))+seg0, int64(len(media))-int64(len(mintedInit))-seg0)
	dir := filepath.Join(pf.dlDir, id)
	for name, data := range map[string]string{
		"media.mp4": string(media), "index.jsonl": index, fmt.Sprintf("subs/%d.vtt", pf.tracks[m.FileID].Text): mintedVTT,
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return id, offline.SignToken(testDownloadKey, id, m.ID, session.Profile.ID)
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

// homeRowJSON is one row of the start page with the M7 lists.
type homeRowJSON struct {
	Key   string `json:"key"`
	Items []struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	} `json:"items"`
	Collections []struct {
		ID    string `json:"id"`
		Count int    `json:"count"`
		Cover *struct {
			ID string `json:"id"`
		} `json:"cover"`
	} `json:"collections"`
	Genres []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"genres"`
}

type homePageJSON struct {
	Rows []homeRowJSON `json:"rows"`
}

func (p homePageJSON) row(key string) homeRowJSON {
	for _, r := range p.Rows {
		if r.Key == key {
			return r
		}
	}
	return homeRowJSON{}
}

func homePage(t *testing.T, rec *httptest.ResponseRecorder) homePageJSON {
	t.Helper()
	var page homePageJSON
	decode(t, rec, &page)
	return page
}

func TestContractChildProfiles(t *testing.T) {
	t.Parallel()
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

	// Positive checks on the start page come first, before reportProgress changes progress and the
	// watchlist operations change the watchlists.
	t.Run("home", func(t *testing.T) {
		all := []string{"picks", "continue", "recent", "watchlist", "collections", "genres"}
		keys, rows := homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "p6", "", nil))
		if !slices.Equal(keys, all) {
			t.Errorf("P6 rows = %v, want %v", keys, all)
		}
		on := f.Rated(6).ID
		for _, key := range []string{"picks", "continue", "recent", "watchlist"} {
			if !slices.Contains(rows[key], on) {
				t.Errorf("P6 row %s = %v lacks the 6-rated movie", key, rows[key])
			}
		}
		// P0 sees one pick (the film rated 0 in library X; no placeholder since M7), has no
		// progress, no watchlist and no member of a collection (E-9: no empty rows).
		other := edgeCase(f, func(m vistest.Movie) bool { return m.OtherLibrary })
		keys, rows = homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "p0", "", nil))
		if want := []string{"picks", "recent", "genres"}; !slices.Equal(keys, want) || !slices.Equal(rows["picks"], []string{other.ID}) {
			t.Errorf("P0 rows = %v (picks %v), want %v with the film of library X", keys, rows["picks"], want)
		}
		if !slices.Contains(rows["recent"], f.Rated(0).ID) {
			t.Errorf("P0 recent = %v lacks the 0-rated movie", rows["recent"])
		}
		keys, rows = homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", "p18", "", nil))
		if !slices.Equal(keys, all) {
			t.Errorf("P18 rows = %v, want %v", keys, all)
		}
		for _, m := range []vistest.Movie{f.Rated(18), f.Unrated()} {
			if !slices.Contains(rows["recent"], m.ID) {
				t.Errorf("P18 recent = %v lacks movie %s", rows["recent"], m.Label)
			}
		}
		if len(rows["picks"]) == 0 || rows["picks"][0] != f.Rated(18).ID {
			t.Errorf("P18 picks = %v, want the 18-rated movie first", rows["picks"])
		}
		if !slices.Equal(rows["watchlist"], []string{f.Rated(18).ID}) {
			t.Errorf("P18 watchlist row = %v", rows["watchlist"])
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
		// E-1, E-3: the pick of P12 carries its note; the global FSK-18 pick never reaches P12.
		p12 := homePage(t, send(t, h, http.MethodGet, BasePath+"/home", "p12", "", nil))
		picks := p12.row("picks").Items
		if len(picks) != 3 || picks[0].ID != f.Rated(6).ID || picks[1].ID != other.ID || picks[2].ID != f.Rated(12).ID ||
			picks[2].Note != f.Discover.PickNote || picks[0].Note != "" {
			t.Errorf("P12 picks = %+v, want the films rated 6, X0 and 12, the last with the note", picks)
		}
		// E-6: the collections row counts visible members only, with a visible cover.
		for _, tc := range []struct {
			token string
			want  map[string]int
		}{
			{"p6", map[string]int{vistest.MixedCollectionID: 1}},
			{"p12", map[string]int{vistest.MixedCollectionID: 2}},
			{"p18", map[string]int{vistest.MixedCollectionID: 3, vistest.ManualCollectionID: 1}},
		} {
			page := homePage(t, send(t, h, http.MethodGet, BasePath+"/home", tc.token, "", nil))
			got := map[string]int{}
			for _, c := range page.row("collections").Collections {
				got[c.ID] = c.Count
				if c.Cover == nil || c.Cover.ID == "" {
					t.Errorf("%s collection %s without cover", tc.token, c.ID)
				}
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("%s collections row = %v, want %v", tc.token, got, tc.want)
			}
			if g := page.row("genres").Genres; len(g) != 1 || g[0].Name != fixtureGenre(pf.tokens[tc.token]) || len(page.row("genres").Items) != 0 {
				t.Errorf("%s genres row = %+v", tc.token, page.row("genres"))
			}
		}
		// E-2 with a fake clock: the global pick of the film rated 0 exists only inside its windows.
		for _, tc := range []struct {
			at   int64
			want bool
		}{
			{vistest.ExpiredPickEnd - 1, true}, {vistest.ExpiredPickEnd, false},
			{vistest.FuturePickStart - 1, false}, {vistest.FuturePickStart, true},
		} {
			o := pf.opts
			at := time.UnixMilli(tc.at)
			o.Catalog = discover.New(f.DB, f.CacheDir, vistest.Logger(), discover.WithClock(func() time.Time { return at }))
			_, rows := homeRows(t, send(t, NewHandler(o), http.MethodGet, BasePath+"/home", "p0", "", nil))
			if got := slices.Contains(rows["picks"], f.Rated(0).ID); got != tc.want {
				t.Errorf("P0 picks at %d = %v, want the film rated 0 %v", tc.at, rows["picks"], tc.want)
			}
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
					// An operation of the M8 base answers 501 until its lane lands; its entry expects it.
					stub := rec.Code == http.StatusNotImplemented && call.want != nil && call.want(f, profile) == http.StatusNotImplemented
					if rec.Code >= 500 && !stub {
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
					if call.download {
						dl, dt := pf.mintDownload(t, token, m)
						path = strings.NewReplacer("{dl}", dl, "{dt}", url.QueryEscape(dt), "{track}", fmt.Sprint(pf.tracks[m.FileID].Text)).Replace(path)
					}
					cookie := token
					if call.streamOnly {
						cookie = ""
					}
					rec := send(t, h, method, BasePath+path, cookie, call.body, call.header)
					if !f.VisibleTo(profile, m) {
						want := call.invisibleStatus
						if call.invisibleFor != nil {
							want = call.invisibleFor(profile)
						}
						switch want {
						case 0:
							if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
								t.Errorf("%s movie %s: status %d (%s), want 404 not_found", op, m.Label, rec.Code, rec.Body.String())
							}
						default:
							if rec.Code != want {
								t.Errorf("%s movie %s: status %d (%s), want %d", op, m.Label, rec.Code, rec.Body.String(), want)
							}
						}
						continue
					}
					okStatus := call.okStatus
					if call.okFor != nil {
						okStatus = call.okFor(m)
					}
					if rec.Code != okStatus {
						t.Errorf("%s movie %s: status %d (%s), want %d", op, m.Label, rec.Code, rec.Body.String(), okStatus)
						continue
					}
					if call.check != nil && okStatus == call.okStatus {
						call.check(t, f, h, token, m, rec)
					}
					assertOnlyVisible(t, f, op, profile, titles, itemIDsIn(t, rec, titles))
				}
			}
		})
	}

	// Favourites of containers (E-8): series and podcasts never pass the per-item loop above. A
	// profile that sees them marks and unmarks them (204), the detail reports the mark; any other
	// profile gets 404. A season is visible but not listable (422 not_listable).
	t.Run("favourites", func(t *testing.T) {
		for _, token := range []string{"p0", "p6", "p12", "p16", "p18", "px", "pg", "pg18"} {
			profile := pf.tokens[token]
			for _, m := range []vistest.Movie{f.Show.Series, f.Audio.Podcast, f.Audio.ExplicitPodcast} {
				path := BasePath + "/items/" + m.ID + "/favorite"
				visible := f.VisibleTo(profile, m)
				want := http.StatusNotFound
				if visible {
					want = http.StatusNoContent
				}
				if rec := send(t, h, http.MethodPut, path, token, "", nil); rec.Code != want {
					t.Errorf("%s addFavorite %s: status %d (%s), want %d", profile.Name, m.Label, rec.Code, rec.Body.String(), want)
				} else if !visible && problemCode(t, rec) != "not_found" {
					t.Errorf("%s addFavorite %s: code %s", profile.Name, m.Label, problemCode(t, rec))
				}
				list := send(t, h, http.MethodGet, BasePath+"/favorites", token, "", nil)
				assertOnlyVisible(t, f, "listFavorites", profile, titles, itemIDsIn(t, list, titles))
				if itemIDsIn(t, list, titles)[m.ID] != visible {
					t.Errorf("%s listFavorites after adding %s = %s", profile.Name, m.Label, list.Body.String())
				}
				if visible {
					var d struct {
						Favorite    bool `json:"favorite"`
						InWatchlist bool `json:"inWatchlist"`
					}
					decode(t, send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, token, "", nil), &d)
					if !d.Favorite {
						t.Errorf("%s getItem %s: favorite false after adding", profile.Name, m.Label)
					}
				}
				if rec := send(t, h, http.MethodDelete, path, token, "", nil); rec.Code != want {
					t.Errorf("%s removeFavorite %s: status %d, want %d", profile.Name, m.Label, rec.Code, want)
				}
				if itemIDsIn(t, send(t, h, http.MethodGet, BasePath+"/favorites", token, "", nil), titles)[m.ID] {
					t.Errorf("%s listFavorites still holds %s", profile.Name, m.Label)
				}
				// The watchlist takes containers too.
				if rec := send(t, h, http.MethodPut, BasePath+"/items/"+m.ID+"/watchlist", token, "", nil); rec.Code != want {
					t.Errorf("%s addToWatchlist %s: status %d, want %d", profile.Name, m.Label, rec.Code, want)
				}
			}
			season := http.StatusNotFound
			if f.VisibleTo(profile, f.Show.Season) {
				season = http.StatusUnprocessableEntity
			}
			rec := send(t, h, http.MethodPut, BasePath+"/items/"+f.Show.Season.ID+"/watchlist", token, "", nil)
			if rec.Code != season || (season == http.StatusUnprocessableEntity && problemCode(t, rec) != "not_listable") {
				t.Errorf("%s addToWatchlist season: status %d (%s), want %d", profile.Name, rec.Code, rec.Body.String(), season)
			}
		}
		// Watchlists stay separate per profile (E-5): what P6 put there is not on the list of P0.
		list := send(t, h, http.MethodGet, BasePath+"/watchlist", "p0", "", nil)
		if itemIDsIn(t, list, titles)[f.Rated(6).ID] {
			t.Errorf("P0 watchlist holds the film rated 6 of P6: %s", list.Body.String())
		}
	})

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
	t.Parallel()
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
	t.Parallel()
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
