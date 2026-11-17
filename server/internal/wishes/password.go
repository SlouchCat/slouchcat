package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The password in the three moments (concept 25, architecture 18.3, build step 1.5b): Slouchcat
// sees it in clear text only when an invitation is accepted, when the account changes its password
// or email and at every sign-in. PasswordSeen takes it from package auth after the commit or the
// check and hands it to Seerr in a goroutine with a time limit of 10 s; the answer to the browser
// does not wait and does not change (N-94). The password lives only in memory: never in a job
// payload, a table or a log line.

// Moments of PasswordSeen (package auth, PasswordMoment*).
const (
	MomentInvitation     = "invitation"
	MomentLogin          = "login"
	MomentPasswordChange = "password_change"
	MomentEmailChange    = "email_change"
)

// handOverTimeout bounds the hand-over of one moment.
const handOverTimeout = 10 * time.Second

// ProvisionEmailTaken is the provision_error of a ready user whose new email Seerr refused because
// another user has it (N-95): the user stayed as it was, the administration sees a hint.
const ProvisionEmailTaken = "email_taken"

// PasswordSeen is the hook of package auth (WithPasswordSeen). It returns at once; the hand-over
// runs in the background, bound to 10 s, and every failure is only logged with the account id.
func (s *Service) PasswordSeen(ctx context.Context, accountID int64, email, password, moment string) {
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), handOverTimeout)
		defer cancel()
		if err := s.handOver(ctx, accountID, email, password, moment); err != nil {
			s.log.WarnContext(ctx, "password not handed to seerr", "account_id", accountID, "moment", moment, "reason", errorClass(err))
		}
	}()
}

// accountLocks holds one hand-over per account at a time (security review L-3): without it a
// login that read next_login could finish after a password change and write the old password.
// The provisioning of an account (provisionJob, ProvisionAccount, syncAccount) holds the same
// lock (review MJ-1): «Alle eröffnen» and an accepted invitation never open two users.
type accountLocks struct {
	mu   sync.Mutex
	held map[int64]chan struct{}
	// waiting, when set, is called once a lock has to wait for another holder (tests).
	waiting func(id int64)
}

// lock waits for the account's hand-over in flight, at most until ctx ends.
func (l *accountLocks) lock(ctx context.Context, id int64) (func(), error) {
	l.mu.Lock()
	if l.held == nil {
		l.held = map[int64]chan struct{}{}
	}
	ch, ok := l.held[id]
	if !ok {
		ch = make(chan struct{}, 1)
		l.held[id] = ch
	}
	waiting := l.waiting
	l.mu.Unlock()
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	default:
	}
	if waiting != nil {
		waiting(id)
	}
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// handOver does what the moment means for Seerr (architecture 18.3), under the lock of the
// account; the mapping is read under the lock, so a login does nothing once a password change
// synced the password.
func (s *Service) handOver(ctx context.Context, accountID int64, email, password, moment string) error {
	unlock, err := s.accountLocks.lock(ctx, accountID)
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil || !cfg.Configured() || !cfg.Enabled {
		return err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return err
	}
	m, err := s.read.GetAccountMapping(ctx, accountID)
	hasRow := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mapping: %w", err)
	}
	ready := hasRow && m.State == StateReady && m.SeerrUserID.Valid
	switch moment {
	case MomentInvitation:
		if !ws.ProvisionAccounts || ready {
			return nil
		}
		return s.provisionInMemory(ctx, accountID, LinkedByInvite, password)
	case MomentEmailChange:
		if !ready {
			if !ws.ProvisionAccounts {
				return nil
			}
			return s.provisionInMemory(ctx, accountID, LinkedByEmailChange, password)
		}
		admin, err := s.provisionSession(ctx)
		if err != nil {
			return err
		}
		if err := s.changeEmail(ctx, admin, accountID, int(m.SeerrUserID.Int64), email); err != nil {
			return err
		}
		if m, err = s.read.GetAccountMapping(ctx, accountID); err != nil {
			return fmt.Errorf("mapping: %w", err)
		}
		if syncable(m.PasswordState) && ws.SyncPassword {
			s.setPassword(ctx, admin, accountID, int(m.SeerrUserID.Int64), password)
		}
		return nil
	case MomentPasswordChange:
		if !ready || !syncable(m.PasswordState) || !ws.SyncPassword {
			return nil
		}
	case MomentLogin:
		if !ready || m.PasswordState != PasswordNextLogin || !ws.SyncPassword {
			return nil
		}
	default:
		return fmt.Errorf("unknown password moment %q", moment)
	}
	admin, err := s.provisionSession(ctx)
	if err != nil {
		return err
	}
	if m.Origin.String == OriginCreated {
		// A user Slouchcat opened for an account without the request right gets the password only
		// once its rights are 0 (review m-1); else it stays next_login.
		acc, err := s.read.GetProvisionAccount(ctx, accountID)
		if err != nil {
			return fmt.Errorf("account: %w", err)
		}
		if acc.CanRequest != 1 {
			if err := s.syncRights(ctx, admin, acc, m); err != nil {
				return err
			}
		}
	}
	s.setPassword(ctx, admin, accountID, int(m.SeerrUserID.Int64), password)
	return nil
}

// syncable: the password of a user in this state follows the account's (synced, next_login); off,
// admin and external never get it (F-24, N-103, N-6).
func syncable(state string) bool { return state == PasswordSynced || state == PasswordNextLogin }

// provisionInMemory opens or links with the password in memory. When Seerr is not reached the
// account waits and a job without the password tries again; the password follows at the next
// sign-in (next_login).
func (s *Service) provisionInMemory(ctx context.Context, accountID int64, linkedBy, password string) error {
	state, err := s.provision(ctx, accountID, linkedBy, password, 0)
	if state != "" && err != nil { // waiting, or ready with its rights or limit still to write
		if qerr := s.queueProvision(context.WithoutCancel(ctx), provisionPayload{AccountID: accountID, Intent: intentRetry, LinkedBy: linkedBy}, provisionPriority); qerr != nil {
			return errors.Join(err, qerr)
		}
		return nil
	}
	return err
}

// changeEmail writes the new email into settings/main of the user (N-5). Seerr refuses an email
// of another user (INVALID_EMAIL): the user stays as it is and the account carries the hint
// email_taken for the administration (N-95). The user is read fresh first: user 1, a managing
// right or a sign-in through Plex, Jellyfin or Emby keeps its email, and the password state
// becomes admin or external (security review H-1, L-2; Seerr's «forgot password» would hand the
// user over otherwise).
func (s *Service) changeEmail(ctx context.Context, admin seerr.Session, accountID int64, seerrUserID int, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := admin.User(ctx, seerrUserID)
	if err != nil {
		return err
	}
	if st := guardedState(int64(u.ID), u.Permissions, u.UserType); st != "" {
		if _, err := s.write.SetMappingPasswordState(ctx, wishq.SetMappingPasswordStateParams{PasswordState: st, AccountID: accountID}); err != nil {
			return fmt.Errorf("password state: %w", err)
		}
		s.log.InfoContext(ctx, "email of a seerr administrator or external user left alone", "account_id", accountID, "seerr_user_id", seerrUserID, "password_state", st)
		return nil
	}
	set, err := admin.UserSettings(ctx, seerrUserID)
	if err != nil {
		return err
	}
	set.Email = &email
	err = admin.SetUserSettings(ctx, seerrUserID, set)
	hint := sql.NullString{}
	switch {
	case errors.Is(err, seerr.ErrInvalidEmail):
		hint = nullString(ProvisionEmailTaken)
		s.log.WarnContext(ctx, "seerr refused the new email of the account", "account_id", accountID, "seerr_user_id", seerrUserID)
	case err != nil:
		return err
	default:
		if err := s.write.SetSeerrUserEmail(ctx, wishq.SetSeerrUserEmailParams{Email: nullString(email), SeerrID: int64(seerrUserID)}); err != nil {
			return fmt.Errorf("store email: %w", err)
		}
	}
	if err := s.write.SetMappingProvisionError(ctx, wishq.SetMappingProvisionErrorParams{ProvisionError: hint, AccountID: accountID}); err != nil {
		return fmt.Errorf("store hint: %w", err)
	}
	return nil
}

// Modes of SetPasswordMode (setSeerrPasswordMode).
const (
	PasswordModeNextLogin = PasswordNextLogin // «Passwort bei der nächsten Anmeldung neu übernehmen»
	PasswordModeOff       = PasswordOff       // «Passwort nicht abgleichen»
)

// SetPasswordMode switches the password sync of one account (setSeerrPasswordMode, N-7, N-103):
// next_login hands the password over at the next sign-in, off never. model.ErrNotFound without a
// ready Seerr user, model.ErrPasswordNotSyncable for an administrator of Seerr or a sign-in through
// Plex, Jellyfin or Emby, a validation error for another mode.
func (s *Service) SetPasswordMode(ctx context.Context, accountID int64, mode string) (model.WishAccountMapping, error) {
	if mode != PasswordModeNextLogin && mode != PasswordModeOff {
		return model.WishAccountMapping{}, (&model.ValidationError{}).Add("mode", "invalid_value")
	}
	m, err := s.read.GetAccountMapping(ctx, accountID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && m.State != StateReady) {
		return model.WishAccountMapping{}, model.ErrNotFound
	}
	if err != nil {
		return model.WishAccountMapping{}, fmt.Errorf("mapping: %w", err)
	}
	if m.PasswordState == PasswordAdmin || m.PasswordState == PasswordExternal {
		return model.WishAccountMapping{}, model.ErrPasswordNotSyncable
	}
	if _, err := s.write.SetMappingPasswordState(ctx, wishq.SetMappingPasswordStateParams{PasswordState: mode, AccountID: accountID}); err != nil {
		return model.WishAccountMapping{}, fmt.Errorf("password state: %w", err)
	}
	s.log.InfoContext(ctx, "seerr password mode set", "account_id", accountID, "mode", mode)
	return s.accountMapping(ctx, accountID)
}

// waitBackground waits for the hand-overs in flight (tests, shutdown).
func (s *Service) waitBackground() { s.bg.Wait() }
