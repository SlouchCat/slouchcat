package visibility

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Reads of M5 (offline playback, plan 3.3 and M5 design 3.3): which profiles of an account may
// see a title (stored with every download, the client only looks it up), the titles of a device's
// downloads for the active profile and the visible episodes of a season (season download, O-2).

// allowedBatch caps the item ids of one AllowedProfiles query.
const allowedBatch = 500

// AllowedProfiles returns, per title, the ids of the account's profiles that may see it, in
// ascending order; a title no profile sees has no entry. It reads across all profiles of the
// account and therefore accepts only the System scope (ErrNotSystem otherwise); only package
// offline calls it, to store downloads.allowed_profile_ids.
func (r *Reader) AllowedProfiles(ctx context.Context, s Scope, accountID int64, itemIDs []string) (map[string][]int64, error) {
	if !s.system {
		return nil, ErrNotSystem
	}
	r.log.DebugContext(ctx, "system scope read", "reason", s.reason, "items", len(itemIDs))
	out := map[string][]int64{}
	ids := slices.Compact(slices.Sorted(slices.Values(itemIDs)))
	for chunk := range slices.Chunk(ids, allowedBatch) {
		rows, err := r.q.ListAllowedProfiles(ctx, scopedq.ListAllowedProfilesParams{AccountID: accountID, ItemIds: chunk})
		if err != nil {
			return nil, fmt.Errorf("list allowed profiles: %w", err)
		}
		for _, row := range rows {
			out[row.ItemID] = append(out[row.ItemID], row.ProfileID)
		}
	}
	for id := range out {
		slices.Sort(out[id])
	}
	return out, nil
}

// Summaries returns the visible titles among itemIDs as they appear in lists, by id; invisible
// and unknown ids are absent.
func (r *Reader) Summaries(ctx context.Context, s Scope, itemIDs []string) (map[string]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	out := map[string]model.ItemSummary{}
	ids := slices.Compact(slices.Sorted(slices.Values(itemIDs)))
	for chunk := range slices.Chunk(ids, allowedBatch) {
		items, err := r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: chunk})
		if err != nil {
			return nil, fmt.Errorf("list visible items: %w", err)
		}
		list, err := r.summaries(ctx, s, pid, items)
		if err != nil {
			return nil, err
		}
		for _, it := range list {
			out[it.ID] = it
		}
	}
	return out, nil
}

// Episodes returns the visible episodes of a visible season in episode order. An invisible or
// unknown season and a title that is not a season return model.ErrNotFound.
func (r *Reader) Episodes(ctx context.Context, s Scope, seasonID string) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	season, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: seasonID})
	if err != nil {
		return nil, notFound(err)
	}
	if season.Kind != "season" {
		return nil, model.ErrNotFound
	}
	items, err := r.q.ListVisibleEpisodes(ctx, scopedq.ListVisibleEpisodesParams{ProfileID: pid, SeasonID: sql.NullString{String: seasonID, Valid: true}})
	if err != nil {
		return nil, fmt.Errorf("list visible episodes: %w", err)
	}
	return r.summaries(ctx, s, pid, items)
}
