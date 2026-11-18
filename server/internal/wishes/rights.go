package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Rights is the request right of a profile (concept 25, users and profiles 6.1 and 9): «Wünsche
// aktiv» and Seerr in use, the switch of the account (the cap) and of the profile, and a Seerr
// user to send as: the user of the account, else the shared user when «Konten ohne
// Seerr-Benutzer» says so (F-4, N-101). The server decides; the client only shows the answer
// (rule 12). Temporary reasons (Seerr unreachable, key refused, another instance waiting) keep the
// right: the wish waits in the outbox.
type Rights struct{ s *Service }

// Sender is the Seerr user a profile sends as.
type Sender struct {
	SeerrUserID int
	Shared      bool // the shared user «slouchcat-gaeste»
	Movies      bool // the user may request films in Seerr
	Shows       bool // and series
}

// ForProfile returns the right of the profile of the account (getSession, build step 1.6).
// model.ErrNotFound when the profile does not belong to the account.
func (r *Rights) ForProfile(ctx context.Context, accountID, profileID int64) (model.WishRights, error) {
	rights, _, err := r.resolve(ctx, accountID, profileID)
	return rights, err
}

// SenderFor returns the right and the Seerr user the profile sends as; Sender is zero without the
// right (rights.Reason says why).
func (r *Rights) SenderFor(ctx context.Context, accountID, profileID int64) (model.WishRights, Sender, error) {
	return r.resolve(ctx, accountID, profileID)
}

func (r *Rights) resolve(ctx context.Context, accountID, profileID int64) (model.WishRights, Sender, error) {
	s := r.s
	row, err := s.read.GetProfileRequestRight(ctx, wishq.GetProfileRequestRightParams{ProfileID: profileID, AccountID: accountID})
	if errors.Is(err, sql.ErrNoRows) {
		return model.WishRights{}, Sender{}, model.ErrNotFound
	}
	if err != nil {
		return model.WishRights{}, Sender{}, fmt.Errorf("request right: %w", err)
	}
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return model.WishRights{}, Sender{}, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.WishRights{}, Sender{}, err
	}
	deny := func(reason string) (model.WishRights, Sender, error) {
		return model.WishRights{Enabled: reason != model.WishReasonDisabled, Reason: reason}, Sender{}, nil
	}
	switch {
	case !ws.Enabled || !cfg.Configured() || !cfg.Enabled:
		return deny(model.WishReasonDisabled)
	case row.AccountCanRequest != 1:
		return deny(model.WishReasonAccountNotAllowed)
	case row.ProfileCanRequest != 1:
		return deny(model.WishReasonProfileNotAllowed)
	}
	var user wishq.SeerrUser
	shared := false
	if row.MappingState.Valid && row.MappingState.String == StateReady && row.SeerrUserID.Valid {
		if user, err = s.read.GetSeerrUser(ctx, row.SeerrUserID.Int64); err != nil {
			return model.WishRights{}, Sender{}, fmt.Errorf("seerr user: %w", err)
		}
	} else {
		if ws.Unmapped != model.WishesUnmappedFallback {
			return deny(model.WishReasonNoMapping)
		}
		user, err = s.read.GetSharedSeerrUser(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return deny(model.WishReasonNoMapping)
		}
		if err != nil {
			return model.WishRights{}, Sender{}, fmt.Errorf("shared seerr user: %w", err)
		}
		shared = true
	}
	if user.GoneAt.Valid {
		return deny(model.WishReasonSeerrUserGone)
	}
	p := seerr.Permission(uint64(max(0, user.Permissions)))
	sender := Sender{
		SeerrUserID: int(user.SeerrID), Shared: shared,
		Movies: p.Has(seerr.PermRequest) || p.Has(seerr.PermRequestMovie),
		Shows:  p.Has(seerr.PermRequest) || p.Has(seerr.PermRequestTV),
	}
	return model.WishRights{
		Enabled: true, CanRequest: true,
		Movies: ws.AllowMovies && sender.Movies, Shows: ws.AllowShows && sender.Shows,
	}, sender, nil
}
