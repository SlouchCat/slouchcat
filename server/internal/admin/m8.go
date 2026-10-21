package admin

import (
	"context"
	"io"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Administration operations of M8 (plan M8, design sections 1 and 3). The base of the milestone
// answers model.ErrNotImplemented (501) for each; lane S1 builds the wizard checks, the system state,
// the dashboard, the job administration, the settings and integrations, the update check and the
// account overview, lane S2 backup, restore and statistics, lane S3 the announcements.

// ListBackups lists the backup directory (A-4).
// M8 lane S2 implements this.
func (s *Service) ListBackups(context.Context) ([]model.Backup, error) {
	return nil, model.ErrNotImplemented
}

// CreateBackup writes a backup archive (A-4).
// M8 lane S2 implements this.
func (s *Service) CreateBackup(context.Context, model.BackupRequest) (model.Backup, error) {
	return model.Backup{}, model.ErrNotImplemented
}

// OpenBackup opens a backup archive for the download.
// M8 lane S2 implements this.
func (s *Service) OpenBackup(context.Context, string) (model.StreamSource, error) {
	return model.StreamSource{}, model.ErrNotImplemented
}

// DeleteBackup deletes a backup archive.
// M8 lane S2 implements this.
func (s *Service) DeleteBackup(context.Context, string) error {
	return model.ErrNotImplemented
}

// RestoreBackupUpload stages an uploaded backup and schedules the restart (A-4).
// M8 lane S2 implements this.
func (s *Service) RestoreBackupUpload(context.Context, io.Reader, string) (model.RestoreResult, error) {
	return model.RestoreResult{}, model.ErrNotImplemented
}

// RestoreBackup stages a backup of the list and schedules the restart (A-4).
// M8 lane S2 implements this.
func (s *Service) RestoreBackup(context.Context, string, string) (model.RestoreResult, error) {
	return model.RestoreResult{}, model.ErrNotImplemented
}

// Stats returns the usage statistics of a period (A-8).
// M8 lane S2 implements this.
func (s *Service) Stats(context.Context, int) (model.Stats, error) {
	return model.Stats{}, model.ErrNotImplemented
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
