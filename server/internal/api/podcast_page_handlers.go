package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The page «Podcast» and the section «Downloads» of the administration (concept 19 part A, 12.2
// and 12.3; contract 1.29.0): one podcast, its episodes with the state of their file, the queue
// of downloads and «Abbrechen» on one of them. Administration views, not ItemRef.

func (e serviceProblem) VisitGetPodcastResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListAdminPodcastEpisodesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitCancelPodcastDownloadResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListPodcastDownloadsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// The contract bounds of the two lists.
const (
	podcastEpisodePageDefault  = 50
	podcastDownloadPageDefault = 100
)

func (s *server) GetPodcast(ctx context.Context, req gen.GetPodcastRequestObject) (gen.GetPodcastResponseObject, error) {
	p, err := s.opts.Admin.GetPodcast(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetPodcast200JSONResponse(toAdminPodcast(p)), nil
}

func (s *server) ListAdminPodcastEpisodes(ctx context.Context, req gen.ListAdminPodcastEpisodesRequestObject) (gen.ListAdminPodcastEpisodesResponseObject, error) {
	limit := deref(req.Params.Limit)
	if limit <= 0 {
		limit = podcastEpisodePageDefault
	}
	page, err := s.opts.Admin.PodcastEpisodes(ctx, req.Id, string(deref(req.Params.Filter)), deref(req.Params.Offset), limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.AdminPodcastEpisodePage{
		Total: page.Total, Offset: page.Offset, Limit: page.Limit,
		Counts:   gen.AdminPodcastEpisodeCounts{All: page.All, WithFile: page.WithFile, WithoutFile: page.Open},
		Episodes: make([]gen.AdminPodcastEpisode, 0, len(page.Episodes)),
	}
	for _, e := range page.Episodes {
		out.Episodes = append(out.Episodes, toAdminPodcastEpisode(e))
	}
	return gen.ListAdminPodcastEpisodes200JSONResponse(out), nil
}

func toAdminPodcastEpisode(e model.AdminPodcastEpisode) gen.AdminPodcastEpisode {
	out := gen.AdminPodcastEpisode{
		Id: e.ID, Title: e.Title, Feed: e.Feed, State: gen.AdminPodcastEpisodeState(e.State),
		PublishedAt: utc(e.PublishedAt), DurationMs: e.DurationMs, SizeBytes: e.SizeBytes,
		FileName: optString(e.FileName), ProgressBytes: e.Progress, TotalBytes: e.Total, RetryAt: utc(e.RetryAt),
	}
	// A code outside the contract's list (a newer server's) is left out rather than sent.
	if code := gen.AdminPodcastEpisodeErrorCode(e.ErrorCode); code.Valid() {
		out.ErrorCode = &code
	}
	return out
}

func (s *server) CancelPodcastDownload(ctx context.Context, req gen.CancelPodcastDownloadRequestObject) (gen.CancelPodcastDownloadResponseObject, error) {
	if err := s.opts.Admin.CancelPodcastDownload(ctx, req.Id, req.EpisodeId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CancelPodcastDownload204Response{}, nil
}

func (s *server) ListPodcastDownloads(ctx context.Context, req gen.ListPodcastDownloadsRequestObject) (gen.ListPodcastDownloadsResponseObject, error) {
	limit := deref(req.Params.Limit)
	if limit <= 0 {
		limit = podcastDownloadPageDefault
	}
	page, err := s.opts.Admin.PodcastDownloadQueue(ctx, limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.PodcastDownloadPage{Total: page.Total, Running: page.Running, Paused: page.Paused,
		Downloads: make([]gen.PodcastDownload, 0, len(page.Downloads))}
	for _, d := range page.Downloads {
		out.Downloads = append(out.Downloads, gen.PodcastDownload{
			EpisodeId: d.EpisodeID, EpisodeTitle: d.EpisodeTitle, PodcastId: d.PodcastID, PodcastTitle: d.PodcastTitle,
			LibraryId: d.LibraryID, State: gen.PodcastDownloadState(d.State), SizeBytes: d.SizeBytes,
			ProgressBytes: d.Progress, TotalBytes: d.Total,
		})
	}
	return gen.ListPodcastDownloads200JSONResponse(out), nil
}
