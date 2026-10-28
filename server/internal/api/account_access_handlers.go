package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Account management of the administration (contract tag admin): the access limits of an account,
// its deletion, and the role it carries. The account list itself is in account_handlers.go; since
// ADR 0014 there is one list for every account instead of a main account and a guest list.

func (e serviceProblem) VisitUpdateAccountAccessResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteAccountResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitSetAccountRoleResponse(w http.ResponseWriter) error { return e.visit(w) }

// UpdateAccountAccess changes ceiling, libraries and the W-8 caps of an account. An administrator
// has no access limits (409 admin_limits_fixed): it is fixed at FSK 18 and all libraries, because
// the administration views read system-wide and without a profile check.
func (s *server) UpdateAccountAccess(ctx context.Context, req gen.UpdateAccountAccessRequestObject) (gen.UpdateAccountAccessResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	b := deref(req.Body)
	upd := model.AccountAccessUpdate{
		MaxRating: ratingPtr(b.MaxRating), AllLibraries: b.AllLibraries, LibraryIDs: b.LibraryIds,
		Limits: fromPlaybackLimits(b.Limits),
	}
	if err := svc.UpdateAccountAccess(ctx, req.Id, upd); err != nil {
		return s.problem(ctx, err), nil
	}
	// A lowered ceiling or a withdrawn library hides titles at once, so a running direct stream
	// that it covers has to end (ADR 0005).
	if err := s.recheckStreams(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateAccountAccess204Response{}, nil
}

// DeleteAccount ends the account's playback (open streams, HLS output), revokes all its sessions
// and deletes it; cookies and stream tokens answer 401 from the next request on. The owner and the
// last administrator are refused with 409 (ADR 0014).
func (s *server) DeleteAccount(ctx context.Context, req gen.DeleteAccountRequestObject) (gen.DeleteAccountResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	var end func(context.Context, int64) error
	if s.opts.Playback != nil {
		end = s.opts.Playback.EndAccount
	}
	if err := svc.DeleteAccount(ctx, req.Id, end); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteAccount204Response{}, nil
}

// SetAccountRole appoints an account as administrator or takes the role away. The middleware has
// already turned away everybody but the owner (x-visibility owner, 403 owner_required); what is
// left to decide here is the state, and the service does that in one write transaction.
func (s *server) SetAccountRole(ctx context.Context, req gen.SetAccountRoleRequestObject) (gen.SetAccountRoleResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	if err := svc.SetAccountRole(ctx, req.Id, string(deref(req.Body).Role)); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetAccountRole204Response{}, nil
}
