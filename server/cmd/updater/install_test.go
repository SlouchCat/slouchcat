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
	"net/url"
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
	f.calls = append(f.calls, append(p.args(), "config", "--format", "json", service))
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

func (f *fakeDocker) Compose(ctx context.Context, p Project, args ...string) error {
	f.mu.Lock()
	gate := f.pullGate
	f.calls = append(f.calls, append(p.args(), args...))
	f.mu.Unlock()
	switch args[0] {
	case "pull":
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
				return ctx.Err()
			}
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
	stop     context.CancelFunc // ends the updater's process context (SIGTERM)
	wait     time.Duration      // u.wait when set; start applies it before Resume
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
	h.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loop" {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		if to := r.URL.Query().Get("redirect"); to != "" {
			http.Redirect(w, r, to, http.StatusFound)
			return
		}
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
	ctx, stop := context.WithCancel(h.t.Context())
	h.t.Cleanup(stop)
	u, token, err := newUpdater(ctx, cfg, h.fake, log)
	if err != nil {
		h.t.Fatal(err)
	}
	h.stop = stop
	u.client.Transport = h.srv.Client().Transport // trusts the test certificate; CheckRedirect stays
	u.every, u.wait = 2*time.Millisecond, 150*time.Millisecond
	if h.wait != 0 {
		u.wait = h.wait
	}
	h.u, h.token = u, token
	h.t.Cleanup(u.wg.Wait)
	u.Resume() // as main does
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
	if s.EnvBackup != ".env.bak-vor-1.16.0" {
		t.Errorf("EnvBackup = %q", s.EnvBackup)
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
		{"http address", func(h *harness) { h.u.cfg.manifestURL = strings.Replace(h.srv.URL, "https:", "http:", 1) }, updater.CodeManifestUnreachable},
		{"redirect to http", func(h *harness) {
			h.u.cfg.manifestURL = h.srv.URL + "/?redirect=" + url.QueryEscape(strings.Replace(h.srv.URL, "https:", "http:", 1))
		}, updater.CodeManifestUnreachable},
		{"redirect loop", func(h *harness) {
			h.u.cfg.manifestURL = h.srv.URL + "/loop"
		}, updater.CodeManifestUnreachable},
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
		// The shared folder deploy/updater/ is the server's to write: nothing compose reads may lie there.
		{"config file in the shared folder", func(h *harness) {
			planted := filepath.Join(h.deploy, "updater", "compose.yaml")
			if err := os.WriteFile(planted, []byte("services: {}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			h.fake.c.Labels[labelConfigFiles] += "," + planted
		}, updater.CodeOutsideDeployDir},
		{"env file in the shared folder", func(h *harness) {
			h.fake.c.Labels[labelEnvFile] = filepath.Join(h.deploy, "updater", ".env")
		}, updater.CodeOutsideDeployDir},
		{"env file linked into the shared folder", func(h *harness) {
			target := filepath.Join(h.deploy, "updater", "slouchcat.env")
			if err := os.WriteFile(target, []byte(oldEnv), 0o600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(h.deploy, "linked.env")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			h.fake.c.Labels[labelEnvFile] = link
		}, updater.CodeOutsideDeployDir},
		{"working dir is the shared folder", func(h *harness) {
			h.fake.c.Labels[labelWorkingDir] = filepath.Join(h.deploy, "updater")
		}, updater.CodeOutsideDeployDir},
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

// An existing backup of the same name (made by hand, or by an earlier install of the version) is
// never replaced: the install takes -2, -3, …, restores the env file from memory and names the
// backup in state.json.
func TestInstallKeepsExistingBackup(t *testing.T) {
	h := newHarness(t)
	handmade := h.env + ".bak-vor-1.16.0"
	if err := os.WriteFile(handmade, []byte("by hand"), 0o640); err != nil {
		t.Fatal(err)
	}
	h.fake.pullErr = errors.New("exit status 1")
	h.start()
	for _, want := range []string{".env.bak-vor-1.16.0-2", ".env.bak-vor-1.16.0-3"} {
		if status, _ := h.install(); status != http.StatusAccepted {
			t.Fatalf("install = %d", status)
		}
		s := h.finish()
		if s.Phase != updater.PhaseFailed || s.EnvBackup != want {
			t.Errorf("state = %+v, want EnvBackup %q", s, want)
		}
		if got := h.readEnv(); got != oldEnv {
			t.Errorf(".env = %q, want the original", got)
		}
		if got, err := os.ReadFile(filepath.Join(h.deploy, want)); err != nil || string(got) != oldEnv {
			t.Errorf("%s = %q, %v", want, got, err)
		}
		if got, err := os.ReadFile(handmade); err != nil || string(got) != "by hand" {
			t.Errorf("the hand-made backup = %q, %v, want it unchanged", got, err)
		}
	}
	if info, _ := os.Stat(handmade); info == nil || info.Mode().Perm() != 0o640 {
		t.Errorf("hand-made backup mode = %v, want 0640 as before", info)
	}
}

// A start after an interruption puts back the backup the install named in state.json, not the
// plain name; a stored name that does not fit is refused and the env file stays as it is.
func TestStartRecoversStoredBackup(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stored  string
		link    bool // stored is a symbolic link to a file with oldEnv
		wantEnv string
		message string
	}{
		{name: "stored name", stored: ".env.bak-vor-1.16.0-2", wantEnv: oldEnv, message: "put back from .env.bak-vor-1.16.0-2"},
		{name: "no stored name: newest match", stored: "", wantEnv: oldEnv, message: "put back from .env.bak-vor-1.16.0-2"},
		{name: "path separator", stored: "../.env.bak-vor-1.16.0-2", wantEnv: newEnv(), message: "stored backup name is not valid"},
		{name: "absolute path", stored: "/etc/passwd", wantEnv: newEnv(), message: "stored backup name is not valid"},
		{name: "other version", stored: ".env.bak-vor-1.17.0", wantEnv: newEnv(), message: "stored backup name is not valid"},
		{name: "symbolic link", stored: ".env.bak-vor-1.16.0-3", link: true, wantEnv: newEnv(), message: "no backup .env.bak-vor-1.16.0-3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			dir := filepath.Join(h.deploy, "updater")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.env, []byte(newEnv()), 0o640); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.env+".bak-vor-1.16.0", []byte("by hand"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.env+".bak-vor-1.16.0-2", []byte(oldEnv), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.link {
				victim := filepath.Join(t.TempDir(), "secret")
				if err := os.WriteFile(victim, []byte(oldEnv), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(victim, h.env+".bak-vor-1.16.0-3"); err != nil {
					t.Fatal(err)
				}
			}
			h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.15.0", "healthy"
			if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile), updater.State{Phase: updater.PhasePulling,
				From: "1.15.0", To: "1.16.0", StartedAt: time.Now(), EnvBackup: tc.stored}, 0o644); err != nil {
				t.Fatal(err)
			}
			h.start()
			h.u.wg.Wait()
			if s := h.u.State(); s.Phase != updater.PhaseFailed || !strings.Contains(s.Message, tc.message) {
				t.Errorf("state = %+v, want message containing %q", s, tc.message)
			}
			if got := h.readEnv(); got != tc.wantEnv {
				t.Errorf(".env = %q, want %q", got, tc.wantEnv)
			}
			if got, _ := os.ReadFile(h.env + ".bak-vor-1.16.0"); string(got) != "by hand" {
				t.Errorf("the hand-made backup = %q, want it unchanged", got)
			}
		})
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
	if s.Phase != updater.PhaseRolledBack || s.Code != updater.CodeUnhealthy || s.Migrated ||
		!strings.Contains(s.Message, "version 1.16.0 did not become healthy within") ||
		!strings.Contains(s.Message, "version 1.15.0 runs again") {
		t.Errorf("state = %+v", s)
	}
	if got := h.readEnv(); got != oldEnv {
		t.Errorf(".env = %q, want the original", got)
	}
	if got := h.fake.verbs(); !slices.Equal(got, []string{"pull", "config", "up", "up"}) {
		t.Errorf("compose calls = %v", got)
	}
}

// A new container that runs healthy but never writes server.json for its version (a release
// before 1.16.0) is not called unhealthy: the message says it did not report back.
func TestInstallHealthyButSilentRollsBack(t *testing.T) {
	h := newHarness(t)
	h.fake.afterUp = func(f *fakeDocker, _ int) { f.c.Health = "healthy" }
	h.start()
	h.install()
	s := h.finish()
	if s.Phase != updater.PhaseRolledBack || s.Code != updater.CodeUnhealthy || s.Migrated ||
		!strings.Contains(s.Message, "version 1.16.0 runs healthy but did not report back (server.json) within") ||
		strings.Contains(s.Message, "did not become healthy") ||
		!strings.Contains(s.Message, "version 1.15.0 runs again") {
		t.Errorf("state = %+v", s)
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

// A new container that crashes for good ends the wait at once, not after u.wait.
func TestInstallStopsWaitingForCrashedContainer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		crash Container
	}{
		{"crash loop", Container{Running: false, Status: "restarting", RestartCount: 3, RestartPolicy: "unless-stopped", ExitCode: 1}},
		{"exited without restart", Container{Running: false, Status: "exited", RestartPolicy: "no", ExitCode: 1}},
		{"on-failure retries used up", Container{Running: false, Status: "exited", RestartCount: 2, RestartPolicy: "on-failure", MaxRetries: 2, ExitCode: 1}},
		{"dead", Container{Running: false, Status: "dead"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.fake.afterUp = func(f *fakeDocker, n int) {
				if n == 1 {
					crash := tc.crash
					crash.Image, crash.Labels = f.c.Image, f.c.Labels
					f.c = crash
					return
				}
				f.c.Running, f.c.Status, f.c.Health, f.c.RestartCount = true, "running", "healthy", 0
			}
			h.start()
			h.u.wait = time.Minute
			begin := time.Now()
			h.install()
			s := h.finish()
			if s.Phase != updater.PhaseRolledBack || s.Code != updater.CodeUnhealthy || !strings.Contains(s.Message, "version 1.16.0 stopped (exited or restarted 3 times) before it became healthy") {
				t.Errorf("state = %+v", s)
			}
			if d := time.Since(begin); d > 20*time.Second {
				t.Errorf("waited %s for a crashed container", d)
			}
		})
	}
}

func TestContainerGaveUp(t *testing.T) {
	for _, tc := range []struct {
		c    Container
		want bool
	}{
		{Container{Running: true, Status: "running", RestartCount: 5, Health: "unhealthy"}, false}, // runs: may still migrate
		{Container{Status: "restarting", RestartCount: 2, RestartPolicy: "unless-stopped"}, false},
		{Container{Status: "restarting", RestartCount: 3, RestartPolicy: "unless-stopped"}, true},
		{Container{Status: "exited", RestartPolicy: "always", ExitCode: 1}, false},
		{Container{Status: "exited", RestartPolicy: "no", ExitCode: 0}, true},
		{Container{Status: "exited", RestartPolicy: "", ExitCode: 1}, true},
		{Container{Status: "exited", RestartPolicy: "on-failure", ExitCode: 0}, true},
		{Container{Status: "exited", RestartPolicy: "on-failure", ExitCode: 1, RestartCount: 1}, false},
		{Container{Status: "exited", RestartPolicy: "on-failure", ExitCode: 1, MaxRetries: 1, RestartCount: 1}, true},
		{Container{Status: "created"}, false},
		{Container{Status: "dead"}, true},
	} {
		if got := tc.c.GaveUp(); got != tc.want {
			t.Errorf("GaveUp(%+v) = %v, want %v", tc.c, got, tc.want)
		}
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

// A SIGTERM during the install: no rollback with a dead context, no final state; state.json keeps
// the running phase for the next start.
func TestInstallStopsOnShutdown(t *testing.T) {
	t.Run("while pulling", func(t *testing.T) {
		h := newHarness(t)
		h.fake.pullGate = make(chan struct{})
		h.start()
		if status, _ := h.install(); status != http.StatusAccepted {
			t.Fatalf("install = %d", status)
		}
		waitPhase(t, h, updater.PhasePulling)
		h.stop()
		if !h.u.drain(5 * time.Second) {
			t.Fatal("the install did not stop")
		}
		if s := diskState(t, h); s.Phase != updater.PhasePulling || !s.FinishedAt.IsZero() {
			t.Errorf("state.json = %+v, want the phase pulling kept", s)
		}
		if got := h.readEnv(); got != oldEnv {
			t.Errorf(".env = %q, want the original (nothing was restarted)", got)
		}
	})
	t.Run("while waiting", func(t *testing.T) {
		h := newHarness(t)
		h.fake.afterUp = func(f *fakeDocker, _ int) { f.c.Health = "starting" } // a long migration
		h.start()
		h.u.wait = time.Minute
		h.install()
		waitPhase(t, h, updater.PhaseWaiting)
		h.stop()
		if !h.u.drain(5 * time.Second) {
			t.Fatal("the install did not stop")
		}
		if s := diskState(t, h); s.Phase != updater.PhaseWaiting || s.Code != "" || !s.FinishedAt.IsZero() {
			t.Errorf("state.json = %+v, want the phase waiting kept", s)
		}
		if got := h.readEnv(); got != newEnv() {
			t.Errorf(".env = %q, want the new version kept (no rollback)", got)
		}
		if got := h.fake.verbs(); !slices.Equal(got, []string{"pull", "config", "up"}) {
			t.Errorf("compose calls = %v, want no rollback", got)
		}
	})
}

// waitPhase waits until the install reached phase ph.
func waitPhase(t *testing.T, h *harness, ph updater.Phase) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.u.State().Phase != ph {
		if time.Now().After(deadline) {
			t.Fatalf("phase = %s, want %s", h.u.State().Phase, ph)
		}
		time.Sleep(time.Millisecond)
	}
}

// diskState reads state.json.
func diskState(t *testing.T, h *harness) updater.State {
	t.Helper()
	var s updater.State
	if err := updater.ReadJSON(filepath.Join(h.deploy, "updater", updater.StateFile), &s); err != nil {
		t.Fatal(err)
	}
	return s
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

func TestStartEndsInterruptedInstall(t *testing.T) {
	migrating := &updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, Backup: "pre-migration-00037.db"}
	migrated := &updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, SchemaAfter: 38, Backup: "pre-migration-00037.db", MigratedAt: time.Now()}
	// No comma in a name: the temporary folder carries it, and the compose label lists files
	// separated by commas.
	for _, tc := range []struct {
		name   string
		phase  updater.Phase
		env    string // content of .env at the start
		backup bool   // .env.bak-vor-1.16.0 with oldEnv exists
		// linkBackup: .env.bak-vor-1.16.0 is a symbolic link to a file with oldEnv; it is not read.
		linkBackup bool
		running    string // version the container runs
		health     string
		rec        *updater.ServerRecord
		want       updater.Phase
		code       updater.Code
		migrated   bool
		wantEnv    string
		message    string
	}{
		{name: "checking", phase: updater.PhaseChecking, env: oldEnv, running: "1.15.0", health: "healthy",
			want: updater.PhaseFailed, wantEnv: oldEnv, message: "nothing was changed"},
		{name: "pulling: env names the new version", phase: updater.PhasePulling, env: newEnv(), backup: true, running: "1.15.0", health: "healthy",
			want: updater.PhaseFailed, wantEnv: oldEnv, message: "put back from .env.bak-vor-1.16.0"},
		{name: "backing up: env not written yet", phase: updater.PhaseBackingUp, env: oldEnv, backup: true, running: "1.15.0", health: "healthy",
			want: updater.PhaseFailed, wantEnv: oldEnv, message: "nothing to put back"},
		{name: "verifying: no backup", phase: updater.PhaseVerifying, env: newEnv(), running: "1.15.0", health: "healthy",
			want: updater.PhaseFailed, wantEnv: newEnv(), message: "no backup"},
		{name: "pulling: backup is a symbolic link", phase: updater.PhasePulling, env: newEnv(), linkBackup: true, running: "1.15.0", health: "healthy",
			want: updater.PhaseFailed, wantEnv: newEnv(), message: "no backup"},
		{name: "waiting: new version healthy", phase: updater.PhaseWaiting, env: newEnv(), backup: true, running: "1.16.0", health: "healthy",
			rec: migrated, want: updater.PhaseDone, migrated: true, wantEnv: newEnv()},
		{name: "restarting: new version healthy without its record", phase: updater.PhaseRestarting, env: newEnv(), backup: true, running: "1.16.0", health: "healthy",
			want: updater.PhaseFailed, wantEnv: newEnv(), message: "nothing was taken back"},
		{name: "waiting: migration running", phase: updater.PhaseWaiting, env: newEnv(), backup: true, running: "1.16.0", health: "unhealthy",
			rec: migrating, want: updater.PhaseFailed, code: updater.CodeMigratedNoRollback, migrated: true, wantEnv: newEnv(), message: "pre-migration-00037.db"},
		{name: "restarting: old version runs", phase: updater.PhaseRestarting, env: newEnv(), backup: true, running: "1.15.0", health: "healthy",
			want: updater.PhaseFailed, wantEnv: newEnv(), message: "nothing was taken back"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			dir := filepath.Join(h.deploy, "updater")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.env, []byte(tc.env), 0o640); err != nil {
				t.Fatal(err)
			}
			if tc.backup {
				if err := os.WriteFile(h.env+".bak-vor-1.16.0", []byte(oldEnv), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.linkBackup {
				victim := filepath.Join(t.TempDir(), "secret")
				if err := os.WriteFile(victim, []byte(oldEnv), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(victim, h.env+".bak-vor-1.16.0"); err != nil {
					t.Fatal(err)
				}
			}
			h.fake.c.Labels[labelVersion], h.fake.c.Health = tc.running, tc.health
			started := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
			if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile),
				updater.State{Phase: tc.phase, From: "1.15.0", To: "1.16.0", StartedAt: started}, 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.rec != nil {
				h.writeServer(*tc.rec)
			}
			h.start()
			h.u.wg.Wait() // a resumed wait runs in the background
			s := h.u.State()
			if s.Phase != tc.want || s.Code != tc.code || s.Migrated != tc.migrated || s.FinishedAt.IsZero() ||
				!strings.Contains(s.Message, tc.message) || s.To != "1.16.0" {
				t.Errorf("state = %+v", s)
			}
			if disk := diskState(t, h); disk.Phase != tc.want {
				t.Errorf("state.json = %+v", disk)
			}
			if got := h.readEnv(); got != tc.wantEnv {
				t.Errorf(".env = %q, want %q", got, tc.wantEnv)
			}
			if info, _ := os.Stat(h.env); info == nil || info.Mode().Perm() != 0o640 {
				t.Errorf(".env mode = %v, want 0640 as before", info)
			}
			if v := h.fake.verbs(); len(v) != 0 {
				t.Errorf("compose ran at the start: %v", v)
			}
			// A finished state stays as it is.
			h.start()
			h.u.wg.Wait()
			if again := h.u.State(); !again.FinishedAt.Equal(s.FinishedAt) {
				t.Errorf("a second start changed the finished state: %+v", again)
			}
		})
	}
}

// An install interrupted after its restart waits again in the background: the phase stays
// waiting, /status answers, /install is busy, and the new version that becomes healthy later
// ends it done.
func TestStartResumesWaitingInBackground(t *testing.T) {
	h := newHarness(t)
	dir := filepath.Join(h.deploy, "updater")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.env, []byte(newEnv()), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile),
		updater.State{Phase: updater.PhaseRestarting, From: "1.15.0", To: "1.16.0", StartedAt: time.Now()}, 0o644); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.16.0", "starting" // still migrating
	h.fake.inspectFn = func(f *fakeDocker) error {
		select {
		case <-gate:
			h.writeServer(updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, SchemaAfter: 38,
				Backup: "pre-migration-00037.db", MigratedAt: time.Now()})
			f.c.Health = "healthy"
			f.inspectFn = nil
		default:
		}
		return nil
	}
	h.wait = time.Minute
	h.start()
	if s := h.u.State(); s.Phase != updater.PhaseWaiting || !s.FinishedAt.IsZero() {
		t.Fatalf("state after the start = %+v, want waiting", s)
	}
	if s := diskState(t, h); s.Phase != updater.PhaseWaiting {
		t.Errorf("state.json = %+v, want waiting", s)
	}
	if status, s := h.request(http.MethodGet, updater.PathStatus, "Bearer "+string(h.token)); status != http.StatusOK || s.Phase != updater.PhaseWaiting {
		t.Errorf("status = %d %+v", status, s)
	}
	if status, s := h.install(); status != http.StatusConflict || s.Code != updater.CodeBusy {
		t.Errorf("install during the resumed wait = %d %+v, want 409 busy", status, s)
	}
	close(gate)
	s := h.finish()
	if s.Phase != updater.PhaseDone || !s.Migrated || s.Backup != "pre-migration-00037.db" || s.Code != "" {
		t.Errorf("final state = %+v", s)
	}
	if v := h.fake.verbs(); len(v) != 0 {
		t.Errorf("compose ran: %v", v)
	}
}

// The resumed wait stops early for a crashed container and leaves its phase on a shutdown.
func TestResumedWaitGaveUpAndShutdown(t *testing.T) {
	prepare := func(t *testing.T) *harness {
		h := newHarness(t)
		dir := filepath.Join(h.deploy, "updater")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile),
			updater.State{Phase: updater.PhaseWaiting, From: "1.15.0", To: "1.16.0", StartedAt: time.Now()}, 0o644); err != nil {
			t.Fatal(err)
		}
		return h
	}
	t.Run("crashed", func(t *testing.T) {
		h := prepare(t)
		h.fake.c.Labels[labelVersion] = "1.16.0"
		h.fake.c.Running, h.fake.c.Status, h.fake.c.RestartCount, h.fake.c.RestartPolicy = false, "restarting", 3, "unless-stopped"
		h.start()
		h.u.wg.Wait()
		if s := h.u.State(); s.Phase != updater.PhaseFailed || s.Code != "" || !strings.Contains(s.Message, "stopped (exited") {
			t.Errorf("state = %+v", s)
		}
	})
	t.Run("unhealthy", func(t *testing.T) {
		h := prepare(t)
		h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.16.0", "starting"
		h.start()
		h.u.wg.Wait()
		if s := h.u.State(); s.Phase != updater.PhaseFailed || s.Code != "" ||
			!strings.Contains(s.Message, "version 1.16.0 did not become healthy within") {
			t.Errorf("state = %+v", s)
		}
	})
	t.Run("silent", func(t *testing.T) {
		h := prepare(t)
		h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.16.0", "healthy"
		h.start()
		h.u.wg.Wait()
		if s := h.u.State(); s.Phase != updater.PhaseFailed || s.Code != "" ||
			!strings.Contains(s.Message, "version 1.16.0 runs healthy but did not report back (server.json) within") ||
			strings.Contains(s.Message, "did not become healthy") {
			t.Errorf("state = %+v", s)
		}
	})
	t.Run("shutdown", func(t *testing.T) {
		h := prepare(t)
		h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.16.0", "starting"
		h.wait = time.Minute
		h.start()
		h.stop()
		if !h.u.drain(5 * time.Second) {
			t.Fatal("the resumed wait did not stop")
		}
		if s := diskState(t, h); s.Phase != updater.PhaseWaiting || !s.FinishedAt.IsZero() {
			t.Errorf("state.json = %+v, want waiting kept", s)
		}
	})
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
		{"SLOUCHCAT_DEPLOY_DIR": dir, "SLOUCHCAT_UPDATE_MANIFEST_URL": "http://x.test/m.json"},
		{"SLOUCHCAT_DEPLOY_DIR": dir, "SLOUCHCAT_UPDATE_MANIFEST_URL": "https:///m.json"},
	} {
		if _, err := loadConfig(func(k string) string { return bad[k] }); err == nil {
			t.Errorf("loadConfig(%v) accepted", bad)
		}
	}
	env = map[string]string{"SLOUCHCAT_DEPLOY_DIR": dir, "PUID": "568", "PGID": "569", "SLOUCHCAT_CONTAINER": "media", "SLOUCHCAT_UPDATE_MANIFEST_URL": "https://x.test/m.json"}
	c, err = loadConfig(func(k string) string { return env[k] })
	if err != nil || c.uid != 568 || c.gid != 569 || c.container != "media" || c.manifestURL != "https://x.test/m.json" {
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

func TestHTTPSRedirects(t *testing.T) {
	req := func(u string) *http.Request {
		r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if err := httpsRedirects(req("https://example.test/b"), []*http.Request{req("https://example.test/a")}); err != nil {
		t.Errorf("https redirect refused: %v", err)
	}
	if err := httpsRedirects(req("http://example.test/b"), []*http.Request{req("https://example.test/a")}); err == nil {
		t.Error("redirect to http followed")
	}
	via := make([]*http.Request, maxRedirects)
	if err := httpsRedirects(req("https://example.test/b"), via); err == nil {
		t.Errorf("redirect %d followed", maxRedirects+1)
	}
}

// A shutdown during the rollback (the env file put back, the old version starting) leaves the
// running phase with RollingBack in state.json; the next start finishes the rollback instead of
// waiting for the new version: rolled_back once the old version runs healthy.
func TestShutdownDuringRollbackFinishesAtTheNextStart(t *testing.T) {
	h := newHarness(t)
	h.fake.afterUp = func(f *fakeDocker, n int) {
		switch n {
		case 1: // the new version crashes without a migration
			f.c.Labels[labelVersion] = "1.16.0"
			f.c.Running, f.c.Status, f.c.RestartCount, f.c.RestartPolicy = false, "restarting", 3, "unless-stopped"
		case 2: // the old version is still starting when the updater stops
			f.c.Labels[labelVersion] = "1.15.0"
			f.c.Running, f.c.Status, f.c.RestartCount, f.c.Health = true, "running", 0, "starting"
		default:
			f.c.Health = "healthy"
		}
	}
	h.wait = time.Minute
	h.start()
	if status, _ := h.install(); status != http.StatusAccepted {
		t.Fatalf("install = %d", status)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.fake.mu.Lock()
		ups := h.fake.ups
		h.fake.mu.Unlock()
		if ups == 2 && h.u.State().RollingBack {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("rollback did not start: ups %d, state %+v", ups, h.u.State())
		}
		time.Sleep(time.Millisecond)
	}
	h.stop()
	if !h.u.drain(5 * time.Second) {
		t.Fatal("the install did not stop")
	}
	if s := diskState(t, h); !s.Phase.Running() || !s.RollingBack || !s.FinishedAt.IsZero() {
		t.Fatalf("state.json = %+v, want a running phase with RollingBack", s)
	}
	if got := h.readEnv(); got != oldEnv {
		t.Fatalf(".env = %q, want the original", got)
	}

	h.start() // the updater comes back
	s := h.finish()
	if s.Phase != updater.PhaseRolledBack || s.Code != updater.CodeUnhealthy || s.RollingBack ||
		!strings.Contains(s.Message, "version 1.15.0 runs again") {
		t.Errorf("state after the next start = %+v", s)
	}
	if got := h.fake.verbs(); !slices.Equal(got, []string{"pull", "config", "up", "up", "up"}) {
		t.Errorf("compose calls = %v", got)
	}
}

// A start that finds a rollback under way finishes it: the env file is put back when it still
// names the new version, compose up starts the old one; rolled_back when the old version runs
// healthy, failed when it does not come back or the env file cannot be put back; a shutdown
// leaves the phase for the next start.
func TestStartFinishesAnInterruptedRollback(t *testing.T) {
	prepare := func(t *testing.T, env string, backup bool) *harness {
		h := newHarness(t)
		dir := filepath.Join(h.deploy, "updater")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile), updater.State{Phase: updater.PhaseWaiting,
			From: "1.15.0", To: "1.16.0", StartedAt: time.Now(), RollingBack: true}, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(h.env, []byte(env), 0o640); err != nil {
			t.Fatal(err)
		}
		if backup {
			if err := os.WriteFile(h.env+".bak-vor-1.16.0", []byte(oldEnv), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.16.0", "unhealthy" // the failed new version
		return h
	}
	oldComesBack := func(f *fakeDocker, _ int) { f.c.Labels[labelVersion], f.c.Health = "1.15.0", "healthy" }

	t.Run("env file still new", func(t *testing.T) {
		h := prepare(t, newEnv(), true)
		h.fake.afterUp = oldComesBack
		h.start()
		s := h.finish()
		if s.Phase != updater.PhaseRolledBack || s.Code != updater.CodeUnhealthy || s.RollingBack || s.FinishedAt.IsZero() {
			t.Errorf("state = %+v", s)
		}
		if got := h.readEnv(); got != oldEnv {
			t.Errorf(".env = %q, want the original", got)
		}
		if got := h.fake.verbs(); !slices.Equal(got, []string{"up"}) {
			t.Errorf("compose calls = %v", got)
		}
	})
	t.Run("old version already back", func(t *testing.T) {
		h := prepare(t, oldEnv, true)
		h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.15.0", "healthy"
		h.start()
		if s := h.finish(); s.Phase != updater.PhaseRolledBack {
			t.Errorf("state = %+v", s)
		}
	})
	t.Run("healthy new version is not the old one", func(t *testing.T) {
		h := prepare(t, oldEnv, true)
		h.fake.c.Health = "healthy" // still 1.16.0: up did not bring the old one back
		h.start()
		if s := h.finish(); s.Phase != updater.PhaseFailed || s.Code != updater.CodeRollbackFailed ||
			!strings.Contains(s.Message, "did not come back") {
			t.Errorf("state = %+v", s)
		}
	})
	t.Run("old version does not come back", func(t *testing.T) {
		h := prepare(t, oldEnv, true)
		h.fake.afterUp = func(f *fakeDocker, _ int) {
			f.c.Labels[labelVersion] = "1.15.0"
			f.c.Running, f.c.Status, f.c.RestartCount, f.c.RestartPolicy = false, "restarting", 3, "unless-stopped"
		}
		h.wait = time.Minute
		h.start()
		if s := h.finish(); s.Phase != updater.PhaseFailed || s.Code != updater.CodeRollbackFailed {
			t.Errorf("state = %+v", s)
		}
	})
	t.Run("no backup", func(t *testing.T) {
		h := prepare(t, newEnv(), false)
		h.start()
		s := h.finish()
		if s.Phase != updater.PhaseFailed || s.Code != updater.CodeRollbackFailed || !strings.Contains(s.Message, "no backup") {
			t.Errorf("state = %+v", s)
		}
		if got := h.fake.verbs(); len(got) != 0 {
			t.Errorf("compose ran: %v", got)
		}
	})
	t.Run("shutdown", func(t *testing.T) {
		h := prepare(t, oldEnv, true)
		h.fake.afterUp = func(f *fakeDocker, _ int) { f.c.Labels[labelVersion], f.c.Health = "1.15.0", "starting" }
		h.wait = time.Minute
		h.start()
		h.stop()
		if !h.u.drain(5 * time.Second) {
			t.Fatal("the resumed rollback did not stop")
		}
		if s := diskState(t, h); s.Phase != updater.PhaseWaiting || !s.RollingBack || !s.FinishedAt.IsZero() {
			t.Errorf("state.json = %+v, want the rollback kept for the next start", s)
		}
	})
}
