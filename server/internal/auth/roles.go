package auth

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// SetAccountRole appoints an account as administrator or takes the role away (ADR 0014).
//
// Who may call it is not decided here: setAccountRole carries x-visibility owner, so the
// middleware turns away everybody but the owner with 403 owner_required before a handler ever
// sees the target account. What this service decides is the state, in this order:
//
//	model.ErrNotFound        unknown account
//	model.ErrOwnerProtected  the target is the owner. Checked before everything else, so the
//	                         message stays the telling one even when the owner happens to be the
//	                         last administrator as well
//	model.ErrLastAdmin       the demotion would leave the server without an administrator
//
// Both checks read inside the write transaction that performs the change, the way CreateOwner
// checks its uniqueness, so two concurrent demotions cannot both find another administrator.
//
// Taking the role away revokes every session of the account: without that, an open administration
// page would keep its controls while every request behind them answered 403. Appointing revokes
// nothing - the middleware decides per request, and nobody loses anything. Streams are not ended
// either way, because neither direction narrows what the account may see (ADR 0014; the
// appointment only widens it to 18 and all libraries).
func (s *Service) SetAccountRole(ctx context.Context, id int64, role string) error {
	if role != model.RoleAdmin && role != model.RoleUser {
		var verr *model.ValidationError
		return verr.Add("role", "invalid_value")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)

	acc, err := q.GetAccount(ctx, id)
	if err != nil {
		return notFoundOr(err, "get account")
	}
	if acc.IsOwner == 1 {
		return model.ErrOwnerProtected
	}
	demotion := role == model.RoleUser && acc.Role == model.RoleAdmin
	if demotion {
		others, err := q.CountOtherAdmins(ctx, acc.ID)
		if err != nil {
			return fmt.Errorf("count other admins: %w", err)
		}
		if others == 0 {
			return model.ErrLastAdmin
		}
	}
	now := s.now().UnixMilli()
	n, err := q.SetAccountRole(ctx, appq.SetAccountRoleParams{ID: acc.ID, Role: role, Now: now})
	if err != nil {
		return fmt.Errorf("set account role: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	if demotion {
		if _, err := q.RevokeAccountSessions(ctx, appq.RevokeAccountSessionsParams{AccountID: acc.ID, Now: nullInt(now)}); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "account role changed", "account_id", acc.ID, "role", role, "sessions_revoked", demotion)
	return nil
}
