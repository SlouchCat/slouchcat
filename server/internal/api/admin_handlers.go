package api

import (
	"context"
	"net/http"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of library administration (contract tag admin). Errors go through serviceProblem
// (auth_handlers.go).

func (e serviceProblem) VisitListLibrariesResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitCreateLibraryResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetLibraryResponse(w http.ResponseWriter) error     { return e.visit(w) }
func (e serviceProblem) VisitScanLibraryResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitAddLibraryRootResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdateLibraryRootResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListMetadataProvidersResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRemoveLibraryRootResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteLibraryResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitAcceptLibraryLossResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (e serviceProblem) VisitSetItemRatingResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ListLibraries(ctx context.Context, _ gen.ListLibrariesRequestObject) (gen.ListLibrariesResponseObject, error) {
	libs, err := s.opts.Admin.ListLibraries(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListLibraries200JSONResponse, 0, len(libs))
	for _, l := range libs {
		out = append(out, toLibrary(l))
	}
	return out, nil
}

func (s *server) CreateLibrary(ctx context.Context, req gen.CreateLibraryRequestObject) (gen.CreateLibraryResponseObject, error) {
	body := deref(req.Body)
	lib, err := s.opts.Admin.CreateLibrary(ctx, libraryRequest(body))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateLibrary201JSONResponse(toLibrary(lib)), nil
}

func (s *server) GetLibrary(ctx context.Context, req gen.GetLibraryRequestObject) (gen.GetLibraryResponseObject, error) {
	lib, err := s.opts.Admin.GetLibrary(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetLibrary200JSONResponse(toLibrary(lib)), nil
}

func (s *server) ScanLibrary(ctx context.Context, req gen.ScanLibraryRequestObject) (gen.ScanLibraryResponseObject, error) {
	lib, err := s.opts.Admin.ScanLibrary(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ScanLibrary202JSONResponse(toLibrary(lib)), nil
}

// AddLibraryRoot adds folders to a library in one go (idea 02, E8; F-13).
func (s *server) AddLibraryRoot(ctx context.Context, req gen.AddLibraryRootRequestObject) (gen.AddLibraryRootResponseObject, error) {
	lib, err := s.opts.Admin.AddLibraryRoots(ctx, req.Id, rootRequests(deref(req.Body).Roots))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.AddLibraryRoot201JSONResponse(toLibrary(lib)), nil
}

// UpdateLibraryRoot changes the content, the metadata sources (ADR 0015) or the path (ADR 0023)
// of a folder.
func (s *server) UpdateLibraryRoot(ctx context.Context, req gen.UpdateLibraryRootRequestObject) (gen.UpdateLibraryRootResponseObject, error) {
	body := deref(req.Body)
	upd := model.LibraryRootUpdate{ConfirmPath: deref(body.ConfirmPath), Path: body.Path}
	if body.Content != nil {
		c := string(*body.Content)
		upd.Content = &c
	}
	if body.Providers != nil {
		upd.Providers = providerNames(*body.Providers)
	}
	lib, err := s.opts.Admin.UpdateLibraryRoot(ctx, req.Id, req.RootId, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateLibraryRoot200JSONResponse(toLibrary(lib)), nil
}

// ListMetadataProviders describes the providers to the folder form (ADR 0015).
func (s *server) ListMetadataProviders(ctx context.Context, _ gen.ListMetadataProvidersRequestObject) (gen.ListMetadataProvidersResponseObject, error) {
	infos, err := s.opts.Admin.ListMetadataProviders(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make([]gen.MetadataProviderInfo, 0, len(infos))
	for _, p := range infos {
		info := gen.MetadataProviderInfo{
			Id: gen.MetadataProvider(p.ID), Configured: p.Configured,
			Roles:     make([]gen.MetadataProviderInfoRoles, 0, len(p.Roles)),
			Contents:  make([]gen.RootContent, 0, len(p.Contents)),
			Languages: append([]string{}, p.Languages...),
		}
		for _, r := range p.Roles {
			info.Roles = append(info.Roles, gen.MetadataProviderInfoRoles(r))
		}
		for _, c := range p.Contents {
			info.Contents = append(info.Contents, gen.RootContent(c))
		}
		out = append(out, info)
	}
	return gen.ListMetadataProviders200JSONResponse(out), nil
}

// RemoveLibraryRoot removes a folder from a library; the typed confirmation travels in the body
// (idea 02, E12), which is why it is a POST and not a DELETE.
func (s *server) RemoveLibraryRoot(ctx context.Context, req gen.RemoveLibraryRootRequestObject) (gen.RemoveLibraryRootResponseObject, error) {
	lib, err := s.opts.Admin.RemoveLibraryRoot(ctx, req.Id, req.RootId, deref(req.Body).ConfirmPath)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RemoveLibraryRoot200JSONResponse(toLibrary(lib)), nil
}

// DeleteLibrary deletes a library for good (concept 20 P-8); the typed name travels in the body,
// which is why it is a POST and not a DELETE.
func (s *server) DeleteLibrary(ctx context.Context, req gen.DeleteLibraryRequestObject) (gen.DeleteLibraryResponseObject, error) {
	session, _ := sessionFrom(ctx)
	out, err := s.opts.Admin.DeleteLibrary(ctx, req.Id, deref(req.Body).ConfirmName, actorOf(session))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteLibrary200JSONResponse{Name: out.Name, Roots: out.Roots, Items: out.Items, Backup: optString(out.Backup)}, nil
}

// actorOf is who asked, for the activity log: the account and the active profile of the session.
func actorOf(session model.Session) model.Actor {
	return model.Actor{AccountID: session.Account.ID, ProfileID: session.Profile.ID}
}

func (s *server) AcceptLibraryLoss(ctx context.Context, req gen.AcceptLibraryLossRequestObject) (gen.AcceptLibraryLossResponseObject, error) {
	lib, err := s.opts.Admin.AcceptLibraryLoss(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.AcceptLibraryLoss202JSONResponse(toLibrary(lib)), nil
}

func (s *server) SetItemRating(ctx context.Context, req gen.SetItemRatingRequestObject) (gen.SetItemRatingResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("rating", "required")), nil
	}
	r, err := s.opts.Admin.SetItemRating(ctx, req.Id, ratingPtr(req.Body.Rating))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	if err := s.recheckStreams(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.AdminItemRating{
		Id: r.ID, Kind: gen.ItemKind(r.Kind), Title: r.Title, OwnRating: optRating(r.OwnRating),
		EffectiveRating: optRating(r.EffectiveRating), RatingSource: optString(r.RatingSource),
	}
	return gen.SetItemRating200JSONResponse(out), nil
}

func optRating(v *int) *gen.Rating {
	if v == nil {
		return nil
	}
	r := gen.Rating(*v)
	return &r
}

// toLibrary maps a library to the contract type; times are UTC.
func toLibrary(l model.Library) gen.Library {
	roots := make([]gen.LibraryRoot, 0, len(l.Roots))
	for _, r := range l.Roots {
		root := gen.LibraryRoot{Id: r.ID, Path: r.Path, FsType: r.FSType, Watch: gen.LibraryRootWatch(r.Watch),
			Content: gen.RootContent(r.Content), Providers: providerEnums(r.Providers),
			ItemCount: r.ItemCount, FileCount: r.FileCount}
		if r.Warning != "" {
			root.Warning = &r.Warning
		}
		roots = append(roots, root)
	}
	scan := gen.LibraryScan{
		State:          gen.ScanState(l.Scan.State),
		ItemCount:      l.Scan.ItemCount,
		FileCount:      l.Scan.FileCount,
		StartedAt:      utc(l.Scan.StartedAt),
		FinishedAt:     utc(l.Scan.FinishedAt),
		LossAcceptedAt: utc(l.Scan.LossAcceptedAt),
	}
	if l.Scan.State != "never" {
		scan.FilesSeen = &l.Scan.FilesSeen
		scan.FilesTotal = &l.Scan.FilesTotal
	}
	if l.Scan.LastError != "" {
		scan.LastError = &l.Scan.LastError
	}
	out := gen.Library{
		Id: l.ID, Name: l.Name, Kind: gen.LibraryKind(l.Kind), DefaultRating: toRating(l.DefaultRating),
		Roots: roots, Scan: scan, CreatedAt: l.CreatedAt.UTC(),
	}
	if k := l.KeyframeIndex; k != nil {
		out.KeyframeIndex = &gen.LibraryKeyframeIndex{VideoFiles: k.VideoFiles, Indexed: k.Indexed, Pending: k.Pending, Unavailable: k.Unavailable}
	}
	if t := l.Trickplay; t != nil {
		out.Trickplay = &gen.LibraryTrickplay{
			Movies: gen.LibraryTrickplayMovies{Mode: gen.TrickplayMoviesMode(t.Movies.Mode), Keep: t.Movies.Keep},
			Series: gen.LibraryTrickplaySeries{Mode: gen.TrickplaySeriesMode(t.Series.Mode), Keep: t.Series.Keep, Scope: gen.TrickplaySeriesScope(t.Series.Scope)},
		}
	}
	if e := l.TrickplayEstimate; e != nil {
		est := gen.LibraryTrickplayEstimate{Movies: toTrickplayKindEstimate(e.Movies), Series: toTrickplayKindEstimate(e.Series)}
		if e.CacheFreeBytes >= 0 {
			est.CacheFreeBytes = &e.CacheFreeBytes
		}
		out.TrickplayEstimate = &est
	}
	if d := l.PodcastDownloads; d != nil {
		out.PodcastDownloads = &gen.LibraryPodcastDownloads{Mode: gen.PodcastMode(d.Mode), Scope: gen.PodcastScope(d.Scope),
			LastN: d.LastN, Parallel: d.Parallel, RootId: d.RootID}
	}
	if f := l.PodcastFolder; f != nil {
		folder := gen.PodcastFolder{RootId: f.RootID, Path: f.Path, Writable: f.Writable, FreeBytes: max(f.FreeBytes, 0)}
		if f.Problem != "" {
			problem := gen.PodcastFolderProblem(f.Problem)
			folder.Problem = &problem
		}
		out.PodcastFolder = &folder
	}
	return out
}

func toTrickplayKindEstimate(k model.TrickplayKindEstimate) gen.LibraryTrickplayKindEstimate {
	return gen.LibraryTrickplayKindEstimate{Titles: k.Titles, Files: k.Files, DurationMs: k.DurationMs,
		PerTitleBytes: k.PerTitleBytes, LibraryBytes: k.LibraryBytes, StoredBytes: k.StoredBytes}
}

// libraryTrickplay maps the trickplay setting of a request; nil stays nil.
func libraryTrickplay(t *gen.LibraryTrickplay) *model.LibraryTrickplay {
	if t == nil {
		return nil
	}
	return &model.LibraryTrickplay{
		Movies: model.TrickplaySetting{Mode: string(t.Movies.Mode), Keep: t.Movies.Keep},
		Series: model.TrickplaySetting{Mode: string(t.Series.Mode), Keep: t.Series.Keep, Scope: string(t.Series.Scope)},
	}
}

// libraryRequest maps the contract request with its folders.
func libraryRequest(body gen.LibraryRequest) model.LibraryRequest {
	req := model.LibraryRequest{Name: body.Name, Kind: string(body.Kind), Roots: rootRequests(body.Roots), Trickplay: libraryTrickplay(body.Trickplay)}
	if body.DefaultRating != nil {
		r := int(*body.DefaultRating)
		req.DefaultRating = &r
	}
	return req
}

// rootRequests maps the folders of a request; a nil list stays an empty list.
func rootRequests(roots []gen.LibraryRootRequest) []model.LibraryRootRequest {
	out := make([]model.LibraryRootRequest, 0, len(roots))
	for _, r := range roots {
		out = append(out, model.LibraryRootRequest{Path: r.Path, Content: string(r.Content), Providers: providerNames(r.Providers)})
	}
	return out
}

func providerNames(ps []gen.MetadataProvider) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, string(p))
	}
	return out
}

func providerEnums(ps []string) []gen.MetadataProvider {
	out := make([]gen.MetadataProvider, 0, len(ps))
	for _, p := range ps {
		out = append(out, gen.MetadataProvider(p))
	}
	return out
}

// toRating maps an optional FSK rating.
func toRating(r *int) *gen.Rating {
	if r == nil {
		return nil
	}
	v := gen.Rating(*r)
	return &v
}

// toChapters maps a chapter list (never nil, the contract requires the array): playback, the
// download manifest and the audiobook detail share it.
func toChapters(chapters []model.Chapter) []gen.Chapter {
	out := make([]gen.Chapter, 0, len(chapters))
	for _, c := range chapters {
		ch := gen.Chapter{StartMs: c.StartMs, EndMs: c.EndMs, Title: optString(c.Title), ImageUrl: optString(c.ImageURL)}
		if c.Auto {
			ch.Auto = &c.Auto
		}
		out = append(out, ch)
	}
	return out
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
