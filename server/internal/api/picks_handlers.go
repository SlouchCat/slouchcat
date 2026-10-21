package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M7 curation operations of the main user (plan M7, design sections 2 and 3.2): Cat's picks,
// collections, the row order and the Trakt state. The base commit answers 501 not_implemented;
// stream B1 replaces these bodies with the admin service.

func (e serviceProblem) VisitListTopPicksResponse(w http.ResponseWriter) error     { return e.visit(w) }
func (e serviceProblem) VisitCreateTopPicksResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitReorderTopPicksResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitUpdateTopPickResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitDeleteTopPickResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitSearchAdminItemsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListCollectionsResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitCreateCollectionResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetAdminCollectionResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateCollectionResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDeleteCollectionResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetCollectionItemsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetHomeRowsResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitSetHomeRowsResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitGetTraktStatusResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRefreshTraktResponse(w http.ResponseWriter) error   { return e.visit(w) }

func (s *server) ListTopPicks(ctx context.Context, _ gen.ListTopPicksRequestObject) (gen.ListTopPicksResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) CreateTopPicks(ctx context.Context, _ gen.CreateTopPicksRequestObject) (gen.CreateTopPicksResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ReorderTopPicks(ctx context.Context, _ gen.ReorderTopPicksRequestObject) (gen.ReorderTopPicksResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) UpdateTopPick(ctx context.Context, _ gen.UpdateTopPickRequestObject) (gen.UpdateTopPickResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) DeleteTopPick(ctx context.Context, _ gen.DeleteTopPickRequestObject) (gen.DeleteTopPickResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) SearchAdminItems(ctx context.Context, _ gen.SearchAdminItemsRequestObject) (gen.SearchAdminItemsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListCollections(ctx context.Context, _ gen.ListCollectionsRequestObject) (gen.ListCollectionsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) CreateCollection(ctx context.Context, _ gen.CreateCollectionRequestObject) (gen.CreateCollectionResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) GetAdminCollection(ctx context.Context, _ gen.GetAdminCollectionRequestObject) (gen.GetAdminCollectionResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) UpdateCollection(ctx context.Context, _ gen.UpdateCollectionRequestObject) (gen.UpdateCollectionResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) DeleteCollection(ctx context.Context, _ gen.DeleteCollectionRequestObject) (gen.DeleteCollectionResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) SetCollectionItems(ctx context.Context, _ gen.SetCollectionItemsRequestObject) (gen.SetCollectionItemsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) GetHomeRows(ctx context.Context, _ gen.GetHomeRowsRequestObject) (gen.GetHomeRowsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) SetHomeRows(ctx context.Context, _ gen.SetHomeRowsRequestObject) (gen.SetHomeRowsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) GetTraktStatus(ctx context.Context, _ gen.GetTraktStatusRequestObject) (gen.GetTraktStatusResponseObject, error) {
	st, err := s.opts.Admin.TraktStatus(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetTraktStatus200JSONResponse{Configured: st.Configured, LastSuccess: utc(st.LastSuccess),
		LastAttempt: utc(st.LastAttempt), LastError: optString(st.LastError), Trending: st.Trending, Popular: st.Popular}, nil
}

func (s *server) RefreshTrakt(ctx context.Context, _ gen.RefreshTraktRequestObject) (gen.RefreshTraktResponseObject, error) {
	if err := s.opts.Admin.RefreshTrakt(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RefreshTrakt202Response{}, nil
}
