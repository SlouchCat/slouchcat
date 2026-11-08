package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/updater"
)

const updImage = "ghcr.io/slouchcat/slouchcat-updater@" + otherDig

// selfHarness is a harness where the server runs release 1.18.0 of the manifest, which names an
// updater image, and the updater (1.17.0) runs in the same compose project as the server.
func selfHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.version = "1.17.0"
	h.manifest = `{"version":"1.18.0","image":"` + testImage + `","updaterImage":"` + updImage + `"}`
	if err := os.WriteFile(filepath.Join(h.deploy, "compose.updater.yaml"), []byte(oldCompose), 0o600); err != nil {
		t.Fatal(err)
	}
	h.fake.c.Labels[labelVersion] = "1.18.0"
	files := filepath.Join(h.deploy, "compose.yaml") + "," + filepath.Join(h.deploy, "compose.updater.yaml")
	h.fake.self = &Container{Image: "ghcr.io/slouchcat/slouchcat-updater:1.17.0", ImageID: "sha256:upd-old",
		Running: true, Status: "running", Labels: map[string]string{
			labelVersion: "1.17.0", labelProject: "slouchcat", labelWorkingDir: h.deploy, labelConfigFiles: files,
		}}
	h.fake.selfDigs = []string{updImage}
	return h
}

func (h *harness) selfEnv() string {
	return oldEnv + "\r\nSLOUCHCAT_UPDATER_VERSION=1.18.0\r\n"
}

func TestSelfUpdate(t *testing.T) {
	h := selfHarness(t)
	h.start()
	h.u.selfUpdate("test", nil)
	if got := h.readEnv(); got != h.selfEnv() {
		t.Errorf(".env = %q", got)
	}
	if got := h.readFile(".env.bak-vor-updater-1.18.0"); got != oldEnv {
		t.Errorf("backup = %q", got)
	}
	if got := h.fake.verbs(); !slices.Equal(got, []string{"pull", "config"}) {
		t.Errorf("compose calls = %v", got)
	}
	if c := h.fake.calls[0]; c[len(c)-1] != updaterService {
		t.Errorf("pull = %v", c)
	}
	compose, upd := filepath.Join(h.deploy, "compose.yaml"), filepath.Join(h.deploy, "compose.updater.yaml")
	p, r := h.u.projectOf(*h.fake.self, defaultUpdaterContainer)
	if r != nil {
		t.Fatal(r)
	}
	want := helperArgs(h.u.cfg, p, updImage, h.env, ".env.bak-vor-updater-1.18.0", "1.17.0")
	if len(h.fake.runs) != 1 || !slices.Equal(h.fake.runs[0], want) {
		t.Fatalf("docker run = %q\nwant %q", h.fake.runs, want)
	}
	if slices.Contains(want, "--rm") || !slices.Contains(want, updImage) {
		t.Errorf("docker run = %q", want)
	}
	// The helper takes these arguments back.
	hp, ignored, err := parseHelper(want[slices.Index(want, "self-update")+1:])
	if err != nil || len(ignored) != 0 || hp.project.Name != "slouchcat" || hp.project.WorkingDir != h.deploy ||
		!slices.Equal(hp.project.ConfigFiles, []string{compose, upd}) || !slices.Equal(hp.project.EnvFiles, []string{h.env}) ||
		hp.env != h.env || hp.backup != ".env.bak-vor-updater-1.18.0" || hp.oldVersion != "1.17.0" || hp.container != "slouchcat-updater" {
		t.Errorf("parseHelper = %+v, %v, ignored %v", hp, err, ignored)
	}
	// The helper recreates this container: /install stays busy meanwhile; state.json is untouched.
	if status, s := h.install(); status != http.StatusConflict || s.Code != updater.CodeBusy {
		t.Errorf("install during the handover = %d %+v", status, s)
	}
	if s := h.u.State(); s.Phase != updater.PhaseIdle {
		t.Errorf("state = %+v", s)
	}
}

// A self-update runs only when the release names a valid updater image, the server runs it, the
// updater is older (a dev build never), no install runs and the version was not tried within 6 h.
func TestSelfUpdateConditions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(h *harness)
	}{
		{"dev build", func(h *harness) { h.u.version = "dev" }},
		{"same version", func(h *harness) { h.u.version = "1.18.0" }},
		{"newer updater", func(h *harness) { h.u.version = "1.19.0" }},
		{"server older", func(h *harness) { h.fake.c.Labels[labelVersion] = "1.17.0" }},
		{"no updater image", func(h *harness) { h.manifest = `{"version":"1.18.0","image":"` + testImage + `"}` }},
		{"server image as updater", func(h *harness) {
			h.manifest = `{"version":"1.18.0","image":"` + testImage + `","updaterImage":"` + testImage + `"}`
		}},
		{"manifest unreachable", func(h *harness) { h.srv.Close() }},
		{"install running", func(h *harness) { h.u.state.Phase = updater.PhasePulling }},
		{"tried recently", func(h *harness) {
			if err := updater.WriteJSON(filepath.Join(h.deploy, "updater", selfUpdateFile),
				selfRecord{Version: "1.18.0", At: time.Now().Add(-time.Hour)}, 0o644); err != nil {
				h.t.Fatal(err)
			}
		}},
		{"own container missing", func(h *harness) { h.fake.self = nil }},
		{"helper runs", func(h *harness) { h.fake.helper = &Container{Running: true, Status: "running"} }},
		{"own container outside", func(h *harness) { h.fake.self.Labels[labelConfigFiles] = "/etc/compose.yaml" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := selfHarness(t)
			h.start()
			tc.setup(h)
			h.u.selfUpdate("test", nil)
			if len(h.fake.runs) != 0 || h.readEnv() != oldEnv {
				t.Errorf("self-update ran: runs %v, .env %q", h.fake.runs, h.readEnv())
			}
			if v := h.fake.verbs(); len(v) != 0 {
				t.Errorf("compose ran: %v", v)
			}
		})
	}
}

// A failed step puts the env file back and changes nothing else; the version is tried again
// only after 6 h, also after a restart.
func TestSelfUpdateFailure(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(h *harness)
	}{
		{"pull", func(h *harness) { h.fake.pullErr = errors.New("exit status 1") }},
		{"digest", func(h *harness) { h.fake.selfDigs = []string{"ghcr.io/slouchcat/slouchcat-updater@" + testDigest} }},
		{"helper", func(h *harness) { h.fake.runErr = errors.New("name in use") }},
		{"helper exits early", func(h *harness) {
			h.fake.logs = []string{"updater self-update: arguments err=flag x needs a value"}
			h.fake.afterRun = func(f *fakeDocker) { f.helper = &Container{Status: "exited", ExitCode: 2} }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := selfHarness(t)
			h.start()
			tc.setup(h)
			h.u.selfUpdate("test", nil)
			if h.readEnv() != oldEnv {
				t.Errorf(".env = %q, want the original", h.readEnv())
			}
			if exists(t, h.env+".bak-vor-updater-1.18.0") {
				t.Error("the backup of the failed self-update is still there")
			}
			if tc.name == "helper exits early" {
				if !slices.Equal(h.fake.rmCont, []string{helperName}) || !strings.Contains(h.logs.String(), "flag x needs a value") {
					t.Errorf("removed containers %v, log %s", h.fake.rmCont, h.logs)
				}
			}
			if s := h.u.State(); s.Phase != updater.PhaseIdle {
				t.Errorf("state = %+v", s)
			}
			if status, _ := h.install(); status == http.StatusConflict {
				t.Error("install busy after a failed self-update")
			}
			h.u.wg.Wait()
			calls := len(h.fake.calls)
			h.start() // a restart within 6 h does not try again
			h.u.state = updater.State{Phase: updater.PhaseIdle}
			h.u.selfUpdate("test", nil)
			if len(h.fake.calls) != calls {
				t.Errorf("tried again within 6 h: %v", h.fake.verbs())
			}
			h.u.now = func() time.Time { return time.Now().Add(7 * time.Hour) }
			h.fake.pullErr, h.fake.selfDigs, h.fake.runErr, h.fake.afterRun = nil, []string{updImage}, nil, nil
			h.fake.c.Labels[labelVersion] = "1.18.0"
			h.u.selfUpdate("test", nil)
			if len(h.fake.runs) == 0 || h.fake.runs[len(h.fake.runs)-1][len(h.fake.runs[len(h.fake.runs)-1])-1] != "slouchcat-updater" {
				t.Errorf("not tried again after 6 h: %v", h.fake.runs)
			}
		})
	}
}

// After an install ends done, the updater follows the server at once.
func TestSelfUpdateAfterInstall(t *testing.T) {
	h := selfHarness(t)
	h.version = "1.15.0"
	h.fake.c.Labels[labelVersion] = "1.15.0"
	h.manifest = `{"version":"1.16.0","image":"` + testImage + `","updaterImage":"` + updImage + `"}`
	newVersionComesUp(h)
	h.start()
	h.install()
	if s := h.finish(); s.Phase != updater.PhaseDone {
		t.Fatalf("state = %+v", s)
	}
	if len(h.fake.runs) != 1 {
		t.Errorf("no self-update after the install: %v", h.fake.runs)
	}
	if !strings.Contains(h.readEnv(), "SLOUCHCAT_UPDATER_VERSION=1.16.0") || !strings.Contains(h.readEnv(), "SLOUCHCAT_VERSION=1.16.0") {
		t.Errorf(".env = %q", h.readEnv())
	}
}

// Watch checks once at the start and then on its ticker, until the process ends.
func TestWatch(t *testing.T) {
	h := selfHarness(t)
	h.start()
	h.u.selfEvery = time.Millisecond
	h.u.wg.Go(h.u.Watch)
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.fake.mu.Lock()
		n := len(h.fake.runs)
		h.fake.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no self-update at the start")
		}
		time.Sleep(time.Millisecond)
	}
	h.stop()
	if !h.u.drain(5 * time.Second) {
		t.Fatal("Watch did not end with the process")
	}
	if len(h.fake.runs) != 1 {
		t.Errorf("runs = %d, want 1 (busy after the handover)", len(h.fake.runs))
	}
}

// newHelper is the helper of a self-update to 1.18.0 against the fake: the env file names the new
// version, its backup the old one.
var newHelperEnv = oldEnv + "\r\nSLOUCHCAT_UPDATER_VERSION=1.18.0\r\n"

func newHelper(t *testing.T) (*harness, helper) {
	t.Helper()
	h := selfHarness(t)
	if err := os.WriteFile(h.env, []byte(h.selfEnv()), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.env+".bak-vor-updater-1.18.0", []byte(oldEnv), 0o600); err != nil {
		t.Fatal(err)
	}
	h.start()
	p, r := h.u.projectOf(*h.fake.self, defaultUpdaterContainer)
	if r != nil {
		t.Fatal(r)
	}
	return h, helper{project: p, env: h.env, backup: ".env.bak-vor-updater-1.18.0", container: defaultUpdaterContainer,
		docker: h.fake, log: h.u.log, version: "1.18.0", every: time.Millisecond, wait: 50 * time.Millisecond, settle: 5 * time.Millisecond}
}

func TestHelperRecreatesUpdater(t *testing.T) {
	h, hp := newHelper(t)
	h.fake.afterUp = func(f *fakeDocker, _ int) {
		f.self.Labels[labelVersion], f.self.Image, f.self.ImageID = "1.18.0", "ghcr.io/slouchcat/slouchcat-updater:1.18.0", "sha256:upd-new"
	}
	const repo = "ghcr.io/slouchcat/slouchcat-updater"
	h.fake.images = []Image{{"sha256:upd-new", repo, "1.18.0", "sha256:a"}, {"sha256:upd-old", repo, "1.17.0", "sha256:b"},
		{"sha256:srv", "ghcr.io/slouchcat/slouchcat", "1.15.0", "sha256:c"}}
	if code := hp.run(t.Context()); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	want := append(hp.project.args(), "up", "-d", "--no-deps", "--force-recreate", "--no-build", "--pull", "never", "updater")
	if len(h.fake.calls) != 1 || !slices.Equal(h.fake.calls[0], want) {
		t.Errorf("compose calls = %q\nwant %q", h.fake.calls, want)
	}
	if h.readEnv() != h.selfEnv() {
		t.Errorf(".env = %q", h.readEnv())
	}
	if !slices.Equal(h.fake.removed, []string{repo + ":1.17.0"}) {
		t.Errorf("removed %v", h.fake.removed)
	}
	if !slices.Equal(h.fake.rmCont, []string{helperName + " --force"}) {
		t.Errorf("removed containers %v, want the helper itself", h.fake.rmCont)
	}
}

// When the new updater does not come up the helper sets the old version in the env file (the
// backup only for a dev build) and starts the old one; a backup name that does not fit stops it
// before that. A failed helper keeps its container for the log.
func TestHelperPutsOldUpdaterBack(t *testing.T) {
	crash := func(f *fakeDocker, n int) {
		if n == 1 { // the new one crash-loops
			f.self.Labels[labelVersion], f.self.Running, f.self.Status, f.self.RestartCount = "1.18.0", false, "restarting", 2
		}
	}
	for _, tc := range []struct {
		name, env, want string
	}{
		{"line set", "", strings.Replace(newHelperEnv, "1.18.0", "1.17.0", 1)},
		{"line missing", oldEnv, oldEnv + "\r\nSLOUCHCAT_UPDATER_VERSION=1.17.0\r\n"},
	} {
		t.Run("old version "+tc.name, func(t *testing.T) {
			h, hp := newHelper(t)
			if tc.env != "" {
				if err := os.WriteFile(h.env, []byte(tc.env), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			hp.oldVersion = "1.17.0"
			h.fake.afterUp = crash
			if code := hp.run(t.Context()); code == 0 {
				t.Fatal("exit code 0")
			}
			if got := h.readEnv(); got != tc.want {
				t.Errorf(".env = %q, want %q", got, tc.want)
			}
			if got := h.fake.verbs(); !slices.Equal(got, []string{"up", "up"}) || len(h.fake.rmCont) != 0 {
				t.Errorf("compose calls = %v, removed %v", got, h.fake.rmCont)
			}
		})
	}
	t.Run("dev build", func(t *testing.T) {
		h, hp := newHelper(t)
		hp.oldVersion = "dev"
		h.fake.afterUp = func(f *fakeDocker, n int) {
			if n == 1 { // the new one crash-loops
				f.self.Labels[labelVersion], f.self.Running, f.self.Status, f.self.RestartCount = "1.18.0", false, "restarting", 2
			}
		}
		if code := hp.run(t.Context()); code == 0 {
			t.Fatal("exit code 0")
		}
		if h.readEnv() != oldEnv {
			t.Errorf(".env = %q, want the original", h.readEnv())
		}
		if got := h.fake.verbs(); !slices.Equal(got, []string{"up", "up"}) {
			t.Errorf("compose calls = %v", got)
		}
		for _, c := range h.fake.calls {
			if c[len(c)-1] != updaterService {
				t.Errorf("the helper touched the server: %v", c)
			}
		}
	})
	t.Run("bad backup name", func(t *testing.T) {
		h, hp := newHelper(t)
		hp.backup = ".env.bak-vor-1.15.0"
		if code := hp.run(t.Context()); code == 0 {
			t.Fatal("exit code 0")
		}
		if h.readEnv() != h.selfEnv() {
			t.Errorf(".env = %q", h.readEnv())
		}
		if got := h.fake.verbs(); !slices.Equal(got, []string{"up"}) {
			t.Errorf("compose calls = %v", got)
		}
	})
}

func TestParseHelperRefuses(t *testing.T) {
	base := []string{"--project", "slouchcat", "--workdir", "/srv/deploy", "-f", "/srv/deploy/compose.yaml", "--env", "/srv/deploy/.env"}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"server service", append(slices.Clone(base), "--service", "slouchcat")},
		{"no service", base},
		{"relative file", append([]string{"--service", "updater", "-f", "compose.yaml"}, base...)},
		{"backup path", append(slices.Clone(base), "--service", "updater", "--backup", "../x")},
		{"extra argument", append(slices.Clone(base), "--service", "updater", "up")},
	} {
		if _, _, err := parseHelper(tc.args); err == nil {
			t.Errorf("%s: parseHelper accepted %q", tc.name, tc.args)
		}
	}
}

func TestLoadConfigUpdaterContainer(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{"SLOUCHCAT_DEPLOY_DIR": dir}
	if c, err := loadConfig(func(k string) string { return env[k] }); err != nil || c.updaterContainer != "slouchcat-updater" {
		t.Errorf("default = %q, %v", c.updaterContainer, err)
	}
	env["SLOUCHCAT_UPDATER_CONTAINER"] = "upd"
	if c, err := loadConfig(func(k string) string { return env[k] }); err != nil || c.updaterContainer != "upd" {
		t.Errorf("set = %q, %v", c.updaterContainer, err)
	}
}

// helperArgs is a contract between an old updater and a new image: the golden file freezes it.
// A change must stay readable for every helper since 1.18.0 (new flags only, each with a value).
func TestHelperArgsContract(t *testing.T) {
	cfg := settings{deployDir: "/srv/slouchcat/deploy", uid: 568, gid: 569, tz: "Europe/Zurich", updaterContainer: "slouchcat-updater"}
	p := Project{Name: "slouchcat", WorkingDir: "/srv/slouchcat/deploy",
		ConfigFiles: []string{"/srv/slouchcat/deploy/compose.yaml", "/srv/slouchcat/deploy/compose.updater.yaml"},
		EnvFiles:    []string{"/srv/slouchcat/deploy/.env"}}
	got := helperArgs(cfg, p, updImage, "/srv/slouchcat/deploy/.env", ".env.bak-vor-updater-1.18.0", "1.17.0")
	data, err := os.ReadFile(filepath.Join("testdata", "helper-args.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !slices.Equal(got, want) {
		t.Errorf("helperArgs =\n%s\nwant (testdata/helper-args.golden)\n%s", strings.Join(got, "\n"), data)
	}
	h, ignored, err := parseHelper(got[slices.Index(got, helperCommand)+1:])
	if err != nil || len(ignored) != 0 || h.oldVersion != "1.17.0" || len(h.project.ConfigFiles) != 2 {
		t.Errorf("parseHelper = %+v, %v, %v", h, ignored, err)
	}
}

// A later updater may pass flags this helper does not know: they are ignored, with their values.
func TestParseHelperIgnoresUnknownFlags(t *testing.T) {
	args := []string{"--future", "x", "--project", "slouchcat", "--later=y", "--workdir", "/srv/deploy", "--flag-at-end-of-group",
		"-f", "/srv/deploy/compose.yaml", "--env", "/srv/deploy/.env", "--service=updater", "--old-version", "1.17.0", "--last"}
	h, ignored, err := parseHelper(args)
	if err != nil || !slices.Equal(ignored, []string{"future", "later", "flag-at-end-of-group", "last"}) ||
		h.project.Name != "slouchcat" || h.project.WorkingDir != "/srv/deploy" || h.oldVersion != "1.17.0" || h.env != "/srv/deploy/.env" {
		t.Errorf("parseHelper = %+v, ignored %v, %v", h, ignored, err)
	}
}

// /install is busy while a helper container runs; a self-update removes an ended one first, with
// the log of a failed one.
func TestHelperContainerBusyAndCleared(t *testing.T) {
	h := selfHarness(t)
	h.fake.helper = &Container{Running: true, Status: "running"}
	h.start()
	if status, s := h.install(); status != http.StatusConflict || s.Code != updater.CodeBusy {
		t.Errorf("install while the helper runs = %d %+v", status, s)
	}
	h.fake.helper = &Container{Status: "exited", ExitCode: 1}
	h.fake.logs = []string{"updater self-update: failed, putting the old updater back"}
	h.u.selfUpdate("test", nil)
	if !slices.Equal(h.fake.rmCont, []string{helperName}) || !strings.Contains(h.logs.String(), "putting the old updater back") {
		t.Errorf("removed containers %v", h.fake.rmCont)
	}
	if len(h.fake.runs) != 1 {
		t.Errorf("no self-update after the cleanup: %v", h.fake.runs)
	}
}
