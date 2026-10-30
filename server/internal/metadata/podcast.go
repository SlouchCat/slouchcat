package metadata

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/feed"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Podcast subscriptions (M6, design 3.4, H-2, AN-2). A subscription is an item of kind podcast in a
// library of kind podcasts with its feed URL; its episodes are podcast_episode items below it, one
// per guid, played and downloaded from their enclosure. Subscribe fetches the feed once right away;
// the job podcast.fetch fetches it again (conditional GET with ETag and Last-Modified), and the
// schedule podcast.poll (every 5 minutes, concept 09) enqueues that job for every subscription
// not fetched for FeedInterval.
//
// Ratings (AN-2): the podcast starts with the library's default rating (rating_source library),
// an explicit channel is FSK 18 (rating_source feed); an explicit episode is FSK 18 (feed), any
// other episode inherits the podcast's rating. Manual ratings and locked fields stay.

const (
	// JobPodcastFetch fetches the feed of one subscription; payload {"podcastId"}.
	JobPodcastFetch = "podcast.fetch"
	// JobPodcastPoll is the schedule that queues podcast.fetch for the due subscriptions (the
	// 5-minute tick of the earlier poller, made visible; no job row ever carries this type).
	JobPodcastPoll = "podcast.poll"
	// FeedInterval is the age after which the poll fetches a feed again (H-2: a new episode
	// appears in less than 65 minutes, with the 5-minute tick and the job's wait).
	FeedInterval = 55 * time.Minute

	podcastFetchPriority = 4
	// feedPollInterval is the default interval of the schedule podcast.poll.
	feedPollInterval = 5 * time.Minute
	feedTimeout      = 30 * time.Second // whole request of a fetch
	subscribeTimeout = 20 * time.Second // the synchronous first fetch (createPodcast)
	maxFeedBytes     = 16 << 20
	maxFeedRedirects = 5
	maxFeedURL       = 2048
	maxFeedEpisodes  = 2000 // the newest episodes of a feed that are stored
	maxFeedError     = 200  // characters of items.feed_error
	feedBatch        = 150  // episodes per transaction (each also writes two texts and fires triggers)
	maxEpisodeCovers = 50   // new episodes with an image of their own queued for image.fetch per fetch
)

// Errors of a fetch.
var (
	errFeedTooLarge = errors.New("feed larger than 16 MiB")
	errFeedRedirect = errors.New("feed redirects too often or to another scheme")
)

// Values of items.feed_origin (ADR 0015): who chose the feed of a podcast. A subscription made
// by hand keeps the LAN exception of package netguard; a feed read from the metadata.json of a
// podcast folder (netguard.FeedOriginFile) never gets it.
const feedOriginAdmin = "admin"

// errFeedStatus is an HTTP status other than 200 and 304.
type errFeedStatus int

func (e errFeedStatus) Error() string { return fmt.Sprintf("feed answered HTTP %d", int(e)) }

// feedFetcher fetches feeds: clients with one policy (30 s, at most 5 redirects, http and https
// only, no address in the local network unless the feed's own host is local; package netguard),
// one limiter for every feed host, the clock of the poller and the User-Agent.
type feedFetcher struct {
	clients netguard.Clients
	test    *http.Client // WithFeedHTTPClient: used for every feed instead of clients
	limiter *ratelimit.Limiter
	now     func() time.Time
	agent   string
}

func newFeedFetcher(c *http.Client, now func() time.Time, agent string) feedFetcher {
	checkRedirect := func(req *http.Request, via []*http.Request) error {
		if len(via) > maxFeedRedirects || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
			return errFeedRedirect
		}
		return nil
	}
	f := feedFetcher{
		clients: netguard.NewClients(http.Client{Timeout: feedTimeout, CheckRedirect: checkRedirect}, 0),
		limiter: ratelimit.New("feeds", 2, 4), now: now, agent: cmp.Or(agent, "Slouchcat"),
	}
	if c != nil {
		cp := *c
		cp.CheckRedirect = checkRedirect
		f.test = &cp
	}
	if f.now == nil {
		f.now = time.Now
	}
	return f
}

// client is the HTTP client of a fetch of feedURL; a feed read from a file (origin "file") is
// always fetched guarded (package netguard, ADR 0015).
func (f feedFetcher) client(ctx context.Context, feedURL, origin string) *http.Client {
	if f.test != nil {
		return f.test
	}
	return f.clients.For(ctx, feedURL, origin)
}

// WithFeedHTTPClient replaces the HTTP client of the feed fetches (tests): the address guard of
// package netguard does not apply then; the redirect policy is always the service's own.
func WithFeedHTTPClient(c *http.Client) Option {
	return func(o *options) { o.feedHTTP = c }
}

// WithFeedClock replaces the clock of the podcast poller and the feed state (tests).
func WithFeedClock(now func() time.Time) Option {
	return func(o *options) { o.feedNow = now }
}

// WithFeedUserAgent sets the User-Agent of feed fetches ("Slouchcat/<version>").
func WithFeedUserAgent(agent string) Option {
	return func(o *options) { o.feedAgent = agent }
}

// fetched is the result of one fetch.
type fetched struct {
	feed        feed.Feed
	etag        string
	modified    string
	notModified bool
}

// fetch gets and parses a feed; etag and modified make it conditional (304 → notModified). Errors
// name the scheme and host of the feed only (netguard.Clean): a private feed carries its
// credentials in the path or query.
func (f feedFetcher) fetch(ctx context.Context, feedURL, origin, etag, modified string) (fetched, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return fetched{}, netguard.Clean(err)
	}
	req.Header.Set("User-Agent", f.agent)
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.5")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if modified != "" {
		req.Header.Set("If-Modified-Since", modified)
	}
	resp, err := f.limiter.Do(ctx, f.client(ctx, feedURL, origin), req)
	if err != nil {
		return fetched{}, netguard.Clean(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return fetched{notModified: true, etag: etag, modified: modified}, nil
	case http.StatusOK:
	default:
		return fetched{}, errFeedStatus(resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return fetched{}, netguard.Clean(err)
	}
	if len(body) > maxFeedBytes {
		return fetched{}, errFeedTooLarge
	}
	parsed, err := feed.Parse(bytes.NewReader(body))
	if err != nil {
		return fetched{}, err
	}
	return fetched{feed: parsed, etag: resp.Header.Get("ETag"), modified: resp.Header.Get("Last-Modified")}, nil
}

// feedInvalid reports whether a fetch error means the content is no podcast feed (rather than the
// feed being unreachable).
func feedInvalid(err error) bool {
	return errors.Is(err, feed.ErrNotRSS) || errors.Is(err, feed.ErrInvalid) || errors.Is(err, errFeedTooLarge)
}

// feedErrorText is the stored feed_error: a stable code and a short text without credentials (the
// URL has no user info, validateFeedURL, and fetch errors name only its scheme and host).
func feedErrorText(err error) string {
	code := "feed_unreachable"
	if feedInvalid(err) {
		code = "feed_invalid"
	}
	text := code + ": " + err.Error()
	if r := []rune(text); len(r) > maxFeedError {
		text = string(r[:maxFeedError])
	}
	return text
}

// validateFeedURL accepts an absolute http or https URL with a host and without credentials.
func validateFeedURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxFeedURL {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", false
	}
	return u.String(), true
}

// Subscribe subscribes a podcast library to a feed (createPodcast): it validates the URL and the
// library, fetches the feed (at most 20 s) and stores the podcast with its episodes, then queues
// the cover. Errors: *model.ValidationError (feedUrl invalid_url, libraryId unknown_library or
// library_kind), model.ErrFeedExists, model.ErrFeedUnreachable, model.ErrFeedInvalid.
func (s *Service) Subscribe(ctx context.Context, libraryID int64, feedURL string) (model.AdminPodcast, error) {
	var verr *model.ValidationError
	clean, ok := validateFeedURL(feedURL)
	if !ok {
		verr = verr.Add("feedUrl", "invalid_url")
	}
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		verr = verr.Add("libraryId", "unknown_library")
	case err != nil:
		return model.AdminPodcast{}, fmt.Errorf("get library: %w", err)
	case lib.Kind != model.LibraryPodcasts:
		verr = verr.Add("libraryId", "library_kind")
	}
	if verr != nil {
		return model.AdminPodcast{}, verr
	}
	if _, err := s.read.PodcastFindByFeed(ctx, systemq.PodcastFindByFeedParams{LibraryID: libraryID, FeedUrl: nullStr(clean)}); err == nil {
		return model.AdminPodcast{}, model.ErrFeedExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return model.AdminPodcast{}, fmt.Errorf("find podcast: %w", err)
	}
	fctx, cancel := context.WithTimeout(ctx, subscribeTimeout)
	res, err := s.feeds.fetch(fctx, clean, feedOriginAdmin, "", "")
	cancel()
	switch {
	case err != nil && feedInvalid(err):
		s.log.InfoContext(ctx, "podcast feed invalid", "library_id", libraryID, "err", err)
		return model.AdminPodcast{}, model.ErrFeedInvalid
	case err != nil:
		s.log.InfoContext(ctx, "podcast feed unreachable", "library_id", libraryID, "err", err)
		return model.AdminPodcast{}, model.ErrFeedUnreachable
	case res.notModified:
		return model.AdminPodcast{}, model.ErrFeedInvalid // 304 without a conditional request
	}
	now := s.feeds.now()
	rating, source := channelRating(res.feed.Explicit, lib.DefaultRating)
	title := cmp.Or(strings.TrimSpace(res.feed.Title), clean)
	p, err := s.write.PodcastInsert(ctx, systemq.PodcastInsertParams{
		ID: ulid.Make().String(), LibraryID: libraryID, Title: title, SortTitle: sortTitle(title),
		Author: nullStr(res.feed.Author), Explicit: boolInt(res.feed.Explicit), CoverUrl: nullStr(httpURL(res.feed.ImageURL)),
		FeedUrl: nullStr(clean), FeedEtag: nullStr(res.etag), FeedModified: nullStr(res.modified), FeedOrigin: nullStr(feedOriginAdmin),
		Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, OwnRating: rating, RatingSource: source,
	})
	if err != nil {
		// The unique index items_podcast_feed refuses a second live subscription that a concurrent
		// request inserted while this one fetched the feed.
		if _, ferr := s.write.PodcastFindByFeed(ctx, systemq.PodcastFindByFeedParams{LibraryID: libraryID, FeedUrl: nullStr(clean)}); ferr == nil {
			return model.AdminPodcast{}, model.ErrFeedExists
		}
		return model.AdminPodcast{}, fmt.Errorf("insert podcast: %w", err)
	}
	if _, err := s.storeEpisodes(ctx, p.ID, libraryID, title, res.feed, nil, now); err != nil {
		// Without its episodes the subscription is of no use; it goes again, so a retry is possible.
		if _, derr := s.write.PodcastSoftDelete(context.WithoutCancel(ctx), systemq.PodcastSoftDeleteParams{
			Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, ID: p.ID}); derr != nil {
			err = errors.Join(err, derr)
		}
		return model.AdminPodcast{}, err
	}
	if res.feed.ImageURL != "" {
		if err := s.enqueueImages(ctx, p.ID, imageFetchPriority); err != nil {
			s.log.WarnContext(ctx, "queue podcast cover", "item_id", p.ID, "err", err)
		}
	}
	s.log.InfoContext(ctx, "podcast subscribed", "item_id", p.ID, "library_id", libraryID, "episodes", len(res.feed.Episodes))
	return s.adminPodcast(ctx, p.ID)
}

// Unsubscribe soft-deletes a subscription and its episodes (deletePodcast); downloads of the
// episodes are revoked at the next sync, the caller ends open streams (RecheckVisibility).
// model.ErrNotFound for an unknown id.
func (s *Service) Unsubscribe(ctx context.Context, id string) error {
	n, err := s.write.PodcastSoftDelete(ctx, systemq.PodcastSoftDeleteParams{Now: sql.NullInt64{Int64: s.feeds.now().UnixMilli(), Valid: true}, ID: id})
	if err != nil {
		return fmt.Errorf("delete podcast: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	s.log.InfoContext(ctx, "podcast unsubscribed", "item_id", id)
	return nil
}

// ListPodcasts returns every live subscription with its feed state, newest first.
func (s *Service) ListPodcasts(ctx context.Context) ([]model.AdminPodcast, error) {
	rows, err := s.read.PodcastList(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list podcasts: %w", err)
	}
	out := make([]model.AdminPodcast, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAdminPodcast(r))
	}
	return out, nil
}

// RefreshPodcast queues the fetch of a subscription now (refreshPodcast); model.ErrNotFound for an
// unknown or deleted one.
func (s *Service) RefreshPodcast(ctx context.Context, id string) (model.AdminPodcast, error) {
	p, err := s.adminPodcast(ctx, id)
	if err != nil {
		return model.AdminPodcast{}, err
	}
	if _, err := s.enqueuePodcast(ctx, id, podcastFetchPriority); err != nil {
		return model.AdminPodcast{}, err
	}
	return p, nil
}

// RefreshLibrary queues the fetch of every subscription of a library (scanLibrary of a podcast
// library) and returns how many it queued.
func (s *Service) RefreshLibrary(ctx context.Context, libraryID int64) (int, error) {
	ids, err := s.read.PodcastListIDsByLibrary(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list podcasts of library %d: %w", libraryID, err)
	}
	for i, id := range ids {
		if _, err := s.enqueuePodcast(ctx, id, podcastFetchPriority); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}

// QueueDuePodcasts enqueues podcast.fetch for every subscription not fetched for FeedInterval and
// returns how many it queued (one tick of the schedule podcast.poll).
func (s *Service) QueueDuePodcasts(ctx context.Context) (int, error) {
	ids, err := s.runPodcastPoll(ctx, s.queue, podcastFetchPriority)
	return len(ids), err
}

// runPodcastPoll is the Run of podcast.poll (jobs.Type): it queues podcast.fetch with prio for
// every subscription not fetched for FeedInterval (the due check is per subscription and stays
// here, not in the schedule) and returns the job ids; a fetch that already waits is raised to prio.
func (s *Service) runPodcastPoll(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	before := s.feeds.now().Add(-FeedInterval).UnixMilli()
	ids, err := s.read.PodcastListDue(ctx, sql.NullInt64{Int64: before, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list due podcasts: %w", err)
	}
	queued := make([]int64, 0, len(ids))
	for _, id := range ids {
		jobID, err := s.enqueuePodcast(ctx, id, prio)
		if err != nil {
			return queued, err
		}
		queued = append(queued, jobID)
	}
	return queued, nil
}

// podcastPayload is the payload of podcast.fetch.
type podcastPayload struct {
	PodcastID string `json:"podcastId"`
}

// enqueuePodcast queues the fetch of a subscription with prio and returns the job id; a fetch
// that already waits under the key is raised to prio and never lowered.
func (s *Service) enqueuePodcast(ctx context.Context, id string, prio int) (int64, error) {
	payload, err := json.Marshal(podcastPayload{PodcastID: id})
	if err != nil {
		return 0, fmt.Errorf("%s payload: %w", JobPodcastFetch, err)
	}
	jobID, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobPodcastFetch, Priority: prio,
		IdempotencyKey: "podcast:" + id, Payload: payload})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s %s: %w", JobPodcastFetch, id, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, jobID, prio); err != nil {
		return 0, fmt.Errorf("raise %s %s: %w", JobPodcastFetch, id, err)
	}
	return jobID, nil
}

func (s *Service) handlePodcastFetch(ctx context.Context, j *jobs.Job) error {
	var p podcastPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("%s payload: %w", JobPodcastFetch, err)
	}
	if !isULID(p.PodcastID) {
		return fmt.Errorf("%s payload: invalid podcastId %q", JobPodcastFetch, p.PodcastID)
	}
	return s.FetchPodcast(ctx, p.PodcastID)
}

// FetchPodcast fetches the feed of a subscription and stores what changed: 304 only records the
// check, 200 updates the channel (except locked fields) and inserts or updates the episodes by
// guid (the newest 2000; episodes that leave the feed stay). A failure is recorded in feed_error
// and returned, so the job retries with backoff. An unknown or deleted podcast is no error.
func (s *Service) FetchPodcast(ctx context.Context, id string) error {
	p, err := s.read.PodcastGet(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && p.DeletedAt.Valid) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get podcast %s: %w", id, err)
	}
	res, err := s.feeds.fetch(ctx, p.FeedUrl.String, p.FeedOrigin.String, p.FeedEtag.String, p.FeedModified.String)
	now := s.feeds.now()
	checked := sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.log.InfoContext(ctx, "podcast feed fetch failed", "item_id", id, "err", err)
		if serr := s.write.PodcastSetChecked(context.WithoutCancel(ctx), systemq.PodcastSetCheckedParams{
			Now: checked, FeedError: nullStr(feedErrorText(err)), ID: id}); serr != nil {
			return errors.Join(err, serr)
		}
		return fmt.Errorf("fetch podcast %s: %w", id, err)
	}
	if res.notModified {
		return s.write.PodcastSetChecked(ctx, systemq.PodcastSetCheckedParams{Now: checked, ID: id})
	}
	locked := lockedFields(p.LockedFields)
	title := cmp.Or(strings.TrimSpace(res.feed.Title), p.Title)
	if locked["title"] {
		title = p.Title
	}
	rating, source := p.OwnRating, p.RatingSource
	if !locked["own_rating"] && (!source.Valid || source.String == "library" || source.String == "feed") {
		rating, source = channelRating(res.feed.Explicit, p.LibraryDefaultRating)
	}
	cover := nullStr(httpURL(res.feed.ImageURL))
	if locked["poster"] {
		cover = p.CoverUrl
	}
	author := nullStr(res.feed.Author)
	if locked["author"] {
		author = p.Author
	}
	if err := s.write.PodcastUpdateChannel(ctx, systemq.PodcastUpdateChannelParams{
		Title: title, SortTitle: sortTitle(title), Author: author, Explicit: boolInt(res.feed.Explicit),
		CoverUrl: cover, OwnRating: rating, RatingSource: source, FeedEtag: nullStr(res.etag), FeedModified: nullStr(res.modified),
		Now: checked, ID: id,
	}); err != nil {
		return fmt.Errorf("update podcast %s: %w", id, err)
	}
	rerated, err := s.storeEpisodes(ctx, id, p.LibraryID, title, res.feed, locked, now)
	if err != nil {
		return err
	}
	if cover.String != p.CoverUrl.String && cover.Valid {
		if err := s.enqueueImages(ctx, id, imageFetchPriority); err != nil {
			s.log.WarnContext(ctx, "queue podcast cover", "item_id", id, "err", err)
		}
	}
	if rating != p.OwnRating || rerated {
		// A stricter podcast or episode ends the streams of profiles that may no longer hear it.
		s.recheckVisibility(ctx)
	}
	return nil
}

// storeEpisodes writes the channel texts (locked ones stay) and the newest episodes of a feed below
// a podcast, in transactions of feedBatch episodes: new guids are inserted, known ones updated
// (titles, texts, enclosure, date, duration, cover, the explicit rating unless manual or locked).
// An episode keeps an image only when it differs from the channel's; image.fetch is queued for the
// newest maxEpisodeCovers new episodes with one. rerated reports a changed rating of a known episode.
// The ids of the inserted episodes go to the new-episode hook (WithNewEpisodes) after their
// transactions, also when a later batch fails, but only those published after the newest episode
// the podcast already had (or undated ones while it has no dated episode): a feed that renews its
// guids or adds old episodes does not announce them as new.
func (s *Service) storeEpisodes(ctx context.Context, podcastID string, libraryID int64, title string, f feed.Feed,
	locked map[string]bool, now time.Time,
) (rerated bool, err error) {
	eps := newestEpisodes(f.Episodes, maxFeedEpisodes)
	parent := sql.NullString{String: podcastID, Valid: true}
	known, err := s.write.PodcastListEpisodes(ctx, parent)
	if err != nil {
		return false, fmt.Errorf("list episodes of %s: %w", podcastID, err)
	}
	byGUID := make(map[string]systemq.PodcastListEpisodesRow, len(known))
	var newest sql.NullInt64 // the newest publication date the podcast had before this fetch
	for _, k := range known {
		byGUID[k.Guid.String] = k
		if k.PublishedAt.Valid && (!newest.Valid || k.PublishedAt.Int64 > newest.Int64) {
			newest = k.PublishedAt
		}
	}
	if err := upsertTexts(ctx, s.write, podcastID, title, f.Description, locked); err != nil {
		return false, err
	}
	// File episodes of a podcast folder (ADR 0015): a new feed episode that matches exactly one
	// of them takes it over instead of becoming a second row.
	fileEpisodes, err := s.write.PodcastListFileEpisodesWithoutGUID(ctx, parent)
	if err != nil {
		return false, fmt.Errorf("list file episodes of %s: %w", podcastID, err)
	}
	w := episodeWrite{parent: parent, libraryID: libraryID, channelImage: httpURL(f.ImageURL), byGUID: byGUID,
		files: fileEpisodes, ms: now.UnixMilli(), now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}}
	var covers, added []string
	defer func() { s.announceEpisodes(ctx, added) }()
	for chunk := range slices.Chunk(eps, feedBatch) {
		var batch []string
		err := s.inTx(ctx, func(q *systemq.Queries) error {
			for _, e := range chunk {
				c, err := w.store(ctx, q, e)
				if err != nil {
					return err
				}
				rerated = rerated || c.rerated
				if c.newCover != "" && len(covers) < maxEpisodeCovers {
					covers = append(covers, c.newCover)
				}
				if c.newID != "" && (!newest.Valid || (c.published.Valid && c.published.Int64 > newest.Int64)) {
					batch = append(batch, c.newID)
				}
			}
			return nil
		})
		if err != nil {
			return false, fmt.Errorf("store episodes of %s: %w", podcastID, err)
		}
		added = append(added, batch...)
	}
	for _, id := range covers {
		if err := s.enqueueImages(ctx, id, stillPriority); err != nil {
			s.log.WarnContext(ctx, "queue episode cover", "item_id", id, "err", err)
		}
	}
	return rerated, nil
}

// newestEpisodes returns at most limit episodes, newest first (undated ones last, in feed order).
func newestEpisodes(eps []feed.Episode, limit int) []feed.Episode {
	out := slices.Clone(eps)
	slices.SortStableFunc(out, func(a, b feed.Episode) int { return b.PublishedAt.Compare(a.PublishedAt) })
	return out[:min(len(out), limit)]
}

// episodeWrite is what storing the episodes of one fetch shares.
type episodeWrite struct {
	parent       sql.NullString
	libraryID    int64
	channelImage string // the channel's image URL; an episode with the same image has none of its own
	byGUID       map[string]systemq.PodcastListEpisodesRow
	files        []systemq.PodcastListFileEpisodesWithoutGUIDRow // file episodes without guid, pairing candidates
	ms           int64
	now          sql.NullInt64
}

// episodeChange is what storing one episode changed: the rating of a known episode, or a new
// episode (newID, its publication date) with an image of its own (newCover, its id).
type episodeChange struct {
	rerated   bool
	newID     string
	newCover  string
	published sql.NullInt64
}

// store inserts or updates one episode inside the caller's transaction; locked fields (title,
// poster, overview, own_rating) keep their stored values.
func (w *episodeWrite) store(ctx context.Context, q *systemq.Queries, e feed.Episode) (episodeChange, error) {
	enclosure := httpURL(e.Enclosure.URL)
	if enclosure == "" {
		return episodeChange{}, nil // only http and https enclosures are played and downloaded
	}
	title := cmp.Or(strings.TrimSpace(e.Title), e.GUID)
	sort := sortTitle(title)
	var published sql.NullInt64
	if !e.PublishedAt.IsZero() {
		published = sql.NullInt64{Int64: e.PublishedAt.UnixMilli(), Valid: true}
	}
	explicit := e.Explicit != nil && *e.Explicit
	rating, source := episodeRating(explicit)
	cover := nullStr(httpURL(e.ImageURL))
	if cover.String == w.channelImage {
		cover = sql.NullString{}
	}
	known, ok := w.byGUID[e.GUID]
	if ok {
		if known.DeletedAt.Valid {
			return episodeChange{}, nil
		}
		locked := lockedFields(known.LockedFields)
		if locked["own_rating"] || known.RatingSource.String == "manual" {
			rating, source = known.OwnRating, known.RatingSource
		}
		if locked["title"] {
			title, sort = known.Title, known.SortTitle
		}
		if locked["poster"] {
			cover = known.CoverUrl
		}
		if err := q.PodcastUpdateEpisode(ctx, systemq.PodcastUpdateEpisodeParams{
			Title: title, SortTitle: sort, PublishedAt: published, DurationMs: positive(e.Duration.Milliseconds()),
			EnclosureUrl: nullStr(enclosure), EnclosureType: nullStr(e.Enclosure.Type), EnclosureLength: positive(e.Enclosure.Length),
			Explicit: boolInt(explicit), CoverUrl: cover, SeasonNumber: positive(int64(e.Season)), EpisodeNumber: positive(int64(e.Number)),
			OwnRating: rating, RatingSource: source, Now: w.ms, ID: known.ID,
		}); err != nil {
			return episodeChange{}, fmt.Errorf("update episode: %w", err)
		}
		return episodeChange{rerated: rating != known.OwnRating}, upsertTexts(ctx, q, known.ID, title, e.Description, locked)
	}
	if adopted, ok := w.adopt(e, published); ok {
		if err := q.PodcastAdoptFileEpisode(ctx, systemq.PodcastAdoptFileEpisodeParams{
			Title: title, SortTitle: sort, Guid: nullStr(e.GUID), PublishedAt: published, DurationMs: positive(e.Duration.Milliseconds()),
			EnclosureUrl: nullStr(enclosure), EnclosureType: nullStr(e.Enclosure.Type), EnclosureLength: positive(e.Enclosure.Length),
			Explicit: boolInt(explicit), CoverUrl: cover, SeasonNumber: positive(int64(e.Season)), EpisodeNumber: positive(int64(e.Number)),
			OwnRating: rating, RatingSource: source, Now: w.ms, ID: adopted.ID,
		}); err != nil {
			return episodeChange{}, fmt.Errorf("adopt file episode: %w", err)
		}
		w.byGUID[e.GUID] = systemq.PodcastListEpisodesRow{ID: adopted.ID, Guid: nullStr(e.GUID), Title: title, SortTitle: sort, LockedFields: "[]",
			PublishedAt: published}
		// Not a new episode: the household has the file already, so no announcement and no cover job.
		return episodeChange{}, upsertTexts(ctx, q, adopted.ID, title, e.Description, nil)
	}
	id := ulid.Make().String()
	if err := q.PodcastInsertEpisode(ctx, systemq.PodcastInsertEpisodeParams{
		ID: id, ParentID: w.parent, LibraryID: w.libraryID, Title: title, SortTitle: sort,
		Guid: nullStr(e.GUID), PublishedAt: published, DurationMs: positive(e.Duration.Milliseconds()),
		EnclosureUrl: nullStr(enclosure), EnclosureType: nullStr(e.Enclosure.Type), EnclosureLength: positive(e.Enclosure.Length),
		Explicit: boolInt(explicit), CoverUrl: cover, SeasonNumber: positive(int64(e.Season)), EpisodeNumber: positive(int64(e.Number)),
		OwnRating: rating, RatingSource: source, Now: w.now,
	}); err != nil {
		return episodeChange{}, fmt.Errorf("insert episode: %w", err)
	}
	w.byGUID[e.GUID] = systemq.PodcastListEpisodesRow{ID: id, Guid: nullStr(e.GUID), Title: title, SortTitle: sort, LockedFields: "[]",
		PublishedAt: published}
	c := episodeChange{newID: id, published: published}
	if cover.Valid {
		c.newCover = id
	}
	return c, upsertTexts(ctx, q, id, title, e.Description, nil)
}

// adopt finds the one file episode a new feed episode stands for (concept 4.5): the same
// publication minute, else the same sort title; the duration within two percent when both are
// known. Exactly one candidate, or none: two files that fit stay apart rather than one of them
// being married to the wrong feed episode. A taken candidate is removed from the list.
func (w *episodeWrite) adopt(e feed.Episode, published sql.NullInt64) (systemq.PodcastListFileEpisodesWithoutGUIDRow, bool) {
	var hits []int
	for i, f := range w.files {
		if !PairsEpisode(published, sortTitle(e.Title), e.Duration.Milliseconds(), f.PublishedAt, f.SortTitle, f.DurationMs) {
			continue
		}
		hits = append(hits, i)
	}
	if len(hits) != 1 {
		return systemq.PodcastListFileEpisodesWithoutGUIDRow{}, false
	}
	row := w.files[hits[0]]
	w.files = slices.Delete(w.files, hits[0], hits[0]+1)
	return row, true
}

// PairsEpisode reports whether a feed episode and a file episode are the same episode: the
// publication minute equal, else the sort titles equal; a known duration on both sides must agree
// within two percent. The scanner's probe step uses the same rule the other way round.
func PairsEpisode(feedPublished sql.NullInt64, feedSortTitle string, feedDurationMs int64,
	filePublished sql.NullInt64, fileSortTitle string, fileDurationMs sql.NullInt64,
) bool {
	sameMinute := feedPublished.Valid && filePublished.Valid && feedPublished.Int64/60000 == filePublished.Int64/60000
	sameTitle := feedSortTitle != "" && feedSortTitle == fileSortTitle
	if !sameMinute && !sameTitle {
		return false
	}
	if feedDurationMs > 0 && fileDurationMs.Valid && fileDurationMs.Int64 > 0 {
		diff := feedDurationMs - fileDurationMs.Int64
		if diff < 0 {
			diff = -diff
		}
		if diff*50 > feedDurationMs {
			return false
		}
	}
	return true
}

// upsertTexts writes title and description for both languages (a feed has one language; the
// profile language picks the row, so both carry the feed's text). A locked title or overview keeps
// the stored text.
func upsertTexts(ctx context.Context, q *systemq.Queries, itemID, title, description string, locked map[string]bool) error {
	stored := map[string]systemq.ItemText{}
	if locked["title"] || locked["overview"] {
		rows, err := q.ListItemTextsByItem(ctx, itemID)
		if err != nil {
			return fmt.Errorf("list texts of %s: %w", itemID, err)
		}
		for _, r := range rows {
			stored[r.Lang] = r
		}
	}
	for _, lang := range []string{"de", "en"} {
		p := systemq.UpsertItemTextParams{ItemID: itemID, Lang: lang, Title: title, Overview: nullStr(description)}
		if old, ok := stored[lang]; ok {
			p.Tagline = old.Tagline
			if locked["title"] {
				p.Title = old.Title
			}
			if locked["overview"] {
				p.Overview = old.Overview
			}
		}
		p.SortKey = sortKey(lang, p.Title)
		if err := q.UpsertItemText(ctx, p); err != nil {
			return fmt.Errorf("store texts of %s: %w", itemID, err)
		}
	}
	return nil
}

func (s *Service) inTx(ctx context.Context, fn func(q *systemq.Queries) error) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(systemq.New(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// adminPodcast reads one live subscription; model.ErrNotFound otherwise.
func (s *Service) adminPodcast(ctx context.Context, id string) (model.AdminPodcast, error) {
	rows, err := s.write.PodcastList(ctx, id)
	if err != nil {
		return model.AdminPodcast{}, fmt.Errorf("get podcast: %w", err)
	}
	if len(rows) != 1 {
		return model.AdminPodcast{}, model.ErrNotFound
	}
	return toAdminPodcast(rows[0]), nil
}

func toAdminPodcast(r systemq.PodcastListRow) model.AdminPodcast {
	out := model.AdminPodcast{
		ID: r.ID, LibraryID: r.LibraryID, Title: r.Title, Author: r.Author.String, FeedURL: r.FeedUrl.String,
		EpisodeCount: int(r.EpisodeCount), Explicit: r.Explicit == 1, LastError: r.FeedError.String,
		CreatedAt: time.UnixMilli(r.CreatedAt).UTC(),
	}
	if r.FeedCheckedAt.Valid {
		t := time.UnixMilli(r.FeedCheckedAt.Int64).UTC()
		out.LastCheckedAt = &t
	}
	out.FeedOrigin, out.LocalEpisodeCount = r.FeedOrigin.String, int(r.LocalEpisodeCount)
	if r.RootID != 0 {
		id := r.RootID
		out.RootID = &id
	}
	return out
}

// channelRating is the rating of a podcast from its feed: FSK 18 (feed) when the channel is
// explicit, else the library default (library; 0 when the library has none).
func channelRating(explicit bool, libraryDefault sql.NullInt64) (sql.NullInt64, sql.NullString) {
	if explicit {
		return sql.NullInt64{Int64: 18, Valid: true}, nullStr("feed")
	}
	return sql.NullInt64{Int64: libraryDefault.Int64, Valid: true}, nullStr("library")
}

// episodeRating is the rating of an episode from its feed: FSK 18 (feed) when explicit, else none
// (the podcast's rating applies through the triggers).
func episodeRating(explicit bool) (sql.NullInt64, sql.NullString) {
	if explicit {
		return sql.NullInt64{Int64: 18, Valid: true}, nullStr("feed")
	}
	return sql.NullInt64{}, sql.NullString{}
}

// httpURL keeps an http or https URL and drops anything else (images and enclosures).
func httpURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// newEpisodesTimeout bounds the new-episode hook of one fetch. The hook runs detached from the
// fetch's context (the episodes are committed, so a shutdown should still announce them), but not
// for ever.
const newEpisodesTimeout = 30 * time.Second

// announceEpisodes tells the new-episode hook about inserted episodes (M7 E-8); a failure is
// logged, the episodes stay stored.
func (s *Service) announceEpisodes(ctx context.Context, ids []string) {
	if s.newEpisodes == nil || len(ids) == 0 {
		return
	}
	hookCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), newEpisodesTimeout)
	defer cancel()
	if err := s.newEpisodes(hookCtx, ids); err != nil {
		s.log.WarnContext(ctx, "notify new podcast episodes", "episodes", len(ids), "err", err)
	}
}

func (s *Service) recheckVisibility(ctx context.Context) {
	if s.recheck == nil {
		return
	}
	if err := s.recheck(ctx); err != nil {
		s.log.WarnContext(ctx, "recheck open streams after a podcast change", "err", err)
	}
}

// sortArticles are the leading articles sortTitle drops (as library.SortTitle; metadata cannot
// import library, whose tests import metadata).
var sortArticles = []string{"the ", "a ", "an ", "der ", "die ", "das ", "ein ", "eine "}

// sortTitle lower-cases a title and drops a leading English or German article.
func sortTitle(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, a := range sortArticles {
		if strings.HasPrefix(t, a) && len(t) > len(a) {
			return strings.TrimSpace(t[len(a):])
		}
	}
	return t
}

func nullStr(v string) sql.NullString { return sql.NullString{String: v, Valid: v != ""} }

func positive(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: v > 0} }
