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

func (e serviceProblem) VisitListLibrariesResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCreateLibraryResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetLibraryResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitScanLibraryResponse(w http.ResponseWriter) error   { return e.visit(w) }
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
		root := gen.LibraryRoot{Id: r.ID, Path: r.Path, FsType: r.FSType, Watch: gen.LibraryRootWatch(r.Watch)}
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
	return gen.Library{
		Id: l.ID, Name: l.Name, Kind: gen.LibraryKind(l.Kind), DefaultRating: toRating(l.DefaultRating),
		Roots: roots, Scan: scan, CreatedAt: l.CreatedAt.UTC(),
	}
}

// libraryRequest maps the contract request; the path is optional (none for podcasts).
func libraryRequest(body gen.LibraryRequest) model.LibraryRequest {
	req := model.LibraryRequest{Name: body.Name, Kind: string(body.Kind), Path: deref(body.Path)}
	if body.DefaultRating != nil {
		r := int(*body.DefaultRating)
		req.DefaultRating = &r
	}
	return req
}

// toRating maps an optional FSK rating.
func toRating(r *int) *gen.Rating {
	if r == nil {
		return nil
	}
	v := gen.Rating(*r)
	return &v
}

// toChapters maps a chapter list (never nil, the contract requires the array).
func toChapters(chapters []model.Chapter) []gen.Chapter {
	out := make([]gen.Chapter, 0, len(chapters))
	for _, c := range chapters {
		out = append(out, gen.Chapter{StartMs: c.StartMs, EndMs: c.EndMs, Title: optString(c.Title)})
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
