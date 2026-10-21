package discover

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M7 discover methods (design section 3.1). The base commit answers model.ErrNotImplemented;
// stream A replaces this file with the search, genres, collection, watchlist and favourite reads
// and writes.

func (s *Service) Search(context.Context, model.Profile, model.SearchQuery) (model.SearchResult, error) {
	return model.SearchResult{}, model.ErrNotImplemented
}

func (s *Service) Genres(context.Context, model.Profile) ([]model.GenreRef, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) Collection(context.Context, model.Profile, string, int, int) (model.Collection, error) {
	return model.Collection{}, model.ErrNotImplemented
}

func (s *Service) Watchlist(context.Context, model.Profile, int, int) ([]model.ItemSummary, int, error) {
	return nil, 0, model.ErrNotImplemented
}

func (s *Service) AddToWatchlist(context.Context, model.Profile, string) error {
	return model.ErrNotImplemented
}

func (s *Service) RemoveFromWatchlist(context.Context, model.Profile, string) error {
	return model.ErrNotImplemented
}

func (s *Service) Favorites(context.Context, model.Profile) ([]model.ItemSummary, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) AddFavorite(context.Context, model.Profile, string) error {
	return model.ErrNotImplemented
}

func (s *Service) RemoveFavorite(context.Context, model.Profile, string) error {
	return model.ErrNotImplemented
}
