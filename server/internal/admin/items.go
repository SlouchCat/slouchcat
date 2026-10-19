package admin

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
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Administration of single titles (M3, design 4.2): the unfiltered view, manual match, refresh,
// trailers and the metadata search. Matching, refreshing and trailers are work of package metadata
// (Metadata); this file validates the input and answers with the administration view.

const (
	maxProviderIDLen = 32
	maxSearchQuery   = 200
	maxTrailerURL    = 512
)

// GetAdminItem returns the administration view of a title: state, provider ids, locked fields,
// ratings, trailers, media files with root and path, and child counts. A deleted or unknown title
// is model.ErrNotFound.
func (s *Service) GetAdminItem(ctx context.Context, itemID string) (model.AdminItem, error) {
	if s.meta == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	it, err := s.liveItem(ctx, s.sys, itemID)
	if err != nil {
		return model.AdminItem{}, err
	}
	out := model.AdminItem{
		ID: it.ID, Kind: it.Kind, Title: it.Title, Year: nullInt(it.Year), LibraryID: it.LibraryID, ParentID: it.ParentID.String,
		SeasonNumber: nullInt(it.SeasonNumber), EpisodeNumber: nullInt(it.EpisodeNumber), EpisodeEnd: nullInt(it.EpisodeEnd),
		MetadataState: it.MetadataState, MetadataError: it.MetadataError.String, MatchedAt: nullTime(it.MatchedAt),
		TMDBID: nullInt64(it.TmdbID), TVDBID: nullInt64(it.TvdbID), IMDBID: it.ImdbID.String,
		OwnRating: nullInt(it.OwnRating), EffectiveRating: nullInt(it.EffectiveRating), RatingSource: it.RatingSource.String,
		TrailerState: it.TrailerState, LockedFields: []string{},
	}
	if err := json.Unmarshal([]byte(it.LockedFields), &out.LockedFields); err != nil || out.LockedFields == nil {
		out.LockedFields = []string{}
	}
	files, err := s.mediaFiles(ctx, []string{it.ID})
	if err != nil {
		return model.AdminItem{}, err
	}
	out.MediaFiles = files[it.ID]
	if out.MediaFiles == nil {
		out.MediaFiles = []model.AdminMediaFile{}
	}
	children, err := s.sys.AdminCountChildren(ctx, sql.NullString{String: it.ID, Valid: true})
	if err != nil {
		return model.AdminItem{}, fmt.Errorf("count children: %w", err)
	}
	out.Seasons, out.Episodes, out.Extras = int(children.Seasons), int(children.Episodes), int(children.Extras)
	if out.Trailers, err = s.meta.ListTrailersSystem(ctx, it.ID); err != nil {
		return model.AdminItem{}, fmt.Errorf("list trailers: %w", err)
	}
	if out.Trailers == nil {
		out.Trailers = []model.Trailer{}
	}
	return out, nil
}

// liveItem reads a title that is not deleted, or model.ErrNotFound.
func (s *Service) liveItem(ctx context.Context, q *systemq.Queries, itemID string) (systemq.Item, error) {
	it, err := q.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && it.DeletedAt.Valid) {
		return systemq.Item{}, model.ErrNotFound
	}
	if err != nil {
		return systemq.Item{}, fmt.Errorf("get item: %w", err)
	}
	return it, nil
}

// mediaFiles returns the media files of the titles by title id, versions in their order.
func (s *Service) mediaFiles(ctx context.Context, itemIDs []string) (map[string][]model.AdminMediaFile, error) {
	out := map[string][]model.AdminMediaFile{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := s.sys.AdminListMediaFiles(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("list media files: %w", err)
	}
	for _, r := range rows {
		out[r.ItemID] = append(out[r.ItemID], model.AdminMediaFile{
			ID: r.ID, RootID: r.RootID, Path: r.RelPath, SizeBytes: r.Size, Available: !r.MissingSince.Valid,
			VersionLabel: r.VersionLabel.String, ProbedAt: nullTime(r.ProbedAt), ProbeError: r.ProbeError.String,
		})
	}
	return out, nil
}

// SetItemMatch assigns a film or series manually to a provider entry (B-6): TMDB for both, TVDB
// for series only. Package metadata stores the id, locks the provider ids and queues the match at
// high priority; the answer is the administration view after the assignment.
func (s *Service) SetItemMatch(ctx context.Context, itemID, provider, providerID string) (model.AdminItem, error) {
	if s.meta == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	providerID = strings.TrimSpace(providerID)
	var verr *model.ValidationError
	if provider != "tmdb" && provider != "tvdb" {
		verr = verr.Add("provider", "invalid_value")
	}
	switch n, err := strconv.ParseInt(providerID, 10, 64); {
	case providerID == "":
		verr = verr.Add("providerId", "required")
	case len(providerID) > maxProviderIDLen:
		verr = verr.Add("providerId", "too_long")
	case err != nil || n <= 0:
		verr = verr.Add("providerId", "invalid_value") // TMDB and TVDB ids are positive numbers
	}
	if verr != nil {
		return model.AdminItem{}, verr
	}
	it, err := s.liveItem(ctx, s.sys, itemID)
	if err != nil {
		return model.AdminItem{}, err
	}
	if (it.Kind != "movie" && it.Kind != "series") || (provider == "tvdb" && it.Kind != "series") {
		return model.AdminItem{}, (&model.ValidationError{}).Add("provider", "invalid_value")
	}
	if err := s.meta.AssignMatch(ctx, itemID, provider, providerID); err != nil {
		return model.AdminItem{}, err
	}
	s.log.InfoContext(ctx, "item matched manually", "item_id", itemID, "provider", provider)
	return s.GetAdminItem(ctx, itemID)
}

// RefreshItem queues the metadata match (which queues images and trailers) and the probes of every
// file of the title; locked fields and manual trailers stay.
func (s *Service) RefreshItem(ctx context.Context, itemID string) (model.AdminItem, error) {
	if s.meta == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	if _, err := s.liveItem(ctx, s.sys, itemID); err != nil {
		return model.AdminItem{}, err
	}
	if err := s.meta.Refresh(ctx, itemID); err != nil {
		return model.AdminItem{}, err
	}
	return s.GetAdminItem(ctx, itemID)
}

var (
	langPattern      = regexp.MustCompile(`^[a-z]{2,3}$`)
	youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,16}$`)
)

// CreateItemTrailer stores a manual trailer from a YouTube URL or a bare video id (B-8). A request
// with both must name the same video.
func (s *Service) CreateItemTrailer(ctx context.Context, itemID, lang, youtubeID, rawURL string) (model.Trailer, error) {
	if s.meta == nil {
		return model.Trailer{}, ErrNotConfigured
	}
	var verr *model.ValidationError
	if !langPattern.MatchString(lang) {
		verr = verr.Add("lang", "invalid_value")
	}
	var id string
	switch {
	case youtubeID == "" && rawURL == "":
		verr = verr.Add("youtubeId", "required")
	case len(rawURL) > maxTrailerURL:
		verr = verr.Add("url", "too_long")
	default:
		fromID, idOK := ParseYouTubeID(youtubeID)
		fromURL, urlOK := ParseYouTubeID(rawURL)
		switch {
		case youtubeID != "" && !idOK:
			verr = verr.Add("youtubeId", "invalid_value")
		case rawURL != "" && !urlOK:
			verr = verr.Add("url", "invalid_value")
		case youtubeID != "" && rawURL != "" && fromID != fromURL:
			verr = verr.Add("youtubeId", "invalid_value")
		case idOK:
			id = fromID
		default:
			id = fromURL
		}
	}
	if verr != nil {
		return model.Trailer{}, verr
	}
	if _, err := s.liveItem(ctx, s.sys, itemID); err != nil {
		return model.Trailer{}, err
	}
	return s.meta.AddManualTrailer(ctx, itemID, lang, id)
}

// DeleteItemTrailer removes a trailer (manual or automatic) of a title.
func (s *Service) DeleteItemTrailer(ctx context.Context, itemID string, trailerID int64) error {
	if s.meta == nil {
		return ErrNotConfigured
	}
	if _, err := s.liveItem(ctx, s.sys, itemID); err != nil {
		return err
	}
	return s.meta.DeleteTrailer(ctx, itemID, trailerID)
}

// ParseYouTubeID returns the video id of a YouTube URL (youtube.com/watch?v=, youtu.be/,
// youtube-nocookie.com/embed/; with or without scheme and www.) or of a bare id
// ([A-Za-z0-9_-]{8,16}). It applies the rules of metadata.ParseYouTubeID without importing it.
func ParseYouTubeID(urlOrID string) (string, bool) {
	v := strings.TrimSpace(urlOrID)
	if youtubeIDPattern.MatchString(v) {
		return v, true
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return "", false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	var id string
	switch host {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		}
	case "youtube-nocookie.com":
		id = strings.TrimPrefix(u.Path, "/embed/")
	}
	if !youtubeIDPattern.MatchString(id) {
		return "", false
	}
	return id, true
}

// SearchMetadata asks a provider for candidates of the manual match (B-6).
func (s *Service) SearchMetadata(ctx context.Context, provider, kind, query string, year *int) ([]model.MetadataCandidate, error) {
	if s.meta == nil {
		return nil, ErrNotConfigured
	}
	query = strings.TrimSpace(query)
	var verr *model.ValidationError
	if provider != "tmdb" && provider != "tvdb" {
		verr = verr.Add("provider", "invalid_value")
	}
	if kind != "movie" && kind != "series" {
		verr = verr.Add("kind", "invalid_value")
	}
	switch {
	case query == "":
		verr = verr.Add("query", "required")
	case utf8.RuneCountInString(query) > maxSearchQuery:
		verr = verr.Add("query", "too_long")
	}
	if year != nil && (*year < 1800 || *year > 2100) {
		verr = verr.Add("year", "invalid_value")
	}
	if verr != nil {
		return nil, verr
	}
	out, err := s.meta.Search(ctx, provider, kind, query, year)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.MetadataCandidate{}
	}
	return out, nil
}

// CancelLibraryScan cancels the queued or running scan of a library; without one the library is
// returned unchanged.
func (s *Service) CancelLibraryScan(ctx context.Context, libraryID int64) (model.Library, error) {
	if s.maint == nil {
		return model.Library{}, ErrNotConfigured
	}
	return s.maint.CancelScan(ctx, libraryID)
}

func nullInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func nullInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func nullTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.UnixMilli(v.Int64).UTC()
	return &t
}
