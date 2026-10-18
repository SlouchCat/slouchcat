package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Handlers of the start page, titles, images, playback and progress (contract tag items).
// Implemented by the catalog/playback executor; the stubs below only keep the package compiling.

func (s *server) GetHome(context.Context, gen.GetHomeRequestObject) (gen.GetHomeResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetItem(context.Context, gen.GetItemRequestObject) (gen.GetItemResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) StartPlayback(context.Context, gen.StartPlaybackRequestObject) (gen.StartPlaybackResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) ReportProgress(context.Context, gen.ReportProgressRequestObject) (gen.ReportProgressResponseObject, error) {
	return nil, errNotImplemented
}

// getItemImage serves a poster or backdrop with scope (binary endpoint, CLAUDE.md rule 6).
func (s *server) getItemImage(w http.ResponseWriter, r *http.Request) {
	s.p.write(w, r, http.StatusNotFound, "not_found", "")
}

// getItemStream serves the media file with Range support (binary endpoint, CLAUDE.md rule 6).
func (s *server) getItemStream(w http.ResponseWriter, r *http.Request) {
	s.p.write(w, r, http.StatusNotFound, "not_found", "")
}
