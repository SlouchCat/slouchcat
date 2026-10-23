// Package ratelimit holds the shared request limiters per external provider (plan section 3.1:
// TMDB at most 20 requests per second, TVDB 5, later Trakt), the backoff after HTTP 429 and the
// daily unit quota of the YouTube Data API (Quota, quota.go).
//
// A Limiter is a token bucket: it refills perSecond tokens per second up to burst, and every Wait
// takes one token, blocking until it is available. Backoff pauses the bucket until a point in
// time; Do wraps an HTTP request with Wait and retries it after a 429 answer, honouring the
// Retry-After header.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Backoff after HTTP 429 without a usable Retry-After header: DefaultBackoff, doubled for every
// further 429 in a row, at most MaxBackoff. A Retry-After value is capped at MaxBackoff too.
const (
	DefaultBackoff = 2 * time.Second
	MaxBackoff     = 60 * time.Second
	// MaxRetries is the number of retries Do makes after a 429 answer.
	MaxRetries = 3
)

// Limiter limits the request rate to one provider. It is safe for concurrent use.
type Limiter struct {
	name  string
	rate  float64 // tokens per second
	burst float64

	mu           sync.Mutex
	tokens       float64
	last         time.Time // time of the last refill; may lie in the future after Backoff
	backoffUntil time.Time
	strikes      int // 429 answers in a row

	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

// Option configures New.
type Option func(*Limiter)

// WithClock replaces time.Now (tests). A test that asserts what happens inside one window has to
// own the clock: on a loaded machine the requests themselves take longer than the window, and the
// bucket refills in the middle of the test.
func WithClock(now func() time.Time) Option {
	return func(l *Limiter) { l.now = now }
}

// New returns a limiter called name (for errors) that allows perSecond requests per second with
// bursts of up to burst requests. It starts full.
func New(name string, perSecond float64, burst int, opts ...Option) *Limiter {
	l := newLimiter(name, perSecond, burst, time.Now, sleepCtx)
	for _, opt := range opts {
		opt(l)
	}
	l.last = l.now()
	return l
}

func newLimiter(name string, perSecond float64, burst int, now func() time.Time,
	sleep func(context.Context, time.Duration) error,
) *Limiter {
	if perSecond <= 0 {
		perSecond = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &Limiter{
		name: name, rate: perSecond, burst: float64(burst), tokens: float64(burst),
		last: now(), now: now, sleep: sleep,
	}
}

// Name returns the provider name of the limiter.
func (l *Limiter) Name() string { return l.name }

// Wait blocks until the limiter grants one request or ctx ends.
func (l *Limiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := l.sleep(ctx, l.reserve()); err != nil {
		return err
	}
	// A Backoff that arrived while this caller slept delays it as well.
	for {
		l.mu.Lock()
		d := l.backoffUntil.Sub(l.now())
		l.mu.Unlock()
		if d <= 0 {
			return nil
		}
		if err := l.sleep(ctx, d); err != nil {
			return err
		}
	}
}

// reserve takes one token and returns how long the caller has to wait for it.
func (l *Limiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.After(l.last) {
		l.tokens = min(l.burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
		l.last = now
	}
	l.tokens--
	wait := l.last.Sub(now)
	if l.tokens < 0 {
		wait += time.Duration(-l.tokens / l.rate * float64(time.Second))
	}
	return wait
}

// Allow takes one token if one is free right now and reports whether it did. Unlike Wait it never
// blocks: an incoming request that finds the bucket empty is refused with HTTP 429 instead of being
// held (plan 3.8 point 8, the webhook receiver of I-8). A running Backoff refuses every call.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Before(l.backoffUntil) {
		return false
	}
	if now.After(l.last) {
		l.tokens = min(l.burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// Backoff makes every Wait block until until. After the pause the limiter grants requests at its
// rate again, without a burst.
func (l *Limiter) Backoff(until time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if until.After(l.backoffUntil) {
		l.backoffUntil = until
	}
	if until.After(l.last) {
		l.last = until
		// One token at until; callers that already reserved keep their (negative) share.
		l.tokens = min(l.tokens, 0) + 1
	}
}

// Do waits for a token, sends req with client and retries up to MaxRetries times after HTTP 429,
// each time after the delay from Retry-After (or the exponential default). It returns the last
// response, which can still be a 429 when every retry was refused. A request with a body must
// have GetBody set to be retried.
func (l *Limiter) Do(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := l.Wait(ctx); err != nil {
			return nil, err
		}
		r := req.Clone(ctx)
		if attempt > 0 && req.Body != nil && req.Body != http.NoBody {
			if req.GetBody == nil {
				return nil, fmt.Errorf("%s: cannot retry a request without GetBody", l.name)
			}
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("%s: reset request body: %w", l.name, err)
			}
			r.Body = body
		}
		resp, err := client.Do(r)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			l.mu.Lock()
			l.strikes = 0
			l.mu.Unlock()
			return resp, nil
		}
		delay := l.penalty(resp.Header.Get("Retry-After"))
		l.Backoff(l.now().Add(delay))
		if attempt >= MaxRetries {
			return resp, nil
		}
		// Drain a little of the body so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		if err := resp.Body.Close(); err != nil && !errors.Is(err, http.ErrBodyReadAfterClose) {
			return nil, fmt.Errorf("%s: close 429 response: %w", l.name, err)
		}
	}
}

// penalty counts one more 429 in a row and returns the delay before the next request.
func (l *Limiter) penalty(retryAfter string) time.Duration {
	l.mu.Lock()
	l.strikes++
	strikes := l.strikes
	now := l.now()
	l.mu.Unlock()
	if d, ok := parseRetryAfter(retryAfter, now); ok {
		return min(d, MaxBackoff)
	}
	return min(DefaultBackoff<<min(strikes-1, 5), MaxBackoff)
}

// parseRetryAfter reads the header as delay seconds or as an HTTP date.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(t.Sub(now), 0), true
	}
	return 0, false
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
