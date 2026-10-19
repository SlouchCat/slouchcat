package auth

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Login and setup limits (plan 3.8 point 2). One argon2id run with the default parameters takes
// 64 MiB, so concurrent runs are bounded process-wide; attempts are bounded per client address.
const (
	maxArgonRuns    = 4
	attemptsPerMin  = 10
	attemptBurst    = 10
	bucketIdleAfter = 10 * time.Minute
	// maxBuckets caps the table: client addresses come from the network (or from a trusted
	// proxy's header), so an attacker could otherwise make it grow without bound.
	maxBuckets = 10_000
)

// argonSlots bounds concurrent argon2id runs; argonRuns counts the runs started (tests).
var (
	argonSlots = make(chan struct{}, maxArgonRuns)
	argonRuns  atomic.Int64
)

// acquireArgon waits for an argon2id slot or for ctx; the caller runs release when done.
func acquireArgon(ctx context.Context) (release func(), err error) {
	select {
	case argonSlots <- struct{}{}:
		argonRuns.Add(1)
		return func() { <-argonSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// attemptLimiter is a token bucket per client address: attemptsPerMin refill per minute, up to
// attemptBurst. Buckets idle for bucketIdleAfter are dropped when a new one is created, and the
// table never holds more than maxBuckets (the least recently used goes first).
type attemptLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// allow takes one token for ip at now or returns model.ErrTooManyAttempts.
func (l *attemptLimiter) allow(ip string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[ip]
	if !ok {
		if l.buckets == nil {
			l.buckets = map[string]*bucket{}
		}
		oldest, oldestAt := "", now
		for k, old := range l.buckets {
			if now.Sub(old.last) > bucketIdleAfter {
				delete(l.buckets, k)
			} else if old.last.Before(oldestAt) || oldest == "" {
				oldest, oldestAt = k, old.last
			}
		}
		if len(l.buckets) >= maxBuckets {
			delete(l.buckets, oldest) // the client that was seen longest ago starts over
		}
		b = &bucket{tokens: attemptBurst, last: now}
		l.buckets[ip] = b
	}
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens = min(attemptBurst, b.tokens+elapsed.Minutes()*attemptsPerMin)
	}
	b.last = now
	if b.tokens < 1 {
		return model.ErrTooManyAttempts
	}
	b.tokens--
	return nil
}

// AllowAttempt counts a login or setup attempt of the client in ctx (model.WithClientIP) and
// returns model.ErrTooManyAttempts when the client exceeded attemptsPerMin.
func (s *Service) AllowAttempt(ctx context.Context) error {
	return s.attempts.allow(model.ClientIP(ctx), s.now())
}
