// Package api wires the HTTP surface of Slouchcat: the generated strict server for the JSON API
// under /api/v1 (contract api/openapi.yaml), RFC 9457 problem responses, request IDs, request
// logging and panic recovery. Everything outside /api/v1 goes to the web app handler.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// BasePath is the prefix of all API routes.
const BasePath = "/api/v1"

// Options configure NewHandler.
type Options struct {
	Version string       // Reported by getHealth.
	Started time.Time    // Process start, for uptime.
	Logger  *slog.Logger // Redacting logger.
	Web     http.Handler // Serves every path outside BasePath.
	Dev     bool         // Development mode: problem details of panics include the panic value.

	now func() time.Time // Overrides the clock in tests.
}

// NewHandler returns the root handler of the server.
func NewHandler(o Options) http.Handler {
	if o.now == nil {
		o.now = time.Now
	}
	p := problems{log: o.Logger, dev: o.Dev}

	r := chi.NewRouter()
	r.Use(requestID, requestLog(o.Logger), p.recoverer)
	r.Route(BasePath, func(r chi.Router) {
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			p.write(w, r, http.StatusNotFound, "not_found", "No API route matches this path.")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			p.write(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "The route does not support this method.")
		})
		strict := gen.NewStrictHandlerWithOptions(&server{opts: o}, nil, gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  p.badRequest,
			ResponseErrorHandlerFunc: p.internal,
		})
		gen.HandlerWithOptions(strict, gen.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: p.badRequest})
	})
	r.Handle("/*", o.Web)
	return r
}

// server implements the generated strict server interface.
type server struct {
	opts Options
}

var _ gen.StrictServerInterface = (*server)(nil)

// GetHealth reports liveness. The server only listens after the database is open and
// migrated, so a response always means ready.
func (s *server) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return gen.GetHealth200JSONResponse{
		Status:        gen.Ok,
		Version:       s.opts.Version,
		UptimeSeconds: max(int64(s.opts.now().Sub(s.opts.Started).Seconds()), 0),
	}, nil
}
