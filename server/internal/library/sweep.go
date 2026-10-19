package library

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// handleSweep soft-deletes titles whose files have all been missing for more than 24 h (plan
// 3.2), and then the seasons and series left without undeleted children. User data stays
// attached to the item; a file that reappears restores it with its season and series.
func (s *Service) handleSweep(ctx context.Context, _ *jobs.Job) error {
	n, err := s.Sweep(ctx)
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "library sweep done", "items_soft_deleted", n)
	return nil
}

// Sweep runs the soft delete once, in one transaction, and returns the number of items deleted:
// movies, episodes and extras whose files are all missing for longer than the grace period, then
// seasons without undeleted children (episodes, extras), then series without undeleted children
// (seasons, extras).
func (s *Service) Sweep(ctx context.Context) (int64, error) {
	now := s.now()
	ms := sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	n, err := q.SoftDeleteItemsMissingSince(ctx, systemq.SoftDeleteItemsMissingSinceParams{
		Now: ms, Before: sql.NullInt64{Int64: now.Add(-missingGrace).UnixMilli(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("soft delete missing items: %w", err)
	}
	seasons, err := q.SoftDeleteEmptySeasons(ctx, ms)
	if err != nil {
		return 0, fmt.Errorf("soft delete empty seasons: %w", err)
	}
	series, err := q.SoftDeleteEmptySeries(ctx, ms)
	if err != nil {
		return 0, fmt.Errorf("soft delete empty series: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sweep: %w", err)
	}
	return n + seasons + series, nil
}
