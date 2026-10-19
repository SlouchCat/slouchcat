package auth

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// ListGuests returns the guest accounts, newest first.
func (s *Service) ListGuests(ctx context.Context) ([]model.Guest, error) {
	rows, err := s.read.ListGuestAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list guests: %w", err)
	}
	links, err := s.read.ListGuestLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list guest libraries: %w", err)
	}
	libs := map[int64][]int64{}
	for _, l := range links {
		libs[l.AccountID] = append(libs[l.AccountID], l.LibraryID)
	}
	out := make([]model.Guest, 0, len(rows))
	for _, r := range rows {
		out = append(out, toGuest(r.ID, r.Email, r.MaxRating, r.AllLibraries, r.CreatedAt, r.LastSeenAt.Int64, r.LastSeenAt.Valid, int(r.ProfileCount), libs[r.ID]))
	}
	return out, nil
}

func toGuest(id int64, email string, maxRating, all, created, lastSeen int64, seen bool, profiles int, libs []int64) model.Guest {
	if libs == nil {
		libs = []int64{}
	}
	g := model.Guest{
		ID: id, Email: email, MaxRating: int(maxRating), AllLibraries: all == 1, LibraryIDs: libs,
		ProfileCount: profiles, CreatedAt: time.UnixMilli(created),
	}
	if seen {
		t := time.UnixMilli(lastSeen)
		g.LastSeenAt = &t
	}
	return g
}

// UpdateGuest changes the rating ceiling and library rights of a guest account (model.ErrNotFound
// for an unknown id or an owner). The view visible_items applies them from the next request on.
func (s *Service) UpdateGuest(ctx context.Context, id int64, in model.GuestUpdate) (model.Guest, error) {
	acc, err := s.read.GetGuestAccount(ctx, id)
	if err != nil {
		return model.Guest{}, notFoundOr(err, "get guest")
	}
	var verr *model.ValidationError
	maxRating, all := acc.MaxRating, acc.AllLibraries
	if in.MaxRating != nil {
		if !slices.Contains(ratingLevels, *in.MaxRating) {
			verr = verr.Add("maxRating", "invalid_value")
		} else {
			maxRating = int64(*in.MaxRating)
		}
	}
	if in.AllLibraries != nil {
		all = boolInt(*in.AllLibraries)
	}
	libs, err := s.read.ListAccountLibraries(ctx, acc.ID)
	if err != nil {
		return model.Guest{}, fmt.Errorf("account libraries: %w", err)
	}
	if all == 1 {
		libs = []int64{}
	} else if in.LibraryIDs != nil {
		if libs, verr, err = s.guestLibraries(ctx, s.read, false, *in.LibraryIDs, verr); err != nil {
			return model.Guest{}, err
		}
	}
	if all == 0 && len(libs) == 0 && !hasField(verr, "libraryIds") {
		verr = verr.Add("libraryIds", "required")
	}
	if verr != nil {
		return model.Guest{}, verr
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Guest{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	now := s.now().UnixMilli()
	if err := q.UpdateAccountLimits(ctx, appq.UpdateAccountLimitsParams{ID: acc.ID, MaxRating: maxRating, AllLibraries: all, Now: now}); err != nil {
		return model.Guest{}, fmt.Errorf("update limits: %w", err)
	}
	// A lowered ceiling also lowers the guest's profiles, so the profile list never shows a level
	// the account no longer allows (the view caps them in any case).
	if _, err := q.ClampProfileRatings(ctx, appq.ClampProfileRatingsParams{AccountID: acc.ID, MaxRating: maxRating, Now: now}); err != nil {
		return model.Guest{}, fmt.Errorf("clamp profile ratings: %w", err)
	}
	if err := q.DeleteAccountLibraries(ctx, acc.ID); err != nil {
		return model.Guest{}, fmt.Errorf("replace account libraries: %w", err)
	}
	for _, lib := range libs {
		if err := q.InsertAccountLibrary(ctx, appq.InsertAccountLibraryParams{AccountID: acc.ID, LibraryID: lib}); err != nil {
			return model.Guest{}, fmt.Errorf("insert account library: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Guest{}, fmt.Errorf("commit: %w", err)
	}
	n, err := s.read.CountProfilesByAccount(ctx, acc.ID)
	if err != nil {
		return model.Guest{}, fmt.Errorf("count profiles: %w", err)
	}
	return toGuest(acc.ID, acc.Email, maxRating, all, acc.CreatedAt, acc.LastSeenAt.Int64, acc.LastSeenAt.Valid, int(n), libs), nil
}

// RevokeAccount revokes a guest account (K-7). The order is deliberate: first every session of the
// account is revoked (own committed statement, so the next request of the guest is a 401 and no new
// stream can be authorised), then end runs (the API ends the account's playback: open streams, HLS
// output), then one transaction deletes the account with its profiles and progress, and end runs
// once more for a stream that a request already past its checks opened in between. From the next
// request on cookies and stream tokens answer 401. An unknown id and an owner account are
// model.ErrNotFound.
func (s *Service) RevokeAccount(ctx context.Context, accountID int64, end func(ctx context.Context, accountID int64) error) error {
	acc, err := s.read.GetGuestAccount(ctx, accountID)
	if err != nil {
		return notFoundOr(err, "get guest")
	}
	if _, err := s.write.RevokeAccountSessions(ctx, appq.RevokeAccountSessionsParams{AccountID: acc.ID, Now: nullInt(s.now().UnixMilli())}); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	if end != nil {
		if err := end(ctx, acc.ID); err != nil {
			return err
		}
	}
	if _, err := s.write.DeleteAccount(ctx, acc.ID); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if end != nil {
		if err := end(ctx, acc.ID); err != nil {
			return err
		}
	}
	s.log.InfoContext(ctx, "guest account revoked", "account_id", acc.ID)
	return nil
}
