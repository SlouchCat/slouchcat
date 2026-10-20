package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M6 audio operations (plan M6, design sections 2 and 3.4). The base commit answers 501
// not_implemented; stream B replaces these bodies with the podcast subscription service and the
// paged episode list.

func (e serviceProblem) VisitUpdateLibraryResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitListPodcastsResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitCreatePodcastResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitDeletePodcastResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitRefreshPodcastResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListPodcastEpisodesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) UpdateLibrary(ctx context.Context, _ gen.UpdateLibraryRequestObject) (gen.UpdateLibraryResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListPodcasts(ctx context.Context, _ gen.ListPodcastsRequestObject) (gen.ListPodcastsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) CreatePodcast(ctx context.Context, _ gen.CreatePodcastRequestObject) (gen.CreatePodcastResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) DeletePodcast(ctx context.Context, _ gen.DeletePodcastRequestObject) (gen.DeletePodcastResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) RefreshPodcast(ctx context.Context, _ gen.RefreshPodcastRequestObject) (gen.RefreshPodcastResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListPodcastEpisodes(ctx context.Context, _ gen.ListPodcastEpisodesRequestObject) (gen.ListPodcastEpisodesResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}
