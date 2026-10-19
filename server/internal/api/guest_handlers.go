package api

import (
	"context"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the guest administration (contract tag admin, x-visibility admin).

func (e serviceProblem) VisitListGuestsResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitUpdateGuestResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRevokeGuestResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ListGuests(ctx context.Context, _ gen.ListGuestsRequestObject) (gen.ListGuestsResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	list, err := svc.ListGuests(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListGuests200JSONResponse, 0, len(list))
	for _, g := range list {
		out = append(out, toGuest(g))
	}
	return out, nil
}

func (s *server) UpdateGuest(ctx context.Context, req gen.UpdateGuestRequestObject) (gen.UpdateGuestResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	b := deref(req.Body)
	g, err := svc.UpdateGuest(ctx, req.Id, model.GuestUpdate{MaxRating: ratingPtr(b.MaxRating), AllLibraries: b.AllLibraries, LibraryIDs: b.LibraryIds})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateGuest200JSONResponse(toGuest(g)), nil
}

// RevokeGuest ends the guest's playback (open streams, HLS output), revokes all its sessions and
// deletes the account; cookies and stream tokens answer 401 from the next request on.
func (s *server) RevokeGuest(ctx context.Context, req gen.RevokeGuestRequestObject) (gen.RevokeGuestResponseObject, error) {
	svc, err := s.access()
	if err != nil {
		return nil, err
	}
	var end func(context.Context, int64) error
	if s.opts.Playback != nil {
		end = s.opts.Playback.EndAccount
	}
	if err := svc.RevokeAccount(ctx, req.Id, end); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RevokeGuest204Response{}, nil
}

func toGuest(g model.Guest) gen.Guest {
	return gen.Guest{
		Id: g.ID, Email: openapi_types.Email(g.Email), MaxRating: gen.Rating(g.MaxRating), AllLibraries: g.AllLibraries,
		LibraryIds: nonNil(g.LibraryIDs), ProfileCount: g.ProfileCount, CreatedAt: g.CreatedAt.UTC(), LastSeenAt: utc(g.LastSeenAt),
	}
}
