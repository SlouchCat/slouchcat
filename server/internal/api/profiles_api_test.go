package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// browser is a minimal cookie jar around the handler: it keeps the cookies the server sets, like a
// browser, so a test can follow the session and the device cookie across requests.
type browser struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
	header  map[string]string
	last    *httptest.ResponseRecorder
}

func newBrowser(t *testing.T, h http.Handler) *browser {
	return &browser{t: t, h: h, cookies: map[string]*http.Cookie{}, header: map[string]string{}}
}

func (b *browser) do(method, path string, body any) *httptest.ResponseRecorder {
	b.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			b.t.Fatal(err)
		}
		rd = bytes.NewReader(data)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(CSRFHeader, CSRFHeaderValue)
	for k, v := range b.header {
		req.Header.Set(k, v)
	}
	for _, c := range b.cookies {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	b.last = rec
	return rec
}

var deviceID32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

func sessionOf(t *testing.T, rec *httptest.ResponseRecorder) gen.SessionInfo {
	t.Helper()
	var info gen.SessionInfo
	decode(t, rec, &info)
	return info
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) gen.Problem {
	t.Helper()
	var p gen.Problem
	decode(t, rec, &p)
	return p
}

// The profile picker flow over HTTP: login without a profile, scoped operations refuse (profile_required),
// the PIN lock counts per profile and device, selecting a profile clears the browser cache, a
// non-primary profile cannot administrate, and the device cookie is issued once.
func TestProfilePickerFlow(t *testing.T) {
	e := newAuthEnv(t)
	owner := newBrowser(t, e.h)

	// The wizard starts the session with the only profile.
	rec := owner.do(http.MethodPost, "/api/v1/setup", setupBody())
	wantStatus(t, rec, http.StatusCreated)
	info := sessionOf(t, rec)
	if info.Profile == nil || !info.Profile.IsPrimary || info.Profile.HasPin || info.Account.MaxRating != 18 || !info.Account.AllLibraries {
		t.Fatalf("setup session = %+v", info)
	}
	primaryID := info.Profile.Id

	// Device cookie: set once with the attributes of the contract, not re-issued while it is sent.
	dev := owner.cookies[DeviceCookie]
	if dev == nil || !deviceID32.MatchString(dev.Value) || !dev.HttpOnly || dev.SameSite != http.SameSiteLaxMode || dev.MaxAge != 400*24*3600 || dev.Path != "/" {
		t.Fatalf("device cookie = %+v", dev)
	}
	for range 2 {
		rec = owner.do(http.MethodGet, "/api/v1/auth/session", nil)
		wantStatus(t, rec, http.StatusOK)
		if got := cookiesNamed(rec, DeviceCookie); len(got) != 0 {
			t.Errorf("device cookie re-issued: %+v", got)
		}
	}
	// A malformed device cookie is replaced; the Secure flag follows the proxy header.
	tampered := newBrowser(t, e.h)
	tampered.cookies[DeviceCookie] = &http.Cookie{Name: DeviceCookie, Value: "not-a-device"}
	tampered.header["X-Forwarded-Proto"] = "https"
	rec = tampered.do(http.MethodGet, "/api/v1/setup", nil)
	got := cookiesNamed(rec, DeviceCookie)
	if len(got) != 1 || !deviceID32.MatchString(got[0].Value) || !got[0].Secure {
		t.Errorf("replaced device cookie = %+v", got)
	}

	// A second profile with a PIN.
	rec = owner.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Kind", "maxRating": 6, "language": "en", "accent": "teal", "pin": "4711"})
	wantStatus(t, rec, http.StatusCreated)
	var kid gen.Profile
	decode(t, rec, &kid)
	if !kid.HasPin || kid.IsPrimary || kid.MaxRating != 6 || kid.Language != gen.En || kid.Accent == nil || *kid.Accent != gen.Teal {
		t.Fatalf("created profile = %+v", kid)
	}
	if strings.Contains(rec.Body.String(), "4711") {
		t.Error("the PIN is in the response")
	}

	// Logout clears the browser cache; the next login lands in the picker (two profiles).
	rec = owner.do(http.MethodPost, "/api/v1/auth/logout", nil)
	wantStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Clear-Site-Data"); got != `"cache"` {
		t.Errorf("logout Clear-Site-Data = %q", got)
	}
	rec = owner.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "marco@example.ch", "password": authTestPassword})
	wantStatus(t, rec, http.StatusOK)
	if info := sessionOf(t, rec); info.Profile != nil || info.Account.Role != gen.AccountRoleOwner {
		t.Fatalf("login with two profiles = %+v, want no active profile", info)
	}
	if strings.Contains(rec.Body.String(), `"profile"`) {
		t.Errorf("login response mentions a profile: %s", rec.Body.String())
	}
	wantStatus(t, owner.do(http.MethodGet, "/api/v1/auth/session", nil), http.StatusOK)

	// Picker: both profiles with whether a PIN is set, nothing else.
	rec = owner.do(http.MethodGet, "/api/v1/auth/profiles", nil)
	var picker []gen.ProfileSummary
	decode(t, rec, &picker)
	if len(picker) != 2 || !picker[0].IsPrimary || picker[1].Name != "Kind" || !picker[1].HasPin || picker[0].HasPin {
		t.Fatalf("picker = %s", rec.Body.String())
	}

	// Scoped and admin operations need a profile first.
	rec = owner.do(http.MethodGet, "/api/v1/home", nil)
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "profile_required" {
		t.Errorf("home without a profile: %d %s", rec.Code, rec.Body.String())
	}
	rec = owner.do(http.MethodGet, "/api/v1/admin/libraries", nil)
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "forbidden" {
		t.Errorf("admin without a profile: %d %s", rec.Code, rec.Body.String())
	}

	// PIN: missing, wrong, wrong, third wrong locks for 30 s on this device.
	selectKid := func(b *browser, pin any) *httptest.ResponseRecorder {
		var body any
		if pin != nil {
			body = map[string]any{"pin": pin}
		}
		return b.do(http.MethodPost, fmt.Sprintf("/api/v1/auth/profiles/%d/select", kid.Id), body)
	}
	if rec := selectKid(owner, nil); rec.Code != http.StatusForbidden || problemCode(t, rec) != "pin_required" {
		t.Fatalf("no PIN: %d %s", rec.Code, rec.Body.String())
	}
	if rec := selectKid(owner, "12"); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("malformed PIN: %d %s", rec.Code, rec.Body.String())
	}
	for range 2 {
		if rec := selectKid(owner, "0000"); rec.Code != http.StatusForbidden || problemCode(t, rec) != "pin_invalid" {
			t.Fatalf("wrong PIN: %d %s", rec.Code, rec.Body.String())
		}
	}
	rec = selectKid(owner, "0000")
	if rec.Code != http.StatusTooManyRequests || problemCode(t, rec) != "pin_locked" || rec.Header().Get("Retry-After") != "30" ||
		problemOf(t, rec).RetryAfterSeconds == nil || *problemOf(t, rec).RetryAfterSeconds != 30 {
		t.Fatalf("third wrong PIN: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	if rec.Header().Get("Clear-Site-Data") != "" {
		t.Error("a refused selection must not clear the cache")
	}
	// While locked even the right PIN is refused ...
	if rec := selectKid(owner, "4711"); rec.Code != http.StatusTooManyRequests || problemCode(t, rec) != "pin_locked" {
		t.Errorf("right PIN while locked: %d %s", rec.Code, rec.Body.String())
	}
	// ... but another device is not affected.
	other := newBrowser(t, e.h)
	other.cookies[SessionCookie] = owner.cookies[SessionCookie]
	rec = selectKid(other, "4711")
	wantStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Clear-Site-Data"); got != `"cache"` {
		t.Errorf("selectProfile Clear-Site-Data = %q, want \"cache\"", got)
	}
	if info := sessionOf(t, rec); info.Profile == nil || info.Profile.Id != kid.Id || info.Profile.MaxRating != 6 {
		t.Fatalf("selected profile = %s", rec.Body.String())
	}
	if len(cookiesNamed(rec, SessionCookie)) != 0 {
		t.Error("the profile switch must not re-issue the session cookie")
	}

	// As the child profile: scoped works, profile management and administration do not.
	if rec := other.do(http.MethodGet, "/api/v1/auth/session", nil); sessionOf(t, rec).Profile == nil || sessionOf(t, rec).Profile.Id != kid.Id {
		t.Errorf("getSession after the switch = %s", rec.Body.String())
	}
	rec = other.do(http.MethodGet, "/api/v1/admin/libraries", nil)
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "forbidden" {
		t.Errorf("admin as the child profile: %d %s", rec.Code, rec.Body.String())
	}
	rec = other.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Sneaky"})
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "primary_profile_required" {
		t.Errorf("createProfile as the child profile: %d %s", rec.Code, rec.Body.String())
	}
	rec = other.do(http.MethodDelete, fmt.Sprintf("/api/v1/profiles/%d/pin", kid.Id), nil)
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "primary_profile_required" {
		t.Errorf("removeProfilePin as the child profile: %d %s", rec.Code, rec.Body.String())
	}

	// Back to the primary profile (no PIN) and manage: the primary profile cannot be deleted, a
	// profile of another account is a 404, an unknown id too.
	rec = other.do(http.MethodPost, fmt.Sprintf("/api/v1/auth/profiles/%d/select", primaryID), nil)
	wantStatus(t, rec, http.StatusOK)
	rec = other.do(http.MethodDelete, fmt.Sprintf("/api/v1/profiles/%d", primaryID), nil)
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "primary_profile" {
		t.Errorf("delete primary: %d %s", rec.Code, rec.Body.String())
	}
	if rec := other.do(http.MethodPost, "/api/v1/auth/profiles/9999/select", nil); rec.Code != http.StatusNotFound {
		t.Errorf("select unknown: %d", rec.Code)
	}
	rec = other.do(http.MethodPut, fmt.Sprintf("/api/v1/profiles/%d/pin", kid.Id), map[string]any{"pin": "123"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("short PIN: %d %s", rec.Code, rec.Body.String())
	}
	wantStatus(t, other.do(http.MethodDelete, fmt.Sprintf("/api/v1/profiles/%d/pin", kid.Id), nil), http.StatusNoContent)
	// Without a PIN the locked device passes: removing the PIN also cleared the lock counters.
	wantStatus(t, selectKid(owner, nil), http.StatusOK)

	// A profile that is deleted while a session has it active sends that session back to the
	// picker (the deleting session is another one, on the primary profile).
	manager := newBrowser(t, e.h)
	wantStatus(t, manager.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "marco@example.ch", "password": authTestPassword}), http.StatusOK)
	wantStatus(t, manager.do(http.MethodPost, fmt.Sprintf("/api/v1/auth/profiles/%d/select", primaryID), nil), http.StatusOK)
	if info := sessionOf(t, owner.do(http.MethodGet, "/api/v1/auth/session", nil)); info.Profile == nil || info.Profile.Id != kid.Id {
		t.Fatalf("the first session must be on the child profile: %+v", info)
	}
	wantStatus(t, manager.do(http.MethodDelete, fmt.Sprintf("/api/v1/profiles/%d", kid.Id), nil), http.StatusNoContent)
	rec = owner.do(http.MethodGet, "/api/v1/auth/session", nil)
	if info := sessionOf(t, rec); rec.Code != http.StatusOK || info.Profile != nil {
		t.Errorf("a session whose profile was deleted must fall back to the picker: %s", rec.Body.String())
	}
	if rec := owner.do(http.MethodGet, "/api/v1/home", nil); rec.Code != http.StatusForbidden || problemCode(t, rec) != "profile_required" {
		t.Errorf("home after the profile was deleted: %d %s", rec.Code, rec.Body.String())
	}
}

// The profile limits: at most ten profiles, accents and languages from the fixed lists, names
// bounded.
func TestProfileValidation(t *testing.T) {
	e := newAuthEnv(t)
	b := newBrowser(t, e.h)
	wantStatus(t, b.do(http.MethodPost, "/api/v1/setup", setupBody()), http.StatusCreated)

	for _, tc := range []struct {
		name string
		body map[string]any
		code string
	}{
		{"empty name", map[string]any{"name": " "}, "required"},
		{"long name", map[string]any{"name": strings.Repeat("x", 61)}, "too_long"},
		{"accent", map[string]any{"name": "A", "accent": "pink"}, "invalid_value"},
		{"unknown library", map[string]any{"name": "A", "allLibraries": false, "libraryIds": []int{42}}, "unknown_library"},
	} {
		rec := b.do(http.MethodPost, "/api/v1/profiles", tc.body)
		// The API schema is enforced by the generated code only for types; the service decides the rest.
		if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d (%s)", tc.name, rec.Code, rec.Body.String())
			continue
		}
		if rec.Code == http.StatusUnprocessableEntity && !strings.Contains(rec.Body.String(), tc.code) {
			t.Errorf("%s: body %s lacks %q", tc.name, rec.Body.String(), tc.code)
		}
	}
	for i := 2; i <= 10; i++ {
		wantStatus(t, b.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": fmt.Sprintf("P%d", i)}), http.StatusCreated)
	}
	rec := b.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "eleven"})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "too_many_profiles") {
		t.Errorf("11th profile: %d %s", rec.Code, rec.Body.String())
	}
	var list []gen.Profile
	decode(t, b.do(http.MethodGet, "/api/v1/profiles", nil), &list)
	if len(list) != 10 || !list[0].IsPrimary {
		t.Errorf("profiles = %d, first primary %v", len(list), len(list) > 0 && list[0].IsPrimary)
	}
	if !slices.IsSortedFunc(list[1:], func(a, b gen.Profile) int { return int(a.Id - b.Id) }) {
		t.Error("profiles after the primary one are not in creation order")
	}
}

// The invitation flow over HTTP: preview, accept (one-time), the guest's limits, and a revoked,
// expired and used link answering 410.
func TestInvitationFlow(t *testing.T) {
	e := newAuthEnv(t)
	owner := newBrowser(t, e.h)
	wantStatus(t, owner.do(http.MethodPost, "/api/v1/setup", setupBody()), http.StatusCreated)

	rec := owner.do(http.MethodPost, "/api/v1/admin/invitations", map[string]any{"note": "Oma", "maxRating": 12, "allLibraries": true})
	wantStatus(t, rec, http.StatusCreated)
	var inv gen.InvitationCreated
	decode(t, rec, &inv)
	if inv.State != gen.Open || inv.Note == nil || *inv.Note != "Oma" || !strings.HasPrefix(inv.Url, "/invite/") || strings.TrimPrefix(inv.Url, "/invite/") != inv.Token || len(inv.Token) != 43 {
		t.Fatalf("created invitation = %s", rec.Body.String())
	}
	// The list never repeats the token.
	rec = owner.do(http.MethodGet, "/api/v1/admin/invitations", nil)
	if strings.Contains(rec.Body.String(), inv.Token) {
		t.Error("the invitation list carries the token")
	}

	guest := newBrowser(t, e.h)
	rec = guest.do(http.MethodGet, "/api/v1/invitations/"+inv.Token, nil)
	wantStatus(t, rec, http.StatusOK)
	var prev gen.InvitationPreview
	decode(t, rec, &prev)
	if prev.MaxRating != 12 || prev.Note == nil || *prev.Note != "Oma" || !prev.AllLibraries {
		t.Errorf("preview = %s", rec.Body.String())
	}
	accept := map[string]any{"email": "oma@example.ch", "password": "oma-password-4711", "profileName": "Oma", "language": "de"}
	rec = guest.do(http.MethodPost, "/api/v1/invitations/"+inv.Token+"/accept", accept)
	wantStatus(t, rec, http.StatusCreated)
	info := sessionOf(t, rec)
	if info.Account.Role != gen.AccountRoleGuest || info.Account.MaxRating != 12 || info.Profile == nil || info.Profile.Name != "Oma" || !info.Profile.IsPrimary {
		t.Fatalf("guest session = %s", rec.Body.String())
	}
	if guest.cookies[SessionCookie] == nil || guest.cookies[DeviceCookie] == nil {
		t.Errorf("accept must set the session cookie and keep the device cookie: %v", guest.cookies)
	}

	// The guest is a guest: no administration, but its own profile management.
	if rec := guest.do(http.MethodGet, "/api/v1/admin/guests", nil); rec.Code != http.StatusForbidden {
		t.Errorf("guest on admin: %d", rec.Code)
	}
	if rec := guest.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Opa", "maxRating": 16}); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "above_account_limit") {
		t.Errorf("guest profile above the ceiling: %d %s", rec.Code, rec.Body.String())
	}
	wantStatus(t, guest.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Opa", "maxRating": 6}), http.StatusCreated)

	// The link is spent: preview and accept answer 410, the list shows it as used with the e-mail.
	for _, call := range []func() *httptest.ResponseRecorder{
		func() *httptest.ResponseRecorder {
			return guest.do(http.MethodGet, "/api/v1/invitations/"+inv.Token, nil)
		},
		func() *httptest.ResponseRecorder {
			accept["email"] = "other@example.ch"
			return guest.do(http.MethodPost, "/api/v1/invitations/"+inv.Token+"/accept", accept)
		},
	} {
		if rec := call(); rec.Code != http.StatusGone || problemCode(t, rec) != "invitation_gone" {
			t.Errorf("used invitation: %d %s", rec.Code, rec.Body.String())
		}
	}
	var list []gen.Invitation
	decode(t, owner.do(http.MethodGet, "/api/v1/admin/invitations", nil), &list)
	if len(list) != 1 || list[0].State != gen.Used || list[0].UsedByEmail == nil || string(*list[0].UsedByEmail) != "oma@example.ch" {
		t.Errorf("invitation list = %+v", list)
	}
	var guests []gen.Guest
	decode(t, owner.do(http.MethodGet, "/api/v1/admin/guests", nil), &guests)
	if len(guests) != 1 || guests[0].ProfileCount != 2 || guests[0].MaxRating != 12 || !guests[0].AllLibraries {
		t.Errorf("guests = %+v", guests)
	}

	// A revoked link answers 410; an unknown one 404; the owner's e-mail cannot be taken.
	rec = owner.do(http.MethodPost, "/api/v1/admin/invitations", map[string]any{"maxRating": 6, "allLibraries": true})
	var second gen.InvitationCreated
	decode(t, rec, &second)
	wantStatus(t, owner.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/invitations/%d", second.Id), nil), http.StatusNoContent)
	wantStatus(t, owner.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/invitations/%d", second.Id), nil), http.StatusNoContent)
	if rec := guest.do(http.MethodGet, "/api/v1/invitations/"+second.Token, nil); rec.Code != http.StatusGone {
		t.Errorf("revoked invitation: %d", rec.Code)
	}
	if rec := guest.do(http.MethodGet, "/api/v1/invitations/"+strings.Repeat("A", 43), nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown invitation: %d", rec.Code)
	}
	rec = owner.do(http.MethodPost, "/api/v1/admin/invitations", map[string]any{"maxRating": 6, "allLibraries": true})
	var third gen.InvitationCreated
	decode(t, rec, &third)
	taken := map[string]any{"email": "MARCO@example.ch", "password": "oma-password-4711", "profileName": "X"}
	rec = newBrowser(t, e.h).do(http.MethodPost, "/api/v1/invitations/"+third.Token+"/accept", taken)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "already_exists") {
		t.Errorf("taken e-mail: %d %s", rec.Code, rec.Body.String())
	}
	if rec := guest.do(http.MethodGet, "/api/v1/invitations/"+third.Token, nil); rec.Code != http.StatusOK {
		t.Errorf("a failed accept must not use the link up: %d", rec.Code)
	}

	// Revoking the guest: its cookie answers 401 at once.
	wantStatus(t, owner.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/guests/%d", guests[0].Id), nil), http.StatusNoContent)
	if rec := guest.do(http.MethodGet, "/api/v1/auth/session", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked guest session: %d", rec.Code)
	}
	if rec := newBrowser(t, e.h).do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "oma@example.ch", "password": "oma-password-4711"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("login of the revoked guest: %d", rec.Code)
	}
}

// Image URLs carry the profile id, so a browser cache never serves one profile's image to another.
func TestImageURLsCarryProfileID(t *testing.T) {
	pf := newProfileFixture(t)
	f := pf.f
	for _, tc := range []struct {
		token string
		id    int64
	}{{"p18", f.P18.ID}, {"p6", f.P6.ID}, {"pg", f.PG.ID}} {
		rec := send(t, pf.h, http.MethodGet, BasePath+"/home", tc.token, "", nil)
		var page struct {
			Rows []struct {
				Items []struct {
					Images map[string]*struct {
						URL string `json:"url"`
					} `json:"images"`
				} `json:"items"`
			} `json:"rows"`
		}
		decode(t, rec, &page)
		n := 0
		for _, row := range page.Rows {
			for _, it := range row.Items {
				for kind, img := range it.Images {
					if img == nil {
						continue
					}
					n++
					if want := fmt.Sprintf("&p=%d", tc.id); !strings.HasSuffix(img.URL, want) || !strings.Contains(img.URL, "?v=") {
						t.Errorf("%s image %s URL = %q, want it to end with %s", tc.token, kind, img.URL, want)
					}
					if got := send(t, pf.h, http.MethodGet, img.URL, tc.token, "", nil); got.Code != http.StatusOK {
						t.Errorf("%s: image URL %s answers %d", tc.token, img.URL, got.Code)
					}
				}
			}
		}
		if n == 0 {
			t.Errorf("%s: no image URLs on the start page", tc.token)
		}
	}
}
