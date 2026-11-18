package model

import (
	"errors"
	"fmt"
	"time"
)

// Requesting wishes (concept 25 stage 2, architecture 6 to 8, data model 2.4 to 2.7 and 4.4/4.5):
// the service side of createWish, withdrawWish, getWishQuota, getWishSeasons, listWishes and
// listAdminWishes. The contract of stage 2 maps these types.

// States of a wish and of a season (data model 2.4, 2.5). Open are queued, pending, processing and
// partial; closed are available, declined, failed and withdrawn.
const (
	WishQueued     = "queued"
	WishPending    = "pending"
	WishProcessing = "processing"
	WishPartial    = "partial" // a series with some wished seasons playable
	WishAvailable  = "available"
	WishDeclined   = "declined"
	WishFailed     = "failed"
	WishWithdrawn  = "withdrawn"
)

// WishDetailImporting is the state detail «wird eingelesen»: Seerr reports the title available,
// Slouchcat cannot play it yet.
const WishDetailImporting = "importing"

// Fail reasons of a wish (wishes.fail_reason).
const (
	WishFailQuota          = "quota"
	WishFailBlocklisted    = "blocklisted"
	WishFailNoPermission   = "no_permission"
	WishFailArrFailed      = "arr_failed"
	WishFailUnreachable24h = "unreachable_24h"
	WishFailRequestGone    = "seerr_request_gone"
	WishFailMediaDeleted   = "seerr_media_deleted"
	WishFailSeerrUserGone  = "seerr_user_gone"
	WishFailTVDBIDMissing  = "tvdb_id_missing"
	WishFailAccountDeleted = "account_deleted"
)

// Values of the fields of Wish and WishSeasons.
const (
	WishMediaMovie           = "movie"
	WishMediaShow            = "show"
	WishRoleOwner            = "owner"
	WishRoleJoined           = "joined"
	WishOriginSlouchcat      = "slouchcat"
	WishHintAlreadyApproved  = "already_approved" // Wish.Hint after a withdrawal that came too late
	WishSeasonsFromSeerr     = "seerr"
	WishSeasonsTMDBFallback  = "tmdb_fallback"
	WishSeasonOfferMissing   = "missing"   // the profile can request it
	WishSeasonOfferRequested = "requested" // another wish holds it open: choosing it joins that wish
	WishSeasonOfferAvailable = "available" // the profile can play it
	WishEventsShown          = 20          // events per wish in an answer
)

// Places a wish comes from (WishCreate.Source); search and detail follow their switches.
const (
	WishSourceSearch         = "search"
	WishSourceDetail         = "detail"
	WishSourceList           = "list"
	WishSourceAdminSpotlight = "admin_spotlight"
)

// Filters of the lists (listWishes state=).
const (
	WishFilterOpen      = "open"
	WishFilterAvailable = "available"
	WishFilterClosed    = "closed"
	WishFilterAll       = "all"
)

// Further reasons of WishNotAllowedError beside those of WishRights.Reason, and the codes of the
// validation errors of createWish.
const (
	WishReasonMoviesNotAllowed = "movies_not_allowed"
	WishReasonShowsNotAllowed  = "shows_not_allowed"
	WishReasonTVDBIDMissing    = "tvdb_id_missing"
	WishReasonWithdrawDisabled = "withdraw_disabled"
	WishReasonStatusOnly       = "status_only"        // «Bereits angeforderte Titel: Nur Zustand zeigen»: no co-wish
	WishReasonSourceNotAllowed = "source_not_allowed" // «Anfordern aus der Suche» or «auf Detailseiten» is off

	WishCodeSeasonNotOffered = "season_not_offered"
	WishCodeNoSeason         = "no_season"
)

// ErrWishNotWithdrawable: the wish is past the approval (409 wish_not_withdrawable, F-17).
var ErrWishNotWithdrawable = errors.New("wish not withdrawable")

// ErrWishNotRetryable: retryWish on a wish that did not fail, failed because its account was
// deleted, whose title another open wish holds now, or whose request Seerr no longer reports as
// failed (409 wish_not_retryable).
var ErrWishNotRetryable = errors.New("wish not retryable")

// ErrSeerrUnreachable: an administration action that needs Seerr right away did not reach it, or
// Seerr is switched off or waits for a new instance (503 seerr_unreachable).
var ErrSeerrUnreachable = errors.New("seerr unreachable")

// WishNotAllowedError is 403 wish_not_allowed: the profile may not request this (Reason is one of
// the WishRights reasons or a WishReason* above).
type WishNotAllowedError struct{ Reason string }

func (e *WishNotAllowedError) Error() string { return "wish not allowed: " + e.Reason }

// WishAlreadyAvailableError is 409 already_available: the title is in the library and the profile
// sees it (ItemID, for the ItemRef of the answer).
type WishAlreadyAvailableError struct{ ItemID string }

func (e *WishAlreadyAvailableError) Error() string { return "already available: " + e.ItemID }

// WishQuotaExceededError is 409 quota_exceeded: the free places of the limit do not reach Needed
// (a film 1, a series its new seasons). Quota carries the state with NextFreeAt; no wish was made.
type WishQuotaExceededError struct {
	Quota  WishQuota
	Needed int
}

func (e *WishQuotaExceededError) Error() string {
	return fmt.Sprintf("wish quota exceeded: %d needed", e.Needed)
}

// WishCreate is the input of createWish. A series names Seasons or sets AllSeasons («Alle
// fehlenden Staffeln»); a film names neither.
type WishCreate struct {
	Media      string // movie or show
	TMDBID     int64
	Seasons    []int
	AllSeasons bool
	Source     string // search, detail, list, admin_spotlight; "" unknown
}

// WishCreated is the answer of createWish. Created: Wish is new (201); otherwise the profile asked
// again or joined (200). Joined lists the wishes the profile joined (a film: the one wish; a
// series: those holding chosen seasons open); with every season joined Wish is nil.
type WishCreated struct {
	Wish    *Wish
	Created bool
	Joined  []Wish
}

// Wish is a wish as one profile sees it.
type Wish struct {
	ID              int64
	Media           string
	TMDBID          int64
	TVDBID          int64
	IMDBID          string
	Title           string
	Year            int
	State           string
	StateDetail     string
	FailReason      string
	Origin          string
	Role            string // owner or joined; "" in the administration
	ProfileName     string // the profile of the entry; only in the wishes of an account (listAccountWishes)
	Seasons         []WishSeason
	ItemID          string             // the library title visibility found; the contract makes it an ItemRef after a visibility check
	External        *ExternalTitleCard // the title outside the library as the profile sees it («Meine Wünsche», createWish); nil in the administration
	CanWithdraw     bool
	CanRequestAgain bool
	Hint            string // already_approved
	// Hidden: the profile may not see the library title, so the wish reads as «wird eingelesen»
	// (architecture 8.3); it carries no episode counts or progress (security review L-2).
	Hidden      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	AvailableAt *time.Time
	Events      []WishEvent // newest first, at most WishEventsShown
}

// WishSeason is one season of a series wish.
type WishSeason struct {
	Number         int
	State          string
	SeerrRequestID int // 0 = none yet
}

// WishEvent is one entry of the history of a wish.
type WishEvent struct {
	At     time.Time
	Kind   string
	Detail string // JSON, no secrets
}

// WishCounts count the wishes of a list by group.
type WishCounts struct {
	Open, Pending, Processing, Partial, Available, Closed int
}

// WishPage is one page of a list of wishes.
type WishPage struct {
	Items  []Wish
	Total  int
	Counts WishCounts
}

// WishSeasons is the season list of the request dialog (getWishSeasons): from Seerr, or from TMDB
// when Seerr was not reached. Season 0 is never offered.
type WishSeasons struct {
	Source  string // seerr or tmdb_fallback
	Seasons []WishSeasonOffer
}

// WishSeasonOffer is one season of WishSeasons.
type WishSeasonOffer struct {
	Number       int
	EpisodeCount int
	State        string // missing, requested, available
}

// AdminWish is a wish in the administration (listAdminWishes): no ItemRef, the requesters with
// account and profile.
type AdminWish struct {
	Wish
	SeerrUserID    int
	SeerrRequestID int
	Attempts       int
	Requesters     []AdminWishRequester
}

// AdminWishRequester is one requester of an AdminWish.
type AdminWishRequester struct {
	AccountID    int64
	AccountEmail string
	ProfileID    int64
	ProfileName  string
	Role         string
	Closed       bool
}
