package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// uploadItemImage takes an image the administrator uploads for a kind of a title (concept 15 7.5):
// an http.Handler (CLAUDE.md rule 6) behind the session middleware (x-visibility admin), body limit
// MaxImageUploadBodyBytes (largeBodies). The title (404) and the kind (422, AdminItem.imageKinds)
// are checked before the body is read. Like restoreBackupUpload it then reads the multipart parts
// in order and takes the part "file", at most model.MaxImageUploadBytes; the service checks the
// format by content and the pixel count before it stores anything. The answer is the
// administration view, like setItemImage.
func (s *server) uploadItemImage(w http.ResponseWriter, r *http.Request) {
	fileError := func(code string) {
		s.p.writeValidation(w, r, (&model.ValidationError{}).Add("file", code))
	}
	itemID, kind := chi.URLParam(r, "id"), chi.URLParam(r, "kind")
	cur, err := s.opts.Admin.GetAdminItem(r.Context(), itemID)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	if !slices.Contains(cur.ImageKinds, kind) {
		s.p.writeValidation(w, r, (&model.ValidationError{}).Add("kind", "invalid_value"))
		return
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
	it, err := s.opts.Admin.UploadItemImage(r.Context(), itemID, kind, data)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(toAdminItem(it)); err != nil {
		s.p.log.Debug("write upload response", "err", err)
	}
}
