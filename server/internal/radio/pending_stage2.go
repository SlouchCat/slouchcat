package radio

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The operations of contract 1.19.0 that the stage 2 build steps implement (build plan
// .omc/plans/webradio-stufe2.md): until then they answer 501. Lane A replaces Cover (cover.go),
// lane B the station and rule operations (own.go, tagrules.go); the last of them deletes this file.

// Cover is implemented by the cover chain.
func (s *Service) Cover(context.Context, int64, string, bool) (model.StreamSource, error) {
	return model.StreamSource{}, model.ErrNotImplemented
}

// CreateStation is implemented by the own stations.
func (s *Service) CreateStation(context.Context, model.AdminRadioStationCreate) (model.AdminRadioStation, error) {
	return model.AdminRadioStation{}, model.ErrNotImplemented
}

// DeleteStation is implemented by the own stations.
func (s *Service) DeleteStation(context.Context, int64) error { return model.ErrNotImplemented }

// TagRules is implemented by the tag rules.
func (s *Service) TagRules(context.Context) (model.RadioTagRules, error) {
	return model.RadioTagRules{}, model.ErrNotImplemented
}

// SetTagRules is implemented by the tag rules.
func (s *Service) SetTagRules(context.Context, []model.RadioTagRule) (model.RadioTagRules, error) {
	return model.RadioTagRules{}, model.ErrNotImplemented
}
