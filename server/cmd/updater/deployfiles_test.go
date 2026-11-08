package main

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/updater"
)

const (
	oldCompose = "services: {}\n"
	newCompose = "services:\n  slouchcat:\n    image: ghcr.io/slouchcat/slouchcat:${SLOUCHCAT_VERSION:-1.16.0}\n"
)

// withDeploy makes the release name compose.yaml with newCompose (served next to the manifest),
// compose.intel.yaml with its current content and compose.nvidia.yaml, which the project does not
// use; oldCompose is a known earlier content of compose.yaml.
func withDeploy(h *harness) {
	withDeployKnown(h, `{"compose.yaml":["`+hexSum([]byte("an older release"))+`","`+hexSum([]byte(oldCompose))+`"]}`)
}

// withDeployKnown is withDeploy with the given deployKnown (JSON).
func withDeployKnown(h *harness, known string) {
	h.files = map[string]string{"/compose.yaml": newCompose}
	h.manifest = `{"version":"1.16.0","image":"` + testImage + `","deploy":{` +
		`"compose.yaml":"` + hexSum([]byte(newCompose)) + `",` +
		`"compose.intel.yaml":"` + hexSum([]byte(oldCompose)) + `",` +
		`"compose.nvidia.yaml":"` + hexSum([]byte("x")) + `"},"deployKnown":` + known + `}`
}

// newVersionComesUp lets the new version start healthy without a migration.
func newVersionComesUp(h *harness) {
	h.fake.afterUp = func(f *fakeDocker, _ int) {
		f.c.Labels[labelVersion] = "1.16.0"
		h.writeServer(updater.ServerRecord{Version: "1.16.0", SchemaBefore: 37, SchemaAfter: 37, MigratedAt: time.Now()})
	}
}

func (h *harness) readFile(name string) string {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.deploy, name))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(data)
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// A compose file the release names with another content is replaced before the pull, with a
// backup and its mode kept; one with the same content, one the project does not use and the env
// files stay.
func TestInstallReplacesComposeFiles(t *testing.T) {
	h := newHarness(t)
	compose := filepath.Join(h.deploy, "compose.yaml")
	if err := os.Chmod(compose, 0o640); err != nil {
		t.Fatal(err)
	}
	withDeploy(h)
	var atPull string
	h.fake.pullGate = make(chan struct{})
	go func() {
		for {
			if data, err := os.ReadFile(compose); err == nil && string(data) == newCompose {
				atPull = string(data)
				close(h.fake.pullGate)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	newVersionComesUp(h)
	h.start()
	if status, _ := h.install(); status != http.StatusAccepted {
		t.Fatalf("install = %d", status)
	}
	s := h.finish()
	if s.Phase != updater.PhaseDone || s.DeployBackup != ".bak-vor-1.16.0" {
		t.Errorf("state = %+v", s)
	}
	if atPull != newCompose || h.readFile("compose.yaml") != newCompose {
		t.Errorf("compose.yaml = %q", h.readFile("compose.yaml"))
	}
	if info, _ := os.Stat(compose); info == nil || info.Mode().Perm() != 0o640 {
		t.Errorf("compose.yaml mode = %v, want 0640 as before", info)
	}
	if got := h.readFile("compose.yaml.bak-vor-1.16.0"); got != oldCompose {
		t.Errorf("backup = %q", got)
	}
	if h.readFile("compose.intel.yaml") != oldCompose || exists(t, filepath.Join(h.deploy, "compose.intel.yaml.bak-vor-1.16.0")) {
		t.Error("compose.intel.yaml (same content) was touched")
	}
	if exists(t, filepath.Join(h.deploy, "compose.nvidia.yaml")) {
		t.Error("a compose file the project does not use was written")
	}
	if got := h.readEnv(); got != newEnv() {
		t.Errorf(".env = %q", got)
	}
}

// One suffix serves every compose file of the project: a hand-made backup of a file the install
// does not replace moves all of them to -2, and nothing overwrites it.
func TestInstallComposeBackupSuffixFreeForAll(t *testing.T) {
	h := newHarness(t)
	handmade := filepath.Join(h.deploy, "compose.intel.yaml.bak-vor-1.16.0")
	if err := os.WriteFile(handmade, []byte("by hand"), 0o600); err != nil {
		t.Fatal(err)
	}
	withDeploy(h)
	newVersionComesUp(h)
	h.start()
	h.install()
	s := h.finish()
	if s.Phase != updater.PhaseDone || s.DeployBackup != ".bak-vor-1.16.0-2" {
		t.Errorf("state = %+v", s)
	}
	if h.readFile("compose.yaml.bak-vor-1.16.0-2") != oldCompose || exists(t, filepath.Join(h.deploy, "compose.yaml.bak-vor-1.16.0")) {
		t.Error("backup of compose.yaml not under the common suffix -2")
	}
	if h.readFile("compose.intel.yaml.bak-vor-1.16.0") != "by hand" {
		t.Error("the hand-made backup changed")
	}
}

// A release file that does not match the manifest (other content, too large) ends the install
// before the pull with deploy_files; the env file comes back and no compose file changes.
func TestInstallComposeFileRefused(t *testing.T) {
	for _, tc := range []struct {
		name, body, msg string
	}{
		{"hash mismatch", "services: {evil: {}}\n", "sha256"},
		{"too large", strings.Repeat("a", updater.MaxReleaseFileBytes+10), "larger than"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			withDeploy(h)
			h.files["/compose.yaml"] = tc.body
			if tc.name == "too large" {
				h.manifest = strings.Replace(h.manifest, hexSum([]byte(newCompose)), hexSum([]byte(tc.body)), 1)
			}
			h.start()
			if status, _ := h.install(); status != http.StatusAccepted {
				t.Fatalf("install = %d", status)
			}
			s := h.finish()
			if s.Phase != updater.PhaseFailed || s.Code != updater.CodeDeployFiles || !strings.Contains(s.Message, tc.msg) {
				t.Errorf("state = %+v", s)
			}
			if h.readEnv() != oldEnv || h.readFile("compose.yaml") != oldCompose {
				t.Error("env file or compose.yaml changed")
			}
			if v := h.fake.verbs(); len(v) != 0 {
				t.Errorf("compose ran: %v", v)
			}
		})
	}
}

// A failed pull and a rollback without a migration put the compose files back, the rollback
// before it starts the old version.
func TestInstallFailurePutsComposeFilesBack(t *testing.T) {
	t.Run("pull failed", func(t *testing.T) {
		h := newHarness(t)
		withDeploy(h)
		h.fake.pullErr = errors.New("exit status 1")
		h.start()
		h.install()
		if s := h.finish(); s.Phase != updater.PhaseFailed || s.Code != updater.CodePullFailed {
			t.Errorf("state = %+v", s)
		}
		if h.readFile("compose.yaml") != oldCompose || h.readEnv() != oldEnv {
			t.Error("compose.yaml or .env not put back")
		}
	})
	t.Run("rollback", func(t *testing.T) {
		h := newHarness(t)
		withDeploy(h)
		var atSecondUp string
		h.fake.afterUp = func(f *fakeDocker, n int) {
			if n == 1 {
				f.c.Labels[labelVersion], f.c.Health = "1.16.0", "unhealthy"
				return
			}
			atSecondUp = h.readFile("compose.yaml")
			f.c.Labels[labelVersion], f.c.Health = "1.15.0", "healthy"
		}
		h.start()
		h.install()
		if s := h.finish(); s.Phase != updater.PhaseRolledBack {
			t.Errorf("state = %+v", s)
		}
		if atSecondUp != oldCompose || h.readFile("compose.yaml") != oldCompose {
			t.Errorf("compose.yaml at the rollback = %q", atSecondUp)
		}
	})
}

// After an interruption the compose files come back from the backups named in state.json: before
// the restart (recover) and in a rollback (rollbackAfterRestart). A forged suffix is refused.
func TestInterruptedInstallPutsComposeFilesBack(t *testing.T) {
	prepare := func(t *testing.T, st updater.State) *harness {
		h := newHarness(t)
		dir := filepath.Join(h.deploy, "updater")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		st.From, st.To, st.StartedAt = "1.15.0", "1.16.0", time.Now()
		if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile), st, 0o644); err != nil {
			t.Fatal(err)
		}
		for name, body := range map[string]string{".env": newEnv(), ".env.bak-vor-1.16.0": oldEnv,
			"compose.yaml": newCompose, "compose.yaml.bak-vor-1.16.0": oldCompose} {
			if err := os.WriteFile(filepath.Join(h.deploy, name), []byte(body), 0o640); err != nil {
				t.Fatal(err)
			}
		}
		return h
	}
	t.Run("pulling", func(t *testing.T) {
		h := prepare(t, updater.State{Phase: updater.PhasePulling, DeployBackup: ".bak-vor-1.16.0"})
		h.start()
		s := h.u.State()
		if s.Phase != updater.PhaseFailed || !strings.Contains(s.Message, "compose files were put back") {
			t.Errorf("state = %+v", s)
		}
		if h.readFile("compose.yaml") != oldCompose || h.readEnv() != oldEnv {
			t.Error("compose.yaml or .env not put back")
		}
	})
	t.Run("rolling back", func(t *testing.T) {
		h := prepare(t, updater.State{Phase: updater.PhaseWaiting, RollingBack: true, DeployBackup: ".bak-vor-1.16.0"})
		h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.16.0", "unhealthy"
		var atUp string
		h.fake.afterUp = func(f *fakeDocker, _ int) {
			atUp = h.readFile("compose.yaml")
			f.c.Labels[labelVersion], f.c.Health = "1.15.0", "healthy"
		}
		h.start()
		if s := h.finish(); s.Phase != updater.PhaseRolledBack {
			t.Errorf("state = %+v", s)
		}
		if atUp != oldCompose {
			t.Errorf("compose.yaml at up = %q", atUp)
		}
	})
	t.Run("forged suffix", func(t *testing.T) {
		h := prepare(t, updater.State{Phase: updater.PhaseWaiting, RollingBack: true, DeployBackup: "/../../etc/x"})
		h.fake.c.Labels[labelVersion], h.fake.c.Health = "1.16.0", "unhealthy"
		h.start()
		s := h.finish()
		if s.Phase != updater.PhaseFailed || s.Code != updater.CodeRollbackFailed || !strings.Contains(s.Message, "compose files") {
			t.Errorf("state = %+v", s)
		}
		if v := h.fake.verbs(); len(v) != 0 {
			t.Errorf("compose ran: %v", v)
		}
	})
}

// The fields of 1.18.0 are optional: an invalid one is dropped with a log line, the install goes on.
func TestManifestOptionalFields(t *testing.T) {
	const upd = "ghcr.io/slouchcat/slouchcat-updater@" + otherDig
	sum := hexSum([]byte("x"))
	for _, tc := range []struct {
		name, fields, image string
		deploy              int
	}{
		{"none", ``, "", 0},
		{"valid", `,"updaterImage":"` + upd + `","deploy":{"compose.yaml":"` + sum + `","compose.intel.yaml":"` + sum + `"}`, upd, 2},
		{"tag dropped", `,"updaterImage":"ghcr.io/slouchcat/slouchcat-updater:1.18.0@` + otherDig + `"`, upd, 0},
		{"server image", `,"updaterImage":"` + testImage + `"`, "", 0},
		{"no digest", `,"updaterImage":"ghcr.io/slouchcat/slouchcat-updater:1.18.0"`, "", 0},
		{"env file", `,"deploy":{"compose.yaml":"` + sum + `",".env":"` + sum + `"}`, "", 0},
		{"path", `,"deploy":{"../compose.yaml":"` + sum + `"}`, "", 0},
		{"short hash", `,"deploy":{"compose.yaml":"abc"}`, "", 0},
		{"upper case hash", `,"deploy":{"compose.yaml":"` + strings.ToUpper(sum) + `"}`, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.manifest = `{"version":"1.16.0","image":"` + testImage + `"` + tc.fields + `}`
			h.start()
			m, err := h.u.fetchManifest(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if got := updaterRef(m.UpdaterImage); got != tc.image || len(m.Deploy) != tc.deploy {
				t.Errorf("updaterImage %q (ref %q), deploy %v", m.UpdaterImage, got, m.Deploy)
			}
		})
	}
}

// Every answer carries the updater's own version; state.json does not.
func TestStatusCarriesUpdaterVersion(t *testing.T) {
	h := newHarness(t)
	h.version = "1.18.0"
	h.fake.pullGate = make(chan struct{})
	h.start()
	if _, s := h.request(http.MethodGet, updater.PathStatus, "Bearer "+string(h.token)); s.Updater != "1.18.0" {
		t.Errorf("status = %+v", s)
	}
	if status, s := h.install(); status != http.StatusAccepted || s.Updater != "1.18.0" {
		t.Errorf("install = %d %+v", status, s)
	}
	if status, s := h.install(); status != http.StatusConflict || s.Updater != "1.18.0" {
		t.Errorf("second install = %d %+v", status, s)
	}
	close(h.fake.pullGate)
	h.finish()
	h.manifest = `{"version":"1.15.0","image":"` + testImage + `"}`
	if status, s := h.install(); status != http.StatusUnprocessableEntity || s.Updater != "1.18.0" {
		t.Errorf("refused install = %d %+v", status, s)
	}
	data, err := os.ReadFile(filepath.Join(h.deploy, "updater", updater.StateFile))
	if err != nil || strings.Contains(string(data), `"updater"`) {
		t.Errorf("state.json = %s, %v", data, err)
	}
}

// Pull: three attempts, then the last stderr line in the message; a full disk is final.
func TestInstallPullRetries(t *testing.T) {
	cmdErr := func(line string, full bool) error {
		return &commandError{verb: "compose", err: errors.New("exit status 1"), line: line, diskFull: full}
	}
	t.Run("third attempt", func(t *testing.T) {
		h := newHarness(t)
		h.fake.pullErrs = []error{cmdErr("timeout", false), cmdErr("timeout", false)}
		newVersionComesUp(h)
		h.start()
		h.install()
		if s := h.finish(); s.Phase != updater.PhaseDone {
			t.Errorf("state = %+v", s)
		}
		if got := h.fake.verbs(); !slices.Equal(got, []string{"pull", "pull", "pull", "config", "up"}) {
			t.Errorf("compose calls = %v", got)
		}
	})
	t.Run("all fail", func(t *testing.T) {
		h := newHarness(t)
		h.fake.pullErr = cmdErr("Error response from daemon: toomanyrequests", false)
		h.start()
		h.install()
		s := h.finish()
		if s.Code != updater.CodePullFailed || s.Message != "docker compose pull failed: Error response from daemon: toomanyrequests" {
			t.Errorf("state = %+v", s)
		}
		if got := h.fake.verbs(); len(got) != 3 {
			t.Errorf("compose calls = %v", got)
		}
	})
	t.Run("disk full", func(t *testing.T) {
		h := newHarness(t)
		h.fake.pullErr = cmdErr("write /var/lib/docker/x: no space left on device", true)
		h.start()
		h.install()
		s := h.finish()
		if s.Phase != updater.PhaseFailed || s.Code != updater.CodeDiskFull || !strings.HasSuffix(s.Message, "no space left on device") {
			t.Errorf("state = %+v", s)
		}
		if got := h.fake.verbs(); len(got) != 1 {
			t.Errorf("compose calls = %v, want one pull", got)
		}
		if h.readEnv() != oldEnv {
			t.Error(".env not put back")
		}
	})
}

// After done the server images go except the running one and the one of from; a failure to
// remove one is only logged.
func TestInstallRemovesOldServerImages(t *testing.T) {
	const repo = "ghcr.io/slouchcat/slouchcat"
	h := newHarness(t)
	newVersionComesUp(h)
	up := h.fake.afterUp
	h.fake.afterUp = func(f *fakeDocker, n int) {
		up(f, n)
		f.c.ImageID = "sha256:new"
	}
	h.fake.images = []Image{
		{"sha256:new", repo, "1.16.0", "sha256:n"},
		{"sha256:new", repo, "latest", "sha256:n"},
		{"sha256:old", repo, "1.15.0", "sha256:o"},
		{"sha256:older", repo, "1.14.0", "sha256:p"},
		{"sha256:inuse", repo, "1.13.0", "sha256:q"},
		{"sha256:dangling", repo, "<none>", "sha256:dd"},
		{"sha256:bare", repo, "<none>", "<none>"},
		{"sha256:upd", "ghcr.io/slouchcat/slouchcat-updater", "1.17.0", "sha256:u"},
	}
	h.fake.rmErr = map[string]error{repo + ":1.13.0": errors.New("image is being used by a container")}
	h.start()
	h.install()
	if s := h.finish(); s.Phase != updater.PhaseDone {
		t.Errorf("state = %+v", s)
	}
	if want := []string{repo + ":1.14.0", repo + "@sha256:dd"}; !slices.Equal(h.fake.removed, want) {
		t.Errorf("removed %v, want %v", h.fake.removed, want)
	}
}

const (
	ownChanges = "compose.yaml has own changes and was not replaced; compare it with the release file"
	noneKnown  = "the release names no earlier content of compose.yaml, so it was not replaced; compare it with the release file"
)

// A compose file whose content no release had (own changes), or with no valid deployKnown, stays;
// the install goes on and names it in its final message, also when it is done.
func TestInstallKeepsComposeFileWithOwnChanges(t *testing.T) {
	sum := hexSum([]byte(oldCompose))
	tooMany := `{"compose.yaml":["` + strings.TrimSuffix(strings.Repeat(sum+`","`, maxKnown+1), `","`) + `"]}`
	for _, tc := range []struct {
		name, known, compose, msg string
	}{
		{"own changes", `{"compose.yaml":["` + sum + `"]}`, "services: {mine: {}}\n", ownChanges},
		{"no deployKnown", `null`, oldCompose, noneKnown},
		{"other file known", `{"compose.intel.yaml":["` + sum + `"]}`, oldCompose, noneKnown},
		{"bad hash", `{"compose.yaml":["` + strings.ToUpper(sum) + `"]}`, oldCompose, noneKnown},
		{"bad name", `{"compose.yaml":["` + sum + `"],".env":["` + sum + `"]}`, oldCompose, noneKnown},
		{"too many", tooMany, oldCompose, noneKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if err := os.WriteFile(filepath.Join(h.deploy, "compose.yaml"), []byte(tc.compose), 0o600); err != nil {
				t.Fatal(err)
			}
			withDeployKnown(h, tc.known)
			newVersionComesUp(h)
			h.start()
			h.install()
			s := h.finish()
			if s.Phase != updater.PhaseDone || s.Message != tc.msg || s.DeployBackup != "" {
				t.Errorf("state = %+v", s)
			}
			if h.readFile("compose.yaml") != tc.compose || exists(t, filepath.Join(h.deploy, "compose.yaml.bak-vor-1.16.0")) {
				t.Error("compose.yaml was touched")
			}
		})
	}
	t.Run("failed install", func(t *testing.T) {
		h := newHarness(t)
		if err := os.WriteFile(filepath.Join(h.deploy, "compose.yaml"), []byte("mine\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		withDeploy(h)
		h.fake.pullErr = errors.New("exit status 1")
		h.start()
		h.install()
		if s := h.finish(); s.Code != updater.CodePullFailed || s.Message != "docker compose pull failed; "+ownChanges {
			t.Errorf("state = %+v", s)
		}
	})
}

// Only a compose file directly in the project folder is replaced, not one in a subfolder or one
// that links out of it.
func TestInstallReplacesOnlyComposeFilesInTheProjectFolder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, h *harness) string // returns the listed file
	}{
		{"subfolder", func(t *testing.T, h *harness) string {
			sub := filepath.Join(h.deploy, "extra", "compose.yaml")
			if err := os.MkdirAll(filepath.Dir(sub), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sub, []byte(oldCompose), 0o600); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
		{"link out of the folder", func(t *testing.T, h *harness) string {
			sub := filepath.Join(h.deploy, "extra", "real.yaml")
			if err := os.MkdirAll(filepath.Dir(sub), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sub, []byte(oldCompose), 0o600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(h.deploy, "compose.yaml")
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(sub, link); err != nil {
				t.Fatal(err)
			}
			return link
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			listed := tc.setup(t, h)
			h.fake.c.Labels[labelConfigFiles] = listed + "," + filepath.Join(h.deploy, "compose.intel.yaml")
			withDeploy(h)
			newVersionComesUp(h)
			h.start()
			h.install()
			if s := h.finish(); s.Phase != updater.PhaseDone || s.DeployBackup != "" || s.Message != "" {
				t.Errorf("state = %+v", s)
			}
			if data, err := os.ReadFile(listed); err != nil || string(data) != oldCompose {
				t.Errorf("%s = %q, %v", listed, data, err)
			}
		})
	}
}
