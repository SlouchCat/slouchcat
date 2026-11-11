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

// The station administration (concept 16 section 5.6; concept 19 part B, B9.3): every station of
// the catalog, those outside the offer included, with its streams, tags and locks; a patch locks
// the fields it sets, so the next sync keeps them, and a lock the admin lifts lets the next sync
// overwrite the field. Since contract 1.28.0 nothing is assigned by hand (B5): the filter values
// derive from the rules file, and a station leaves the offer through an exception (offer.go).

// lockFields are the fields a lock may hold (radio_station_locks.field).
var lockFields = []string{"name", "logo", "region", "stream"}

// Values of AdminRadioQuery.State and Sort.
const (
	adminStateReachable = "reachable"
	adminStateBroken    = "broken"
	adminStateNoLogo    = "noLogo"
	adminSortPopularity = "popularity"
	adminSortName       = "name"
)

// AdminStations lists the stations for the administration: with Offered those of the stored
// offer, else the whole catalog; Provider, State and Source narrow, Sort orders by popularity (the
// default) or name. The list never queues a logo (B7.2).
func (s *Service) AdminStations(ctx context.Context, q model.AdminRadioQuery) (model.AdminRadioStationPage, error) {
	var verr *model.ValidationError
	switch q.State {
	case "", adminStateReachable, adminStateBroken, adminStateNoLogo:
	default:
		verr = verr.Add("state", "invalid_value")
	}
	switch q.Source {
	case "", sourceCatalog, sourceOwn:
	default:
		verr = verr.Add("source", "invalid_value")
	}
	switch q.Sort {
	case "", adminSortPopularity, adminSortName:
	default:
		verr = verr.Add("sort", "invalid_value")
	}
	if verr != nil {
		return model.AdminRadioStationPage{}, verr
	}
	idx, err := s.index(ctx)
	if err != nil {
		return model.AdminRadioStationPage{}, err
	}
	o, err := s.offer(ctx)
	if err != nil {
		return model.AdminRadioStationPage{}, err
	}
	search := rules.Fold(q.Q)
	var hits []*indexEntry
	for _, e := range idx.entries {
		switch {
		case q.Offered && !o.offered(e),
			search != "" && !strings.Contains(e.search, search),
			q.Country != "" && e.st.Country != q.Country,
			q.Provider != "" && e.st.Provider != q.Provider,
			q.State == adminStateReachable && e.broken,
			q.State == adminStateBroken && !e.broken,
			q.State == adminStateNoLogo && e.st.LogoHash != "",
			q.Source == sourceOwn && !e.own,
			q.Source == sourceCatalog && e.own:
			continue
		}
		hits = append(hits, e)
	}
	sortBy := model.RadioSortPopular
	if q.Sort == adminSortName {
		sortBy = model.RadioSortName
	}
	sortEntries(hits, sortBy, nil)
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

// AdminStation returns one station with its streams (best first), tags, locks and whether it is
// in the stored offer.
func (s *Service) AdminStation(ctx context.Context, stationID int64) (model.AdminRadioStation, error) {
	o, err := s.offer(ctx)
	if err != nil {
		return model.AdminRadioStation{}, err
	}
	return adminStation(ctx, appq.New(s.db.Read), o, stationID)
}

func adminStation(ctx context.Context, q *appq.Queries, o *offerState, id int64) (model.AdminRadioStation, error) {
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
	a := model.AdminRadioStation{RadioStation: entries[0].st, Source: row.Source, Offered: o.offered(entries[0]),
		Failures: int(row.Failures), LogoState: row.LogoState, Tags: []string{},
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
	if a.Wikidata, err = stationWikidata(ctx, q, id); err != nil {
		return model.AdminRadioStation{}, err
	}
	a.Sources = stationSources(a)
	return a, nil
}

// stationSources names where each value of a station comes from (B9.3, stage B2): the catalog,
// the own station's entry, the administration (a locked field, an uploaded logo), the best
// stream's host, the homepage or Wikidata.
func stationSources(a model.AdminRadioStation) model.RadioSources {
	locked := func(f string) bool { return slices.Contains(a.Locks, f) }
	base := model.RadioSourceCatalog
	if a.Source == sourceOwn {
		base = model.RadioSourceOwn
	}
	of := func(value bool, lock string) string {
		switch {
		case !value:
			return ""
		case lock != "" && locked(lock):
			return model.RadioSourceAdmin
		}
		return base
	}
	src := model.RadioSources{
		Name: of(a.Name != "", "name"), Country: of(a.Country != "", ""), Region: of(a.Region != "", "region"),
		Languages: of(len(a.Languages) > 0, ""), Homepage: of(a.Homepage != "", ""),
	}
	switch a.LogoState {
	case logoStateCatalog:
		src.Logo = model.RadioSourceCatalog
	case logoStateWebsite:
		src.Logo = model.RadioSourceWebsite
	case logoStateOwn:
		src.Logo = model.RadioSourceAdmin
	}
	switch {
	case a.Provider == "":
	case a.Wikidata != nil && a.Wikidata.Owner != "" && a.Wikidata.Owner == a.Provider:
		src.Provider = model.RadioSourceWikidata
	default:
		src.Provider = model.RadioSourceStream
	}
	return src
}

// UpdateStation applies a patch: a set field locks itself; Locks, when set, replaces the locks
// before the fields of the patch add theirs. Stream address, homepage, country and languages are
// fields of own stations only (own.go, not_own otherwise); a new stream address ends a running
// relay of the station.
func (s *Service) UpdateStation(ctx context.Context, stationID int64, p model.AdminRadioStationPatch) (model.AdminRadioStation, error) {
	own, verr, err := s.checkOwnPatch(ctx, stationID, p)
	if err != nil {
		return model.AdminRadioStation{}, err
	}
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
	upd := appq.UpdateRadioStationAdminParams{Name: row.Name, Region: row.Region,
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
	// No sync derives an own station again: its facets follow its name here, through the rules file
	// (catalog stations get theirs from the next sync).
	if row.Source == sourceOwn && upd.Name != row.Name {
		derived, _ := deriveFacets(rules.Default(), upd.Name, tags)
		if err := writeFacets(ctx, q, stationID, derived, facetNames, false, true); err != nil {
			return model.AdminRadioStation{}, err
		}
	}
	urlChanged, err := writeOwnPatch(ctx, q, stationID, own, upd.Now)
	if err != nil {
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
	// The best stream follows the preferred one while the lock holds, so its host (B7.4) may
	// have changed: the provider is derived again from the first stream of the ranking.
	if p.PreferredStreamID != nil || p.ClearPreferredStream {
		streams, err := q.ListRadioStreamsRanked(ctx, stationID)
		if err != nil {
			return model.AdminRadioStation{}, fmt.Errorf("list radio streams: %w", err)
		}
		provider := ""
		if len(streams) > 0 {
			provider = providerOf(bestURL(streams[0].Url, streams[0].UrlResolved.String))
		}
		if err := q.SetRadioStationProvider(ctx, appq.SetRadioStationProviderParams{Provider: nullText(provider), ID: stationID}); err != nil {
			return model.AdminRadioStation{}, fmt.Errorf("set radio provider: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("update radio station: commit: %w", err)
	}
	s.reloadStation(ctx, stationID)
	if urlChanged {
		s.closeStation(ctx, stationID) // the relay plays the old address
		s.probeOwnLater(stationID, own.streamURL)
	}
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
// and reports format, codec, bitrate and the first title within probeTimeout. Stations outside the
// offer may be probed (administration).
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
