package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Announcements writes, publishes and deletes the announcements of the main user (M8, A-7; package
// notify, which also sends them on the channels and shows them to the profiles). Errors:
// *model.ValidationError, model.ErrNotFound, model.ErrAnnouncementPublished (changing a published
// one).
type Announcements interface {
	ListAnnouncements(ctx context.Context) ([]model.Announcement, error)
	CreateAnnouncement(ctx context.Context, creator model.Account, in model.AnnouncementInput) (model.Announcement, error)
	UpdateAnnouncement(ctx context.Context, id int64, title, body string) (model.Announcement, error)
	PublishAnnouncement(ctx context.Context, id int64) (model.Announcement, error)
	DeleteAnnouncement(ctx context.Context, id int64) error
}

// WithAnnouncements sets the announcements and returns the receiver.
func (s *Service) WithAnnouncements(a Announcements) *Service {
	s.announcements = a
	return s
}

// ListAnnouncements returns every announcement, newest first (A-7).
func (s *Service) ListAnnouncements(ctx context.Context) ([]model.Announcement, error) {
	if s.announcements == nil {
		return nil, ErrNotConfigured
	}
	return s.announcements.ListAnnouncements(ctx)
}

// CreateAnnouncement writes an announcement and publishes it on request (A-7).
func (s *Service) CreateAnnouncement(ctx context.Context, creator model.Account, in model.AnnouncementInput) (model.Announcement, error) {
	if s.announcements == nil {
		return model.Announcement{}, ErrNotConfigured
	}
	return s.announcements.CreateAnnouncement(ctx, creator, in)
}

// UpdateAnnouncement changes an unpublished announcement.
func (s *Service) UpdateAnnouncement(ctx context.Context, id int64, title, body string) (model.Announcement, error) {
	if s.announcements == nil {
		return model.Announcement{}, ErrNotConfigured
	}
	return s.announcements.UpdateAnnouncement(ctx, id, title, body)
}

// PublishAnnouncement publishes an announcement and sends it on the channels once.
func (s *Service) PublishAnnouncement(ctx context.Context, id int64) (model.Announcement, error) {
	if s.announcements == nil {
		return model.Announcement{}, ErrNotConfigured
	}
	return s.announcements.PublishAnnouncement(ctx, id)
}

// DeleteAnnouncement deletes an announcement with its acknowledgements.
func (s *Service) DeleteAnnouncement(ctx context.Context, id int64) error {
	if s.announcements == nil {
		return ErrNotConfigured
	}
	return s.announcements.DeleteAnnouncement(ctx, id)
}
