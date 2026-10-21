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

// Stats returns the usage statistics of a period (A-8).
// M8 lane S2 implements this.
func (s *Service) Stats(context.Context, int) (model.Stats, error) {
	return model.Stats{}, model.ErrNotImplemented
}
