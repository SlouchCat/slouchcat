package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Profile PINs (K-4, AN-11). A PIN is 4 to 8 digits, stored as an argon2id PHC string with the
// parameters and the process-wide argon2id slots of the passwords. Wrong PINs are counted per
// profile and device (cookie slouchcat_device) in pin_attempts, so the lock survives restarts: from
// the third failure on the profile is locked for 30 s, doubling with every further failure up to
// 15 min; a correct PIN resets the counter. While the lock lasts the PIN is not even checked.
const (
	pinLockAfter = 3
	pinLockBase  = 30 * time.Second
	pinLockMax   = 15 * time.Minute

	// anonymousDevice stands in for requests without a device cookie (service callers in tests).
	anonymousDevice = "00000000000000000000000000000000"
)

var (
	pinPattern    = regexp.MustCompile(`^[0-9]{4,8}$`)
	devicePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// ValidPIN reports whether pin is 4 to 8 digits.
func ValidPIN(pin string) bool { return pinPattern.MatchString(pin) }

// lockDuration is the lock that follows the n-th consecutive failure (0 before the third).
func lockDuration(failures int64) time.Duration {
	if failures < pinLockAfter {
		return 0
	}
	exp := min(failures-pinLockAfter, 10)
	return min(pinLockBase<<exp, pinLockMax)
}

// hashPIN hashes a validated PIN in an argon2id slot.
func (s *Service) hashPIN(ctx context.Context, pin string) (string, error) {
	release, err := acquireArgon(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	return hashPassword(pin, s.params)
}

// deviceID is the device of the request, or anonymousDevice.
func deviceID(ctx context.Context) string {
	if id := model.DeviceID(ctx); devicePattern.MatchString(id) {
		return id
	}
	return anonymousDevice
}

// touchDevice records the device of the request (user agent, last use) and returns its id, or ""
// when the request carries none.
func (s *Service) touchDevice(ctx context.Context, userAgent string) (string, error) {
	id := model.DeviceID(ctx)
	if !devicePattern.MatchString(id) {
		return "", nil
	}
	err := s.write.UpsertDevice(ctx, appq.UpsertDeviceParams{
		ID: id, UserAgent: sql.NullString{String: truncate(userAgent, maxUserAgentLen), Valid: userAgent != ""}, Now: s.now().UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("upsert device: %w", err)
	}
	return id, nil
}

// checkPIN verifies pin against the profile's hash under the lock rules. It returns nil,
// model.ErrPinRequired (no PIN sent), model.ErrPinInvalid (wrong, no lock yet) or a
// *model.PinLockedError (locked now or by this attempt). Attempts of one service are serialised, so
// concurrent guesses cannot slip past the counter.
func (s *Service) checkPIN(ctx context.Context, profile appq.Profile, pin string) error {
	s.pinMu.Lock()
	defer s.pinMu.Unlock()
	device := deviceID(ctx)
	now := s.now()
	att, err := s.write.GetPinAttempt(ctx, appq.GetPinAttemptParams{ProfileID: profile.ID, DeviceID: device})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get pin attempt: %w", err)
	}
	if until := time.UnixMilli(att.LockedUntil); until.After(now) {
		return &model.PinLockedError{RetryAfter: until.Sub(now)}
	}
	if pin == "" {
		return model.ErrPinRequired
	}
	release, err := acquireArgon(ctx)
	if err != nil {
		return err
	}
	ok, err := verifyPassword(pin, profile.PinHash.String)
	release()
	if err != nil {
		return fmt.Errorf("profile %d: %w", profile.ID, err)
	}
	if ok {
		if err := s.write.DeletePinAttempt(ctx, appq.DeletePinAttemptParams{ProfileID: profile.ID, DeviceID: device}); err != nil {
			return fmt.Errorf("reset pin attempts: %w", err)
		}
		return nil
	}
	failures := att.Failures + 1
	lock := lockDuration(failures)
	if _, err := s.touchDeviceID(ctx, device); err != nil {
		return err
	}
	var lockedUntil int64
	if lock > 0 {
		lockedUntil = now.Add(lock).UnixMilli()
	}
	err = s.write.UpsertPinAttempt(ctx, appq.UpsertPinAttemptParams{
		ProfileID: profile.ID, DeviceID: device, Failures: failures, LockedUntil: lockedUntil, Now: now.UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("store pin attempt: %w", err)
	}
	s.log.WarnContext(ctx, "wrong profile pin", "profile_id", profile.ID, "failures", failures, "lock_seconds", int(lock.Seconds()))
	if lock > 0 {
		return &model.PinLockedError{RetryAfter: lock}
	}
	return model.ErrPinInvalid
}

// touchDeviceID makes sure the device row exists (pin_attempts references it).
func (s *Service) touchDeviceID(ctx context.Context, id string) (string, error) {
	err := s.write.UpsertDevice(ctx, appq.UpsertDeviceParams{ID: id, Now: s.now().UnixMilli()})
	if err != nil {
		return "", fmt.Errorf("upsert device: %w", err)
	}
	return id, nil
}

// SelectProfile makes the profile the active profile of the session (K-4) and returns the updated
// session. A profile of another account is model.ErrNotFound; a profile with a PIN needs it (see
// checkPIN for the errors). The switch happens in place; the caller ends the playback sessions of
// the browser session afterwards (api: Playback.EndSession).
func (s *Service) SelectProfile(ctx context.Context, session model.Session, profileID int64, pin string) (model.Session, error) {
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: profileID, AccountID: session.Account.ID})
	if err != nil {
		return model.Session{}, notFoundOr(err, "get profile")
	}
	if row.PinHash.Valid {
		if pin != "" && !ValidPIN(pin) {
			return model.Session{}, (&model.ValidationError{}).Add("pin", "invalid_pin")
		}
		if err := s.AllowAttempt(ctx); err != nil {
			return model.Session{}, err
		}
		if err := s.checkPIN(ctx, row, pin); err != nil {
			return model.Session{}, err
		}
	}
	device, err := s.touchDevice(ctx, "")
	if err != nil {
		return model.Session{}, err
	}
	n, err := s.write.SetSessionProfile(ctx, appq.SetSessionProfileParams{
		ID: session.ID, ProfileID: sql.NullInt64{Int64: row.ID, Valid: true},
		DeviceID: sql.NullString{String: device, Valid: device != ""}, Now: s.now().UnixMilli(),
	})
	if err != nil {
		return model.Session{}, fmt.Errorf("set session profile: %w", err)
	}
	if n == 0 {
		return model.Session{}, model.ErrNotFound
	}
	prof, err := s.profileModel(ctx, s.write, row)
	if err != nil {
		return model.Session{}, err
	}
	session.Profile = prof
	return session, nil
}
