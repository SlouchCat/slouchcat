package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the M3 administration (contract tag admin): the administration view of a title,
// manual match, refresh, trailers, mass rating, metadata search, maintenance lists, merge,
// cleanup, relink (ADR 0023) and scan cancellation. The answers carry ids and paths, never an
// ItemRef.

func (e serviceProblem) VisitCancelLibraryScanResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetAdminItemResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetItemMatchResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRefreshItemResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitListItemImageCandidatesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetItemImageResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitClearItemImageResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetItemOriginalLanguageResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateItemMetadataResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListAdminGenresResponse(w http.ResponseWriter) error { return e.visit(w) }
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
func (e serviceProblem) VisitListDuplicatesResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitMergeItemsResponse(w http.ResponseWriter) error      { return e.visit(w) }
func (e serviceProblem) VisitCleanupItemsResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitListRelinkPairsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRelinkItemsResponse(w http.ResponseWriter) error     { return e.visit(w) }

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

func (s *server) ListItemImageCandidates(ctx context.Context, req gen.ListItemImageCandidatesRequestObject) (gen.ListItemImageCandidatesResponseObject, error) {
	list, err := s.opts.Admin.ItemImageCandidates(ctx, req.Id, string(req.Params.Kind), deref(req.Params.Refresh))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListItemImageCandidates200JSONResponse{Candidates: make([]gen.ImageCandidate, 0, len(list.Candidates)), Unavailable: list.Unavailable}
	if out.Unavailable == nil {
		out.Unavailable = []string{}
	}
	for _, c := range list.Candidates {
		preview := c.PreviewURL
		if preview == "" {
			preview = candidatePreviewURL(req.Id, c.Source)
		}
		out.Candidates = append(out.Candidates, gen.ImageCandidate{Provider: c.Provider, Lang: c.Lang, Width: c.Width, Height: c.Height,
			Score: c.Score, VoteCount: c.VoteCount, Source: c.Source, PreviewUrl: preview, Chosen: c.Chosen})
	}
	return out, nil
}

func (s *server) SetItemImage(ctx context.Context, req gen.SetItemImageRequestObject) (gen.SetItemImageResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("source", "required")), nil
	}
	it, err := s.opts.Admin.SetItemImage(ctx, req.Id, string(req.Kind), req.Body.Provider, req.Body.Source)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetItemImage200JSONResponse(toAdminItem(it)), nil
}

func (s *server) ClearItemImage(ctx context.Context, req gen.ClearItemImageRequestObject) (gen.ClearItemImageResponseObject, error) {
	it, err := s.opts.Admin.ClearItemImage(ctx, req.Id, string(req.Kind))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ClearItemImage200JSONResponse(toAdminItem(it)), nil
}

func (s *server) SetItemOriginalLanguage(ctx context.Context, req gen.SetItemOriginalLanguageRequestObject) (gen.SetItemOriginalLanguageResponseObject, error) {
	// A missing language is required; null lifts the lock, a string sets the language.
	if req.Body == nil || len(req.Body.Language) == 0 {
		return s.problem(ctx, (&model.ValidationError{}).Add("language", "required")), nil
	}
	var lang *string
	if err := json.Unmarshal(req.Body.Language, &lang); err != nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("language", "invalid_value")), nil
	}
	it, err := s.opts.Admin.SetItemOriginalLanguage(ctx, req.Id, lang)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetItemOriginalLanguage200JSONResponse(toAdminItem(it)), nil
}

// UpdateItemMetadata is the metadata editor (concept 15 7.5, contract 1.15.0); an empty body
// changes nothing.
func (s *server) UpdateItemMetadata(ctx context.Context, req gen.UpdateItemMetadataRequestObject) (gen.UpdateItemMetadataResponseObject, error) {
	var edit model.ItemMetadataEdit
	if b := req.Body; b != nil {
		edit = model.ItemMetadataEdit{Title: b.Title, OriginalTitle: b.OriginalTitle, SortTitle: b.SortTitle, Year: b.Year,
			ReleaseDate: b.ReleaseDate, Tagline: b.Tagline, Overview: b.Overview, Genres: b.Genres}
		if b.Unlock != nil {
			for _, f := range *b.Unlock {
				edit.Unlock = append(edit.Unlock, string(f))
			}
		}
	}
	it, err := s.opts.Admin.UpdateItemMetadata(ctx, req.Id, edit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateItemMetadata200JSONResponse(toAdminItem(it)), nil
}

func (s *server) ListAdminGenres(ctx context.Context, _ gen.ListAdminGenresRequestObject) (gen.ListAdminGenresResponseObject, error) {
	lang, genres, err := s.opts.Admin.ListAdminGenres(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	if genres == nil {
		genres = []string{}
	}
	return gen.ListAdminGenres200JSONResponse{Lang: lang, Genres: genres}, nil
}

// toAdminItemMetadata maps the editor's view of a title.
func toAdminItemMetadata(m *model.AdminItemMetadata) *gen.AdminItemMetadata {
	if m == nil {
		return nil
	}
	out := &gen.AdminItemMetadata{Lang: m.Lang, Title: m.Title, OriginalTitle: m.OriginalTitle, SortTitle: m.SortTitle, Year: m.Year,
		ReleaseDate: m.ReleaseDate, Tagline: m.Tagline, Overview: m.Overview, Genres: m.Genres,
		Locked: make([]gen.AdminMetadataField, 0, len(m.Locked)), Sources: make([]gen.AdminMetadataSource, 0, len(m.Sources))}
	if out.Genres == nil {
		out.Genres = []string{}
	}
	for _, f := range m.Locked {
		out.Locked = append(out.Locked, gen.AdminMetadataField(f))
	}
	for _, src := range m.Sources {
		out.Sources = append(out.Sources, gen.AdminMetadataSource{Field: gen.AdminMetadataField(src.Field), Provider: src.Provider})
	}
	return out
}

// candidatePreviewURL is the address of getItemImageCandidatePreview for a local candidate.
func candidatePreviewURL(itemID, source string) string {
	return BasePath + "/admin/items/" + url.PathEscape(itemID) + "/image-candidates/preview?source=" + url.QueryEscape(source)
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

func (s *server) ListRelinkPairs(ctx context.Context, req gen.ListRelinkPairsRequestObject) (gen.ListRelinkPairsResponseObject, error) {
	list, err := s.opts.Admin.ListRelinkPairs(ctx, req.Params.LibraryId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListRelinkPairs200JSONResponse{Total: int64(list.Total), Pairs: make([]gen.RelinkPair, 0, len(list.Pairs))}
	for _, p := range list.Pairs {
		u := p.UserData
		out.Pairs = append(out.Pairs, gen.RelinkPair{
			DeletedId: p.DeletedID, LiveId: p.LiveID, LibraryId: p.LibraryID, LibraryName: p.LibraryName,
			Kind: gen.RelinkPairKind(p.Kind), Title: p.Title, Year: p.Year, DeletedTitle: p.DeletedTitle,
			DeletedYear: p.DeletedYear, DeletedAt: p.DeletedAt.UTC(),
			UserData: gen.RelinkUserData{
				Progress: u.Progress, ProgressEvents: u.ProgressEvents, ProgressDays: u.ProgressDays,
				PlaybackSessions: u.PlaybackSessions, Watchlist: u.Watchlist, Favorites: u.Favorites,
				CollectionItems: u.CollectionItems, Notifications: u.Notifications, TopPicks: u.TopPicks,
				Downloads: u.Downloads, ActivityEvents: u.ActivityEvents,
			},
		})
	}
	return out, nil
}

func (s *server) RelinkItems(ctx context.Context, req gen.RelinkItemsRequestObject) (gen.RelinkItemsResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("pairs", "required")), nil
	}
	pairs := make([]model.RelinkPairRef, 0, len(req.Body.Pairs))
	for _, p := range req.Body.Pairs {
		pairs = append(pairs, model.RelinkPairRef{DeletedID: p.DeletedId, LiveID: p.LiveId})
	}
	res, err := s.opts.Admin.RelinkItems(ctx, pairs)
	// Streams of the deleted copies end.
	if err := s.recheckAfter(ctx, err); err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.RelinkItems200JSONResponse{Relinked: make([]gen.RelinkedPair, 0, len(res.Relinked)), Skipped: int64(res.Skipped)}
	for _, d := range res.Relinked {
		out.Relinked = append(out.Relinked, gen.RelinkedPair{KeptId: d.KeptID, RemovedId: d.RemovedID, Merged: d.Merged, Moved: d.Moved})
	}
	if res.StoppedBy != "" {
		stopped := gen.RelinkOutcomeStoppedBy(res.StoppedBy)
		out.StoppedBy = &stopped
	}
	return out, nil
}

// toAdminItem maps the administration view; times are UTC.
func toAdminItem(it model.AdminItem) gen.AdminItem {
	out := gen.AdminItem{
		Id: it.ID, Kind: gen.ItemKind(it.Kind), Title: it.Title, Year: it.Year, LibraryId: it.LibraryID,
		SeasonNumber: it.SeasonNumber, EpisodeNumber: it.EpisodeNumber, EpisodeEnd: it.EpisodeEnd,
		MetadataState: gen.AdminItemMetadataState(it.MetadataState), MetadataError: optString(it.MetadataError), MatchedAt: utc(it.MatchedAt),
		TmdbId: it.TMDBID, TvdbId: it.TVDBID, ImdbId: optString(it.IMDBID), LockedFields: it.LockedFields,
		OwnRating: optRating(it.OwnRating), EffectiveRating: optRating(it.EffectiveRating), RatingSource: optString(it.RatingSource),
		TrailerState:     gen.AdminItemTrailerState(it.TrailerState),
		Trailers:         make([]gen.Trailer, 0, len(it.Trailers)),
		MediaFiles:       make([]gen.AdminMediaFile, 0, len(it.MediaFiles)),
		AnilistId:        it.AniListID,
		OriginalLanguage: it.OriginalLanguage,
		Providers:        providerEnums(it.Providers),
		FieldSources:     make([]gen.FieldSource, 0, len(it.FieldSources)),
		Metadata:         toAdminItemMetadata(it.Metadata),
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
	if it.Images != nil {
		images := make([]gen.AdminItemImage, 0, len(it.Images))
		for _, img := range it.Images {
			g := gen.AdminItemImage{Kind: gen.ImageKind(img.Kind), Chosen: img.Chosen, Locked: img.Locked,
				Provider: optString(img.Provider), Lang: optString(img.Lang), Source: optString(img.Source)}
			switch {
			case img.PreviewURL != "":
				g.PreviewUrl = &img.PreviewURL
			case img.Provider != "":
				preview := candidatePreviewURL(it.ID, img.Source)
				g.PreviewUrl = &preview
			}
			images = append(images, g)
		}
		out.Images = &images
	}
	if it.ImageKinds != nil {
		kinds := make([]gen.ImageKind, 0, len(it.ImageKinds))
		for _, k := range it.ImageKinds {
			kinds = append(kinds, gen.ImageKind(k))
		}
		out.ImageKinds = &kinds
	}
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
