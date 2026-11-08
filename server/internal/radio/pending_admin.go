package radio

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The administration operations of contract 1.19.0 answer 501 until the stage 2 build (lane B:
// own.go) replaces them; the last of those steps deletes this file.

// CreateStation is implemented by the own stations.
func (s *Service) CreateStation(context.Context, model.AdminRadioStationCreate) (model.AdminRadioStation, error) {
	return model.AdminRadioStation{}, model.ErrNotImplemented
}

// DeleteStation is implemented by the own stations.
func (s *Service) DeleteStation(context.Context, int64) error { return model.ErrNotImplemented }
