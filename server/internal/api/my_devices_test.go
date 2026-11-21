package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

type myDevice struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	SessionKind *string `json:"sessionKind"`
	SignedInAt  *string `json:"signedInAt"`
	Downloads   int     `json:"downloads"`
	Current     bool    `json:"current"`
}

// sessionCookies returns the Set-Cookie headers of the session cookie.
func sessionCookies(rec *httptest.ResponseRecorder) []string {
	var out []string
	for _, c := range rec.Header().Values("Set-Cookie") {
		if strings.HasPrefix(c, SessionCookie+"=") {
			out = append(out, c)
		}
	}
	return out
}

func listMyDevices(t *testing.T, h http.Handler, token string) map[string]myDevice {
	t.Helper()
	sender := send
	if token == "app" {
		sender = sendBearer
	}
	rec := sender(t, h, http.MethodGet, BasePath+"/account/devices", token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("listMyDevices for %s: %d %s", token, rec.Code, rec.Body.String())
	}
	var list []myDevice
	decode(t, rec, &list)
	out := map[string]myDevice{}
	for _, d := range list {
		out[d.ID] = d
	}
	assertNoSecrets(t, "listMyDevices", rec)
	return out
}

// count runs a count(*) query against the fixture's database.
func count(t *testing.T, pf profileFixture, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pf.f.DB.Write.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Concept 27, 4.5: «Meine Geräte» lists the devices by the account of their sessions and downloads,
// and signing a device out removes the account's sessions, downloads, push subscriptions and
// auto-download rules on it and nothing of another account on the same device.
func TestMyDevices(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	ctx := context.Background()

	// A device the owner (P18) and the guest (PG) share, each with a session, a download, a push
	// subscription and an auto-download rule.
	shared := pf.sessions["p6"].DeviceID
	for name, who := range map[string]struct {
		account model.Account
		profile model.Profile
	}{"own-shared": {f.Account, f.P18}, "guest-shared": {f.Guest, f.PG}} {
		_, token, err := pf.a.StartSession(model.WithDeviceID(ctx, shared), who.account, who.profile, "shared")
		if err != nil {
			t.Fatal(err)
		}
		pf.pa.tokens[name] = token
	}
	ownDownload := createDownload(t, h, "own-shared", f.Rated(6).ID, `{"quality":"medium"}`)
	// One download per device and title: the guest takes another title.
	guestDownload := createDownload(t, h, "guest-shared", f.Rated(12).ID, `{"quality":"medium"}`)
	if ownDownload == guestDownload {
		t.Fatal("one download for two accounts")
	}
	for _, s := range []struct {
		account int64
		profile int64
	}{{f.Account.ID, f.P18.ID}, {f.Guest.ID, f.PG.ID}} {
		if _, err := pf.f.DB.Write.Exec(`INSERT INTO push_subscriptions (account_id, device_id, endpoint, p256dh, auth, created_at)
			VALUES (?, ?, ?, 'p', 'a', 1)`, s.account, shared, fmt.Sprintf("https://push.example.test/%d", s.account)); err != nil {
			t.Fatal(err)
		}
		if _, err := pf.f.DB.Write.Exec(`INSERT INTO download_rules (profile_id, device_id, updated_at) VALUES (?, ?, 1)`, s.profile, shared); err != nil {
			t.Fatal(err)
		}
	}

	// A device that only holds a download of the owner: its session signed out.
	away := pf.newSession(t, "away", f.P18)
	createDownload(t, h, "away", f.Rated(6).ID, `{"quality":"low"}`)
	if rec := send(t, h, http.MethodPost, BasePath+"/auth/logout", "away", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	// A device only the guest holds.
	guestOnly := pf.sessions["pg"].DeviceID
	// The owner's phone.
	app := pf.newBearerSession(t, "app", f.P18, auth.AppDevicePhone)
	pf.newBearerSession(t, "tv", f.P18, auth.AppDeviceTV)

	t.Run("list", func(t *testing.T) {
		list := listMyDevices(t, h, "p18")
		if d, ok := list[pf.sessions["p18"].DeviceID]; !ok || !d.Current || d.SessionKind == nil || *d.SessionKind != model.SessionBrowser {
			t.Errorf("own browser = %+v (listed %v), want current with a browser session", d, ok)
		}
		if d := list[shared]; d.Current || d.SessionKind == nil || d.SignedInAt == nil || d.Downloads != 1 {
			t.Errorf("shared device = %+v, want a session and the one download of the owner", d)
		}
		if d, ok := list[away.DeviceID]; !ok || d.SessionKind != nil || d.SignedInAt != nil || d.Downloads != 1 {
			t.Errorf("device with downloads only = %+v (listed %v), want it without session", d, ok)
		}
		if d := list[app.DeviceID]; d.SessionKind == nil || *d.SessionKind != model.SessionApp || d.Name == nil || *d.Name == "" {
			t.Errorf("phone = %+v, want an app session with its name", d)
		}
		if _, ok := list[guestOnly]; ok {
			t.Error("the guest's device is in the owner's list")
		}
		if _, ok := list[auth.ReservedDeviceID]; ok {
			t.Error("the reserved device is listed")
		}
		// Every profile may read; the phone sees itself as this device.
		if _, ok := listMyDevices(t, h, "p6")[shared]; !ok {
			t.Error("a child profile does not see the shared device")
		}
		if d := listMyDevices(t, h, "app")[app.DeviceID]; !d.Current {
			t.Errorf("phone's own entry = %+v, want current", d)
		}
	})

	t.Run("refused", func(t *testing.T) {
		path := BasePath + "/account/devices/" + shared
		if rec := send(t, h, http.MethodDelete, path, "p6", "", nil); rec.Code != http.StatusForbidden || problemCode(t, rec) != "primary_profile_required" {
			t.Errorf("child profile signs out: %d %s", rec.Code, rec.Body.String())
		}
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			target := BasePath + "/account/devices"
			if method == http.MethodDelete {
				target = path
			}
			if rec := sendBearer(t, h, method, target, "tv", "", nil); rec.Code != http.StatusForbidden || problemCode(t, rec) != "client_not_allowed" {
				t.Errorf("device session %s: %d %s", method, rec.Code, rec.Body.String())
			}
		}
		for name, id := range map[string]string{"guest's device": guestOnly, "reserved": auth.ReservedDeviceID, "unknown": strings.Repeat("0", 32)} {
			if rec := send(t, h, http.MethodDelete, BasePath+"/account/devices/"+id, "p18", "", nil); rec.Code != http.StatusNotFound {
				t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
			}
		}
		if n := count(t, pf, `SELECT count(*) FROM downloads WHERE device_id = ?`, shared); n != 2 {
			t.Errorf("downloads on the shared device after the refusals: %d", n)
		}
	})

	t.Run("sign out", func(t *testing.T) {
		rec := send(t, h, http.MethodDelete, BasePath+"/account/devices/"+shared, "p18", "", nil)
		if rec.Code != http.StatusNoContent || len(sessionCookies(rec)) != 0 {
			t.Fatalf("sign out: %d %q %s", rec.Code, sessionCookies(rec), rec.Body.String())
		}
		if rec := send(t, h, http.MethodGet, BasePath+"/auth/session", "own-shared", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("the owner's session on the device: %d, want 401", rec.Code)
		}
		if rec := send(t, h, http.MethodGet, BasePath+"/auth/session", "guest-shared", "", nil); rec.Code != http.StatusOK {
			t.Errorf("the guest's session on the device: %d, want 200", rec.Code)
		}
		for what, c := range map[string]struct {
			query      string
			owner, all int
		}{
			"downloads":          {`SELECT count(*) FROM downloads WHERE device_id = ?1 AND account_id = ?2`, 0, 1},
			"push subscriptions": {`SELECT count(*) FROM push_subscriptions WHERE device_id = ?1 AND account_id = ?2`, 0, 1},
			"rules": {`SELECT count(*) FROM download_rules WHERE device_id = ?1
				AND profile_id IN (SELECT id FROM profiles WHERE account_id = ?2)`, 0, 1},
		} {
			if n := count(t, pf, c.query, shared, f.Account.ID); n != c.owner {
				t.Errorf("%s of the owner: %d, want %d", what, n, c.owner)
			}
			if n := count(t, pf, c.query, shared, f.Guest.ID); n != c.all {
				t.Errorf("%s of the guest: %d, want %d", what, n, c.all)
			}
		}
		// The device learns it from the revocation list; the guest's download is not on it.
		if n := count(t, pf, `SELECT count(*) FROM download_tombstones WHERE device_id = ? AND id = ?`, shared, ownDownload); n != 1 {
			t.Errorf("tombstone of the owner's download: %d", n)
		}
		if n := count(t, pf, `SELECT count(*) FROM download_tombstones WHERE id = ?`, guestDownload); n != 0 {
			t.Errorf("tombstone of the guest's download: %d", n)
		}
		// P6 had its session there too (the shared device is its own): it is gone from the list.
		if _, ok := listMyDevices(t, h, "p18")[shared]; ok {
			t.Error("the signed-out device is still listed")
		}
		if rec := send(t, h, http.MethodGet, BasePath+"/auth/session", "p6", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("P6's session on the signed-out device: %d, want 401", rec.Code)
		}
	})

	t.Run("own device", func(t *testing.T) {
		if rec := sendBearer(t, h, http.MethodDelete, BasePath+"/account/devices/"+app.DeviceID, "app", "", nil); rec.Code != http.StatusNoContent {
			t.Fatalf("phone signs itself out: %d %s", rec.Code, rec.Body.String())
		} else {
			noCookie(t, "phone sign-out", rec)
		}
		if rec := sendBearer(t, h, http.MethodGet, BasePath+"/auth/session", "app", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("phone after its sign-out: %d, want 401", rec.Code)
		}
		own := pf.sessions["p18"].DeviceID
		rec := send(t, h, http.MethodDelete, BasePath+"/account/devices/"+own, "p18", "", nil)
		if c := sessionCookies(rec); rec.Code != http.StatusNoContent || len(c) != 1 || !strings.Contains(c[0], "Max-Age=0") {
			t.Errorf("browser signs itself out: %d, session cookie %q, want it cleared", rec.Code, c)
		}
		if rec := send(t, h, http.MethodGet, BasePath+"/auth/session", "p18", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("browser after its sign-out: %d, want 401", rec.Code)
		}
	})
}

// Concept 27, 4.5: forgetting a device in the administration removes its push subscriptions of every
// account before the device row (afterwards their device id would be empty).
func TestDeleteDeviceDropsPushSubscriptions(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	device := pf.sessions["p6"].DeviceID
	for _, account := range []int64{f.Account.ID, f.Guest.ID} {
		if _, err := pf.f.DB.Write.Exec(`INSERT INTO push_subscriptions (account_id, device_id, endpoint, p256dh, auth, created_at)
			VALUES (?, ?, ?, 'p', 'a', 1)`, account, device, fmt.Sprintf("https://push.example.test/%d", account)); err != nil {
			t.Fatal(err)
		}
	}
	other := pf.sessions["p12"].DeviceID
	if _, err := pf.f.DB.Write.Exec(`INSERT INTO push_subscriptions (account_id, device_id, endpoint, p256dh, auth, created_at)
		VALUES (?, ?, 'https://push.example.test/other', 'p', 'a', 1)`, f.Account.ID, other); err != nil {
		t.Fatal(err)
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/devices/"+device, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete device: %d %s", rec.Code, rec.Body.String())
	}
	if n := count(t, pf, `SELECT count(*) FROM push_subscriptions WHERE endpoint LIKE 'https://push.example.test/%' AND endpoint <> 'https://push.example.test/other'`); n != 0 {
		t.Errorf("subscriptions of the forgotten device: %d, want 0", n)
	}
	if n := count(t, pf, `SELECT count(*) FROM push_subscriptions WHERE device_id = ?`, other); n != 1 {
		t.Errorf("subscription of another device: %d, want 1", n)
	}
}
