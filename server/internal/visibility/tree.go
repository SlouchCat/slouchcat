package visibility

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Reads of M3 (design 4.1): the series tree (seasons, episodes, specials, multi-episode files),
// extras, cast, trailers and people. Every query joins visible_items, so a season or episode the
// profile may not see is absent from every list, and a person exists for the profile only through
// the visible titles of their credits.

// treeExtras fills the child counts of series and seasons, the parents the profile may see and the
// series titles of seasons and episodes (profile language, then English, then the parsed title).
func (r *Reader) treeExtras(ctx context.Context, pid int64, lang string, ids []string, out *extras) error {
	counts, err := r.q.ListVisibleChildCounts(ctx, scopedq.ListVisibleChildCountsParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return fmt.Errorf("list child counts: %w", err)
	}
	for _, c := range counts {
		out.childCounts[c.ID] = int(c.ChildCount)
	}
	parents, err := r.q.ListVisibleParents(ctx, scopedq.ListVisibleParentsParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return fmt.Errorf("list visible parents: %w", err)
	}
	for _, p := range parents {
		out.parents[p.ID] = p.ParentID.String
	}
	series, err := r.q.ListVisibleSeriesOf(ctx, scopedq.ListVisibleSeriesOfParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return fmt.Errorf("list series of items: %w", err)
	}
	if len(series) == 0 {
		return nil
	}
	seriesIDs := make([]string, 0, len(series))
	for _, s := range series {
		if !slices.Contains(seriesIDs, s.SeriesID) {
			seriesIDs = append(seriesIDs, s.SeriesID)
		}
	}
	texts, err := r.q.ListItemTexts(ctx, scopedq.ListItemTextsParams{ProfileID: pid, ItemIds: seriesIDs})
	if err != nil {
		return fmt.Errorf("list series texts: %w", err)
	}
	bySeries := map[string][]scopedq.ItemText{}
	for _, t := range texts {
		bySeries[t.ItemID] = append(bySeries[t.ItemID], t)
	}
	for _, s := range series {
		out.seriesTitles[s.ItemID] = pickTexts(bySeries[s.SeriesID], lang, s.SeriesTitle).title
	}
	return nil
}

// completeDetail adds what the detail page of each kind shows: seasons and extras of a series,
// episodes, extras and the series of a season, the season, series and next episode of an episode,
// the extras of a film, and for every kind the cast and the number of trailers.
func (r *Reader) completeDetail(ctx context.Context, s Scope, pid int64, it scopedq.Item, d *model.ItemDetail) error {
	self := sql.NullString{String: it.ID, Valid: true}
	var err error
	switch it.Kind {
	case "series":
		d.Seasons, err = r.list(ctx, s, pid, "seasons")(r.q.ListVisibleSeasons(ctx, scopedq.ListVisibleSeasonsParams{ProfileID: pid, SeriesID: self}))
	case "season":
		d.Episodes, err = r.list(ctx, s, pid, "episodes")(r.q.ListVisibleEpisodes(ctx, scopedq.ListVisibleEpisodesParams{ProfileID: pid, SeasonID: self}))
	}
	if err != nil {
		return err
	}
	switch it.Kind {
	case "movie", "series", "season":
		d.Extras, err = r.list(ctx, s, pid, "extras")(r.q.ListVisibleExtras(ctx, scopedq.ListVisibleExtrasParams{ProfileID: pid, ParentID: self}))
		if err != nil {
			return err
		}
	case "episode":
		if err := r.episodeChain(ctx, s, pid, it, d); err != nil {
			return err
		}
	}
	if it.Kind == "season" && it.ParentID.Valid {
		series, err := r.list(ctx, s, pid, "series")(r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: []string{it.ParentID.String}}))
		if err != nil {
			return err
		}
		if len(series) == 1 {
			d.Series = &series[0]
		}
	}
	credits, err := r.q.ListVisibleCredits(ctx, scopedq.ListVisibleCreditsParams{ProfileID: pid, ItemID: it.ID})
	if err != nil {
		return fmt.Errorf("list credits: %w", err)
	}
	d.Cast = make([]model.CastMember, 0, len(credits))
	for _, c := range credits {
		d.Cast = append(d.Cast, model.CastMember{PersonID: c.PersonID, Name: c.Name, Role: c.Role, Character: c.Character.String})
	}
	n, err := r.q.CountVisibleTrailers(ctx, scopedq.CountVisibleTrailersParams{ProfileID: pid, ItemID: it.ID})
	if err != nil {
		return fmt.Errorf("count trailers: %w", err)
	}
	d.TrailerCount = int(n)
	return nil
}

// list returns a function that summarizes the result of one of the tree queries; what names the
// list in errors.
func (r *Reader) list(ctx context.Context, s Scope, pid int64, what string) func([]scopedq.Item, error) ([]model.ItemSummary, error) {
	return func(items []scopedq.Item, err error) ([]model.ItemSummary, error) {
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", what, err)
		}
		return r.summaries(ctx, s, pid, items)
	}
}

// episodeChain sets the season and the series of an episode and its next visible episode: a
// higher number in the same season (after the last episode of a multi-episode file), else the
// first episode of a later season; specials only follow a special.
func (r *Reader) episodeChain(ctx context.Context, s Scope, pid int64, it scopedq.Item, d *model.ItemDetail) error {
	if !it.ParentID.Valid {
		return nil
	}
	seasons, err := r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: []string{it.ParentID.String}})
	if err != nil {
		return fmt.Errorf("get season: %w", err)
	}
	if len(seasons) != 1 || seasons[0].Kind != "season" {
		return nil
	}
	season := seasons[0]
	chain := []scopedq.Item{season}
	if season.ParentID.Valid {
		series, err := r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: []string{season.ParentID.String}})
		if err != nil {
			return fmt.Errorf("get series: %w", err)
		}
		chain = append(chain, series...)
	}
	sums, err := r.summaries(ctx, s, pid, chain)
	if err != nil {
		return err
	}
	d.Season = &sums[0]
	if len(sums) < 2 || sums[1].Kind != "series" {
		return nil
	}
	d.Series = &sums[1]
	// A NULL column scans as the zero NullInt64, so cmp.Or picks the first set value.
	seasonNumber := cmp.Or(it.SeasonNumber, season.SeasonNumber)
	last := cmp.Or(it.EpisodeEnd, it.EpisodeNumber)
	if !seasonNumber.Valid || !last.Valid {
		return nil
	}
	next, err := r.q.GetNextVisibleEpisode(ctx, scopedq.GetNextVisibleEpisodeParams{
		ProfileID: pid, SeriesID: season.ParentID, ItemID: it.ID, SeasonNumber: seasonNumber, LastEpisode: last,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("next episode: %w", err)
	}
	sum, err := r.summaries(ctx, s, pid, []scopedq.Item{next})
	if err != nil {
		return err
	}
	d.NextEpisode = &sum[0]
	return nil
}

// Trailers returns the trailers of a visible title: the profile language first, then English, then
// the other languages; model.ErrNotFound for an invisible title.
func (r *Reader) Trailers(ctx context.Context, s Scope, itemID string) ([]model.Trailer, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	if _, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID}); err != nil {
		return nil, notFound(err)
	}
	rows, err := r.q.ListVisibleTrailers(ctx, scopedq.ListVisibleTrailersParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return nil, fmt.Errorf("list trailers: %w", err)
	}
	rank := func(lang string) int {
		switch lang {
		case s.language:
			return 0
		case fallbackLang:
			return 1
		default:
			return 2
		}
	}
	slices.SortStableFunc(rows, func(a, b scopedq.Trailer) int { return cmp.Compare(rank(a.Lang), rank(b.Lang)) })
	out := make([]model.Trailer, 0, len(rows))
	for _, t := range rows {
		out = append(out, model.Trailer{ID: t.ID, Lang: t.Lang, YouTubeID: t.YoutubeID, Title: t.Title.String, Source: t.Source, Official: t.Official == 1})
	}
	return out, nil
}

// Person returns a person with the credits on titles the profile may see. A person without such a
// credit, like an unknown person, is model.ErrNotFound.
func (r *Reader) Person(ctx context.Context, s Scope, personID int64) (model.Person, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.Person{}, err
	}
	rows, err := r.q.ListVisiblePersonCredits(ctx, scopedq.ListVisiblePersonCreditsParams{ProfileID: pid, PersonID: personID})
	if err != nil {
		return model.Person{}, fmt.Errorf("list person credits: %w", err)
	}
	if len(rows) == 0 {
		return model.Person{}, model.ErrNotFound
	}
	items := make([]scopedq.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.Item)
	}
	sums, err := r.summaries(ctx, s, pid, items)
	if err != nil {
		return model.Person{}, err
	}
	out := model.Person{ID: personID, Name: rows[0].PersonName, Credits: make([]model.PersonCredit, 0, len(rows))}
	for i, row := range rows {
		out.Credits = append(out.Credits, model.PersonCredit{Item: sums[i], Role: row.Role, Character: row.Character.String})
	}
	return out, nil
}
