// Package api wires the HTTP surface of Slouchcat: the generated strict server for the JSON API
// under /api/v1 (contract api/openapi.yaml), the binary handlers for images, media streams, the
// the backup download, the HLS endpoints of playback sessions (stream token,
// stream_token.go), RFC 9457 problem responses, request IDs, request logging, panic recovery,
// Referrer-Policy, the CSRF header rule and the session middleware that enforces x-visibility per
// route. Everything outside /api/v1 goes to the web app handler; the start page's app shell
// carries a preload header for the hero image (preload.go). The web app and the JSON routes are
// gzipped (compressText); the binary routes are not.
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
	hlsMasterPath    = "/playback/sessions/{id}/master.m3u8"
	hlsMediaPath     = "/playback/sessions/{id}/media.m3u8"
	hlsInitPath      = "/playback/sessions/{id}/init.mp4"
	hlsSegmentPath   = "/playback/sessions/{id}/seg/{n}"
	subtitlePath     = "/playback/sessions/{id}/subtitles/{track}.vtt"
	// subtitleRoute mounts subtitlePath: chi does not split "{track}.vtt" inside one segment, so the
	// handler parses the segment (getPlaybackSubtitle).
	subtitleRoute = "/playback/sessions/{id}/subtitles/{track}"
	// Prepared downloads (M5), authorised by the download token dt (download_binary.go); the
	// subtitle route parses "{track}.vtt" like subtitleRoute.
	downloadFilePath      = "/downloads/{id}/file"
	downloadSubtitlePath  = "/downloads/{id}/subtitles/{track}.vtt"
	downloadSubtitleRoute = "/downloads/{id}/subtitles/{track}"
	// adminImagePath serves the images of the administration tables system-wide (M11 step 5b).
	adminImagePath = "/admin/items/{id}/images/{kind}/{size}"
	// candidatePreviewPath serves a local image file of a title as preview of the image choice
	// (concept 10 9.2).
	candidatePreviewPath = "/admin/items/{id}/image-candidates/preview"
	// chapterImagePath serves the image of a chapter of a film or an episode (concept 13 stage 3).
	chapterImagePath = "/items/{id}/chapters/{idx}/image"
	// itemImageUploadPath takes an image the administrator uploads for a kind of a title
	// (concept 15 7.5); multipart, larger body limit (largeBodies).
	itemImageUploadPath = "/admin/items/{id}/images/{kind}/upload"
	// Webradio (concept 16 stage 1, contract 1.17.0): the live HLS of a listen session, authorised
	// by the listen token rt (radioAuth, radio_binary.go), the station images for viewers and the
	// administration, and the logo upload (multipart, largeBodies).
	radioLivePlaylistPath = "/radio/live/{listenId}/index.m3u8"
	radioLiveInitPath     = "/radio/live/{listenId}/init/{gen}"
	radioLiveSegmentPath  = "/radio/live/{listenId}/seg/{n}"
	radioImagePath        = "/radio/images/{id}/{kind}/{size}"
	adminRadioImagePath   = "/admin/radio/images/{id}/{kind}/{size}"
	radioLogoUploadPath   = "/admin/radio/stations/{stationId}/logo"
	// The cover of a title (concept 16 stage 2, contract 1.19.0).
	radioCoverPath = "/radio/covers/{trackId}/{size}"
)

var binaryRoutes = map[string]string{
	"GET " + BasePath + binaryImagePath:  "scoped",
	"GET " + BasePath + chapterImagePath: "scoped",
	"GET " + BasePath + binaryStreamPath: "scoped",
	"GET " + BasePath + hlsMasterPath:    "scoped",
	"GET " + BasePath + hlsMediaPath:     "scoped",
	"GET " + BasePath + hlsInitPath:      "scoped",
	"GET " + BasePath + hlsSegmentPath:   "scoped",
	"GET " + BasePath + subtitlePath:     "scoped",

	"GET " + BasePath + downloadFilePath:     "scoped",
	"GET " + BasePath + downloadSubtitlePath: "scoped",

	"GET " + BasePath + backupDownloadPath:   "admin",
	"GET " + BasePath + adminImagePath:       "admin",
	"GET " + BasePath + candidatePreviewPath: "admin",
	"PUT " + BasePath + itemImageUploadPath:  "admin",

	"GET " + BasePath + radioLivePlaylistPath: "scoped",
	"GET " + BasePath + radioLiveInitPath:     "scoped",
	"GET " + BasePath + radioLiveSegmentPath:  "scoped",
	"GET " + BasePath + radioImagePath:        "scoped",
	"GET " + BasePath + radioCoverPath:        "scoped",
	"GET " + BasePath + adminRadioImagePath:   "admin",
	"PUT " + BasePath + radioLogoUploadPath:   "admin",
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
	Offline  OfflineService // Downloads, sync and the download token (M5).
	Notify   NotifyService  // In-app inbox (M7, E-8), channels and announcements (M8); nil answers 501 on them.
	// Integrations receives the webhooks of Radarr and Sonarr (M9, I-8); nil answers 501.
	Integrations IntegrationService
	// Radio is the webradio (concept 16, contract 1.17.0); nil answers 501 on the JSON operations,
	// the station images and the logo upload, and 401 on the live routes (radioAuth).
	Radio RadioService

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
			Middlewares:      []gen.MiddlewareFunc{pickUpdateNulls, authorize, compressText},
			ErrorHandlerFunc: p.badRequest,
		})
		// Binary endpoints (CLAUDE.md rule 6) share the session middleware; the media stream and the
		// HLS endpoints are scoped through the stream token instead of the cookie (plan 5.4), the
		// prepared downloads through the download token (M5). Only the HLS and subtitle routes
		// answer a remux ended for a changed source with 409 (concept 14, 6.4).
		streamAuth := p.streamAuth(o.Playback, false)
		hlsAuth := p.streamAuth(o.Playback, true)
		r.With(authorize).Get(binaryImagePath, s.getItemImage)
		r.With(authorize).Get(chapterImagePath, s.getChapterImage)
		r.With(streamAuth).Get(binaryStreamPath, s.getItemStream)
		r.With(hlsAuth).Get(hlsMasterPath, s.getPlaybackMaster)
		r.With(hlsAuth).Get(hlsMediaPath, s.getPlaybackMedia)
		r.With(hlsAuth).Get(hlsInitPath, s.getPlaybackInit)
		r.With(hlsAuth).Get(hlsSegmentPath, s.getPlaybackSegment)
		r.With(hlsAuth).Get(subtitleRoute, s.getPlaybackSubtitle)
		downloadAuth := p.downloadAuth(o.Offline)
		r.With(downloadAuth).Get(downloadFilePath, s.getDownloadFile)
		r.With(downloadAuth).Get(downloadSubtitleRoute, s.getDownloadSubtitle)
		r.With(authorize).Get(backupDownloadPath, s.downloadBackup)
		r.With(authorize).Get(adminImagePath, s.getAdminItemImage)
		r.With(authorize).Get(candidatePreviewPath, s.getItemImageCandidatePreview)
		r.With(authorize).Put(itemImageUploadPath, s.uploadItemImage)
		radioAuth := p.radioAuth(o.Radio)
		r.With(radioAuth).Get(radioLivePlaylistPath, s.getRadioLivePlaylist)
		r.With(radioAuth).Get(radioLiveInitPath, s.getRadioLiveInit)
		r.With(radioAuth).Get(radioLiveSegmentPath, s.getRadioLiveSegment)
		r.With(authorize).Get(radioImagePath, s.getRadioImage)
		r.With(authorize).Get(radioCoverPath, s.getRadioCover)
		r.With(authorize).Get(adminRadioImagePath, s.getAdminRadioImage)
		r.With(authorize).Put(radioLogoUploadPath, s.uploadRadioStationLogo)
	})
	// The catalog names the first stage title, or the first title without a stage, when it
	// implements heroPicker (discover.Service does); the start page's app shell then carries a
	// preload header for its image (preload.go).
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
		Status:        gen.HealthStatusOk,
		Version:       s.opts.Version,
		UptimeSeconds: max(int64(s.opts.now().Sub(s.opts.Started).Seconds()), 0),
	}, nil
}
