package model

import "errors"

// Wishes through Seerr (concept 25): the connection to Seerr and the settings of the wishes
// (data model 2.12). The contract schemas SeerrSettings and WishSettings are the source.

// Errors of the Seerr connection and the wish settings.
var (
	ErrSeerrNotConfigured = errors.New("seerr not configured")          // 422 seerr_not_configured: no address or no api key
	ErrSeerrKeyRequired   = errors.New("seerr api key required")        // 422 seerr_key_required: a new host needs the key again
	ErrSeerrNotTested     = errors.New("seerr connection not tested")   // 409 seerr_not_tested: «Wünsche aktiv» needs a successful test (T-1)
	ErrSeerrUserTaken     = errors.New("seerr user taken")              // 409 seerr_user_taken: the user belongs to another account or is the shared user
	ErrInstanceUnchanged  = errors.New("seerr instance unchanged")      // 409 instance_unchanged: no other instance is waiting for a decision
	ErrInstanceChanged    = errors.New("other seerr instance detected") // no call on behalf of a user until the administrator decides
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
