// Package limit is the rate limit of the relay services: a token bucket per key, in memory only
// (concept 27, rules D-1, D-2 and D-5).
//
// The number of keys is bounded. When the table is full, buckets that have refilled completely are
// dropped first (they carry no state worth keeping), at most once a second, because a sweep walks
// the whole table. If the table is still full, an arbitrary bucket gives way to the new key. A
// newcomer is therefore never refused because strangers filled the table (security review M-2);
// what a flood of new keys gains is that some of its own buckets start fresh, which the limit per
// larger network and the service-wide limit still bound.
package limit

import (
	"sync"
	"time"
)

// Limiter allows Burst requests at once per key and refills Rate tokens per second.
type Limiter struct {
	rate    float64
	burst   float64
	maxKeys int
	now     func() time.Time

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// New returns a limiter. rate is in tokens per second, burst the size of a bucket, maxKeys the
// most keys held at once.
func New(rate float64, burst, maxKeys int) *Limiter {
	return &Limiter{rate: rate, burst: float64(burst), maxKeys: maxKeys, now: time.Now, buckets: map[string]*bucket{}}
}

// SetClock replaces the clock (tests).
func (l *Limiter) SetClock(now func() time.Time) { l.now = now }

// Allow takes one token from the bucket of key and reports whether there was one.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxKeys && now.Sub(l.lastSweep) >= time.Second {
			l.sweepLocked(now)
		}
		for k := range l.buckets {
			if len(l.buckets) < l.maxKeys {
				break
			}
			delete(l.buckets, k) // map order is arbitrary: an arbitrary bucket gives way
		}
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Len is the number of keys held (tests and the start-up check).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// Sweep drops the buckets that are full again. The services call it once a minute.
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(l.now())
}

func (l *Limiter) sweepLocked(now time.Time) {
	l.lastSweep = now
	for k, b := range l.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst {
			delete(l.buckets, k)
		}
	}
}
