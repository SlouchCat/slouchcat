package radio

import (
	"math/rand/v2"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// logoHostRetry is how long a logo job waits whose host another logo job holds (B7.2).
const logoHostRetry = 15 * time.Second

// logoJobSpread is the time one logo job is expected to hold its host; the waits of the jobs of
// a busy host spread over that much per waiting job (retryDelay).
const logoJobSpread = 2 * time.Second

// logoWaiterMax is how long a deferred job counts as waiting: a job cancelled meanwhile never
// comes back to take its host.
const logoWaiterMax = time.Hour

// logoPNGSlots is how many PNGs pngFromWebP makes at once.
const logoPNGSlots = 2

// logoFetchTimeout bounds one request of the logo chain, body included (measurement B0,
// docs/perf/19-radio.md: a homepage that streams events never ends).
const logoFetchTimeout = 20 * time.Second

// logoState is the logo part of the service (lane A): the logo chain of concept 16 section 6
// (catalog favicon, website, own upload) run through the job queue, scaled to squares. Since
// concept 19 part B (B7.2, OB-11) all logo jobs together fetch at most four addresses a second, and
// at most one job at a time fetches from a host: a job whose host is taken goes back to the queue
// (jobs.RetryLater, no attempt used) instead of waiting, so it never blocks a worker.
type logoState struct {
	limiter   *ratelimit.Limiter // four requests a second for all logo jobs together
	hostRetry time.Duration      // logoHostRetry; tests shorten it
	jobSpread time.Duration      // logoJobSpread; tests shorten it
	// fetchTimeout is logoFetchTimeout; tests shorten it.
	fetchTimeout time.Duration
	// pngSlots bounds the PNGs made from a WebP on request (pngFromWebP) to logoPNGSlots at once.
	pngSlots chan struct{}

	mu    sync.Mutex
	hosts map[string]bool // the hosts a running logo job holds
	// waiting are the jobs sent back because a host was taken, by job id: their hosts and when.
	waiting map[int64]logoWaiter
}

type logoWaiter struct {
	hosts []string
	at    time.Time
}

func newLogoState() *logoState {
	return &logoState{limiter: ratelimit.New("radio-logo", 4, 4), hostRetry: logoHostRetry, jobSpread: logoJobSpread, fetchTimeout: logoFetchTimeout, pngSlots: make(chan struct{}, logoPNGSlots),
		hosts: map[string]bool{}, waiting: map[int64]logoWaiter{}}
}

// lockHosts takes every host of hosts for job, or none when one of them is taken.
func (l *logoState) lockHosts(job int64, hosts []string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slices.ContainsFunc(hosts, func(h string) bool { return l.hosts[h] }) {
		return false
	}
	for _, h := range hosts {
		l.hosts[h] = true
	}
	delete(l.waiting, job)
	return true
}

// unlockHosts gives the hosts of a job back.
func (l *logoState) unlockHosts(hosts []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, h := range hosts {
		delete(l.hosts, h)
	}
}

// logoHosts returns the hosts a logo job fetches from first: those of the catalog's logo address
// and of the homepage, as written (before a redirect), lower case, without repeats.
func logoHosts(addrs ...string) []string {
	var out []string
	for _, a := range addrs {
		u, err := url.Parse(strings.TrimSpace(a))
		if err != nil || u.Hostname() == "" {
			continue
		}
		if h := strings.ToLower(u.Hostname()); !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out
}

// retryDelay records job as waiting for hosts and returns its wait: hostRetry plus a random part
// of up to hostRetry and logoJobSpread for every job that waits for the same host. Without the
// spread all jobs of a busy host woke at the same time, one took the host and the others waited
// again, so the host ran one job each hostRetry (85 stations of 1.FM in Switzerland); with a
// spread that ignores how many wait, the 509 stations of laut.fm in Germany came back about seven
// times a second, each a write of the queue and a line in the log (measurement B0,
// docs/perf/19-radio.md).
func (l *logoState) retryDelay(job int64, hosts []string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, w := range l.waiting {
		if now.Sub(w.at) > logoWaiterMax {
			delete(l.waiting, id)
		}
	}
	l.waiting[job] = logoWaiter{hosts: hosts, at: now}
	most := 0
	for _, h := range hosts {
		n := 0
		for _, w := range l.waiting {
			if slices.Contains(w.hosts, h) {
				n++
			}
		}
		most = max(most, n)
	}
	window := l.hostRetry + time.Duration(most)*l.jobSpread
	return l.hostRetry + rand.N(window) //nolint:gosec // G404: a spread of waits, nothing secret
}
