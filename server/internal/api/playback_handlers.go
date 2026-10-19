package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of playback sessions (plan 5.4, M1b): stopPlayback (JSON, cookie session) and the HLS
// endpoints (binary, CLAUDE.md rule 6) authorised by the stream token through streamAuth. URLs
// inside the playlists are relative and carry the same token, so a native player can follow them.

const mediaTypeHLS = "application/vnd.apple.mpegurl"

func (e serviceProblem) VisitStopPlaybackResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) StopPlayback(ctx context.Context, req gen.StopPlaybackRequestObject) (gen.StopPlaybackResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.opts.Playback.Stop(ctx, session, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.StopPlayback204Response{}, nil
}

// hlsStream returns the HLS output of the token's playback session; the path id must be the
// session the token names.
func (s *server) hlsStream(w http.ResponseWriter, r *http.Request) (model.HLSStream, bool) {
	g, ok := grantFrom(r.Context())
	if !ok || s.opts.Playback == nil {
		s.p.internal(w, r, errNoItemsServices)
		return nil, false
	}
	if chi.URLParam(r, "id") != g.ID {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return nil, false
	}
	hs, err := s.opts.Playback.HLS(r.Context(), g)
	if err != nil {
		s.p.mapError(w, r, err)
		return nil, false
	}
	return hs, true
}

// tokenQuery is "?st=<token>" for URLs inside playlists.
func tokenQuery(r *http.Request) string {
	return "?st=" + url.QueryEscape(r.URL.Query().Get("st"))
}

// writePlaylist sends a playlist. It echoes the request's token, URL-escaped, in a non-HTML media
// type that browsers must not sniff, so it cannot carry markup.
func writePlaylist(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", mediaTypeHLS)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body) //nolint:gosec // G705: playlist (not HTML, nosniff), the echoed token is URL-escaped
}

func (s *server) getPlaybackMaster(w http.ResponseWriter, r *http.Request) {
	hs, ok := s.hlsStream(w, r)
	if !ok {
		return
	}
	writePlaylist(w, hs.MasterPlaylist("media.m3u8"+tokenQuery(r)))
}

func (s *server) getPlaybackMedia(w http.ResponseWriter, r *http.Request) {
	hs, ok := s.hlsStream(w, r)
	if !ok {
		return
	}
	q := tokenQuery(r)
	writePlaylist(w, hs.MediaPlaylist("init.mp4"+q, func(i int) string { return "seg/" + strconv.Itoa(i) + q }))
}

func (s *server) getPlaybackInit(w http.ResponseWriter, r *http.Request) {
	hs, ok := s.hlsStream(w, r)
	if !ok {
		return
	}
	s.serveFragment(w, r, func(ctx context.Context) (io.ReadSeekCloser, error) { return hs.OpenInit(ctx) })
}

func (s *server) getPlaybackSegment(w http.ResponseWriter, r *http.Request) {
	hs, ok := s.hlsStream(w, r)
	if !ok {
		return
	}
	n, err := strconv.Atoi(chi.URLParam(r, "n"))
	if err != nil || n < 0 {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	s.serveFragment(w, r, func(ctx context.Context) (io.ReadSeekCloser, error) { return hs.OpenSegment(ctx, n) })
}

// serveFragment serves an init or media segment as video/mp4. Segments never change once
// produced, but the token in the URL must not end up in shared caches: private, no-store.
func (s *server) serveFragment(w http.ResponseWriter, r *http.Request, open func(context.Context) (io.ReadSeekCloser, error)) {
	f, err := open(r.Context())
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(r.Context().Err(), context.Canceled) {
			return // the player moved on (seek); nothing to send
		}
		s.p.mapError(w, r, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", time.Time{}, f)
}
