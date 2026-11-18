package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
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
		Open: st.Counts.Open, Queued: st.Counts.Queued,
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
	if out.TagsFailed, out.DownloadsStalled, err = s.arrHints(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// maxArrHints caps each list of the stage 4 hints; the overview names a few titles and the count.
const maxArrHints = 20

// arrHints are the hints «n Wünsche ohne Profil-Tag» (tags failed, not those waiting for the
// title) and «n Downloads verzögert» (a fresh read of the queue says stalled).
func (s *Service) arrHints(ctx context.Context) (tags, stalled []model.WishServiceHint, err error) {
	tagRows, err := s.read.ListFailedTagHints(ctx, maxArrHints)
	if err != nil {
		return nil, nil, fmt.Errorf("failed tag hints: %w", err)
	}
	tags = make([]model.WishServiceHint, 0, len(tagRows))
	for _, r := range tagRows {
		tags = append(tags, model.WishServiceHint{WishID: r.WishID, Title: r.Title, Service: r.Service, Error: r.LastError})
	}
	rows, err := s.read.ListStalledProgressHints(ctx, wishq.ListStalledProgressHintsParams{
		FreshSince: s.now().Add(-progressStale).UnixMilli(), MaxRows: maxArrHints})
	if err != nil {
		return nil, nil, fmt.Errorf("stalled download hints: %w", err)
	}
	stalled = make([]model.WishServiceHint, 0, len(rows))
	for _, r := range rows {
		stalled = append(stalled, model.WishServiceHint{WishID: r.WishID, Title: r.Title, Service: r.Service})
	}
	return tags, stalled, nil
}

// SpotlightWishes returns the wish of each title for the missing entries of Verwaltung ›
// Spotlight (open first, else the newest; absent without any) and whether the administration can
// request: Seerr with address and key, switched on, «Wünsche aktiv» (concept 25 step 3.3, N-71).
func (s *Service) SpotlightWishes(ctx context.Context, titles []model.TitleKey) (map[model.TitleKey]model.WishMark, bool, error) {
	out := map[model.TitleKey]model.WishMark{}
	for _, t := range titles {
		w, err := s.read.GetTitleWish(ctx, wishq.GetTitleWishParams{Media: t.Media, TmdbID: t.TMDBID})
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue
		case err != nil:
			return nil, false, fmt.Errorf("wish of title: %w", err)
		}
		out[t] = model.WishMark{ID: w.ID, State: w.State, StateDetail: w.StateDetail.String}
	}
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return nil, false, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return nil, false, err
	}
	return out, cfg.Configured() && cfg.Enabled && ws.Enabled, nil
}
