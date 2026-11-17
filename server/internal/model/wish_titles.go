package model

import "time"

// Titles outside the library as a profile sees them (concept 25 stage 2, data model 4.2 and 4.5):
// the card of the search section and of «Meine Wünsche» (ExternalTitleRef), the preview page with
// its wish card (getExternalTitle). The contract maps these types; images are kinds, the contract
// makes Slouchcat's own addresses of them (never TMDB's).

// Reasons why a profile cannot request a title it sees, beside those of WishRights and of
// WishNotAllowedError.
const (
	WishReasonAlreadyRequested = "already_requested" // the profile has an open wish for it (own or joined)
)

// Season states on the preview page (WishSeasonOffer states).
const (
	ExternalSeasonMissing   = WishSeasonOfferMissing
	ExternalSeasonRequested = WishSeasonOfferRequested
	ExternalSeasonAvailable = WishSeasonOfferAvailable
)

// WishMark is the wish of the profile on a title: its open wish, own or joined.
type WishMark struct {
	ID          int64
	State       string
	StateDetail string
}

// ExternalTitleCard is a title outside the library in a list (ExternalTitleRef).
type ExternalTitleCard struct {
	Media         string
	TMDBID        int64
	Title         string
	Year          int  // 0 = unknown
	MinAge        *int // nil = unknown
	ImageKinds    []string
	HasTrailer    bool
	Wish          *WishMark
	CanRequest    bool
	RequestReason string
}

// ExternalTitlePreview is the preview page of a title outside the library with its wish card
// (getExternalTitle).
type ExternalTitlePreview struct {
	ExternalTitleCard
	OriginalTitle  string
	Overview       string
	Tagline        string
	ReleaseDate    string // YYYY-MM-DD; "" = unknown
	RuntimeMinutes int
	Genres         []string // in the profile's language
	Status         string
	Seasons        []ExternalSeasonState // series only, season 0 left out
	NextEpisode    *ExternalEpisodeDate
	Trailers       []ExternalTrailerRef
	Request        WishRequestCard
	LastWish       *WishLast // the newest closed wish of the profile while it has no open one
}

// ExternalSeasonState is one season on the preview page.
type ExternalSeasonState struct {
	Number       int
	EpisodeCount int
	AirDate      string
	State        string // ExternalSeason*
}

// ExternalEpisodeDate names an episode with its air date.
type ExternalEpisodeDate struct {
	Season, Episode int
	AirDate         string
}

// ExternalTrailerRef is a YouTube trailer of a title outside the library.
type ExternalTrailerRef struct {
	Key  string
	Lang string
}

// WishRequestCard is what the request dialog needs: the right, the reason without it, whether
// Seerr approves at once and the limit of the user the profile sends as.
type WishRequestCard struct {
	CanRequest  bool
	Reason      string
	AutoApprove bool
	Quota       *WishQuota
}

// WishLast is a closed wish of the profile (state, when and why).
type WishLast struct {
	ID     int64
	State  string
	At     time.Time
	Reason string // fail_reason of a failed wish
}

// ExternalTitlePage is one page of the search section «Noch nicht in der Bibliothek».
type ExternalTitlePage struct {
	Items []ExternalTitleCard
	Total int
}
