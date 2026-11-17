package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Wishes through Seerr, stage 1 (concept 25, contract 1.41.0): the connection and the wish
// settings live in package admin (Options.Admin), the test, the state, the instance, the Seerr
// users and the mapping in package wishes (Options.Wishes; nil answers 501). No answer carries a
// key or a password.

// WishesService is implemented by package wishes.
type WishesService interface {
	// TestConnection runs the check list; model.ErrSeerrNotConfigured, model.ErrSeerrKeyRequired,
	// *model.ValidationError.
	TestConnection(ctx context.Context, in model.SeerrTestInput) (model.SeerrTestResult, error)
	Status(ctx context.Context) (model.SeerrStatus, error)
	// ConfirmInstance: action confirm or reset; model.ErrInstanceUnchanged, *model.ValidationError.
	ConfirmInstance(ctx context.Context, action string) (model.SeerrStatus, error)
	SeerrUsers(ctx context.Context) ([]model.SeerrUser, error)
	// RefreshUsers queues seerr.users; model.ErrSeerrNotConfigured.
	RefreshUsers(ctx context.Context) error
	WishMappings(ctx context.Context) (model.WishMappings, error)
	// SetAccountMapping sets (or with nil removes) the Seerr user of an account by hand;
	// model.ErrNotFound, model.ErrSeerrUserTaken.
	SetAccountMapping(ctx context.Context, accountID int64, seerrUserID *int, by int64) (model.WishAccountMapping, error)
	// ProvisionAccount opens or links the Seerr user of an account at once (state waiting when Seerr
	// was not reached); model.ErrNotFound, model.ErrProvisionDisabled, model.ErrSeerrNotConfigured.
	ProvisionAccount(ctx context.Context, accountID, by int64) (model.WishAccountMapping, error)
	// ProvisionAll queues it for every account without a ready user; errors as ProvisionAccount.
	ProvisionAll(ctx context.Context, by int64) (link, create int, err error)
}

func (e serviceProblem) VisitGetSeerrSettingsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdateSeerrSettingsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitTestSeerrConnectionResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetSeerrStatusResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitConfirmSeerrInstanceResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListSeerrUsersResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRefreshSeerrUsersResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetWishMappingsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetAccountWishMappingResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitProvisionSeerrAccountResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitProvisionSeerrAccountsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetWishSettingsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetWishSettingsResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) GetSeerrSettings(ctx context.Context, _ gen.GetSeerrSettingsRequestObject) (gen.GetSeerrSettingsResponseObject, error) {
	st, err := s.opts.Admin.SeerrSettings(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetSeerrSettings200JSONResponse(toSeerrSettings(st)), nil
}

func (s *server) UpdateSeerrSettings(ctx context.Context, req gen.UpdateSeerrSettingsRequestObject) (gen.UpdateSeerrSettingsResponseObject, error) {
	b := deref(req.Body)
	st, err := s.opts.Admin.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{
		Enabled: b.Enabled, URL: b.Url, APIKey: b.ApiKey, Clear: b.Clear != nil && *b.Clear,
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateSeerrSettings200JSONResponse(toSeerrSettings(st)), nil
}

func toSeerrSettings(st model.SeerrSettings) gen.SeerrSettings {
	return gen.SeerrSettings{
		Enabled: st.Enabled, Url: optString(st.URL), UrlSource: gen.SettingSource(st.URLSource),
		ApiKey: gen.KeyState{Source: gen.SettingSource(st.Key.Source), Hint: optString(st.Key.Hint)},
	}
}

func (s *server) TestSeerrConnection(ctx context.Context, req gen.TestSeerrConnectionRequestObject) (gen.TestSeerrConnectionResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	var in model.SeerrTestInput
	if b := req.Body; b != nil {
		in.URL, in.APIKey = deref(b.Url), deref(b.ApiKey)
	}
	res, err := s.opts.Wishes.TestConnection(ctx, in)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.TestSeerrConnection200JSONResponse{Ok: res.OK, Version: optString(res.Version), Checks: make([]gen.SeerrCheck, 0, len(res.Checks))}
	for _, c := range res.Checks {
		out.Checks = append(out.Checks, gen.SeerrCheck{Key: gen.SeerrCheckKey(c.Key), Level: gen.SeerrCheckLevel(c.Level), Detail: optString(c.Detail)})
	}
	return out, nil
}

func (s *server) GetSeerrStatus(ctx context.Context, _ gen.GetSeerrStatusRequestObject) (gen.GetSeerrStatusResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	st, err := s.opts.Wishes.Status(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetSeerrStatus200JSONResponse(toSeerrStatus(st)), nil
}

func (s *server) ConfirmSeerrInstance(ctx context.Context, req gen.ConfirmSeerrInstanceRequestObject) (gen.ConfirmSeerrInstanceResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	st, err := s.opts.Wishes.ConfirmInstance(ctx, string(deref(req.Body).Action))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ConfirmSeerrInstance200JSONResponse(toSeerrStatus(st)), nil
}

func toSeerrStatus(st model.SeerrStatus) gen.SeerrStatus {
	out := gen.SeerrStatus{
		State: gen.SeerrStatusState(st.State), Version: optString(st.Version), LastSuccess: st.LastSuccess,
		LastAttempt: st.LastAttempt, LastError: optString(st.LastError), InstanceChanged: st.InstanceChanged,
		Tested: st.Tested, Log: make([]gen.SeerrLogEntry, 0, len(st.Log)),
	}
	out.Counts.Open, out.Counts.Queued, out.Counts.Unmapped = st.Counts.Open, st.Counts.Queued, st.Counts.Unmapped
	for _, l := range st.Log {
		e := gen.SeerrLogEntry{At: l.At, Method: l.Method, Path: l.Path, DurationMs: l.DurationMS, Error: optString(l.Error)}
		if l.Status != 0 {
			status := l.Status
			e.Status = &status
		}
		out.Log = append(out.Log, e)
	}
	return out
}

func (s *server) ListSeerrUsers(ctx context.Context, _ gen.ListSeerrUsersRequestObject) (gen.ListSeerrUsersResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	users, err := s.opts.Wishes.SeerrUsers(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListSeerrUsers200JSONResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toSeerrUser(u))
	}
	return out, nil
}

func (s *server) RefreshSeerrUsers(ctx context.Context, _ gen.RefreshSeerrUsersRequestObject) (gen.RefreshSeerrUsersResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Wishes.RefreshUsers(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RefreshSeerrUsers202Response{}, nil
}

func toSeerrUser(u model.SeerrUser) gen.SeerrUser {
	out := gen.SeerrUser{
		Id: u.ID, DisplayName: u.DisplayName, Email: optString(u.Email), UserType: gen.SeerrUserUserType(u.UserType),
		IsAdmin: u.IsAdmin, CanRequestMovies: u.CanRequestMovies, CanRequestShows: u.CanRequestShows,
		AutoApprove: u.AutoApprove, Shared: u.Shared, Gone: u.Gone,
		Quota: gen.SeerrUserQuota{MovieLimit: u.Quota.MovieLimit, MovieDays: u.Quota.MovieDays, TvLimit: u.Quota.TVLimit, TvDays: u.Quota.TVDays},
	}
	if m := u.MappedTo; m != nil {
		out.MappedTo = &struct {
			AccountId int64  `json:"accountId"`
			Email     string `json:"email"`
		}{AccountId: m.AccountID, Email: m.Email}
	}
	return out
}

func (s *server) GetWishMappings(ctx context.Context, _ gen.GetWishMappingsRequestObject) (gen.GetWishMappingsResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	m, err := s.opts.Wishes.WishMappings(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetWishMappings200JSONResponse{
		Accounts: make([]gen.WishAccountMapping, 0, len(m.Accounts)), UnlinkedUsers: make([]gen.SeerrUser, 0, len(m.UnlinkedUsers)),
		Opened: m.Opened, Total: m.Total,
	}
	for _, a := range m.Accounts {
		out.Accounts = append(out.Accounts, toWishAccountMapping(a))
	}
	for _, u := range m.UnlinkedUsers {
		out.UnlinkedUsers = append(out.UnlinkedUsers, toSeerrUser(u))
	}
	if m.SharedUser != nil {
		su := toSeerrUser(*m.SharedUser)
		out.SharedUser = &su
	}
	return out, nil
}

func (s *server) SetAccountWishMapping(ctx context.Context, req gen.SetAccountWishMappingRequestObject) (gen.SetAccountWishMappingResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	var by int64
	if session, ok := sessionFrom(ctx); ok {
		by = session.Account.ID
	}
	a, err := s.opts.Wishes.SetAccountMapping(ctx, req.AccountId, deref(req.Body).SeerrUserId, by)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetAccountWishMapping200JSONResponse(toWishAccountMapping(a)), nil
}

func (s *server) ProvisionSeerrAccount(ctx context.Context, req gen.ProvisionSeerrAccountRequestObject) (gen.ProvisionSeerrAccountResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	a, err := s.opts.Wishes.ProvisionAccount(ctx, req.AccountId, sessionAccountID(ctx))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	if a.State == "waiting" {
		return gen.ProvisionSeerrAccount202JSONResponse(toWishAccountMapping(a)), nil
	}
	return gen.ProvisionSeerrAccount200JSONResponse(toWishAccountMapping(a)), nil
}

func (s *server) ProvisionSeerrAccounts(ctx context.Context, _ gen.ProvisionSeerrAccountsRequestObject) (gen.ProvisionSeerrAccountsResponseObject, error) {
	if s.opts.Wishes == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	link, create, err := s.opts.Wishes.ProvisionAll(ctx, sessionAccountID(ctx))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ProvisionSeerrAccounts202JSONResponse{Link: link, Create: create}, nil
}

// sessionAccountID is the account of the session (the administrator who acts), 0 without one.
func sessionAccountID(ctx context.Context) int64 {
	if session, ok := sessionFrom(ctx); ok {
		return session.Account.ID
	}
	return 0
}

func toWishAccountMapping(a model.WishAccountMapping) gen.WishAccountMapping {
	out := gen.WishAccountMapping{
		AccountId: a.AccountID, Email: a.Email, CanRequest: a.CanRequest, SeerrUserId: a.SeerrUserID,
		State: gen.WishAccountMappingState(a.State), PasswordSyncedAt: a.PasswordSyncedAt, LinkedAt: a.LinkedAt,
		ProvisionError: optString(a.ProvisionError),
		Limit: gen.AccountWishLimit{
			Mode: gen.AccountWishLimitMode(a.Limit.Mode), SeerrAdmin: a.Limit.SeerrAdmin,
			Effective: toWishLimits(a.Limit.Effective),
		},
	}
	if a.PasswordState != "" {
		ps := gen.WishAccountMappingPasswordState(a.PasswordState)
		out.PasswordState = &ps
	}
	if a.LinkedBy != "" {
		lb := gen.WishAccountMappingLinkedBy(a.LinkedBy)
		out.LinkedBy = &lb
	}
	if l := a.Limit.Movie; l != nil {
		out.Limit.Movie = &gen.WishLimit{Limit: l.Limit, Days: l.Days}
	}
	if l := a.Limit.TV; l != nil {
		out.Limit.Tv = &gen.WishLimit{Limit: l.Limit, Days: l.Days}
	}
	return out
}

func toWishLimits(l model.WishLimits) gen.WishLimits {
	return gen.WishLimits{
		Movie: gen.WishLimit{Limit: l.Movie.Limit, Days: l.Movie.Days},
		Tv:    gen.WishLimit{Limit: l.TV.Limit, Days: l.TV.Days},
	}
}

func (s *server) GetWishSettings(ctx context.Context, _ gen.GetWishSettingsRequestObject) (gen.GetWishSettingsResponseObject, error) {
	ws, err := s.opts.Admin.WishSettings(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetWishSettings200JSONResponse(toWishSettings(ws)), nil
}

func (s *server) SetWishSettings(ctx context.Context, req gen.SetWishSettingsRequestObject) (gen.SetWishSettingsResponseObject, error) {
	b := deref(req.Body)
	upd := model.WishSettingsUpdate{
		Enabled: b.Enabled, AllowMovies: b.AllowMovies, AllowShows: b.AllowShows, FromSearch: b.FromSearch,
		FromDetail: b.FromDetail, NewAccountsCanRequest: b.NewAccountsCanRequest, AllowWithdraw: b.AllowWithdraw,
		PINExternalHideTitle: b.PinExternalHideTitle, ProvisionAccounts: b.ProvisionAccounts, SyncPassword: b.SyncPassword,
	}
	if v := b.AlreadyRequested; v != nil {
		s := string(*v)
		upd.AlreadyRequested = &s
	}
	if v := b.Unmapped; v != nil {
		s := string(*v)
		upd.Unmapped = &s
	}
	if l := b.DefaultLimit; l != nil {
		upd.DefaultLimit = &model.WishLimits{
			Movie: model.WishLimit{Limit: l.Movie.Limit, Days: l.Movie.Days},
			TV:    model.WishLimit{Limit: l.Tv.Limit, Days: l.Tv.Days},
		}
	}
	ws, err := s.opts.Admin.SetWishSettings(ctx, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetWishSettings200JSONResponse(toWishSettings(ws)), nil
}

func toWishSettings(ws model.WishSettings) gen.WishSettings {
	return gen.WishSettings{
		Enabled: ws.Enabled, AllowMovies: ws.AllowMovies, AllowShows: ws.AllowShows, FromSearch: ws.FromSearch,
		FromDetail: ws.FromDetail, AlreadyRequested: gen.WishSettingsAlreadyRequested(ws.AlreadyRequested),
		Unmapped: gen.WishSettingsUnmapped(ws.Unmapped), NewAccountsCanRequest: ws.NewAccountsCanRequest,
		AllowWithdraw: ws.AllowWithdraw, PinExternalHideTitle: ws.PINExternalHideTitle,
		ProvisionAccounts: ws.ProvisionAccounts, SyncPassword: ws.SyncPassword, DefaultLimit: toWishLimits(ws.DefaultLimit),
	}
}
