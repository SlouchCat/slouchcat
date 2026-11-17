package model

import (
	"errors"
	"time"
)

// Wishes through Seerr (concept 25): the connection to Seerr and the settings of the wishes
// (data model 2.12). The contract schemas SeerrSettings and WishSettings are the source.

// Errors of the Seerr connection and the wish settings.
var (
	ErrSeerrNotConfigured  = errors.New("seerr not configured")          // 422 seerr_not_configured: no address or no api key
	ErrSeerrKeyRequired    = errors.New("seerr api key required")        // 422 seerr_key_required: a new host needs the key again
	ErrSeerrNotTested      = errors.New("seerr connection not tested")   // 409 seerr_not_tested: «Wünsche aktiv» needs a successful test (T-1)
	ErrSeerrUserTaken      = errors.New("seerr user taken")              // 409 seerr_user_taken: the user belongs to another account or is the shared user
	ErrInstanceUnchanged   = errors.New("seerr instance unchanged")      // 409 instance_unchanged: no other instance is waiting for a decision
	ErrInstanceChanged     = errors.New("other seerr instance detected") // no call on behalf of a user until the administrator decides
	ErrProvisionDisabled   = errors.New("seerr provisioning disabled")   // 409 provision_disabled: «Neue Konten in Seerr eröffnen» is off
	ErrPasswordNotSyncable = errors.New("seerr password not syncable")   // 409 password_not_syncable: an administrator of Seerr or a sign-in through Plex, Jellyfin or Emby
)

// Codes of the validation errors.
const (
	CodeSeerrURLInvalid = "seerr_url_invalid"
)

// SeerrSettings is the connection to Seerr without its key (getSeerrSettings). URLSource and
// Key.Source say where the values come from: the environment (SLOUCHCAT_SEERR_URL,
// SLOUCHCAT_SEERR_API_KEY) wins over the stored ones.
type SeerrSettings struct {
	Enabled   bool
	URL       string
	URLSource string // env, stored, none
	Key       KeyState
}

// SeerrSettingsUpdate changes the connection (updateSeerrSettings); nil fields stay. An empty
// APIKey keeps the stored key; Clear removes the stored address and key («Löschen …»).
type SeerrSettingsUpdate struct {
	Enabled *bool
	URL     *string
	APIKey  *string
	Clear   bool
}

// SeerrConfig is the connection package wishes calls Seerr with. APIKey is a secret: never log it.
type SeerrConfig struct {
	Enabled bool
	URL     string
	APIKey  string
}

// Configured reports whether an address and a key are set.
func (c SeerrConfig) Configured() bool { return c.URL != "" && c.APIKey != "" }

// WishLimit is a limit in Seerr: Limit titles (films) or seasons (series) in Days days; Limit 0 =
// no limit.
type WishLimit struct {
	Limit int
	Days  int
}

// WishLimits is the limit of films and of seasons.
type WishLimits struct {
	Movie WishLimit
	TV    WishLimit
}

// Values of WishSettings.
const (
	WishesAlreadyJoin   = "join"
	WishesAlreadyStatus = "status"

	WishesUnmappedFallback = "fallback"
	WishesUnmappedDeny     = "deny"
)

// WishSettings are the settings of Medien › Wünsche › Einstellungen (data model 2.12).
type WishSettings struct {
	Enabled               bool   // «Wünsche aktiv», off by default; on only after a successful test (T-1)
	AllowMovies           bool   // «Filme erlauben»
	AllowShows            bool   // «Serien erlauben»
	FromSearch            bool   // «Anfordern aus der Suche»
	FromDetail            bool   // «Anfordern auf Detailseiten»
	AlreadyRequested      string // join (F-7) or status
	Unmapped              string // fallback (F-4: the shared user, default) or deny
	NewAccountsCanRequest bool   // «Neue Konten dürfen anfordern»
	AllowWithdraw         bool   // «Zurückziehen erlauben» (F-17)
	PINExternalHideTitle  bool   // «Externe Nachrichten bei PIN-Profilen ohne Titel» (F-15)
	ProvisionAccounts     bool   // «Neue Konten in Seerr eröffnen» (N-8)
	SyncPassword          bool   // «Passwort abgleichen» (N-8)
	DefaultLimit          WishLimits
}

// WishSettingsUpdate changes the wish settings (setWishSettings); nil fields stay.
type WishSettingsUpdate struct {
	Enabled, AllowMovies, AllowShows, FromSearch, FromDetail   *bool
	AlreadyRequested, Unmapped                                 *string
	NewAccountsCanRequest, AllowWithdraw, PINExternalHideTitle *bool
	ProvisionAccounts, SyncPassword                            *bool
	DefaultLimit                                               *WishLimits
}

// The connection test (testSeerrConnection, architecture 15.1, administration 4.1).

// Keys of the checks.
const (
	SeerrCheckReachable         = "reachable"
	SeerrCheckVersion           = "version"
	SeerrCheckKeyValid          = "key_valid"
	SeerrCheckManageUsers       = "manage_users"
	SeerrCheckInstance          = "instance"
	SeerrCheckActAsUser         = "act_as_user"
	SeerrCheckCSRFOff           = "csrf_off"
	SeerrCheckRadarr            = "radarr"
	SeerrCheckSonarr            = "sonarr"
	SeerrCheckLocalLogin        = "local_login"
	SeerrCheckTVDBSeasons       = "tvdb_seasons"
	SeerrCheckPasswordTransport = "password_transport"
)

// Levels of a check: a warning never fails the test.
const (
	SeerrLevelOK   = "ok"
	SeerrLevelWarn = "warn"
	SeerrLevelFail = "fail"
)

// Details of a check (the version check carries the version, radarr and sonarr the number of
// servers, tvdb_seasons the source Seerr uses).
const (
	SeerrDetailNotVerified     = "not_verified"     // version below Jellyseerr 2.7 or Seerr 3.0
	SeerrDetailNotTestable     = "not_testable"     // no user other than user 1 to act as
	SeerrDetailInstanceChanged = "instance_changed" // another instance; no call on behalf of a user
	SeerrDetailNotUserOne      = "not_user_one"     // the key does not act as user 1
	SeerrDetailNotAvailable    = "not_available"    // the route does not exist in this version
	SeerrDetailOff             = "off"
	SeerrDetailTimeout         = "timeout"
	SeerrDetailRefused         = "refused"
	SeerrDetailRedirect        = "redirect"
)

// SeerrTestInput are values not saved yet; "" uses the saved one.
type SeerrTestInput struct {
	URL    string
	APIKey string
}

// SeerrCheck is one check of the test.
type SeerrCheck struct {
	Key, Level, Detail string
}

// SeerrTestResult is the check list; OK when no check failed.
type SeerrTestResult struct {
	OK      bool
	Version string
	Checks  []SeerrCheck
}

// States of the connection (getSeerrStatus).
const (
	SeerrStateNotConfigured   = "not_configured"
	SeerrStateDisabled        = "disabled"
	SeerrStateUntested        = "untested" // configured, but neither a test nor a job reached Seerr yet
	SeerrStateConnected       = "connected"
	SeerrStateKeyInvalid      = "key_invalid"
	SeerrStateUnreachable     = "unreachable"
	SeerrStateInstanceChanged = "instance_changed"
)

// SeerrStatus is the state of the connection. LastError is the class of the last error;
// InstanceChanged says that another instance waits for confirmSeerrInstance; Tested says that
// «Wünsche aktiv» may be switched on (T-1).
type SeerrStatus struct {
	State                    string
	Version                  string
	LastSuccess, LastAttempt *time.Time
	LastError                string
	InstanceChanged          bool
	Tested                   bool
	Counts                   SeerrCounts
	Log                      []SeerrLogEntry
}

// SeerrCounts are the counters of the status; Open and Queued count wishes (stage 2), Unmapped the
// accounts that may request without a Seerr user.
type SeerrCounts struct {
	Open, Queued, Unmapped int
}

// SeerrLogEntry is one call of the log (seerr_log); Status 0 = no answer.
type SeerrLogEntry struct {
	At         time.Time
	Method     string
	Path       string
	Status     int
	DurationMS int64
	Error      string
}

// SeerrUser is a user of the mirror (listSeerrUsers). The rights are derived in the server.
type SeerrUser struct {
	ID               int
	DisplayName      string
	Email            string
	UserType         int // 1 Plex, 2 local, 3 Jellyfin, 4 Emby
	IsAdmin          bool
	CanRequestMovies bool
	CanRequestShows  bool
	AutoApprove      bool
	Shared           bool // the shared user «slouchcat-gaeste»
	Gone             bool
	Quota            SeerrUserQuota
	MappedTo         *SeerrUserAccount
}

// SeerrUserQuota are the limit fields of the user as Seerr holds them: nil = the instance's
// default, 0 = no limit.
type SeerrUserQuota struct {
	MovieLimit, MovieDays, TVLimit, TVDays *int
}

// SeerrUserAccount is the account a Seerr user belongs to.
type SeerrUserAccount struct {
	AccountID int64
	Email     string
}

// State of an account in WishMappings: the origin of a ready mapping, waiting, or none.
const WishMappingNone = "none"

// Modes of the limit of an account.
const (
	WishLimitDefault   = "default"
	WishLimitCustom    = "custom"
	WishLimitUnlimited = "unlimited"
)

// AccountWishLimit is the limit of an account: the default, an own one or none. Effective is the
// limit that applies (Limit 0 = none); SeerrAdmin: the user is an administrator of Seerr and has
// no limit there.
type AccountWishLimit struct {
	Mode       string
	Movie, TV  *WishLimit
	Effective  WishLimits
	SeerrAdmin bool
}

// WishAccountMapping is the Seerr user of one account. State is created, linked, manual, waiting
// or none; LinkedBy admin, invite or email_change.
type WishAccountMapping struct {
	AccountID        int64
	Email            string
	CanRequest       bool
	SeerrUserID      *int
	State            string
	PasswordState    string
	PasswordSyncedAt *time.Time
	LinkedBy         string
	LinkedAt         *time.Time
	ProvisionError   string
	Limit            AccountWishLimit
	Quota            *WishQuota // the state in Seerr; nil while never read
}

// Reasons of an unlimited WishQuota.
const (
	WishQuotaReasonAdmin   = "admin"    // the Seerr user has MANAGE_USERS: «Kein Limit»
	WishQuotaReasonNoLimit = "no_limit" // both limits are 0: «Unbegrenzt»
)

// WishQuota is the limit state of a Seerr user (architecture 19.3): the effective limit with what
// Seerr counts, also requests made directly in Seerr (E-3). A half with limit 0 is nil.
type WishQuota struct {
	Unlimited bool
	Reason    string
	Movie, TV *WishQuotaPart
	At        *time.Time
}

// WishQuotaPart is one half of WishQuota; Queued counts wishes waiting for the outbox (stage 2).
type WishQuotaPart struct {
	Limit, Days, Used, Queued, Remaining int
	NextFreeAt                           *time.Time
}

// WishMappings is the page «Seerr-Benutzer» (getWishMappings): Opened counts the accounts with a
// ready user.
type WishMappings struct {
	Accounts      []WishAccountMapping
	UnlinkedUsers []SeerrUser
	SharedUser    *SeerrUser
	SharedQuota   *WishQuota // the default limit, shared by every account that sends through it
	Opened, Total int
}

// Reasons why a profile cannot request (users and profiles 9).
const (
	WishReasonDisabled          = "disabled"
	WishReasonAccountNotAllowed = "account_not_allowed"
	WishReasonProfileNotAllowed = "profile_not_allowed"
	WishReasonNoMapping         = "no_mapping"
	WishReasonSeerrUserGone     = "seerr_user_gone"
)

// WishRights is the request right of a profile (SessionInfo.wishes, build step 1.6).
type WishRights struct {
	Enabled    bool
	CanRequest bool
	Reason     string
	Movies     bool
	Shows      bool
}

// WishLinkHint is the hint «Neu verknüpft» of the overview (N-93): an account linked to an existing
// Seerr user after an invitation or an email change, for seven days from LinkedAt.
type WishLinkHint struct {
	AccountID        int64
	AccountEmail     string
	SeerrUserID      int
	SeerrDisplayName string
	LinkedBy         string // invite or email_change
	LinkedAt         time.Time
}
