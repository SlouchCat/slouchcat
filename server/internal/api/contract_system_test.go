package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8 lane S1 at the HTTP surface (A-1, A-2, A-3, A-5): the wizard checks and the system state of
// getSetupStatus, the shapes of the administration answers, the update notice of the session and
// the own account change.

// contractPushRotator stores a fixed VAPID pair, as package notify would after generating one.
type contractPushRotator struct{ a *admin.Service }

func (r contractPushRotator) RotateVAPID(ctx context.Context) error {
	return r.a.SetVAPIDKeys(ctx, "BContract-FAKE-public", "contract-FAKE-private")
}

// contractManifest serves a release manifest of version 0.0.1 and returns its URL.
func contractManifest(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.0.1","url":"https://example.org/releases/0.0.1","notes":"Test"}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/manifest.json"
}

// A-1: before the first account getSetupStatus carries the system state and both checks answer;
// afterwards the state is gone and the checks answer 409 setup_done.
func TestSetupChecksOverHTTP(t *testing.T) {
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodGet, "/api/v1/setup", nil, nil)
	wantStatus(t, rec, http.StatusOK)
	var st gen.SetupStatus
	decode(t, rec, &st)
	if !st.SetupRequired || st.System == nil || st.System.Gpu.Backend != gen.GPUInfoBackendSoftware || st.System.Version == "" {
		t.Fatalf("setup status = %s", rec.Body.String())
	}

	rec = e.call(t, http.MethodPost, "/api/v1/setup/path-check", map[string]any{"path": t.TempDir()}, nil)
	wantStatus(t, rec, http.StatusOK)
	var pc gen.PathCheck
	decode(t, rec, &pc)
	if !pc.Ok || pc.FsType == nil || pc.WatchMode == nil || pc.Problem != nil {
		t.Errorf("path check of a directory = %s", rec.Body.String())
	}
	rec = e.call(t, http.MethodPost, "/api/v1/setup/path-check", map[string]any{"path": "relative/path"}, nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &pc)
	if pc.Ok || pc.Problem == nil || *pc.Problem != gen.PathCheckProblemNotAbsolute {
		t.Errorf("path check of a relative path = %s", rec.Body.String())
	}
	rec = e.call(t, http.MethodPost, "/api/v1/setup/path-check", map[string]any{"path": ""}, nil)
	wantStatus(t, rec, http.StatusUnprocessableEntity)

	e.tmdb.err = model.ErrTMDBKeyInvalid
	rec = e.call(t, http.MethodPost, "/api/v1/setup/tmdb-check", map[string]any{"tmdbApiKey": authTestKey}, nil)
	wantStatus(t, rec, http.StatusOK)
	var tc gen.TMDBCheck
	decode(t, rec, &tc)
	if tc.Ok || tc.Problem == nil || *tc.Problem != gen.Invalid {
		t.Errorf("key check = %s", rec.Body.String())
	}
	if strings.Contains(e.logs.String(), authTestKey) {
		t.Error("the checked key reached the log")
	}
	e.tmdb.err = nil

	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil), http.StatusCreated)
	rec = e.call(t, http.MethodGet, "/api/v1/setup", nil, nil)
	var after gen.SetupStatus
	decode(t, rec, &after)
	if after.SetupRequired || after.System != nil {
		t.Errorf("setup status after the wizard = %s", rec.Body.String())
	}
	for _, path := range []string{"/api/v1/setup/path-check", "/api/v1/setup/tmdb-check"} {
		rec = e.call(t, http.MethodPost, path, map[string]any{"path": "/tmp", "tmdbApiKey": authTestKey}, nil)
		if rec.Code != http.StatusConflict || problemCode(t, rec) != "setup_done" {
			t.Errorf("%s after the wizard: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

// A-2: the account changes its e-mail and password with the current password; the session stays.
func TestUpdateAccountOverHTTP(t *testing.T) {
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	cookie := sessionCookie(t, rec)

	rec = e.call(t, http.MethodPut, "/api/v1/auth/account", map[string]any{"currentPassword": "wrong-password", "email": "neu@example.ch"}, cookie)
	if rec.Code != http.StatusUnauthorized || problemCode(t, rec) != "invalid_credentials" {
		t.Fatalf("wrong password: %d %s", rec.Code, rec.Body.String())
	}
	rec = e.call(t, http.MethodPut, "/api/v1/auth/account",
		map[string]any{"currentPassword": authTestPassword, "email": "neu@example.ch", "newPassword": "FAKE-new-password-0815"}, cookie)
	wantStatus(t, rec, http.StatusOK)
	var info gen.SessionInfo
	decode(t, rec, &info)
	if info.Account.Email != "neu@example.ch" || info.Profile == nil {
		t.Errorf("session after the change = %s", rec.Body.String())
	}
	wantStatus(t, e.call(t, http.MethodGet, "/api/v1/auth/session", nil, cookie), http.StatusOK)
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/auth/login",
		map[string]any{"email": "neu@example.ch", "password": "FAKE-new-password-0815"}, nil), http.StatusOK)
	for _, secret := range []string{authTestPassword, "FAKE-new-password-0815"} {
		if strings.Contains(e.logs.String(), secret) {
			t.Errorf("log carries %q", secret)
		}
	}
}

// A-3, A-5 on the contract fixture: the administration answers have their shape, and after a
// check of the mock manifest the main account's session carries the update notice while a guest's
// does not.
func TestAdminSystemShapesAndUpdateNotice(t *testing.T) {
	pf := newProfileFixture(t)
	h := pf.h

	rec := send(t, h, http.MethodGet, BasePath+"/admin/system", "p18", "", nil)
	var sys gen.SystemInfo
	decode(t, rec, &sys)
	if rec.Code != http.StatusOK || sys.Version != "test" || sys.ConfigFs == "" || sys.Gpu.Tonemapper == "" {
		t.Errorf("system = %d %s", rec.Code, rec.Body.String())
	}

	rec = send(t, h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
	var d gen.Dashboard
	decode(t, rec, &d)
	if rec.Code != http.StatusOK || len(d.Integrations) != 8 || d.Jobs.RecentFailed == nil || d.Errors == nil || d.Streams == nil ||
		d.Storage.Cache.TotalBytes <= 0 || !d.Update.Enabled {
		t.Errorf("dashboard = %d %s", rec.Code, rec.Body.String())
	}

	rec = send(t, h, http.MethodGet, BasePath+"/admin/accounts", "p18", "", nil)
	var accounts []gen.AdminAccount
	decode(t, rec, &accounts)
	if rec.Code != http.StatusOK || len(accounts) != 2 || accounts[0].Role != gen.AdminAccountRoleOwner || accounts[1].Role != gen.AdminAccountRoleGuest ||
		len(accounts[0].Profiles) != 6 || !accounts[0].Profiles[0].IsPrimary || accounts[1].AllLibraries || len(accounts[1].LibraryIds) != 1 {
		t.Errorf("accounts = %d %s", rec.Code, rec.Body.String())
	}

	rec = send(t, h, http.MethodGet, BasePath+"/admin/settings", "p18", "", nil)
	var st gen.AdminSettings
	decode(t, rec, &st)
	if rec.Code != http.StatusOK || st.Keys.Tmdb.Source != gen.SettingSourceNone || st.Push.Configured {
		t.Errorf("settings = %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodPut, BasePath+"/admin/settings", "p18", `{"keys":{"tvdb":"tvdb-FAKE-contract-key-0000"},"publicUrl":"https://media.example.ch/"}`, nil)
	decode(t, rec, &st)
	if rec.Code != http.StatusOK || st.Keys.Tvdb.Source != gen.SettingSourceStored || deref(st.Keys.Tvdb.Hint) != "0000" ||
		deref(st.PublicUrl) != "https://media.example.ch" || strings.Contains(rec.Body.String(), "tvdb-FAKE") {
		t.Errorf("settings after the update = %d %s", rec.Code, rec.Body.String())
	}

	sessionNotice := func(token string) *gen.UpdateNotice {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/auth/session", token, "", nil)
		var info gen.SessionInfo
		decode(t, rec, &info)
		return info.UpdateAvailable
	}
	if n := sessionNotice("p18"); n != nil {
		t.Errorf("notice before any check: %+v", n)
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/update/check", "p18", "", nil)
	var u gen.UpdateState
	decode(t, rec, &u)
	if rec.Code != http.StatusOK || !u.Available || deref(u.LatestVersion) != "0.0.1" {
		t.Fatalf("check = %d %s", rec.Code, rec.Body.String())
	}
	for _, token := range []string{"p18", "p6", "none"} {
		if n := sessionNotice(token); n == nil || n.Version != "0.0.1" || n.Url != "https://example.org/releases/0.0.1" {
			t.Errorf("%s: notice = %+v, want 0.0.1 for every session of the main account", token, n)
		}
	}
	if n := sessionNotice("pg"); n != nil {
		t.Errorf("guest: notice = %+v, want none", n)
	}
}
