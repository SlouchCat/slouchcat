package notify

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Notification channels and announcements of M8 (plan 5.2, 5.5, A-6, A-7; design sections 1 and
// 3.2). The base of the milestone answers model.ErrNotImplemented (501) for each; lane S3 builds
// them.

// Channels returns the e-mail, push and Telegram channels of the session's account.
// M8 lane S3 implements this.
func (s *Service) Channels(context.Context, model.Session) ([]model.ChannelSetting, error) {
	return nil, model.ErrNotImplemented
}

// UpdateChannel switches a channel and stores its events.
// M8 lane S3 implements this.
func (s *Service) UpdateChannel(context.Context, model.Session, string, model.ChannelSettingUpdate) (model.ChannelSetting, error) {
	return model.ChannelSetting{}, model.ErrNotImplemented
}

// TestChannel sends a test message on a channel.
// M8 lane S3 implements this.
func (s *Service) TestChannel(context.Context, model.Session, string) error {
	return model.ErrNotImplemented
}

// CreateTelegramLink issues a new Telegram link code.
// M8 lane S3 implements this.
func (s *Service) CreateTelegramLink(context.Context, model.Session) (model.ChannelSetting, error) {
	return model.ChannelSetting{}, model.ErrNotImplemented
}

// DeleteTelegramLink unlinks the Telegram chat.
// M8 lane S3 implements this.
func (s *Service) DeleteTelegramLink(context.Context, model.Session) error {
	return model.ErrNotImplemented
}

// PushPublicKey returns the VAPID public key.
// M8 lane S3 implements this.
func (s *Service) PushPublicKey(context.Context) (string, error) {
	return "", model.ErrNotImplemented
}

// CreatePushSubscription stores the push subscription of a browser.
// M8 lane S3 implements this.
func (s *Service) CreatePushSubscription(context.Context, model.Session, model.PushSubscriptionInput) (model.PushSubscription, error) {
	return model.PushSubscription{}, model.ErrNotImplemented
}

// DeletePushSubscription removes a push subscription of the session's account.
// M8 lane S3 implements this.
func (s *Service) DeletePushSubscription(context.Context, model.Session, int64) error {
	return model.ErrNotImplemented
}

// Announcements returns the published announcements the profile has not acknowledged.
// M8 lane S3 implements this.
func (s *Service) Announcements(context.Context, model.Profile) ([]model.ProfileAnnouncement, error) {
	return nil, model.ErrNotImplemented
}

// AckAnnouncement acknowledges an announcement for the profile.
// M8 lane S3 implements this.
func (s *Service) AckAnnouncement(context.Context, model.Profile, int64) error {
	return model.ErrNotImplemented
}
