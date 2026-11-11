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
func (e serviceProblem) VisitUpdatePodcastResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDownloadPodcastEpisodesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitEstimatePodcastDownloadsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// UpdateLibrary renames a library or sets the default rating of an audio library; the open streams
// are re-checked, because titles rated by the library may have changed their rating.
func (s *server) UpdateLibrary(ctx context.Context, req gen.UpdateLibraryRequestObject) (gen.UpdateLibraryResponseObject, error) {
	body := deref(req.Body)
	upd := model.LibraryUpdate{Name: body.Name, Trickplay: libraryTrickplay(body.Trickplay)}
	if d := body.PodcastDownloads; d != nil {
		upd.PodcastDownloads = &model.PodcastDownloads{Mode: string(d.Mode), Scope: string(d.Scope), LastN: d.LastN,
			Parallel: d.Parallel, RootID: d.RootId}
	}
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
	p, err := s.opts.Admin.CreatePodcast(ctx, model.PodcastCreate{LibraryID: body.LibraryId, FeedURL: body.FeedUrl,
		SubscribeOnly: body.LoadEpisodes != nil && !*body.LoadEpisodes})
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

// UpdatePodcast stores the own setting of stored episodes of a podcast (concept 19 part A, 5.1).
func (s *server) UpdatePodcast(ctx context.Context, req gen.UpdatePodcastRequestObject) (gen.UpdatePodcastResponseObject, error) {
	body := deref(req.Body)
	upd := model.PodcastDownloadsUpdate{Mode: (*string)(body.Mode), Scope: (*string)(body.Scope), LastN: body.LastN}
	p, err := s.opts.Admin.UpdatePodcast(ctx, req.Id, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdatePodcast200JSONResponse(toAdminPodcast(p)), nil
}

// DownloadPodcastEpisodes queues the episodes without file of a podcast, or the given ones (5.3).
func (s *server) DownloadPodcastEpisodes(ctx context.Context, req gen.DownloadPodcastEpisodesRequestObject) (gen.DownloadPodcastEpisodesResponseObject, error) {
	var ids []string
	if req.Body != nil && req.Body.EpisodeIds != nil {
		ids = *req.Body.EpisodeIds
	}
	n, err := s.opts.Admin.DownloadPodcastEpisodes(ctx, req.Id, ids)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DownloadPodcastEpisodes202JSONResponse(gen.PodcastDownloadQueued{Queued: n}), nil
}

// EstimatePodcastDownloads answers what an imagined setting would load once (12.3).
func (s *server) EstimatePodcastDownloads(ctx context.Context, req gen.EstimatePodcastDownloadsRequestObject) (gen.EstimatePodcastDownloadsResponseObject, error) {
	body := deref(req.Body)
	est, err := s.opts.Admin.EstimatePodcastDownloads(ctx, req.Id, model.PodcastDownloadEstimateRequest{
		PodcastID: deref(body.PodcastId), Mode: string(body.Mode), Scope: string(body.Scope), LastN: deref(body.LastN),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.EstimatePodcastDownloads200JSONResponse(gen.PodcastDownloadEstimate{
		Episodes: est.Episodes, Bytes: max(est.Bytes, 0), FreeBytes: max(est.FreeBytes, 0),
	}), nil
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
	out := gen.AdminPodcast{
		Id: p.ID, LibraryId: p.LibraryID, Title: p.Title, Author: optString(p.Author), FeedUrl: optString(p.FeedURL),
		EpisodeCount: p.EpisodeCount, LocalEpisodeCount: p.LocalEpisodeCount, Explicit: p.Explicit,
		LastCheckedAt: utc(p.LastCheckedAt), LastError: optString(p.LastError), CreatedAt: p.CreatedAt.UTC(),
		RootId: p.RootID, CoverUrl: adminPosterURL(p.ID, p.CoverHash),
	}
	if p.FeedOrigin != "" {
		origin := gen.AdminPodcastFeedOrigin(p.FeedOrigin)
		out.FeedOrigin = &origin
	}
	// The stored episodes (contract 1.27.0); the setting is absent for a podcast without feed.
	out.StoredBytes, out.Savable, out.UnclearEpisodeCount = p.StoredBytes, p.Savable, p.UnclearEpisodeCount
	out.Downloads = gen.PodcastDownloadCounts{Queued: p.DownloadsQueued, Running: p.DownloadsRunning, Failed: p.DownloadsFailed}
	out.LatestEpisodeAt = utc(p.LatestEpisodeAt)
	if p.EffectiveMode != "" {
		mode, scope := gen.AdminPodcastMode(p.Mode), gen.AdminPodcastScope(p.Scope)
		effMode, effScope, effN := gen.PodcastMode(p.EffectiveMode), gen.PodcastScope(p.EffectiveScope), p.EffectiveLastN
		out.Mode, out.Scope, out.LastN = &mode, &scope, p.LastN
		out.EffectiveMode, out.EffectiveScope, out.EffectiveLastN = &effMode, &effScope, &effN
	}
	return out
}
