package radio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The station administration (concept 16 section 5.6, «Ausnahmen» of stage 1): every station,
// hidden ones included, with its streams, tags, facets and locks; a patch locks the fields it sets,
// so the next sync keeps them, and a lock the admin lifts lets the next sync overwrite the field.

// lockFields are the fields a lock may hold (radio_station_locks.field).
var lockFields = []string{"name", "logo", "genre", "kind", "mood", "region", "stream"}

// Values of AdminRadioQuery.State.
const (
	adminStateReachable = "reachable"
	adminStateBroken    = "broken"
	adminStateHidden    = "hidden"
)

// AdminStations lists the stations for the administration; Exceptions keeps the changed (a lock),
// hidden, broken and recommended ones.
func (s *Service) AdminStations(ctx context.Context, q model.AdminRadioQuery) (model.AdminRadioStationPage, error) {
	switch q.State {
	case "", adminStateReachable, adminStateBroken, adminStateHidden:
	default:
		return model.AdminRadioStationPage{}, (*model.ValidationError)(nil).Add("state", "invalid_value")
	}
	idx, err := s.index(ctx)
	if err != nil {
		return model.AdminRadioStationPage{}, err
	}
	search := rules.Fold(q.Q)
	genre := facetBits(rules.FacetGenre, []string{q.Genre})
	var hits []*indexEntry
	for _, e := range idx.entries {
		switch {
		case search != "" && !strings.Contains(e.search, search),
			q.Country != "" && e.st.Country != q.Country,
			q.Genre != "" && e.genres&genre == 0,
			q.State == adminStateReachable && (e.broken || e.hidden),
			q.State == adminStateBroken && !e.broken,
			q.State == adminStateHidden && !e.hidden,
			q.Exceptions && !e.locked && !e.hidden && !e.broken && !e.st.Recommended:
			continue
		}
		hits = append(hits, e)
	}
	sortEntries(hits, model.RadioSortName, nil)
	page := model.AdminRadioStationPage{Total: len(hits), Offset: q.Offset, Items: []model.AdminRadioStation{}}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if q.Offset >= len(hits) {
		return page, nil
	}
	for _, e := range hits[q.Offset:min(len(hits), q.Offset+limit)] {
		a, err := s.AdminStation(ctx, e.st.ID)
		if errors.Is(err, model.ErrNotFound) {
			continue // deleted since the index pass
		}
		if err != nil {
			return model.AdminRadioStationPage{}, err
		}
		page.Items = append(page.Items, a)
	}
	return page, nil
}

// AdminStation returns one station with its streams (best first), tags, facets and locks.
func (s *Service) AdminStation(ctx context.Context, stationID int64) (model.AdminRadioStation, error) {
	return adminStation(ctx, appq.New(s.db.Read), stationID)
}

func adminStation(ctx context.Context, q *appq.Queries, id int64) (model.AdminRadioStation, error) {
	entries, err := loadEntries(ctx, q, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return model.AdminRadioStation{}, err
	}
	if len(entries) == 0 {
		return model.AdminRadioStation{}, model.ErrNotFound
	}
	row, err := q.GetRadioStation(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AdminRadioStation{}, model.ErrNotFound
	}
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("get radio station: %w", err)
	}
	a := model.AdminRadioStation{RadioStation: entries[0].st, Source: row.Source, Hidden: row.Hidden == 1,
		Failures: int(row.Failures), LogoState: row.LogoState, Tags: []string{}, Facets: []model.RadioStationFacet{},
		Locks: []string{}, Streams: []model.RadioStream{},
		CreatedAt: time.UnixMilli(row.CreatedAt).UTC(), UpdatedAt: time.UnixMilli(row.UpdatedAt).UTC()}
	if row.BrokenAt.Valid {
		a.BrokenAt = time.UnixMilli(row.BrokenAt.Int64).UTC()
	}
	tags, err := q.ListRadioStationTags(ctx, id)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("list radio tags: %w", err)
	}
	a.Tags = append(a.Tags, tags...)
	facets, err := q.ListRadioStationFacetsByStation(ctx, id)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("list radio facets: %w", err)
	}
	for _, f := range facets {
		a.Facets = append(a.Facets, model.RadioStationFacet{Facet: f.Facet, Value: f.Value, Manual: f.Manual == 1})
	}
	if a.Locks, err = q.ListRadioStationLocks(ctx, id); err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("list radio locks: %w", err)
	}
	if a.Locks == nil {
		a.Locks = []string{}
	}
	n, err := q.CountRadioStationAliases(ctx, id)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("count radio aliases: %w", err)
	}
	a.AliasCount = int(n)
	streams, err := q.ListRadioStreamsRanked(ctx, id)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("list radio streams: %w", err)
	}
	for i, st := range streams {
		a.Streams = append(a.Streams, model.RadioStream{ID: st.ID, URL: st.Url, URLResolved: st.UrlResolved.String,
			Codec: st.Codec.String, BitrateKbps: int(st.Bitrate.Int64), HLS: st.Hls == 1, Reachable: st.Lastcheckok == 1,
			Preferred: st.Preferred == 1, Rank: i})
	}
	return a, nil
}

// UpdateStation applies a patch: a set field locks itself; Locks, when set, replaces the locks
// before the fields of the patch add theirs.
func (s *Service) UpdateStation(ctx context.Context, stationID int64, p model.AdminRadioStationPatch) (model.AdminRadioStation, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("update radio station: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	row, err := q.GetRadioStation(ctx, stationID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AdminRadioStation{}, model.ErrNotFound
	}
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("get radio station: %w", err)
	}
	curLocks, err := q.ListRadioStationLocks(ctx, stationID)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("list radio locks: %w", err)
	}
	locks := map[string]bool{}
	for _, l := range curLocks {
		locks[l] = true
	}
	var verr *model.ValidationError
	if p.Locks != nil {
		locks = map[string]bool{}
		for _, l := range *p.Locks {
			if !slices.Contains(lockFields, l) {
				verr = verr.Add("locks", "invalid_value")
				break
			}
			locks[l] = true
		}
	}
	upd := appq.UpdateRadioStationAdminParams{Name: row.Name, Region: row.Region, Hidden: row.Hidden,
		Recommended: row.Recommended, Now: s.now().UnixMilli(), ID: stationID}
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if n := utf8.RuneCountInString(name); n < 1 || n > syncNameMax {
			verr = verr.Add("name", "invalid_value")
		}
		upd.Name, locks["name"] = name, true
	}
	switch {
	case p.ClearRegion:
		upd.Region, locks["region"] = sql.NullString{}, true
	case p.Region != nil:
		if !isoRegion.MatchString(*p.Region) {
			verr = verr.Add("region", "invalid_value")
		}
		upd.Region, locks["region"] = sql.NullString{String: *p.Region, Valid: true}, true
	}
	facetPatch := map[string]*[]string{rules.FacetGenre: p.Genres, rules.FacetKind: p.Kinds, rules.FacetMood: p.Moods}
	for _, facet := range facetNames {
		vals := facetPatch[facet]
		if vals == nil {
			continue
		}
		for _, v := range *vals {
			if !knownFacetValue(facet, v) {
				verr = verr.Add(facet+"s", "invalid_value")
				break
			}
		}
		locks[facet] = true
	}
	if p.PreferredStreamID != nil {
		streams, err := q.ListRadioStreamsRanked(ctx, stationID)
		if err != nil {
			return model.AdminRadioStation{}, fmt.Errorf("list radio streams: %w", err)
		}
		if !slices.ContainsFunc(streams, func(st appq.RadioStream) bool { return st.ID == *p.PreferredStreamID }) {
			verr = verr.Add("preferredStreamId", "invalid_value")
		}
		locks["stream"] = true
	}
	if p.ClearPreferredStream {
		delete(locks, "stream")
	}
	if p.Hidden != nil {
		upd.Hidden = flag01(*p.Hidden)
	}
	if p.Recommended != nil {
		upd.Recommended = flag01(*p.Recommended)
	}
	if verr != nil {
		return model.AdminRadioStation{}, verr
	}
	tags, err := q.ListRadioStationTags(ctx, stationID)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("list radio tags: %w", err)
	}
	upd.SortKey, upd.SearchText = rules.Fold(upd.Name), searchText(upd.Name, upd.Region.String, tags)
	if err := q.UpdateRadioStationAdmin(ctx, upd); err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("update radio station: %w", err)
	}
	var patched []string
	values := map[string][]string{}
	for _, facet := range facetNames {
		if vals := facetPatch[facet]; vals != nil {
			patched, values[facet] = append(patched, facet), *vals
		}
	}
	if err := writeFacets(ctx, q, stationID, values, patched, true, true); err != nil {
		return model.AdminRadioStation{}, err
	}
	switch {
	case p.ClearPreferredStream:
		if err := q.SetRadioStreamPreferred(ctx, appq.SetRadioStreamPreferredParams{StationID: stationID}); err != nil {
			return model.AdminRadioStation{}, fmt.Errorf("clear preferred radio stream: %w", err)
		}
	case p.PreferredStreamID != nil:
		if err := q.SetRadioStreamPreferred(ctx, appq.SetRadioStreamPreferredParams{StationID: stationID,
			StreamID: sql.NullInt64{Int64: *p.PreferredStreamID, Valid: true}}); err != nil {
			return model.AdminRadioStation{}, fmt.Errorf("set preferred radio stream: %w", err)
		}
	}
	if err := q.DeleteRadioStationLocks(ctx, stationID); err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("delete radio locks: %w", err)
	}
	for _, l := range lockFields {
		if !locks[l] {
			continue
		}
		if err := q.InsertRadioStationLock(ctx, appq.InsertRadioStationLockParams{StationID: stationID, Field: l}); err != nil {
			return model.AdminRadioStation{}, fmt.Errorf("insert radio lock: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("update radio station: commit: %w", err)
	}
	s.reloadStation(ctx, stationID)
	s.closeUnreachable(ctx, stationID)
	return s.AdminStation(ctx, stationID)
}

// SetLogo stores an uploaded logo and locks the field logo (logo.go setOwnLogo).
func (s *Service) SetLogo(ctx context.Context, stationID int64, r io.Reader) (model.AdminRadioStation, error) {
	if err := s.setOwnLogo(ctx, stationID, r); err != nil {
		return model.AdminRadioStation{}, err
	}
	return s.AdminStation(ctx, stationID)
}

// DeleteLogo removes an uploaded logo and lifts the lock, so the logo chain runs again.
func (s *Service) DeleteLogo(ctx context.Context, stationID int64) (model.AdminRadioStation, error) {
	if err := s.deleteOwnLogo(ctx, stationID); err != nil {
		return model.AdminRadioStation{}, err
	}
	return s.AdminStation(ctx, stationID)
}

const (
	// probeTimeout bounds a whole probe (contract probeRadioStation).
	probeTimeout = 10 * time.Second
	// probeTitleWait is how long a probe reads the stream for a first title.
	probeTitleWait = 3 * time.Second
)

// Probe opens a stream of the station (streamID 0: the best ranked) through the guarded client
// and reports format, codec, bitrate and the first title within probeTimeout. Hidden stations
// may be probed (administration).
func (s *Service) Probe(ctx context.Context, stationID, streamID int64) (model.RadioProbe, error) {
	q := appq.New(s.db.Read)
	if _, err := q.GetRadioStation(ctx, stationID); errors.Is(err, sql.ErrNoRows) {
		return model.RadioProbe{}, model.ErrNotFound
	} else if err != nil {
		return model.RadioProbe{}, fmt.Errorf("get radio station: %w", err)
	}
	streams, err := q.ListRadioStreamsRanked(ctx, stationID)
	if err != nil {
		return model.RadioProbe{}, fmt.Errorf("list radio streams: %w", err)
	}
	i := 0
	if streamID != 0 {
		i = slices.IndexFunc(streams, func(st appq.RadioStream) bool { return st.ID == streamID })
	}
	if i < 0 || len(streams) == 0 {
		return model.RadioProbe{}, model.ErrNotFound
	}
	target := streams[i].Url
	if streams[i].UrlResolved.Valid && streams[i].UrlResolved.String != "" {
		target = streams[i].UrlResolved.String
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	started := time.Now()
	st, err := s.netClient().openStream(ctx, target)
	if err != nil {
		if ctx.Err() != nil {
			return model.RadioProbe{}, fmt.Errorf("%w: no answer within %v", model.ErrRadioUnreachable, probeTimeout)
		}
		return model.RadioProbe{}, err
	}
	defer func() { _ = st.Close() }()
	info := st.Info()
	p := model.RadioProbe{Format: info.Input.Format, Codec: info.Input.Codec, BitrateKbps: info.Input.BitrateKbps,
		SampleRate: info.Input.SampleRate, Channels: info.Input.Channels, HasMetadata: info.MetaInt > 0,
		Response: time.Since(started)}
	if p.BitrateKbps == 0 {
		p.BitrateKbps = info.ICYBitrate
	}
	wait, stop := context.WithTimeout(ctx, probeTitleWait)
	defer stop()
	for {
		ev, err := st.Next(wait)
		if err != nil {
			break // the wait ended or the stream broke: the probe reports what it saw
		}
		if ev.TitleSet && ev.Title != "" {
			p.Title, p.HasMetadata = ev.Title, true
			break
		}
	}
	return p, nil
}
