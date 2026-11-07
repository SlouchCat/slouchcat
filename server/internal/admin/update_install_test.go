package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/updater"
)

const fakeUpdaterToken = "updater-FAKE-token-0123456789abcdef"

// fakeUpdater is the companion of ADR 0024 over HTTP: it checks the token and answers /status and
// /install with what the test sets.
type fakeUpdater struct {
	*httptest.Server
	mu            sync.Mutex
	statusCode    int
	installCode   int
	installBody   string
	statusCalls   int
	installCalls  int
	authorization []string
}

func newFakeUpdater(t *testing.T) *fakeUpdater {
	t.Helper()
	u := &fakeUpdater{statusCode: http.StatusOK, installCode: http.StatusAccepted,
		installBody: `{"phase":"checking","from":"1.15.0","to":"1.16.0","startedAt":"2026-11-07T10:00:00Z"}`}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.authorization = append(u.authorization, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") != "Bearer "+fakeUpdaterToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == updater.PathStatus:
			u.statusCalls++
			w.WriteHeader(u.statusCode)
			_, _ = w.Write([]byte(`{"phase":"idle"}`))
		case r.Method == http.MethodPost && r.URL.Path == updater.PathInstall:
			u.installCalls++
			w.WriteHeader(u.installCode)
			_, _ = w.Write([]byte(u.installBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(u.Close)
	return u
}

func (u *fakeUpdater) set(code int, body string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.installCode, u.installBody = code, body
}

func (u *fakeUpdater) calls() (status, install int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.statusCalls, u.installCalls
}

// updaterDir is a shared folder with the token.
func updaterDir(t *testing.T, token string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, updater.TokenFile), []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeState(t *testing.T, dir string, st updater.State) {
	t.Helper()
	if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile), st, 0o644); err != nil {
		t.Fatal(err)
	}
}

// installUpdate maps the companion's answers: 202 with its state, 409 busy, 422 with its code and
// message, 401 and anything else as no companion.
func TestInstallUpdate(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	fu := newFakeUpdater(t)
	f.svc.WithUpdater(fu.URL+"/", updaterDir(t, fakeUpdaterToken), nil)

	in, err := f.svc.InstallUpdate(ctx)
	if err != nil || in.Phase != "checking" || in.From != "1.15.0" || in.To != "1.16.0" || in.StartedAt == nil ||
		!in.StartedAt.Equal(time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("202: %+v, %v", in, err)
	}
	fu.set(http.StatusConflict, `{"phase":"pulling","code":"busy"}`)
	if _, err := f.svc.InstallUpdate(ctx); !errors.Is(err, model.ErrUpdateRunning) {
		t.Errorf("409: %v, want ErrUpdateRunning", err)
	}
	for _, code := range []updater.Code{updater.CodeNotNewer, updater.CodeNoDigest, updater.CodeNoComposeLabels,
		updater.CodeOutsideDeployDir, updater.CodeManifestInvalid, updater.CodeManifestUnreachable} {
		fu.set(http.StatusUnprocessableEntity, `{"phase":"failed","code":"`+string(code)+`","message":"refused: `+string(code)+`"}`)
		_, err := f.svc.InstallUpdate(ctx)
		var refused *model.UpdateRefusedError
		if !errors.As(err, &refused) || !errors.Is(err, model.ErrUpdateRefused) || refused.Code != string(code) ||
			refused.Message != "refused: "+string(code) {
			t.Errorf("422 %s: %v", code, err)
		}
	}
	fu.set(http.StatusUnprocessableEntity, `not json`)
	var refused *model.UpdateRefusedError
	if _, err := f.svc.InstallUpdate(ctx); !errors.As(err, &refused) || refused.Code != "update_refused" {
		t.Errorf("422 without a body: %v, want update_refused", err)
	}
	for _, status := range []int{http.StatusInternalServerError, http.StatusNotFound} {
		fu.set(status, `{}`)
		if _, err := f.svc.InstallUpdate(ctx); !errors.Is(err, model.ErrUpdaterUnavailable) {
			t.Errorf("%d: %v, want ErrUpdaterUnavailable", status, err)
		}
	}
	// A wrong token is refused with 401: no companion for the server.
	f.svc.WithUpdater(fu.URL, updaterDir(t, "updater-FAKE-wrong-token"), nil)
	if _, err := f.svc.InstallUpdate(ctx); !errors.Is(err, model.ErrUpdaterUnavailable) {
		t.Errorf("401: %v, want ErrUpdaterUnavailable", err)
	}
	// Neither token reaches the log (rule 10).
	if logs := f.logs.String(); strings.Contains(logs, "FAKE") {
		t.Errorf("a token in the log:\n%s", logs)
	}
	if !strings.Contains(fu.authorization[0], "Bearer "+fakeUpdaterToken) {
		t.Errorf("authorization header %q", fu.authorization[0])
	}
}

// Without a companion, with a missing token or with a companion that is down there is no install
// and no available updater; nothing waits for long.
func TestInstallUpdateWithoutCompanion(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if _, err := f.svc.InstallUpdate(ctx); !errors.Is(err, model.ErrUpdaterUnavailable) {
		t.Errorf("not configured: %v", err)
	}
	st, err := f.svc.UpdateState(ctx)
	if err != nil || st.UpdaterAvailable || st.Install != nil {
		t.Errorf("state without companion: %+v, %v", st, err)
	}

	fu := newFakeUpdater(t)
	f.svc.WithUpdater(fu.URL, t.TempDir(), nil) // no token file
	if _, err := f.svc.InstallUpdate(ctx); !errors.Is(err, model.ErrUpdaterUnavailable) {
		t.Errorf("no token: %v", err)
	}
	if _, install := fu.calls(); install != 0 {
		t.Errorf("a request without the token reached the companion")
	}

	down := httptest.NewServer(http.NotFoundHandler())
	url := down.URL
	down.Close()
	f.svc.WithUpdater(url, updaterDir(t, fakeUpdaterToken), nil)
	start := time.Now()
	if _, err := f.svc.InstallUpdate(ctx); !errors.Is(err, model.ErrUpdaterUnavailable) {
		t.Errorf("down: %v", err)
	}
	if st, err := f.svc.UpdateState(ctx); err != nil || st.UpdaterAvailable {
		t.Errorf("state with the companion down: %+v, %v", st, err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("a companion that is down took %v", d)
	}
}

// updaterAvailable asks GET /status with the token and keeps the answer for updaterProbeTTL.
func TestUpdaterAvailableIsCached(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return now }
	fu := newFakeUpdater(t)
	f.svc.WithUpdater(fu.URL, updaterDir(t, fakeUpdaterToken), nil)
	for range 3 {
		if st, err := f.svc.UpdateState(ctx); err != nil || !st.UpdaterAvailable {
			t.Fatalf("available: %+v, %v", st, err)
		}
	}
	if status, _ := fu.calls(); status != 1 {
		t.Errorf("status calls = %d, want 1 within the cache", status)
	}
	fu.mu.Lock()
	fu.statusCode = http.StatusServiceUnavailable
	fu.mu.Unlock()
	now = now.Add(updaterProbeTTL)
	if st, err := f.svc.UpdateState(ctx); err != nil || st.UpdaterAvailable {
		t.Errorf("after the cache with a 503: %+v, %v", st, err)
	}
	// The header notice never asks the companion.
	now = now.Add(updaterProbeTTL)
	_ = f.svc.UpdateAvailable(ctx)
	if status, _ := fu.calls(); status != 2 {
		t.Errorf("status calls = %d, want 2 (UpdateAvailable must not probe)", status)
	}
}

// InstallStatus reads state.json: idle when it is missing, garbage, too large or names an unknown
// phase; the state otherwise, also in UpdateState.install.
func TestInstallStatusFromStateFile(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	dir := t.TempDir()
	f.svc.WithUpdater("", dir, nil) // the companion is down or gone: the file still counts
	path := filepath.Join(dir, updater.StateFile)
	for name, content := range map[string]string{
		"missing":       "",
		"garbage":       "{not json",
		"too large":     `{"phase":"idle","message":"` + strings.Repeat("x", updater.MaxFileBytes) + `"}`,
		"unknown phase": `{"phase":"exploding"}`,
	} {
		_ = os.Remove(path)
		if content != "" {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		in, err := f.svc.InstallStatus(ctx)
		if err != nil || in.Phase != "idle" || in.To != "" {
			t.Errorf("%s: %+v, %v; want idle", name, in, err)
		}
		if st, err := f.svc.UpdateState(ctx); err != nil || st.Install != nil {
			t.Errorf("%s: UpdateState.Install = %+v, %v; want none", name, st.Install, err)
		}
	}
	if n := strings.Count(f.logs.String(), "updater state unreadable"); n != 1 {
		t.Errorf("warnings for a bad state.json = %d, want 1 until it reads again", n)
	}
	finished := time.Date(2026, 11, 7, 10, 5, 0, 0, time.UTC)
	writeState(t, dir, updater.State{Phase: updater.PhaseFailed, From: "1.15.0", To: "1.16.0", Migrated: true,
		Backup: "pre-migration-1.16.0-20261107T100200.000Z.db", Code: updater.CodeMigratedNoRollback,
		Message: "a migration ran", StartedAt: finished.Add(-5 * time.Minute), FinishedAt: finished})
	in, err := f.svc.InstallStatus(ctx)
	if err != nil || in.Phase != "failed" || !in.Migrated || in.Backup != "pre-migration-1.16.0-20261107T100200.000Z.db" ||
		in.Code != "migrated_no_rollback" || in.FinishedAt == nil || !in.FinishedAt.Equal(finished) {
		t.Errorf("failed install: %+v, %v", in, err)
	}
	st, err := f.svc.UpdateState(ctx)
	if err != nil || st.Install == nil || st.Install.Phase != "failed" || st.UpdaterAvailable {
		t.Errorf("UpdateState: %+v, %v", st, err)
	}
}

// failedReports records the event errors of failed installs.
type failedReports struct {
	mu  sync.Mutex
	got []model.UpdateInstall
}

func (r *failedReports) report(_ context.Context, in model.UpdateInstall) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, in)
	return nil
}

func (r *failedReports) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.got)
}

// A rolled back or failed install is reported once: not again on the next read, not after a
// restart of the server (the setting update_install_reported), again for a newer one; a done
// install and a refusal before anything changed raise nothing.
func TestReportFailedInstallOnce(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	dir := t.TempDir()
	reports := &failedReports{}
	f.svc.WithUpdater("", dir, reports.report)
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

	writeState(t, dir, updater.State{Phase: updater.PhaseDone, From: "1.15.0", To: "1.16.0", StartedAt: at, FinishedAt: at.Add(time.Minute)})
	f.svc.WatchUpdateInstall(ctx)
	writeState(t, dir, updater.State{Phase: updater.PhaseFailed, Code: updater.CodeNotNewer, StartedAt: at, FinishedAt: at.Add(2 * time.Minute)})
	f.svc.WatchUpdateInstall(ctx)
	if reports.count() != 0 {
		t.Fatalf("done and a refusal reported: %+v", reports.got)
	}

	rolled := updater.State{Phase: updater.PhaseRolledBack, From: "1.15.0", To: "1.16.0", Code: updater.CodeUnhealthy,
		StartedAt: at, FinishedAt: at.Add(3 * time.Minute)}
	writeState(t, dir, rolled)
	f.svc.WatchUpdateInstall(ctx)
	if _, err := f.svc.InstallStatus(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateState(ctx); err != nil {
		t.Fatal(err)
	}
	if reports.count() != 1 || reports.got[0].Phase != "rolled_back" || reports.got[0].To != "1.16.0" || reports.got[0].Code != "unhealthy" {
		t.Fatalf("reports after three reads: %+v", reports.got)
	}
	if v, ok, err := f.svc.setting(ctx, SettingUpdateInstallReported); err != nil || !ok || v != "2026-11-07T10:03:00Z" {
		t.Errorf("setting = %q %v %v", v, ok, err)
	}

	// The server restarts: a new service on the same database reports nothing again.
	restarted := New(f.db, f.auth, f.libs, f.ring, f.tmdb, func(string) string { return "" }, f.svc.log)
	again := &failedReports{}
	restarted.WithUpdater("", dir, again.report)
	restarted.WatchUpdateInstall(ctx)
	if again.count() != 0 {
		t.Fatalf("reported again after the restart: %+v", again.got)
	}
	// A newer failed install is reported.
	writeState(t, dir, updater.State{Phase: updater.PhaseFailed, From: "1.15.0", To: "1.16.1", Migrated: true,
		Backup: "pre-migration-1.16.1-x.db", Code: updater.CodeMigratedNoRollback, StartedAt: at.Add(time.Hour), FinishedAt: at.Add(time.Hour + time.Minute)})
	restarted.WatchUpdateInstall(ctx)
	if again.count() != 1 || !again.got[0].Migrated || again.got[0].Backup != "pre-migration-1.16.1-x.db" {
		t.Errorf("newer failure: %+v", again.got)
	}
}

// A send that fails is tried again at the next read instead of being marked reported.
func TestReportFailedInstallRetriesAfterAFailedSend(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	dir := t.TempDir()
	var calls int
	f.svc.WithUpdater("", dir, func(context.Context, model.UpdateInstall) error {
		calls++
		if calls == 1 {
			return errors.New("queue full")
		}
		return nil
	})
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	writeState(t, dir, updater.State{Phase: updater.PhaseRolledBack, Code: updater.CodeUnhealthy, StartedAt: at, FinishedAt: at.Add(time.Minute)})
	for range 3 {
		if _, err := f.svc.InstallStatus(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Errorf("send calls = %d, want 2 (one failed, one sent)", calls)
	}
}

// At the start the install may still run (the companion waits for this server or brings the old
// one back): WatchUpdateInstall looks again until it ended and reports it then.
func TestWatchUpdateInstallFollowsARunningInstall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newFixture(t)
	dir := t.TempDir()
	reports := &failedReports{}
	f.svc.WithUpdater("", dir, reports.report)
	f.svc.ops.updater.watchEvery = 10 * time.Millisecond
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	writeState(t, dir, updater.State{Phase: updater.PhaseWaiting, From: "1.15.0", To: "1.16.0", StartedAt: at})
	done := make(chan struct{})
	go func() {
		f.svc.WatchUpdateInstall(ctx)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	if reports.count() != 0 {
		t.Fatal("reported a running install")
	}
	writeState(t, dir, updater.State{Phase: updater.PhaseRolledBack, From: "1.15.0", To: "1.16.0", Code: updater.CodeUnhealthy,
		StartedAt: at, FinishedAt: at.Add(4 * time.Minute)})
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("the watch did not end after the install ended")
	}
	if reports.count() != 1 {
		t.Errorf("reports = %+v, want the rollback once", reports.got)
	}
}
