package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Mapping is the Seerr user of each account (concept 25, users and profiles 5 to 7, data model
// 2.2): the list of the Seerr users, the overview per account and the choice by hand. There is no
// user per profile (F-27, N-100); the shared user «slouchcat-gaeste» never belongs to an account.
type Mapping struct{ s *Service }

// Origins, states and password states of a mapping (data model 2.2).
const (
	OriginCreated = "created"
	OriginLinked  = "linked"
	OriginManual  = "manual"

	StateReady   = "ready"
	StateWaiting = "waiting"

	PasswordSynced    = "synced"
	PasswordNextLogin = "next_login"
	PasswordOff       = "off"
	PasswordAdmin     = "admin"
	PasswordExternal  = "external"
)

// SeerrUsers returns every known Seerr user with the account it belongs to (listSeerrUsers).
func (m *Mapping) SeerrUsers(ctx context.Context) ([]model.SeerrUser, error) {
	rows, err := m.s.read.ListSeerrUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list seerr users: %w", err)
	}
	out := make([]model.SeerrUser, 0, len(rows))
	for _, r := range rows {
		u := toSeerrUser(r.SeerrUser)
		if r.MappedAccountID.Valid {
			u.MappedTo = &model.SeerrUserAccount{AccountID: r.MappedAccountID.Int64, Email: r.MappedAccountEmail.String}
		}
		out = append(out, u)
	}
	return out, nil
}

// WishMappings returns every account with its Seerr user, limit and state, the users without an
// account and the shared user (getWishMappings).
func (m *Mapping) WishMappings(ctx context.Context) (model.WishMappings, error) {
	users, err := m.SeerrUsers(ctx)
	if err != nil {
		return model.WishMappings{}, err
	}
	byID := make(map[int]model.SeerrUser, len(users))
	out := model.WishMappings{Accounts: []model.WishAccountMapping{}, UnlinkedUsers: []model.SeerrUser{}}
	for _, u := range users {
		byID[u.ID] = u
		switch {
		case u.Shared:
			su := u
			out.SharedUser = &su
		case u.MappedTo == nil:
			out.UnlinkedUsers = append(out.UnlinkedUsers, u)
		}
	}
	settings, err := m.s.cfg.WishSettings(ctx)
	if err != nil {
		return model.WishMappings{}, err
	}
	limits, err := m.s.read.ListAccountWishLimits(ctx)
	if err != nil {
		return model.WishMappings{}, fmt.Errorf("list limits: %w", err)
	}
	limitOf := make(map[int64]wishq.WishAccountLimit, len(limits))
	for _, l := range limits {
		limitOf[l.AccountID] = l
	}
	rows, err := m.s.read.ListAccountMappings(ctx)
	if err != nil {
		return model.WishMappings{}, fmt.Errorf("list mappings: %w", err)
	}
	for _, r := range rows {
		a := model.WishAccountMapping{AccountID: r.AccountID, Email: r.AccountEmail, CanRequest: r.AccountCanRequest == 1, State: model.WishMappingNone}
		var user *model.SeerrUser
		if r.State.Valid {
			a.State = r.State.String
			if r.State.String == StateReady {
				a.State = r.Origin.String
			}
			a.PasswordState = r.PasswordState.String
			a.PasswordSyncedAt = timeOf(r.PasswordSyncedAt)
			a.LinkedBy = r.LinkedBy.String
			a.LinkedAt = timeOf(r.LinkedAt)
			a.ProvisionError = r.ProvisionError.String
			if r.SeerrUserID.Valid {
				id := int(r.SeerrUserID.Int64)
				a.SeerrUserID = &id
				if u, ok := byID[id]; ok {
					user = &u
				}
			}
		}
		if a.State != model.WishMappingNone && a.State != StateWaiting {
			out.Opened++
		}
		l, has := limitOf[r.AccountID]
		a.Limit = accountLimit(l, has, settings.DefaultLimit, user != nil && user.IsAdmin)
		if user != nil && r.State.String == StateReady {
			row, err := m.s.read.GetSeerrUser(ctx, int64(user.ID))
			if err != nil {
				return model.WishMappings{}, fmt.Errorf("seerr user: %w", err)
			}
			a.Quota = wishQuota(row, a.Limit.Effective)
		}
		out.Accounts = append(out.Accounts, a)
	}
	out.Total = len(out.Accounts)
	if out.SharedUser != nil {
		row, err := m.s.read.GetSeerrUser(ctx, int64(out.SharedUser.ID))
		if err != nil {
			return model.WishMappings{}, fmt.Errorf("shared seerr user: %w", err)
		}
		out.SharedQuota = wishQuota(row, settings.DefaultLimit)
	}
	return out, nil
}

// accountLimit resolves the limit of an account: its row or the default (data model 2.13).
// Effective 0 means no limit.
func accountLimit(l wishq.WishAccountLimit, has bool, def model.WishLimits, seerrAdmin bool) model.AccountWishLimit {
	out := model.AccountWishLimit{Mode: model.WishLimitDefault, Effective: def, SeerrAdmin: seerrAdmin}
	if has {
		switch l.Mode {
		case model.WishLimitCustom:
			movie := model.WishLimit{Limit: int(l.MovieLimit.Int64), Days: int(l.MovieDays.Int64)}
			tv := model.WishLimit{Limit: int(l.TvLimit.Int64), Days: int(l.TvDays.Int64)}
			out.Mode, out.Movie, out.TV = model.WishLimitCustom, &movie, &tv
			out.Effective = model.WishLimits{Movie: movie, TV: tv}
		case model.WishLimitUnlimited:
			out.Mode = model.WishLimitUnlimited
			out.Effective = model.WishLimits{Movie: model.WishLimit{Days: def.Movie.Days}, TV: model.WishLimit{Days: def.TV.Days}}
		}
	}
	return out
}

// SetAccountMapping sets the Seerr user of an account by hand («Anderen Benutzer wählen», origin
// manual) or removes it with nil («Verknüpfung lösen»; the user stays in Seerr). The password of a
// user chosen by hand is not synced (F-24, N-103: 'off'; 'admin' for an administrator of Seerr,
// 'external' for a user who signs in through Plex, Jellyfin or Emby). Rights in Seerr are never
// touched (N-9). model.ErrNotFound for an unknown account or Seerr user, model.ErrSeerrUserTaken
// when the user belongs to another account, is the shared user or is gone from Seerr.
func (m *Mapping) SetAccountMapping(ctx context.Context, accountID int64, seerrUserID *int, by int64) (model.WishAccountMapping, error) {
	s := m.s
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.WishAccountMapping{}, err
	}
	err = s.inTx(ctx, func(q *wishq.Queries) error {
		if n, err := q.AccountExists(ctx, accountID); err != nil {
			return fmt.Errorf("account: %w", err)
		} else if n == 0 {
			return model.ErrNotFound
		}
		if seerrUserID == nil {
			_, err := q.DeleteAccountMapping(ctx, accountID)
			return err
		}
		u, err := q.GetSeerrUser(ctx, int64(*seerrUserID))
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("seerr user: %w", err)
		}
		if u.IsShared == 1 || u.GoneAt.Valid {
			return model.ErrSeerrUserTaken
		}
		if other, err := q.GetMappingBySeerrUser(ctx, sql.NullInt64{Int64: u.SeerrID, Valid: true}); err == nil && other.AccountID != accountID {
			return model.ErrSeerrUserTaken
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("mapping of seerr user: %w", err)
		}
		err = q.SetManualMapping(ctx, wishq.SetManualMappingParams{
			AccountID: accountID, SeerrUserID: sql.NullInt64{Int64: u.SeerrID, Valid: true},
			PasswordState: linkedPasswordState(u, PasswordOff), Now: s.now().UnixMilli(), SetBy: sql.NullInt64{Int64: by, Valid: by > 0},
		})
		if err != nil && strings.Contains(err.Error(), "seerr_user_taken") {
			return model.ErrSeerrUserTaken
		}
		if err != nil {
			return err
		}
		// N-112: the limit the user has in Seerr becomes the account's, unless it has its own.
		return s.adoptLimit(ctx, q, accountID, toSeerrUser(u).Quota, u.IsAdmin == 1, ws.DefaultLimit, by)
	})
	if err != nil {
		return model.WishAccountMapping{}, err
	}
	s.log.InfoContext(ctx, "seerr user of account set by hand", "account_id", accountID, "seerr_user_id", seerrUserID)
	if seerrUserID != nil {
		// The job writes the limit to the user (limit_written_at is empty after the choice).
		if err := s.queueProvision(ctx, provisionPayload{AccountID: accountID, LinkedBy: LinkedByAdmin, SetBy: by}, jobs.ManualPriority); err != nil {
			s.log.WarnContext(ctx, "limit of the chosen user not queued", "account_id", accountID, "err", err)
		}
	}
	all, err := m.WishMappings(ctx)
	if err != nil {
		return model.WishAccountMapping{}, err
	}
	for _, a := range all.Accounts {
		if a.AccountID == accountID {
			return a, nil
		}
	}
	return model.WishAccountMapping{}, model.ErrNotFound
}

// linkedPasswordState is the password state of a user Slouchcat did not open: 'admin' for an
// administrator of Seerr (user 1 included), 'external' for a sign-in through Plex, Jellyfin or
// Emby (N-6), else fallback.
func linkedPasswordState(u wishq.SeerrUser, fallback string) string {
	switch {
	case u.IsAdmin == 1 || u.SeerrID == 1:
		return PasswordAdmin
	case u.UserType != int64(seerr.UserLocal):
		return PasswordExternal
	}
	return fallback
}

// LinkedPasswordState is linkedPasswordState for the provisioning (build steps 1.5a and 1.5b): the
// state a user gets when Slouchcat links it, with fallback for a local user without
// administration ('next_login' after an invitation, 'off' when an administrator linked it, N-103).
func (m *Mapping) LinkedPasswordState(ctx context.Context, seerrUserID int, fallback string) (string, error) {
	u, err := m.s.read.GetSeerrUser(ctx, int64(seerrUserID))
	if errors.Is(err, sql.ErrNoRows) {
		return "", model.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("seerr user: %w", err)
	}
	return linkedPasswordState(u, fallback), nil
}

// SharedUser returns the shared user «slouchcat-gaeste», nil while there is none (build step 1.5a
// creates it and records it with MarkShared).
func (m *Mapping) SharedUser(ctx context.Context) (*model.SeerrUser, error) {
	u, err := m.s.read.GetSharedSeerrUser(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("shared seerr user: %w", err)
	}
	out := toSeerrUser(u)
	return &out, nil
}

// MarkShared records the user as the shared user (N-101) in place of an earlier one; the user must
// be in the mirror (model.ErrNotFound) and belong to no account (model.ErrSeerrUserTaken).
func (m *Mapping) MarkShared(ctx context.Context, seerrUserID int) error {
	return m.s.inTx(ctx, func(q *wishq.Queries) error {
		if err := q.ClearSharedSeerrUser(ctx); err != nil {
			return fmt.Errorf("clear shared user: %w", err)
		}
		n, err := q.SetSharedSeerrUser(ctx, int64(seerrUserID))
		switch {
		case err != nil && strings.Contains(err.Error(), "seerr_user_taken"):
			return model.ErrSeerrUserTaken
		case err != nil:
			return fmt.Errorf("set shared user: %w", err)
		case n == 0:
			return model.ErrNotFound
		}
		return nil
	})
}

func toSeerrUser(r wishq.SeerrUser) model.SeerrUser {
	p := seerr.Permission(uint64(max(0, r.Permissions)))
	return model.SeerrUser{
		ID: int(r.SeerrID), DisplayName: r.DisplayName, Email: r.Email.String, UserType: int(r.UserType),
		IsAdmin: r.IsAdmin == 1, Shared: r.IsShared == 1, Gone: r.GoneAt.Valid,
		CanRequestMovies: p.Has(seerr.PermRequest) || p.Has(seerr.PermRequestMovie),
		CanRequestShows:  p.Has(seerr.PermRequest) || p.Has(seerr.PermRequestTV),
		AutoApprove:      p.Has(seerr.PermManageRequests) || p.Has(seerr.PermAutoApprove),
		Quota: model.SeerrUserQuota{
			MovieLimit: intOf(r.MovieQuotaLimit), MovieDays: intOf(r.MovieQuotaDays),
			TVLimit: intOf(r.TvQuotaLimit), TVDays: intOf(r.TvQuotaDays),
		},
	}
}

func intOf(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func timeOf(ms sql.NullInt64) *time.Time {
	if !ms.Valid {
		return nil
	}
	t := time.UnixMilli(ms.Int64).UTC()
	return &t
}
