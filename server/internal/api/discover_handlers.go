package api

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// M7 discover operations of the active profile (plan M7, design sections 2 and 3.1): search,
// genres, collections, watchlist and favourites. The catalog service reads through the visibility
// layer; an invisible title or a collection without a visible member answers 404 not_found.

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

// maxSearchQuery is the longest search text of the contract (SearchRequest.query maxLength).
const maxSearchQuery = 100

func (s *server) Search(ctx context.Context, req gen.SearchRequestObject) (gen.SearchResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	b := req.Body
	q := model.SearchQuery{
		Text: strings.TrimSpace(deref(b.Query)), Genre: deref(b.Genre), YearFrom: deref(b.YearFrom), YearTo: deref(b.YearTo),
		MaxRating: -1, DeviceID: session.DeviceID, Offset: max(deref(b.Offset), 0), Limit: deref(b.Limit),
	}
	if b.MaxRating != nil {
		q.MaxRating = int(*b.MaxRating)
	}
	for _, t := range deref(b.MediaTypes) {
		q.MediaTypes = append(q.MediaTypes, string(t))
	}
	for _, a := range deref(b.Availability) {
		q.Availability = append(q.Availability, string(a))
	}
	// A text of two characters and more searches; without one, at least one filter browses.
	var verr *model.ValidationError
	switch n := utf8.RuneCountInString(q.Text); {
	case n > maxSearchQuery:
		verr = verr.Add("query", "too_long")
	case n > 0 && n < visibility.MinSearchChars:
		verr = verr.Add("query", "too_short")
	case n == 0 && q.Genre == "" && b.YearFrom == nil && b.YearTo == nil && b.MaxRating == nil && len(q.MediaTypes) == 0 && len(q.Availability) == 0:
		verr = verr.Add("query", "required")
	}
	if verr != nil {
		return s.problem(ctx, verr), nil
	}
	if q.Limit <= 0 {
		q.Limit = visibility.DefaultSearchPage
	}
	q.Limit = min(q.Limit, visibility.MaxSearchPage)
	res, err := s.opts.Catalog.Search(ctx, session.Profile, q)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	pid := session.Profile.ID
	out := gen.SearchResult{
		Items: toItemRefList(pid, res.Items), Total: res.Total, Offset: q.Offset, Limit: q.Limit,
		People: make([]gen.PersonRef, 0, len(res.People)), Collections: toCollectionRefs(pid, res.Collections),
	}
	for _, p := range res.People {
		out.People = append(out.People, toPersonRef(p))
	}
	return gen.Search200JSONResponse(out), nil
}

func (s *server) ListGenres(ctx context.Context, _ gen.ListGenresRequestObject) (gen.ListGenresResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	genres, err := s.opts.Catalog.Genres(ctx, session.Profile)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListGenres200JSONResponse(toGenreRefs(genres)), nil
}

func (s *server) GetCollection(ctx context.Context, req gen.GetCollectionRequestObject) (gen.GetCollectionResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	offset, limit := pageParams(req.Params.Offset, req.Params.Limit)
	c, err := s.opts.Catalog.Collection(ctx, session.Profile, req.Id, offset, limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetCollection200JSONResponse(gen.Collection{
		Id: c.ID, Name: c.Name, Overview: optString(c.Overview), Items: toItemRefList(session.Profile.ID, c.Items),
		Total: c.Total, Offset: offset, Limit: limit,
	}), nil
}

func (s *server) ListWatchlist(ctx context.Context, req gen.ListWatchlistRequestObject) (gen.ListWatchlistResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	offset, limit := pageParams(req.Params.Offset, req.Params.Limit)
	items, total, err := s.opts.Catalog.Watchlist(ctx, session.Profile, offset, limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListWatchlist200JSONResponse(gen.ItemPage{Items: toItemRefList(session.Profile.ID, items), Total: total, Offset: offset, Limit: limit}), nil
}

func (s *server) AddToWatchlist(ctx context.Context, req gen.AddToWatchlistRequestObject) (gen.AddToWatchlistResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.opts.Catalog.AddToWatchlist(ctx, session.Profile, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.AddToWatchlist204Response{}, nil
}

func (s *server) RemoveFromWatchlist(ctx context.Context, req gen.RemoveFromWatchlistRequestObject) (gen.RemoveFromWatchlistResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.opts.Catalog.RemoveFromWatchlist(ctx, session.Profile, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RemoveFromWatchlist204Response{}, nil
}

func (s *server) ListFavorites(ctx context.Context, _ gen.ListFavoritesRequestObject) (gen.ListFavoritesResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.opts.Catalog.Favorites(ctx, session.Profile)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListFavorites200JSONResponse(toItemRefList(session.Profile.ID, items)), nil
}

func (s *server) AddFavorite(ctx context.Context, req gen.AddFavoriteRequestObject) (gen.AddFavoriteResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.opts.Catalog.AddFavorite(ctx, session.Profile, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.AddFavorite204Response{}, nil
}

func (s *server) RemoveFavorite(ctx context.Context, req gen.RemoveFavoriteRequestObject) (gen.RemoveFavoriteResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.opts.Catalog.RemoveFavorite(ctx, session.Profile, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RemoveFavorite204Response{}, nil
}

// pageParams applies the contract bounds of the paged lists: offset from 0, limit 1..200 (50 when
// absent).
func pageParams(offset, limit *int) (int, int) {
	o, l := max(deref(offset), 0), deref(limit)
	if l <= 0 {
		l = visibility.DefaultPage
	}
	return o, min(l, visibility.MaxPage)
}

// toItemRefList maps a list of titles; never nil.
func toItemRefList(profileID int64, items []model.ItemSummary) []gen.ItemRef {
	out := make([]gen.ItemRef, 0, len(items))
	for _, it := range items {
		out = append(out, toItemRef(profileID, it))
	}
	return out
}

func toPersonRef(p model.PersonRef) gen.PersonRef {
	return gen.PersonRef{Id: p.ID, Name: p.Name}
}

func toGenreRef(g model.GenreRef) gen.GenreRef {
	return gen.GenreRef{Name: g.Name, Count: g.Count}
}

// toGenreRefs maps genres; never nil.
func toGenreRefs(genres []model.GenreRef) []gen.GenreRef {
	out := make([]gen.GenreRef, 0, len(genres))
	for _, g := range genres {
		out = append(out, toGenreRef(g))
	}
	return out
}

// toCollectionRef maps a collection with its visible member count and its cover.
func toCollectionRef(profileID int64, c model.CollectionRef) gen.CollectionRef {
	return gen.CollectionRef{Id: c.ID, Name: c.Name, Count: c.Count, Cover: optItemRef(profileID, c.Cover)}
}

// toCollectionRefs maps collections; never nil.
func toCollectionRefs(profileID int64, cs []model.CollectionRef) []gen.CollectionRef {
	out := make([]gen.CollectionRef, 0, len(cs))
	for _, c := range cs {
		out = append(out, toCollectionRef(profileID, c))
	}
	return out
}
