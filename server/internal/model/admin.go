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
	ErrRestartPending        = errors.New("restart pending")              // 409 restart_pending (concept 11)
	ErrBackupBusy            = errors.New("backup busy")                  // 409 backup_busy (concept 11)
	ErrEmailNotConfigured    = errors.New("email not configured")         // 422 email_not_configured
	ErrEmailSendFailed       = errors.New("email send failed")            // 502 email_send_failed (see SendError)
	ErrTelegramTokenInvalid  = errors.New("telegram token invalid")       // 422 telegram_token_invalid
	ErrTelegramUnreachable   = errors.New("telegram unreachable")         // 422 telegram_unreachable
	ErrEmailTaken            = errors.New("email taken")                  // 409 email_taken
	ErrJobNotFailed          = errors.New("job not failed")               // 409 job_not_failed
	ErrJobTypeNotRunnable    = errors.New("job type not runnable")        // 409 job_type_not_runnable (concept 09: event types)
	ErrJobTypePaused         = errors.New("job type paused")              // 409 job_type_paused
	ErrJobTypeNotScheduled   = errors.New("job type not scheduled")       // 409 job_type_not_scheduled
	ErrJobTypeNotWindowable  = errors.New("job type not windowable")      // 409 job_type_not_windowable (concept 15)
	ErrUpdateCheckDisabled   = errors.New("update check disabled")        // 422 update_check_disabled
	ErrRankTaken             = errors.New("spotlight rank taken")         // 409 rank_taken (M11)
	ErrRefreshTooSoon        = errors.New("refresh too soon")             // 429 too_many_requests (M11: one manual spotlight refresh a minute)
	ErrUpdateRunning         = errors.New("update running")               // 409 update_running (concept 15 P-8)
	ErrUpdaterUnavailable    = errors.New("updater unavailable")          // 503 updater_unavailable (concept 15 P-8)
	ErrUpdateRefused         = errors.New("update refused")               // 422 with the updater's code (see UpdateRefusedError)
)

// UpdateRefusedError is ErrUpdateRefused with the reason the updater companion gave before it
// changed anything (Code, for example "not_newer"; Message, its English explanation without a
// secret); errors.Is(err, ErrUpdateRefused) holds.
type UpdateRefusedError struct {
	Code    string
	Message string
}

func (e *UpdateRefusedError) Error() string { return fmt.Sprintf("%v: %s", ErrUpdateRefused, e.Code) }

// Is makes errors.Is(err, ErrUpdateRefused) true.
func (e *UpdateRefusedError) Is(target error) bool { return target == ErrUpdateRefused }

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
	Webhooks     WebhookStatus // the "Webhooks" tile (M9, I-8)
	Errors       []ErrorEntry  // newest first, at most 50
	Update       UpdateState
	Browse       BrowseHealth // the folder browser of the library form (idea 02)
	Attention    DashboardAttention
	Counts       DashboardCounts
	Activity     []ActivityEntry // the activity log, newest first, at most 6 (concept 11)
}

// Known kinds of the activity log (concept 11 section 11.4, ADR 0019). The contract has no enum
// for them, so a new kind breaks no client; admin.RecordActivity refuses a kind not listed here.
const (
	ActivityPlaybackStarted   = "playback_started"
	ActivityPlaybackCompleted = "playback_completed"
	ActivityPlaybackStopped   = "playback_stopped"
	ActivityScanStarted       = "scan_started"
	ActivityScanDone          = "scan_done"
	ActivityScanFailed        = "scan_failed"
	ActivityScanAborted       = "scan_aborted"
	ActivityJobFailed         = "job_failed"
	ActivityBackupCreated     = "backup_created"
	ActivityBackupRestored    = "backup_restored"
	ActivityServerStarted     = "server_started"
	ActivityServerRestart     = "server_restart"
)

// What started a full scan (ActivityDetail.Trigger).
const (
	ScanTriggerManual   = "manual"
	ScanTriggerSchedule = "schedule"
	ScanTriggerWatcher  = "watcher"
)

// ActivityEvent is one event for the activity log, handed to admin.RecordActivity by the hooks in
// playback, library, jobs, admin and main. Zero values mean "none".
type ActivityEvent struct {
	Kind      string
	At        time.Time // zero: now
	AccountID int64
	ProfileID int64
	ItemID    string
	LibraryID int64
	Subject   string // name of the title or the library at the time of the event; empty: read from ItemID
	Detail    ActivityDetail
}

// ActivityDetail holds the few numbers and fixed codes of an event (ActivityEntry.detail): no
// secret, no path, no IP address and no free error text (rule 10). Reason is one of
// root_missing, root_empty, loss_over_threshold, cancelled or error; By is admin or revoked.
type ActivityDetail struct {
	NewTitles *int   `json:"newTitles,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	Version   string `json:"version,omitempty"`
	Trigger   string `json:"trigger,omitempty"`
	Reason    string `json:"reason,omitempty"`
	JobType   string `json:"jobType,omitempty"`
	Count     int    `json:"count,omitempty"`
	By        string `json:"by,omitempty"`
}

// ActivityEntry is one row of the activity log as the dashboard shows it. Account and profile
// are read when the log is read; empty when there was none or it is gone.
type ActivityEntry struct {
	ID           int64
	At           time.Time
	Kind         string
	ProfileName  string
	AccountEmail string
	ItemTitle    string
	LibraryName  string
	Detail       ActivityDetail
}

// DashboardAttention is what needs the administrator's attention (concept 11).
type DashboardAttention struct {
	AbortedScans []AbortedScan // scans the loss protection aborted, loss not accepted
}

// AbortedScan is a library whose last scan the loss protection aborted.
type AbortedScan struct {
	LibraryID int64
	Name      string
	Reason    string // abort code (root_missing, root_empty, loss_over_threshold) or a message
}

// DashboardCounts are the counts of the admin sidebar (concept 11), at most 60 s old.
type DashboardCounts struct {
	Titles          int // items of every library, like LibraryScan.ItemCount
	Accounts        int
	MaintenanceOpen int // the five cheap maintenance counters, without rating_mismatch
}

// JobCounts counts the active and failed jobs, with the newest failed ones.
type JobCounts struct {
	Queued       int
	Running      int
	Failed       int
	Paused       int          // paused job types (concept 09)
	RecentFailed []JobSummary // newest first, at most 10
	Active       []JobRun     // running jobs, newest start first, at most 3 (concept 11)
	NextRun      *JobNextRun  // earliest next run of a switched-on schedule of an unpaused type; nil without
	FailedByType []JobTypeFailures
}

// JobNextRun is the next scheduled run of a job type.
type JobNextRun struct {
	Type string
	At   time.Time
}

// JobTypeFailures are the failed jobs of one type with the newest failure.
type JobTypeFailures struct {
	Type         string
	Count        int
	LastError    string // redacted
	LastFailedAt time.Time
}

// Background jobs of concept 09 (ADR 0017): a job type and its runs as the page "Hintergrundjobs"
// sees them. Every estimate is the server's; the client counts seconds up and decides nothing.

// Job run states beyond the queue's own: planned is queued with a start in the future.
const JobRunPlanned = "planned"

// Sources of a remaining-time estimate (JobRun.Estimate).
const (
	JobEstimateMeasured = "measured"
	JobEstimateMedian   = "median"
	JobEstimateNone     = "none"
)

// JobRun is a run of a job type: JobSummary plus priority, subject, times, progress and estimate.
type JobRun struct {
	JobSummary
	Priority             int
	RunAfter             time.Time // zero when not waiting
	Subject              string    // derived from the payload, redacted; "" when unknown
	StartedAt            time.Time
	FinishedAt           time.Time
	Duration             time.Duration // of the last attempt; while running, since its start
	HasDuration          bool
	Skipped              bool
	Progress             *JobProgress
	EstimatedSecondsLeft *int
	Estimate             string // JobEstimateMeasured, JobEstimateMedian, JobEstimateNone
	OverridesPause       bool   // runs while its type is paused (a playback request, concept 14)
	OverridesWindow      bool   // starts outside the time window of its type (a playback request, concept 15)
}

// JobProgress is the reported progress of a run.
type JobProgress struct {
	Done  int64
	Total int64 // 0: extent unknown
	Note  string
}

// JobWindow is a time window in minutes of the server's day; EndMinute is exclusive and may lie
// before StartMinute (across midnight).
type JobWindow struct {
	StartMinute int
	EndMinute   int
}

// JobSchedule is the schedule of a schedule type.
type JobSchedule struct {
	EverySeconds    int
	NextRunAt       time.Time
	LastFiredAt     time.Time // zero: never fired
	Enabled         bool
	Window          *JobWindow
	TimeZone        string
	MinEverySeconds int
	MaxEverySeconds int
	OnceAt          time.Time // one-off date; zero: none
	OnceLocal       string    // OnceAt as wall time in TimeZone (2006-01-02T15:04); "" without a date
}

// JobScheduleUpdate names the fields updateJobSchedule changes; nil means unchanged, ClearWindow
// removes the window, ClearOnce the one-off date (OnceLocal is ignored then).
type JobScheduleUpdate struct {
	EverySeconds *int
	Window       *JobWindow
	ClearWindow  bool
	OnceLocal    *string
	ClearOnce    bool
	Enabled      *bool
}

// JobTypeInfo is one job type on the page.
type JobTypeInfo struct {
	Type                  string
	Group                 string // library, metadata, delivery, system
	Trigger               string // schedule, event, manual
	Pool                  string // general, download, notify, index
	Paused                bool
	PausedAt              time.Time
	Stopping              bool // paused and a run still coming to a halt
	Runnable              bool
	Schedule              *JobSchedule
	Queued                int
	Planned               int
	Running               int
	Failed                int
	RunningRuns           []JobRun // at most 10
	LastRun               *JobRun
	MedianSeconds         *int
	QueuedEstimateSeconds *int
	Known                 bool // false for a row in job_types without a registered type
	// OverridesPauseForPlayback: runs a playback requests run while the type is paused (concept 14).
	OverridesPauseForPlayback bool
	// Windowable: the type may be held to a time window (concept 15, 4.3); Window is that window
	// (nil: none), WindowOpensAt when it opens next while it is closed and a waiting run without
	// override_window waits for it (zero otherwise), and
	// WindowTimeZone the name of the server's zone of the window ("" when not windowable).
	Windowable     bool
	Window         *JobWindow
	WindowOpensAt  time.Time
	WindowTimeZone string
}

// JobTypeWindowUpdate is the body of updateJobTypeWindow: ClearWindow removes the window and wins
// over Window; neither changes nothing.
type JobTypeWindowUpdate struct {
	Window      *JobWindow
	ClearWindow bool
}

// JobPool is a worker pool with its backlog.
type JobPool struct {
	Pool           string
	Workers        int
	BacklogSeconds *int
}

// JobOverview is the answer of listJobTypes.
type JobOverview struct {
	Pools  []JobPool
	Paused int
	Types  []JobTypeInfo
}

// JobRunPage is the answer of listJobRuns.
type JobRunPage struct {
	Total int
	Runs  []JobRun
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
	// Development: CurrentVersion is no semantic version (a bare commit id, an image tag or
	// "dev"), so it cannot be compared with a release and Available stays false.
	Development bool
	Available   bool
	URL         string
	Notes       string
	CheckedAt   *time.Time
	Error       string
	// UpdaterConfigured: an address of the updater companion is set (config.UpdaterURL).
	UpdaterConfigured bool
	// UpdaterAvailable: the server reaches the updater companion with its token (ADR 0024).
	UpdaterAvailable bool
	// Install is the last install the companion reported (state.json); nil when none.
	Install *UpdateInstall
}

// UpdateInstall is the state of an install by the updater companion (updater.State; contract
// UpdateInstallStatus).
type UpdateInstall struct {
	Phase      string // idle, checking, backing_up, pulling, verifying, restarting, waiting, done, rolled_back, failed
	From, To   string
	StartedAt  *time.Time
	FinishedAt *time.Time
	Migrated   bool
	Backup     string // pre-migration backup when known
	Code       string
	Message    string
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
	// Fanart.tv (concept 10 step 6): the project key and the optional personal client key.
	Fanart       KeyState
	FanartClient KeyState
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
	ClearPublicURL    = "publicUrl"
	ClearTMDB         = "tmdb"
	ClearTVDB         = "tvdb"
	ClearTVDBPin      = "tvdbPin"
	ClearYouTube      = "youtube"
	ClearTrakt        = "trakt"
	ClearFanart       = "fanart"
	ClearFanartClient = "fanartClient"
	ClearEmail        = "email"
	ClearTelegram     = "telegram"
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
	TMDB         *string
	TVDB         *string
	TVDBPin      *string
	YouTube      *string
	Trakt        *string
	Fanart       *string
	FanartClient *string
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

// MaxBackupBytes caps what a backup to restore may unpack to; the upload may be as large plus its
// multipart framing.
const MaxBackupBytes = 2 << 30

// MaxImageUploadBytes caps an image the administrator uploads for a title (uploadItemImage,
// concept 15 7.5); the request may be as large plus its multipart framing.
const MaxImageUploadBytes = 20 << 20

// BackupRequest creates a backup; a passphrase is required with secrets.
type BackupRequest struct {
	IncludeSecrets bool
	Passphrase     string
}

// RestartAccepted answers a requested restart (concept 11); the server restarts at RestartAt.
type RestartAccepted struct {
	RestartAt time.Time
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
	Role         string // admin or user (ADR 0014)
	IsOwner      bool   // the one account that cannot be deleted or demoted
	MaxRating    int
	AllLibraries bool
	LibraryIDs   []int64
	Profiles     []ProfileRef
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	Channels     []ChannelSummary
	Limits       PlaybackLimits // W-8 caps of the account (WatchLimitMinutes unused)
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
	// WatchTimeByQuarterHour is the played time per quarter hour of the week (StatsQuarterHours
	// values, Monday 00:00 first) over the last TimeWindowDays days in the time zone TimeZone
	// (concept 12, 11.4).
	WatchTimeByQuarterHour []int64
	TimeWindowDays         int
	TimeZone               string // IANA name
}

// StatsQuarterHours is the length of Stats.WatchTimeByQuarterHour: 7 days of 96 quarter hours.
const StatsQuarterHours = 7 * 96

// StatTitle is a watched title (no ItemRef: the administration view is unfiltered).
type StatTitle struct {
	ItemID    string
	Title     string
	Kind      string
	Year      *int
	Plays     int
	WatchedMs int64
	// ImageItemID names the title whose images the row shows: the title itself, for an episode its
	// series (concept 12, 11.4). PosterHash and BackdropHash are the content hashes of its
	// AdminPosterSize poster and AdminBackdropSize backdrop; "" = none stored.
	ImageItemID  string
	PosterHash   string
	BackdropHash string
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
