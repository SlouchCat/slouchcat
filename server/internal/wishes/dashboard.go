package wishes

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The overview of the administration (concept 25, administration 2, build step 1.6): the
// integration status «Seerr» and the hints of the wishes.

// IntegrationStatus is the dashboard entry `seerr`: missing without address or key, disabled when
// switched off, error when the key is refused, Seerr is unreachable or another instance waits,
// else configured (detail untested before a test or a job reached Seerr).
func (s *Service) IntegrationStatus(ctx context.Context) (model.IntegrationStatus, error) {
	st, err := s.Status(ctx)
	if err != nil {
		return model.IntegrationStatus{}, err
	}
	out := model.IntegrationStatus{Key: "seerr", State: model.IntegrationConfigured, CheckedAt: st.LastSuccess}
	switch st.State {
	case model.SeerrStateNotConfigured:
		out.State, out.CheckedAt = model.IntegrationMissing, nil
	case model.SeerrStateDisabled:
		out.State = model.IntegrationDisabled
	case model.SeerrStateKeyInvalid, model.SeerrStateUnreachable, model.SeerrStateInstanceChanged:
		out.State, out.Detail, out.CheckedAt = model.IntegrationError, st.State, st.LastAttempt
	case model.SeerrStateUntested:
		out.Detail = model.SeerrStateUntested
	}
	return out, nil
}

// DashboardWishes returns the hints of the wishes; nil while Seerr has no address or key.
func (s *Service) DashboardWishes(ctx context.Context) (*model.DashboardWishes, error) {
	st, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	if st.State == model.SeerrStateNotConfigured {
		return nil, nil
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := &model.DashboardWishes{
		Unmapped: st.Counts.Unmapped, UnmappedFallback: ws.Unmapped == model.WishesUnmappedFallback,
		Linked: []model.WishLinkHint{}, QuotaReached: []model.WishQuotaHint{},
	}
	switch st.State {
	case model.SeerrStateKeyInvalid, model.SeerrStateUnreachable, model.SeerrStateInstanceChanged:
		out.SeerrState = st.State
	}
	if out.Linked, err = s.RecentLinks(ctx); err != nil {
		return nil, err
	}
	all, err := s.WishMappings(ctx)
	if err != nil {
		return nil, err
	}
	gone := map[int]bool{}
	users, err := s.SeerrUsers(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		gone[u.ID] = u.Gone
	}
	for _, a := range all.Accounts {
		if a.SeerrUserID != nil && gone[*a.SeerrUserID] {
			out.GoneUsers++
		}
		q := a.Quota
		if q == nil || q.Unlimited {
			continue
		}
		h := model.WishQuotaHint{AccountID: a.AccountID, AccountEmail: a.Email}
		if q.Movie != nil && q.Movie.Remaining == 0 {
			h.Movie = q.Movie
		}
		if q.TV != nil && q.TV.Remaining == 0 {
			h.TV = q.TV
		}
		if h.Movie != nil || h.TV != nil {
			out.QuotaReached = append(out.QuotaReached, h)
		}
	}
	return out, nil
}
