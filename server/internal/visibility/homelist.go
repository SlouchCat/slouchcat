package visibility

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// The lists behind the row titles of the start page (listHomeRow, concept 20 P-7, 8.3): the same
// titles in the same order and by the same rules as the rows, without the rows' cut to
// MaxRowItems. Each takes content, one kind of the top level ("" = all: movie, series, audiobook,
// podcast; the next episode of a series counts as series), and a page, offset clamped to 0 and
// above, limit to 1..MaxLibraryPage (0 or less = DefaultLibraryPage).

// MaxContinueList caps the list «Weiterschauen»: its query always computes the whole list (one entry
// per series), so the page is cut in Go from at most this many entries (8.3).
const MaxContinueList = 200

// topKind is the kind of the title of the top level of an item: a series for its episodes and
// seasons, a podcast for its episodes, else the item's own kind.
func topKind(kind string) string {
	switch kind {
	case "episode", "season":
		return "series"
	case "podcast_episode":
		return "podcast"
	}
	return kind
}

// slicePage returns the items of one page of all and the number of all.
func slicePage[T any](all []T, offset, limit int) ([]T, int) {
	if offset >= len(all) {
		return nil, len(all)
	}
	return all[offset:min(offset+limit, len(all))], len(all)
}

// RecentPage returns one page of the most recently added visible titles and their number.
func (r *Reader) RecentPage(ctx context.Context, s Scope, content string, offset, limit int) ([]model.ItemSummary, int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	offset, limit = libraryPage(offset, limit)
	total, err := r.q.CountRecentlyAdded(ctx, scopedq.CountRecentlyAddedParams{ProfileID: pid, Content: content})
	if err != nil {
		return nil, 0, fmt.Errorf("count recently added: %w", err)
	}
	items, err := r.q.ListRecentlyAddedPage(ctx, scopedq.ListRecentlyAddedPageParams{
		ProfileID: pid, Content: content, OffsetCount: int64(offset), LimitCount: int64(limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list recently added: %w", err)
	}
	out, err := r.summaries(ctx, s, pid, items)
	return out, int(total), err
}

// ReleasedPage returns one page of the titles released in the last ReleasedMonths (the rule of
// Released) and their number.
func (r *Reader) ReleasedPage(ctx context.Context, s Scope, content string, offset, limit int) ([]model.ItemSummary, int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	offset, limit = libraryPage(offset, limit)
	today := r.now().UTC()
	since := sql.NullString{String: today.AddDate(0, -ReleasedMonths, 0).Format(time.DateOnly), Valid: true}
	until := sql.NullString{String: today.Format(time.DateOnly), Valid: true}
	total, err := r.q.CountRecentlyReleased(ctx, scopedq.CountRecentlyReleasedParams{ProfileID: pid, Content: content, Since: since, Today: until})
	if err != nil {
		return nil, 0, fmt.Errorf("count recently released: %w", err)
	}
	rows, err := r.q.ListRecentlyReleasedPage(ctx, scopedq.ListRecentlyReleasedPageParams{
		ProfileID: pid, Content: content, Since: since, Today: until, OffsetCount: int64(offset), LimitCount: int64(limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list recently released: %w", err)
	}
	items := make([]scopedq.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.Item)
	}
	out, err := r.summaries(ctx, s, pid, items)
	return out, int(total), err
}

// ContinuePage returns one page of the list «Weiterschauen» (the rule of ContinueWatching) and its
// length: the query computes at most MaxContinueList entries once per call, the filter and the page
// are cut in Go. Between two pages the order may shift while the profile watches (8.3).
func (r *Reader) ContinuePage(ctx context.Context, s Scope, content string, offset, limit int) ([]model.ItemSummary, int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	offset, limit = libraryPage(offset, limit)
	items, err := r.q.ListContinueWatching(ctx, scopedq.ListContinueWatchingParams{
		ProfileID: pid, LimitCount: MaxContinueList, Since: r.now().Add(-NextEpisodeWindow).UnixMilli(),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list continue watching: %w", err)
	}
	if content != "" {
		kept := items[:0]
		for _, it := range items {
			if topKind(it.Kind) == content {
				kept = append(kept, it)
			}
		}
		items = kept
	}
	page, total := slicePage(items, offset, limit)
	out, err := r.summaries(ctx, s, pid, page)
	return out, total, err
}

// SpotlightPage returns one page of the spotlight list (the rule of SpotlightRow: hidden entries
// left out, stale entries too, the pinned ones at their place) without the cut to item_count, the
// number of its titles after the filter, and whether the row exists on the start page: enabled,
// at least one hit and at least min_hits hits before the filter (plan 5.3, spotlightRow of
// package discover). Without the row the page is empty.
func (r *Reader) SpotlightPage(ctx context.Context, s Scope, content string, offset, limit int) ([]model.ItemSummary, int, bool, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, false, err
	}
	offset, limit = libraryPage(offset, limit)
	cfg, err := r.app.GetSpotlightConfig(ctx)
	if err != nil {
		return nil, 0, false, fmt.Errorf("get spotlight config: %w", err)
	}
	if cfg.Enabled == 0 {
		return nil, 0, false, nil
	}
	minFetched := r.now().Add(-2 * time.Duration(cfg.IntervalHours) * time.Hour).UnixMilli()
	rows, err := r.q.ListSpotlightRow(ctx, scopedq.ListSpotlightRowParams{ProfileID: pid, ListKey: cfg.ListKey, MinFetched: minFetched})
	if err != nil {
		return nil, 0, false, fmt.Errorf("list spotlight row: %w", err)
	}
	ordered := placeSpotlight(rows)
	if hits := len(ordered); hits == 0 || hits < int(cfg.MinHits) {
		return nil, 0, false, nil
	}
	if content != "" {
		kept := ordered[:0]
		for _, row := range ordered {
			if topKind(row.Item.Kind) == content {
				kept = append(kept, row)
			}
		}
		ordered = kept
	}
	page, total := slicePage(ordered, offset, limit)
	items := make([]scopedq.Item, 0, len(page))
	notes := make(map[string]string, len(page))
	for _, row := range page {
		items = append(items, row.Item)
		notes[row.Item.ID] = row.Note.String
	}
	out, err := r.summaries(ctx, s, pid, items)
	if err != nil {
		return nil, 0, false, err
	}
	for i := range out {
		out[i].Note = notes[out[i].ID]
	}
	return out, total, true, nil
}
