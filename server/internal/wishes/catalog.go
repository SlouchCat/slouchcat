package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The missing seasons of a series of the library (concept 25 stage 4, build step 4.7; data model
// 4.6 getItem.catalog and canRequestSeasons, functional concept M-06). visibility decides what the
// profile sees of the season catalog; this file adds what the wishes know: a missing season that an
// open wish holds is «bereits angefordert» (queued or pending) or «in Beschaffung» (processing),
// and whether the profile may request seasons from the series page at all.

// Season states of SeriesCatalog beyond those of visibility.
const (
	CatalogSeasonRequested  = "requested"  // an open wish holds it and waits for Seerr or its approval
	CatalogSeasonProcessing = "processing" // an open wish holds it and Seerr procures it
)

// CatalogSeason is one season of the series page: present, missing, announced (visibility), or a
// missing season an open wish holds (requested, processing).
type CatalogSeason struct {
	Number       int
	EpisodeCount int    // by TMDB, 0 = unknown
	AirDate      string // YYYY-MM-DD, "" = unknown
	Episodes     int    // episodes the profile can play
	State        string
}

// CatalogEpisode names an episode with its air date.
type CatalogEpisode struct {
	Season, Episode int
	AirDate         string
}

// SeriesCatalog is the season catalog of a series of the library for one profile (getItem).
type SeriesCatalog struct {
	Status       string
	InProduction bool
	Seasons      []CatalogSeason
	NextEpisode  *CatalogEpisode
	LastEpisode  *CatalogEpisode
	// Present and Total are «Present von Total Staffeln vorhanden».
	Present, Total int
	// CanRequestSeasons: the profile may request series, «Anfordern auf Detailseiten» is on and a
	// missing season is free, or held by another wish it may join («Bereits angeforderte Titel:
	// Mitwünschen»).
	CanRequestSeasons bool
}

// seriesCatalogs is the part of visibility.Reader the series page needs; the Library of the tests
// may leave it out (then no series has a catalog).
type seriesCatalogs interface {
	SeriesCatalog(ctx context.Context, s visibility.Scope, right visibility.RequestRight, seriesID string) (*visibility.SeriesCatalog, error)
}

// SeriesCatalogOf returns the catalog of the series seriesID (TMDB id tmdbID, 0 when unknown) for
// the profile, or nil when it shows none: no right for series (F-9), wishes off, no catalog row.
// model.ErrNotFound for a series the profile does not see.
func (s *Service) SeriesCatalogOf(ctx context.Context, p model.Profile, seriesID string, tmdbID int64) (*SeriesCatalog, error) {
	lib, ok := s.lib.(seriesCatalogs)
	if !ok {
		return nil, nil
	}
	rights, err := s.ForProfile(ctx, p.AccountID, p.ID)
	if err != nil {
		return nil, err
	}
	vc, err := lib.SeriesCatalog(ctx, visibility.Profile(p), requestRight(rights), seriesID)
	if err != nil || vc == nil {
		return nil, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := &SeriesCatalog{Status: vc.Status, InProduction: vc.InProduction, Present: vc.Present, Total: vc.Total,
		NextEpisode: catalogEpisode(vc.NextEpisode), LastEpisode: catalogEpisode(vc.LastEpisode)}
	free, joinable := false, false
	for _, se := range vc.Seasons {
		cs := CatalogSeason{Number: se.Number, EpisodeCount: se.EpisodeCount, AirDate: se.AirDate, Episodes: se.Episodes, State: string(se.State)}
		if se.State == visibility.SeasonMissing {
			state, own, err := s.heldSeason(ctx, p.ID, tmdbID, se.Number)
			if err != nil {
				return nil, err
			}
			switch {
			case state == "":
				free = true
			case own: // the profile's own open wish holds it: nothing to join (review Minor 3)
				cs.State = state
			default:
				cs.State, joinable = state, true
			}
		}
		out.Seasons = append(out.Seasons, cs)
	}
	out.CanRequestSeasons = vc.CanRequestSeasons && ws.FromDetail &&
		(free || joinable && ws.AlreadyRequested != model.WishesAlreadyStatus)
	return out, nil
}

// heldSeason is the state of the open wish season of the series and season: requested,
// processing, or "" when no open wish holds it; own reports that the profile is a requester of
// that wish.
func (s *Service) heldSeason(ctx context.Context, profileID, tmdbID int64, number int) (state string, own bool, err error) {
	if tmdbID < 1 {
		return "", false, nil
	}
	id, err := s.read.GetOpenSeasonWish(ctx, wishq.GetOpenSeasonWishParams{TmdbID: tmdbID, SeasonNumber: int64(number)})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("open season: %w", err)
	}
	r, err := s.read.GetWishRequester(ctx, wishq.GetWishRequesterParams{WishID: id, ProfileID: profileID})
	switch {
	case err == nil:
		own = !r.ClosedAt.Valid
	case !errors.Is(err, sql.ErrNoRows):
		return "", false, fmt.Errorf("requester: %w", err)
	}
	seasons, err := s.read.ListWishSeasons(ctx, id)
	if err != nil {
		return "", false, fmt.Errorf("wish seasons: %w", err)
	}
	for _, se := range seasons {
		if int(se.SeasonNumber) == number && se.State == model.WishProcessing {
			return CatalogSeasonProcessing, own, nil
		}
	}
	return CatalogSeasonRequested, own, nil
}

func catalogEpisode(e *visibility.CatalogEpisode) *CatalogEpisode {
	if e == nil {
		return nil
	}
	return &CatalogEpisode{Season: e.Season, Episode: e.Episode, AirDate: e.AirDate}
}

// TestArrService is TestArrConnection by the service name of the contract (radarr, sonarr); an
// unknown name is a *model.ValidationError on service.
func (s *Service) TestArrService(ctx context.Context, service string) (ArrTestResult, error) {
	kind, err := ParseArrService(service)
	if err != nil {
		return ArrTestResult{}, &model.ValidationError{Fields: []model.FieldError{{Field: "service", Code: "invalid"}}}
	}
	return s.TestArrConnection(ctx, kind)
}
