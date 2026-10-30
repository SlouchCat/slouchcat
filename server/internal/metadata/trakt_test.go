package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
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
	"github.com/slouchcat/slouchcat/server/internal/trakt"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const traktTestID = "trakt-secret-client-id"

// traktKeys is a key source with a Trakt client id only.
type traktKeys struct {
	staticKey
	id string
}

func (k traktKeys) TraktClientID(context.Context) (string, error) { return k.id, nil }

// fakeTrakt serves the public lists of the spotlight kinds the tests configure: trending in the
// {watchers, movie|show} shape, popular flat, boxoffice in the {revenue, movie} shape, one entry
// without a TMDB id in each, and the user list jane-doe/best-ever with a movie, a season and a
// show. status != 0 answers with that status, empty answers every list with [], delay holds every
// answer that long (or until the client gives up). Every request path is recorded in order.
type fakeTrakt struct {
	srv    *httptest.Server
	status atomic.Int32
	empty  atomic.Bool
	delay  atomic.Int64 // nanoseconds
	hits   atomic.Int32

	mu    sync.Mutex
	paths []string
	times []time.Time
}

// traktUserList is the address of the user list the fake serves; its user and slug must never
// reach the log, a stored error or an error record (rule 10, plan 8.4).
const traktUserList = "https://trakt.tv/users/jane-doe/lists/best-ever"

func newFakeTrakt(t *testing.T) *fakeTrakt {
	t.Helper()
	f := &fakeTrakt{}
	mux := http.NewServeMux()
	serve := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.hits.Add(1)
			f.mu.Lock()
			f.paths = append(f.paths, r.URL.Path)
			f.times = append(f.times, time.Now())
			f.mu.Unlock()
			if d := f.delay.Load(); d > 0 {
				select {
				case <-time.After(time.Duration(d)):
				case <-r.Context().Done():
					return
				}
			}
			if r.Header.Get("trakt-api-key") != traktTestID || r.Header.Get("trakt-api-version") != "2" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if s := f.status.Load(); s != 0 {
				w.WriteHeader(int(s))
				return
			}
			if f.empty.Load() {
				_, _ = w.Write([]byte(`[]`))
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
	mux.HandleFunc("GET /movies/boxoffice", serve(`[{"revenue": 9000, "movie": {"title": "Ohne", "ids": {"slug": "ohne"}}},
		{"revenue": 7000, "movie": {"title": "Film D", "year": 2026, "ids": {"tmdb": 31}}}]`))
	mux.HandleFunc("GET /users/jane-doe/lists/best-ever/items", serve(`[
		{"rank": 1, "type": "movie", "movie": {"title": "Film L", "year": 2010, "ids": {"tmdb": 41}}},
		{"rank": 2, "type": "season", "season": {"number": 1, "ids": {"tmdb": 4200}}, "show": {"title": "Serie S", "ids": {"tmdb": 42}}},
		{"rank": 3, "type": "show", "show": {"title": "Serie L", "year": 2011, "ids": {"tmdb": 43}}}]`))
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// requests returns the recorded request paths in order.
func (f *fakeTrakt) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.paths)
}

// requestTimes returns the arrival times of the recorded requests in order.
func (f *fakeTrakt) requestTimes() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.times)
}

// forget drops the recorded requests.
func (f *fakeTrakt) forget() {
	f.mu.Lock()
	f.paths, f.times = nil, nil
	f.mu.Unlock()
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
// answers KeySource.TraktClientID. opts are applied after the defaults and replace them.
func newTraktEnv(t *testing.T, fixedID string, keys KeySource, opts ...TraktOption) *traktEnv {
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
	opts = append([]TraktOption{WithTraktBaseURL(fake.srv.URL), WithTraktHTTPClient(fake.srv.Client()),
		WithTraktClock(clock.now), WithTraktLimiter(ratelimit.New("trakt-test", 1000, 100))}, opts...)
	svc := New(db, q, keys, t.TempDir(), transcode.Runner{}, log, WithTrakt(fixedID, opts...))
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

// configure writes set (an SQL SET clause with args) straight into the singleton
// spotlight_config, past the validation of the administration.
func (e *traktEnv) configure(t *testing.T, set string, args ...any) {
	t.Helper()
	if _, err := e.db.Write.Exec(`UPDATE spotlight_config SET `+set, args...); err != nil {
		t.Fatal(err)
	}
}

// leaks returns where each of secrets appears: in the log, in a stored setting (the stored error
// among them) or in a message or attribute of an error record.
func (e *traktEnv) leaks(t *testing.T, secrets ...string) []string {
	t.Helper()
	var settings string
	if err := e.db.Read.QueryRow(`SELECT COALESCE(group_concat(key || '=' || value, char(10)), '') FROM settings`).Scan(&settings); err != nil {
		t.Fatal(err)
	}
	places := map[string]string{"log": e.logs.String(), "settings": settings}
	for i, r := range e.ring.Entries() {
		text := r.Message
		for k, v := range r.Attrs {
			text += "\n" + k + "=" + v
		}
		places[fmt.Sprintf("error record %d", i)] = text
	}
	var out []string
	for where, text := range places {
		for _, s := range secrets {
			if strings.Contains(text, s) {
				out = append(out, where+" names "+s)
			}
		}
	}
	return out
}

// E-7, I-7, ADR 0011: with the spotlight_config of migration 00021 (trending, both media) a run
// replaces the spotlight entries with the trending list of both media under list_key trending
// (entries without a TMDB id skipped, no other list fetched), a failure empties them, records the
// error and logs exactly one error record, the next success clears it. The client id never reaches
// the log.
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
	if got := e.fake.requests(); !slices.Equal(got, []string{"/movies/trending", "/shows/trending"}) {
		t.Errorf("requests = %v, want the two trending lists only", got)
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
	// Plan 8.4: exactly one Error record per failed run, in the log as well.
	if n := strings.Count(e.logs.String(), "level=ERROR"); n != 1 {
		t.Errorf("log has %d error lines after 503, want 1:\n%s", n, e.logs.String())
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
	for _, p := range e.fake.requests() {
		if p != "/movies/trending" && p != "/shows/trending" {
			t.Errorf("request to %s, want the trending lists only", p)
		}
	}
	var stored int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM settings WHERE value LIKE '%' || ? || '%'`, traktTestID).Scan(&stored); err != nil || stored != 0 {
		t.Errorf("settings hold the client id: %d %v", stored, err)
	}
}

// E-7, S-8, concept 09: the Run of the schedule trakt.refresh queues the nightly job with the
// priority the scheduler passes, and the seed of the first schedule row takes the interval from
// spotlight_config and the last attempt of the earlier poller. The default of migration 00021
// (24 hours) and both bounds of the administration (6 and 48 hours) prove that the interval
// follows the configuration. The manual refresh raises the waiting job and is never lowered.
func TestTraktSchedule(t *testing.T) {
	t.Parallel()
	for _, hours := range []int{24, 6, 48} {
		t.Run(fmt.Sprintf("%dh", hours), func(t *testing.T) {
			t.Parallel()
			e := newTraktEnv(t, "", traktKeys{id: traktTestID})
			if hours != 24 {
				e.configure(t, `interval_hours = ?`, hours)
			}
			ctx := context.Background()
			start := e.clock.now()
			interval := time.Duration(hours) * time.Hour
			// No attempt yet: the first row is due now.
			seed, err := e.svc.traktSeed(ctx)
			if err != nil || seed.Every != interval || !seed.Enabled || !seed.LastRun.IsZero() {
				t.Fatalf("seed before any run = %+v, %v; want every %v, enabled, no last run", seed, err, interval)
			}
			ids, err := e.svc.runTrakt(ctx, e.queue, traktPriority)
			if err != nil || len(ids) != 1 {
				t.Fatalf("scheduled run: %v, %v; want one job", ids, err)
			}
			// The worker runs the job; the seed now continues from the attempt.
			if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
				t.Fatal(err)
			}
			if seed, err := e.svc.traktSeed(ctx); err != nil || !seed.LastRun.Equal(start) {
				t.Errorf("seed after the run = %+v, %v; want the last attempt %v", seed, err, start)
			}
			var queued int
			if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = 'trakt' AND priority = 2`,
				JobTraktRefresh).Scan(&queued); err != nil || queued != 1 {
				t.Errorf("trakt jobs = %d %v, want one active job", queued, err)
			}
			// "Jetzt abrufen" does not vanish behind the queued nightly run (same key): it raises
			// that job to the manual priority, so it overtakes a scan backlog of priority 2.
			if err := e.svc.RefreshTrakt(ctx); err != nil {
				t.Errorf("refresh: %v", err)
			}
			var manual int
			if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = 'trakt' AND state = 'queued' AND priority = ?`,
				JobTraktRefresh, traktManualPriority).Scan(&manual); err != nil || manual != 1 {
				t.Errorf("queued trakt jobs at manual priority = %d %v, want the one waiting job raised", manual, err)
			}
			// The next scheduled run keeps the raised priority: Enqueue returns the waiting job and
			// the raise never lowers.
			e.clock.set(start.Add(3 * interval))
			if ids, err := e.svc.runTrakt(ctx, e.queue, traktPriority); err != nil || len(ids) != 1 {
				t.Fatalf("scheduled run with a waiting job: %v, %v", ids, err)
			}
			if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND priority = ?`,
				JobTraktRefresh, traktManualPriority).Scan(&manual); err != nil || manual != 1 {
				t.Errorf("queued trakt jobs after the nightly run = %d %v, want still one at the manual priority", manual, err)
			}
		})
	}
}

// RegisterTypes registers trakt.refresh as a schedule type with the seed, and podcast.poll as the
// visible 5-minute tick without a handler of its own.
func TestRegisterTypesSchedules(t *testing.T) {
	t.Parallel()
	e := newTraktEnv(t, "", traktKeys{id: traktTestID})
	r := jobs.NewRegistry()
	e.svc.RegisterTypes(r)
	trakt, ok := r.Lookup(JobTraktRefresh)
	if !ok || !trakt.Scheduled() || trakt.Seed == nil || trakt.StartDelay != nil || trakt.Handler == nil || trakt.Priority != traktPriority {
		t.Errorf("trakt.refresh = %+v, want a schedule type with seed, handler and no start delay", trakt)
	}
	poll, ok := r.Lookup(JobPodcastPoll)
	if !ok || !poll.Scheduled() || poll.Handler != nil || poll.Every != feedPollInterval || poll.StartDelay == nil || *poll.StartDelay != 0 {
		t.Errorf("podcast.poll = %+v, want a schedule type every %v at the start without a handler", poll, feedPollInterval)
	}
	if vote, ok := r.Lookup(JobVoteBackfill); !ok || vote.Trigger != jobs.TriggerEvent || vote.Run != nil {
		t.Errorf("metadata.vote_backfill = %+v, want an event type without Run (concept 08 queues it)", vote)
	}
	w := jobs.NewWorker(e.queue, "w", slog.New(slog.DiscardHandler))
	r.Attach(w, jobs.PoolGeneral)
	if len(w.Handlers()) != 7 {
		t.Errorf("handlers attached = %v, want the seven types with a handler", w.Handlers())
	}
}

// Without a client id there is no job and the status says so.
func TestTraktNotConfigured(t *testing.T) {
	t.Parallel()
	e := newTraktEnv(t, "", staticKey(""))
	ctx := context.Background()
	if ids, err := e.svc.runTrakt(ctx, e.queue, traktPriority); len(ids) != 0 || err != nil {
		t.Errorf("scheduled run without client id: %v %v", ids, err)
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

	// The client id goes: the next scheduled run empties the table and queues nothing (the
	// special case stays in the Run of the type, concept 09 4.3).
	keys.id = ""
	if ids, err := e.svc.runTrakt(ctx, e.queue, traktPriority); len(ids) != 0 || err != nil {
		t.Errorf("scheduled run without client id: %v %v", ids, err)
	}
	if got := e.rows(t); len(got) != 0 {
		t.Errorf("rows without client id = %v", got)
	}
	if st := e.status(t); st.Configured || st.Entries != 0 {
		t.Errorf("status without client id = %+v", st)
	}
	// A second run finds nothing to empty.
	if ids, err := e.svc.runTrakt(ctx, e.queue, traktPriority); len(ids) != 0 || err != nil {
		t.Errorf("second run without client id: %v %v", ids, err)
	}
}

// Plan 5.6, ADR 0011: the run fetches the list spotlight_config names, along the path of its kind,
// and replaces the entries of the previous list with it under the new list_key. A kind with media
// both asks its movie and its show path, a single medium only one of them, boxoffice its one movie
// path; a user list is one request whose medium is a filter after the fetch (seasons dropped).
func TestSpotlightFetchFollowsConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, set string
		args      []any
		key       string
		paths     []string
		rows      []string
	}{
		{name: "popular", set: `list_key = 'popular'`, key: "popular",
			paths: []string{"/movies/popular", "/shows/popular"},
			rows:  []string{"popular movie 1 13 1999", "popular show 1 22 0", "popular show 2 23 2001"}},
		{name: "trending movies", set: `media = 'movie'`, key: "trending",
			paths: []string{"/movies/trending"},
			rows:  []string{"trending movie 1 11 2020", "trending movie 3 12 2022"}},
		{name: "popular shows", set: `list_key = 'popular', media = 'show'`, key: "popular",
			paths: []string{"/shows/popular"},
			rows:  []string{"popular show 1 22 0", "popular show 2 23 2001"}},
		{name: "boxoffice", set: `list_key = 'boxoffice', media = 'movie'`, key: "boxoffice",
			paths: []string{"/movies/boxoffice"},
			rows:  []string{"boxoffice movie 2 31 2026"}},
		{name: "user list", set: `list_key = 'url', list_url = ?, media = 'both'`, args: []any{traktUserList}, key: "url",
			paths: []string{"/users/jane-doe/lists/best-ever/items"},
			rows:  []string{"url movie 1 41 2010", "url show 3 43 2011"}},
		{name: "user list movies", set: `list_key = 'url', list_url = ?, media = 'movie'`, args: []any{traktUserList}, key: "url",
			paths: []string{"/users/jane-doe/lists/best-ever/items"},
			rows:  []string{"url movie 1 41 2010"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newTraktEnv(t, traktTestID, staticKey(""))
			ctx := context.Background()
			// A first run with the default list, so the switch has entries to replace.
			if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
				t.Fatal(err)
			}
			e.fake.forget()
			e.configure(t, tc.set, tc.args...)
			e.clock.set(e.clock.now().Add(time.Hour))
			if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
				t.Fatal(err)
			}
			if got := e.fake.requests(); !slices.Equal(got, tc.paths) {
				t.Errorf("requests = %v, want %v", got, tc.paths)
			}
			if got := e.rows(t); !slices.Equal(got, tc.rows) {
				t.Errorf("rows = %v, want %v", got, tc.rows)
			}
			if st := e.status(t); st.ListKey != tc.key || st.Entries != len(tc.rows) || st.LastError != "" {
				t.Errorf("status = %+v", st)
			}
		})
	}
}

// Rule 10, plan 8.4: the address of a user list can name a Trakt user. Neither after a success nor
// after a failure does the user, the list slug or the client id appear in the log, in a stored
// setting (the stored error among them) or in an error record. The success line names the list by
// its key, its media and the first eight hex digits of a SHA-256 of the address, with the count of
// entries and the duration; the failure is exactly one error record.
func TestSpotlightUserListSecrets(t *testing.T) {
	t.Parallel()
	e := newTraktEnv(t, traktTestID, staticKey(""))
	e.configure(t, `list_key = 'url', list_url = ?, media = 'both'`, traktUserList)
	ctx := context.Background()
	secrets := []string{"jane-doe", "best-ever", traktTestID}
	sum := sha256.Sum256([]byte(traktUserList))
	label := "url both " + hex.EncodeToString(sum[:])[:8]

	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	if got := e.rows(t); len(got) != 2 {
		t.Fatalf("rows = %v, want the movie and the show of the user list", got)
	}
	line := regexp.MustCompile(`level=INFO msg="spotlight list refreshed" list="(url both [0-9a-f]{8})" entries=2 duration=\S+`)
	if m := line.FindAllStringSubmatch(e.logs.String(), -1); len(m) != 1 || m[0][1] != label {
		t.Errorf("success lines = %q, want one naming %q:\n%s", m, label, e.logs.String())
	}
	if l := e.leaks(t, secrets...); len(l) != 0 {
		t.Errorf("after success: %v\n%s", l, e.logs.String())
	}

	e.clock.set(e.clock.now().Add(time.Hour))
	e.fake.status.Store(http.StatusServiceUnavailable)
	if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err == nil {
		t.Error("503 run returned no error")
	}
	if got := e.rows(t); len(got) != 0 {
		t.Errorf("rows after 503 = %v", got)
	}
	if st := e.status(t); !strings.Contains(st.LastError, "503") || !strings.HasPrefix(st.LastError, label+":") {
		t.Errorf("stored error = %q, want the list label %q and the status", st.LastError, label)
	}
	if errs := e.ring.Entries(); len(errs) != 1 || errs[0].Source != "trakt" || !strings.Contains(errs[0].Attrs["err"], "503") {
		t.Errorf("error records after 503 = %+v", errs)
	}
	if n := strings.Count(e.logs.String(), "level=ERROR"); n != 1 {
		t.Errorf("log has %d error lines after 503, want 1:\n%s", n, e.logs.String())
	}
	if l := e.leaks(t, secrets...); len(l) != 0 {
		t.Errorf("after failure: %v\n%s", l, e.logs.String())
	}
}

// Plan 5.6, 5.7: a configuration that no kind offers, written past the administration, fails the
// run like a fetch error: no request, the entries emptied, the error stored, one error record, the
// job retried. A watchlist address is refused the same way, without naming its user.
func TestSpotlightInvalidConfigFails(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, set string
		args      []any
		want      error
	}{
		{name: "boxoffice with shows", set: `list_key = 'boxoffice', media = 'both'`, want: trakt.ErrInvalidList},
		{name: "user list without address", set: `list_key = 'url', list_url = NULL`, want: trakt.ErrInvalidList},
		{name: "watchlist", set: `list_key = 'url', list_url = ?`, args: []any{"https://trakt.tv/users/jane-doe/watchlist"},
			want: trakt.ErrWatchlist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newTraktEnv(t, traktTestID, staticKey(""))
			ctx := context.Background()
			if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
				t.Fatal(err)
			}
			e.fake.forget()
			e.configure(t, tc.set, tc.args...)
			e.clock.set(e.clock.now().Add(time.Hour))
			if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); !errors.Is(err, tc.want) {
				t.Errorf("run = %v, want %v", err, tc.want)
			}
			if got := e.fake.requests(); len(got) != 0 {
				t.Errorf("requests = %v, want none", got)
			}
			if got := e.rows(t); len(got) != 0 {
				t.Errorf("rows = %v, want none", got)
			}
			if st := e.status(t); st.LastError == "" || st.Entries != 0 || !st.LastAttempt.Equal(e.clock.now()) {
				t.Errorf("status = %+v", st)
			}
			if errs := e.ring.Entries(); len(errs) != 1 || errs[0].Source != "trakt" {
				t.Errorf("error records = %+v", errs)
			}
			if l := e.leaks(t, "jane-doe", traktTestID); len(l) != 0 {
				t.Errorf("%v\n%s", l, e.logs.String())
			}
		})
	}
}

// Plan 8.2: after a 500 and after a timeout the entries are empty, the error is stored and logged
// once, and the job retries. An empty list is no failure: the run succeeds with no entries.
func TestSpotlightFetchFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		prepare func(*fakeTrakt)
		failed  string // part of the stored error; "" for a successful run
	}{
		{name: "500", prepare: func(f *fakeTrakt) { f.status.Store(http.StatusInternalServerError) }, failed: "500"},
		{name: "timeout", prepare: func(f *fakeTrakt) { f.delay.Store(int64(5 * time.Second)) }, failed: trakt.ErrUnavailable.Error()},
		{name: "empty list", prepare: func(f *fakeTrakt) { f.empty.Store(true) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newTraktEnv(t, traktTestID, staticKey(""), WithTraktHTTPClient(&http.Client{Timeout: 200 * time.Millisecond}))
			ctx := context.Background()
			if err := e.svc.handleTraktRefresh(ctx, &jobs.Job{}); err != nil {
				t.Fatal(err)
			}
			first := e.clock.now()
			e.clock.set(first.Add(time.Hour))
			tc.prepare(e.fake)
			err := e.svc.handleTraktRefresh(ctx, &jobs.Job{})
			if got := e.rows(t); len(got) != 0 {
				t.Errorf("rows = %v, want none", got)
			}
			st := e.status(t)
			errs := e.ring.Entries()
			if tc.failed == "" {
				if err != nil || st.LastError != "" || !st.LastSuccess.Equal(first.Add(time.Hour)) || len(errs) != 0 {
					t.Errorf("run = %v, status = %+v, error records = %+v", err, st, errs)
				}
				if !strings.Contains(e.logs.String(), `msg="spotlight list refreshed" list="trending both" entries=0`) {
					t.Errorf("no success line with no entries:\n%s", e.logs.String())
				}
				return
			}
			if err == nil {
				t.Error("failed run returned no error, want a retry")
			}
			if !strings.Contains(st.LastError, tc.failed) || !st.LastSuccess.Equal(first) {
				t.Errorf("status = %+v, want the error %q", st, tc.failed)
			}
			if len(errs) != 1 || errs[0].Source != "trakt" {
				t.Errorf("error records = %+v", errs)
			}
		})
	}
}

// Plan 8.2, R-9: the run goes through the configured limiter. With one request per second and no
// burst the two requests of the default list lie at least a second apart.
// Not parallel: this test waits on the clock (an interval, a debounce window or a
// cancellation), and parallel load would stretch the wait past what it asserts.
func TestTraktRefreshUsesLimiter(t *testing.T) {
	e := newTraktEnv(t, traktTestID, staticKey(""), WithTraktLimiter(ratelimit.New("trakt-pace", 1, 1)))
	if err := e.svc.handleTraktRefresh(context.Background(), &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	at := e.fake.requestTimes()
	if len(at) != 2 {
		t.Fatalf("requests = %v, want two", e.fake.requests())
	}
	if gap := at[1].Sub(at[0]); gap < 900*time.Millisecond {
		t.Errorf("requests %v apart, want about a second", gap)
	}
}
