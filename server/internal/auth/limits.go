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

// Limiter is a token bucket per client address: perMin tokens refill per minute, up to burst.
// Buckets idle for bucketIdleAfter are dropped when a new one is created, and the table never
// holds more than maxBuckets (the least recently used goes first).
//
// The zero Limiter is the limit of login, setup, PIN and invitation attempts, which share one
// pot. NewLimiter makes a pot of its own for an operation that must not empty that one -- the
// wizard's root list, which the setup page fetches on every load (idea 02, E5).
type Limiter struct {
	mu      sync.Mutex
	perMin  float64
	burst   float64
	buckets map[string]*bucket
}

// NewLimiter returns a limiter with its own pot: perMin tokens per minute and client address, up
// to burst at once.
func NewLimiter(perMin, burst float64) *Limiter {
	return &Limiter{perMin: perMin, burst: burst}
}

// rates returns the configured rates, or those of the shared attempt pot for the zero Limiter.
func (l *Limiter) rates() (perMin, burst float64) {
	if l.perMin == 0 {
		return attemptsPerMin, attemptBurst
	}
	return l.perMin, l.burst
}

type bucket struct {
	tokens float64
	last   time.Time
}

// Allow takes one token for ip at now or returns model.ErrTooManyAttempts.
func (l *Limiter) Allow(ip string, now time.Time) error {
	perMin, burst := l.rates()
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
		b = &bucket{tokens: burst, last: now}
		l.buckets[ip] = b
	}
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens = min(burst, b.tokens+elapsed.Minutes()*perMin)
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
	return s.attempts.Allow(model.ClientIP(ctx), s.now())
}
