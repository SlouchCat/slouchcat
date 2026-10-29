package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the M3 administration (contract tag admin): the administration view of a title,
// manual match, refresh, trailers, mass rating, metadata search, maintenance lists, merge,
// cleanup and scan cancellation. The answers carry ids and paths, never an ItemRef.

func (e serviceProblem) VisitCancelLibraryScanResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetAdminItemResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetItemMatchResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRefreshItemResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitCreateItemTrailerResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteItemTrailerResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetItemsRatingResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSearchMetadataResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListMaintenanceIssuesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListDuplicatesResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitMergeItemsResponse(w http.ResponseWriter) error     { return e.visit(w) }
func (e serviceProblem) VisitCleanupItemsResponse(w http.ResponseWriter) error   { return e.visit(w) }

func (s *server) CancelLibraryScan(ctx context.Context, req gen.CancelLibraryScanRequestObject) (gen.CancelLibraryScanResponseObject, error) {
	lib, err := s.opts.Admin.CancelLibraryScan(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CancelLibraryScan202JSONResponse(toLibrary(lib)), nil
}

func (s *server) GetAdminItem(ctx context.Context, req gen.GetAdminItemRequestObject) (gen.GetAdminItemResponseObject, error) {
	it, err := s.opts.Admin.GetAdminItem(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetAdminItem200JSONResponse(toAdminItem(it)), nil
}

func (s *server) SetItemMatch(ctx context.Context, req gen.SetItemMatchRequestObject) (gen.SetItemMatchResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("provider", "required")), nil
	}
	it, err := s.opts.Admin.SetItemMatch(ctx, req.Id, string(req.Body.Provider), req.Body.ProviderId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetItemMatch202JSONResponse(toAdminItem(it)), nil
}

func (s *server) RefreshItem(ctx context.Context, req gen.RefreshItemRequestObject) (gen.RefreshItemResponseObject, error) {
	it, err := s.opts.Admin.RefreshItem(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RefreshItem202JSONResponse(toAdminItem(it)), nil
}

func (s *server) CreateItemTrailer(ctx context.Context, req gen.CreateItemTrailerRequestObject) (gen.CreateItemTrailerResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("lang", "required")), nil
	}
	tr, err := s.opts.Admin.CreateItemTrailer(ctx, req.Id, req.Body.Lang, deref(req.Body.YoutubeId), deref(req.Body.Url))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateItemTrailer201JSONResponse(toTrailer(tr)), nil
}

func (s *server) DeleteItemTrailer(ctx context.Context, req gen.DeleteItemTrailerRequestObject) (gen.DeleteItemTrailerResponseObject, error) {
	if err := s.opts.Admin.DeleteItemTrailer(ctx, req.Id, req.TrailerId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteItemTrailer204Response{}, nil
}

func (s *server) SetItemsRating(ctx context.Context, req gen.SetItemsRatingRequestObject) (gen.SetItemsRatingResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("itemIds", "required")), nil
	}
	n, err := s.opts.Admin.SetItemsRating(ctx, req.Body.ItemIds, ratingPtr(req.Body.Rating))
	// One re-check for the whole request: streams of titles the profile may no longer see end.
	if err := s.recheckAfter(ctx, err); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetItemsRating200JSONResponse{Updated: n}, nil
}

// recheckAfter re-checks the open direct-play streams after a mass change of titles (rating,
// merge, cleanup), also when the change failed half way, because the batches before the failure
// are committed. The change's own error wins over a failed re-check.
func (s *server) recheckAfter(ctx context.Context, err error) error {
	if rerr := s.recheckStreams(ctx); rerr != nil && err == nil {
		return rerr
	}
	return err
}

func (s *server) SearchMetadata(ctx context.Context, req gen.SearchMetadataRequestObject) (gen.SearchMetadataResponseObject, error) {
	p := req.Params
	list, err := s.opts.Admin.SearchMetadata(ctx, string(p.Provider), string(p.Kind), p.Query, p.Year)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.SearchMetadata200JSONResponse, 0, len(list))
	for _, c := range list {
		out = append(out, gen.MetadataCandidate{
			Provider: gen.MetadataProvider(c.Provider), ProviderId: c.ProviderID, Title: c.Title, OriginalTitle: optString(c.OriginalTitle),
			Year: c.Year, Overview: optString(c.Overview), PosterUrl: optString(c.PosterURL),
		})
	}
	return out, nil
}

func (s *server) ListMaintenanceIssues(ctx context.Context, req gen.ListMaintenanceIssuesRequestObject) (gen.ListMaintenanceIssuesResponseObject, error) {
	p := req.Params
	page, err := s.opts.Admin.ListIssues(ctx, string(p.Kind), p.LibraryId, deref(p.Limit), deref(p.Offset))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListMaintenanceIssues200JSONResponse{Total: page.Total, Items: make([]gen.Issue, 0, len(page.Items))}
	for _, is := range page.Items {
		out.Items = append(out.Items, gen.Issue{
			Kind: gen.IssueKind(is.Kind), ItemId: is.ItemID, ItemKind: gen.ItemKind(is.ItemKind), Title: is.Title, Year: is.Year,
			LibraryId: is.LibraryID, Path: optString(is.Path), MediaFileId: optString(is.MediaFileID), Detail: optString(is.Detail),
		})
	}
	return out, nil
}

func (s *server) ListDuplicates(ctx context.Context, req gen.ListDuplicatesRequestObject) (gen.ListDuplicatesResponseObject, error) {
	groups, err := s.opts.Admin.ListDuplicates(ctx, req.Params.LibraryId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListDuplicates200JSONResponse, 0, len(groups))
	for _, g := range groups {
		group := gen.DuplicateGroup{Reason: gen.DuplicateGroupReason(g.Reason)}
		for _, it := range g.Items {
			files := make([]gen.AdminMediaFile, 0, len(it.Files))
			for _, f := range it.Files {
				files = append(files, toAdminMediaFile(f))
			}
			libraryID := it.LibraryID
			group.Items = append(group.Items, struct {
				Files     []gen.AdminMediaFile `json:"files"`
				ItemId    gen.ItemID           `json:"itemId"`
				Kind      gen.ItemKind         `json:"kind"`
				LibraryId *int64               `json:"libraryId,omitempty"`
				Title     string               `json:"title"`
				Year      *int                 `json:"year,omitempty"`
			}{Files: files, ItemId: it.ItemID, Kind: gen.ItemKind(it.Kind), LibraryId: &libraryID, Title: it.Title, Year: it.Year})
		}
		out = append(out, group)
	}
	return out, nil
}

func (s *server) MergeItems(ctx context.Context, req gen.MergeItemsRequestObject) (gen.MergeItemsResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("targetId", "required")), nil
	}
	it, err := s.opts.Admin.MergeItems(ctx, req.Body.TargetId, req.Body.SourceIds)
	// Streams of the deleted sources end.
	if err := s.recheckAfter(ctx, err); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.MergeItems200JSONResponse(toAdminItem(it)), nil
}

func (s *server) CleanupItems(ctx context.Context, req gen.CleanupItemsRequestObject) (gen.CleanupItemsResponseObject, error) {
	var items, files []string
	if b := req.Body; b != nil {
		items, files = deref(b.ItemIds), deref(b.MediaFileIds)
	}
	removedItems, removedFiles, err := s.opts.Admin.CleanupItems(ctx, items, files)
	// Streams of soft-deleted titles and removed files end.
	if err := s.recheckAfter(ctx, err); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CleanupItems200JSONResponse{Items: removedItems, Files: removedFiles}, nil
}

// toAdminItem maps the administration view; times are UTC.
func toAdminItem(it model.AdminItem) gen.AdminItem {
	out := gen.AdminItem{
		Id: it.ID, Kind: gen.ItemKind(it.Kind), Title: it.Title, Year: it.Year, LibraryId: it.LibraryID,
		SeasonNumber: it.SeasonNumber, EpisodeNumber: it.EpisodeNumber, EpisodeEnd: it.EpisodeEnd,
		MetadataState: gen.AdminItemMetadataState(it.MetadataState), MetadataError: optString(it.MetadataError), MatchedAt: utc(it.MatchedAt),
		TmdbId: it.TMDBID, TvdbId: it.TVDBID, ImdbId: optString(it.IMDBID), LockedFields: it.LockedFields,
		OwnRating: optRating(it.OwnRating), EffectiveRating: optRating(it.EffectiveRating), RatingSource: optString(it.RatingSource),
		TrailerState: gen.AdminItemTrailerState(it.TrailerState),
		Trailers:     make([]gen.Trailer, 0, len(it.Trailers)),
		MediaFiles:   make([]gen.AdminMediaFile, 0, len(it.MediaFiles)),
		AnilistId:    it.AniListID,
		Providers:    providerEnums(it.Providers),
		FieldSources: make([]gen.FieldSource, 0, len(it.FieldSources)),
	}
	for _, f := range it.FieldSources {
		out.FieldSources = append(out.FieldSources, gen.FieldSource{Field: f.Field, Lang: f.Lang, Provider: f.Provider})
	}
	if it.ParentID != "" {
		out.ParentId = &it.ParentID
	}
	if out.LockedFields == nil {
		out.LockedFields = []string{}
	}
	for _, t := range it.Trailers {
		out.Trailers = append(out.Trailers, toTrailer(t))
	}
	for _, f := range it.MediaFiles {
		out.MediaFiles = append(out.MediaFiles, toAdminMediaFile(f))
	}
	out.Children.Seasons, out.Children.Episodes, out.Children.Extras = it.Seasons, it.Episodes, it.Extras
	return out
}

func toAdminMediaFile(f model.AdminMediaFile) gen.AdminMediaFile {
	out := gen.AdminMediaFile{
		Id: f.ID, Path: f.Path, SizeBytes: f.SizeBytes, Available: f.Available, VersionLabel: optString(f.VersionLabel),
		ProbedAt: utc(f.ProbedAt), ProbeError: optString(f.ProbeError),
	}
	if f.RootID != 0 {
		out.RootId = &f.RootID
	}
	return out
}
