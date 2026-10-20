package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Base stubs of M5 (501 not_implemented); the owning stream replaces them (M5 design section 0).

func (e serviceProblem) VisitGetDownloadOptionsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitCreateDownloadsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDeleteDownloadResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetDownloadManifestResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSyncOfflineResponse(w http.ResponseWriter) error      { return e.visit(w) }
func (e serviceProblem) VisitGetDownloadRulesResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetDownloadRulesResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCheckOfflineRevocationsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) GetDownloadOptions(ctx context.Context, _ gen.GetDownloadOptionsRequestObject) (gen.GetDownloadOptionsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) CreateDownloads(ctx context.Context, _ gen.CreateDownloadsRequestObject) (gen.CreateDownloadsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) DeleteDownload(ctx context.Context, _ gen.DeleteDownloadRequestObject) (gen.DeleteDownloadResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) GetDownloadManifest(ctx context.Context, _ gen.GetDownloadManifestRequestObject) (gen.GetDownloadManifestResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) SyncOffline(ctx context.Context, _ gen.SyncOfflineRequestObject) (gen.SyncOfflineResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) GetDownloadRules(ctx context.Context, _ gen.GetDownloadRulesRequestObject) (gen.GetDownloadRulesResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) SetDownloadRules(ctx context.Context, _ gen.SetDownloadRulesRequestObject) (gen.SetDownloadRulesResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) CheckOfflineRevocations(ctx context.Context, _ gen.CheckOfflineRevocationsRequestObject) (gen.CheckOfflineRevocationsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}
