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

// fakeTrakt serves the four lists: trending in the {watchers, movie|show} shape, popular flat,
// one entry without a TMDB id in each; status != 0 answers with that status.
type fakeTrakt struct {
	srv    *httptest.Server
	status atomic.Int32
	hits   atomic.Int32
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
	mux.HandleFunc("GET /movies/popular", serve(`[{"title": "Film C", "year": 1999, "ids": {"tmdb": 13}}, {"title": "Ohne", "ids": {}}]`))
	mux.HandleFunc("GET /shows/popular", serve(`[{"title": "Serie B", "ids": {"tmdb": 22}}, {"title": "Serie C", "year": 2001, "ids": {"tmdb": 23}}]`))
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
}

// newTraktEnv returns a service with the fake Trakt; fixedID is passed to WithTrakt and keys
// answers KeySource.TraktClientID.
func newTraktEnv(t *testing.T, fixedID string, keys KeySource) *traktEnv {
	t.Helper()
	logs := &bytes.Buffer{}
	log := slog.New(logging.NewRedactingHandler(slog.NewTextHandler(&lockedWriter{w: logs}, nil)))
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
	return &traktEnv{db: db, svc: svc, queue: q, clock: clock, fake: fake, logs: logs}
}

// rows returns list/media/position/tmdb of every stored entry in key order.
func (e *traktEnv) rows(t *testing.T) []string {
	t.Helper()
	rs, err := e.db.Read.Query(`SELECT list || ' ' || media || ' ' || position || ' ' || tmdb_id || ' ' || COALESCE(year, 0)
	                            FROM trakt_lists ORDER BY list DESC, media, position`)
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

// E-7, I-7: a run replaces the lists (entries without a TMDB id skipped), a failure empties them
// and records the error, the next success clears it. The client id never reaches the log.
func TestTraktRefresh(t *testing.T) {
	e := newTraktEnv(t, traktTestID, staticKey(""))
	ctx := context.Background()
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"trending movie 1 11 2020", "trending movie 3 12 2022", "trending show 1 21 2019",
		"popular movie 1 13 1999", "popular show 1 22 0", "popular show 2 23 2001"}
	if got := e.rows(t); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	first := e.clock.now()
	st := e.status(t)
	if !st.Configured || st.Trending != 3 || st.Popular != 3 || st.LastError != "" ||
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
	if st.Trending != 0 || st.Popular != 0 || !strings.Contains(st.LastError, "503") || !st.LastSuccess.Equal(first) ||
		!st.LastAttempt.Equal(first.Add(time.Hour)) {
		t.Errorf("status after 503 = %+v", st)
	}

	// 401: emptied as well, but no retry.
	e.fake.status.Store(http.StatusUnauthorized)
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Errorf("401 run: %v", err)
	}
	if st := e.status(t); !strings.Contains(st.LastError, "401") {
		t.Errorf("status after 401 = %+v", st)
	}

	e.clock.set(first.Add(2 * time.Hour))
	e.fake.status.Store(0)
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	if st := e.status(t); st.LastError != "" || st.Trending != 3 || !st.LastSuccess.Equal(first.Add(2*time.Hour)) {
		t.Errorf("status after recovery = %+v", st)
	}
	if strings.Contains(e.logs.String(), traktTestID) {
		t.Errorf("log names the client id:\n%s", e.logs.String())
	}
	var stored int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM settings WHERE value LIKE '%' || ? || '%'`, traktTestID).Scan(&stored); err != nil || stored != 0 {
		t.Errorf("settings hold the client id: %d %v", stored, err)
	}
}

// E-7: the schedule queues the first run right away, the next one 24 hours after the last
// attempt, none in between; the admin refresh queues one any time.
func TestTraktSchedule(t *testing.T) {
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
