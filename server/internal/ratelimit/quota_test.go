package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestQuotaTakeAndReset(t *testing.T) {
	var mu sync.Mutex
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC) // 03:00 PDT
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	q := NewQuota("youtube", 10000, MidnightPacific, WithQuotaClock(clock))
	ctx := context.Background()
	if want := time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC); !q.ResetAt().Equal(want) {
		t.Fatalf("reset at %v, want %v (midnight PDT)", q.ResetAt(), want)
	}
	for i := range 100 {
		if !q.Take(ctx, 100) {
			t.Fatalf("search %d refused", i+1)
		}
	}
	if q.Take(ctx, 1) || q.Remaining() != 0 {
		t.Fatalf("budget not exhausted after 10'000 units (remaining %d)", q.Remaining())
	}
	mu.Lock()
	now = time.Date(2026, 10, 20, 6, 59, 59, 0, time.UTC)
	mu.Unlock()
	if q.Take(ctx, 1) {
		t.Fatal("unit granted before the reset")
	}
	mu.Lock()
	now = time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC)
	mu.Unlock()
	if !q.Take(ctx, 100) || q.Remaining() != 9900 {
		t.Fatalf("after the reset: remaining %d, want 9900", q.Remaining())
	}
	// A refused take spends nothing.
	if q.Take(ctx, 9901) || q.Remaining() != 9900 {
		t.Fatalf("oversized take changed the budget: %d", q.Remaining())
	}
	q.Exhaust()
	if q.Take(ctx, 1) {
		t.Fatal("unit granted after Exhaust")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	mu.Lock()
	now = now.Add(48 * time.Hour)
	mu.Unlock()
	if q.Take(cancelled, 1) {
		t.Fatal("unit granted to a cancelled context")
	}
	if q.Remaining() != 10000 {
		t.Fatalf("remaining %d after two days, want 10000", q.Remaining())
	}
}

func TestMidnightPacific(t *testing.T) {
	cases := []struct{ now, want time.Time }{
		// Summer time (UTC-7) and winter time (UTC-8).
		{time.Date(2026, 7, 1, 6, 59, 0, 0, time.UTC), time.Date(2026, 7, 1, 7, 0, 0, 0, time.UTC)},
		{time.Date(2026, 7, 1, 7, 0, 0, 0, time.UTC), time.Date(2026, 7, 2, 7, 0, 0, 0, time.UTC)},
		{time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC), time.Date(2026, 12, 2, 8, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := MidnightPacific(c.now); !got.Equal(c.want) {
			t.Errorf("MidnightPacific(%v) = %v, want %v", c.now, got.UTC(), c.want)
		}
	}
}
