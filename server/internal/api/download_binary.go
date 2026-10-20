package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Download-token authorisation (M5 design 2): the prepared download file and its WebVTT sidecars
// accept no cookie; they are authorised by the query parameter dt, which names the download and the
// profile it was issued to. The fetch runs in the background of the web app (also after a profile
// switch), and every request re-checks the download and that the profile still sees the title.

var errNoOffline = errors.New("offline service not configured")

type downloadGrantKey struct{}

// downloadGrantFrom returns the download grant the downloadAuth middleware resolved.
func downloadGrantFrom(ctx context.Context) (model.DownloadGrant, bool) {
	g, ok := ctx.Value(downloadGrantKey{}).(model.DownloadGrant)
	return g, ok
}

// downloadAuth resolves the download token of a download route: a missing or forged token answers
// 401 unauthenticated, a token whose profile cannot see the title (any more) 404 not_found, and so
// does a token of another download than the path names.
func (p problems) downloadAuth(off OfflineService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if off == nil {
				p.write(w, r, http.StatusUnauthorized, "unauthenticated", "")
				return
			}
			g, err := off.AuthorizeDownload(r.Context(), r.URL.Query().Get("dt"))
			if err != nil {
				p.mapError(w, r, err)
				return
			}
			if chi.URLParam(r, "id") != g.DownloadID {
				p.write(w, r, http.StatusNotFound, "not_found", "")
				return
			}
			ctx := context.WithValue(r.Context(), requestKey{}, r)
			ctx = context.WithValue(ctx, downloadGrantKey{}, g)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getDownloadFile serves the fragmented MP4 of a prepared download with Range support (binary
// endpoint, CLAUDE.md rule 6): http.ServeContent answers 206 for a satisfiable range and 416
// otherwise; a removed server copy answers 410 download_evicted.
func (s *server) getDownloadFile(w http.ResponseWriter, r *http.Request) {
	g, ok := downloadGrantFrom(r.Context())
	if !ok || s.opts.Offline == nil {
		s.p.internal(w, r, errNoOffline)
		return
	}
	src, err := s.opts.Offline.OpenFile(r.Context(), g)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// getDownloadSubtitle serves a WebVTT sidecar of a prepared download (binary endpoint), authorised
// like getDownloadFile; a track the download does not carry answers 404.
func (s *server) getDownloadSubtitle(w http.ResponseWriter, r *http.Request) {
	g, ok := downloadGrantFrom(r.Context())
	if !ok || s.opts.Offline == nil {
		s.p.internal(w, r, errNoOffline)
		return
	}
	name := chi.URLParam(r, "track")
	track, err := strconv.ParseInt(strings.TrimSuffix(name, ".vtt"), 10, 64)
	if !strings.HasSuffix(name, ".vtt") || err != nil || track <= 0 {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	src, err := s.opts.Offline.OpenSubtitle(r.Context(), g, track)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}
