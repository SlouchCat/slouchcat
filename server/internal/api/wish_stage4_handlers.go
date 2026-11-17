package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/integrations"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/wishes"
)

// Wishes through Seerr, stage 4 (concept 25 build step 4.7, contract 1.41.0): the missing seasons
// on the series page, approving and declining, taking over the requests made in Seerr, the instant
// notice of Seerr, Radarr and Sonarr for the profile tags and the progress. The answers of the
// administration carry no ItemRef and no key; the progress of the viewers carries no message, no
// service and no path (N-83).

// WishStage4Service is the part of package wishes for stage 4 (part of WishesService).
type WishStage4Service interface {
	// SeriesCatalogOf is getItem.catalog; nil without the right for series or without a catalog.
	SeriesCatalogOf(ctx context.Context, p model.Profile, seriesID string, tmdbID int64) (*wishes.SeriesCatalog, error)
	// ApproveWish and DeclineWish: model.ErrNotFound, model.ErrWishNotPending,
	// model.ErrInstanceChanged, model.ErrSeerrNotConfigured, model.ErrSeerrUnreachable,
	// model.ErrSeerrFailed.
	ApproveWish(ctx context.Context, wishID, accountID int64) (model.AdminWish, error)
	DeclineWish(ctx context.Context, wishID, accountID int64) (model.AdminWish, error)
	// ImportSeerrRequests queues the take-over; model.ErrSeerrNotConfigured, model.ErrNotFound.
	ImportSeerrRequests(ctx context.Context, seerrUserID *int) (int, error)
	CountSeerrRequests(ctx context.Context, seerrUserID int) (int, error)
	ArrStatus(ctx context.Context) (wishes.ArrStatus, error)
	UpdateArrSettings(ctx context.Context, upd wishes.ArrSettingsUpdate) (wishes.ArrStatus, error)
	TestArrService(ctx context.Context, service string) (wishes.ArrTestResult, error)
	// SetProfileTagName: model.ErrNotFound, *model.ValidationError, wishes.ErrTagNameTaken.
	SetProfileTagName(ctx context.Context, profileID int64, name *string) error
	ProfileTagOf(ctx context.Context, profileID int64) (wishes.ProfileTag, error)
	WishTagsOf(ctx context.Context, wishIDs []int64) (map[int64][]wishes.WishTag, error)
	ProgressOf(ctx context.Context, wishIDs []int64) (map[int64]wishes.WishProgress, error)
	AdminProgressOf(ctx context.Context, wishIDs []int64) (map[int64]wishes.AdminWishProgress, error)
}

func (e serviceProblem) VisitApproveWishResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDeclineWishResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitImportSeerrRequestsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitCountSeerrRequestsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetArrStatusResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdateArrSettingsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitTestArrConnectionResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetProfileTagNameResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetSeerrWebhookSetupResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// itemCatalog adds the season catalog of a series to getItem (absent for other kinds, without
// the right for series and without stored seasons).
func (s *server) itemCatalog(ctx context.Context, p model.Profile, d model.ItemDetail, out *gen.ItemDetail) error {
	if s.opts.Wishes == nil || d.Kind != "series" {
		return nil
	}
	var tmdbID int64
	if d.TMDBID != nil {
		tmdbID = *d.TMDBID
	}
	c, err := s.opts.Wishes.SeriesCatalogOf(ctx, p, d.ID, tmdbID)
	if err != nil || c == nil {
		return err
	}
	cat := gen.SeriesCatalog{InProduction: c.InProduction, Present: c.Present, Total: c.Total, Status: optString(c.Status),
		Seasons:     make([]gen.SeriesCatalogSeason, 0, len(c.Seasons)),
		NextEpisode: toCatalogEpisode(c.NextEpisode), LastEpisode: toCatalogEpisode(c.LastEpisode)}
	for _, se := range c.Seasons {
		cat.Seasons = append(cat.Seasons, gen.SeriesCatalogSeason{Number: se.Number, EpisodeCount: se.EpisodeCount,
			Episodes: se.Episodes, AirDate: optDate(se.AirDate), State: gen.SeriesCatalogSeasonState(se.State)})
	}
	out.Catalog = &cat
	out.CanRequestSeasons = &c.CanRequestSeasons
	return nil
}

func toCatalogEpisode(e *wishes.CatalogEpisode) *gen.SeriesCatalogEpisode {
	if e == nil {
		return nil
	}
	return &gen.SeriesCatalogEpisode{Season: e.Season, Episode: e.Episode, AirDate: optDate(e.AirDate)}
}

func (s *server) ApproveWish(ctx context.Context, req gen.ApproveWishRequestObject) (gen.ApproveWishResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	a, err := s.opts.Wishes.ApproveWish(ctx, req.Id, sessionAccountID(ctx))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out, err := s.adminWishes(ctx, []model.AdminWish{a})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ApproveWish200JSONResponse(out[0]), nil
}

func (s *server) DeclineWish(ctx context.Context, req gen.DeclineWishRequestObject) (gen.DeclineWishResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	a, err := s.opts.Wishes.DeclineWish(ctx, req.Id, sessionAccountID(ctx))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out, err := s.adminWishes(ctx, []model.AdminWish{a})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeclineWish200JSONResponse(out[0]), nil
}

// adminWishes maps wishes of the administration with what stage 4 adds: the display name of the
// Seerr user, the tag of every requester and the progress (with message, never a path).
func (s *server) adminWishes(ctx context.Context, list []model.AdminWish) ([]gen.AdminWish, error) {
	svc := s.opts.Wishes
	ids := make([]int64, 0, len(list))
	for _, a := range list {
		ids = append(ids, a.ID)
	}
	tags, err := svc.WishTagsOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	progress, err := svc.AdminProgressOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	users, err := svc.SeerrUsers(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(users))
	for _, u := range users {
		names[u.ID] = u.DisplayName
	}
	out := make([]gen.AdminWish, 0, len(list))
	for _, a := range list {
		g := toAdminWish(a)
		if n := names[a.SeerrUserID]; a.SeerrUserID > 0 && n != "" {
			g.SeerrUserName = &n
		}
		byProfile := map[int64]wishes.WishTag{}
		for _, t := range tags[a.ID] {
			byProfile[t.ProfileID] = t
		}
		for i := range g.Requesters {
			if t, ok := byProfile[g.Requesters[i].ProfileId]; ok {
				g.Requesters[i].Tag = toRequesterTag(t)
			}
		}
		if p, ok := progress[a.ID]; ok {
			g.Progress = toAdminWishProgress(p)
		}
		out = append(out, g)
	}
	return out, nil
}

func toRequesterTag(t wishes.WishTag) *gen.WishRequesterTag {
	out := &gen.WishRequesterTag{Name: t.Name, Service: gen.WishRequesterTagService(t.Service), State: gen.WishRequesterTagState(t.State),
		LastError: optString(t.LastError)}
	if !t.SetAt.IsZero() {
		at := t.SetAt.UTC()
		out.SetAt = &at
	}
	return out
}

// toWishProgress is the progress of the viewers; nil without a stage.
func toWishProgress(p wishes.WishProgress) *gen.WishProgress {
	if p.Stage == "" {
		return nil
	}
	return &gen.WishProgress{Stage: gen.WishProgressStage(p.Stage), Percent: p.Percent, EtaSeconds: p.ETASeconds,
		Episodes: toProgressEpisodes(p.Episodes), ReadAt: p.ReadAt.UTC()}
}

func toAdminWishProgress(p wishes.AdminWishProgress) *gen.AdminWishProgress {
	out := &gen.AdminWishProgress{Service: gen.AdminWishProgressService(p.Service), Stale: p.Stale, Message: optString(p.Message),
		Percent: p.Percent, EtaSeconds: p.ETASeconds, Episodes: toProgressEpisodes(p.Episodes)}
	if p.Stage != "" {
		st := gen.AdminWishProgressStage(p.Stage)
		out.Stage = &st
	}
	if p.Error != "" {
		e := gen.AdminWishProgressError(p.Error)
		out.Error = &e
	}
	if !p.ReadAt.IsZero() {
		at := p.ReadAt.UTC()
		out.ReadAt = &at
	}
	return out
}

func toProgressEpisodes(e *wishes.ProgressEpisodes) *gen.WishProgressEpisodes {
	if e == nil {
		return nil
	}
	return &gen.WishProgressEpisodes{Done: e.Done, Total: e.Total}
}

func (s *server) ImportSeerrRequests(ctx context.Context, req gen.ImportSeerrRequestsRequestObject) (gen.ImportSeerrRequestsResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	var user *int
	if req.Body != nil && req.Body.SeerrUserId != nil {
		if *req.Body.SeerrUserId < 1 {
			return s.problem(ctx, &model.ValidationError{Fields: []model.FieldError{{Field: "seerrUserId", Code: "invalid"}}}), nil
		}
		user = req.Body.SeerrUserId
	}
	n, err := s.opts.Wishes.ImportSeerrRequests(ctx, user)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ImportSeerrRequests202JSONResponse{Queued: n}, nil
}

func (s *server) CountSeerrRequests(ctx context.Context, req gen.CountSeerrRequestsRequestObject) (gen.CountSeerrRequestsResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if req.SeerrUserId < 1 {
		return s.problem(ctx, model.ErrNotFound), nil
	}
	n, err := s.opts.Wishes.CountSeerrRequests(ctx, req.SeerrUserId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CountSeerrRequests200JSONResponse{Count: n}, nil
}

func (s *server) GetArrStatus(ctx context.Context, _ gen.GetArrStatusRequestObject) (gen.GetArrStatusResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	st, err := s.opts.Wishes.ArrStatus(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetArrStatus200JSONResponse(toArrStatus(st)), nil
}

func (s *server) UpdateArrSettings(ctx context.Context, req gen.UpdateArrSettingsRequestObject) (gen.UpdateArrSettingsResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	b := deref(req.Body)
	st, err := s.opts.Wishes.UpdateArrSettings(ctx, wishes.ArrSettingsUpdate{RadarrURL: b.RadarrUrl, SonarrURL: b.SonarrUrl})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateArrSettings200JSONResponse(toArrStatus(st)), nil
}

func toArrStatus(st wishes.ArrStatus) gen.ArrStatus {
	return gen.ArrStatus{Radarr: toArrServiceStatus(st.Radarr), Sonarr: toArrServiceStatus(st.Sonarr)}
}

func toArrServiceStatus(st wishes.ArrServiceStatus) gen.ArrServiceStatus {
	out := gen.ArrServiceStatus{Source: gen.ArrServiceStatusSource(st.Source), Address: optString(st.Address), Confirmed: st.Confirmed,
		Reachable: st.Reachable, TagRequests: st.TagRequests, LastError: optString(st.LastError)}
	if !st.LastAt.IsZero() {
		at := st.LastAt.UTC()
		out.LastAt = &at
	}
	out.Counts.Set, out.Counts.Pending, out.Counts.Failed = st.Counts.Set, st.Counts.Pending, st.Counts.Failed
	out.Progress.Enabled, out.Progress.Tracked, out.Progress.Error = st.Progress.Enabled, st.Progress.Tracked, optString(st.Progress.Error)
	if t := st.Progress.LastReadAt; t != nil {
		at := t.UTC()
		out.Progress.LastReadAt = &at
	}
	return out
}

func (s *server) TestArrConnection(ctx context.Context, req gen.TestArrConnectionRequestObject) (gen.TestArrConnectionResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	b := deref(req.Body)
	res, err := s.opts.Wishes.TestArrService(ctx, string(b.Service))
	switch {
	case errors.Is(err, wishes.ErrArrNotInSeerr):
		// Seerr has no default server of the service: a result, not a failure of Seerr.
		return gen.TestArrConnection200JSONResponse{Source: gen.ArrTestResultSourceSeerr, Error: optString("not_in_seerr")}, nil
	case err != nil:
		return s.problem(ctx, err), nil
	}
	out := gen.TestArrConnection200JSONResponse{Reachable: res.Reachable, KeyValid: res.KeyValid, TagRequests: res.TagRequests,
		Host: optString(res.Host), Source: gen.ArrTestResultSource(res.Source), Error: optString(res.Error)}
	if out.Source == "" {
		out.Source = gen.ArrTestResultSourceSeerr
	}
	return out, nil
}

func (s *server) SetProfileTagName(ctx context.Context, req gen.SetProfileTagNameRequestObject) (gen.SetProfileTagNameResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Wishes.SetProfileTagName(ctx, req.ProfileId, deref(req.Body).Name); err != nil {
		return s.problem(ctx, err), nil
	}
	t, err := s.opts.Wishes.ProfileTagOf(ctx, req.ProfileId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetProfileTagName200JSONResponse{ProfileId: req.ProfileId, Name: t.Name, Custom: t.Custom}, nil
}

// mappingProfiles adds the profiles of every account with their tag names to getWishMappings.
func (s *server) mappingProfiles(ctx context.Context, accounts []gen.WishAccountMapping) error {
	if s.opts.Profiles == nil {
		return nil
	}
	for i := range accounts {
		profiles, err := s.opts.Profiles.ListMyProfiles(ctx, accounts[i].AccountId)
		if err != nil {
			return err
		}
		list := make([]gen.WishProfileTag, 0, len(profiles))
		for _, p := range profiles {
			t, err := s.opts.Wishes.ProfileTagOf(ctx, p.ID)
			if err != nil {
				return err
			}
			list = append(list, gen.WishProfileTag{ProfileId: p.ID, Name: p.Name, TagName: t.Name, TagNameCustom: t.Custom})
		}
		accounts[i].Profiles = &list
	}
	return nil
}

// GetSeerrWebhookSetup is what the administrator enters in Seerr for the instant notice.
func (s *server) GetSeerrWebhookSetup(context.Context, gen.GetSeerrWebhookSetupRequestObject) (gen.GetSeerrWebhookSetupResponseObject, error) {
	st := integrations.SeerrWebhookSetup()
	return gen.GetSeerrWebhookSetup200JSONResponse{JsonPayload: st.JSONPayload, Types: st.Types, TypeNames: st.TypeNames}, nil
}
