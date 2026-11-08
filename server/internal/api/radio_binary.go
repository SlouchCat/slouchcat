package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Binary webradio endpoints of concept 16 stage 1 (CLAUDE.md rule 6, contract 1.17.0): the live
// HLS of a listen session, authorised by the listen token rt instead of the cookie (radioAuth),
// the station images for viewers (session middleware, x-visibility scoped) and the administration,
// and the logo upload. Without the radio service the live routes answer 401 like the stream token
// routes without playback, the images and the upload 501.

// radioPlaylistGzipMin is the playlist size from which the playlist handler gzips: compressText
// covers the strict routes only (concept 16 section 12.3), and a short delta list is not worth it.
const radioPlaylistGzipMin = 512

// radioSegmentMaxAge is the cache lifetime of init and media segments: at most the longest buffer
// (60 minutes); a segment never changes under its URL.
const radioSegmentMaxAge = "3600"

type radioGrantKey struct{}

// radioGrantFrom returns the grant the radioAuth middleware resolved.
func radioGrantFrom(ctx context.Context) (model.RadioGrant, bool) {
	g, ok := ctx.Value(radioGrantKey{}).(model.RadioGrant)
	return g, ok
}

// radioAuth resolves the listen token rt of a live route for the listen session in the path: a
// missing, forged, foreign, ended or expired token answers 401 unauthenticated; so does every live
// route of a server without the radio service.
func (p problems) radioAuth(radio RadioService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if radio == nil {
				p.write(w, r, http.StatusUnauthorized, "unauthenticated", "")
				return
			}
			g, err := radio.AuthorizeLive(r.Context(), chi.URLParam(r, "listenId"), r.URL.Query().Get("rt"))
			if errors.Is(err, model.ErrNotFound) {
				err = model.ErrUnauthenticated
			}
			if err != nil {
				p.mapError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), requestKey{}, r)
			ctx = context.WithValue(ctx, radioGrantKey{}, g)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getRadioLivePlaylist serves the live playlist of a listen session; gzip when the client takes
// it and the list is longer than radioPlaylistGzipMin.
func (s *server) getRadioLivePlaylist(w http.ResponseWriter, r *http.Request) {
	g, ok := radioGrantFrom(r.Context())
	if !ok || s.opts.Radio == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	body, err := s.opts.Radio.Playlist(r.Context(), g, r.URL.Query().Get("_HLS_skip"))
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", mediaTypeHLS)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	h.Add("Vary", "Accept-Encoding")
	if len(body) > radioPlaylistGzipMin && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.DefaultCompression) // the level is valid
		_, _ = zw.Write(body)                                       // writes to a bytes.Buffer never fail
		_ = zw.Close()
		body = buf.Bytes()
		h.Set("Content-Encoding", "gzip")
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if _, err := w.Write(body); err != nil { //nolint:gosec // G705: playlist (not HTML, nosniff), the service URL-escapes the token it puts in
		s.p.log.Debug("write radio playlist", "err", err)
	}
}

// getRadioLiveInit serves the initialisation segment of a relay generation; a gen that is no
// decimal number answers 404.
func (s *server) getRadioLiveInit(w http.ResponseWriter, r *http.Request) {
	g, ok := radioGrantFrom(r.Context())
	if !ok || s.opts.Radio == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	raw := chi.URLParam(r, "gen")
	gen, err := strconv.Atoi(raw)
	if err != nil || gen < 0 || strconv.Itoa(gen) != raw {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	body, err := s.opts.Radio.Init(r.Context(), g, gen)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	writeRadioSegment(s, w, body, nil)
}

// getRadioLiveSegment serves one media segment; the write is cut when the listen session is
// revoked while the client reads slowly.
func (s *server) getRadioLiveSegment(w http.ResponseWriter, r *http.Request) {
	g, ok := radioGrantFrom(r.Context())
	if !ok || s.opts.Radio == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	raw := chi.URLParam(r, "n")
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 || strconv.FormatInt(n, 10) != raw {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	body, done, err := s.opts.Radio.Segment(r.Context(), g, n)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	writeRadioSegment(s, w, body, done)
}

// writeRadioSegment writes an fMP4 init or media segment; a closing done cuts the write.
func writeRadioSegment(s *server, w http.ResponseWriter, body []byte, done <-chan struct{}) {
	if done != nil {
		defer abortOnRevoke(w, done)()
	}
	h := w.Header()
	h.Set("Content-Type", "audio/mp4")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, max-age="+radioSegmentMaxAge+", immutable")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if _, err := w.Write(body); err != nil {
		s.p.log.Debug("write radio segment", "err", err)
	}
}

// radioImageSizes are the sizes of a station image (contract RadioImageSize), per kind.
var radioImageSizes = map[string]map[string]bool{
	"logo": {"w96": true, "w256": true, "w512": true},
}

// getRadioImage serves a station image for the active profile (viewer rules: a hidden or excluded
// station answers 404).
func (s *server) getRadioImage(w http.ResponseWriter, r *http.Request) {
	s.serveRadioImage(w, r, false)
}

// getAdminRadioImage serves a station image for the administration, hidden stations included.
func (s *server) getAdminRadioImage(w http.ResponseWriter, r *http.Request) {
	s.serveRadioImage(w, r, true)
}

func (s *server) serveRadioImage(w http.ResponseWriter, r *http.Request, admin bool) {
	if s.opts.Radio == nil {
		s.p.mapError(w, r, model.ErrNotImplemented)
		return
	}
	raw, kind, size := chi.URLParam(r, "id"), chi.URLParam(r, "kind"), chi.URLParam(r, "size")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != raw || !radioImageSizes[kind][size] {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	w.Header().Add("Vary", "Accept")
	src, err := s.opts.Radio.Image(r.Context(), id, kind, size, acceptsWebP(r.Header.Values("Accept")), admin)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// getRadioCover serves the cover of a title for the active profile (contract 1.19.0): any profile
// session may read it, a cover names no station; an unknown or expired track answers 404.
func (s *server) getRadioCover(w http.ResponseWriter, r *http.Request) {
	if s.opts.Radio == nil {
		s.p.mapError(w, r, model.ErrNotImplemented)
		return
	}
	raw, size := chi.URLParam(r, "trackId"), chi.URLParam(r, "size")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != raw || !radioImageSizes["logo"][size] {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	w.Header().Add("Vary", "Accept")
	src, err := s.opts.Radio.Cover(r.Context(), id, size, acceptsWebP(r.Header.Values("Accept")))
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// uploadRadioStationLogo takes a logo the administrator uploads for a station (x-visibility admin,
// body limit MaxImageUploadBodyBytes through largeBodies): the multipart part "file", at most
// model.MaxImageUploadBytes, like uploadItemImage. The service checks the format by content and
// the pixel count. The answer is the administration view of the station.
func (s *server) uploadRadioStationLogo(w http.ResponseWriter, r *http.Request) {
	if s.opts.Radio == nil {
		s.p.mapError(w, r, model.ErrNotImplemented)
		return
	}
	raw := chi.URLParam(r, "stationId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != raw {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	if _, err := s.opts.Radio.AdminStation(r.Context(), id); err != nil {
		s.p.mapError(w, r, err)
		return
	}
	fileError := func(code string) {
		s.p.writeValidation(w, r, (&model.ValidationError{}).Add("file", code))
	}
	mr, err := r.MultipartReader()
	if err != nil {
		fileError("invalid_value")
		return
	}
	var data []byte
	for {
		part, err := mr.NextPart()
		var maxErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			fileError("required")
			return
		case errors.As(err, &maxErr):
			fileError("too_large")
			return
		case err != nil:
			fileError("invalid_value")
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		data, err = io.ReadAll(io.LimitReader(part, model.MaxImageUploadBytes+1))
		_ = part.Close()
		switch {
		case errors.As(err, &maxErr) || len(data) > model.MaxImageUploadBytes:
			fileError("too_large")
			return
		case err != nil:
			fileError("invalid_value")
			return
		}
		break
	}
	a, err := s.opts.Radio.SetLogo(r.Context(), id, bytes.NewReader(data))
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(toAdminRadioStation(a)); err != nil {
		s.p.log.Debug("write upload response", "err", err)
	}
}
