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

// retryWish of the administration (concept 25, architecture 8.1 and 8.2; data model 4.5): a failed
// wish goes back the way it failed. A request Seerr reports as FAILED (fail_reason arr_failed)
// would lock the title, so Seerr retries it (POST /request/{id}/retry) and the wish is processing
// again; every other failure (submission, limit, a request or media gone) goes back to the outbox
// as queued, and the 24 h of the outbox count anew from the retry.

// eventRetried is the event of a retry; the outbox reads its time.
const eventRetried = "retried"

// eventRetriedWith is the event of a failed wish whose requesters went to a retried wish of the
// same Seerr request.
const eventRetriedWith = "retried_with"

// joinFailedOfRequests: the other wishes that failed on one of the retried requests stay closed,
// and their open requesters join the retried wish as co-wishers, so they hear of the availability
// (review Minor 5).
func (s *Service) joinFailedOfRequests(ctx context.Context, q *wishq.Queries, wishID int64, ids []int, now int64) error {
	req := make([]sql.NullInt64, 0, len(ids))
	for _, id := range ids {
		req = append(req, num64(int64(id)))
	}
	others, err := q.ListOtherFailedWishesOfRequests(ctx, wishq.ListOtherFailedWishesOfRequestsParams{WishID: wishID, RequestIds: req, SeasonRequestIds: req})
	if err != nil {
		return fmt.Errorf("other failed wishes: %w", err)
	}
	for _, other := range others {
		reqs, err := q.ListWishRequesters(ctx, other)
		if err != nil {
			return fmt.Errorf("requesters: %w", err)
		}
		for _, r := range reqs {
			if r.ClosedAt.Valid {
				continue
			}
			if _, err := q.InsertWishRequester(ctx, wishq.InsertWishRequesterParams{WishID: wishID, ProfileID: r.ProfileID, AccountID: r.AccountID, Role: model.WishRoleJoined, Now: now}); err != nil {
				return fmt.Errorf("join retried wish: %w", err)
			}
		}
		if err := s.event(ctx, q, other, eventRetriedWith, map[string]any{"wishId": wishID}); err != nil {
			return err
		}
	}
	return nil
}

// RetryWish retries the failed wish id for the administrator accountID (named in the event):
// model.ErrNotFound for an unknown wish,
// model.ErrWishNotRetryable when it did not fail, failed with its account or another open wish
// holds the title, model.ErrSeerrNotConfigured or model.ErrSeerrUnreachable when Seerr's retry is
// needed and Seerr does not answer.
func (s *Service) RetryWish(ctx context.Context, id, accountID int64) error {
	w, err := s.read.GetWish(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("wish: %w", err)
	}
	if w.State != model.WishFailed || w.FailReason.String == model.WishFailAccountDeleted {
		return model.ErrWishNotRetryable
	}
	release, ok := s.locks.try(id)
	if !ok {
		return model.ErrWishNotRetryable // a submission or withdrawal of the wish runs right now
	}
	defer release()
	if w.FailReason.String == model.WishFailArrFailed {
		return s.retryInSeerr(ctx, w, accountID)
	}
	if err := s.reopenFailed(ctx, w, model.WishQueued, nil, accountID); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "failed wish back in the outbox", "wish_id", id, "reason", w.FailReason.String)
	return s.queueWish(ctx, JobSubmit, id, 0)
}

// retryInSeerr asks Seerr to retry the failed requests of the wish (the film's or those of its
// failed seasons) and moves the wish to processing.
func (s *Service) retryInSeerr(ctx context.Context, w wishq.Wish, accountID int64) error {
	var ids []int
	if w.Media == model.WishMediaMovie {
		if w.SeerrRequestID.Valid {
			ids = append(ids, int(w.SeerrRequestID.Int64))
		}
	} else {
		seasons, err := s.read.ListWishSeasons(ctx, w.ID)
		if err != nil {
			return fmt.Errorf("seasons: %w", err)
		}
		for _, se := range seasons {
			if se.State == model.WishFailed && se.SeerrRequestID.Valid && !slices.Contains(ids, int(se.SeerrRequestID.Int64)) {
				ids = append(ids, int(se.SeerrRequestID.Int64))
			}
		}
	}
	if len(ids) == 0 {
		return model.ErrWishNotRetryable
	}
	admin, err := s.AdminSession(ctx)
	switch {
	case errors.Is(err, model.ErrSeerrNotConfigured):
		return err
	case paused(err):
		return fmt.Errorf("%w: %w", model.ErrSeerrUnreachable, err)
	case err != nil:
		return err
	}
	retried := 0
	for _, rid := range ids {
		_, err := admin.RetryRequest(ctx, rid)
		switch {
		case err == nil:
			retried++
		case errors.Is(err, seerr.ErrConflict), errors.Is(err, seerr.ErrNotFound):
			// No longer failed or gone: the sync brings the request's state.
			s.log.InfoContext(ctx, "seerr request not retried", "wish_id", w.ID, "seerr_request_id", rid, "reason", errorClass(err))
		case seerrErr(err) != nil:
			return fmt.Errorf("%w: %s", model.ErrSeerrUnreachable, errorClass(err))
		default:
			return err
		}
	}
	if retried == 0 {
		return model.ErrWishNotRetryable
	}
	if err := s.reopenFailed(ctx, w, model.WishProcessing, ids, accountID); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "failed wish retried in seerr", "wish_id", w.ID, "requests", retried)
	return nil
}

// reopenFailed moves the failed wish and its failed seasons (of the requests ids, nil: all) to state,
// clears the fail reason and the closing time and writes the event retried. Back to the outbox
// (queued) the old request ids go into the event and are cleared, so the outbox sends those seasons
// (or the film) again instead of looping «attached» (review M-2). Another open wish of the title
// (a partial unique index) makes it model.ErrWishNotRetryable. accountID is the acting
// administrator (security review N-2), 0 for none.
func (s *Service) reopenFailed(ctx context.Context, w wishq.Wish, state string, ids []int, accountID int64) error {
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		now := s.now().UnixMilli()
		var dropped []int64
		if state == model.WishQueued && w.SeerrRequestID.Valid {
			dropped = append(dropped, w.SeerrRequestID.Int64)
			if _, err := q.ClearWishRequest(ctx, wishq.ClearWishRequestParams{Now: now, ID: w.ID}); err != nil {
				return fmt.Errorf("clear request: %w", err)
			}
		}
		n, err := q.SetWishState(ctx, wishq.SetWishStateParams{State: state, Now: now, ID: w.ID})
		if err != nil {
			return fmt.Errorf("reopen wish: %w", err)
		}
		if n == 0 {
			return model.ErrNotFound
		}
		seasons, err := q.ListWishSeasons(ctx, w.ID)
		if err != nil {
			return fmt.Errorf("seasons: %w", err)
		}
		for _, se := range seasons {
			if se.State != model.WishFailed || (ids != nil && !slices.Contains(ids, int(se.SeerrRequestID.Int64))) {
				continue
			}
			if state == model.WishQueued {
				if se.SeerrRequestID.Valid && !slices.Contains(dropped, se.SeerrRequestID.Int64) {
					dropped = append(dropped, se.SeerrRequestID.Int64)
				}
				if _, err := q.SetWishSeasonRequest(ctx, wishq.SetWishSeasonRequestParams{State: state, WishID: w.ID, SeasonNumber: se.SeasonNumber}); err != nil {
					return fmt.Errorf("season request: %w", err)
				}
				continue
			}
			if _, err := q.SetWishSeasonState(ctx, wishq.SetWishSeasonStateParams{State: state, WishID: w.ID, SeasonNumber: se.SeasonNumber}); err != nil {
				return fmt.Errorf("season state: %w", err)
			}
		}
		d := map[string]any{"from": model.WishFailed, "to": state, "reason": w.FailReason.String}
		if len(dropped) > 0 {
			d["requestIds"] = dropped
		}
		if accountID > 0 {
			d["accountId"] = accountID
		}
		if err := s.event(ctx, q, w.ID, eventRetried, d); err != nil {
			return err
		}
		if state == model.WishProcessing {
			return s.joinFailedOfRequests(ctx, q, w.ID, ids, now)
		}
		return nil
	})
	if isUnique(err) {
		return model.ErrWishNotRetryable
	}
	return err
}
