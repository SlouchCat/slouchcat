package metadata

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const traktTestID = "trakt-secret-client-id"

// traktKeys is a key source with a Trakt client id only.
type traktKeys struct {
	staticKey
	id string
}

func (k traktKeys) TraktClientID(context.Context) (string, error) { return k.id, nil }

// fakeTrakt serves the four public lists: trending in the {watchers, movie|show} shape, popular
// flat, one entry without a TMDB id in each; status != 0 answers with that status. popularHits
// counts the requests of a popular list, which a run must not fetch (the spotlight configuration
// of migration 00021 holds the one list trending).
type fakeTrakt struct {
	srv         *httptest.Server
	status      atomic.Int32
	hits        atomic.Int32
	popularHits atomic.Int32
}

func newFakeTrakt(t *testing.T) *fakeTrakt {
	t.Helper()
	f := &fakeTrakt{}
	mux := http.NewServeMux()
	serve := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.hits.Add(1)
			if r.Header.Get("trakt-api-key") != traktTestID || r.Header.Get("trakt-api-version") != "2" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if s := f.status.Load(); s != 0 {
				w.WriteHeader(int(s))
				return
			}
			_, _ = w.Write([]byte(body))
		}
	}
	mux.HandleFunc("GET /movies/trending", serve(`[{"watchers": 3, "movie": {"title": "Film A", "year": 2020, "ids": {"tmdb": 11}}},
		{"watchers": 2, "movie": {"title": "Ohne", "year": 2021, "ids": {"slug": "ohne"}}},
		{"watchers": 1, "movie": {"title": "Film B", "year": 2022, "ids": {"tmdb": 12}}}]`))
	mux.HandleFunc("GET /shows/trending", serve(`[{"watchers": 5, "show": {"title": "Serie A", "year": 2019, "ids": {"tmdb": 21}}}]`))
	popular := func(body string) http.HandlerFunc {
		h := serve(body)
		return func(w http.ResponseWriter, r *http.Request) {
			f.popularHits.Add(1)
			h(w, r)
		}
	}
	mux.HandleFunc("GET /movies/popular", popular(`[{"title": "Film C", "year": 1999, "ids": {"tmdb": 13}}, {"title": "Ohne", "ids": {}}]`))
	mux.HandleFunc("GET /shows/popular", popular(`[{"title": "Serie B", "ids": {"tmdb": 22}}, {"title": "Serie C", "year": 2001, "ids": {"tmdb": 23}}]`))
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

type traktEnv struct {
	db    *store.DB
	svc   *Service
	queue *jobs.Queue
	clock *testClock
	fake  *fakeTrakt
	logs  *bytes.Buffer
	ring  *logging.ErrorRing
}

// newTraktEnv returns a service with the fake Trakt; fixedID is passed to WithTrakt and keys
// answers KeySource.TraktClientID.
func newTraktEnv(t *testing.T, fixedID string, keys KeySource) *traktEnv {
	t.Helper()
	logs := &bytes.Buffer{}
	ring := logging.NewErrorRing(logging.ErrorRingSize)
	log := slog.New(logging.NewRedactingHandler(ring.Wrap(slog.NewTextHandler(&lockedWriter{w: logs}, nil))))
	db, err := store.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clock := &testClock{t: time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)}
	fake := newFakeTrakt(t)
	q := jobs.NewQueue(db.Write, clock.now)
	svc := New(db, q, keys, t.TempDir(), transcode.Runner{}, log, WithTrakt(fixedID, WithTraktBaseURL(fake.srv.URL),
		WithTraktHTTPClient(fake.srv.Client()), WithTraktClock(clock.now), WithTraktLimiter(ratelimit.New("trakt-test", 1000, 100))))
	return &traktEnv{db: db, svc: svc, queue: q, clock: clock, fake: fake, logs: logs, ring: ring}
}

// rows returns list_key/media/position/tmdb/year of every stored spotlight entry in key order.
func (e *traktEnv) rows(t *testing.T) []string {
	t.Helper()
	rs, err := e.db.Read.Query(`SELECT list_key || ' ' || media || ' ' || position || ' ' || tmdb_id || ' ' || COALESCE(year, 0)
	                            FROM spotlight_entries ORDER BY list_key, media, position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var s string
		if err := rs.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rs.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *traktEnv) status(t *testing.T) model.TraktStatus {
	t.Helper()
	st, err := e.svc.TraktStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// E-7, I-7, ADR 0011: a run replaces the spotlight entries with the trending list of both media
// under list_key trending (entries without a TMDB id skipped, the popular lists not fetched), a
// failure empties them and records the error, the next success clears it. The client id never
// reaches the log.
func TestTraktRefresh(t *testing.T) {
	t.Parallel()
	e := newTraktEnv(t, traktTestID, staticKey(""))
	ctx := context.Background()
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"trending movie 1 11 2020", "trending movie 3 12 2022", "trending show 1 21 2019"}
	if got := e.rows(t); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if n := e.fake.hits.Load(); n != 2 || e.fake.popularHits.Load() != 0 {
		t.Errorf("requests = %d (popular %d), want the two trending lists only", n, e.fake.popularHits.Load())
	}
	first := e.clock.now()
	st := e.status(t)
	if !st.Configured || st.ListKey != "trending" || st.Entries != 3 || st.LastError != "" ||
		st.LastSuccess == nil || !st.LastSuccess.Equal(first) || st.LastAttempt == nil || !st.LastAttempt.Equal(first) {
		t.Errorf("status after success = %+v", st)
	}

	// 503: the rows disappear (I-7), the error is stored, the job retries.
	e.clock.set(first.Add(time.Hour))
	e.fake.status.Store(http.StatusServiceUnavailable)
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err == nil {
		t.Error("503 run returned no error")
	}
	if got := e.rows(t); len(got) != 0 {
		t.Errorf("rows after 503 = %v", got)
	}
	st = e.status(t)
	if st.Entries != 0 || st.ListKey != "trending" || !strings.Contains(st.LastError, "503") || !st.LastSuccess.Equal(first) ||
		!st.LastAttempt.Equal(first.Add(time.Hour)) {
		t.Errorf("status after 503 = %+v", st)
	}
	// A-3: the failure is an error record of its own for the dashboard, with Trakt as its source.
	if errs := e.ring.Entries(); len(errs) != 1 || errs[0].Source != "trakt" ||
		errs[0].Message != "trakt refresh failed; lists emptied" || !strings.Contains(errs[0].Attrs["err"], "503") {
		t.Errorf("error records after 503 = %+v", errs)
	}

	// 401: emptied as well, but no retry.
	e.fake.status.Store(http.StatusUnauthorized)
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Errorf("401 run: %v", err)
	}
	if st := e.status(t); !strings.Contains(st.LastError, "401") {
		t.Errorf("status after 401 = %+v", st)
	}
	if errs := e.ring.Entries(); len(errs) != 2 || errs[0].Source != "trakt" || !strings.Contains(errs[0].Attrs["err"], "401") {
		t.Errorf("error records after 401 = %+v", errs)
	}

	e.clock.set(first.Add(2 * time.Hour))
	e.fake.status.Store(0)
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	if st := e.status(t); st.LastError != "" || st.Entries != 3 || !st.LastSuccess.Equal(first.Add(2*time.Hour)) {
		t.Errorf("status after recovery = %+v", st)
	}
	if strings.Contains(e.logs.String(), traktTestID) {
		t.Errorf("log names the client id:\n%s", e.logs.String())
	}
	if errs := e.ring.Entries(); len(errs) != 2 {
		t.Errorf("error records after recovery = %+v", errs)
	}
	if n := e.fake.popularHits.Load(); n != 0 {
		t.Errorf("popular requests = %d, want none", n)
	}
	var stored int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM settings WHERE value LIKE '%' || ? || '%'`, traktTestID).Scan(&stored); err != nil || stored != 0 {
		t.Errorf("settings hold the client id: %d %v", stored, err)
	}
}

// E-7: the schedule queues the first run right away, the next one 24 hours after the last
// attempt, none in between; the admin refresh queues one any time.
func TestTraktSchedule(t *testing.T) {
	t.Parallel()
	e := newTraktEnv(t, "", traktKeys{id: traktTestID})
	ctx := context.Background()
	start := e.clock.now()
	due := func(at time.Duration) bool {
		t.Helper()
		e.clock.set(start.Add(at))
		ok, err := e.svc.QueueDueTrakt(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !due(0) {
		t.Fatal("first run not queued")
	}
	// The worker runs the job.
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Duration{15 * time.Minute, 12 * time.Hour, 24*time.Hour - time.Minute} {
		if due(at) {
			t.Errorf("queued again after %v", at)
		}
	}
	if !due(24 * time.Hour) {
		t.Error("not queued after 24 hours")
	}
	var queued int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = 'trakt' AND priority = 2`,
		JobTraktRefresh).Scan(&queued); err != nil || queued != 1 {
		t.Errorf("trakt jobs = %d %v, want one active job", queued, err)
	}
	if err := e.svc.RefreshTrakt(ctx); err != nil {
		t.Errorf("refresh: %v", err)
	}
}

// RunTrakt checks at once on start.
// Not parallel: this test waits on the clock (an interval, a debounce window or a
// cancellation), and parallel load would stretch the wait past what it asserts.
func TestRunTraktStartsAtOnce(t *testing.T) {
	e := newTraktEnv(t, traktTestID, staticKey(""))
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() { e.svc.RunTrakt(ctx) })
	defer func() { cancel(); wg.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var n int
		if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ?`, JobTraktRefresh).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("RunTrakt queued nothing at the start")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Without a client id there is no job and the status says so.
func TestTraktNotConfigured(t *testing.T) {
	t.Parallel()
	e := newTraktEnv(t, "", staticKey(""))
	ctx := context.Background()
	if ok, err := e.svc.QueueDueTrakt(ctx); ok || err != nil {
		t.Errorf("queue due without client id: %v %v", ok, err)
	}
	if err := e.svc.RefreshTrakt(ctx); !errors.Is(err, model.ErrProviderNotConfigured) {
		t.Errorf("refresh without client id: %v", err)
	}
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil || e.fake.hits.Load() != 0 {
		t.Errorf("job without client id: %v, %d requests", err, e.fake.hits.Load())
	}
	if st := e.status(t); st.Configured || st.LastAttempt != nil {
		t.Errorf("status = %+v", st)
	}
	// A service without a key source at all (the podcast service of the API fixtures).
	svc := New(e.db, e.queue, nil, t.TempDir(), transcode.Runner{}, slog.New(slog.DiscardHandler))
	if st, err := svc.TraktStatus(ctx); err != nil || st.Configured {
		t.Errorf("status without key source = %+v %v", st, err)
	}
}

// W1 (M7 review), S-8: while the client id stays but no run succeeds for two intervals (a hung
// job), the status counts no entries, like the start page (visibility.SpotlightRow). The staleness
// limit is twice spotlight_config.interval_hours, not a constant: two intervals prove that the
// limit follows the configuration, with the boundary inclusive. Removing the client id empties the
// stored entries, so the start page drops the spotlight row as documented.
func TestTraktRowsGoWithoutClientID(t *testing.T) {
	t.Parallel()
	keys := &traktKeys{id: traktTestID}
	e := newTraktEnv(t, "", keys)
	ctx := context.Background()
	start := e.clock.now()
	for _, hours := range []int{6, 48} {
		if _, err := e.db.Write.Exec(`UPDATE spotlight_config SET interval_hours = ?`, hours); err != nil {
			t.Fatal(err)
		}
		e.clock.set(start)
		if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
			t.Fatal(err)
		}
		if st := e.status(t); st.Entries != 3 || st.ListKey != "trending" {
			t.Fatalf("interval %dh: status after the run = %+v", hours, st)
		}
		limit := 2 * time.Duration(hours) * time.Hour
		// A hung job: the entries stay stored but are stale after two intervals.
		e.clock.set(start.Add(limit - time.Minute))
		if st := e.status(t); st.Entries != 3 {
			t.Errorf("interval %dh: status just before the entries turn stale = %+v", hours, st)
		}
		e.clock.set(start.Add(limit))
		if st := e.status(t); st.Entries != 3 {
			t.Errorf("interval %dh: status when the entries turn stale = %+v (boundary is inclusive)", hours, st)
		}
		e.clock.set(start.Add(limit + time.Minute))
		if st := e.status(t); st.Entries != 0 || !st.Configured {
			t.Errorf("interval %dh: status with stale entries = %+v, want no entries", hours, st)
		}
		if len(e.rows(t)) == 0 {
			t.Fatalf("interval %dh: stale entries were deleted, want them kept until the next run", hours)
		}
	}

	// The client id goes: the next tick empties the table and queues nothing.
	keys.id = ""
	if ok, err := e.svc.QueueDueTrakt(ctx); ok || err != nil {
		t.Errorf("queue due without client id: %v %v", ok, err)
	}
	if got := e.rows(t); len(got) != 0 {
		t.Errorf("rows without client id = %v", got)
	}
	if st := e.status(t); st.Configured || st.Entries != 0 {
		t.Errorf("status without client id = %+v", st)
	}
	// A second tick finds nothing to empty.
	if ok, err := e.svc.QueueDueTrakt(ctx); ok || err != nil {
		t.Errorf("second tick without client id: %v %v", ok, err)
	}
}
