package visibility

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Reads of the check mark «gesehen» (concept 20, P-5 and P-14, queries/scoped/watched.sql).

// WatchTargets returns the titles the check mark of a visible title sets or removes (concept 20
// section 6.4), in episode order: the title itself for a film, episode, extra, audiobook or
// podcast episode; the visible episodes of a season, of every season of a series (specials first)
// or of a podcast (oldest first). Episodes the profile may not see are never among them. Each
// carries the duration a missing progress row starts with, and an episode of a series whether it
// comes before the newest started or finished visible episode of that series (Behind, with that
// row's updated_at in NewestAt). An invisible or unknown title is
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
		out = append(out, model.WatchTarget{ItemID: row.ID, DurationMs: row.DurationMs, Behind: row.Behind == 1, NewestAt: row.NewestAt})
	}
	return out, nil
}

// MaxWatchedPage caps a page of listWatched (concept 20 section 15.4).
const MaxWatchedPage = 100

// watchedContents are the contents of listWatched (contract RootContent): the kinds of its entries.
var watchedContents = []string{"movie", "series", "audiobook", "podcast"}

// Watched returns one page of the page «Gesehen» (listWatched, concept 20 section 15.4) and the
// size of the whole list: every film and audiobook with watched_at, every series and podcast with
// at least one visible watched episode, once each; radio, extras, seasons and episodes never.
// Titles the profile may not see are left out and only visible episodes count. Sort watched (the
// default) puts the newest last watched first, sort title orders by the sort key of the text chain
// (metadata language, interface language, file name); ties by the other order, then the id. limit
// is clamped to 1..MaxWatchedPage (0 = DefaultPage), offset to 0 and above. A content or sort
// outside its values is a *model.ValidationError (invalid_value).
func (r *Reader) Watched(ctx context.Context, s Scope, q model.WatchedQuery) (model.WatchedPage, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.WatchedPage{}, err
	}
	var verr *model.ValidationError
	if q.Content != "" && !slices.Contains(watchedContents, q.Content) {
		verr = verr.Add("content", "invalid_value")
	}
	if q.Sort != "" && q.Sort != model.WatchedSortWatched && q.Sort != model.WatchedSortTitle {
		verr = verr.Add("sort", "invalid_value")
	}
	if verr != nil {
		return model.WatchedPage{}, verr
	}
	offset, limit := max(q.Offset, 0), q.Limit
	if limit <= 0 {
		limit = DefaultPage
	}
	limit = min(limit, MaxWatchedPage)
	byTitle := int64(0)
	if q.Sort == model.WatchedSortTitle {
		byTitle = 1
	}
	rows, err := r.q.ListWatched(ctx, scopedq.ListWatchedParams{
		ProfileID: pid, Kind: q.Content, ByTitle: byTitle, MetaLang: s.metaLang, UiLang: s.language,
		LimitCount: int64(limit), OffsetCount: int64(offset),
	})
	if err != nil {
		return model.WatchedPage{}, fmt.Errorf("list watched: %w", err)
	}
	out := model.WatchedPage{Items: make([]model.WatchedEntry, 0, len(rows))}
	if len(rows) == 0 {
		if offset > 0 { // a page beyond the end carries no total
			n, err := r.q.CountWatched(ctx, scopedq.CountWatchedParams{ProfileID: pid, Kind: q.Content})
			if err != nil {
				return model.WatchedPage{}, fmt.Errorf("count watched: %w", err)
			}
			out.Total = int(n)
		}
		return out, nil
	}
	out.Total = int(rows[0].Total)
	ids := make([]string, 0, len(rows))
	var containers []string
	for _, row := range rows {
		ids = append(ids, row.ID)
		if row.Kind == "series" || row.Kind == "podcast" {
			containers = append(containers, row.ID)
		}
	}
	episodes := map[string]int{}
	if len(containers) > 0 {
		counts, err := r.q.ListVisibleEpisodeCounts(ctx, scopedq.ListVisibleEpisodeCountsParams{ProfileID: pid, ItemIds: containers})
		if err != nil {
			return model.WatchedPage{}, fmt.Errorf("count episodes: %w", err)
		}
		for _, c := range counts {
			episodes[c.ID] = int(c.EpisodeCount)
		}
	}
	items, err := r.ordered(ctx, s, pid, ids)
	if err != nil {
		return model.WatchedPage{}, err
	}
	at := make(map[string]int64, len(rows))
	for _, row := range rows {
		at[row.ID] = row.LastWatchedAt
	}
	for _, it := range items { // ordered keeps the order and drops a title that went invisible since
		e := model.WatchedEntry{Item: it, LastWatchedAt: time.UnixMilli(at[it.ID]).UTC()}
		if n, ok := episodes[it.ID]; ok {
			e.EpisodeCount = &n
		}
		out.Items = append(out.Items, e)
	}
	return out, nil
}
