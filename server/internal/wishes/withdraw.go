package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Withdrawing (concept 25, architecture 6.6; F-17): a profile that joined withdraws only itself; an
// owner with others hands the wish to the oldest of them; an owner alone withdraws the wish while
// it is queued or pending. The wish is withdrawn at once and marked «in Seerr noch zu löschen»;
// wishes.withdraw then deletes the own pending request in Seerr, also one Slouchcat never learnt
// the id of (a lost answer), so no request stays behind. A request that was approved meanwhile
// stays in Seerr and the wish opens again with the hint «War schon freigegeben».

// WithdrawWish withdraws the profile from a wish (withdrawWish): model.ErrNotFound for a wish that
// is not the profile's, *model.WishNotAllowedError (withdraw_disabled) when «Zurückziehen
// erlauben» is off, model.ErrWishNotWithdrawable past the approval or for a closed wish.
func (s *Service) WithdrawWish(ctx context.Context, p model.Profile, wishID int64) error {
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return err
	}
	r, err := s.read.GetWishRequester(ctx, wishq.GetWishRequesterParams{WishID: wishID, ProfileID: p.ID})
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (r.AccountID != p.AccountID || r.ClosedAt.Valid)) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("requester: %w", err)
	}
	if !ws.AllowWithdraw {
		return &model.WishNotAllowedError{Reason: model.WishReasonWithdrawDisabled}
	}
	queue := false
	err = s.inTx(ctx, func(q *wishq.Queries) error {
		w, err := q.GetWish(ctx, wishID)
		if err != nil {
			return fmt.Errorf("wish: %w", err)
		}
		if !isOpen(w.State) {
			return model.ErrWishNotWithdrawable
		}
		if r.Role == model.WishRoleJoined {
			if _, err := q.DeleteWishRequester(ctx, wishq.DeleteWishRequesterParams{WishID: wishID, ProfileID: p.ID}); err != nil {
				return fmt.Errorf("leave wish: %w", err)
			}
			return s.event(ctx, q, wishID, "withdrawn", map[string]any{"profileId": p.ID, "role": model.WishRoleJoined})
		}
		others, err := q.ListOpenWishRequesters(ctx, wishID)
		if err != nil {
			return fmt.Errorf("requesters: %w", err)
		}
		others = slices.DeleteFunc(others, func(o wishq.WishRequester) bool { return o.ProfileID == p.ID })
		if len(others) > 0 {
			// The oldest co-wisher becomes the owner; the request in Seerr stays.
			if _, err := q.DeleteWishRequester(ctx, wishq.DeleteWishRequesterParams{WishID: wishID, ProfileID: p.ID}); err != nil {
				return fmt.Errorf("leave wish: %w", err)
			}
			if _, err := q.SetWishRequesterRole(ctx, wishq.SetWishRequesterRoleParams{Role: model.WishRoleOwner, WishID: wishID, ProfileID: others[0].ProfileID}); err != nil {
				return fmt.Errorf("new owner: %w", err)
			}
			return s.event(ctx, q, wishID, "owner_changed", map[string]any{"from": p.ID, "to": others[0].ProfileID})
		}
		if w.State != model.WishQueued && w.State != model.WishPending {
			return model.ErrWishNotWithdrawable
		}
		now := s.now().UnixMilli()
		if _, err := q.SetWishState(ctx, wishq.SetWishStateParams{State: model.WishWithdrawn, Now: now, ClosedAt: num64(now), ID: wishID}); err != nil {
			return fmt.Errorf("withdraw: %w", err)
		}
		if err := s.closeSeasons(ctx, q, wishID, model.WishWithdrawn); err != nil {
			return err
		}
		// Nothing can be in Seerr when no attempt ever left and none runs now.
		release, idle := s.locks.try(wishID)
		if idle {
			defer release()
		}
		if !idle || w.Attempts > 0 || w.SubmittedAt.Valid {
			queue = true
			if _, err := q.SetWishWithdrawPending(ctx, wishq.SetWishWithdrawPendingParams{WithdrawPending: 1, Now: now, ID: wishID}); err != nil {
				return fmt.Errorf("withdraw pending: %w", err)
			}
		}
		return s.event(ctx, q, wishID, "withdrawn", map[string]any{"profileId": p.ID, "role": model.WishRoleOwner})
	})
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "wish withdrawn", "wish_id", wishID, "role", r.Role, "in_seerr", queue)
	if queue {
		return s.queueWish(ctx, JobWithdraw, wishID, 0)
	}
	return nil
}

func (s *Service) handleWithdraw(ctx context.Context, j *jobs.Job) error {
	var p wishPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.WishID < 1 {
		return jobs.Final(fmt.Errorf("%s: bad payload", JobWithdraw))
	}
	return s.withdraw(ctx, p.WishID)
}

// withdraw deletes the request of a withdrawn wish in Seerr (architecture 6.6 steps 2 to 7).
func (s *Service) withdraw(ctx context.Context, wishID int64) error {
	release, ok := s.locks.try(wishID)
	if !ok {
		return jobs.RetryLater(withdrawRetry, "a submission of the wish runs")
	}
	defer release()
	w, err := s.read.GetWish(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("wish: %w", err)
	}
	if w.WithdrawPending == 0 {
		return nil
	}
	if !w.SeerrUserID.Valid {
		return s.withdrawDone(ctx, w, nil)
	}
	uid := int(w.SeerrUserID.Int64)
	sess, err := s.sessionFor(ctx, w)
	if paused(err) {
		return nil // wishes.sync queues it again when Seerr is back; the mark stays
	}
	if err != nil {
		return err
	}
	var mi *seerr.Media
	if w.Media == model.WishMediaMovie {
		d, err := sess.Movie(ctx, int(w.TmdbID))
		if err != nil {
			return s.withdrawRefused(ctx, sess, w, err)
		}
		mi = d.MediaInfo
	} else {
		d, err := sess.TV(ctx, int(w.TmdbID))
		if err != nil {
			return s.withdrawRefused(ctx, sess, w, err)
		}
		mi = d.MediaInfo
	}
	known, seasons, err := s.knownRequests(ctx, w)
	if err != nil {
		return err
	}
	var stay []seerr.Request // requests that stay in Seerr: approved, or another user's
	var deleted []int
	if mi != nil {
		for _, r := range mi.Requests {
			own := r.RequestedBy != nil && r.RequestedBy.ID == uid
			ours := slices.Contains(known, r.ID) || (own && (w.Media == model.WishMediaMovie || carries(r, seasons)))
			switch {
			case !ours || r.Is4K:
			case !own && isOpenRequest(r):
				stay = append(stay, r)
			case own && r.Status == seerr.RequestPending:
				err := sess.DeleteRequest(ctx, r.ID)
				switch {
				case err == nil, errors.Is(err, seerr.ErrNotFound):
					deleted = append(deleted, r.ID)
				case errors.Is(err, seerr.ErrNoPermission):
					// No longer pending: read it again and keep it when it was approved.
					if cur, gerr := sess.Request(ctx, r.ID); gerr == nil && isOpenRequest(cur) {
						stay = append(stay, cur)
					} else if gerr != nil && !errors.Is(gerr, seerr.ErrNotFound) {
						return s.withdrawRetryLater(ctx, w)
					}
				default:
					return s.withdrawRefused(ctx, sess, w, err)
				}
			case own && (r.Status == seerr.RequestApproved || r.Status == seerr.RequestCompleted):
				stay = append(stay, r)
			}
		}
	}
	if len(stay) > 0 {
		return s.reopen(ctx, w, stay, mi)
	}
	return s.withdrawDone(ctx, w, deleted)
}

// knownRequests are the request ids Slouchcat stored for the wish and, for a series, its seasons.
func (s *Service) knownRequests(ctx context.Context, w wishq.Wish) ([]int, []int, error) {
	var ids, seasons []int
	if w.SeerrRequestID.Valid {
		ids = append(ids, int(w.SeerrRequestID.Int64))
	}
	rows, err := s.read.ListWishSeasons(ctx, w.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("seasons: %w", err)
	}
	for _, se := range rows {
		seasons = append(seasons, int(se.SeasonNumber))
		if se.SeerrRequestID.Valid && !slices.Contains(ids, int(se.SeerrRequestID.Int64)) {
			ids = append(ids, int(se.SeerrRequestID.Int64))
		}
	}
	return ids, seasons, nil
}

func carries(r seerr.Request, seasons []int) bool {
	return slices.ContainsFunc(r.Seasons, func(sr seerr.SeasonRequest) bool { return slices.Contains(seasons, sr.SeasonNumber) })
}

// withdrawRefused handles an error of Seerr during the withdrawal (architecture 6.6 step 7): a
// user that is gone took its requests with it; a refused key or an unreachable Seerr keeps the
// mark and tries again after the ladder, without a 24-h limit.
func (s *Service) withdrawRefused(ctx context.Context, sess seerr.Session, w wishq.Wish, err error) error {
	if errors.Is(err, seerr.ErrAccessDenied) {
		why := sess.ExplainRefusal(ctx, err)
		if errors.Is(why, seerr.ErrUserGone) {
			return s.withdrawDone(ctx, w, nil)
		}
		if errors.Is(why, seerr.ErrKeyInvalid) {
			s.recordOutcome(ctx, why)
		}
	}
	if errors.Is(err, seerr.ErrKeyInvalid) {
		s.recordOutcome(ctx, err)
	}
	return s.withdrawRetryLater(ctx, w)
}

func (s *Service) withdrawRetryLater(ctx context.Context, w wishq.Wish) error {
	now := s.now()
	delay := ladderDelay(w.Attempts + 1)
	if _, err := s.write.SetWishAttempt(ctx, wishq.SetWishAttemptParams{NextAttemptAt: num64(now.Add(delay).UnixMilli()), Now: now.UnixMilli(), ID: w.ID}); err != nil {
		return fmt.Errorf("attempt: %w", err)
	}
	return jobs.RetryLater(delay, "seerr not reached")
}

// withdrawDone clears the mark: nothing of the wish is left in Seerr.
func (s *Service) withdrawDone(ctx context.Context, w wishq.Wish, deleted []int) error {
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		if _, err := q.SetWishWithdrawPending(ctx, wishq.SetWishWithdrawPendingParams{WithdrawPending: 0, Now: s.now().UnixMilli(), ID: w.ID}); err != nil {
			return fmt.Errorf("withdraw pending: %w", err)
		}
		if len(deleted) == 0 {
			return nil
		}
		return s.event(ctx, q, w.ID, "withdraw_deleted", map[string]any{"requestIds": deleted})
	})
	if err == nil {
		s.log.InfoContext(ctx, "withdrawn wish cleared in seerr", "wish_id", w.ID, "deleted", len(deleted))
	}
	return err
}

// reopen opens a withdrawn wish again whose request stays in Seerr (architecture 6.6 step 6): its
// state follows the request; the profile reads «War schon freigegeben». Seasons whose request was
// deleted stay withdrawn. When another wish holds the title open meanwhile, the wish stays
// withdrawn.
func (s *Service) reopen(ctx context.Context, w wishq.Wish, stay []seerr.Request, mi *seerr.Media) error {
	ms := seerr.MediaUnknown
	if mi != nil {
		ms = mi.Status
	}
	state, detail := "", ""
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		now := s.now().UnixMilli()
		var reqID sql.NullInt64
		if w.Media == model.WishMediaMovie {
			state, detail, _ = requestState(stay[0].Status, ms)
			reqID = num64(int64(stay[0].ID))
		} else {
			seasons, err := q.ListWishSeasons(ctx, w.ID)
			if err != nil {
				return fmt.Errorf("seasons: %w", err)
			}
			var states []string
			for _, se := range seasons {
				st := se.State
				for _, r := range stay {
					if carries(r, []int{int(se.SeasonNumber)}) {
						st, _, _ = requestState(r.Status, seasonMedia(mi, int(se.SeasonNumber)))
						if _, err := q.SetWishSeasonRequest(ctx, wishq.SetWishSeasonRequestParams{SeerrRequestID: num64(int64(r.ID)),
							SeerrUpdatedAt: updatedOf(r), State: st, WishID: w.ID, SeasonNumber: se.SeasonNumber}); err != nil {
							return err
						}
						break
					}
				}
				states = append(states, st)
			}
			state = showState(states)
		}
		if _, err := q.SetWishSeerr(ctx, wishq.SetWishSeerrParams{SeerrRequestID: reqID, Now: now, ID: w.ID}); err != nil {
			return err
		}
		p := wishq.SetWishStateParams{State: state, StateDetail: nullString(detail), Now: now, ID: w.ID}
		if state == model.WishProcessing {
			p.ApprovedAt = num64(now)
		}
		if _, err := q.SetWishState(ctx, p); err != nil {
			return err
		}
		if _, err := q.SetWishWithdrawPending(ctx, wishq.SetWishWithdrawPendingParams{WithdrawPending: 0, Now: now, ID: w.ID}); err != nil {
			return err
		}
		return s.event(ctx, q, w.ID, "reopened", map[string]any{"state": state, "hint": model.WishHintAlreadyApproved})
	})
	if isUnique(err) {
		s.log.InfoContext(ctx, "withdrawn wish not reopened, another wish holds the title", "wish_id", w.ID)
		return s.withdrawDone(ctx, w, nil)
	}
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	s.log.InfoContext(ctx, "withdrawn wish reopened, its request stays in seerr", "wish_id", w.ID, "state", state)
	return nil
}
