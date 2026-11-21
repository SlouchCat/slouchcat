package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 27 4.2 and 4.3 (ADR 0040): appLogin, the kinds of session and what the cleanup spares.

// appOwner creates the owner with testPassword and returns the service, its clock and the account.
func appOwner(t *testing.T) (*Service, *fakeClock, model.Account) {
	t.Helper()
	s, clock := newService(t)
	acc, _, err := s.CreateOwner(context.Background(), testEmail, testPassword, "de")
	if err != nil {
		t.Fatal(err)
	}
	return s, clock, acc
}

func TestAppLoginKinds(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ device, kind string }{
		{AppDevicePhone, model.SessionApp}, {AppDeviceTablet, model.SessionApp}, {AppDeviceTV, model.SessionDevice},
	} {
		t.Run(c.device, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			s, clock, acc := appOwner(t)
			var seen []string
			s.WithPasswordSeen(func(_ context.Context, _ int64, _, _, moment string) { seen = append(seen, moment) })
			sess, token, err := s.AppLogin(ctx, testEmail, testPassword, "  Pixel 9 ", c.device, "", "okhttp/5")
			if err != nil {
				t.Fatal(err)
			}
			if sess.Kind != c.kind || !sess.IsBearer() || !sess.ExpiresAt.Equal(NeverExpires) {
				t.Fatalf("session kind %q expires %v, want %q and %v", sess.Kind, sess.ExpiresAt, c.kind, NeverExpires)
			}
			if len(token) != 43 || !devicePattern.MatchString(sess.DeviceID) {
				t.Fatalf("token %d characters, device %q", len(token), sess.DeviceID)
			}
			if len(seen) != 1 || seen[0] != PasswordMomentLogin {
				t.Errorf("password seen %v, want [login] (the Seerr hand-over of login)", seen)
			}
			dev, err := s.read.GetDevice(ctx, sess.DeviceID)
			if err != nil {
				t.Fatal(err)
			}
			if dev.Name.String != "Pixel 9" || dev.Type.String != c.device {
				t.Errorf("device row name %q type %q, want Pixel 9 and %s", dev.Name.String, dev.Type.String, c.device)
			}
			// Use after a while: the contact moves, the expiry does not, and nothing asks for a cookie.
			clock.advance(40 * 24 * time.Hour)
			got, extended, err := s.Resolve(ctx, token)
			if err != nil {
				t.Fatal(err)
			}
			if extended || !got.ExpiresAt.Equal(NeverExpires) || got.Kind != c.kind || got.Account.ID != acc.ID || got.DeviceID != sess.DeviceID {
				t.Fatalf("resolved %+v extended %v", got, extended)
			}
			row, err := s.read.GetSessionByTokenHash(ctx, appq.GetSessionByTokenHashParams{TokenHash: hashToken(token), Now: clock.now().UnixMilli()})
			if err != nil {
				t.Fatal(err)
			}
			if row.LastSeenAt != clock.now().UnixMilli() || row.ExpiresAt != NeverExpires.UnixMilli() {
				t.Errorf("row last seen %d expires %d, want %d and %d", row.LastSeenAt, row.ExpiresAt, clock.now().UnixMilli(), NeverExpires.UnixMilli())
			}
		})
	}
}

// A browser session keeps its sliding expiry.
func TestBrowserSessionStillSlides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clock, _ := appOwner(t)
	sess, token, err := s.Login(ctx, testEmail, testPassword, "Firefox")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Kind != model.SessionBrowser || sess.IsBearer() {
		t.Fatalf("login kind %q", sess.Kind)
	}
	clock.advance(time.Hour)
	got, extended, err := s.Resolve(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if !extended || !got.ExpiresAt.Equal(clock.now().Add(SessionLifetime)) {
		t.Errorf("extended %v expires %v, want true and %v", extended, got.ExpiresAt, clock.now().Add(SessionLifetime))
	}
}

func TestAppLoginRefuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _, _ := appOwner(t)
	if _, _, err := s.AppLogin(ctx, testEmail, "wrong password!", "Pixel", AppDevicePhone, "", ""); !errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("wrong password: %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := s.AppLogin(ctx, "nobody@example.ch", testPassword, "Pixel", AppDevicePhone, "", ""); !errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("unknown e-mail: %v, want ErrInvalidCredentials", err)
	}
	long := make([]rune, maxDeviceName+1)
	for i := range long {
		long[i] = 'x'
	}
	for _, c := range []struct{ name, kind, field, code string }{
		{"", AppDevicePhone, "deviceName", "required"},
		{"   ", AppDevicePhone, "deviceName", "required"},
		{string(long), AppDevicePhone, "deviceName", "too_long"},
		{"\u200b\u202e\n", AppDevicePhone, "deviceName", "required"}, // only hidden characters (L-4)
		{"Pixel", "desktop", "kind", "invalid_value"},
		{"Pixel", "", "kind", "invalid_value"},
	} {
		_, _, err := s.AppLogin(ctx, testEmail, testPassword, c.name, c.kind, "", "")
		var verr *model.ValidationError
		if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != c.field || verr.Fields[0].Code != c.code {
			t.Errorf("name %q kind %q: %v, want %s %s", c.name, c.kind, err, c.field, c.code)
		}
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sessions`).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d sessions after refused sign-ins (%v), want none", n, err)
	}
}

// appLogin counts in the pot of login (per client address).
func TestAppLoginSharesTheAttemptPot(t *testing.T) {
	t.Parallel()
	s, _, _ := appOwner(t)
	ctx := model.WithClientIP(context.Background(), "198.51.100.7")
	for range attemptBurst {
		if _, _, err := s.Login(ctx, testEmail, "wrong password!", ""); !errors.Is(err, model.ErrInvalidCredentials) {
			t.Fatalf("login: %v", err)
		}
	}
	if _, _, err := s.AppLogin(ctx, testEmail, testPassword, "Pixel", AppDevicePhone, "", ""); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("appLogin after %d failed logins: %v, want ErrTooManyAttempts", attemptBurst, err)
	}
}

// The device id an app brings along: kept when it is its own or new, refused when another account
// holds a living session or a download on it, when it is the reserved id or malformed.
func TestAppLoginDeviceID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := vistest.New(t)
	f.AddGuest(t)
	s := New(f.DB, vistest.Logger())
	start := func(account model.Account, device string) model.Session {
		t.Helper()
		sess, _, err := s.StartAppSession(ctx, account, model.Profile{}, "Phone", AppDevicePhone, device, "")
		if err != nil {
			t.Fatal(err)
		}
		return sess
	}
	fresh := "0123456789abcdef0123456789abcdef"
	if got := start(f.Account, fresh).DeviceID; got != fresh {
		t.Errorf("new device id: got %q, want it kept", got)
	}
	if got := start(f.Account, fresh).DeviceID; got != fresh {
		t.Errorf("own device id: got %q, want it kept", got)
	}
	if got := start(f.Guest, fresh).DeviceID; got == fresh {
		t.Error("the guest took over the device the owner's session lives on")
	}
	for _, bad := range []string{ReservedDeviceID, "ABCDEF0123456789ABCDEF0123456789", "short"} {
		if got := start(f.Account, bad).DeviceID; got == bad || !devicePattern.MatchString(got) {
			t.Errorf("device id %q: got %q, want a fresh one", bad, got)
		}
	}
	// A download of the owner on a device without a session also keeps the guest away.
	held := "fedcba9876543210fedcba9876543210"
	m := f.Rated(6)
	if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, 1)`, held); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO downloads (id, account_id, profile_id, device_id, item_id, media_file_id, quality, audio_stream,
		state, estimated_bytes, allowed_profile_ids, created_at, updated_at) VALUES ('01JAPPLOGIN000000000000001', ?, ?, ?, ?, ?, 'low', 1, 'ready', 1, '[]', 1, 1)`,
		f.Account.ID, f.P18.ID, held, m.ID, m.FileID); err != nil {
		t.Fatal(err)
	}
	if got := start(f.Guest, held).DeviceID; got == held {
		t.Error("the guest took over a device with the owner's download")
	}
	if got := start(f.Account, held).DeviceID; got != held {
		t.Errorf("the owner's own device with its download: got %q, want it kept", got)
	}
	// The name of the last sign-in wins; a later playback start does not touch it (package playback).
	start(f.Account, held)
	if dev, err := s.read.GetDevice(ctx, held); err != nil || dev.Name.String != "Phone" || dev.Type.String != AppDevicePhone {
		t.Errorf("device row %+v (%v)", dev, err)
	}
}

// The cleanup after 400 days spares a device with a living app or device session (and so its
// downloads); a browser device without a living bearer session goes as before.
func TestStaleCleanupSparesBearerDevices(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clock, acc := appOwner(t)
	app, appToken, err := s.AppLogin(ctx, testEmail, testPassword, "Pixel", AppDevicePhone, "", "")
	if err != nil {
		t.Fatal(err)
	}
	tv, _, err := s.AppLogin(ctx, testEmail, testPassword, "TV", AppDeviceTV, "", "")
	if err != nil {
		t.Fatal(err)
	}
	browser, _, err := s.Login(ctx, testEmail, testPassword, "Firefox")
	if err != nil {
		t.Fatal(err)
	}
	revoked, _, err := s.AppLogin(ctx, testEmail, testPassword, "Old phone", AppDevicePhone, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	clock.advance(deviceRetention + 24*time.Hour)
	if _, err := s.DeleteExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		device string
		kept   bool
	}{"app": {app.DeviceID, true}, "tv": {tv.DeviceID, true}, "browser": {browser.DeviceID, false}, "revoked app": {revoked.DeviceID, false}} {
		_, err := s.read.GetDevice(ctx, c.device)
		if kept := err == nil; kept != c.kept {
			t.Errorf("%s device kept %v (%v), want %v", name, kept, err, c.kept)
		}
	}
	got, _, err := s.Resolve(ctx, appToken)
	if err != nil || got.DeviceID != app.DeviceID || got.Account.ID != acc.ID {
		t.Fatalf("app session after 401 days: %+v, %v", got, err)
	}
}

// A vanished device ends an app or device session (401) instead of giving it a fresh id; a browser
// session from before migration 00007 still gets one.
func TestVanishedDeviceEndsBearerSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _, _ := appOwner(t)
	app, appToken, err := s.AppLogin(ctx, testEmail, testPassword, "Pixel", AppDevicePhone, "", "")
	if err != nil {
		t.Fatal(err)
	}
	browser, browserToken, err := s.Login(ctx, testEmail, testPassword, "Firefox")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{app.DeviceID, browser.DeviceID} {
		if _, err := s.write.DeleteDevice(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.Resolve(ctx, appToken); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("app session without device: %v, want ErrNotFound", err)
	}
	var revokedAt *int64
	if err := s.db.QueryRowContext(ctx, `SELECT revoked_at FROM sessions WHERE id = ?`, app.ID).Scan(&revokedAt); err != nil || revokedAt == nil {
		t.Errorf("app session without device not revoked (%v)", err)
	}
	got, _, err := s.Resolve(ctx, browserToken)
	if err != nil || got.DeviceID == "" || got.DeviceID == browser.DeviceID {
		t.Errorf("browser session without device: %+v, %v; want a fresh device", got, err)
	}
}

// A new appLogin of the account on the same device ends the older app session there (code review
// m-5, security review L-3): a token the app lost does not live on. A browser session on the device
// and an app session on another device stay.
func TestAppLoginReplacesTheOlderSessionOfTheDevice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _, acc := appOwner(t)
	first, oldToken, err := s.AppLogin(ctx, testEmail, testPassword, "Pixel", AppDevicePhone, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	browser, browserToken, err := s.StartSession(model.WithDeviceID(ctx, first.DeviceID), acc, model.Profile{}, "Firefox")
	if err != nil {
		t.Fatal(err)
	}
	_, otherToken, err := s.AppLogin(ctx, testEmail, testPassword, "Tablet", AppDeviceTablet, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	second, newToken, err := s.AppLogin(ctx, testEmail, testPassword, "Pixel", AppDevicePhone, first.DeviceID, "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	if second.DeviceID != first.DeviceID || browser.DeviceID != first.DeviceID {
		t.Fatalf("devices %s, %s, %s; want one", first.DeviceID, second.DeviceID, browser.DeviceID)
	}
	if _, _, err := s.Resolve(ctx, oldToken); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("older app token on the device: %v, want revoked", err)
	}
	for name, token := range map[string]string{"new": newToken, "browser": browserToken, "other device": otherToken} {
		if _, _, err := s.Resolve(ctx, token); err != nil {
			t.Errorf("%s: %v, want alive", name, err)
		}
	}
}
