package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/updater"
)

const (
	testDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	otherDig   = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testImage  = "ghcr.io/slouchcat/slouchcat@" + testDigest
	oldEnv     = "# Slouchcat\r\nSLOUCHCAT_VERSION=1.15.0\r\nSLOUCHCAT_SMTP_PASSWORD=secret-FAKE\r\nPUID=1000"
)

// fakeDocker records the compose calls and plays the server container.
type fakeDocker struct {
	mu        sync.Mutex
	c         Container
	digests   []string
	pullErr   error
	upErr     error
	pullGate  chan struct{} // when set, pull waits for it
	calls     [][]string
	ups       int
	afterUp   func(f *fakeDocker, n int) // called with the lock held after the n-th up (1-based)
	inspectFn func(f *fakeDocker) error
}

func (f *fakeDocker) Container(_ context.Context, name string) (Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name != "slouchcat" {
		return Container{}, errors.New("no such container")
	}
	if f.inspectFn != nil {
		if err := f.inspectFn(f); err != nil {
			return Container{}, err
		}
	}
	return f.c, nil
}

func (f *fakeDocker) ComposeImage(_ context.Context, p Project, service string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append(p.args(), "config", "--images", service))
	return "ghcr.io/slouchcat/slouchcat:1.16.0", nil
}

func (f *fakeDocker) RepoDigests(context.Context, string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.digests == nil {
		return nil, errors.New("no such image")
	}
	return f.digests, nil
}

func (f *fakeDocker) Compose(_ context.Context, p Project, args ...string) error {
	f.mu.Lock()
	gate := f.pullGate
	f.calls = append(f.calls, append(p.args(), args...))
	f.mu.Unlock()
	switch args[0] {
	case "pull":
		if gate != nil {
			<-gate
		}
		return f.pullErr
	case "up":
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.upErr != nil {
			return f.upErr
		}
		f.ups++
		if f.afterUp != nil {
			f.afterUp(f, f.ups)
		}
	}
	return nil
}

// verbs returns the compose subcommands called, in order ("pull", "config", "up").
func (f *fakeDocker) verbs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if i := slices.IndexFunc(c, func(a string) bool { return a == "pull" || a == "up" || a == "config" }); i >= 0 {
			out = append(out, c[i])
		}
	}
	return out
}

// harness is one deploy folder with a manifest server and an updater.
type harness struct {
	t        *testing.T
	deploy   string
	env      string
	fake     *fakeDocker
	manifest string
	u        *Updater
	token    []byte
	logs     *bytes.Buffer
	srv      *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	deploy, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, deploy: deploy, env: filepath.Join(deploy, ".env"), logs: &bytes.Buffer{},
		manifest: `{"version":"1.16.0","image":"` + testImage + `"}`}
	for _, f := range []string{"compose.yaml", "compose.intel.yaml"} {
		if err := os.WriteFile(filepath.Join(deploy, f), []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(h.env, []byte(oldEnv), 0o640); err != nil {
		t.Fatal(err)
	}
	h.fake = &fakeDocker{
		c: Container{
			Image:   "ghcr.io/slouchcat/slouchcat:1.15.0",
			Running: true, Health: "healthy",
			Labels: map[string]string{
				labelVersion:     "1.15.0",
				labelProject:     "slouchcat",
				labelWorkingDir:  deploy,
				labelConfigFiles: filepath.Join(deploy, "compose.yaml") + "," + filepath.Join(deploy, "compose.intel.yaml"),
			},
		},
		digests: []string{testImage},
	}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(h.manifest))
	}))
	t.Cleanup(h.srv.Close)
	return h
}

// start creates the updater (what main does at start-up).
func (h *harness) start() {
	h.t.Helper()
	log, err := logging.New(h.logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		h.t.Fatal(err)
	}
	cfg := settings{deployDir: h.deploy, uid: os.Getuid(), gid: os.Getgid(), manifestURL: h.srv.URL, container: "slouchcat"}
	u, token, err := newUpdater(h.t.Context(), cfg, h.fake, log)
	if err != nil {
		h.t.Fatal(err)
	}
	u.client = h.srv.Client()
	u.every, u.wait = 2*time.Millisecond, 150*time.Millisecond
	h.u, h.token = u, token
	h.t.Cleanup(u.wg.Wait)
}

// install posts /install with the token and returns the status and the decoded state.
func (h *harness) install() (int, updater.State) {
	return h.request(http.MethodPost, updater.PathInstall, "Bearer "+string(h.token))
}

func (h *harness) request(method, path, auth string) (int, updater.State) {
	h.t.Helper()
	req := httptest.NewRequestWithContext(h.t.Context(), method, path, http.NoBody)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.u.handler(h.token).ServeHTTP(rec, req)
	var s updater.State
	if rec.Code != http.StatusUnauthorized {
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			h.t.Fatalf("%s %s: body %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code, s
}

// finish waits for the install goroutine and returns the final state, checking that state.json
// holds the same.
func (h *harness) finish() updater.State {
	h.t.Helper()
	h.u.wg.Wait()
	s := h.u.State()
	var disk updater.State
	if err := updater.ReadJSON(filepath.Join(h.deploy, "updater", updater.StateFile), &disk); err != nil {
		h.t.Fatal(err)
	}
	if !disk.StartedAt.Equal(s.StartedAt) || !disk.FinishedAt.Equal(s.FinishedAt) {
		h.t.Errorf("state.json times differ from the state")
	}
	disk.StartedAt, disk.FinishedAt = s.StartedAt, s.FinishedAt
	if disk != s {
		h.t.Errorf("state.json = %+v, state = %+v", disk, s)
	}
	if strings.Contains(h.logs.String(), string(h.token)) {
		h.t.Error("the token appears in the log")
	}
	return s
}

func (h *harness) readEnv() string {
	h.t.Helper()
	data, err := os.ReadFile(h.env)
	if err != nil {
		h.t.Fatal(err)
	}
	return string(data)
}

// writeServer writes server.json as the new server would.
func (h *harness) writeServer(rec updater.ServerRecord) {
	h.t.Helper()
	if err := updater.WriteJSON(filepath.Join(h.deploy, "updater", updater.ServerFile), rec, 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func newEnv() string { return strings.Replace(oldEnv, "1.15.0", "1.16.0", 1) }

func TestInstallSuccess(t *testing.T) {
	h := newHarness(t)
	migratedAt := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	h.fake.afterUp = func(f *fakeDocker, _ int) {
		f.c.Labels[labelVersion] = "1.16.0"
		f.c.Health = "starting"
		f.inspectFn = func(f *fakeDocker) error {
			// The new server migrates while unhealthy, then writes its record and becomes healthy.
			h.writeServer(updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, SchemaAfter: 38,
				Backup: "pre-migration-00037.db", MigratedAt: migratedAt})
			f.c.Health = "healthy"
			f.inspectFn = nil
			return nil
		}
	}
	h.start()
	status, s := h.install()
	if status != http.StatusAccepted || s.Phase != updater.PhaseBackingUp || s.From != "1.15.0" || s.To != "1.16.0" {
		t.Fatalf("install = %d %+v", status, s)
	}
	s = h.finish()
	if s.Phase != updater.PhaseDone || s.Code != "" || !s.Migrated || s.Backup != "pre-migration-00037.db" || s.FinishedAt.IsZero() {
		t.Errorf("final state = %+v", s)
	}
	if got := h.readEnv(); got != newEnv() {
		t.Errorf(".env = %q, want %q", got, newEnv())
	}
	bak := h.env + ".bak-vor-1.16.0"
	data, err := os.ReadFile(bak)
	if err != nil || string(data) != oldEnv {
		t.Errorf("backup = %q, %v", data, err)
	}
	if info, _ := os.Stat(bak); info == nil || info.Mode().Perm() != 0o600 {
		t.Errorf("backup mode = %v", info)
	}
	if info, _ := os.Stat(h.env); info == nil || info.Mode().Perm() != 0o640 {
		t.Errorf(".env mode after the install = %v, want 0640 as before", info)
	}
	if info, _ := os.Stat(filepath.Join(h.deploy, "updater", updater.StateFile)); info == nil || info.Mode().Perm() != 0o644 {
		t.Errorf("state.json mode = %v", info)
	}
	if got, want := h.fake.verbs(), []string{"pull", "config", "up"}; !slices.Equal(got, want) {
		t.Errorf("compose calls = %v, want %v", got, want)
	}
	wantUp := []string{"compose", "--project-name", "slouchcat", "--project-directory", h.deploy,
		"--env-file", h.env, "-f", filepath.Join(h.deploy, "compose.yaml"), "-f", filepath.Join(h.deploy, "compose.intel.yaml"),
		"up", "-d", "--no-deps", "--no-build", "slouchcat"}
	if got := h.fake.calls[len(h.fake.calls)-1]; !slices.Equal(got, wantUp) {
		t.Errorf("up = %q\nwant %q", got, wantUp)
	}
}

func TestInstallRefusals(t *testing.T) {
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "compose.yaml")
	if err := os.WriteFile(outsideFile, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		setup func(h *harness)
		code  updater.Code
	}{
		{"not newer", func(h *harness) { h.manifest = `{"version":"1.15.0","image":"` + testImage + `"}` }, updater.CodeNotNewer},
		{"older", func(h *harness) { h.manifest = `{"version":"1.14.0","image":"` + testImage + `"}` }, updater.CodeNotNewer},
		{"dev build", func(h *harness) { h.fake.c.Labels[labelVersion] = "dev" }, updater.CodeNotNewer},
		{"line break in version", func(h *harness) { h.manifest = `{"version":"1.16.0\nX=1","image":"` + testImage + `"}` }, updater.CodeManifestInvalid},
		{"pre-release", func(h *harness) { h.manifest = `{"version":"1.16.0-rc.1","image":"` + testImage + `"}` }, updater.CodeManifestInvalid},
		{"not json", func(h *harness) { h.manifest = `<html>` }, updater.CodeManifestInvalid},
		{"too large", func(h *harness) { h.manifest = `{"version":"1.16.0","x":"` + strings.Repeat("a", 70<<10) + `"}` }, updater.CodeManifestInvalid},
		{"unreachable", func(h *harness) { h.srv.Close() }, updater.CodeManifestUnreachable},
		{"no digest", func(h *harness) { h.manifest = `{"version":"1.16.0"}` }, updater.CodeNoDigest},
		{"tag only", func(h *harness) { h.manifest = `{"version":"1.16.0","image":"ghcr.io/slouchcat/slouchcat:1.16.0"}` }, updater.CodeNoDigest},
		{"no labels", func(h *harness) {
			for _, k := range []string{labelProject, labelWorkingDir, labelConfigFiles} {
				delete(h.fake.c.Labels, k)
			}
		}, updater.CodeNoComposeLabels},
		{"no container", func(h *harness) { h.fake.inspectFn = func(*fakeDocker) error { return errors.New("gone") } }, updater.CodeNoComposeLabels},
		{"config file outside", func(h *harness) {
			h.fake.c.Labels[labelConfigFiles] += "," + outsideFile
		}, updater.CodeOutsideDeployDir},
		{"symlink pointing outside", func(h *harness) {
			link := filepath.Join(h.deploy, "compose.link.yaml")
			if err := os.Symlink(outsideFile, link); err != nil {
				t.Fatal(err)
			}
			h.fake.c.Labels[labelConfigFiles] += "," + link
		}, updater.CodeOutsideDeployDir},
		{"dot dot", func(h *harness) {
			rel, err := filepath.Rel(h.deploy, outsideFile)
			if err != nil {
				t.Fatal(err)
			}
			h.fake.c.Labels[labelConfigFiles] = h.deploy + "/" + rel // not cleaned, as a label could be
		}, updater.CodeOutsideDeployDir},
		{"env file outside", func(h *harness) { h.fake.c.Labels[labelEnvFile] = filepath.Join(outside, ".env") }, updater.CodeOutsideDeployDir},
		{"working dir outside", func(h *harness) { h.fake.c.Labels[labelWorkingDir] = outside }, updater.CodeOutsideDeployDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.start()
			tc.setup(h)
			status, s := h.install()
			if status != http.StatusUnprocessableEntity || s.Phase != updater.PhaseFailed || s.Code != tc.code || s.Message == "" {
				t.Fatalf("install = %d %+v, want 422 %s", status, s, tc.code)
			}
			if got := h.u.State(); got.Phase != updater.PhaseIdle {
				t.Errorf("state after a refusal = %+v, want idle", got)
			}
			var disk updater.State
			if err := updater.ReadJSON(filepath.Join(h.deploy, "updater", updater.StateFile), &disk); err != nil || disk.Phase != updater.PhaseIdle {
				t.Errorf("state.json after a refusal = %+v, %v", disk, err)
			}
			if got := h.readEnv(); got != oldEnv {
				t.Errorf(".env changed by a refusal: %q", got)
			}
			if v := h.fake.verbs(); len(v) != 0 {
				t.Errorf("compose ran on a refusal: %v", v)
			}
		})
	}
}

func TestInstallPullFailedRestoresEnv(t *testing.T) {
	h := newHarness(t)
	h.fake.pullErr = errors.New("exit status 1")
	h.start()
	if status, _ := h.install(); status != http.StatusAccepted {
		t.Fatalf("install = %d", status)
	}
	s := h.finish()
	if s.Phase != updater.PhaseFailed || s.Code != updater.CodePullFailed {
		t.Errorf("state = %+v", s)
	}
	if got := h.readEnv(); got != oldEnv {
		t.Errorf(".env = %q, want the original", got)
	}
}

func TestInstallDigestMismatch(t *testing.T) {
	h := newHarness(t)
	h.fake.digests = []string{"ghcr.io/slouchcat/slouchcat@" + otherDig}
	h.start()
	if status, _ := h.install(); status != http.StatusAccepted {
		t.Fatalf("install = %d", status)
	}
	s := h.finish()
	if s.Phase != updater.PhaseFailed || s.Code != updater.CodeDigestMismatch {
		t.Errorf("state = %+v", s)
	}
	if got := h.readEnv(); got != oldEnv {
		t.Errorf(".env = %q, want the original byte for byte", got)
	}
	if got := h.fake.verbs(); slices.Contains(got, "up") {
		t.Errorf("restarted despite the mismatch: %v", got)
	}
}

func TestInstallDigestFromMirror(t *testing.T) {
	// The digest decides, not the repository name.
	h := newHarness(t)
	h.fake.digests = []string{"mirror.example.test/slouchcat@" + testDigest}
	h.fake.afterUp = func(f *fakeDocker, _ int) {
		h.writeServer(updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, SchemaAfter: 37, MigratedAt: time.Now()})
	}
	h.start()
	h.install()
	if s := h.finish(); s.Phase != updater.PhaseDone || s.Migrated {
		t.Errorf("state = %+v", s)
	}
}

func TestInstallUnhealthyRollsBack(t *testing.T) {
	h := newHarness(t)
	h.fake.afterUp = func(f *fakeDocker, n int) {
		if n == 1 {
			// The new version never becomes healthy and never got to its migrations.
			f.c.Health = "unhealthy"
			return
		}
		f.c.Health = "healthy" // the old version again
	}
	h.start()
	h.install()
	s := h.finish()
	if s.Phase != updater.PhaseRolledBack || s.Code != updater.CodeUnhealthy || s.Migrated {
		t.Errorf("state = %+v", s)
	}
	if got := h.readEnv(); got != oldEnv {
		t.Errorf(".env = %q, want the original", got)
	}
	if got := h.fake.verbs(); !slices.Equal(got, []string{"pull", "config", "up", "up"}) {
		t.Errorf("compose calls = %v", got)
	}
}

func TestInstallUnhealthyStaleRecordRollsBack(t *testing.T) {
	// A server.json of the old version without a finished migration is not the new one's.
	h := newHarness(t)
	h.start()
	h.writeServer(updater.ServerRecord{Version: "1.15.0", SchemaBefore: 37})
	h.fake.afterUp = func(f *fakeDocker, n int) {
		f.c.Health = map[bool]string{true: "unhealthy", false: "healthy"}[n == 1]
	}
	h.install()
	if s := h.finish(); s.Phase != updater.PhaseRolledBack {
		t.Errorf("state = %+v", s)
	}
}

func TestInstallUnhealthyAfterMigrationKeepsNewVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		rec  updater.ServerRecord
	}{
		{"migrations still running", updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, Backup: "pre-migration-00037.db"}},
		{"migrated then failed", updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, SchemaAfter: 38,
			Backup: "pre-migration-00037.db", MigratedAt: time.Now()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.fake.afterUp = func(f *fakeDocker, _ int) {
				f.c.Health = "unhealthy"
				h.writeServer(tc.rec)
			}
			h.start()
			if status, s := h.install(); status != http.StatusAccepted {
				t.Fatalf("install = %d %+v", status, s)
			}
			s := h.finish()
			if s.Phase != updater.PhaseFailed || s.Code != updater.CodeMigratedNoRollback || !s.Migrated ||
				s.Backup != "pre-migration-00037.db" || !strings.Contains(s.Message, "pre-migration-00037.db") {
				t.Errorf("state = %+v", s)
			}
			if got := h.readEnv(); got != newEnv() {
				t.Errorf(".env = %q, want the new version kept", got)
			}
			if got := h.fake.verbs(); !slices.Equal(got, []string{"pull", "config", "up"}) {
				t.Errorf("compose calls = %v, want no rollback", got)
			}
		})
	}
}

func TestInstallRollbackFailed(t *testing.T) {
	h := newHarness(t)
	h.fake.afterUp = func(f *fakeDocker, _ int) { f.c.Health = "unhealthy" }
	h.start()
	h.install()
	s := h.finish()
	if s.Phase != updater.PhaseFailed || s.Code != updater.CodeRollbackFailed {
		t.Errorf("state = %+v", s)
	}
	if got := h.readEnv(); got != oldEnv {
		t.Errorf(".env = %q, want the original", got)
	}
}

func TestInstallUpFailedRollsBack(t *testing.T) {
	h := newHarness(t)
	h.start()
	h.fake.upErr = errors.New("exit status 1")
	h.fake.afterUp = nil
	h.install()
	s := h.finish()
	// The second up fails as well: the rollback could not start the old version.
	if s.Phase != updater.PhaseFailed || s.Code != updater.CodeRollbackFailed {
		t.Errorf("state = %+v", s)
	}
	if got := h.readEnv(); got != oldEnv {
		t.Errorf(".env = %q, want the original", got)
	}
}

func TestInstallBusy(t *testing.T) {
	h := newHarness(t)
	gate := make(chan struct{})
	h.fake.pullGate = gate
	h.fake.afterUp = func(f *fakeDocker, _ int) {
		h.writeServer(updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, SchemaAfter: 37, MigratedAt: time.Now()})
	}
	h.start()
	if status, _ := h.install(); status != http.StatusAccepted {
		t.Fatalf("first install = %d", status)
	}
	status, s := h.install()
	if status != http.StatusConflict || s.Code != updater.CodeBusy || !s.Phase.Running() || s.To != "1.16.0" {
		t.Errorf("second install = %d %+v, want 409 busy", status, s)
	}
	if _, st := h.request(http.MethodGet, updater.PathStatus, "Bearer "+string(h.token)); !st.Phase.Running() {
		t.Errorf("status while running = %+v", st)
	}
	close(gate)
	if s := h.finish(); s.Phase != updater.PhaseDone {
		t.Errorf("state = %+v", s)
	}
}

func TestAuth(t *testing.T) {
	h := newHarness(t)
	h.start()
	for _, auth := range []string{"", "Bearer", "Bearer ", "Bearer wrong", "Basic " + string(h.token),
		"bearer" + string(h.token), "Bearer " + string(h.token) + "x"} {
		for _, path := range []string{updater.PathStatus, updater.PathInstall} {
			method := map[string]string{updater.PathStatus: http.MethodGet, updater.PathInstall: http.MethodPost}[path]
			if status, _ := h.request(method, path, auth); status != http.StatusUnauthorized {
				t.Errorf("%s %s with %q = %d, want 401", method, path, auth, status)
			}
		}
	}
	if v := h.fake.verbs(); len(v) != 0 {
		t.Errorf("compose ran without the token: %v", v)
	}
	status, s := h.request(http.MethodGet, updater.PathStatus, "Bearer "+string(h.token))
	if status != http.StatusOK || s.Phase != updater.PhaseIdle {
		t.Errorf("status = %d %+v", status, s)
	}
}

func TestStartCreatesTokenAndFolder(t *testing.T) {
	h := newHarness(t)
	h.start()
	dir := filepath.Join(h.deploy, "updater")
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("folder = %v, %v", info, err)
	}
	path := filepath.Join(dir, updater.TokenFile)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("token file = %v, %v", info, err)
	}
	if len(h.token) != 64 || strings.Trim(string(h.token), "0123456789abcdef") != "" {
		t.Errorf("token is not 32 bytes of hex: %d characters", len(h.token))
	}
	first := string(h.token)
	h.start()
	if string(h.token) != first {
		t.Error("a second start replaced the token")
	}
}

func TestStartMarksInterruptedInstallFailed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rec      *updater.ServerRecord
		code     updater.Code
		migrated bool
	}{
		{"before the restart", nil, "", false},
		{"during a migration", &updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, Backup: "pre-migration-00037.db"},
			updater.CodeMigratedNoRollback, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			dir := filepath.Join(h.deploy, "updater")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			started := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
			if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile),
				updater.State{Phase: updater.PhasePulling, From: "1.15.0", To: "1.16.0", StartedAt: started}, 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.rec != nil {
				h.writeServer(*tc.rec)
			}
			h.start()
			s := h.u.State()
			if s.Phase != updater.PhaseFailed || s.Code != tc.code || s.Migrated != tc.migrated ||
				s.FinishedAt.IsZero() || !strings.Contains(s.Message, "pulling") || s.To != "1.16.0" {
				t.Errorf("state = %+v", s)
			}
			var disk updater.State
			if err := updater.ReadJSON(filepath.Join(dir, updater.StateFile), &disk); err != nil || disk.Phase != updater.PhaseFailed {
				t.Errorf("state.json = %+v, %v", disk, err)
			}
			// A finished state stays as it is.
			h.start()
			if again := h.u.State(); !again.FinishedAt.Equal(s.FinishedAt) {
				t.Errorf("a second start changed the finished state: %+v", again)
			}
		})
	}
}

func TestRunningVersion(t *testing.T) {
	for _, tc := range []struct {
		image, label, want string
	}{
		{"ghcr.io/slouchcat/slouchcat:1.15.0", "1.15.0", "1.15.0"},
		// The label wins: a branch tag or latest says nothing.
		{"ghcr.io/slouchcat/slouchcat:hintergrundjobs", "v1.4.0-26-gced5636", "v1.4.0-26-gced5636"},
		{"ghcr.io/slouchcat/slouchcat:latest", "1.15.0", "1.15.0"},
		{"ghcr.io/slouchcat/slouchcat:1.15.0", "", "1.15.0"},
		{"localhost:5000/slouchcat:1.15.0@sha256:abc", "", "1.15.0"},
		{"localhost:5000/slouchcat", "", ""},
	} {
		c := Container{Image: tc.image, Labels: map[string]string{}}
		if tc.label != "" {
			c.Labels[labelVersion] = tc.label
		}
		if got := runningVersion(c); got != tc.want {
			t.Errorf("runningVersion(%q, %q) = %q, want %q", tc.image, tc.label, got, tc.want)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{"SLOUCHCAT_DEPLOY_DIR": dir}
	c, err := loadConfig(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.uid != 1000 || c.gid != 1000 || c.container != "slouchcat" || c.manifestURL == "" {
		t.Errorf("defaults = %+v", c)
	}
	for _, bad := range []map[string]string{
		{},
		{"SLOUCHCAT_DEPLOY_DIR": "deploy"},
		{"SLOUCHCAT_DEPLOY_DIR": filepath.Join(dir, "missing")},
		{"SLOUCHCAT_DEPLOY_DIR": dir, "PUID": "abc"},
		{"SLOUCHCAT_DEPLOY_DIR": dir, "PGID": "-1"},
	} {
		if _, err := loadConfig(func(k string) string { return bad[k] }); err == nil {
			t.Errorf("loadConfig(%v) accepted", bad)
		}
	}
	env = map[string]string{"SLOUCHCAT_DEPLOY_DIR": dir, "PUID": "568", "PGID": "569", "SLOUCHCAT_CONTAINER": "media", "SLOUCHCAT_UPDATE_MANIFEST_URL": "http://x.test/m.json"}
	c, err = loadConfig(func(k string) string { return env[k] })
	if err != nil || c.uid != 568 || c.gid != 569 || c.container != "media" || c.manifestURL != "http://x.test/m.json" {
		t.Errorf("loadConfig = %+v, %v", c, err)
	}
}

func TestDockerEnv(t *testing.T) {
	env := map[string]string{"PATH": "/usr/bin", "HOME": "/root", "DOCKER_HOST": "unix:///x.sock",
		"SLOUCHCAT_DEPLOY_DIR": "/srv/deploy", "SLOUCHCAT_VERSION": "9.9.9", "COMPOSE_FILE": "x.yaml"}
	got := dockerEnv(func(k string) string { return env[k] })
	want := []string{"PATH=/usr/bin", "HOME=/root", "DOCKER_HOST=unix:///x.sock"}
	if !slices.Equal(got, want) {
		t.Errorf("dockerEnv = %v, want %v", got, want)
	}
}

func TestProjectArgsWithoutEnvLabel(t *testing.T) {
	h := newHarness(t)
	h.start()
	p, r := h.u.project(h.fake.c)
	if r != nil {
		t.Fatal(r)
	}
	if !slices.Equal(p.EnvFiles, []string{h.env}) {
		t.Errorf("env files = %v, want <working dir>/.env", p.EnvFiles)
	}
	if _, err := os.Stat(h.env + ".bak-vor-1.16.0"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a backup exists before any install: %v", err)
	}
}
