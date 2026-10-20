package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Offline PIN material (AN-19, M5 design 3.6). A device that holds downloads checks PINs without
// the server, so the sync hands it verifier material, never the PIN or its argon2id hash:
//
//   - pin_salt: 16 random bytes as hex, new with every normal PIN (SetProfilePIN) and cleared
//     with it. The client derives its own PBKDF2 verifier of the normal PIN from it after the
//     next successful online PIN entry; a new salt (PIN changed) invalidates the old verifier.
//   - offline_pin: the PBKDF2-SHA256 verifier of a separate, longer offline PIN (6 to 12 digits)
//     that guards the profile offline instead of the normal PIN.
//
// Protection goal: a curious child on a shared device, not a manipulated browser (plan 3.3).
// Neither PINs nor verifiers are ever logged.

const (
	pinSaltBytes     = 16 // salt of the normal PIN (32 hex characters, migration 00013)
	offlineSaltBytes = 16
	offlineKeyBytes  = 32
)

var offlinePinPattern = regexp.MustCompile(`^[0-9]{6,12}$`)

// ValidOfflinePIN reports whether pin is 6 to 12 digits.
func ValidOfflinePIN(pin string) bool { return offlinePinPattern.MatchString(pin) }

// newPinSalt returns a fresh salt of the normal PIN: 16 random bytes as 32 lower-case hex
// characters.
func newPinSalt() (sql.NullString, error) {
	var b [pinSaltBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return sql.NullString{}, fmt.Errorf("pin salt: %w", err)
	}
	return sql.NullString{String: hex.EncodeToString(b[:]), Valid: true}, nil
}

// storedVerifier is the JSON form of a model.PinVerifier in profiles.offline_pin.
type storedVerifier struct {
	Salt       string `json:"salt"`
	Iterations int    `json:"iterations"`
	Hash       string `json:"hash"`
}

// offlinePinVerifier derives the PBKDF2-SHA256 verifier of pin with salt (model.OfflinePinIterations
// iterations, 32-byte key); salt and hash are base64url without padding. The client computes the
// same to check a PIN offline.
func offlinePinVerifier(pin string, salt []byte) (model.PinVerifier, error) {
	key, err := pbkdf2.Key(sha256.New, pin, salt, model.OfflinePinIterations, offlineKeyBytes)
	if err != nil {
		return model.PinVerifier{}, fmt.Errorf("offline pin verifier: %w", err)
	}
	return model.PinVerifier{
		Salt:       base64.RawURLEncoding.EncodeToString(salt),
		Iterations: model.OfflinePinIterations,
		Hash:       base64.RawURLEncoding.EncodeToString(key),
	}, nil
}

// newOfflineVerifier derives the verifier of a validated offline PIN with a fresh salt. The run
// takes one of the process-wide hashing slots of argon2id, so concurrent requests cannot tie up
// every CPU.
func newOfflineVerifier(ctx context.Context, pin string) (string, error) {
	salt := make([]byte, offlineSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("offline pin salt: %w", err)
	}
	release, err := acquireArgon(ctx)
	if err != nil {
		return "", err
	}
	v, err := offlinePinVerifier(pin, salt)
	release()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(storedVerifier(v))
	if err != nil {
		return "", fmt.Errorf("encode offline pin verifier: %w", err)
	}
	return string(data), nil
}

// SetOfflinePIN sets or replaces the separate offline PIN of a profile of the account (AN-19).
// Only the primary profile may (model.ErrPrimaryProfileRequired); a profile of another account is
// model.ErrNotFound; a PIN that is not 6 to 12 digits is a *model.ValidationError (pin
// invalid_pin).
func (s *Service) SetOfflinePIN(ctx context.Context, session model.Session, profileID int64, pin string) error {
	if err := requirePrimary(session); err != nil {
		return err
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: profileID, AccountID: session.Account.ID})
	if err != nil {
		return notFoundOr(err, "get profile")
	}
	if !ValidOfflinePIN(pin) {
		return (&model.ValidationError{}).Add("pin", "invalid_pin")
	}
	verifier, err := newOfflineVerifier(ctx, pin)
	if err != nil {
		return err
	}
	if err := s.storeOfflinePIN(ctx, row.ID, sql.NullString{String: verifier, Valid: true}); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "offline pin set", "account_id", session.Account.ID, "profile_id", row.ID)
	return nil
}

// RemoveOfflinePIN removes the offline PIN of a profile of the account (errors as SetOfflinePIN);
// a profile without one is fine. Offline the normal PIN guards the profile again.
func (s *Service) RemoveOfflinePIN(ctx context.Context, session model.Session, profileID int64) error {
	if err := requirePrimary(session); err != nil {
		return err
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: profileID, AccountID: session.Account.ID})
	if err != nil {
		return notFoundOr(err, "get profile")
	}
	return s.storeOfflinePIN(ctx, row.ID, sql.NullString{})
}

func (s *Service) storeOfflinePIN(ctx context.Context, profileID int64, verifier sql.NullString) error {
	err := s.write.SetProfileOfflinePin(ctx, appq.SetProfileOfflinePinParams{ID: profileID, OfflinePin: verifier, Now: s.now().UnixMilli()})
	if err != nil {
		return fmt.Errorf("set offline pin: %w", err)
	}
	return nil
}

// OfflinePins returns the offline PIN material of the given profiles of the account for the sync
// of a device (M5 design 3.4): whether a profile has a normal PIN, its salt (empty without PIN)
// and the verifier of its offline PIN (nil without one). Profiles of another account, and unknown
// ids, are silently absent.
func (s *Service) OfflinePins(ctx context.Context, accountID int64, profileIDs []int64) (map[int64]model.OfflinePin, error) {
	out := make(map[int64]model.OfflinePin, len(profileIDs))
	if len(profileIDs) == 0 {
		return out, nil
	}
	want := make(map[int64]bool, len(profileIDs))
	for _, id := range profileIDs {
		want[id] = true
	}
	rows, err := s.read.ListProfilesByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	for _, r := range rows {
		if !want[r.ID] {
			continue
		}
		pin := model.OfflinePin{HasPin: r.PinHash.Valid}
		if pin.HasPin {
			pin.PinSalt = r.PinSalt.String
		}
		if r.OfflinePin.Valid {
			var v storedVerifier
			if err := json.Unmarshal([]byte(r.OfflinePin.String), &v); err != nil || v.Salt == "" || v.Hash == "" || v.Iterations <= 0 {
				return nil, fmt.Errorf("profile %d: unreadable offline pin verifier", r.ID)
			}
			verifier := model.PinVerifier(v)
			pin.OfflinePin = &verifier
		}
		out[r.ID] = pin
	}
	return out, nil
}
