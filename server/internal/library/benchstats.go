package library

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// JobStats counts the jobs created since a point in time (scan benchmark, plan M3 step 5).
type JobStats struct {
	Total         int // jobs created
	Done          int // finished successfully
	Active        int // still queued or running
	Repeated      int // attempts > 1: retried, or claimed again after a lost lease
	LeaseExpired  int // failed because the lease expired after the last attempt
	DuplicateKeys int // probe.file keys held by more than one job (a file probed twice)
}

// JobStats returns the job counts since since.
func (s *Service) JobStats(ctx context.Context, since time.Time) (JobStats, error) {
	q := systemq.New(s.db.Read)
	row, err := q.JobStatsSince(ctx, since.UnixMilli())
	if err != nil {
		return JobStats{}, fmt.Errorf("job stats: %w", err)
	}
	dup, err := q.CountDuplicateJobKeys(ctx, systemq.CountDuplicateJobKeysParams{
		Since: since.UnixMilli(), Prefix: sql.NullString{String: "probe:%", Valid: true},
	})
	if err != nil {
		return JobStats{}, fmt.Errorf("duplicate jobs: %w", err)
	}
	return JobStats{
		Total: int(row.Total), Done: int(row.Done), Active: int(row.Active), Repeated: int(row.Repeated),
		LeaseExpired: int(row.LeaseExpired), DuplicateKeys: int(dup),
	}, nil
}

// LibraryContent is the content of a library by item kind, with the media files whose probe
// failed.
type LibraryContent struct {
	Items       map[string]int // undeleted items by kind
	ProbeErrors int
}

// Content returns the content of a library.
func (s *Service) Content(ctx context.Context, libraryID int64) (LibraryContent, error) {
	q := systemq.New(s.db.Read)
	rows, err := q.CountLibraryItemsByKind(ctx, libraryID)
	if err != nil {
		return LibraryContent{}, fmt.Errorf("count items: %w", err)
	}
	c := LibraryContent{Items: make(map[string]int, len(rows))}
	for _, r := range rows {
		c.Items[r.Kind] = int(r.N)
	}
	n, err := q.CountLibraryProbeErrors(ctx, libraryID)
	if err != nil {
		return LibraryContent{}, fmt.Errorf("count probe errors: %w", err)
	}
	c.ProbeErrors = int(n)
	return c, nil
}
