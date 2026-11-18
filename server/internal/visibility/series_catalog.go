package visibility

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The season catalog of a series of the library (concept 25 step 4.1, data model section 2.10):
// package metadata stores the seasons TMDB lists (job series.catalog), this file decides what a
// profile sees of them. Season 0 (specials, «Extras») is never part of it (Anhang I, I-40). Only a
// profile that may request series sees the catalog at all (F-9: a profile that may not request
// sees no missing seasons, and the series page shows it the series as before, M-06).

// SeasonState is the state of one season of the catalog for a profile.
type SeasonState string

const (
	// SeasonPresent: the profile can play at least one episode of the season.
	SeasonPresent SeasonState = "present"
	// SeasonMissing: the season has aired (or is airing) and the profile can play none of it; it
	// can be requested.
	SeasonMissing SeasonState = "missing"
	// SeasonAnnounced: the season has not aired yet (no episodes, no air date or one in the
	// future); it cannot be requested.
	SeasonAnnounced SeasonState = "announced"
)

// CatalogSeason is one season of the catalog.
type CatalogSeason struct {
	Number       int
	EpisodeCount int    // episodes by TMDB; 0 = unknown (a library season TMDB does not list)
	AirDate      string // YYYY-MM-DD, "" = unknown
	Episodes     int    // episodes the profile can play (visibility.VisibleSeasons)
	State        SeasonState
}

// CatalogEpisode names the last aired or the next episode of a series.
type CatalogEpisode struct {
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
	AirDate string `json:"airDate,omitempty"`
}

// SeriesCatalog is the catalog of a series for one profile.
type SeriesCatalog struct {
	Status       string // TMDB status ("Returning Series", "Ended", ...); "" = unknown
	InProduction bool
	// Seasons in number order, without season 0: every season TMDB lists plus every season the
	// profile can play that TMDB does not list, less the library seasons it can play none of
	// (HiddenSeasons: not listed, not counted, not requestable).
	Seasons     []CatalogSeason
	NextEpisode *CatalogEpisode // nil = none known («Läuft noch, nächste Folge am …»)
	LastEpisode *CatalogEpisode
	// Present and Total are the numbers of «Present von Total Staffeln vorhanden»: the present
	// seasons and the present plus missing ones (announced seasons do not count).
	Present, Total int
	// Missing lists the numbers of the missing seasons in order.
	Missing []int
	// CanRequestSeasons is true when the profile may request series and at least one season is
	// missing (getItem.canRequestSeasons).
	CanRequestSeasons bool
	FetchedAt         time.Time
}

// catalogSeasonJSON is one element of series_catalog.seasons.
type catalogSeasonJSON struct {
	Number       int    `json:"number"`
	EpisodeCount int    `json:"episodeCount"`
	AirDate      string `json:"airDate,omitempty"`
}

// storedCatalog is a decoded series_catalog row.
type storedCatalog struct {
	row        systemq.SeriesCatalog
	seasons    []catalogSeasonJSON // without season 0, in number order
	next, last *CatalogEpisode
}

// loadCatalog reads the catalog row of a series; found is false without row.
func (r *Reader) loadCatalog(ctx context.Context, seriesID string) (storedCatalog, bool, error) {
	row, err := r.sys.GetSeriesCatalog(ctx, seriesID)
	if errors.Is(err, sql.ErrNoRows) {
		return storedCatalog{}, false, nil
	}
	if err != nil {
		return storedCatalog{}, false, fmt.Errorf("series catalog %s: %w", seriesID, err)
	}
	c := storedCatalog{row: row}
	var all []catalogSeasonJSON
	if err := json.Unmarshal([]byte(row.Seasons), &all); err != nil {
		return storedCatalog{}, false, fmt.Errorf("series catalog %s: seasons: %w", seriesID, err)
	}
	for _, se := range all {
		if se.Number > 0 {
			c.seasons = append(c.seasons, se)
		}
	}
	slices.SortFunc(c.seasons, func(a, b catalogSeasonJSON) int { return a.Number - b.Number })
	for _, f := range []struct {
		raw sql.NullString
		out **CatalogEpisode
	}{{row.NextEpisode, &c.next}, {row.LastEpisode, &c.last}} {
		if !f.raw.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(f.raw.String), f.out); err != nil {
			return storedCatalog{}, false, fmt.Errorf("series catalog %s: episode: %w", seriesID, err)
		}
	}
	return c, true, nil
}

// aired reports whether season se has aired by today (YYYY-MM-DD): it has episodes and an air date
// up to today, or the last aired episode lies in it or a later season.
func (c storedCatalog) aired(se catalogSeasonJSON, today string) bool {
	if se.EpisodeCount <= 0 {
		return false
	}
	if se.AirDate != "" && se.AirDate <= today {
		return true
	}
	return c.last != nil && c.last.Season >= se.Number
}

// SeriesCatalog returns the catalog of a series the profile sees, with the state of every season
// for it. A series the profile does not see answers model.ErrNotFound (whatever its right). It is
// nil, without error, for a profile without the request right for series (right nil or
// right(MediaShow) false: F-9) and for a series without catalog row or without seasons in it
// (no TMDB id, not fetched yet, unknown to TMDB): the page then shows no missing seasons.
func (r *Reader) SeriesCatalog(ctx context.Context, s Scope, right RequestRight, seriesID string) (*SeriesCatalog, error) {
	visible, err := r.VisibleSeasons(ctx, s, seriesID)
	if err != nil {
		return nil, err
	}
	if right == nil || !right(MediaShow) {
		return nil, nil
	}
	c, found, err := r.loadCatalog(ctx, seriesID)
	if err != nil || !found || len(c.seasons) == 0 {
		return nil, err
	}
	hidden, err := r.hiddenSeasons(ctx, seriesID, visible)
	if err != nil {
		return nil, err
	}
	today := r.now().Format(time.DateOnly)
	out := &SeriesCatalog{Status: c.row.Status.String, InProduction: c.row.InProduction == 1, NextEpisode: c.next,
		LastEpisode: c.last, FetchedAt: time.UnixMilli(c.row.FetchedAt)}
	listed := map[int]bool{}
	for _, se := range c.seasons {
		listed[se.Number] = true
		if slices.Contains(hidden, se.Number) {
			continue // a library season the profile cannot play is not listed at all (HiddenSeasons)
		}
		cs := CatalogSeason{Number: se.Number, EpisodeCount: se.EpisodeCount, AirDate: se.AirDate, Episodes: visible[se.Number]}
		switch {
		case cs.Episodes > 0:
			cs.State = SeasonPresent
		case c.aired(se, today):
			cs.State = SeasonMissing
		default:
			cs.State = SeasonAnnounced
		}
		out.Seasons = append(out.Seasons, cs)
	}
	for n, eps := range visible {
		if n > 0 && eps > 0 && !listed[n] {
			out.Seasons = append(out.Seasons, CatalogSeason{Number: n, Episodes: eps, State: SeasonPresent})
		}
	}
	slices.SortFunc(out.Seasons, func(a, b CatalogSeason) int { return a.Number - b.Number })
	for _, cs := range out.Seasons {
		switch cs.State {
		case SeasonPresent:
			out.Present++
			out.Total++
		case SeasonMissing:
			out.Total++
			out.Missing = append(out.Missing, cs.Number)
		case SeasonAnnounced:
		}
	}
	out.CanRequestSeasons = len(out.Missing) > 0
	return out, nil
}

// SeasonTarget is the number of episodes that makes a season of a series complete.
type SeasonTarget struct {
	// Episodes is the season's episode count by TMDB; for a season still on air (the next episode
	// lies in it) the episodes aired so far, 0 when none has aired yet.
	Episodes int
	Airing   bool
}

// CatalogSeasonTarget returns the target of season (1 or more) of a series from series_catalog,
// without a profile (for the wish producers, which read no title table themselves). found is false
// without catalog row, for season 0 and for a season TMDB does not list or lists without episodes:
// the caller falls back to external_titles then.
func (r *Reader) CatalogSeasonTarget(ctx context.Context, seriesID string, season int) (SeasonTarget, bool, error) {
	if season < 1 {
		return SeasonTarget{}, false, nil
	}
	c, found, err := r.loadCatalog(ctx, seriesID)
	if err != nil || !found {
		return SeasonTarget{}, false, err
	}
	i := slices.IndexFunc(c.seasons, func(se catalogSeasonJSON) bool { return se.Number == season })
	if i < 0 || c.seasons[i].EpisodeCount <= 0 {
		return SeasonTarget{}, false, nil
	}
	if c.next == nil || c.next.Season != season {
		return SeasonTarget{Episodes: c.seasons[i].EpisodeCount}, true, nil
	}
	t := SeasonTarget{Airing: true}
	if c.last != nil && c.last.Season == season {
		t.Episodes = min(c.last.Episode, c.seasons[i].EpisodeCount)
	}
	return t, true, nil
}
