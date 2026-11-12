package visibility

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Reads of the check mark «gesehen» (concept 20, P-5 and P-14, queries/scoped/watched.sql).

// WatchTargets returns the titles the check mark of a visible title sets or removes (concept 20
// section 6.4), in episode order: the title itself for a film, episode, extra, audiobook or
// podcast episode; the visible episodes of a season, of every season of a series (specials first)
// or of a podcast (oldest first). Episodes the profile may not see are never among them. Each
// carries the duration a missing progress row starts with. An invisible or unknown title is
// model.ErrNotFound; a series without a visible episode returns none.
func (r *Reader) WatchTargets(ctx context.Context, s Scope, itemID string) ([]model.WatchTarget, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	if _, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID}); err != nil {
		return nil, notFound(err)
	}
	rows, err := r.q.ListVisibleWatchTargets(ctx, scopedq.ListVisibleWatchTargetsParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return nil, fmt.Errorf("list watch targets: %w", err)
	}
	out := make([]model.WatchTarget, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.WatchTarget{ItemID: row.ID, DurationMs: row.DurationMs})
	}
	return out, nil
}
