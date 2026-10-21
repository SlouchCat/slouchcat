package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M7 discover operations of the active profile (plan M7, design sections 2 and 3.1): search,
// genres, collections, watchlist and favourites. The base commit answers 501 not_implemented;
// stream A replaces these bodies with the catalog service.

func (e serviceProblem) VisitSearchResponse(w http.ResponseWriter) error         { return e.visit(w) }
func (e serviceProblem) VisitListGenresResponse(w http.ResponseWriter) error     { return e.visit(w) }
func (e serviceProblem) VisitGetCollectionResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitListWatchlistResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitAddToWatchlistResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRemoveFromWatchlistResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListFavoritesResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitAddFavoriteResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitRemoveFavoriteResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) Search(ctx context.Context, _ gen.SearchRequestObject) (gen.SearchResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListGenres(ctx context.Context, _ gen.ListGenresRequestObject) (gen.ListGenresResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) GetCollection(ctx context.Context, _ gen.GetCollectionRequestObject) (gen.GetCollectionResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListWatchlist(ctx context.Context, _ gen.ListWatchlistRequestObject) (gen.ListWatchlistResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) AddToWatchlist(ctx context.Context, _ gen.AddToWatchlistRequestObject) (gen.AddToWatchlistResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) RemoveFromWatchlist(ctx context.Context, _ gen.RemoveFromWatchlistRequestObject) (gen.RemoveFromWatchlistResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListFavorites(ctx context.Context, _ gen.ListFavoritesRequestObject) (gen.ListFavoritesResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) AddFavorite(ctx context.Context, _ gen.AddFavoriteRequestObject) (gen.AddFavoriteResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) RemoveFavorite(ctx context.Context, _ gen.RemoveFavoriteRequestObject) (gen.RemoveFavoriteResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}
