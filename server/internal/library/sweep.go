package library

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// handleSweep soft-deletes titles whose files have all been missing for more than 24 h (plan
// 3.2). User data stays attached to the item; a file that reappears restores it.
func (s *Service) handleSweep(ctx context.Context, _ *jobs.Job) error {
	n, err := s.Sweep(ctx)
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "library sweep done", "items_soft_deleted", n)
	return nil
}

// Sweep runs the soft delete once and returns the number of items deleted.
func (s *Service) Sweep(ctx context.Context) (int64, error) {
	now := s.now()
	n, err := systemq.New(s.db.Write).SoftDeleteItemsMissingSince(ctx, systemq.SoftDeleteItemsMissingSinceParams{
		Now:    sql.NullInt64{Int64: now.UnixMilli(), Valid: true},
		Before: sql.NullInt64{Int64: now.Add(-missingGrace).UnixMilli(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("soft delete missing items: %w", err)
	}
	return n, nil
}
