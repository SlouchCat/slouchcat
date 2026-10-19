package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Trailer chain (plan 5.3, design 3.2): trailer.find {itemId}, key trailer:<itemId>, priority 1,
// for movies and series with a tmdb_id. The steps run in order, each only for the languages the
// earlier ones missed:
//
//  1. TMDB videos (/movie|tv/{id}/videos in de-DE and en-US, video languages de, en and none):
//     type Trailer on YouTube, official first, then the newest, one per video language.
//  2. YouTube Data API v3 for de and en (youtube.go), within the daily quota. An exhausted quota
//     leaves the title pending; QueuePendingTrailers queues it again after the next scan.
//  3. An AITrailerProvider; M3 ships only the nop implementation (SLOUCHCAT_TRAILER_AI=off).
//
// The job replaces the automatic rows of the title and keeps the manual ones; trailer_state is
// found (any row), none, failed (TMDB rejected the key) or stays pending (quota).
const (
	JobTrailerFind  = "trailer.find"
	trailerPriority = 1

	trailerPending = "pending"
	trailerFound   = "found"
	trailerNone    = "none"
	trailerFailed  = "failed"

	sourceTMDB    = "tmdb"
	sourceYouTube = "youtube"
	sourceAI      = "ai"
	sourceManual  = "manual"
)

// trailerLangs are the languages the trailer chain looks for.
var trailerLangs = []string{"de", "en"}

// TrailerQuery describes the title a TrailerProvider looks for.
type TrailerQuery struct {
	Kind          string // movie or series
	TMDBID        int64
	Titles        map[string]string // per language
	OriginalTitle string
	Year          int
	Langs         []string // languages still without a trailer
}

// TrailerHit is one trailer found for a language.
type TrailerHit struct {
	Lang      string
	YouTubeID string
	Title     string
	Source    string // tmdb, youtube, ai
	Official  bool
}

// TrailerProvider finds at most one trailer per language of q.Langs.
type TrailerProvider interface {
	Name() string
	FindTrailers(ctx context.Context, q TrailerQuery) ([]TrailerHit, error)
}

// AITrailerProvider is the last step of the chain: a language model with web search that names
// trailers the other providers missed. M3 has no real implementation (SLOUCHCAT_TRAILER_AI=off).
type AITrailerProvider interface {
	TrailerProvider
}

// nopAITrailers is the AITrailerProvider of M3: it finds nothing.
type nopAITrailers struct{}

func (nopAITrailers) Name() string { return sourceAI }

func (nopAITrailers) FindTrailers(context.Context, TrailerQuery) ([]TrailerHit, error) {
	return nil, nil
}

// newAITrailers returns the AI step for the SLOUCHCAT_TRAILER_AI value: "off" (or empty) is the nop
// provider; any other value is not available in M3, logged once and treated as off.
func newAITrailers(mode string, log *slog.Logger) AITrailerProvider {
	if mode != "" && mode != "off" {
		log.Warn("trailer AI provider not available, using off", "trailer_ai", mode)
	}
	return nopAITrailers{}
}

// tmdbTrailers is the TMDB step of the chain.
type tmdbTrailers struct {
	client *Client
	keys   KeySource
}

func (p *tmdbTrailers) Name() string { return sourceTMDB }

func (p *tmdbTrailers) FindTrailers(ctx context.Context, q TrailerQuery) ([]TrailerHit, error) {
	key, err := p.keys.TMDBKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("read tmdb key: %w", err)
	}
	if key == "" {
		return nil, errNotConfigured
	}
	c := p.client.WithKey(key)
	kind := "movie"
	if q.Kind == "series" {
		kind = "tv"
	}
	seen := map[string]bool{}
	var videos []Video
	for _, l := range trailerLangs {
		vs, err := c.Videos(ctx, kind, q.TMDBID, tmdbLang(l))
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		for _, v := range vs {
			if !seen[v.Key] {
				seen[v.Key] = true
				videos = append(videos, v)
			}
		}
	}
	return pickTMDBTrailers(videos, q.Langs), nil
}

// pickTMDBTrailers keeps YouTube trailers in the wanted languages and picks one per language:
// official first, then the newest.
func pickTMDBTrailers(videos []Video, langs []string) []TrailerHit {
	want := map[string]bool{}
	for _, l := range langs {
		want[l] = true
	}
	var keep []Video
	for _, v := range videos {
		lang := strings.ToLower(v.Lang)
		if v.Type == "Trailer" && v.Site == "YouTube" && want[lang] && youtubeIDPattern.MatchString(v.Key) {
			v.Lang = lang
			keep = append(keep, v)
		}
	}
	sort.SliceStable(keep, func(i, j int) bool {
		if keep[i].Official != keep[j].Official {
			return keep[i].Official
		}
		return keep[i].PublishedAt > keep[j].PublishedAt // RFC 3339 in UTC sorts as text
	})
	var out []TrailerHit
	done := map[string]bool{}
	for _, v := range keep {
		if done[v.Lang] {
			continue
		}
		done[v.Lang] = true
		out = append(out, TrailerHit{Lang: v.Lang, YouTubeID: v.Key, Title: v.Name, Source: sourceTMDB, Official: v.Official})
	}
	return out
}

func (s *Service) handleTrailerFind(ctx context.Context, j *jobs.Job) error {
	itemID, err := parsePayload(j)
	if err != nil {
		return err
	}
	return s.findTrailers(ctx, itemID)
}

// findTrailers runs the trailer chain for one title.
func (s *Service) findTrailers(ctx context.Context, itemID string) error {
	item, err := s.read.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("trailer.find: load item %s: %w", itemID, err)
	}
	if item.DeletedAt.Valid || (item.Kind != "movie" && item.Kind != "series") || !item.TmdbID.Valid ||
		lockedFields(item.LockedFields)["trailer"] {
		return nil
	}
	texts, err := s.read.ListItemTextsByItem(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("trailer.find: load texts %s: %w", item.ID, err)
	}
	q := TrailerQuery{Kind: item.Kind, TMDBID: item.TmdbID.Int64, Titles: map[string]string{}, OriginalTitle: item.OriginalTitle.String}
	for _, t := range texts {
		q.Titles[t.Lang] = t.Title
	}
	if q.Titles["en"] == "" {
		q.Titles["en"] = item.Title
	}
	if item.Year.Valid {
		q.Year = int(item.Year.Int64)
	}

	var hits []TrailerHit
	missing := func() []string {
		var out []string
		for _, l := range trailerLangs {
			if !hasLang(hits, l) {
				out = append(out, l)
			}
		}
		return out
	}
	q.Langs = trailerLangs
	found, err := s.trailersTMDB.FindTrailers(ctx, q)
	switch {
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		s.log.Warn("tmdb key rejected", "item_id", item.ID)
		return s.setTrailerState(ctx, item.ID, trailerFailed)
	case errors.Is(err, errNotConfigured):
	case err != nil:
		return fmt.Errorf("trailer.find %s: %w", item.ID, err)
	}
	hits = append(hits, found...)

	quotaBlocked := false
	if q.Langs = missing(); len(q.Langs) > 0 {
		found, err := s.trailersYouTube.FindTrailers(ctx, q)
		hits = append(hits, found...)
		switch {
		case errors.Is(err, errQuotaExhausted):
			quotaBlocked = true
			s.log.Info("youtube quota exhausted, trailer search postponed", "item_id", item.ID)
		case errors.Is(err, errYouTubeKeyInvalid):
			s.log.Warn("youtube key rejected, trailer search skipped", "item_id", item.ID)
		case errors.Is(err, errNotConfigured):
		case err != nil:
			return fmt.Errorf("trailer.find %s: %w", item.ID, err)
		}
	}
	if q.Langs = missing(); len(q.Langs) > 0 && !quotaBlocked {
		found, err := s.trailersAI.FindTrailers(ctx, q)
		if err != nil {
			return fmt.Errorf("trailer.find %s: %w", item.ID, err)
		}
		hits = append(hits, found...)
	}
	return s.storeTrailers(ctx, item.ID, hits, quotaBlocked && len(missing()) > 0)
}

func hasLang(hits []TrailerHit, lang string) bool {
	for _, h := range hits {
		if h.Lang == lang {
			return true
		}
	}
	return false
}

// storeTrailers replaces the automatic trailers of the item with hits in one transaction and sets
// the state: pending when the quota blocked a language, else found or none.
func (s *Service) storeTrailers(ctx context.Context, itemID string, hits []TrailerHit, pending bool) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("trailer.find: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	now := s.now().UnixMilli()
	if err := q.DeleteAutoTrailers(ctx, itemID); err != nil {
		return fmt.Errorf("trailer.find: delete trailers of %s: %w", itemID, err)
	}
	for _, h := range hits {
		if err := q.InsertTrailer(ctx, systemq.InsertTrailerParams{
			ItemID: itemID, Lang: h.Lang, YoutubeID: h.YouTubeID, Title: nullString(h.Title), Source: h.Source,
			Official: boolInt(h.Official), Now: now,
		}); err != nil {
			return fmt.Errorf("trailer.find: insert trailer of %s: %w", itemID, err)
		}
	}
	n, err := q.CountTrailers(ctx, itemID)
	if err != nil {
		return fmt.Errorf("trailer.find: count trailers of %s: %w", itemID, err)
	}
	state := trailerNone
	switch {
	case pending:
		state = trailerPending
	case n > 0:
		state = trailerFound
	}
	if err := q.SetTrailerState(ctx, systemq.SetTrailerStateParams{
		TrailerState: state, CheckedAt: sql.NullInt64{Int64: now, Valid: true}, Now: now, ID: itemID,
	}); err != nil {
		return fmt.Errorf("trailer.find: state of %s: %w", itemID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("trailer.find: commit %s: %w", itemID, err)
	}
	s.log.Info("trailers checked", "item_id", itemID, "found", len(hits), "trailer_state", state)
	return nil
}

func (s *Service) setTrailerState(ctx context.Context, itemID, state string) error {
	now := s.now().UnixMilli()
	if err := s.write.SetTrailerState(ctx, systemq.SetTrailerStateParams{
		TrailerState: state, CheckedAt: sql.NullInt64{Int64: now, Valid: true}, Now: now, ID: itemID,
	}); err != nil {
		return fmt.Errorf("trailer.find: state of %s: %w", itemID, err)
	}
	return nil
}

func (s *Service) enqueueTrailers(ctx context.Context, itemID string) error {
	return s.enqueueJob(ctx, JobTrailerFind, "trailer:", itemID, trailerPriority)
}

// QueuePendingTrailers enqueues trailer.find for every matched movie and series of the library with
// a tmdb_id whose trailer_state is pending and that has no active trailer job (for example after
// the YouTube quota ran out). It returns the number of jobs queued.
func (s *Service) QueuePendingTrailers(ctx context.Context, libraryID int64) (int, error) {
	ids, err := s.read.ListPendingTrailerItems(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list pending trailers: %w", err)
	}
	for i, id := range ids {
		if err := s.enqueueTrailers(ctx, id); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}

// youtubeIDPattern is the shape of a YouTube video ID (trailers.youtube_id, 8 to 16 characters).
var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,16}$`)

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
