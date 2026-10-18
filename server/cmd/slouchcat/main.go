// Command slouchcat is the Slouchcat media server.
//
// Usage:
//
//	slouchcat [serve] [flags]     run the server (default)
//	slouchcat healthcheck [flags] exit 0 when the local server answers GET /api/v1/healthz with 200
//
// The healthcheck subcommand serves the container HEALTHCHECK, so the image needs no curl. Both
// subcommands read the same flags and SLOUCHCAT_* environment variables.
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

	"github.com/slouchcat/slouchcat/server/internal/api"
	"github.com/slouchcat/slouchcat/server/internal/config"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/store"
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
	default:
		fmt.Fprintf(os.Stderr, "slouchcat: unknown command %q (want serve or healthcheck)\n", cmd)
		return 2
	}
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

	var workers sync.WaitGroup
	worker := jobs.NewWorker(jobs.NewQueue(db.Write, nil), "worker-1", log)
	workers.Go(func() { worker.Run(ctx) })
	defer func() {
		stop() // also on error paths, so the worker returns before the database closes
		workers.Wait()
	}()

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler: api.NewHandler(api.Options{
			Version: version, Started: started, Logger: log, Web: webassets.Handler(), Dev: cfg.Dev,
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
