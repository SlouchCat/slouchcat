package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Job administration of the dashboard (M8, A-3): counters, the newest failed jobs and their retry.
// Cancel is the queue's own.

// Stats counts the queued, running and failed jobs; done and cancelled ones are history.
func (q *Queue) Stats(ctx context.Context) (queued, running, failed int, err error) {
	row, err := q.q.JobCountsByState(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("count jobs: %w", err)
	}
	return int(row.Queued), int(row.Running), int(row.Failed), nil
}

// ListFailed returns the newest failed jobs, at most limit, with their last error.
func (q *Queue) ListFailed(ctx context.Context, limit int) ([]Job, error) {
	rows, err := q.q.ListFailedJobs(ctx, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("list failed jobs: %w", err)
	}
	out := make([]Job, 0, len(rows))
	for _, r := range rows {
		out = append(out, Job{
			ID: r.ID, Type: r.Type, State: State(r.State), Attempts: int(r.Attempts), MaxAttempts: int(r.MaxAttempts),
			LastError: r.LastError.String, CreatedAt: time.UnixMilli(r.CreatedAt), UpdatedAt: time.UnixMilli(r.UpdatedAt),
		})
	}
	return out, nil
}

// Retry queues a failed job again at once with its attempts reset and returns it. An unknown job
// is model.ErrNotFound, one that has not failed model.ErrJobNotFailed. When an active job holds
// the same idempotency key, the work is queued already: the failed job stays and the active one
// is returned.
func (q *Queue) Retry(ctx context.Context, id int64) (Job, error) {
	n, err := q.q.RetryFailedJob(ctx, systemq.RetryFailedJobParams{Now: q.now().UnixMilli(), ID: id})
	if err != nil {
		return Job{}, fmt.Errorf("retry job %d: %w", id, err)
	}
	row, err := q.q.GetJob(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, model.ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("get job %d: %w", id, err)
	}
	j := fromRow(row)
	switch {
	case n == 1:
		return j, nil
	case j.State != StateFailed:
		return Job{}, model.ErrJobNotFailed
	}
	active, err := q.q.GetActiveJobIDByKey(ctx, row.IdempotencyKey)
	if err != nil {
		return Job{}, fmt.Errorf("active job of job %d: %w", id, err)
	}
	return q.Get(ctx, active)
}

// FailedRetry is the outcome of RetryFailed: Retried runs are queued again, Skipped stay failed
// (an active job holds their key), Merged ended cancelled (an older failure of a key whose newest
// failed run was queued).
type FailedRetry struct{ Retried, Skipped, Merged int }

// failedScope is the type filter of the bulk actions: nil for every type.
func failedScope(typ string) any {
	if typ == "" {
		return nil
	}
	return typ
}

// RetryFailed queues every failed job of typ ("" for every type) again with its attempts reset,
// in one transaction (retryFailedJobs, concept 20 P-6). The rule of Retry holds: a job whose key
// an active job holds stays failed. Of several failed jobs with the same key only the newest is
// queued and the older ones are cancelled; one UPDATE over all of them would otherwise break the
// unique index of active keys and fail as a whole.
func (q *Queue) RetryFailed(ctx context.Context, typ string) (FailedRetry, error) {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return FailedRetry{}, fmt.Errorf("retry failed jobs: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	w := q.q.WithTx(tx)
	now, scope := q.now().UnixMilli(), failedScope(typ)
	merged, err := w.MergeFailedJobs(ctx, systemq.MergeFailedJobsParams{Now: now, Type: scope})
	if err != nil {
		return FailedRetry{}, fmt.Errorf("merge failed jobs: %w", err)
	}
	retried, err := w.RetryFailedJobs(ctx, systemq.RetryFailedJobsParams{Now: now, Type: scope})
	if err != nil {
		return FailedRetry{}, fmt.Errorf("retry failed jobs: %w", err)
	}
	skipped, err := w.CountFailedJobs(ctx, scope)
	if err != nil {
		return FailedRetry{}, fmt.Errorf("count failed jobs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return FailedRetry{}, fmt.Errorf("retry failed jobs: commit: %w", err)
	}
	return FailedRetry{Retried: int(retried), Skipped: int(skipped), Merged: int(merged)}, nil
}

// DismissFailed cancels every failed job of typ ("" for every type) in one statement
// (dismissFailedJobs, concept 20 P-6) and returns how many; jobs.prune removes them later.
func (q *Queue) DismissFailed(ctx context.Context, typ string) (int, error) {
	n, err := q.q.DismissFailedJobs(ctx, systemq.DismissFailedJobsParams{Now: q.now().UnixMilli(), Type: failedScope(typ)})
	if err != nil {
		return 0, fmt.Errorf("dismiss failed jobs: %w", err)
	}
	return int(n), nil
}
