package jobs

import (
	"context"
	"errors"
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
