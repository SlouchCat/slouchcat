package metadata

import (
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
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Podcast subscriptions (M6, design 3.4, H-2, AN-2). A subscription is an item of kind podcast in a
// library of kind podcasts with its feed URL; its episodes are podcast_episode items below it, one
// per guid, played and downloaded from their enclosure. Subscribe fetches the feed once right away;
// the job podcast.fetch fetches it again (conditional GET with ETag and Last-Modified), and
// RunPodcasts enqueues that job for every subscription not fetched for FeedInterval.
//
// Ratings (AN-2): the podcast starts with the library's default rating (rating_source library),
// an explicit channel is FSK 18 (rating_source feed); an explicit episode is FSK 18 (feed), any
// other episode inherits the podcast's rating. Manual ratings and locked fields stay.

const (
	// JobPodcastFetch fetches the feed of one subscription; payload {"podcastId"}.
	JobPodcastFetch = "podcast.fetch"
	// FeedInterval is the age after which the poller fetches a feed again (H-2: a new episode
	// appears in less than 65 minutes).
	FeedInterval = 60 * time.Minute

	podcastFetchPriority = 4
	feedPollInterval     = 5 * time.Minute
	feedTimeout          = 30 * time.Second // whole request of a fetch
	subscribeTimeout     = 20 * time.Second // the synchronous first fetch (createPodcast)
	maxFeedBytes         = 16 << 20
	maxFeedRedirects     = 5
	maxFeedURL           = 2048
	maxFeedEpisodes      = 2000 // the newest episodes of a feed that are stored
	maxFeedError         = 200  // characters of items.feed_error
	feedBatch            = 500  // episodes per transaction (plan 3.6)
)

// Errors of a fetch.
var (
	errFeedTooLarge = errors.New("feed larger than 16 MiB")
	errFeedRedirect = errors.New("feed redirects too often or to another scheme")
)

// errFeedStatus is an HTTP status other than 200 and 304.
type errFeedStatus int

func (e errFeedStatus) Error() string { return fmt.Sprintf("feed answered HTTP %d", int(e)) }

// feedFetcher fetches feeds: one client (30 s, at most 5 redirects, http and https only), one
// limiter for every feed host, the clock of the poller and the User-Agent.
type feedFetcher struct {
	client  *http.Client
	limiter *ratelimit.Limiter
	now     func() time.Time
	agent   string
}

func newFeedFetcher(c *http.Client, now func() time.Time, agent string) feedFetcher {
	client := &http.Client{Timeout: feedTimeout}
	if c != nil {
		cp := *c
		client = &cp
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxFeedRedirects || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
			return errFeedRedirect
		}
		return nil
	}
	if now == nil {
		now = time.Now
	}
	return feedFetcher{client: client, limiter: ratelimit.New("feeds", 2, 4), now: now, agent: cmp.Or(agent, "Slouchcat")}
}

// WithFeedHTTPClient replaces the HTTP client of the feed fetches (tests); the redirect policy is
// always the service's own.
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

// fetch gets and parses a feed; etag and modified make it conditional (304 → notModified).
func (f feedFetcher) fetch(ctx context.Context, feedURL, etag, modified string) (fetched, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return fetched{}, err
	}
	req.Header.Set("User-Agent", f.agent)
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.5")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if modified != "" {
		req.Header.Set("If-Modified-Since", modified)
	}
	resp, err := f.limiter.Do(ctx, f.client, req)
	if err != nil {
		return fetched{}, err
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
		return fetched{}, err
	}
	if len(body) > maxFeedBytes {
		return fetched{}, errFeedTooLarge
	}
	parsed, err := feed.Parse(strings.NewReader(string(body)))
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
// URL has none, validateFeedURL).
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
	res, err := s.feeds.fetch(fctx, clean, "", "")
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
		FeedUrl: nullStr(clean), FeedEtag: nullStr(res.etag), FeedModified: nullStr(res.modified),
		Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, OwnRating: rating, RatingSource: source,
	})
	if err != nil {
		return model.AdminPodcast{}, fmt.Errorf("insert podcast: %w", err)
	}
	if err := s.storeEpisodes(ctx, p.ID, libraryID, title, res.feed, now); err != nil {
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
	if err := s.enqueuePodcast(ctx, id); err != nil {
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
		if err := s.enqueuePodcast(ctx, id); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}

// RunPodcasts enqueues podcast.fetch for every subscription whose last fetch is FeedInterval old,
// once at the start and then every 5 minutes, until ctx ends (H-2).
func (s *Service) RunPodcasts(ctx context.Context) {
	t := time.NewTicker(feedPollInterval)
	defer t.Stop()
	for {
		if n, err := s.QueueDuePodcasts(ctx); err != nil && ctx.Err() == nil {
			s.log.WarnContext(ctx, "queue due podcasts", "queued", n, "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// QueueDuePodcasts enqueues podcast.fetch for every subscription not fetched for FeedInterval and
// returns how many it queued (one tick of RunPodcasts).
func (s *Service) QueueDuePodcasts(ctx context.Context) (int, error) {
	before := s.feeds.now().Add(-FeedInterval).UnixMilli()
	ids, err := s.read.PodcastListDue(ctx, sql.NullInt64{Int64: before, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("list due podcasts: %w", err)
	}
	for i, id := range ids {
		if err := s.enqueuePodcast(ctx, id); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}

// podcastPayload is the payload of podcast.fetch.
type podcastPayload struct {
	PodcastID string `json:"podcastId"`
}

func (s *Service) enqueuePodcast(ctx context.Context, id string) error {
	payload, err := json.Marshal(podcastPayload{PodcastID: id})
	if err != nil {
		return fmt.Errorf("%s payload: %w", JobPodcastFetch, err)
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobPodcastFetch, Priority: podcastFetchPriority,
		IdempotencyKey: "podcast:" + id, Payload: payload}); err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobPodcastFetch, id, err)
	}
	return nil
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
	res, err := s.feeds.fetch(ctx, p.FeedUrl.String, p.FeedEtag.String, p.FeedModified.String)
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
	if err := s.write.PodcastUpdateChannel(ctx, systemq.PodcastUpdateChannelParams{
		Title: title, SortTitle: sortTitle(title), Author: nullStr(res.feed.Author), Explicit: boolInt(res.feed.Explicit),
		CoverUrl: cover, OwnRating: rating, RatingSource: source, FeedEtag: nullStr(res.etag), FeedModified: nullStr(res.modified),
		Now: checked, ID: id,
	}); err != nil {
		return fmt.Errorf("update podcast %s: %w", id, err)
	}
	if err := s.storeEpisodes(ctx, id, p.LibraryID, title, res.feed, now); err != nil {
		return err
	}
	if cover.String != p.CoverUrl.String && cover.Valid {
		if err := s.enqueueImages(ctx, id, imageFetchPriority); err != nil {
			s.log.WarnContext(ctx, "queue podcast cover", "item_id", id, "err", err)
		}
	}
	if rating != p.OwnRating {
		s.recheckVisibility(ctx)
	}
	return nil
}

// storeEpisodes writes the channel texts and the newest episodes of a feed below a podcast, in
// transactions of 500 episodes: new guids are inserted, known ones updated (titles, texts,
// enclosure, date, duration, the explicit rating unless manual or locked).
func (s *Service) storeEpisodes(ctx context.Context, podcastID string, libraryID int64, title string, f feed.Feed, now time.Time) error {
	eps := newestEpisodes(f.Episodes, maxFeedEpisodes)
	parent := sql.NullString{String: podcastID, Valid: true}
	known, err := s.write.PodcastListEpisodes(ctx, parent)
	if err != nil {
		return fmt.Errorf("list episodes of %s: %w", podcastID, err)
	}
	byGUID := make(map[string]systemq.PodcastListEpisodesRow, len(known))
	for _, k := range known {
		byGUID[k.Guid.String] = k
	}
	if err := upsertTexts(ctx, s.write, podcastID, title, f.Description); err != nil {
		return err
	}
	ms := now.UnixMilli()
	nowN := sql.NullInt64{Int64: ms, Valid: true}
	for chunk := range slices.Chunk(eps, feedBatch) {
		err := s.inTx(ctx, func(q *systemq.Queries) error {
			for _, e := range chunk {
				if err := storeEpisode(ctx, q, parent, libraryID, e, byGUID, ms, nowN); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("store episodes of %s: %w", podcastID, err)
		}
	}
	return nil
}

// newestEpisodes returns at most limit episodes, newest first (undated ones last, in feed order).
func newestEpisodes(eps []feed.Episode, limit int) []feed.Episode {
	out := slices.Clone(eps)
	slices.SortStableFunc(out, func(a, b feed.Episode) int { return b.PublishedAt.Compare(a.PublishedAt) })
	return out[:min(len(out), limit)]
}

// storeEpisode inserts or updates one episode inside the caller's transaction.
func storeEpisode(ctx context.Context, q *systemq.Queries, parent sql.NullString, libraryID int64, e feed.Episode,
	byGUID map[string]systemq.PodcastListEpisodesRow, ms int64, now sql.NullInt64,
) error {
	enclosure := httpURL(e.Enclosure.URL)
	if enclosure == "" {
		return nil // only http and https enclosures are played and downloaded
	}
	title := cmp.Or(strings.TrimSpace(e.Title), e.GUID)
	var published sql.NullInt64
	if !e.PublishedAt.IsZero() {
		published = sql.NullInt64{Int64: e.PublishedAt.UnixMilli(), Valid: true}
	}
	explicit := e.Explicit != nil && *e.Explicit
	rating, source := episodeRating(explicit)
	cover := nullStr(httpURL(e.ImageURL))
	known, ok := byGUID[e.GUID]
	if ok {
		if known.DeletedAt.Valid {
			return nil
		}
		locked := lockedFields(known.LockedFields)
		if locked["own_rating"] || known.RatingSource.String == "manual" {
			rating, source = known.OwnRating, known.RatingSource
		}
		if err := q.PodcastUpdateEpisode(ctx, systemq.PodcastUpdateEpisodeParams{
			Title: title, SortTitle: sortTitle(title), PublishedAt: published, DurationMs: positive(e.Duration.Milliseconds()),
			EnclosureUrl: nullStr(enclosure), EnclosureType: nullStr(e.Enclosure.Type), EnclosureLength: positive(e.Enclosure.Length),
			Explicit: boolInt(explicit), CoverUrl: cover, SeasonNumber: positive(int64(e.Season)), EpisodeNumber: positive(int64(e.Number)),
			OwnRating: rating, RatingSource: source, Now: ms, ID: known.ID,
		}); err != nil {
			return fmt.Errorf("update episode: %w", err)
		}
		if locked["title"] {
			return nil
		}
		return upsertTexts(ctx, q, known.ID, title, e.Description)
	}
	id := ulid.Make().String()
	if err := q.PodcastInsertEpisode(ctx, systemq.PodcastInsertEpisodeParams{
		ID: id, ParentID: parent, LibraryID: libraryID, Title: title, SortTitle: sortTitle(title),
		Guid: nullStr(e.GUID), PublishedAt: published, DurationMs: positive(e.Duration.Milliseconds()),
		EnclosureUrl: nullStr(enclosure), EnclosureType: nullStr(e.Enclosure.Type), EnclosureLength: positive(e.Enclosure.Length),
		Explicit: boolInt(explicit), CoverUrl: cover, SeasonNumber: positive(int64(e.Season)), EpisodeNumber: positive(int64(e.Number)),
		OwnRating: rating, RatingSource: source, Now: now,
	}); err != nil {
		return fmt.Errorf("insert episode: %w", err)
	}
	byGUID[e.GUID] = systemq.PodcastListEpisodesRow{ID: id, Guid: nullStr(e.GUID), LockedFields: "[]"}
	return upsertTexts(ctx, q, id, title, e.Description)
}

// upsertTexts writes title and description for both languages (a feed has one language; the
// profile language picks the row, so both carry the feed's text).
func upsertTexts(ctx context.Context, q *systemq.Queries, itemID, title, description string) error {
	for _, lang := range []string{"de", "en"} {
		if err := q.UpsertItemText(ctx, systemq.UpsertItemTextParams{ItemID: itemID, Lang: lang, Title: title, Overview: nullStr(description)}); err != nil {
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
