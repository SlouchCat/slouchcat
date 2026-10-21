package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M7 curation methods (design section 3.2). The base commit answers model.ErrNotImplemented;
// stream B1 replaces this file with picks, collections, the row order and the admin item search,
// stream B2 the Trakt state.

func (s *Service) ListTopPicks(context.Context) ([]model.AdminPick, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) CreateTopPicks(context.Context, model.PickRequest) ([]model.AdminPick, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) UpdateTopPick(context.Context, int64, model.PickUpdate) (model.AdminPick, error) {
	return model.AdminPick{}, model.ErrNotImplemented
}

func (s *Service) DeleteTopPick(context.Context, int64) error { return model.ErrNotImplemented }

func (s *Service) ReorderTopPicks(context.Context, []int64) error { return model.ErrNotImplemented }

func (s *Service) SearchItems(context.Context, string, int) ([]model.AdminItemHit, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) ListCollections(context.Context) ([]model.AdminCollection, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) GetCollection(context.Context, string) (model.AdminCollection, error) {
	return model.AdminCollection{}, model.ErrNotImplemented
}

func (s *Service) CreateCollection(context.Context, string) (model.AdminCollection, error) {
	return model.AdminCollection{}, model.ErrNotImplemented
}

func (s *Service) UpdateCollection(context.Context, string, string) (model.AdminCollection, error) {
	return model.AdminCollection{}, model.ErrNotImplemented
}

func (s *Service) DeleteCollection(context.Context, string) error { return model.ErrNotImplemented }

func (s *Service) SetCollectionItems(context.Context, string, []string) (model.AdminCollection, error) {
	return model.AdminCollection{}, model.ErrNotImplemented
}

func (s *Service) HomeRows(context.Context) ([]model.HomeRowSetting, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) SetHomeRows(context.Context, []model.HomeRowSetting) ([]model.HomeRowSetting, error) {
	return nil, model.ErrNotImplemented
}

func (s *Service) TraktStatus(context.Context) (model.TraktStatus, error) {
	return model.TraktStatus{}, model.ErrNotImplemented
}

func (s *Service) RefreshTrakt(context.Context) error { return model.ErrNotImplemented }
