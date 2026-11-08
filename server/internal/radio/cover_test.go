package radio

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

// The cover chain against fake services: one httptest server answers for MusicBrainz, the Cover
// Art Archive (with its redirect to an archive host), iTunes, their image hosts and a stream image.

// pngImage returns a PNG of w × h pixels.
func pngImage(t testing.TB, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// coverFake is the fake of every service of the chain: routes by path, hits counted per path.
type coverFake struct {
	srv    *httptest.Server
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	hits   map[string]int
	agents []string
	query  map[string]string // the last raw query per path
}

func newCoverFake(t *testing.T) *coverFake {
	f := &coverFake{routes: map[string]http.HandlerFunc{}, hits: map[string]int{}, query: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits[r.URL.Path]++
		f.agents = append(f.agents, r.UserAgent())
		f.query[r.URL.Path] = r.URL.RawQuery
		h := f.routes[r.URL.Path]
		f.mu.Unlock()
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *coverFake) route(path string, h http.HandlerFunc) {
	f.mu.Lock()
	f.routes[path] = h
	f.mu.Unlock()
}

func (f *coverFake) body(path, contentType string, b []byte) {
	f.route(path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(b)
	})
}

func (f *coverFake) hit(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func (f *coverFake) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, h := range f.hits {
		n += h
	}
	return n
}

type coverEnv struct {
	t     *testing.T
	ctx   context.Context
	db    *store.DB
	s     *Service
	fake  *coverFake
	clock *fakeClock
	cache string
	logs  *bytes.Buffer
}

func newCoverEnv(t *testing.T) *coverEnv {
	t.Helper()
	ctx := context.Background()
	db, err := storetest.Open(ctx, store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: testLogger()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &coverEnv{t: t, ctx: ctx, db: db, fake: newCoverFake(t), clock: &fakeClock{t: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)},
		cache: t.TempDir(), logs: &bytes.Buffer{}}
	log := slog.New(slog.NewTextHandler(&lockedWriter{w: e.logs}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e.s = withDialControl(New(db, log, WithRunner(testRunner()), WithCacheDir(e.cache), WithClock(e.clock.Now),
		WithUserAgent("Slouchcat/1.19.0")), loopbackOnly)
	e.s.cov.mb, e.s.cov.itunes = ratelimit.New("test-mb", 1000, 1000), ratelimit.New("test-itunes", 1000, 1000)
	e.s.cov.mbBase, e.s.cov.caaBase, e.s.cov.itunesBase = e.fake.srv.URL, e.fake.srv.URL, e.fake.srv.URL
	return e
}

// lockedWriter serialises the log writes of the worker and the test.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// lookup runs the chain for a title (and a StreamUrl) and returns the row of the title's key, or
// of the image's key for a job with a stream image.
func (e *coverEnv) lookup(artist, title, streamURL string) (coverJob, appq.RadioTrack, bool) {
	e.t.Helper()
	j, ok := coverJobFor(titleEntry{RadioNowEntry: model.RadioNowEntry{Artist: artist, Title: title}, streamURL: streamURL})
	if !ok {
		e.t.Fatalf("no lookup for %q - %q", artist, title)
	}
	e.s.lookupCover(e.ctx, j)
	row, ok := e.row(j.key)
	return j, row, ok
}

func (e *coverEnv) row(key string) (appq.RadioTrack, bool) {
	e.t.Helper()
	row, err := appq.New(e.db.Read).GetRadioTrackByKey(e.ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return row, true
}

// mbAnswer is a recording search answer: one recording of score with the releases (id, status,
// primary type, release group).
func mbAnswer(score int, releases ...[4]string) []byte {
	var b strings.Builder
	b.WriteString(`{"recordings":[{"score":` + strconv.Itoa(score) + `,"releases":[`)
	for i, r := range releases {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"` + r[0] + `","status":"` + r[1] + `","release-group":{"id":"` + r[3] + `","primary-type":"` + r[2] + `"}}`)
	}
	b.WriteString(`]}]}`)
	return []byte(b.String())
}

func (e *coverEnv) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Write.ExecContext(e.ctx, query, args...); err != nil {
		e.t.Fatal(err)
	}
}

// openCover reads a cover through Cover.
func (e *coverEnv) openCover(id int64, size string, webp bool) ([]byte, string, error) {
	e.t.Helper()
	src, err := e.s.Cover(e.ctx, id, size, webp)
	if err != nil {
		return nil, "", err
	}
	defer src.File.Close()
	b, err := io.ReadAll(src.File)
	if err != nil {
		e.t.Fatal(err)
	}
	return b, src.MediaType, nil
}

func TestCoverFromStreamImageFirst(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.body("/stream/c.png", "image/png", pngImage(t, 300, 300))
	j, row, ok := e.lookup("Band", "Song", e.fake.srv.URL+"/stream/c.png")
	if !ok || row.State != trackFound || row.Source.String != coverSourceStream || !strings.HasPrefix(j.key, "url:") {
		t.Fatalf("row %+v %v, key %q", row, ok, j.key)
	}
	if e.fake.hit("/ws/2/recording") != 0 || e.fake.hit("/search") != 0 {
		t.Fatal("the search ran although the stream announced an image")
	}
	for _, size := range []string{"w96", "w256", "w512"} {
		for _, webp := range []bool{true, false} {
			b, mt, err := e.openCover(row.ID, size, webp)
			if err != nil || len(b) == 0 || webp != (mt == "image/webp") {
				t.Fatalf("cover %s webp %v: %d bytes %q %v", size, webp, len(b), mt, err)
			}
		}
	}
	if _, _, err := e.openCover(row.ID, "w100", true); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("size w100: %v", err)
	}
	if _, _, err := e.openCover(row.ID+1, "w256", true); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown track: %v", err)
	}
}

// MusicBrainz finds the recording, the official album wins over a compilation, the Cover Art
// Archive redirects to its archive host; iTunes is not asked.
func TestCoverFromMusicBrainz(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.body("/ws/2/recording", "application/json", mbAnswer(100,
		[4]string{"rel-comp", "Bootleg", "Album", "rg-comp"}, [4]string{"rel-album", "Official", "Album", "rg-album"}))
	e.fake.route("/release/rel-album/front-500", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, e.fake.srv.URL+"/archive/rel-album.png", http.StatusTemporaryRedirect)
	})
	e.fake.body("/archive/rel-album.png", "image/png", pngImage(t, 500, 500))
	_, row, ok := e.lookup("Daft Punk", "One More Time (Radio Edit)", "")
	if !ok || row.State != trackFound || row.Source.String != coverSourceMB || row.Key != "daft punk|one more time" {
		t.Fatalf("row %+v %v", row, ok)
	}
	if e.fake.hit("/release/rel-comp/front-500") != 0 || e.fake.hit("/archive/rel-album.png") != 1 || e.fake.hit("/search") != 0 {
		t.Fatalf("hits %v", e.fake.hits)
	}
	q := e.fake.query["/ws/2/recording"]
	if !strings.Contains(q, "fmt=json") || !strings.Contains(q, "limit=5") ||
		!strings.Contains(q, "query=artist%3A%22Daft+Punk%22+AND+recording%3A%22One+More+Time%22") {
		t.Fatalf("search query %q", q)
	}
	for _, a := range e.fake.agents {
		if a != "Slouchcat/1.19.0 (+https://github.com/SlouchCat/slouchcat)" {
			t.Fatalf("User-Agent %q", a)
		}
	}
}

// Without a cover of the release the one of its release group serves; a score below 90 is no
// match.
func TestCoverReleaseGroupAndScore(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.body("/ws/2/recording", "application/json", mbAnswer(95, [4]string{"rel-1", "Official", "Single", "rg-1"}))
	e.fake.body("/release-group/rg-1/front-500", "image/png", pngImage(t, 500, 500))
	if _, row, ok := e.lookup("A", "B", ""); !ok || row.State != trackFound || e.fake.hit("/release/rel-1/front-500") != 1 {
		t.Fatalf("release group fallback: %+v %v", row, ok)
	}
	e.fake.body("/ws/2/recording", "application/json", mbAnswer(80, [4]string{"rel-2", "Official", "Album", "rg-2"}))
	if _, row, ok := e.lookup("C", "D", ""); !ok || row.State != trackNone || e.fake.hit("/release/rel-2/front-500") != 0 {
		t.Fatalf("score 80: %+v %v", row, ok)
	}
}

// iTunes serves when MusicBrainz finds nothing: the first result of the same artist, at 600 px,
// from the store of the server's country.
func TestCoverFromITunes(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.body("/ws/2/recording", "application/json", []byte(`{"recordings":[]}`))
	e.fake.body("/search", "application/json", []byte(`{"results":[
		{"artistName":"Someone Else","artworkUrl100":"`+e.fake.srv.URL+`/wrong/100x100bb.png"},
		{"artistName":"Böse Band","artworkUrl100":"`+e.fake.srv.URL+`/art/100x100bb.png"}]}`))
	e.fake.body("/art/600x600bb.png", "image/png", pngImage(t, 600, 600))
	_, row, ok := e.lookup("Bose Band feat. Gast", "Lied [Live]", "")
	if !ok || row.State != trackFound || row.Source.String != coverSourceITunes {
		t.Fatalf("row %+v %v", row, ok)
	}
	q := e.fake.query["/search"]
	if !strings.Contains(q, "entity=song") || !strings.Contains(q, "media=music") || !regexp.MustCompile(`country=[a-z]{2}(&|$)`).MatchString(q) ||
		!strings.Contains(q, "term=Bose+Band+Lied") || e.fake.hit("/wrong/100x100bb.png") != 0 {
		t.Fatalf("search %q, hits %v", q, e.fake.hits)
	}
}

// A hit is kept 30 days, a miss 24 hours; then the chain runs again.
func TestCoverCacheHitAndMiss(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.body("/ws/2/recording", "application/json", mbAnswer(100, [4]string{"r", "Official", "Album", "g"}))
	e.fake.body("/release/r/front-500", "image/png", pngImage(t, 500, 500))
	_, found, _ := e.lookup("Found", "Song", "")
	n := e.fake.total()
	e.lookup("Found", "Song", "")
	if e.fake.total() != n {
		t.Fatal("a cached hit asked again")
	}
	e.fake.body("/ws/2/recording", "application/json", []byte(`{"recordings":[]}`))
	_, miss, _ := e.lookup("Missing", "Song", "")
	if miss.State != trackNone || miss.ExpiresAt-miss.CheckedAt != coverMiss.Milliseconds() ||
		found.ExpiresAt-found.CheckedAt != coverHit.Milliseconds() {
		t.Fatalf("found %+v, miss %+v", found, miss)
	}
	n = e.fake.total()
	e.lookup("Missing", "Song", "")
	if e.fake.total() != n {
		t.Fatal("a cached miss asked again")
	}
	e.clock.Add(25 * time.Hour)
	e.lookup("Missing", "Song", "")
	if e.fake.total() == n {
		t.Fatal("an expired miss was not looked up again")
	}
	// Found → none after 30 days: the files go.
	e.clock.Add(30 * 24 * time.Hour)
	dir := filepath.Join(e.cache, coverDir(found.ID))
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if _, row, _ := e.lookup("Found", "Song", ""); row.State != trackNone || row.ID != found.ID {
		t.Fatalf("row after expiry %+v", row)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cover files after found → none: %v", err)
	}
	if _, _, err := e.openCover(found.ID, "w256", true); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("cover of a miss: %v", err)
	}
}

// With both searches switched off only the stream's own image is used.
func TestCoverSettingsOff(t *testing.T) {
	e := newCoverEnv(t)
	e.exec(`UPDATE radio_config SET cover_musicbrainz = 0, cover_itunes = 0`)
	e.fake.body("/stream/c.png", "image/png", pngImage(t, 200, 200))
	if _, row, ok := e.lookup("Band", "Song", e.fake.srv.URL+"/stream/c.png"); !ok || row.State != trackFound {
		t.Fatalf("stream image with the searches off: %+v %v", row, ok)
	}
	e.lookup("Band", "Other", "")
	if e.fake.hit("/ws/2/recording") != 0 || e.fake.hit("/search") != 0 {
		t.Fatalf("a search ran while switched off: %v", e.fake.hits)
	}
	if _, ok := e.row(trackKey("Band", "Other")); ok {
		t.Fatal("a lookup that asked nobody wrote a row")
	}
	e.exec(`UPDATE radio_config SET cover_musicbrainz = 0, cover_itunes = 1`)
	e.fake.body("/search", "application/json", []byte(`{"results":[]}`))
	e.lookup("Band", "Third", "")
	if e.fake.hit("/ws/2/recording") != 0 || e.fake.hit("/search") != 1 {
		t.Fatalf("MusicBrainz off, iTunes on: %v", e.fake.hits)
	}
}

// A busy service or a broken network writes nothing (the next title change asks again); the logs
// name the host, never the query.
func TestCoverTransientFailureNotCached(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.route("/ws/2/recording", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	e.fake.route("/search", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	if _, _, ok := e.lookup("Secret Artist", "Hidden Song", ""); ok {
		t.Fatal("a transient failure was cached")
	}
	if e.fake.hit("/ws/2/recording") != 1 || e.fake.hit("/search") != 1 {
		t.Fatalf("hits %v", e.fake.hits)
	}
	logs := e.logs.String()
	if strings.Contains(logs, "Hidden") || strings.Contains(logs, "query=") || strings.Contains(logs, "term=") ||
		!strings.Contains(logs, "site=http://127.0.0.1") {
		t.Fatalf("logs:\n%s", logs)
	}
	if e.s.cov.mb.Allow() || e.s.cov.itunes.Allow() {
		t.Fatal("a 503 or 429 did not pause the limiter")
	}
}

// The guard refuses local addresses: the production dial control refuses the loopback, and a
// redirect of the Cover Art Archive to a local address is refused although the test control
// allows the loopback.
func TestCoverLocalAddressRefused(t *testing.T) {
	e := newCoverEnv(t)
	e.s.rel.control = controlFunc // the production guard (the test build is not radiotest)
	e.s.rel.net = nil
	e.s.rel.netOnce = sync.Once{}
	e.fake.body("/stream/c.png", "image/png", pngImage(t, 200, 200))
	j, row, ok := e.lookup("Band", "Song", e.fake.srv.URL+"/stream/c.png")
	if e.fake.total() != 0 || !ok || row.State != trackNone {
		t.Fatalf("stream image on 127.0.0.1: %d requests, row %+v %v", e.fake.total(), row, ok)
	}
	if _, ok := e.row(trackKey(j.artist, j.title)); ok {
		t.Fatal("a refused search was cached")
	}

	e = newCoverEnv(t)
	e.fake.body("/ws/2/recording", "application/json", mbAnswer(100, [4]string{"r", "Official", "Album", "g"}))
	e.fake.route("/release/r/front-500", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://10.20.30.40/cover.jpg", http.StatusTemporaryRedirect)
	})
	e.fake.body("/search", "application/json", []byte(`{"results":[]}`))
	if _, row, ok := e.lookup("Band", "Song", ""); !ok || row.State != trackNone || e.fake.hit("/release-group/g/front-500") != 1 {
		t.Fatalf("redirect to a local address: %+v %v, hits %v", row, ok, e.fake.hits)
	}
	if !strings.Contains(e.logs.String(), "local network refused") {
		t.Fatalf("logs:\n%s", e.logs.String())
	}
}

// Images over 10 MiB, answers over 1 MiB, images under 96 px and SVG are no cover.
func TestCoverSizeCapsAndFormats(t *testing.T) {
	e := newCoverEnv(t)
	big := append(pngImage(t, 200, 200), make([]byte, maxCoverImage)...)
	for name, body := range map[string][]byte{
		"big.png":   big,
		"small.png": pngImage(t, 64, 64),
		"svg.png":   []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="500" height="500"></svg>`),
	} {
		e.fake.body("/stream/"+name, "image/png", body)
		if _, row, ok := e.lookup("", "", e.fake.srv.URL+"/stream/"+name); !ok || row.State != trackNone || e.fake.hit("/stream/"+name) != 1 {
			t.Errorf("%s: %+v %v", name, row, ok)
		}
	}
	e.fake.body("/ws/2/recording", "application/json", append([]byte(`{"recordings":[`), bytes.Repeat([]byte(" "), maxCoverJSON)...))
	e.fake.body("/search", "application/json", []byte(`{"results":[]}`))
	if _, row, ok := e.lookup("Band", "Long", ""); !ok || row.State != trackNone {
		t.Fatalf("search answer over 1 MiB: %+v %v", row, ok)
	}
	entries, err := os.ReadDir(filepath.Join(e.cache, "radio", "covers"))
	if err != nil && !errors.Is(err, os.ErrNotExist) || len(entries) != 0 {
		t.Fatalf("cover directory after refused images: %v %v", entries, err)
	}
}

// Only image addresses (by extension) are fetched from StreamUrl; the search key leaves out the
// noise, the display keeps it.
func TestCoverJobs(t *testing.T) {
	for _, tc := range []struct {
		artist, title, url string
		key                string
		ok                 bool
	}{
		{"Daft Punk", "One More Time (Radio Edit)", "", "daft punk|one more time", true},
		{"Beyoncé feat. Jay-Z", "Crazy in Love [Live]", "", "beyonce|crazy in love", true},
		{"Band", "Song", "https://radio.example/", "band|song", true},
		{"Band", "Song", "ftp://radio.example/c.jpg", "band|song", true},
		{"", "", "https://img.example/a/c.JPG?size=1", "url:", true},
		{"", "", "https://radio.example/", "", false},
		{"", "", "", "", false},
	} {
		j, ok := coverJobFor(titleEntry{RadioNowEntry: model.RadioNowEntry{Artist: tc.artist, Title: tc.title}, streamURL: tc.url})
		if ok != tc.ok || !strings.HasPrefix(j.key, tc.key) || tc.key == "url:" && len(j.key) != 4+64 {
			t.Errorf("%q - %q (%q): key %q %v, want %q %v", tc.artist, tc.title, tc.url, j.key, ok, tc.key, tc.ok)
		}
	}
	if got := luceneQuoted(`AC\DC "live"`); got != `"AC\\DC \"live\""` {
		t.Errorf("lucene phrase %s", got)
	}
}

// The queue holds 64 lookups; a key queued already is not queued twice, more are dropped.
func TestCoverQueueDedupesAndDrops(t *testing.T) {
	s := New(nil, testLogger())
	for i := range coverQueueSize + 10 {
		s.queueCover(coverJob{key: "k" + strconv.Itoa(i)})
		s.queueCover(coverJob{key: "k0"})
	}
	if n := len(s.cov.queue); n != coverQueueSize {
		t.Fatalf("%d queued, want %d", n, coverQueueSize)
	}
	if s.cov.queued["k70"] || !s.cov.queued["k0"] || len(s.cov.queued) != coverQueueSize {
		t.Fatalf("queued keys %d", len(s.cov.queued))
	}
}

// MusicBrainz is asked at most once a second, iTunes 20 times a minute.
func TestCoverRateLimits(t *testing.T) {
	c := newCoverState()
	ctx := context.Background()
	start := time.Now()
	for range 2 {
		if err := c.mb.Wait(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 900*time.Millisecond {
		t.Fatalf("two MusicBrainz requests within %v", d)
	}
	if err := c.itunes.Wait(ctx); err != nil || c.itunes.Allow() {
		t.Fatalf("a second iTunes request right after the first: %v", err)
	}
}

// The daily sweep deletes expired rows and the files of their covers.
func TestCoverSweep(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.body("/stream/c.png", "image/png", pngImage(t, 200, 200))
	_, found, _ := e.lookup("", "", e.fake.srv.URL+"/stream/c.png")
	e.fake.body("/ws/2/recording", "application/json", []byte(`{"recordings":[]}`))
	e.fake.body("/search", "application/json", []byte(`{"results":[]}`))
	_, miss, _ := e.lookup("No", "Cover", "")
	dir := filepath.Join(e.cache, coverDir(found.ID))
	start := e.clock.Now()
	e.s.sweepCovers(e.ctx, start.Add(time.Hour))
	if _, ok := e.row(found.Key); !ok {
		t.Fatal("a valid row was swept")
	}
	// The miss has expired, but the last sweep was less than a day ago.
	e.s.sweepCovers(e.ctx, start.Add(coverMiss+30*time.Minute))
	if _, ok := e.row(miss.Key); !ok {
		t.Fatal("swept twice within a day")
	}
	stale := filepath.Join(e.cache, "radio", "covers", "new-01JRADIOTESTSTALE000000000")
	if err := os.MkdirAll(stale, 0o750); err != nil {
		t.Fatal(err)
	}
	e.clock.Add(2 * coverHit)
	e.s.sweepCovers(e.ctx, e.clock.Now())
	if _, ok := e.row(found.Key); ok {
		t.Fatal("expired hit kept")
	}
	if _, ok := e.row(miss.Key); ok {
		t.Fatal("expired miss kept")
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("files of a swept cover: %v", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("render left behind by a stopped process: %v", err)
	}
}

// The relay's timeline entries get the cover of their title when the worker has it, also the
// entries behind live, and Now returns it.
func TestCoverReachesTheTimeline(t *testing.T) {
	e := newRelayEnv(t)
	fake := newCoverFake(t)
	e.s.o.cacheDir = t.TempDir()
	e.s.cov.mb, e.s.cov.itunes = ratelimit.New("test-mb", 1000, 1000), ratelimit.New("test-itunes", 1000, 1000)
	e.s.cov.mbBase, e.s.cov.caaBase, e.s.cov.itunesBase = fake.srv.URL, fake.srv.URL, fake.srv.URL
	fake.body("/ws/2/recording", "application/json", mbAnswer(100, [4]string{"r", "Official", "Album", "g"}))
	fake.body("/release/r/front-500", "image/png", pngImage(t, 500, 500))
	fake.body("/search", "application/json", []byte(`{"results":[]}`))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.s.runCovers(ctx)
	m := mp3Mount(t, false)
	m.MetaInt = 8192
	m.Titles = []radiotest.Title{{At: 8192, Text: "Band - Song"}, {At: 20 * 8192, Text: "Other - Title"}}
	m.Once = true
	e.st.Set("/live", m)
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, _ := e.listen(e.f.P18, id)
	deadline := time.Now().Add(30 * time.Second)
	for {
		now, err := e.s.Now(context.Background(), sess, id, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		covered := 0
		for _, en := range now.Entries {
			if en.CoverTrackID != 0 && en.CoverHash != "" {
				covered++
			}
		}
		if len(now.Entries) >= 2 && covered == len(now.Entries) { // the station repeats after a reconnect
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("now %+v", now)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// The queries of the cover cache read one row through a key; the sweep walks the expiry index.
func TestCoverQueriesUseKeys(t *testing.T) {
	e := newCatEnv(t)
	src, err := os.ReadFile(filepath.Join("..", "..", "queries", "app", "radio_tracks.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for name, arg := range map[string]sql.NamedArg{"GetRadioTrackByKey": sql.Named("key", "a|b"),
		"GetRadioTrack": sql.Named("id", 1), "DeleteExpiredRadioTracks": sql.Named("now", 1)} {
		plan := explainArgs(t, e, oneStatement(t, src, name), arg)
		if strings.Contains(plan, "SCAN ") || !strings.Contains(plan, "SEARCH ") {
			t.Errorf("%s does not read through a key:\n%s", name, plan)
		}
	}
}

// explainArgs returns the detail lines of EXPLAIN QUERY PLAN for query with its one parameter.
func explainArgs(t *testing.T, e *catEnv, query string, arg sql.NamedArg) string {
	t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, "EXPLAIN QUERY PLAN "+query, arg)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return plan.String()
}
