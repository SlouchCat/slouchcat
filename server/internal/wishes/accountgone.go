package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The wishes of a deleted account or profile (concept 25, architecture 8.1 «Konto gelöscht»,
// users and profiles 10; AK-128). Before the account goes, its wishes with a co-wisher of another
// account go to the oldest of them; the others are closed: open ones fail with account_deleted,
// every one gets closed_at, and their request ids go into the history, so they lock no title and
// the sync takes their requests over as «nicht zuordenbar». A queued wish that goes to a co-wisher
// is sent as that co-wisher's Seerr user: nothing of it is in Seerr yet. The wishes a deleted
// profile owns go to the primary profile of its account; what the profile joined falls away.

// eventAccountDeleted is the history entry of a wish whose account was deleted.
const eventAccountDeleted = "account_deleted"

// closeAccountWishes hands over or closes the wishes of the account before it is deleted, in q
// (the transaction that deletes the account, review Minor 1), and returns the queued wishes to
// send as their new owner after the commit.
func (s *Service) closeAccountWishes(ctx context.Context, q *wishq.Queries, accountID int64) ([]repointWish, error) {
	var repoint []repointWish
	closed, handed := 0, 0
	err := func() error {
		ids, err := q.ListAccountWishIDs(ctx, accountID)
		if err != nil {
			return fmt.Errorf("wishes of account: %w", err)
		}
		now := s.now().UnixMilli()
		for _, id := range ids {
			w, err := q.GetWish(ctx, id)
			if err != nil {
				return fmt.Errorf("wish: %w", err)
			}
			reqs, err := q.ListWishRequesters(ctx, id)
			if err != nil {
				return fmt.Errorf("requesters: %w", err)
			}
			owned := false
			var others []wishq.WishRequester
			for _, r := range reqs {
				switch {
				case r.AccountID == accountID:
					owned = owned || r.Role == model.WishRoleOwner
				case !r.ClosedAt.Valid:
					others = append(others, r)
				}
			}
			if !owned {
				continue // what the account joined falls away with it
			}
			if _, err := q.DeleteAccountWishRequesters(ctx, wishq.DeleteAccountWishRequestersParams{WishID: id, AccountID: accountID}); err != nil {
				return fmt.Errorf("requesters of account: %w", err)
			}
			if len(others) > 0 {
				next := oldest(others)
				if _, err := q.SetWishRequesterRole(ctx, wishq.SetWishRequesterRoleParams{Role: model.WishRoleOwner, WishID: id, ProfileID: next.ProfileID}); err != nil {
					return fmt.Errorf("new owner: %w", err)
				}
				if w.State == model.WishQueued {
					repoint = append(repoint, repointWish{wishID: id, accountID: next.AccountID, profileID: next.ProfileID})
				}
				handed++
				if err := s.event(ctx, q, id, "owner_changed", map[string]any{"to": next.ProfileID, "reason": eventAccountDeleted}); err != nil {
					return err
				}
				continue
			}
			if err := s.closeOfDeleted(ctx, q, w, now); err != nil {
				return err
			}
			closed++
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}
	s.log.InfoContext(ctx, "wishes of a deleted account", "account_id", accountID, "closed", closed, "handed_over", handed)
	return repoint, nil
}

// oldest is the requester that joined first.
func oldest(rs []wishq.WishRequester) wishq.WishRequester {
	out := rs[0]
	for _, r := range rs[1:] {
		if r.CreatedAt < out.CreatedAt || (r.CreatedAt == out.CreatedAt && r.ProfileID < out.ProfileID) {
			out = r
		}
	}
	return out
}

// closeOfDeleted closes a wish whose only account is deleted: an open one fails with
// account_deleted (its seasons too), every one gets closed_at, the request ids go into the event.
func (s *Service) closeOfDeleted(ctx context.Context, q *wishq.Queries, w wishq.Wish, now int64) error {
	ids, err := wishRequestIDs(ctx, q, w)
	if err != nil {
		return err
	}
	detail := map[string]any{"from": w.State, "requestIds": ids}
	if isOpen(w.State) {
		if _, err := q.SetWishState(ctx, wishq.SetWishStateParams{State: model.WishFailed, FailReason: nullString(model.WishFailAccountDeleted),
			Now: now, ClosedAt: num64(now), ID: w.ID}); err != nil {
			return fmt.Errorf("close wish: %w", err)
		}
		if err := s.closeSeasons(ctx, q, w.ID, model.WishFailed); err != nil {
			return err
		}
		detail["to"] = model.WishFailed
	} else if _, err := q.SetWishClosedAt(ctx, wishq.SetWishClosedAtParams{Now: now, ID: w.ID}); err != nil {
		return fmt.Errorf("closed at: %w", err)
	}
	if _, err := q.ClearWishRequest(ctx, wishq.ClearWishRequestParams{Now: now, ID: w.ID}); err != nil {
		return fmt.Errorf("clear request: %w", err)
	}
	if _, err := q.ClearWishSeasonRequests(ctx, w.ID); err != nil {
		return fmt.Errorf("clear season requests: %w", err)
	}
	return s.event(ctx, q, w.ID, eventAccountDeleted, detail)
}

// wishRequestIDs are the request ids of the wish and its seasons.
func wishRequestIDs(ctx context.Context, q *wishq.Queries, w wishq.Wish) ([]int64, error) {
	ids := []int64{}
	if w.SeerrRequestID.Valid {
		ids = append(ids, w.SeerrRequestID.Int64)
	}
	seasons, err := q.ListWishSeasons(ctx, w.ID)
	if err != nil {
		return nil, fmt.Errorf("seasons: %w", err)
	}
	for _, se := range seasons {
		if se.SeerrRequestID.Valid && !slices.Contains(ids, se.SeerrRequestID.Int64) {
			ids = append(ids, se.SeerrRequestID.Int64)
		}
	}
	return ids, nil
}

// repointWish is a queued wish that went to a co-wisher of another account.
type repointWish struct {
	wishID, accountID, profileID int64
}

// repoint sends a queued wish that went to a co-wisher as that co-wisher's Seerr user; nothing of
// it is in Seerr yet. Without a user to send as, it fails with seerr_user_gone. A submission that
// runs right now keeps the lock; the next attempt re-resolves the sender (orphanSender).
func (s *Service) repoint(ctx context.Context, r repointWish) {
	release, ok := s.locks.try(r.wishID)
	if !ok {
		return
	}
	defer release()
	rights, sender, err := s.SenderFor(ctx, r.accountID, r.profileID)
	if err == nil && !rights.CanRequest {
		err = s.failWish(ctx, r.wishID, model.WishFailSeerrUserGone, map[string]any{"detail": "mapping_changed"})
	} else if err == nil {
		_, err = s.write.SetWishSeerrUser(ctx, wishq.SetWishSeerrUserParams{SeerrUserID: num64(int64(sender.SeerrUserID)), Now: s.now().UnixMilli(), ID: r.wishID})
	}
	if err != nil {
		s.log.WarnContext(ctx, "queued wish of a deleted account not handed over", "wish_id", r.wishID, "err", err)
	}
}

// ProfileDeleting is the hook of package auth before a profile is deleted (users and profiles 10),
// in tx, the transaction that deletes the profile (review m-6): the wishes the profile owns go to
// the primary profile of its account, with the event owner_changed; what it joined falls away
// with it. Nothing changes in Seerr; an error stops the deletion.
func (s *Service) ProfileDeleting(ctx context.Context, accountID, profileID int64, tx *sql.Tx) error {
	q := wishq.New(tx)
	primary, err := q.GetPrimaryProfile(ctx, accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("primary profile: %w", err)
	}
	if primary == profileID {
		return nil
	}
	ids, err := q.ListProfileOwnedWishIDs(ctx, profileID)
	if err != nil {
		return fmt.Errorf("wishes of profile: %w", err)
	}
	for _, id := range ids {
		_, err := q.GetWishRequester(ctx, wishq.GetWishRequesterParams{WishID: id, ProfileID: primary})
		switch {
		case err == nil: // the primary profile joined already: it becomes the owner
			if _, err := q.DeleteWishRequester(ctx, wishq.DeleteWishRequesterParams{WishID: id, ProfileID: profileID}); err != nil {
				return fmt.Errorf("leave wish: %w", err)
			}
			if _, err := q.SetWishRequesterRole(ctx, wishq.SetWishRequesterRoleParams{Role: model.WishRoleOwner, WishID: id, ProfileID: primary}); err != nil {
				return fmt.Errorf("new owner: %w", err)
			}
		case errors.Is(err, sql.ErrNoRows):
			if _, err := q.MoveWishRequester(ctx, wishq.MoveWishRequesterParams{ToProfileID: primary, WishID: id, FromProfileID: profileID}); err != nil {
				return fmt.Errorf("move owner: %w", err)
			}
		default:
			return fmt.Errorf("requester: %w", err)
		}
		if err := s.event(ctx, q, id, "owner_changed", map[string]any{"from": profileID, "to": primary, "reason": "profile_deleted"}); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		s.log.InfoContext(ctx, "wishes of a deleted profile go to the primary profile", "profile_id", profileID, "wishes", len(ids))
	}
	return nil
}
