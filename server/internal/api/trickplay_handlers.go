package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Trickplay (concept 18 part A, contract 1.23.0): the preview set of a media file to ask again while
// it is made (getTrickplay, strict server), its sheets (getTrickplaySheet) and the sheets of a
// prepared download (getDownloadTrickplaySheet), both binary endpoints (CLAUDE.md rule 6).

func (e serviceProblem) VisitGetTrickplayResponse(w http.ResponseWriter) error { return e.visit(w) }

// GetTrickplay returns the preview set of the media file of a visible title for the active profile.
func (s *server) GetTrickplay(ctx context.Context, req gen.GetTrickplayRequestObject) (gen.GetTrickplayResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok || s.opts.Playback == nil {
		return nil, errNoItemsServices
	}
	tp, err := s.opts.Playback.Trickplay(ctx, session.Profile, req.Id, req.Params.MediaFileId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetTrickplay200JSONResponse(*toTrickplay(&tp)), nil
}

// toTrickplay maps a preview set; nil stays nil.
func toTrickplay(tp *model.Trickplay) *gen.Trickplay {
	if tp == nil {
		return nil
	}
	out := &gen.Trickplay{
		Version: tp.Version, State: gen.TrickplayState(tp.State), IntervalMs: int(tp.IntervalMs),
		TileWidth: tp.TileWidth, TileHeight: tp.TileHeight, Columns: tp.Columns, Rows: tp.Rows,
		PointsMs: tp.PointsMs, Sheets: make([]gen.TrickplaySheet, 0, len(tp.Sheets)),
	}
	if out.PointsMs == nil {
		out.PointsMs = []int64{}
	}
	for _, sh := range tp.Sheets {
		out.Sheets = append(out.Sheets, gen.TrickplaySheet{Index: sh.Index, Url: sh.URL})
	}
	return out
}

// sheetNumber parses the decimal sheet number of a trickplay route; ok is false for anything else.
func sheetNumber(r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "sheet")
	n, err := strconv.Atoi(raw)
	return n, err == nil && n >= 0 && strconv.Itoa(n) == raw
}

// getTrickplaySheet serves a sheet of the preview set of a media file of a visible title (binary
// endpoint) for the active profile. The sheet of the file's current version is cached for a year
// when v names that version (the URL of Trickplay.sheets); with another or no v the same sheet goes
// out with no-cache and its ETag.
func (s *server) getTrickplaySheet(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFrom(r.Context())
	if !ok || s.opts.Playback == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	n, ok := sheetNumber(r)
	if !ok {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	src, version, err := s.opts.Playback.TrickplaySheet(r.Context(), session.Profile, chi.URLParam(r, "id"), r.URL.Query().Get("mediaFileId"), n)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("ETag", src.ETag)
	if v := r.URL.Query().Get("v"); v != "" && v == version {
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-cache")
	}
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// getDownloadTrickplaySheet serves a sheet of the preview set copied into a prepared download
// (binary endpoint), authorised like getDownloadFile; a sheet the download does not carry answers
// 404.
func (s *server) getDownloadTrickplaySheet(w http.ResponseWriter, r *http.Request) {
	g, ok := downloadGrantFrom(r.Context())
	if !ok || s.opts.Offline == nil {
		s.p.internal(w, r, errNoOffline)
		return
	}
	n, ok := sheetNumber(r)
	if !ok {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	src, err := s.opts.Offline.OpenTrickplaySheet(r.Context(), g, n)
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
