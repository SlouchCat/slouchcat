// Command updater is the updater companion of Slouchcat (concept 15 section 9.3, ADR 0024). It
// runs in its own container with the Docker socket and the deploy folder, listens only on the
// internal Docker network and installs the newest release on the server's request: it reads the
// release manifest itself, sets SLOUCHCAT_VERSION in the env file of the running compose project,
// pulls, checks the digest, recreates the server container and waits for it. A failed start
// without a migration is taken back; with a migration it only reports. It never updates itself.
//
// Environment:
//
//	SLOUCHCAT_DEPLOY_DIR           host path of deploy/, mounted at the same path (required)
//	PUID, PGID                     owner of deploy/updater/ and its files (default 1000)
//	SLOUCHCAT_UPDATE_MANIFEST_URL  release manifest, https only (default config.DefaultUpdateManifestURL)
//	SLOUCHCAT_CONTAINER            container name of the server (default slouchcat)
//	SLOUCHCAT_LOG_FORMAT           json (default) or text
//
// The protocol with the server (paths, files, phases, codes) is package internal/updater.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/config"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/updater"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	shutdownTimeout = 5 * time.Second
	// drainTimeout bounds the wait for a running install at shutdown; it stays below the
	// stop_grace_period of 30 s in compose.updater.yaml, after which Docker kills the process.
	drainTimeout = 25 * time.Second
)

func main() {
	os.Exit(run())
}

func run() int {
	format := logging.Format(strings.TrimSpace(os.Getenv("SLOUCHCAT_LOG_FORMAT")))
	if format == "" {
		format = logging.FormatJSON
	}
	log, err := logging.New(os.Stdout, format, slog.LevelInfo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "updater:", err)
		return 2
	}
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Error("updater: configuration", "err", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	docker := dockerCLI{failed: func(args []string, err error, last string) {
		log.Warn("updater: docker command failed", "args", strings.Join(args, " "), "err", err, "last_line", last)
	}}
	u, token, err := newUpdater(ctx, cfg, docker, log)
	if err != nil {
		log.Error("updater: start", "err", err)
		return 1
	}
	u.Resume()
	srv := &http.Server{
		Addr:              updater.ListenAddr,
		Handler:           u.handler(token),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// POST /install answers after its checks (manifest 20 s, docker inspect).
		WriteTimeout: checkTimeout + 10*time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("updater: listening", "addr", updater.ListenAddr, "version", version, "deploy_dir", cfg.deployDir, "container", cfg.container)
	select {
	case err := <-errc:
		log.Error("updater: listen", "err", err)
		return 1
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(sctx)
	// A running install stops at its next step and leaves its phase in state.json; the next
	// start takes over (recover). compose.updater.yaml gives the container stop_grace_period 30s.
	if !u.drain(drainTimeout) {
		log.Warn("updater: install still running at exit; the next start takes over")
	}
	return 0
}

// settings is the environment of the updater.
type settings struct {
	deployDir   string
	uid, gid    int
	manifestURL string
	container   string
}

// sharedDir is the folder in deploy/ shared with the server.
const sharedDir = "updater"

// dir is <deploy>/updater, the folder shared with the server.
func (c settings) dir() string { return filepath.Join(c.deployDir, sharedDir) }

func loadConfig(getenv func(string) string) (settings, error) {
	c := settings{
		manifestURL: strings.TrimSpace(getenv("SLOUCHCAT_UPDATE_MANIFEST_URL")),
		container:   strings.TrimSpace(getenv("SLOUCHCAT_CONTAINER")),
	}
	if c.manifestURL == "" {
		c.manifestURL = config.DefaultUpdateManifestURL // the manifest the server's update check reads
	}
	if c.container == "" {
		c.container = service
	}
	if u, err := url.Parse(c.manifestURL); err != nil || u.Scheme != "https" || u.Host == "" {
		return c, fmt.Errorf("SLOUCHCAT_UPDATE_MANIFEST_URL must be an https URL, got %q", c.manifestURL)
	}
	dir := strings.TrimSpace(getenv("SLOUCHCAT_DEPLOY_DIR"))
	if dir == "" || !filepath.IsAbs(dir) {
		return c, fmt.Errorf("SLOUCHCAT_DEPLOY_DIR must be an absolute path, got %q", dir)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		return c, fmt.Errorf("SLOUCHCAT_DEPLOY_DIR: %w", err)
	}
	c.deployDir = resolved
	for _, id := range []struct {
		key string
		dst *int
	}{{"PUID", &c.uid}, {"PGID", &c.gid}} {
		v := strings.TrimSpace(getenv(id.key))
		if v == "" {
			*id.dst = 1000
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return c, fmt.Errorf("%s must be a number, got %q", id.key, v)
		}
		*id.dst = n
	}
	return c, nil
}

// newUpdater prepares deploy/updater/ (folder, token, state) and returns the updater and the
// token.
func newUpdater(ctx context.Context, cfg settings, docker Docker, log *slog.Logger) (*Updater, []byte, error) {
	dir := cfg.dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, err
	}
	if err := ownDir(dir, cfg.uid, cfg.gid); err != nil {
		return nil, nil, err
	}
	token, err := ensureToken(filepath.Join(dir, updater.TokenFile), cfg.uid, cfg.gid, log)
	if err != nil {
		return nil, nil, err
	}
	u := &Updater{
		cfg: cfg, docker: docker, log: log, now: time.Now, ctx: ctx,
		client: &http.Client{Timeout: manifestTimeout, CheckRedirect: httpsRedirects},
		every:  pollInterval, wait: waitTimeout,
	}
	u.recover()
	return u, token, nil
}

// maxRedirects is as many redirects as the manifest request follows (the net/http default).
const maxRedirects = 10

// httpsRedirects follows a redirect of the manifest request only to https: the manifest decides
// which image runs with the Docker socket.
func httpsRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("redirect to %s refused: not https", req.URL.Redacted())
	}
	return nil
}

// ownDir gives the folder mode 0700 and the owner uid:gid through its open descriptor, without
// following a symbolic link: no chmod or chown by path in the updater (ADR 0024).
func ownDir(dir string, uid, gid int) error {
	d, err := os.OpenFile(dir, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0) //nolint:gosec // G304: <deploy>/updater, from the operator-configured deploy folder
	if err != nil {
		return err
	}
	err = d.Chmod(0o700) // a folder needs x; only PUID:PGID may enter it
	if err == nil {
		err = d.Chown(uid, gid)
	}
	return errors.Join(err, d.Close())
}

// ensureToken reads the token file or creates it: 32 random bytes as hex, 0600, owned by
// PUID:PGID so that the server can read it. The token is never logged (CLAUDE.md rule 10).
//
// The server can write the folder. The file is therefore opened without following a symbolic
// link and without blocking, mode and owner go through the open descriptor, and anything but a
// regular file with one link (a symbolic link, a hard link, a FIFO) is removed, not its target,
// and replaced by a new token.
func ensureToken(path string, uid, gid int, log *slog.Logger) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // G304: fixed file name inside the deploy folder
	switch {
	case err == nil:
		token, regular, err := readToken(f, uid, gid)
		if regular || err != nil {
			return token, err
		}
		log.Warn("updater: token is not a regular file, replacing it", "path", path)
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	case errors.Is(err, syscall.ELOOP):
		log.Warn("updater: token is a symbolic link, replacing it", "path", path)
		if err := os.Remove(path); err != nil { // removes the link, not its target
			return nil, err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := []byte(hex.EncodeToString(raw))
	if err := writeAtomic(path, token, 0o600, uid, gid); err != nil {
		return nil, fmt.Errorf("write token file: %w", err)
	}
	return token, nil
}

// readToken reads the open token file and gives it mode 0600 and the owner uid:gid through the
// descriptor. regular is false (and nothing changed) when f is not a regular file with one link.
func readToken(f *os.File, uid, gid int) (token []byte, regular bool, err error) {
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !info.Mode().IsRegular() || !ok || st.Nlink != 1 {
		return nil, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return nil, true, err
	}
	token = []byte(strings.TrimSpace(string(data)))
	if len(token) < 32 {
		return nil, true, errors.New("token file is too short; delete it to get a new one")
	}
	if err := f.Chmod(0o600); err != nil {
		return nil, true, err
	}
	return token, true, f.Chown(uid, gid)
}

// recover loads state.json. An install that was running when the updater stopped did not
// finish, and the next start ends it:
//
//   - checking, backing_up, pulling, verifying: the old container still runs. The env file is put
//     back from the backup the install wrote (.env.bak-vor-<to>, -2, …) when it still names the new version and the backup exists;
//     the install is failed.
//   - restarting, waiting: compose up may have run. The install goes on in the phase waiting, and
//     Resume waits for the new version in the background as run does (u.wait from now, with the
//     early end of Container.GaveUp); /status answers meanwhile and /install is busy. It ends
//     done when the new version runs healthy and server.json names it, otherwise failed, with
//     the migration of server.json when there was one. Nothing is taken back at start-up: the
//     new server may be migrating, and a recreate in the middle of a migration is worse than a
//     report. The administrator decides (manual update steps or the backup).
//   - a running phase with RollingBack: the install was being taken back (no migration). Resume
//     finishes the rollback in the background (rollbackAfterRestart): rolled_back when the old
//     version runs healthy again, otherwise failed.
func (u *Updater) recover() {
	var s updater.State
	err := updater.ReadJSON(u.statePath(), &s)
	if errors.Is(err, fs.ErrNotExist) {
		u.state = updater.State{Phase: updater.PhaseIdle}
		return
	}
	if err != nil {
		u.log.Warn("updater: unreadable state, starting idle", "err", err)
		u.state = updater.State{Phase: updater.PhaseIdle}
		return
	}
	u.state = s
	if !s.Phase.Running() {
		return
	}
	interrupted := s.Phase
	if s.RollingBack {
		u.resumeFrom = interrupted
		u.log.Warn("updater: interrupted rollback, taking the install back again", "interrupted", interrupted, "to", s.To)
		return
	}
	if interrupted == updater.PhaseRestarting || interrupted == updater.PhaseWaiting {
		s.Phase = updater.PhaseWaiting
		u.resumeFrom = interrupted
		u.log.Warn("updater: interrupted install, waiting for the new version again", "interrupted", interrupted, "to", s.To)
		u.set(s)
		return
	}
	ctx, cancel := context.WithTimeout(u.ctx, checkTimeout)
	defer cancel()
	s.Phase, s.FinishedAt = updater.PhaseFailed, u.now()
	if interrupted == updater.PhaseChecking {
		s.Message = "the updater stopped while checking the release; nothing was changed"
	} else {
		msg, _ := u.recoverEnv(ctx, s.To, s.EnvBackup)
		s.Message = fmt.Sprintf("the updater stopped during the phase %s before the restart; ", interrupted) + msg
	}
	u.log.Warn("updater: interrupted install ended", "interrupted", interrupted, "phase", s.Phase, "to", s.To)
	u.set(s)
}

// Resume starts the background wait for an install that recover found interrupted after its
// restart; without one it does nothing. main calls it once after newUpdater.
func (u *Updater) Resume() {
	if u.resumeFrom == "" {
		return
	}
	s, interrupted := u.State(), u.resumeFrom
	u.resumeFrom = ""
	if s.RollingBack {
		u.wg.Go(func() { u.rollbackAfterRestart(s) })
		return
	}
	u.wg.Go(func() { u.resumeWait(s, interrupted) })
}

// resumeWait waits for version s.To as run does after compose up and ends the install, without a
// rollback; s is in the phase waiting. A shutdown leaves the phase for the next start.
func (u *Updater) resumeWait(s updater.State, interrupted updater.Phase) {
	rec, end := u.waitFor(u.ctx, s.To)
	if u.ctx.Err() != nil {
		u.log.Warn("updater: stopped while waiting again; the next start takes over", "to", s.To)
		return
	}
	s.FinishedAt = u.now()
	switch end {
	case waitOK:
		s.Phase, s.Message = updater.PhaseDone, ""
		s.Migrated, s.Backup = rec.Migrated(), rec.Backup
	default:
		s.Phase = updater.PhaseFailed
		cause := u.failCause(end)
		if rec, migrated := u.migrated(s.To); migrated && s.To != "" {
			s.Migrated, s.Backup, s.Code = true, rec.Backup, updater.CodeMigratedNoRollback
			s.Message = fmt.Sprintf("the updater stopped during the phase %s; version %s migrated the database and %s; back only with the backup %s",
				interrupted, s.To, cause, rec.Backup)
		} else {
			s.Message = fmt.Sprintf("the updater stopped during the phase %s and version %s %s; nothing was taken back: check the server and SLOUCHCAT_VERSION in the env file",
				interrupted, s.To, cause)
		}
	}
	u.set(s)
	u.log.Info("updater: interrupted install finished", "phase", s.Phase, "code", s.Code, "message", s.Message, "from", s.From, "to", s.To)
}

// recoverEnv puts the env file back from the backup taken before installing to, when the file
// still names to and the backup exists, and returns what it did for the message. backup is the
// base name the install stored in state.json ("" for none: the newest match is used, see
// findBackup). ok reports that the env file no longer names to (it was put back, or it named
// another version already).
func (u *Updater) recoverEnv(ctx context.Context, to, backup string) (msg string, ok bool) {
	const check = "check SLOUCHCAT_VERSION in the env file"
	if !updater.ValidRelease(to) {
		return check, false
	}
	c, err := u.docker.Container(ctx, u.cfg.container)
	if err != nil {
		return check, false
	}
	p, r := u.project(c)
	if r != nil {
		return check, false
	}
	path, err := envTarget(p.EnvFiles)
	if err != nil {
		return check, false
	}
	env, err := readEnvFile(path, u.cfg.uid, u.cfg.gid)
	if err != nil {
		return check, false
	}
	if v, _ := envVersion(env.data); v != to {
		return "the env file does not name version " + to + ", nothing to put back", true
	}
	path, err = env.findBackup(to, backup)
	if err != nil {
		u.log.Error("updater: env backup", "err", err)
		return "the stored backup name is not valid; " + check, false
	}
	data, _, err := readRegular(path)
	if err != nil {
		return "no backup " + filepath.Base(path) + "; " + check, false
	}
	if err := env.write(data); err != nil {
		u.log.Error("updater: restore env file", "err", err)
		return "could not put the env file back: " + check, false
	}
	return "the env file was put back from " + filepath.Base(path), true
}

// handler serves GET /status and POST /install, both only with the token.
func (u *Updater) handler(token []byte) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+updater.PathStatus, func(w http.ResponseWriter, _ *http.Request) {
		writeState(w, http.StatusOK, u.State())
	})
	mux.HandleFunc("POST "+updater.PathInstall, func(w http.ResponseWriter, _ *http.Request) {
		s, status := u.Start()
		writeState(w, status, s)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), token) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func writeState(w http.ResponseWriter, status int, s updater.State) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(s)
}
