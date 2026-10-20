package auth

import (
	"bytes"
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// pinColumns reads pin_salt and offline_pin of a profile.
func pinColumns(t *testing.T, s *Service, profileID int64) (salt, offline sql.NullString) {
	t.Helper()
	if err := s.db.QueryRow(`SELECT pin_salt, offline_pin FROM profiles WHERE id = ?`, profileID).Scan(&salt, &offline); err != nil {
		t.Fatal(err)
	}
	return salt, offline
}

var hexSalt = regexp.MustCompile(`^[0-9a-f]{32}$`)

// AN-19: every new normal PIN gets a fresh salt (an offline verifier of the old PIN stops
// working), removing the PIN clears the salt, and neither touches the offline PIN.
func TestPinSaltFollowsPIN(t *testing.T) {
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)

	plain, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Ohne")})
	if err != nil {
		t.Fatal(err)
	}
	if salt, _ := pinColumns(t, s, plain.ID); salt.Valid {
		t.Errorf("profile without PIN has a salt %q", salt.String)
	}
	kid, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Kind"), PIN: str("4711")})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := pinColumns(t, s, kid.ID)
	if !hexSalt.MatchString(first.String) {
		t.Fatalf("salt of a profile created with a PIN = %q", first.String)
	}
	if err := s.SetOfflinePIN(ctx, sess, kid.ID, "13572468"); err != nil {
		t.Fatal(err)
	}
	_, offline := pinColumns(t, s, kid.ID)

	seen := []string{first.String}
	for _, pin := range []string{"4711", "8080"} { // the same PIN again also gets a new salt
		if err := s.SetProfilePIN(ctx, sess, kid.ID, pin); err != nil {
			t.Fatal(err)
		}
		salt, off := pinColumns(t, s, kid.ID)
		if !hexSalt.MatchString(salt.String) || slices.Contains(seen, salt.String) {
			t.Fatalf("salt after setting PIN %s = %q, seen %v", pin, salt.String, seen)
		}
		if off != offline {
			t.Errorf("SetProfilePIN changed the offline PIN")
		}
		seen = append(seen, salt.String)
	}
	if err := s.RemoveProfilePIN(ctx, sess, kid.ID); err != nil {
		t.Fatal(err)
	}
	salt, off := pinColumns(t, s, kid.ID)
	if salt.Valid {
		t.Errorf("salt after removing the PIN = %q", salt.String)
	}
	if off != offline || !off.Valid {
		t.Errorf("RemoveProfilePIN changed the offline PIN: %v", off)
	}
}

// verifies reports whether the stored verifier JSON verifies pin with PBKDF2-SHA256, computed here
// independently of the service.
func verifies(t *testing.T, stored, pin string) bool {
	t.Helper()
	var v struct {
		Salt       string `json:"salt"`
		Iterations int    `json:"iterations"`
		Hash       string `json:"hash"`
	}
	if err := json.Unmarshal([]byte(stored), &v); err != nil {
		t.Fatalf("stored verifier %q: %v", stored, err)
	}
	salt, err := base64.RawURLEncoding.DecodeString(v.Salt)
	if err != nil || len(salt) != 16 {
		t.Fatalf("salt %q: %d bytes, %v", v.Salt, len(salt), err)
	}
	hash, err := base64.RawURLEncoding.DecodeString(v.Hash)
	if err != nil || len(hash) != 32 {
		t.Fatalf("hash %q: %d bytes, %v", v.Hash, len(hash), err)
	}
	if v.Iterations != 600_000 {
		t.Fatalf("iterations = %d, want 600000", v.Iterations)
	}
	key, err := pbkdf2.Key(sha256.New, pin, salt, v.Iterations, 32)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(key, hash)
}

// AN-19: the separate offline PIN (6 to 12 digits) is stored as a PBKDF2-SHA256 verifier that
// verifies exactly that PIN; only the primary profile manages it, and only for its own account.
func TestOfflinePIN(t *testing.T) {
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	kid, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Kind"), MaxRating: num(6)})
	if err != nil {
		t.Fatal(err)
	}
	if kid.HasOfflinePin {
		t.Fatal("new profile reports an offline PIN")
	}

	if err := s.SetOfflinePIN(ctx, sess, kid.ID, "246813"); err != nil {
		t.Fatal(err)
	}
	_, stored := pinColumns(t, s, kid.ID)
	if !stored.Valid || strings.Contains(stored.String, "246813") {
		t.Fatalf("stored offline PIN = %q", stored.String)
	}
	if !verifies(t, stored.String, "246813") || verifies(t, stored.String, "246814") {
		t.Fatal("the stored verifier does not verify exactly the offline PIN")
	}
	profiles, err := s.ListProfiles(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range profiles {
		if want := p.ID == kid.ID; p.HasOfflinePin != want {
			t.Errorf("profile %s: HasOfflinePin = %v, want %v", p.Name, p.HasOfflinePin, want)
		}
	}
	if kid.HasPin {
		t.Error("the offline PIN set the normal PIN")
	}

	// A new offline PIN: a new salt, the old one no longer verifies.
	if err := s.SetOfflinePIN(ctx, sess, kid.ID, "123456789012"); err != nil {
		t.Fatal(err)
	}
	_, replaced := pinColumns(t, s, kid.ID)
	if replaced == stored || !verifies(t, replaced.String, "123456789012") || verifies(t, replaced.String, "246813") {
		t.Fatalf("replaced verifier = %q", replaced.String)
	}

	for _, pin := range []string{"", "12345", "1234567890123", "12a456", " 123456", "１２３４５６"} {
		if err := s.SetOfflinePIN(ctx, sess, kid.ID, pin); !slices.Equal(validationCodes(err), []string{"pin:invalid_pin"}) {
			t.Errorf("offline PIN %q: %v", pin, err)
		}
	}
	if _, after := pinColumns(t, s, kid.ID); after != replaced {
		t.Error("an invalid offline PIN changed the stored one")
	}

	// Authorisation: the primary profile only; another account's profile is not found.
	kidSession, err := s.SelectProfile(ctx, sess, kid.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetOfflinePIN(ctx, kidSession, kid.ID, "654321"); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("set as a non-primary profile: %v", err)
	}
	if err := s.RemoveOfflinePIN(ctx, kidSession, kid.ID); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("remove as a non-primary profile: %v", err)
	}
	other := addGuestAccount(t, s, "other@example.ch", 18, true, nil)
	foreign, err := s.write.InsertProfile(context.Background(), appq.InsertProfileParams{AccountID: other.ID, Name: "Fremd", MaxRating: 18, Language: "de", AllLibraries: 1, IsDefault: 1, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetOfflinePIN(ctx, sess, foreign.ID, "654321"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("set for another account: %v", err)
	}
	if err := s.RemoveOfflinePIN(ctx, sess, foreign.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("remove for another account: %v", err)
	}
	if _, off := pinColumns(t, s, foreign.ID); off.Valid {
		t.Error("foreign profile got an offline PIN")
	}

	// Removal: idempotent, and the normal PIN is unaffected.
	if err := s.SetProfilePIN(ctx, sess, kid.ID, "4711"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.RemoveOfflinePIN(ctx, sess, kid.ID); err != nil {
			t.Fatal(err)
		}
	}
	salt, off := pinColumns(t, s, kid.ID)
	if off.Valid || !salt.Valid {
		t.Errorf("after removing the offline PIN: offline %v, salt %v", off, salt)
	}
	row, err := s.read.GetProfile(context.Background(), kid.ID)
	if err != nil || !row.PinHash.Valid {
		t.Errorf("normal PIN after removing the offline PIN: %v", err)
	}
}

// The sync gets the PIN material of the asked profiles of one account only: the salt of a normal
// PIN, the offline verifier, nothing for a profile without either, and nothing for profiles of
// other accounts or unknown ids.
func TestOfflinePinsScoping(t *testing.T) {
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	withPin, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("PIN"), PIN: str("4711")})
	if err != nil {
		t.Fatal(err)
	}
	withOffline, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Offline")})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetOfflinePIN(ctx, sess, withOffline.ID, "97531086"); err != nil {
		t.Fatal(err)
	}
	other := addGuestAccount(t, s, "other@example.ch", 18, true, nil)
	foreign, err := s.write.InsertProfile(context.Background(), appq.InsertProfileParams{
		AccountID: other.ID, Name: "Fremd", MaxRating: 18, Language: "de", AllLibraries: 1, IsDefault: 1, Now: 1,
		PinHash: sql.NullString{String: "x", Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.OfflinePins(ctx, sess.Account.ID, []int64{sess.Profile.ID, withPin.ID, withOffline.ID, foreign.ID, 99_999})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("offline pins = %+v, want the three profiles of the account", got)
	}
	if _, ok := got[foreign.ID]; ok {
		t.Error("a profile of another account is included")
	}
	if p := got[sess.Profile.ID]; p.HasPin || p.PinSalt != "" || p.OfflinePin != nil {
		t.Errorf("primary profile without PINs = %+v", p)
	}
	salt, _ := pinColumns(t, s, withPin.ID)
	if p := got[withPin.ID]; !p.HasPin || p.PinSalt != salt.String || !hexSalt.MatchString(p.PinSalt) || p.OfflinePin != nil {
		t.Errorf("profile with a PIN = %+v, stored salt %q", p, salt.String)
	}
	p := got[withOffline.ID]
	if p.HasPin || p.PinSalt != "" || p.OfflinePin == nil || p.OfflinePin.Iterations != model.OfflinePinIterations {
		t.Fatalf("profile with an offline PIN = %+v", p)
	}
	data, err := json.Marshal(map[string]any{"salt": p.OfflinePin.Salt, "iterations": p.OfflinePin.Iterations, "hash": p.OfflinePin.Hash})
	if err != nil {
		t.Fatal(err)
	}
	if !verifies(t, string(data), "97531086") {
		t.Error("the delivered verifier does not verify the offline PIN")
	}

	if got, err := s.OfflinePins(ctx, other.ID, []int64{withPin.ID}); err != nil || len(got) != 0 {
		t.Errorf("another account asking for this account's profile: %+v, %v", got, err)
	}
	if got, err := s.OfflinePins(ctx, sess.Account.ID, nil); err != nil || len(got) != 0 {
		t.Errorf("no ids: %+v, %v", got, err)
	}
}
