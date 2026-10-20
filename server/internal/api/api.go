// Package api wires the HTTP surface of Slouchcat: the generated strict server for the JSON API
// under /api/v1 (contract api/openapi.yaml), the binary handlers for images, media streams and the
// spike download, the HLS endpoints of playback sessions (stream token, stream_token.go), RFC 9457
// problem responses, request IDs, request logging, panic recovery, Referrer-Policy, the CSRF
// header rule and the session middleware that enforces x-visibility per route. Everything
// outside /api/v1 goes to the web app handler; the start page's app shell carries a preload
// header for the hero image (preload.go). The web app and the JSON routes are gzipped
// (compressText); the binary routes are not.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
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
	hlsMasterPath    = "/playback/sessions/{id}/master.m3u8"
	hlsMediaPath     = "/playback/sessions/{id}/media.m3u8"
	hlsInitPath      = "/playback/sessions/{id}/init.mp4"
	hlsSegmentPath   = "/playback/sessions/{id}/seg/{n}"
	subtitlePath     = "/playback/sessions/{id}/subtitles/{track}.vtt"
	// subtitleRoute mounts subtitlePath: chi does not split "{track}.vtt" inside one segment, so the
	// handler parses the segment (getPlaybackSubtitle).
	subtitleRoute = "/playback/sessions/{id}/subtitles/{track}"
)

var binaryRoutes = map[string]string{
	"GET " + BasePath + binaryImagePath:  "scoped",
	"GET " + BasePath + binaryStreamPath: "scoped",
	"GET " + BasePath + binarySpikePath:  "public",
	"GET " + BasePath + hlsMasterPath:    "scoped",
	"GET " + BasePath + hlsMediaPath:     "scoped",
	"GET " + BasePath + hlsInitPath:      "scoped",
	"GET " + BasePath + hlsSegmentPath:   "scoped",
	"GET " + BasePath + subtitlePath:     "scoped",
}

// Options configure NewHandler.
type Options struct {
	Version string       // Reported by getHealth.
	Started time.Time    // Process start, for uptime.
	Logger  *slog.Logger // Redacting logger.
	Web     http.Handler // Serves every path outside BasePath.
	Dev     bool         // Development mode: problem details of panics include the panic value.

	Auth     AuthService
	Profiles ProfileService // Profile picker, PIN and profile management (account operations).
	Access   AccessService  // Invitations and guest accounts.
	Admin    AdminService
	Catalog  CatalogService
	Playback PlaybackService
	SpikeDir string // Directory with the spike download files; empty disables the endpoint.

	// TrustedProxies are the networks whose X-Forwarded-For header is believed (config
	// SLOUCHCAT_TRUSTED_PROXIES); empty ignores the header.
	TrustedProxies []netip.Prefix

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
	r.Use(securityHeaders, requestID, requestLog(o.Logger), p.recoverer, MaxBody(MaxBodyBytes))
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
		// Binary endpoints (CLAUDE.md rule 6) share the session middleware; the media stream and the
		// HLS endpoints are scoped through the stream token instead of the cookie (plan 5.4).
		streamAuth := p.streamAuth(o.Playback)
		r.With(authorize).Get(binaryImagePath, s.getItemImage)
		r.With(authorize).Get(binarySpikePath, s.getSpikeFile)
		r.With(streamAuth).Get(binaryStreamPath, s.getItemStream)
		r.With(streamAuth).Get(hlsMasterPath, s.getPlaybackMaster)
		r.With(streamAuth).Get(hlsMediaPath, s.getPlaybackMedia)
		r.With(streamAuth).Get(hlsInitPath, s.getPlaybackInit)
		r.With(streamAuth).Get(hlsSegmentPath, s.getPlaybackSegment)
		r.With(streamAuth).Get(subtitleRoute, s.getPlaybackSubtitle)
	})
	// The catalog names the first hero slide when it implements heroPicker (discover.Service does);
	// the start page's app shell then carries a preload header for its image (preload.go).
	picker, _ := o.Catalog.(heroPicker)
	r.Handle("/*", compressText(startPagePreload(o.Auth, picker, o.Logger)(o.Web)))
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
