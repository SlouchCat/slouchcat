package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The folder browser of the library form (idea 02). The contract comes first and the handlers
// follow in the next commit: the generated strict server is one interface, and a server that
// does not implement all of it does not compile (api.go, var _ gen.StrictServerInterface).

func (e serviceProblem) VisitBrowseServerPathsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListBrowseRootsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListSetupRootsResponse(w http.ResponseWriter) error  { return e.visit(w) }

func (s *server) BrowseServerPaths(ctx context.Context, _ gen.BrowseServerPathsRequestObject) (gen.BrowseServerPathsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListBrowseRoots(ctx context.Context, _ gen.ListBrowseRootsRequestObject) (gen.ListBrowseRootsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) ListSetupRoots(ctx context.Context, _ gen.ListSetupRootsRequestObject) (gen.ListSetupRootsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}
