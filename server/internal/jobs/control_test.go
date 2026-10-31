package jobs

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Pause, cancel, progress and history of concept 09 (sections 4.1, 4.4, 6.2, 6.3), on the
// injected clock of the queue and workers with a 5 ms heartbeat.

func newControlEnv(t *testing.T, types ...Type) (*Registry, *Queue, *clock) {
	t.Helper()
	q, c := newTestQueue(t)
	r := NewRegistry(q, discard)
	for _, ty := range types {
		r.Register(ty)
	}
	s := NewScheduler(q, r, discard, WithLocation(testZone))
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return r, q, c
}

func eventType(name string, h Handler) Type {
	return Type{Name: name, Group: GroupLibrary, Pool: PoolGeneral, Trigger: TriggerEvent, Priority: 3, Handler: h}
}

// A paused type is not claimed; resuming it claims again.
func TestPausedTypeIsNotClaimed(t *testing.T) {
	ctx := context.Background()
	noop := func(context.Context, *Job) error { return nil }
	r, q, _ := newControlEnv(t, eventType("a", noop), eventType("b", noop))
	mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "a1"})
	mustEnqueue(t, q, NewJob{Type: "b", IdempotencyKey: "b1"})
	if err := r.Pause(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Pause(ctx, "a"); err != nil {
		t.Errorf("second pause: %v, want idempotent", err)
	}
	if err := r.Pause(ctx, "nobody.home"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("pause of an unknown type: %v", err)
	}
	if paused, err := q.PausedTypes(ctx); err != nil || len(paused) != 1 || paused[0] != "a" {
		t.Errorf("paused types = %v %v", paused, err)
	}
	j := mustClaim(t, q, "w1")
	if j == nil || j.Type != "b" {
		t.Fatalf("claimed %+v, want the job of the type that is not paused", j)
	}
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("claimed %+v while its type is paused", j)
	}
	if err := r.Resume(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if j := mustClaim(t, q, "w1"); j == nil || j.Type != "a" {
		t.Fatalf("claimed %+v after resuming, want the job of type a", j)
	}
}

// A running job of a paused type stops at once and goes back to the queue without a used
// attempt, whatever the handler returned (nil included), and nobody hears of a failure; five
// pauses in a row leave attempts at 0. Resuming runs it to the end.
func TestPauseStopsRunningJobWithoutAttempt(t *testing.T) {
	ctx := context.Background()
	var started, finished atomic.Int32
	release := make(chan struct{})
	handler := func(ctx context.Context, j *Job) error {
		started.Add(1)
		select {
		case <-ctx.Done():
			return nil // a handler that swallows the cancel must still be released, not completed
		case <-release:
			finished.Add(1)
			return nil
		}
	}
	r, q, _ := newControlEnv(t, eventType("scan", handler))
	w := NewWorker(q, "w1", discard)
	r.Attach(w)
	var failures atomic.Int32
	w.OnFinalFailure(func(context.Context, Job, error) { failures.Add(1) })
	startWorker(t, w)
	id := mustEnqueue(t, q, NewJob{Type: "scan", IdempotencyKey: "scan:1", MaxAttempts: 3})

	for round := 1; round <= 5; round++ {
		waitForState(t, q, id, StateRunning)
		waitFor(t, func() bool { return int(started.Load()) == round })
		if err := r.Pause(ctx, "scan"); err != nil {
			t.Fatal(err)
		}
		j := waitForState(t, q, id, StateQueued)
		if j.Attempts != 0 {
			t.Fatalf("round %d: attempts = %d after the pause, want 0 (released, not failed)", round, j.Attempts)
		}
		waitFor(t, func() bool { return r.Running("scan") == 0 })
		if err := r.Resume(ctx, "scan"); err != nil {
			t.Fatal(err)
		}
	}
	waitForState(t, q, id, StateRunning)
	close(release)
	j := waitForState(t, q, id, StateDone)
	if j.Attempts != 1 || finished.Load() != 1 || failures.Load() != 0 {
		t.Errorf("after resuming: attempts %d, finished %d, failures %d; want 1, 1, 0", j.Attempts, finished.Load(), failures.Load())
	}
	if j.StartedAt.IsZero() || j.FinishedAt.IsZero() || j.FinishedAt.Before(j.StartedAt) {
		t.Errorf("done job without start and end: %+v", j)
	}
}

// The heartbeat is the fallback: a run that holds an old lease (no run list, as after a restart)
// notices the pause at its next heartbeat and is released.
func TestPauseNoticedByHeartbeat(t *testing.T) {
	ctx := context.Background()
	handler := func(ctx context.Context, j *Job) error {
		<-ctx.Done()
		return ctx.Err()
	}
	r, q, _ := newControlEnv(t, eventType("slow", handler))
	w := NewWorker(q, "w1", discard)
	w.Handle("slow", handler) // not attached: no run list
	startWorker(t, w)
	id := mustEnqueue(t, q, NewJob{Type: "slow"})
	waitForState(t, q, id, StateRunning)
	if err := r.Pause(ctx, "slow"); err != nil {
		t.Fatal(err)
	}
	if j := waitForState(t, q, id, StateQueued); j.Attempts != 0 {
		t.Errorf("attempts after the pause by heartbeat = %d, want 0", j.Attempts)
	}
}

// The administration's cancel runs through the cancel hook of the type, cancels the row and
// stops a running job at once; CancelRuns takes the queued jobs, with running also the running.
func TestCancelThroughHook(t *testing.T) {
	ctx := context.Background()
	var hooked []int64
	handler := func(ctx context.Context, j *Job) error {
		<-ctx.Done()
		return errors.New("interrupted")
	}
	ty := eventType("dl", handler)
	ty.Cancel = func(_ context.Context, j Job) error { hooked = append(hooked, j.ID); return nil }
	r, q, _ := newControlEnv(t, ty, eventType("other", handler))
	w := NewWorker(q, "w1", discard)
	r.Attach(w)
	startWorker(t, w)
	running := mustEnqueue(t, q, NewJob{Type: "dl", IdempotencyKey: "dl:1"})
	waitForState(t, q, running, StateRunning)
	queued1 := mustEnqueue(t, q, NewJob{Type: "dl", IdempotencyKey: "dl:2"})
	queued2 := mustEnqueue(t, q, NewJob{Type: "dl", IdempotencyKey: "dl:3"})
	other := mustEnqueue(t, q, NewJob{Type: "other", IdempotencyKey: "o:1"})

	ok, err := r.CancelJob(ctx, queued1)
	if err != nil || !ok {
		t.Fatalf("cancel queued: %v %v", ok, err)
	}
	if ok, err := r.CancelJob(ctx, queued1); err != nil || ok {
		t.Errorf("second cancel: %v %v, want nothing to cancel", ok, err)
	}
	if ok, err := r.CancelJob(ctx, 9999); err != nil || ok {
		t.Errorf("cancel unknown: %v %v", ok, err)
	}
	n, err := r.CancelRuns(ctx, "dl", false)
	if err != nil || n != 1 {
		t.Fatalf("cancel queued runs: %d %v, want the one waiting job", n, err)
	}
	if j, _ := q.Get(ctx, running); j.State != StateRunning {
		t.Fatalf("running job cancelled with scope queued: %+v", j)
	}
	n, err = r.CancelRuns(ctx, "dl", true)
	if err != nil || n != 1 {
		t.Fatalf("cancel all runs: %d %v, want the running job", n, err)
	}
	j := waitForState(t, q, running, StateCancelled)
	if j.FinishedAt.IsZero() {
		t.Errorf("cancelled job without finished_at: %+v", j)
	}
	waitFor(t, func() bool { return r.Running("dl") == 0 })
	if j, _ := q.Get(ctx, running); j.State != StateCancelled || j.Attempts != 1 {
		t.Errorf("cancelled running job after the handler returned = %+v, want cancelled (not failed, not retried)", j)
	}
	if len(hooked) != 3 || hooked[0] != queued1 || hooked[1] != queued2 || hooked[2] != running {
		t.Errorf("cancel hook saw %v, want %d, %d, %d", hooked, queued1, queued2, running)
	}
	if j, _ := q.Get(ctx, other); j.State == StateCancelled {
		t.Errorf("job of another type cancelled")
	}
}

// Progress is written at most every ProgressInterval and once more when the handler returned;
// the note is redacted and cut; Skip marks the run; the median counts runs that did work.
func TestProgressThrottledAndHistory(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	r := NewRegistry(q, discard)
	w := NewWorker(q, "w1", discard)
	r.Attach(w)
	id := mustEnqueue(t, q, NewJob{Type: "p"})
	j, err := q.Claim(ctx, "w1", []string{"p"})
	if err != nil || j == nil || j.StartedAt.IsZero() {
		t.Fatalf("claim: %+v %v, want started_at set", j, err)
	}
	rep := newReporter(q, "w1", id, discard)
	hctx := context.WithValue(ctx, reporterKey{}, rep)
	Progress(hctx, 1, 10, "first")
	Progress(hctx, 2, 10, "second")
	Progress(hctx, 3, 10, "third")
	if got, _ := q.Get(ctx, id); got.Progress.Done != 1 || got.Progress.Note != "first" {
		t.Errorf("progress after three reports in 0 s = %+v, want the first only", got.Progress)
	}
	c.Advance(ProgressInterval)
	Progress(hctx, 4, 10, "token=abcdef1234567890 "+strings.Repeat("x", 300))
	got, _ := q.Get(ctx, id)
	if got.Progress.Done != 4 || strings.Contains(got.Progress.Note, "abcdef") || len([]rune(got.Progress.Note)) != MaxProgressNote {
		t.Errorf("progress after the interval = done %d, note %q (%d runes); want 4, redacted, %d runes", got.Progress.Done, got.Progress.Note, len([]rune(got.Progress.Note)), MaxProgressNote)
	}
	Progress(hctx, 5, 10, "pending")
	rep.flush(ctx) // the worker flushes once more when the handler returns
	if got, _ := q.Get(ctx, id); got.Progress.Done != 5 || got.Progress.Note != "pending" {
		t.Errorf("progress after the final flush = %+v", got.Progress)
	}
	Skip(hctx)
	if got, _ := q.Get(ctx, id); !got.Skipped {
		t.Errorf("skip not recorded: %+v", got)
	}
	// Progress outside a job does nothing and does not panic.
	Progress(ctx, 1, 1, "nowhere")
	Skip(ctx)
	if err := q.Complete(ctx, "w1", id); err != nil {
		t.Fatal(err)
	}
	// A retry clears start, end, progress and skipped of the failed attempt.
	fid := mustEnqueue(t, q, NewJob{Type: "p", MaxAttempts: 1})
	fj, _ := q.Claim(ctx, "w1", []string{"p"})
	Progress(context.WithValue(ctx, reporterKey{}, newReporter(q, "w1", fid, discard)), 7, 9, "x")
	if err := q.Fail(ctx, "w1", fj, errTest); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.Get(ctx, fid); got.State != StateFailed || got.FinishedAt.IsZero() || got.Progress.Done != 7 {
		t.Fatalf("failed job = %+v, want finished_at and the last progress kept", got)
	}
	if _, err := q.Retry(ctx, fid); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.Get(ctx, fid); !got.StartedAt.IsZero() || !got.FinishedAt.IsZero() || got.Progress.Done != 0 || got.Skipped {
		t.Errorf("retried job = %+v, want start, end and progress cleared", got)
	}
}

// The median takes the newest HistoryKeep done runs that did work: skipped runs and runs
// without a start do not count, fewer than MedianMinRuns are not known.
func TestMedianDuration(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	run := func(typ string, d time.Duration, skipped bool) {
		t.Helper()
		id := mustEnqueue(t, q, NewJob{Type: typ})
		if _, err := q.Claim(ctx, "w", []string{typ}); err != nil {
			t.Fatal(err)
		}
		if skipped {
			if err := q.MarkSkipped(ctx, "w", id); err != nil {
				t.Fatal(err)
			}
		}
		c.Advance(d)
		if err := q.Complete(ctx, "w", id); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second, 40 * time.Second} {
		run("m", d, false)
	}
	if m, n, ok, err := q.MedianDuration(ctx, "m"); err != nil || ok || n != 4 || m != 25*time.Second {
		t.Errorf("median of four = %v %d %v %v, want 25 s of 4, not known", m, n, ok, err)
	}
	run("m", 6*time.Hour, false) // the first NAS scan: an outlier the median ignores
	run("m", time.Millisecond, true)
	run("m", time.Millisecond, true)
	if m, n, ok, err := q.MedianDuration(ctx, "m"); err != nil || !ok || n != 5 || m != 30*time.Second {
		t.Errorf("median of five = %v %d %v %v, want 30 s of 5, known (skipped runs ignored)", m, n, ok, err)
	}
	if m, n, ok, err := q.MedianDuration(ctx, "none"); err != nil || ok || n != 0 || m != 0 {
		t.Errorf("median without runs = %v %d %v %v", m, n, ok, err)
	}
}

// jobs.prune removes done and cancelled runs older than HistoryAge but keeps the newest
// HistoryKeep per type, leaves failed runs and works in batches of PruneBatch.
func TestPruneKeepsNewestAndFailed(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	finish := func(typ string, state State) {
		t.Helper()
		id := mustEnqueue(t, q, NewJob{Type: typ, MaxAttempts: 1})
		j, err := q.Claim(ctx, "w", []string{typ})
		if err != nil || j == nil {
			t.Fatal(err)
		}
		switch state {
		case StateDone:
			err = q.Complete(ctx, "w", id)
		case StateFailed:
			err = q.Fail(ctx, "w", j, errTest)
		case StateCancelled:
			_, err = q.Cancel(ctx, id)
		}
		if err != nil {
			t.Fatal(err)
		}
		c.Advance(time.Second)
	}
	for range PruneBatch + 60 {
		finish("old", StateDone)
	}
	for range 10 {
		finish("old", StateCancelled)
		finish("old", StateFailed)
	}
	for range 10 {
		finish("young", StateDone)
	}
	c.Advance(HistoryAge) // everything above is older than the age now
	for range 3 {
		finish("old", StateDone) // recent runs
	}
	n, err := q.Prune(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count := func(typ string, state State) int {
		var n int
		if err := q.db.QueryRowContext(ctx, `SELECT count(*) FROM jobs WHERE type = ? AND state = ?`, typ, string(state)).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	kept := count("old", StateDone) + count("old", StateCancelled)
	if kept != HistoryKeep {
		t.Errorf("done and cancelled runs of 'old' after prune = %d, want the newest %d", kept, HistoryKeep)
	}
	if count("old", StateFailed) != 10 {
		t.Errorf("failed runs pruned: %d left, want 10", count("old", StateFailed))
	}
	if count("young", StateDone) != 10 {
		t.Errorf("young runs pruned: %d left, want 10 (fewer than %d)", count("young", StateDone), HistoryKeep)
	}
	if want := int64(PruneBatch + 60 + 10 + 3 - HistoryKeep); n != want { // all done and cancelled runs of 'old' but the newest 50
		t.Errorf("pruned %d rows, want %d", n, want)
	}
	// The three recent runs are among the kept ones.
	var recent int
	if err := q.db.QueryRowContext(ctx, `SELECT count(*) FROM jobs WHERE type = 'old' AND state = 'done' AND finished_at >= ?`, c.Now().Add(-10*time.Second).UnixMilli()).Scan(&recent); err != nil || recent != 3 {
		t.Errorf("recent runs kept = %d %v, want 3", recent, err)
	}
	// The registry carries jobs.prune as a schedule type whose handler prunes.
	r := NewRegistry(q, discard)
	ty, ok := r.Lookup(JobPrune)
	if !ok || !ty.Scheduled() || ty.Every != PruneInterval || ty.StartDelay != nil {
		t.Fatalf("jobs.prune = %+v", ty)
	}
	if err := ty.Handler(ctx, &Job{}); err != nil {
		t.Errorf("prune handler: %v", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A report that comes before the interval is written by the timer when the interval is over
// (B-1), stop drops that timer, and the reporter marks where the progress series began: the first
// report with an extent, again when the extent changes or done falls (B-3). The registry serves
// it for the runs on its list.
func TestProgressTimerAndSeries(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	r := NewRegistry(q, discard)
	id := mustEnqueue(t, q, NewJob{Type: "p"})
	if j, err := q.Claim(ctx, "w1", []string{"p"}); err != nil || j == nil {
		t.Fatalf("claim: %+v %v", j, err)
	}
	rep := newReporter(q, "w1", id, discard)
	rep.interval = 50 * time.Millisecond
	hctx := context.WithValue(ctx, reporterKey{}, rep)
	r.runs.add(id, "p", func(error) {}, rep)
	defer r.runs.remove(id)

	Progress(hctx, 1, 0, "walk") // written at once
	Progress(hctx, 7, 0, "walk") // too early: the timer writes it
	if got, _ := q.Get(ctx, id); got.Progress.Done != 1 {
		t.Fatalf("progress right after the early report = %+v, want 1", got.Progress)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got, _ := q.Get(ctx, id); got.Progress.Done == 7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the early report was not written by the timer")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, _, ok := rep.series(); ok {
		t.Error("a report without an extent opened a series")
	}

	// The series: first extent at t1 with done 125; same extent keeps it; done falling and a
	// new extent reset it.
	c.Advance(time.Minute)
	t1 := c.Now()
	Progress(hctx, 125, 1791, "write")
	c.Advance(10 * time.Second)
	Progress(hctx, 250, 1791, "write")
	if since, done, ok := r.ProgressSince(id); !ok || !since.Equal(t1) || done != 125 {
		t.Errorf("series after two reports = %v %d %v, want %v 125 true", since, done, ok, t1)
	}
	c.Advance(10 * time.Second)
	t2 := c.Now()
	Progress(hctx, 3, 1791, "next phase")
	if since, done, ok := r.ProgressSince(id); !ok || !since.Equal(t2) || done != 3 {
		t.Errorf("series after done fell = %v %d %v, want %v 3 true", since, done, ok, t2)
	}
	c.Advance(10 * time.Second)
	t3 := c.Now()
	Progress(hctx, 4, 2000, "new extent")
	if since, done, ok := r.ProgressSince(id); !ok || !since.Equal(t3) || done != 4 {
		t.Errorf("series after the extent changed = %v %d %v, want %v 4 true", since, done, ok, t3)
	}
	if _, _, ok := r.ProgressSince(id + 1000); ok {
		t.Error("a job that does not run here has a series")
	}

	// stop drops a waiting timer: the pending report stays unwritten.
	Progress(hctx, 5, 2000, "pending") // right after the write of "new extent": too early
	rep.stop()
	time.Sleep(120 * time.Millisecond)
	if got, _ := q.Get(ctx, id); got.Progress.Done != 4 {
		t.Errorf("progress after stop = %+v, want the report before the stop (4)", got.Progress)
	}
	rep.flush(ctx) // the worker's final flush still writes it
	if got, _ := q.Get(ctx, id); got.Progress.Done != 5 {
		t.Errorf("progress after the final flush = %+v, want 5", got.Progress)
	}

	// The Recorder of the handler tests collects the same calls.
	rec := &Recorder{}
	rctx := WithRecorder(ctx, rec)
	Progress(rctx, 1, 2, "a")
	Skip(rctx)
	if last, ok := rec.Last(); !ok || last.Done != 1 || last.Note != "a" || !rec.Skipped {
		t.Errorf("recorder = %+v skipped %v", rec.Reports, rec.Skipped)
	}
}
