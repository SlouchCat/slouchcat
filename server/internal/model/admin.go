package model

import (
	"errors"
	"fmt"
	"time"
)

// Administration of M8 (plan M8, design sections 1 and 3): the wizard checks (A-1), the system state
// and the dashboard (A-3), settings and integrations (A-2), backup and restore (A-4), the update
// check (A-5), the account overview and the statistics (A-8). The contract schemas of the same names
// in api/openapi.yaml are the source.

// Errors of the administration (M8).
var (
	ErrEnvWins               = errors.New("value set by the environment") // 422 env_wins (see EnvWinsError)
	ErrPassphraseRequired    = errors.New("passphrase required")          // 422 passphrase_required
	ErrPassphraseTooShort    = errors.New("passphrase too short")         // 422 passphrase_too_short
	ErrPassphraseWrong       = errors.New("passphrase wrong")             // 422 passphrase_wrong
	ErrBackupInvalid         = errors.New("backup invalid")               // 422 backup_invalid
	ErrBackupNewerThanServer = errors.New("backup newer than server")     // 422 backup_newer_than_server
	ErrEmailNotConfigured    = errors.New("email not configured")         // 422 email_not_configured
	ErrEmailSendFailed       = errors.New("email send failed")            // 502 email_send_failed (see SendError)
	ErrTelegramTokenInvalid  = errors.New("telegram token invalid")       // 422 telegram_token_invalid
	ErrTelegramUnreachable   = errors.New("telegram unreachable")         // 422 telegram_unreachable
	ErrEmailTaken            = errors.New("email taken")                  // 409 email_taken
	ErrJobNotFailed          = errors.New("job not failed")               // 409 job_not_failed
	ErrUpdateCheckDisabled   = errors.New("update check disabled")        // 422 update_check_disabled
)

// EnvWinsError is ErrEnvWins with the request field (contract name, for example "keys.tmdb") whose
// value comes from the environment; errors.Is(err, ErrEnvWins) holds.
type EnvWinsError struct {
	Field string
}

func (e *EnvWinsError) Error() string { return fmt.Sprintf("%v: %s", ErrEnvWins, e.Field) }

// Is makes errors.Is(err, ErrEnvWins) true.
func (e *EnvWinsError) Is(target error) bool { return target == ErrEnvWins }

// SystemInfo is the server's state for the wizard and the dashboard; it carries no secret.
type SystemInfo struct {
	Version   string
	StartedAt time.Time
	GPU       GPUInfo
	Inotify   InotifyInfo
	ConfigFS  string // file system type of the config directory; "unknown" when undetected
	CacheFS   string
}

// GPUInfo is the transcode backend found at start and the GPU devices visible in the container.
type GPUInfo struct {
	Backend         string // nvenc, qsv, vaapi, software
	HardwareEncode  bool
	Tonemapper      string // cuda, placebo, vpp_qsv, vaapi, cpu
	HWTonemap       bool
	FFmpegVersion   string
	ComposeOverride string // nvidia, intel, amd; empty = no GPU device visible
	DeviceDRI       bool   // a render node /dev/dri/renderD* exists
	DeviceNvidia    bool   // /dev/nvidia0 exists
}

// InotifyInfo is the inotify watch limit and its use by the library watcher.
type InotifyInfo struct {
	MaxUserWatches int64 // 0 when unreadable
	WatchedDirs    int64
	UsagePercent   float64
	Warn           bool // limit below 65536 or more than 80 % used
}

// PathCheck describes a library path for the wizard; a bad path is no error but a Problem.
type PathCheck struct {
	OK        bool
	FSType    string // empty with a problem
	WatchMode string // inotify, poll, inotify_poll; empty with a problem
	Warning   string // network_share, fuse_unknown or empty
	Problem   string // not_absolute, missing, not_directory, not_readable; set exactly when !OK
}

// TMDBCheck is the result of a TMDB key check in the wizard.
type TMDBCheck struct {
	OK      bool
	Problem string // invalid, unreachable; set exactly when !OK
}

// Dashboard is the admin overview (A-3).
type Dashboard struct {
	Streams      []AdminStream
	Jobs         JobCounts
	Config       StorageInfo
	Cache        StorageInfo
	Integrations []IntegrationStatus
	Errors       []ErrorEntry // newest first, at most 50
	Update       UpdateState
}

// JobCounts counts the active and failed jobs, with the newest failed ones.
type JobCounts struct {
	Queued       int
	Running      int
	Failed       int
	RecentFailed []JobSummary // newest first, at most 10
}

// JobSummary is a job as the administration sees it.
type JobSummary struct {
	ID          int64
	Type        string
	State       string // queued, running, done, failed, cancelled
	Attempts    int
	MaxAttempts int
	LastError   string // redacted
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// StorageInfo is the space of a directory's file system.
type StorageInfo struct {
	Path       string
	TotalBytes int64
	FreeBytes  int64
}

// Integration keys and states of the dashboard (IntegrationStatus).
const (
	IntegrationConfigured = "configured"
	IntegrationMissing    = "missing"
	IntegrationError      = "error"
	IntegrationDisabled   = "disabled"
)

// IntegrationStatus is the state of one integration: tmdb, tvdb, youtube, trakt, email, push,
// telegram or updateCheck.
type IntegrationStatus struct {
	Key       string
	State     string // configured, missing, error, disabled
	Detail    string // why the state is error; never a secret
	CheckedAt *time.Time
}

// ErrorEntry is one error record of the server log (redacted).
type ErrorEntry struct {
	At      time.Time
	Source  string // component or job type
	Message string
	JobID   int64 // 0 = none
	JobType string
}

// UpdateState is the state of the update check (A-5).
type UpdateState struct {
	Enabled        bool
	CurrentVersion string
	LatestVersion  string
	Available      bool
	URL            string
	Notes          string
	CheckedAt      *time.Time
	Error          string
}

// UpdateNotice is the header badge of the main account: a newer version is published.
type UpdateNotice struct {
	Version string
	URL     string
}

// Sources of a setting (SettingSource): the environment wins over a stored value.
const (
	SourceEnv    = "env"
	SourceStored = "stored"
	SourceNone   = "none"
)

// KeyState tells where a provider key comes from; Hint is the last four characters of a stored key.
type KeyState struct {
	Source string
	Hint   string
}

// AdminSettings are the server settings and integrations; secrets are never part of it.
type AdminSettings struct {
	PublicURL          string
	PublicURLSource    string
	UpdateCheckEnabled bool
	Keys               AdminKeys
	Email              EmailSettings
	Telegram           TelegramSettings
	Push               PushSettings
}

// AdminKeys are the states of the provider keys.
type AdminKeys struct {
	TMDB    KeyState
	TVDB    KeyState
	TVDBPin KeyState
	YouTube KeyState
	Trakt   KeyState
}

// EmailSettings is the e-mail server without its password.
type EmailSettings struct {
	Source string
	Host   string
	Port   int
	User   string
	From   string
	TLS    string // starttls, tls, none
}

// TelegramSettings is the Telegram bot without its token.
type TelegramSettings struct {
	Source      string
	BotUsername string
}

// PushSettings is the Web Push key pair without its private key.
type PushSettings struct {
	Configured bool
	PublicKey  string
}

// Fields of AdminSettingsUpdate.Clear.
const (
	ClearPublicURL = "publicUrl"
	ClearTMDB      = "tmdb"
	ClearTVDB      = "tvdb"
	ClearTVDBPin   = "tvdbPin"
	ClearYouTube   = "youtube"
	ClearTrakt     = "trakt"
	ClearEmail     = "email"
	ClearTelegram  = "telegram"
)

// AdminSettingsUpdate changes settings: nil fields keep their value, Clear removes stored values.
type AdminSettingsUpdate struct {
	PublicURL          *string
	UpdateCheckEnabled *bool
	Keys               AdminKeysUpdate
	Email              *EmailSettingsUpdate
	Telegram           *TelegramSettingsUpdate
	Clear              []string // Clear* constants
}

// AdminKeysUpdate sets provider keys; nil keeps a key.
type AdminKeysUpdate struct {
	TMDB    *string
	TVDB    *string
	TVDBPin *string
	YouTube *string
	Trakt   *string
}

// EmailSettingsUpdate replaces the e-mail server; a nil Password keeps the stored one.
type EmailSettingsUpdate struct {
	Host     string
	Port     int
	User     string
	Password *string
	From     string
	TLS      string // starttls, tls, none
}

// TelegramSettingsUpdate replaces the bot token.
type TelegramSettingsUpdate struct {
	BotToken string
}

// Backup kinds (Backup.Kind).
const (
	BackupManual = "manual"
	BackupAuto   = "auto"
)

// Backup is an archive in the backup directory.
type Backup struct {
	Name            string
	CreatedAt       time.Time
	SizeBytes       int64
	IncludesSecrets bool
	Encrypted       bool
	Version         string // empty when unknown
	Kind            string // manual, auto
}

// BackupRequest creates a backup; a passphrase is required with secrets.
type BackupRequest struct {
	IncludeSecrets bool
	Passphrase     string
}

// RestoreResult answers a staged restore; the server restarts at RestartAt.
type RestoreResult struct {
	RestartAt       time.Time
	Version         string
	IncludesSecrets bool
}

// AdminAccount is an account as the administration sees it (A-2).
type AdminAccount struct {
	ID           int64
	Email        string
	Role         string // owner, guest
	MaxRating    int
	AllLibraries bool
	LibraryIDs   []int64
	Profiles     []ProfileRef
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	Channels     []ChannelSummary
}

// ProfileRef is a profile in the account overview.
type ProfileRef struct {
	ID        int64
	Name      string
	MaxRating int
	IsPrimary bool
}

// AccountUpdate changes the e-mail or the password of the signed-in account; nil keeps a value.
type AccountUpdate struct {
	Email           *string
	CurrentPassword string
	NewPassword     *string
}

// Stats are the usage statistics of a period (A-8), unfiltered.
type Stats struct {
	Days      int
	TopTitles []StatTitle
	WatchTime []StatProfile
	Playback  StatPlayback
	Devices   []StatDevice
}

// StatTitle is a watched title (no ItemRef: the administration view is unfiltered).
type StatTitle struct {
	ItemID    string
	Title     string
	Kind      string
	Year      *int
	Plays     int
	WatchedMs int64
}

// StatProfile is the watch time of a profile.
type StatProfile struct {
	ProfileID    int64
	ProfileName  string
	AccountEmail string
	WatchedMs    int64
	Plays        int
}

// StatPlayback counts the playback sessions by mode.
type StatPlayback struct {
	Direct         int
	Remux          int
	Transcode      int
	TranscodeShare float64 // 0..1
	HardwareShare  float64 // 0..1, transcodes on the GPU per transcodes
}

// StatDevice is the watch time per device type.
type StatDevice struct {
	Type      string // desktop, phone, tablet, tv, other, unknown
	Count     int
	WatchedMs int64
}
