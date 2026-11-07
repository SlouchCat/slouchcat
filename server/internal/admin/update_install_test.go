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
	// installGate, when set, holds POST /install until it is closed; statusDelay slows GET /status.
	installGate chan struct{}
	statusDelay time.Duration
}

func newFakeUpdater(t *testing.T) *fakeUpdater {
	t.Helper()
	u := &fakeUpdater{statusCode: http.StatusOK, installCode: http.StatusAccepted,
		installBody: `{"phase":"backing_up","from":"1.15.0","to":"1.16.0","startedAt":"2026-11-07T10:00:00Z"}`}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		gate, delay := u.installGate, u.statusDelay
		u.mu.Unlock()
		if r.URL.Path == updater.PathInstall && gate != nil {
			<-gate
		}
		if r.URL.Path == updater.PathStatus {
			time.Sleep(delay)
		}
		u.mu.Lock()
		defer u.mu.Unlock()
		u.authorization = append(u.authorization, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") != "Bearer "+fakeUpdaterToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized) // plain text, like the companion
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

	// The companion answers the install after its checks, up to about 25 s.
	if updaterInstallTimeout < 30*time.Second {
		t.Errorf("install timeout %v, want at least 30 s", updaterInstallTimeout)
	}
	in, err := f.svc.InstallUpdate(ctx)
	if err != nil || in.Phase != "backing_up" || in.From != "1.15.0" || in.To != "1.16.0" || in.StartedAt == nil ||
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
	f.svc.WithSystem(SystemSource{Version: "1.15.0"})
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
	f.svc.followInstall(ctx)
	writeState(t, dir, updater.State{Phase: updater.PhaseFailed, Code: updater.CodeNotNewer, StartedAt: at, FinishedAt: at.Add(2 * time.Minute)})
	f.svc.followInstall(ctx)
	if reports.count() != 0 {
		t.Fatalf("done and a refusal reported: %+v", reports.got)
	}

	rolled := updater.State{Phase: updater.PhaseRolledBack, From: "1.15.0", To: "1.16.0", Code: updater.CodeUnhealthy,
		StartedAt: at, FinishedAt: at.Add(3 * time.Minute)}
	writeState(t, dir, rolled)
	f.svc.followInstall(ctx)
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

	// A failure without a code (the .env backup failed, or the companion found an interrupted
	// install at its start) carries only a message and is reported as well.
	noCode := updater.State{Phase: updater.PhaseFailed, From: "1.15.0", To: "1.16.0", Message: "interrupted install found at start",
		StartedAt: at, FinishedAt: at.Add(4 * time.Minute)}
	writeState(t, dir, noCode)
	f.svc.followInstall(ctx)
	if reports.count() != 2 || reports.got[1].Code != "" || reports.got[1].Message != "interrupted install found at start" {
		t.Fatalf("failure without a code: %+v", reports.got)
	}

	// The server restarts: a new service on the same database reports nothing again.
	restarted := New(f.db, f.auth, f.libs, f.ring, f.tmdb, func(string) string { return "" }, f.svc.log)
	again := &failedReports{}
	restarted.WithUpdater("", dir, again.report)
	restarted.followInstall(ctx)
	if again.count() != 0 {
		t.Fatalf("reported again after the restart: %+v", again.got)
	}
	// A newer failed install is reported.
	writeState(t, dir, updater.State{Phase: updater.PhaseFailed, From: "1.15.0", To: "1.16.1", Migrated: true,
		Backup: "pre-migration-1.16.1-x.db", Code: updater.CodeMigratedNoRollback, StartedAt: at.Add(time.Hour), FinishedAt: at.Add(time.Hour + time.Minute)})
	restarted.followInstall(ctx)
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
// one back): followInstall looks again until it ended and reports it then.
func TestWatchUpdateInstallFollowsARunningInstall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newFixture(t)
	dir := t.TempDir()
	reports := &failedReports{}
	f.svc.WithUpdater("", dir, reports.report)
	f.svc.ops.updater.watchEvery = 10 * time.Millisecond
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return at.Add(time.Minute) } // a fresh install, not a stale one
	writeState(t, dir, updater.State{Phase: updater.PhaseWaiting, From: "1.15.0", To: "1.16.0", StartedAt: at})
	done := make(chan struct{})
	go func() {
		f.svc.followInstall(ctx)
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

// The stale bound lies above the longest install of the companion (pull, wait, rollback), so the
// server never calls an install interrupted that the companion still carries through.
func TestInstallStaleAfterCoversTheCompanion(t *testing.T) {
	if installStaleAfter <= updater.InstallWorstCase {
		t.Fatalf("installStaleAfter %s, companion worst case %s", installStaleAfter, updater.InstallWorstCase)
	}
	if want := updater.PullTimeout + 2*updater.WaitTimeout; updater.InstallWorstCase < want {
		t.Fatalf("worst case %s leaves out the pull and both waits (%s)", updater.InstallWorstCase, want)
	}
}

// followInstall has no time limit of its own while an install runs: it follows past the old 25
// minutes and, while the companion answers, past the stale bound too, until the install ends. It
// stops once the install counts as interrupted (older than the bound, companion silent), without
// an event.
func TestFollowInstallUntilFinalOrStale(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	clock := at.Add(30 * time.Minute)
	setClock := func(t time.Time) { clockMu.Lock(); clock = t; clockMu.Unlock() }
	follow := func(svc *Service) chan struct{} {
		done := make(chan struct{})
		go func() {
			svc.followInstall(ctx)
			close(done)
		}()
		return done
	}
	ended := func(done chan struct{}) bool {
		select {
		case <-done:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}
	running := updater.State{Phase: updater.PhasePulling, From: "1.15.0", To: "1.16.0", StartedAt: at}

	f := newFixture(t)
	f.svc.now = func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return clock }
	dir := updaterDir(t, fakeUpdaterToken)
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()
	reports := &failedReports{}
	f.svc.WithUpdater(downURL, dir, reports.report)
	f.svc.ops.updater.watchEvery = 10 * time.Millisecond
	writeState(t, dir, running)
	done := follow(f.svc)
	if ended(done) {
		t.Fatal("stopped following an install 30 minutes old")
	}
	setClock(at.Add(installStaleAfter))
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("still following a stale install with the companion silent")
	}
	if reports.count() != 0 {
		t.Errorf("reports = %+v, want none for an interrupted install", reports.got)
	}

	setClock(at.Add(2 * installStaleAfter))
	fu := newFakeUpdater(t)
	f.svc.WithUpdater(fu.URL, dir, reports.report)
	f.svc.ops.updater.watchEvery = 10 * time.Millisecond
	done = follow(f.svc)
	if ended(done) {
		t.Fatal("stopped following an old install while the companion answers")
	}
	writeState(t, dir, updater.State{Phase: updater.PhaseRolledBack, From: "1.15.0", To: "1.16.0", Code: updater.CodeUnhealthy,
		StartedAt: at, FinishedAt: at.Add(2 * installStaleAfter)})
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("the watch did not end after the install ended")
	}
	if reports.count() != 1 {
		t.Errorf("reports = %+v, want the rollback once", reports.got)
	}
}

// The server decides what the card shows (rule 12): a running phase older than installStaleAfter while
// the companion does not answer was interrupted and is shown as failed without a code; with the
// companion answering it still runs. A finished install that names neither from nor to as the
// running version is old and not shown; a self-built image of the release counts as it.
func TestShownInstall(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return now }
	f.svc.WithSystem(SystemSource{Version: "1.16.0-3-gabc1234"})
	dir := updaterDir(t, fakeUpdaterToken)
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()
	f.svc.WithUpdater(downURL, dir, nil)

	running := updater.State{Phase: updater.PhasePulling, From: "1.15.0", To: "1.16.0", StartedAt: now.Add(-installStaleAfter + time.Minute)}
	writeState(t, dir, running)
	if in, err := f.svc.InstallStatus(ctx); err != nil || in.Phase != "pulling" {
		t.Errorf("fresh running install: %+v, %v", in, err)
	}
	running.StartedAt = now.Add(-installStaleAfter)
	writeState(t, dir, running)
	in, err := f.svc.InstallStatus(ctx)
	if err != nil || in.Phase != "failed" || in.Code != "" || !strings.Contains(in.Message, "pulling") ||
		in.From != "1.15.0" || in.To != "1.16.0" {
		t.Errorf("stale running install, companion down: %+v, %v", in, err)
	}
	if st, err := f.svc.UpdateState(ctx); err != nil || st.Install == nil || st.Install.Phase != "failed" ||
		!st.UpdaterConfigured || st.UpdaterAvailable {
		t.Errorf("UpdateState of a stale install: %+v, %v", st, err)
	}
	if f.svc.installRunning(ctx) {
		t.Error("a stale install still counts as running")
	}
	fu := newFakeUpdater(t)
	f.svc.WithUpdater(fu.URL, dir, nil)
	if in, err := f.svc.InstallStatus(ctx); err != nil || in.Phase != "pulling" || !f.svc.installRunning(ctx) {
		t.Errorf("stale running install, companion answers: %+v, %v", in, err)
	}

	for name, tc := range map[string]struct {
		st    updater.State
		shown bool
	}{
		"done to the running release":        {updater.State{Phase: updater.PhaseDone, From: "1.15.0", To: "1.16.0"}, true},
		"rolled back to the running release": {updater.State{Phase: updater.PhaseRolledBack, From: "1.16.0", To: "1.17.0"}, true},
		"an install before a manual update":  {updater.State{Phase: updater.PhaseDone, From: "1.14.0", To: "1.15.0"}, false},
		"a failure without versions":         {updater.State{Phase: updater.PhaseFailed, Code: updater.CodePullFailed}, false},
	} {
		tc.st.StartedAt, tc.st.FinishedAt = now.Add(-time.Hour), now.Add(-50*time.Minute)
		writeState(t, dir, tc.st)
		in, err := f.svc.InstallStatus(ctx)
		st, err2 := f.svc.UpdateState(ctx)
		if err := errors.Join(err, err2); err != nil {
			t.Fatal(err)
		}
		if tc.shown && (in.Phase != string(tc.st.Phase) || st.Install == nil) {
			t.Errorf("%s: %+v / %+v, want shown", name, in, st.Install)
		}
		if !tc.shown && (in.Phase != "idle" || st.Install != nil) {
			t.Errorf("%s: %+v / %+v, want idle and no install", name, in, st.Install)
		}
	}
}

// An install, a restart, a restore and a new backup exclude each other in the server (rule 12):
// the install answers restart_pending and backup_busy like the restart and holds the lock until
// the companion answered; while it waits for the answer and while state.json shows a running
// install, the restart, the restore and a new backup answer update_running.
func TestInstallUpdateExcludesRestartAndBackups(t *testing.T) {
	a := newActivityFixture(t)
	a.svc.WithBackups(t.TempDir(), "test").WithSystem(SystemSource{Version: "1.15.0"})
	a.svc.WithRestart(func() {})
	backup, err := a.svc.CreateBackup(a.ctx, model.BackupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	fu := newFakeUpdater(t)
	dir := updaterDir(t, fakeUpdaterToken)
	a.svc.WithUpdater(fu.URL, dir, nil)

	a.svc.backups.mu.Lock()
	_, err = a.svc.InstallUpdate(a.ctx)
	a.svc.backups.mu.Unlock()
	if !errors.Is(err, model.ErrBackupBusy) {
		t.Errorf("install during a backup = %v, want ErrBackupBusy", err)
	}
	a.svc.backups.restartPending.Store(true)
	if _, err := a.svc.InstallUpdate(a.ctx); !errors.Is(err, model.ErrRestartPending) {
		t.Errorf("install with a restart pending = %v, want ErrRestartPending", err)
	}
	a.svc.backups.restartPending.Store(false)
	if _, install := fu.calls(); install != 0 {
		t.Fatalf("a refused install reached the companion %d times", install)
	}

	// The companion takes its time to answer: the lock stays held, the others wait for nothing.
	gate := make(chan struct{})
	fu.mu.Lock()
	fu.installGate = gate
	fu.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := a.svc.InstallUpdate(a.ctx)
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !a.svc.ops.updater.starting.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := a.svc.RestartServer(a.ctx, a.f.Account.ID); !errors.Is(err, model.ErrUpdateRunning) {
		t.Errorf("restart while the install starts = %v, want ErrUpdateRunning", err)
	}
	if _, err := a.svc.CreateBackup(a.ctx, model.BackupRequest{}); !errors.Is(err, model.ErrUpdateRunning) {
		t.Errorf("backup while the install starts = %v, want ErrUpdateRunning", err)
	}
	if _, err := a.svc.RestoreBackup(a.ctx, backup.Name, ""); !errors.Is(err, model.ErrUpdateRunning) {
		t.Errorf("restore while the install starts = %v, want ErrUpdateRunning", err)
	}
	close(gate)
	if err := <-done; err != nil {
		t.Fatalf("install: %v", err)
	}
	if !a.svc.backups.mu.TryLock() {
		t.Fatal("the install kept the lock after the answer")
	}
	a.svc.backups.mu.Unlock()

	// state.json shows the running install (the companion wrote it with the 202).
	writeState(t, dir, updater.State{Phase: updater.PhasePulling, From: "1.15.0", To: "1.16.0", StartedAt: a.now.Add(-time.Minute)})
	if _, err := a.svc.RestartServer(a.ctx, a.f.Account.ID); !errors.Is(err, model.ErrUpdateRunning) {
		t.Errorf("restart during the install = %v, want ErrUpdateRunning", err)
	}
	if _, err := a.svc.CreateBackup(a.ctx, model.BackupRequest{}); !errors.Is(err, model.ErrUpdateRunning) {
		t.Errorf("backup during the install = %v, want ErrUpdateRunning", err)
	}
	if _, err := a.svc.RestoreBackup(a.ctx, backup.Name, ""); !errors.Is(err, model.ErrUpdateRunning) {
		t.Errorf("restore during the install = %v, want ErrUpdateRunning", err)
	}
	if a.svc.backups.restartPending.Load() || a.count(t, `kind = 'server_restart'`) != 0 {
		t.Error("a refused restart left a pending restart or a log row")
	}

	// Once it ended (here: pull_failed, the old server keeps running), all goes through again.
	writeState(t, dir, updater.State{Phase: updater.PhaseFailed, From: "1.15.0", To: "1.16.0", Code: updater.CodePullFailed,
		StartedAt: a.now.Add(-time.Minute), FinishedAt: a.now})
	if _, err := a.svc.CreateBackup(a.ctx, model.BackupRequest{}); err != nil {
		t.Errorf("backup after the install: %v", err)
	}
	if _, err := a.svc.RestartServer(a.ctx, a.f.Account.ID); err != nil {
		t.Errorf("restart after the install: %v", err)
	}
}

// After a 202 the watch follows the install at once: a failure while this server keeps running
// (pull_failed) is reported without any read of the admin pages, and a send that fails is tried
// again until it went out.
func TestWatchUpdateInstallFollowsAStartedInstall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newFixture(t)
	fu := newFakeUpdater(t)
	dir := updaterDir(t, fakeUpdaterToken)
	var mu sync.Mutex
	var tries int
	var sent []model.UpdateInstall
	f.svc.WithUpdater(fu.URL, dir, func(_ context.Context, in model.UpdateInstall) error {
		mu.Lock()
		defer mu.Unlock()
		tries++
		if tries < 3 {
			return errors.New("queue full")
		}
		sent = append(sent, in)
		return nil
	})
	f.svc.ops.updater.watchEvery = 10 * time.Millisecond
	watching := make(chan struct{})
	go func() {
		f.svc.WatchUpdateInstall(ctx)
		close(watching)
	}()
	time.Sleep(20 * time.Millisecond) // the start pass finds no state.json and waits for a kick
	at := time.Now().UTC()
	writeState(t, dir, updater.State{Phase: updater.PhaseBackingUp, From: "1.15.0", To: "1.16.0", StartedAt: at})
	if _, err := f.svc.InstallUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	writeState(t, dir, updater.State{Phase: updater.PhaseFailed, From: "1.15.0", To: "1.16.0", Code: updater.CodePullFailed,
		StartedAt: at, FinishedAt: at.Add(time.Second)})
	for {
		mu.Lock()
		n := len(sent)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("the failed install was not reported")
		case <-time.After(10 * time.Millisecond):
		}
	}
	mu.Lock()
	if tries != 3 || len(sent) != 1 || sent[0].Code != "pull_failed" {
		t.Errorf("tries %d, sent %+v; want two failed tries, then the report", tries, sent)
	}
	mu.Unlock()
	cancel()
	<-watching
}

// A 202 whose body does not read answers a running install (checking), not the previous
// state.json, which may hold a done install and reload the page at once.
func TestInstallUpdateAcceptedWithoutABody(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.svc.WithSystem(SystemSource{Version: "1.15.0"})
	fu := newFakeUpdater(t)
	dir := updaterDir(t, fakeUpdaterToken)
	f.svc.WithUpdater(fu.URL, dir, nil)
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	writeState(t, dir, updater.State{Phase: updater.PhaseDone, From: "1.14.0", To: "1.15.0", StartedAt: at, FinishedAt: at})
	fu.set(http.StatusAccepted, `not json`)
	in, err := f.svc.InstallUpdate(ctx)
	if err != nil || in.Phase != "checking" || in.To != "" {
		t.Errorf("202 without a readable body: %+v, %v", in, err)
	}
}

// A slow companion holds no read of the admin pages (the dashboard poll) for longer than
// updaterProbeWait, and
// readers at the same time share one probe instead of queueing behind it.
func TestUpdaterProbeDoesNotHoldTheDashboard(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	fu := newFakeUpdater(t)
	fu.statusDelay = 1500 * time.Millisecond
	f.svc.WithUpdater(fu.URL, updaterDir(t, fakeUpdaterToken), nil)
	start := time.Now()
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			if _, err := f.svc.UpdateState(ctx); err != nil { // the dashboard's update state
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if d := time.Since(start); d > time.Second {
		t.Errorf("three reads with a slow companion took %v", d)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := f.svc.UpdateState(ctx); err == nil && st.UpdaterAvailable {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status, _ := fu.calls(); status != 1 {
		t.Errorf("status calls = %d, want one shared probe", status)
	}
}
