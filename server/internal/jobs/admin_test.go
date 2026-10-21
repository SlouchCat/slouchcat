package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
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
