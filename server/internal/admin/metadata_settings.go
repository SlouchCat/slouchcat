package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Metadata settings (concept 10, ADR 0018): the administration passes the five operations on to
// package metadata (WithMetadata); without it they answer ErrNotConfigured (500).

// MetadataSettings returns the server's metadata settings.
func (s *Service) MetadataSettings(ctx context.Context) (model.MetadataSettings, error) {
	if s.meta == nil {
		return model.MetadataSettings{}, ErrNotConfigured
	}
	return s.meta.MetadataSettings(ctx)
}

// UpdateMetadataSettings stores the server's metadata settings (*model.ValidationError).
func (s *Service) UpdateMetadataSettings(ctx context.Context, m model.MetadataSettings) (model.MetadataSettings, error) {
	if s.meta == nil {
		return model.MetadataSettings{}, ErrNotConfigured
	}
	return s.meta.UpdateMetadataSettings(ctx, m)
}

// LibraryMetadata returns the deviation of a library (model.ErrNotFound).
func (s *Service) LibraryMetadata(ctx context.Context, libraryID int64) (model.LibraryMetadata, error) {
	if s.meta == nil {
		return model.LibraryMetadata{}, ErrNotConfigured
	}
	return s.meta.LibraryMetadata(ctx, libraryID)
}

// UpdateLibraryMetadata stores the deviation of a library (model.ErrNotFound, *model.ValidationError).
func (s *Service) UpdateLibraryMetadata(ctx context.Context, libraryID int64, textLangs *[]string, images *model.LibraryImages) (model.LibraryMetadata, error) {
	if s.meta == nil {
		return model.LibraryMetadata{}, ErrNotConfigured
	}
	return s.meta.UpdateLibraryMetadata(ctx, libraryID, textLangs, images)
}

// ApplyMetadata queues metadata.apply (*model.ValidationError).
func (s *Service) ApplyMetadata(ctx context.Context, req model.ApplyMetadataRequest) (model.ApplyMetadataResult, error) {
	if s.meta == nil {
		return model.ApplyMetadataResult{}, ErrNotConfigured
	}
	return s.meta.Apply(ctx, req)
}
