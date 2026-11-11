package radio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The catalog as a profile sees it (concept 16 sections 5.4 and 10.2): lists, facets and search
// from the index, one station, the favourites in their order and the preferences. The decisions
// about visibility live here, in the server (CLAUDE.md rule 12).

// favoriteIDs returns the station ids the profile marked, in its order.
func (s *Service) favoriteIDs(ctx context.Context, profileID int64) ([]int64, error) {
	ids, err := appq.New(s.db.Read).ListRadioFavoriteStationIDs(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("list radio favourites: %w", err)
	}
	return ids, nil
}

func idSet(ids []int64) map[int64]bool {
	m := make(map[int64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// catalogLoading reports whether the catalog never synced (model.ErrRadioCatalogLoading): no
// station in the index and no entry counted by a successful sync. Only an empty index reads the
// settings.
func (s *Service) catalogLoading(ctx context.Context, idx *stationIndex) (bool, error) {
	if len(idx.entries) > 0 {
		return false, nil
	}
	cfg, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return false, fmt.Errorf("get radio config: %w", err)
	}
	return cfg.EntriesTotal == 0, nil
}

// prepareQuery returns the index, the view, the parsed filter and the favourite set of the
// profile for a list or facet request (model.ErrRadioCatalogLoading before the first sync).
func (s *Service) prepareQuery(ctx context.Context, session model.Session, f model.RadioFilter) (*stationIndex, catalogView, stationFilter, map[int64]bool, error) {
	idx, err := s.index(ctx)
	if err != nil {
		return nil, catalogView{}, stationFilter{}, nil, err
	}
	v, err := s.view(ctx)
	if err != nil {
		return nil, catalogView{}, stationFilter{}, nil, err
	}
	if loading, err := s.catalogLoading(ctx, idx); err != nil {
		return nil, catalogView{}, stationFilter{}, nil, err
	} else if loading {
		return nil, catalogView{}, stationFilter{}, nil, model.ErrRadioCatalogLoading
	}
	ids, err := s.favoriteIDs(ctx, session.Profile.ID)
	if err != nil {
		return nil, catalogView{}, stationFilter{}, nil, err
	}
	fav := idSet(ids)
	sf, err := newStationFilter(f, fav)
	if err != nil {
		return nil, catalogView{}, stationFilter{}, nil, err
	}
	return idx, v, sf, fav, nil
}

// ListStations returns one page of the listed stations that pass the query.
func (s *Service) ListStations(ctx context.Context, session model.Session, q model.RadioQuery) (model.RadioStationPage, error) {
	switch q.Sort {
	case "", model.RadioSortPopular, model.RadioSortName, model.RadioSortQuality, model.RadioSortNew:
	default:
		return model.RadioStationPage{}, (*model.ValidationError)(nil).Add("sort", "invalid_value")
	}
	idx, v, sf, fav, err := s.prepareQuery(ctx, session, q.RadioFilter)
	if err != nil {
		return model.RadioStationPage{}, err
	}
	var hits []*indexEntry
	for _, e := range idx.entries {
		if v.listed(e) && sf.fails(e) == 0 {
			hits = append(hits, e)
		}
	}
	sortEntries(hits, q.Sort, sf.countries)
	page := model.RadioStationPage{Total: len(hits), Offset: q.Offset, Items: []model.RadioStation{}}
	limit := q.Limit
	if limit <= 0 {
		limit = 60
	}
	if q.Offset < len(hits) {
		hits = hits[q.Offset:min(len(hits), q.Offset+limit)]
	} else {
		hits = nil
	}
	ids := make([]int64, 0, len(hits))
	for _, e := range hits {
		st := e.st
		st.Favorite = fav[st.ID]
		page.Items = append(page.Items, st)
		ids = append(ids, st.ID)
	}
	if len(ids) > 0 {
		s.onListed(ctx, ids)
	}
	return page, nil
}

// Facets counts the values of every filter over the listed stations; a facet's counts ignore its
// own selection. OfferEmpty says that no station at all is in the offer (B4.5).
func (s *Service) Facets(ctx context.Context, session model.Session, q model.RadioQuery) (model.RadioFacets, error) {
	idx, v, sf, _, err := s.prepareQuery(ctx, session, q.RadioFilter)
	if err != nil {
		return model.RadioFacets{}, err
	}
	c := newFacetCounter()
	empty := true
	for _, e := range idx.entries {
		if v.reachable(e) {
			empty = false
		}
		if v.listed(e) {
			c.add(e, sf.fails(e))
		}
	}
	out := c.result()
	out.OfferEmpty = empty
	return out, nil
}

// reachableEntry returns the index entry of a station the profile may reach (model.ErrNotFound
// for an unknown one or one outside the offer).
func (s *Service) reachableEntry(ctx context.Context, stationID int64) (*indexEntry, error) {
	idx, err := s.index(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.view(ctx)
	if err != nil {
		return nil, err
	}
	e := idx.byID[stationID]
	if e == nil || !v.reachable(e) {
		return nil, model.ErrNotFound
	}
	return e, nil
}

// Station returns one station, broken and missing ones included.
func (s *Service) Station(ctx context.Context, session model.Session, stationID int64) (model.RadioStation, error) {
	e, err := s.reachableEntry(ctx, stationID)
	if err != nil {
		return model.RadioStation{}, err
	}
	ids, err := s.favoriteIDs(ctx, session.Profile.ID)
	if err != nil {
		return model.RadioStation{}, err
	}
	st := e.st
	st.Favorite = slices.Contains(ids, stationID)
	return st, nil
}

// reachableFavorites returns the favourites of the profile the profile may reach, in its order,
// and all favourite ids (those outside the offer keep their rows).
func (s *Service) reachableFavorites(ctx context.Context, profileID int64) ([]model.RadioStation, []int64, error) {
	ids, err := s.favoriteIDs(ctx, profileID)
	if err != nil {
		return nil, nil, err
	}
	idx, err := s.index(ctx)
	if err != nil {
		return nil, nil, err
	}
	v, err := s.view(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := []model.RadioStation{}
	for _, id := range ids {
		e := idx.byID[id]
		if e == nil || !v.reachable(e) {
			continue
		}
		st := e.st
		st.Favorite = true
		out = append(out, st)
	}
	return out, ids, nil
}

// Favorites returns the profile's favourites in its order, without those outside the offer.
func (s *Service) Favorites(ctx context.Context, session model.Session) ([]model.RadioStation, error) {
	out, _, err := s.reachableFavorites(ctx, session.Profile.ID)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		ids := make([]int64, 0, len(out))
		for _, st := range out {
			ids = append(ids, st.ID)
		}
		s.onListed(ctx, ids)
	}
	return out, nil
}

// SetFavoriteOrder stores the complete new order of the favourites the profile sees; another set
// of ids is a *model.ValidationError on stationIds. Favourites outside the offer keep their rows
// behind the visible ones.
func (s *Service) SetFavoriteOrder(ctx context.Context, session model.Session, stationIDs []int64) error {
	visible, all, err := s.reachableFavorites(ctx, session.Profile.ID)
	if err != nil {
		return err
	}
	want := make([]int64, 0, len(visible))
	for _, st := range visible {
		want = append(want, st.ID)
	}
	got := slices.Clone(stationIDs)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return (*model.ValidationError)(nil).Add("stationIds", "invalid_value")
	}
	order := slices.Clone(stationIDs)
	for _, id := range all {
		if !slices.Contains(order, id) {
			order = append(order, id)
		}
	}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set radio favourite order: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	for pos, id := range order {
		if _, err := q.SetRadioFavoritePosition(ctx, appq.SetRadioFavoritePositionParams{Position: int64(pos), ProfileID: session.Profile.ID, StationID: id}); err != nil {
			return fmt.Errorf("set radio favourite position: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("set radio favourite order: commit: %w", err)
	}
	return nil
}

// AddFavorite marks a station the profile may reach (model.ErrNotFound otherwise); adding it
// again keeps its place.
func (s *Service) AddFavorite(ctx context.Context, session model.Session, stationID int64) error {
	if _, err := s.reachableEntry(ctx, stationID); err != nil {
		return err
	}
	if err := appq.New(s.db.Write).AddRadioFavorite(ctx, appq.AddRadioFavoriteParams{ProfileID: session.Profile.ID, StationID: stationID, Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("add radio favourite: %w", err)
	}
	s.onListed(ctx, []int64{stationID})
	return nil
}

// RemoveFavorite unmarks a station (idempotent).
func (s *Service) RemoveFavorite(ctx context.Context, session model.Session, stationID int64) error {
	if err := appq.New(s.db.Write).RemoveRadioFavorite(ctx, appq.RemoveRadioFavoriteParams{ProfileID: session.Profile.ID, StationID: stationID}); err != nil {
		return fmt.Errorf("remove radio favourite: %w", err)
	}
	return nil
}

// settingRatingCountry is the metadata setting the default country falls back to (R-9).
const settingRatingCountry = "metadata.rating_country"

// defaultCountry is the country a profile without its own starts with (R-9): the country of the
// server's time zone, else the metadata rating country, else DE (its default).
func (s *Service) defaultCountry(ctx context.Context) (string, error) {
	if c := rules.ZoneCountry(s.cat.zone()); c != "" {
		return c, nil
	}
	v, err := appq.New(s.db.Read).GetSetting(ctx, settingRatingCountry)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "DE", nil
	case err != nil:
		return "", fmt.Errorf("get %s: %w", settingRatingCountry, err)
	case countryCode.MatchString(v):
		return v, nil
	}
	return "DE", nil
}

// Preferences returns the radio preferences of the profile (F-12); DefaultCountry is filled when
// the profile has none. EffectiveCountry is DefaultCountry while the offer lists stations of it,
// else empty, and the countries of LastFilter outside the offer are left out (B4.5, OB-10); the
// stored values stay.
func (s *Service) Preferences(ctx context.Context, session model.Session) (model.RadioPreferences, error) {
	row, err := appq.New(s.db.Read).GetRadioProfileState(ctx, session.Profile.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.RadioPreferences{}, fmt.Errorf("get radio profile state: %w", err)
	}
	p := model.RadioPreferences{DefaultCountry: row.DefaultCountry.String, LastSort: row.LastSort.String}
	if row.LastFilter.Valid {
		var f model.RadioFilter
		if json.Unmarshal([]byte(row.LastFilter.String), &f) == nil {
			p.LastFilter = &f
		}
	}
	if p.DefaultCountry == "" {
		if p.DefaultCountry, err = s.defaultCountry(ctx); err != nil {
			return model.RadioPreferences{}, err
		}
	}
	offered, err := s.listedCountries(ctx)
	if err != nil {
		return model.RadioPreferences{}, err
	}
	if offered[p.DefaultCountry] {
		p.EffectiveCountry = p.DefaultCountry
	}
	if p.LastFilter != nil && len(p.LastFilter.Countries) > 0 {
		p.LastFilter.Countries = slices.DeleteFunc(p.LastFilter.Countries, func(c string) bool { return !offered[c] })
	}
	return p, nil
}

// listedCountries returns the countries of the stations the lists show under the offer.
func (s *Service) listedCountries(ctx context.Context) (map[string]bool, error) {
	idx, err := s.index(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.view(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, e := range idx.entries {
		if e.st.Country != "" && !out[e.st.Country] && v.listed(e) {
			out[e.st.Country] = true
		}
	}
	return out, nil
}

// SetPreferences replaces the preferences; a bad country or sort is a *model.ValidationError.
func (s *Service) SetPreferences(ctx context.Context, session model.Session, p model.RadioPreferences) (model.RadioPreferences, error) {
	var verr *model.ValidationError
	if p.DefaultCountry != "" && !countryCode.MatchString(p.DefaultCountry) {
		verr = verr.Add("defaultCountry", "invalid_value")
	}
	switch p.LastSort {
	case "", model.RadioSortPopular, model.RadioSortName, model.RadioSortQuality, model.RadioSortNew:
	default:
		verr = verr.Add("lastSort", "invalid_value")
	}
	if verr != nil {
		return model.RadioPreferences{}, verr
	}
	arg := appq.UpsertRadioProfileStateParams{ProfileID: session.Profile.ID, Now: s.now().UnixMilli(),
		DefaultCountry: sql.NullString{String: p.DefaultCountry, Valid: p.DefaultCountry != ""},
		LastSort:       sql.NullString{String: p.LastSort, Valid: p.LastSort != ""}}
	if p.LastFilter != nil {
		raw, err := json.Marshal(p.LastFilter)
		if err != nil {
			return model.RadioPreferences{}, fmt.Errorf("marshal radio filter: %w", err)
		}
		arg.LastFilter = sql.NullString{String: string(raw), Valid: true}
	}
	if err := appq.New(s.db.Write).UpsertRadioProfileState(ctx, arg); err != nil {
		return model.RadioPreferences{}, fmt.Errorf("upsert radio profile state: %w", err)
	}
	return s.Preferences(ctx, session)
}
