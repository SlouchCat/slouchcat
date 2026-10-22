package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// UpdateAccount changes the e-mail or the password of the session's account after checking the
// current password (M8, A-2) and returns the session with the changed account. Every call counts
// against the attempt limit of the client (model.ClientIP of ctx), like a login. A new password
// revokes every other session of the account; the calling session stays. The e-mail is compared
// normalised; another account with it is model.ErrEmailTaken. Passwords never reach the log.
func (s *Service) UpdateAccount(ctx context.Context, session model.Session, upd model.AccountUpdate) (model.Session, error) {
	var verr *model.ValidationError
	if upd.Email == nil && upd.NewPassword == nil {
		verr = verr.Add("email", "required")
	}
	email := ""
	if upd.Email != nil {
		email = NormalizeEmail(*upd.Email)
		if !validEmail(email) {
			verr = verr.Add("email", "invalid_email")
		}
	}
	switch {
	case upd.CurrentPassword == "":
		verr = verr.Add("currentPassword", "required")
	case utf8.RuneCountInString(upd.CurrentPassword) > maxPasswordLen:
		verr = verr.Add("currentPassword", "too_long")
	}
	if upd.NewPassword != nil {
		switch n := utf8.RuneCountInString(*upd.NewPassword); {
		case n < minPasswordLen:
			verr = verr.Add("newPassword", "too_short")
		case n > maxPasswordLen:
			verr = verr.Add("newPassword", "too_long")
		}
	}
	if verr != nil {
		return model.Session{}, verr
	}
	if err := s.AllowAttempt(ctx); err != nil {
		return model.Session{}, err
	}
	acc, err := s.read.GetAccount(ctx, session.Account.ID)
	if err != nil {
		return model.Session{}, notFoundOr(err, "get account")
	}
	ok, err := s.verify(ctx, upd.CurrentPassword, acc.PasswordHash)
	if errors.Is(err, errBadHash) {
		// No password matches an unreadable hash: the answer is the one for a wrong password.
		s.log.WarnContext(ctx, "stored password hash unreadable", "account_id", acc.ID)
		ok, err = false, nil
	}
	if err != nil {
		return model.Session{}, fmt.Errorf("account %d: %w", acc.ID, err)
	}
	if !ok {
		return model.Session{}, model.ErrInvalidCredentials
	}
	hash := ""
	if upd.NewPassword != nil {
		release, err := acquireArgon(ctx)
		if err != nil {
			return model.Session{}, err
		}
		hash, err = hashPassword(*upd.NewPassword, s.params)
		release()
		if err != nil {
			return model.Session{}, err
		}
	}

	now := s.now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Session{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	emailChanged := upd.Email != nil && email != acc.Email
	if emailChanged {
		other, err := q.GetAccountByEmail(ctx, email)
		switch {
		case err == nil && other.ID != acc.ID:
			return model.Session{}, model.ErrEmailTaken
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return model.Session{}, fmt.Errorf("get account: %w", err)
		}
		if _, err := q.UpdateAccountEmail(ctx, appq.UpdateAccountEmailParams{Email: email, Now: now, ID: acc.ID}); err != nil {
			return model.Session{}, fmt.Errorf("update e-mail: %w", err)
		}
		session.Account.Email = email
	}
	revoked := int64(0)
	if hash != "" {
		if _, err := q.UpdateAccountPassword(ctx, appq.UpdateAccountPasswordParams{PasswordHash: hash, Now: now, ID: acc.ID}); err != nil {
			return model.Session{}, fmt.Errorf("update password: %w", err)
		}
		if revoked, err = q.RevokeOtherAccountSessions(ctx, appq.RevokeOtherAccountSessionsParams{
			Now: nullInt(now), AccountID: acc.ID, KeepID: session.ID,
		}); err != nil {
			return model.Session{}, fmt.Errorf("revoke other sessions: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Session{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "account updated", "account_id", acc.ID, "email_changed", emailChanged,
		"password_changed", hash != "", "sessions_revoked", revoked)
	return session, nil
}
