package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Temporary stubs for the M3 operations of api/openapi.yaml. Each handler file of M3 replaces the
// stub of its operations and removes it here; the file disappears once every operation is served.

var errNotImplemented = errors.New("operation not implemented yet")

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

func (s *server) CancelLibraryScan(ctx context.Context, _ gen.CancelLibraryScanRequestObject) (gen.CancelLibraryScanResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) GetAdminItem(ctx context.Context, _ gen.GetAdminItemRequestObject) (gen.GetAdminItemResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) SetItemMatch(ctx context.Context, _ gen.SetItemMatchRequestObject) (gen.SetItemMatchResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) RefreshItem(ctx context.Context, _ gen.RefreshItemRequestObject) (gen.RefreshItemResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) CreateItemTrailer(ctx context.Context, _ gen.CreateItemTrailerRequestObject) (gen.CreateItemTrailerResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) DeleteItemTrailer(ctx context.Context, _ gen.DeleteItemTrailerRequestObject) (gen.DeleteItemTrailerResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) SetItemsRating(ctx context.Context, _ gen.SetItemsRatingRequestObject) (gen.SetItemsRatingResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) SearchMetadata(ctx context.Context, _ gen.SearchMetadataRequestObject) (gen.SearchMetadataResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) ListMaintenanceIssues(ctx context.Context, _ gen.ListMaintenanceIssuesRequestObject) (gen.ListMaintenanceIssuesResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) ListDuplicates(ctx context.Context, _ gen.ListDuplicatesRequestObject) (gen.ListDuplicatesResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) MergeItems(ctx context.Context, _ gen.MergeItemsRequestObject) (gen.MergeItemsResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}

func (s *server) CleanupItems(ctx context.Context, _ gen.CleanupItemsRequestObject) (gen.CleanupItemsResponseObject, error) {
	return s.problem(ctx, errNotImplemented), nil
}
