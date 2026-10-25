// Command slouchcat is the Slouchcat media server.
//
// Usage:
//
//	slouchcat [serve] [flags]     run the server (default)
//	slouchcat healthcheck [flags] exit 0 when the local server answers GET /api/v1/healthz with 200
//	slouchcat dump [flags]        write a replayable SQL snapshot of the database to stdout
//	slouchcat watchbench --root DIR [--passes N] [--inotify]  measure change detection of a root
//	slouchcat hlsbench --file F [--streams N] [--mode transcode|remux]  measure concurrent HLS sessions
//	slouchcat scanbench --root DIR [--kind movies|series|both] [--workers N] [--json]  measure scans
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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api"
	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/browse"
	"github.com/slouchcat/slouchcat/server/internal/config"
	"github.com/slouchcat/slouchcat/server/internal/discover"
	"github.com/slouchcat/slouchcat/server/internal/integrations"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
	"github.com/slouchcat/slouchcat/server/internal/offline"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
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
	if cmd == "scanbench" {
		return scanbench(args) // own flags, temporary database
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
	// The error ring keeps the newest error records for the admin dashboard (M8, A-3).
	log, errRing, err := logging.NewWithErrors(os.Stdout, cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slouchcat:", err)
		return 2
	}
	log.Info("starting slouchcat", "version", version, "config", cfg)
	if w := cfg.TrustedProxyWarning(); w != "" {
		log.Warn(w)
	}
	if w := cfg.BrowseRootWarning(); w != "" {
		log.Warn(w)
	}
	var restart atomic.Bool
	err = serveUntilSignal(cfg, log, errRing, started, &restart)
	again, code := afterStop(err, restart.Load())
	switch {
	case again && err != nil:
		log.Error("server stopped with error; restarting anyway", "err", err)
	case err != nil:
		log.Error("server stopped with error", "err", err)
	}
	if again {
		reexec(log)
	}
	if code == 0 {
		log.Info("server stopped")
	}
	return code
}

// afterStop decides how serve ends once the server stopped with err: it runs the binary again
// whenever a restart was requested, also after an error on the way down (the staged restore of
// the restart is applied only by the next start), and otherwise exits 1 on an error. A failed
// exec ends with 0, so the container's restart policy starts the server again.
func afterStop(err error, restart bool) (again bool, code int) {
	switch {
	case restart:
		return true, 0
	case err != nil:
		return false, 1
	}
	return false, 0
}

// reexec replaces the stopped server with a fresh start of the same binary (a staged restore,
// A-4): same PID, so Docker and the end-to-end runner see no exit. When the exec fails the process
// exits with 0 and the container's restart policy starts it again.
func reexec(log *slog.Logger) {
	exe, err := os.Executable()
	if err == nil {
		log.Info("restarting")
		err = syscall.Exec(exe, os.Args, os.Environ()) //nolint:gosec // G204: the server's own binary with its own arguments
	}
	log.Error("restart failed; exiting for the container to start again", "err", err)
}

// serveUntilSignal runs the server until SIGINT or SIGTERM, or until the administration asks for a
// restart after a staged restore: then restart is set and the server shuts down the same way.
func serveUntilSignal(cfg config.Config, log *slog.Logger, errRing *logging.ErrorRing, started time.Time, restart *atomic.Bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	requestRestart := func() {
		log.Info("restart requested")
		restart.Store(true)
		stop()
	}

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
	keys := &lazyKeys{cfg: cfg}
	// Playback exists before the metadata jobs run: a match that changes a rating ends the open
	// direct-play streams of titles that became invisible (M3 review, finding 20).
	playbackSvc, mgr, caps, closeTranscoder := newPlayback(ctx, cfg, db, runner, keyring, log)
	// In-app inbox (M7, E-8): new episodes of favourite series and podcasts, checked against the
	// profile's visibility before an entry is stored. Channels (M8, A-6): deliveries through the
	// queue; the Telegram base URL and http push endpoints (mocks) only in development mode.
	if !cfg.Dev && cfg.TelegramBaseURL != "" {
		log.Warn("SLOUCHCAT_TELEGRAM_BASE_URL is ignored without --dev")
	}
	notifySvc := notify.New(db, visibility.New(db, log), log, notify.WithQueue(queue), notify.WithDevPush(cfg.Dev),
		notify.WithSenders(notify.Senders{Telegram: &telegram.Client{BaseURL: cfg.TelegramAPIBase()}}),
		notify.WithAttemptLimit(authSvc.AllowAttempt))
	authSvc.WithAccountCreated(notifySvc.EnsureDefaults).WithInvitationAccepted(notifySvc.InvitationAccepted)
	// The Trakt base URL points the lists at a mock; it is honoured in development mode only.
	traktBase := ""
	if cfg.Dev {
		traktBase = cfg.TraktBaseURL
	} else if cfg.TraktBaseURL != "" {
		log.Warn("SLOUCHCAT_TRAKT_BASE_URL is ignored without --dev")
	}
	metaSvc := metadata.New(db, queue, keys, cfg.CacheDir, runner, log, metadata.WithTrailerAI(cfg.TrailerAI),
		metadata.WithVisibilityRecheck(playbackSvc.RecheckVisibility), metadata.WithFeedUserAgent("Slouchcat/"+version),
		metadata.WithNewEpisodes(notifySvc.NewEpisodes),
		metadata.WithTrakt(cfg.TraktClientID, metadata.WithTraktBaseURL(traktBase)))
	libSvc := library.New(db, queue, runner, log,
		library.WithImageCheck(metaSvc.QueueIncompleteImages),
		library.WithMetadataCheck(metaSvc.QueuePendingMetadata),
		library.WithNewEpisodes(notifySvc.NewEpisodes), library.WithScanAborted(notifySvc.ScanAborted),
		library.WithRootRemoved(notifySvc.RootRemoved),
		library.WithBrowseRoots(cfg.BrowseRoots))
	adminSvc := admin.New(db, authSvc, libSvc, keyring, metaSvc, os.Getenv, log)
	adminSvc.WithQueue(queue).WithLibraryMaintenance(libSvc).WithMetadata(metaSvc).WithStreams(playbackSvc).WithPodcasts(metaSvc).
		WithTrakt(metaSvc).WithBackups(cfg.ConfigDir, version).WithRestart(requestRestart).
		// The browsable roots are classified once, here: the wizard's root list answers before
		// any account exists and must make no system call per request (idea 02, E5).
		WithBrowse(cfg.BrowseRoots, browse.ClassifyRoots(cfg.BrowseRoots, log),
			browse.New(log, browse.WithConfinedTo(browse.ResolveRoots(cfg.BrowseRoots, log))))
	wireAdmin(adminSvc, cfg, queue, errRing, libSvc, caps, started)
	keys.admin = adminSvc
	// The channels (M8): admin stores the SMTP, Telegram and VAPID settings that notify reads, and
	// notify checks tokens, sends test mails, rotates the push keys and reports the channel state
	// for the dashboard.
	adminSvc.WithAnnouncements(notifySvc).WithTelegram(notifySvc).WithEmailTester(notifySvc).WithPushRotator(notifySvc).WithChannels(notifySvc)
	notifySvc.SetConfig(adminSvc)
	// Incoming Radarr/Sonarr webhooks (M9, I-8): the address shown once carries the public URL of
	// the administration's settings, so it can be pasted into "Connect → Webhook" as it is.
	integrationSvc := integrations.New(db, libSvc, log, integrations.WithBaseURL(func(ctx context.Context) string {
		u, err := adminSvc.PublicURL(ctx)
		if err != nil {
			log.WarnContext(ctx, "public url for the webhook address", "err", err)
			return ""
		}
		return u + api.BasePath
	}))
	adminSvc.WithIntegrations(integrationSvc)
	// Offline downloads (M5): prepared in <cache>/downloads within the download budget, encoded with
	// the transcode manager's admission (live playback preempts them, plan 3.5).
	offlineSvc := offline.New(db, visibility.New(db, log), queue, runner, log,
		offline.WithDir(cfg.DownloadsDir()), offline.WithBudget(int64(cfg.DownloadBudgetGB)<<30),
		offline.WithTokenKey(keyring.Derive("download-token")), offline.WithManager(mgr, caps),
		offline.WithPlayback(playbackSvc), offline.WithPins(authSvc))

	var workers sync.WaitGroup
	for i := range cfg.Workers {
		worker := jobs.NewWorker(queue, fmt.Sprintf("worker-%d", i+1), log)
		libSvc.RegisterJobs(worker)
		metaSvc.RegisterJobs(worker)
		adminSvc.RegisterJobs(worker)
		worker.OnFinalFailure(notifySvc.JobFailed)
		workers.Go(func() { worker.Run(ctx) })
	}
	// A worker of its own for download preparations: workers claim only the job types they
	// registered, so long encodes never hold up scans and probes.
	downloadWorker := jobs.NewWorker(queue, "download-1", log)
	offlineSvc.RegisterJobs(downloadWorker)
	downloadWorker.OnFinalFailure(notifySvc.JobFailed)
	workers.Go(func() { downloadWorker.Run(ctx) })
	// Notification deliveries (M8) on a worker of their own: a slow mail server never holds up
	// scans. Run polls Telegram while link codes are open and prunes old deliveries.
	notifyWorker := jobs.NewWorker(queue, "notify-1", log)
	notifySvc.RegisterJobs(notifyWorker)
	workers.Go(func() { notifyWorker.Run(ctx) })
	workers.Go(func() { notifySvc.Run(ctx) })
	workers.Go(func() {
		if err := libSvc.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("library watchers stopped", "err", err)
		}
	})
	// Podcast feeds (M6, H-2): every subscription is fetched again after 60 minutes.
	workers.Go(func() { metaSvc.RunPodcasts(ctx) })
	// Trakt lists (M7, E-7): once a day when a client id is configured.
	workers.Go(func() { metaSvc.RunTrakt(ctx) })
	// Update check (M8, A-5): 60 s after the start, then once a day while it is switched on.
	workers.Go(func() { adminSvc.RunUpdateCheck(ctx) })
	defer func() {
		stop() // also on error paths, so the workers return before the database closes
		workers.Wait()
	}()

	sweepCtx, stopSweep := context.WithCancel(ctx)
	var sweeper sync.WaitGroup
	sweeper.Go(func() { playbackSvc.Run(sweepCtx) })
	sweeper.Go(func() { offlineSvc.Run(sweepCtx) })
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
			Offline:  offlineSvc,
			Notify:   notifySvc,

			Integrations: integrationSvc,
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
	return shutdown(srv, shutdownTimeout, restart.Load(), log)
}

// shutdown stops srv and waits up to timeout for the running requests. On the way to a restart a
// request still running then (a direct-play stream or an offline download keeps its connection for
// minutes) is cut off: its client resumes against the restarted server, so this is a normal
// restart and no error.
func shutdown(srv *http.Server, timeout time.Duration, restarting bool, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := srv.Shutdown(ctx)
	if err == nil || !restarting || !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	log.Info("closing the remaining connections for the restart", "waited_ms", timeout.Milliseconds())
	_ = srv.Close() // the listeners closed with Shutdown already; this ends the open connections
	return nil
}

// wireAdmin hands the administration of M8 its collaborators: the job queue, the error ring of the
// log, the start state (version, transcode capabilities, library watchers, directories), the
// notification settings of the environment and the release manifest of the update check. The
// collaborators of package notify (bot token check, test mail, VAPID rotation, channel state:
// WithTelegram, WithEmailTester, WithPushRotator, WithChannels) are wired where notify builds them.
func wireAdmin(a *admin.Service, cfg config.Config, queue *jobs.Queue, errRing *logging.ErrorRing, lib *library.Service,
	caps transcode.Capabilities, started time.Time,
) {
	a.WithJobs(queue).WithErrors(errRing).WithUpdateCheck(cfg.ManifestURL(), nil).
		WithSystem(admin.SystemSource{
			Version: version, StartedAt: started, ConfigDir: cfg.ConfigDir, CacheDir: cfg.CacheDir, Caps: caps,
			WatchedDirs: lib.WatchedDirs,
		}).
		WithNotifyEnv(admin.NotifyEnv{
			SMTP: model.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword,
				From: cfg.SMTPFrom, TLS: cfg.SMTPTLS},
			TelegramBotToken: cfg.TelegramBotToken, PublicURL: cfg.PublicURL,
		})
}

// lazyKeys hands the admin service to metadata as its key source after both exist.
type lazyKeys struct {
	admin *admin.Service
	cfg   config.Config
}

func (k *lazyKeys) TMDBKey(ctx context.Context) (string, error) {
	return k.admin.TMDBKey(ctx)
}

// TVDBKey returns the TVDB key and PIN from one source: both from the environment (config) when
// the key is set there, else both from the stored settings (admin.TVDBKey applies the same rule).
func (k *lazyKeys) TVDBKey(ctx context.Context) (key, pin string, err error) {
	if k.cfg.TVDBAPIKey != "" {
		return k.cfg.TVDBAPIKey, k.cfg.TVDBPIN, nil
	}
	return k.admin.TVDBKey(ctx)
}

// YouTubeKey returns the YouTube Data API key: the environment (config) wins, else the stored setting.
func (k *lazyKeys) YouTubeKey(ctx context.Context) (string, error) {
	if k.cfg.YouTubeAPIKey != "" {
		return k.cfg.YouTubeAPIKey, nil
	}
	return k.admin.YouTubeKey(ctx)
}

// TraktClientID returns the Trakt client id: the environment (config) wins, else the stored setting.
func (k *lazyKeys) TraktClientID(ctx context.Context) (string, error) {
	if k.cfg.TraktClientID != "" {
		return k.cfg.TraktClientID, nil
	}
	return k.admin.TraktClientID(ctx)
}

// newPlayback detects the transcode backend (SLOUCHCAT_HW), starts the transcode manager on
// <cache>/transcode and returns the playback service with the stream-token key of the key file
// (plan 5.4) and the subtitle cache below <cache>/subtitles. Without a usable ffmpeg the server
// still runs: titles then play directly only. Without a subtitle converter
// (playback.WithSubtitleConverter) text subtitles answer 404. It also returns the manager (nil
// without a backend) and the capabilities of the backend, which the download encodes share. The
// returned function stops every transcode on shutdown.
func newPlayback(ctx context.Context, cfg config.Config, db *store.DB, runner transcode.Runner, keyring *secrets.Keyring, log *slog.Logger) (*playback.Service, *transcode.Manager, transcode.Capabilities, func()) {
	opts := []playback.Option{
		playback.WithTokenKey(keyring.Derive("stream-token")), playback.WithCacheDir(cfg.CacheDir),
		// WebVTT conversion of text subtitles (W-4) runs through ffmpeg even when no transcode
		// backend is usable: the converter only needs the binary.
		playback.WithSubtitleConverter(runner.Subtitles, runner.SubtitlesFile),
	}
	backend, caps, err := transcode.DetectTonemap(ctx, runner, cfg.HW, cfg.HDRTonemap, log)
	if err != nil {
		log.Error("transcode backend detection failed; HLS disabled, titles play directly only", "hw", cfg.HW, "err", err)
		return playback.New(db, log, opts...), nil, transcode.Capabilities{Backend: transcode.BackendSoftware}, func() {}
	}
	mgr, err := transcode.NewManager(transcode.ManagerOptions{
		Runner: runner, WorkDir: cfg.TranscodeDir(), Backend: backend, Caps: caps,
		Budget: cfg.TranscodeBudget, Log: log,
	})
	if err != nil {
		log.Error("transcode manager failed; HLS disabled, titles play directly only", "err", err)
		return playback.New(db, log, opts...), nil, caps, func() {}
	}
	stats := mgr.Stats()
	log.Info("transcode ready", "backend", string(backend), "budget", stats.Budget, "hw_tonemap", caps.HWTonemap,
		"tonemapper", caps.Tonemapper, "ffmpeg", caps.Version)
	opts = append(opts, playback.WithTranscoder(playback.NewManagerTranscoder(mgr, backend)), playback.WithKeyframeIndex(runner.Keyframes))
	return playback.New(db, log, opts...), mgr, caps, mgr.Close
}
