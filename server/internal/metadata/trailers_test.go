package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

const (
	ytKey      = "yt-test-api-key-AIzaFAKE"
	ytQuotaKey = "yt-quota-exceeded-key-FAKE"
)

// testClock is a settable clock for the YouTube quota.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

// fakeYouTube serves search.list and videos.list of the YouTube Data API v3.
type fakeYouTube struct {
	srv *httptest.Server

	mu      sync.Mutex
	queries []string // q of every search
	videos  int      // videos.list calls
}

// ytVideos are the videos the fake knows: id → title, channel, duration.
var ytVideos = map[string][3]string{
	"enShort0001": {"Searched Film (2002) Trailer", "Clips", "PT45S"},                      // too short
	"enPlain0001": {"Searched Film Trailer", "Movie Clips", "PT2M10S"},                     // fits
	"enOffic0001": {"SEARCHED FILM – Official Trailer [HD]", "Studio Official", "PT2M30S"}, // official channel wins
	"enOther0001": {"Other Film Official Trailer", "Studio Official", "PT2M"},              // another title
	"enLong00001": {"Searched Film Trailer Reaction", "Fans", "PT12M"},                     // too long
	"deTrail0001": {"Searched Film Trailer Deutsch German", "KinoCheck", "PT1M50S"},
	"deNoTrl0001": {"Searched Film Kritik", "Filmkritik", "PT3M"}, // no "trailer"
}

func newFakeYouTube(t *testing.T) *fakeYouTube {
	t.Helper()
	f := &fakeYouTube{}
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	keyOK := func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Query().Get("key") {
		case ytKey:
			return true
		case ytQuotaKey:
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": 403, "message": "quota",
				"errors": []any{map[string]any{"domain": "youtube.quota", "reason": "quotaExceeded"}}}})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": 400, "message": "API key not valid.",
				"errors":  []any{map[string]any{"domain": "global", "reason": "badRequest"}},
				"details": []any{map[string]any{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "API_KEY_INVALID"}}}})
		}
		return false
	}
	mux.HandleFunc("GET /youtube/v3/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f.mu.Lock()
		f.queries = append(f.queries, q.Get("q"))
		f.mu.Unlock()
		if !keyOK(w, r) {
			return
		}
		if q.Get("part") != "snippet" || q.Get("type") != "video" || q.Get("videoDuration") != "short" || q.Get("maxResults") != "5" {
			t.Errorf("unexpected search parameters %s", r.URL.RawQuery)
		}
		var ids []string
		switch {
		case !strings.Contains(q.Get("q"), "Searched Film"):
		case q.Get("relevanceLanguage") == "de":
			ids = []string{"deNoTrl0001", "deTrail0001"}
		case q.Get("relevanceLanguage") == "en":
			ids = []string{"enShort0001", "enPlain0001", "enOther0001", "enOffic0001", "enLong00001"}
		}
		items := []any{}
		for _, id := range ids {
			items = append(items, map[string]any{"kind": "youtube#searchResult", "id": map[string]any{"kind": "youtube#video", "videoId": id},
				"snippet": map[string]any{"title": ytVideos[id][0], "channelTitle": ytVideos[id][1]}})
		}
		writeJSON(w, http.StatusOK, map[string]any{"kind": "youtube#searchListResponse", "items": items})
	})
	mux.HandleFunc("GET /youtube/v3/videos", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.videos++
		f.mu.Unlock()
		if !keyOK(w, r) {
			return
		}
		if r.URL.Query().Get("part") != "contentDetails,snippet" {
			t.Errorf("unexpected videos parameters %s", r.URL.RawQuery)
		}
		items := []any{}
		for _, id := range strings.Split(r.URL.Query().Get("id"), ",") {
			v, ok := ytVideos[id]
			if !ok {
				continue
			}
			items = append(items, map[string]any{"kind": "youtube#video", "id": id,
				"snippet":        map[string]any{"title": v[0], "channelTitle": v[1], "publishedAt": "2002-01-01T00:00:00Z"},
				"contentDetails": map[string]any{"duration": v[2], "definition": "hd"}})
		}
		writeJSON(w, http.StatusOK, map[string]any{"kind": "youtube#videoListResponse", "items": items})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeYouTube) searches() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.queries)
}

type trailerRow struct {
	Lang, YouTubeID, Title, Source string
	Official                       bool
}

func (e *env) trailers(t *testing.T, id string) []trailerRow {
	t.Helper()
	rows, err := e.svc.read.ListTrailersByItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var out []trailerRow
	for _, r := range rows {
		out = append(out, trailerRow{r.Lang, r.YoutubeID, r.Title.String, r.Source, r.Official == 1})
	}
	return out
}

func (e *env) trailerState(t *testing.T, id string) (string, int64) {
	t.Helper()
	var state string
	var checked *int64
	if err := e.db.Read.QueryRow(`SELECT trailer_state, trailer_checked_at FROM items WHERE id = ?`, id).Scan(&state, &checked); err != nil {
		t.Fatal(err)
	}
	if checked == nil {
		return state, 0
	}
	return state, *checked
}

func TestTrailersFromTMDB(t *testing.T) {
	e := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.jobList(t, JobTrailerFind); !slices.Equal(got, []string{"trailer:" + idByTMDB + " p1"}) {
		t.Fatalf("trailer jobs %q", got)
	}
	if err := e.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	want := []trailerRow{
		{"de", "deOfficial1", "Offizieller Trailer", "tmdb", true},
		{"en", "enOfficial1", "Official Trailer 2", "tmdb", true},
	}
	if got := e.trailers(t, idByTMDB); !slices.Equal(got, want) {
		t.Fatalf("trailers %+v, want %+v", got, want)
	}
	if state, at := e.trailerState(t, idByTMDB); state != "found" || at != testNow.UnixMilli() {
		t.Fatalf("state %s at %d", state, at)
	}
	if n := len(e.yt.searches()); n != 0 {
		t.Fatalf("%d YouTube searches although TMDB had both languages", n)
	}
	if e.tmdb.count("/3/movie/550/videos") != 2 {
		t.Fatalf("videos requested %d times, want 2 (de-DE, en-US)", e.tmdb.count("/3/movie/550/videos"))
	}
}

func TestTrailersYouTubeFallback(t *testing.T) {
	e := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2002})
	if err := e.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.findTrailers(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	want := []trailerRow{
		{"de", "deTrail0001", "Searched Film Trailer Deutsch German", "youtube", false},
		{"en", "enOffic0001", "SEARCHED FILM – Official Trailer [HD]", "youtube", true},
	}
	if got := e.trailers(t, idBySearch); !slices.Equal(got, want) {
		t.Fatalf("trailers %+v, want %+v", got, want)
	}
	// The German texts fall back to the English title (no translation in the fixture).
	wantQ := []string{"Searched Film 2001 trailer deutsch", "Searched Film 2001 trailer"}
	if got := e.yt.searches(); !slices.Equal(got, wantQ) {
		t.Fatalf("searches %q, want %q", got, wantQ)
	}
	if r := e.quota.Remaining(); r != youtubeDailyUnits-2*(youtubeSearchCost+youtubeVideosCost) {
		t.Fatalf("quota remaining %d", r)
	}
	if state, _ := e.trailerState(t, idBySearch); state != "found" {
		t.Fatalf("state %s", state)
	}
}

func TestTrailersQuotaExhaustedStaysPending(t *testing.T) {
	e := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2002})
	if err := e.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	// 150 units left: the German search and lookup fit, the English search does not.
	if !e.quota.Take(ctx, youtubeDailyUnits-150) {
		t.Fatal("quota setup")
	}
	if err := e.svc.findTrailers(ctx, idBySearch); err != nil {
		t.Fatalf("an exhausted quota must end the job without error: %v", err)
	}
	if got := e.trailers(t, idBySearch); len(got) != 1 || got[0].Lang != "de" {
		t.Fatalf("trailers %+v, want the German one", got)
	}
	if state, _ := e.trailerState(t, idBySearch); state != "pending" {
		t.Fatalf("state %s, want pending", state)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueuePendingTrailers(ctx, 1); err != nil || n != 1 {
		t.Fatalf("QueuePendingTrailers = %d, %v; want 1", n, err)
	}
	if n, err := e.svc.QueuePendingTrailers(ctx, 1); err != nil || n != 0 {
		t.Fatalf("second QueuePendingTrailers = %d, %v; want 0 (job active)", n, err)
	}
	// After the reset at midnight Pacific time the next run finds both.
	e.clock.set(testNow.Add(24 * time.Hour))
	if err := e.svc.findTrailers(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if got := e.trailers(t, idBySearch); len(got) != 2 {
		t.Fatalf("trailers after the reset %+v", got)
	}
	if state, _ := e.trailerState(t, idBySearch); state != "found" {
		t.Fatalf("state after the reset %s", state)
	}

	// Google's own "quota exceeded" answer (count lost by a restart) also keeps the title pending.
	e2 := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytQuotaKey})
	e2.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2002})
	if err := e2.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if err := e2.svc.findTrailers(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if state, _ := e2.trailerState(t, idBySearch); state != "pending" || e2.quota.Remaining() != 0 {
		t.Fatalf("state %s, quota %d; want pending and an exhausted quota", state, e2.quota.Remaining())
	}
	if n := len(e2.yt.searches()); n != 1 {
		t.Fatalf("%d searches after quotaExceeded, want 1", n)
	}
}

func TestTrailersRefreshKeepsManualRows(t *testing.T) {
	e := newEnvWith(t, testKeys{tmdb: goodToken})
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at)
	                         VALUES (?, 'en', 'staleAuto01', 'Old', 'youtube', 0, 1),
	                                (?, 'de', 'manualPick1', NULL, 'manual', 0, 1),
	                                (?, 'de', 'deOfficial1', NULL, 'manual', 0, 1)`, idByTMDB, idByTMDB, idByTMDB)
	if err := e.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	want := []trailerRow{
		{"de", "manualPick1", "", "manual", false},
		{"de", "deOfficial1", "", "manual", false}, // the TMDB hit equal to a manual row keeps the manual row
		{"en", "enOfficial1", "Official Trailer 2", "tmdb", true},
	}
	if got := e.trailers(t, idByTMDB); !slices.Equal(got, want) {
		t.Fatalf("trailers %+v, want %+v", got, want)
	}
	// A refresh without TMDB hits leaves the manual rows and the state found.
	mustExec(t, e.db.Write, `UPDATE items SET tmdb_id = 601 WHERE id = ?`, idByTMDB)
	if err := e.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.trailers(t, idByTMDB); len(got) != 2 || got[0].Source != "manual" || got[1].Source != "manual" {
		t.Fatalf("trailers after refresh %+v", got)
	}
	if state, _ := e.trailerState(t, idByTMDB); state != "found" {
		t.Fatalf("state %s", state)
	}
	// Nothing at all: none.
	mustExec(t, e.db.Write, `DELETE FROM trailers`)
	if err := e.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if state, _ := e.trailerState(t, idByTMDB); state != "none" {
		t.Fatalf("state %s, want none", state)
	}
}

func TestTrailersSkipAndFail(t *testing.T) {
	e := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	ctx := context.Background()
	// Locked trailers, series without tmdb_id: no request.
	e.addItem(t, itemSeed{id: idLocked, title: "Locked", tmdbID: 550, locked: `["trailer"]`})
	e.addNode(t, node{id: idSeries, kind: "series", title: "TVDB only", tvdbID: 424242, state: "matched"})
	for _, id := range []string{idLocked, idSeries} {
		if err := e.svc.findTrailers(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.tmdb.total(); n != 0 {
		t.Fatalf("%d TMDB requests", n)
	}
	// A series with tmdb_id uses /tv/{id}/videos; the missing German one is searched on YouTube.
	mustExec(t, e.db.Write, `UPDATE items SET tmdb_id = 900 WHERE id = ?`, idSeries)
	if err := e.svc.findTrailers(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if got := e.trailers(t, idSeries); len(got) != 1 || got[0].YouTubeID != "tvTrailer01" {
		t.Fatalf("series trailers %+v", got)
	}
	if q := e.yt.searches(); len(q) != 1 || q[0] != "TVDB only trailer deutsch" {
		t.Fatalf("series searches %q", q)
	}
	// A rejected TMDB key fails the trailer state without a retry.
	e2 := newEnvWith(t, testKeys{tmdb: "revoked"})
	e2.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e2.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if state, _ := e2.trailerState(t, idByTMDB); state != "failed" {
		t.Fatalf("state %s, want failed", state)
	}
	// A rejected YouTube key skips YouTube.
	e3 := newEnvWith(t, testKeys{tmdb: goodToken, youtube: "not-a-key"})
	e3.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2002, tmdbID: 601})
	if err := e3.svc.findTrailers(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if state, _ := e3.trailerState(t, idBySearch); state != "none" {
		t.Fatalf("state %s, want none", state)
	}
}

func TestTrailerAIConfig(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	if _, ok := newAITrailers("off", log).(nopAITrailers); !ok || buf.Len() != 0 {
		t.Fatalf("off: %q", buf.String())
	}
	p := newAITrailers("openai", log)
	if hits, err := p.FindTrailers(context.Background(), TrailerQuery{Langs: []string{"de"}}); err != nil || len(hits) != 0 {
		t.Fatalf("nop provider found %v, %v", hits, err)
	}
	if !strings.Contains(buf.String(), "trailer AI provider not available") {
		t.Fatalf("no warning for an unknown mode: %q", buf.String())
	}
}

func TestParseISODuration(t *testing.T) {
	for in, want := range map[string]int{"PT45S": 45, "PT2M10S": 130, "PT1H": 3600, "P1DT1S": 86401, "PT12M": 720} {
		if got, ok := parseISODuration(in); !ok || got != want {
			t.Errorf("%s = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "P", "PT", "2M", "PTXS"} {
		if _, ok := parseISODuration(in); ok {
			t.Errorf("%q accepted", in)
		}
	}
}

// TestNoSecretsInLogs runs the TMDB, TVDB and YouTube clients against failing servers with fake
// keys and logs their errors through the redacting handler (CLAUDE.md rule 10).
func TestNoSecretsInLogs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(logging.NewRedactingHandler(slog.NewTextHandler(&lockedWriter{w: &buf}, nil)))
	keys := testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin, youtube: ytKey}
	ctx := context.Background()

	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer broken.Close()

	var errs []error
	for _, base := range []string{closedURL, broken.URL} {
		tmdb := &tmdbProvider{client: NewClient(base, base, &http.Client{Timeout: time.Second}), keys: keys}
		_, err := tmdb.Movie(ctx, "550", textLangs)
		errs = append(errs, err)
		tvdb := newTVDBProvider(base, base, &http.Client{Timeout: time.Second}, keys)
		_, err = tvdb.Series(ctx, "424242", textLangs)
		errs = append(errs, err)
		yt := newYouTubeTrailers(base, &http.Client{Timeout: time.Second}, newTestQuota(), keys)
		_, err = yt.FindTrailers(ctx, TrailerQuery{Titles: map[string]string{"en": "Searched Film"}, Langs: []string{"en"}})
		errs = append(errs, err)
	}
	for i, err := range errs {
		if err == nil {
			t.Fatalf("call %d succeeded against a failing server", i)
		}
		for _, secret := range []string{goodToken, tvdbKey, tvdbPin, ytKey} {
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error %d carries a secret: %v", i, err)
			}
		}
		log.Warn("provider call failed", "err", err)
	}
	// A URL with the YouTube key, logged as a string, an error and a message, is masked as well.
	raw := "https://www.googleapis.com/youtube/v3/search?q=x&key=" + ytKey
	log.Warn("request "+raw, "url", raw, "err", errors.New(`Get "`+raw+`": timeout`),
		"youtube_api_key", ytKey, "tvdb_api_key", tvdbKey, "tvdb_pin", tvdbPin)

	out := buf.String()
	for _, secret := range []string{goodToken, tvdbKey, tvdbPin, ytKey} {
		if strings.Contains(out, secret) {
			t.Errorf("secret %q in logs:\n%s", secret, out)
		}
	}
	if !strings.Contains(out, "key="+logging.Redacted) {
		t.Errorf("YouTube URL not masked:\n%s", out)
	}
}

func newTestQuota() *ratelimit.Quota {
	return ratelimit.NewQuota("youtube", youtubeDailyUnits, ratelimit.MidnightPacific)
}
