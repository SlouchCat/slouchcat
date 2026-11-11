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
	// fetchTimeout is logoFetchTimeout; tests shorten it.
	fetchTimeout time.Duration

	mu    sync.Mutex
	hosts map[string]bool // the hosts a running logo job holds
}

func newLogoState() *logoState {
	return &logoState{limiter: ratelimit.New("radio-logo", 4, 4), hostRetry: logoHostRetry, fetchTimeout: logoFetchTimeout,
		hosts: map[string]bool{}}
}

// lockHosts takes every host of hosts for one job, or none when one of them is taken.
func (l *logoState) lockHosts(hosts []string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slices.ContainsFunc(hosts, func(h string) bool { return l.hosts[h] }) {
		return false
	}
	for _, h := range hosts {
		l.hosts[h] = true
	}
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

// retryDelay is the wait of a job whose host is taken: hostRetry plus up to hostRetry at random.
// Without the spread all jobs of a busy host (85 stations of 1.FM in Switzerland) woke at the
// same time, one took the host and the others waited again, so the host ran one job each
// hostRetry (measurement B0, docs/perf/19-radio.md).
func (l *logoState) retryDelay() time.Duration {
	return l.hostRetry + rand.N(l.hostRetry)
}
