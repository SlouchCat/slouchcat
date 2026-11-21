package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// App sign-in (concept 27 4.2, ADR 0040): the Android apps sign in with e-mail and password and get
// a bearer token instead of the cookie. The kind of the device decides the kind of the session: a
// phone or a tablet gets an app session, a TV a device session.

// Device kinds of appLogin (AppLoginRequest.kind); they are also the device type of the row.
const (
	AppDevicePhone  = "phone"
	AppDeviceTablet = "tablet"
	AppDeviceTV     = "tv"
)

// maxDeviceName is the column limit of devices.name (migration 00011).
const maxDeviceName = 80

// SessionKindOfDevice returns the session kind a device kind of appLogin signs in with and whether
// the device kind is known.
func SessionKindOfDevice(deviceKind string) (string, bool) {
	switch deviceKind {
	case AppDevicePhone, AppDeviceTablet:
		return model.SessionApp, true
	case AppDeviceTV:
		return model.SessionDevice, true
	default:
		return "", false
	}
}

// AppLogin checks the credentials like Login (same attempt pot, same Seerr hand-over of the
// password) and starts an app or device session. The device row is written right away with the
// name and type the app reported. deviceID is the id the app already holds, if any: it is taken
// only when it is well-formed, not the reserved id of the PIN ceiling and no other account holds a
// living session or a download on it; otherwise the server hands out a fresh one. The returned
// session carries the device id (Session.DeviceID) and the plain token, which exists only in this
// answer.
//
// Errors: *model.ValidationError (email, password, deviceName, kind), model.ErrTooManyAttempts,
// model.ErrInvalidCredentials.
func (s *Service) AppLogin(ctx context.Context, email, password, deviceName, deviceKind, deviceID, userAgent string) (model.Session, string, error) {
	name := strings.TrimSpace(deviceName)
	var verr *model.ValidationError
	switch {
	case name == "":
		verr = verr.Add("deviceName", "required")
	case utf8.RuneCountInString(name) > maxDeviceName:
		verr = verr.Add("deviceName", "too_long")
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		verr = verr.Add("deviceName", "invalid_value")
	}
	_, ok := SessionKindOfDevice(deviceKind)
	if !ok {
		verr = verr.Add("kind", "invalid_value")
	}
	if verr != nil {
		// The other fields are checked the same way Login checks them, so the client sees all at once.
		if _, shape := s.checkCredentialsShape(email, password); shape != nil {
			verr.Fields = append(verr.Fields, shape.Fields...)
		}
		return model.Session{}, "", verr
	}
	acc, err := s.checkCredentials(ctx, email, password)
	if err != nil {
		return model.Session{}, "", err
	}
	account, profile, err := s.signInProfile(ctx, acc)
	if err != nil {
		return model.Session{}, "", err
	}
	sess, token, err := s.StartAppSession(ctx, account, profile, name, deviceKind, deviceID, userAgent)
	if err != nil {
		return model.Session{}, "", err
	}
	s.seen(ctx, acc.ID, acc.Email, password, PasswordMomentLogin)
	return sess, token, nil
}

// StartAppSession starts the app or device session of AppLogin for account, without a password
// check: the session kind follows deviceKind (SessionKindOfDevice), the device row gets deviceName
// and deviceKind, and deviceID is kept under the rules of AppLogin. A zero profile starts the
// session without an active profile. deviceName must be valid (AppLogin checks it).
func (s *Service) StartAppSession(ctx context.Context, account model.Account, profile model.Profile, deviceName, deviceKind, deviceID, userAgent string) (model.Session, string, error) {
	kind, ok := SessionKindOfDevice(deviceKind)
	if !ok {
		return model.Session{}, "", (&model.ValidationError{}).Add("kind", "invalid_value")
	}
	device, err := s.appDevice(ctx, account.ID, deviceID)
	if err != nil {
		return model.Session{}, "", err
	}
	if err := s.write.UpsertNamedDevice(ctx, appq.UpsertNamedDeviceParams{
		ID: device, UserAgent: sql.NullString{String: truncate(userAgent, maxUserAgentLen), Valid: userAgent != ""},
		Now: s.now().UnixMilli(), Name: sql.NullString{String: deviceName, Valid: true}, Type: sql.NullString{String: deviceKind, Valid: true},
	}); err != nil {
		return model.Session{}, "", fmt.Errorf("upsert device: %w", err)
	}
	sess, token, err := s.insertSession(ctx, account, profile, userAgent, device, kind)
	if err != nil {
		return model.Session{}, "", err
	}
	s.log.InfoContext(ctx, "app session started", "account_id", account.ID, "session_id", sess.ID, "kind", kind)
	return sess, token, nil
}

// appDevice returns the device of an app sign-in: the id the app brought along when it may keep it
// (see AppLogin), else a fresh one.
func (s *Service) appDevice(ctx context.Context, accountID int64, brought string) (string, error) {
	if devicePattern.MatchString(brought) && brought != profileCeilingDevice {
		held, err := s.write.DeviceHeldByOtherAccount(ctx, appq.DeviceHeldByOtherAccountParams{
			ID: sql.NullString{String: brought, Valid: true}, AccountID: accountID, Now: s.now().UnixMilli(),
		})
		if err != nil {
			return "", fmt.Errorf("device of another account: %w", err)
		}
		if !held.Bool {
			return brought, nil
		}
	}
	return newDeviceID()
}

// checkCredentialsShape is the field check of checkCredentials without the attempt count and the
// lookup; nil when e-mail and password have a usable shape.
func (s *Service) checkCredentialsShape(email, password string) (string, *model.ValidationError) {
	email = NormalizeEmail(email)
	var verr *model.ValidationError
	switch {
	case email == "":
		verr = verr.Add("email", "required")
	case utf8.RuneCountInString(email) > maxEmailLen:
		verr = verr.Add("email", "too_long")
	}
	switch {
	case password == "":
		verr = verr.Add("password", "required")
	case utf8.RuneCountInString(password) > maxPasswordLen:
		verr = verr.Add("password", "too_long")
	}
	return email, verr
}
