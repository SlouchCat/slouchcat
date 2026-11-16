package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Metadata settings (concept 10, ADR 0018): the administration passes the three operations on to
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

// ApplyMetadata queues metadata.apply (*model.ValidationError).
func (s *Service) ApplyMetadata(ctx context.Context, req model.ApplyMetadataRequest) (model.ApplyMetadataResult, error) {
	if s.meta == nil {
		return model.ApplyMetadataResult{}, ErrNotConfigured
	}
	return s.meta.Apply(ctx, req)
}
