package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
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
//     created from Type.Every and Type.Seed (the setting and history of the earlier poller, and
//     a window the first run is aligned into).
//
// The one-off date (job_schedules.once_at, concept 14 6.7, ADR 0022) is a promise, not a rhythm:
//
//   - A row is due when its one-off date has come, whatever its switch (enabled = 0 does not hold
//     the date back; the switch is the interval's) and its window (who says 03:30 means 03:30).
//   - The date fires Run with the type's normal priority, then clears the date and sets
//     last_fired_at; next_run_at stays (a date does not shift the rhythm, like "Jetzt
//     ausführen"). The clearing names the date that was read, so a date the administrator set
//     again between reading and firing stays. Date and interval in the same tick run once.
//   - A paused type skips the date: it lapses with a log, like the pause skips interval runs.
//   - Missed while the server was down: at start a date at most OnceCatchUp old fires once, after
//     the type's StartDelay (at the first tick without one); an older date lapses with a log.
//   - UpdateSchedule takes the date as wall time in the scheduler's location (OnceLayout), from
//     now + OnceMinLead to now + OnceMaxLead, and stores it in ms UTC.

// Bounds of an interval (the CHECK of job_schedules.every_ms).
const (
	MinEvery = 5 * time.Minute
	MaxEvery = 7 * 24 * time.Hour
	// SchedulerTick is how often the scheduler reads the due schedules.
	SchedulerTick = 60 * time.Second
	minutesPerDay = 24 * 60
	// OnceLayout is the wall time of a one-off date in the scheduler's location.
	OnceLayout = "2006-01-02T15:04"
	// Bounds of a one-off date relative to now, and how old a missed one may be at start.
	OnceMinLead = time.Minute
	OnceMaxLead = 366 * 24 * time.Hour
	OnceCatchUp = 24 * time.Hour
)

// onceShape is the fixed width of OnceLayout: the layout element "15" alone also takes a one-digit
// hour ("2026-12-01T3:30").
var onceShape = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$`)

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

// OpensAt returns when the window opens next after now in loc, and false while now lies in it
// (the "waits until 01:00" of a type with a window, concept 15 4.3). Same rule as the alignment of
// a schedule's next run.
func (w Window) OpensAt(now time.Time, loc *time.Location) (time.Time, bool) {
	at := alignIn(now, &w, loc)
	return at, !at.Equal(now)
}

// minuteOfDay returns the minute of the day of t in loc (0..1439), the clock of every window.
func minuteOfDay(t time.Time, loc *time.Location) int {
	lt := t.In(loc)
	return lt.Hour()*60 + lt.Minute()
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
	// OnceAt is the one-off date (zero: none), OnceLocal the same as wall time in the scheduler's
	// location (OnceLayout; "" without a date).
	OnceAt    time.Time
	OnceLocal string
}

// ScheduleUpdate names the fields UpdateSchedule changes; a nil field stays as it is. ClearWindow
// removes the window (Window is ignored then), ClearOnce the one-off date (OnceLocal is ignored
// then).
type ScheduleUpdate struct {
	Every       *time.Duration
	Enabled     *bool
	Window      *Window
	ClearWindow bool
	// OnceLocal sets the one-off date as wall time in the scheduler's location (OnceLayout).
	OnceLocal *string
	ClearOnce bool
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
// container). NewScheduler hands it to the queue, so the windows of the job types (Queue.Claim)
// read the same clock as the windows of the schedules.
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
	q.loc.Store(s.loc)
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
	var window *Window
	if t.Seed != nil {
		seed, err := t.Seed(ctx)
		if err != nil {
			return fmt.Errorf("seed schedule of %s: %w", t.Name, err)
		}
		if seed.Every > 0 {
			every = seed.Every
		}
		enabled, last, window = seed.Enabled, seed.LastRun, seed.Window
	}
	if every < MinEvery || every > MaxEvery {
		return fmt.Errorf("seed schedule of %s: interval %v outside %v..%v", t.Name, every, MinEvery, MaxEvery)
	}
	if window != nil && !window.valid() {
		return fmt.Errorf("seed schedule of %s: invalid window %+v", t.Name, *window)
	}
	next := now
	if !last.IsZero() && last.Add(every).After(now) {
		next = last.Add(every)
	}
	// A seeded window holds the first run as well: Tick fires a due interval without looking at
	// the window, so an unaligned first row would run at once, outside it.
	next = s.align(next, window)
	ws, we := windowColumns(window)
	var delay sql.NullInt64
	if t.StartDelay != nil {
		delay = sql.NullInt64{Int64: t.StartDelay.Milliseconds(), Valid: true}
	}
	created, err := s.sys.InsertJobSchedule(ctx, systemq.InsertJobScheduleParams{
		Type: t.Name, EveryMs: every.Milliseconds(), StartDelayMs: delay, Enabled: boolInt(enabled),
		WindowStart: ws, WindowEnd: we, NextRunAt: next.UnixMilli(), Now: now.UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("insert schedule of %s: %w", t.Name, err)
	}
	if created == 1 {
		s.log.InfoContext(ctx, "schedule created", "job_type", t.Name, "every", every, "enabled", enabled, "next_run_at", next.UTC())
	}
	row, err := s.sys.GetJobSchedule(ctx, t.Name)
	if err != nil {
		return fmt.Errorf("get schedule of %s: %w", t.Name, err)
	}
	if err := s.missedOnce(ctx, t, row, now); err != nil {
		return err
	}
	if t.StartDelay == nil {
		return nil
	}
	atStart := s.align(now.Add(*t.StartDelay), windowOf(row.WindowStart, row.WindowEnd))
	if _, err := s.sys.SetJobScheduleNextRun(ctx, systemq.SetJobScheduleNextRunParams{NextRunAt: atStart.UnixMilli(), Now: now.UnixMilli(), Type: t.Name}); err != nil {
		return fmt.Errorf("set start of %s: %w", t.Name, err)
	}
	return nil
}

// missedOnce applies the start rule of a one-off date that passed while the server was down: at
// most OnceCatchUp old it fires once, after the type's StartDelay (at the first tick without one);
// older it lapses.
func (s *Scheduler) missedOnce(ctx context.Context, t Type, row systemq.JobSchedule, now time.Time) error {
	if !row.OnceAt.Valid || row.OnceAt.Int64 > now.UnixMilli() {
		return nil
	}
	at := time.UnixMilli(row.OnceAt.Int64)
	var moved sql.NullInt64
	switch {
	case now.Sub(at) > OnceCatchUp:
		s.log.InfoContext(ctx, "one-off run dropped, missed by more than a day", "job_type", t.Name, "once_at", at.UTC())
	case t.StartDelay != nil && *t.StartDelay > 0:
		moved = sql.NullInt64{Int64: now.Add(*t.StartDelay).UnixMilli(), Valid: true}
	default:
		return nil // due: the first tick fires it
	}
	if _, err := s.sys.ReplaceJobScheduleOnce(ctx, systemq.ReplaceJobScheduleOnceParams{OnceAt: moved, Now: now.UnixMilli(), Type: t.Name, ReadOnceAt: row.OnceAt}); err != nil {
		return fmt.Errorf("start rule of the one-off date of %s: %w", t.Name, err)
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

// Tick runs every enabled schedule whose next run has come and moves it on, and every schedule
// whose one-off date has come and clears the date; both in one row run once. A run that fails is
// logged and tried again at the next tick (neither its next run nor its date moves); a paused type
// is skipped, its next run moves on and its date lapses; a type that is not registered is skipped
// and stays.
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
		intervalDue := row.Enabled == 1 && row.NextRunAt <= now.UnixMilli()
		onceDue := row.OnceAt.Valid && row.OnceAt.Int64 <= now.UnixMilli()
		switch {
		case row.PausedAt.Valid:
			if intervalDue {
				log.InfoContext(ctx, "scheduled run skipped, job type paused")
			}
			if onceDue {
				log.InfoContext(ctx, "one-off run dropped, job type paused", "once_at", time.UnixMilli(row.OnceAt.Int64).UTC())
				if _, err := s.sys.ReplaceJobScheduleOnce(ctx, systemq.ReplaceJobScheduleOnceParams{Now: now.UnixMilli(), Type: row.Type, ReadOnceAt: row.OnceAt}); err != nil {
					return fmt.Errorf("drop one-off date of %s: %w", row.Type, err)
				}
			}
		default:
			ids, err := t.Run(ctx, s.q, t.Priority)
			if err != nil {
				if ctx.Err() == nil {
					log.WarnContext(ctx, "scheduled run", "err", err)
				}
				continue
			}
			log.InfoContext(ctx, "scheduled run queued", "jobs", len(ids), "one_off", onceDue)
			if onceDue {
				if _, err := s.sys.MarkJobScheduleOnceFired(ctx, systemq.MarkJobScheduleOnceFiredParams{Now: now.UnixMilli(), Type: row.Type, FiredOnceAt: row.OnceAt}); err != nil {
					return fmt.Errorf("mark one-off date of %s fired: %w", row.Type, err)
				}
			}
		}
		if !intervalDue {
			continue
		}
		next := s.align(now.Add(time.Duration(row.EveryMs)*time.Millisecond), windowOf(row.WindowStart, row.WindowEnd))
		if _, err := s.sys.MarkJobScheduleFired(ctx, systemq.MarkJobScheduleFiredParams{NextRunAt: next.UnixMilli(), Now: now.UnixMilli(), Type: row.Type}); err != nil {
			return fmt.Errorf("mark schedule %s fired: %w", row.Type, err)
		}
	}
	return nil
}

// UpdateSchedule changes interval, switch, window and one-off date of the schedule of typ and
// realigns its next run (last firing plus interval, at least now, then the window); an update of
// the date alone leaves the next run as it is. An unknown type is model.ErrNotFound; an interval
// outside MinEvery..MaxEvery, an invalid window or a date that is no wall time (OnceLayout) or lies
// outside now + OnceMinLead .. now + OnceMaxLead a model.ValidationError (fields everySeconds,
// window, onceLocal; codes invalid_value, out_of_range).
func (s *Scheduler) UpdateSchedule(ctx context.Context, typ string, u ScheduleUpdate) (Schedule, error) {
	row, err := s.sys.GetJobSchedule(ctx, typ)
	if errors.Is(err, sql.ErrNoRows) {
		return Schedule{}, model.ErrNotFound
	}
	if err != nil {
		return Schedule{}, fmt.Errorf("get schedule of %s: %w", typ, err)
	}
	cur := fromScheduleRow(row, s.loc)
	now := s.now()
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
	var once sql.NullInt64
	if !u.ClearOnce && u.OnceLocal != nil {
		at, err := time.ParseInLocation(OnceLayout, *u.OnceLocal, s.loc)
		switch {
		case err != nil || !onceShape.MatchString(*u.OnceLocal):
			verr = verr.Add("onceLocal", "invalid_value")
		case at.Before(now.Add(OnceMinLead)) || at.After(now.Add(OnceMaxLead)):
			verr = verr.Add("onceLocal", "out_of_range")
		default:
			once = sql.NullInt64{Int64: at.UnixMilli(), Valid: true}
		}
	}
	if verr != nil {
		return Schedule{}, verr
	}
	// The date and the interval part are written in one transaction, so a failing second
	// statement does not leave the date stored behind an error.
	tx, err := s.q.db.BeginTx(ctx, nil)
	if err != nil {
		return Schedule{}, fmt.Errorf("update schedule of %s: begin: %w", typ, err)
	}
	defer func() { _ = tx.Rollback() }()
	w := s.sys.WithTx(tx)
	setOnce := u.ClearOnce || u.OnceLocal != nil
	if setOnce {
		if _, err := w.SetJobScheduleOnce(ctx, systemq.SetJobScheduleOnceParams{OnceAt: once, Now: now.UnixMilli(), Type: typ}); err != nil {
			return Schedule{}, fmt.Errorf("set one-off date of %s: %w", typ, err)
		}
	}
	dateOnly := setOnce && u.Every == nil && u.Enabled == nil && u.Window == nil && !u.ClearWindow
	var next time.Time
	if !dateOnly {
		base := now.Add(every)
		if !cur.LastFiredAt.IsZero() {
			base = cur.LastFiredAt.Add(every)
		}
		if base.Before(now) {
			base = now
		}
		next = s.align(base, window)
		ws, we := windowColumns(window)
		n, err := w.UpdateJobSchedule(ctx, systemq.UpdateJobScheduleParams{
			EveryMs: every.Milliseconds(), Enabled: boolInt(enabled), WindowStart: ws, WindowEnd: we,
			NextRunAt: next.UnixMilli(), Now: now.UnixMilli(), Type: typ,
		})
		if err != nil {
			return Schedule{}, fmt.Errorf("update schedule of %s: %w", typ, err)
		}
		if n == 0 {
			return Schedule{}, model.ErrNotFound
		}
	}
	if err := tx.Commit(); err != nil {
		return Schedule{}, fmt.Errorf("update schedule of %s: commit: %w", typ, err)
	}
	if setOnce {
		s.log.InfoContext(ctx, "one-off date updated", "job_type", typ, "once_at", nullMsUTC(once))
	}
	if !dateOnly {
		s.log.InfoContext(ctx, "schedule updated", "job_type", typ, "every", every, "enabled", enabled, "window", window != nil, "next_run_at", next.UTC())
	}
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
	return fromScheduleRow(row, s.loc), nil
}

// Schedules returns every schedule row, by type.
func (s *Scheduler) Schedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.sys.ListJobSchedules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	out := make([]Schedule, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromScheduleRow(r, s.loc))
	}
	return out, nil
}

// Location returns the location of the time windows.
func (s *Scheduler) Location() *time.Location { return s.loc }

// align moves t to the next start of the window when t lies outside it; without a window t
// stands. Minutes of the day are read in the scheduler's location, so a window "03:00 to 05:00"
// means the server's local time, also across a change of daylight saving time.
func (s *Scheduler) align(t time.Time, w *Window) time.Time { return alignIn(t, w, s.loc) }

// alignIn is align in loc.
func alignIn(t time.Time, w *Window, loc *time.Location) time.Time {
	if w == nil {
		return t
	}
	if w.contains(minuteOfDay(t, loc)) {
		return t
	}
	y, m, d := t.In(loc).Date()
	start := time.Date(y, m, d, 0, w.Start, 0, 0, loc)
	if !start.After(t) {
		start = time.Date(y, m, d+1, 0, w.Start, 0, 0, loc)
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

// nullMsUTC returns a nullable Unix-millisecond value as a UTC time for a log, nil for NULL.
func nullMsUTC(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return time.UnixMilli(v.Int64).UTC()
}

func fromScheduleRow(r systemq.JobSchedule, loc *time.Location) Schedule {
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
	if r.OnceAt.Valid {
		out.OnceAt = time.UnixMilli(r.OnceAt.Int64).UTC()
		out.OnceLocal = out.OnceAt.In(loc).Format(OnceLayout)
	}
	return out
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
