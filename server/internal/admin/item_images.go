package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The image choice per title (concept 10 6.4, 7.3): package metadata asks the sources and keeps
// the choice; this file answers with the administration view, like the other title operations.

// ItemImageCandidates lists the candidates of every source for one kind of a title.
func (s *Service) ItemImageCandidates(ctx context.Context, itemID, kind string, refresh bool) (model.ImageCandidateList, error) {
	if s.meta == nil {
		return model.ImageCandidateList{}, ErrNotConfigured
	}
	return s.meta.ImageCandidates(ctx, itemID, kind, refresh)
}

// SetItemImage chooses and locks the image of a kind and answers with the administration view.
func (s *Service) SetItemImage(ctx context.Context, itemID, kind, provider, source string) (model.AdminItem, error) {
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	if s.meta == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	if err := s.meta.SetItemImage(ctx, itemID, kind, provider, source); err != nil {
		return model.AdminItem{}, err
	}
	return s.GetAdminItem(ctx, itemID)
}

// ClearItemImage lifts the choice and the lock of a kind and answers with the administration view.
func (s *Service) ClearItemImage(ctx context.Context, itemID, kind string) (model.AdminItem, error) {
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	if s.meta == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	if err := s.meta.ClearItemImage(ctx, itemID, kind); err != nil {
		return model.AdminItem{}, err
	}
	return s.GetAdminItem(ctx, itemID)
}

// ItemImageCandidatePreview opens a local candidate of a title (getItemImageCandidatePreview).
func (s *Service) ItemImageCandidatePreview(ctx context.Context, itemID, source string) (model.StreamSource, error) {
	if s.meta == nil {
		return model.StreamSource{}, ErrNotConfigured
	}
	return s.meta.LocalImagePreview(ctx, itemID, source)
}
