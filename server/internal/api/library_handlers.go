package api

import (
	"context"
	"net/http"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Library views of the active profile (concept 08, ADR 0016): the libraries of the side
// navigation, one page of a library with its facets and the user picks. The catalog
// service reads through the visibility layer; a library without a visible title for the profile
// (unknown, not permitted or empty) answers 404 not_found, never 403.

func (e serviceProblem) VisitListVisibleLibrariesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListLibraryItemsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListPicksResponse(w http.ResponseWriter) error        { return e.visit(w) }
func (e serviceProblem) VisitListHomeRowResponse(w http.ResponseWriter) error      { return e.visit(w) }

func (s *server) ListVisibleLibraries(ctx context.Context, _ gen.ListVisibleLibrariesRequestObject) (gen.ListVisibleLibrariesResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := s.opts.Catalog.VisibleLibraries(ctx, session.Profile)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListVisibleLibraries200JSONResponse, 0, len(libs))
	for _, l := range libs {
		contents := make([]gen.RootContent, 0, len(l.Contents))
		for _, c := range l.Contents {
			contents = append(contents, gen.RootContent(c))
		}
		out = append(out, gen.LibraryNav{Id: l.ID, Name: l.Name, Kind: gen.LibraryKind(l.Kind), Contents: contents, ItemCount: l.ItemCount})
	}
	return out, nil
}

// ListLibraryItems checks the bounds the strict server leaves to the handler (years, genre length,
// rating, like Search); the catalog checks what depends on the query and the library (sort, order,
// status, letter, content).
func (s *server) ListLibraryItems(ctx context.Context, req gen.ListLibraryItemsRequestObject) (gen.ListLibraryItemsResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	q := model.LibraryQuery{
		Genre: deref(p.Genre), YearFrom: deref(p.YearFrom), YearTo: deref(p.YearTo), MaxRating: -1,
		Letter: deref(p.Letter), Offset: max(deref(p.Offset), 0), Limit: deref(p.Limit),
	}
	if p.Content != nil {
		q.Content = string(*p.Content)
	}
	if p.Sort != nil {
		q.Sort = string(*p.Sort)
	}
	if p.Order != nil {
		q.Order = string(*p.Order)
	}
	if p.MaxRating != nil {
		q.MaxRating = int(*p.MaxRating)
	}
	for _, st := range deref(p.Status) {
		q.Status = append(q.Status, string(st))
	}
	if verr := libraryFilterProblems(p.Genre, p.YearFrom, p.YearTo, p.MaxRating); verr != nil {
		return s.problem(ctx, verr), nil
	}
	if q.Limit <= 0 {
		q.Limit = visibility.DefaultLibraryPage
	}
	q.Limit = min(q.Limit, visibility.MaxLibraryPage)
	page, err := s.opts.Catalog.LibraryItems(ctx, session.Profile, req.Id, q)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	ratings := make([]gen.Rating, 0, len(page.Facets.Ratings))
	for _, r := range page.Facets.Ratings {
		ratings = append(ratings, gen.Rating(r))
	}
	letters := page.Facets.Letters
	if letters == nil {
		letters = []string{}
	}
	facets := gen.LibraryFacets{Letters: letters, Genres: toGenreRefs(page.Facets.Genres), Ratings: ratings}
	if page.Facets.LetterStarts != nil {
		starts := make([]gen.LetterStart, 0, len(page.Facets.LetterStarts))
		for _, ls := range page.Facets.LetterStarts {
			starts = append(starts, gen.LetterStart{Letter: ls.Letter, Offset: ls.Offset})
		}
		facets.LetterStarts = &starts
	}
	return gen.ListLibraryItems200JSONResponse(gen.LibraryItemsPage{
		Items: toItemRefList(session.Profile.ID, page.Items), Total: page.Total, Offset: q.Offset, Limit: q.Limit,
		Facets: facets,
	}), nil
}

// libraryFilterProblems checks the filter bounds of a library page that the strict server leaves
// to the handler (years, genre length, rating, like Search); listLibraryItems and listAudioGroups
// share them.
func libraryFilterProblems(genre *string, yearFrom, yearTo *int, maxRating *gen.Rating) *model.ValidationError {
	var verr *model.ValidationError
	if utf8.RuneCountInString(deref(genre)) > maxSearchGenre {
		verr = verr.Add("genre", "too_long")
	}
	if y := yearFrom; y != nil && (*y < minSearchYear || *y > maxSearchYear) {
		verr = verr.Add("yearFrom", "invalid_value")
	}
	if y := yearTo; y != nil && (*y < minSearchYear || *y > maxSearchYear) {
		verr = verr.Add("yearTo", "invalid_value")
	}
	if maxRating != nil && !maxRating.Valid() {
		verr = verr.Add("maxRating", "invalid_value")
	}
	return verr
}

func (s *server) ListPicks(ctx context.Context, req gen.ListPicksRequestObject) (gen.ListPicksResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	offset, limit := max(deref(req.Params.Offset), 0), deref(req.Params.Limit)
	if limit <= 0 {
		limit = visibility.DefaultLibraryPage
	}
	limit = min(limit, visibility.MaxLibraryPage)
	items, total, err := s.opts.Catalog.PicksPage(ctx, session.Profile, offset, limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListPicks200JSONResponse(gen.PicksPage{Items: toItemRefList(session.Profile.ID, items), Total: total, Offset: offset, Limit: limit}), nil
}

// ListHomeRow answers one page of the list behind a row title of the start page (concept 20 P-7).
func (s *server) ListHomeRow(ctx context.Context, req gen.ListHomeRowRequestObject) (gen.ListHomeRowResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	content := ""
	if p.Content != nil {
		if !p.Content.Valid() {
			return s.problem(ctx, (&model.ValidationError{}).Add("content", "invalid_value")), nil
		}
		content = string(*p.Content)
	}
	if !req.Key.Valid() {
		return s.problem(ctx, model.ErrNotFound), nil
	}
	offset, limit := max(deref(p.Offset), 0), deref(p.Limit)
	if limit <= 0 {
		limit = visibility.DefaultLibraryPage
	}
	limit = min(limit, visibility.MaxLibraryPage)
	items, total, err := s.opts.Catalog.HomeList(ctx, session.Profile, string(req.Key), content, offset, limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListHomeRow200JSONResponse(gen.ItemPage{Items: toItemRefList(session.Profile.ID, items), Total: total, Offset: offset, Limit: limit}), nil
}
