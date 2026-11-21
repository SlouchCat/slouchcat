package api

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/auth"
)

// Concept 27 sections 6 and 10 (ADR 0040) over HTTP: the voucher, its redeem with the sealed
// device token, and the proof of the server.

func newTVKeyB64(t *testing.T) (*ecdh.PrivateKey, string) {
	t.Helper()
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k, base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes())
}

func TestPairingOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	pf.newBearerSession(t, "appchild", pf.f.P12, auth.AppDevicePhone)
	pf.newBearerSession(t, "tv", pf.f.P18, auth.AppDeviceTV)
	tv, tvKey := newTVKeyB64(t)
	body := `{"tvKey":"` + tvKey + `"}`
	const path = BasePath + "/account/pairing-vouchers"

	for _, c := range []struct {
		name, token, code string
		bearer            bool
	}{
		{"browser", "p18", "app_session_required", false},
		{"device", "tv", "client_not_allowed", true},
		{"child", "appchild", "primary_profile_required", true},
	} {
		rec := send(t, h, http.MethodPost, path, c.token, body, nil)
		if c.bearer {
			rec = sendBearer(t, h, http.MethodPost, path, c.token, body, nil)
		}
		if rec.Code != http.StatusForbidden || problemCode(t, rec) != c.code {
			t.Errorf("%s: %d %s, want 403 %s", c.name, rec.Code, rec.Body.String(), c.code)
		}
	}

	rec := sendBearer(t, h, http.MethodPost, path, "app", body, nil)
	if rec.Code != http.StatusCreated || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("app session: %d %s (Cache-Control %q)", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
	}
	noCookie(t, "createPairingVoucher", rec)
	var v gen.PairingVoucher
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || len(v.Voucher) != 43 {
		t.Fatalf("voucher %q: %v", v.Voucher, err)
	}

	redeem := `{"voucher":"` + v.Voucher + `","tvKey":"` + tvKey + `","deviceName":"Wohnzimmer","systemVersion":"Android TV 14"}`
	rec = sendBearer(t, h, http.MethodPost, BasePath+"/auth/pairing/redeem", "", redeem, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("redeem: %d %s", rec.Code, rec.Body.String())
	}
	noCookie(t, "redeemPairingVoucher", rec) // neither a session nor a device cookie
	var out gen.PairingRedeemed
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if string(out.AccountEmail) != pf.f.Account.Email {
		t.Errorf("accountEmail %q, want %q", out.AccountEmail, pf.f.Account.Email)
	}
	sealed, err := base64.RawURLEncoding.DecodeString(out.SealedToken)
	if err != nil {
		t.Fatal(err)
	}
	sk, _ := hpke.NewDHKEMPrivateKey(tv)
	plain, err := hpke.Open(sk, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(auth.RedeemInfo), sealed)
	if err != nil {
		t.Fatal(err)
	}
	var answer struct {
		Token    string `json:"token"`
		DeviceID string `json:"deviceId"`
	}
	if err := json.Unmarshal(plain, &answer); err != nil {
		t.Fatal(err)
	}
	pf.pa.tokens["paired"] = answer.Token
	rec = sendBearer(t, h, http.MethodGet, BasePath+"/auth/session", "paired", "", nil)
	var info gen.SessionInfo
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &info) != nil || info.Account.Id != pf.f.Account.ID {
		t.Fatalf("session of the sealed token: %d %s", rec.Code, rec.Body.String())
	}
	// A device session: the TV may not issue vouchers itself (x-clients [app]).
	if rec := sendBearer(t, h, http.MethodPost, path, "paired", body, nil); rec.Code != http.StatusForbidden || problemCode(t, rec) != "client_not_allowed" {
		t.Errorf("voucher with the TV's token: %d %s", rec.Code, rec.Body.String())
	}
	rec = sendBearer(t, h, http.MethodPost, BasePath+"/auth/pairing/redeem", "", redeem, nil)
	if rec.Code != http.StatusGone || problemCode(t, rec) != "voucher_invalid" {
		t.Errorf("second redeem: %d %s", rec.Code, rec.Body.String())
	}
}

// getHealth names the key, proveServer signs with it; a nonce that is not 32 bytes is 422, and
// the proofs of one address run out (429 with Retry-After).
func TestProveServerOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	rec := sendBearer(t, h, http.MethodGet, BasePath+"/healthz", "", "", nil)
	var health gen.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	pub, err := base64.RawURLEncoding.DecodeString(health.ServerKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatalf("serverKey %q", health.ServerKey)
	}
	nonce := make([]byte, auth.NonceBytes)
	_, _ = rand.Read(nonce)
	rec = sendBearer(t, h, http.MethodPost, BasePath+"/server/prove", "", `{"nonce":"`+base64.RawURLEncoding.EncodeToString(nonce)+`"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("prove: %d %s", rec.Code, rec.Body.String())
	}
	noCookie(t, "proveServer", rec)
	var proof gen.ServerProof
	if err := json.Unmarshal(rec.Body.Bytes(), &proof); err != nil {
		t.Fatal(err)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(proof.Signature)
	if proof.ServerKey != health.ServerKey || !ed25519.Verify(pub, append([]byte(auth.ServerProofPrefix), nonce...), sig) {
		t.Fatalf("proof %+v does not verify with %s", proof, health.ServerKey)
	}
	rec = sendBearer(t, h, http.MethodPost, BasePath+"/server/prove", "", `{"nonce":"short"}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
		t.Errorf("short nonce: %d %s", rec.Code, rec.Body.String())
	}
	var last int
	for range 40 {
		rec = sendBearer(t, h, http.MethodPost, BasePath+"/server/prove", "", `{"nonce":"`+base64.RawURLEncoding.EncodeToString(nonce)+`"}`, nil)
		if last = rec.Code; last != http.StatusOK {
			break
		}
	}
	if last != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || problemCode(t, rec) != "too_many_attempts" {
		t.Errorf("after many proofs: %d %s", last, rec.Body.String())
	}
}

// The whole way of a push subscription of the phone app (security review M-1 of part 1): the app
// registers it with a child profile active, it carries the session's device, and signing the device
// out under «Meine Geräte» or logging out removes it with the session.
func TestAppPushSubscriptionEndsWithTheDevice(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	count := func(device string) int {
		t.Helper()
		var n int
		if err := pf.f.DB.Read.QueryRow(`SELECT count(*) FROM push_subscriptions WHERE device_id = ?`, device).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, way := range []string{"my devices", "logout"} {
		name := "push-" + strings.ReplaceAll(way, " ", "-")
		app := pf.newBearerSession(t, name, pf.f.P12, auth.AppDevicePhone)
		rec := sendBearer(t, h, http.MethodPost, BasePath+"/me/push-subscriptions", name,
			pushSubscriptionBody("https://push.slouchcat.com/v1/push/1."+name), nil)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: subscribe with a child profile: %d %s", way, rec.Code, rec.Body.String())
		}
		if count(app.DeviceID) != 1 {
			t.Fatalf("%s: the subscription does not carry the app's device", way)
		}
		switch way {
		case "my devices":
			rec = send(t, h, http.MethodDelete, BasePath+"/account/devices/"+app.DeviceID, "p18", "", nil)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("signOutMyDevice: %d %s", rec.Code, rec.Body.String())
			}
		case "logout":
			if rec = sendBearer(t, h, http.MethodPost, BasePath+"/auth/logout", name, "", nil); rec.Code != http.StatusNoContent {
				t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
			}
		}
		if n := count(app.DeviceID); n != 0 {
			t.Errorf("%s: %d subscriptions of the device left", way, n)
		}
		if rec = sendBearer(t, h, http.MethodGet, BasePath+"/auth/session", name, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: the app token still works: %d", way, rec.Code)
		}
	}
}
