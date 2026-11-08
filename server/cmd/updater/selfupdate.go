package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/updater"
	semver "github.com/slouchcat/slouchcat/server/internal/version"
)

// The updater follows the server (ADR 0024 addendum point 2): once the server runs the release of
// the manifest and the updater is older, it sets SLOUCHCAT_UPDATER_VERSION in its env file, pulls
// its own image, checks the digest of the manifest field updaterImage and starts a short-lived
// helper container from the new image (`slouchcat-updater self-update`) that recreates the service
// updater: a container cannot replace itself without cutting off the command that does it. When
// the new updater does not come up, the helper puts the env file back and starts the old one. A
// self-update never touches the server or the install record in state.json.

const (
	// helperCommand is the subcommand of the helper container; helperName its container name.
	helperCommand = "self-update"
	helperName    = "slouchcat-updater-self-update"
	// defaultUpdaterContainer is container_name of the service updater in compose.updater.yaml.
	defaultUpdaterContainer = "slouchcat-updater"
	// updaterBinary is the binary in deploy/Dockerfile.updater; dockerSocket the socket it uses.
	updaterBinary = "/usr/local/bin/slouchcat-updater"
	dockerSocket  = "/var/run/docker.sock"

	// selfEvery is the interval of the self-update check (Watch); selfRetry how long a tried
	// version waits before the next try, also across restarts (selfUpdateFile).
	selfEvery = 6 * time.Hour
	selfRetry = 6 * time.Hour
	// selfHold keeps /install busy after the helper started: it recreates this container, twice
	// when the new one fails (compose up, the wait, compose up again).
	selfHold = 2*commandTimeout + helperWait
	// selfUpdateFile records the last self-update tried (selfRecord).
	selfUpdateFile = "self-update.json"

	// helperWait is how long the helper waits for the new updater; helperSettle how long the new
	// container must run unchanged (no restart) before it counts.
	helperWait   = 60 * time.Second
	helperSettle = 10 * time.Second
)

// Watch lets the updater follow the server: once at the start (after recover and Resume), then
// every u.selfEvery until the process ends. main runs it under u.wg, so that drain waits for a
// self-update under way.
func (u *Updater) Watch() {
	u.selfUpdate("start", nil)
	t := time.NewTicker(u.selfEvery)
	defer t.Stop()
	for {
		select {
		case <-u.ctx.Done():
			return
		case <-t.C:
			u.selfUpdate("timer", nil)
		}
	}
}

// afterDone runs after an install ended done: it removes the server images no longer needed
// (ADR 0024 addendum point 5) and lets the updater follow the server. Neither changes the result
// of the install.
func (u *Updater) afterDone(from string) {
	ctx, cancel := context.WithTimeout(u.ctx, manifestTimeout)
	m, err := u.fetchManifest(ctx)
	cancel()
	if err != nil {
		u.log.Info("updater: after the install: manifest not read, no image cleanup and no self-update", "err", err)
		return
	}
	u.removeServerImages(m, from)
	u.selfUpdate("install done", &m)
}

// removeServerImages removes the local images of the server repository (the manifest field image)
// except the one the server runs now and the one of version from, the target of a rollback.
func (u *Updater) removeServerImages(m manifest, from string) {
	if digestRef.FindStringSubmatch(m.Image) == nil {
		return
	}
	ctx, cancel := context.WithTimeout(u.ctx, commandTimeout)
	defer cancel()
	c, err := u.docker.Container(ctx, u.cfg.container)
	if err != nil || c.ImageID == "" {
		u.log.Info("updater: image cleanup skipped: the image of the server is not known", "err", err)
		return
	}
	removeImages(ctx, u.docker, u.log, repository(m.Image), c.ImageID, from)
}

// removeImages removes the local images of repo except the image keepID and every image with one
// of keepTags, by tag (repo:tag) or, without one, by digest (repo@sha256:…). docker image rm runs
// without --force, so an image a container uses stays; a failure is only logged.
func removeImages(ctx context.Context, d Docker, log *slog.Logger, repo, keepID string, keepTags ...string) {
	images, err := d.Images(ctx, repo)
	if err != nil {
		log.Info("updater: image cleanup skipped", "repository", repo, "err", err)
		return
	}
	keep := map[string]bool{keepID: true}
	for _, im := range images {
		if im.Repository == repo && slices.Contains(keepTags, im.Tag) {
			keep[im.ID] = true
		}
	}
	done := map[string]bool{}
	for _, im := range images {
		if im.Repository != repo || im.ID == "" || keep[im.ID] {
			continue
		}
		var ref string
		switch {
		case im.Tag != "" && im.Tag != "<none>":
			ref = repo + ":" + im.Tag
		case strings.HasPrefix(im.Digest, "sha256:"):
			ref = repo + "@" + im.Digest
		default:
			continue
		}
		if done[ref] {
			continue
		}
		done[ref] = true
		if err := d.RemoveImage(ctx, ref); err != nil {
			log.Info("updater: image kept", "image", ref, "err", err)
			continue
		}
		log.Info("updater: image removed", "image", ref)
	}
}

// selfUpdate updates the updater when the manifest (m, or fetched when nil) names a valid
// updaterImage, the server runs the version of the manifest and the updater is older; why names
// the trigger for the log. It runs only while no install runs, and a version is tried at most once
// in selfRetry. A failure is logged and changes nothing else.
func (u *Updater) selfUpdate(why string, m *manifest) {
	if _, ok := semver.Parse(u.version); !ok {
		return // a build without a release version ("dev") never updates itself
	}
	if !u.beginSelf() {
		u.log.Info("updater: self-update check skipped: an install or a self-update runs", "trigger", why)
		return
	}
	handedOver := false
	defer func() { u.endSelf(handedOver) }()
	if m == nil {
		ctx, cancel := context.WithTimeout(u.ctx, manifestTimeout)
		got, err := u.fetchManifest(ctx)
		cancel()
		if err != nil {
			u.log.Info("updater: self-update check: manifest not read", "trigger", why, "err", err)
			return
		}
		m = &got
	}
	ref := updaterRef(m.UpdaterImage)
	if ref == "" || !updater.Newer(m.Version, u.version) {
		return
	}
	ctx, cancel := context.WithTimeout(u.ctx, commandTimeout)
	c, err := u.docker.Container(ctx, u.cfg.container)
	cancel()
	if err != nil || runningVersion(c) != m.Version {
		u.log.Info("updater: self-update waits until the server runs the release", "release", m.Version, "updater", u.version, "trigger", why)
		return
	}
	if !u.claimSelfUpdate(m.Version) {
		u.log.Info("updater: self-update to the release was tried less than 6 h ago", "release", m.Version, "trigger", why)
		return
	}
	u.log.Info("updater: self-update started", "from", u.version, "to", m.Version, "image", ref, "trigger", why)
	if err := u.selfSteps(*m, ref); err != nil {
		u.log.Error("updater: self-update failed; the next try comes in 6 h at the earliest", "to", m.Version, "err", err)
		return
	}
	handedOver = true
	u.log.Info("updater: self-update handed over to the helper container", "helper", helperName, "to", m.Version)
}

// beginSelf marks a self-update as running unless an install or another self-update runs.
func (u *Updater) beginSelf() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.busy() {
		return false
	}
	u.selfBusy = true
	return true
}

// endSelf ends the steps of a self-update; after a handover to the helper /install stays busy for
// selfHold.
func (u *Updater) endSelf(handedOver bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.selfBusy = false
	if handedOver {
		u.selfHold = u.now().Add(selfHold)
	}
}

// selfRecord is the content of self-update.json: the last self-update tried. It survives the
// restart the helper causes when the new updater fails, so that the old one does not try the
// same version again at once. The server can write the folder; a forged record only delays a
// self-update (an entry in the future does not count).
type selfRecord struct {
	Version string    `json:"version"`
	At      time.Time `json:"at"`
}

// claimSelfUpdate records a try of version v and reports whether it may run: not when v was
// tried less than selfRetry ago, nor when the record cannot be written.
func (u *Updater) claimSelfUpdate(v string) bool {
	path := filepath.Join(u.cfg.dir(), selfUpdateFile)
	now := u.now()
	var rec selfRecord
	if err := updater.ReadJSON(path, &rec); err == nil && rec.Version == v && !rec.At.After(now) && now.Sub(rec.At) < selfRetry {
		return false
	}
	if err := updater.WriteJSONOwned(path, selfRecord{Version: v, At: now}, 0o644, u.cfg.uid, u.cfg.gid); err != nil {
		u.log.Error("updater: self-update record", "err", err)
		return false
	}
	return true
}

// selfSteps runs the self-update to the version of m up to the start of the helper; ref is the
// image (repository@digest). Every failure puts the env file back.
func (u *Updater) selfSteps(m manifest, ref string) error {
	ctx := u.ctx
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	c, err := u.docker.Container(cctx, u.cfg.updaterContainer)
	cancel()
	if err != nil {
		return fmt.Errorf("own container %s: %w", u.cfg.updaterContainer, err)
	}
	p, r := u.projectOf(c, u.cfg.updaterContainer)
	if r != nil {
		return r
	}
	target, err := envTargetFor(p.EnvFiles, updaterVersionKey)
	if err != nil {
		return err
	}
	env, err := readEnvFile(target, u.cfg.uid, u.cfg.gid)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}
	backup, err := env.backup("updater-" + m.Version)
	if err != nil {
		return fmt.Errorf("back up env file: %w", err)
	}
	u.log.Info("updater: self-update: env file backed up", "env", env.path, "backup", backup)
	if err := env.write(setValue(env.data, updaterVersionKey, m.Version)); err != nil {
		u.restoreEnv(env) // the rename may have happened before the error
		return fmt.Errorf("write env file: %w", err)
	}
	u.log.Info("updater: self-update: env file names the new updater", "key", updaterVersionKey, "version", m.Version)
	fail := func(err error) error {
		u.restoreEnv(env)
		return err
	}
	if err := u.pull(ctx, p, updaterService); err != nil {
		return fail(errors.New(failedMessage("docker compose pull "+updaterService+" failed", err)))
	}
	_, digest, _ := strings.Cut(ref, "@")
	cctx, cancel = context.WithTimeout(ctx, commandTimeout)
	code, msg := u.verifyDigest(cctx, p, updaterService, digest)
	cancel()
	if code != "" {
		return fail(errors.New(msg))
	}
	u.log.Info("updater: self-update: image pulled, digest checked", "image", ref)
	cctx, cancel = context.WithTimeout(ctx, commandTimeout)
	err = u.docker.Run(cctx, helperArgs(u.cfg, p, ref, env.path, backup)...)
	cancel()
	if err != nil {
		return fail(fmt.Errorf("start the helper container: %w", err))
	}
	return nil
}

// helperArgs are the arguments of docker run for the helper container: the Docker socket and the
// deploy folder at the host's path, no network (it only talks to the socket), the new image ref
// with the updater binary as entrypoint, and the project of the updater container.
func helperArgs(cfg settings, p Project, ref, env, backup string) []string {
	args := []string{"-d", "--rm", "--name", helperName, "--network", "none",
		"-v", dockerSocket + ":" + dockerSocket,
		"-v", cfg.deployDir + ":" + cfg.deployDir,
		"--entrypoint", updaterBinary, ref, helperCommand,
		"--project", p.Name, "--workdir", p.WorkingDir}
	for _, f := range p.ConfigFiles {
		args = append(args, "-f", f)
	}
	for _, f := range p.EnvFiles {
		args = append(args, "--env-file", f)
	}
	return append(args, "--service", updaterService, "--env", env, "--backup", backup, "--container", cfg.updaterContainer)
}

// helper is `slouchcat-updater self-update`, run in the helper container from the new image: it
// recreates the service updater and waits for the new version; when that fails it puts the env
// file back and recreates the old one. It never touches another service.
type helper struct {
	project   Project
	env       string // the env file the self-update changed
	backup    string // base name of its backup next to it; "" when the file did not exist
	container string // container name of the updater

	docker  Docker
	log     *slog.Logger
	version string // the helper's own version: the one the new updater must report

	every, wait, settle time.Duration
}

// stringList is a flag given more than once.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// parseHelper reads the arguments helperArgs writes.
func parseHelper(args []string) (helper, error) {
	flags := flag.NewFlagSet(helperCommand, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var h helper
	var files, envFiles stringList
	flags.StringVar(&h.project.Name, "project", "", "compose project name")
	flags.StringVar(&h.project.WorkingDir, "workdir", "", "compose project directory")
	flags.Var(&files, "f", "compose file (repeated)")
	flags.Var(&envFiles, "env-file", "env file (repeated)")
	svc := flags.String("service", "", "the service to recreate: updater")
	flags.StringVar(&h.env, "env", "", "the env file the self-update changed")
	flags.StringVar(&h.backup, "backup", "", "base name of its backup")
	flags.StringVar(&h.container, "container", defaultUpdaterContainer, "container name of the updater")
	if err := flags.Parse(args); err != nil {
		return h, err
	}
	if flags.NArg() > 0 {
		return h, fmt.Errorf("unexpected arguments %q", flags.Args())
	}
	if *svc != updaterService {
		return h, fmt.Errorf("the helper recreates only the service %s, not %q", updaterService, *svc)
	}
	h.project.ConfigFiles, h.project.EnvFiles = files, envFiles
	if h.project.Name == "" || len(files) == 0 || h.container == "" {
		return h, errors.New("project name, compose files and container are required")
	}
	for _, p := range slices.Concat([]string{h.project.WorkingDir, h.env}, files, envFiles) {
		if !filepath.IsAbs(p) {
			return h, fmt.Errorf("path %q is not absolute", p)
		}
	}
	if h.backup != "" && filepath.Base(h.backup) != h.backup {
		return h, fmt.Errorf("backup %q is not a base name", h.backup)
	}
	return h, nil
}

// selfUpdateMain is the command self-update; it returns the exit code.
func selfUpdateMain(args []string) int {
	log, err := newLog()
	if err != nil {
		fmt.Fprintln(os.Stderr, "updater self-update:", err)
		return 2
	}
	h, err := parseHelper(args)
	if err != nil {
		log.Error("updater self-update: arguments", "err", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	h.docker = dockerCLI{failed: func(args []string, err error, last string) {
		log.Warn("updater self-update: docker command failed", "args", strings.Join(args, " "), "err", err, "last_line", last)
	}}
	h.log, h.version = log, version
	h.every, h.wait, h.settle = pollInterval, helperWait, helperSettle
	return h.run(ctx)
}

// run recreates the updater with the new version; on failure it puts the old one back. It
// returns the exit code.
func (h helper) run(ctx context.Context) int {
	h.log.Info("updater self-update: recreating the updater", "version", h.version, "container", h.container)
	err := h.up(ctx)
	if err == nil {
		c, ok := h.waitUpdater(ctx)
		if ok {
			h.log.Info("updater self-update: the new updater runs", "version", h.version)
			cctx, cancel := context.WithTimeout(ctx, commandTimeout)
			if c.ImageID != "" {
				removeImages(cctx, h.docker, h.log, repository(c.Image), c.ImageID)
			}
			cancel()
			return 0
		}
		err = fmt.Errorf("the updater did not run version %s steadily within %s", h.version, h.wait)
	}
	h.log.Error("updater self-update: failed, putting the old updater back", "err", err)
	if err := h.restoreEnv(); err != nil {
		h.log.Error("updater self-update: env file not put back; the old updater is not started", "env", h.env, "err", err)
		return 1
	}
	h.log.Info("updater self-update: env file put back", "env", h.env, "backup", h.backup)
	if err := h.up(context.WithoutCancel(ctx)); err != nil {
		h.log.Error("updater self-update: the old updater did not start", "err", err)
		return 1
	}
	h.log.Info("updater self-update: the old updater was started again")
	return 1
}

// up recreates only the service updater with what is on disk now, from a local image.
func (h helper) up(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	return h.docker.Compose(ctx, h.project, "up", "-d", "--no-deps", "--force-recreate", "--no-build", "--pull", "never", updaterService)
}

// waitUpdater polls the updater container every h.every for up to h.wait until it runs, has not
// restarted and reports h.version (its image label), unchanged for h.settle.
func (h helper) waitUpdater(ctx context.Context) (Container, bool) {
	deadline := time.Now().Add(h.wait)
	var since time.Time
	for {
		cctx, cancel := context.WithTimeout(ctx, commandTimeout)
		c, err := h.docker.Container(cctx, h.container)
		cancel()
		if err == nil && c.Running && c.Status == "running" && c.RestartCount == 0 && c.Labels[labelVersion] == h.version {
			if since.IsZero() {
				since = time.Now()
			}
			if time.Since(since) >= h.settle {
				return c, true
			}
		} else {
			since = time.Time{}
		}
		if !time.Now().Before(deadline) {
			return Container{}, false
		}
		select {
		case <-ctx.Done():
			return Container{}, false
		case <-time.After(h.every):
		}
	}
}

// restoreEnv puts the env file back from its backup, or removes it when the self-update created
// it. The backup name must be one the self-update writes next to this file.
func (h helper) restoreEnv() error {
	env, err := readEnvFile(h.env, os.Getuid(), os.Getgid())
	if err != nil {
		return err
	}
	if h.backup == "" {
		return envFile{path: env.path}.restore() // the file did not exist before
	}
	re, err := regexp.Compile(`^` + regexp.QuoteMeta(filepath.Base(env.path)+".bak-vor-updater-") + `\d+\.\d+\.\d+(?:-[0-9]+)?$`)
	if err != nil {
		return err
	}
	if !re.MatchString(h.backup) {
		return fmt.Errorf("%w: %q", errBackupName, truncate(h.backup, 80))
	}
	data, _, err := readRegular(filepath.Join(filepath.Dir(env.path), h.backup))
	if err != nil {
		return err
	}
	return env.write(data)
}
