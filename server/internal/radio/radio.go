// Package radio is the webradio of concept 16 (ADR 0025, contract 1.17.0): the Radio Browser
// catalog with its daily sync, the merge of catalog entries into stations with a stable id, the
// derived facets, the favourites and preferences of a profile, the relays that play a station
// through the server as live HLS, the listen sessions with their token and listen time, and the
// logo chain. It never reads title tables (CLAUDE.md rule 2); its tables live in store/appq.
//
// The service is split into parts with one owner each, so the lanes of the build plan work in
// their own files of the one package: catalogState (catalog, sync, index, admin; catalog_state.go),
// relayState (relays, listen sessions, live HLS; relay_state.go), logoState (logo chain;
// logo_state.go) and coverState (cover chain; cover.go). Cross-part calls go through the hooks onListed and onStationChanged, which New
// sets (to no-ops until the wiring replaces them), so a caller never checks them for nil;
// onStationsChanged is the batch form of onStationChanged.
package radio

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// WatchClock reports the K-5 watch time of a profile (playback.Service); radio counts its listen
// time into it and ends listen sessions at the limit.
type WatchClock interface {
	WatchTime(ctx context.Context, profile model.Profile, now time.Time) (*model.WatchTime, error)
}

// options are the collaborators and settings New takes (Option).
type options struct {
	runner    transcode.Runner // ffmpeg for the relays and the logo scaling
	cacheDir  string           // logos below <cacheDir>/radio
	dir       string           // relay buffers (tmpfs /run/slouchcat/radio)
	tokenKey  []byte           // HMAC key of the listen tokens
	watch     WatchClock       // K-5 watch time; nil = no daily limit is checked
	userAgent string           // User-Agent of every outgoing request
	queue     *jobs.Queue
	scheduler *jobs.Scheduler
}

// Option configures New.
type Option func(*Service)

// WithRunner sets the ffmpeg runner of the relays and the logo scaling.
func WithRunner(r transcode.Runner) Option { return func(s *Service) { s.o.runner = r } }

// WithCacheDir sets the cache directory; logos live below <dir>/radio.
func WithCacheDir(dir string) Option { return func(s *Service) { s.o.cacheDir = dir } }

// WithDir sets the directory of the relay buffers (config RadioDir, the tmpfs of the compose file).
func WithDir(dir string) Option { return func(s *Service) { s.o.dir = dir } }

// WithTokenKey sets the HMAC key of the listen tokens (secrets.Keyring.Derive("radio-token")).
func WithTokenKey(key []byte) Option { return func(s *Service) { s.o.tokenKey = key } }

// WithWatchClock sets the K-5 watch time source (playback.Service).
func WithWatchClock(w WatchClock) Option { return func(s *Service) { s.o.watch = w } }

// WithUserAgent sets the User-Agent of the catalog, stream and logo requests.
func WithUserAgent(agent string) Option { return func(s *Service) { s.o.userAgent = agent } }

// WithQueue sets the job queue of the catalog sync and the logo jobs.
func WithQueue(q *jobs.Queue) Option { return func(s *Service) { s.o.queue = q } }

// WithScheduler sets the scheduler of the daily catalog sync.
func WithScheduler(sc *jobs.Scheduler) Option { return func(s *Service) { s.o.scheduler = sc } }

// WithClock replaces the clock (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// Service implements the radio service of the API (api.RadioService).
type Service struct {
	db  *store.DB
	log *slog.Logger
	now func() time.Time
	o   options

	cat  *catalogState // catalog, sync, index, favourites, preferences, admin (lane B)
	rel  *relayState   // relays, listen sessions, live HLS, listen time (lane A)
	logo *logoState    // logo chain (lane A)
	cov  *coverState   // cover chain (lane A, cover.go)

	// onListed hears the station ids a list or the favourites handed out, so the logo part
	// queues their logos (queueLogos, set in New).
	onListed func(ctx context.Context, stationIDs []int64)
	// onStationChanged hears a station row the relay or logo part wrote (logo_state, logo_hash,
	// broken_at, failures, shows_titles), so the catalog part reloads that row into its index
	// (reloadStation, set in New).
	onStationChanged func(ctx context.Context, stationID int64)
	// onStationsChanged is onStationChanged for many stations at once, with one copy of the index
	// (reloadStations, set in New); the logo queue of a list uses it.
	onStationsChanged func(ctx context.Context, stationIDs []int64)
}

// New returns the radio service; log is the redacting logger.
func New(db *store.DB, log *slog.Logger, opts ...Option) *Service {
	s := &Service{db: db, log: log, now: time.Now,
		onListed:          func(context.Context, []int64) {},
		onStationChanged:  func(context.Context, int64) {},
		onStationsChanged: func(context.Context, []int64) {},
	}
	for _, o := range opts {
		o(s)
	}
	s.cat = newCatalogState(s.o.userAgent)
	s.rel = newRelayState()
	s.logo = newLogoState()
	s.cov = newCoverState()
	// The catalog goes through the guarded client of the relay part (the radiotest build allows
	// the loopback there), created at the first request so a test can still swap the control.
	s.cat.fetch.lazy = func() *http.Client { return s.netClient().http }
	s.onStationChanged = s.reloadStation
	s.onStationsChanged = s.reloadStations
	s.onListed = s.queueLogos
	return s
}
