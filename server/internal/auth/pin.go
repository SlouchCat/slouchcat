package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Profile PINs (K-4, AN-11). A PIN is 4 to 8 digits, stored as an argon2id PHC string with the
// parameters and the process-wide argon2id slots of the passwords.
//
// Wrong PINs are counted per profile and device in pin_attempts, so the lock survives restarts:
// from the third failure on the profile is locked for 30 s, doubling with every further failure up
// to 15 min; a correct PIN resets the counter, and a counter whose last failure is older than 24 h
// starts again at zero. While a lock lasts the PIN is not even checked.
//
// The device is the one the session was created on (sessions.device_id, fixed at login), not the
// cookie of the current request: a client that drops or changes its device cookie keeps the counter
// of its session. A client that logs in again with a fresh cookie gets a fresh device, so on top
// every profile has a ceiling over all devices: more than 20 failures within an hour lock the
// profile for 15 min for everyone. The ceiling is the pin_attempts row of the reserved pseudo device
// profileCeilingDevice (failures = failures in the window, updated_at = start of the window).
const (
	pinLockAfter = 3
	pinLockBase  = 30 * time.Second
	pinLockMax   = 15 * time.Minute
	pinDecay     = 24 * time.Hour

	pinCeilingFailures = 20
	pinCeilingWindow   = time.Hour
	pinCeilingLock     = 15 * time.Minute

	// profileCeilingDevice is the reserved pin_attempts device of the per-profile ceiling.
	profileCeilingDevice = "ffffffffffffffffffffffffffffffff"
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

// newDeviceID is a random device id (128 bits, 32 hex characters).
func newDeviceID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("device id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// sessionDevice returns the device a new session belongs to and records it (user agent, last use):
// the device of the request (model.DeviceID) when it is well-formed, else a fresh random one.
func (s *Service) sessionDevice(ctx context.Context, userAgent string) (string, error) {
	id := model.DeviceID(ctx)
	if !devicePattern.MatchString(id) || id == profileCeilingDevice {
		var err error
		if id, err = newDeviceID(); err != nil {
			return "", err
		}
	}
	err := s.write.UpsertDevice(ctx, appq.UpsertDeviceParams{
		ID: id, UserAgent: sql.NullString{String: truncate(userAgent, maxUserAgentLen), Valid: userAgent != ""}, Now: s.now().UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("upsert device: %w", err)
	}
	return id, nil
}

// assignDevice gives a session without device (created before migration 00007) a fresh one.
func (s *Service) assignDevice(ctx context.Context, sessionID int64) (string, error) {
	id, err := newDeviceID()
	if err != nil {
		return "", err
	}
	if err := s.write.UpsertDevice(ctx, appq.UpsertDeviceParams{ID: id, Now: s.now().UnixMilli()}); err != nil {
		return "", fmt.Errorf("upsert device: %w", err)
	}
	if err := s.write.AssignSessionDevice(ctx, appq.AssignSessionDeviceParams{ID: sessionID, DeviceID: sql.NullString{String: id, Valid: true}}); err != nil {
		return "", fmt.Errorf("assign session device: %w", err)
	}
	return id, nil
}

// pinAttempt reads the counter row; a missing row is the zero value.
func (s *Service) pinAttempt(ctx context.Context, profileID int64, device string) (appq.PinAttempt, error) {
	att, err := s.write.GetPinAttempt(ctx, appq.GetPinAttemptParams{ProfileID: profileID, DeviceID: device})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return appq.PinAttempt{}, fmt.Errorf("get pin attempt: %w", err)
	}
	return att, nil
}

// checkPIN verifies pin against the profile's hash under the lock rules for the session's device.
// It returns nil, model.ErrPinRequired (no PIN sent), model.ErrPinInvalid (wrong, no lock yet) or a
// *model.PinLockedError (locked now or by this attempt). Attempts of one service are serialised,
// so concurrent guesses cannot slip past the counters.
func (s *Service) checkPIN(ctx context.Context, device string, profile appq.Profile, pin string) error {
	s.pinMu.Lock()
	defer s.pinMu.Unlock()
	now := s.now()
	ceiling, err := s.pinAttempt(ctx, profile.ID, profileCeilingDevice)
	if err != nil {
		return err
	}
	att, err := s.pinAttempt(ctx, profile.ID, device)
	if err != nil {
		return err
	}
	for _, until := range []int64{ceiling.LockedUntil, att.LockedUntil} {
		if t := time.UnixMilli(until); t.After(now) {
			return &model.PinLockedError{RetryAfter: t.Sub(now)}
		}
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

	// Wrong PIN: the device counter (restarting after 24 h without failure) and the ceiling.
	failures := att.Failures + 1
	if att.Failures > 0 && now.Sub(time.UnixMilli(att.UpdatedAt)) > pinDecay {
		failures = 1
	}
	lock := lockDuration(failures)
	var lockedUntil int64
	if lock > 0 {
		lockedUntil = now.Add(lock).UnixMilli()
	}
	if err := s.touchDeviceRow(ctx, device); err != nil {
		return err
	}
	err = s.write.UpsertPinAttempt(ctx, appq.UpsertPinAttemptParams{
		ProfileID: profile.ID, DeviceID: device, Failures: failures, LockedUntil: lockedUntil, Now: now.UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("store pin attempt: %w", err)
	}

	windowStart, inWindow := ceiling.UpdatedAt, ceiling.Failures
	if ceiling.Failures == 0 || now.Sub(time.UnixMilli(ceiling.UpdatedAt)) > pinCeilingWindow {
		windowStart, inWindow = now.UnixMilli(), 0
	}
	inWindow++
	var ceilingLock int64
	if inWindow > pinCeilingFailures {
		ceilingLock = now.Add(pinCeilingLock).UnixMilli()
		lock = max(lock, pinCeilingLock)
	}
	if err := s.touchDeviceRow(ctx, profileCeilingDevice); err != nil {
		return err
	}
	err = s.write.UpsertPinAttempt(ctx, appq.UpsertPinAttemptParams{
		ProfileID: profile.ID, DeviceID: profileCeilingDevice, Failures: inWindow, LockedUntil: ceilingLock, Now: windowStart,
	})
	if err != nil {
		return fmt.Errorf("store pin ceiling: %w", err)
	}
	s.log.WarnContext(ctx, "wrong profile pin", "profile_id", profile.ID, "failures", failures,
		"profile_failures_in_window", inWindow, "lock_seconds", int(lock.Seconds()))
	if lock > 0 {
		return &model.PinLockedError{RetryAfter: lock}
	}
	return model.ErrPinInvalid
}

// touchDeviceRow makes sure the device row exists (pin_attempts references it).
func (s *Service) touchDeviceRow(ctx context.Context, id string) error {
	if err := s.write.UpsertDevice(ctx, appq.UpsertDeviceParams{ID: id, Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("upsert device: %w", err)
	}
	return nil
}

// SelectProfile makes the profile the active profile of the session (K-4) and returns the updated
// session. A profile of another account (or one deleted in the meantime) is model.ErrNotFound; a
// profile with a PIN needs it (see checkPIN for the errors). The switch happens in place; the
// caller ends the playback sessions of the browser session afterwards (api: Playback.EndSession).
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
		device := session.DeviceID
		if device == "" {
			if device, err = s.assignDevice(ctx, session.ID); err != nil {
				return model.Session{}, err
			}
		}
		if err := s.checkPIN(ctx, device, row, pin); err != nil {
			return model.Session{}, err
		}
	}
	n, err := s.write.SetSessionProfile(ctx, appq.SetSessionProfileParams{
		ID: session.ID, ProfileID: sql.NullInt64{Int64: row.ID, Valid: true}, Now: s.now().UnixMilli(),
	})
	if err != nil {
		return model.Session{}, fmt.Errorf("set session profile: %w", err)
	}
	if n == 0 {
		return model.Session{}, model.ErrNotFound // session revoked or profile deleted meanwhile
	}
	prof, err := s.profileModel(ctx, s.write, row)
	if err != nil {
		return model.Session{}, err
	}
	session.Profile = prof
	return session, nil
}
