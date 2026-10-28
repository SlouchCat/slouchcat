package auth

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// UpdateAccountAccess changes the rating ceiling, the library rights and the W-8 caps (bandwidth,
// output height, concurrent streams; they bound every profile of the account) of one account.
// The view visible_items applies the rights from the next request on, the caps apply from the
// next playback start.
//
// Only an account that administers nothing has access limits: an administrator is fixed at FSK 18
// and all libraries (ADR 0014, decision 12 f, and the table CHECK of migration 00022), because the
// administration views read system-wide and without a profile check. For an administrator this
// answers model.ErrAdminLimitsFixed; an unknown id is model.ErrNotFound.
func (s *Service) UpdateAccountAccess(ctx context.Context, id int64, in model.AccountAccessUpdate) error {
	acc, err := s.read.GetAccount(ctx, id)
	if err != nil {
		return notFoundOr(err, "get account")
	}
	if acc.Role == model.RoleAdmin {
		return model.ErrAdminLimitsFixed
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
	bitrate, height, streams := acc.MaxBitrateKbps, acc.MaxHeight, acc.MaxStreams
	if in.Limits != nil {
		caps := *in.Limits
		caps.WatchLimitMinutes = 0 // a profile setting; ignored for accounts
		verr = validateLimits(caps, false, verr)
		bitrate, height, streams, _ = limitColumns(caps)
	}
	libs, err := s.read.ListAccountLibraries(ctx, acc.ID)
	if err != nil {
		return fmt.Errorf("account libraries: %w", err)
	}
	if all == 1 {
		libs = []int64{}
	} else if in.LibraryIDs != nil {
		if libs, verr, err = s.guestLibraries(ctx, s.read, false, *in.LibraryIDs, verr); err != nil {
			return err
		}
	}
	if all == 0 && len(libs) == 0 && !hasField(verr, "libraryIds") {
		verr = verr.Add("libraryIds", "required")
	}
	if verr != nil {
		return verr
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	now := s.now().UnixMilli()
	if err := q.UpdateAccountLimits(ctx, appq.UpdateAccountLimitsParams{
		ID: acc.ID, MaxRating: maxRating, AllLibraries: all, MaxBitrateKbps: bitrate, MaxHeight: height, MaxStreams: streams, Now: now,
	}); err != nil {
		return fmt.Errorf("update limits: %w", err)
	}
	// A lowered ceiling also lowers the account's profiles, so the profile list never shows a
	// level the account no longer allows (the view caps them in any case).
	if _, err := q.ClampProfileRatings(ctx, appq.ClampProfileRatingsParams{AccountID: acc.ID, MaxRating: maxRating, Now: now}); err != nil {
		return fmt.Errorf("clamp profile ratings: %w", err)
	}
	if err := q.DeleteAccountLibraries(ctx, acc.ID); err != nil {
		return fmt.Errorf("replace account libraries: %w", err)
	}
	for _, lib := range libs {
		if err := q.InsertAccountLibrary(ctx, appq.InsertAccountLibraryParams{AccountID: acc.ID, LibraryID: lib}); err != nil {
			return fmt.Errorf("insert account library: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// DeleteAccount deletes an account with its profiles and progress (K-7). The order is deliberate:
// first every session of the account is revoked (own committed statement, so the next request is a
// 401 and no new stream can be authorised), then end runs (the API ends the account's playback:
// open streams, HLS output), then one transaction deletes the account, and end runs once more for
// a stream that a request already past its checks opened in between. From the next request on
// cookies and stream tokens answer 401.
//
// Two conflicts come first (ADR 0014): the owner is never deleted (model.ErrOwnerProtected), and
// neither is the last administrator (model.ErrLastAdmin). The second cannot normally be reached -
// the owner is an administrator and cannot be deleted or demoted, so an administrator is always
// left - and that is exactly why the check is cheap to keep: it is the guard for a database that
// somehow has no owner. Because that invariant holds, reading it before the chain is enough; no
// concurrent demotion can take the last administrator away behind our back.
func (s *Service) DeleteAccount(ctx context.Context, accountID int64, end func(ctx context.Context, accountID int64) error) error {
	acc, err := s.read.GetAccount(ctx, accountID)
	if err != nil {
		return notFoundOr(err, "get account")
	}
	if acc.IsOwner == 1 {
		return model.ErrOwnerProtected
	}
	if acc.Role == model.RoleAdmin {
		others, err := s.read.CountOtherAdmins(ctx, acc.ID)
		if err != nil {
			return fmt.Errorf("count other admins: %w", err)
		}
		if others == 0 {
			return model.ErrLastAdmin
		}
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
	s.log.InfoContext(ctx, "account deleted", "account_id", acc.ID, "role", acc.Role)
	return nil
}
