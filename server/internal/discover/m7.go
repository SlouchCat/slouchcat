package discover

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Discover operations of M7 (design 3.1): search, genres, collections, the watchlist (E-5) and the
// favourites (E-8) of a profile. Reads go through the visibility layer; the watchlist and favourite
// writes run only after it found the title visible, so an invisible title answers
// model.ErrNotFound and is never stored.

// Search searches titles, people and collections the profile may see, or browses the titles by the
// filters without a text (E-4).
func (s *Service) Search(ctx context.Context, profile model.Profile, q model.SearchQuery) (model.SearchResult, error) {
	return s.vis.Search(ctx, visibility.Profile(profile), q)
}

// Genres returns the genres of the visible titles in the profile language, most frequent first.
func (s *Service) Genres(ctx context.Context, profile model.Profile) ([]model.GenreRef, error) {
	return s.vis.Genres(ctx, visibility.Profile(profile), visibility.MaxRowItems)
}

// Collection returns a collection with one page of its visible members, or model.ErrNotFound when
// it has none (E-6).
func (s *Service) Collection(ctx context.Context, profile model.Profile, id string, offset, limit int) (model.Collection, error) {
	return s.vis.Collection(ctx, visibility.Profile(profile), id, offset, limit)
}

// Watchlist returns one page of the profile's watchlist, newest first, and its total.
func (s *Service) Watchlist(ctx context.Context, profile model.Profile, offset, limit int) ([]model.ItemSummary, int, error) {
	return s.vis.Watchlist(ctx, visibility.Profile(profile), offset, limit)
}

// AddToWatchlist puts a visible title on the profile's watchlist (idempotent); seasons and extras
// are model.ErrNotListable.
func (s *Service) AddToWatchlist(ctx context.Context, profile model.Profile, itemID string) error {
	it, err := s.vis.Summary(ctx, visibility.Profile(profile), itemID)
	if err != nil {
		return err
	}
	if it.Kind == "season" || it.Kind == "extra" {
		return model.ErrNotListable
	}
	if err := s.write.AddWatchlist(ctx, appq.AddWatchlistParams{ProfileID: profile.ID, ItemID: it.ID, Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("add to watchlist: %w", err)
	}
	return nil
}

// RemoveFromWatchlist takes a visible title off the profile's watchlist (idempotent).
func (s *Service) RemoveFromWatchlist(ctx context.Context, profile model.Profile, itemID string) error {
	if err := s.vis.Visible(ctx, visibility.Profile(profile), itemID); err != nil {
		return err
	}
	if err := s.write.RemoveWatchlist(ctx, appq.RemoveWatchlistParams{ProfileID: profile.ID, ItemID: itemID}); err != nil {
		return fmt.Errorf("remove from watchlist: %w", err)
	}
	return nil
}

// Favorites returns the profile's visible favourite series and podcasts, newest first.
func (s *Service) Favorites(ctx context.Context, profile model.Profile) ([]model.ItemSummary, error) {
	return s.vis.Favorites(ctx, visibility.Profile(profile))
}

// AddFavorite marks a visible series or podcast as favourite of the profile (idempotent); any other
// kind is model.ErrNotFavoritable.
func (s *Service) AddFavorite(ctx context.Context, profile model.Profile, itemID string) error {
	it, err := s.vis.Summary(ctx, visibility.Profile(profile), itemID)
	if err != nil {
		return err
	}
	if it.Kind != "series" && it.Kind != "podcast" {
		return model.ErrNotFavoritable
	}
	if err := s.write.AddFavorite(ctx, appq.AddFavoriteParams{ProfileID: profile.ID, ItemID: it.ID, Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("add favorite: %w", err)
	}
	return nil
}

// RemoveFavorite removes the favourite mark of a visible title (idempotent).
func (s *Service) RemoveFavorite(ctx context.Context, profile model.Profile, itemID string) error {
	if err := s.vis.Visible(ctx, visibility.Profile(profile), itemID); err != nil {
		return err
	}
	if err := s.write.RemoveFavorite(ctx, appq.RemoveFavoriteParams{ProfileID: profile.ID, ItemID: itemID}); err != nil {
		return fmt.Errorf("remove favorite: %w", err)
	}
	return nil
}
