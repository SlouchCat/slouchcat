package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The tests of the scheduler run on the injected clock of the queue (newTestQueue, 18 October
// 2026 12:00 UTC) and a fixed zone two hours east of UTC, so "local" minutes of the day differ
// from UTC and a window test would notice a UTC calculation.

var testZone = time.FixedZone("test+2", 2*3600)

// counter records the runs of a fake schedule type and enqueues one job per run under a fixed
// key, raising a waiting one, like the Run of every real type does.
type counter struct {
	name string
	runs atomic.Int32
	fail atomic.Bool
}

func (c *counter) run(ctx context.Context, q *Queue, prio int) ([]int64, error) {
	if c.fail.Load() {
		return nil, errors.New("boom")
	}
	c.runs.Add(1)
	id, err := q.Enqueue(ctx, NewJob{Type: c.name, Priority: prio, IdempotencyKey: c.name})
	if err != nil {
		return nil, err
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, err
	}
	return []int64{id}, nil
}

func scheduled(c *counter, every time.Duration, delay *time.Duration) Type {
	return Type{
		Name: c.name, Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerSchedule, Priority: 2,
		Handler: func(context.Context, *Job) error { return nil }, Run: c.run, Every: every, StartDelay: delay,
	}
}

func dur(d time.Duration) *time.Duration { return &d }

type schedEnv struct {
	t     *testing.T
	ctx   context.Context
	q     *Queue
	clock *clock
	reg   *Registry
	s     *Scheduler
}

func newSchedEnv(t *testing.T, types ...Type) *schedEnv {
	t.Helper()
	q, c := newTestQueue(t)
	reg := NewRegistry()
	for _, ty := range types {
		reg.Register(ty)
	}
	s := NewScheduler(q, reg, discard, WithLocation(testZone))
	e := &schedEnv{t: t, ctx: context.Background(), q: q, clock: c, reg: reg, s: s}
	if err := s.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *schedEnv) tick() {
	e.t.Helper()
	if err := e.s.Tick(e.ctx); err != nil {
		e.t.Fatal(err)
	}
}

func (e *schedEnv) at(d time.Duration) {
	e.t.Helper()
	e.clock.Advance(d)
	e.tick()
}

func (e *schedEnv) schedule(typ string) Schedule {
	e.t.Helper()
	s, err := e.s.Schedule(e.ctx, typ)
	if err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *schedEnv) update(typ string, u ScheduleUpdate) Schedule {
	e.t.Helper()
	s, err := e.s.UpdateSchedule(e.ctx, typ, u)
	if err != nil {
		e.t.Fatal(err)
	}
	return s
}

// local returns the clock's day at hour:minute in the test zone.
func (e *schedEnv) local(dayOffset, hour, minute int) time.Time {
	y, m, d := e.clock.Now().In(testZone).Date()
	return time.Date(y, m, d+dayOffset, hour, minute, 0, 0, testZone)
}

// Start fires a type with StartDelay after that delay (0 at once) and leaves the next run of a
// type without one as it stands, so a restart does not shift its rhythm.
func TestSchedulerStartDelay(t *testing.T) {
	atOnce, later, none := &counter{name: "a.now"}, &counter{name: "b.later"}, &counter{name: "c.none"}
	e := newSchedEnv(t, scheduled(atOnce, time.Hour, dur(0)), scheduled(later, time.Hour, dur(90*time.Second)), scheduled(none, time.Hour, nil))
	e.tick()
	if atOnce.runs.Load() != 1 || later.runs.Load() != 0 || none.runs.Load() != 1 {
		t.Fatalf("after the first tick: runs = %d, %d, %d; want 1, 0, 1 (a new row without history is due now)", atOnce.runs.Load(), later.runs.Load(), none.runs.Load())
	}
	e.at(89 * time.Second)
	if later.runs.Load() != 0 {
		t.Fatal("fired before its start delay")
	}
	e.at(time.Second)
	if later.runs.Load() != 1 {
		t.Fatal("did not fire after its start delay")
	}
	// Restart five minutes later: the rows exist, their next runs lie in the future.
	e.clock.Advance(5 * time.Minute)
	if err := e.s.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.tick()
	if atOnce.runs.Load() != 2 {
		t.Errorf("start delay 0 did not fire again at the restart: runs = %d", atOnce.runs.Load())
	}
	if none.runs.Load() != 1 {
		t.Errorf("a type without start delay fired at the restart: runs = %d", none.runs.Load())
	}
	if later.runs.Load() != 1 {
		t.Errorf("start delay 90 s fired at once at the restart: runs = %d", later.runs.Load())
	}
	e.at(90 * time.Second)
	if later.runs.Load() != 2 {
		t.Errorf("start delay 90 s did not fire after the restart: runs = %d", later.runs.Load())
	}
	if s := e.schedule("c.none"); !s.NextRunAt.Equal(e.clock.Now().Add(-(5*time.Minute + 180*time.Second)).Add(time.Hour)) {
		t.Errorf("next run of the untouched type = %v, want one hour after its first firing", s.NextRunAt)
	}
}

// After a firing the next run is one interval later; a run is not repeated before.
func TestSchedulerFiresEveryInterval(t *testing.T) {
	c := &counter{name: "a.every"}
	e := newSchedEnv(t, scheduled(c, 6*time.Hour, dur(0)))
	e.tick()
	e.at(6*time.Hour - time.Second)
	if c.runs.Load() != 1 {
		t.Fatalf("runs = %d before the interval", c.runs.Load())
	}
	e.at(time.Second)
	if c.runs.Load() != 2 {
		t.Fatalf("runs = %d after the interval", c.runs.Load())
	}
	s := e.schedule("a.every")
	if !s.LastFiredAt.Equal(e.clock.Now()) || !s.NextRunAt.Equal(e.clock.Now().Add(6*time.Hour)) {
		t.Errorf("schedule after the firing = %+v", s)
	}
	// Missed runs are not made up for: a day later there is one run, not four.
	e.at(24 * time.Hour)
	if c.runs.Load() != 3 {
		t.Errorf("runs after a day without ticks = %d, want 3 (one, not the missed ones)", c.runs.Load())
	}
}

// A disabled schedule does not fire; enabling it again brings the next run forward to now
// when the last firing is longer ago than the interval.
func TestSchedulerDisabledDoesNotFire(t *testing.T) {
	c := &counter{name: "a.switch"}
	e := newSchedEnv(t, scheduled(c, time.Hour, dur(0)))
	e.tick()
	off := false
	e.update("a.switch", ScheduleUpdate{Enabled: &off})
	e.at(3 * time.Hour)
	if c.runs.Load() != 1 {
		t.Fatalf("a disabled schedule fired: runs = %d", c.runs.Load())
	}
	on := true
	if s := e.update("a.switch", ScheduleUpdate{Enabled: &on}); !s.NextRunAt.Equal(e.clock.Now()) {
		t.Errorf("next run after enabling = %v, want now (the last firing is three hours ago)", s.NextRunAt)
	}
	e.tick()
	if c.runs.Load() != 2 {
		t.Errorf("runs after enabling = %d", c.runs.Load())
	}
}

// A window that spans midnight (22:00 to 02:00 local) holds the runs inside it; the next run
// after the window moves to the next start of the window.
func TestSchedulerWindowAcrossMidnight(t *testing.T) {
	c := &counter{name: "a.night"}
	e := newSchedEnv(t, scheduled(c, time.Hour, nil))
	e.tick() // 14:00 local
	s := e.update("a.night", ScheduleUpdate{Window: &Window{Start: 22 * 60, End: 2 * 60}})
	if want := e.local(0, 22, 0); !s.NextRunAt.Equal(want) {
		t.Fatalf("next run with the window = %v, want %v", s.NextRunAt.In(testZone), want)
	}
	// Tick just before and at the start of the window.
	e.clock.Advance(e.local(0, 21, 59).Sub(e.clock.Now()))
	e.tick()
	if c.runs.Load() != 1 {
		t.Fatalf("fired before the window: runs = %d", c.runs.Load())
	}
	fired := []time.Time{}
	for i := range 5 {
		e.at(time.Minute)
		if i == 0 && c.runs.Load() != 2 {
			t.Fatalf("did not fire at the start of the window")
		}
		fired = append(fired, e.schedule("a.night").LastFiredAt)
		e.clock.Advance(59 * time.Minute)
	}
	// 22:00, 23:00, 00:00 and 01:00 fire inside the window; 02:00 is outside (exclusive end).
	if c.runs.Load() != 5 {
		t.Errorf("runs across midnight = %d, want 5 (22, 23, 0, 1 and the one at 14:00)", c.runs.Load())
	}
	if last := fired[len(fired)-1]; !last.Equal(e.local(0, 1, 0)) {
		t.Errorf("last firing = %v, want 01:00 local", last.In(testZone))
	}
	if next := e.schedule("a.night").NextRunAt; !next.Equal(e.local(0, 22, 0)) {
		t.Errorf("next run after the window = %v, want 22:00 local of the same day", next.In(testZone))
	}
}

// Changing the window moves the next run: into the window, and back to the plain rhythm when
// the window is removed.
func TestSchedulerWindowChangeMovesNextRun(t *testing.T) {
	c := &counter{name: "a.win"}
	e := newSchedEnv(t, scheduled(c, 6*time.Hour, dur(0)))
	e.tick() // fired at 14:00 local, next 20:00 local
	plain := e.schedule("a.win").NextRunAt
	s := e.update("a.win", ScheduleUpdate{Window: &Window{Start: 3 * 60, End: 5 * 60}})
	if want := e.local(1, 3, 0); !s.NextRunAt.Equal(want) {
		t.Errorf("next run with the window 03:00-05:00 = %v, want %v", s.NextRunAt.In(testZone), want)
	}
	s = e.update("a.win", ScheduleUpdate{ClearWindow: true})
	if !s.NextRunAt.Equal(plain) || s.Window != nil {
		t.Errorf("next run without the window = %v, want %v (last firing plus interval)", s.NextRunAt.In(testZone), plain.In(testZone))
	}
	// A shorter interval brings the next run forward, from the last firing.
	s = e.update("a.win", ScheduleUpdate{Every: dur(2 * time.Hour)})
	if want := e.local(0, 16, 0); !s.NextRunAt.Equal(want) {
		t.Errorf("next run with a shorter interval = %v, want %v", s.NextRunAt.In(testZone), want)
	}
}

// An interval shorter than the window fires inside the window at the interval (1 h in 02:00 to
// 05:00: three times), a longer one once per window.
func TestSchedulerShortIntervalFiresRepeatedlyInWindow(t *testing.T) {
	short, long := &counter{name: "a.short"}, &counter{name: "b.long"}
	e := newSchedEnv(t, scheduled(short, time.Hour, nil), scheduled(long, 6*time.Hour, nil))
	e.tick()
	w := &Window{Start: 2 * 60, End: 5 * 60}
	e.update("a.short", ScheduleUpdate{Window: w})
	e.update("b.long", ScheduleUpdate{Window: w})
	short.runs.Store(0)
	long.runs.Store(0)
	// Tick every 10 minutes for two days.
	for range 2 * 24 * 6 {
		e.at(10 * time.Minute)
	}
	if short.runs.Load() != 6 {
		t.Errorf("runs of the hourly type in two windows = %d, want 6 (02:00, 03:00, 04:00 per night)", short.runs.Load())
	}
	if long.runs.Load() != 2 {
		t.Errorf("runs of the 6-hour type in two windows = %d, want 2 (once per window)", long.runs.Load())
	}
}

// A paused type is not run, but its next run moves on: the pause skips runs, it does not shift
// the rhythm.
func TestSchedulerPausedTypeSkipsRun(t *testing.T) {
	c := &counter{name: "a.paused"}
	e := newSchedEnv(t, scheduled(c, time.Hour, dur(0)))
	if _, err := e.q.db.ExecContext(e.ctx, `UPDATE job_types SET paused_at = ? WHERE type = 'a.paused'`, e.clock.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	e.tick()
	if c.runs.Load() != 0 {
		t.Fatalf("a paused type ran")
	}
	s := e.schedule("a.paused")
	if !s.LastFiredAt.Equal(e.clock.Now()) || !s.NextRunAt.Equal(e.clock.Now().Add(time.Hour)) {
		t.Errorf("schedule of the paused type = %+v, want fired now and next in an hour", s)
	}
	if _, err := e.q.db.ExecContext(e.ctx, `UPDATE job_types SET paused_at = NULL WHERE type = 'a.paused'`); err != nil {
		t.Fatal(err)
	}
	e.at(time.Hour)
	if c.runs.Load() != 1 {
		t.Errorf("runs after resuming = %d", c.runs.Load())
	}
}

// A row whose type is not registered (after a downgrade) is skipped and stays as it is.
func TestSchedulerSkipsUnregisteredRow(t *testing.T) {
	c := &counter{name: "a.known"}
	e := newSchedEnv(t, scheduled(c, time.Hour, dur(0)))
	past := e.clock.Now().Add(-time.Hour).UnixMilli()
	if _, err := e.q.db.ExecContext(e.ctx, `INSERT INTO job_types (type, updated_at) VALUES ('ghost.job', ?)`, past); err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.db.ExecContext(e.ctx, `INSERT INTO job_schedules (type, every_ms, enabled, next_run_at, updated_at) VALUES ('ghost.job', 3600000, 1, ?, ?)`, past, past); err != nil {
		t.Fatal(err)
	}
	e.tick()
	if c.runs.Load() != 1 {
		t.Errorf("the registered type did not run beside the unknown row")
	}
	ghost := e.schedule("ghost.job")
	if !ghost.NextRunAt.Equal(time.UnixMilli(past)) || !ghost.LastFiredAt.IsZero() {
		t.Errorf("unknown row changed: %+v", ghost)
	}
	var n int
	if err := e.q.db.QueryRowContext(e.ctx, `SELECT count(*) FROM jobs WHERE type = 'ghost.job'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("jobs of the unknown type = %d %v", n, err)
	}
}

// A run that fails is tried again at the next tick; its next run does not move.
func TestSchedulerRetriesFailedRunAtNextTick(t *testing.T) {
	c := &counter{name: "a.flaky"}
	e := newSchedEnv(t, scheduled(c, time.Hour, dur(0)))
	c.fail.Store(true)
	e.tick()
	if s := e.schedule("a.flaky"); !s.LastFiredAt.IsZero() {
		t.Errorf("a failed run counted as fired: %+v", s)
	}
	c.fail.Store(false)
	e.at(time.Minute)
	if c.runs.Load() != 1 {
		t.Errorf("runs after the retry = %d", c.runs.Load())
	}
}

// A missing row takes interval, switch and last run from the type's seed: the next run is the
// last run plus the interval, at least now; a row that exists keeps its values (the table is
// the source once it has a row).
func TestSchedulerSeedAndExistingRow(t *testing.T) {
	q, c := newTestQueue(t)
	now := c.Now()
	recent, stale := &counter{name: "a.recent"}, &counter{name: "b.stale"}
	off := &counter{name: "c.off"}
	seeded := func(cnt *counter, seed ScheduleSeed) Type {
		ty := scheduled(cnt, 24*time.Hour, nil)
		ty.Seed = func(context.Context) (ScheduleSeed, error) { return seed, nil }
		return ty
	}
	reg := NewRegistry()
	reg.Register(seeded(recent, ScheduleSeed{Every: 6 * time.Hour, Enabled: true, LastRun: now.Add(-4 * time.Hour)}))
	reg.Register(seeded(stale, ScheduleSeed{Every: 6 * time.Hour, Enabled: true, LastRun: now.Add(-10 * time.Hour)}))
	reg.Register(seeded(off, ScheduleSeed{Enabled: false}))
	s := NewScheduler(q, reg, discard, WithLocation(testZone))
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	get := func(typ string) Schedule {
		t.Helper()
		row, err := s.Schedule(ctx, typ)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	if r := get("a.recent"); r.Every != 6*time.Hour || !r.NextRunAt.Equal(now.Add(2*time.Hour)) || !r.Enabled {
		t.Errorf("seeded from a recent run: %+v, want next in two hours", r)
	}
	if r := get("b.stale"); !r.NextRunAt.Equal(now) {
		t.Errorf("seeded from a stale run: next = %v, want now (lower bound)", r.NextRunAt)
	}
	if r := get("c.off"); r.Enabled || r.Every != 24*time.Hour {
		t.Errorf("seeded switched off: %+v", r)
	}
	// The administrator changes the interval; a restart keeps it and the seed is not asked again.
	if _, err := s.UpdateSchedule(ctx, "a.recent", ScheduleUpdate{Every: dur(12 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if r := get("a.recent"); r.Every != 12*time.Hour {
		t.Errorf("interval after a restart = %v, want the administrator's 12 h", r.Every)
	}
	var types int
	if err := q.db.QueryRowContext(ctx, `SELECT count(*) FROM job_types`).Scan(&types); err != nil || types != 3 {
		t.Errorf("job_types rows = %d %v, want 3", types, err)
	}
}

// UpdateSchedule checks the bounds of the interval and the window and knows its types.
func TestSchedulerUpdateScheduleValidates(t *testing.T) {
	c := &counter{name: "a.valid"}
	e := newSchedEnv(t, scheduled(c, time.Hour, nil))
	var verr *model.ValidationError
	if _, err := e.s.UpdateSchedule(e.ctx, "a.valid", ScheduleUpdate{Every: dur(time.Minute)}); !errors.As(err, &verr) || verr.Fields[0].Field != "everySeconds" {
		t.Errorf("one minute: %v", err)
	}
	if _, err := e.s.UpdateSchedule(e.ctx, "a.valid", ScheduleUpdate{Every: dur(8 * 24 * time.Hour)}); !errors.As(err, &verr) {
		t.Errorf("eight days: %v", err)
	}
	for _, w := range []Window{{Start: 60, End: 60}, {Start: -1, End: 60}, {Start: 0, End: 1440}} {
		if _, err := e.s.UpdateSchedule(e.ctx, "a.valid", ScheduleUpdate{Window: &w}); !errors.As(err, &verr) || verr.Fields[0].Field != "window" {
			t.Errorf("window %+v: %v", w, err)
		}
	}
	if _, err := e.s.UpdateSchedule(e.ctx, "nobody.home", ScheduleUpdate{Every: dur(time.Hour)}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown type: %v", err)
	}
	if s := e.update("a.valid", ScheduleUpdate{Every: dur(MinEvery), Window: &Window{Start: 0, End: 1439}}); s.Every != MinEvery || s.Window == nil {
		t.Errorf("valid bounds refused: %+v", s)
	}
}

// The registry refuses inconsistent types and attaches handlers by pool.
func TestRegistryValidatesAndAttachesByPool(t *testing.T) {
	handler := func(context.Context, *Job) error { return nil }
	run := func(context.Context, *Queue, int) ([]int64, error) { return nil, nil }
	bad := []Type{
		{Name: "", Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerEvent, Handler: handler},
		{Name: "x", Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerEvent, Handler: handler, Priority: 10},
		{Name: "x", Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerSchedule, Handler: handler, Every: time.Hour},   // no Run
		{Name: "x", Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerSchedule, Handler: handler, Run: run, Every: 0}, // no interval
		{Name: "x", Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerEvent, Handler: handler, Run: run},              // event with Run
		{Name: "x", Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerManual, Handler: handler},                       // manual without Run
		{Name: "x", Pool: PoolGeneral, Trigger: TriggerEvent, Handler: handler},                                            // no group
	}
	for i, ty := range bad {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("type %d accepted: %+v", i, ty)
				}
			}()
			NewRegistry().Register(ty)
		}()
	}
	r := NewRegistry()
	r.Register(Type{Name: "g.one", Group: GroupLibrary, Pool: PoolGeneral, Trigger: TriggerEvent, Handler: handler})
	r.Register(Type{Name: "d.one", Group: GroupDelivery, Pool: PoolDownload, Trigger: TriggerEvent, Handler: handler})
	r.Register(Type{Name: "p.poll", Group: GroupMetadata, Pool: PoolGeneral, Trigger: TriggerSchedule, Run: run, Every: 5 * time.Minute}) // no handler
	func() {
		defer func() {
			if recover() == nil {
				t.Error("duplicate name accepted")
			}
		}()
		r.Register(Type{Name: "g.one", Group: GroupLibrary, Pool: PoolGeneral, Trigger: TriggerEvent, Handler: handler})
	}()
	q, _ := newTestQueue(t)
	general, download, all := NewWorker(q, "g", discard), NewWorker(q, "d", discard), NewWorker(q, "a", discard)
	r.Attach(general, PoolGeneral)
	r.Attach(download, PoolDownload)
	r.Attach(all)
	if len(general.handlers) != 1 || general.handlers["g.one"] == nil {
		t.Errorf("general worker handles %v", general.handlers)
	}
	if len(download.handlers) != 1 || download.handlers["d.one"] == nil {
		t.Errorf("download worker handles %v", download.handlers)
	}
	if len(all.handlers) != 2 {
		t.Errorf("worker of all pools handles %v, want the two types with a handler", all.handlers)
	}
	if got := r.Types(); len(got) != 3 || got[0].Name != "g.one" || got[2].Name != "p.poll" {
		t.Errorf("Types() = %v", got)
	}
}
