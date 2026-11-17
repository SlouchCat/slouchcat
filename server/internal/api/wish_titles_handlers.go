package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Wishes through Seerr, stage 2 (concept 25, contract 1.41.0): titles outside the library, the
// wishes of the active profile and the list of the administration, through package wishes
// (Options.Wishes; nil answers 501). Not visible is 404; 403 wish_not_allowed is about the right to
// request, never about a title (data model 4.4). Every ItemRef passes the visibility of the active
// profile (rule 2).

// Images of titles outside the library (binary, CLAUDE.md rule 6).
const (
	externalImagePath      = "/titles/{media}/{tmdbId}/images/{kind}/{size}"
	adminExternalImagePath = "/admin/titles/{media}/{tmdbId}/images/{kind}/{size}"
)

// externalImageCache: the address carries no content hash, so the browser revalidates daily by
// the ETag.
const externalImageCache = "private, max-age=86400"

// WishTitlesService is the part of package wishes for stage 2 (part of WishesService).
type WishTitlesService interface {
	ExternalTitle(ctx context.Context, p model.Profile, media string, tmdbID int64) (model.ExternalTitlePreview, error)
	ExternalTitleImage(ctx context.Context, p model.Profile, media string, tmdbID int64, kind, size string, preferWebP bool) (model.StreamSource, error)
	AdminExternalTitleImage(ctx context.Context, media string, tmdbID int64, kind, size string, preferWebP bool) (model.StreamSource, error)
	SearchExternalTitles(ctx context.Context, p model.Profile, q, media string, offset, limit int) (model.ExternalTitlePage, error)
	ItemSummary(ctx context.Context, p model.Profile, itemID string) (model.ItemSummary, error)
	CreateWish(ctx context.Context, p model.Profile, in model.WishCreate) (model.WishCreated, error)
	WithdrawWish(ctx context.Context, p model.Profile, wishID int64) error
	ProfileWishes(ctx context.Context, p model.Profile, filter string, offset, limit int) (model.WishPage, error)
	AccountWishes(ctx context.Context, p model.Profile, filter string, offset, limit int) (model.WishPage, error)
	RetryWish(ctx context.Context, id int64) error
	WishQuota(ctx context.Context, p model.Profile) (model.WishQuota, error)
	WishSeasons(ctx context.Context, p model.Profile, tmdbID int64) (model.WishSeasons, error)
	AdminWishes(ctx context.Context, filter string, unmapped bool, offset, limit int) ([]model.AdminWish, int, model.WishCounts, error)
	SyncNow(ctx context.Context) error
}

const (
	defaultWishPage = 50
	maxWishPage     = 100
)

func (e serviceProblem) VisitSearchExternalTitlesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetExternalTitleResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetWishSeasonsResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitListWishesResponse(w http.ResponseWriter) error       { return e.visit(w) }
func (e serviceProblem) VisitCreateWishResponse(w http.ResponseWriter) error       { return e.visit(w) }
func (e serviceProblem) VisitGetWishQuotaResponse(w http.ResponseWriter) error     { return e.visit(w) }
func (e serviceProblem) VisitWithdrawWishResponse(w http.ResponseWriter) error     { return e.visit(w) }
func (e serviceProblem) VisitListAdminWishesResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitSyncWishesNowResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitListAccountWishesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRetryWishResponse(w http.ResponseWriter) error { return e.visit(w) }

// wishSession returns the session and the service; ok is false without the service (501).
func (s *server) wishSession(ctx context.Context) (model.Session, WishesService, bool, error) {
	session, found := sessionFrom(ctx)
	if !found {
		return model.Session{}, nil, false, errNoSession
	}
	return session, s.opts.Wishes, s.opts.Wishes != nil, nil
}

// wishProblem is the problem of a wish error with its extension, ok false for other errors.
func (s *server) wishProblem(ctx context.Context, svc WishTitlesService, p model.Profile, err error) (gen.WishProblem, bool) {
	var notAllowed *model.WishNotAllowedError
	var available *model.WishAlreadyAvailableError
	var quota *model.WishQuotaExceededError
	var out gen.WishProblem
	switch {
	case errors.As(err, &notAllowed):
		out = wishProblemBody(ctx, http.StatusForbidden, "wish_not_allowed", "The profile may not request this.")
		out.Reason = optString(notAllowed.Reason)
	case errors.As(err, &available):
		sum, serr := svc.ItemSummary(ctx, p, available.ItemID)
		if serr != nil {
			return gen.WishProblem{}, false // not visible after all: the caller answers 404 or 500
		}
		out = wishProblemBody(ctx, http.StatusConflict, "already_available", "The library has the title.")
		item := toItemRef(p.ID, sum)
		out.Item = &item
	case errors.As(err, &quota):
		out = wishProblemBody(ctx, http.StatusConflict, "quota_exceeded", "The limit of the Seerr user is used up.")
		out.Quota = toWishQuota(&quota.Quota)
		out.Needed = &quota.Needed
	default:
		return gen.WishProblem{}, false
	}
	return out, true
}

func wishProblemBody(ctx context.Context, status int, code, detail string) gen.WishProblem {
	out := gen.WishProblem{Type: "about:blank", Title: http.StatusText(status), Status: status, Code: code, Detail: &detail}
	if r := requestFrom(ctx); r != nil {
		instance := logging.RedactString(r.URL.Path)
		out.Instance = &instance
	}
	return out
}

// alreadyAvailableGone turns an already_available whose title the profile cannot see after all
// into a 404.
func alreadyAvailableGone(err error) error {
	if errors.As(err, new(*model.WishAlreadyAvailableError)) {
		return model.ErrNotFound
	}
	return err
}

func (s *server) SearchExternalTitles(ctx context.Context, req gen.SearchExternalTitlesRequestObject) (gen.SearchExternalTitlesResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	media := ""
	if req.Params.Media != nil {
		media = string(*req.Params.Media)
	}
	page, err := svc.SearchExternalTitles(ctx, session.Profile, req.Params.Q, media, deref(req.Params.Offset), deref(req.Params.Limit))
	if err != nil {
		return s.problem(ctx, tmdbDown(err)), nil
	}
	out := gen.SearchExternalTitles200JSONResponse{Items: make([]gen.ExternalTitleRef, 0, len(page.Items)), Total: page.Total}
	for _, c := range page.Items {
		out.Items = append(out.Items, toExternalTitleRef(c))
	}
	return out, nil
}

// tmdbDown answers an unreachable TMDB of the viewer operations with 503 (the administration's
// tmdb_unreachable is 422, the key check).
func tmdbDown(err error) error {
	if errors.Is(err, model.ErrTMDBUnreachable) || errors.Is(err, model.ErrProviderNotConfigured) || errors.Is(err, model.ErrTMDBKeyInvalid) {
		return errTMDBDown
	}
	return err
}

// errTMDBDown is mapped to 503 tmdb_unreachable by the wish handlers.
var errTMDBDown = errors.New("tmdb unreachable for a viewer")

func (s *server) GetExternalTitle(ctx context.Context, req gen.GetExternalTitleRequestObject) (gen.GetExternalTitleResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	t, err := svc.ExternalTitle(ctx, session.Profile, string(req.Media), req.TmdbId)
	if err != nil {
		if body, ok := s.wishProblem(ctx, svc, session.Profile, err); ok {
			return gen.GetExternalTitle409ApplicationProblemPlusJSONResponse(body), nil
		}
		return s.problem(ctx, tmdbDown(alreadyAvailableGone(err))), nil
	}
	out := gen.GetExternalTitle200JSONResponse{
		Media: gen.ExternalMedia(t.Media), TmdbId: t.TMDBID, Title: t.Title, Year: optInt(t.Year), MinAge: t.MinAge,
		Images: externalImages(t.ExternalTitleCard), HasTrailer: t.HasTrailer, Wish: toWishMark(t.Wish),
		CanRequest: t.CanRequest, RequestReason: requestReason(t.RequestReason),
		OriginalTitle: optString(t.OriginalTitle), Overview: optString(t.Overview), Tagline: optString(t.Tagline),
		ReleaseDate: optDate(t.ReleaseDate), RuntimeMinutes: optInt(t.RuntimeMinutes), Genres: t.Genres, Status: optString(t.Status),
		Seasons: make([]gen.ExternalTitleSeason, 0, len(t.Seasons)), Trailers: make([]gen.ExternalTrailer, 0, len(t.Trailers)),
		Request: gen.WishRequestCard{CanRequest: t.Request.CanRequest, Reason: requestReason(t.Request.Reason),
			AutoApprove: t.Request.AutoApprove, Quota: toWishQuota(t.Request.Quota)},
	}
	if out.Genres == nil {
		out.Genres = []string{}
	}
	if sh := t.Request.Shared; sh != nil {
		out.Request.Shared = &gen.WishShared{State: gen.WishState(sh.State), StateDetail: stateDetail(sh.StateDetail), CanJoin: sh.CanJoin}
	}
	if out.Wish != nil {
		progress, err := svc.ProgressOf(ctx, []int64{out.Wish.Id})
		if err != nil {
			return s.problem(ctx, err), nil
		}
		if p, ok := progress[out.Wish.Id]; ok {
			out.Wish.Progress = toWishProgress(p)
		}
	}
	for _, se := range t.Seasons {
		out.Seasons = append(out.Seasons, gen.ExternalTitleSeason{Number: se.Number, EpisodeCount: se.EpisodeCount,
			AirDate: optDate(se.AirDate), State: gen.ExternalTitleSeasonState(se.State)})
	}
	for _, tr := range t.Trailers {
		out.Trailers = append(out.Trailers, gen.ExternalTrailer{Key: tr.Key, Lang: optString(tr.Lang)})
	}
	if e := t.NextEpisode; e != nil {
		out.NextEpisode = &gen.ExternalEpisode{Season: e.Season, Episode: e.Episode, AirDate: optDate(e.AirDate)}
	}
	if l := t.LastWish; l != nil {
		out.LastWish = &gen.WishLast{Id: l.ID, State: gen.WishState(l.State), At: l.At}
		if l.Reason != "" {
			r := gen.WishFailReason(l.Reason)
			out.LastWish.Reason = &r
		}
	}
	return out, nil
}

func (s *server) GetWishSeasons(ctx context.Context, req gen.GetWishSeasonsRequestObject) (gen.GetWishSeasonsResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	ws, err := svc.WishSeasons(ctx, session.Profile, req.TmdbId)
	if err != nil {
		// Without the right for series the dialog does not exist (data model 4.5: 404).
		if errors.As(err, new(*model.WishNotAllowedError)) {
			err = model.ErrNotFound
		}
		return s.problem(ctx, tmdbDown(alreadyAvailableGone(err))), nil
	}
	out := gen.GetWishSeasons200JSONResponse{Source: gen.WishSeasonsSource(ws.Source), Seasons: make([]gen.WishSeasonOffer, 0, len(ws.Seasons))}
	for _, se := range ws.Seasons {
		out.Seasons = append(out.Seasons, gen.WishSeasonOffer{Number: se.Number, EpisodeCount: se.EpisodeCount, State: gen.WishSeasonOfferState(se.State)})
	}
	return out, nil
}

func (s *server) ListWishes(ctx context.Context, req gen.ListWishesRequestObject) (gen.ListWishesResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	filter := ""
	if req.Params.State != nil {
		filter = string(*req.Params.State)
	}
	page, err := svc.ProfileWishes(ctx, session.Profile, filter, max(0, deref(req.Params.Offset)), wishLimit(req.Params.Limit))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListWishes200JSONResponse{Items: make([]gen.Wish, 0, len(page.Items)), Total: page.Total, Counts: toWishCounts(page.Counts)}
	progress, err := s.wishProgress(ctx, page.Items)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	for _, w := range page.Items {
		g, err := s.toWish(ctx, svc, session.Profile, w)
		if err != nil {
			return s.problem(ctx, err), nil
		}
		g.Progress = progress[w.ID]
		out.Items = append(out.Items, g)
	}
	return out, nil
}

// ListAccountWishes is listAccountWishes: the wishes of every profile of the account, for the
// primary profile only (403 primary_profile_required).
func (s *server) ListAccountWishes(ctx context.Context, req gen.ListAccountWishesRequestObject) (gen.ListAccountWishesResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if !session.IsPrimary() {
		return s.problem(ctx, model.ErrPrimaryProfileRequired), nil
	}
	filter := ""
	if req.Params.State != nil {
		filter = string(*req.Params.State)
	}
	page, err := svc.AccountWishes(ctx, session.Profile, filter, max(0, deref(req.Params.Offset)), wishLimit(req.Params.Limit))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListAccountWishes200JSONResponse{Items: make([]gen.Wish, 0, len(page.Items)), Total: page.Total, Counts: toWishCounts(page.Counts)}
	progress, err := s.wishProgress(ctx, page.Items)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	for _, w := range page.Items {
		g, err := s.toWish(ctx, svc, session.Profile, w)
		if err != nil {
			return s.problem(ctx, err), nil
		}
		g.Progress = progress[w.ID]
		g.ProfileName = optString(w.ProfileName)
		out.Items = append(out.Items, g)
	}
	return out, nil
}

// RetryWish is retryWish of the administration.
func (s *server) RetryWish(ctx context.Context, req gen.RetryWishRequestObject) (gen.RetryWishResponseObject, error) {
	svc := s.opts.Wishes
	if svc == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := svc.RetryWish(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RetryWish202Response{}, nil
}

func wishLimit(l *int) int {
	n := deref(l)
	if n <= 0 {
		return defaultWishPage
	}
	return min(n, maxWishPage)
}

func (s *server) CreateWish(ctx context.Context, req gen.CreateWishRequestObject) (gen.CreateWishResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	b := deref(req.Body)
	in := model.WishCreate{Media: string(b.Media), TMDBID: b.TmdbId, Seasons: deref(b.Seasons), AllSeasons: deref(b.AllSeasons)}
	if b.Source != nil {
		in.Source = string(*b.Source)
	}
	res, err := svc.CreateWish(ctx, session.Profile, in)
	if err != nil {
		if body, ok := s.wishProblem(ctx, svc, session.Profile, err); ok {
			if body.Status == http.StatusForbidden {
				return gen.CreateWish403ApplicationProblemPlusJSONResponse(body), nil
			}
			return gen.CreateWish409ApplicationProblemPlusJSONResponse(body), nil
		}
		return s.problem(ctx, tmdbDown(alreadyAvailableGone(err))), nil
	}
	w := res.Wish
	if w == nil && len(res.Joined) > 0 {
		w = &res.Joined[0]
	}
	if w == nil {
		return nil, errors.New("createWish: no wish in the answer")
	}
	g, err := s.toWish(ctx, svc, session.Profile, *w)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	if res.Created {
		return gen.CreateWish201JSONResponse(g), nil
	}
	return gen.CreateWish200JSONResponse(g), nil
}

func (s *server) GetWishQuota(ctx context.Context, _ gen.GetWishQuotaRequestObject) (gen.GetWishQuotaResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	q, err := svc.WishQuota(ctx, session.Profile)
	if errors.As(err, new(*model.WishNotAllowedError)) {
		err = model.ErrNotFound // a profile that cannot request has no limit to show (F-9)
	}
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetWishQuota200JSONResponse(*toWishQuota(&q)), nil
}

func (s *server) WithdrawWish(ctx context.Context, req gen.WithdrawWishRequestObject) (gen.WithdrawWishResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := svc.WithdrawWish(ctx, session.Profile, req.Id); err != nil {
		if body, ok := s.wishProblem(ctx, svc, session.Profile, err); ok {
			return gen.WithdrawWish403ApplicationProblemPlusJSONResponse(body), nil
		}
		return s.problem(ctx, err), nil
	}
	return gen.WithdrawWish204Response{}, nil
}

func (s *server) ListAdminWishes(ctx context.Context, req gen.ListAdminWishesRequestObject) (gen.ListAdminWishesResponseObject, error) {
	svc := s.opts.Wishes
	if svc == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	filter := ""
	if req.Params.State != nil {
		filter = string(*req.Params.State)
	}
	items, total, counts, err := svc.AdminWishes(ctx, filter, deref(req.Params.Unmapped), max(0, deref(req.Params.Offset)), wishLimit(req.Params.Limit))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	list, err := s.adminWishes(ctx, items)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListAdminWishes200JSONResponse{Items: list, Total: total, Counts: toWishCounts(counts)}, nil
}

func (s *server) SyncWishesNow(ctx context.Context, _ gen.SyncWishesNowRequestObject) (gen.SyncWishesNowResponseObject, error) {
	svc := s.opts.Wishes
	if svc == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := svc.SyncNow(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SyncWishesNow202Response{}, nil
}

// toWish maps a wish of the profile; item only when the profile sees the library title.
func (s *server) toWish(ctx context.Context, svc WishTitlesService, p model.Profile, w model.Wish) (gen.Wish, error) {
	out := gen.Wish{
		Id: w.ID, Media: gen.ExternalMedia(w.Media), TmdbId: w.TMDBID, Title: w.Title, Year: optInt(w.Year),
		State: gen.WishState(w.State), StateDetail: stateDetail(w.StateDetail), FailReason: failReason(w.FailReason),
		Origin: gen.WishOrigin(w.Origin), Role: gen.WishRole(w.Role), Seasons: make([]gen.WishSeasonState, 0, len(w.Seasons)),
		CanWithdraw: w.CanWithdraw, CanRequestAgain: w.CanRequestAgain, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
		AvailableAt: w.AvailableAt, Events: toWishEvents(w.Events),
	}
	if w.Hint != "" {
		h := gen.WishHint(w.Hint)
		out.Hint = &h
	}
	for _, se := range w.Seasons {
		out.Seasons = append(out.Seasons, gen.WishSeasonState{Number: se.Number, State: gen.WishState(se.State)})
	}
	if w.External != nil {
		ref := toExternalTitleRef(*w.External)
		out.External = &ref
	}
	if w.ItemID != "" {
		sum, err := svc.ItemSummary(ctx, p, w.ItemID)
		switch {
		case err == nil:
			item := toItemRef(p.ID, sum)
			out.Item = &item
		case !errors.Is(err, model.ErrNotFound):
			return gen.Wish{}, err
		}
	}
	return out, nil
}

func toWishEvents(events []model.WishEvent) []gen.WishEvent {
	out := make([]gen.WishEvent, 0, len(events))
	for _, e := range events {
		out = append(out, gen.WishEvent{At: e.At, Kind: e.Kind, Detail: optString(e.Detail)})
	}
	return out
}

func toWishCounts(c model.WishCounts) gen.WishCounts {
	return gen.WishCounts{Open: c.Open, Pending: c.Pending, Processing: c.Processing, Partial: c.Partial, Available: c.Available, Closed: c.Closed}
}

// toAdminWish maps a wish of the administration: no ItemRef, the poster through the admin route.
func toAdminWish(a model.AdminWish) gen.AdminWish {
	out := gen.AdminWish{
		Id: a.ID, State: gen.WishState(a.State), StateDetail: stateDetail(a.StateDetail), FailReason: failReason(a.FailReason),
		Origin: gen.AdminWishOrigin(a.Origin), Attempts: a.Attempts, Events: toWishEvents(a.Events),
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, AvailableAt: a.AvailableAt,
		Seasons: make([]gen.AdminWishSeason, 0, len(a.Seasons)), Requesters: make([]gen.AdminWishRequester, 0, len(a.Requesters)),
		Title: gen.AdminWishTitle{
			ItemId: optString(a.ItemID), Media: gen.ExternalMedia(a.Media), TmdbId: a.TMDBID, ImdbId: optString(a.IMDBID),
			Title: a.Title, Year: optInt(a.Year),
			PosterUrl: BasePath + fmt.Sprintf("/admin/titles/%s/%d/images/poster/w200", a.Media, a.TMDBID),
		},
	}
	if a.TVDBID > 0 {
		out.Title.TvdbId = &a.TVDBID
	}
	if a.SeerrUserID > 0 {
		out.SeerrUserId = &a.SeerrUserID
	}
	if a.SeerrRequestID > 0 {
		out.SeerrRequestId = &a.SeerrRequestID
	}
	for _, se := range a.Seasons {
		g := gen.AdminWishSeason{Number: se.Number, State: gen.WishState(se.State)}
		if se.SeerrRequestID > 0 {
			id := se.SeerrRequestID
			g.SeerrRequestId = &id
		}
		out.Seasons = append(out.Seasons, g)
	}
	for _, r := range a.Requesters {
		out.Requesters = append(out.Requesters, gen.AdminWishRequester{AccountId: r.AccountID, AccountEmail: r.AccountEmail,
			ProfileId: r.ProfileID, ProfileName: r.ProfileName, Role: gen.AdminWishRequesterRole(r.Role), Closed: r.Closed})
	}
	return out
}

func toExternalTitleRef(c model.ExternalTitleCard) gen.ExternalTitleRef {
	return gen.ExternalTitleRef{
		Media: gen.ExternalMedia(c.Media), TmdbId: c.TMDBID, Title: c.Title, Year: optInt(c.Year), MinAge: c.MinAge,
		Images: externalImages(c), HasTrailer: c.HasTrailer, Wish: toWishMark(c.Wish),
		CanRequest: c.CanRequest, RequestReason: requestReason(c.RequestReason),
	}
}

// externalImages are the references of the images of a title outside the library, in the sizes of
// ItemImages, with the cache-busting v and the profile id p (never TMDB's addresses).
func externalImages(c model.ExternalTitleCard) gen.ExternalTitleImages {
	var out gen.ExternalTitleImages
	base := BasePath + fmt.Sprintf("/titles/%s/%d/images/", c.Media, c.TMDBID)
	slot := map[string]**gen.ImageRef{
		"poster/w400": &out.Poster, "poster/w200": &out.PosterSmall, "poster/w800": &out.PosterLarge,
		"backdrop/w1280": &out.Backdrop, "backdrop/w780": &out.BackdropSmall, "backdrop/w1920": &out.BackdropLarge,
		"logo/w400": &out.Logo,
	}
	for _, im := range c.Images {
		if dst, ok := slot[im.Kind+"/"+im.Size]; ok {
			*dst = &gen.ImageRef{Url: base + im.Kind + "/" + im.Size + "?v=" + url.QueryEscape(im.Version) +
				"&p=" + strconv.FormatInt(c.ProfileID, 10), Width: im.Width, Height: im.Height}
		}
	}
	return out
}

func toWishMark(m *model.WishMark) *gen.WishMark {
	if m == nil {
		return nil
	}
	return &gen.WishMark{Id: m.ID, State: gen.WishState(m.State), StateDetail: stateDetail(m.StateDetail)}
}

func requestReason(r string) *gen.WishRequestReason {
	if r == "" {
		return nil
	}
	out := gen.WishRequestReason(r)
	return &out
}

func stateDetail(d string) *gen.WishStateDetail {
	if d == "" {
		return nil
	}
	out := gen.WishStateDetail(d)
	return &out
}

func failReason(r string) *gen.WishFailReason {
	if r == "" {
		return nil
	}
	out := gen.WishFailReason(r)
	return &out
}

func optInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

// getExternalTitleImage serves an image of a title outside the library with scope (binary
// endpoint, CLAUDE.md rule 6): the title is checked like getExternalTitle, then metadata opens
// the stored image (fetched from TMDB on its first request).
func (s *server) getExternalTitleImage(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFrom(r.Context())
	svc := s.opts.Wishes
	if !ok || svc == nil {
		s.p.mapError(w, r, model.ErrNotImplemented)
		return
	}
	media, tmdbID, kind, size, valid := externalImageParams(r)
	if !valid {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	w.Header().Add("Vary", "Accept")
	src, err := svc.ExternalTitleImage(r.Context(), session.Profile, media, tmdbID, kind, size, acceptsWebP(r.Header.Values("Accept")))
	s.serveExternalImage(w, r, src, err)
}

// getAdminExternalTitleImage serves an image of a stored title outside the library for the
// administration (binary endpoint, CLAUDE.md rule 6), without a profile check (x-visibility admin).
func (s *server) getAdminExternalTitleImage(w http.ResponseWriter, r *http.Request) {
	svc := s.opts.Wishes
	if svc == nil {
		s.p.mapError(w, r, model.ErrNotImplemented)
		return
	}
	media, tmdbID, kind, size, valid := externalImageParams(r)
	if !valid {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	w.Header().Add("Vary", "Accept")
	src, err := svc.AdminExternalTitleImage(r.Context(), media, tmdbID, kind, size, acceptsWebP(r.Header.Values("Accept")))
	s.serveExternalImage(w, r, src, err)
}

// externalImageParams reads and checks the path of an image route; valid false is 404.
func externalImageParams(r *http.Request) (media string, tmdbID int64, kind, size string, valid bool) {
	media, kind, size = chi.URLParam(r, "media"), chi.URLParam(r, "kind"), chi.URLParam(r, "size")
	raw := chi.URLParam(r, "tmdbId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != raw || (media != "movie" && media != "show") ||
		!imageKinds[kind] || !imageSizes[kind][size] || size == "w3840" {
		return "", 0, "", "", false
	}
	return media, id, kind, size, true
}

// serveExternalImage answers an opened image with its ETag for a day in the private cache, or
// maps err (an unreachable TMDB is 503).
func (s *server) serveExternalImage(w http.ResponseWriter, r *http.Request, src model.StreamSource, err error) {
	if errors.Is(err, model.ErrTMDBUnreachable) {
		s.p.write(w, r, http.StatusServiceUnavailable, "tmdb_unreachable", "TMDB did not deliver the image; try again later.")
		return
	}
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	if src.ETag != "" {
		w.Header().Set("ETag", src.ETag)
	}
	w.Header().Set("Cache-Control", externalImageCache)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// wishProgress is the progress viewers see of the wishes (N-83: no message, no service).
func (s *server) wishProgress(ctx context.Context, list []model.Wish) (map[int64]*gen.WishProgress, error) {
	ids := make([]int64, 0, len(list))
	for _, w := range list {
		ids = append(ids, w.ID)
	}
	progress, err := s.opts.Wishes.ProgressOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*gen.WishProgress, len(progress))
	for id, p := range progress {
		out[id] = toWishProgress(p)
	}
	return out, nil
}
