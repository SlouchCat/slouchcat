package api

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// OfflineService prepares downloads and keeps devices in step (package offline, M5). Every method
// that takes the session reads titles with its active profile: an invisible title is
// model.ErrNotFound (404). The device is session.DeviceID; methods that need one answer
// model.ErrDeviceRequired without it.
type OfflineService interface {
	// Options returns the download profiles of a movie, an episode or a season with size estimates
	// (model.ErrNotFound, model.ErrNotDownloadable).
	Options(ctx context.Context, session model.Session, itemID, mediaFileID string) (model.DownloadOptions, error)
	// Create downloads a movie, an episode or a season's visible episodes to the device; a title the
	// device has is returned as it is (model.ErrNotFound, model.ErrNotDownloadable,
	// *model.ValidationError, model.ErrDeviceRequired).
	Create(ctx context.Context, session model.Session, itemID string, req model.DownloadRequest) ([]model.DownloadState, error)
	// Delete removes a download of the session's account (model.ErrNotFound otherwise).
	Delete(ctx context.Context, session model.Session, downloadID string) error
	// Manifest describes a prepared download of the device with token URLs (model.ErrNotFound,
	// model.ErrDownloadNotReady, model.ErrDownloadEvicted).
	Manifest(ctx context.Context, session model.Session, downloadID string) (model.DownloadManifest, error)
	// Sync records fetches, applies the auto-download rules and answers the device's downloads,
	// the revocation list, the offline profiles and the rule (model.ErrDeviceRequired,
	// *model.ValidationError).
	Sync(ctx context.Context, session model.Session, req model.OfflineSyncRequest) (model.OfflineSync, error)
	// Rules and SetRules read and store the auto-download rule of the active profile on the device.
	Rules(ctx context.Context, session model.Session) (model.DownloadRules, error)
	SetRules(ctx context.Context, session model.Session, r model.DownloadRules) (model.DownloadRules, error)
	// Revocations returns the reported ids the device of the cookie must delete (public; an empty
	// device id gives an empty list; *model.ValidationError).
	Revocations(ctx context.Context, deviceID string, ids []string) ([]string, error)
	// AuthorizeDownload resolves a download token: model.ErrUnauthenticated (missing, malformed,
	// forged), model.ErrNotFound (unknown download, the token's profile no longer of the account or
	// no longer seeing the title).
	AuthorizeDownload(ctx context.Context, token string) (model.DownloadGrant, error)
	// OpenFile opens the prepared file of a grant (model.ErrDownloadEvicted when removed).
	OpenFile(ctx context.Context, g model.DownloadGrant) (model.StreamSource, error)
	// OpenSubtitle opens a WebVTT sidecar of a grant (model.ErrNotFound for a track it does not
	// carry, model.ErrDownloadEvicted when removed).
	OpenSubtitle(ctx context.Context, g model.DownloadGrant, trackID int64) (model.StreamSource, error)
}
