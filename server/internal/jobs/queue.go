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
//   - Priority 0 to 9, higher runs first; within a priority the oldest job runs first.
//   - Idempotency: an IdempotencyKey is unique among queued and running jobs. Enqueueing the same
//     key again returns the ID of the active job instead of adding a duplicate. After the job has
//     finished, the key can be used again.
//   - Debounce: set Delay to DebounceWindow together with a key. The first enqueue waits for the
//     window, every enqueue of the same key inside it coalesces into that job. A caller that
//     needs the work of both enqueues merges it into the pending job with SetQueuedPayload.
//   - Retry: a failed attempt is retried after 10 s · 2^(attempt-1) (10 s, 20 s, 40 s, …) until
//     MaxAttempts is reached; then the job ends in state failed with the last error.
//   - Cancel: queued and running jobs can be cancelled; a running handler sees its context
//     cancelled at the next heartbeat.
//
// The table job_schedules holds cron-like schedules; the scheduler that fills the queue from it
// is not part of this package yet.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
)

// ErrLeaseLost is returned when the job is no longer running under the caller's lease: it was
// cancelled, completed, or its lease expired and another worker claimed it.
var ErrLeaseLost = errors.New("job lease lost")

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
}

// Queue is the persistent queue on the writer connection of the database.
type Queue struct {
	db  *sql.DB
	q   *systemq.Queries
	now func() time.Time
}

// NewQueue returns a queue on db. now is the clock; nil means time.Now.
func NewQueue(db *sql.DB, now func() time.Time) *Queue {
	if now == nil {
		now = time.Now
	}
	return &Queue{db: db, q: systemq.New(db), now: now}
}

// Enqueue adds a job and returns its ID. With an IdempotencyKey that an active job already
// holds, it returns the ID of that job instead.
func (q *Queue) Enqueue(ctx context.Context, j NewJob) (int64, error) {
	if j.Type == "" {
		return 0, errors.New("job type must not be empty")
	}
	if j.Priority < 0 || j.Priority > 9 {
		return 0, fmt.Errorf("priority %d outside 0..9", j.Priority)
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
		return 0, errors.New("payload is not valid JSON")
	}
	key := sql.NullString{String: j.IdempotencyKey, Valid: j.IdempotencyKey != ""}
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

// Claim leases the next runnable job of one of types for workerID. It returns (nil, nil) when
// no job is runnable. Jobs whose lease expired after their last attempt are failed first; the
// other expired leases return to the queue and compete again by priority and age.
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

// Cancel cancels a queued or running job, or dismisses a failed one. It reports whether a job was
// cancelled.
func (q *Queue) Cancel(ctx context.Context, id int64) (bool, error) {
	n, err := q.q.CancelJob(ctx, systemq.CancelJobParams{Now: q.now().UnixMilli(), ID: id})
	if err != nil {
		return false, fmt.Errorf("cancel job %d: %w", id, err)
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

func fromRow(r systemq.Job) Job {
	return Job{
		ID:             r.ID,
		Type:           r.Type,
		Priority:       int(r.Priority),
		IdempotencyKey: r.IdempotencyKey.String,
		Payload:        json.RawMessage(r.Payload),
		State:          State(r.State),
		RunAfter:       time.UnixMilli(r.RunAfter),
		LeaseUntil:     time.UnixMilli(r.LeaseUntil.Int64),
		Attempts:       int(r.Attempts),
		MaxAttempts:    int(r.MaxAttempts),
		LastError:      r.LastError.String,
		CreatedAt:      time.UnixMilli(r.CreatedAt),
		UpdatedAt:      time.UnixMilli(r.UpdatedAt),
	}
}
