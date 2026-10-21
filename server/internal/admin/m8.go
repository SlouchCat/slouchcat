package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Administration operations of M8 (plan M8, design sections 1 and 3). The base of the milestone
// answers model.ErrNotImplemented (501) for each; lane S1 builds the wizard checks, the system state,
// the dashboard, the job administration, the settings and integrations, the update check and the
// account overview, lane S2 backup, restore and statistics, lane S3 the announcements.

// CheckSetupPath describes a library path for the wizard (A-1).
// M8 lane S1 implements this.
func (s *Service) CheckSetupPath(context.Context, string) (model.PathCheck, error) {
	return model.PathCheck{}, model.ErrNotImplemented
}

// CheckSetupTMDBKey checks a TMDB key for the wizard (A-1).
// M8 lane S1 implements this.
func (s *Service) CheckSetupTMDBKey(context.Context, string) (model.TMDBCheck, error) {
	return model.TMDBCheck{}, model.ErrNotImplemented
}

// SystemInfo returns the server's state for the wizard and the dashboard.
// M8 lane S1 implements this.
func (s *Service) SystemInfo(context.Context) (model.SystemInfo, error) {
	return model.SystemInfo{}, model.ErrNotImplemented
}

// Dashboard returns the admin overview (A-3).
// M8 lane S1 implements this.
func (s *Service) Dashboard(context.Context) (model.Dashboard, error) {
	return model.Dashboard{}, model.ErrNotImplemented
}

// RetryJob queues a failed job again.
// M8 lane S1 implements this.
func (s *Service) RetryJob(context.Context, int64) (model.JobSummary, error) {
	return model.JobSummary{}, model.ErrNotImplemented
}

// CancelJob cancels a queued or running job.
// M8 lane S1 implements this.
func (s *Service) CancelJob(context.Context, int64) error {
	return model.ErrNotImplemented
}

// Settings returns the server settings and integrations (A-2).
// M8 lane S1 implements this.
func (s *Service) Settings(context.Context) (model.AdminSettings, error) {
	return model.AdminSettings{}, model.ErrNotImplemented
}

// UpdateSettings changes the server settings and integrations (A-2).
// M8 lane S1 implements this.
func (s *Service) UpdateSettings(context.Context, model.AdminSettingsUpdate) (model.AdminSettings, error) {
	return model.AdminSettings{}, model.ErrNotImplemented
}

// SendTestEmail sends a test mail through the configured e-mail server.
// M8 lane S1 implements this.
func (s *Service) SendTestEmail(context.Context, string) error {
	return model.ErrNotImplemented
}

// RotatePushKeys creates a new VAPID key pair.
// M8 lane S1 implements this.
func (s *Service) RotatePushKeys(context.Context) (model.AdminSettings, error) {
	return model.AdminSettings{}, model.ErrNotImplemented
}

// UpdateState returns the state of the update check (A-5).
// M8 lane S1 implements this.
func (s *Service) UpdateState(context.Context) (model.UpdateState, error) {
	return model.UpdateState{}, model.ErrNotImplemented
}

// CheckForUpdate reads the release manifest now (A-5).
// M8 lane S1 implements this.
func (s *Service) CheckForUpdate(context.Context) (model.UpdateState, error) {
	return model.UpdateState{}, model.ErrNotImplemented
}

// ListAccounts returns every account with profiles and channels (A-2).
// M8 lane S1 implements this.
func (s *Service) ListAccounts(context.Context) ([]model.AdminAccount, error) {
	return nil, model.ErrNotImplemented
}

// ListAnnouncements returns every announcement (A-7).
// M8 lane S3 implements this.
func (s *Service) ListAnnouncements(context.Context) ([]model.Announcement, error) {
	return nil, model.ErrNotImplemented
}

// CreateAnnouncement writes an announcement and publishes it on request (A-7).
// M8 lane S3 implements this.
func (s *Service) CreateAnnouncement(context.Context, model.Account, model.AnnouncementInput) (model.Announcement, error) {
	return model.Announcement{}, model.ErrNotImplemented
}

// UpdateAnnouncement changes an unpublished announcement.
// M8 lane S3 implements this.
func (s *Service) UpdateAnnouncement(context.Context, int64, string, string) (model.Announcement, error) {
	return model.Announcement{}, model.ErrNotImplemented
}

// PublishAnnouncement publishes an announcement and sends it on the channels.
// M8 lane S3 implements this.
func (s *Service) PublishAnnouncement(context.Context, int64) (model.Announcement, error) {
	return model.Announcement{}, model.ErrNotImplemented
}

// DeleteAnnouncement deletes an announcement with its acknowledgements.
// M8 lane S3 implements this.
func (s *Service) DeleteAnnouncement(context.Context, int64) error {
	return model.ErrNotImplemented
}
