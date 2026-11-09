package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// M8: OnFinalFailure hears of a job that used its last attempt, not of one that is retried.
func TestWorkerOnFinalFailure(t *testing.T) {
	q, _ := newTestQueue(t)
	w := NewWorker(q, "w1", discard)
	w.Handle("a", func(context.Context, *Job) error { return errTest })
	type report struct {
		job Job
		err error
	}
	got := make(chan report, 4)
	w.OnFinalFailure(func(_ context.Context, j Job, err error) { got <- report{j, err} })
	startWorker(t, w)

	retried := mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 2})
	j := waitForState(t, q, retried, StateQueued)
	for j.Attempts == 0 {
		j = waitForState(t, q, retried, StateQueued)
	}
	last := mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 1})
	waitForState(t, q, last, StateFailed)
	select {
	case r := <-got:
		if r.job.ID != last || r.job.Type != "a" || r.job.Attempts != 1 || !errors.Is(r.err, errTest) {
			t.Errorf("report = %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no report of the final failure")
	}
	select {
	case r := <-got:
		t.Errorf("unexpected report %+v", r)
	case <-time.After(50 * time.Millisecond):
	}
}

// Concept 11: a second hook (the activity log next to notify.JobFailed) hears of the same final
// failure, after the first one.
func TestWorkerOnFinalFailureRunsEveryHook(t *testing.T) {
	q, _ := newTestQueue(t)
	w := NewWorker(q, "w1", discard)
	w.Handle("a", func(context.Context, *Job) error { return errTest })
	got := make(chan string, 4)
	w.OnFinalFailure(func(_ context.Context, j Job, _ error) { got <- "notify:" + j.Type })
	w.OnFinalFailure(func(_ context.Context, j Job, _ error) { got <- "activity:" + j.Type })
	startWorker(t, w)
	waitForState(t, q, mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 1}), StateFailed)
	for _, want := range []string{"notify:a", "activity:a"} {
		select {
		case r := <-got:
			if r != want {
				t.Errorf("hook call = %q, want %q", r, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no call %q", want)
		}
	}
}

// Concept 18 (trickplay.generate): a handler error wrapped in Final ends the job as failed at its
// first attempt and is reported as final; an attempt an expired lease cut short gets the next one.
func TestWorkerFinalError(t *testing.T) {
	q, clk := newTestQueue(t)
	w := NewWorker(q, "w1", discard)
	w.Handle("a", func(context.Context, *Job) error { return fmt.Errorf("wrapped: %w", Final(errTest)) })
	got := make(chan Job, 2)
	w.OnFinalFailure(func(_ context.Context, j Job, err error) {
		if errors.Is(err, errTest) {
			got <- j
		}
	})
	startWorker(t, w)
	id := mustEnqueue(t, q, NewJob{Type: "a", MaxAttempts: 2})
	if j := waitForState(t, q, id, StateFailed); j.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", j.Attempts)
	}
	select {
	case j := <-got:
		if j.ID != id {
			t.Errorf("final failure of %d, want %d", j.ID, id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no report of the final failure")
	}

	// An expired lease of the first attempt (a crash) returns the job to the queue.
	other := mustEnqueue(t, q, NewJob{Type: "b", MaxAttempts: 2})
	if j, err := q.Claim(context.Background(), "gone", []string{"b"}); err != nil || j == nil || j.ID != other {
		t.Fatalf("claim = %+v, %v", j, err)
	}
	clk.Advance(LeaseDuration + time.Second)
	j, err := q.Claim(context.Background(), "w2", []string{"b"})
	if err != nil || j == nil || j.ID != other || j.Attempts != 2 {
		t.Fatalf("claim after the expired lease = %+v, %v", j, err)
	}
}
