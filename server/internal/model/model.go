// Package model holds the plain data types that cross package boundaries between the services
// (auth, admin, visibility, discover, playback, library, metadata) and the HTTP layer (api). It has
// no dependencies, so every package can import it without cycles. Shapes follow api/openapi.yaml;
// the api package maps them to the generated types.
package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
)

// Sentinel errors that the api package maps to problem responses.
var (
	ErrNotFound           = errors.New("not found")            // 404 not_found (also for invisible items)
	ErrInvalidCredentials = errors.New("invalid credentials")  // 401 invalid_credentials
	ErrSetupDone          = errors.New("setup already done")   // 409 setup_done
	ErrLibraryPathInvalid = errors.New("library path invalid") // 422 library_path_invalid
	// ErrScanRunning: a library root cannot be removed while a scan of that library runs, because
	// the scan loads the roots once and writes media_files with their ids afterwards (409
	// scan_running, idea 02, E12).
	ErrScanRunning = errors.New("scan running") // 409 scan_running
	// ErrConfirmMismatch: the typed confirmation does not repeat what is about to be removed
	// (422 confirm_mismatch, idea 02, E12).
	ErrConfirmMismatch = errors.New("confirmation does not match") // 422 confirm_mismatch
	ErrTMDBKeyInvalid  = errors.New("tmdb key invalid")            // 422 tmdb_key_invalid
	ErrTMDBUnreachable = errors.New("tmdb unreachable")            // 422 tmdb_unreachable
	ErrTooManyAttempts = errors.New("too many attempts")           // 429 too_many_attempts
	// ErrProviderNotConfigured: a metadata provider without key was asked (422 provider_not_configured).
	ErrProviderNotConfigured = errors.New("provider not configured")
)

// Audio errors (M6).
var (
	ErrFeedExists           = errors.New("feed already subscribed")      // 409 feed_exists
	ErrFeedUnreachable      = errors.New("feed unreachable")             // 422 feed_unreachable
	ErrFeedInvalid          = errors.New("feed invalid")                 // 422 feed_invalid
	ErrRatingBelowDefault   = errors.New("rating below library default") // 422 rating_below_library_default
	ErrEnclosureUnavailable = errors.New("enclosure unavailable")        // 502 enclosure_unavailable
)

// Discover and curation errors (M7).
var (
	ErrNotListable    = errors.New("not listable")    // 422 not_listable (seasons and extras on the watchlist)
	ErrNotFavoritable = errors.New("not favoritable") // 422 not_favoritable (favourites: series and podcasts only)
	ErrNotEditable    = errors.New("not editable")    // 422 not_editable (TMDB collections)
)

// Incoming webhook errors (M9, I-8).
var (
	// ErrWebhookToken: no integration holds the token in the path (401 invalid_webhook_token).
	ErrWebhookToken = errors.New("unknown webhook token")
	// ErrWebhookPathOutside: the reported path lies under no library root after the integration's
	// mapping (422 path_outside_roots).
	ErrWebhookPathOutside = errors.New("webhook path outside every library root")
	// ErrWebhookRateLimited: more than ten requests in a second for one integration
	// (429 too_many_requests).
	ErrWebhookRateLimited = errors.New("too many webhook requests")
)

// Profile and access errors (M2).
var (
	ErrPinRequired            = errors.New("pin required")             // 403 pin_required
	ErrPinInvalid             = errors.New("pin invalid")              // 403 pin_invalid
	ErrPinLocked              = errors.New("pin locked")               // 429 pin_locked (see PinLockedError)
	ErrPrimaryProfileRequired = errors.New("primary profile required") // 403 primary_profile_required
	ErrPrimaryProfile         = errors.New("primary profile")          // 409 primary_profile
	ErrInvitationGone         = errors.New("invitation gone")          // 410 invitation_gone
)

// Role errors (ADR 0014). They are conflicts, not permission questions: the caller has the right,
// the state forbids the step. Who may call at all is the visibility level owner, enforced in the
// middleware (403 owner_required).
var (
	ErrOwnerProtected   = errors.New("owner protected")    // 409 owner_protected
	ErrLastAdmin        = errors.New("last administrator") // 409 last_admin
	ErrAdminLimitsFixed = errors.New("admin limits fixed") // 409 admin_limits_fixed
)

// ScanRunningError is ErrScanRunning with the name of the library whose scan runs;
// errors.Is(err, ErrScanRunning) holds.
type ScanRunningError struct {
	Library string
}

func (e *ScanRunningError) Error() string {
	return fmt.Sprintf("scan of library %q running", e.Library)
}

// Is makes errors.Is(err, ErrScanRunning) true.
func (e *ScanRunningError) Is(target error) bool { return target == ErrScanRunning }

// PinLockedError is ErrPinLocked with the remaining lock; errors.Is(err, ErrPinLocked) holds.
type PinLockedError struct {
	RetryAfter time.Duration
}

func (e *PinLockedError) Error() string {
	return fmt.Sprintf("pin locked for another %s", e.RetryAfter.Round(time.Second))
}

// Is makes errors.Is(err, ErrPinLocked) true.
func (e *PinLockedError) Is(target error) bool { return target == ErrPinLocked }

type deviceIDKey struct{}

// WithDeviceID stores the device id of the request (cookie slouchcat_device): PIN locks count per
// profile and device, and a new session remembers its device.
func WithDeviceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, deviceIDKey{}, id)
}

// DeviceID returns the id stored by WithDeviceID, or "".
func DeviceID(ctx context.Context) string {
	id, _ := ctx.Value(deviceIDKey{}).(string)
	return id
}

type clientIPKey struct{}

// WithClientIP stores the client address of the request (login and setup rate limit).
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIP returns the client address stored by WithClientIP, or "".
func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

// FieldError names one invalid request field with a stable code (required, too_short, invalid_email, …).
type FieldError struct {
	Field string
	Code  string
}

// ValidationError carries field errors; the api package answers 422 validation_failed.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}

// Add appends a field error and returns the receiver, which stays nil-safe for callers that
// create it lazily.
func (e *ValidationError) Add(field, code string) *ValidationError {
	if e == nil {
		e = &ValidationError{}
	}
	e.Fields = append(e.Fields, FieldError{Field: field, Code: code})
	return e
}

// Account is the main (owner) or a guest account (M2).
type Account struct {
	ID           int64
	Email        string
	Role         string // admin (server and libraries) or user (its own profiles only), ADR 0014
	IsOwner      bool   // the one account that cannot be deleted or demoted (accounts.is_owner)
	MaxRating    int    // ceiling of every profile of the account (18 for an administrator)
	AllLibraries bool
	LibraryIDs   []int64        // libraries of the account; empty when AllLibraries
	Limits       PlaybackLimits // W-8 caps of the account (WatchLimitMinutes unused)
}

// Role values of Account.Role. The owner is not a third value: it is an administrator with
// IsOwner set, so "is an administrator" is one comparison and cannot be half forgotten.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Profile is a viewing profile; its MaxRating and library rights drive the visibility layer
// (capped by the account's own limits in the view visible_items).
type Profile struct {
	ID        int64
	AccountID int64
	Name      string
	MaxRating int    // 0, 6, 12, 16, 18
	Language  string // de, en: the interface language
	// MetadataLanguage is the language of the metadata the profile reads (ADR 0018), one of the
	// server's available metadata languages; empty = like Language.
	MetadataLanguage string
	AllLibraries     bool
	LibraryIDs       []int64 // libraries of a profile with AllLibraries false
	Accent           string
	HasPin           bool
	HasOfflinePin    bool // a separate offline PIN is set (AN-19)
	IsPrimary        bool // the account's primary profile: manages profiles, cannot be deleted
	// Limits are the profile's own W-8/K-5 limits (zero = none); Effective applies the account's caps.
	Limits    PlaybackLimits
	Effective PlaybackLimits
	// Prefs are the language and subtitle preferences the server chooses tracks by (concept 13).
	Prefs PlaybackPrefs
}

// Subtitle rules of a profile (concept 13 section 11.3, profiles.subtitle_mode).
const (
	SubtitleModeOff     = "off"
	SubtitleModeForced  = "forced" // the default (F-3)
	SubtitleModeForeign = "foreign"
	SubtitleModeAlways  = "always"
)

// PlaybackPrefs are the language and subtitle preferences of a profile (concept 13 section 11.4):
// audio and subtitle languages as ISO 639-1 codes in order of preference, and the subtitle rule.
type PlaybackPrefs struct {
	AudioLanguages    []string
	SubtitleMode      string
	SubtitleLanguages []string
}

// PlaybackLimits are the W-8 limits (bandwidth, output height, concurrent streams) and the K-5
// daily watch-time limit; a zero value means no limit. Accounts (guests) carry the first three as
// caps over every profile of the account.
type PlaybackLimits struct {
	MaxBitrateKbps    int
	MaxHeight         int // 360, 480, 720, 1080, 2160
	MaxStreams        int
	WatchLimitMinutes int
}

// Cap returns the stricter of l and other per field (zero = no limit).
func (l PlaybackLimits) Cap(other PlaybackLimits) PlaybackLimits {
	minLimit := func(a, b int) int {
		switch {
		case a == 0:
			return b
		case b == 0:
			return a
		}
		return min(a, b)
	}
	return PlaybackLimits{
		MaxBitrateKbps:    minLimit(l.MaxBitrateKbps, other.MaxBitrateKbps),
		MaxHeight:         minLimit(l.MaxHeight, other.MaxHeight),
		MaxStreams:        minLimit(l.MaxStreams, other.MaxStreams),
		WatchLimitMinutes: minLimit(l.WatchLimitMinutes, other.WatchLimitMinutes),
	}
}

// Session is a resolved browser session. A session without an active profile (signed in, profile
// not chosen yet) has the zero Profile; HasProfile tells them apart.
type Session struct {
	ID        int64
	Account   Account
	Profile   Profile
	DeviceID  string // device the session started on (PIN locks count per profile and device)
	ExpiresAt time.Time
}

// IsAdmin reports whether the session's account administers the server (ADR 0014). Together with
// IsPrimary it is what the middleware checks for x-visibility admin.
func (s Session) IsAdmin() bool { return s.Account.Role == RoleAdmin }

// IsOwner reports whether the session belongs to the owner, the one account that appoints and
// demotes administrators (x-visibility owner).
func (s Session) IsOwner() bool { return s.Account.IsOwner }

// HasProfile reports whether a profile is active (profile ids start at 1).
func (s Session) HasProfile() bool { return s.Profile.ID != 0 }

// IsPrimary reports whether the active profile is the account's primary profile.
func (s Session) IsPrimary() bool { return s.HasProfile() && s.Profile.IsPrimary }

// ProfileInput carries the fields of createProfile and updateProfile; nil fields keep their value
// (update) or take the default (create).
type ProfileInput struct {
	Name      *string
	MaxRating *int
	Language  *string
	// MetadataLanguage is the metadata language (ADR 0018); "" resets it to the interface language.
	MetadataLanguage *string
	Accent           *string
	AllLibraries     *bool
	LibraryIDs       *[]int64
	PIN              *string         // create only
	Limits           *PlaybackLimits // replaces all limits at once; nil keeps them
	// The playback preferences (concept 13); each nil field keeps its value (update) or takes the
	// column default (create).
	AudioLanguages    *[]string
	SubtitleMode      *string
	SubtitleLanguages *[]string
}

// AccountAccessUpdate changes what an account may see and how much it may stream; nil fields keep
// their value. Only an account with RoleUser has these limits (ADR 0014).
type AccountAccessUpdate struct {
	MaxRating    *int
	AllLibraries *bool
	LibraryIDs   *[]int64
	Limits       *PlaybackLimits // replaces the three W-8 caps at once; nil keeps them
}

// InvitationRequest creates an invitation link.
type InvitationRequest struct {
	Note         string
	MaxRating    int
	AllLibraries bool
	LibraryIDs   []int64
}

// Invitation is an invitation link with its state (open, used, expired, revoked). Token and URL
// are set only on the created invitation.
type Invitation struct {
	ID           int64
	Note         string
	State        string
	MaxRating    int
	AllLibraries bool
	LibraryIDs   []int64
	CreatedAt    time.Time
	ExpiresAt    time.Time
	UsedAt       *time.Time
	UsedByEmail  string
	Token        string
}

// InvitationPreview is what the invited person sees before accepting.
type InvitationPreview struct {
	Note         string
	MaxRating    int
	AllLibraries bool
	Libraries    []LibraryRef
	ExpiresAt    time.Time
}

// LibraryRef names a library.
type LibraryRef struct {
	ID            int64
	Name          string
	Kind          string // movies, series, audiobooks, podcasts; set by ListAccountLibraries
	DefaultRating *int   // audio libraries only
}

// AcceptInvitation is the input of the guest sign-up.
type AcceptInvitation struct {
	Token       string
	Email       string
	Password    string
	ProfileName string
	Language    string // de, en; empty = de
}

// ItemRating is the administration view of the ratings of a title (not scoped to a profile).
type ItemRating struct {
	ID              string
	Kind            string
	Title           string
	OwnRating       *int
	EffectiveRating *int
	RatingSource    string
}

// Administration views of M3 (not scoped to a profile; never ItemRef in the contract).

// AdminMediaFile is a media file as the administrator sees it, with its path.
type AdminMediaFile struct {
	ID           string
	RootID       int64
	Path         string // relative to the library root
	SizeBytes    int64
	Available    bool
	VersionLabel string
	ProbedAt     *time.Time
	ProbeError   string
}

// AdminItem is the unfiltered view of a title.
type AdminItem struct {
	ID              string
	Kind            string
	Title           string
	Year            *int
	LibraryID       int64
	ParentID        string
	SeasonNumber    *int
	EpisodeNumber   *int
	EpisodeEnd      *int
	MetadataState   string
	MetadataError   string
	MatchedAt       *time.Time
	TMDBID          *int64
	TVDBID          *int64
	IMDBID          string
	LockedFields    []string
	OwnRating       *int
	EffectiveRating *int
	RatingSource    string
	TrailerState    string
	Trailers        []Trailer
	MediaFiles      []AdminMediaFile
	Providers       []string      // the ordered metadata sources of the title (ADR 0015)
	AniListID       *int64        // the AniList id, when the fill job found one
	FieldSources    []FieldSource // which provider won which field
	Seasons         int           // child counts
	Episodes        int
	Extras          int
	Images          []AdminItemImage // the decided image source per choosable kind (concept 10 7.3)
	// ImageKinds are the kinds the administrator can choose or upload now (concept 15 7.5): an
	// episode's still only while the setting "episode stills" is on.
	ImageKinds []string
	// OriginalLanguage is the ISO 639-1 code of the original language (concept 13 11.7); "" when it
	// was looked up and is unknown, nil while it was not looked up.
	OriginalLanguage *string
	// Metadata is the editable metadata in the server's metadata language (concept 15 7.4,
	// contract 1.15.0).
	Metadata *AdminItemMetadata
}

// MetadataCandidate is a search result of a metadata provider (manual match, B-6).
type MetadataCandidate struct {
	Provider      string // tmdb, tvdb
	ProviderID    string
	Title         string
	OriginalTitle string
	Year          *int
	Overview      string
	PosterURL     string // small poster at the provider's image host
}

// Issue is one entry of the maintenance lists (B-9).
type Issue struct {
	Kind        string // unmatched, no_images, unrated, orphaned, unplayable, rating_mismatch
	ItemID      string
	ItemKind    string
	Title       string
	Year        *int
	LibraryID   int64
	Path        string
	MediaFileID string
	Detail      string
}

// IssuePage is one page of issues with the total count.
type IssuePage struct {
	Total int
	Items []Issue
}

// DuplicateItem is one title of a duplicate group with its files.
type DuplicateItem struct {
	ItemID    string
	Kind      string
	Title     string
	Year      *int
	LibraryID int64
	Files     []AdminMediaFile
}

// DuplicateGroup is a set of titles or files that exist twice (B-9).
type DuplicateGroup struct {
	Reason string // provider_id, title_year, file_hash
	Items  []DuplicateItem
}

// RelinkUserData counts the user data rows of a title and its descendants per table (ADR 0023).
type RelinkUserData struct {
	Progress         int64
	ProgressEvents   int64
	ProgressDays     int64
	PlaybackSessions int64
	Watchlist        int64
	Favorites        int64
	CollectionItems  int64
	Notifications    int64
	UserPicks        int64
	Downloads        int64
	ActivityEvents   int64
}

// RelinkPair is a soft-deleted film or series and the live copy that would merge into it, with
// the user data on the deleted title (ADR 0023, maintenance action D1).
type RelinkPair struct {
	DeletedID, LiveID string
	LibraryID         int64
	LibraryName       string
	Kind              string // movie or series
	Title             string // of the live copy
	Year              *int   // of the live copy
	DeletedTitle      string
	DeletedYear       *int
	DeletedAt         time.Time
	UserData          RelinkUserData
}

// RelinkPairList is the first pairs and the number of all pairs.
type RelinkPairList struct {
	Total int
	Pairs []RelinkPair
}

// RelinkPairRef names a pair to merge: the deleted title and its live copy.
type RelinkPairRef struct{ DeletedID, LiveID string }

// RelinkedPair is one merged pair: the kept (formerly deleted) id, the removed copy, the merged
// and the moved nodes.
type RelinkedPair struct {
	KeptID, RemovedID string
	Merged, Moved     int
}

// RelinkOutcome is the result of the maintenance relink: the merged pairs, the skipped ones and,
// when a running scan stopped the run before the last pair, StoppedBy "scan_running".
type RelinkOutcome struct {
	Relinked  []RelinkedPair
	Skipped   int
	StoppedBy string
}

// Image is a poster or backdrop reference as the client uses it.
type Image struct {
	Kind        string // poster, backdrop
	Size        string // w200, w400, w800 (poster), w780, w1280, w1920 (backdrop)
	ContentHash string
	Width       int
	Height      int
}

// Progress is the merged playback position of a profile for an item.
type Progress struct {
	PositionMs   int64
	DurationMs   int64
	Completed    bool
	UpdatedAt    time.Time
	PlaybackRate *float64   // H-3: speed the profile last used for the title, nil until set
	WatchTime    *WatchTime // K-5: present when the profile has a daily limit
}

// WatchTime is the watch time of a profile today against its daily limit (K-5).
type WatchTime struct {
	LimitMinutes     int
	UsedMinutes      int
	RemainingMinutes int
}

// WatchDay is one day of the watch-time report (K-5).
type WatchDay struct {
	Date    string // YYYY-MM-DD in the server's local time zone
	Minutes int
}

// WatchReport is the watch-time report of one profile, oldest day first.
type WatchReport struct {
	ProfileID    int64
	LimitMinutes int // 0 = none
	Days         []WatchDay
}

// Percent returns the position as a percentage of the duration (0 when the duration is unknown).
func (p Progress) Percent() float64 {
	if p.DurationMs <= 0 {
		return 0
	}
	return min(100, float64(p.PositionMs)*100/float64(p.DurationMs))
}

// ItemSummary is a title in a list (ItemRef in the contract).
type ItemSummary struct {
	ID             string
	Kind           string
	Title          string
	Year           *int
	RuntimeMinutes *int
	Rating         *int   // effective FSK rating, nil when unknown
	Poster         *Image // w400
	Backdrop       *Image // w1280
	BackdropSmall  *Image // backdrop in w780, nil when that size is not stored
	// Logo is the title logo in w400 (ADR 0018), set only on the detail and the spotlight entries.
	Logo          *Image
	PosterSmall   *Image // w200 (M1b image pipeline)
	PosterLarge   *Image // w800
	BackdropLarge *Image // w1920
	BackdropUhd   *Image // w3840, only from an original of at least 2560 px (4K, high density)
	Accent        string // #rrggbb from the poster palette, "" until the pipeline has run
	Progress      *Progress

	// Series tree and extras (M3). ParentID is the season of an episode, the series of a season or
	// the parent title of an extra. SeriesTitle is set on seasons and episodes (profile language).
	ParentID      string
	SeasonNumber  *int
	EpisodeNumber *int
	EpisodeEnd    *int // last episode of a file that holds several episodes
	SeriesTitle   string
	ExtraKind     string // trailer, featurette, deleted_scene, behind_the_scenes, interview, scene, short, other
	ChildCount    *int   // visible seasons of a series, visible episodes of a season
	// WatchedCount is the number of visible episodes of a season the profile has completed
	// (concept 17 section 2.3, contract 1.20.0); set on seasons only.
	WatchedCount *int
	// Watched marks a series or podcast whose visible episodes the profile has all completed, by the
	// rule of the library status watched (contract 1.22.0); nil on other kinds and on a series or
	// podcast without visible episodes.
	Watched *bool
	// SeriesRef is the visible series of an episode with its title and images (concept 15 P-1,
	// contract 1.14.0 ItemRef.seriesRef); nil on every other kind.
	SeriesRef *SeriesRef

	// Audio (M6). Author of an audiobook or podcast; PublishedAt and DurationMs of a podcast episode,
	// DurationMs also the book-wide duration of an audiobook; Explicit marks itunes:explicit (AN-2).
	Author      string
	PublishedAt *time.Time
	DurationMs  *int64
	Explicit    bool

	// Note is the main user's note on a pick (M7, E-2) or on a curated spotlight entry (M11); set
	// in the rows picks and spotlight.
	Note string
	// HasTrailer says whether a trailer is stored (M11, D3); nil = not looked up (the contract's
	// optional ItemRef.hasTrailer), filled for the rows spotlight and picks of the start page only.
	HasTrailer *bool
	// Overview is the overview in the profile language (concept 17 section 2.3); the contract
	// passes it on only in the episode list of a season's detail (ItemDetail.Overview is the
	// detail's own, the same text).
	Overview string
	// InWatchlist says whether the title is on the profile's watchlist (E-5); on every summary
	// since contract 1.20.0 (concept 17 D-9, the watchlist button on the cover).
	InWatchlist bool
}

// SeriesRef is the series of an episode as its card shows it: id, title in the profile language and
// the images of the series in the sizes of ItemSummary (concept 15 P-1).
type SeriesRef struct {
	ID            string
	Title         string
	Poster        *Image // w400
	PosterSmall   *Image // w200
	PosterLarge   *Image // w800
	Backdrop      *Image // w1280
	BackdropSmall *Image // w780
	BackdropLarge *Image // w1920
	BackdropUhd   *Image // w3840
}

// CastMember is one credit of a title (B-3).
type CastMember struct {
	PersonID  int64
	Name      string
	Role      string // cast, director, writer, creator
	Character string
	Image     *Image // the portrait (w185 JPEG row, ADR 0026), nil without one
}

// Trailer is a YouTube trailer of a title in one language (B-8, W-10).
type Trailer struct {
	ID        int64
	Lang      string // ISO 639-1
	YouTubeID string
	Title     string
	Source    string // tmdb, youtube, ai, manual
	Official  bool
}

// EmbedURL is the IFrame player URL of the trailer (youtube-nocookie.com, plan 5.3).
func (t Trailer) EmbedURL() string {
	return "https://www.youtube-nocookie.com/embed/" + t.YouTubeID
}

// PersonCredit is one visible title of a person.
type PersonCredit struct {
	Item      ItemSummary
	Role      string
	Character string
}

// Person with the credits the profile may see.
type Person struct {
	ID      int64
	Name    string
	Credits []PersonCredit
	Image   *Image // the portrait (w185 JPEG row, ADR 0026), nil without one
}

// MediaFile describes one playable file of a title.
type MediaFile struct {
	ID           string
	Container    string
	MimeType     string
	VideoCodec   string
	AudioCodec   string
	Width        int
	Height       int
	DurationMs   int64
	SizeBytes    int64
	VersionLabel string
	Available    bool
	Bitrate      int64  // container bit rate in bit/s, 0 unknown
	HDR          string // hdr10, hdr10plus, dolby_vision, hlg; empty for SDR
	Tracks       *TrackList
}

// Track is one audio or subtitle track of a media file (embedded stream or sidecar file).
type Track struct {
	ID       int64
	Index    int    // stream index; -1 for a sidecar
	Kind     string // audio, subtitle
	Codec    string
	Language string
	Title    string
	Default  bool
	Forced   bool
	Source   string // embedded, sidecar
	RelPath  string // sidecar file relative to the library root (never sent to clients)
	Text     bool   // text subtitle format deliverable as WebVTT
	Channels int
	// HearingImpaired marks a subtitle track for the deaf and hard of hearing (SDH; sidecar .sdh,
	// .hi, .cc or such a title, tracks.is_hearing_impaired); it never leaves the server.
	HearingImpaired bool
}

// TrackList are the tracks of one media file.
type TrackList struct {
	MediaFileID string
	Audio       []Track
	Subtitles   []Track
}

// Chapter is a chapter mark of a media file.
type Chapter struct {
	StartMs int64
	EndMs   int64
	Title   string
	// Auto marks a replacement chapter that the server set every 10 minutes (concept 13, F-6).
	Auto bool
	// ImageURL is the relative URL of the chapter image of a film or an episode, set only when the
	// image exists (concept 13 stage 3); never in a download manifest.
	ImageURL string
}

// Segment is a detected intro or outro of an episode (W-9).
type Segment struct {
	StartMs int64
	EndMs   int64
	Source  string // fingerprint, chapter, manual
}

// Segments are the intro and outro of a title; nil when unknown.
type Segments struct {
	Intro *Segment
	Outro *Segment
}

// Quality rungs of the ladder (W-8, plan 3.5). QualityAuto picks the highest allowed rung up to
// 1080p; Quality2160 is only an explicit choice on a hardware backend (ADR 0021). QualityOriginal
// is no rung: the video is never re-encoded (direct play or remux, concept 13 11.5).
const (
	QualityAuto     = "auto"
	QualityOriginal = "original"
	Quality2160     = "2160p"
	Quality1080     = "1080p"
	Quality720      = "720p"
	Quality480      = "480p"
	Quality360      = "360p"
)

// QualityOption is one rung of the ladder as offered to the player.
type QualityOption struct {
	Quality   string
	Width     int
	Height    int
	Bitrate   int    // peak video bit rate in bit/s
	Codec     string // video codec of the output (ffprobe name), empty when unknown
	Direct    bool   // the video is not re-encoded (direct play or remux)
	Available bool
	Current   bool
}

// Subtitle delivery modes.
const (
	SubtitleAuto = "auto"
	SubtitleText = "text"
	SubtitleBurn = "burn"
)

// ItemDetail is a title with everything the detail page shows.
type ItemDetail struct {
	ItemSummary
	OriginalTitle string
	Tagline       string
	Overview      string
	// Discover (M7): a favourite of the profile; the visible collections (the watchlist mark is
	// on ItemSummary since contract 1.20.0).
	Favorite    bool
	Collections []CollectionRef
	// RecommendedByMe: the profile holds a user pick of the title that has not expired (concept 18
	// part B, 14.6); never who else or how many recommended it.
	RecommendedByMe bool
	ReleaseDate     string // YYYY-MM-DD or empty
	Genres          []string
	TMDBID          *int64
	TVDBID          *int64
	MediaFiles      []MediaFile // versions, primary first; VersionLabel always set

	Series       *ItemSummary  // of a season or an episode
	Season       *ItemSummary  // of an episode
	Seasons      []ItemSummary // of a series, by number, specials last
	Episodes     []ItemSummary // of a season, by number
	Extras       []ItemSummary // of a film, series or season
	NextEpisode  *ItemSummary  // next visible episode of the same series
	NextUp       *NextUp       // of a series (every season) or a season (that season), concept 17 3.3
	Cast         []CastMember
	TrailerCount int

	// Audio (M6): narrator, series and volume of an audiobook and its book-wide chapters; the newest
	// visible episodes of a podcast (Episodes, at most 50) with their total; the podcast of an episode.
	Narrator     string
	SeriesName   string
	SeriesIndex  *float64
	Chapters     []Chapter
	EpisodeCount *int
	Podcast      *ItemSummary
}

// NextUp is the episode the big button of a series or season plays (contract 1.20.0, concept 17
// section 3.3): Reason is start, resume, next or rewatch.
type NextUp struct {
	Reason  string
	Episode ItemSummary
}

// HomeRow is one row of the start page (M7: key order from home_rows). Collections and Genres
// are set only on the rows collections and genres, whose Items stay empty.
// HomePage is the answer of getHome: the rows in stored order and the key of the row the client
// draws as the stage (ADR 0011: spotlight, when that row is present; "" when the page has no
// stage). The server decides, the client only draws (CLAUDE.md rule 12).
type HomePage struct {
	Rows    []HomeRow
	HeroRow string
	// HeroStart is the index into the stage row's Items the stage begins with, drawn once per
	// page view by the server (concept 08, F-2, ADR 0016); nil without a stage.
	// HeroIntervalSeconds is spotlight_config.rotation_seconds (F-10); 0 without a stage.
	HeroStart           *int
	HeroIntervalSeconds int
}

type HomeRow struct {
	// ID names the row (concept 15 P-7): the key for the fixed rows, "library:<id>" for a row
	// library; Key is its kind (spotlight, picks, continue, recent, released, watchlist,
	// collections, genres, libraries, library, radio).
	ID          string
	Key         string
	LibraryID   int64  // the library of a row library, else 0
	LibraryName string // its name, the row title «Neu in …»
	Items       []ItemSummary
	Collections []CollectionRef
	Genres      []GenreRef
	Libraries   []LibraryTile      // only in the row libraries
	Stations    []RadioHomeStation // only in the row radio («Meine Sender», concept 16 stage 2)
}

// LibraryTile is one tile of the row «Meine Medien» (concept 15 P-7): a library the profile sees
// with the number of its visible top-level titles and the newest of them for the collage.
type LibraryTile struct {
	ID        int64
	Name      string
	Kind      string
	ItemCount int
	Items     []ItemSummary // at most LibraryTileCovers, newest first
}

// LibraryTileCovers is the number of posters of a tile's collage.
const LibraryTileCovers = 4

// MaxVisibleHomeRows caps the visible start page rows (F-12, E-9): hidden rows do not count, and
// the start page never shows more.
const MaxVisibleHomeRows = 15

// Home row keys (model.HomeRow.Key, table home_rows), in the default order of ADR 0011.
const (
	RowSpotlight   = "spotlight"
	RowPicks       = "picks"
	RowContinue    = "continue"
	RowRecent      = "recent"
	RowReleased    = "released"
	RowWatchlist   = "watchlist"
	RowCollections = "collections"
	RowGenres      = "genres"
	RowLibraries   = "libraries" // «Meine Medien», concept 15 P-7
	RowLibrary     = "library"   // «Neu in …», one row per library
	RowRadio       = "radio"     // «Meine Sender», concept 16 stage 2
)

// HomeRowKeys lists every row key: the eight fixed rows in the default order, then the two kinds
// of concept 15 P-7 and «Meine Sender» of concept 16.
var HomeRowKeys = []string{RowSpotlight, RowPicks, RowContinue, RowRecent, RowReleased, RowWatchlist, RowCollections, RowGenres,
	RowLibraries, RowLibrary, RowRadio}

// Discover (M7). ---------------------------------------------------------------------------

// HomeRowSetting is the stored order and visibility of one start page row (E-9).
type HomeRowSetting struct {
	ID          string // the key, or "library:<id>" for a row library (concept 15 P-7)
	Key         string
	LibraryID   int64  // the library of a row library, else 0
	LibraryName string // its name
	Position    int
	Hidden      bool
}

// SearchQuery is a search of the active profile (E-4). MaxRating -1 means no rating filter;
// DeviceID is the session's device for the availability filter download.
type SearchQuery struct {
	Text         string
	Genre        string
	YearFrom     int
	YearTo       int
	MaxRating    int
	MediaTypes   []string // movie, series, audiobook, podcast
	Availability []string // local, download
	DeviceID     string
	Offset       int
	Limit        int
}

// SearchResult is the answer of search: titles paged, people and collections the first ten.
type SearchResult struct {
	Items       []ItemSummary
	Total       int
	People      []PersonRef
	Collections []CollectionRef
}

// PersonRef is a person in search results.
type PersonRef struct {
	ID    int64
	Name  string
	Image *Image // the portrait (w185 JPEG row, ADR 0026), nil without one
}

// GenreRef is a genre with the number of visible titles.
type GenreRef struct {
	Name  string
	Count int
}

// CollectionRef is a collection in rows, search results and on the detail page; Count counts
// visible members and Cover is the first visible member (nil when none).
type CollectionRef struct {
	ID    string
	Name  string
	Count int
	Cover *ItemSummary
	// Members are the first visible members in collection order, at most 20 (concept 17 section
	// 4.2, contract 1.20.0); set only on the collections of a title's detail, nil elsewhere.
	Members []ItemSummary
	// Image is the TMDB poster (w400 JPEG row, ADR 0026), set only when the profile sees every
	// live member.
	Image *Image
}

// Collection is a collection with one page of its visible members.
type Collection struct {
	ID       string
	Name     string
	Overview string
	Items    []ItemSummary
	Total    int
	Image    *Image // the TMDB poster like CollectionRef.Image
}

// Notification is one entry of a profile's in-app inbox (E-8).
type Notification struct {
	ID        int64
	ProfileID int64
	Kind      string // new_episode
	CreatedAt time.Time
	ReadAt    *time.Time
	Item      ItemSummary  // the new episode
	Parent    *ItemSummary // the favourite series or podcast, nil when not visible
}

// AdminUserPick is a title of the user picks in the administration (concept 18 part B, 14.7, ADR
// 0028): anonymous, the number of recommendations that have not expired and the newest of them.
type AdminUserPick struct {
	ItemID        string
	Kind          string
	Title         string
	Year          *int
	Count         int
	PickedOn      string // day of the newest recommendation (YYYY-MM-DD, server zone), never the time
	ExpiresInDays int    // whole days until the title leaves the list, rounded up
	PosterHash    string // content hash of the AdminPosterSize poster; "" = none stored
}

// UserPick is the answer to a recommendation (recommendItem): when it expires unless renewed and
// the retention in days.
type UserPick struct {
	ExpiresAt     time.Time
	RetentionDays int
}

// Recommendable reports whether a title of this kind may be a user pick: every kind except seasons
// and extras (concept 18 part B, 14.4).
func Recommendable(kind string) bool { return kind != "season" && kind != "extra" }

// Bounds of the retention of the user picks in days (picks_config.retention_days, default 60;
// concept 18 part B, 14.7).
const (
	PicksRetentionMin = 7
	PicksRetentionMax = 365
)

// AdminPosterSize is the poster size of the administration tables (posterUrl, M11 step 5b).
const AdminPosterSize = "w200"

// AdminBackdropSize is the backdrop size of the administration tiles (backdropUrl, concept 12).
const AdminBackdropSize = "w780"

// AdminItemHit is a title found by the admin search (never an ItemRef).
type AdminItemHit struct {
	ID          string
	Kind        string
	Title       string
	Year        *int
	LibraryID   int64
	ParentTitle string
}

// AdminCollection is a collection in the administration, unfiltered (E-6).
type AdminCollection struct {
	ID        string
	Source    string // tmdb, manual
	Name      string
	NameDe    string
	TMDBID    *int64
	ItemCount int
	Items     []AdminCollectionItem
}

// AdminCollectionItem is one member of a collection in the administration.
type AdminCollectionItem struct {
	ItemID     string
	Kind       string
	Title      string
	Year       *int
	Position   int
	PosterHash string // content hash of the AdminPosterSize poster; "" = none stored
}

// TraktStatus is the state of the spotlight list fetched from Trakt (E-7, I-7, ADR 0011): ListKey
// is the configured list (spotlight_config.list_key), Entries the fetched entries of all media that
// are not yet stale (fetched within twice the configured interval), before the library intersection.
type TraktStatus struct {
	Configured  bool
	LastSuccess *time.Time
	LastAttempt *time.Time
	LastError   string
	ListKey     string
	Entries     int
}

// Spotlight administration (M11, ADR 0011). The configuration is the singleton row of
// spotlight_config; the list kinds come from package trakt. An entry is one row of the fetched
// list with its library hit and its override; an orphan is an override without an entry.
type SpotlightConfig struct {
	Enabled       bool
	ListKey       string
	ListURL       string // only with ListKey "url"
	Media         string // movie, show, both
	ItemCount     int    // 5..20
	MinHits       int    // 0..10
	IntervalHours int    // 6..48
	// RotationSeconds is how long the stage shows one title (concept 08, F-10): 3..15, default 5.
	RotationSeconds int
	ListKinds       []SpotlightListKind
}

// SpotlightListKind is one kind of public Trakt list (trakt.Kind) for the administration.
type SpotlightListKind struct {
	Key          string
	Media        []string
	MediaFilters bool
	NeedsURL     bool
}

// SpotlightConfigUpdate is the writable part of SpotlightConfig (setSpotlightConfig).
type SpotlightConfigUpdate struct {
	Enabled       bool
	ListKey       string
	ListURL       string
	Media         string
	ItemCount     int
	MinHits       int
	IntervalHours int
	// RotationSeconds is optional: nil keeps the stored value (contract 1.4.0, an older
	// administration client must not reset it silently).
	RotationSeconds *int
}

// SpotlightEntries is the answer of getSpotlightEntries.
type SpotlightEntries struct {
	ListKey string
	Entries []SpotlightEntry
	Orphans []SpotlightEntry
}

// SpotlightEntry is one fetched entry with its library hit (ItemID "" = no hit, never on the start
// page) and its override; Orphaned marks an override without an entry (Position 0).
type SpotlightEntry struct {
	Media      string // movie, show
	TmdbID     int64
	Position   int
	Title      string
	Year       *int
	ItemID     string
	ItemTitle  string
	Hidden     bool
	PinnedRank *int
	Note       string
	Orphaned   bool
	PosterHash string // content hash of the hit's AdminPosterSize poster; "" = no hit or none stored
}

// SpotlightOverrideUpdate is the body of setSpotlightEntryOverride: a nil pointer keeps the value,
// Clear* removes it (JSON null); Empty is true for an empty body, which removes the override.
type SpotlightOverrideUpdate struct {
	Hidden          *bool
	PinnedRank      *int
	ClearPinnedRank bool
	Note            *string
	ClearNote       bool
	Empty           bool
}

// PlaybackInfo is the answer of startPlayback.
type PlaybackInfo struct {
	Mode              string // direct, hls
	MediaKind         string // video, audio (M6: audiobooks and podcast episodes use the audio player)
	Parts             []PlaybackPart
	Item              ItemSummary
	MediaFile         MediaFile
	StreamURL         string // Range URL (direct) or master playlist (hls), both with the stream token
	ResumePositionMs  int64
	PlaybackSessionID string
	Transcoding       *Transcoding // nil for direct play
	Tracks            TrackList
	Selection         TrackSelection
	SubtitleURL       string // WebVTT URL with the stream token when a text track is selected
	Qualities         []QualityOption
	Chapters          []Chapter
	Segments          Segments
	NextEpisode       *ItemSummary
	WatchTime         *WatchTime
	Device            *DeviceRef
	// Trickplay are the preview tiles of the seek bar (concept 18 part A); nil for audio titles,
	// titles without video and the library setting off.
	Trickplay *Trickplay
}

// Trickplay is the preview set of one media file for the app (concept 18, 8.2): the grid, the
// time of every point and the sheets that exist, with the state of the set (queued, running,
// complete, failed).
type Trickplay struct {
	Version    string
	State      string
	IntervalMs int64
	TileWidth  int
	TileHeight int
	Columns    int
	Rows       int
	PointsMs   []int64
	Sheets     []TrickplaySheet
}

// TrickplaySheet is one sheet of a preview set with its relative URL.
type TrickplaySheet struct {
	Index int
	URL   string
}

// Media kinds of a playback (PlaybackInfo.MediaKind).
const (
	MediaKindVideo = "video"
	MediaKindAudio = "audio"
)

// PlaybackPart is one part of a multi-part audiobook on the single playback timeline (M6).
type PlaybackPart struct {
	MediaFileID string
	StartMs     int64
	DurationMs  int64
}

// TrackSelection names the tracks a playback delivers.
type TrackSelection struct {
	AudioTrack       int    // stream index, -1 when the file has no audio
	SubtitleTrack    int64  // track id, 0 = none
	SubtitleDelivery string // text, burn; empty = none
	// AudioReason and SubtitleReason say why these tracks play (trackpick.Audio*, trackpick.Subtitle*
	// values, concept 13 11.3); empty when the server reports none.
	AudioReason    string
	SubtitleReason string
}

// DeviceRef names the device of a playback.
type DeviceRef struct {
	ID   string
	Name string
}

// ProgressEvent is one raw event from a client.
type ProgressEvent struct {
	MediaFileID       string
	PlaybackSessionID string // optional: the session whose position is recorded (W-6)
	DeviceID          string
	PositionMs        int64
	DurationMs        int64
	PlayedMs          int64
	Seq               int64
	State             string // playing, paused, ended
	ClientTime        time.Time
	PlaybackRate      *float64 // H-3: speed in use, stored per profile and title when present
}

// StreamSource is an opened media or image file ready for http.ServeContent. The caller closes it.
type StreamSource struct {
	File      io.ReadSeekCloser
	MediaType string
	Size      int64
	ModTime   time.Time
	Name      string // for logging only, never a host path with user data
	ETag      string // strong validator (quoted) for If-Range; empty = none
}

// Families of libraries (ADR 0015): a library only says which contents its folders may hold.
// video libraries scan folders of films and series; audiobooks libraries hold books (a folder);
// podcasts libraries hold feed subscriptions and, since ADR 0015, podcast folders. The former kinds
// movies and series are burnt and never reused.
const (
	LibraryVideo      = "video"
	LibraryAudiobooks = "audiobooks"
	LibraryPodcasts   = "podcasts"
)

// ValidLibraryKind reports whether kind names a library family.
func ValidLibraryKind(kind string) bool {
	switch kind {
	case LibraryVideo, LibraryAudiobooks, LibraryPodcasts:
		return true
	}
	return false
}

// AudioLibraryKind reports whether a library of this kind carries a default age rating (AN-2).
func AudioLibraryKind(kind string) bool { return kind == LibraryAudiobooks || kind == LibraryPodcasts }

// Contents of a library root (library_roots.content, ADR 0015): which name parser and which
// identifier read the folder.
const (
	RootMovie     = "movie"
	RootSeries    = "series"
	RootAudiobook = "audiobook"
	RootPodcast   = "podcast"
)

// ValidRootContent reports whether content names a root content.
func ValidRootContent(content string) bool {
	switch content {
	case RootMovie, RootSeries, RootAudiobook, RootPodcast:
		return true
	}
	return false
}

// ContentAllowed reports whether a root of this content may live in a library of this family
// (F-3: only video mixes).
func ContentAllowed(kind, content string) bool {
	switch kind {
	case LibraryVideo:
		return content == RootMovie || content == RootSeries
	case LibraryAudiobooks:
		return content == RootAudiobook
	case LibraryPodcasts:
		return content == RootPodcast
	}
	return false
}

// TitleKindOfContent is the item kind of the titles a root content produces.
func TitleKindOfContent(content string) string {
	switch content {
	case RootSeries:
		return "series"
	case RootAudiobook:
		return "audiobook"
	case RootPodcast:
		return "podcast"
	}
	return "movie"
}

// Metadata providers (ADR 0015). tmdb and tvdb identify titles and fill fields, anilist only
// fills, local is the file-based identifier of audiobooks and podcast folders.
const (
	ProviderTMDB    = "tmdb"
	ProviderTVDB    = "tvdb"
	ProviderAniList = "anilist"
	ProviderLocal   = "local"
)

// Providers lists every provider in the order the interface shows them.
var Providers = []string{ProviderTMDB, ProviderTVDB, ProviderAniList, ProviderLocal}

// Error codes of ValidRootProviders (field errors of a 422 validation_failed).
const (
	CodeProvidersNotAllowed = "providers_not_allowed"
	CodeContentNotAllowed   = "content_not_allowed"
	CodeRootsRequired       = "roots_required"
	CodeRootOverlap         = "root_overlap"
	// CodePathWithContent refuses a new path of a folder together with a new content (ADR 0023).
	CodePathWithContent = "path_with_content"
	// CodeRootFilesMissing refuses a new path of a folder under which none of its known files
	// lies (ADR 0023): a wrong folder would let the next scan lose them all.
	CodeRootFilesMissing = "root_files_missing"
)

// identifiers lists the providers that may stand first in the list of a root content, fillers
// those that may follow (concept section 2).
var (
	identifiers = map[string][]string{
		RootMovie:     {ProviderTMDB},
		RootSeries:    {ProviderTMDB, ProviderTVDB},
		RootAudiobook: {ProviderLocal},
		RootPodcast:   {ProviderLocal},
	}
	fillers = map[string][]string{
		RootMovie:     {ProviderAniList},
		RootSeries:    {ProviderTMDB, ProviderTVDB, ProviderAniList},
		RootAudiobook: {},
		RootPodcast:   {},
	}
)

// ProvidersForContent returns the providers a root of this content may carry, identifiers first.
func ProvidersForContent(content string) []string {
	out := append([]string{}, identifiers[content]...)
	for _, f := range fillers[content] {
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// CanIdentify reports whether provider may stand first in the list of a root of this content.
func CanIdentify(content, provider string) bool {
	return slices.Contains(identifiers[content], provider)
}

// ValidRootProviders reports whether providers is a valid ordered list for a root of this
// content: not empty, no duplicates, the first entry an allowed identifier, every further entry
// an allowed filler.
func ValidRootProviders(content string, providers []string) bool {
	if len(providers) == 0 || !ValidRootContent(content) {
		return false
	}
	if !CanIdentify(content, providers[0]) {
		return false
	}
	seen := map[string]bool{providers[0]: true}
	for _, p := range providers[1:] {
		if seen[p] || !slices.Contains(fillers[content], p) {
			return false
		}
		seen[p] = true
	}
	return true
}

// DefaultProviders is the list a new root of this content starts with: the order the code used
// before ADR 0015 (TMDB first, TVDB as the fallback of series).
func DefaultProviders(content string) []string {
	switch content {
	case RootSeries:
		return []string{ProviderTMDB, ProviderTVDB}
	case RootAudiobook, RootPodcast:
		return []string{ProviderLocal}
	}
	return []string{ProviderTMDB}
}

// ParseProviders reads a stored provider list (library_roots.providers, items.providers). An
// unreadable value is the empty list; callers treat it like a list without filler.
func ParseProviders(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

// ProvidersJSON renders a provider list for storage.
func ProvidersJSON(providers []string) string {
	if providers == nil {
		providers = []string{}
	}
	raw, _ := json.Marshal(providers) // a []string always marshals
	return string(raw)
}

// MetadataSettings are the server's metadata settings as the administration sees them (concept 10,
// ADR 0018, getMetadataSettings); KnownLanguages, AffectedTitles and LocalImageTitles are read-only.
type MetadataSettings struct {
	TextLangs        []string
	ImageLangs       map[string][]string // poster, backdrop, logo; "" = without text
	ImageKinds       []string            // logo, season_poster, episode_still
	ImageMinWidth    map[string]int      // poster, backdrop
	ImageByVote      bool
	LocalImages      bool
	RatingCountry    string
	TMDBIncludeAdult bool
	FanartEnabled    bool
	KnownLanguages   []string
	AffectedTitles   int
	LocalImageTitles int
}

// LibraryImages is the image deviation of a library (concept 10 4.5).
type LibraryImages struct {
	Langs map[string][]string
	Kinds []string
}

// LibraryMetadata is the deviation of a library (nil = like the server) with the effective values.
type LibraryMetadata struct {
	LibraryID      int64
	TextLangs      *[]string
	Images         *LibraryImages
	Effective      MetadataSettings
	AffectedTitles int
}

// ApplyMetadataRequest queues metadata.apply (concept 10 6.5).
type ApplyMetadataRequest struct {
	LibraryIDs []int64  // empty = every library
	What       []string // texts, images
}

// ApplyMetadataResult is the queued run and the number of titles it covers.
type ApplyMetadataResult struct {
	JobID  int64
	Titles int
}

// MetadataProviderInfo describes one provider to the administration (listMetadataProviders).
type MetadataProviderInfo struct {
	ID         string
	Configured bool     // a key is present, or none is needed
	Roles      []string // identify, fill
	Contents   []string // root contents the provider may stand in the list of
	Languages  []string // text languages it delivers (de, en, …)
}

// FieldSource names the provider that won one field of a title (item_field_sources).
type FieldSource struct {
	Field    string
	Lang     string // empty for fields without a language
	Provider string
}

// LibraryRequest creates a library with its folders (F-13): every folder brings its content and
// its provider list; video and audiobook libraries need at least one, a podcast library may start
// without (its titles come from feeds as well).
type LibraryRequest struct {
	Name          string
	Kind          string // video, audiobooks, podcasts
	Roots         []LibraryRootRequest
	DefaultRating *int // audiobooks and podcasts only (AN-2); nil = 0 for those kinds
	// Trickplay is the trickplay setting of a video library (concept 18, 6.1); nil = the defaults
	// (DefaultLibraryTrickplay). Not allowed for audio libraries.
	Trickplay *LibraryTrickplay
}

// LibraryRootRequest is one folder of a library with what it holds and who describes it.
type LibraryRootRequest struct {
	Path      string
	Content   string   // movie, series, audiobook, podcast
	Providers []string // ordered; the first identifies, the others fill
}

// LibraryRootUpdate changes the content, the provider list or the path of a folder
// (updateLibraryRoot). A content change empties the folder's titles like a removal would and
// therefore repeats the path (ConfirmPath); a list change and a new path (ADR 0023 point 4) need no
// confirmation.
type LibraryRootUpdate struct {
	Content     *string
	Providers   []string // nil = unchanged
	Path        *string  // nil = unchanged
	ConfirmPath string
}

// LibraryUpdate changes the name or the default rating of a library (updateLibrary).
type LibraryUpdate struct {
	Name          *string
	DefaultRating *int
	Trickplay     *LibraryTrickplay // video libraries only (concept 18, 6.1)
}

// SetupRequest is the input of the first-start wizard.
type SetupRequest struct {
	Email    string
	Password string
	Language string // de, en; empty = de
	TMDBKey  string
	Library  LibraryRequest
}

// LibraryRoot is one directory of a library.
type LibraryRoot struct {
	ID        int64
	Path      string
	FSType    string
	Watch     string   // inotify, poll, inotify_poll
	Warning   string   // network_share, fuse_unknown or empty
	Content   string   // movie, series, audiobook, podcast
	Providers []string // ordered metadata sources
	// ItemCount is the number of titles with a file in this folder, the upper bound of what
	// removing it takes out of the library; FileCount is the files in it, which all go with it
	// (idea 02, E12).
	ItemCount int
	FileCount int
}

// LibraryScan is the state of the running or last scan.
type LibraryScan struct {
	State          string // never, queued, running, done, failed, aborted
	StartedAt      *time.Time
	FinishedAt     *time.Time
	FilesSeen      int
	FilesTotal     int
	LastError      string
	LossAcceptedAt *time.Time
	ItemCount      int
	FileCount      int
}

// Library with its roots and scan state.
type Library struct {
	ID            int64
	Name          string
	Kind          string
	DefaultRating *int // audio libraries only (AN-2)
	Roots         []LibraryRoot
	Scan          LibraryScan
	KeyframeIndex *LibraryKeyframeIndex // video libraries only (concept 14, F-7)
	Trickplay     *LibraryTrickplay     // video libraries only (concept 18, 6.1)
	// TrickplayEstimate is the storage estimate of the previews (concept 18, 9); getLibrary of a
	// video library only.
	TrickplayEstimate *LibraryTrickplayEstimate
	CreatedAt         time.Time
}

// Trickplay modes of a library (concept 18, 6.1; libraries.trickplay_movies and
// trickplay_series). TrickplaySeriesStart is for series only.
const (
	TrickplayAhead       = "ahead"
	TrickplayOnStart     = "on_start"
	TrickplaySeriesStart = "series_start"
	TrickplayOff         = "off"
)

// Scopes of the series look-ahead of TrickplaySeriesStart (F-14).
const (
	TrickplayScopeSeries      = "series"
	TrickplayScopeSeasonRest  = "season_rest"
	TrickplayScopeNextEpisode = "next_episode"
)

// TrickplaySetting is the trickplay setting of one kind of title in a library: the mode, Keep
// («Speichern», read only with on_start) and Scope (series only, read only with series_start).
type TrickplaySetting struct {
	Mode  string
	Keep  bool
	Scope string
}

// Stores reports whether the setting keeps the sets for good (ahead, series_start, on_start with
// Keep); a setting that does not store keeps them only while a playback is open.
func (t TrickplaySetting) Stores() bool {
	return t.Mode == TrickplayAhead || t.Mode == TrickplaySeriesStart || (t.Mode == TrickplayOnStart && t.Keep)
}

// LibraryTrickplay is the trickplay setting of a video library, one for films (titles of kind
// movie) and one for series (kind episode).
type LibraryTrickplay struct {
	Movies TrickplaySetting
	Series TrickplaySetting
}

// DefaultLibraryTrickplay is the setting of a new video library (F-11): «Im Voraus» for both, with
// «Speichern» and the whole series as scope.
func DefaultLibraryTrickplay() LibraryTrickplay {
	return LibraryTrickplay{
		Movies: TrickplaySetting{Mode: TrickplayAhead, Keep: true},
		Series: TrickplaySetting{Mode: TrickplayAhead, Keep: true, Scope: TrickplayScopeSeries},
	}
}

// For returns the setting of a title kind: movie the films, episode the series; any other kind
// has no trickplay (an empty setting, Mode "").
func (t LibraryTrickplay) For(kind string) TrickplaySetting {
	switch kind {
	case "movie":
		return t.Movies
	case "episode":
		return t.Series
	}
	return TrickplaySetting{}
}

// Validate returns the field errors of t below field ("trickplay"): an unknown mode or scope is
// invalid_value, series_start for films too.
func (t LibraryTrickplay) Validate(verr *ValidationError, field string) *ValidationError {
	switch t.Movies.Mode {
	case TrickplayAhead, TrickplayOnStart, TrickplayOff:
	default:
		verr = verr.Add(field+".movies.mode", "invalid_value")
	}
	switch t.Series.Mode {
	case TrickplayAhead, TrickplayOnStart, TrickplaySeriesStart, TrickplayOff:
	default:
		verr = verr.Add(field+".series.mode", "invalid_value")
	}
	switch t.Series.Scope {
	case TrickplayScopeSeries, TrickplayScopeSeasonRest, TrickplayScopeNextEpisode:
	default:
		verr = verr.Add(field+".series.scope", "invalid_value")
	}
	return verr
}

// LibraryTrickplayEstimate is the storage estimate of the previews of a video library per kind
// (concept 18, 9) and the free space of the cache drive (-1: unknown).
type LibraryTrickplayEstimate struct {
	Movies         TrickplayKindEstimate
	Series         TrickplayKindEstimate
	CacheFreeBytes int64
}

// TrickplayKindEstimate is the estimate of one kind: titles with a present file, the present files
// with a duration and their running time, the estimate per title (mean) and for the library, and
// the bytes the stored sets take today.
type TrickplayKindEstimate struct {
	Titles        int
	Files         int
	DurationMs    int64
	PerTitleBytes int64
	LibraryBytes  int64
	StoredBytes   int64
}

// LibraryKeyframeIndex is the keyframe index coverage of the files of a video library that are
// worth indexing (concept 14, 4.1): Indexed + Pending + Unavailable = VideoFiles.
type LibraryKeyframeIndex struct {
	VideoFiles  int
	Indexed     int // keyframes_source cues, mp4 or scan
	Pending     int // keyframes_source NULL
	Unavailable int // keyframes_source none
}

// AdminPodcast is a podcast subscription as the administration sees it (M6, H-2); not an ItemSummary.
type AdminPodcast struct {
	ID            string
	LibraryID     int64
	Title         string
	Author        string
	FeedURL       string
	EpisodeCount  int
	Explicit      bool
	LastCheckedAt *time.Time
	LastError     string
	CreatedAt     time.Time
	// FeedOrigin names who chose the feed (admin, file); empty for a folder podcast without feed.
	// RootID is the folder the podcast was read from, LocalEpisodeCount its episodes with a file.
	FeedOrigin        string
	RootID            *int64
	LocalEpisodeCount int
}

// PodcastCreate subscribes a podcast library to a feed.
type PodcastCreate struct {
	LibraryID int64
	FeedURL   string
}

// Playback sessions and stream tokens (M1b, plan 5.4).

// ErrUnauthenticated: a stream token is missing, forged, revoked or expired (401 unauthenticated).
// ErrTranscodeBusy: the transcode budget stayed exhausted (503 transcode_busy).
// ErrSegmentRestarted: a newer seek restarted the encoder while a segment request waited (503
// segment_restarted, retry after a second).
// ErrPlaybackSourceChanged: the file of a remux playback changed since it started, so its lost
// HLS output cannot be rebuilt; the playback session ended (409 playback_source_changed, concept
// 14, 6.4).
var (
	ErrUnauthenticated       = errors.New("unauthenticated")
	ErrTranscodeBusy         = errors.New("transcode budget exhausted")
	ErrSegmentRestarted      = errors.New("segment request superseded by a newer seek")
	ErrPlaybackSourceChanged = errors.New("playback source changed since the start")
)

// PlaybackCapabilities are the codecs and the HLS support the client reports with startPlayback
// (canPlayType checks; device profiles follow in M4).
type PlaybackCapabilities struct {
	Containers []string `json:"containers,omitempty"` // mp4, webm, mkv; empty = MP4 family only
	Video      []string `json:"video"`                // h264, hevc, …
	Audio      []string `json:"audio"`                // aac, ac3, eac3, …
	HLS        string   `json:"hls"`                  // native, mse, none
	MaxWidth   int      `json:"maxWidth,omitempty"`   // screen size in device pixels, 0 unknown
	MaxHeight  int      `json:"maxHeight,omitempty"`
	HDR        bool     `json:"hdr"` // the display shows HDR (false is stored and shown as such)
	Name       string   `json:"name,omitempty"`
	Type       string   `json:"type,omitempty"` // desktop, phone, tablet, tv, other
}

// PlaybackRequest is the body of startPlayback.
type PlaybackRequest struct {
	MediaFileID      string // empty = primary file
	DeviceID         string
	Capabilities     *PlaybackCapabilities // nil = H.264/AAC with MSE (a browser of the M1a client)
	AudioTrack       *int                  // stream index; nil = the server chooses (the file's default track)
	SubtitleTrack    int64                 // track id; 0 = the server chooses (none)
	SubtitleOff      bool                  // no subtitles; refused together with SubtitleTrack
	SubtitleDelivery string                // auto, text, burn; empty = auto
	Quality          string                // auto, 1080p, …; empty = auto
	QualityCap       string                // a remembered rung of the device (V-17); empty = none
	// RememberForSeries keeps the languages of a choice by hand for the series of an episode.
	RememberForSeries bool
	PositionMs        *int64 // start position; nil = the stored resume position
}

// Transcoding describes an HLS playback: remux (copy video) or transcode, the encoder backend and
// why the file is not played directly.
type Transcoding struct {
	Mode     string // remux, transcode
	Backend  string // nvenc, qsv, vaapi, software; empty for remux
	Hardware bool   // encoded on a GPU
	Quality  string
	Width    int
	Height   int
	Bitrate  int // peak video bit rate in bit/s
	Tonemap  bool
	Reason   string
}

// Playback session end reasons (playback_sessions.stopped_by).
const (
	StoppedByClient  = "client"
	StoppedByAdmin   = "admin"
	StoppedByLimit   = "limit"
	StoppedByLogout  = "logout"
	StoppedByRevoked = "revoked"
)

// PlaybackState is what the server remembers of a playback session in memory
// (getPlaybackSessionState, concept 13): whether it stopped and, when known, who stopped it (one
// of the StoppedBy values; empty for a running or an expired playback).
type PlaybackState struct {
	Stopped   bool
	StoppedBy string
}

// Errors of the W-8 and K-5 limits. ErrStreamLimit: the profile runs as many streams as allowed
// (409 stream_limit_reached). ErrWatchLimit: the daily watch time is used up (403
// watch_limit_reached).
var (
	ErrStreamLimit = errors.New("stream limit reached")
	ErrWatchLimit  = errors.New("watch-time limit reached")
)

// StreamLimitError is ErrStreamLimit with the numbers of the problem extensions (maxStreams,
// activeStreams).
type StreamLimitError struct {
	MaxStreams    int
	ActiveStreams int
}

func (e *StreamLimitError) Error() string {
	return fmt.Sprintf("%v: %d of %d streams running", ErrStreamLimit, e.ActiveStreams, e.MaxStreams)
}

func (e *StreamLimitError) Is(target error) bool { return target == ErrStreamLimit }

// WatchLimitError is ErrWatchLimit with the numbers of the problem extensions (limitMinutes,
// usedMinutes).
type WatchLimitError struct {
	LimitMinutes int
	UsedMinutes  int
}

func (e *WatchLimitError) Error() string {
	return fmt.Sprintf("%v: %d of %d minutes used", ErrWatchLimit, e.UsedMinutes, e.LimitMinutes)
}

func (e *WatchLimitError) Is(target error) bool { return target == ErrWatchLimit }

// AdminDevice is a device as the administration sees it (W-7).
type AdminDevice struct {
	ID             string
	Name           string
	Type           string
	UserAgent      string
	OwnerEmail     string
	OwnerProfile   string
	CreatedAt      time.Time
	LastSeenAt     time.Time
	Capabilities   *PlaybackCapabilities
	CapabilitiesAt *time.Time
	ActiveStreams  int
}

// AdminStream is a running playback session as the administration sees it (W-6).
type AdminStream struct {
	ID             string
	AccountEmail   string
	ProfileName    string
	Device         *DeviceRef
	ItemID         string
	ItemTitle      string
	MediaFileID    string
	Mode           string // direct, remux, transcode
	Backend        string
	Hardware       bool
	Quality        string
	Width          int
	Height         int
	Bitrate        int
	Reason         string
	StartedAt      time.Time
	LastActivityAt time.Time
	PositionMs     *int64
	DurationMs     *int64
	// PosterItemID and PosterHash name the poster of the overview (concept 11): the title's own,
	// for an episode its series'; empty without a stored poster.
	PosterItemID string
	PosterHash   string
	// BackdropItemID and BackdropHash name the AdminBackdropSize backdrop the same way (concept 12).
	BackdropItemID string
	BackdropHash   string
	// Kind is what the stream plays (concept 18, 17.4): the item kind movie, episode, extra,
	// audiobook or podcast_episode.
	Kind string
	// ParentTitle is the podcast of a podcast episode and the movie or series of an extra.
	ParentTitle string
	// ChapterTitle is an audiobook's chapter at PositionMs.
	ChapterTitle string
	// PartCount is the number of parts of an audiobook of several files (0 otherwise).
	PartCount int
	// AudioCodec and AudioBitrate (bit/s, 0 unknown) describe the audio of an audiobook or a
	// podcast episode: the probe's when played directly, aac and the session's over HLS, the
	// enclosure's codec from a feed.
	AudioCodec   string
	AudioBitrate int
	// Source is StreamSourceFeed for a podcast episode played from its enclosure, else StreamSourceFile.
	Source string
}

// Where a stream's audio or video comes from (AdminStream.Source).
const (
	StreamSourceFile = "file"
	StreamSourceFeed = "feed"
)

// PlaybackGrant is a valid stream token resolved to its playback session: the browser session
// (account and profile) it was issued to and the title and media file it covers.
type PlaybackGrant struct {
	ID          string // playback session id (ULID)
	Session     Session
	ItemID      string
	MediaFileID string
	Mode        string // direct, remux, transcode
	// SubtitleTrack is the text subtitle track the playback delivers (0 = none), for the WebVTT
	// endpoint, which serves only that track.
	SubtitleTrack int64
}

// HLSStream serves the playlists and fMP4 segments of one HLS playback. URLs inside playlists are
// supplied by the caller, so the stream token stays in the HTTP layer.
type HLSStream interface {
	MasterPlaylist(mediaURL string) []byte
	MediaPlaylist(initURL string, segmentURL func(i int) string) []byte
	OpenInit(ctx context.Context) (io.ReadSeekCloser, error)
	OpenSegment(ctx context.Context, i int) (io.ReadSeekCloser, error)
}

// Library views (concept 08, ADR 0016). -----------------------------------------------------

// LibraryNav is one library of the side navigation (contract LibraryNav): what the active profile
// sees of it. Contents are the content kinds of the library's folders (RootContent values), each
// once, in enum order; ItemCount counts visible top-level titles only.
type LibraryNav struct {
	ID        int64
	Name      string
	Kind      string // LibraryKind
	Contents  []string
	ItemCount int
}

// Sort orders of the library page (contract LibrarySort, F-6, F-12).
const (
	LibrarySortTitle    = "title"
	LibrarySortAdded    = "added"
	LibrarySortReleased = "released"
	LibrarySortVote     = "vote"
	LibrarySortRuntime  = "runtime"
	LibrarySortAge      = "age"
	LibrarySortPlayed   = "played"
	LibrarySortAuthor   = "author" // audiobook libraries only
)

// LibrarySorts lists every sort order in contract order.
var LibrarySorts = []string{LibrarySortTitle, LibrarySortAdded, LibrarySortReleased, LibrarySortVote,
	LibrarySortRuntime, LibrarySortAge, LibrarySortPlayed, LibrarySortAuthor}

// LibrarySortAscending reports whether sort is ascending by default (title, age, author); every
// other order is descending by default.
func LibrarySortAscending(sort string) bool {
	return sort == LibrarySortTitle || sort == LibrarySortAge || sort == LibrarySortAuthor
}

// Watch status of a title for a profile (contract LibraryItemStatus, F-8); exclusive per title.
const (
	LibraryStatusUnwatched = "unwatched"
	LibraryStatusStarted   = "started"
	LibraryStatusWatched   = "watched"
)

// LibraryQuery is one page request of the library page (listLibraryItems). Zero values mean
// "no filter"; MaxRating -1 means none (0 is a valid ceiling). Order "" is the default of Sort.
type LibraryQuery struct {
	Content   string // a RootContent value, "" = all
	Sort      string // a LibrarySort* value, "" = title
	Order     string // asc, desc, "" = default of Sort
	Genre     string
	YearFrom  int
	YearTo    int
	MaxRating int // -1 = none; the reader takes min(profile ceiling, MaxRating)
	Status    []string
	Letter    string // one of A..Z or #, "" = none; only with Sort title
	Offset    int
	Limit     int
}

// LibraryFacets are the filter values the library page may offer (contract LibraryFacets).
type LibraryFacets struct {
	Letters []string   // present initial letters of the filtered set, A..Z then #
	Genres  []GenreRef // genres with visible titles in the library, most frequent first
	Ratings []int      // present ratings at or below the profile ceiling, ascending
	// LetterStarts are the offsets of the first title of each initial letter in the active
	// direction, only with sort title at offset 0 (nil otherwise; concept 17 section 6).
	LetterStarts []LetterStart
}

// LetterStart is the offset of the first title of an initial letter (contract LetterStart).
type LetterStart struct {
	Letter string // A..Z or #
	Offset int
}

// LibraryPage is one page of a library with its facets (contract LibraryItemsPage).
type LibraryPage struct {
	Items  []ItemSummary
	Total  int
	Facets LibraryFacets
}

// ImageCandidate is one image a source offers for a kind of a title (listItemImageCandidates,
// concept 10 6.4). PreviewURL is the provider's small size; empty for a local file (the API sets
// the address of the preview handler).
type ImageCandidate struct {
	Provider   string
	Lang       string
	Width      int
	Height     int
	Score      float64
	VoteCount  int
	Source     string
	PreviewURL string
	Chosen     bool
}

// ImageCandidateList is the answer of listItemImageCandidates: the candidates and the providers
// that did not answer.
type ImageCandidateList struct {
	Candidates  []ImageCandidate
	Unavailable []string
}

// AdminItemImage is the decided image source of a kind for the title page (AdminItem.images,
// concept 10 7.3); Locked without Provider is a lock without choice. StoredSize and StoredHash name
// the stored image of such a lock (an upload) the preview shows through getAdminItemImage; "" = none
// stored.
type AdminItemImage struct {
	Kind       string
	Provider   string
	Lang       string
	Source     string
	PreviewURL string
	StoredSize string
	StoredHash string
	Chosen     bool
	Locked     bool
}
