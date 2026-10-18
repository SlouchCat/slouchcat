// Package api wires the HTTP surface of Slouchcat: the generated strict server for the JSON API
// under /api/v1 (contract api/openapi.yaml), the binary handlers for images, media streams and the
// spike download, RFC 9457 problem responses, request IDs, request logging, panic recovery, the
// CSRF header rule and the session middleware that enforces x-visibility per route. Everything
// outside /api/v1 goes to the web app handler. The web app and the JSON routes are gzipped
// (compressText); the binary routes are not.
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

// Binary endpoints are excluded from the generated server (oapi-codegen.yaml) and therefore from
// the embedded spec, so their x-visibility is repeated here; the contract test checks that these
// values match api/openapi.yaml.
const (
	binaryImagePath  = "/items/{id}/images/{kind}/{size}"
	binaryStreamPath = "/items/{id}/stream"
	binarySpikePath  = "/spike/{variant}"
)

var binaryRoutes = map[string]string{
	"GET " + BasePath + binaryImagePath:  "scoped",
	"GET " + BasePath + binaryStreamPath: "scoped",
	"GET " + BasePath + binarySpikePath:  "public",
}

// Options configure NewHandler.
type Options struct {
	Version string       // Reported by getHealth.
	Started time.Time    // Process start, for uptime.
	Logger  *slog.Logger // Redacting logger.
	Web     http.Handler // Serves every path outside BasePath.
	Dev     bool         // Development mode: problem details of panics include the panic value.

	Auth     AuthService
	Admin    AdminService
	Catalog  CatalogService
	Playback PlaybackService
	SpikeDir string // Directory with the spike download files; empty disables the endpoint.

	now func() time.Time // Overrides the clock in tests.
}

// NewHandler returns the root handler of the server.
func NewHandler(o Options) http.Handler {
	if o.now == nil {
		o.now = time.Now
	}
	p := problems{log: o.Logger, dev: o.Dev}
	spec, err := gen.GetSpec()
	if err != nil {
		panic(err) // the embedded contract is generated from api/openapi.yaml and always parses
	}
	visibility := visibilityOf(spec, BasePath)
	for route, vis := range binaryRoutes {
		visibility[route] = vis
	}
	authorize := p.authorize(o.Auth, visibility)
	s := &server{opts: o, p: p}

	r := chi.NewRouter()
	r.Use(requestID, requestLog(o.Logger), p.recoverer)
	r.Route(BasePath, func(r chi.Router) {
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			p.write(w, r, http.StatusNotFound, "not_found", "No API route matches this path.")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			p.write(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "The route does not support this method.")
		})
		strict := gen.NewStrictHandlerWithOptions(s, nil, gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  p.badRequest,
			ResponseErrorHandlerFunc: p.internal,
		})
		gen.HandlerWithOptions(strict, gen.ChiServerOptions{
			BaseRouter:       r,
			Middlewares:      []gen.MiddlewareFunc{authorize, compressText},
			ErrorHandlerFunc: p.badRequest,
		})
		// Binary endpoints (CLAUDE.md rule 6) share the session middleware.
		r.With(authorize).Get(binaryImagePath, s.getItemImage)
		r.With(authorize).Get(binaryStreamPath, s.getItemStream)
		r.With(authorize).Get(binarySpikePath, s.getSpikeFile)
	})
	r.Handle("/*", compressText(o.Web))
	return r
}

// server implements the generated strict server interface; the handlers live in one file per
// feature (auth_handlers.go, admin_handlers.go, item_handlers.go, …).
type server struct {
	opts Options
	p    problems
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
