package visibility

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// The reasons of NextUp (contract 1.20.0, concept 17 section 3.3).
const (
	NextUpStart   = "start"
	NextUpResume  = "resume"
	NextUpNext    = "next"
	NextUpRewatch = "rewatch"
)

// nextUpEpisode is a visible episode of a series as nextUp reads it: season number (the season's
// when the episode has none), episode number, the last episode of a file with several, and the
// progress of the profile (zero without a row).
type nextUpEpisode struct {
	ID                    string
	Season, Number, End   *int
	Completed             bool
	PositionMs, UpdatedAt int64
}

// nextUp returns the reason and the episode the big button of a series (or of one season) plays,
// by the rule of contract 1.20.0 (NextUp, concept 17 section 3.3). eps are the visible episodes in
// the order of ListSeriesNextUpEpisodes: by season, number and id; episodes without a number last
// within their season, episodes without a season last. Without an episode ok is false.
//
//   - Regular episodes are those of season 1 or later; without one every episode is regular (a
//     series of specials only, the season page of the specials).
//   - The newest activity is the episode with the newest progress that is completed or started
//     (position > 0), then the greater id, without a time limit.
//   - No activity: start, the first regular episode.
//   - The newest one started, not completed: resume, that episode.
//   - The newest one completed: next, the first regular episode after it by season and number
//     (after its last episode for a file with several), watched or not. That step is empty when
//     the completed episode is not regular (a special of a series with regular episodes) or lacks
//     its season or episode number; then next, the first regular episode not completed; when every
//     regular episode is completed, rewatch with the first one.
func nextUp(eps []nextUpEpisode) (reason string, ep nextUpEpisode, ok bool) {
	regular := make([]nextUpEpisode, 0, len(eps))
	for _, e := range eps {
		if e.Season != nil && *e.Season >= 1 {
			regular = append(regular, e)
		}
	}
	allRegular := len(regular) == 0
	if allRegular {
		regular = eps
	}
	if len(regular) == 0 {
		return "", nextUpEpisode{}, false
	}
	newest := -1
	for i, e := range eps {
		if !e.Completed && e.PositionMs <= 0 {
			continue
		}
		if newest < 0 || e.UpdatedAt > eps[newest].UpdatedAt || (e.UpdatedAt == eps[newest].UpdatedAt && e.ID > eps[newest].ID) {
			newest = i
		}
	}
	if newest < 0 {
		return NextUpStart, regular[0], true
	}
	n := eps[newest]
	if !n.Completed {
		return NextUpResume, n, true
	}
	isRegular := allRegular || (n.Season != nil && *n.Season >= 1)
	if isRegular && n.Season != nil && n.Number != nil {
		last := *n.Number
		if n.End != nil {
			last = *n.End
		}
		for _, e := range regular {
			if e.Season == nil {
				continue
			}
			if *e.Season > *n.Season || (*e.Season == *n.Season && e.Number != nil && *e.Number > last) {
				return NextUpNext, e, true
			}
		}
	}
	for _, e := range regular {
		if !e.Completed {
			return NextUpNext, e, true
		}
	}
	return NextUpRewatch, regular[0], true
}

// nextUpOf sets d.NextUp from the visible episodes of a series, or of one of its seasons when
// seasonID is not empty; without a visible episode it stays nil.
func (r *Reader) nextUpOf(ctx context.Context, s Scope, pid int64, seriesID sql.NullString, seasonID string, d *model.ItemDetail) error {
	rows, err := r.q.ListSeriesNextUpEpisodes(ctx, scopedq.ListSeriesNextUpEpisodesParams{ProfileID: pid, SeriesID: seriesID, SeasonID: seasonID})
	if err != nil {
		return fmt.Errorf("list next up episodes: %w", err)
	}
	eps := make([]nextUpEpisode, 0, len(rows))
	for _, row := range rows {
		eps = append(eps, nextUpEpisode{
			// A NULL column scans as the zero NullInt64, so cmp.Or picks the first set value.
			ID: row.ID, Season: optInt(cmp.Or(row.SeasonNumber, row.SeasonSeasonNumber)), Number: optInt(row.EpisodeNumber), End: optInt(row.EpisodeEnd),
			Completed: row.Completed == 1, PositionMs: row.PositionMs, UpdatedAt: row.UpdatedAt,
		})
	}
	reason, ep, ok := nextUp(eps)
	if !ok {
		return nil
	}
	sums, err := r.list(ctx, s, pid, "next up")(r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: []string{ep.ID}}))
	if err != nil {
		return err
	}
	if len(sums) == 1 {
		d.NextUp = &model.NextUp{Reason: reason, Episode: sums[0]}
	}
	return nil
}
