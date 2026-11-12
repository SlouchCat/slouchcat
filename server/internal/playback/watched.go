package playback

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// SetWatched sets or removes the check mark «gesehen» of a visible title by hand (setItemWatched,
// concept 20 section 6.4). The titles come from visibility.WatchTargets: the title itself, or every
// visible episode of a season, series or podcast; an episode the profile may not see stays as it
// is. All of them change in one transaction.
//
//   - watched: completed, the position at the end of the stored duration (a new row takes the
//     target's duration, 0 when none is known), watched_at and updated_at now. Over several
//     episodes the times are staggered by one millisecond in episode order, the last episode the
//     newest: continue watching and the offline rule "next episode" pick up after the last one
//     instead of an arbitrary one of equal time.
//   - not watched: watched_at goes; a finished title starts again at 0, a running position (a
//     second run) stays, updated_at stays.
//
// The check mark by hand writes no activity event (O-2): the log shows real playbacks. An
// invisible or unknown title is model.ErrNotFound.
func (s *Service) SetWatched(ctx context.Context, profile model.Profile, itemID string, watched bool) error {
	targets, err := s.vis.WatchTargets(ctx, visibility.Profile(profile), itemID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin watched: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	for i, t := range targets {
		if watched {
			err = q.SetProgressWatched(ctx, appq.SetProgressWatchedParams{
				ProfileID: profile.ID, ItemID: t.ItemID, DurationMs: t.DurationMs, At: now - int64(len(targets)-1-i),
			})
		} else {
			err = q.ClearProgressWatched(ctx, appq.ClearProgressWatchedParams{ProfileID: profile.ID, ItemID: t.ItemID})
		}
		if err != nil {
			return fmt.Errorf("set watched %s: %w", t.ItemID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit watched: %w", err)
	}
	return nil
}
