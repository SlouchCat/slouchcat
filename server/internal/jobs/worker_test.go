package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var errTest = errors.New("test failure")

// startWorker runs w until the test ends or stop is called; stop waits for Run to return.
func startWorker(t *testing.T, w *Worker) (stop func()) {
	t.Helper()
	w.poll = 5 * time.Millisecond
	w.heartbeat = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	stop = func() {
		cancel()
		<-done
	}
	t.Cleanup(stop)
	return stop
}

func waitForState(t *testing.T, q *Queue, id int64, want State) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, err := q.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if j.State == want {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %d stays %s, want %s", id, j.State, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWorkerRunsHandlers(t *testing.T) {
	q, _ := newTestQueue(t)
	w := NewWorker(q, "w1", discard)
	got := make(chan string, 1)
	w.Handle("a", func(_ context.Context, j *Job) error {
		got <- string(j.Payload)
		return nil
	})
	w.Handle("b", func(context.Context, *Job) error { panic("boom") })
	startWorker(t, w)

	ok := mustEnqueue(t, q, NewJob{Type: "a", Payload: []byte(`{"path":"/movies"}`)})
	waitForState(t, q, ok, StateDone)
	if p := <-got; p != `{"path":"/movies"}` {
		t.Fatalf("payload = %s", p)
	}

	panics := mustEnqueue(t, q, NewJob{Type: "b"})
	j := waitForState(t, q, panics, StateQueued)
	for j.Attempts == 0 {
		j = waitForState(t, q, panics, StateQueued)
	}
	if !strings.Contains(j.LastError, "panic: boom") {
		t.Fatalf("panic must fail the attempt, got %+v", j)
	}
}

func TestWorkerStopsCancelledJob(t *testing.T) {
	q, _ := newTestQueue(t)
	w := NewWorker(q, "w1", discard)
	started, stopped := make(chan struct{}), make(chan struct{})
	w.Handle("a", func(ctx context.Context, _ *Job) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	})
	startWorker(t, w)

	id := mustEnqueue(t, q, NewJob{Type: "a"})
	<-started
	if ok, err := q.Cancel(context.Background(), id); err != nil || !ok {
		t.Fatalf("cancel = %v, %v", ok, err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("handler context not cancelled after Cancel")
	}
	waitForState(t, q, id, StateCancelled)
}

func TestWorkerShutdownReleasesJob(t *testing.T) {
	q, _ := newTestQueue(t)
	w := NewWorker(q, "w1", discard)
	started := make(chan struct{})
	w.Handle("a", func(ctx context.Context, _ *Job) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	stop := startWorker(t, w)

	id := mustEnqueue(t, q, NewJob{Type: "a"})
	<-started
	stop()
	j, err := q.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if j.State != StateQueued || j.Attempts != 0 {
		t.Fatalf("interrupted job must be queued again without a used attempt: %+v", j)
	}
}
