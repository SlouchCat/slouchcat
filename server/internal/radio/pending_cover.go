package radio

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Cover of contract 1.19.0 answers 501 until the cover chain of the stage 2 build (lane A, cover.go)
// replaces it; that step deletes this file.
func (s *Service) Cover(context.Context, int64, string, bool) (model.StreamSource, error) {
	return model.StreamSource{}, model.ErrNotImplemented
}
