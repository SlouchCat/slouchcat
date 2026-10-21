package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// M6 audio operations (plan M6, design sections 2 and 3.4): library settings with the default age
// rating (AN-2), the podcast subscriptions of the administration (H-2) and the paged episode list
// of a podcast.

func (e serviceProblem) VisitUpdateLibraryResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitListPodcastsResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitCreatePodcastResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitDeletePodcastResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitRefreshPodcastResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListPodcastEpisodesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// UpdateLibrary renames a library or sets the default rating of an audio library; the open streams
// are re-checked, because titles rated by the library may have changed their rating.
func (s *server) UpdateLibrary(ctx context.Context, req gen.UpdateLibraryRequestObject) (gen.UpdateLibraryResponseObject, error) {
	body := deref(req.Body)
	upd := model.LibraryUpdate{Name: body.Name}
	if body.DefaultRating != nil {
		r := int(*body.DefaultRating)
		upd.DefaultRating = &r
	}
	lib, err := s.opts.Admin.UpdateLibrary(ctx, req.Id, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	if err := s.recheckStreams(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateLibrary200JSONResponse(toLibrary(lib)), nil
}

func (s *server) ListPodcasts(ctx context.Context, _ gen.ListPodcastsRequestObject) (gen.ListPodcastsResponseObject, error) {
	list, err := s.opts.Admin.ListPodcasts(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListPodcasts200JSONResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toAdminPodcast(p))
	}
	return out, nil
}

func (s *server) CreatePodcast(ctx context.Context, req gen.CreatePodcastRequestObject) (gen.CreatePodcastResponseObject, error) {
	body := deref(req.Body)
	p, err := s.opts.Admin.CreatePodcast(ctx, model.PodcastCreate{LibraryID: body.LibraryId, FeedURL: body.FeedUrl})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreatePodcast201JSONResponse(toAdminPodcast(p)), nil
}

// DeletePodcast removes a subscription; open streams of its episodes end.
func (s *server) DeletePodcast(ctx context.Context, req gen.DeletePodcastRequestObject) (gen.DeletePodcastResponseObject, error) {
	if err := s.opts.Admin.DeletePodcast(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	if err := s.recheckStreams(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeletePodcast204Response{}, nil
}

func (s *server) RefreshPodcast(ctx context.Context, req gen.RefreshPodcastRequestObject) (gen.RefreshPodcastResponseObject, error) {
	p, err := s.opts.Admin.RefreshPodcast(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RefreshPodcast202JSONResponse(toAdminPodcast(p)), nil
}

func (s *server) ListPodcastEpisodes(ctx context.Context, req gen.ListPodcastEpisodesRequestObject) (gen.ListPodcastEpisodesResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	// The contract bounds: offset from 0, limit 1..200 (50 when absent).
	offset, limit := max(deref(req.Params.Offset), 0), deref(req.Params.Limit)
	if limit <= 0 {
		limit = visibility.DefaultEpisodePage
	}
	limit = min(limit, visibility.MaxEpisodePage)
	eps, total, err := s.opts.Catalog.PodcastEpisodes(ctx, session.Profile, req.Id, offset, limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.PodcastEpisodes{Episodes: make([]gen.ItemRef, 0, len(eps)), Total: total, Offset: offset, Limit: limit}
	for _, e := range eps {
		out.Episodes = append(out.Episodes, toItemRef(session.Profile.ID, e))
	}
	return gen.ListPodcastEpisodes200JSONResponse(out), nil
}

// toAdminPodcast maps a subscription; times are UTC.
func toAdminPodcast(p model.AdminPodcast) gen.AdminPodcast {
	return gen.AdminPodcast{
		Id: p.ID, LibraryId: p.LibraryID, Title: p.Title, Author: optString(p.Author), FeedUrl: p.FeedURL,
		EpisodeCount: p.EpisodeCount, Explicit: p.Explicit, LastCheckedAt: utc(p.LastCheckedAt),
		LastError: optString(p.LastError), CreatedAt: p.CreatedAt.UTC(),
	}
}
