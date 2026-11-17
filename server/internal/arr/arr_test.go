package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/arr/arrtest"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

func fast() *ratelimit.Limiter { return ratelimit.New("arr-test", 1000, 100) }

func newClient(t *testing.T, f *arrtest.Server, opts ...Option) *Client {
	t.Helper()
	c, err := New(Kind(f.Kind), f.URL, arrtest.APIKey, append([]Option{WithLimiter(fast())}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewRefusesBadAddresses(t *testing.T) {
	for _, raw := range []string{"", "ftp://radarr.lan", "http://", "http://user:secret-pw@radarr.lan", "http://radarr.lan/?apikey=x", "http://radarr.lan/#x", "::"} {
		_, err := New(Radarr, raw, "key")
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("New(%q) = %v, want ErrInvalid", raw, err)
		}
		if err != nil && strings.Contains(err.Error(), "secret-pw") {
			t.Errorf("error quotes the address: %v", err)
		}
	}
	if _, err := New(Radarr, "http://radarr.lan", " "); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty key: %v", err)
	}
	if _, err := New("lidarr", "http://radarr.lan", "key"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown kind: %v", err)
	}
	for raw, want := range map[string]string{
		"http://192.168.1.125:7878/":         "http://192.168.1.125:7878",
		"https://media.example.org/sonarr/":  "https://media.example.org/sonarr",
		"http://radarr.lan:7878/api/v3":      "http://radarr.lan:7878",
		" https://media.example.org/api/v3/": "https://media.example.org",
	} {
		c, err := New(Sonarr, raw, "key")
		if err != nil || c.base != want {
			t.Errorf("New(%q) base = %v, %v; want %s", raw, c, err, want)
		}
	}
}

func TestDefaults(t *testing.T) {
	c, err := New(Radarr, "http://radarr.lan", "key")
	if err != nil {
		t.Fatal(err)
	}
	if c.http.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", c.http.Timeout)
	}
	if tr := c.http.Transport.(*http.Transport); tr.Proxy != nil {
		t.Error("transport uses a proxy from the environment")
	}
	if Rate != 2 || Burst != 2 || MaxResponseBytes != 4<<20 {
		t.Errorf("rate %d burst %d size %d", Rate, Burst, MaxResponseBytes)
	}
	if c.Kind() != Radarr || c.Site() != "http://radarr.lan" {
		t.Errorf("kind %s site %s", c.Kind(), c.Site())
	}
}

func TestLANAddressReachable(t *testing.T) {
	f := arrtest.NewRadarr(t)
	u, _ := url.Parse(f.URL)
	ip := netip.MustParseAddr(u.Hostname())
	if !netguard.Local(ip) {
		t.Fatalf("%s is not a local address; the test proves nothing", ip)
	}
	if _, err := newClient(t, f).Tags(context.Background()); err != nil {
		t.Fatalf("Tags on %s: %v", ip, err)
	}
}

func TestRedirectRefused(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer target.Close()
	f := arrtest.NewRadarr(t)
	for _, status := range []int{http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		f.SetFailure("GET /api/v3/tag", arrtest.Failure{Status: status, Location: target.URL + "/api/v3/tag"})
		_, err := newClient(t, f).Tags(context.Background())
		if !errors.Is(err, ErrRedirect) || !errors.Is(err, ErrUnreachable) {
			t.Errorf("status %d: err = %v, want ErrRedirect and ErrUnreachable", status, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("redirect target got %d requests", n)
	}
}

func TestTimeout(t *testing.T) {
	f := arrtest.NewRadarr(t)
	f.SetFailure("GET /api/v3/queue", arrtest.Failure{Hang: true})
	start := time.Now()
	_, err := newClient(t, f, WithTimeout(300*time.Millisecond)).Queue(context.Background())
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("call took %v", d)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = newClient(t, f).Queue(ctx)
	if !errors.Is(err, ErrUnreachable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("caller deadline: err = %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = newClient(t, f).Queue(ctx)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnreachable) {
		t.Errorf("cancelled: err = %v", err)
	}
}

func TestUnreachable(t *testing.T) {
	f := arrtest.NewSonarr(t)
	c := newClient(t, f)
	f.Close()
	_, err := c.Tags(context.Background())
	if !errors.Is(err, ErrUnreachable) || strings.Contains(err.Error(), arrtest.APIKey) {
		t.Errorf("err = %v", err)
	}
}

func TestRateLimit(t *testing.T) {
	f := arrtest.NewRadarr(t)
	c, err := New(Radarr, f.URL, arrtest.APIKey) // the default limiter
	if err != nil {
		t.Fatal(err)
	}
	const n = 6 // 2 in the burst, 4 more at 2 per second: at least 2 s
	start := time.Now()
	var mu sync.Mutex
	var done []time.Duration
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			if _, err := c.Tags(context.Background()); err != nil {
				t.Error(err)
			}
			mu.Lock()
			done = append(done, time.Since(start))
			mu.Unlock()
		})
	}
	wg.Wait()
	if d := time.Since(start); d < 1800*time.Millisecond {
		t.Errorf("%d calls took %v, want at least 1.8 s at 2 per second", n, d)
	}
	early := 0
	for _, d := range done {
		if d < 900*time.Millisecond {
			early++
		}
	}
	if early > 4 { // burst 2 plus 2 per second
		t.Errorf("%d calls within 0.9 s", early)
	}
	if got := f.Calls("GET /api/v3/tag"); got != n {
		t.Errorf("fake saw %d calls", got)
	}
}

func TestWrongKeyAndSecrets(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logging.New(&buf, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	f := arrtest.NewRadarr(t)
	const wrongKey = "wrong-arr-key-5a3f9c1d"
	wrong, err := New(Radarr, f.URL, wrongKey, WithLimiter(fast()), WithLogger(logger))
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() error{
		func() error { _, err := wrong.Tags(context.Background()); return err },
		func() error { _, err := wrong.Queue(context.Background()); return err },
		func() error { _, err := wrong.MovieByTMDB(context.Background(), 603); return err },
	} {
		err := call()
		if !errors.Is(err, ErrKeyInvalid) || strings.Contains(err.Error(), wrongKey) {
			t.Errorf("wrong key: %v", err)
		}
	}

	// The service echoing the key in its message: the error shows "[redacted]".
	c := newClient(t, f, WithLogger(logger))
	f.SetFailure("GET /api/v3/queue", arrtest.Failure{Status: 500, Body: `{"message":"bad header X-Api-Key=` + arrtest.APIKey + `"}`})
	_, err = c.Queue(context.Background())
	if !errors.Is(err, ErrServer) || strings.Contains(err.Error(), arrtest.APIKey) || !strings.Contains(err.Error(), redacted) {
		t.Errorf("echoed key: %v", err)
	}
	if _, err := c.Tags(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, s := range []string{fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), fmt.Sprintf("%v", Credentials{Key: arrtest.APIKey})} {
		if strings.Contains(s, arrtest.APIKey) {
			t.Errorf("printed value shows the key: %s", s)
		}
	}
	if b, _ := json.Marshal(Credentials{Key: arrtest.APIKey}); bytes.Contains(b, []byte(arrtest.APIKey)) {
		t.Errorf("json shows the key: %s", b)
	}
	out := buf.String()
	if strings.Contains(out, arrtest.APIKey) || strings.Contains(out, wrongKey) {
		t.Errorf("log shows a key:\n%s", out)
	}
	if !strings.Contains(out, `"path":"/api/v3/tag"`) || !strings.Contains(out, `"service":"radarr"`) || !strings.Contains(out, `"status":401`) {
		t.Errorf("log lacks the call lines:\n%s", out)
	}
	if strings.Contains(out, "records") || strings.Contains(out, "label") {
		t.Errorf("log shows a body:\n%s", out)
	}
}

func TestErrorClasses(t *testing.T) {
	f := arrtest.NewRadarr(t)
	c := newClient(t, f)
	for _, tc := range []struct {
		status int
		body   string
		want   error
		msg    string
	}{
		{400, `[{"propertyName":"Label","errorMessage":"Allowed characters a-z, 0-9 and -","attemptedValue":"x y","severity":"error"}]`, ErrValidation, "Label: Allowed characters a-z, 0-9 and -"},
		{422, `{"message":"unprocessable"}`, ErrValidation, "unprocessable"},
		{401, ``, ErrKeyInvalid, ""},
		{404, `{"message":"NotFound"}`, ErrNotFound, "NotFound"},
		{405, ``, ErrBadRequest, ""},
		{500, `{"message":"Database is locked","description":"stack"}`, ErrServer, "Database is locked"},
		{503, `<html>down</html>`, ErrServer, ""},
	} {
		f.SetFailure("GET /api/v3/tag", arrtest.Failure{Status: tc.status, Body: tc.body})
		_, err := c.Tags(context.Background())
		ae, ok := errors.AsType[*Error](err)
		if !errors.Is(err, tc.want) || !ok || ae.Message != tc.msg || ae.Status != tc.status || ae.Service != Radarr || ae.Op != "GET /api/v3/tag" {
			t.Errorf("status %d: err = %v, want %v with message %q", tc.status, err, tc.want, tc.msg)
		}
	}
	f.SetFailure("GET /api/v3/tag", arrtest.Failure{Status: 200, Body: `{"not":"a list"}`})
	if _, err := c.Tags(context.Background()); !errors.Is(err, ErrServer) {
		t.Errorf("wrong shape: %v", err)
	}
}

func TestOversizedAnswer(t *testing.T) {
	f := arrtest.NewRadarr(t)
	f.SetFailure("GET /api/v3/tag", arrtest.Failure{Status: 200, Body: "[" + strings.Repeat(`{"id":1,"label":"a"},`, MaxResponseBytes/20) + `{"id":1,"label":"a"}]`})
	if _, err := newClient(t, f).Tags(context.Background()); !errors.Is(err, ErrServer) {
		t.Errorf("err = %v, want ErrServer", err)
	}
}

func TestEnsureTag(t *testing.T) {
	for _, f := range []*arrtest.Server{arrtest.NewRadarr(t), arrtest.NewSonarr(t)} {
		c := newClient(t, f)
		ctx := context.Background()
		f.AddTag("1-marco") // Seerr's own tag (I-55)

		tag, created, err := c.EnsureTag(ctx, "sc-kinder")
		if err != nil || !created || tag.ID < 1 || tag.Label != "sc-kinder" {
			t.Fatalf("%s first EnsureTag = %+v, %v, %v", f.Kind, tag, created, err)
		}
		again, created, err := c.EnsureTag(ctx, "sc-kinder")
		if err != nil || created || again.ID != tag.ID {
			t.Errorf("%s second EnsureTag = %+v, %v, %v; want id %d, not created", f.Kind, again, created, err, tag.ID)
		}
		if n := f.Calls("POST /api/v3/tag"); n != 1 {
			t.Errorf("%s: %d POSTs, want 1", f.Kind, n)
		}
		if n := f.Calls("GET /api/v3/tag"); n != 2 {
			t.Errorf("%s: %d list calls, want 2", f.Kind, n)
		}

		// An invalid label is refused before any call.
		f.ResetCalls()
		for _, bad := range []string{"", "Sc-Kinder", "sc kinder", "jürg", "-sc"} {
			if _, _, err := c.EnsureTag(ctx, bad); !errors.Is(err, ErrInvalid) {
				t.Errorf("%s EnsureTag(%q) = %v, want ErrInvalid", f.Kind, bad, err)
			}
		}
		if n := len(f.CallLog()); n != 0 {
			t.Errorf("%s: invalid labels sent %d calls", f.Kind, n)
		}
		if got := f.Tags(); len(got) != 2 || got[tag.ID] != "sc-kinder" {
			t.Errorf("%s tags = %v", f.Kind, got)
		}
	}
}

// TestFakeMatchesTheRealServices pins the behaviour of the fakes measured in appendix I: POST
// lower-cases the label and is idempotent (I-50, I-51); Radarr refuses labels outside
// ^[a-z0-9-]+$ with its validation array, Sonarr takes them (I-52).
func TestFakeMatchesTheRealServices(t *testing.T) {
	ctx := context.Background()
	radarr, sonarr := arrtest.NewRadarr(t), arrtest.NewSonarr(t)
	rc, sc := newClient(t, radarr), newClient(t, sonarr)

	var a, b Tag
	if err := rc.call(ctx, http.MethodPost, "/tag", nil, Tag{Label: "SC-Test2"}, &a); err != nil || a.Label != "sc-test2" {
		t.Fatalf("radarr POST SC-Test2 = %+v, %v", a, err)
	}
	if err := rc.call(ctx, http.MethodPost, "/tag", nil, Tag{Label: "sc-test2"}, &b); err != nil || b.ID != a.ID {
		t.Errorf("radarr POST again = %+v, %v; want id %d", b, err, a.ID)
	}
	for _, label := range []string{"sc kinder", "jürg", "sc_kinder"} {
		err := rc.call(ctx, http.MethodPost, "/tag", nil, Tag{Label: label}, nil)
		ae, _ := errors.AsType[*Error](err)
		if !errors.Is(err, ErrValidation) || ae == nil || ae.Message != "Label: "+arrtest.TextLabel {
			t.Errorf("radarr POST %q = %v, want ErrValidation with the label message", label, err)
		}
		if err := sc.call(ctx, http.MethodPost, "/tag", nil, Tag{Label: label}, nil); err != nil {
			t.Errorf("sonarr POST %q = %v, want 201", label, err)
		}
	}
}

func TestFindTitles(t *testing.T) {
	ctx := context.Background()
	radarr := arrtest.NewRadarr(t)
	radarr.AddMovie(arrtest.Movie{ID: 1, TMDBID: 603, Title: "Matrix", Tags: []int{4}, HasFile: true})
	radarr.AddMovie(arrtest.Movie{ID: 2, TMDBID: 604, Title: "Reloaded"})
	rc := newClient(t, radarr)

	m, err := rc.MovieByTMDB(ctx, 603)
	if err != nil || m.ID != 1 || !m.HasFile || !slices.Equal(m.Tags, []int{4}) || m.TMDBID != 603 {
		t.Errorf("movie 603 = %+v, %v", m, err)
	}
	if m, err := rc.MovieByTMDB(ctx, 604); err != nil || m.ID != 2 || m.HasFile || len(m.Tags) != 0 {
		t.Errorf("movie 604 = %+v, %v", m, err)
	}
	if _, err := rc.MovieByTMDB(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing movie: %v, want ErrNotFound", err)
	}
	if got := radarr.CallLog()[0].Query.Get("tmdbId"); got != "603" {
		t.Errorf("tmdbId sent = %q", got)
	}

	sonarr := arrtest.NewSonarr(t)
	sonarr.AddSeries(arrtest.Series{ID: 7, TVDBID: 81189, Title: "Breaking Bad", Tags: []int{2}, Seasons: []arrtest.Season{
		{Number: 0, EpisodeCount: 0, TotalEpisodeCount: 9},
		{Number: 1, Monitored: true, EpisodeFileCount: 7, EpisodeCount: 7, TotalEpisodeCount: 7},
		{Number: 2, Monitored: true, EpisodeFileCount: 3, EpisodeCount: 13, TotalEpisodeCount: 13},
		{Number: 3, Monitored: true, NoStatistics: true},
	}})
	sc := newClient(t, sonarr)
	s, err := sc.SeriesByTVDB(ctx, 81189)
	if err != nil || s.ID != 7 || s.TVDBID != 81189 || !slices.Equal(s.Tags, []int{2}) {
		t.Fatalf("series = %+v, %v", s, err)
	}
	if s.Statistics != (Statistics{EpisodeFileCount: 10, EpisodeCount: 20, TotalEpisodeCount: 29}) {
		t.Errorf("statistics = %+v", s.Statistics)
	}
	if len(s.Seasons) != 4 || s.Seasons[2].Number != 2 || !s.Seasons[2].Monitored || s.Seasons[2].Statistics == nil ||
		*s.Seasons[2].Statistics != (Statistics{EpisodeFileCount: 3, EpisodeCount: 13, TotalEpisodeCount: 13}) || s.Seasons[3].Statistics != nil {
		t.Errorf("seasons = %+v", s.Seasons)
	}
	if _, err := sc.SeriesByTVDB(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing series: %v, want ErrNotFound", err)
	}

	// The other service's lookup and invalid ids are refused before any call.
	radarr.ResetCalls()
	sonarr.ResetCalls()
	if _, err := rc.SeriesByTVDB(ctx, 81189); !errors.Is(err, ErrInvalid) {
		t.Errorf("series on radarr: %v", err)
	}
	if _, err := sc.MovieByTMDB(ctx, 603); !errors.Is(err, ErrInvalid) {
		t.Errorf("movie on sonarr: %v", err)
	}
	if _, err := rc.MovieByTMDB(ctx, 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("tmdb id 0: %v", err)
	}
	if n := len(radarr.CallLog()) + len(sonarr.CallLog()); n != 0 {
		t.Errorf("refused lookups sent %d calls", n)
	}
}

func TestAttachTag(t *testing.T) {
	ctx := context.Background()
	radarr := arrtest.NewRadarr(t)
	radarr.AddMovie(arrtest.Movie{ID: 1, TMDBID: 603, Tags: []int{4}})
	rc := newClient(t, radarr)
	if err := rc.AttachTag(ctx, 9, 1); err != nil {
		t.Fatal(err)
	}
	if err := rc.AttachTag(ctx, 9, 1); err != nil { // twice changes nothing
		t.Fatal(err)
	}
	if m, _ := radarr.Movie(1); !slices.Equal(m.Tags, []int{4, 9}) {
		t.Errorf("movie tags = %v, want [4 9]", m.Tags)
	}
	var body map[string]any
	if err := json.Unmarshal(radarr.CallLog()[0].Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["applyTags"] != "add" || fmt.Sprint(body["movieIds"]) != "[1]" || fmt.Sprint(body["tags"]) != "[9]" || len(body) != 3 {
		t.Errorf("radarr editor body = %v", body)
	}

	sonarr := arrtest.NewSonarr(t)
	sonarr.AddSeries(arrtest.Series{ID: 7, TVDBID: 81189, Tags: []int{2}})
	sonarr.AddSeries(arrtest.Series{ID: 8, TVDBID: 81190})
	sc := newClient(t, sonarr)
	if err := sc.AttachTag(ctx, 3, 7, 8); err != nil {
		t.Fatal(err)
	}
	s7, _ := sonarr.Series(7)
	s8, _ := sonarr.Series(8)
	if !slices.Equal(s7.Tags, []int{2, 3}) || !slices.Equal(s8.Tags, []int{3}) {
		t.Errorf("series tags = %v, %v", s7.Tags, s8.Tags)
	}
	call := sonarr.CallLog()[0]
	if call.Route != "PUT /api/v3/series/editor" || !bytes.Contains(call.Body, []byte(`"applyTags":"add"`)) || !bytes.Contains(call.Body, []byte(`"seriesIds":[7,8]`)) {
		t.Errorf("sonarr editor call = %s %s", call.Route, call.Body)
	}

	sonarr.ResetCalls()
	for _, bad := range [][]int{{0}, {}, {7, -1}} {
		if err := sc.AttachTag(ctx, 3, bad...); !errors.Is(err, ErrInvalid) {
			t.Errorf("AttachTag(3, %v) = %v", bad, err)
		}
	}
	if err := sc.AttachTag(ctx, 0, 7); !errors.Is(err, ErrInvalid) {
		t.Errorf("tag id 0: %v", err)
	}
	if n := len(sonarr.CallLog()); n != 0 {
		t.Errorf("refused attaches sent %d calls", n)
	}
}

func TestQueueRadarrTwoPages(t *testing.T) {
	f := arrtest.NewRadarr(t)
	f.SetQueue(arrtest.RadarrQueue()...)
	f.SetMaxPageSize(2) // four records on two pages
	q, err := newClient(t, f).Queue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n := f.Calls("GET /api/v3/queue"); n != 2 {
		t.Errorf("%d queue calls, want 2", n)
	}
	for i, c := range f.CallLog() {
		if c.Query.Get("page") != fmt.Sprint(i+1) || c.Query.Get("pageSize") != fmt.Sprint(QueuePageSize) {
			t.Errorf("call %d query = %v", i, c.Query)
		}
	}
	if q.Total != 4 || q.Skipped != 1 || len(q.Records) != 3 {
		t.Fatalf("queue = total %d skipped %d records %d", q.Total, q.Skipped, len(q.Records))
	}
	down, stuck, bare := q.Records[0], q.Records[1], q.Records[2]
	if down.ID != 101 || down.TitleID != 1 || !down.SizeKnown || down.Size != 1000 || down.SizeLeft != 550 ||
		!down.TimeLeftKnown || down.TimeLeft != 12*time.Minute || !down.EstimatedEnd.Equal(time.Date(2026, 11, 17, 12, 12, 0, 0, time.UTC)) ||
		down.Status != "downloading" || down.TrackedDownloadState != "downloading" || down.TrackedDownloadStatus != "ok" ||
		down.DownloadClient != "SABnzbd" || down.Protocol != "usenet" || down.SeasonKnown || len(down.StatusMessages) != 0 {
		t.Errorf("downloading record = %+v", down)
	}
	if stuck.TitleID != 2 || stuck.Status != "completed" || stuck.TrackedDownloadState != "importBlocked" || stuck.TrackedDownloadStatus != "warning" ||
		len(stuck.StatusMessages) != 1 || stuck.StatusMessages[0].Title != "Film.Two.2023.2160p" ||
		!slices.Equal(stuck.StatusMessages[0].Messages, []string{"No files found are eligible for import"}) || stuck.SizeLeft != 0 || !stuck.SizeKnown {
		t.Errorf("stuck record = %+v", stuck)
	}
	if bare.TitleID != 3 || bare.SizeKnown || bare.TimeLeftKnown || !bare.EstimatedEnd.IsZero() || bare.Status != "queued" || bare.TrackedDownloadStatus != "" {
		t.Errorf("record with missing fields = %+v", bare)
	}
	// The output path is never read into a record.
	if s := fmt.Sprintf("%+v", q); strings.Contains(s, "/downloads") {
		t.Errorf("queue carries a path: %s", s)
	}
}

func TestQueueSonarr(t *testing.T) {
	f := arrtest.NewSonarr(t)
	f.SetQueue(arrtest.SonarrQueue()...)
	q, err := newClient(t, f).Queue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.Calls("GET /api/v3/queue") != 1 || q.Total != 3 || q.Skipped != 1 || len(q.Records) != 2 {
		t.Fatalf("queue = %+v, calls %d", q, f.Calls("GET /api/v3/queue"))
	}
	a, b := q.Records[0], q.Records[1]
	if a.TitleID != 7 || a.EpisodeID != 71 || !a.SeasonKnown || a.SeasonNumber != 1 || a.Size != 2000 || a.SizeLeft != 500 ||
		!a.TimeLeftKnown || a.TimeLeft != 26*time.Hour+3*time.Minute+4*time.Second {
		t.Errorf("record a = %+v", a)
	}
	if b.TitleID != 7 || b.EpisodeID != 72 || b.TimeLeftKnown || b.TimeLeft != 0 {
		t.Errorf("record b = %+v", b)
	}

	// An empty queue is one call.
	f.SetQueue()
	f.ResetCalls()
	q, err = newClient(t, f).Queue(context.Background())
	if err != nil || len(q.Records) != 0 || q.Skipped != 0 || f.Calls("GET /api/v3/queue") != 1 {
		t.Errorf("empty queue = %+v, %v, calls %d", q, err, f.Calls("GET /api/v3/queue"))
	}
}

func TestQueueManyPages(t *testing.T) {
	f := arrtest.NewRadarr(t)
	var recs []arrtest.Record
	for i := 1; i <= 450; i++ {
		recs = append(recs, arrtest.Record{"id": i, "movieId": i, "size": 10.0, "sizeleft": 5.0})
	}
	f.SetQueue(recs...)
	q, err := newClient(t, f).Queue(context.Background())
	if err != nil || len(q.Records) != 450 || f.Calls("GET /api/v3/queue") != 3 {
		t.Errorf("records %d, err %v, calls %d", len(q.Records), err, f.Calls("GET /api/v3/queue"))
	}
	// A service that keeps answering the same records never ends the loop: the client stops.
	f.SetMaxPageSize(1)
	f.SetFailure("GET /api/v3/queue", arrtest.Failure{Status: 200, Body: `{"page":1,"pageSize":1,"totalRecords":100000,"records":[{"id":1,"movieId":1}]}`})
	if _, err := newClient(t, f).Queue(context.Background()); !errors.Is(err, ErrServer) {
		t.Errorf("endless queue: %v", err)
	}
}

// allowedRoutes are the only calls package arr may make (rule 11, architecture 2.6).
var allowedRoutes = map[arrtest.Kind][]string{
	arrtest.Radarr: {"GET /api/v3/tag", "POST /api/v3/tag", "GET /api/v3/movie", "PUT /api/v3/movie/editor", "GET /api/v3/queue"},
	arrtest.Sonarr: {"GET /api/v3/tag", "POST /api/v3/tag", "GET /api/v3/series", "PUT /api/v3/series/editor", "GET /api/v3/queue"},
}

// TestOnlyAllowedEndpoints runs every method of the client against both fakes and proves from
// the call log that nothing but the allowed endpoints was called, the editor only with "add".
func TestOnlyAllowedEndpoints(t *testing.T) {
	ctx := context.Background()
	radarr, sonarr := arrtest.NewRadarr(t), arrtest.NewSonarr(t)
	radarr.AddMovie(arrtest.Movie{ID: 1, TMDBID: 603})
	sonarr.AddSeries(arrtest.Series{ID: 7, TVDBID: 81189})
	radarr.SetQueue(arrtest.RadarrQueue()...)
	sonarr.SetQueue(arrtest.SonarrQueue()...)
	for _, f := range []*arrtest.Server{radarr, sonarr} {
		c := newClient(t, f)
		_, _ = c.Tags(ctx)
		tag, _, err := c.EnsureTag(ctx, "sc-noah")
		if err != nil {
			t.Fatal(err)
		}
		_, _, _ = c.EnsureTag(ctx, "sc-noah")
		_, _ = c.MovieByTMDB(ctx, 603)
		_, _ = c.MovieByTMDB(ctx, 999)
		_, _ = c.SeriesByTVDB(ctx, 81189)
		_, _ = c.SeriesByTVDB(ctx, 1)
		if err := c.AttachTag(ctx, tag.ID, map[arrtest.Kind]int{arrtest.Radarr: 1, arrtest.Sonarr: 7}[f.Kind]); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Queue(ctx); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(slices.Sorted(slices.Values(f.Routes())), slices.Sorted(slices.Values(allowedRoutes[f.Kind]))) {
			t.Errorf("%s fake routes %v", f.Kind, f.Routes())
		}
		seen := map[string]bool{}
		for _, call := range f.CallLog() {
			seen[call.Route] = true
			if !slices.Contains(allowedRoutes[f.Kind], call.Route) {
				t.Errorf("%s: call outside the allowed endpoints: %s", f.Kind, call.Route)
			}
			if strings.Contains(call.Path, "notification") || strings.Contains(call.Path, "command") {
				t.Errorf("%s: forbidden call %s", f.Kind, call.Route)
			}
			if strings.HasSuffix(call.Route, "/editor") && !bytes.Contains(call.Body, []byte(`"applyTags":"add"`)) {
				t.Errorf("%s: editor without add: %s", f.Kind, call.Body)
			}
			if bytes.Contains(call.Body, []byte("remove")) || bytes.Contains(call.Body, []byte("replace")) {
				t.Errorf("%s: body removes or replaces: %s", f.Kind, call.Body)
			}
			if !call.HasKey {
				t.Errorf("%s: %s without key", f.Kind, call.Route)
			}
		}
		for _, r := range allowedRoutes[f.Kind] {
			if !seen[r] {
				t.Errorf("%s: %s never called; the test does not cover it", f.Kind, r)
			}
		}
		if n := f.Calls("POST /api/v3/tag"); n != 1 {
			t.Errorf("%s: %d tag creations, want 1", f.Kind, n)
		}
	}
}
