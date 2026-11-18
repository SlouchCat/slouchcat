package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Approving and declining in the administration (concept 25 stage 4, build step 4.4, F-16; data
// model 4.5 approveWish, declineWish). Both only pass the administrator's choice on to Seerr
// (POST /request/{id}/approve|decline as user 1, which has MANAGE_REQUESTS) for every request of
// the wish that waits for an approval: the request of a film, the requests of the pending seasons
// of a series. Slouchcat decides nothing itself (rule 11): the state follows Seerr's answer on the
// normal path of the sync (architecture 8.1), so an approval moves the wish to processing and
// tells its requesters through the status channel, a decline closes it as declined. The history
// names the acting account.

// ApproveWish approves every request of the wish that waits for an approval in Seerr (a pending
// film; the pending seasons of a series, also of a series wish that is processing or partial as a
// whole) and returns the wish as the administration sees it. Errors: model.ErrNotFound for an
// unknown wish; model.ErrWishNotPending when nothing of the wish is pending, or Seerr says none of
// its requests is pending any more (the state is then read anew);
// model.ErrInstanceChanged while another Seerr instance waits for the administrator;
// model.ErrSeerrNotConfigured without address or key; model.ErrSeerrUnreachable when Seerr does
// not answer or is switched off; model.ErrSeerrFailed when Seerr answers with another error.
// When Seerr fails before the first request, nothing changes.
func (s *Service) ApproveWish(ctx context.Context, wishID, accountID int64) (model.AdminWish, error) {
	return s.review(ctx, wishID, accountID, true)
}

// DeclineWish declines the pending requests of the wish in Seerr; the rule and the errors are those
// of ApproveWish.
func (s *Service) DeclineWish(ctx context.Context, wishID, accountID int64) (model.AdminWish, error) {
	return s.review(ctx, wishID, accountID, false)
}

func (s *Service) review(ctx context.Context, wishID, accountID int64, approve bool) (model.AdminWish, error) {
	w, err := s.read.GetWish(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AdminWish{}, model.ErrNotFound
	}
	if err != nil {
		return model.AdminWish{}, fmt.Errorf("wish: %w", err)
	}
	// The rule (review, open question): a film while the wish is pending; a series while any of
	// its seasons waits in Seerr, also when the wish as a whole is processing or partial.
	if !isOpen(w.State) || (w.Media == model.WishMediaMovie && w.State != model.WishPending) {
		return model.AdminWish{}, model.ErrWishNotPending
	}
	ids, err := s.pendingRequests(ctx, w)
	if err != nil {
		return model.AdminWish{}, err
	}
	if len(ids) == 0 {
		return model.AdminWish{}, model.ErrWishNotPending
	}
	admin, err := s.reviewSession(ctx)
	if err != nil {
		return model.AdminWish{}, err
	}
	var answers []seerr.Request
	var decided []int
	var callErr error
	for _, id := range ids {
		var r seerr.Request
		if approve {
			r, err = admin.ApproveRequest(ctx, id)
		} else {
			r, err = admin.DeclineRequest(ctx, id)
		}
		if errors.Is(err, seerr.ErrConflict) {
			// Not pending in Seerr any more (approved or declined there): read its state.
			r, err = admin.Request(ctx, id)
		} else if err == nil {
			decided = append(decided, id)
		}
		if err != nil {
			callErr = err
			break
		}
		answers = append(answers, r)
	}
	if len(answers) > 0 {
		if err := s.applyReview(ctx, w.ID, answers, decided, accountID, approve); err != nil {
			return model.AdminWish{}, err
		}
	}
	if callErr != nil {
		s.recordOutcome(ctx, seerrErr(callErr))
		return model.AdminWish{}, reviewErr(callErr)
	}
	s.recordOutcome(ctx, nil)
	if len(decided) == 0 {
		return model.AdminWish{}, model.ErrWishNotPending
	}
	s.log.InfoContext(ctx, "wish reviewed in seerr", "wish_id", w.ID, "approve", approve, "requests", len(decided), "account_id", accountID)
	return s.adminWish(ctx, w.ID)
}

// pendingRequests are the request ids of the wish that wait for an approval: the film's, or those
// of the pending seasons of a series (several when the seasons went out in more than one request).
func (s *Service) pendingRequests(ctx context.Context, w wishq.Wish) ([]int, error) {
	if w.Media == model.WishMediaMovie {
		if !w.SeerrRequestID.Valid {
			return nil, nil
		}
		return []int{int(w.SeerrRequestID.Int64)}, nil
	}
	seasons, err := s.read.ListWishSeasons(ctx, w.ID)
	if err != nil {
		return nil, fmt.Errorf("seasons: %w", err)
	}
	var ids []int
	for _, se := range seasons {
		if se.State == model.WishPending && se.SeerrRequestID.Valid && !slices.Contains(ids, int(se.SeerrRequestID.Int64)) {
			ids = append(ids, int(se.SeerrRequestID.Int64))
		}
	}
	return ids, nil
}

// reviewSession is the administrator session of a review. It refuses while another instance
// waits for the administrator: a request id would mean another request there (architecture 7.4).
func (s *Service) reviewSession(ctx context.Context) (seerr.Session, error) {
	if pending, err := s.instancePending(ctx); err != nil {
		return seerr.Session{}, err
	} else if pending != nil {
		return seerr.Session{}, model.ErrInstanceChanged
	}
	admin, err := s.AdminSession(ctx)
	if errors.Is(err, errDisabled) {
		return seerr.Session{}, fmt.Errorf("%w: seerr is switched off", model.ErrSeerrUnreachable)
	}
	return admin, err
}

// applyReview writes the requests Seerr answered with on the path of the sync and the history
// entry of the acting account, then announces the changes.
func (s *Service) applyReview(ctx context.Context, wishID int64, answers []seerr.Request, decided []int, accountID int64, approve bool) error {
	var changes []change
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		changes = changes[:0]
		for _, r := range answers {
			ch, _, err := s.applyRequest(ctx, q, r)
			if err != nil {
				return err
			}
			changes = append(changes, ch...)
		}
		if len(decided) == 0 {
			return nil
		}
		kind := "declined_by_admin"
		if approve {
			kind = "approved_by_admin"
		}
		return s.event(ctx, q, wishID, kind, map[string]any{"accountId": accountID, "requestIds": decided})
	})
	if err != nil {
		return err
	}
	s.afterChanges(ctx, changes)
	return nil
}

// reviewErr maps an error of Seerr to the errors of approveWish and declineWish.
func reviewErr(err error) error {
	var se *seerr.Error
	switch {
	case errors.Is(err, seerr.ErrUnreachable):
		return fmt.Errorf("%w: %w", model.ErrSeerrUnreachable, err)
	case errors.As(err, &se):
		return fmt.Errorf("%w: %w", model.ErrSeerrFailed, err)
	}
	return err
}

// adminWish returns one wish as the administration lists it.
func (s *Service) adminWish(ctx context.Context, wishID int64) (model.AdminWish, error) {
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.AdminWish{}, err
	}
	w, err := s.read.GetWish(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AdminWish{}, model.ErrNotFound
	}
	if err != nil {
		return model.AdminWish{}, fmt.Errorf("wish: %w", err)
	}
	names, err := s.read.ListWishRequesterNames(ctx, wishID)
	if err != nil {
		return model.AdminWish{}, fmt.Errorf("requesters: %w", err)
	}
	a := model.AdminWish{SeerrUserID: int(w.SeerrUserID.Int64), SeerrRequestID: int(w.SeerrRequestID.Int64), Attempts: int(w.Attempts),
		Requesters: []model.AdminWishRequester{}}
	for _, n := range names {
		a.Requesters = append(a.Requesters, model.AdminWishRequester{AccountID: n.AccountID, AccountEmail: n.AccountEmail,
			ProfileID: n.ProfileID, ProfileName: n.ProfileName, Role: n.Role, Closed: n.ClosedAt.Valid})
	}
	if a.Wish, err = s.toWish(ctx, w, "", ws); err != nil {
		return model.AdminWish{}, err
	}
	return a, nil
}
