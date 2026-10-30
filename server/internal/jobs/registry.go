package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"
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

// Registry holds the registered job types in registration order.
type Registry struct {
	mu    sync.RWMutex
	types map[string]Type
	order []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{types: map[string]Type{}}
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

// Attach registers the handlers of the types of pools on w (all pools when none is named). Call
// it before w.Run.
func (r *Registry) Attach(w *Worker, pools ...Pool) {
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
