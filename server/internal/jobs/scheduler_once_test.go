package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
	_ "time/tzdata" // Europe/Zurich for the wall time of a one-off date

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The one-off date of concept 14, 6.7 (job_schedules.once_at), on the injected clock and the
// fixed test zone of scheduler_test.go.

// wall returns t as the wall time of a one-off date in the test zone.
func wall(t time.Time) *string {
	s := t.In(testZone).Format(OnceLayout)
	return &s
}

// setOnce writes once_at directly, as a date stored before the server went down.
func (e *schedEnv) setOnce(typ string, at time.Time) {
	e.t.Helper()
	if _, err := e.q.db.ExecContext(e.ctx, `UPDATE job_schedules SET once_at = ? WHERE type = ?`, at.UnixMilli(), typ); err != nil {
		e.t.Fatal(err)
	}
}

// disable switches the interval of typ off, so that only its date counts.
func (e *schedEnv) disable(typ string) {
	e.t.Helper()
	off := false
	e.update(typ, ScheduleUpdate{Enabled: &off})
}

// restart starts a new scheduler on the same database and clock, as after a restart.
func (e *schedEnv) restart() {
	e.t.Helper()
	e.s = NewScheduler(e.q, e.reg, discard, WithLocation(testZone))
	if err := e.s.Start(e.ctx); err != nil {
		e.t.Fatal(err)
	}
}

// The date fires once at its time with the type's normal priority, clears itself and sets
// last_fired_at, and leaves next_run_at where the interval put it.
func TestSchedulerOnceFiresAtItsDate(t *testing.T) {
	c := &counter{name: "a.once"}
	e := newSchedEnv(t, scheduled(c, 24*time.Hour, nil))
	e.tick() // the first interval run; the next one lies a day ahead
	next := e.schedule("a.once").NextRunAt
	e.clock.Advance(time.Hour)
	at := e.clock.Now().Add(2 * time.Hour)
	s := e.update("a.once", ScheduleUpdate{OnceLocal: wall(at)})
	if !s.OnceAt.Equal(at) || s.OnceLocal != *wall(at) || !s.NextRunAt.Equal(next) {
		t.Fatalf("schedule with a date = %+v, want the date %v and the next run %v unchanged", s, at, next)
	}
	e.at(2*time.Hour - time.Minute)
	if c.runs.Load() != 1 {
		t.Fatalf("the date fired early")
	}
	e.at(time.Minute)
	if c.runs.Load() != 2 {
		t.Fatalf("runs at the date = %d, want 2", c.runs.Load())
	}
	if j, err := e.q.Get(e.ctx, 1); err != nil || j.Priority != 2 {
		t.Errorf("the date queued %+v %v, want the type's normal priority 2", j, err)
	}
	s = e.schedule("a.once")
	if !s.OnceAt.IsZero() || s.OnceLocal != "" || !s.LastFiredAt.Equal(e.clock.Now()) || !s.NextRunAt.Equal(next) {
		t.Errorf("schedule after the date = %+v, want no date, fired now, next run %v", s, next)
	}
	e.at(time.Hour)
	if c.runs.Load() != 2 {
		t.Errorf("the date fired twice")
	}
}

// The window and the switch belong to the interval: a date outside the window and on a disabled
// schedule fires all the same.
func TestSchedulerOnceIgnoresWindowAndSwitch(t *testing.T) {
	c := &counter{name: "a.night"}
	e := newSchedEnv(t, scheduled(c, time.Hour, nil))
	off := false
	// Window 02:00 to 04:00 local; the clock is at 14:00 local.
	e.update("a.night", ScheduleUpdate{Enabled: &off, Window: &Window{Start: 120, End: 240}})
	at := e.local(0, 15, 30)
	e.update("a.night", ScheduleUpdate{OnceLocal: wall(at)})
	e.at(at.Sub(e.clock.Now()))
	if c.runs.Load() != 1 {
		t.Fatalf("runs = %d, want the date to fire outside the window and while disabled", c.runs.Load())
	}
	if s := e.schedule("a.night"); s.Enabled || !s.OnceAt.IsZero() {
		t.Errorf("schedule after the date = %+v", s)
	}
}

// A paused type skips the date: it lapses (O-4) and does not fire after the pause either.
func TestSchedulerOnceLapsesWhilePaused(t *testing.T) {
	c := &counter{name: "a.paused"}
	e := newSchedEnv(t, scheduled(c, 24*time.Hour, nil))
	e.disable("a.paused")
	e.update("a.paused", ScheduleUpdate{OnceLocal: wall(e.clock.Now().Add(time.Hour))})
	if err := e.reg.Pause(e.ctx, "a.paused"); err != nil {
		t.Fatal(err)
	}
	e.at(time.Hour)
	if c.runs.Load() != 0 {
		t.Fatalf("a paused type ran its date")
	}
	if s := e.schedule("a.paused"); !s.OnceAt.IsZero() || !s.LastFiredAt.IsZero() {
		t.Errorf("schedule after the skipped date = %+v, want no date and no firing", s)
	}
	if err := e.reg.Resume(e.ctx, "a.paused"); err != nil {
		t.Fatal(err)
	}
	e.at(time.Minute)
	if c.runs.Load() != 0 {
		t.Errorf("the lapsed date fired after the pause")
	}
}

// A date missed while the server was down fires once at the start when it is at most a day old
// (O-5), after the type's StartDelay; an older one lapses.
func TestSchedulerOnceMissedAtStart(t *testing.T) {
	recent, delayed, old := &counter{name: "a.recent"}, &counter{name: "b.delayed"}, &counter{name: "c.old"}
	e := newSchedEnv(t, scheduled(recent, 24*time.Hour, nil), scheduled(delayed, 24*time.Hour, dur(time.Minute)),
		scheduled(old, 24*time.Hour, nil))
	for _, typ := range []string{"a.recent", "b.delayed", "c.old"} {
		e.disable(typ)
	}
	e.clock.Advance(48 * time.Hour)
	// The server was down: a.recent missed its date by exactly a day, b.delayed by an hour, c.old
	// by a day and a minute.
	e.setOnce("a.recent", e.clock.Now().Add(-OnceCatchUp))
	e.setOnce("b.delayed", e.clock.Now().Add(-time.Hour))
	e.setOnce("c.old", e.clock.Now().Add(-OnceCatchUp-time.Minute))
	e.restart()
	if s := e.schedule("c.old"); !s.OnceAt.IsZero() {
		t.Errorf("a date missed by more than a day stays: %+v", s)
	}
	if s := e.schedule("b.delayed"); !s.OnceAt.Equal(e.clock.Now().Add(time.Minute)) {
		t.Errorf("date of the type with a start delay = %v, want now + 1 min", s.OnceAt)
	}
	e.tick()
	if recent.runs.Load() != 1 || delayed.runs.Load() != 0 || old.runs.Load() != 0 {
		t.Fatalf("runs at the start = %d, %d, %d; want the recent date only", recent.runs.Load(), delayed.runs.Load(), old.runs.Load())
	}
	e.at(time.Minute)
	if delayed.runs.Load() != 1 || recent.runs.Load() != 1 || old.runs.Load() != 0 {
		t.Errorf("runs after the start delay = %d, %d, %d; want the delayed date once more", recent.runs.Load(), delayed.runs.Load(), old.runs.Load())
	}
}

// Date and interval in the same tick run the type once; both move on.
func TestSchedulerOnceAndIntervalInOneTick(t *testing.T) {
	c := &counter{name: "a.both"}
	e := newSchedEnv(t, scheduled(c, time.Hour, dur(0)))
	e.tick()
	e.update("a.both", ScheduleUpdate{OnceLocal: wall(e.clock.Now().Add(time.Hour))})
	e.at(time.Hour)
	if c.runs.Load() != 2 {
		t.Fatalf("runs = %d, want 2 (start, then one for date and interval together)", c.runs.Load())
	}
	if s := e.schedule("a.both"); !s.OnceAt.IsZero() || !s.NextRunAt.Equal(e.clock.Now().Add(time.Hour)) {
		t.Errorf("schedule after the shared tick = %+v", s)
	}
}

// A date the administrator sets again while the scheduler fires the one it read stays.
func TestSchedulerOnceSetAgainWhileFiringStays(t *testing.T) {
	c := &counter{name: "a.race"}
	e := newSchedEnv(t)
	again := e.clock.Now().Add(26 * time.Hour)
	ty := scheduled(c, 24*time.Hour, nil)
	ty.Run = func(ctx context.Context, q *Queue, prio int) ([]int64, error) {
		e.setOnce("a.race", again) // the administrator's update between reading and firing
		return c.run(ctx, q, prio)
	}
	e.reg.Register(ty)
	e.restart()
	e.disable("a.race")
	e.update("a.race", ScheduleUpdate{OnceLocal: wall(e.clock.Now().Add(time.Hour))})
	e.at(time.Hour)
	if c.runs.Load() != 1 {
		t.Fatalf("runs = %d", c.runs.Load())
	}
	if s := e.schedule("a.race"); !s.OnceAt.Equal(again) || !s.LastFiredAt.IsZero() {
		t.Errorf("schedule = %+v, want the new date %v kept (and the firing not marked on it)", s, again)
	}
}

// The wall time is read in the scheduler's location: 03:30 in Zurich in winter is 02:30 UTC, in
// summer 01:30 UTC.
func TestSchedulerOnceWallTimeInZurich(t *testing.T) {
	zurich, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	c := &counter{name: "a.zurich"}
	q, _ := newTestQueue(t)
	reg := NewRegistry(q, discard)
	reg.Register(scheduled(c, 24*time.Hour, nil))
	s := NewScheduler(q, reg, discard, WithLocation(zurich))
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for local, want := range map[string]time.Time{
		"2026-12-01T03:30": time.Date(2026, 12, 1, 2, 30, 0, 0, time.UTC),
		"2027-06-01T03:30": time.Date(2027, 6, 1, 1, 30, 0, 0, time.UTC),
	} {
		got, err := s.UpdateSchedule(ctx, "a.zurich", ScheduleUpdate{OnceLocal: &local})
		if err != nil {
			t.Fatal(err)
		}
		var stored int64
		if err := q.db.QueryRowContext(ctx, `SELECT once_at FROM job_schedules WHERE type = 'a.zurich'`).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != want.UnixMilli() || !got.OnceAt.Equal(want) || got.OnceLocal != local {
			t.Errorf("%s stored as %v (schedule %v, %q), want %v", local, time.UnixMilli(stored).UTC(), got.OnceAt, got.OnceLocal, want)
		}
	}
}

// UpdateSchedule refuses a date that is no wall time or lies outside now + 1 min .. now + 366 d,
// clears a date on request, and an update of the date alone leaves the next run as it is.
func TestSchedulerOnceValidatesAndClears(t *testing.T) {
	c := &counter{name: "a.valid"}
	e := newSchedEnv(t, scheduled(c, time.Hour, dur(0)))
	next := e.schedule("a.valid").NextRunAt
	now := e.clock.Now()
	for value, code := range map[string]string{
		"tomorrow":                              "invalid_value",
		"2026-10-18 15:00":                      "invalid_value",
		*wall(now):                              "out_of_range",
		*wall(now.Add(-time.Hour)):              "out_of_range",
		*wall(now.Add(OnceMaxLead + time.Hour)): "out_of_range",
	} {
		var verr *model.ValidationError
		_, err := e.s.UpdateSchedule(e.ctx, "a.valid", ScheduleUpdate{OnceLocal: &value})
		if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != "onceLocal" || verr.Fields[0].Code != code {
			t.Errorf("date %q: %v, want onceLocal %s", value, err, code)
		}
	}
	if s := e.schedule("a.valid"); !s.OnceAt.IsZero() {
		t.Fatalf("a refused date was stored: %+v", s)
	}
	for _, at := range []time.Time{now.Add(OnceMinLead), now.Add(OnceMaxLead)} {
		if s := e.update("a.valid", ScheduleUpdate{OnceLocal: wall(at)}); !s.OnceAt.Equal(at) || !s.NextRunAt.Equal(next) {
			t.Errorf("date %v: schedule %+v, want the date and the next run %v unchanged", at, s, next)
		}
	}
	if s := e.update("a.valid", ScheduleUpdate{ClearOnce: true, OnceLocal: wall(now.Add(time.Hour))}); !s.OnceAt.IsZero() || s.OnceLocal != "" {
		t.Errorf("schedule after ClearOnce = %+v, want no date", s)
	}
}
