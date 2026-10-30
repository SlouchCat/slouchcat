// Package metadata enriches titles from the metadata providers (plan sections 3.1 and 3.4 step 4,
// design M3 section 3, B-3, B-6, AN-8, AN-15) and runs the image pipeline into <cache>/images
// (M1b: scaled WebP and JPEG through ffmpeg).
//
// Providers (provider.go): TMDB is the main source for movies and series (tmdb.go,
// tmdb_provider.go); TVDB v4 (tvdb.go) is optional with its own key and is asked for series only,
// when TMDB finds nothing or the administrator assigned a TVDB ID. Every request runs through the
// provider's ratelimit.Limiter (TMDB 20/s, TVDB 5/s).
//
// Jobs:
//
//   - metadata.match, payload {"itemId": "<ULID>"}, key match:<itemId> (enqueued by library, by
//     QueuePendingMetadata and by the series job). Movie: resolves the TMDB ID (tmdb_id, else
//     imdb_id through /find, else a title search with the parsed year ±1), fetches de-DE and
//     en-US with release dates and credits, writes item_texts for de and en, the item columns, the
//     image paths and the credits, maps the certification after AN-8 and enqueues image.fetch.
//     Series: resolves TMDB (tmdb_id, else tvdb_id or imdb_id through /find, else a title search
//     with year ±1), else TVDB; writes the series like a movie (rating from the content ratings,
//     rating_source "tmdb:DE" or "tvdb:DE"), then every season of the library with its episodes
//     (one transaction per season), and enqueues image.fetch for the series, seasons and episode
//     stills and metadata.match for episodes that still wait. Season: skipped
//     (season_via_series) unless the series job already matched it. Episode: needs a matched
//     series (else skipped/series_unmatched); an episode that is already matched makes no
//     provider call. Other kinds: skipped (unsupported_kind). A write that finds other provider
//     ids or locked fields on the title (or its series) than the job resolved with, because a
//     manual match or refresh came in meanwhile, writes nothing and fails the attempt, so the queue
//     retries with the new ids. A manual match or refresh raises a queued match job to priority 7.
//   - metadata.fill, payload {"itemId": "<ULID>"}, key fill:<itemId>, priority 2 (fill.go, ADR 0015):
//     walks the provider list of a matched movie or series (items.providers) front to back,
//     fetches each provider's record once (TMDB, TVDB; AniList through the mapping list of
//     Fribb, anilist.go and anime_lists.go) and decides field by field: the first provider with
//     a value wins, item_field_sources records who, a locked field is never touched, a rating is
//     only raised. Seasons and episodes follow when two providers describe them. Queued in the
//     transaction of every match, by a list change of a folder and by QueuePendingMetadata for
//     titles whose provenance is missing or stale.
//   - metadata.vote_backfill, payload {"after": "<ULID>"}, key vote_backfill (vote_backfill:<after>
//     for a successor), priority 2 (vote_backfill.go, concept 08 4.2): fills items.vote_average of
//     the titles matched before migration 00026 in batches of 200, asking TMDB and AniList through
//     their rate limiters; each batch queues the next. QueuePendingMetadata starts it when no job
//     of the type is active and a title waits.
//   - image.fetch, payload {"itemId": "<ULID>"}, key image:<itemId>, priority 2 (episode stills 1):
//     downloads the original of the stored image paths (items.tmdb_poster_path for movies, series
//     and seasons; items.tmdb_backdrop_path for movies, series and episode stills) once, scales it
//     with ffmpeg (poster w200, w400, w800; backdrop w780, w1280, w1920) into WebP and JPEG,
//     computes the accent colour from the w200 poster and upserts one images row per kind, size
//     and format (images.go). A kind whose derived rows stem from the current original and whose
//     files exist is skipped, so the job also fills in a size added later and regenerates deleted
//     files; the library scan enqueues it for such titles. A locked kind is not fetched, but its
//     sizes are scaled from the stored image. A palette failure keeps the previous accent; a
//     poster without accent is retried a bounded number of times (maxAccentRetries).
//   - trailer.find, payload {"itemId": "<ULID>"}, key trailer:<itemId>, priority 1: the trailer
//     chain TMDB videos → YouTube Data API (daily quota) → AI (off in M3) for movies and series
//     with a tmdb_id (trailers.go, youtube.go). metadata.match enqueues it after every match;
//     QueuePendingMetadata (after every full scan) enqueues it for titles still pending.
//   - podcast.fetch, payload {"podcastId": "<ULID>"}, key podcast:<podcastId>, priority 4: fetches
//     the RSS feed of a podcast subscription (podcast.go, M6 H-2) and stores the channel and its
//     episodes; RunPodcasts enqueues it for every subscription not fetched for FeedInterval.
//   - trakt.refresh, payload {}, key trakt, priority 2: fetches the public Trakt list trending
//     (movies and shows, top 100 each) and replaces spotlight_entries; a failure empties the
//     table (I-7). RunTrakt enqueues it once a day when a client id is configured (trakt.go, M7).
//
// Locked fields: items.locked_fields is a JSON array of field names that enrichment never
// overwrites (B-6). The names are the items columns original_title, year, release_date,
// runtime_minutes, genres, tmdb_id, tvdb_id, imdb_id, own_rating, vote_average, the text fields
// title, overview and tagline (item_texts, both languages), the image kinds poster and backdrop,
// cast (the credits of the title) and trailer (the automatic trailers). A rating whose
// rating_source is nfo or manual is never replaced either.
//
// Genres: items.genres holds the provider genre names per language as a JSON object
// {"de": ["Drama", …], "en": ["Drama", …]}; the API layer picks the profile language.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Job types handled by this package.
const (
	JobMatch      = "metadata.match"
	JobImageFetch = "image.fetch"
)

// imageFetchPriority runs image downloads before bulk work but after interactive jobs.
const imageFetchPriority = 2

// KeySource returns the configured provider keys. An empty key without error means that no key
// is configured (the provider is then not used).
type KeySource interface {
	// TMDBKey returns the TMDB v4 read access token.
	TMDBKey(ctx context.Context) (string, error)
	// TVDBKey returns the TVDB v4 API key and the optional subscriber PIN.
	TVDBKey(ctx context.Context) (key, pin string, err error)
	// YouTubeKey returns the YouTube Data API v3 key.
	YouTubeKey(ctx context.Context) (string, error)
	// TraktClientID returns the Trakt client id (M7, E-7).
	TraktClientID(ctx context.Context) (string, error)
}

// Service runs the metadata jobs and checks TMDB keys.
type Service struct {
	db     *store.DB
	write  *systemq.Queries
	read   *systemq.Queries
	queue  *jobs.Queue
	keys   KeySource
	cache  paths.Root
	cacheE error // set when cacheDir is not usable as a root
	runner transcode.Runner
	log    *slog.Logger
	client *Client
	tmdb   *tmdbProvider
	tvdb   *tvdbProvider
	now    func() time.Time

	anilist    *aniListProvider // AniList, a filler (ADR 0015; anilist.go)
	animeLists *animeLists      // the mapping list of Fribb (anime_lists.go)

	trailersTMDB    TrailerProvider
	trailersYouTube TrailerProvider
	trailersAI      AITrailerProvider

	// recheck ends open direct-play streams whose title became invisible (WithVisibilityRecheck).
	recheck func(ctx context.Context) error
	// newEpisodes is told the ids of new podcast episodes after their transaction (WithNewEpisodes).
	newEpisodes func(ctx context.Context, ids []string) error

	trakt traktConfig // Trakt lists (trakt.go)

	accentMu      sync.Mutex
	accentRetries map[string]int // per item: queued image checks for a missing accent

	feeds feedFetcher // podcast feeds (podcast.go)
}

type options struct {
	apiBase     string
	imageBase   string
	tvdbBase    string
	tvdbArtBase string
	anilistBase string
	anilistImgs string
	anilistOn   bool
	animeLists  string
	animeHTTP   *http.Client
	youtubeBase string
	quota       *ratelimit.Quota
	trailerAI   string
	http        *http.Client
	now         func() time.Time
	recheck     func(ctx context.Context) error
	feedHTTP    *http.Client
	feedNow     func() time.Time
	feedAgent   string
	newEpisodes func(ctx context.Context, ids []string) error
	traktID     string
	trakt       traktOptions
}

// Option configures New.
type Option func(*options)

// WithBaseURL replaces the TMDB API and image base URLs (tests use an httptest.Server).
func WithBaseURL(api, images string) Option {
	return func(o *options) { o.apiBase, o.imageBase = api, images }
}

// WithTVDBBaseURL replaces the TVDB API and artwork base URLs (tests use an httptest.Server).
func WithTVDBBaseURL(api, artworks string) Option {
	return func(o *options) { o.tvdbBase, o.tvdbArtBase = api, artworks }
}

// WithAniList switches the AniList filler on or off server-wide (SLOUCHCAT_ANILIST; off keeps
// the ticks on the folders, listMetadataProviders reports configured false).
func WithAniList(enabled bool) Option {
	return func(o *options) { o.anilistOn = enabled }
}

// WithAniListBaseURL replaces the AniList GraphQL endpoint and image host (tests use an
// httptest.Server).
func WithAniListBaseURL(api, images string) Option {
	return func(o *options) { o.anilistBase, o.anilistImgs = api, images }
}

// WithAnimeListsURL replaces the URL of the mapping list and the client that fetches it (tests
// use an httptest.Server).
func WithAnimeListsURL(url string, hc *http.Client) Option {
	return func(o *options) { o.animeLists, o.animeHTTP = url, hc }
}

// WithYouTubeBaseURL replaces the YouTube Data API base URL (tests use an httptest.Server).
func WithYouTubeBaseURL(api string) Option {
	return func(o *options) { o.youtubeBase = api }
}

// WithYouTubeQuota replaces the daily YouTube quota (default 10'000 units, reset at midnight
// Pacific time); tests pass one with a mock clock.
func WithYouTubeQuota(q *ratelimit.Quota) Option {
	return func(o *options) { o.quota = q }
}

// WithTrailerAI sets the AI step of the trailer chain (SLOUCHCAT_TRAILER_AI). M3 knows only
// "off"; any other value logs a warning and stays off.
func WithTrailerAI(mode string) Option {
	return func(o *options) { o.trailerAI = mode }
}

// WithVisibilityRecheck sets the function a match calls after it changed the own rating of a
// title (the triggers pass it on to seasons, episodes and extras), so open direct-play streams of
// a title that became invisible end (playback.Service.RecheckVisibility). Without it nothing is
// rechecked.
func WithVisibilityRecheck(recheck func(ctx context.Context) error) Option {
	return func(o *options) { o.recheck = recheck }
}

// WithNewEpisodes sets the function a podcast fetch calls with the ids of the episodes it inserted,
// after their transaction (notify.Service.NewEpisodes, M7 E-8). An error is logged.
func WithNewEpisodes(fn func(ctx context.Context, ids []string) error) Option {
	return func(o *options) { o.newEpisodes = fn }
}

// WithHTTPClient replaces the HTTP client (default: 15 s timeout).
func WithHTTPClient(c *http.Client) Option {
	return func(o *options) { o.http = c }
}

// WithClock replaces the clock used for matched_at, updated_at and created_at.
func WithClock(now func() time.Time) Option {
	return func(o *options) { o.now = now }
}

// New returns the service. cacheDir is the absolute, existing cache directory; images go to
// <cacheDir>/images. runner scales the images (ffmpeg).
func New(db *store.DB, queue *jobs.Queue, keys KeySource, cacheDir string, runner transcode.Runner, log *slog.Logger, opts ...Option) *Service {
	o := options{apiBase: DefaultAPIBase, imageBase: DefaultImageBase, tvdbBase: DefaultTVDBBase,
		tvdbArtBase: DefaultTVDBArtworkBase, youtubeBase: DefaultYouTubeBase, trailerAI: "off", now: time.Now,
		anilistBase: DefaultAniListBase, anilistImgs: DefaultAniListImageBase, anilistOn: true, animeLists: DefaultAnimeListsURL}
	for _, opt := range opts {
		opt(&o)
	}
	root, err := paths.NewRoot(cacheDir)
	if err != nil {
		err = fmt.Errorf("metadata cache dir: %w", err)
	}
	client := NewClient(o.apiBase, o.imageBase, o.http)
	client.log = log
	if o.quota == nil {
		o.quota = ratelimit.NewQuota("youtube", youtubeDailyUnits, ratelimit.MidnightPacific)
	}
	tvdb := newTVDBProvider(o.tvdbBase, o.tvdbArtBase, o.http, keys)
	tvdb.log = log
	return &Service{
		db:     db,
		write:  systemq.New(db.Write),
		read:   systemq.New(db.Read),
		queue:  queue,
		keys:   keys,
		cache:  root,
		cacheE: err,
		runner: runner,
		log:    log,
		client: client,
		tmdb:   &tmdbProvider{client: client, keys: keys},
		tvdb:   tvdb,
		now:    o.now,

		anilist:    newAniListProvider(o.anilistBase, o.anilistImgs, o.http, o.anilistOn, log),
		animeLists: newAnimeLists(o.animeLists, o.animeHTTP, root, o.now, log),

		trailersTMDB:    &tmdbTrailers{client: client, keys: keys},
		trailersYouTube: newYouTubeTrailers(o.youtubeBase, o.http, o.quota, keys),
		trailersAI:      newAITrailers(o.trailerAI, log),
		recheck:         o.recheck,
		newEpisodes:     o.newEpisodes,
		trakt:           newTraktConfig(o.traktID, o.trakt, o.now, o.feedAgent),

		accentRetries: map[string]int{},
		feeds:         newFeedFetcher(o.feedHTTP, o.feedNow, o.feedAgent),
	}
}

// RegisterJobs registers the handlers of metadata.match, metadata.fill, metadata.vote_backfill,
// image.fetch, trailer.find, podcast.fetch and trakt.refresh on w.
func (s *Service) RegisterJobs(w *jobs.Worker) {
	w.Handle(JobMatch, s.handleMatch)
	w.Handle(JobFill, s.handleFill)
	w.Handle(JobVoteBackfill, s.handleVoteBackfill)
	w.Handle(JobImageFetch, s.handleImageFetch)
	w.Handle(JobTrailerFind, s.handleTrailerFind)
	w.Handle(JobPodcastFetch, s.handlePodcastFetch)
	w.Handle(JobTraktRefresh, s.handleTraktRefresh)
}

// CheckKey validates a TMDB key. It returns nil, model.ErrTMDBKeyInvalid or
// model.ErrTMDBUnreachable.
func (s *Service) CheckKey(ctx context.Context, key string) error {
	return s.client.CheckKey(ctx, key)
}

// itemPayload is the payload of both jobs.
type itemPayload struct {
	ItemID string `json:"itemId"`
}

func parsePayload(j *jobs.Job) (string, error) {
	var p itemPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return "", fmt.Errorf("%s payload: %w", j.Type, err)
	}
	if !isULID(p.ItemID) {
		return "", fmt.Errorf("%s payload: invalid itemId %q", j.Type, p.ItemID)
	}
	return p.ItemID, nil
}

// logRequest records an outbound provider request at debug level: provider, host and path, never
// the query (the YouTube key travels there) or a header. Operators count requests per host with
// it (docs/perf/bibliotheken.md).
func logRequest(log *slog.Logger, provider string, req *http.Request) {
	if log != nil {
		log.Debug("metadata request", "provider", provider, "host", req.URL.Host, "path", req.URL.Path)
	}
}

// isULID accepts the 26 Crockford base32 characters of an item ID, which also makes the ID safe
// as a directory name under the cache root.
func isULID(id string) bool {
	if len(id) != 26 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}

// lockedFields parses items.locked_fields; an invalid value locks nothing.
func lockedFields(raw string) map[string]bool {
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return map[string]bool{}
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}
