package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The time window of the heavy event types (concept 15, 4.3): outside the window no worker claims
// a job of the type, except one with override_window; the pause comes first. The clock of the
// queue starts at 14:00 in testZone (newControlEnv hands the scheduler's zone to the queue).

func windowType(name string, h Handler) Type {
	t := eventType(name, h)
	t.Windowable = true
	return t
}

var noopHandler = func(context.Context, *Job) error { return nil }

// setLocal sets the clock to hour:minute in testZone, dayOffset days after the start day of the
// test clock (18 October 2026).
func setLocal(c *clock, dayOffset, hour, minute int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = time.Date(2026, 10, 18+dayOffset, hour, minute, 0, 0, testZone)
}

func mustSetWindow(t *testing.T, r *Registry, typ string, w *Window) {
	t.Helper()
	if err := r.SetWindow(context.Background(), typ, w); err != nil {
		t.Fatal(err)
	}
}

// A window from 23:00 to 02:00 holds the jobs of its type outside it and lets them run inside it,
// across midnight and with the end exclusive; a type without a window is claimed at any minute.
func TestWindowClaimAcrossMidnight(t *testing.T) {
	ctx := context.Background()
	r, q, c := newControlEnv(t, windowType("a", noopHandler), eventType("b", noopHandler))
	if got := q.Location(); got != testZone {
		t.Fatalf("queue location = %v, want the scheduler's %v", got, testZone)
	}
	mustSetWindow(t, r, "a", &Window{Start: 23 * 60, End: 2 * 60})
	for _, tc := range []struct {
		day, hour, minute int
		claimed           bool
	}{
		{0, 14, 0, false},
		{0, 22, 59, false},
		{0, 23, 0, true},
		{0, 23, 59, true},
		{1, 0, 0, true},
		{1, 1, 59, true},
		{1, 2, 0, false}, // the end is exclusive
		{1, 12, 0, false},
	} {
		at := fmt.Sprintf("day %d %02d:%02d", tc.day, tc.hour, tc.minute)
		setLocal(c, tc.day, tc.hour, tc.minute)
		a := mustEnqueue(t, q, NewJob{Type: "a", Priority: 9, IdempotencyKey: "a:" + at})
		b := mustEnqueue(t, q, NewJob{Type: "b", Priority: 1, IdempotencyKey: "b:" + at})
		want := b
		if tc.claimed {
			want = a // priority 9 before 1
		}
		if j := mustClaim(t, q, "w1"); j == nil || j.ID != want {
			t.Fatalf("%s: claimed %+v, want job %d (window open: %v)", at, j, want, tc.claimed)
		}
		if tc.claimed {
			if j := mustClaim(t, q, "w1"); j == nil || j.ID != b {
				t.Fatalf("%s: second claim %+v, want the job of the type without a window", at, j)
			}
		} else if j := mustClaim(t, q, "w1"); j != nil {
			t.Fatalf("%s: claimed %+v outside the window", at, j)
		}
		// Finish both, so no lease of this round comes back when the clock jumps.
		for _, id := range []int64{a, b} {
			if j, _ := q.Get(ctx, id); j.State == StateRunning {
				if err := q.Complete(ctx, "w1", id); err != nil {
					t.Fatal(err)
				}
			} else if _, err := q.Cancel(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// The window is read in the scheduler's location: 03:00 to 05:00 in testZone (UTC+2) is 01:00 to
// 03:00 UTC.
func TestWindowUsesSchedulerLocation(t *testing.T) {
	r, q, c := newControlEnv(t, windowType("a", noopHandler))
	mustSetWindow(t, r, "a", &Window{Start: 3 * 60, End: 5 * 60})
	id := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "a"})
	c.mu.Lock()
	c.now = time.Date(2026, 10, 19, 4, 30, 0, 0, time.UTC) // 06:30 local
	c.mu.Unlock()
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("claimed %+v at 04:30 UTC = 06:30 local", j)
	}
	c.mu.Lock()
	c.now = time.Date(2026, 10, 19, 2, 30, 0, 0, time.UTC) // 04:30 local
	c.mu.Unlock()
	if j := mustClaim(t, q, "w1"); j == nil || j.ID != id {
		t.Fatalf("claimed %+v at 02:30 UTC = 04:30 local, want job %d", j, id)
	}
}

// The pause comes first: a paused type runs inside its window only with override_pause, and
// override_window does not lift the pause.
func TestWindowPauseComesFirst(t *testing.T) {
	ctx := context.Background()
	r, q, c := newControlEnv(t, windowType("a", noopHandler))
	mustSetWindow(t, r, "a", &Window{Start: 22 * 60, End: 6 * 60})
	plain := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "plain"})
	windowOnly, err := q.EnqueueOverridingWindow(ctx, NewJob{Type: "a", IdempotencyKey: "window"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Pause(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	setLocal(c, 0, 23, 0) // inside the window
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("claimed %+v inside the window of a paused type", j)
	}
	both := mustOverride(t, q, NewJob{Type: "a", IdempotencyKey: "playback"})
	if j := mustClaim(t, q, "w1"); j == nil || j.ID != both.ID || !j.OverridesPause || !j.OverridesWindow {
		t.Fatalf("claimed %+v, want the playback job %d with both overrides", j, both.ID)
	}
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("claimed %+v: override_window lifted the pause", j)
	}
	if err := r.Resume(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if j := mustClaim(t, q, "w1"); j == nil || (j.ID != plain && j.ID != windowOnly) {
			t.Fatalf("claimed %+v after resuming inside the window", j)
		}
	}
}

// A job with override_window starts outside the window; a job with override_pause alone (a playback
// request queued before migration 00034) stays held by the window.
func TestWindowOverrides(t *testing.T) {
	ctx := context.Background()
	r, q, _ := newControlEnv(t, windowType("a", noopHandler))
	mustSetWindow(t, r, "a", &Window{Start: 1 * 60, End: 5 * 60}) // closed at 14:00
	pauseOnly := mustOverride(t, q, NewJob{Type: "a", Priority: 9, IdempotencyKey: "old-playback"})
	if _, err := q.db.ExecContext(ctx, `UPDATE jobs SET override_window = 0 WHERE id = ?`, pauseOnly.ID); err != nil {
		t.Fatal(err)
	}
	mustEnqueue(t, q, NewJob{Type: "a", Priority: 9, IdempotencyKey: "plain"})
	windowed, err := q.EnqueueOverridingWindow(ctx, NewJob{Type: "a", Priority: 1, IdempotencyKey: "after-playback"})
	if err != nil {
		t.Fatal(err)
	}
	j := mustClaim(t, q, "w1")
	if j == nil || j.ID != windowed || !j.OverridesWindow || j.OverridesPause {
		t.Fatalf("claimed %+v, want only the job %d with override_window", j, windowed)
	}
	if j := mustClaim(t, q, "w1"); j != nil {
		t.Fatalf("claimed %+v outside the window: override_pause alone lifted it", j)
	}
	// A playback today sets both overrides: it starts outside the window.
	playback := mustOverride(t, q, NewJob{Type: "a", IdempotencyKey: "playback"})
	if j := mustClaim(t, q, "w1"); j == nil || j.ID != playback.ID {
		t.Fatalf("claimed %+v, want the playback job %d", j, playback.ID)
	}
}

// override_window does not count against MaxPauseOverrides: three jobs with it leave all three
// places of the pause override free, and the cap itself is unchanged.
func TestWindowOverrideKeepsPauseCap(t *testing.T) {
	ctx := context.Background()
	q, _ := newTestQueue(t)
	if MaxPauseOverrides != 3 {
		t.Fatalf("MaxPauseOverrides = %d, want 3 (O-7)", MaxPauseOverrides)
	}
	for i := range 3 {
		id, err := q.EnqueueOverridingWindow(ctx, NewJob{Type: "a", IdempotencyKey: fmt.Sprintf("w%d", i)})
		if err != nil {
			t.Fatal(err)
		}
		if j, _ := q.Get(ctx, id); !j.OverridesWindow || j.OverridesPause {
			t.Fatalf("job %d = %+v, want override_window only", id, j)
		}
	}
	for i := range MaxPauseOverrides + 1 {
		got := mustOverride(t, q, NewJob{Type: "a", IdempotencyKey: fmt.Sprintf("p%d", i)})
		if want := i < MaxPauseOverrides; got.OverridesPause != want {
			t.Errorf("pause override %d = %+v, want override_pause %v", i+1, got, want)
		}
		if j, _ := q.Get(ctx, got.ID); !j.OverridesWindow {
			t.Errorf("playback job %d without override_window", got.ID)
		}
	}
}

// EnqueueOverridingWindow on a key that a waiting job holds raises that job: priority up (never
// down), override_window set, its start and override_pause unchanged; a running job stays as it is.
// RaiseQueuedJobForPlayback sets override_window as well.
func TestEnqueueOverridingWindowRaisesWaitingJob(t *testing.T) {
	ctx := context.Background()
	q, c := newTestQueue(t)
	waiting := mustEnqueue(t, q, NewJob{Type: "a", Priority: 1, IdempotencyKey: "chapters:1", Delay: time.Hour})
	before, _ := q.Get(ctx, waiting)
	c.Advance(time.Minute)
	id, err := q.EnqueueOverridingWindow(ctx, NewJob{Type: "a", Priority: 5, IdempotencyKey: "chapters:1", Delay: 30 * time.Minute})
	if err != nil || id != waiting {
		t.Fatalf("enqueue = %d %v, want the waiting job %d", id, err, waiting)
	}
	got, _ := q.Get(ctx, waiting)
	if got.Priority != 5 || !got.OverridesWindow || got.OverridesPause || !got.RunAfter.Equal(before.RunAfter) {
		t.Errorf("raised job = %+v, want priority 5, override_window, no override_pause, start unchanged", got)
	}
	if _, err := q.EnqueueOverridingWindow(ctx, NewJob{Type: "a", Priority: 2, IdempotencyKey: "chapters:1"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.Get(ctx, waiting); got.Priority != 5 {
		t.Errorf("a raise to 2 lowered the priority to %d", got.Priority)
	}

	runningID := mustEnqueue(t, q, NewJob{Type: "b", Priority: 1, IdempotencyKey: "chapters:2"})
	running, err := q.Claim(ctx, "w1", []string{"b"})
	if err != nil || running == nil || running.ID != runningID {
		t.Fatalf("claim = %+v %v", running, err)
	}
	if id, err := q.EnqueueOverridingWindow(ctx, NewJob{Type: "b", Priority: 5, IdempotencyKey: "chapters:2"}); err != nil || id != runningID {
		t.Fatalf("enqueue on a running key = %d %v", id, err)
	}
	if got, _ := q.Get(ctx, runningID); got.State != StateRunning || got.Priority != 1 || got.OverridesWindow {
		t.Errorf("running job = %+v, want unchanged", got)
	}

	keyframes := mustEnqueue(t, q, NewJob{Type: "a", Priority: 1, IdempotencyKey: "keyframes:1"})
	if r, err := q.RaiseQueuedJobForPlayback(ctx, keyframes, 9); err != nil || !r.Raised {
		t.Fatalf("raise = %+v %v", r, err)
	}
	if got, _ := q.Get(ctx, keyframes); !got.OverridesWindow || !got.OverridesPause {
		t.Errorf("job raised for a playback = %+v, want both overrides", got)
	}
}

// Idle workers poll the queue (Worker.poll), so a window that opens, and a window that is removed,
// let a waiting job run without any other enqueue to wake them.
func TestWindowOpeningWakesIdleWorker(t *testing.T) {
	r, q, c := newControlEnv(t, windowType("a", noopHandler))
	mustSetWindow(t, r, "a", &Window{Start: 22 * 60, End: 6 * 60})
	w := NewWorker(q, "w1", discard)
	r.Attach(w)
	first := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "first"})
	startWorker(t, w)
	time.Sleep(50 * time.Millisecond) // ten polls outside the window
	if j, _ := q.Get(context.Background(), first); j.State != StateQueued {
		t.Fatalf("job outside the window = %+v, want queued", j)
	}
	setLocal(c, 0, 22, 0)
	waitForState(t, q, first, StateDone)

	setLocal(c, 1, 12, 0) // closed again
	second := mustEnqueue(t, q, NewJob{Type: "a", IdempotencyKey: "second"})
	time.Sleep(50 * time.Millisecond)
	if j, _ := q.Get(context.Background(), second); j.State != StateQueued {
		t.Fatalf("job outside the window = %+v, want queued", j)
	}
	mustSetWindow(t, r, "a", nil)
	waitForState(t, q, second, StateDone)
}

// SetWindow: only a registered windowable type, a valid window, nil removes it.
func TestSetWindow(t *testing.T) {
	ctx := context.Background()
	r, _, _ := newControlEnv(t, windowType("a", noopHandler), eventType("b", noopHandler))
	for _, typ := range []string{"b", "nobody.home"} {
		if err := r.SetWindow(ctx, typ, &Window{Start: 0, End: 60}); !errors.Is(err, ErrNotWindowable) {
			t.Errorf("SetWindow(%s) = %v, want ErrNotWindowable", typ, err)
		}
	}
	for _, w := range []Window{{Start: 60, End: 60}, {Start: -1, End: 60}, {Start: 0, End: 1440}} {
		var verr *model.ValidationError
		if err := r.SetWindow(ctx, "a", &w); !errors.As(err, &verr) || verr.Fields[0].Field != "window" {
			t.Errorf("SetWindow(%+v) = %v, want a validation error on window", w, err)
		}
	}
	mustSetWindow(t, r, "a", &Window{Start: 23 * 60, End: 60})
	if w, err := r.Window(ctx, "a"); err != nil || w == nil || *w != (Window{Start: 23 * 60, End: 60}) {
		t.Errorf("window = %+v %v", w, err)
	}
	mustSetWindow(t, r, "a", nil)
	if w, err := r.Window(ctx, "a"); err != nil || w != nil {
		t.Errorf("window after removing it = %+v %v", w, err)
	}
	if _, err := r.Window(ctx, "nobody.home"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("window of an unknown type: %v", err)
	}
}

// Only an event type may carry Windowable; a schedule or manual type has its window in the schedule.
func TestValidateTypeWindowable(t *testing.T) {
	run := func(context.Context, *Queue, int) ([]int64, error) { return nil, nil }
	event := windowType("e", noopHandler)
	schedule := Type{Name: "s", Group: GroupLibrary, Pool: PoolGeneral, Trigger: TriggerSchedule, Priority: 1,
		Handler: noopHandler, Run: run, Every: time.Hour, Windowable: true}
	manual := Type{Name: "m", Group: GroupLibrary, Pool: PoolGeneral, Trigger: TriggerManual, Priority: 1,
		Handler: noopHandler, Run: run, Windowable: true}
	if err := validateType(event); err != nil {
		t.Errorf("windowable event type: %v", err)
	}
	for _, ty := range []Type{schedule, manual} {
		if err := validateType(ty); err == nil {
			t.Errorf("windowable %s type accepted", ty.Trigger)
		}
		ty.Windowable = false
		if err := validateType(ty); err != nil {
			t.Errorf("%s type without Windowable: %v", ty.Trigger, err)
		}
	}
}

// OpensAt follows the alignment of the schedules: inside the window it is open, outside it names
// the next start, across midnight too.
func TestWindowOpensAt(t *testing.T) {
	day := func(d, h, m int) time.Time { return time.Date(2026, 10, 18+d, h, m, 0, 0, testZone) }
	night := Window{Start: 23 * 60, End: 2 * 60}
	for _, tc := range []struct {
		now    time.Time
		at     time.Time
		closed bool
	}{
		{day(0, 14, 0), day(0, 23, 0), true},
		{day(0, 23, 30), day(0, 23, 30), false},
		{day(1, 1, 59), day(1, 1, 59), false},
		{day(1, 2, 0), day(1, 23, 0), true},
	} {
		at, closed := night.OpensAt(tc.now, testZone)
		if closed != tc.closed || !at.Equal(tc.at) {
			t.Errorf("OpensAt(%v) = %v %v, want %v %v", tc.now, at, closed, tc.at, tc.closed)
		}
	}
}
