// Command slouchcat is the Slouchcat media server.
//
// Usage:
//
//	slouchcat [serve] [flags]     run the server (default)
//	slouchcat healthcheck [flags] exit 0 when the local server answers GET /api/v1/healthz with 200
//	slouchcat dump [flags]        write a replayable SQL snapshot of the database to stdout
//	slouchcat watchbench --root DIR [--passes N] [--inotify]  measure change detection of a root
//	slouchcat hlsbench --file F [--streams N] [--mode transcode|remux]  measure concurrent HLS sessions
//
// The healthcheck subcommand serves the container HEALTHCHECK, so the image needs no curl. dump
// produces the frozen milestone states in testdata/db (plan 6.2 point 4). All subcommands read the
// same flags and SLOUCHCAT_* environment variables.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api"
	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/config"
	"github.com/slouchcat/slouchcat/server/internal/discover"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/webassets"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	healthcheckTimeout = 2 * time.Second
	shutdownTimeout    = 10 * time.Second
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	if cmd == "watchbench" {
		return watchbench(args) // own flags, no server configuration
	}
	if cmd == "hlsbench" {
		return hlsbench(args) // own flags, no server configuration
	}
	cfg, err := config.Load(args, os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "usage: slouchcat [serve|healthcheck] [flags]")
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "slouchcat:", err)
		return 2
	}
	switch cmd {
	case "serve":
		return serve(cfg)
	case "healthcheck":
		return healthcheck(cfg)
	case "dump":
		return dump(cfg)
	case "watchbench":
		return watchbench(args)
	default:
		fmt.Fprintf(os.Stderr, "slouchcat: unknown command %q (want serve, healthcheck or dump)\n", cmd)
		return 2
	}
}

// dump opens the database (running pending migrations like serve does) and writes the SQL
// snapshot to stdout.
func dump(cfg config.Config) int {
	log, err := logging.New(os.Stderr, cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slouchcat:", err)
		return 2
	}
	ctx := context.Background()
	db, err := store.Open(ctx, store.Options{ConfigDir: cfg.ConfigDir, Version: version, Logger: log})
	if err != nil {
		fmt.Fprintln(os.Stderr, "dump:", err)
		return 1
	}
	defer db.Close()
	if err := store.Dump(ctx, db.Read, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "dump:", err)
		return 1
	}
	return 0
}

// healthcheck probes the server on the loopback interface, or on the configured host when the
// server listens on one specific address.
func healthcheck(cfg config.Config) int {
	host, port, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()
	url := "http://" + net.JoinHostPort(host, port) + api.BasePath + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.Status)
		return 1
	}
	return 0
}

func serve(cfg config.Config) int {
	started := time.Now()
	log, err := logging.New(os.Stdout, cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slouchcat:", err)
		return 2
	}
	log.Info("starting slouchcat", "version", version, "config", cfg)
	if err := serveUntilSignal(cfg, log, started); err != nil {
		log.Error("server stopped with error", "err", err)
		return 1
	}
	log.Info("server stopped")
	return 0
}

func serveUntilSignal(cfg config.Config, log *slog.Logger, started time.Time) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for _, dir := range []string{cfg.ConfigDir, cfg.CacheDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
	}
	db, err := store.Open(ctx, store.Options{ConfigDir: cfg.ConfigDir, Version: version, Logger: log})
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("close database", "err", err)
		}
	}()

	keyring, err := secrets.Load(cfg.ConfigDir, log)
	if err != nil {
		return err
	}
	queue := jobs.NewQueue(db.Write, nil)
	runner := transcode.Runner{FFmpegPath: cfg.FFmpeg, FFprobePath: cfg.FFprobe}
	authSvc := auth.New(db, log)
	// admin stores and reveals the TMDB key, metadata checks and uses it: the key source is
	// resolved lazily so both can be constructed without a cycle.
	keys := &lazyKeys{}
	metaSvc := metadata.New(db, queue, keys, cfg.CacheDir, runner, log)
	libSvc := library.New(db, queue, runner, log, library.WithImageCheck(metaSvc.QueueIncompleteImages))
	adminSvc := admin.New(db, authSvc, libSvc, keyring, metaSvc, os.Getenv, log)
	keys.admin = adminSvc

	var workers sync.WaitGroup
	for i := range cfg.Workers {
		worker := jobs.NewWorker(queue, fmt.Sprintf("worker-%d", i+1), log)
		libSvc.RegisterJobs(worker)
		metaSvc.RegisterJobs(worker)
		workers.Go(func() { worker.Run(ctx) })
	}
	workers.Go(func() {
		if err := libSvc.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("library watchers stopped", "err", err)
		}
	})
	defer func() {
		stop() // also on error paths, so the workers return before the database closes
		workers.Wait()
	}()

	playbackSvc, closeTranscoder := newPlayback(ctx, cfg, db, runner, keyring, log)
	sweepCtx, stopSweep := context.WithCancel(ctx)
	var sweeper sync.WaitGroup
	sweeper.Go(func() { playbackSvc.Run(sweepCtx) })
	defer func() {
		stopSweep() // the sweep stops transcodes, so it ends before the manager closes
		sweeper.Wait()
		closeTranscoder()
	}()

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler: api.NewHandler(api.Options{
			Version: version, Started: started, Logger: log, Web: webassets.Handler(), Dev: cfg.Dev,
			Auth: authSvc, Profiles: authSvc, Access: authSvc, Admin: adminSvc, TrustedProxies: cfg.TrustedProxies,
			Catalog:  discover.New(db, cfg.CacheDir, log),
			Playback: playbackSvc,
			SpikeDir: cfg.SpikeDir,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Info("listening", "addr", ln.Addr().String(), "startup_ms", time.Since(started).Milliseconds())

	select {
	case err := <-serveErr:
		stop()
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// lazyKeys hands the admin service to metadata as its key source after both exist.
type lazyKeys struct {
	admin *admin.Service
}

func (k *lazyKeys) TMDBKey(ctx context.Context) (string, error) {
	return k.admin.TMDBKey(ctx)
}

// newPlayback detects the transcode backend (SLOUCHCAT_HW), starts the transcode manager on
// <cache>/transcode and returns the playback service with the stream-token key of the key file
// (plan 5.4). Without a usable ffmpeg the server still runs: titles then play directly only. The
// returned function stops every transcode on shutdown.
func newPlayback(ctx context.Context, cfg config.Config, db *store.DB, runner transcode.Runner, keyring *secrets.Keyring, log *slog.Logger) (*playback.Service, func()) {
	opts := []playback.Option{playback.WithTokenKey(keyring.Derive("stream-token"))}
	backend, caps, err := transcode.Detect(ctx, runner, cfg.HW, log)
	if err != nil {
		log.Error("transcode backend detection failed; HLS disabled, titles play directly only", "hw", cfg.HW, "err", err)
		return playback.New(db, log, opts...), func() {}
	}
	mgr, err := transcode.NewManager(transcode.ManagerOptions{
		Runner: runner, WorkDir: cfg.TranscodeDir(), Backend: backend, Caps: caps,
		Budget: cfg.TranscodeBudget, Log: log,
	})
	if err != nil {
		log.Error("transcode manager failed; HLS disabled, titles play directly only", "err", err)
		return playback.New(db, log, opts...), func() {}
	}
	stats := mgr.Stats()
	log.Info("transcode ready", "backend", string(backend), "budget", stats.Budget, "hw_tonemap", caps.HWTonemap, "ffmpeg", caps.Version)
	opts = append(opts, playback.WithTranscoder(playback.NewManagerTranscoder(mgr, backend)), playback.WithKeyframeIndex(runner.Keyframes))
	return playback.New(db, log, opts...), mgr.Close
}
