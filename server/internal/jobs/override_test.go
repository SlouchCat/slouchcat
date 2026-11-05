package jobs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// The pause override of concept 14, 6.4 and 6.5 (jobs.override_pause): playback requests run while
// their type is paused, at most MaxPauseOverrides at a time.

func mustOverride(t *testing.T, q *Queue, j NewJob) PauseOverride {
	t.Helper()
	r, err := q.EnqueueOverridingPause(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A paused type is not claimed, except its job with override_pause.
func TestPausedTypeClaimsOverridingJob(t *testing.T) {
	ctx := context.Background()
	noop := func(context.Context, *Job) error { return nil }
	r, q, _ := newControlEnv(t, eventType("a", noop))
	plain := mustEnqueue(t, q, NewJob{Type: "a", Priority: 9, IdempotencyKey: "plain"})
	if err := r.Pause(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	ov := mustOverride(t, q, NewJob{Type: "a", Priority: 1, IdempotencyKey: "playback"})
	if !ov.Inserted || !ov.OverridesPause {
		t.Fatalf("EnqueueOverridingPause = %+v, want a new job with the override", ov)
	}
	j := mustClaim(t, q, "w1")
	if j == nil || j.ID != ov.ID || !j.OverridesPause {
		t.Fatalf("claimed %+v, want the overriding job %d (the plain one waits for the pause)", j, ov.ID)
	}
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("claimed %+v while its type is paused", j)
	}
	if err := r.Resume(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if j := mustClaim(t, q, "w1"); j == nil || j.ID != plain || j.OverridesPause {
		t.Fatalf("claimed %+v after resuming, want the plain job %d", j, plain)
	}
}

// A pause stops the plain run of a type at once but not its overriding run, neither through the
// run list nor through the heartbeat of a worker without one.
func TestPauseDoesNotStopOverridingRun(t *testing.T) {
	ctx := context.Background()
	release := make(chan struct{})
	handler := func(ctx context.Context, j *Job) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	}
	r, q, _ := newControlEnv(t, eventType("read", handler))
	attached, bare, plainWorker := NewWorker(q, "w1", discard), NewWorker(q, "w2", discard), NewWorker(q, "w3", discard)
	r.Attach(attached)
	bare.Handle("read", handler) // no run list: only the heartbeat sees the pause
	first := mustOverride(t, q, NewJob{Type: "read", IdempotencyKey: "playback:1"})
	second := mustOverride(t, q, NewJob{Type: "read", IdempotencyKey: "playback:2"})
	startWorker(t, attached)
	startWorker(t, bare)
	waitForState(t, q, first.ID, StateRunning)
	waitForState(t, q, second.ID, StateRunning)
	plain := mustEnqueue(t, q, NewJob{Type: "read", IdempotencyKey: "scan:1"})
	r.Attach(plainWorker)
	startWorker(t, plainWorker)
	waitForState(t, q, plain, StateRunning)

	if err := r.Pause(ctx, "read"); err != nil {
		t.Fatal(err)
	}
	if j := waitForState(t, q, plain, StateQueued); j.Attempts != 0 {
		t.Errorf("the plain run after the pause: attempts %d, want 0 (released)", j.Attempts)
	}
	time.Sleep(50 * time.Millisecond) // ten heartbeats of the test workers
	for _, id := range []int64{first.ID, second.ID} {
		if j, err := q.Get(ctx, id); err != nil || j.State != StateRunning {
			t.Errorf("overriding job %d after the pause = %+v %v, want still running", id, j, err)
		}
	}
	close(release)
	waitForState(t, q, first.ID, StateDone)
	waitForState(t, q, second.ID, StateDone)
	if j, _ := q.Get(ctx, plain); j.State != StateQueued {
		t.Errorf("the plain job ran during the pause: %+v", j)
	}
}

// At most MaxPauseOverrides queued or running jobs carry the override: the fourth one is queued
// without it, a key that an active job holds inserts nothing, and a finished job frees its place.
func TestEnqueueOverridingPauseCap(t *testing.T) {
	ctx := context.Background()
	q, _ := newTestQueue(t)
	var got []PauseOverride
	for i := range MaxPauseOverrides + 1 {
		got = append(got, mustOverride(t, q, NewJob{Type: "a", Priority: 9, IdempotencyKey: fmt.Sprintf("k%d", i)}))
	}
	for i, r := range got {
		if want := i < MaxPauseOverrides; !r.Inserted || r.OverridesPause != want {
			t.Errorf("enqueue %d = %+v, want inserted, override %v", i+1, r, want)
		}
		if j, _ := q.Get(ctx, r.ID); j.OverridesPause != r.OverridesPause || j.Priority != 9 {
			t.Errorf("job %d = %+v, want the override %v at priority 9", r.ID, j, r.OverridesPause)
		}
	}
	if again := mustOverride(t, q, NewJob{Type: "a", Priority: 9, IdempotencyKey: "k0"}); again.Inserted || again.ID != got[0].ID || !again.OverridesPause {
		t.Errorf("same key again = %+v, want the active job %d unchanged", again, got[0].ID)
	}
	j := mustClaim(t, q, "w1")
	if err := q.Complete(ctx, "w1", j.ID); err != nil {
		t.Fatal(err)
	}
	if r := mustOverride(t, q, NewJob{Type: "a", IdempotencyKey: "k9"}); !r.OverridesPause {
		t.Errorf("after a job with the override finished: %+v, want the override again", r)
	}
	if _, err := q.EnqueueOverridingPause(ctx, NewJob{Type: "a", Priority: 10}); err == nil {
		t.Error("priority 10 accepted")
	}
}

// A job that holds the key can end between the skipped insert and the lookup of its ID: the insert
// is repeated once instead of failing with sql.ErrNoRows.
func TestEnqueueOverridingPauseRepeatsWhenHolderEnds(t *testing.T) {
	ctx := context.Background()
	q, _ := newTestQueue(t)
	holder := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "keyframes:1"})
	j := mustClaim(t, q, "w1")
	calls := 0
	q.afterSkippedInsert = func() {
		calls++
		if err := q.Complete(ctx, "w1", j.ID); err != nil { // the holder ends right after the skipped insert
			t.Error(err)
		}
	}
	r, err := q.EnqueueOverridingPause(ctx, NewJob{Type: "a", Priority: 9, IdempotencyKey: "keyframes:1"})
	if err != nil {
		t.Fatalf("EnqueueOverridingPause = %v, want the insert repeated", err)
	}
	if calls != 1 || !r.Inserted || r.ID == holder || !r.OverridesPause {
		t.Errorf("result = %+v after %d skipped inserts, want a new job (not %d) with the override", r, calls, holder)
	}
}

// Four concurrent playback starts cannot pass the cap: the check is part of the insert.
func TestEnqueueOverridingPauseCapConcurrent(t *testing.T) {
	q, _ := newTestQueue(t)
	const starts = MaxPauseOverrides + 1
	var wg sync.WaitGroup
	gate := make(chan struct{})
	results := make([]PauseOverride, starts)
	errs := make([]error, starts)
	for i := range starts {
		wg.Go(func() {
			<-gate
			results[i], errs[i] = q.EnqueueOverridingPause(context.Background(), NewJob{Type: "a", Priority: 9, IdempotencyKey: fmt.Sprintf("c%d", i)})
		})
	}
	close(gate)
	wg.Wait()
	with := 0
	for i, r := range results {
		if errs[i] != nil || !r.Inserted {
			t.Fatalf("start %d = %+v %v", i, r, errs[i])
		}
		if r.OverridesPause {
			with++
		}
	}
	var stored int
	if err := q.db.QueryRowContext(context.Background(), `SELECT count(*) FROM jobs WHERE override_pause = 1`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if with != MaxPauseOverrides || stored != MaxPauseOverrides {
		t.Errorf("jobs with the override = %d (stored %d), want %d of %d", with, stored, MaxPauseOverrides, starts)
	}
}

// Raising a waiting job for a playback: the priority rises and never falls, a job in backoff runs
// now, and the override follows the same cap.
func TestRaiseQueuedJobForPlayback(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	id := mustEnqueue(t, q, NewJob{Type: "a", Priority: 1, IdempotencyKey: "keyframes:1"})
	j := mustClaim(t, q, "w1")
	if err := q.Fail(ctx, "w1", j, errTest); err != nil {
		t.Fatal(err)
	}
	c.Advance(time.Second) // in backoff until 10 s after the failure
	r, err := q.RaiseQueuedJobForPlayback(ctx, id, 9)
	if err != nil || !r.Raised || r.ID != id || !r.OverridesPause {
		t.Fatalf("raise = %+v %v, want raised with the override", r, err)
	}
	got, _ := q.Get(ctx, id)
	if got.Priority != 9 || !got.RunAfter.Equal(c.Now()) || !got.OverridesPause || got.Attempts != 1 {
		t.Errorf("raised job = %+v, want priority 9, run_after now, override, the attempt kept", got)
	}
	if r, err := q.RaiseQueuedJobForPlayback(ctx, id, 2); err != nil || !r.Raised || !r.OverridesPause {
		t.Errorf("second raise = %+v %v", r, err)
	}
	if got, _ := q.Get(ctx, id); got.Priority != 9 {
		t.Errorf("a raise to 2 lowered the priority to %d", got.Priority)
	}

	// The cap: two more jobs with the override fill it, a fourth waiting job is raised without.
	mustOverride(t, q, NewJob{Type: "a", IdempotencyKey: "keyframes:2"})
	mustOverride(t, q, NewJob{Type: "a", IdempotencyKey: "keyframes:3"})
	fourth := mustEnqueue(t, q, NewJob{Type: "a", Priority: 1, IdempotencyKey: "keyframes:4"})
	if r, err := q.RaiseQueuedJobForPlayback(ctx, fourth, 9); err != nil || !r.Raised || r.OverridesPause {
		t.Errorf("raise of the fourth = %+v %v, want raised without the override", r, err)
	}
	if got, _ := q.Get(ctx, fourth); got.Priority != 9 || got.OverridesPause {
		t.Errorf("fourth job = %+v", got)
	}
	if _, err := q.RaiseQueuedJobForPlayback(ctx, fourth, -1); err == nil {
		t.Error("priority -1 accepted")
	}
}

// A running job stays as it is: no new priority, no override, no cancel.
func TestRaiseKeepsRunningJob(t *testing.T) {
	ctx := context.Background()
	q, _ := newTestQueue(t)
	id := mustEnqueue(t, q, NewJob{Type: "a", Priority: 1, IdempotencyKey: "keyframes:1"})
	before := mustClaim(t, q, "w1")
	r, err := q.RaiseQueuedJobForPlayback(ctx, id, 9)
	if err != nil || r.Raised || r.OverridesPause || r.ID != id {
		t.Fatalf("raise of a running job = %+v %v, want nothing raised", r, err)
	}
	got, _ := q.Get(ctx, id)
	if got.State != StateRunning || got.Priority != 1 || got.OverridesPause || !got.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("running job after the raise = %+v, want unchanged", got)
	}
	if err := q.Complete(ctx, "w1", id); err != nil {
		t.Fatal(err)
	}
	if r, err := q.RaiseQueuedJobForPlayback(ctx, id, 9); err != nil || r.Raised {
		t.Errorf("raise of a done job = %+v %v", r, err)
	}
}

// The pool index has one worker: Attach with PoolIndex hands it the types of that pool only, and
// the general workers never claim them, so two index jobs never run at once.
func TestPoolIndexRunsOneJobAtATime(t *testing.T) {
	var mu sync.Mutex
	running, most := 0, 0
	read := func(context.Context, *Job) error {
		mu.Lock()
		running++
		most = max(most, running)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
		return nil
	}
	index := Type{Name: "probe.read", Group: GroupLibrary, Pool: PoolIndex, Trigger: TriggerEvent, Priority: 1, Handler: read}
	r, q, _ := newControlEnv(t, index, eventType("general", func(context.Context, *Job) error { return nil }))
	indexWorker := NewWorker(q, "index-1", discard)
	r.Attach(indexWorker, PoolIndex)
	if got := indexWorker.Handlers(); len(got) != 1 || got[0] != "probe.read" {
		t.Fatalf("index worker handles %v", got)
	}
	for i := range 3 {
		w := NewWorker(q, fmt.Sprintf("worker-%d", i+1), discard)
		r.Attach(w, PoolGeneral)
		startWorker(t, w)
	}
	startWorker(t, indexWorker)
	var ids []int64
	for i := range 5 {
		ids = append(ids, mustEnqueue(t, q, NewJob{Type: "probe.read", IdempotencyKey: fmt.Sprintf("read:%d", i)}))
		ids = append(ids, mustOverride(t, q, NewJob{Type: "probe.read", IdempotencyKey: fmt.Sprintf("playback:%d", i)}).ID)
	}
	for _, id := range ids {
		waitForState(t, q, id, StateDone)
	}
	mu.Lock()
	defer mu.Unlock()
	if most != 1 {
		t.Errorf("index jobs at once = %d, want 1", most)
	}
}
