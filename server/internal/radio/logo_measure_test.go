//go:build radiomeasure

package radio

// The measurement of stage B0 of concept 19 part B (docs/perf/19-radio.md section 3): the real logo
// chain (catalog favicon, homepage icons, 96 px minimum, scaling to 96/256/512 in WebP and PNG) run
// over the catalog stations of one country of a copy of a real database, with the shared limiter at
// MEASURE_RATE requests a second, the per-host lock of handleLogo and MEASURE_WORKERS workers that
// put a job whose host is taken back for logoHostRetry, as the job queue does. Never part of a
// normal test run (build tag radiomeasure); it fetches from the internet.
//
//	go test -c -tags radiomeasure -o radiomeasure.test ./internal/radio
//	MEASURE_CONFIG=<dir with a copy of slouchcat.db> MEASURE_COUNTRY=CH MEASURE_RATE=4 \
//	  MEASURE_CACHE=<empty dir> ./radiomeasure.test -test.run TestMeasureLogoChain -test.v -test.timeout 3h

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// countingTransport counts the requests, their status codes and the body bytes read.
type countingTransport struct {
	next     http.RoundTripper
	requests atomic.Int64
	bytes    atomic.Int64
	mu       sync.Mutex
	status   map[int]int
	failures int
}

type countingBody struct {
	io.ReadCloser
	n *atomic.Int64
}

func (b countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n.Add(int64(n))
	return n, err
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.requests.Add(1)
	resp, err := c.next.RoundTrip(r)
	c.mu.Lock()
	if err != nil {
		c.failures++
	} else {
		c.status[resp.StatusCode]++
	}
	c.mu.Unlock()
	if err == nil {
		resp.Body = countingBody{ReadCloser: resp.Body, n: &c.bytes}
	}
	return resp, err
}

func measureEnv(t *testing.T, name string) string {
	v := os.Getenv(name)
	if v == "" {
		t.Fatalf("%s is not set", name)
	}
	return v
}

func TestMeasureLogoChain(t *testing.T) {
	ctx := context.Background()
	configDir, country, cacheDir := measureEnv(t, "MEASURE_CONFIG"), measureEnv(t, "MEASURE_COUNTRY"), measureEnv(t, "MEASURE_CACHE")
	rate, err := strconv.ParseFloat(measureEnv(t, "MEASURE_RATE"), 64)
	if err != nil || rate <= 0 {
		t.Fatalf("MEASURE_RATE: %q", os.Getenv("MEASURE_RATE"))
	}
	workers := 4
	if v := os.Getenv("MEASURE_WORKERS"); v != "" {
		if workers, err = strconv.Atoi(v); err != nil || workers < 1 {
			t.Fatalf("MEASURE_WORKERS: %q", v)
		}
	}
	ffmpeg := os.Getenv("MEASURE_FFMPEG")
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.Open(ctx, store.Options{ConfigDir: configDir, Version: "radiomeasure", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db, log, WithRunner(transcode.Runner{FFmpegPath: ffmpeg}), WithCacheDir(cacheDir),
		WithUserAgent("Slouchcat/radiomeasure (+https://github.com/SlouchCat/slouchcat)"))
	s.logo.limiter = ratelimit.New("radio-logo-measure", rate, max(1, int(rate)))
	ct := &countingTransport{next: s.netClient().http.Transport, status: map[int]int{}}
	if ct.next == nil {
		ct.next = http.DefaultTransport
	}
	s.netClient().http.Transport = ct
	s.onStationChanged = func(context.Context, int64) {}

	rows, err := db.Read.QueryContext(ctx, `SELECT id FROM radio_stations WHERE source = 'catalog' AND country = ?
		ORDER BY votes + clicks DESC, id`, country)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatalf("no catalog station in %s", country)
	}

	type pending struct {
		id    int64
		ready time.Time
	}
	var (
		mu       sync.Mutex
		queue    = make([]pending, 0, len(ids))
		left     = len(ids)
		retries  int
		durs     []time.Duration
		maxHosts int
	)
	for _, id := range ids {
		queue = append(queue, pending{id: id})
	}
	next := func() (pending, bool, time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		if left == 0 {
			return pending{}, false, 0
		}
		now := time.Now()
		for i, p := range queue {
			if !p.ready.After(now) {
				queue = slices.Delete(queue, i, i+1)
				return p, true, 0
			}
		}
		return pending{}, true, 200 * time.Millisecond
	}
	started := time.Now()
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Minute):
			}
			mu.Lock()
			t.Logf("progress %s: %d of %d done, %d host retries, %d requests", time.Since(started).Round(time.Second),
				len(ids)-left, len(ids), retries, ct.requests.Load())
			mu.Unlock()
		}
	}()
	defer close(stop)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for {
				p, more, wait := next()
				if !more {
					return
				}
				if wait > 0 {
					time.Sleep(wait)
					continue
				}
				payload, _ := json.Marshal(logoPayload{StationID: p.id})
				t0 := time.Now()
				err := s.handleLogo(ctx, &jobs.Job{Type: JobLogoFetch, Payload: payload})
				d := time.Since(t0)
				s.logo.mu.Lock()
				maxHosts = max(maxHosts, len(s.logo.hosts))
				s.logo.mu.Unlock()
				mu.Lock()
				var rl *jobs.RetryLaterError
				switch {
				case err != nil && asRetryLater(err, &rl):
					retries++
					queue = append(queue, pending{id: p.id, ready: time.Now().Add(rl.After)})
				default:
					if err != nil {
						t.Logf("station %d: %v", p.id, err)
					}
					left--
					durs = append(durs, d)
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	total := time.Since(started)

	states := map[string]int{}
	srows, err := db.Read.QueryContext(ctx, `SELECT logo_state, count(*) FROM radio_stations
		WHERE source = 'catalog' AND country = ? GROUP BY logo_state`, country)
	if err != nil {
		t.Fatal(err)
	}
	for srows.Next() {
		var st string
		var n int
		if err := srows.Scan(&st, &n); err != nil {
			t.Fatal(err)
		}
		states[st] = n
	}
	_ = srows.Close()
	var files, size int64
	_ = filepath.WalkDir(filepath.Join(cacheDir, "images", "radio"), func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				files++
				size += info.Size()
			}
		}
		return nil
	})
	slices.Sort(durs)
	pct := func(p float64) time.Duration { return durs[min(len(durs)-1, int(float64(len(durs))*p))] }
	logos := states[logoStateCatalog] + states[logoStateWebsite]
	t.Logf("country=%s stations=%d rate=%.0f/s workers=%d", country, len(ids), rate, workers)
	t.Logf("total=%s requests=%d bytes=%d (%.1f MB) failures=%d status=%v", total.Round(time.Second),
		ct.requests.Load(), ct.bytes.Load(), float64(ct.bytes.Load())/1e6, ct.failures, ct.status)
	t.Logf("job p50=%s p90=%s max=%s host retries=%d most hosts held at once=%d", pct(0.5).Round(time.Millisecond),
		pct(0.9).Round(time.Millisecond), durs[len(durs)-1].Round(time.Millisecond), retries, maxHosts)
	t.Logf("states=%v logos=%d (%.0f %%) files=%d size=%d (%.1f MB, %.0f KB per logo)", states, logos,
		100*float64(logos)/float64(len(ids)), files, size, float64(size)/1e6, float64(size)/1024/float64(max(1, logos)))
}

func asRetryLater(err error, target **jobs.RetryLaterError) bool {
	for e := err; e != nil; {
		if rl, ok := e.(*jobs.RetryLaterError); ok {
			*target = rl
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
