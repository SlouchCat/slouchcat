package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 27 4.1 to 4.4 and 11.5 (ADR 0040): the bearer path of the Android apps, the pairing of
// token and carrier, the x-clients list and the app version gate.

// newBearerSession starts an app session (deviceKind phone or tablet) or a device session (tv) for
// the account of profile p and registers its token under the logical name, like newSession does
// for a browser session. sendBearer sends the name as the bearer token.
func (pf profileFixture) newBearerSession(t *testing.T, name string, p model.Profile, deviceKind string) model.Session {
	t.Helper()
	account := pf.f.Account
	switch {
	case pf.f.Guest.ID != 0 && p.AccountID == pf.f.Guest.ID:
		account = pf.f.Guest
	case pf.f.Admin2.ID != 0 && p.AccountID == pf.f.Admin2.ID:
		account = pf.f.Admin2
	}
	session, token, err := pf.a.StartAppSession(context.Background(), account, p, "Test "+deviceKind, deviceKind, "", "contract-test")
	if err != nil {
		t.Fatal(err)
	}
	pf.pa.tokens[name] = token
	pf.sessions[name] = session
	return session
}

// sendBearer is send with token as Authorization: Bearer and without the session cookie.
func sendBearer(t *testing.T, h http.Handler, method, path, token, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(CSRFHeader, CSRFHeaderValue)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		if v == "" {
			req.Header.Del(k)
			continue
		}
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// noCookie fails when the answer sets any cookie (concept 27 4.3).
func noCookie(t *testing.T, label string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if c := rec.Header().Values("Set-Cookie"); len(c) != 0 {
		t.Errorf("%s: answer sets cookies %v", label, c)
	}
}

// Every wrong pairing of token and carrier is refused; a bearer request never gets a cookie.
func TestBearerCarrierPairings(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	pf.newBearerSession(t, "tv", pf.f.P18, auth.AppDeviceTV)
	session := BasePath + "/auth/session"

	cookieWith := func(token string, extra map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, session, nil)
		req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for _, c := range []struct {
		name   string
		rec    *httptest.ResponseRecorder
		status int
		bearer bool // the answer must not set a cookie
	}{
		{"app token as bearer", sendBearer(t, h, http.MethodGet, session, "app", "", nil), http.StatusOK, true},
		{"tv token as bearer", sendBearer(t, h, http.MethodGet, session, "tv", "", nil), http.StatusOK, true},
		{"browser token as bearer", sendBearer(t, h, http.MethodGet, session, "p18", "", nil), http.StatusUnauthorized, true},
		{"unknown bearer", sendBearer(t, h, http.MethodGet, session, "no-such-token", "", nil), http.StatusUnauthorized, true},
		// Another scheme counts as no header: the browser way (code review M-1).
		{"other scheme", sendBearer(t, h, http.MethodGet, session, "", "", map[string]string{"Authorization": "Basic YTpi"}), http.StatusUnauthorized, false},
		{"cookie and another scheme", cookieWith("p18", map[string]string{"Authorization": "Basic YTpi"}), http.StatusOK, false},
		{"app token in the cookie", cookieWith("app", nil), http.StatusUnauthorized, false},
		{"tv token in the cookie", cookieWith("tv", nil), http.StatusUnauthorized, false},
		{"cookie and bearer together", cookieWith("p18", map[string]string{"Authorization": "Bearer app"}), http.StatusUnauthorized, true},
		{"cookie and bearer of the same app", cookieWith("app", map[string]string{"Authorization": "Bearer app"}), http.StatusUnauthorized, true},
		{"browser cookie alone", cookieWith("p18", nil), http.StatusOK, false},
	} {
		if c.rec.Code != c.status {
			t.Errorf("%s: status %d (%s), want %d", c.name, c.rec.Code, c.rec.Body.String(), c.status)
			continue
		}
		if c.status == http.StatusUnauthorized && problemCode(t, c.rec) != "unauthenticated" {
			t.Errorf("%s: code %s, want unauthenticated", c.name, problemCode(t, c.rec))
		}
		if c.bearer {
			noCookie(t, c.name, c.rec)
		}
		if c.status == http.StatusOK {
			var info gen.SessionInfo
			decode(t, c.rec, &info)
			if info.Account.Id != pf.f.Account.ID || info.Profile == nil || info.Profile.Id != pf.f.P18.ID {
				t.Errorf("%s: session %+v", c.name, info)
			}
		}
	}
	// A public operation with a refused bearer still answers, without a cookie.
	rec := sendBearer(t, h, http.MethodGet, BasePath+"/healthz", "p18", "", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("getHealth with a browser token as bearer: %d", rec.Code)
	}
	noCookie(t, "getHealth", rec)
	// The CSRF header stays mandatory for state-changing requests.
	rec = sendBearer(t, h, http.MethodPut, BasePath+"/items/"+pf.f.Rated(0).ID+"/watchlist", "app", "", map[string]string{CSRFHeader: ""})
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "csrf_header_missing" {
		t.Errorf("bearer write without the CSRF header: %d %s", rec.Code, rec.Body.String())
	}
	// The binary routes of the session middleware take the token as well.
	rec = sendBearer(t, h, http.MethodGet, BasePath+"/items/"+pf.f.Rated(0).ID+"/images/poster/w342", "tv", "", nil)
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Errorf("poster with a tv token: %d %s", rec.Code, rec.Body.String())
	}
	noCookie(t, "poster", rec)
}

// logout with a bearer revokes the session, without cookie and without Clear-Site-Data.
func TestBearerLogout(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	for _, kind := range []string{auth.AppDevicePhone, auth.AppDeviceTV} {
		pf.newBearerSession(t, kind, pf.f.P18, kind)
		rec := sendBearer(t, pf.h, http.MethodPost, BasePath+"/auth/logout", kind, "", nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s: logout %d %s", kind, rec.Code, rec.Body.String())
		}
		noCookie(t, kind+" logout", rec)
		if v := rec.Header().Get("Clear-Site-Data"); v != "" {
			t.Errorf("%s: logout sets Clear-Site-Data %q", kind, v)
		}
		rec = sendBearer(t, pf.h, http.MethodGet, BasePath+"/auth/session", kind, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: session after logout %d", kind, rec.Code)
		}
		noCookie(t, kind+" 401", rec)
	}
}

// tokenRoutes are the binary routes authorised by their own token (st, dt, rt) instead of the
// session: a bearer alone opens none of them. Their x-clients still hold: the token comes from an
// operation that checks it (startPlayback, getDownloadManifest, startRadioListen).
var tokenRoutes = map[string]bool{
	"getItemStream": true, "getPlaybackMaster": true, "getPlaybackMedia": true, "getPlaybackInit": true,
	"getPlaybackSegment": true, "getPlaybackSubtitle": true,
	"getDownloadFile": true, "getDownloadSubtitle": true, "getDownloadTrickplaySheet": true,
	"getRadioLivePlaylist": true, "getRadioLiveInit": true, "getRadioLiveSegment": true,
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// The test of concept 27 4.4: every operation of the contract, called with an app token and a TV
// token of the owner with the primary profile active. Where x-clients lacks the kind (and on every
// admin and owner operation), the answer is 403 client_not_allowed; where it names it, the bearer
// gets past the gate. With an ordinary account the test would prove nothing: the role check
// refuses first.
func TestBearerClientsCoverEveryOperation(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	paths, _ := doc["paths"].(map[string]any)
	type op struct {
		id, method, path, vis string
		clients               []string
		query                 string // the required query parameters, so the gate is what answers
	}
	var ops []op
	for path, item := range paths {
		methods, _ := item.(map[string]any)
		for _, m := range httpMethods {
			o, ok := methods[m].(map[string]any)
			if !ok {
				continue
			}
			id, _ := o["operationId"].(string)
			vis, _ := o["x-visibility"].(string)
			params := append(asList(methods["parameters"]), asList(o["parameters"])...)
			var query []string
			for _, raw := range params {
				p, _ := raw.(map[string]any)
				if ref, ok := p["$ref"].(string); ok {
					p, _ = resolve(doc, ref).(map[string]any)
				}
				if p["in"] == "query" && p["required"] == true {
					query = append(query, fmt.Sprint(p["name"])+"=1")
				}
			}
			ops = append(ops, op{id: id, method: strings.ToUpper(m), path: path, vis: vis, clients: clientList(o), query: strings.Join(query, "&")})
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].id < ops[j].id })
	pf := newProfileFixture(t)
	counts := map[string]map[bool]int{clientApp: {}, clientTV: {}}
	n := 0
	for _, o := range ops {
		for _, c := range []struct{ client, device string }{{clientApp, auth.AppDevicePhone}, {clientTV, auth.AppDeviceTV}} {
			n++
			name := "bearer-" + c.client + "-" + o.id
			pf.newBearerSession(t, name, pf.f.P18, c.device) // a fresh session: logout and the like end it
			body := ""
			if o.method != http.MethodGet && o.method != http.MethodDelete {
				body = "{}"
			}
			path := BasePath + pathParam.ReplaceAllString(o.path, "999999")
			if o.query != "" {
				path += "?" + o.query
			}
			rec := sendBearer(t, pf.h, o.method, path, name, body, nil)
			code := ""
			if rec.Code >= 400 && strings.Contains(rec.Header().Get("Content-Type"), "json") {
				code = problemCode(t, rec)
			}
			allowed := false
			for _, cl := range o.clients {
				allowed = allowed || cl == c.client
			}
			switch {
			case o.vis == "public":
				if code == "client_not_allowed" {
					t.Errorf("%s (public) with a %s token: client_not_allowed", o.id, c.client)
				}
			case tokenRoutes[o.id]:
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("%s with a %s token and no route token: %d %s, want 401", o.id, c.client, rec.Code, code)
				}
			case allowed:
				counts[c.client][true]++
				if code == "client_not_allowed" || code == "unauthenticated" {
					t.Errorf("%s (%s, x-clients %v) with a %s token: %d %s", o.id, o.vis, o.clients, c.client, rec.Code, code)
				}
			default:
				counts[c.client][false]++
				if rec.Code != http.StatusForbidden || code != "client_not_allowed" {
					t.Errorf("%s (%s, x-clients %v) with a %s token: %d %s, want 403 client_not_allowed", o.id, o.vis, o.clients, c.client, rec.Code, code)
				}
			}
			noCookie(t, o.id+" "+c.client, rec)
		}
	}
	t.Logf("%d calls; app: %d allowed, %d refused; tv: %d allowed, %d refused", n,
		counts[clientApp][true], counts[clientApp][false], counts[clientTV][true], counts[clientTV][false])
	if counts[clientTV][true] == 0 || counts[clientApp][false] == 0 {
		t.Error("the sweep reached no operation on one side of the gate")
	}
}

// A too old app gets 426 app_too_old everywhere but getHealth; an app at the least version, an
// unknown app name and a malformed header pass.
func TestAppTooOld(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	o := pf.opts
	o.MinClientVersions = map[string]string{"mobile": "9.0.0", "tv": "9.1.0"}
	h := NewHandler(o)
	pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	login := `{"email":"nobody@example.ch","password":"wrong-password","deviceName":"Pixel","kind":"phone"}`
	for _, c := range []struct {
		client string
		old    bool
	}{
		{"mobile/1.2.3", true}, {"mobile/8.99.99", true}, {"tv/9.0.9", true}, {"mobile/9.0.0-dev", false},
		{"mobile/9.0.0", false}, {"tv/9.1.0", false}, {"mobile/10.0.0", false}, {"desktop/0.0.1", false},
		{"mobile/1.2", false}, {"garbage", false}, {"", false},
	} {
		header := map[string]string{ClientHeader: c.client}
		for _, call := range []struct{ method, path, body string }{
			{http.MethodGet, BasePath + "/auth/session", ""},
			{http.MethodPost, BasePath + "/auth/app-login", login},
		} {
			rec := sendBearer(t, h, call.method, call.path, "app", call.body, header)
			tooOld := rec.Code == http.StatusUpgradeRequired && problemCode(t, rec) == "app_too_old"
			if tooOld != c.old {
				t.Errorf("%q %s: %d %s, want too old %v", c.client, call.path, rec.Code, rec.Body.String(), c.old)
			}
		}
		rec := sendBearer(t, h, http.MethodGet, BasePath+"/healthz", "", "", header)
		var health gen.Health
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &health) != nil || health.ContractVersion == "" {
			t.Errorf("%q getHealth: %d %s", c.client, rec.Code, rec.Body.String())
		}
	}
	// The default lets every app through.
	if rec := sendBearer(t, pf.h, http.MethodGet, BasePath+"/auth/session", "app", "", map[string]string{ClientHeader: "mobile/0.0.1"}); rec.Code != http.StatusOK {
		t.Errorf("default minimum: %d %s", rec.Code, rec.Body.String())
	}
}

// checkOfflineRevocations knows the device of an app by X-Slouchcat-Device and sets no cookie.
func TestRevocationsByDeviceHeader(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	device := "a1b2c3d4e5f60718293a4b5c6d7e8f90"
	gone := "01J9HEADRTMB00000000000000"
	if _, err := pf.f.DB.Write.Exec(`INSERT INTO download_tombstones (id, device_id, removed_at) VALUES (?, ?, 1)`, gone, device); err != nil {
		t.Fatal(err)
	}
	body := `{"ids":["` + gone + `","01J9KEPT000000000000000000"]}`
	for _, c := range []struct {
		name   string
		header map[string]string
		want   []string
	}{
		{"header", map[string]string{DeviceHeader: device}, []string{gone}},
		{"other device", map[string]string{DeviceHeader: "ffffffffffffffffffffffffffff0000"}, []string{}},
		{"malformed", map[string]string{DeviceHeader: "not-a-device"}, []string{}},
	} {
		rec := sendBearer(t, pf.h, http.MethodPost, BasePath+"/offline/revocations", "", body, c.header)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", c.name, rec.Code, rec.Body.String())
		}
		var out gen.RevocationList
		decode(t, rec, &out)
		if strings.Join(out.Revoked, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: revoked %v, want %v", c.name, out.Revoked, c.want)
		}
		noCookie(t, c.name, rec)
	}
}

// The administration's device list names the kind of the living session.
func TestDeviceListShowsSessionKind(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	app := pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	tv := pf.newBearerSession(t, "tv", pf.f.P12, auth.AppDeviceTV)
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/devices", "p18", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("listDevices: %d %s", rec.Code, rec.Body.String())
	}
	var list []gen.AdminDevice
	decode(t, rec, &list)
	kinds := map[string]string{}
	for _, d := range list {
		if d.SessionKind != nil {
			kinds[d.Id] = string(*d.SessionKind)
		}
	}
	if kinds[app.DeviceID] != model.SessionApp || kinds[tv.DeviceID] != model.SessionDevice || kinds[pf.sessions["p18"].DeviceID] != model.SessionBrowser {
		t.Errorf("session kinds %v", kinds)
	}
}

// appLogin over HTTP: the answer carries token, device id and session and sets no cookie; the token
// works as bearer and is never logged; a second sign-in with the device id keeps it.
func TestAppLoginEndToEnd(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil), http.StatusCreated)
	login := func(deviceID string, header map[string]string) gen.AppLogin {
		t.Helper()
		body := `{"email":"marco@example.ch","password":"` + authTestPassword + `","deviceName":"Pixel 9","kind":"phone"`
		if deviceID != "" {
			body += `,"deviceId":"` + deviceID + `"`
		}
		rec := sendBearer(t, e.h, http.MethodPost, "/api/v1/auth/app-login", "", body+"}", header)
		wantStatus(t, rec, http.StatusOK)
		noCookie(t, "appLogin", rec)
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control %q, want no-store", cc)
		}
		var out gen.AppLogin
		decode(t, rec, &out)
		if len(out.Token) != 43 || !deviceIDPattern.MatchString(out.DeviceId) || string(out.Session.Account.Email) != "marco@example.ch" {
			t.Fatalf("appLogin answered %+v", out)
		}
		return out
	}
	first := login("", nil)
	rec := sendBearer(t, e.h, http.MethodGet, "/api/v1/auth/session", first.Token, "", nil)
	wantStatus(t, rec, http.StatusOK)
	noCookie(t, "getSession", rec)
	if again := login(first.DeviceId, nil); again.DeviceId != first.DeviceId || again.Token == first.Token {
		t.Errorf("second sign-in with the device id: %s, want %s and a new token", again.DeviceId, first.DeviceId)
	}
	if byHeader := login("", map[string]string{DeviceHeader: first.DeviceId}); byHeader.DeviceId != first.DeviceId {
		t.Errorf("device id from the header: %s, want %s", byHeader.DeviceId, first.DeviceId)
	}
	rec = sendBearer(t, e.h, http.MethodPost, "/api/v1/auth/app-login", "",
		`{"email":"marco@example.ch","password":"wrong-password","deviceName":"Pixel 9","kind":"phone"}`, nil)
	wantStatus(t, rec, http.StatusUnauthorized)
	noCookie(t, "refused appLogin", rec)
	rec = sendBearer(t, e.h, http.MethodPost, "/api/v1/auth/app-login", "",
		`{"email":"marco@example.ch","password":"`+authTestPassword+`","deviceName":"Pixel 9","kind":"desktop"}`, nil)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"kind"`) {
		t.Errorf("unknown kind: %s", rec.Body.String())
	}
	if out := e.logs.String(); strings.Contains(out, first.Token) || strings.Contains(out, authTestPassword) || strings.Contains(out, first.DeviceId) {
		t.Errorf("secret in logs: %s", out)
	}
}
