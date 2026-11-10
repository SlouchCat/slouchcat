package api

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Book sources (contract 1.26.0, concept 19 stage D3, D6): the match preview of an audiobook and
// the page «Hörbücher: Quellen». Administration views with ids, never an ItemRef.

// errBookSourcesNotBuilt answers the operations of contract 1.26.0 whose service step 3 of stage
// D3 builds; until then they answer 500.
var errBookSourcesNotBuilt = errors.New("not built yet: concept 19 D3 step 3")

// bookProviders are the providers that search and match audiobooks.
var bookProviders = []string{model.ProviderAudible, model.ProviderDNB, model.ProviderAppleBooks,
	model.ProviderOpenLibrary, model.ProviderGoogleBooks}

func isBookProvider(p string) bool { return slices.Contains(bookProviders, p) }

func (e serviceProblem) VisitGetMatchPreviewResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetAudiobookSourcesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateAudiobookSourcesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitMatchAllAudiobooksResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListAudiobookReviewResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) GetMatchPreview(ctx context.Context, _ gen.GetMatchPreviewRequestObject) (gen.GetMatchPreviewResponseObject, error) {
	return s.problem(ctx, errBookSourcesNotBuilt), nil
}

func (s *server) GetAudiobookSources(ctx context.Context, _ gen.GetAudiobookSourcesRequestObject) (gen.GetAudiobookSourcesResponseObject, error) {
	return s.problem(ctx, errBookSourcesNotBuilt), nil
}

func (s *server) UpdateAudiobookSources(ctx context.Context, _ gen.UpdateAudiobookSourcesRequestObject) (gen.UpdateAudiobookSourcesResponseObject, error) {
	return s.problem(ctx, errBookSourcesNotBuilt), nil
}

func (s *server) MatchAllAudiobooks(ctx context.Context, _ gen.MatchAllAudiobooksRequestObject) (gen.MatchAllAudiobooksResponseObject, error) {
	return s.problem(ctx, errBookSourcesNotBuilt), nil
}

func (s *server) ListAudiobookReview(ctx context.Context, _ gen.ListAudiobookReviewRequestObject) (gen.ListAudiobookReviewResponseObject, error) {
	return s.problem(ctx, errBookSourcesNotBuilt), nil
}
