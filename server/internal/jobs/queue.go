// Package jobs is the persistent job queue of Slouchcat in SQLite (plan section 3.6, ADR 0002).
//
// A job is claimed with a lease (LeaseDuration) that its worker extends by heartbeat. A worker
// that dies stops extending; once the lease expires, another worker claims the job again. Every
// handler must therefore be idempotent by design: it can run more than once for the same job,
// partially or completely, and must reach the same result. Write handlers in batches (at most
// 500 rows per transaction) so a transaction stays far below the lease.
//
// Semantics:
//
//   - Priority 0 to 9, higher runs first; within a priority the oldest job runs first. The
//     ladder of every job type (the constants live next to the enqueuing code; keep this list
//     current when one moves):
//     9 probe.keyframes of a playback that fell back to transcoding (library.QueueKeyframesForPlayback)
//     8 update.check, the daily release check (admin): seconds of work, before every bulk job
//     7 manual metadata.match and refresh, manual trakt.refresh ("Jetzt abrufen")
//     6 scan.path
//     5 scan.library, download.prepare, chapters.images of a new file, a re-probe or after a playback
//     (library, concept 13)
//     4 metadata.match of movies and series, podcast.fetch, the match after a cleared override,
//     library.relink, the daily radio.catalog.sync (radio)
//     3 probe.file, metadata.match of episodes, notify.deliver
//     2 metadata.fill, metadata.vote_backfill, image.fetch of posters, segments.detect,
//     library.sweep, nightly trakt.refresh
//     1 probe.keyframes of a scan, image.fetch of episode stills, trailer.find, chapters.images of
//     the backfill (chapters.images_backfill)
//     A job that runs once a day and finishes in seconds belongs above the bulk rungs (6 and
//     below): a library import queues tens of thousands of jobs at 1 and 2, and a daily job at
//     the same rung waits behind all of them.
//   - Idempotency: an IdempotencyKey is unique among queued and running jobs. Enqueueing the same
//     key again returns the ID of the active job instead of adding a duplicate. After the job has
//     finished, the key can be used again.
//   - Debounce: set Delay to DebounceWindow together with a key. The first enqueue waits for the
//     window, every enqueue of the same key inside it coalesces into that job. A caller that
//     needs the work of both enqueues merges it into the pending job with SetQueuedPayload.
//   - Retry: a failed attempt is retried after 10 s · 2^(attempt-1) (10 s, 20 s, 40 s, …) until
//     MaxAttempts is reached; then the job ends in state failed with the last error.
//   - Retry later: a handler that cannot start yet (another job holds a lock it needs) returns
//     RetryLater; the job waits for the given time without using an attempt.
//   - Cancel: queued and running jobs can be cancelled; a running handler sees its context
//     cancelled at the next heartbeat.
//   - Pause override (concept 14, 6.5, ADR 0022): a job with override_pause is claimed while its
//     type is paused and a pause does not stop it. Only EnqueueOverridingPause and
//     RaiseQueuedJobForPlayback set it, for at most MaxPauseOverrides queued or running jobs at a
//     time, and only package library calls them (forbidigo in .golangci.yml).
//   - Time window (concept 15, 4.3): a type with a window (Type.Windowable, Registry.SetWindow) is
//     claimed only inside it, in minutes of the day in the scheduler's location; a job that runs
//     when the window closes finishes. A job with override_window is claimed outside it (a
//     playback request: EnqueueOverridingWindow, and EnqueueOverridingPause and
//     RaiseQueuedJobForPlayback set it as well; package library only, forbidigo). It has no cap.
//     The pause comes first: override_pause does not lift the window, override_window not the
//     pause. Idle workers poll every second (Worker.poll), so a window that opens, is set or is
//     removed takes effect at the next poll without a signal.
//
// Job types and schedules (concept 09, ADR 0017): every service registers its job types with
// Registry.Register (registry.go: group, pool, trigger, priority, handler, Run, interval), and the
// Scheduler (scheduler.go) runs the types with a schedule from the table job_schedules, with
// intervals, a time window and a switch per type instead of the ticker loops of before.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Timing of leases, retries and debouncing.
const (
	LeaseDuration      = 5 * time.Minute
	HeartbeatInterval  = 60 * time.Second
	RetryBase          = 10 * time.Second
	DebounceWindow     = 2 * time.Second
	DefaultMaxAttempts = 5
	maxBackoffShift    = 20 // caps the backoff at about 121 days
	// MaxPauseOverrides is how many queued or running jobs may carry override_pause at once (O-7):
	// a viewer who starts many titles without an index cannot lift the pause for all of them.
	MaxPauseOverrides = 3
)

// ErrLeaseLost is returned when the job is no longer running under the caller's lease: it was
// cancelled, completed, or its lease expired and another worker claimed it.
var ErrLeaseLost = errors.New("job lease lost")

// ErrPaused is the cause of a job context that the pause of its type ended (Registry.Pause): the
// worker releases the job without counting the attempt, whatever the handler returned.
var ErrPaused = errors.New("job type paused")

// ErrCancelled is the cause of a job context that an administrator's cancel ended
// (Registry.CancelJob, CancelRuns); the queue row is cancelled already.
var ErrCancelled = errors.New("job cancelled")

// RetryLaterError is returned by a handler that cannot start its job yet, for example because
// another job holds a lock it needs: the worker queues the job again after After without counting
// the attempt (Queue.Defer). Wrapped errors count too.
type RetryLaterError struct {
	After  time.Duration
	Reason string
}

func (e *RetryLaterError) Error() string { return "retry later: " + e.Reason }

// RetryLater returns a RetryLaterError: the job runs again after after, without using an attempt.
func RetryLater(after time.Duration, reason string) error {
	return &RetryLaterError{After: after, Reason: reason}
}

// State is the lifecycle state of a job.
type State string

// Job states.
const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateDone      State = "done"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// NewJob describes a job to enqueue.
type NewJob struct {
	Type           string          // Handler name, for example "scan.path".
	Priority       int             // 0 (lowest) to 9 (highest).
	IdempotencyKey string          // Optional; see the package documentation.
	Payload        json.RawMessage // Optional JSON payload; defaults to {}.
	Delay          time.Duration   // Earliest start relative to now; DebounceWindow to debounce.
	MaxAttempts    int             // 0 means DefaultMaxAttempts.
}

// Job is a job as stored in the queue.
type Job struct {
	ID             int64
	Type           string
	Priority       int
	IdempotencyKey string
	Payload        json.RawMessage
	State          State
	RunAfter       time.Time
	LeaseUntil     time.Time
	Attempts       int
	MaxAttempts    int
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// StartedAt is the start of the last attempt (zero before the first claim), FinishedAt the
	// end of the run in a final state (done, failed, cancelled; zero while it runs or waits).
	StartedAt  time.Time
	FinishedAt time.Time
	Progress   RunProgress // the last reported progress of the attempt
	Skipped    bool        // the run had nothing to do (Skip)
	// OverridesPause: the job is claimed and keeps running while its type is paused (a playback
	// request, EnqueueOverridingPause).
	OverridesPause bool
	// OverridesWindow: the job is claimed outside the time window of its type (a playback request,
	// EnqueueOverridingWindow; concept 15, 4.3).
	OverridesWindow bool
}

// Duration returns how long the last attempt ran: FinishedAt - StartedAt in a final state, the
// time since StartedAt while it runs; zero without a start.
func (j Job) Duration(now time.Time) time.Duration {
	switch {
	case j.StartedAt.IsZero():
		return 0
	case !j.FinishedAt.IsZero():
		return max(j.FinishedAt.Sub(j.StartedAt), 0)
	case j.State == StateRunning:
		return max(now.Sub(j.StartedAt), 0)
	}
	return 0
}

// Queue is the persistent queue on the writer connection of the database.
type Queue struct {
	db  *sql.DB
	q   *systemq.Queries
	now func() time.Time
	// loc is the location of the time windows of the types: the scheduler's (NewScheduler stores
	// it here), time.Local until then. Claim reads the minute of the day in it.
	loc atomic.Pointer[time.Location]
	// pruneTasks are the further cleanups of jobs.prune (AddPruneTask).
	pruneTasks []func(ctx context.Context) (int64, error)
	// afterSkippedInsert is a seam for tests: EnqueueOverridingPause calls it between a skipped
	// insert and the lookup of the job that holds the key.
	afterSkippedInsert func()
}

// NewQueue returns a queue on db. now is the clock; nil means time.Now.
func NewQueue(db *sql.DB, now func() time.Time) *Queue {
	if now == nil {
		now = time.Now
	}
	q := &Queue{db: db, q: systemq.New(db), now: now, afterSkippedInsert: func() {}}
	q.loc.Store(time.Local)
	return q
}

// Location returns the location of the time windows of the types (the scheduler's).
func (q *Queue) Location() *time.Location { return q.loc.Load() }

// Enqueue adds a job and returns its ID. With an IdempotencyKey that an active job already
// holds, it returns the ID of that job instead.
func (q *Queue) Enqueue(ctx context.Context, j NewJob) (int64, error) {
	payload, maxAttempts, key, err := j.columns()
	if err != nil {
		return 0, err
	}
	now := q.now()

	id, err := q.q.InsertJob(ctx, systemq.InsertJobParams{
		Type:           j.Type,
		Priority:       int64(j.Priority),
		IdempotencyKey: key,
		Payload:        string(payload),
		RunAfter:       now.Add(j.Delay).UnixMilli(),
		MaxAttempts:    int64(maxAttempts),
		Now:            now.UnixMilli(),
	})
	if errors.Is(err, sql.ErrNoRows) && key.Valid {
		// The insert was skipped because an active job holds the key.
		id, err = q.q.GetActiveJobIDByKey(ctx, key)
	}
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", j.Type, err)
	}
	return id, nil
}

// columns validates j and returns the payload ({} by default), the attempts (DefaultMaxAttempts by
// default) and the idempotency key of its row.
func (j NewJob) columns() (json.RawMessage, int, sql.NullString, error) {
	if j.Type == "" {
		return nil, 0, sql.NullString{}, errors.New("job type must not be empty")
	}
	if j.Priority < 0 || j.Priority > 9 {
		return nil, 0, sql.NullString{}, fmt.Errorf("priority %d outside 0..9", j.Priority)
	}
	maxAttempts := j.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = DefaultMaxAttempts
	}
	payload := j.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		return nil, 0, sql.NullString{}, errors.New("payload is not valid JSON")
	}
	return payload, maxAttempts, sql.NullString{String: j.IdempotencyKey, Valid: j.IdempotencyKey != ""}, nil
}

// PauseOverride says what EnqueueOverridingPause or RaiseQueuedJobForPlayback did.
type PauseOverride struct {
	ID int64
	// Inserted: EnqueueOverridingPause queued a new job. False: an active job holds the key and ID
	// is that job, unchanged (RaiseQueuedJobForPlayback raises it).
	Inserted bool
	// Raised: RaiseQueuedJobForPlayback changed the waiting job. False: it runs (and stays as it
	// is) or has ended.
	Raised bool
	// OverridesPause: the job carries override_pause and runs while its type is paused; false
	// when MaxPauseOverrides other jobs carry it (the job waits for the end of the pause).
	OverridesPause bool
}

// EnqueueOverridingPause is Enqueue for a job a playback needs now (concept 14, 6.4 and 6.5): the
// new job carries override_pause while fewer than MaxPauseOverrides queued or running jobs carry
// it, checked in the same statement as the insert, so concurrent starts cannot pass the cap, and
// always override_window (concept 15, 4.3: a playback lifts the time window, which has no cap).
// When an active job holds the key, nothing is inserted and Inserted is false. Only package
// library calls it (forbidigo).
func (q *Queue) EnqueueOverridingPause(ctx context.Context, j NewJob) (PauseOverride, error) {
	payload, maxAttempts, key, err := j.columns()
	if err != nil {
		return PauseOverride{}, err
	}
	now := q.now()
	// A job that holds the key can end between the skipped insert and the lookup of its ID; the
	// insert is then repeated once (the key is free by now).
	for attempt := 0; ; attempt++ {
		row, err := q.q.InsertJobOverridingPause(ctx, systemq.InsertJobOverridingPauseParams{
			Type:           j.Type,
			Priority:       int64(j.Priority),
			IdempotencyKey: key,
			Payload:        string(payload),
			RunAfter:       now.Add(j.Delay).UnixMilli(),
			MaxAttempts:    int64(maxAttempts),
			MaxOverrides:   MaxPauseOverrides,
			Now:            now.UnixMilli(),
		})
		if err == nil {
			return PauseOverride{ID: row.ID, Inserted: true, OverridesPause: row.OverridePause == 1}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) || !key.Valid {
			return PauseOverride{}, fmt.Errorf("enqueue %s: %w", j.Type, err)
		}
		// The insert was skipped because an active job holds the key.
		q.afterSkippedInsert()
		id, err := q.q.GetActiveJobIDByKey(ctx, key)
		if errors.Is(err, sql.ErrNoRows) && attempt == 0 {
			continue
		}
		if err != nil {
			return PauseOverride{}, fmt.Errorf("enqueue %s: %w", j.Type, err)
		}
		cur, err := q.Get(ctx, id)
		if err != nil {
			return PauseOverride{}, err
		}
		return PauseOverride{ID: id, OverridesPause: cur.OverridesPause}, nil
	}
}

// RaiseQueuedJobForPlayback raises the waiting job id for a playback (concept 14, 6.4): its
// priority to at least priority (never lower), override_pause by the cap of
// EnqueueOverridingPause (a job that carries it keeps it), override_window without a cap (concept
// 15, 4.3), and a job in backoff or delay runs now
// (run_after = min(run_after, now)). A running job stays as it is (no cancel, decision of 26
// October 2026); Raised is false then, and for a job that has ended. Only package library calls it
// (forbidigo).
func (q *Queue) RaiseQueuedJobForPlayback(ctx context.Context, id int64, priority int) (PauseOverride, error) {
	if priority < 0 || priority > 9 {
		return PauseOverride{}, fmt.Errorf("priority %d outside 0..9", priority)
	}
	override, err := q.q.RaiseQueuedJobForPlayback(ctx, systemq.RaiseQueuedJobForPlaybackParams{
		Priority: int64(priority), MaxOverrides: MaxPauseOverrides, Now: q.now().UnixMilli(), ID: id,
	})
	if errors.Is(err, sql.ErrNoRows) {
		cur, gerr := q.Get(ctx, id)
		if gerr != nil {
			return PauseOverride{}, gerr
		}
		return PauseOverride{ID: id, OverridesPause: cur.OverridesPause}, nil
	}
	if err != nil {
		return PauseOverride{}, fmt.Errorf("raise job %d for playback: %w", id, err)
	}
	return PauseOverride{ID: id, Raised: true, OverridesPause: override == 1}, nil
}

// EnqueueOverridingWindow is Enqueue for a job a playback asks for that may start outside the time
// window of its type (concept 15, 4.3: chapters.images after a playback). It carries
// override_window, not override_pause (a paused type stays paused, and MaxPauseOverrides does not
// count it). When an active job holds the key, that job is raised like RaiseQueuedPriority and
// gets override_window while it waits; a running job stays as it is. It returns the job's ID. Only
// package library calls it (forbidigo).
func (q *Queue) EnqueueOverridingWindow(ctx context.Context, j NewJob) (int64, error) {
	payload, maxAttempts, key, err := j.columns()
	if err != nil {
		return 0, err
	}
	now := q.now()
	id, err := q.q.InsertJobOverridingWindow(ctx, systemq.InsertJobOverridingWindowParams{
		Type:           j.Type,
		Priority:       int64(j.Priority),
		IdempotencyKey: key,
		Payload:        string(payload),
		RunAfter:       now.Add(j.Delay).UnixMilli(),
		MaxAttempts:    int64(maxAttempts),
		Now:            now.UnixMilli(),
	})
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) || !key.Valid {
		return 0, fmt.Errorf("enqueue %s: %w", j.Type, err)
	}
	// The insert was skipped because an active job holds the key.
	if id, err = q.q.GetActiveJobIDByKey(ctx, key); err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", j.Type, err)
	}
	if _, err := q.q.RaiseQueuedJobOverridingWindow(ctx, systemq.RaiseQueuedJobOverridingWindowParams{
		Priority: int64(j.Priority), Now: now.UnixMilli(), ID: id,
	}); err != nil {
		return 0, fmt.Errorf("raise job %d over the window: %w", id, err)
	}
	return id, nil
}

// Claim leases the next runnable job of one of types for workerID. It returns (nil, nil) when
// no job is runnable. Jobs whose lease expired after their last attempt are failed first; the
// other expired leases return to the queue and compete again by priority and age. A paused type
// and a type outside its time window are not claimed (see the package documentation).
func (q *Queue) Claim(ctx context.Context, workerID string, types []string) (*Job, error) {
	if len(types) == 0 {
		return nil, nil
	}
	now := q.now()
	if _, err := q.q.FailExhaustedLeases(ctx, now.UnixMilli()); err != nil {
		return nil, fmt.Errorf("fail exhausted leases: %w", err)
	}
	if _, err := q.q.RequeueExpiredLeases(ctx, now.UnixMilli()); err != nil {
		return nil, fmt.Errorf("requeue expired leases: %w", err)
	}
	row, err := q.q.ClaimJob(ctx, systemq.ClaimJobParams{
		WorkerID:   sql.NullString{String: workerID, Valid: true},
		LeaseUntil: sql.NullInt64{Int64: now.Add(LeaseDuration).UnixMilli(), Valid: true},
		Now:        now.UnixMilli(),
		Types:      types,
		Minute:     int64(minuteOfDay(now, q.Location())),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim: %w", err)
	}
	j := fromRow(row)
	return &j, nil
}

// Heartbeat extends the lease of a running job.
func (q *Queue) Heartbeat(ctx context.Context, workerID string, id int64) error {
	now := q.now()
	n, err := q.q.ExtendLease(ctx, systemq.ExtendLeaseParams{
		LeaseUntil: sql.NullInt64{Int64: now.Add(LeaseDuration).UnixMilli(), Valid: true},
		Now:        now.UnixMilli(),
		ID:         id,
		WorkerID:   sql.NullString{String: workerID, Valid: true},
	})
	return leaseResult("heartbeat", n, err)
}

// Complete marks a running job as done.
func (q *Queue) Complete(ctx context.Context, workerID string, id int64) error {
	n, err := q.q.CompleteJob(ctx, systemq.CompleteJobParams{
		Now: q.now().UnixMilli(), ID: id, WorkerID: sql.NullString{String: workerID, Valid: true},
	})
	return leaseResult("complete", n, err)
}

// Fail records a failed attempt of a running job. The job is retried with exponential backoff
// until it has used MaxAttempts; then it ends in state failed.
func (q *Queue) Fail(ctx context.Context, workerID string, j *Job, cause error) error {
	now := q.now()
	state, runAfter := StateFailed, now
	if j.Attempts < j.MaxAttempts {
		state, runAfter = StateQueued, now.Add(Backoff(j.Attempts))
	}
	n, err := q.q.FailJob(ctx, systemq.FailJobParams{
		State:     string(state),
		RunAfter:  runAfter.UnixMilli(),
		LastError: sql.NullString{String: cause.Error(), Valid: true},
		Now:       now.UnixMilli(),
		ID:        j.ID,
		WorkerID:  sql.NullString{String: workerID, Valid: true},
	})
	return leaseResult("fail", n, err)
}

// Release returns a running job to the queue without counting the attempt, for example when the
// worker shuts down while the handler runs.
func (q *Queue) Release(ctx context.Context, workerID string, id int64) error {
	n, err := q.q.ReleaseJob(ctx, systemq.ReleaseJobParams{
		Now: q.now().UnixMilli(), ID: id, WorkerID: sql.NullString{String: workerID, Valid: true},
	})
	return leaseResult("release", n, err)
}

// Defer returns a running job to the queue to run again after after, without counting the attempt
// (RetryLater).
func (q *Queue) Defer(ctx context.Context, workerID string, id int64, after time.Duration) error {
	now := q.now()
	n, err := q.q.DeferJob(ctx, systemq.DeferJobParams{
		RunAfter: now.Add(after).UnixMilli(), Now: now.UnixMilli(), ID: id, WorkerID: sql.NullString{String: workerID, Valid: true},
	})
	return leaseResult("defer", n, err)
}

// Cancel cancels a queued or running job, or dismisses a failed one. It reports whether a job was
// cancelled.
func (q *Queue) Cancel(ctx context.Context, id int64) (bool, error) {
	n, err := q.q.CancelJob(ctx, systemq.CancelJobParams{Now: q.now().UnixMilli(), ID: id})
	if err != nil {
		return false, fmt.Errorf("cancel job %d: %w", id, err)
	}
	return n == 1, nil
}

// SetRunningPayload replaces the payload of a running job, so its handler finds a note in its
// next run (a retry or a RetryLater keeps the payload). It reports false when the job does not
// run (any more).
func (q *Queue) SetRunningPayload(ctx context.Context, id int64, payload json.RawMessage) (bool, error) {
	if !json.Valid(payload) {
		return false, errors.New("payload is not valid JSON")
	}
	n, err := q.q.SetRunningJobPayload(ctx, systemq.SetRunningJobPayloadParams{Payload: string(payload), Now: q.now().UnixMilli(), ID: id})
	if err != nil {
		return false, fmt.Errorf("set payload of job %d: %w", id, err)
	}
	return n == 1, nil
}

// SetQueuedPayload replaces the payload of a queued job, for handlers that coalesce work into
// one pending job (scan.path merges directories). It reports false when the job is no longer
// queued: a worker claimed it, or it ended.
func (q *Queue) SetQueuedPayload(ctx context.Context, id int64, payload json.RawMessage) (bool, error) {
	if !json.Valid(payload) {
		return false, errors.New("payload is not valid JSON")
	}
	n, err := q.q.SetQueuedJobPayload(ctx, systemq.SetQueuedJobPayloadParams{Payload: string(payload), Now: q.now().UnixMilli(), ID: id})
	if err != nil {
		return false, fmt.Errorf("set payload of job %d: %w", id, err)
	}
	return n == 1, nil
}

// RaiseQueuedPriority raises the priority of a job that no worker has claimed yet to at least
// priority (0..9) and reports whether the job was still queued. It never lowers a priority: a
// manual request overtakes a scheduled job that holds the same idempotency key, instead of being
// swallowed by it (Enqueue returns the waiting job on a key conflict).
func (q *Queue) RaiseQueuedPriority(ctx context.Context, id int64, priority int) (bool, error) {
	if priority < 0 || priority > 9 {
		return false, fmt.Errorf("priority %d outside 0..9", priority)
	}
	n, err := q.q.RaiseQueuedJobPriority(ctx, systemq.RaiseQueuedJobPriorityParams{Priority: int64(priority), Now: q.now().UnixMilli(), ID: id})
	if err != nil {
		return false, fmt.Errorf("raise priority of job %d: %w", id, err)
	}
	return n == 1, nil
}

// PullQueuedForward lets a job that no worker has claimed yet and that waits for a later start
// (Delay) run now, keeping its priority and attempts, and reports whether it did. A claimed,
// finished or already due job stays as it is.
func (q *Queue) PullQueuedForward(ctx context.Context, id int64) (bool, error) {
	n, err := q.q.PullQueuedJobForward(ctx, systemq.PullQueuedJobForwardParams{Now: q.now().UnixMilli(), ID: id})
	if err != nil {
		return false, fmt.Errorf("pull job %d forward: %w", id, err)
	}
	return n == 1, nil
}

// ListActive returns the queued and running jobs of typ, oldest first.
func (q *Queue) ListActive(ctx context.Context, typ string) ([]Job, error) {
	rows, err := q.q.ListActiveJobsByType(ctx, typ)
	if err != nil {
		return nil, fmt.Errorf("list active jobs of %s: %w", typ, err)
	}
	out := make([]Job, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, nil
}

// PausedTypes returns the paused job types, sorted.
func (q *Queue) PausedTypes(ctx context.Context) ([]string, error) {
	types, err := q.q.ListPausedJobTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list paused job types: %w", err)
	}
	return types, nil
}

// TypePaused reports whether typ is paused; a type without a row is not.
func (q *Queue) TypePaused(ctx context.Context, typ string) (bool, error) {
	row, err := q.q.GetJobType(ctx, typ)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get job type %s: %w", typ, err)
	}
	return row.PausedAt.Valid, nil
}

// Get returns a job by ID.
func (q *Queue) Get(ctx context.Context, id int64) (Job, error) {
	row, err := q.q.GetJob(ctx, id)
	if err != nil {
		return Job{}, fmt.Errorf("get job %d: %w", id, err)
	}
	return fromRow(row), nil
}

// Backoff returns the wait before the next attempt after attempt number attempt (1-based).
func Backoff(attempt int) time.Duration {
	return RetryBase << min(max(attempt-1, 0), maxBackoffShift)
}

func leaseResult(op string, rows int64, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if rows == 0 {
		return ErrLeaseLost
	}
	return nil
}

// FromRow maps a row of the table jobs (package admin reads the page's lists itself).
func FromRow(r systemq.Job) Job { return fromRow(r) }

func fromRow(r systemq.Job) Job {
	return Job{
		ID:              r.ID,
		Type:            r.Type,
		Priority:        int(r.Priority),
		IdempotencyKey:  r.IdempotencyKey.String,
		Payload:         json.RawMessage(r.Payload),
		State:           State(r.State),
		RunAfter:        time.UnixMilli(r.RunAfter),
		LeaseUntil:      time.UnixMilli(r.LeaseUntil.Int64),
		Attempts:        int(r.Attempts),
		MaxAttempts:     int(r.MaxAttempts),
		LastError:       r.LastError.String,
		CreatedAt:       time.UnixMilli(r.CreatedAt),
		UpdatedAt:       time.UnixMilli(r.UpdatedAt),
		StartedAt:       msTime(r.StartedAt),
		FinishedAt:      msTime(r.FinishedAt),
		Progress:        RunProgress{Done: r.ProgressDone.Int64, Total: r.ProgressTotal.Int64, Note: r.ProgressNote.String},
		Skipped:         r.Skipped == 1,
		OverridesPause:  r.OverridePause == 1,
		OverridesWindow: r.OverrideWindow == 1,
	}
}

// msTime converts a nullable Unix-millisecond column; NULL is the zero time.
func msTime(v sql.NullInt64) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return time.UnixMilli(v.Int64)
}
