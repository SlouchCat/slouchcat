package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Base stubs of M5 (501 not_implemented); the owning stream replaces them (M5 design section 0).

func (e serviceProblem) VisitSetOfflinePinResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitRemoveOfflinePinResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUploadOfflineEventsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) SetOfflinePin(ctx context.Context, _ gen.SetOfflinePinRequestObject) (gen.SetOfflinePinResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) RemoveOfflinePin(ctx context.Context, _ gen.RemoveOfflinePinRequestObject) (gen.RemoveOfflinePinResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) UploadOfflineEvents(ctx context.Context, _ gen.UploadOfflineEventsRequestObject) (gen.UploadOfflineEventsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}
