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
//	SLOUCHCAT_UPDATE_MANIFEST_URL  release manifest (default config.DefaultUpdateManifestURL)
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
	"io/fs"
	"log/slog"
	"net/http"
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

const shutdownTimeout = 5 * time.Second

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

	docker := dockerCLI{output: func(args []string, out []byte) {
		log.Warn("updater: docker command failed", "args", strings.Join(args, " "), "output", tail(out, 4096))
	}}
	u, token, err := newUpdater(ctx, cfg, docker, log)
	if err != nil {
		log.Error("updater: start", "err", err)
		return 1
	}
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
	// A running install dies with the process; the next start marks it failed.
	return 0
}

// tail returns the last n bytes of out as a string.
func tail(out []byte, n int) string {
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return string(out)
}

// settings is the environment of the updater.
type settings struct {
	deployDir   string
	uid, gid    int
	manifestURL string
	container   string
}

// dir is <deploy>/updater, the folder shared with the server.
func (c settings) dir() string { return filepath.Join(c.deployDir, "updater") }

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
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: a folder needs x; only PUID:PGID may enter it
		return nil, nil, err
	}
	if err := os.Chown(dir, cfg.uid, cfg.gid); err != nil {
		return nil, nil, err
	}
	token, err := ensureToken(filepath.Join(dir, updater.TokenFile), cfg.uid, cfg.gid)
	if err != nil {
		return nil, nil, err
	}
	u := &Updater{
		cfg: cfg, docker: docker, log: log, now: time.Now, ctx: ctx,
		client: &http.Client{Timeout: manifestTimeout},
		every:  pollInterval, wait: waitTimeout,
	}
	u.recover()
	return u, token, nil
}

// ensureToken reads the token file or creates it: 32 random bytes as hex, 0600, owned by
// PUID:PGID so that the server can read it. The token is never logged (CLAUDE.md rule 10).
func ensureToken(path string, uid, gid int) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: fixed file name inside the deploy folder
	if err == nil {
		token := []byte(strings.TrimSpace(string(data)))
		if len(token) < 32 {
			return nil, errors.New("token file is too short; delete it to get a new one")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, err
		}
		return token, os.Chown(path, uid, gid)
	}
	if !errors.Is(err, fs.ErrNotExist) {
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

// recover loads state.json. An install that was running when the updater stopped did not
// finish: it becomes failed, with the migration of server.json when the new server got that far.
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
	s.Message = fmt.Sprintf("the updater stopped during the phase %s; check SLOUCHCAT_VERSION in the env file", s.Phase)
	s.Phase, s.FinishedAt = updater.PhaseFailed, u.now()
	if rec, ok := u.migrated(s.To); ok && s.To != "" {
		s.Migrated, s.Backup, s.Code = true, rec.Backup, updater.CodeMigratedNoRollback
	}
	u.log.Warn("updater: interrupted install marked failed", "to", s.To)
	u.set(s)
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
