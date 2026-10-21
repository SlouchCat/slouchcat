package api

import (
	"context"
	"net/http"

	"github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8 accounts (plan M8, A-2; design section 1): the account overview of the administration and the
// change of e-mail and password by the signed-in account itself. Passwords only travel into the
// service.

func (e serviceProblem) VisitListAccountsResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitUpdateAccountResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ListAccounts(ctx context.Context, _ gen.ListAccountsRequestObject) (gen.ListAccountsResponseObject, error) {
	list, err := s.opts.Admin.ListAccounts(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListAccounts200JSONResponse, 0, len(list))
	for _, a := range list {
		item := gen.AdminAccount{
			Id: a.ID, Email: types.Email(a.Email), Role: gen.AdminAccountRole(a.Role), MaxRating: gen.Rating(a.MaxRating),
			AllLibraries: a.AllLibraries, LibraryIds: nonNil(a.LibraryIDs), LastSeenAt: utc(a.LastSeenAt), CreatedAt: a.CreatedAt.UTC(),
			Profiles: make([]gen.ProfileRef, 0, len(a.Profiles)), Channels: make([]gen.ChannelSummary, 0, len(a.Channels)),
		}
		for _, p := range a.Profiles {
			item.Profiles = append(item.Profiles, gen.ProfileRef{Id: p.ID, Name: p.Name, MaxRating: gen.Rating(p.MaxRating), IsPrimary: p.IsPrimary})
		}
		for _, c := range a.Channels {
			item.Channels = append(item.Channels, gen.ChannelSummary{Channel: gen.Channel(c.Channel), Enabled: c.Enabled})
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *server) UpdateAccount(ctx context.Context, req gen.UpdateAccountRequestObject) (gen.UpdateAccountResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	b := deref(req.Body)
	upd := model.AccountUpdate{CurrentPassword: deref(b.CurrentPassword), NewPassword: b.NewPassword}
	if b.Email != nil {
		email := string(*b.Email)
		upd.Email = &email
	}
	updated, err := svc.UpdateAccount(ctx, session, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateAccount200JSONResponse(toSessionInfo(updated)), nil
}
