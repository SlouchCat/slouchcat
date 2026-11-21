package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// subscribe stores a push subscription of the account on the device and returns its endpoint.
func subscribe(t *testing.T, s *Service, accountID int64, device string) string {
	t.Helper()
	endpoint := fmt.Sprintf("https://push.example.test/%d/%s", accountID, device)
	if _, err := s.db.Exec(`INSERT INTO push_subscriptions (account_id, device_id, endpoint, p256dh, auth, created_at)
		VALUES (?, ?, ?, 'p', 'a', 1)`, accountID, device, endpoint); err != nil {
		t.Fatal(err)
	}
	return endpoint
}

// subscribed reports which of the endpoints still have a subscription.
func subscribed(t *testing.T, s *Service, endpoints ...string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, e := range endpoints {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM push_subscriptions WHERE endpoint = ?`, e).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[e] = n == 1
	}
	return out
}

// F-24 and concept 27, 4.5: a new password revokes the app and device sessions of the account like
// every other session, and takes the push subscriptions of their devices along; browsers keep
// theirs (F-40), the calling session stays.
func TestPasswordChangeEndsAppAndDeviceSessions(t *testing.T) {
	t.Parallel()
	ctx := model.WithClientIP(context.Background(), "192.0.2.12")
	s, _, acc := appOwner(t)
	prof, err := s.read.GetDefaultProfile(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	profile := model.Profile{ID: prof.ID, AccountID: acc.ID, IsPrimary: true}
	calling, _, err := s.StartSession(ctx, acc, profile, "calling")
	if err != nil {
		t.Fatal(err)
	}
	web, webToken, err := s.StartSession(ctx, acc, profile, "web")
	if err != nil {
		t.Fatal(err)
	}
	phone, phoneToken, err := s.AppLogin(ctx, testEmail, testPassword, "Phone", AppDevicePhone, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	tv, tvToken, err := s.AppLogin(ctx, testEmail, testPassword, "TV", AppDeviceTV, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	pushCalling := subscribe(t, s, acc.ID, calling.DeviceID)
	pushWeb := subscribe(t, s, acc.ID, web.DeviceID)
	pushPhone := subscribe(t, s, acc.ID, phone.DeviceID)
	pushTV := subscribe(t, s, acc.ID, tv.DeviceID)

	if _, err := s.UpdateAccount(ctx, calling, model.AccountUpdate{CurrentPassword: testPassword, NewPassword: ptr("a brand new password")}); err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"browser": webToken, "app": phoneToken, "device": tvToken} {
		if _, _, err := s.Resolve(ctx, token); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s session after the password change: %v, want revoked", name, err)
		}
	}
	got := subscribed(t, s, pushCalling, pushWeb, pushPhone, pushTV)
	want := map[string]bool{pushCalling: true, pushWeb: true, pushPhone: false, pushTV: false}
	for e, w := range want {
		if got[e] != w {
			t.Errorf("subscription %s kept = %v, want %v", e, got[e], w)
		}
	}
}

// Concept 27, 4.5: the logout of an app or device session removes the account's push
// subscriptions of its device; a browser's logout keeps its subscription (F-40).
func TestLogoutDropsAppPushSubscriptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _, acc := appOwner(t)
	prof, err := s.read.GetDefaultProfile(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	web, _, err := s.StartSession(ctx, acc, model.Profile{ID: prof.ID, AccountID: acc.ID, IsPrimary: true}, "web")
	if err != nil {
		t.Fatal(err)
	}
	phone, _, err := s.AppLogin(ctx, testEmail, testPassword, "Phone", AppDevicePhone, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	tv, _, err := s.AppLogin(ctx, testEmail, testPassword, "TV", AppDeviceTV, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	pushWeb := subscribe(t, s, acc.ID, web.DeviceID)
	pushPhone := subscribe(t, s, acc.ID, phone.DeviceID)
	pushTV := subscribe(t, s, acc.ID, tv.DeviceID)
	for _, id := range []int64{web.ID, phone.ID, tv.ID} {
		if err := s.Logout(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	got := subscribed(t, s, pushWeb, pushPhone, pushTV)
	if !got[pushWeb] || got[pushPhone] || got[pushTV] {
		t.Errorf("after the logouts: browser %v, phone %v, TV %v; want true, false, false", got[pushWeb], got[pushPhone], got[pushTV])
	}
}
