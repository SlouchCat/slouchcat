package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/slouchcat/slouchcat/server/internal/api"
	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/discover"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Timing of the scan benchmark (design section 6).
const (
	homeInterval  = 250 * time.Millisecond // start page request while the scan runs
	stateInterval = 100 * time.Millisecond // poll of the scan state and the job queue
	// searchRequests is the number of searches after the first run (P-11, E-4: p95 < 200 ms from
	// the second character), each with the two-letter prefix of a real title.
	searchRequests = 200
	// searchSamplePages is the number of title pages (100 each) the prefixes are taken from,
	// spread over the whole library.
	searchSamplePages = 10
)

// scanbench measures the first scan, a rescan without changes and an interrupted scan of a
// library tree (plan M3 step 5, docs/perf/M3.md) on a temporary database with the real library
// service, job workers, watchers and the HTTP handler of the start page; after the first run it
// sends searchRequests searches with two-letter prefixes of real titles (M7, P-11):
//
//	slouchcat scanbench --root DIR [--kind movies|series|both] [--workers N] [--interrupt 10s]
//	                    [--timeout 30m] [--ffprobe ffprobe] [--log-level error] [--json]
//
// The libraries are created on DIR/Movies and DIR/TV. Metadata jobs are registered as no-ops (no
// TMDB key is needed); probe.file fails fast on the empty files of the placeholder tree and
// records probe_error.
func scanbench(args []string) int {
	fs := flag.NewFlagSet("scanbench", flag.ContinueOnError)
	root := fs.String("root", "", "tree with Movies/ and TV/ (task testdata:tree)")
	kind := fs.String("kind", "both", "movies, series or both")
	workers := fs.Int("workers", 4, "job workers")
	interrupt := fs.Duration("interrupt", 10*time.Second, "cancel the interrupted run after this time")
	timeout := fs.Duration("timeout", 30*time.Minute, "limit of each phase")
	ffprobe := fs.String("ffprobe", "ffprobe", "ffprobe binary")
	logLevel := fs.String("log-level", "error", "log level of the services (debug, info, warn, error)")
	asJSON := fs.Bool("json", false, "print JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *root == "" || *workers < 1 || (*kind != "movies" && *kind != "series" && *kind != "both") {
		fmt.Fprintln(os.Stderr, "usage: slouchcat scanbench --root DIR [--kind movies|series|both] [--workers N] [--interrupt 10s] [--timeout 30m] [--json]")
		return 2
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		fmt.Fprintln(os.Stderr, "scanbench:", err)
		return 2
	}
	log, err := logging.New(os.Stderr, logging.FormatText, level)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scanbench:", err)
		return 2
	}
	o := scanbenchOptions{root: *root, workers: *workers, interrupt: *interrupt, timeout: *timeout,
		runner: transcode.Runner{FFprobePath: *ffprobe}, log: log}
	if *kind != "series" {
		o.libs = append(o.libs, benchLibrary{name: "Filme", kind: "movies", dir: filepath.Join(*root, "Movies")})
	}
	if *kind != "movies" {
		o.libs = append(o.libs, benchLibrary{name: "Serien", kind: "series", dir: filepath.Join(*root, "TV")})
	}
	rep, err := runScanbench(context.Background(), o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scanbench:", err)
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(os.Stderr, "scanbench:", err)
			return 1
		}
		return 0
	}
	printScanbench(os.Stdout, rep)
	return 0
}

type benchLibrary struct {
	name, kind, dir string
}

type scanbenchOptions struct {
	root      string
	libs      []benchLibrary
	workers   int
	interrupt time.Duration
	timeout   time.Duration
	runner    transcode.Runner
	log       *slog.Logger
}

// scanbenchReport is the result; durations are in milliseconds in JSON.
type scanbenchReport struct {
	Root      string          `json:"root"`
	Workers   int             `json:"workers"`
	Libraries []libraryReport `json:"libraries"`
	// Start page latency while the first scan and its follow-up jobs run.
	Home homeStats `json:"home"`
	// POST /search after the first run (M7).
	Search searchStats `json:"search"`
	// Whole first run: write transactions of the scans and the job queue until it drained.
	Batches        int        `json:"batches"`
	LongestBatchMs float64    `json:"longestBatchMs"`
	DrainMs        float64    `json:"drainMs"`
	Jobs           jobsReport `json:"jobs"`
	// Rescan of the unchanged tree.
	RescanMs      float64 `json:"rescanMs"`
	RescanOps     int     `json:"rescanOps"`
	RescanBatches int     `json:"rescanBatches"`
	// Interrupted run on a fresh database: cancelled after Interrupt, then queued again.
	Interrupt interruptReport `json:"interrupt"`
}

type libraryReport struct {
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Dir         string         `json:"dir"`
	ScanMs      float64        `json:"scanMs"`
	State       string         `json:"state"`
	Files       int            `json:"files"`
	MediaFiles  int            `json:"mediaFiles"`
	Items       int            `json:"items"`
	ItemsByKind map[string]int `json:"itemsByKind"`
	ProbeErrors int            `json:"probeErrors"`
}

type homeStats struct {
	Requests   int     `json:"requests"`
	Errors     int     `json:"errors"`
	FirstError string  `json:"firstError,omitempty"` // status and body of the first failed request
	P50Ms      float64 `json:"p50Ms"`
	P95Ms      float64 `json:"p95Ms"`
	MaxMs      float64 `json:"maxMs"`
}

// searchStats is the latency of the searches after the first run; Prefixes counts the distinct
// two-letter prefixes taken from the titles.
type searchStats struct {
	Requests   int     `json:"requests"`
	Prefixes   int     `json:"prefixes"`
	Errors     int     `json:"errors"`
	FirstError string  `json:"firstError,omitempty"`
	P50Ms      float64 `json:"search_p50_ms"`
	P95Ms      float64 `json:"search_p95_ms"`
	MaxMs      float64 `json:"search_max_ms"`
}

type jobsReport struct {
	Total         int `json:"total"`
	Done          int `json:"done"`
	Repeated      int `json:"repeated"`      // attempts > 1: must be 0
	LeaseExpired  int `json:"leaseExpired"`  // must be 0
	DuplicateKeys int `json:"duplicateKeys"` // probe.file queued twice for a file: must be 0
}

type interruptReport struct {
	AfterMs        float64          `json:"afterMs"`
	FinishedBefore bool             `json:"finishedBefore"` // the scan was done before the cancel
	Libraries      []interruptedLib `json:"libraries"`
	ResumeMs       float64          `json:"resumeMs"`
	Converged      bool             `json:"converged"` // item counts equal the first run
}

type interruptedLib struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	LastError  string `json:"lastError"`
	FilesSeen  int    `json:"filesSeen"`
	FilesTotal int    `json:"filesTotal"`
	Items      int    `json:"items"`
	FinalItems int    `json:"finalItems"`
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// benchEnv is one temporary server: database, library service, workers, watchers, HTTP handler
// and a session of the owner profile.
type benchEnv struct {
	dir     string
	db      *store.DB
	lib     *library.Service
	handler http.Handler
	cookie  *http.Cookie
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newBenchEnv(ctx context.Context, o scanbenchOptions) (*benchEnv, error) {
	dir, err := os.MkdirTemp("", "slouchcat-scanbench-")
	if err != nil {
		return nil, err
	}
	e := &benchEnv{dir: dir}
	configDir, cacheDir := filepath.Join(dir, "config"), filepath.Join(dir, "cache")
	for _, d := range []string{configDir, cacheDir} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			e.close()
			return nil, err
		}
	}
	if e.db, err = store.Open(ctx, store.Options{ConfigDir: configDir, Version: version, Logger: o.log}); err != nil {
		e.close()
		return nil, err
	}
	queue := jobs.NewQueue(e.db.Write, nil)
	e.lib = library.New(e.db, queue, o.runner, o.log)
	authSvc := auth.New(e.db, o.log)
	password := make([]byte, 16)
	if _, err := rand.Read(password); err != nil {
		e.close()
		return nil, err
	}
	account, profile, err := authSvc.CreateOwner(ctx, "scanbench@example.org", hex.EncodeToString(password), "de")
	if err != nil {
		e.close()
		return nil, fmt.Errorf("create owner: %w", err)
	}
	_, token, err := authSvc.StartSession(ctx, account, profile, "scanbench")
	if err != nil {
		e.close()
		return nil, fmt.Errorf("start session: %w", err)
	}
	e.cookie = &http.Cookie{Name: api.SessionCookie, Value: token, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	e.handler = api.NewHandler(api.Options{
		Version: version, Started: time.Now(), Logger: o.log, Web: http.NotFoundHandler(),
		Auth: authSvc, Profiles: authSvc, Access: authSvc, Catalog: discover.New(e.db, cacheDir, o.log),
		Playback: playback.New(e.db, o.log), // progress rows of the start page; no transcoder
	})

	runCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	noop := func(context.Context, *jobs.Job) error { return nil }
	for i := range o.workers {
		w := jobs.NewWorker(queue, fmt.Sprintf("bench-%d", i+1), o.log)
		e.lib.RegisterJobs(w)
		w.Handle(library.JobMetadataMatch, noop)
		e.wg.Go(func() { w.Run(runCtx) })
	}
	e.wg.Go(func() {
		if err := e.lib.Start(runCtx); err != nil && runCtx.Err() == nil {
			o.log.Error("library watchers stopped", "err", err)
		}
	})
	return e, nil
}

func (e *benchEnv) close() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
	if e.db != nil {
		_ = e.db.Close()
	}
	_ = os.RemoveAll(e.dir)
}

// home requests the start page once through the handler and returns its duration and, for a
// failed request, its status and body.
func (e *benchEnv) home(ctx context.Context) (time.Duration, string) {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, api.BasePath+"/home", nil)
	req.AddCookie(e.cookie)
	rec := httptest.NewRecorder()
	start := time.Now()
	e.handler.ServeHTTP(rec, req)
	d := time.Since(start)
	if rec.Code != http.StatusOK {
		return d, fmt.Sprintf("%d %.200s", rec.Code, rec.Body.String())
	}
	return d, ""
}

// post sends a JSON POST through the handler and returns the response and its duration.
func (e *benchEnv) post(ctx context.Context, path string, body []byte) (*httptest.ResponseRecorder, time.Duration) {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, api.BasePath+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(api.CSRFHeader, api.CSRFHeaderValue)
	req.AddCookie(e.cookie)
	rec := httptest.NewRecorder()
	start := time.Now()
	e.handler.ServeHTTP(rec, req)
	return rec, time.Since(start)
}

// searchPhase takes two-letter prefixes from the titles of the scanned library (browsing it through
// the search operation by media type, page by page over the whole range) and sends searchRequests
// searches with them in turn.
func (e *benchEnv) searchPhase(ctx context.Context) (searchStats, error) {
	var st searchStats
	type page struct {
		Total int `json:"total"`
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
	}
	browse := func(offset int) (page, error) {
		body, err := json.Marshal(map[string]any{"mediaTypes": []string{"movie", "series"}, "limit": 100, "offset": offset})
		if err != nil {
			return page{}, err
		}
		rec, _ := e.post(ctx, "/search", body)
		var p page
		if rec.Code != http.StatusOK {
			return p, fmt.Errorf("browse titles: %d %.200s", rec.Code, rec.Body.String())
		}
		return p, json.Unmarshal(rec.Body.Bytes(), &p)
	}
	first, err := browse(0)
	if err != nil {
		return st, err
	}
	var prefixes []string
	seen := map[string]bool{}
	for i := range searchSamplePages {
		p := first
		if i > 0 {
			if p, err = browse(first.Total * i / searchSamplePages); err != nil {
				return st, err
			}
		}
		for _, it := range p.Items {
			if prefix := titlePrefix(it.Title); prefix != "" && !seen[prefix] {
				seen[prefix] = true
				prefixes = append(prefixes, prefix)
			}
		}
	}
	st.Prefixes = len(prefixes)
	if len(prefixes) == 0 {
		return st, errors.New("search phase: no title to take a prefix from")
	}
	lat := make([]time.Duration, 0, searchRequests)
	for i := range searchRequests {
		body, err := json.Marshal(map[string]string{"query": prefixes[i%len(prefixes)]})
		if err != nil {
			return st, err
		}
		rec, d := e.post(ctx, "/search", body)
		lat = append(lat, d)
		if rec.Code != http.StatusOK {
			st.Errors++
			if st.FirstError == "" {
				st.FirstError = fmt.Sprintf("%d %.200s", rec.Code, rec.Body.String())
			}
		}
	}
	h := homePercentiles(lat, homeStats{})
	st.Requests, st.P50Ms, st.P95Ms, st.MaxMs = h.Requests, h.P50Ms, h.P95Ms, h.MaxMs
	return st, nil
}

// titlePrefix returns the first two letters or digits of a title's first word, or "" when the
// title has no such word.
func titlePrefix(title string) string {
	for _, w := range strings.FieldsFunc(title, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if r := []rune(w); len(r) >= 2 {
			return string(r[:2])
		}
	}
	return ""
}

// waitScans waits until no library of ids is queued or running and returns the finish time of
// each library (from the poll).
func (e *benchEnv) waitScans(ctx context.Context, ids []int64, timeout time.Duration) (map[int64]time.Time, error) {
	deadline := time.Now().Add(timeout)
	done := map[int64]time.Time{}
	for len(done) < len(ids) {
		for _, id := range ids {
			if _, ok := done[id]; ok {
				continue
			}
			l, err := e.lib.Get(ctx, id)
			if err != nil {
				return nil, err
			}
			if l.Scan.State != "queued" && l.Scan.State != "running" && l.Scan.State != "never" {
				done[id] = time.Now()
			}
		}
		if len(done) == len(ids) {
			break
		}
		if time.Now().After(deadline) {
			return nil, errors.New("scan did not finish within the timeout")
		}
		time.Sleep(stateInterval)
	}
	return done, nil
}

// waitDrain waits until every job created since since has finished.
func (e *benchEnv) waitDrain(ctx context.Context, since time.Time, timeout time.Duration) (library.JobStats, error) {
	deadline := time.Now().Add(timeout)
	for {
		st, err := e.lib.JobStats(ctx, since)
		if err != nil {
			return st, err
		}
		if st.Active == 0 {
			return st, nil
		}
		if time.Now().After(deadline) {
			return st, fmt.Errorf("%d jobs still active after the timeout", st.Active)
		}
		time.Sleep(stateInterval)
	}
}

func (e *benchEnv) create(ctx context.Context, libs []benchLibrary) ([]int64, error) {
	ids := make([]int64, 0, len(libs))
	for _, b := range libs {
		l, err := e.lib.Create(ctx, model.LibraryRequest{Name: b.name, Kind: b.kind, Path: b.dir})
		if err != nil {
			return nil, fmt.Errorf("create library %s on %s: %w", b.name, b.dir, err)
		}
		ids = append(ids, l.ID)
	}
	return ids, nil
}

func runScanbench(ctx context.Context, o scanbenchOptions) (scanbenchReport, error) {
	rep := scanbenchReport{Root: o.root, Workers: o.workers}
	e, err := newBenchEnv(ctx, o)
	if err != nil {
		return rep, err
	}
	defer e.close()

	// First run: scans, then the follow-up jobs, with the start page polled throughout.
	start := time.Now()
	since := start.Add(-time.Second)
	ids, err := e.create(ctx, o.libs)
	if err != nil {
		return rep, err
	}
	var lat []time.Duration
	pollCtx, stopPoll := context.WithCancel(ctx)
	var poller sync.WaitGroup
	poller.Go(func() {
		t := time.NewTicker(homeInterval)
		defer t.Stop()
		for {
			d, failure := e.home(ctx)
			lat = append(lat, d)
			if failure != "" {
				rep.Home.Errors++
				if rep.Home.FirstError == "" {
					rep.Home.FirstError = failure
				}
			}
			select {
			case <-pollCtx.Done():
				return
			case <-t.C:
			}
		}
	})
	finished, err := e.waitScans(ctx, ids, o.timeout)
	if err != nil {
		stopPoll()
		poller.Wait()
		return rep, err
	}
	st, err := e.waitDrain(ctx, since, o.timeout)
	stopPoll()
	poller.Wait()
	if err != nil {
		return rep, err
	}
	rep.DrainMs = ms(time.Since(start))
	rep.Home = homePercentiles(lat, rep.Home)
	if rep.Search, err = e.searchPhase(ctx); err != nil {
		return rep, err
	}
	rep.Jobs = jobsReport{Total: st.Total, Done: st.Done, Repeated: st.Repeated, LeaseExpired: st.LeaseExpired, DuplicateKeys: st.DuplicateKeys}
	ws := e.lib.TakeWriteStats()
	rep.Batches, rep.LongestBatchMs = ws.Batches, ms(ws.Longest)
	for i, id := range ids {
		lr, err := e.libraryReport(ctx, o.libs[i], id)
		if err != nil {
			return rep, err
		}
		lr.ScanMs = ms(finished[id].Sub(start))
		rep.Libraries = append(rep.Libraries, lr)
	}

	// Rescan of the unchanged tree: nothing may be written.
	start = time.Now()
	for _, id := range ids {
		if _, err := e.lib.QueueScan(ctx, id); err != nil {
			return rep, err
		}
	}
	if _, err := e.waitScans(ctx, ids, o.timeout); err != nil {
		return rep, err
	}
	rep.RescanMs = ms(time.Since(start))
	ws = e.lib.TakeWriteStats()
	rep.RescanOps, rep.RescanBatches = ws.Ops, ws.Batches

	rep.Interrupt, err = runInterrupted(ctx, o, rep.Libraries)
	return rep, err
}

func (e *benchEnv) libraryReport(ctx context.Context, b benchLibrary, id int64) (libraryReport, error) {
	l, err := e.lib.Get(ctx, id)
	if err != nil {
		return libraryReport{}, err
	}
	c, err := e.lib.Content(ctx, id)
	if err != nil {
		return libraryReport{}, err
	}
	return libraryReport{
		Name: b.name, Kind: b.kind, Dir: b.dir, State: l.Scan.State, Files: l.Scan.FilesSeen, MediaFiles: l.Scan.FileCount,
		Items: l.Scan.ItemCount, ItemsByKind: c.Items, ProbeErrors: c.ProbeErrors,
	}, nil
}

// runInterrupted scans the tree on a fresh database, cancels the scans after o.interrupt (as the
// administrator does, CancelScan), records state and progress, queues the scans again and checks
// that the result equals the first run.
func runInterrupted(ctx context.Context, o scanbenchOptions, full []libraryReport) (interruptReport, error) {
	var rep interruptReport
	e, err := newBenchEnv(ctx, o)
	if err != nil {
		return rep, err
	}
	defer e.close()
	start := time.Now()
	ids, err := e.create(ctx, o.libs)
	if err != nil {
		return rep, err
	}
	time.Sleep(o.interrupt)
	rep.AfterMs = ms(time.Since(start))
	rep.FinishedBefore = true
	for i, id := range ids {
		l, err := e.lib.CancelScan(ctx, id)
		if err != nil {
			return rep, err
		}
		if l.Scan.State == "aborted" {
			rep.FinishedBefore = false
		}
		rep.Libraries = append(rep.Libraries, interruptedLib{
			Name: o.libs[i].name, State: l.Scan.State, LastError: l.Scan.LastError,
			FilesSeen: l.Scan.FilesSeen, FilesTotal: l.Scan.FilesTotal, Items: l.Scan.ItemCount,
		})
	}
	start = time.Now()
	for _, id := range ids {
		if _, err := e.lib.QueueScan(ctx, id); err != nil {
			return rep, err
		}
	}
	if _, err := e.waitScans(ctx, ids, o.timeout); err != nil {
		return rep, err
	}
	rep.ResumeMs = ms(time.Since(start))
	rep.Converged = true
	for i, id := range ids {
		l, err := e.lib.Get(ctx, id)
		if err != nil {
			return rep, err
		}
		rep.Libraries[i].FinalItems = l.Scan.ItemCount
		if l.Scan.State != "done" || l.Scan.ItemCount != full[i].Items {
			rep.Converged = false
		}
	}
	return rep, nil
}

func homePercentiles(lat []time.Duration, h homeStats) homeStats {
	h.Requests = len(lat)
	if len(lat) == 0 {
		return h
	}
	s := slices.Clone(lat)
	slices.Sort(s)
	pick := func(p float64) time.Duration { return s[min(len(s)-1, int(p*float64(len(s))))] }
	h.P50Ms, h.P95Ms, h.MaxMs = ms(pick(0.50)), ms(pick(0.95)), ms(s[len(s)-1])
	return h
}

func printScanbench(w io.Writer, rep scanbenchReport) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "root\t%s\nworkers\t%d\n\n", rep.Root, rep.Workers)
	_, _ = fmt.Fprintln(tw, "library\tkind\tstate\tfiles\tmedia files\titems\tby kind\tprobe errors\tfirst scan")
	for _, l := range rep.Libraries {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%v\t%d\t%.1f s\n", l.Name, l.Kind, l.State, l.Files, l.MediaFiles,
			l.Items, l.ItemsByKind, l.ProbeErrors, l.ScanMs/1000)
	}
	_, _ = fmt.Fprintf(tw, "\nfirst run until the queue drained\t%.1f s\n", rep.DrainMs/1000)
	_, _ = fmt.Fprintf(tw, "write transactions\t%d (longest %.0f ms)\n", rep.Batches, rep.LongestBatchMs)
	_, _ = fmt.Fprintf(tw, "jobs\t%d created, %d done, %d with attempts > 1, %d lease expired, %d probe keys twice\n",
		rep.Jobs.Total, rep.Jobs.Done, rep.Jobs.Repeated, rep.Jobs.LeaseExpired, rep.Jobs.DuplicateKeys)
	_, _ = fmt.Fprintf(tw, "GET /home during the run\t%d requests, %d errors, p50 %.1f ms, p95 %.1f ms, max %.1f ms\n",
		rep.Home.Requests, rep.Home.Errors, rep.Home.P50Ms, rep.Home.P95Ms, rep.Home.MaxMs)
	if rep.Home.FirstError != "" {
		_, _ = fmt.Fprintf(tw, "first failed GET /home\t%s\n", rep.Home.FirstError)
	}
	sr := rep.Search
	_, _ = fmt.Fprintf(tw, "POST /search after the run\t%d requests over %d prefixes, %d errors\n", sr.Requests, sr.Prefixes, sr.Errors)
	_, _ = fmt.Fprintf(tw, "search_p50_ms\t%.1f\nsearch_p95_ms\t%.1f\nsearch_max_ms\t%.1f\n", sr.P50Ms, sr.P95Ms, sr.MaxMs)
	if sr.FirstError != "" {
		_, _ = fmt.Fprintf(tw, "first failed POST /search\t%s\n", sr.FirstError)
	}
	_, _ = fmt.Fprintf(tw, "rescan without changes\t%.1f s, %d operations in %d transactions\n", rep.RescanMs/1000, rep.RescanOps, rep.RescanBatches)
	ir := rep.Interrupt
	_, _ = fmt.Fprintf(tw, "interrupted run\tcancelled after %.1f s (finished before: %v), resumed in %.1f s, converged: %v\n",
		ir.AfterMs/1000, ir.FinishedBefore, ir.ResumeMs/1000, ir.Converged)
	for _, l := range ir.Libraries {
		_, _ = fmt.Fprintf(tw, "  %s\t%s/%s at %d of %d files, %d items; after the resume %d items\n",
			l.Name, l.State, l.LastError, l.FilesSeen, l.FilesTotal, l.Items, l.FinalItems)
	}
	_ = tw.Flush()
}
