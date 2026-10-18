package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store"
)

// clock is a manually advanced clock, safe for concurrent use.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func newTestQueue(t *testing.T) (*Queue, *clock) {
	t.Helper()
	db, err := store.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: discard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	c := &clock{now: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	return NewQueue(db.Write, c.Now), c
}

func mustEnqueue(t *testing.T, q *Queue, j NewJob) int64 {
	t.Helper()
	id, err := q.Enqueue(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustClaim(t *testing.T, q *Queue, worker string) *Job {
	t.Helper()
	j, err := q.Claim(context.Background(), worker, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestEnqueueDeduplicatesByIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	q, _ := newTestQueue(t)

	first := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "scan:/movies"})
	second := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "scan:/movies"})
	other := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "scan:/series"})
	unkeyed1 := mustEnqueue(t, q, NewJob{Type: "a"})
	unkeyed2 := mustEnqueue(t, q, NewJob{Type: "a"})
	if first != second {
		t.Fatalf("same key must return the existing job: %d != %d", first, second)
	}
	if other == first || unkeyed1 == unkeyed2 {
		t.Fatal("different or missing keys must create separate jobs")
	}

	// Still deduplicated while running; reusable after completion.
	j := mustClaim(t, q, "w1")
	if j.ID != first {
		t.Fatalf("claimed %d, want %d", j.ID, first)
	}
	if id := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "scan:/movies"}); id != first {
		t.Fatalf("running job must keep its key: got %d", id)
	}
	if err := q.Complete(ctx, "w1", first); err != nil {
		t.Fatal(err)
	}
	if id := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "scan:/movies"}); id == first {
		t.Fatal("finished job must release its key")
	}
}

func TestEnqueueValidates(t *testing.T) {
	q, _ := newTestQueue(t)
	for name, j := range map[string]NewJob{
		"empty type":   {},
		"priority -1":  {Type: "a", Priority: -1},
		"priority 10":  {Type: "a", Priority: 10},
		"invalid json": {Type: "a", Payload: []byte("{")},
	} {
		if _, err := q.Enqueue(context.Background(), j); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestDebounceCoalescesWithinWindow(t *testing.T) {
	q, c := newTestQueue(t)
	id := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "scan.path:/movies/A", Delay: DebounceWindow})
	c.Advance(time.Second)
	if got := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "scan.path:/movies/A", Delay: DebounceWindow}); got != id {
		t.Fatalf("enqueue inside the window must coalesce: %d != %d", got, id)
	}
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("debounced job claimed before the window ended: %+v", j)
	}
	c.Advance(time.Second)
	if j := mustClaim(t, q, "w1"); j == nil || j.ID != id {
		t.Fatalf("debounced job not claimable after the window: %+v", j)
	}
}

func TestClaimOrdersByPriorityThenAge(t *testing.T) {
	q, c := newTestQueue(t)
	low := mustEnqueue(t, q, NewJob{Type: "a", Priority: 1})
	c.Advance(time.Millisecond)
	midOld := mustEnqueue(t, q, NewJob{Type: "b", Priority: 5})
	c.Advance(time.Millisecond)
	high := mustEnqueue(t, q, NewJob{Type: "a", Priority: 9})
	c.Advance(time.Millisecond)
	midNew := mustEnqueue(t, q, NewJob{Type: "a", Priority: 5})
	mustEnqueue(t, q, NewJob{Type: "unhandled", Priority: 9})

	for _, want := range []int64{high, midOld, midNew, low} {
		j := mustClaim(t, q, "w1")
		if j == nil || j.ID != want {
			t.Fatalf("claimed %+v, want job %d", j, want)
		}
		if j.State != StateRunning || j.Attempts != 1 || !j.LeaseUntil.Equal(c.Now().Add(LeaseDuration)) {
			t.Fatalf("claimed job has wrong lease state: %+v", j)
		}
	}
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("jobs of unregistered types must not be claimed: %+v", j)
	}
}

func TestExpiredLeaseIsReclaimed(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	id := mustEnqueue(t, q, NewJob{Type: "a"})

	if j := mustClaim(t, q, "w1"); j == nil || j.ID != id {
		t.Fatalf("w1 claim: %+v", j)
	}
	c.Advance(LeaseDuration - time.Second)
	if err := q.Heartbeat(ctx, "w1", id); err != nil {
		t.Fatalf("heartbeat within the lease: %v", err)
	}
	c.Advance(LeaseDuration - time.Second)
	if j := mustClaim(t, q, "w2"); j != nil {
		t.Fatalf("job with an extended lease must not be reclaimed: %+v", j)
	}

	c.Advance(2 * time.Second) // lease expired
	j := mustClaim(t, q, "w2")
	if j == nil || j.ID != id || j.Attempts != 2 {
		t.Fatalf("w2 must reclaim the expired job as attempt 2: %+v", j)
	}
	if err := q.Heartbeat(ctx, "w1", id); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old worker heartbeat = %v, want ErrLeaseLost", err)
	}
	if err := q.Complete(ctx, "w1", id); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old worker complete = %v, want ErrLeaseLost", err)
	}
	if err := q.Complete(ctx, "w2", id); err != nil {
		t.Fatal(err)
	}
}

func TestExhaustedLeaseFailsJob(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	id := mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 1})
	mustClaim(t, q, "w1")
	c.Advance(LeaseDuration + time.Second)
	if j := mustClaim(t, q, "w2"); j != nil {
		t.Fatalf("job without attempts left must not be reclaimed: %+v", j)
	}
	j, err := q.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if j.State != StateFailed || j.LastError == "" {
		t.Fatalf("want failed job with error, got %+v", j)
	}
}

func TestFailRetriesWithBackoff(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	id := mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 3})

	for attempt, wait := range []time.Duration{10 * time.Second, 20 * time.Second} {
		j := mustClaim(t, q, "w1")
		if j == nil || j.Attempts != attempt+1 {
			t.Fatalf("attempt %d: claimed %+v", attempt+1, j)
		}
		if err := q.Fail(ctx, "w1", j, errTest); err != nil {
			t.Fatal(err)
		}
		got, err := q.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != StateQueued || !got.RunAfter.Equal(c.Now().Add(wait)) || got.LastError != errTest.Error() {
			t.Fatalf("after attempt %d: %+v, want queued with run_after +%s", attempt+1, got, wait)
		}
		c.Advance(wait - time.Millisecond)
		if j := mustClaim(t, q, "w1"); j != nil {
			t.Fatalf("job claimed before its backoff ended: %+v", j)
		}
		c.Advance(time.Millisecond)
	}

	j := mustClaim(t, q, "w1")
	if err := q.Fail(ctx, "w1", j, errTest); err != nil {
		t.Fatal(err)
	}
	got, err := q.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateFailed || got.Attempts != 3 {
		t.Fatalf("after the last attempt: %+v, want failed with 3 attempts", got)
	}
	if b := Backoff(4); b != 80*time.Second {
		t.Fatalf("Backoff(4) = %s, want 80s", b)
	}
}

func TestCancel(t *testing.T) {
	ctx := context.Background()
	q, _ := newTestQueue(t)
	queued := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "k"})
	if ok, err := q.Cancel(ctx, queued); err != nil || !ok {
		t.Fatalf("cancel queued = %v, %v", ok, err)
	}
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("cancelled job claimed: %+v", j)
	}
	if id := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "k"}); id == queued {
		t.Fatal("cancelled job must release its key")
	}

	running := mustClaim(t, q, "w1")
	if ok, err := q.Cancel(ctx, running.ID); err != nil || !ok {
		t.Fatalf("cancel running = %v, %v", ok, err)
	}
	if err := q.Heartbeat(ctx, "w1", running.ID); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("heartbeat of a cancelled job = %v, want ErrLeaseLost", err)
	}
	if ok, err := q.Cancel(ctx, running.ID); err != nil || ok {
		t.Fatalf("cancelling twice = %v, %v; want false", ok, err)
	}
	got, err := q.Get(ctx, running.ID)
	if err != nil || got.State != StateCancelled {
		t.Fatalf("state = %s, %v; want cancelled", got.State, err)
	}
}
