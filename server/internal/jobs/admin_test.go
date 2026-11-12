package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// failFinally claims job id of type "a" or "b" and fails it with one attempt left.
func failFinally(t *testing.T, q *Queue, msg string) *Job {
	t.Helper()
	j := mustClaim(t, q, "w1")
	if j == nil {
		t.Fatal("nothing to claim")
	}
	if err := q.Fail(context.Background(), "w1", j, errors.New(msg)); err != nil {
		t.Fatal(err)
	}
	return j
}

func TestStatsListFailedAndRetry(t *testing.T) {
	ctx := context.Background()
	q, clk := newTestQueue(t)

	first := mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 1, IdempotencyKey: "first"})
	failFinally(t, q, "first broke")
	clk.Advance(time.Second)
	second := mustEnqueue(t, q, NewJob{Type: "b", MaxAttempts: 1})
	failFinally(t, q, "second broke")
	mustEnqueue(t, q, NewJob{Type: "a", Delay: time.Hour})
	mustEnqueue(t, q, NewJob{Type: "b"})
	if running := mustClaim(t, q, "w2"); running == nil {
		t.Fatal("no job to run")
	}

	queued, running, failed, err := q.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if queued != 1 || running != 1 || failed != 2 {
		t.Errorf("stats = %d queued, %d running, %d failed; want 1, 1, 2", queued, running, failed)
	}
	list, err := q.ListFailed(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != second || list[1].ID != first || list[0].LastError != "second broke" ||
		list[0].Attempts != 1 || list[0].MaxAttempts != 1 || list[0].State != StateFailed || list[0].UpdatedAt.IsZero() {
		t.Fatalf("failed jobs = %+v, want second then first", list)
	}
	if one, err := q.ListFailed(ctx, 1); err != nil || len(one) != 1 {
		t.Fatalf("limit 1: %v, %v", one, err)
	}

	clk.Advance(time.Minute)
	j, err := q.Retry(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if j.ID != second || j.State != StateQueued || j.Attempts != 0 || !j.RunAfter.Equal(clk.Now()) {
		t.Errorf("retried job = %+v, want queued with attempts 0 to run now", j)
	}
	if _, err := q.Retry(ctx, second); !errors.Is(err, model.ErrJobNotFailed) {
		t.Errorf("retry of a queued job: %v, want ErrJobNotFailed", err)
	}
	if _, err := q.Retry(ctx, 9999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("retry of an unknown job: %v, want ErrNotFound", err)
	}

	// The key of the failed job is held by an active job: the retry answers that job.
	active := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "first", Delay: time.Hour})
	j, err = q.Retry(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if j.ID != active {
		t.Errorf("retry with an active duplicate returned job %d, want %d", j.ID, active)
	}
	if got, err := q.Get(ctx, first); err != nil || got.State != StateFailed {
		t.Errorf("failed job after the refused retry: %+v, %v", got, err)
	}
}

// Concept 20, P-6: RetryFailed queues every failed job of a type or of all in one transaction.
// Two failures share the key "dup": only the newer is queued, the older one is merged (one UPDATE
// over both would break the unique index jobs_active_idempotency_key). The failure with key
// "held" has an active twin and stays failed. DismissFailed cancels what is left.
func TestRetryAndDismissFailed(t *testing.T) {
	ctx := context.Background()
	q, clk := newTestQueue(t)
	fail := func(j NewJob) int64 {
		t.Helper()
		j.MaxAttempts = 1
		id := mustEnqueue(t, q, j)
		failFinally(t, q, "broke")
		clk.Advance(time.Second)
		return id
	}
	dupOld := fail(NewJob{Type: "a", IdempotencyKey: "dup"})
	dupNew := fail(NewJob{Type: "a", IdempotencyKey: "dup"})
	held := fail(NewJob{Type: "a", IdempotencyKey: "held"})
	plain := fail(NewJob{Type: "b"})
	twin := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "held", Delay: time.Hour})
	state := func(id int64) State {
		t.Helper()
		j, err := q.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return j.State
	}

	// The filter: only b.
	if r, err := q.RetryFailed(ctx, "b"); err != nil || r != (FailedRetry{Retried: 1}) {
		t.Fatalf("retry of b = %+v, %v; want 1 retried", r, err)
	}
	if state(plain) != StateQueued || state(dupNew) != StateFailed {
		t.Fatalf("after the retry of b: plain %s, dup %s", state(plain), state(dupNew))
	}
	// Every type: the newer dup is queued, the older merged, held skipped.
	r, err := q.RetryFailed(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if r != (FailedRetry{Retried: 1, Skipped: 1, Merged: 1}) {
		t.Fatalf("retry of every type = %+v, want 1 retried, 1 skipped, 1 merged", r)
	}
	if state(dupNew) != StateQueued || state(dupOld) != StateCancelled || state(held) != StateFailed || state(twin) != StateQueued {
		t.Fatalf("states: dupNew %s, dupOld %s, held %s, twin %s", state(dupNew), state(dupOld), state(held), state(twin))
	}
	if j, err := q.Get(ctx, dupNew); err != nil || j.Attempts != 0 || !j.RunAfter.Equal(clk.Now()) {
		t.Fatalf("requeued job = %+v, %v; want attempts 0 to run now", j, err)
	}

	if n, err := q.DismissFailed(ctx, "b"); err != nil || n != 0 {
		t.Fatalf("dismiss of b = %d, %v; want 0", n, err)
	}
	if n, err := q.DismissFailed(ctx, ""); err != nil || n != 1 {
		t.Fatalf("dismiss of every type = %d, %v; want 1", n, err)
	}
	if state(held) != StateCancelled || state(twin) != StateQueued {
		t.Fatalf("after the dismiss: held %s, twin %s", state(held), state(twin))
	}
	if r, err := q.RetryFailed(ctx, ""); err != nil || r != (FailedRetry{}) {
		t.Fatalf("retry without failures = %+v, %v", r, err)
	}
}

// Concept 11: the queries of the overview. ListRunningJobs lists the running jobs of every type,
// the newest start first, at most the limit; ListFailedJobsNewestPerType has one row per type with
// the newest failure and the number of failures, the type with the newest failure first.
func TestDashboardOverviewQueries(t *testing.T) {
	ctx := context.Background()
	q, clk := newTestQueue(t)

	// Failures: a at 0 s ("a old"), b at 1 s, a at 2 s ("a new"): a is newest with two failures.
	mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 1})
	failFinally(t, q, "a old")
	clk.Advance(time.Second)
	mustEnqueue(t, q, NewJob{Type: "b", MaxAttempts: 1})
	failFinally(t, q, "b broke")
	clk.Advance(time.Second)
	aNew := mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 1})
	failFinally(t, q, "a new")

	rows, err := q.q.ListFailedJobsNewestPerType(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Type != "a" || rows[0].FailedCount != 2 || rows[0].LastError.String != "a new" ||
		rows[0].UpdatedAt != clk.Now().UnixMilli() || rows[1].Type != "b" || rows[1].FailedCount != 1 || rows[1].LastError.String != "b broke" {
		t.Fatalf("failed by type = %+v, want a (2, a new) then b (1, b broke)", rows)
	}
	// A retried failure leaves the group; b now has the newest failure.
	if _, err := q.Retry(ctx, aNew); err != nil {
		t.Fatal(err)
	}
	rows, err = q.q.ListFailedJobsNewestPerType(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Type != "b" || rows[1].Type != "a" || rows[1].FailedCount != 1 || rows[1].LastError.String != "a old" {
		t.Fatalf("after the retry: %+v, want b then a (1, a old)", rows)
	}

	// Running: the retried a job and four more, started one second apart; a queued one stays out.
	var started []int64
	for range 4 {
		mustEnqueue(t, q, NewJob{Type: "b"})
	}
	for range 5 {
		clk.Advance(time.Second)
		j := mustClaim(t, q, "w")
		if j == nil {
			t.Fatal("nothing to claim")
		}
		started = append(started, j.ID)
	}
	mustEnqueue(t, q, NewJob{Type: "a"})
	running, err := q.q.ListRunningJobs(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 3 || running[0].ID != started[4] || running[1].ID != started[3] || running[2].ID != started[2] {
		t.Fatalf("running = %v, want the last three started %v, newest first", ids(running), started[2:])
	}
	for _, r := range running {
		if r.State != string(StateRunning) {
			t.Errorf("job %d is %s, want running", r.ID, r.State)
		}
	}
}

func ids(rows []systemq.Job) []int64 {
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}
