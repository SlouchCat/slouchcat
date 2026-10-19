package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of invitation links (contract tag invitations: public preview and acceptance) and their
// administration (contract tag admin). Guests are in guest_handlers.go.

var errNoAccessService = errors.New("access service not configured")

func (e serviceProblem) VisitGetInvitationResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitAcceptInvitationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListInvitationsResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitCreateInvitationResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRevokeInvitationResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) access() (AccessService, error) {
	if s.opts.Access == nil {
		return nil, errNoAccessService
	}
	return s.opts.Access, nil
}

func (s *server) GetInvitation(ctx context.Context, req gen.GetInvitationRequestObject) (gen.GetInvitationResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	prev, err := svc.GetInvitation(model.WithClientIP(ctx, s.clientAddr(ctx)), req.Token)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.InvitationPreview{
		Note: optString(prev.Note), MaxRating: gen.Rating(prev.MaxRating), AllLibraries: prev.AllLibraries,
		ExpiresAt: prev.ExpiresAt.UTC(),
		Libraries: make([]struct {
			Id   int64  `json:"id"`
			Name string `json:"name"`
		}, 0, len(prev.Libraries)),
	}
	for _, l := range prev.Libraries {
		out.Libraries = append(out.Libraries, struct {
			Id   int64  `json:"id"`
			Name string `json:"name"`
		}{Id: l.ID, Name: l.Name})
	}
	return gen.GetInvitation200JSONResponse(out), nil
}

func (s *server) AcceptInvitation(ctx context.Context, req gen.AcceptInvitationRequestObject) (gen.AcceptInvitationResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	b := deref(req.Body)
	in := model.AcceptInvitation{
		Token: req.Token, Email: string(b.Email), Password: deref(b.Password), ProfileName: b.ProfileName,
		Language: string(deref(b.Language)),
	}
	session, token, err := svc.AcceptInvitation(model.WithClientIP(ctx, s.clientAddr(ctx)), in, userAgent(ctx))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return sessionStarted{status: http.StatusCreated, r: requestFrom(ctx), session: session, token: token}, nil
}

func (s *server) ListInvitations(ctx context.Context, _ gen.ListInvitationsRequestObject) (gen.ListInvitationsResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	list, err := svc.ListInvitations(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListInvitations200JSONResponse, 0, len(list))
	for _, inv := range list {
		out = append(out, toInvitation(inv))
	}
	return out, nil
}

func (s *server) CreateInvitation(ctx context.Context, req gen.CreateInvitationRequestObject) (gen.CreateInvitationResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	session, ok := sessionFrom(ctx)
	if !ok {
		return nil, errNoSession
	}
	b := deref(req.Body)
	inv, err := svc.CreateInvitation(ctx, session.Account, model.InvitationRequest{
		Note: deref(b.Note), MaxRating: int(b.MaxRating), AllLibraries: b.AllLibraries, LibraryIDs: deref(b.LibraryIds),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	base := toInvitation(inv)
	return gen.CreateInvitation201JSONResponse{
		AllLibraries: base.AllLibraries, CreatedAt: base.CreatedAt, ExpiresAt: base.ExpiresAt, Id: base.Id,
		LibraryIds: base.LibraryIds, MaxRating: base.MaxRating, Note: base.Note, State: base.State,
		UsedAt: base.UsedAt, UsedByEmail: base.UsedByEmail,
		Token: inv.Token, Url: "/invite/" + url.PathEscape(inv.Token),
	}, nil
}

func (s *server) RevokeInvitation(ctx context.Context, req gen.RevokeInvitationRequestObject) (gen.RevokeInvitationResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	if err := svc.RevokeInvitation(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RevokeInvitation204Response{}, nil
}

func toInvitation(inv model.Invitation) gen.Invitation {
	out := gen.Invitation{
		Id: inv.ID, Note: optString(inv.Note), State: gen.InvitationState(inv.State), MaxRating: gen.Rating(inv.MaxRating),
		AllLibraries: inv.AllLibraries, LibraryIds: nonNil(inv.LibraryIDs),
		CreatedAt: inv.CreatedAt.UTC(), ExpiresAt: inv.ExpiresAt.UTC(), UsedAt: utc(inv.UsedAt),
	}
	if inv.UsedByEmail != "" {
		e := openapi_types.Email(inv.UsedByEmail)
		out.UsedByEmail = &e
	}
	return out
}
