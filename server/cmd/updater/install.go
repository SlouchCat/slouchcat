package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/updater"
)

const (
	// service is the compose service of the server; an install never touches another one, the
	// updater included (ADR 0024 point 13). updaterService is the updater's own, which only a
	// self-update recreates (ADR 0024 addendum point 2).
	service        = "slouchcat"
	updaterService = "updater"

	labelProject     = "com.docker.compose.project"
	labelConfigFiles = "com.docker.compose.project.config_files"
	labelWorkingDir  = "com.docker.compose.project.working_dir"
	labelEnvFile     = "com.docker.compose.project.environment_file"
	// labelVersion is set by deploy/Dockerfile from the same build argument that goes into the
	// binary (main.version), so it is the version the server itself reports.
	labelVersion = "org.opencontainers.image.version"

	manifestTimeout  = 20 * time.Second
	maxManifestBytes = updater.MaxManifestBytes
	// The bounds of an install are shared with the server (updater.InstallWorstCase).
	checkTimeout   = updater.CheckTimeout
	pullTimeout    = updater.PullTimeout
	commandTimeout = updater.CommandTimeout
	waitTimeout    = updater.WaitTimeout
	pollInterval   = 2 * time.Second
)

// pullPauses are the waits between the attempts of docker compose pull (ADR 0024 addendum
// point 4): three attempts, all within pullTimeout.
var pullPauses = []time.Duration{15 * time.Second, 45 * time.Second}

// digestRef is the form of the manifest field image: a repository with a sha256 digest.
var digestRef = regexp.MustCompile(`^[a-z0-9][a-z0-9._/:-]*@(sha256:[0-9a-f]{64})$`)

// Updater runs one install at a time and keeps its state in state.json.
type Updater struct {
	cfg    settings
	docker Docker
	client *http.Client
	log    *slog.Logger
	now    func() time.Time

	// version is the updater's own (main.version); /status reports it, and a self-update
	// compares it with the release.
	version string

	// The waits; tests shorten them.
	every, wait time.Duration
	// pullPauses are the waits between pull attempts; selfEvery the interval of the self-update
	// check (Watch).
	pullPauses []time.Duration
	selfEvery  time.Duration
	// helperCheck is how long a self-update watches its helper container after the start.
	helperCheck time.Duration

	// ctx is the lifetime of the process; an install runs under it, not under its request.
	ctx context.Context
	wg  sync.WaitGroup
	// resumeFrom is the phase (restarting, waiting) of an install recover found interrupted
	// after its restart; Resume waits for it in the background.
	resumeFrom updater.Phase

	mu    sync.Mutex
	state updater.State
	// selfBusy is set while a self-update runs its steps, selfHold until when /install stays busy
	// after the helper started (it recreates this container). An install and a self-update never
	// overlap.
	selfBusy bool
	selfHold time.Time
}

// plan is an accepted install.
type plan struct {
	from, to string
	digest   string // sha256:… of the manifest
	project  Project
	env      string              // the env file whose SLOUCHCAT_VERSION line is set
	deploy   map[string]string   // compose files of the release: base name → sha256 (manifest)
	known    map[string][]string // their sha256 in all earlier releases (manifest deployKnown)
}

// refusal stops an install before anything changed.
type refusal struct {
	code     updater.Code
	msg      string
	from, to string
}

func (r *refusal) Error() string { return string(r.code) + ": " + r.msg }

func refuse(code updater.Code, format string, args ...any) *refusal {
	return &refusal{code: code, msg: fmt.Sprintf(format, args...)}
}

// State returns the current state.
func (u *Updater) State() updater.State {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

// statePath is <deploy>/updater/state.json.
func (u *Updater) statePath() string { return filepath.Join(u.cfg.dir(), updater.StateFile) }

// busy reports whether an install or a self-update runs; u.mu is held.
func (u *Updater) busy() bool {
	return u.state.Phase.Running() || u.selfBusy || u.now().Before(u.selfHold)
}

// set makes s the current state and writes it to state.json.
func (u *Updater) set(s updater.State) {
	u.mu.Lock()
	u.state = s
	u.mu.Unlock()
	u.persist(s)
}

// persist writes state.json (0644, owned by PUID:PGID, so the server can read it). Mode and
// owner go on the temporary file before the rename, never on the path: the server can write the
// folder, and a chown by path would follow a link it put there.
func (u *Updater) persist(s updater.State) {
	if err := updater.WriteJSONOwned(u.statePath(), s, 0o644, u.cfg.uid, u.cfg.gid); err != nil {
		u.log.Error("updater: write state", "err", err)
	}
}

// Start begins an install. It runs the checks first and answers with the status code of the
// HTTP response: 202 and the new state when the install runs, 409 while one runs already (or a
// self-update), 422
// with a failed state (not written to state.json) when a check refused it.
func (u *Updater) Start() (updater.State, int) {
	if u.helperRunning() { // a self-update helper is recreating the updater
		s := u.State()
		s.Code = updater.CodeBusy
		return s, http.StatusConflict
	}
	u.mu.Lock()
	if u.busy() {
		s := u.state
		u.mu.Unlock()
		s.Code = updater.CodeBusy
		return s, http.StatusConflict
	}
	prev := u.state
	checking := updater.State{Phase: updater.PhaseChecking, StartedAt: u.now()}
	u.state = checking
	u.mu.Unlock()
	u.persist(checking)

	ctx, cancel := context.WithTimeout(u.ctx, checkTimeout)
	defer cancel()
	p, err := u.check(ctx)
	if err != nil {
		var r *refusal
		if !errors.As(err, &r) {
			r = &refusal{code: updater.CodeManifestInvalid, msg: err.Error()}
		}
		u.set(prev) // nothing changed: state.json keeps the last install
		u.log.Info("updater: install refused", "code", r.code, "message", r.msg)
		return updater.State{
			Phase: updater.PhaseFailed, From: r.from, To: r.to, StartedAt: checking.StartedAt,
			FinishedAt: u.now(), Code: r.code, Message: r.msg,
		}, http.StatusUnprocessableEntity
	}
	s := updater.State{Phase: updater.PhaseBackingUp, From: p.from, To: p.to, StartedAt: checking.StartedAt}
	u.set(s)
	u.log.Info("updater: install started", "from", p.from, "to", p.to)
	u.wg.Go(func() { u.run(p, s) })
	return s, http.StatusAccepted
}

// check reads the manifest and the running container and returns the plan, or a refusal.
func (u *Updater) check(ctx context.Context) (plan, error) {
	m, err := u.fetchManifest(ctx)
	if err != nil {
		return plan{}, err
	}
	c, err := u.docker.Container(ctx, u.cfg.container)
	if err != nil {
		return plan{}, refuse(updater.CodeNoComposeLabels, "container %s not found", u.cfg.container)
	}
	from := runningVersion(c)
	if !updater.Newer(m.Version, from) {
		r := refuse(updater.CodeNotNewer, "release %s is not newer than the running version %s", m.Version, from)
		r.from, r.to = from, m.Version
		return plan{}, r
	}
	withVersions := func(r *refusal) error {
		r.from, r.to = from, m.Version
		return r
	}
	match := digestRef.FindStringSubmatch(m.Image)
	if match == nil {
		return plan{}, withVersions(refuse(updater.CodeNoDigest, "the manifest names no image digest"))
	}
	proj, r := u.project(c)
	if r != nil {
		return plan{}, withVersions(r)
	}
	env, err := envTarget(proj.EnvFiles)
	if err != nil {
		return plan{}, withVersions(refuse(updater.CodeOutsideDeployDir, "%v", err))
	}
	return plan{from: from, to: m.Version, digest: match[1], project: proj, env: env, deploy: m.Deploy, known: m.DeployKnown}, nil
}

// manifest is the part of the release manifest the updater reads. UpdaterImage and Deploy came
// with 1.18.0 (ADR 0024 addendum points 2 and 3); an older manifest has neither.
type manifest struct {
	Version      string            `json:"version"`
	Image        string            `json:"image"`
	UpdaterImage string            `json:"updaterImage"`
	Deploy       map[string]string `json:"deploy"`
	// DeployKnown lists for each compose file the sha256 it had in every earlier release: only a
	// file with one of them is replaced, a file with own changes stays.
	DeployKnown map[string][]string `json:"deployKnown"`
}

// maxKnown bounds the earlier hashes of one file in deployKnown.
const maxKnown = 200

var (
	// deployName is the form of a file name in the manifest field deploy: a compose file of
	// deploy/, never .env or a path.
	deployName = regexp.MustCompile(`^compose[a-z0-9.-]*\.yaml$`)
	// sha256Hex is a sha256 in lowercase hex.
	sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// checkOptional drops a field since 1.18.0 that does not have its form, with a log line instead
// of a refusal: an install needs neither. A deploy list with one bad entry is dropped whole, so
// the compose files of a release are replaced together or not at all.
func (u *Updater) checkOptional(m *manifest) {
	if m.UpdaterImage != "" && updaterRef(m.UpdaterImage) == "" {
		u.log.Warn("updater: manifest field updaterImage ignored: not a slouchcat-updater image with a digest",
			"updater_image", truncate(m.UpdaterImage, 200))
		m.UpdaterImage = ""
	}
	for name, sum := range m.Deploy {
		if !deployName.MatchString(name) || !sha256Hex.MatchString(sum) {
			u.log.Warn("updater: manifest field deploy ignored: an entry is not a compose file with a sha256",
				"name", truncate(name, 80))
			m.Deploy = nil
			break
		}
	}
	if !validKnown(m.DeployKnown) {
		u.log.Warn("updater: manifest field deployKnown ignored: an entry is not a compose file with at most 200 sha256")
		m.DeployKnown = nil // as if missing: no compose file counts as unchanged
	}
}

// validKnown reports whether every entry of deployKnown is a compose file name with at most
// maxKnown sha256 values.
func validKnown(known map[string][]string) bool {
	for name, sums := range known {
		if !deployName.MatchString(name) || len(sums) > maxKnown {
			return false
		}
		for _, sum := range sums {
			if !sha256Hex.MatchString(sum) {
				return false
			}
		}
	}
	return true
}

// updaterRef returns the manifest field updaterImage as repository@digest (a tag dropped), or ""
// when it is not a digest reference (digestRef) of a repository named slouchcat-updater.
func updaterRef(s string) string {
	match := digestRef.FindStringSubmatch(s)
	if match == nil {
		return ""
	}
	repo := repository(s)
	if path.Base(repo) != "slouchcat-updater" {
		return ""
	}
	return repo + "@" + match[1]
}

// repository returns an image reference without its digest and tag.
func repository(ref string) string {
	ref, _, _ = strings.Cut(ref, "@")
	if i := strings.LastIndexByte(ref, ':'); i > strings.LastIndexByte(ref, '/') {
		ref = ref[:i]
	}
	return ref
}

func (u *Updater) fetchManifest(ctx context.Context) (manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, manifestTimeout)
	defer cancel()
	if p, err := url.Parse(u.cfg.manifestURL); err != nil || p.Scheme != "https" {
		return manifest{}, refuse(updater.CodeManifestUnreachable, "manifest address is not https")
	}
	data, err := updater.FetchManifest(ctx, u.client, u.cfg.manifestURL, "slouchcat-updater")
	if se := (*updater.StatusError)(nil); errors.As(err, &se) {
		return manifest{}, refuse(updater.CodeManifestUnreachable, "manifest: HTTP %d", se.Code)
	}
	if err != nil {
		return manifest{}, refuse(updater.CodeManifestUnreachable, "manifest: %v", err)
	}
	if len(data) > maxManifestBytes {
		return manifest{}, refuse(updater.CodeManifestInvalid, "manifest larger than %d bytes", maxManifestBytes)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, refuse(updater.CodeManifestInvalid, "manifest: %v", err)
	}
	if !updater.ValidRelease(m.Version) {
		return manifest{}, refuse(updater.CodeManifestInvalid, "manifest version %q is not a release", m.Version)
	}
	u.checkOptional(&m)
	return m, nil
}

// runningVersion is the version of the running server: the image label
// org.opencontainers.image.version, which deploy/Dockerfile fills from the build argument that
// also goes into the binary, so it is what the server reports and its update check compares.
// The tag is only the fallback for an image without the label: it is often not a version at all
// ("latest", "main-b87ac59", a branch name), and a locally built image keeps the tag of the
// release it replaced.
func runningVersion(c Container) string {
	if v := strings.TrimSpace(c.Labels[labelVersion]); v != "" {
		return v
	}
	ref, _, _ := strings.Cut(c.Image, "@")
	if i := strings.LastIndexByte(ref, ':'); i > strings.LastIndexByte(ref, '/') {
		return ref[i+1:]
	}
	return ""
}

// project reads the compose labels of the server container (projectOf).
func (u *Updater) project(c Container) (Project, *refusal) { return u.projectOf(c, u.cfg.container) }

// projectOf reads the compose labels of the container name and checks that every file lies
// inside the deploy folder (ADR 0024 points 5 and 6).
func (u *Updater) projectOf(c Container, name string) (Project, *refusal) {
	p := Project{Name: c.Labels[labelProject], WorkingDir: c.Labels[labelWorkingDir]}
	for f := range strings.SplitSeq(c.Labels[labelConfigFiles], ",") {
		if f = strings.TrimSpace(f); f != "" {
			p.ConfigFiles = append(p.ConfigFiles, f)
		}
	}
	if p.Name == "" || p.WorkingDir == "" || len(p.ConfigFiles) == 0 {
		return Project{}, refuse(updater.CodeNoComposeLabels, "container %s was not started by docker compose", name)
	}
	for f := range strings.SplitSeq(c.Labels[labelEnvFile], ",") {
		if f = strings.TrimSpace(f); f != "" {
			p.EnvFiles = append(p.EnvFiles, f)
		}
	}
	if len(p.EnvFiles) == 0 {
		// Compose reads .env in the project folder when no --env-file is given; it records no
		// label then.
		p.EnvFiles = []string{filepath.Join(p.WorkingDir, ".env")}
	}
	// The working folder counts too: relative paths of the compose files (./config, ./cache)
	// resolve against it, and only the deploy folder is mounted at the host's path.
	for _, f := range append(append([]string{p.WorkingDir}, p.ConfigFiles...), p.EnvFiles...) {
		if !inside(u.cfg.deployDir, f) {
			return Project{}, refuse(updater.CodeOutsideDeployDir, "%s lies outside SLOUCHCAT_DEPLOY_DIR %s", f, u.cfg.deployDir)
		}
	}
	return p, nil
}

// run carries an accepted install through; s is its state in the phase backing_up.
func (u *Updater) run(p plan, s updater.State) {
	ctx := u.ctx
	phase := func(ph updater.Phase) {
		s.Phase = ph
		u.set(s)
		u.log.Info("updater: phase", "phase", ph, "to", p.to)
	}
	// note names the compose files left alone for own changes; it goes into every final message.
	var note string
	finish := func(ph updater.Phase, code updater.Code, msg string) {
		s.Phase, s.Code, s.Message, s.FinishedAt, s.RollingBack = ph, code, withNote(msg, note), u.now(), false
		u.set(s)
		u.log.Info("updater: install finished", "phase", ph, "code", code, "message", msg, "from", p.from, "to", p.to)
	}
	// interrupted reports whether the updater is being stopped. The install then ends where it
	// is, without a rollback (its commands would run with a dead context) and without a final
	// state: state.json keeps the running phase, and recover takes over at the next start.
	interrupted := func() bool {
		if ctx.Err() == nil {
			return false
		}
		u.log.Warn("updater: stopped during the install; the next start takes over", "phase", s.Phase, "to", p.to)
		return true
	}
	// A recreate must not be cut off half-way by the shutdown: compose up runs to its own end
	// (the updater waits for it, see drain).
	upCtx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.WithoutCancel(ctx), commandTimeout)
	}

	// backing_up: save the env file, then set the one line.
	env, err := readEnvFile(p.env, u.cfg.uid, u.cfg.gid)
	if err == nil {
		s.EnvBackup, err = env.backup(p.to)
	}
	if err != nil {
		u.log.Error("updater: back up env file", "err", err)
		finish(updater.PhaseFailed, "", "could not back up the env file: "+err.Error())
		return
	}
	u.set(s) // names the backup before the env file changes: a start after an interruption puts that one back
	if err := env.write(setVersion(env.data, p.to)); err != nil {
		u.log.Error("updater: write env file", "err", err)
		u.restoreEnv(env) // the rename may have happened before the error
		finish(updater.PhaseFailed, "", "could not write the env file: "+err.Error())
		return
	}
	// The compose files of the release, still before the pull (ADR 0024 addendum point 3).
	deploy, note, err := u.replaceDeployFiles(ctx, p, &s)
	if err != nil {
		u.log.Error("updater: compose files", "err", err)
		u.restoreEnv(env)
		if interrupted() {
			return
		}
		finish(updater.PhaseFailed, updater.CodeDeployFiles, "could not replace the compose files of the release: "+err.Error())
		return
	}
	// undo puts back the env file and the compose files where the install stops before the
	// restart.
	undo := func() {
		u.restoreDeploy(deploy)
		u.restoreEnv(env)
	}

	// pulling
	phase(updater.PhasePulling)
	err = u.pull(ctx, p.project, service)
	if err != nil {
		undo()
		if interrupted() {
			return
		}
		code := updater.CodePullFailed
		if diskFull(err) {
			code = updater.CodeDiskFull
		}
		finish(updater.PhaseFailed, code, failedMessage("docker compose pull failed", err))
		return
	}

	// verifying: the image compose will start must carry the digest of the manifest. compose
	// pull only warns when a service with a build section fails to pull, so a missing image ends
	// here as well.
	phase(updater.PhaseVerifying)
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	code, msg := u.verify(cctx, p)
	cancel()
	if code != "" || ctx.Err() != nil {
		undo()
		if interrupted() {
			return
		}
		finish(updater.PhaseFailed, code, msg)
		return
	}

	// restarting: no --wait. The health check of the image (3 retries, 5 s apart) calls the
	// container unhealthy after about 16 s while a long migration runs; compose would give up
	// although the server is fine. The updater waits itself, below.
	phase(updater.PhaseRestarting)
	cause := updater.CodeUnhealthy
	var causeMsg string
	cctx, cancel = upCtx()
	err = u.up(cctx, p.project)
	cancel()
	if err == nil {
		// waiting
		phase(updater.PhaseWaiting)
		rec, end := u.waitFor(ctx, p.to)
		if end == waitOK {
			s.Migrated = rec.Migrated()
			s.Backup = rec.Backup
			finish(updater.PhaseDone, "", "")
			u.afterDone(p.from)
			return
		}
		causeMsg = fmt.Sprintf("version %s %s", p.to, u.failCause(end))
	} else {
		causeMsg = fmt.Sprintf("docker compose up failed for version %s", p.to)
	}
	if interrupted() {
		return
	}

	// The new version failed. With a migration nothing is taken back (F-14): the old server
	// is not built for the newer schema, and the way back is the backup taken before it.
	if rec, ok := u.migrated(p.to); ok {
		s.Migrated, s.Backup = true, rec.Backup
		finish(updater.PhaseFailed, updater.CodeMigratedNoRollback,
			fmt.Sprintf("version %s migrated the database and did not become healthy; back only with the backup %s", p.to, rec.Backup))
		return
	}
	// The rollback starts: from here a start after an interruption finishes it (recover).
	s.RollingBack = true
	u.set(s)
	if err := env.restore(); err != nil {
		u.log.Error("updater: restore env file", "err", err)
		finish(updater.PhaseFailed, updater.CodeRollbackFailed, "could not restore the env file: "+err.Error())
		return
	}
	if err := putBack(deploy); err != nil {
		u.log.Error("updater: restore compose files", "err", err)
		finish(updater.PhaseFailed, updater.CodeRollbackFailed, "could not restore the compose files: "+err.Error())
		return
	}
	cctx, cancel = upCtx()
	err = u.up(cctx, p.project)
	cancel()
	healthy := false
	if err == nil {
		healthy, _ = u.waitHealthy(ctx)
	}
	if healthy {
		finish(updater.PhaseRolledBack, cause, causeMsg+fmt.Sprintf("; version %s runs again", p.from))
		return
	}
	if interrupted() {
		return
	}
	finish(updater.PhaseFailed, updater.CodeRollbackFailed, causeMsg+fmt.Sprintf("; version %s did not come back either", p.from))
}

// rollbackAfterRestart finishes a rollback that an interruption cut off; s is in a running phase
// with RollingBack set. The new version failed without a migration (run checked that before it
// set RollingBack), so the old one may come back: the env file is put back when it still names
// s.To, compose up starts what it names (nothing happens when the old version already runs), and
// the install ends rolled_back once version s.From runs healthy, otherwise failed. A shutdown
// leaves the phase for the next start.
func (u *Updater) rollbackAfterRestart(s updater.State) {
	ctx := u.ctx
	finish := func(ph updater.Phase, code updater.Code, msg string) {
		s.Phase, s.Code, s.Message, s.FinishedAt, s.RollingBack = ph, code, msg, u.now(), false
		u.set(s)
		u.log.Info("updater: interrupted rollback finished", "phase", ph, "code", code, "message", msg, "from", s.From, "to", s.To)
	}
	cause := fmt.Sprintf("version %s did not become healthy and the updater stopped while taking it back", s.To)
	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	envMsg, envOK := u.recoverEnv(cctx, s.To, s.EnvBackup)
	cancel()
	var proj Project
	if envOK {
		cctx, cancel = context.WithTimeout(ctx, commandTimeout)
		c, err := u.docker.Container(cctx, u.cfg.container)
		cancel()
		var r *refusal
		if err == nil {
			proj, r = u.project(c)
		}
		envOK = err == nil && r == nil
	}
	if !envOK {
		if ctx.Err() != nil {
			return
		}
		finish(updater.PhaseFailed, updater.CodeRollbackFailed, cause+"; "+envMsg)
		return
	}
	if err := u.restoreDeployBackups(proj, s.To, s.DeployBackup); err != nil {
		u.log.Error("updater: restore compose files", "err", err)
		finish(updater.PhaseFailed, updater.CodeRollbackFailed, cause+"; could not restore the compose files: "+err.Error())
		return
	}
	cctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), commandTimeout)
	err := u.up(cctx, proj)
	cancel()
	healthy := false
	if err == nil {
		healthy, _ = u.poll(ctx, func(c Container) bool { return c.Healthy() && runningVersion(c) == s.From })
	}
	if healthy {
		finish(updater.PhaseRolledBack, updater.CodeUnhealthy, cause+fmt.Sprintf("; version %s runs again", s.From))
		return
	}
	if ctx.Err() != nil {
		u.log.Warn("updater: stopped while taking the install back again; the next start takes over", "to", s.To)
		return
	}
	finish(updater.PhaseFailed, updater.CodeRollbackFailed, cause+fmt.Sprintf("; version %s did not come back either", s.From))
}

// drain waits up to d for a running install to end once the process context is done; it reports
// whether it ended.
func (u *Updater) drain(d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		u.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// verify compares the digest of the image compose resolves now with the manifest.
func (u *Updater) verify(ctx context.Context, p plan) (updater.Code, string) {
	return u.verifyDigest(ctx, p.project, service, p.digest)
}

// verifyDigest compares the digest of the image compose resolves now for svc with digest.
func (u *Updater) verifyDigest(ctx context.Context, p Project, svc, digest string) (updater.Code, string) {
	ref, err := u.docker.ComposeImage(ctx, p, svc)
	if err != nil {
		return updater.CodePullFailed, "could not resolve the image of the service"
	}
	digests, err := u.docker.RepoDigests(ctx, ref)
	if err != nil {
		return updater.CodePullFailed, fmt.Sprintf("image %s is not there after the pull", ref)
	}
	for _, d := range digests {
		// The digest is content-addressed: equal digests are the same image, whatever the
		// repository name (a mirror, a renamed registry).
		if _, got, ok := strings.Cut(d, "@"); ok && got == digest {
			return "", ""
		}
	}
	return updater.CodeDigestMismatch, fmt.Sprintf("image %s does not carry the digest of the manifest", ref)
}

// up recreates only the server container with what is on disk now.
func (u *Updater) up(ctx context.Context, p Project) error {
	return u.docker.Compose(ctx, p, "up", "-d", "--no-deps", "--no-build", service)
}

// pull runs docker compose pull for svc up to len(u.pullPauses)+1 times, with those pauses in
// between, all within pullTimeout (ADR 0024 addendum point 4). A full disk is final, and so is
// the end of ctx.
func (u *Updater) pull(ctx context.Context, p Project, svc string) error {
	ctx, cancel := context.WithTimeout(ctx, pullTimeout)
	defer cancel()
	for i := 0; ; i++ {
		err := u.docker.Compose(ctx, p, "pull", svc)
		if err == nil || diskFull(err) || i >= len(u.pullPauses) || ctx.Err() != nil {
			return err
		}
		u.log.Warn("updater: pull failed, trying again", "service", svc, "attempt", i+1, "pause", u.pullPauses[i], "err", err)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(u.pullPauses[i]):
		}
	}
}

// withNote appends note to msg ("; " between them).
func withNote(msg, note string) string {
	if note == "" || msg == "" {
		return msg + note
	}
	return msg + "; " + note
}

// failedMessage is msg with the stderr line of the failed docker call, when it has one.
func failedMessage(msg string, err error) string {
	if line := errorLine(err); line != "" {
		return msg + ": " + line
	}
	return msg
}

// restoreEnv puts the env file back and logs a failure; used where the install stops anyway.
func (u *Updater) restoreEnv(env envFile) {
	if err := env.restore(); err != nil {
		u.log.Error("updater: restore env file", "err", err)
	}
}

// waitEnd tells why waitFor ended.
type waitEnd int

const (
	waitOK        waitEnd = iota // healthy and server.json reports the version
	waitGaveUp                   // the container stopped for good (Container.GaveUp)
	waitSilent                   // healthy at the last look, but server.json never reported the version
	waitUnhealthy                // not healthy within the wait (or the wait was cut off)
)

// waitFor waits until the container is healthy and server.json reports version v. "unhealthy"
// is not final within the wait: the server may still be migrating. A release before 1.16.0
// runs healthy but writes no server.json: waitSilent tells that from waitUnhealthy.
func (u *Updater) waitFor(ctx context.Context, v string) (rec updater.ServerRecord, end waitEnd) {
	healthy := false
	ok, gaveUp := u.poll(ctx, func(c Container) bool {
		healthy = c.Healthy()
		if !healthy {
			return false
		}
		r, err := u.serverRecord()
		rec = r
		return err == nil && r.Version == v
	})
	switch {
	case ok:
		return rec, waitOK
	case gaveUp:
		return rec, waitGaveUp
	case healthy:
		return rec, waitSilent
	}
	return rec, waitUnhealthy
}

// failCause words why the wait for version v ended without success, for the message after
// "version v": the install says "version v <cause>".
func (u *Updater) failCause(end waitEnd) string {
	switch end {
	case waitGaveUp:
		return fmt.Sprintf("stopped (exited or restarted %d times) before it became healthy", maxRestarts)
	case waitSilent:
		return fmt.Sprintf("runs healthy but did not report back (server.json) within %s", u.wait)
	}
	return fmt.Sprintf("did not become healthy within %s", u.wait)
}

// waitHealthy waits until the container is healthy (the old server after a rollback; a release
// before 1.16.0 writes no server.json).
func (u *Updater) waitHealthy(ctx context.Context) (ok, gaveUp bool) {
	return u.poll(ctx, Container.Healthy)
}

// poll inspects the container every u.every until done reports true, u.wait has passed or the
// container gave up (gaveUp: it exited for good or restarted maxRestarts times; waiting longer
// would not help).
func (u *Updater) poll(ctx context.Context, done func(Container) bool) (ok, gaveUp bool) {
	deadline := time.Now().Add(u.wait)
	for {
		cctx, cancel := context.WithTimeout(ctx, commandTimeout)
		c, err := u.docker.Container(cctx, u.cfg.container)
		cancel()
		if err == nil && done(c) {
			return true, false
		}
		if err == nil && c.GaveUp() {
			u.log.Warn("updater: container stopped for good", "status", c.Status, "exit_code", c.ExitCode, "restarts", c.RestartCount)
			return false, true
		}
		if !time.Now().Before(deadline) {
			return false, false
		}
		select {
		case <-ctx.Done():
			return false, false
		case <-time.After(u.every):
		}
	}
}

// serverRecord reads server.json.
func (u *Updater) serverRecord() (updater.ServerRecord, error) {
	var rec updater.ServerRecord
	err := updater.ReadJSON(filepath.Join(u.cfg.dir(), updater.ServerFile), &rec)
	return rec, err
}

// migrated reports whether version v ran a migration according to server.json: a record of v
// whose schema changed, or one that started its migrations and never finished them.
func (u *Updater) migrated(v string) (updater.ServerRecord, bool) {
	rec, err := u.serverRecord()
	if err != nil || rec.Version != v {
		return rec, false
	}
	return rec, rec.Migrated()
}
