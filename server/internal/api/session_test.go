package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
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
		return model.Session{ID: 1, Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}, Profile: model.Profile{ID: 1, Name: "Marco", MaxRating: 18, Language: "de"}}, false, nil
	case "guest":
		return model.Session{ID: 2, Account: model.Account{ID: 2, Email: "g@b.c", Role: "guest"}, Profile: model.Profile{ID: 2, Name: "Gast", MaxRating: 18, Language: "en"}}, false, nil
	case "sliding":
		// Resolve moved the expiry of this session.
		return model.Session{ID: 3, Account: model.Account{ID: 1, Email: "a@b.c", Role: "owner"}, Profile: model.Profile{ID: 1, Name: "Marco", MaxRating: 18, Language: "de"},
			ExpiresAt: slidingExpiry}, true, nil
	}
	return model.Session{}, false, model.ErrNotFound
}

var slidingExpiry = time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)

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
		if v != "public" && v != "scoped" && v != "admin" {
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
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != SessionCookie || cookies[0].Value != "sliding" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %+v, want the refreshed session cookie", cookies)
	}
	if !cookies[0].Expires.Equal(slidingExpiry) || cookies[0].MaxAge <= 0 {
		t.Errorf("cookie expiry = %v (max-age %d), want %v", cookies[0].Expires, cookies[0].MaxAge, slidingExpiry)
	}

	rec = request(t, h, http.MethodGet, "/api/v1/auth/session", "owner", false)
	if n := len(rec.Result().Cookies()); n != 0 {
		t.Errorf("unchanged session set %d cookies", n)
	}

	// Logout of an extended session leaves only the clearing cookie.
	rec = request(t, h, http.MethodPost, "/api/v1/auth/logout", "sliding", true)
	if cookies := rec.Result().Cookies(); len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Errorf("logout cookies = %+v, want only the clearing one", cookies)
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct{ remote, xff, want string }{
		{"192.0.2.1:1234", "", "192.0.2.1"},
		{"192.0.2.1:1234", "203.0.113.5, 10.0.0.1", "203.0.113.5"},
		{"[2001:db8::1]:443", "", "2001:db8::1"},
		{"192.0.2.1:1234", "not-an-ip", "192.0.2.1"},
		{"192.0.2.1:1234", "::ffff:198.51.100.2", "198.51.100.2"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = tc.remote
		if tc.xff != "" {
			req.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := clientIP(req); got != tc.want {
			t.Errorf("clientIP(%q, %q) = %q, want %q", tc.remote, tc.xff, got, tc.want)
		}
	}
}
