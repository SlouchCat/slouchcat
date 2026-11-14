package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/youtubeid"
)

// Trailer chain (plan 5.3, design 3.2): trailer.find {itemId}, key trailer:<itemId>, priority 1,
// for movies and series with a tmdb_id. The steps run in order, each only for the languages the
// earlier ones missed:
//
//  1. TMDB videos (/movie|tv/{id}/videos, one request per trailer language, video languages the
//     trailer languages of the library and none; concept 21 P-4): type Trailer on YouTube, one per
//     video language, a trailer of a country the language list names first (pt-BR: BR), then by
//     the rank of resolution and «official» (tmdbTrailerRank, concept 21 P-1), the newest on a tie.
//  2. YouTube Data API v3 for the primary subtags of the effective metadata language list (E-6,
//     ADR 0018; youtube.go), within the daily quota. An exhausted quota
//     leaves the title pending; QueuePendingTrailers queues it again after the next scan.
//  3. An AITrailerProvider; M3 ships only the nop implementation (SLOUCHCAT_TRAILER_AI=off).
//  4. The best TMDB trailer without language for every language still missing (concept 21 O-7:
//     the last rank, after YouTube), unless the YouTube quota blocked a language.
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

// trailerLangsOf returns the languages the trailer chain looks for: the primary subtags of the
// effective text language list of the title's library (pt-BR -> pt; trailers.lang holds two or
// three letters). The chain of ADR 0006 is unchanged, only the list is no longer fixed to de, en.
// regions holds, per primary subtag, the countries the list names for it in order (pt-BR, pt-PT
// -> pt: BR, PT), whose trailers go first (concept 21 O-8).
func (s *Service) trailerLangsOf(ctx context.Context, item systemq.Item) (langs []string, regions map[string][]string, err error) {
	textLangs, err := s.textLangsOf(ctx, item)
	if err != nil {
		return nil, nil, err
	}
	return primaryLanguages(textLangs), trailerRegions(textLangs), nil
}

// trailerRegions maps each primary subtag of langs to the countries its region variants name, in
// order (upper case, as TMDB's iso_3166_1).
func trailerRegions(langs []string) map[string][]string {
	out := map[string][]string{}
	for _, l := range langs {
		primary, region, ok := strings.Cut(l, "-")
		if !ok || region == "" {
			continue
		}
		if region = strings.ToUpper(region); !slices.Contains(out[primary], region) {
			out[primary] = append(out[primary], region)
		}
	}
	return out
}

// TrailerQuery describes the title a TrailerProvider looks for.
type TrailerQuery struct {
	Kind          string // movie or series
	TMDBID        int64
	Titles        map[string]string // per language
	OriginalTitle string
	Year          int
	Langs         []string            // languages still without a trailer
	Regions       map[string][]string // preferred countries per language (trailerRegions)
}

// TrailerHit is one trailer found for a language. The TMDB step also returns its best trailer
// without language with Lang "", the fallback of the chain's last step.
type TrailerHit struct {
	Lang      string
	YouTubeID string
	Title     string
	Source    string // tmdb, youtube, ai
	Official  bool
	Size      int // TMDB resolution, 0 when unknown (YouTube says only hd or sd)
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
	for _, l := range q.Langs {
		vs, err := c.Videos(ctx, kind, q.TMDBID, tmdbLang(l), q.Langs)
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
	hits := pickTMDBTrailers(videos, q.Langs, q.Regions)
	if h, ok := pickLanguagelessTrailer(videos); ok {
		hits = append(hits, h)
	}
	return hits, nil
}

// tmdbTrailerRank is the rank of a TMDB trailer in the table of concept 21 section 2.2 (Marco,
// 12 November 2026), 1 the best: from 720 up «official» stays in front (2160, 1080, 720 official,
// then the same unofficial); below 720 an official trailer loses to every one from 720 up (480
// official, 360 or unknown official, 480 unofficial, 360 or unknown unofficial). Ranks 11 and 12
// are the YouTube search (hd, sd; pickYouTubeVideo). TMDB names 360, 480, 720, 1080 and 2160; a
// value in between counts as the next lower one.
func tmdbTrailerRank(size int, official bool) int {
	var rank int
	switch {
	case size >= 2160:
		rank = 1
	case size >= 1080:
		rank = 2
	case size >= 720:
		rank = 3
	case size >= 480:
		if official {
			return 7
		}
		return 9
	default:
		if official {
			return 8
		}
		return 10
	}
	if !official {
		rank += 3
	}
	return rank
}

// pickTMDBTrailers keeps YouTube trailers in the wanted languages and picks one per language: a
// trailer of a country regions names for the language first, in the order of the list (concept 21
// O-8), then by tmdbTrailerRank, the newest on a tie.
func pickTMDBTrailers(videos []Video, langs []string, regions map[string][]string) []TrailerHit {
	want := map[string]bool{}
	for _, l := range langs {
		want[l] = true
	}
	var keep []Video
	for _, v := range videos {
		lang := strings.ToLower(v.Lang)
		if isYouTubeTrailer(v) && want[lang] {
			v.Lang = lang
			keep = append(keep, v)
		}
	}
	country := func(v Video) int {
		list := regions[v.Lang]
		if i := slices.Index(list, strings.ToUpper(v.Country)); i >= 0 {
			return i
		}
		return len(list)
	}
	sort.SliceStable(keep, func(i, j int) bool {
		if ci, cj := country(keep[i]), country(keep[j]); ci != cj {
			return ci < cj
		}
		ri, rj := tmdbTrailerRank(keep[i].Size, keep[i].Official), tmdbTrailerRank(keep[j].Size, keep[j].Official)
		if ri != rj {
			return ri < rj
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
		out = append(out, TrailerHit{Lang: v.Lang, YouTubeID: v.Key, Title: v.Name, Source: sourceTMDB, Official: v.Official, Size: v.Size})
	}
	return out
}

func isYouTubeTrailer(v Video) bool {
	return v.Type == "Trailer" && v.Site == "YouTube" && youtubeid.Valid(v.Key)
}

// pickLanguagelessTrailer picks the best TMDB trailer without language (iso_639_1 null) by
// tmdbTrailerRank, the newest on a tie; Lang stays "" (concept 21 O-7).
func pickLanguagelessTrailer(videos []Video) (TrailerHit, bool) {
	var best *Video
	for i := range videos {
		v := &videos[i]
		if v.Lang != "" || !isYouTubeTrailer(*v) {
			continue
		}
		if best == nil {
			best = v
			continue
		}
		rv, rb := tmdbTrailerRank(v.Size, v.Official), tmdbTrailerRank(best.Size, best.Official)
		if rv < rb || (rv == rb && v.PublishedAt > best.PublishedAt) {
			best = v
		}
	}
	if best == nil {
		return TrailerHit{}, false
	}
	return TrailerHit{YouTubeID: best.Key, Title: best.Name, Source: sourceTMDB, Official: best.Official, Size: best.Size}, true
}

func (s *Service) handleTrailerFind(ctx context.Context, j *jobs.Job) error {
	p, err := parseItemPayload(j)
	if err != nil {
		return err
	}
	return s.trailerChain(ctx, p.ItemID, p.Recheck)
}

// findTrailers runs the trailer chain for one title.
func (s *Service) findTrailers(ctx context.Context, itemID string) error {
	return s.trailerChain(ctx, itemID, false)
}

// recheckTrailers picks the trailers of a title with found trailers again by resolution, once
// after the update to concept 21 stage 2 (O-3): the TMDB step only, without a YouTube search, so
// the run spends no quota. A language TMDB has nothing for keeps its automatic rows (a YouTube hit
// of an earlier run), then the language-less fallback applies; the manual rows stay as always. A
// rejected or missing TMDB key leaves the title as it is.
func (s *Service) recheckTrailers(ctx context.Context, itemID string) error {
	return s.trailerChain(ctx, itemID, true)
}

func (s *Service) trailerChain(ctx context.Context, itemID string, recheck bool) error {
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

	langs, regions, err := s.trailerLangsOf(ctx, item)
	if err != nil {
		return fmt.Errorf("trailer.find %s: %w", item.ID, err)
	}
	q.Regions = regions
	var hits []TrailerHit
	missing := func() []string {
		var out []string
		for _, l := range langs {
			if !hasLang(hits, l) {
				out = append(out, l)
			}
		}
		return out
	}
	q.Langs = langs
	found, err := s.trailersTMDB.FindTrailers(ctx, q)
	switch {
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		s.log.Warn("tmdb key rejected", "item_id", item.ID)
		s.rejectKey(ctx, ProviderTMDB)
		if recheck {
			return nil
		}
		return s.setTrailerState(ctx, item.ID, trailerFailed)
	case errors.Is(err, errNotConfigured):
		if recheck {
			return nil
		}
	case err != nil:
		return fmt.Errorf("trailer.find %s: %w", item.ID, err)
	}
	var languageless *TrailerHit
	for _, h := range found {
		if h.Lang == "" {
			languageless = &h
			continue
		}
		hits = append(hits, h)
	}
	// The last rank (O-7): the trailer without language for every language still missing.
	fallback := func() {
		if languageless == nil {
			return
		}
		for _, l := range missing() {
			h := *languageless
			h.Lang = l
			hits = append(hits, h)
		}
	}

	if recheck {
		rows, err := s.read.ListTrailersByItem(ctx, item.ID)
		if err != nil {
			return fmt.Errorf("trailer.find: load trailers of %s: %w", item.ID, err)
		}
		for _, r := range rows {
			if r.Source != sourceManual && slices.Contains(missing(), r.Lang) {
				hits = append(hits, TrailerHit{Lang: r.Lang, YouTubeID: r.YoutubeID, Title: r.Title.String, Source: r.Source,
					Official: r.Official == 1, Size: int(r.Size.Int64)})
			}
		}
		fallback()
		return s.storeTrailers(ctx, item.ID, hits, false)
	}

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
	// A language the quota blocked stays pending for YouTube instead of taking the fallback.
	if !quotaBlocked {
		fallback()
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
			Official: boolInt(h.Official), Size: sql.NullInt64{Int64: int64(h.Size), Valid: h.Size > 0}, Now: now,
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

// SettingTrailersRechecked marks that the trailers found before concept 21 stage 2 were queued
// once for the choice by resolution (O-3).
const SettingTrailersRechecked = "metadata.trailers_rechecked"

// QueueTrailerRecheckOnce queues, once at the first start after the update to concept 21 stage 2,
// a recheck (trailer.find with recheck, recheckTrailers) for every movie and series with
// trailer_state found and an automatic trailer (decision O-3 of 14 November 2026); titles with
// manual trailers only stay as they are. A title whose trailer job is already queued keeps that
// job, which runs the whole chain. The setting remembers a complete run; a failed one starts again
// at the next start. It returns the number of titles queued.
func (s *Service) QueueTrailerRecheckOnce(ctx context.Context) (int, error) {
	if _, err := s.read.GetSystemSetting(ctx, SettingTrailersRechecked); err == nil {
		return 0, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("read %s: %w", SettingTrailersRechecked, err)
	}
	ids, err := s.read.ListTrailerRecheckItems(ctx)
	if err != nil {
		return 0, fmt.Errorf("list titles to recheck trailers: %w", err)
	}
	for i, id := range ids {
		payload, err := json.Marshal(itemPayload{ItemID: id, Recheck: true})
		if err != nil {
			return i, fmt.Errorf("%s payload: %w", JobTrailerFind, err)
		}
		if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
			Type: JobTrailerFind, Priority: trailerPriority, IdempotencyKey: "trailer:" + id, Payload: payload,
		}); err != nil {
			return i, fmt.Errorf("enqueue %s %s: %w", JobTrailerFind, id, err)
		}
	}
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{
		Key: SettingTrailersRechecked, Value: "1", Now: s.now().UnixMilli(),
	}); err != nil {
		return len(ids), fmt.Errorf("write %s: %w", SettingTrailersRechecked, err)
	}
	if len(ids) > 0 {
		s.log.InfoContext(ctx, "trailers queued for the choice by resolution", "titles", len(ids))
	}
	return len(ids), nil
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
