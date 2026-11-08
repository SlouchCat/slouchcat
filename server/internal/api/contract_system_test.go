package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
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
	t.Parallel()
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
	t.Parallel()
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

// Concept 11 (side finding): the dashboard carries the number of paused job types, 0 before and 1
// after pausing one type, so the tile "Pausiert" can appear.
func TestDashboardMapsPausedJobTypes(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	paused := func() int {
		t.Helper()
		rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var d gen.Dashboard
		decode(t, rec, &d)
		if d.Jobs.Paused == nil {
			t.Fatalf("dashboard without jobs.paused: %s", rec.Body.String())
		}
		return *d.Jobs.Paused
	}
	if n := paused(); n != 0 {
		t.Errorf("paused before = %d, want 0", n)
	}
	wantStatus(t, send(t, pf.h, http.MethodPost, BasePath+"/admin/job-types/download.prepare/pause", "p18", "", nil), http.StatusOK)
	if n := paused(); n != 1 {
		t.Errorf("paused after pausing download.prepare = %d, want 1", n)
	}
}

// Concept 11 (contract 1.7.0): every new field of getDashboard over HTTP. A running and a failed
// job, an aborted scan, a stream of an episode with the poster of its series, and the counts of
// the sidebar, whose account count follows a deletion on the accounts page at once.
func TestDashboardOverviewOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h, ctx := pf.f, pf.h, context.Background()
	dashboard := func() (gen.Dashboard, string) {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var d gen.Dashboard
		decode(t, rec, &d)
		return d, rec.Body.String()
	}
	d, body := dashboard()
	for _, field := range []string{`"active":[]`, `"failedByType":[]`, `"attention":{"abortedScans":[]}`, `"counts":{`} {
		if !strings.Contains(body, field) {
			t.Errorf("empty dashboard lacks %s: %s", field, body)
		}
	}
	accounts := d.Counts.Accounts
	// maintenanceOpen is the sum of the five cheap issue kinds; the fixture has two such cases.
	open := 0
	for _, kind := range []string{"unmatched", "no_images", "unrated", "orphaned", "unplayable"} {
		rec := send(t, h, http.MethodGet, BasePath+"/admin/maintenance/issues?kind="+kind, "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var page gen.IssuePage
		decode(t, rec, &page)
		open += page.Total
	}
	if accounts != 3 || d.Counts.Titles <= 0 || d.Counts.MaintenanceOpen != 2 || open != 2 {
		t.Errorf("counts = %+v (issues %d), want the three accounts of the fixture, its titles and two maintenance cases", d.Counts, open)
	}

	// One running and one failed job of notify.deliver.
	running, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: "notify.deliver"})
	if err != nil {
		t.Fatal(err)
	}
	if j, err := pf.queue.Claim(ctx, "w", []string{"notify.deliver"}); err != nil || j == nil || j.ID != running {
		t.Fatalf("claim: %v, %v", j, err)
	}
	if _, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: "download.prepare", MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}
	j, err := pf.queue.Claim(ctx, "w", []string{"download.prepare"})
	if err != nil || j == nil {
		t.Fatalf("claim: %v, %v", j, err)
	}
	if err := pf.queue.Fail(ctx, "w", j, errors.New("token=dash-FAKE-token broke")); err != nil {
		t.Fatal(err)
	}
	// The scan of the fixture's library was aborted by the loss protection.
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE libraries SET last_scan_state = 'aborted', last_scan_error = 'root_missing' WHERE id = (SELECT min(id) FROM libraries)`); err != nil {
		t.Fatal(err)
	}
	var libID int64
	var libName string
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT id, name FROM libraries ORDER BY id LIMIT 1`).Scan(&libID, &libName); err != nil {
		t.Fatal(err)
	}
	// Every schedule in 10 h, the first one by name in 1 h: that is the next run.
	next := time.Now().Add(time.Hour).Truncate(time.Millisecond).UTC()
	var nextType string
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT type FROM job_schedules WHERE enabled = 1 ORDER BY type LIMIT 1`).Scan(&nextType); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE job_schedules SET next_run_at = CASE WHEN type = ? THEN ? ELSE ? END`,
		nextType, next.UnixMilli(), next.Add(9*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	// An episode streams; its series has a poster.
	if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO images (item_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
	    VALUES (?, 'poster', ?, 'jpeg', 'https://example.org/p.jpg', 'p.jpg', 'seriesHash', 'image/jpeg', 200, 300, 1, 1)
	    ON CONFLICT (item_id, kind, size, format) DO UPDATE SET content_hash = excluded.content_hash`, f.Show.Series.ID, model.AdminPosterSize); err != nil {
		t.Fatal(err)
	}
	if _, code, _ := play(t, h, "p18", f.Show.E1.ID, `{}`); code != http.StatusOK {
		t.Fatalf("play = %d", code)
	}

	d, body = dashboard()
	if len(d.Jobs.Active) != 1 || d.Jobs.Active[0].Id != running || d.Jobs.Active[0].Type != "notify.deliver" || d.Jobs.Active[0].State != gen.JobRunStateRunning {
		t.Errorf("active = %+v", d.Jobs.Active)
	}
	if len(d.Jobs.FailedByType) != 1 {
		t.Fatalf("failed by type = %+v", d.Jobs.FailedByType)
	}
	fb := d.Jobs.FailedByType[0]
	if fb.Type != "download.prepare" || fb.Count != 1 || fb.LastError == nil || strings.Contains(*fb.LastError, "dash-FAKE-token") ||
		fb.LastFailedAt.IsZero() || fb.LastFailedAt.Location() != time.UTC {
		t.Errorf("failure = %+v (%v)", fb, deref(fb.LastError))
	}
	if strings.Contains(body, "dash-FAKE-token") {
		t.Error("the dashboard carries the secret of the failed job")
	}
	if n := d.Jobs.NextRun; n == nil || n.Type != nextType || !n.At.Equal(next) || n.At.Location() != time.UTC {
		t.Errorf("next run = %+v, want %s at %v", n, nextType, next)
	}
	want := gen.AbortedScan{LibraryId: libID, Name: libName, Reason: "root_missing"}
	if len(d.Attention.AbortedScans) != 1 || d.Attention.AbortedScans[0] != want {
		t.Errorf("aborted scans = %+v, want %+v", d.Attention.AbortedScans, want)
	}
	wantPoster := BasePath + "/admin/items/" + f.Show.Series.ID + "/images/poster/" + model.AdminPosterSize + "?v=seriesHash"
	if len(d.Streams) != 1 || d.Streams[0].ItemId != f.Show.E1.ID || deref(d.Streams[0].PosterUrl) != wantPoster {
		t.Errorf("streams = %+v, want E1 with the series poster %s", d.Streams, wantPoster)
	}

	wantStatus(t, send(t, h, http.MethodDelete, BasePath+"/admin/accounts/"+strconv.FormatInt(f.Guest.ID, 10), "p18", "", nil), http.StatusNoContent)
	if d, _ = dashboard(); d.Counts.Accounts != accounts-1 {
		t.Errorf("accounts after deleting one = %d, want %d at once", d.Counts.Accounts, accounts-1)
	}
	// A redeemed invitation creates an account; the count follows at once.
	rec := send(t, h, http.MethodPost, BasePath+"/admin/invitations", "p18", `{"maxRating":12,"allLibraries":true}`, nil)
	wantStatus(t, rec, http.StatusCreated)
	var inv gen.InvitationCreated
	decode(t, rec, &inv)
	rec = send(t, h, http.MethodPost, BasePath+"/invitations/"+inv.Token+"/accept", "",
		`{"email":"neu@example.ch","password":"FAKE-guest-password-4711","profileName":"Neu","language":"de"}`, nil)
	wantStatus(t, rec, http.StatusCreated)
	if d, _ = dashboard(); d.Counts.Accounts != accounts {
		t.Errorf("accounts after a redeemed invitation = %d, want %d at once", d.Counts.Accounts, accounts)
	}
}

// A-3, A-5 on the contract fixture: the administration answers have their shape, and after a
// check of the mock manifest the main account's session carries the update notice while a guest's
// does not.
func TestAdminSystemShapesAndUpdateNotice(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h

	rec := send(t, h, http.MethodGet, BasePath+"/admin/system", "p18", "", nil)
	var sys gen.SystemInfo
	decode(t, rec, &sys)
	if rec.Code != http.StatusOK || sys.Version != "0.0.0-test" || sys.ConfigFs == "" || sys.Gpu.Tonemapper == "" {
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
	// A-2 with ADR 0014: the owner first, then the second administrator, then the account that
	// administers nothing. Only the first carries the owner badge.
	if rec.Code != http.StatusOK || len(accounts) != 3 ||
		accounts[0].Role != gen.AdminAccountRoleAdmin || !accounts[0].IsOwner ||
		accounts[1].Role != gen.AdminAccountRoleAdmin || accounts[1].IsOwner ||
		accounts[2].Role != gen.AdminAccountRoleUser || accounts[2].IsOwner ||
		len(accounts[0].Profiles) != 6 || !accounts[0].Profiles[0].IsPrimary || accounts[2].AllLibraries || len(accounts[2].LibraryIds) != 1 {
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
	// The fixture runs as 0.0.0-test, a pre-release of 0.0.0 and so older than 0.0.1; a label
	// that is no version at all (a bare commit id) would be a development build without a notice.
	if rec.Code != http.StatusOK || !u.Available || u.Development || deref(u.LatestVersion) != "0.0.1" ||
		u.LastSuccessAt == nil || u.CheckedAt == nil || !u.LastSuccessAt.Equal(*u.CheckedAt) {
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

// Concept 11 step 2 (contract 1.7.0): Dashboard.activity over HTTP. Without events the list is
// empty, not absent; after a scan and a playback the entries come newest first with the profile
// name, the e-mail address and the title read by the server, and no sentence.
func TestDashboardActivityOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h, ctx := pf.f, pf.h, context.Background()
	pf.pb.WithActivity(pf.admin.RecordActivity)
	dashboard := func() (gen.Dashboard, string) {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var d gen.Dashboard
		decode(t, rec, &d)
		return d, rec.Body.String()
	}
	if d, body := dashboard(); len(d.Activity) != 0 || !strings.Contains(body, `"activity":[]`) {
		t.Fatalf("activity without events = %s", body)
	}
	newTitles := 3
	pf.admin.RecordActivity(ctx, model.ActivityEvent{
		Kind: model.ActivityScanDone, At: time.Now().Add(-time.Minute), LibraryID: f.LibraryID(), Subject: "Filme",
		Detail: model.ActivityDetail{NewTitles: &newTitles},
	})
	movie := f.Rated(12)
	if _, code, _ := play(t, h, "p18", movie.ID, `{}`); code != http.StatusOK {
		t.Fatalf("play = %d", code)
	}
	var d gen.Dashboard
	var body string
	deadline := time.Now().Add(5 * time.Second)
	for d, body = dashboard(); len(d.Activity) < 2 && time.Now().Before(deadline); d, body = dashboard() {
		time.Sleep(20 * time.Millisecond) // the start is written in the background
	}
	if len(d.Activity) != 2 {
		t.Fatalf("activity = %s", body)
	}
	p, s := d.Activity[0], d.Activity[1]
	if p.Kind != model.ActivityPlaybackStarted || deref(p.ProfileName) != f.P18.Name || deref(p.AccountEmail) != f.Account.Email ||
		p.ItemTitle == nil || *p.ItemTitle == "" || p.LibraryName != nil || p.At.Location() != time.UTC {
		t.Errorf("playback entry = %+v", p)
	}
	if s.Kind != model.ActivityScanDone || deref(s.LibraryName) != "Filme" || s.ItemTitle != nil || s.ProfileName != nil ||
		s.Detail.NewTitles == nil || *s.Detail.NewTitles != 3 || s.Detail.Reason != nil {
		t.Errorf("scan entry = %+v", s)
	}
	if !p.At.After(s.At) || p.Id == s.Id {
		t.Errorf("not newest first: %v, %v", p.At, s.At)
	}
	if !strings.Contains(body, `"detail":{}`) || !strings.Contains(body, `"detail":{"newTitles":3}`) {
		t.Errorf("detail fields not omitted when empty: %s", body)
	}
}

// Concept 11 step 3: restartServer over HTTP with the fixture's fake restart. 202 with restartAt,
// the fake runs about a second later, a second call and a restore answer 409 restart_pending,
// and the dashboard lists server_restart with the account that asked.
func TestRestartServerOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	rec := send(t, h, http.MethodPost, BasePath+"/admin/backups", "p18", `{"includeSecrets":false}`, nil)
	wantStatus(t, rec, http.StatusCreated)
	var backup gen.Backup
	decode(t, rec, &backup)

	before := time.Now()
	rec = send(t, h, http.MethodPost, BasePath+"/admin/server/restart", "p18", "", nil)
	wantStatus(t, rec, http.StatusAccepted)
	var res gen.RestartAccepted
	decode(t, rec, &res)
	if !res.RestartAt.After(before) || res.RestartAt.Location() != time.UTC {
		t.Errorf("restartAt = %v, want after %v in UTC", res.RestartAt, before)
	}
	for _, path := range []string{"/admin/server/restart", "/admin/backups/" + backup.Name + "/restore"} {
		rec = send(t, h, http.MethodPost, BasePath+path, "p18", "", nil)
		if rec.Code != http.StatusConflict || problemCode(t, rec) != "restart_pending" {
			t.Errorf("%s while pending: %d %s, want 409 restart_pending", path, rec.Code, rec.Body.String())
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for pf.restarts.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := pf.restarts.Load(); n != 1 {
		t.Errorf("fake restart called %d times, want 1", n)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var d gen.Dashboard
	decode(t, rec, &d)
	var restarts []gen.ActivityEntry
	for _, a := range d.Activity {
		if a.Kind == model.ActivityServerRestart {
			restarts = append(restarts, a)
		}
	}
	if len(restarts) != 1 || deref(restarts[0].AccountEmail) != pf.f.Account.Email || restarts[0].ProfileName != nil {
		t.Errorf("server_restart entries = %+v, want one of %s", restarts, pf.f.Account.Email)
	}
}
