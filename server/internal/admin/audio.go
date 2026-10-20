package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Audio libraries (M6, design 3.4, AN-2): library settings with the default age rating, the
// podcast subscriptions of package metadata, and the rule that a manual rating in an audio library
// may only be stricter than the library default.

// Podcasts is implemented by package metadata: the podcast subscriptions (H-2).
type Podcasts interface {
	// Subscribe fetches the feed once and stores the podcast (*model.ValidationError,
	// model.ErrFeedExists, model.ErrFeedUnreachable, model.ErrFeedInvalid).
	Subscribe(ctx context.Context, libraryID int64, feedURL string) (model.AdminPodcast, error)
	// Unsubscribe soft-deletes a subscription with its episodes (model.ErrNotFound).
	Unsubscribe(ctx context.Context, id string) error
	// ListPodcasts returns every live subscription, newest first.
	ListPodcasts(ctx context.Context) ([]model.AdminPodcast, error)
	// RefreshPodcast queues the fetch of a subscription (model.ErrNotFound).
	RefreshPodcast(ctx context.Context, id string) (model.AdminPodcast, error)
	// RefreshLibrary queues the fetch of every subscription of a library.
	RefreshLibrary(ctx context.Context, libraryID int64) (int, error)
}

// WithPodcasts sets the podcast subscriptions of package metadata and returns the receiver.
func (s *Service) WithPodcasts(p Podcasts) *Service {
	s.podcasts = p
	return s
}

// UpdateLibrary renames a library or sets the default rating of an audio library (updateLibrary).
// A new default reaches every title of the library whose rating still comes from the library
// (rating_source library) in the same transaction; manual, feed and tag ratings stay. Errors:
// model.ErrNotFound, *model.ValidationError (name required or too_long, defaultRating
// invalid_value, not_allowed for video libraries).
func (s *Service) UpdateLibrary(ctx context.Context, id int64, upd model.LibraryUpdate) (model.Library, error) {
	lib, err := s.read.GetLibrary(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Library{}, model.ErrNotFound
	}
	if err != nil {
		return model.Library{}, fmt.Errorf("get library: %w", err)
	}
	var verr *model.ValidationError
	name := lib.Name
	if upd.Name != nil {
		name = strings.TrimSpace(*upd.Name)
		switch {
		case name == "":
			verr = verr.Add("name", "required")
		case utf8.RuneCountInString(name) > maxLibraryName:
			verr = verr.Add("name", "too_long")
		}
	}
	rating := lib.DefaultRating
	if upd.DefaultRating != nil {
		switch {
		case !model.AudioLibraryKind(lib.Kind):
			verr = verr.Add("defaultRating", "not_allowed")
		case !validRating(upd.DefaultRating):
			verr = verr.Add("defaultRating", "invalid_value")
		default:
			rating = sql.NullInt64{Int64: int64(*upd.DefaultRating), Valid: true}
		}
	}
	if verr != nil {
		return model.Library{}, verr
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Library{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := s.now().UnixMilli()
	if _, err := appq.New(tx).UpdateLibrarySettings(ctx, appq.UpdateLibrarySettingsParams{Name: name, DefaultRating: rating, Now: now, ID: id}); err != nil {
		return model.Library{}, fmt.Errorf("update library: %w", err)
	}
	var titles int64
	if rating != lib.DefaultRating && rating.Valid {
		if titles, err = systemq.New(tx).AdminApplyLibraryRating(ctx, systemq.AdminApplyLibraryRatingParams{
			OwnRating: rating, Now: now, LibraryID: id,
		}); err != nil {
			return model.Library{}, fmt.Errorf("apply library rating: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Library{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "library updated", "library_id", id, "default_rating_changed", rating != lib.DefaultRating, "titles_rerated", titles)
	return s.libraries.Get(ctx, id)
}

// checkRatingFloor rejects a manual rating below the default rating of the title's audio library
// (AN-2: a title may only be rated stricter than its library); model.ErrRatingBelowDefault.
func (s *Service) checkRatingFloor(ctx context.Context, item systemq.Item, rating *int) error {
	if rating == nil {
		return nil
	}
	lib, err := s.read.GetLibrary(ctx, item.LibraryID)
	if err != nil {
		return fmt.Errorf("get library of %s: %w", item.ID, err)
	}
	if lib.DefaultRating.Valid && int64(*rating) < lib.DefaultRating.Int64 {
		return model.ErrRatingBelowDefault
	}
	return nil
}

// ListPodcasts returns every podcast subscription (listPodcasts).
func (s *Service) ListPodcasts(ctx context.Context) ([]model.AdminPodcast, error) {
	if s.podcasts == nil {
		return nil, ErrNotConfigured
	}
	return s.podcasts.ListPodcasts(ctx)
}

// CreatePodcast subscribes a podcast library to a feed (createPodcast).
func (s *Service) CreatePodcast(ctx context.Context, req model.PodcastCreate) (model.AdminPodcast, error) {
	if s.podcasts == nil {
		return model.AdminPodcast{}, ErrNotConfigured
	}
	return s.podcasts.Subscribe(ctx, req.LibraryID, req.FeedURL)
}

// DeletePodcast removes a subscription with its episodes (deletePodcast).
func (s *Service) DeletePodcast(ctx context.Context, id string) error {
	if s.podcasts == nil {
		return ErrNotConfigured
	}
	return s.podcasts.Unsubscribe(ctx, id)
}

// RefreshPodcast queues the fetch of a subscription now (refreshPodcast).
func (s *Service) RefreshPodcast(ctx context.Context, id string) (model.AdminPodcast, error) {
	if s.podcasts == nil {
		return model.AdminPodcast{}, ErrNotConfigured
	}
	return s.podcasts.RefreshPodcast(ctx, id)
}
