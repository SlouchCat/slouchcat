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
