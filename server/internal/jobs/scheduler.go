package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The scheduler (concept 09 section 6.1, ADR 0017) replaces the five ticker loops of library,
// metadata and admin: one goroutine per server, a tick every minute, and the table job_schedules
// as the single place that knows the next run of every schedule type. Intervals instead of cron
// expressions; a run the server missed while it was down is not made up for (a rhythm, not a
// calendar, like the pollers before). Rules, each with a test in scheduler_test.go:
//
//   - A paused type (job_types.paused_at) is not run, but its next run moves on: the pause skips
//     runs, it does not shift the rhythm.
//   - UpdateSchedule sets next_run_at = last_fired_at + every (now + every without a last firing),
//     at least now, then aligns it to the window; a shorter interval brings the next run forward.
//   - After every firing and every update the next run is aligned to the window (window_start and
//     window_end in minutes of the day in the scheduler's location, the server's TZ; end < start
//     spans midnight): a next run outside the window moves to the next start of the window. An
//     interval shorter than the window fires inside it at the interval, a longer one once per
//     window.
//   - A row whose type is not registered (after a downgrade) is skipped and stays.
//   - At start, a type with StartDelay fires after that delay (0: at once); without one the next
//     run of an existing row stands, so a restart does not shift the rhythm. A missing row is
//     created from Type.Every and Type.Seed (the setting and history of the earlier poller).

// Bounds of an interval (the CHECK of job_schedules.every_ms).
const (
	MinEvery = 5 * time.Minute
	MaxEvery = 7 * 24 * time.Hour
	// SchedulerTick is how often the scheduler reads the due schedules.
	SchedulerTick = 60 * time.Second
	minutesPerDay = 24 * 60
)

// Window is a time window in minutes of the server's day; End is exclusive, End < Start spans
// midnight.
type Window struct {
	Start int
	End   int
}

// contains reports whether minute of the day lies in the window.
func (w Window) contains(minute int) bool {
	if w.Start < w.End {
		return minute >= w.Start && minute < w.End
	}
	return minute >= w.Start || minute < w.End
}

func (w Window) valid() bool {
	return w.Start >= 0 && w.Start < minutesPerDay && w.End >= 0 && w.End < minutesPerDay && w.Start != w.End
}

// Schedule is one row of job_schedules.
type Schedule struct {
	Type        string
	Every       time.Duration
	StartDelay  *time.Duration
	Enabled     bool
	Window      *Window
	NextRunAt   time.Time
	LastFiredAt time.Time // zero: never fired
	UpdatedAt   time.Time
}

// ScheduleUpdate names the fields UpdateSchedule changes; a nil field stays as it is. ClearWindow
// removes the window (Window is ignored then).
type ScheduleUpdate struct {
	Every       *time.Duration
	Enabled     *bool
	Window      *Window
	ClearWindow bool
}

// Scheduler runs the schedule types of a registry from job_schedules.
type Scheduler struct {
	q    *Queue
	r    *Registry
	sys  *systemq.Queries
	log  *slog.Logger
	now  func() time.Time
	loc  *time.Location
	tick time.Duration
}

// SchedulerOption configures NewScheduler.
type SchedulerOption func(*Scheduler)

// WithLocation sets the location of the time windows (default time.Local, the TZ of the
// container).
func WithLocation(loc *time.Location) SchedulerOption {
	return func(s *Scheduler) { s.loc = loc }
}

// WithTick sets the tick (default SchedulerTick), for tests.
func WithTick(d time.Duration) SchedulerOption {
	return func(s *Scheduler) { s.tick = d }
}

// NewScheduler returns a scheduler of the types of r on the database of q, with q's clock.
func NewScheduler(q *Queue, r *Registry, log *slog.Logger, opts ...SchedulerOption) *Scheduler {
	s := &Scheduler{q: q, r: r, sys: q.q, log: log.With("component", "scheduler"), now: q.now, loc: time.Local, tick: SchedulerTick}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Start writes the row in job_types of every registered type, creates the missing schedule rows
// and applies the start rule (StartDelay). Call it once, after every service registered its types
// and before Run.
func (s *Scheduler) Start(ctx context.Context) error {
	now := s.now()
	types := s.r.Types()
	for _, t := range types {
		if err := s.sys.EnsureJobType(ctx, systemq.EnsureJobTypeParams{Type: t.Name, Now: now.UnixMilli()}); err != nil {
			return fmt.Errorf("ensure job type %s: %w", t.Name, err)
		}
	}
	for _, t := range types {
		if !t.Scheduled() {
			continue
		}
		if err := s.ensureSchedule(ctx, t, now); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchedule creates the schedule row of t when it is missing, from Every and Seed, and
// applies the start rule to the row.
func (s *Scheduler) ensureSchedule(ctx context.Context, t Type, now time.Time) error {
	every, enabled, last := t.Every, true, time.Time{}
	if t.Seed != nil {
		seed, err := t.Seed(ctx)
		if err != nil {
			return fmt.Errorf("seed schedule of %s: %w", t.Name, err)
		}
		if seed.Every > 0 {
			every = seed.Every
		}
		enabled, last = seed.Enabled, seed.LastRun
	}
	if every < MinEvery || every > MaxEvery {
		return fmt.Errorf("seed schedule of %s: interval %v outside %v..%v", t.Name, every, MinEvery, MaxEvery)
	}
	next := now
	if !last.IsZero() && last.Add(every).After(now) {
		next = last.Add(every)
	}
	var delay sql.NullInt64
	if t.StartDelay != nil {
		delay = sql.NullInt64{Int64: t.StartDelay.Milliseconds(), Valid: true}
	}
	created, err := s.sys.InsertJobSchedule(ctx, systemq.InsertJobScheduleParams{
		Type: t.Name, EveryMs: every.Milliseconds(), StartDelayMs: delay, Enabled: boolInt(enabled),
		NextRunAt: next.UnixMilli(), Now: now.UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("insert schedule of %s: %w", t.Name, err)
	}
	if created == 1 {
		s.log.InfoContext(ctx, "schedule created", "job_type", t.Name, "every", every, "enabled", enabled, "next_run_at", next.UTC())
	}
	if t.StartDelay == nil {
		return nil
	}
	row, err := s.sys.GetJobSchedule(ctx, t.Name)
	if err != nil {
		return fmt.Errorf("get schedule of %s: %w", t.Name, err)
	}
	atStart := s.align(now.Add(*t.StartDelay), windowOf(row.WindowStart, row.WindowEnd))
	if _, err := s.sys.SetJobScheduleNextRun(ctx, systemq.SetJobScheduleNextRunParams{NextRunAt: atStart.UnixMilli(), Now: now.UnixMilli(), Type: t.Name}); err != nil {
		return fmt.Errorf("set start of %s: %w", t.Name, err)
	}
	return nil
}

// Run ticks at once and then every tick until ctx ends.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.tick)
	defer t.Stop()
	for {
		if err := s.Tick(ctx); err != nil && ctx.Err() == nil {
			s.log.ErrorContext(ctx, "scheduler tick", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick runs every enabled schedule whose next run has come and moves it on. A run that fails is
// logged and tried again at the next tick (its next run does not move); a paused type is skipped
// and moves on; a type that is not registered is skipped and stays.
func (s *Scheduler) Tick(ctx context.Context) error {
	now := s.now()
	rows, err := s.sys.ListDueJobSchedules(ctx, now.UnixMilli())
	if err != nil {
		return fmt.Errorf("list due schedules: %w", err)
	}
	for _, row := range rows {
		t, ok := s.r.Lookup(row.Type)
		if !ok || t.Run == nil {
			s.log.DebugContext(ctx, "schedule of an unregistered job type skipped", "job_type", row.Type)
			continue
		}
		log := s.log.With("job_type", row.Type)
		switch {
		case row.PausedAt.Valid:
			log.InfoContext(ctx, "scheduled run skipped, job type paused")
		default:
			ids, err := t.Run(ctx, s.q, t.Priority)
			if err != nil {
				if ctx.Err() == nil {
					log.WarnContext(ctx, "scheduled run", "err", err)
				}
				continue
			}
			log.InfoContext(ctx, "scheduled run queued", "jobs", len(ids))
		}
		next := s.align(now.Add(time.Duration(row.EveryMs)*time.Millisecond), windowOf(row.WindowStart, row.WindowEnd))
		if _, err := s.sys.MarkJobScheduleFired(ctx, systemq.MarkJobScheduleFiredParams{NextRunAt: next.UnixMilli(), Now: now.UnixMilli(), Type: row.Type}); err != nil {
			return fmt.Errorf("mark schedule %s fired: %w", row.Type, err)
		}
	}
	return nil
}

// UpdateSchedule changes interval, switch and window of the schedule of typ and realigns its
// next run (last firing plus interval, at least now, then the window). An unknown type is
// model.ErrNotFound; an interval outside MinEvery..MaxEvery or an invalid window a
// model.ValidationError (fields everySeconds, window).
func (s *Scheduler) UpdateSchedule(ctx context.Context, typ string, u ScheduleUpdate) (Schedule, error) {
	row, err := s.sys.GetJobSchedule(ctx, typ)
	if errors.Is(err, sql.ErrNoRows) {
		return Schedule{}, model.ErrNotFound
	}
	if err != nil {
		return Schedule{}, fmt.Errorf("get schedule of %s: %w", typ, err)
	}
	cur := fromScheduleRow(row)
	every, enabled, window := cur.Every, cur.Enabled, cur.Window
	if u.Every != nil {
		every = *u.Every
	}
	if u.Enabled != nil {
		enabled = *u.Enabled
	}
	switch {
	case u.ClearWindow:
		window = nil
	case u.Window != nil:
		w := *u.Window
		window = &w
	}
	var verr *model.ValidationError
	if every < MinEvery || every > MaxEvery {
		verr = verr.Add("everySeconds", "invalid_value")
	}
	if window != nil && !window.valid() {
		verr = verr.Add("window", "invalid_value")
	}
	if verr != nil {
		return Schedule{}, verr
	}
	now := s.now()
	base := now.Add(every)
	if !cur.LastFiredAt.IsZero() {
		base = cur.LastFiredAt.Add(every)
	}
	if base.Before(now) {
		base = now
	}
	next := s.align(base, window)
	ws, we := windowColumns(window)
	n, err := s.sys.UpdateJobSchedule(ctx, systemq.UpdateJobScheduleParams{
		EveryMs: every.Milliseconds(), Enabled: boolInt(enabled), WindowStart: ws, WindowEnd: we,
		NextRunAt: next.UnixMilli(), Now: now.UnixMilli(), Type: typ,
	})
	if err != nil {
		return Schedule{}, fmt.Errorf("update schedule of %s: %w", typ, err)
	}
	if n == 0 {
		return Schedule{}, model.ErrNotFound
	}
	s.log.InfoContext(ctx, "schedule updated", "job_type", typ, "every", every, "enabled", enabled, "window", window != nil, "next_run_at", next.UTC())
	return s.Schedule(ctx, typ)
}

// Schedule returns the schedule of typ; model.ErrNotFound without a row.
func (s *Scheduler) Schedule(ctx context.Context, typ string) (Schedule, error) {
	row, err := s.sys.GetJobSchedule(ctx, typ)
	if errors.Is(err, sql.ErrNoRows) {
		return Schedule{}, model.ErrNotFound
	}
	if err != nil {
		return Schedule{}, fmt.Errorf("get schedule of %s: %w", typ, err)
	}
	return fromScheduleRow(row), nil
}

// Schedules returns every schedule row, by type.
func (s *Scheduler) Schedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.sys.ListJobSchedules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	out := make([]Schedule, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromScheduleRow(r))
	}
	return out, nil
}

// Location returns the location of the time windows.
func (s *Scheduler) Location() *time.Location { return s.loc }

// align moves t to the next start of the window when t lies outside it; without a window t
// stands. Minutes of the day are read in the scheduler's location, so a window "03:00 to 05:00"
// means the server's local time, also across a change of daylight saving time.
func (s *Scheduler) align(t time.Time, w *Window) time.Time {
	if w == nil {
		return t
	}
	lt := t.In(s.loc)
	if w.contains(lt.Hour()*60 + lt.Minute()) {
		return t
	}
	y, m, d := lt.Date()
	start := time.Date(y, m, d, 0, w.Start, 0, 0, s.loc)
	if !start.After(t) {
		start = time.Date(y, m, d+1, 0, w.Start, 0, 0, s.loc)
	}
	return start
}

func windowOf(start, end sql.NullInt64) *Window {
	if !start.Valid || !end.Valid {
		return nil
	}
	return &Window{Start: int(start.Int64), End: int(end.Int64)}
}

func windowColumns(w *Window) (start, end sql.NullInt64) {
	if w == nil {
		return sql.NullInt64{}, sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(w.Start), Valid: true}, sql.NullInt64{Int64: int64(w.End), Valid: true}
}

func fromScheduleRow(r systemq.JobSchedule) Schedule {
	out := Schedule{
		Type: r.Type, Every: time.Duration(r.EveryMs) * time.Millisecond, Enabled: r.Enabled == 1,
		Window: windowOf(r.WindowStart, r.WindowEnd), NextRunAt: time.UnixMilli(r.NextRunAt).UTC(),
		UpdatedAt: time.UnixMilli(r.UpdatedAt).UTC(),
	}
	if r.StartDelayMs.Valid {
		d := time.Duration(r.StartDelayMs.Int64) * time.Millisecond
		out.StartDelay = &d
	}
	if r.LastFiredAt.Valid {
		out.LastFiredAt = time.UnixMilli(r.LastFiredAt.Int64).UTC()
	}
	return out
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
