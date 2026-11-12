package playback

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// SetWatched sets or removes the check mark «gesehen» of a visible title by hand (setItemWatched,
// concept 20 section 6.4). The titles come from visibility.WatchTargets: the title itself, or every
// visible episode of a season, series or podcast; an episode the profile may not see stays as it
// is. All of them change in one transaction. The targets are read before it, on the read
// connection of package visibility (rule 2); a playback event or a scan in between is accepted
// (section 6.4): at worst one episode keeps the state of that event.
//
//   - watched: completed, the position at the end of the stored duration (a new row takes the
//     target's duration, 0 when none is known) and watched_at now. Over several episodes the
//     times are staggered by one millisecond in episode order, the last episode the newest:
//     continue watching and the offline rule "next episode" pick up after the last one instead of
//     an arbitrary one of equal time. A title that is already watched stays as it is, its
//     watched_at and updated_at included (decision of 12 November 2026): marking a series keeps
//     the dates of the episodes seen before, only the newly marked ones get a time.
//   - updated_at of a newly marked title is its watched_at, except for an episode behind the
//     newest started or finished episode of its series (Behind, decision of 12 November 2026):
//     it gets a time below that episode's (NewestAt) and below its watched_at, a stored row keeps
//     its own when that is older. Continue watching (ListContinueWatching, nextUp) and the
//     offline rule "next episode" (ListRecentEpisodeProgress) read the newest row of a series, so
//     checking S01E03 while watching season 4 leaves both on season 4. An episode at or after
//     the newest one becomes the newest itself, and both readers move on after it.
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
			at := now - int64(len(targets)-1-i)
			updated := at
			if t.Behind {
				updated = min(t.NewestAt, at) - 1
			}
			err = q.SetProgressWatched(ctx, appq.SetProgressWatchedParams{
				ProfileID: profile.ID, ItemID: t.ItemID, DurationMs: t.DurationMs, UpdatedAt: updated,
				At: sql.NullInt64{Int64: at, Valid: true},
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
