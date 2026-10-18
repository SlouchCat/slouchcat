package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock advances only when a limiter sleeps, so the tests check exact delays without waiting.
type fakeClock struct {
	mu     sync.Mutex
	t      time.Time
	slept  []time.Duration
	total  time.Duration
	nowFn  func() time.Time
	sleepF func(context.Context, time.Duration) error
}

func newFakeClock() *fakeClock {
	c := &fakeClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	c.nowFn = func() time.Time {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.t
	}
	c.sleepF = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if d <= 0 {
			return nil
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		c.t = c.t.Add(d)
		c.slept = append(c.slept, d)
		c.total += d
		return nil
	}
	return c
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func (c *fakeClock) elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

func TestWaitEnforcesRate(t *testing.T) {
	c := newFakeClock()
	l := newLimiter("test", 20, 20, c.nowFn, c.sleepF)
	ctx := context.Background()
	for range 20 {
		if err := l.Wait(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := c.elapsed(); got != 0 {
		t.Fatalf("burst of 20 waited %v, want 0", got)
	}
	for range 20 {
		if err := l.Wait(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// 20 more requests at 20/s take one second.
	if got := c.elapsed(); got < 999*time.Millisecond || got > 1001*time.Millisecond {
		t.Fatalf("40 requests took %v, want 1s", got)
	}
}

func TestWaitRefillsOverTime(t *testing.T) {
	c := newFakeClock()
	l := newLimiter("test", 10, 2, c.nowFn, c.sleepF)
	ctx := context.Background()
	for range 2 {
		_ = l.Wait(ctx)
	}
	c.advance(time.Second) // refills to the burst of 2, not 10
	for range 2 {
		_ = l.Wait(ctx)
	}
	if got := c.elapsed(); got != 0 {
		t.Fatalf("waited %v after refill, want 0", got)
	}
	_ = l.Wait(ctx)
	if got := c.elapsed(); got != 100*time.Millisecond {
		t.Fatalf("third request waited %v, want 100ms", got)
	}
}

func TestBackoffDelaysNextWait(t *testing.T) {
	c := newFakeClock()
	l := newLimiter("test", 20, 20, c.nowFn, c.sleepF)
	l.Backoff(c.nowFn().Add(5 * time.Second))
	if err := l.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := c.elapsed(); got != 5*time.Second {
		t.Fatalf("waited %v, want 5s", got)
	}
	// After the pause the bucket grants at its rate, without a burst.
	_ = l.Wait(context.Background())
	if got := c.elapsed(); got != 5*time.Second+50*time.Millisecond {
		t.Fatalf("waited %v, want 5.05s", got)
	}
}

func TestWaitHonoursCancelledContext(t *testing.T) {
	l := New("test", 1, 1)
	_ = l.Wait(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := l.Wait(ctx); err == nil {
		t.Fatal("Wait returned nil although the context ended before a token was free")
	}
}

func TestDoRetriesAfterRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newFakeClock()
	l := newLimiter("test", 20, 20, c.nowFn, c.sleepF)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := l.Do(context.Background(), srv.Client(), req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls.Load() != 2 {
		t.Fatalf("status %d after %d calls, want 200 after 2", resp.StatusCode, calls.Load())
	}
	if got := c.elapsed(); got != time.Second {
		t.Fatalf("retry waited %v, want the Retry-After of 1s", got)
	}
}

func TestDoGivesUpAfterMaxRetriesWithExponentialBackoff(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newFakeClock()
	l := newLimiter("test", 20, 20, c.nowFn, c.sleepF)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := l.Do(context.Background(), srv.Client(), req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || calls.Load() != MaxRetries+1 {
		t.Fatalf("status %d after %d calls, want 429 after %d", resp.StatusCode, calls.Load(), MaxRetries+1)
	}
	// 2 s, 4 s, 8 s before the three retries.
	if got := c.elapsed(); got != 14*time.Second {
		t.Fatalf("backoff total %v, want 14s", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"1", time.Second, true},
		{" 30 ", 30 * time.Second, true},
		{now.Add(10 * time.Second).Format(http.TimeFormat), 10 * time.Second, true},
		{"", 0, false},
		{"-3", 0, false},
		{"soon", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseRetryAfter(tc.in, now)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseRetryAfter(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPenaltyCapsRetryAfter(t *testing.T) {
	c := newFakeClock()
	l := newLimiter("test", 1, 1, c.nowFn, c.sleepF)
	if got := l.penalty("3600"); got != MaxBackoff {
		t.Fatalf("penalty = %v, want %v", got, MaxBackoff)
	}
	for range 10 {
		l.penalty("")
	}
	if got := l.penalty(""); got != MaxBackoff {
		t.Fatalf("penalty after many strikes = %v, want %v", got, MaxBackoff)
	}
}
