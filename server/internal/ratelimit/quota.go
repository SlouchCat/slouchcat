package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Quota is a budget of units per period for a provider that counts requests by cost instead of by
// rate (YouTube Data API v3: 10'000 units per day, reset at midnight Pacific time; a search costs
// 100 units, a video lookup 1). Take spends units while some are left; the budget refills at the
// reset time that next computes. The count lives in memory: after a restart the provider's own
// answer (quota exceeded) is the backstop, reported through Exhaust. A Quota is safe for
// concurrent use.
type Quota struct {
	name  string
	limit int
	next  func(now time.Time) time.Time
	now   func() time.Time

	mu      sync.Mutex
	used    int
	resetAt time.Time
}

// QuotaOption configures NewQuota.
type QuotaOption func(*Quota)

// WithQuotaClock replaces the clock (tests).
func WithQuotaClock(now func() time.Time) QuotaOption {
	return func(q *Quota) { q.now = now }
}

// NewQuota returns a full budget of units called name; next returns the reset time after now
// (for example MidnightPacific).
func NewQuota(name string, units int, next func(now time.Time) time.Time, opts ...QuotaOption) *Quota {
	q := &Quota{name: name, limit: max(units, 0), next: next, now: time.Now}
	for _, opt := range opts {
		opt(q)
	}
	q.resetAt = next(q.now())
	return q
}

// Name returns the provider name of the quota.
func (q *Quota) Name() string { return q.name }

// refill starts a new period when the reset time has passed. The caller holds mu.
func (q *Quota) refill() {
	if now := q.now(); !now.Before(q.resetAt) {
		q.used = 0
		q.resetAt = q.next(now)
	}
}

// Take spends units and reports true when that many are left in the current period, else it
// spends nothing and reports false. A done ctx takes nothing.
func (q *Quota) Take(ctx context.Context, units int) bool {
	if ctx.Err() != nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refill()
	if units < 0 || q.used+units > q.limit {
		return false
	}
	q.used += units
	return true
}

// Exhaust marks the budget of the current period as spent (the provider answered "quota
// exceeded", for example after a restart lost the count).
func (q *Quota) Exhaust() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refill()
	q.used = q.limit
}

// Remaining returns the units left in the current period.
func (q *Quota) Remaining() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refill()
	return q.limit - q.used
}

// ResetAt returns the end of the current period.
func (q *Quota) ResetAt() time.Time {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refill()
	return q.resetAt
}

// pacific is the time zone of the YouTube quota reset. Without the zone database (tzdata is part
// of the image) it falls back to standard time, which resets an hour late in summer.
var pacific = func() *time.Location {
	if loc, err := time.LoadLocation("America/Los_Angeles"); err == nil {
		return loc
	}
	return time.FixedZone("PST", -8*60*60)
}()

// MidnightPacific returns the next midnight in America/Los_Angeles after now, when the daily
// quotas of Google APIs reset.
func MidnightPacific(now time.Time) time.Time {
	t := now.In(pacific)
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, pacific)
}
