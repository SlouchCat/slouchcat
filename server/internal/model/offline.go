package model

import (
	"errors"
	"time"
)

// Offline playback (M5): downloads to a device, the sync of a device, offline PIN material, offline
// watch time and uploaded offline progress events. The contract schemas of the same names in
// api/openapi.yaml are the source; the M5 design (session notes m5-design.md) explains them.

// Errors of the offline API.
var (
	ErrDeviceRequired   = errors.New("session has no device")     // 409 device_required
	ErrNotDownloadable  = errors.New("title not downloadable")    // 422 not_downloadable
	ErrDownloadNotReady = errors.New("download not ready")        // 409 download_not_ready
	ErrDownloadEvicted  = errors.New("download copy evicted")     // 410 download_evicted
	ErrNotImplemented   = errors.New("operation not implemented") // 501 not_implemented (base stubs only)
)

// Download qualities (DownloadQuality): H.264 and AAC in fragmented MP4.
const (
	DownloadHigh   = "high"   // ≤ 1080p, 6000 kbit/s video, 160 kbit/s audio
	DownloadMedium = "medium" // ≤ 720p, 3000 kbit/s video, 128 kbit/s audio
	DownloadLow    = "low"    // ≤ 480p, 1200 kbit/s video, 96 kbit/s audio
)

// Download states (DownloadStateValue).
const (
	DownloadQueued    = "queued"
	DownloadPreparing = "preparing"
	DownloadPaused    = "paused"
	DownloadReady     = "ready"
	DownloadFailed    = "failed"
	DownloadEvicted   = "evicted"
)

// Download error codes (DownloadState.error).
const (
	DownloadErrBudgetFull   = "budget_full"
	DownloadErrNotVisible   = "not_visible"
	DownloadErrEncodeFailed = "encode_failed"
)

// Results of an uploaded offline event (OfflineEventResult.results).
const (
	OfflineEventAccepted  = "accepted"
	OfflineEventDuplicate = "duplicate"
	OfflineEventRejected  = "rejected"
)

// OfflinePinIterations is the PBKDF2-SHA256 iteration count of offline PIN verifiers (AN-19, plan 3.3).
const OfflinePinIterations = 600_000

// DownloadProfile is the target of one download quality.
type DownloadProfile struct {
	Quality      string
	MaxHeight    int
	VideoBitrate int // kbit/s
	AudioBitrate int // kbit/s
}

// DownloadProfiles are the three download qualities of the M5 design, best first.
var DownloadProfiles = []DownloadProfile{
	{Quality: DownloadHigh, MaxHeight: 1080, VideoBitrate: 6000, AudioBitrate: 160},
	{Quality: DownloadMedium, MaxHeight: 720, VideoBitrate: 3000, AudioBitrate: 128},
	{Quality: DownloadLow, MaxHeight: 480, VideoBitrate: 1200, AudioBitrate: 96},
}

// DownloadProfileFor returns the profile of a quality and whether it exists.
func DownloadProfileFor(quality string) (DownloadProfile, bool) {
	for _, p := range DownloadProfiles {
		if p.Quality == quality {
			return p, true
		}
	}
	return DownloadProfile{}, false
}

// DownloadQualityOption is one quality of DownloadOptions with its size estimate.
type DownloadQualityOption struct {
	Quality        string
	Width, Height  int
	VideoBitrate   int // kbit/s
	AudioBitrate   int // kbit/s
	EstimatedBytes int64
	Copy           bool // video copied from the source
}

// DownloadOptions are the download choices of a movie, an episode or a season.
type DownloadOptions struct {
	MediaFileID  string
	DurationMs   int64
	EpisodeCount int // seasons
	Qualities    []DownloadQualityOption
	AudioTracks  []Track
	Subtitles    []Track // text tracks only
	Existing     *DownloadState
}

// DownloadRequest asks for downloads of a title (createDownloads).
type DownloadRequest struct {
	Quality       string
	AudioTrack    *int   // stream index, single titles only
	AudioLanguage string // seasons
	MediaFileID   string // single titles only
}

// DownloadState is a download as the device sees it. Item is set only when the active profile may
// see the title (then it is in AllowedProfileIDs).
type DownloadState struct {
	ID                string
	State             string
	Quality           string
	Auto              bool
	AllowedProfileIDs []int64
	EstimatedBytes    int64
	SizeBytes         int64 // 0 while not ready
	SegmentsDone      int
	SegmentsTotal     int // 0 while unknown
	Error             string
	CreatedAt         time.Time
	ReadyAt           time.Time // zero while not ready
	Item              *ItemSummary
}

// DownloadSegment is one media segment of a prepared download: its time on the timeline and its
// byte range in the file.
type DownloadSegment struct {
	StartMs, DurationMs int64
	Offset, Length      int64
}

// DownloadSubtitle is a WebVTT sidecar of a download with its token URL.
type DownloadSubtitle struct {
	Track Track
	URL   string
}

// DownloadManifest describes a prepared download (getDownloadManifest).
type DownloadManifest struct {
	ID, ItemID    string
	SizeBytes     int64
	DurationMs    int64
	MimeType      string
	Codecs        string
	Width, Height int
	InitLength    int64
	Segments      []DownloadSegment
	FileURL       string
	Audio         *Track
	Subtitles     []DownloadSubtitle
}

// DownloadGrant is a resolved download token: the download, its title and the profile the token
// was issued to.
type DownloadGrant struct {
	DownloadID string
	ItemID     string
	AccountID  int64
	ProfileID  int64
}

// HeldDownload is a download the device reports in a sync.
type HeldDownload struct {
	ID       string
	Complete bool
}

// OfflineSyncRequest is the body of syncOffline.
type OfflineSyncRequest struct {
	Held    []HeldDownload
	Network string // wifi, cellular, unknown or empty
}

// PinVerifier is a PBKDF2-SHA256 verifier (salt and hash base64url without padding).
type PinVerifier struct {
	Salt       string
	Iterations int
	Hash       string
}

// OfflinePin is the offline PIN material of one profile: whether it has a PIN, the salt of the
// normal PIN (hex, the client derives its own verifier) and the verifier of the separate offline PIN.
type OfflinePin struct {
	HasPin     bool
	PinSalt    string
	OfflinePin *PinVerifier
}

// OfflineWatchTime is the remaining watch time of today at the sync and the moment the day resets
// (AN-18).
type OfflineWatchTime struct {
	LimitMinutes     int
	RemainingMinutes int
	ResetsAt         time.Time
}

// OfflineProfile is what the offline library needs of a profile.
type OfflineProfile struct {
	ID            int64
	Name          string
	Accent        string
	Language      string
	IsPrimary     bool
	HasPin        bool
	PinSalt       string
	PinIterations int
	OfflinePin    *PinVerifier
	WatchTime     *OfflineWatchTime
}

// DownloadRules is the auto-download rule of a profile on a device (O-3).
type DownloadRules struct {
	Enabled            bool
	NextEpisodes       bool
	NewPodcastEpisodes bool // stored for M6, no effect before
	WifiOnly           bool
	MaxCount           int
	Quality            string
}

// DefaultDownloadRules is the rule of a profile and device without a stored one.
var DefaultDownloadRules = DownloadRules{NextEpisodes: true, WifiOnly: true, MaxCount: 3, Quality: DownloadMedium}

// OfflineSync is the answer of syncOffline.
type OfflineSync struct {
	ServerTime time.Time
	Downloads  []DownloadState
	Revoked    []string
	Profiles   []OfflineProfile
	Rules      DownloadRules
}

// OfflineEvent is a progress event buffered during offline playback.
type OfflineEvent struct {
	ProfileID   int64
	ItemID      string
	PlaybackID  string // client-created id of one offline playback
	DownloadID  string
	MediaFileID string
	PositionMs  int64
	DurationMs  int64
	PlayedMs    int64
	Seq         int64
	State       string
	ClientTime  time.Time
}

// OfflineEventResult is the answer of uploadOfflineEvents; Results has one entry per event in the
// order of the request.
type OfflineEventResult struct {
	Accepted, Duplicates, Rejected int
	Results                        []string
}
