package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// TitleDeletion is implemented by package library (concept 20 P-9): the preview and the delete
// of a title, and the page «Entfernte Titel».
type TitleDeletion interface {
	ItemDeletion(ctx context.Context, itemID, lang string) (model.ItemDeletionPreview, error)
	DeleteItem(ctx context.Context, itemID, lang, mode, confirmTitle string, expected model.ExpectedFiles, by model.Actor) (model.ItemDeletionResult, error)
	ListRemovedItems(ctx context.Context, limit, offset int) (model.RemovedItemPage, error)
	RestoreRemovedItem(ctx context.Context, itemID string) (int, error)
}

// WithTitleDeletion sets the deletion of titles; without it the operations answer
// ErrNotConfigured.
func (s *Service) WithTitleDeletion(d TitleDeletion) *Service {
	s.deletion = d
	return s
}

// deletionLang is the language of the title the dialog shows and the administrator types: the
// server's first text language (the editor's), "de" without metadata settings.
func (s *Service) deletionLang(ctx context.Context) (string, error) {
	if s.meta == nil {
		return "de", nil
	}
	return s.serverTextLang(ctx)
}

// GetItemDeletion is the preview of DeleteItem (getItemDeletion).
func (s *Service) GetItemDeletion(ctx context.Context, itemID string) (model.ItemDeletionPreview, error) {
	if s.deletion == nil {
		return model.ItemDeletionPreview{}, ErrNotConfigured
	}
	lang, err := s.deletionLang(ctx)
	if err != nil {
		return model.ItemDeletionPreview{}, err
	}
	return s.deletion.ItemDeletion(ctx, itemID, lang)
}

// DeleteItem deletes a title (deleteItem); the typed title is compared inside package library.
func (s *Service) DeleteItem(ctx context.Context, itemID, mode, confirmTitle string, expected model.ExpectedFiles, by model.Actor) (model.ItemDeletionResult, error) {
	if s.deletion == nil {
		return model.ItemDeletionResult{}, ErrNotConfigured
	}
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	lang, err := s.deletionLang(ctx)
	if err != nil {
		return model.ItemDeletionResult{}, err
	}
	return s.deletion.DeleteItem(ctx, itemID, lang, mode, confirmTitle, expected, by)
}

// ListRemovedItems is the page «Entfernte Titel» (listRemovedItems).
func (s *Service) ListRemovedItems(ctx context.Context, limit, offset int) (model.RemovedItemPage, error) {
	if s.deletion == nil {
		return model.RemovedItemPage{}, ErrNotConfigured
	}
	return s.deletion.ListRemovedItems(ctx, limit, offset)
}

// RestoreRemovedItem is «Wieder aufnehmen» (restoreRemovedItem).
func (s *Service) RestoreRemovedItem(ctx context.Context, itemID string) (int, error) {
	if s.deletion == nil {
		return 0, ErrNotConfigured
	}
	defer s.ForgetDashboardCounts()
	return s.deletion.RestoreRemovedItem(ctx, itemID)
}
