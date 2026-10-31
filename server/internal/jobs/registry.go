package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Job types as a concept of their own (concept 09, ADR 0017): every service registers what a job
// type is, not only how one job of it runs. The registry hands the handlers to the workers of
// the type's pool, the scheduler runs the types with a schedule, and the administration page of
// step 4 reads group, trigger and pool from it. Everything about a type except its pause lives
// here, in code; the database holds the pause (job_types) and the schedule (job_schedules).

// Group is the group a type belongs to on the administration page.
type Group string

// Groups of the administration page.
const (
	GroupLibrary  Group = "library"
	GroupMetadata Group = "metadata"
	GroupDelivery Group = "delivery"
	GroupSystem   Group = "system"
)

// Pool is the worker pool that runs the jobs of a type (main.go starts one set of workers per
// pool; a worker claims only the types attached to it).
type Pool string

// Worker pools.
const (
	PoolGeneral  Pool = "general"
	PoolDownload Pool = "download"
	PoolNotify   Pool = "notify"
)

// Trigger says what starts a run of the type: the scheduler, an event in the server (a scan, a
// playback, a notification) or an administrator by hand.
type Trigger string

// Triggers.
const (
	TriggerSchedule Trigger = "schedule"
	TriggerEvent    Trigger = "event"
	TriggerManual   Trigger = "manual"
)

// ManualPriority is the priority of a run an administrator starts by hand ("Jetzt ausführen",
// "Jetzt abrufen", the manual match; decision of 26 October 2026): above every scheduled and
// bulk job, below the keyframe index of a playback (9).
const ManualPriority = 7

// Type describes one job type for the worker, the scheduler and the administration page.
type Type struct {
	Name    string  // "scan.library"
	Group   Group   // library, metadata, delivery, system
	Pool    Pool    // general, download, notify
	Trigger Trigger // schedule, event, manual
	// Priority is the priority of a scheduled run of this type; a manual run uses ManualPriority.
	Priority int
	// Handler runs one job (as before). nil for a schedule type whose runs are jobs of another
	// type (podcast.poll queues podcast.fetch).
	Handler Handler
	// Run enqueues what one run of this type means (every library, every due feed) with prio
	// and returns the job ids. It must raise a waiting job that holds the same idempotency key
	// to prio (RaiseQueuedPriority, decision of 26 October 2026). nil for event types.
	Run func(ctx context.Context, q *Queue, prio int) ([]int64, error)
	// Cancel ends the domain state of job j before the queue cancels it (a scan ends aborted,
	// a download ends failed so that Reconcile does not queue it again). nil: queue only.
	Cancel func(ctx context.Context, j Job) error
	// Every is the interval of a schedule type: the default of the first row in job_schedules;
	// once the row exists the table is the source (UpdateSchedule changes it).
	Every time.Duration
	// StartDelay says when the schedule fires after the server started: nil = do not fire at
	// start (the next run of the row stands), 0 = at once, 60 s = update.check.
	StartDelay *time.Duration
	// Seed returns the values of the first schedule row of this type from an existing setting
	// and the history of the earlier poller (trakt.refresh: spotlight_config.interval_hours and
	// the last attempt; update.check: update_check_enabled and the last check). nil: Every,
	// enabled, no history.
	Seed func(ctx context.Context) (ScheduleSeed, error)
}

// ScheduleSeed are the values of the first schedule row of a type (Type.Seed).
type ScheduleSeed struct {
	Every   time.Duration // 0: Type.Every
	Enabled bool
	LastRun time.Time // zero: never ran; else next_run_at = LastRun + Every, at least now
}

// Scheduled reports whether the type has a schedule.
func (t Type) Scheduled() bool { return t.Trigger == TriggerSchedule }

// Registry holds the registered job types in registration order, the list of running jobs of
// the workers attached to it, and the pause and cancel of types (concept 09, 6.3).
type Registry struct {
	q     *Queue
	log   *slog.Logger
	mu    sync.RWMutex
	types map[string]Type
	order []string
	runs  *runList
}

// NewRegistry returns a registry on the queue q with the housekeeping type jobs.prune
// registered.
func NewRegistry(q *Queue, log *slog.Logger) *Registry {
	r := &Registry{q: q, log: log.With("component", "jobs"), types: map[string]Type{}, runs: newRunList()}
	r.Register(pruneType(q))
	return r
}

// Queue returns the queue of the registry.
func (r *Registry) Queue() *Queue { return r.q }

// runList is the list of the jobs the attached workers run right now, with the cancel of each
// job's context, so a pause or a cancel stops a run in under a second instead of at the next
// heartbeat.
type runList struct {
	mu   sync.Mutex
	runs map[int64]runEntry
}

type runEntry struct {
	typ    string
	cancel context.CancelCauseFunc
	rep    *reporter // nil in tests that add a run without one
}

func newRunList() *runList { return &runList{runs: map[int64]runEntry{}} }

func (l *runList) add(id int64, typ string, cancel context.CancelCauseFunc, rep *reporter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.runs[id] = runEntry{typ: typ, cancel: cancel, rep: rep}
}

// series returns the progress series of the running job id (see reporter.series).
func (l *runList) series(id int64) (time.Time, int64, bool) {
	l.mu.Lock()
	e, ok := l.runs[id]
	l.mu.Unlock()
	if !ok || e.rep == nil {
		return time.Time{}, 0, false
	}
	return e.rep.series()
}

func (l *runList) remove(id int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.runs, id)
}

// cancelType ends the context of every running job of typ with cause and returns how many.
func (l *runList) cancelType(typ string, cause error) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.runs {
		if e.typ == typ {
			e.cancel(cause)
			n++
		}
	}
	return n
}

// cancelJob ends the context of the running job id with cause and reports whether it ran here.
func (l *runList) cancelJob(id int64, cause error) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.runs[id]
	if ok {
		e.cancel(cause)
	}
	return ok
}

// count returns how many jobs of typ run right now.
func (l *runList) count(typ string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.runs {
		if e.typ == typ {
			n++
		}
	}
	return n
}

// Running returns how many jobs of typ the attached workers run right now (after a pause: the
// runs that are still stopping).
func (r *Registry) Running(typ string) int { return r.runs.count(typ) }

// ProgressSince returns when the current progress series of the running job id began on an
// attached worker and the done count it began with (concept 09, 6.2; the measured estimate rates
// the work since then instead of since the start of the run, which may hold phases without
// progress). ok is false for a job that does not run here or has reported no extent yet.
func (r *Registry) ProgressSince(id int64) (since time.Time, done int64, ok bool) {
	return r.runs.series(id)
}

// Pause pauses the job type typ (F-1, F-6): no worker claims its jobs any more, and every running
// job of the type is stopped at once and goes back to the queue without a used attempt. The pause
// is a row in job_types and survives a restart; a type without a row is model.ErrNotFound.
// Idempotent.
func (r *Registry) Pause(ctx context.Context, typ string) error {
	now := r.q.now()
	n, err := r.q.q.PauseJobType(ctx, systemq.PauseJobTypeParams{Now: now.UnixMilli(), Type: typ})
	if err != nil {
		return fmt.Errorf("pause %s: %w", typ, err)
	}
	if n == 0 {
		if _, err := r.q.q.GetJobType(ctx, typ); errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		} else if err != nil {
			return fmt.Errorf("get job type %s: %w", typ, err)
		}
	}
	stopped := r.runs.cancelType(typ, ErrPaused)
	r.log.InfoContext(ctx, "job type paused", "job_type", typ, "stopping", stopped)
	return nil
}

// Resume ends the pause of typ; model.ErrNotFound without a row. Idempotent.
func (r *Registry) Resume(ctx context.Context, typ string) error {
	n, err := r.q.q.ResumeJobType(ctx, systemq.ResumeJobTypeParams{Now: r.q.now().UnixMilli(), Type: typ})
	if err != nil {
		return fmt.Errorf("resume %s: %w", typ, err)
	}
	if n == 0 {
		if _, err := r.q.q.GetJobType(ctx, typ); errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		} else if err != nil {
			return fmt.Errorf("get job type %s: %w", typ, err)
		}
	}
	r.log.InfoContext(ctx, "job type resumed", "job_type", typ)
	return nil
}

// CancelJob cancels one job through the cancel hook of its type (Type.Cancel: a scan ends
// aborted, a download failed), then in the queue, and stops a running one at once. It reports
// whether a job was cancelled (false: unknown, or already in a final state other than failed).
func (r *Registry) CancelJob(ctx context.Context, id int64) (bool, error) {
	j, err := r.q.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return r.cancel(ctx, j)
}

// CancelRuns cancels the queued jobs of typ, and with running also the running ones (F-2), each
// through the cancel hook of the type. It returns how many it cancelled.
func (r *Registry) CancelRuns(ctx context.Context, typ string, running bool) (int, error) {
	jobs, err := r.q.ListActive(ctx, typ)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, j := range jobs {
		if j.State == StateRunning && !running {
			continue
		}
		ok, err := r.cancel(ctx, j)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

func (r *Registry) cancel(ctx context.Context, j Job) (bool, error) {
	// The hook runs for a job that still waits or runs; a failed one is only dismissed, a
	// finished one is nothing to cancel.
	if t, ok := r.Lookup(j.Type); ok && t.Cancel != nil && (j.State == StateQueued || j.State == StateRunning) {
		if err := t.Cancel(ctx, j); err != nil {
			r.log.WarnContext(ctx, "cancel hook", "job_id", j.ID, "job_type", j.Type, "err", err)
		}
	}
	// The hook may have cancelled the row itself (CancelScan); the queue's cancel is then a no-op
	// and the running handler is still stopped through the run list.
	cancelled, err := r.q.Cancel(ctx, j.ID)
	if err != nil {
		return false, err
	}
	if !cancelled && (j.State == StateQueued || j.State == StateRunning) {
		if cur, err := r.q.Get(ctx, j.ID); err == nil && cur.State == StateCancelled {
			cancelled = true // the hook cancelled the row
		}
	}
	stopped := r.runs.cancelJob(j.ID, ErrCancelled)
	if cancelled || stopped {
		r.log.InfoContext(ctx, "job cancelled by the administration", "job_id", j.ID, "job_type", j.Type, "was_running", stopped)
	}
	return cancelled || stopped, nil
}

// Register adds t. A type that is not consistent (no name, a schedule type without Run or
// interval, an event type with Run, a priority outside 0..9, a name registered twice) is a
// programming error and panics at startup, like a duplicate http route.
func (r *Registry) Register(t Type) {
	if err := validateType(t); err != nil {
		panic(fmt.Sprintf("jobs: register %q: %v", t.Name, err))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.types[t.Name]; dup {
		panic(fmt.Sprintf("jobs: job type %q registered twice", t.Name))
	}
	r.types[t.Name] = t
	r.order = append(r.order, t.Name)
}

func validateType(t Type) error {
	switch {
	case t.Name == "":
		return fmt.Errorf("name must not be empty")
	case t.Priority < 0 || t.Priority > 9:
		return fmt.Errorf("priority %d outside 0..9", t.Priority)
	case t.Group == "" || t.Pool == "" || t.Trigger == "":
		return fmt.Errorf("group, pool and trigger are required")
	case t.Handler == nil && t.Run == nil:
		return fmt.Errorf("a type needs a handler or a run function")
	}
	switch t.Trigger {
	case TriggerSchedule:
		if t.Run == nil {
			return fmt.Errorf("a schedule type needs Run")
		}
		if t.Every < MinEvery || t.Every > MaxEvery {
			return fmt.Errorf("interval %v outside %v..%v", t.Every, MinEvery, MaxEvery)
		}
	case TriggerEvent:
		if t.Run != nil || t.Every != 0 || t.StartDelay != nil || t.Seed != nil {
			return fmt.Errorf("an event type has neither Run nor a schedule")
		}
	case TriggerManual:
		if t.Run == nil {
			return fmt.Errorf("a manual type needs Run")
		}
		if t.Every != 0 || t.StartDelay != nil || t.Seed != nil {
			return fmt.Errorf("a manual type has no schedule")
		}
	default:
		return fmt.Errorf("unknown trigger %q", t.Trigger)
	}
	return nil
}

// Types returns the registered types in registration order.
func (r *Registry) Types() []Type {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Type, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.types[name])
	}
	return out
}

// Lookup returns the type with name.
func (r *Registry) Lookup(name string) (Type, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.types[name]
	return t, ok
}

// Attach registers the handlers of the types of pools on w (all pools when none is named) and
// puts w's runs on the registry's run list. Call it before w.Run.
func (r *Registry) Attach(w *Worker, pools ...Pool) {
	w.runs = r.runs
	for _, t := range r.Types() {
		if t.Handler == nil || (len(pools) > 0 && !containsPool(pools, t.Pool)) {
			continue
		}
		w.Handle(t.Name, t.Handler)
	}
}

func containsPool(pools []Pool, p Pool) bool {
	for _, c := range pools {
		if c == p {
			return true
		}
	}
	return false
}
