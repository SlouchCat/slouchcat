package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/config"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// fakeAuth resolves two fixed tokens: "owner" and "guest".
type fakeAuth struct{}

func (fakeAuth) SetupRequired(context.Context) (bool, error) { return false, nil }
func (fakeAuth) Login(context.Context, string, string, string) (model.Session, string, error) {
	return model.Session{}, "", model.ErrInvalidCredentials
}
func (fakeAuth) Logout(context.Context, int64) error { return nil }
func (fakeAuth) Resolve(_ context.Context, token string) (model.Session, bool, error) {
	switch token {
	case "owner":
		return model.Session{ID: 1, Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}, Profile: model.Profile{ID: 1, Name: "Marco", MaxRating: 18, Language: "de", IsPrimary: true}}, false, nil
	case "nopro":
		// Signed in as the owner, profile not chosen yet.
		return model.Session{ID: 4, Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}}, false, nil
	case "kid":
		// The owner's account with a non-primary profile.
		return model.Session{ID: 5, Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}, Profile: model.Profile{ID: 3, Name: "Kind", MaxRating: 6, Language: "de"}}, false, nil
	case "guest":
		return model.Session{ID: 2, Account: model.Account{ID: 2, Email: "g@b.c", Role: "guest"}, Profile: model.Profile{ID: 2, Name: "Gast", MaxRating: 18, Language: "en"}}, false, nil
	case "sliding":
		// Resolve moved the expiry of this session.
		return model.Session{ID: 3, Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}, Profile: model.Profile{ID: 1, Name: "Marco", MaxRating: 18, Language: "de", IsPrimary: true},
			ExpiresAt: slidingExpiry}, true, nil
	case "device", "device-sliding":
		// A session started on sessionDevice; the second one Resolve extended.
		return model.Session{ID: 6, Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}, Profile: model.Profile{ID: 1, Name: "Marco", MaxRating: 18, Language: "de", IsPrimary: true},
			DeviceID: sessionDevice, ExpiresAt: slidingExpiry}, token == "device-sliding", nil
	}
	return model.Session{}, false, model.ErrNotFound
}

var slidingExpiry = time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)

// sessionDevice is the device of the fake sessions "device" and "device-sliding".
var sessionDevice = strings.Repeat("d", 32)

func newSessionTestHandler() http.Handler {
	return NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Web: http.NotFoundHandler(), Auth: fakeAuth{},
	})
}

func request(t *testing.T, h http.Handler, method, path, token string, csrf bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	}
	if csrf {
		req.Header.Set(CSRFHeader, CSRFHeaderValue)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p gen.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("status %d, body %q is no problem: %v", rec.Code, rec.Body.String(), err)
	}
	return p.Code
}

// The session middleware reads x-visibility from the embedded contract per route: public passes
// without a session, scoped needs one, admin needs the owner, and the binary routes mounted by
// hand are covered as well. Stubbed handlers answer 500 behind the gate, which proves the gate
// opened.
func TestAuthorizeByVisibility(t *testing.T) {
	h := newSessionTestHandler()
	cases := []struct {
		name          string
		method, path  string
		token         string
		csrf          bool
		wantStatus    int
		wantCode      string
		allowInternal bool
	}{
		{name: "public without session", method: "GET", path: "/api/v1/healthz", wantStatus: 200},
		{name: "scoped without session", method: "GET", path: "/api/v1/auth/session", wantStatus: 401, wantCode: "unauthenticated"},
		{name: "scoped with bad token", method: "GET", path: "/api/v1/home", token: "nope", wantStatus: 401, wantCode: "unauthenticated"},
		{name: "scoped with session reaches handler", method: "GET", path: "/api/v1/home", token: "owner", wantStatus: 500, allowInternal: true},
		{name: "account without profile passes", method: "GET", path: "/api/v1/auth/session", token: "nopro", wantStatus: 200},
		{name: "account profile list reaches handler", method: "GET", path: "/api/v1/auth/profiles", token: "nopro", wantStatus: 500, allowInternal: true},
		{name: "scoped without profile", method: "GET", path: "/api/v1/home", token: "nopro", wantStatus: 403, wantCode: "profile_required"},
		{name: "image without profile", method: "GET", path: "/api/v1/items/01ARZ3NDEKTSV4RRFFQ69G5FAV/images/poster/w400", token: "nopro", wantStatus: 403, wantCode: "profile_required"},
		{name: "admin with non-primary profile", method: "GET", path: "/api/v1/admin/libraries", token: "kid", wantStatus: 403, wantCode: "forbidden"},
		{name: "admin without profile", method: "GET", path: "/api/v1/admin/libraries", token: "nopro", wantStatus: 403, wantCode: "forbidden"},
		{name: "admin as guest", method: "GET", path: "/api/v1/admin/libraries", token: "guest", wantStatus: 403, wantCode: "forbidden"},
		{name: "admin as owner reaches handler", method: "GET", path: "/api/v1/admin/libraries", token: "owner", wantStatus: 500, allowInternal: true},
		{name: "post without csrf header", method: "POST", path: "/api/v1/auth/logout", token: "owner", wantStatus: 403, wantCode: "csrf_header_missing"},
		{name: "public post without csrf header", method: "POST", path: "/api/v1/auth/login", wantStatus: 403, wantCode: "csrf_header_missing"},
		{name: "post with csrf header reaches handler", method: "POST", path: "/api/v1/auth/logout", token: "owner", csrf: true, wantStatus: 204},
		{name: "image route without session", method: "GET", path: "/api/v1/items/01ARZ3NDEKTSV4RRFFQ69G5FAV/images/poster/w500", wantStatus: 401, wantCode: "unauthenticated"},
		{name: "stream route without session", method: "GET", path: "/api/v1/items/01ARZ3NDEKTSV4RRFFQ69G5FAV/stream", wantStatus: 401, wantCode: "unauthenticated"},
		{name: "spike is public but disabled", method: "GET", path: "/api/v1/spike/fmp4", wantStatus: 404, wantCode: "not_found"},
		{name: "unknown route stays 404", method: "POST", path: "/api/v1/nothing", wantStatus: 404, wantCode: "not_found"},
		{name: "wrong method stays 405", method: "POST", path: "/api/v1/healthz", wantStatus: 405, wantCode: "method_not_allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, h, tc.method, tc.path, tc.token, tc.csrf)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode != "" && problemCode(t, rec) != tc.wantCode {
				t.Errorf("code = %s, want %s", problemCode(t, rec), tc.wantCode)
			}
			if tc.allowInternal && problemCode(t, rec) != "internal_error" {
				t.Errorf("expected the stub behind the gate, got %s", rec.Body.String())
			}
		})
	}
}

// Every operation of the contract is known to the middleware with a usable marker.
func TestEveryRouteHasVisibility(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	vis := visibilityOf(spec, BasePath)
	if len(vis) < 15 {
		t.Fatalf("only %d routes with x-visibility", len(vis))
	}
	for route, v := range vis {
		if v != "public" && v != "account" && v != "scoped" && v != "admin" {
			t.Errorf("%s: unusable x-visibility %q", route, v)
		}
	}
}

func TestMapError(t *testing.T) {
	h := newSessionTestHandler()
	p := problems{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	_ = h
	cases := map[error]struct {
		status int
		code   string
	}{
		model.ErrNotFound:                                        {404, "not_found"},
		model.ErrInvalidCredentials:                              {401, "invalid_credentials"},
		model.ErrSetupDone:                                       {409, "setup_done"},
		model.ErrTooManyAttempts:                                 {429, "too_many_attempts"},
		model.ErrLibraryPathInvalid:                              {422, "library_path_invalid"},
		model.ErrTMDBKeyInvalid:                                  {422, "tmdb_key_invalid"},
		model.ErrTMDBUnreachable:                                 {422, "tmdb_unreachable"},
		model.ErrPinRequired:                                     {403, "pin_required"},
		model.ErrPinInvalid:                                      {403, "pin_invalid"},
		&model.PinLockedError{RetryAfter: 90 * time.Second}:      {429, "pin_locked"},
		model.ErrPrimaryProfileRequired:                          {403, "primary_profile_required"},
		model.ErrPrimaryProfile:                                  {409, "primary_profile"},
		model.ErrInvitationGone:                                  {410, "invitation_gone"},
		(&model.ValidationError{}).Add("email", "invalid_email"): {422, "validation_failed"},
		errors.New("boom"):                                       {500, "internal_error"},
	}
	for err, want := range cases {
		rec := httptest.NewRecorder()
		p.mapError(rec, httptest.NewRequest(http.MethodGet, "/x", nil), err)
		if rec.Code != want.status || problemCode(t, rec) != want.code {
			t.Errorf("%v: got %d %s, want %d %s", err, rec.Code, problemCode(t, rec), want.status, want.code)
		}
		if want.code == "validation_failed" && !strings.Contains(rec.Body.String(), `"field":"email"`) {
			t.Errorf("validation problem lacks field errors: %s", rec.Body.String())
		}
	}
}

func TestSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	now := time.Now()
	setSessionCookie(rec, req, "tok", sessionExpiry(now))
	c := rec.Result().Cookies()[0]
	if c.Name != SessionCookie || c.Value != "tok" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure {
		t.Errorf("plain-http cookie = %+v", c)
	}
	if got := c.Expires.Sub(now); got < 29*24*time.Hour || got > 31*24*time.Hour {
		t.Errorf("expiry = %v, want about 30 days", got)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	setSessionCookie(rec, req, "tok", sessionExpiry(now))
	if !rec.Result().Cookies()[0].Secure {
		t.Error("cookie behind an HTTPS proxy must be Secure")
	}
	rec = httptest.NewRecorder()
	clearSessionCookie(rec, req)
	if c := rec.Result().Cookies()[0]; c.MaxAge != -1 {
		t.Errorf("clear cookie MaxAge = %d", c.MaxAge)
	}
	info := toSessionInfo(model.Session{Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}, Profile: model.Profile{ID: 2, Name: "M", MaxRating: 18, Language: "de"}})
	if info.Profile.MaxRating != 18 || info.Profile.Language != gen.De || string(info.Account.Email) != "a@b.c" {
		t.Errorf("toSessionInfo = %+v", info)
	}
}

func TestContextHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	ctx := context.WithValue(context.WithValue(context.Background(), requestKey{}, req), sessionKey{}, model.Session{ID: 7})
	if requestFrom(ctx) != req {
		t.Error("requestFrom did not return the stored request")
	}
	if s, ok := sessionFrom(ctx); !ok || s.ID != 7 {
		t.Errorf("sessionFrom = %+v, %v", s, ok)
	}
	if _, ok := sessionFrom(context.Background()); ok {
		t.Error("sessionFrom must report no session on an empty context")
	}
}

// A request whose session Resolve extended carries a fresh cookie with the new expiry; a session
// that was not extended gets no cookie.
func TestAuthorizeReissuesExtendedSessionCookie(t *testing.T) {
	h := newSessionTestHandler()
	rec := request(t, h, http.MethodGet, "/api/v1/auth/session", "sliding", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	cookies := cookiesNamed(rec, SessionCookie)
	if len(cookies) != 1 || cookies[0].Value != "sliding" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %+v, want the refreshed session cookie", cookies)
	}
	if !cookies[0].Expires.Equal(slidingExpiry) || cookies[0].MaxAge <= 0 {
		t.Errorf("cookie expiry = %v (max-age %d), want %v", cookies[0].Expires, cookies[0].MaxAge, slidingExpiry)
	}

	rec = request(t, h, http.MethodGet, "/api/v1/auth/session", "owner", false)
	if n := len(cookiesNamed(rec, SessionCookie)); n != 0 {
		t.Errorf("unchanged session set %d session cookies", n)
	}

	// Logout of an extended session leaves only the clearing cookie.
	rec = request(t, h, http.MethodPost, "/api/v1/auth/logout", "sliding", true)
	if cookies := cookiesNamed(rec, SessionCookie); len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Errorf("logout cookies = %+v, want only the clearing one", cookies)
	}
}

// M5 review W6: the device cookie follows the session's device and lives as long as the session.
// A cookie that expired or was replaced while the sliding session lived on is rewritten to the
// session's device (the revocation list of checkOfflineRevocations is read by the cookie, K-7),
// and an extended session renews it with a fresh MaxAge.
func TestDeviceCookieFollowsTheSession(t *testing.T) {
	h := newSessionTestHandler()
	get := func(token, cookie string) []*http.Cookie {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
		}
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: DeviceCookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return cookiesNamed(rec, DeviceCookie)
	}
	renewed := func(name string, got []*http.Cookie, want string) {
		t.Helper()
		if len(got) != 1 || got[0].Value != want || got[0].MaxAge != deviceCookieDays*24*3600 || !got[0].HttpOnly || got[0].Path != "/" {
			t.Errorf("%s: device cookies %+v, want %s for %d days", name, got, want, deviceCookieDays)
		}
	}
	other := strings.Repeat("c", 32)
	renewed("another cookie", get("device", other), sessionDevice)
	renewed("no cookie", get("device", ""), sessionDevice)
	if got := get("device", sessionDevice); len(got) != 0 {
		t.Errorf("matching cookie re-issued: %+v", got)
	}
	renewed("extended session", get("device-sliding", sessionDevice), sessionDevice)
	// Without a session the cookie stays, and a request without one gets a fresh id.
	if got := get("", other); len(got) != 0 {
		t.Errorf("cookie without session re-issued: %+v", got)
	}
	if got := get("", ""); len(got) != 1 || !deviceIDPattern.MatchString(got[0].Value) || got[0].Value == sessionDevice {
		t.Errorf("fresh device cookie = %+v", got)
	}
	// A session from before devices (no device id) keeps the cookie.
	if got := get("owner", other); len(got) != 0 {
		t.Errorf("session without device re-issued the cookie: %+v", got)
	}
}

func cookiesNamed(rec *httptest.ResponseRecorder, name string) []*http.Cookie {
	var out []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

func TestClientIP(t *testing.T) {
	private, err := config.ParseTrustedProxies("private")
	if err != nil {
		t.Fatal(err)
	}
	cidr, err := config.ParseTrustedProxies("203.0.113.0/24, 2001:db8::1")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		trusted      []netip.Prefix
		remote, xff  string
		want         string
		xffHeaderTwo string
	}{
		{name: "no proxies: header ignored", remote: "192.0.2.1:1234", xff: "203.0.113.5, 10.0.0.1", want: "192.0.2.1"},
		{name: "no proxies: plain address", remote: "192.0.2.1:1234", want: "192.0.2.1"},
		{name: "ipv6 remote", remote: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "private proxy, client in header", trusted: private, remote: "10.0.0.2:1234", xff: "198.51.100.7", want: "198.51.100.7"},
		{name: "private proxy, chain: nearest untrusted hop", trusted: private, remote: "172.16.0.9:1", xff: "6.6.6.6, 198.51.100.7, 10.0.0.1", want: "198.51.100.7"},
		{name: "private proxy, forged left entries are skipped", trusted: private, remote: "192.168.1.2:1", xff: "1.2.3.4", xffHeaderTwo: "198.51.100.7", want: "198.51.100.7"},
		{name: "private proxy, loopback", trusted: private, remote: "127.0.0.1:5", xff: "198.51.100.9", want: "198.51.100.9"},
		{name: "private proxy, unique local ipv6", trusted: private, remote: "[fd00::5]:5", xff: "2001:db8::77", want: "2001:db8::77"},
		{name: "untrusted peer: header ignored", trusted: private, remote: "192.0.2.1:1234", xff: "198.51.100.7", want: "192.0.2.1"},
		{name: "malformed chain is not believed", trusted: private, remote: "10.0.0.2:1", xff: "not-an-ip", want: "10.0.0.2"},
		{name: "mapped ipv4", trusted: private, remote: "10.0.0.2:1", xff: "::ffff:198.51.100.2", want: "198.51.100.2"},
		{name: "only trusted hops: first entry", trusted: private, remote: "10.0.0.2:1", xff: "10.1.1.1, 10.0.0.3", want: "10.1.1.1"},
		{name: "cidr list and single address", trusted: cidr, remote: "203.0.113.9:1", xff: "198.51.100.1", want: "198.51.100.1"},
		{name: "cidr list: outside", trusted: cidr, remote: "10.0.0.9:1", xff: "198.51.100.1", want: "10.0.0.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/x", nil)
			req.RemoteAddr = tc.remote
			if tc.xff != "" {
				req.Header.Add("X-Forwarded-For", tc.xff)
			}
			if tc.xffHeaderTwo != "" {
				req.Header.Add("X-Forwarded-For", tc.xffHeaderTwo)
			}
			if got := clientIP(req, tc.trusted); got != tc.want {
				t.Errorf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}
