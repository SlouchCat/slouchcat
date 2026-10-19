package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Administration functions of the metadata package (design 3.2 and 4.2): provider search, manual
// match, refresh and manual trailers. The admin package calls them behind its owner checks; they
// do not check permissions themselves.

// Limits of the administration input.
const (
	maxSearchQueryLen = 200
	maxSearchResults  = 20
	// JobProbeFile is the library's probe job (library.JobProbeFile); refresh only enqueues it.
	JobProbeFile  = "probe.file"
	probePriority = 3
)

// trailerLangPattern is the shape of trailers.lang (ISO 639-1 or 639-2).
var trailerLangPattern = regexp.MustCompile(`^[a-z]{2,3}$`)

// Search asks a provider ("tmdb", "tvdb") for movies or series ("movie", "series") matching query,
// optionally released in year (manual match, B-6). Errors: *model.ValidationError (provider,
// kind, query, year), model.ErrTMDBKeyInvalid (TMDB without key or with a rejected key; TVDB with
// a rejected key), model.ErrProviderNotConfigured (TVDB without key), model.ErrTMDBUnreachable
// (the provider did not answer). TVDB has no movies: a movie search there returns no candidates.
func (s *Service) Search(ctx context.Context, provider, kind, query string, year *int) ([]model.MetadataCandidate, error) {
	var v *model.ValidationError
	if provider != ProviderTMDB && provider != ProviderTVDB {
		v = v.Add("provider", "invalid_value")
	}
	if kind != "movie" && kind != "series" {
		v = v.Add("kind", "invalid_value")
	}
	query = strings.TrimSpace(query)
	switch {
	case query == "":
		v = v.Add("query", "required")
	case utf8.RuneCountInString(query) > maxSearchQueryLen:
		v = v.Add("query", "too_long")
	}
	if year != nil && (*year < 1800 || *year > 2200) {
		v = v.Add("year", "invalid_value")
	}
	if v != nil {
		return nil, v
	}
	var prov Provider = s.tmdb
	if provider == ProviderTVDB {
		prov = s.tvdb
	}
	if !prov.Configured(ctx) {
		if provider == ProviderTMDB {
			return nil, fmt.Errorf("%w: no tmdb key configured", model.ErrTMDBKeyInvalid)
		}
		return nil, model.ErrProviderNotConfigured
	}
	var cs []Candidate
	var err error
	if kind == "movie" {
		cs, err = prov.SearchMovie(ctx, MovieQuery{Title: query, Year: year})
	} else {
		cs, err = prov.SearchSeries(ctx, SeriesQuery{Title: query, Year: year})
	}
	if errors.Is(err, errNotConfigured) {
		return nil, model.ErrProviderNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("metadata search: %w", err)
	}
	out := make([]model.MetadataCandidate, 0, min(len(cs), maxSearchResults))
	for _, c := range cs {
		if len(out) == maxSearchResults {
			break
		}
		out = append(out, model.MetadataCandidate{Provider: c.Provider, ProviderID: c.ID, Title: c.Title,
			OriginalTitle: c.OriginalTitle, Year: c.Year, Overview: c.Overview, PosterURL: c.PosterURL})
	}
	return out, nil
}

// AssignMatch assigns a provider ID to a movie or series by hand (B-6): it sets tmdb_id or
// tvdb_id, clears the other provider ID and imdb_id, locks the three IDs, sets metadata and
// trailers pending, removes the automatic trailers and enqueues metadata.match at priority 7 (the
// match brings texts, images and trailers of the assigned title). Errors: model.ErrNotFound,
// *model.ValidationError (provider invalid_value for other kinds and for TVDB with a movie,
// providerId invalid_value).
func (s *Service) AssignMatch(ctx context.Context, itemID, provider, providerID string) error {
	var v *model.ValidationError
	if provider != ProviderTMDB && provider != ProviderTVDB {
		v = v.Add("provider", "invalid_value")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(providerID), 10, 64)
	if err != nil || id <= 0 {
		v = v.Add("providerId", "invalid_value")
	}
	if v != nil {
		return v
	}
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return err
	}
	if (item.Kind != "movie" && item.Kind != "series") || (provider == ProviderTVDB && item.Kind != "series") {
		return v.Add("provider", "invalid_value")
	}
	p := systemq.AssignItemMatchParams{ID: item.ID, Now: s.now().UnixMilli(),
		LockedFields: addLocked(item.LockedFields, "tmdb_id", "tvdb_id", "imdb_id")}
	if provider == ProviderTMDB {
		p.TmdbID = sql.NullInt64{Int64: id, Valid: true}
	} else {
		p.TvdbID = sql.NullInt64{Int64: id, Valid: true}
	}

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("assign match: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	if err := q.AssignItemMatch(ctx, p); err != nil {
		return fmt.Errorf("assign match %s: %w", item.ID, err)
	}
	if err := q.DeleteAutoTrailers(ctx, item.ID); err != nil {
		return fmt.Errorf("assign match %s: delete trailers: %w", item.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("assign match %s: commit: %w", item.ID, err)
	}
	s.log.Info("manual match assigned", "item_id", item.ID, "provider", provider, "provider_id", id)
	return s.enqueueMatch(ctx, item.ID, matchManualPriority)
}

// Refresh re-runs the enrichment of a title: metadata.match at priority 7 (images and trailers
// follow from the match) and probe.file (key probe:<fileId>, priority 3) for every media file of
// the item. A matched episode is set pending first, because its match job skips matched
// episodes; a season refreshes its series. Locked fields stay. Errors: model.ErrNotFound.
func (s *Service) Refresh(ctx context.Context, itemID string) error {
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return err
	}
	target := item.ID
	switch {
	case item.Kind == "episode" && item.MetadataState != statePending:
		if err := s.write.SetItemMetadataState(ctx, systemq.SetItemMetadataStateParams{
			MetadataState: statePending, MatchedAt: item.MatchedAt, Now: s.now().UnixMilli(), ID: item.ID,
		}); err != nil {
			return fmt.Errorf("refresh %s: %w", item.ID, err)
		}
	case item.Kind == "season" && item.ParentID.Valid:
		target = item.ParentID.String
	}
	if err := s.enqueueMatch(ctx, target, matchManualPriority); err != nil {
		return fmt.Errorf("refresh %s: %w", item.ID, err)
	}
	files, err := s.read.ListMediaFileIDsByItem(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("refresh %s: list files: %w", item.ID, err)
	}
	for _, f := range files {
		payload, err := json.Marshal(struct {
			MediaFileID string `json:"mediaFileId"`
		}{f})
		if err != nil {
			return fmt.Errorf("refresh %s: probe payload: %w", item.ID, err)
		}
		if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobProbeFile, Priority: probePriority,
			IdempotencyKey: "probe:" + f, Payload: payload}); err != nil {
			return fmt.Errorf("refresh %s: %w", item.ID, err)
		}
	}
	return nil
}

// AddManualTrailer adds a trailer by hand (source manual, survives every trailer.find); a row of
// the same language and video from a provider becomes manual. It sets trailer_state found.
// Errors: *model.ValidationError (lang, youtubeId invalid_value), model.ErrNotFound.
func (s *Service) AddManualTrailer(ctx context.Context, itemID, lang, youtubeID string) (model.Trailer, error) {
	var v *model.ValidationError
	if !trailerLangPattern.MatchString(lang) {
		v = v.Add("lang", "invalid_value")
	}
	if !youtubeIDPattern.MatchString(youtubeID) {
		v = v.Add("youtubeId", "invalid_value")
	}
	if v != nil {
		return model.Trailer{}, v
	}
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return model.Trailer{}, err
	}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return model.Trailer{}, fmt.Errorf("add trailer: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	now := s.now().UnixMilli()
	row, err := q.UpsertManualTrailer(ctx, systemq.UpsertManualTrailerParams{ItemID: item.ID, Lang: lang, YoutubeID: youtubeID, Now: now})
	if err != nil {
		return model.Trailer{}, fmt.Errorf("add trailer to %s: %w", item.ID, err)
	}
	if err := q.SetTrailerState(ctx, systemq.SetTrailerStateParams{TrailerState: trailerFound,
		CheckedAt: item.TrailerCheckedAt, Now: now, ID: item.ID}); err != nil {
		return model.Trailer{}, fmt.Errorf("add trailer to %s: state: %w", item.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return model.Trailer{}, fmt.Errorf("add trailer to %s: commit: %w", item.ID, err)
	}
	return toTrailer(row), nil
}

// DeleteTrailer removes one trailer of the item (manual or automatic). When none is left the
// trailer state becomes none. Errors: model.ErrNotFound (no such trailer at the item).
func (s *Service) DeleteTrailer(ctx context.Context, itemID string, trailerID int64) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete trailer: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	n, err := q.DeleteTrailer(ctx, systemq.DeleteTrailerParams{ID: trailerID, ItemID: itemID})
	if err != nil {
		return fmt.Errorf("delete trailer %d: %w", trailerID, err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	left, err := q.CountTrailers(ctx, itemID)
	if err != nil {
		return fmt.Errorf("delete trailer %d: count: %w", trailerID, err)
	}
	if left == 0 {
		item, err := q.GetItemSystem(ctx, itemID)
		if err != nil {
			return fmt.Errorf("delete trailer %d: load item: %w", trailerID, err)
		}
		if err := q.SetTrailerState(ctx, systemq.SetTrailerStateParams{TrailerState: trailerNone,
			CheckedAt: item.TrailerCheckedAt, Now: s.now().UnixMilli(), ID: itemID}); err != nil {
			return fmt.Errorf("delete trailer %d: state: %w", trailerID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete trailer %d: commit: %w", trailerID, err)
	}
	return nil
}

// ListTrailersSystem returns every trailer of the item, unfiltered (administration), by language,
// official first. Errors: model.ErrNotFound.
func (s *Service) ListTrailersSystem(ctx context.Context, itemID string) ([]model.Trailer, error) {
	if _, err := s.liveItem(ctx, itemID); err != nil {
		return nil, err
	}
	rows, err := s.read.ListTrailersByItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("list trailers of %s: %w", itemID, err)
	}
	out := make([]model.Trailer, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTrailer(r))
	}
	return out, nil
}

// ParseYouTubeID extracts the video ID from a YouTube URL (youtube.com/watch?v=, youtu.be/,
// youtube.com/embed|shorts|live|v/, youtube-nocookie.com/embed/) or accepts a bare ID.
func ParseYouTubeID(urlOrID string) (string, bool) {
	in := strings.TrimSpace(urlOrID)
	if youtubeIDPattern.MatchString(in) {
		return in, true
	}
	if !strings.Contains(in, "://") {
		in = "https://" + in
	}
	u, err := url.Parse(in)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	host := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."), "m.")
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	id := ""
	switch host {
	case "youtu.be":
		id = segs[0]
	case "youtube.com", "music.youtube.com":
		switch {
		case segs[0] == "watch":
			id = u.Query().Get("v")
		case len(segs) == 2 && (segs[0] == "embed" || segs[0] == "shorts" || segs[0] == "live" || segs[0] == "v"):
			id = segs[1]
		}
	case "youtube-nocookie.com":
		if len(segs) == 2 && segs[0] == "embed" {
			id = segs[1]
		}
	}
	if !youtubeIDPattern.MatchString(id) {
		return "", false
	}
	return id, true
}

// liveItem loads an item that exists and is not soft-deleted, else model.ErrNotFound.
func (s *Service) liveItem(ctx context.Context, itemID string) (systemq.Item, error) {
	if !isULID(itemID) {
		return systemq.Item{}, model.ErrNotFound
	}
	item, err := s.read.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt.Valid) {
		return systemq.Item{}, model.ErrNotFound
	}
	if err != nil {
		return systemq.Item{}, fmt.Errorf("load item %s: %w", itemID, err)
	}
	return item, nil
}

// addLocked returns the locked_fields JSON with names appended (stored order, no duplicates).
func addLocked(raw string, names ...string) string {
	var cur []string
	if err := json.Unmarshal([]byte(raw), &cur); err != nil {
		cur = nil
	}
	have := map[string]bool{}
	for _, n := range cur {
		have[n] = true
	}
	for _, n := range names {
		if !have[n] {
			cur = append(cur, n)
			have[n] = true
		}
	}
	out, err := json.Marshal(cur)
	if err != nil {
		return raw
	}
	return string(out)
}

func toTrailer(t systemq.Trailer) model.Trailer {
	return model.Trailer{ID: t.ID, Lang: t.Lang, YouTubeID: t.YoutubeID, Title: t.Title.String, Source: t.Source, Official: t.Official == 1}
}
