// Package metadata enriches titles from TMDB (plan sections 3.1 and 3.4 step 4, B-3, B-6, AN-8)
// and runs the image pipeline into <cache>/images (M1b: scaled WebP and JPEG through ffmpeg).
//
// Jobs:
//
//   - metadata.match, payload {"itemId": "<ULID>"}, key match:<itemId> (enqueued by library):
//     resolves the TMDB ID (tmdb_id, else imdb_id through /find, else a title search with the
//     parsed year ±1), fetches de-DE and en-US, writes item_texts for de and en and the item
//     columns, maps the certification after AN-8 and enqueues image.fetch.
//   - image.fetch, payload {"itemId": "<ULID>"}, key image:<itemId>, priority 2: downloads the
//     TMDB original of the poster and the backdrop once, scales it with ffmpeg (poster w200, w400,
//     w800; backdrop w780, w1280, w1920) into WebP and JPEG, computes the accent colour from the
//     w200 poster and upserts one images row per kind, size and format (images.go). A kind whose
//     derived rows stem from the current original and whose files exist is skipped, so the job
//     also fills in a size added later and regenerates deleted files; the library scan enqueues
//     it for such titles. A locked kind is not fetched from TMDB, but its sizes are scaled from
//     the stored image. A palette failure keeps the previous accent; a poster without accent is
//     retried a bounded number of times (maxAccentRetries).
//
// Locked fields: items.locked_fields is a JSON array of field names that enrichment never
// overwrites (B-6). The names are the items columns original_title, year, release_date,
// runtime_minutes, genres, tmdb_id, imdb_id, own_rating, the text fields title, overview and
// tagline (item_texts, both languages) and the image kinds poster and backdrop. A rating whose
// rating_source is nfo or manual is never replaced either.
//
// Genres: items.genres holds the TMDB genre names per language as a JSON object
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

// KeySource returns the configured TMDB key (v4 read access token). An empty key without error
// means that no key is configured.
type KeySource interface {
	TMDBKey(ctx context.Context) (string, error)
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
	now    func() time.Time

	accentMu      sync.Mutex
	accentRetries map[string]int // per item: queued image checks for a missing accent
}

type options struct {
	apiBase   string
	imageBase string
	http      *http.Client
	now       func() time.Time
}

// Option configures New.
type Option func(*options)

// WithBaseURL replaces the TMDB API and image base URLs (tests use an httptest.Server).
func WithBaseURL(api, images string) Option {
	return func(o *options) { o.apiBase, o.imageBase = api, images }
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
	o := options{apiBase: DefaultAPIBase, imageBase: DefaultImageBase, now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	root, err := paths.NewRoot(cacheDir)
	if err != nil {
		err = fmt.Errorf("metadata cache dir: %w", err)
	}
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
		client: NewClient(o.apiBase, o.imageBase, o.http),
		now:    o.now,

		accentRetries: map[string]int{},
	}
}

// RegisterJobs registers the handlers of metadata.match and image.fetch on w.
func (s *Service) RegisterJobs(w *jobs.Worker) {
	w.Handle(JobMatch, s.handleMatch)
	w.Handle(JobImageFetch, s.handleImageFetch)
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
