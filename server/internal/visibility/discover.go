package visibility

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Discover reads of M7 (design 3.1, queries/scoped/discover.sql): the watchlist and the favourites
// of a profile, collections, genres and the spotlight row (M11, queries/scoped/spotlight.sql). Each
// joins visible_items: an entry whose title became invisible is left out, a collection exists for
// the profile only through a visible member and counts only visible members, a spotlight entry
// shows only a visible library title.

// Pages of the watchlist and of a collection: at most MaxPage titles, DefaultPage without a limit.
const (
	DefaultPage = 50
	MaxPage     = 200
)

// MaxFavorites caps the favourites list.
const MaxFavorites = 500

// page clamps offset to 0 and above and limit to 1..MaxPage (0 or less = DefaultPage).
func page(offset, limit int) (int, int) {
	if limit <= 0 {
		limit = DefaultPage
	}
	return max(offset, 0), min(limit, MaxPage)
}

// Watchlist returns one page of the profile's watchlist, newest entry first, and the number of its
// visible entries (E-5). limit is clamped to 1..MaxPage (0 = DefaultPage).
func (r *Reader) Watchlist(ctx context.Context, s Scope, offset, limit int) ([]model.ItemSummary, int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	offset, limit = page(offset, limit)
	total, err := r.q.CountWatchlist(ctx, pid)
	if err != nil {
		return nil, 0, fmt.Errorf("count watchlist: %w", err)
	}
	items, err := r.watchlistPage(ctx, s, pid, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return items, int(total), nil
}

// WatchlistRow returns the newest entries of the profile's watchlist for the start page, at most
// limit (clamped to 1..MaxRowItems), without counting the whole list.
func (r *Reader) WatchlistRow(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	return r.watchlistPage(ctx, s, pid, 0, int(clamp(limit)))
}

func (r *Reader) watchlistPage(ctx context.Context, s Scope, pid int64, offset, limit int) ([]model.ItemSummary, error) {
	return r.list(ctx, s, pid, "watchlist")(r.q.ListWatchlist(ctx, scopedq.ListWatchlistParams{
		ProfileID: pid, LimitCount: int64(limit), OffsetCount: int64(offset),
	}))
}

// InWatchlist reports whether a visible title is on the profile's watchlist; an invisible one never is.
func (r *Reader) InWatchlist(ctx context.Context, s Scope, itemID string) (bool, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return false, err
	}
	n, err := r.q.GetWatchlistEntry(ctx, scopedq.GetWatchlistEntryParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return false, fmt.Errorf("get watchlist entry: %w", err)
	}
	return n > 0, nil
}

// Favorites returns the profile's favourite series and podcasts that it may still see, newest
// first (E-8), at most MaxFavorites.
func (r *Reader) Favorites(ctx context.Context, s Scope) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	return r.list(ctx, s, pid, "favorites")(r.q.ListFavorites(ctx, scopedq.ListFavoritesParams{ProfileID: pid, LimitCount: MaxFavorites}))
}

// IsFavorite reports whether a visible title is a favourite of the profile; an invisible one never is.
func (r *Reader) IsFavorite(ctx context.Context, s Scope, itemID string) (bool, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return false, err
	}
	n, err := r.q.GetFavorite(ctx, scopedq.GetFavoriteParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return false, fmt.Errorf("get favorite: %w", err)
	}
	return n > 0, nil
}

// Genres returns the genres of the visible films, series, audiobooks and podcasts in the profile
// language with their counts, most frequent first, at most limit (clamped to 1..MaxRowItems).
func (r *Reader) Genres(ctx context.Context, s Scope, limit int) ([]model.GenreRef, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListVisibleGenres(ctx, scopedq.ListVisibleGenresParams{ProfileID: pid, Lang: s.language, LimitCount: clamp(limit)})
	if err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	out := make([]model.GenreRef, 0, len(rows))
	for _, g := range rows {
		out = append(out, model.GenreRef{Name: g.Name, Count: int(g.ItemCount)})
	}
	return out, nil
}

// Collections returns the collections with at least one visible member, by name, at most limit
// (clamped to 1..MaxRowItems), each with the number of visible members and its cover (E-6).
func (r *Reader) Collections(ctx context.Context, s Scope, limit int) ([]model.CollectionRef, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListVisibleCollections(ctx, scopedq.ListVisibleCollectionsParams{ProfileID: pid, LimitCount: clamp(limit)})
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	refs := make([]collectionRow, 0, len(rows))
	for _, c := range rows {
		refs = append(refs, collectionRow(c))
	}
	return r.collectionRefs(ctx, s, pid, refs)
}

// collectionsByID returns the collections of ids that have a visible member, in the order of ids.
func (r *Reader) collectionsByID(ctx context.Context, s Scope, pid int64, ids []string) ([]model.CollectionRef, error) {
	if len(ids) == 0 {
		return []model.CollectionRef{}, nil
	}
	rows, err := r.q.ListVisibleCollectionsByID(ctx, scopedq.ListVisibleCollectionsByIDParams{ProfileID: pid, CollectionIds: ids})
	if err != nil {
		return nil, fmt.Errorf("list collections by id: %w", err)
	}
	refs := make([]collectionRow, 0, len(rows))
	for _, c := range rows {
		refs = append(refs, collectionRow(c))
	}
	pos := make(map[string]int, len(ids))
	for i, id := range ids {
		pos[id] = i
	}
	slices.SortFunc(refs, func(a, b collectionRow) int { return pos[a.ID] - pos[b.ID] })
	return r.collectionRefs(ctx, s, pid, refs)
}

// collectionsOf returns the collections a visible title belongs to, by name.
func (r *Reader) collectionsOf(ctx context.Context, s Scope, pid int64, itemID string) ([]model.CollectionRef, error) {
	rows, err := r.q.ListCollectionsOfItem(ctx, scopedq.ListCollectionsOfItemParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return nil, fmt.Errorf("list collections of item: %w", err)
	}
	refs := make([]collectionRow, 0, len(rows))
	for _, c := range rows {
		refs = append(refs, collectionRow(c))
	}
	return r.collectionRefs(ctx, s, pid, refs)
}

// collectionRow is the common shape of the collection list queries.
type collectionRow struct {
	ID          string
	Name        string
	NameDe      sql.NullString
	MemberCount int64
	CoverID     string
}

// collectionRefs names the collections in the profile language and adds their covers.
func (r *Reader) collectionRefs(ctx context.Context, s Scope, pid int64, rows []collectionRow) ([]model.CollectionRef, error) {
	coverIDs := make([]string, 0, len(rows))
	for _, c := range rows {
		if c.CoverID != "" && !slices.Contains(coverIDs, c.CoverID) {
			coverIDs = append(coverIDs, c.CoverID)
		}
	}
	covers := map[string]model.ItemSummary{}
	if len(coverIDs) > 0 {
		items, err := r.list(ctx, s, pid, "collection covers")(r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: coverIDs}))
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			covers[it.ID] = it
		}
	}
	out := make([]model.CollectionRef, 0, len(rows))
	for _, c := range rows {
		ref := model.CollectionRef{ID: c.ID, Name: localized(s.language, c.Name, c.NameDe), Count: int(c.MemberCount)}
		if cover, ok := covers[c.CoverID]; ok {
			ref.Cover = &cover
		}
		out = append(out, ref)
	}
	return out, nil
}

// localized returns the German text for a German profile when there is one, else the English (or
// admin) text, and the German one when that is empty.
func localized(lang, en string, de sql.NullString) string {
	if (lang == "de" && de.String != "") || en == "" {
		return de.String
	}
	return en
}

// Collection returns a collection with one page of its visible members in collection order
// (position, then release date) and their number; a collection without a visible member is
// model.ErrNotFound (10.2). limit is clamped to 1..MaxPage (0 = DefaultPage).
func (r *Reader) Collection(ctx context.Context, s Scope, id string, offset, limit int) (model.Collection, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.Collection{}, err
	}
	c, err := r.q.GetVisibleCollection(ctx, scopedq.GetVisibleCollectionParams{ProfileID: pid, CollectionID: id})
	if err != nil {
		return model.Collection{}, notFound(err)
	}
	offset, limit = page(offset, limit)
	items, err := r.list(ctx, s, pid, "collection items")(r.q.ListVisibleCollectionItems(ctx, scopedq.ListVisibleCollectionItemsParams{
		ProfileID: pid, CollectionID: id, LimitCount: int64(limit), OffsetCount: int64(offset),
	}))
	if err != nil {
		return model.Collection{}, err
	}
	return model.Collection{
		ID:       c.ID,
		Name:     localized(s.language, c.Name, c.NameDe),
		Overview: localized(s.language, c.Overview.String, c.OverviewDe),
		Items:    items,
		Total:    int(c.MemberCount),
	}, nil
}

// SpotlightRow returns the visible library titles of the configured spotlight list
// (spotlight_config, movies and shows matched by TMDB id) with the curation of the main user
// applied, and the number of hits the row had before the cut (the min_hits of the configuration
// count hits, not shown titles, plan 5.3). limit is clamped to 1..MaxRowItems. Entries fetched more
// than twice spotlight_config.interval_hours ago by the Reader's clock are stale and left out
// (S-8), so a refresh job that stopped running (client id removed, worker hung) does not leave an
// old list on the start page for ever. A disabled configuration gives no row.
//
// The order is the plan's rule (5.3), in this sequence: SQL filters hidden entries and delivers
// the natural list order; Go deduplicates (a title listed twice, pinned or not, appears once);
// the pinned entries are INSERTED at their 1-based pinned_rank, ascending, a rank beyond the end
// appends; the cut to limit comes last, so a pinned entry never falls to the cut before it took
// its place. "At place N" is an insertion, not a sort key: both earlier sort-based readings of the
// rule put a moved entry on the wrong place in one direction.
func (r *Reader) SpotlightRow(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	cfg, err := r.app.GetSpotlightConfig(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("get spotlight config: %w", err)
	}
	if cfg.Enabled == 0 {
		return nil, 0, nil
	}
	minFetched := r.now().Add(-2 * time.Duration(cfg.IntervalHours) * time.Hour).UnixMilli()
	rows, err := r.q.ListSpotlightRow(ctx, scopedq.ListSpotlightRowParams{ProfileID: pid, ListKey: cfg.ListKey, MinFetched: minFetched})
	if err != nil {
		return nil, 0, fmt.Errorf("list spotlight row: %w", err)
	}
	ordered := placeSpotlight(rows)
	hits := len(ordered)
	if n := int(clamp(limit)); len(ordered) > n {
		ordered = ordered[:n]
	}
	items := make([]scopedq.Item, 0, len(ordered))
	notes := make(map[string]string, len(ordered))
	for _, row := range ordered {
		items = append(items, row.Item)
		notes[row.Item.ID] = row.Note.String
	}
	out, err := r.summaries(ctx, s, pid, items)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		out[i].Note = notes[out[i].ID]
	}
	return out, hits, nil
}

// placeSpotlight deduplicates rows by title (first occurrence wins, so a pinned duplicate stays
// pinned) and inserts the pinned ones at their rank, ascending; see SpotlightRow.
func placeSpotlight(rows []scopedq.ListSpotlightRowRow) []scopedq.ListSpotlightRowRow {
	seen := make(map[string]bool, len(rows))
	free := make([]scopedq.ListSpotlightRowRow, 0, len(rows))
	var pinned []scopedq.ListSpotlightRowRow
	for _, row := range rows {
		if seen[row.Item.ID] {
			continue
		}
		seen[row.Item.ID] = true
		if row.PinnedRank.Valid {
			pinned = append(pinned, row)
		} else {
			free = append(free, row)
		}
	}
	slices.SortStableFunc(pinned, func(a, b scopedq.ListSpotlightRowRow) int {
		return cmp.Compare(a.PinnedRank.Int64, b.PinnedRank.Int64)
	})
	out := free
	for _, row := range pinned {
		at := min(int(row.PinnedRank.Int64)-1, len(out))
		out = slices.Insert(out, max(at, 0), row)
	}
	return out
}

// discoverDetail sets the watchlist and favourite marks and the visible collections of a title on
// its detail.
func (r *Reader) discoverDetail(ctx context.Context, s Scope, pid int64, itemID string, d *model.ItemDetail) error {
	n, err := r.q.GetWatchlistEntry(ctx, scopedq.GetWatchlistEntryParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return fmt.Errorf("get watchlist entry: %w", err)
	}
	d.InWatchlist = n > 0
	n, err = r.q.GetFavorite(ctx, scopedq.GetFavoriteParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return fmt.Errorf("get favorite: %w", err)
	}
	d.Favorite = n > 0
	d.Collections, err = r.collectionsOf(ctx, s, pid, itemID)
	return err
}
