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
	"github.com/slouchcat/slouchcat/server/internal/radio"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/updater"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/webassets"
)

// The radio service implements the API's webradio (package radio must not import api).
var _ api.RadioService = (*radio.Service)(nil)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	healthcheckTimeout = 2 * time.Second
	shutdownTimeout    = 10 * time.Second
	// activityDrainTimeout bounds the wait for the playback starts still being written to the
	// activity log on shutdown (each write is bounded by the same 5 s in admin).
	activityDrainTimeout = 5 * time.Second
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
	// The start record for the updater companion (ADR 0024): server.json in its shared folder
	// before and after the migrations; without the folder nothing is written. The pre-migration
	// backup an earlier start of this version recorded is never pruned.
	rec := updater.NewRecorder(cfg.UpdaterDir(), version, log)
	db, err := store.Open(ctx, store.Options{ConfigDir: cfg.ConfigDir, Version: version, Logger: log,
		Schema: store.SchemaHooks{Before: rec.Before, After: rec.After, KeepBackup: rec.KeptBackup()}})
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
	// Registered after the database's Close, so it runs before it: the start events still being
	// written to the activity log finish first (after the HTTP server and the workers stopped).
	defer func() {
		drain, cancel := context.WithTimeout(context.Background(), activityDrainTimeout)
		defer cancel()
		playbackSvc.WaitActivity(drain)
	}()
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
	// The e2e suite's shortened spotlight limit (wireAdmin); without --dev the minute holds.
	if !cfg.Dev && cfg.TestSpotlightRefreshGap != "" {
		log.Warn("SLOUCHCAT_TEST_SPOTLIGHT_REFRESH_GAP is ignored without --dev")
	}
	if !cfg.Dev && cfg.TestUpdaterDir != "" {
		log.Warn("SLOUCHCAT_TEST_UPDATER_DIR is ignored without --dev")
	}
	// The match hands a matched title to the library, which is created after the metadata service
	// because it takes the metadata checks; the closure reads libSvc when a match runs.
	var libSvc *library.Service
	relink := func(ctx context.Context, itemID string) (bool, error) {
		return libSvc.QueueRelink(ctx, itemID)
	}
	metaSvc := metadata.New(db, queue, keys, cfg.CacheDir, runner, log, metadata.WithTrailerAI(cfg.TrailerAI),
		metadata.WithVisibilityRecheck(playbackSvc.RecheckVisibility), metadata.WithFeedUserAgent("Slouchcat/"+version),
		metadata.WithNewEpisodes(notifySvc.NewEpisodes), metadata.WithAniList(cfg.AniList), metadata.WithRelink(relink),
		metadata.WithTrakt(cfg.TraktClientID, metadata.WithTraktBaseURL(traktBase)))
	libSvc = library.New(db, queue, runner, log,
		library.WithImageCheck(metaSvc.QueueIncompleteImages),
		library.WithMetadataCheck(metaSvc.QueuePendingMetadata),
		library.WithNewEpisodes(notifySvc.NewEpisodes), library.WithScanAborted(notifySvc.ScanAborted),
		library.WithRootRemoved(notifySvc.RootRemoved), library.WithRootContentChanged(notifySvc.RootContentChanged),
		library.WithBrowseRoots(cfg.BrowseRoots), library.WithVisibilityRecheck(playbackSvc.RecheckVisibility),
		library.WithCacheDir(cfg.CacheDir))
	authSvc.WithMetadataLanguages(metaSvc.AvailableMetadataLanguages)
	adminSvc := admin.New(db, authSvc, libSvc, keyring, metaSvc, os.Getenv, log)
	// The zone of the watch-time grid, resolved once by name (concept 12).
	zone, zoneName := admin.ResolveZone(os.Getenv)
	adminSvc.WithLocation(zone)
	log.Info("time zone of the statistics", "zone", zoneName)
	adminSvc.WithAniList(cfg.AniList).WithQueue(queue).WithLibraryMaintenance(libSvc).WithMetadata(metaSvc).WithStreams(playbackSvc).WithPodcasts(metaSvc).
		WithTrakt(metaSvc).WithBackups(cfg.ConfigDir, version).WithRestart(requestRestart).
		// The browsable roots are classified once, here: the wizard's root list answers before
		// any account exists and must make no system call per request (idea 02, E5).
		WithBrowse(cfg.BrowseRoots, browse.ClassifyRoots(cfg.BrowseRoots, log),
			browse.New(log, browse.WithConfinedTo(browse.ResolveRoots(cfg.BrowseRoots, log))))
	wireAdmin(adminSvc, cfg, queue, errRing, libSvc, caps, started)
	keys.admin = adminSvc
	// The activity log of the overview (concept 11, ADR 0019) lives in admin; playback and library
	// exist before it, so their hooks are set here, before any request or worker runs. jobs.prune
	// cleans it up. A restore swapped in at this start is recorded in the restored database.
	playbackSvc.WithActivity(adminSvc.RecordActivity)
	libSvc.WithActivity(adminSvc.RecordActivity)
	queue.AddPruneTask(adminSvc.PruneActivity)
	recordRestored(ctx, adminSvc.RecordActivity, db.Restored)
	// The channels (M8): admin stores the SMTP, Telegram and VAPID settings that notify reads, and
	// notify checks tokens, sends test mails, rotates the push keys and reports the channel state
	// for the dashboard.
	adminSvc.WithAnnouncements(notifySvc).WithTelegram(notifySvc).WithEmailTester(notifySvc).WithPushRotator(notifySvc).WithChannels(notifySvc)
	notifySvc.SetConfig(adminSvc)
	// The updater companion (concept 15 P-8, ADR 0024): optional; a failed or rolled back install
	// goes to the administration once as the event errors.
	adminSvc.WithUpdater(cfg.UpdaterURL, cfg.UpdaterDir(), notifySvc.UpdateInstallFailed)
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

	// The job types (concept 09, ADR 0017): every service registers what its job types are; the
	// registry hands the handlers to the workers of each pool, and the scheduler runs the types
	// with a schedule (full scan, sweep, podcast poll, Trakt lists, update check) from
	// job_schedules, which replaces the ticker loops the services ran until 1.4.0.
	registry := jobs.NewRegistry(queue, log)
	// The schedules and the job windows read the zone of the statistics: time.Local is the same
	// clock but only named "Local", and its name reaches the admin (windowTimeZone, timeZone).
	scheduler := jobs.NewScheduler(queue, registry, log, jobs.WithLocation(zone))
	// The webradio (concept 16, ADR 0025): the Radio Browser catalog, relays as live HLS in the
	// tmpfs of the compose file, listen time in the daily watch time of playback. Playback exists
	// first, so the radio hears a profile's end through a setter instead of a constructor cycle.
	radioSvc := radio.New(db, log, radio.WithRunner(runner), radio.WithCacheDir(cfg.CacheDir),
		radio.WithDir(cfg.RadioDir), radio.WithTokenKey(keyring.Derive("radio-token")),
		radio.WithWatchClock(playbackSvc), radio.WithUserAgent("Slouchcat/"+version),
		radio.WithQueue(queue), radio.WithScheduler(scheduler))
	playbackSvc.OnProfileEnd(radioSvc.EndProfile)
	libSvc.RegisterTypes(registry)
	metaSvc.RegisterTypes(registry)
	adminSvc.RegisterTypes(registry)
	offlineSvc.RegisterTypes(registry)
	notifySvc.RegisterTypes(registry)
	radioSvc.RegisterTypes(registry)
	if err := scheduler.Start(ctx); err != nil {
		return fmt.Errorf("start job scheduler: %w", err)
	}
	// The index of the catalog, and the first sync ten minutes ahead while it is empty (R-3).
	if err := radioSvc.Start(ctx); err != nil {
		return fmt.Errorf("start radio: %w", err)
	}
	poolCounts := poolWorkers(cfg.Workers)
	adminSvc.WithJobControl(registry, scheduler, poolCounts)

	var workers sync.WaitGroup
	startWorkers(ctx, &workers, queue, registry, poolCounts, log, notifySvc.JobFailed, adminSvc.RecordJobFailure)
	// Run polls Telegram while link codes are open and prunes old deliveries.
	workers.Go(func() { notifySvc.Run(ctx) })
	workers.Go(func() { scheduler.Run(ctx) })
	// The end of an install the companion reported (state.json), once, also while it still runs,
	// and of every install this server starts, until the server stops.
	workers.Go(func() { adminSvc.WatchUpdateInstall(ctx) })
	// The library watchers; the full scan at the start and every 6 h is the schedule scan.library.
	workers.Go(func() {
		if err := libSvc.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("library watchers stopped", "err", err)
		}
	})
	// Votes of the titles matched before migration 00026 (concept 08, 4.2): one chain of
	// metadata.vote_backfill after the update; idempotent, and every complete scan asks again.
	workers.Go(func() {
		if _, err := metaSvc.QueueVoteBackfill(ctx); err != nil && ctx.Err() == nil {
			log.Warn("vote backfill not queued", "err", err)
		}
		// Original languages of the titles matched before migration 00032 (concept 13, 11.7):
		// queued once at the first start after it, later only by hand.
		if _, err := metaSvc.QueueOriginalLanguageBackfillOnce(ctx); err != nil && ctx.Err() == nil {
			log.Warn("original language backfill not queued", "err", err)
		}
	})
	defer func() {
		stop() // also on error paths, so the workers return before the database closes
		workers.Wait()
	}()

	sweepCtx, stopSweep := context.WithCancel(ctx)
	var sweeper sync.WaitGroup
	sweeper.Go(func() { playbackSvc.Run(sweepCtx) })
	sweeper.Go(func() { offlineSvc.Run(sweepCtx) })
	// Listen sessions of the last run end, the 30-s sweep runs, and every relay stops at the end.
	sweeper.Go(func() { radioSvc.Run(sweepCtx) })
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
			// -1 draws (concept 08, F-2); the row «Meine Sender» reads the radio (concept 16, 10.2).
			Catalog:  discover.New(db, cfg.CacheDir, log, discover.WithHeroStart(cfg.HeroStart), discover.WithRadio(radioSvc)),
			Playback: playbackSvc,
			Offline:  offlineSvc,
			Notify:   notifySvc,
			Radio:    radioSvc,

			Integrations: integrationSvc,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Info("listening", "addr", ln.Addr().String(), "startup_ms", time.Since(started).Milliseconds())
	recordStarted(ctx, adminSvc.RecordActivity, version)

	select {
	case err := <-serveErr:
		stop()
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	log.Info("shutting down")
	return shutdown(srv, shutdownTimeout, restart.Load(), log)
}

// poolWorkers returns the workers per pool: general workers from SLOUCHCAT_WORKERS, one each for
// downloads, notifications, the keyframe index and trickplay (concept 09 F-9, concept 14 F-2,
// concept 18 O-1; not adjustable).
func poolWorkers(general int) map[jobs.Pool]int {
	return map[jobs.Pool]int{jobs.PoolGeneral: general, jobs.PoolDownload: 1, jobs.PoolNotify: 1, jobs.PoolIndex: 1, jobs.PoolTrickplay: 1}
}

// startWorkers starts the workers of counts (poolWorkers) on wg, each claiming only the job types
// of its pool: worker-1 … for the general pool, download-1, notify-1, index-1 and trickplay-1 for
// the others.
// onFinalFailure hears of a job that failed its last attempt on every pool but notify.
func startWorkers(ctx context.Context, wg *sync.WaitGroup, queue *jobs.Queue, registry *jobs.Registry, counts map[jobs.Pool]int,
	log *slog.Logger, onFinalFailure ...func(context.Context, jobs.Job, error)) {
	// The general workers, then a worker of its own for download preparations (long encodes never
	// hold up scans and probes), for notification deliveries (M8: a slow mail server never holds
	// up scans) and for the full keyframe reads (concept 14, 6.3: a read of minutes never holds
	// up a general worker, and two never run at once) and for the preview tiles (concept 18).
	for _, pool := range []jobs.Pool{jobs.PoolGeneral, jobs.PoolDownload, jobs.PoolNotify, jobs.PoolIndex, jobs.PoolTrickplay} {
		prefix := string(pool)
		if pool == jobs.PoolGeneral {
			prefix = "worker"
		}
		for i := range counts[pool] {
			w := jobs.NewWorker(queue, fmt.Sprintf("%s-%d", prefix, i+1), log)
			registry.Attach(w, pool)
			if pool != jobs.PoolNotify {
				for _, fn := range onFinalFailure {
					w.OnFinalFailure(fn)
				}
			}
			wg.Go(func() { w.Run(ctx) })
		}
	}
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
		WithSpotlightRefreshGap(cfg.SpotlightRefreshGap()). // 0 outside --dev: the minute holds
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

// FanartKeys reads the Fanart.tv keys (environment first, then the encrypted settings).
func (k *lazyKeys) FanartKeys(ctx context.Context) (string, string, error) {
	return k.admin.FanartKeys(ctx)
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
	opts = append(opts, playback.WithTranscoder(playback.NewManagerTranscoder(mgr)))
	return playback.New(db, log, opts...), mgr, caps, mgr.Close
}

// recordRestored records backup_restored when store.Open swapped in a staged restore at this start
// (concept 11): a row written before the restart went with the replaced database.
func recordRestored(ctx context.Context, record func(context.Context, model.ActivityEvent), restored bool) {
	if restored {
		record(ctx, model.ActivityEvent{Kind: model.ActivityBackupRestored})
	}
}

// recordStarted records server_started with the version once the server listens (concept 11).
func recordStarted(ctx context.Context, record func(context.Context, model.ActivityEvent), version string) {
	record(ctx, model.ActivityEvent{Kind: model.ActivityServerStarted, Detail: model.ActivityDetail{Version: version}})
}
