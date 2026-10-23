package playback

import (
	"context"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Condensation of old playback events (plan 3.2, M10, migration 00019). progress_events keeps the
// raw rows of the last ProgressRetention; everything older becomes one row per profile, title,
// device and UTC day in progress_days, and the raw rows of that day are deleted in the same
// transaction. Without it the table grows without bound. The usage statistics (A-8,
// queries/system/admin_stats.sql) read the raw events and the condensed days and add them up, so
// their numbers do not change when a day is condensed.
//
// The sweep (sessions.go, every minute) calls CondenseProgress. It finds work once a day, when a
// new UTC day falls out of the retention window; the check itself is one indexed MIN. One whole day
// is condensed per transaction, so the single writer (plan 3.6) is never held for long, a run that
// stops in the middle leaves whole days behind, and a second run condenses nothing twice: a day's
// raw rows are gone when its day row exists.

const (
	// ProgressRetention is how long the raw rows of progress_events are kept before they are
	// condensed into progress_days (plan 3.2).
	ProgressRetention = 90 * 24 * time.Hour
	// dayMs is one UTC day in milliseconds; the day number of an event is server_time / dayMs.
	dayMs = int64(24 * time.Hour / time.Millisecond)
)

// CondenseProgress condenses every raw progress event older than ProgressRetention into
// progress_days, one whole UTC day per transaction, and returns the number of raw rows removed.
// It is idempotent: a second run over the same data finds nothing left and changes nothing.
func (s *Service) CondenseProgress(ctx context.Context, now time.Time) (int64, error) {
	// Only whole days are condensed: the boundary moves back to the start of the UTC day that
	// contains now-ProgressRetention, so a day is never half condensed and half left behind.
	before := (now.Add(-ProgressRetention).UnixMilli() / dayMs) * dayMs
	var removed int64
	for {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		day, err := s.read.OldestCondensableDay(ctx, before)
		if err != nil {
			return removed, fmt.Errorf("oldest condensable day: %w", err)
		}
		if day < 0 { // nothing older than the boundary
			return removed, nil
		}
		n, err := s.condenseDay(ctx, day)
		if err != nil {
			return removed, err
		}
		removed += n
	}
}

// condenseDay adds one UTC day to progress_days and deletes its raw events, in one transaction.
func (s *Service) condenseDay(ctx context.Context, day int64) (int64, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin condense of day %d: %w", day, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	if err := q.CondenseProgressDay(ctx, day); err != nil {
		return 0, fmt.Errorf("condense day %d: %w", day, err)
	}
	n, err := q.DeleteCondensedProgressEvents(ctx, day)
	if err != nil {
		return 0, fmt.Errorf("delete the condensed events of day %d: %w", day, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit the condensed day %d: %w", day, err)
	}
	return n, nil
}

// sweepCondense is the sweep's step: it condenses and logs, errors included (the sweep goes on).
func (s *Service) sweepCondense(ctx context.Context, now time.Time) {
	n, err := s.CondenseProgress(ctx, now)
	if err != nil && ctx.Err() == nil {
		s.log.WarnContext(ctx, "condense progress events", "err", err)
	}
	if n > 0 {
		s.log.InfoContext(ctx, "progress events condensed", "events", n, "older_than", ProgressRetention.String())
	}
}
