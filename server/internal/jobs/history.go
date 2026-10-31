package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// History of the runs (concept 09, 4.4 and 6.2; F-8): done and cancelled runs stay HistoryAge,
// at least the newest HistoryKeep per type, and the daily job jobs.prune removes the rest in
// batches. Failed runs stay until the administrator retries or dismisses them. The median
// duration of a type over its newest HistoryKeep runs that did work is the base of every estimate.

// Bounds of the history.
const (
	// JobPrune is the housekeeping job of the queue, a schedule type of the group system.
	JobPrune      = "jobs.prune"
	HistoryAge    = 30 * 24 * time.Hour
	HistoryKeep   = 50
	PruneBatch    = 500
	PruneInterval = 24 * time.Hour
	prunePriority = 2
	// MedianMinRuns is how many runs with a duration a type needs before its median counts.
	MedianMinRuns = 5
)

// MedianDuration returns the median duration of the newest HistoryKeep done runs of typ that
// did work (skipped = 0) and how many runs it rests on; a median with fewer than MedianMinRuns
// runs is reported but not known (ok = false).
func (q *Queue) MedianDuration(ctx context.Context, typ string) (median time.Duration, runs int, ok bool, err error) {
	rows, err := q.q.ListJobDurations(ctx, systemq.ListJobDurationsParams{Type: typ, LimitCount: HistoryKeep})
	if err != nil {
		return 0, 0, false, fmt.Errorf("list durations of %s: %w", typ, err)
	}
	if len(rows) == 0 {
		return 0, 0, false, nil
	}
	ms := make([]int64, 0, len(rows))
	for _, r := range rows {
		ms = append(ms, max(r, 0))
	}
	slices.Sort(ms)
	mid := len(ms) / 2
	m := ms[mid]
	if len(ms)%2 == 0 {
		m = (ms[mid-1] + ms[mid]) / 2
	}
	return time.Duration(m) * time.Millisecond, len(ms), len(ms) >= MedianMinRuns, nil
}

// Prune removes the done and cancelled runs older than HistoryAge, except the newest HistoryKeep
// of every type, in batches of PruneBatch rows, and returns how many rows it deleted. It checks
// ctx between batches.
func (q *Queue) Prune(ctx context.Context) (int64, error) {
	types, err := q.q.ListHistoryJobTypes(ctx)
	if err != nil {
		return 0, fmt.Errorf("list job types with history: %w", err)
	}
	before := sql.NullInt64{Int64: q.now().Add(-HistoryAge).UnixMilli(), Valid: true}
	var total int64
	for _, typ := range types {
		for {
			if err := ctx.Err(); err != nil {
				return total, err
			}
			n, err := q.q.PruneJobs(ctx, systemq.PruneJobsParams{Type: typ, Before: before, Keep: HistoryKeep, Batch: PruneBatch})
			if err != nil {
				return total, fmt.Errorf("prune %s: %w", typ, err)
			}
			total += n
			if n < PruneBatch {
				break
			}
		}
	}
	return total, nil
}

// pruneType is the job type jobs.prune, registered by NewRegistry: daily, no firing at the
// start (its first row is due at once after the migration and cleans up the backlog).
func pruneType(q *Queue) Type {
	return Type{
		Name: JobPrune, Group: GroupSystem, Pool: PoolGeneral, Trigger: TriggerSchedule, Priority: prunePriority,
		Handler: func(ctx context.Context, j *Job) error {
			n, err := q.Prune(ctx)
			if err != nil {
				return err
			}
			if n == 0 {
				Skip(ctx)
			}
			return nil
		},
		Run: func(ctx context.Context, q *Queue, prio int) ([]int64, error) {
			id, err := q.Enqueue(ctx, NewJob{Type: JobPrune, Priority: prio, IdempotencyKey: JobPrune})
			if err != nil {
				return nil, err
			}
			if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
				return nil, err
			}
			return []int64{id}, nil
		},
		Every: PruneInterval,
	}
}
