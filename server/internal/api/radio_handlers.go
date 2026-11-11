package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Webradio JSON operations of concept 16 stage 1 (contract 1.17.0, ADR 0025): the station list,
// facets, favourites, preferences and listen sessions of the active profile and the radio
// administration, through the radio service (Options.Radio; nil answers 501). No response carries
// an ItemRef; a station the profile may not reach answers 404 from the service.

// Page bounds of the radio lists (contract listRadioStations, listAdminRadioStations).
const (
	defaultRadioPage      = 60
	maxRadioPage          = 120
	defaultAdminRadioPage = 50
	maxAdminRadioPage     = 200
)

// Nullable fields of the station PATCH body, by their contract names (pickUpdateNulls).
var radioStationNullFields = []string{"region", "preferredStreamId", "homepage", "country"}

// defaultOfferListPage is the page of the preview list without a limit (contract
// RadioOfferListRequest.limit).
const defaultOfferListPage = 100

func (e serviceProblem) VisitListRadioStationsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetRadioFacetsResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetRadioStationResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListRadioFavoritesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetRadioFavoriteOrderResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitAddRadioFavoriteResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRemoveRadioFavoriteResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitStartRadioListenResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRadioListenHeartbeatResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitEndRadioListenResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetRadioNowResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitGetRadioPreferencesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetRadioPreferencesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetRadioConfigResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdateRadioConfigResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetRadioCatalogResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSyncRadioCatalogResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListAdminRadioStationsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetAdminRadioStationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateAdminRadioStationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitProbeRadioStationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteRadioStationLogoResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListRadioRelaysResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitStopRadioListenerResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitCreateRadioStationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteRadioStationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetRadioOfferResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitPreviewRadioOfferResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitApplyRadioOfferResponse(w http.ResponseWriter) error { return e.visit(w) }

// radioListenEnded answers a heartbeat whose relay ended: 410 radio_unreachable (the contract's
// heartbeat code; elsewhere model.ErrRadioUnreachable is 502).
type radioListenEnded struct{ serviceProblem }

func (e radioListenEnded) VisitRadioListenHeartbeatResponse(w http.ResponseWriter) error {
	e.p.write(w, e.r, http.StatusGone, "radio_unreachable", "The relay of the listen session ended; start it again.")
	return nil
}

// radioSession returns the session of a radio operation; ok is false on a server without the
// radio service, which answers 501.
func (s *server) radioSession(ctx context.Context) (session model.Session, ok bool, err error) {
	session, found := sessionFrom(ctx)
	if !found {
		return model.Session{}, false, errNoSession
	}
	return session, s.opts.Radio != nil, nil
}

// radioFilter maps the filter parameters of listRadioStations and getRadioFacets.
func radioFilter(q *string, country, region, language, genre, kind, mood *[]string, quality *[]gen.RadioQuality,
	titles, favorites *bool) model.RadioFilter {
	f := model.RadioFilter{Q: deref(q), Countries: deref(country), Regions: deref(region), Languages: deref(language),
		Genres: deref(genre), Kinds: deref(kind), Moods: deref(mood), Titles: deref(titles), Favorites: deref(favorites)}
	for _, v := range deref(quality) {
		f.Qualities = append(f.Qualities, string(v))
	}
	return f
}

// fromRadioFilter maps the contract's RadioFilter object (preferences).
func fromRadioFilter(f *gen.RadioFilter) *model.RadioFilter {
	if f == nil {
		return nil
	}
	out := radioFilter(f.Q, f.Country, f.Region, f.Language, f.Genre, f.Kind, f.Mood, f.Quality, f.Titles, f.Favorites)
	return &out
}

// toRadioFilter maps a stored filter to the contract object; empty lists stay absent.
func toRadioFilter(f *model.RadioFilter) *gen.RadioFilter {
	if f == nil {
		return nil
	}
	list := func(v []string) *[]string {
		if len(v) == 0 {
			return nil
		}
		return &v
	}
	out := &gen.RadioFilter{Country: list(f.Countries), Region: list(f.Regions), Language: list(f.Languages),
		Genre: list(f.Genres), Kind: list(f.Kinds), Mood: list(f.Moods)}
	if f.Q != "" {
		out.Q = &f.Q
	}
	if len(f.Qualities) > 0 {
		q := make([]gen.RadioQuality, 0, len(f.Qualities))
		for _, v := range f.Qualities {
			q = append(q, gen.RadioQuality(v))
		}
		out.Quality = &q
	}
	if f.Titles {
		out.Titles = &f.Titles
	}
	if f.Favorites {
		out.Favorites = &f.Favorites
	}
	return out
}

// radioPage checks offset and limit of a radio list; def and maxLimit are the contract bounds.
func radioPage(offset, limit *int, def, maxLimit int) (int, int, error) {
	var verr *model.ValidationError
	o, l := deref(offset), def
	if limit != nil {
		l = *limit
	}
	if o < 0 {
		verr = verr.Add("offset", "invalid_value")
	}
	if l < 1 || l > maxLimit {
		verr = verr.Add("limit", "invalid_value")
	}
	if verr != nil {
		return 0, 0, verr
	}
	return o, l, nil
}

// radioLogoURL is the image URL of a station logo in w256 with the content hash as v; nil without
// a logo (the client shows the initials).
func radioLogoURL(path string, id int64, hash string) *gen.RadioLogo {
	if hash == "" {
		return nil
	}
	return &gen.RadioLogo{Url: fmt.Sprintf("%s%s/%d/logo/w256?v=%s", BasePath, path, id, url.QueryEscape(hash))}
}

// optMs maps a time to Unix ms, the zero time (absent) to nil.
func optMs(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

// strings0 keeps a required list of the contract from becoming null.
func strings0(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func toRadioStation(st model.RadioStation) gen.RadioStation {
	out := gen.RadioStation{Id: st.ID, Name: st.Name, Country: optString(st.Country), Region: optString(st.Region),
		Languages: strings0(st.Languages), Genres: strings0(st.Genres), Kinds: strings0(st.Kinds), Moods: strings0(st.Moods),
		Codec: optString(st.Codec), BitrateKbps: optPositive(st.BitrateKbps), Hls: st.HLS, ShowsTitles: st.ShowsTitles,
		Favorite: st.Favorite, Recommended: st.Recommended, State: gen.RadioStationState(st.State),
		MissingSince: optMs(st.MissingSince), RemovalAt: optMs(st.RemovalAt),
		Logo: radioLogoURL("/radio/images", st.ID, st.LogoHash), Homepage: optString(st.Homepage),
		Provider: optString(st.Provider)}
	if st.Quality != "" {
		q := gen.RadioQuality(st.Quality)
		out.Quality = &q
	}
	return out
}

func toRadioStations(list []model.RadioStation) []gen.RadioStation {
	out := make([]gen.RadioStation, 0, len(list))
	for _, st := range list {
		out = append(out, toRadioStation(st))
	}
	return out
}

func toRadioFacetCounts(list []model.RadioFacetCount) []gen.RadioFacetCount {
	out := make([]gen.RadioFacetCount, 0, len(list))
	for _, c := range list {
		out = append(out, gen.RadioFacetCount{Value: c.Value, Count: c.Count})
	}
	return out
}

func toAdminRadioStation(a model.AdminRadioStation) gen.AdminRadioStation {
	st := toRadioStation(a.RadioStation)
	out := gen.AdminRadioStation{Id: st.Id, Name: st.Name, Country: st.Country, Region: st.Region, Languages: st.Languages,
		Genres: st.Genres, Kinds: st.Kinds, Moods: st.Moods, Quality: st.Quality, Codec: st.Codec, BitrateKbps: st.BitrateKbps,
		Hls: st.Hls, ShowsTitles: st.ShowsTitles, Recommended: st.Recommended, State: st.State,
		MissingSince: st.MissingSince, RemovalAt: st.RemovalAt, Homepage: st.Homepage,
		Logo:   radioLogoURL("/admin/radio/images", a.ID, a.LogoHash),
		Source: gen.RadioStationSource(a.Source), Offered: a.Offered, Provider: optString(a.Provider),
		Failures: a.Failures, BrokenAt: optMs(a.BrokenAt), Tags: strings0(a.Tags),
		Locks: make([]gen.RadioLockField, 0, len(a.Locks)), LogoState: gen.RadioLogoState(a.LogoState),
		AliasCount: a.AliasCount, Streams: make([]gen.RadioStream, 0, len(a.Streams)),
		CreatedAt: a.CreatedAt.UnixMilli(), UpdatedAt: a.UpdatedAt.UnixMilli()}
	for _, l := range a.Locks {
		out.Locks = append(out.Locks, gen.RadioLockField(l))
	}
	for _, s := range a.Streams {
		out.Streams = append(out.Streams, gen.RadioStream{Id: s.ID, Url: s.URL, UrlResolved: optString(s.URLResolved),
			Codec: optString(s.Codec), BitrateKbps: optPositive(s.BitrateKbps), Hls: s.HLS, Reachable: s.Reachable,
			Preferred: s.Preferred, Rank: s.Rank})
	}
	src := func(v string) *gen.RadioValueSource {
		if v == "" {
			return nil
		}
		x := gen.RadioValueSource(v)
		return &x
	}
	out.Sources = gen.AdminRadioStationSources{Name: src(a.Sources.Name), Country: src(a.Sources.Country),
		Region: src(a.Sources.Region), Languages: src(a.Sources.Languages), Homepage: src(a.Sources.Homepage),
		Logo: src(a.Sources.Logo), Provider: src(a.Sources.Provider)}
	if w := a.Wikidata; w != nil {
		out.Wikidata = &gen.AdminRadioWikidata{Id: w.QID, OwnerId: optString(w.OwnerQID), Owner: optString(w.Owner),
			Founded: optString(w.Founded), FetchedAt: w.FetchedAt.UnixMilli()}
	}
	return out
}

func toRadioConfig(c model.RadioConfig) gen.RadioConfig {
	return gen.RadioConfig{BufferMinutes: gen.RadioConfigBufferMinutes(c.BufferMinutes), BudgetMb: c.BudgetMB,
		MaxRelays: c.MaxRelays, GraceSeconds: c.GraceSeconds, BufferMode: gen.RadioBufferMode(c.BufferMode), MaxRelaysByBudget: c.MaxRelaysByBudget,
		CoverMusicbrainz: c.CoverMusicBrainz, CoverItunes: c.CoverITunes, WikidataEnabled: c.WikidataEnabled}
}

// radioCoverURL is the URL of getRadioCover (w256) for a title's cover, nil without one.
func radioCoverURL(trackID int64, hash string) *gen.RadioCover {
	if trackID == 0 || hash == "" {
		return nil
	}
	return &gen.RadioCover{Url: fmt.Sprintf("%s/radio/covers/%d/w256?v=%s", BasePath, trackID, url.QueryEscape(hash))}
}

func toRadioNowEntry(e model.RadioNowEntry) gen.RadioNowEntry {
	return gen.RadioNowEntry{At: e.At.UnixMilli(), Text: e.Text, Artist: optString(e.Artist), Title: optString(e.Title),
		Cover: radioCoverURL(e.CoverTrackID, e.CoverHash)}
}

// toRadioHomeStations maps the tiles of the start page row radio («Meine Sender», contract 1.19.0).
func toRadioHomeStations(list []model.RadioHomeStation) []gen.RadioHomeStation {
	out := make([]gen.RadioHomeStation, 0, len(list))
	for _, h := range list {
		tile := gen.RadioHomeStation{Station: toRadioStation(h.Station)}
		if h.Now != nil {
			now := toRadioNowEntry(*h.Now)
			tile.Now = &now
		}
		out = append(out, tile)
	}
	return out
}

func toRadioPreferences(p model.RadioPreferences) gen.RadioPreferences {
	out := gen.RadioPreferences{DefaultCountry: p.DefaultCountry, EffectiveCountry: optString(p.EffectiveCountry),
		LastFilter: toRadioFilter(p.LastFilter)}
	if p.LastSort != "" {
		sort := gen.RadioSort(p.LastSort)
		out.LastSort = &sort
	}
	return out
}

func (s *server) ListRadioStations(ctx context.Context, req gen.ListRadioStationsRequestObject) (gen.ListRadioStationsResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	p := req.Params
	offset, limit, err := radioPage(p.Offset, p.Limit, defaultRadioPage, maxRadioPage)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	q := model.RadioQuery{RadioFilter: radioFilter(p.Q, p.Country, p.Region, p.Language, p.Genre, p.Kind, p.Mood, p.Quality, p.Titles, p.Favorites),
		Sort: string(deref(p.Sort)), Offset: offset, Limit: limit}
	page, err := s.opts.Radio.ListStations(ctx, session, q)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListRadioStations200JSONResponse{Items: toRadioStations(page.Items), Total: page.Total, Offset: page.Offset}, nil
}

func (s *server) GetRadioFacets(ctx context.Context, req gen.GetRadioFacetsRequestObject) (gen.GetRadioFacetsResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	p := req.Params
	q := model.RadioQuery{RadioFilter: radioFilter(p.Q, p.Country, p.Region, p.Language, p.Genre, p.Kind, p.Mood, p.Quality, p.Titles, p.Favorites)}
	f, err := s.opts.Radio.Facets(ctx, session, q)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetRadioFacets200JSONResponse{Country: toRadioFacetCounts(f.Countries), Region: toRadioFacetCounts(f.Regions),
		Language: toRadioFacetCounts(f.Languages), Genre: toRadioFacetCounts(f.Genres), Kind: toRadioFacetCounts(f.Kinds),
		Mood: toRadioFacetCounts(f.Moods), Quality: toRadioFacetCounts(f.Qualities), Titles: f.Titles,
		OfferEmpty: f.OfferEmpty}, nil
}

func (s *server) GetRadioStation(ctx context.Context, req gen.GetRadioStationRequestObject) (gen.GetRadioStationResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	st, err := s.opts.Radio.Station(ctx, session, req.StationId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetRadioStation200JSONResponse(toRadioStation(st)), nil
}

func (s *server) ListRadioFavorites(ctx context.Context, _ gen.ListRadioFavoritesRequestObject) (gen.ListRadioFavoritesResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	list, err := s.opts.Radio.Favorites(ctx, session)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListRadioFavorites200JSONResponse(toRadioStations(list)), nil
}

func (s *server) SetRadioFavoriteOrder(ctx context.Context, req gen.SetRadioFavoriteOrderRequestObject) (gen.SetRadioFavoriteOrderResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("stationIds", "required")), nil
	}
	if err := s.opts.Radio.SetFavoriteOrder(ctx, session, req.Body.StationIds); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetRadioFavoriteOrder204Response{}, nil
}

func (s *server) AddRadioFavorite(ctx context.Context, req gen.AddRadioFavoriteRequestObject) (gen.AddRadioFavoriteResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Radio.AddFavorite(ctx, session, req.StationId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.AddRadioFavorite204Response{}, nil
}

func (s *server) RemoveRadioFavorite(ctx context.Context, req gen.RemoveRadioFavoriteRequestObject) (gen.RemoveRadioFavoriteResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Radio.RemoveFavorite(ctx, session, req.StationId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RemoveRadioFavorite204Response{}, nil
}

func (s *server) StartRadioListen(ctx context.Context, req gen.StartRadioListenRequestObject) (gen.StartRadioListenResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	l, err := s.opts.Radio.Listen(ctx, session, req.StationId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	playlist := BasePath + "/radio/live/" + url.PathEscape(l.ListenID) + "/index.m3u8?rt=" + url.QueryEscape(l.Token)
	return gen.StartRadioListen201JSONResponse{ListenId: l.ListenID, PlaylistUrl: playlist, HeartbeatSeconds: l.HeartbeatSeconds,
		StartOffsetSeconds: l.StartOffsetSeconds, TimeShiftSeconds: l.TimeShiftSeconds, Station: toRadioStation(l.Station)}, nil
}

func (s *server) RadioListenHeartbeat(ctx context.Context, req gen.RadioListenHeartbeatRequestObject) (gen.RadioListenHeartbeatResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("playing", "required")), nil
	}
	hb, err := s.opts.Radio.Heartbeat(ctx, session, req.ListenId, req.Body.Playing)
	if errors.Is(err, model.ErrRadioUnreachable) {
		return radioListenEnded{s.problem(ctx, err)}, nil
	}
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RadioListenHeartbeat200JSONResponse{ExpiresAt: hb.ExpiresAt.UnixMilli(), WatchTime: toWatchTime(hb.WatchTime)}, nil
}

func (s *server) EndRadioListen(ctx context.Context, req gen.EndRadioListenRequestObject) (gen.EndRadioListenResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Radio.EndListen(ctx, session, req.ListenId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.EndRadioListen204Response{}, nil
}

func (s *server) GetRadioNow(ctx context.Context, req gen.GetRadioNowRequestObject) (gen.GetRadioNowResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	var since time.Time
	if req.Params.Since != nil {
		since = time.UnixMilli(*req.Params.Since)
	}
	now, err := s.opts.Radio.Now(ctx, session, req.StationId, since)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetRadioNow200JSONResponse{Live: now.Live, Entries: make([]gen.RadioNowEntry, 0, len(now.Entries))}
	for _, e := range now.Entries {
		out.Entries = append(out.Entries, toRadioNowEntry(e))
	}
	return out, nil
}

func (s *server) GetRadioPreferences(ctx context.Context, _ gen.GetRadioPreferencesRequestObject) (gen.GetRadioPreferencesResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	p, err := s.opts.Radio.Preferences(ctx, session)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetRadioPreferences200JSONResponse(toRadioPreferences(p)), nil
}

func (s *server) SetRadioPreferences(ctx context.Context, req gen.SetRadioPreferencesRequestObject) (gen.SetRadioPreferencesResponseObject, error) {
	session, ok, err := s.radioSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("defaultCountry", "required")), nil
	}
	in := model.RadioPreferences{DefaultCountry: req.Body.DefaultCountry, LastFilter: fromRadioFilter(req.Body.LastFilter),
		LastSort: string(deref(req.Body.LastSort))}
	p, err := s.opts.Radio.SetPreferences(ctx, session, in)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetRadioPreferences200JSONResponse(toRadioPreferences(p)), nil
}

// The administration (x-visibility admin): the session middleware let only an administrator with
// the primary profile through.

func (s *server) GetRadioConfig(ctx context.Context, _ gen.GetRadioConfigRequestObject) (gen.GetRadioConfigResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	c, err := s.opts.Radio.Config(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetRadioConfig200JSONResponse(toRadioConfig(c)), nil
}

func (s *server) UpdateRadioConfig(ctx context.Context, req gen.UpdateRadioConfigRequestObject) (gen.UpdateRadioConfigResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	b := req.Body
	upd := model.RadioConfigUpdate{BudgetMB: b.BudgetMb, MaxRelays: b.MaxRelays, GraceSeconds: b.GraceSeconds,
		CoverMusicBrainz: b.CoverMusicbrainz, CoverITunes: b.CoverItunes, WikidataEnabled: b.WikidataEnabled}
	if b.BufferMinutes != nil {
		m := int(*b.BufferMinutes)
		upd.BufferMinutes = &m
	}
	c, err := s.opts.Radio.UpdateConfig(ctx, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateRadioConfig200JSONResponse(toRadioConfig(c)), nil
}

func (s *server) GetRadioCatalog(ctx context.Context, _ gen.GetRadioCatalogRequestObject) (gen.GetRadioCatalogResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	c, err := s.opts.Radio.CatalogState(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetRadioCatalog200JSONResponse{Source: gen.RadioCatalogStateSource(c.Source), LastSyncAt: optMs(c.LastSyncAt),
		LastSyncError: optString(c.LastSyncError), SyncFailures: c.SyncFailures, EntriesTotal: c.EntriesTotal,
		EntriesReachable: c.EntriesReachable, StationsTotal: c.StationsTotal, LastAdded: c.LastAdded,
		LastRemoved: c.LastRemoved, NextRunAt: optMs(c.NextRunAt)}
	if c.RunningJobID != 0 {
		out.RunningJobId = &c.RunningJobID
	}
	return out, nil
}

func (s *server) SyncRadioCatalog(ctx context.Context, _ gen.SyncRadioCatalogRequestObject) (gen.SyncRadioCatalogResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	id, err := s.opts.Radio.SyncCatalog(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SyncRadioCatalog202JSONResponse{JobId: id}, nil
}

func (s *server) ListAdminRadioStations(ctx context.Context, req gen.ListAdminRadioStationsRequestObject) (gen.ListAdminRadioStationsResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	p := req.Params
	offset, limit, err := radioPage(p.Offset, p.Limit, defaultAdminRadioPage, maxAdminRadioPage)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	offered := true
	if p.Offered != nil {
		offered = *p.Offered
	}
	q := model.AdminRadioQuery{Q: deref(p.Q), Country: deref(p.Country), Provider: deref(p.Provider),
		State: string(deref(p.State)), Source: string(deref(p.Source)), Offered: offered, Sort: string(deref(p.Sort)),
		Offset: offset, Limit: limit}
	page, err := s.opts.Radio.AdminStations(ctx, q)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListAdminRadioStations200JSONResponse{Items: make([]gen.AdminRadioStation, 0, len(page.Items)),
		Total: page.Total, Offset: page.Offset}
	for _, a := range page.Items {
		out.Items = append(out.Items, toAdminRadioStation(a))
	}
	return out, nil
}

func (s *server) GetAdminRadioStation(ctx context.Context, req gen.GetAdminRadioStationRequestObject) (gen.GetAdminRadioStationResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	a, err := s.opts.Radio.AdminStation(ctx, req.StationId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetAdminRadioStation200JSONResponse(toAdminRadioStation(a)), nil
}

func (s *server) UpdateAdminRadioStation(ctx context.Context, req gen.UpdateAdminRadioStationRequestObject) (gen.UpdateAdminRadioStationResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	b := req.Body
	nulls, _ := ctx.Value(pickNullsKey{}).(map[string]bool)
	patch := model.AdminRadioStationPatch{Name: b.Name, Region: b.Region, ClearRegion: nulls["region"],
		PreferredStreamID: b.PreferredStreamId, ClearPreferredStream: nulls["preferredStreamId"],
		Recommended: b.Recommended, StreamURL: b.StreamUrl, Homepage: b.Homepage,
		ClearHomepage: nulls["homepage"], Country: b.Country, ClearCountry: nulls["country"], Languages: b.Languages}
	if b.Locks != nil {
		locks := make([]string, 0, len(*b.Locks))
		for _, l := range *b.Locks {
			locks = append(locks, string(l))
		}
		patch.Locks = &locks
	}
	a, err := s.opts.Radio.UpdateStation(ctx, req.StationId, patch)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateAdminRadioStation200JSONResponse(toAdminRadioStation(a)), nil
}

func (s *server) ProbeRadioStation(ctx context.Context, req gen.ProbeRadioStationRequestObject) (gen.ProbeRadioStationResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	var streamID int64
	if req.Body != nil {
		streamID = deref(req.Body.StreamId)
	}
	p, err := s.opts.Radio.Probe(ctx, req.StationId, streamID)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ProbeRadioStation200JSONResponse{Format: p.Format, Codec: p.Codec, BitrateKbps: p.BitrateKbps,
		SampleRate: p.SampleRate, Channels: p.Channels, HasMetadata: p.HasMetadata, Title: optString(p.Title),
		ResponseMs: int(p.Response.Milliseconds())}, nil
}

func (s *server) DeleteRadioStationLogo(ctx context.Context, req gen.DeleteRadioStationLogoRequestObject) (gen.DeleteRadioStationLogoResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	a, err := s.opts.Radio.DeleteLogo(ctx, req.StationId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteRadioStationLogo200JSONResponse(toAdminRadioStation(a)), nil
}

// CreateRadioStation adds an own station (contract 1.19.0, concept 16 section 5.6).
func (s *server) CreateRadioStation(ctx context.Context, req gen.CreateRadioStationRequestObject) (gen.CreateRadioStationResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	b := req.Body
	c := model.AdminRadioStationCreate{Name: b.Name, StreamURL: b.StreamUrl, Homepage: deref(b.Homepage), Country: deref(b.Country),
		Region: deref(b.Region), Languages: deref(b.Languages)}
	a, err := s.opts.Radio.CreateStation(ctx, c)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateRadioStation201JSONResponse(toAdminRadioStation(a)), nil
}

// DeleteRadioStation deletes an own station (contract 1.19.0).
func (s *server) DeleteRadioStation(ctx context.Context, req gen.DeleteRadioStationRequestObject) (gen.DeleteRadioStationResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Radio.DeleteStation(ctx, req.StationId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteRadioStation204Response{}, nil
}

// fromRadioOfferSteps maps the steps of a draft (contract RadioOfferStep).
func fromRadioOfferSteps(list []gen.RadioOfferStep) []model.RadioOfferStep {
	out := make([]model.RadioOfferStep, 0, len(list))
	for _, st := range list {
		out = append(out, model.RadioOfferStep{Op: string(st.Op), Country: deref(st.Country), Q: deref(st.Q),
			StationIDs: deref(st.StationIds), Value: deref(st.Value)})
	}
	return out
}

func toRadioOfferTotals(t model.RadioOfferTotals) gen.RadioOfferTotals {
	return gen.RadioOfferTotals{Offered: t.Offered, OfferedUnreachable: t.OfferedUnreachable, Countries: t.Countries,
		Marked: t.Marked, Demarked: t.Demarked, Own: t.Own, WithLogo: t.WithLogo, RemovableUnreachable: t.RemovableUnreachable}
}

func toRadioOfferCountries(list []model.RadioOfferCountry) []gen.RadioOfferCountry {
	out := make([]gen.RadioOfferCountry, 0, len(list))
	for _, c := range list {
		rc := gen.RadioOfferCountry{Code: c.Code, Continent: gen.RadioContinent(c.Continent), Offered: c.Offered,
			Total: c.Total, Unreachable: c.Unreachable}
		if c.Mode != "" {
			mode := gen.RadioOfferMode(c.Mode)
			rc.Mode = &mode
		}
		out = append(out, rc)
	}
	return out
}

func toRadioOffer(o model.RadioOffer) gen.RadioOffer {
	return gen.RadioOffer{Version: o.Version, HideBroken: o.HideBroken, Totals: toRadioOfferTotals(o.Totals),
		Countries: toRadioOfferCountries(o.Countries)}
}

// GetRadioOffer answers the stored offer (contract 1.28.0, concept 19 part B).
func (s *server) GetRadioOffer(ctx context.Context, _ gen.GetRadioOfferRequestObject) (gen.GetRadioOfferResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	o, err := s.opts.Radio.Offer(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetRadioOffer200JSONResponse(toRadioOffer(o)), nil
}

// PreviewRadioOffer computes a draft without storing it (contract 1.28.0).
func (s *server) PreviewRadioOffer(ctx context.Context, req gen.PreviewRadioOfferRequestObject) (gen.PreviewRadioOfferResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	var list *model.RadioOfferListQuery
	if l := req.Body.List; l != nil {
		list = &model.RadioOfferListQuery{Country: l.Country, Q: deref(l.Q), Offset: deref(l.Offset), Limit: defaultOfferListPage}
		if l.Limit != nil {
			list.Limit = *l.Limit
		}
	}
	p, err := s.opts.Radio.PreviewOffer(ctx, fromRadioOfferSteps(req.Body.Steps), list)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.PreviewRadioOffer200JSONResponse{Version: p.Version, HideBroken: p.HideBroken,
		Totals: toRadioOfferTotals(p.Totals), Diff: gen.RadioOfferDiff{Added: p.Added, Removed: p.Removed},
		Countries: toRadioOfferCountries(p.Countries)}
	if p.List != nil {
		page := &gen.RadioOfferStationPage{Total: p.List.Total, Offset: p.List.Offset,
			Items: make([]gen.RadioOfferStation, 0, len(p.List.Items))}
		for _, it := range p.List.Items {
			page.Items = append(page.Items, gen.RadioOfferStation{Id: it.ID, Name: it.Name,
				Logo: radioLogoURL("/admin/radio/images", it.ID, it.LogoHash), Tags: strings0(it.Tags), Broken: it.Broken,
				Offered: it.Offered})
		}
		out.List = page
	}
	return out, nil
}

// ApplyRadioOffer stores a draft on the version it is based on (contract 1.28.0; 409
// offer_conflict when the offer changed meanwhile).
func (s *server) ApplyRadioOffer(ctx context.Context, req gen.ApplyRadioOfferRequestObject) (gen.ApplyRadioOfferResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	a, err := s.opts.Radio.ApplyOffer(ctx, req.Body.Version, fromRadioOfferSteps(req.Body.Steps))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	ids := a.DemarkedStationIDs
	if ids == nil {
		ids = []int64{}
	}
	return gen.ApplyRadioOffer200JSONResponse{Offer: toRadioOffer(a.Offer), QueuedLogos: a.QueuedLogos,
		DemarkedStationIds: ids}, nil
}

func (s *server) ListRadioRelays(ctx context.Context, _ gen.ListRadioRelaysRequestObject) (gen.ListRadioRelaysResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	rel, err := s.opts.Radio.Relays(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	l := rel.Load
	out := gen.ListRadioRelays200JSONResponse{Relays: make([]gen.RadioRelay, 0, len(rel.Relays)),
		Load: gen.RadioLoad{Relays: l.Relays, MaxRelays: l.MaxRelays, BufferBytes: l.BufferBytes, BudgetBytes: l.BudgetBytes,
			UploadKbps: l.UploadKbps, BufferMode: gen.RadioBufferMode(l.BufferMode)}}
	for _, r := range rel.Relays {
		g := gen.RadioRelay{StationId: r.StationID, Name: r.Name, Codec: optString(r.Codec), BitrateKbps: optPositive(r.BitrateKbps),
			Transcoded: r.Transcoded, StartedAt: r.StartedAt.UnixMilli(), BufferSeconds: r.BufferSeconds,
			BufferBytes: r.BufferBytes, State: gen.RadioRelayState(r.State),
			Listeners: make([]gen.RadioRelayListener, 0, len(r.Listeners)), Logo: radioLogoURL("/admin/radio/images", r.StationID, r.LogoHash)}
		for _, li := range r.Listeners {
			l := gen.RadioRelayListener{ListenId: li.ListenID, ProfileName: li.ProfileName,
				AccountEmail: li.AccountEmail, Playing: li.Playing, Since: li.Since.UnixMilli()}
			if d := li.Device; d != nil {
				l.Device = &struct {
					Id   string  `json:"id"`
					Name *string `json:"name,omitempty"`
				}{Id: d.ID, Name: optString(d.Name)}
			}
			g.Listeners = append(g.Listeners, l)
		}
		out.Relays = append(out.Relays, g)
	}
	return out, nil
}

func (s *server) StopRadioListener(ctx context.Context, req gen.StopRadioListenerRequestObject) (gen.StopRadioListenerResponseObject, error) {
	if s.opts.Radio == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Radio.StopListener(ctx, req.ListenId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.StopRadioListener204Response{}, nil
}
