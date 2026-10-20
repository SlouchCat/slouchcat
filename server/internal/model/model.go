// Package model holds the plain data types that cross package boundaries between the services
// (auth, admin, visibility, discover, playback, library, metadata) and the HTTP layer (api). It has
// no dependencies, so every package can import it without cycles. Shapes follow api/openapi.yaml;
// the api package maps them to the generated types.
package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Sentinel errors that the api package maps to problem responses.
var (
	ErrNotFound           = errors.New("not found")            // 404 not_found (also for invisible items)
	ErrInvalidCredentials = errors.New("invalid credentials")  // 401 invalid_credentials
	ErrSetupDone          = errors.New("setup already done")   // 409 setup_done
	ErrLibraryPathInvalid = errors.New("library path invalid") // 422 library_path_invalid
	ErrTMDBKeyInvalid     = errors.New("tmdb key invalid")     // 422 tmdb_key_invalid
	ErrTMDBUnreachable    = errors.New("tmdb unreachable")     // 422 tmdb_unreachable
	ErrTooManyAttempts    = errors.New("too many attempts")    // 429 too_many_attempts
	// ErrProviderNotConfigured: a metadata provider without key was asked (422 provider_not_configured).
	ErrProviderNotConfigured = errors.New("provider not configured")
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
	Role         string // owner, guest
	MaxRating    int    // ceiling of every profile of the account (18 for the owner)
	AllLibraries bool
	LibraryIDs   []int64        // libraries of a guest; empty when AllLibraries
	Limits       PlaybackLimits // W-8 caps of a guest account (WatchLimitMinutes unused)
}

// Profile is a viewing profile; its MaxRating and library rights drive the visibility layer
// (capped by the account's own limits in the view visible_items).
type Profile struct {
	ID           int64
	AccountID    int64
	Name         string
	MaxRating    int    // 0, 6, 12, 16, 18
	Language     string // de, en
	AllLibraries bool
	LibraryIDs   []int64 // libraries of a profile with AllLibraries false
	Accent       string
	HasPin       bool
	IsPrimary    bool // the account's primary profile: manages profiles, cannot be deleted
	// Limits are the profile's own W-8/K-5 limits (zero = none); Effective applies the account's caps.
	Limits    PlaybackLimits
	Effective PlaybackLimits
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

// IsOwner reports whether the session belongs to the main account.
func (s Session) IsOwner() bool { return s.Account.Role == "owner" }

// HasProfile reports whether a profile is active (profile ids start at 1).
func (s Session) HasProfile() bool { return s.Profile.ID != 0 }

// IsPrimary reports whether the active profile is the account's primary profile.
func (s Session) IsPrimary() bool { return s.HasProfile() && s.Profile.IsPrimary }

// ProfileInput carries the fields of createProfile and updateProfile; nil fields keep their value
// (update) or take the default (create).
type ProfileInput struct {
	Name         *string
	MaxRating    *int
	Language     *string
	Accent       *string
	AllLibraries *bool
	LibraryIDs   *[]int64
	PIN          *string         // create only
	Limits       *PlaybackLimits // replaces all limits at once; nil keeps them
}

// Guest is a guest account with its limits.
type Guest struct {
	ID           int64
	Email        string
	MaxRating    int
	AllLibraries bool
	LibraryIDs   []int64
	ProfileCount int
	CreatedAt    time.Time
	LastSeenAt   *time.Time
	Limits       PlaybackLimits // W-8 caps (WatchLimitMinutes unused)
}

// GuestUpdate changes the limits of a guest account; nil fields keep their value.
type GuestUpdate struct {
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
	ID   int64
	Name string
	Kind string // movies, series; set by ListAccountLibraries
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
	Seasons         int // child counts
	Episodes        int
	Extras          int
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
	PositionMs int64
	DurationMs int64
	Completed  bool
	UpdatedAt  time.Time
	WatchTime  *WatchTime // K-5: present when the profile has a daily limit
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
	PosterSmall    *Image // w200 (M1b image pipeline)
	PosterLarge    *Image // w800
	BackdropLarge  *Image // w1920
	Accent         string // #rrggbb from the poster palette, "" until the pipeline has run
	Progress       *Progress

	// Series tree and extras (M3). ParentID is the season of an episode, the series of a season or
	// the parent title of an extra. SeriesTitle is set on seasons and episodes (profile language).
	ParentID      string
	SeasonNumber  *int
	EpisodeNumber *int
	EpisodeEnd    *int // last episode of a file that holds several episodes
	SeriesTitle   string
	ExtraKind     string // trailer, featurette, deleted_scene, behind_the_scenes, interview, scene, short, other
	ChildCount    *int   // visible seasons of a series, visible episodes of a season
}

// CastMember is one credit of a title (B-3).
type CastMember struct {
	PersonID  int64
	Name      string
	Role      string // cast, director, writer, creator
	Character string
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

// Quality rungs of the ladder (W-8, plan 3.5). QualityAuto picks the highest allowed rung.
const (
	QualityAuto = "auto"
	Quality1080 = "1080p"
	Quality720  = "720p"
	Quality480  = "480p"
	Quality360  = "360p"
)

// QualityOption is one rung of the ladder as offered to the player.
type QualityOption struct {
	Quality   string
	Width     int
	Height    int
	Bitrate   int // peak video bit rate in bit/s
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
	ReleaseDate   string // YYYY-MM-DD or empty
	Genres        []string
	TMDBID        *int64
	TVDBID        *int64
	MediaFiles    []MediaFile // versions, primary first; VersionLabel always set

	Series       *ItemSummary  // of a season or an episode
	Season       *ItemSummary  // of an episode
	Seasons      []ItemSummary // of a series, by number, specials last
	Episodes     []ItemSummary // of a season, by number
	Extras       []ItemSummary // of a film, series or season
	NextEpisode  *ItemSummary  // next visible episode of the same series
	Cast         []CastMember
	TrailerCount int
}

// HomeRow is one row of the start page.
type HomeRow struct {
	Key   string // picks, continue, recent
	Items []ItemSummary
}

// PlaybackInfo is the answer of startPlayback.
type PlaybackInfo struct {
	Mode              string // direct, hls
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
}

// TrackSelection names the tracks a playback delivers.
type TrackSelection struct {
	AudioTrack       int    // stream index, -1 when the file has no audio
	SubtitleTrack    int64  // track id, 0 = none
	SubtitleDelivery string // text, burn; empty = none
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
}

// StreamSource is an opened media or image file ready for http.ServeContent. The caller closes it.
type StreamSource struct {
	File      io.ReadSeekCloser
	MediaType string
	Size      int64
	ModTime   time.Time
	Name      string // for logging only, never a host path with user data
}

// LibraryRequest creates a library with one root.
type LibraryRequest struct {
	Name string
	Kind string // movies, series
	Path string
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
	ID      int64
	Path    string
	FSType  string
	Watch   string // inotify, poll, inotify_poll
	Warning string // network_share, fuse_unknown or empty
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
	ID        int64
	Name      string
	Kind      string
	Roots     []LibraryRoot
	Scan      LibraryScan
	CreatedAt time.Time
}

// Playback sessions and stream tokens (M1b, plan 5.4).

// ErrUnauthenticated: a stream token is missing, forged, revoked or expired (401 unauthenticated).
// ErrTranscodeBusy: the transcode budget stayed exhausted (503 transcode_busy).
// ErrSegmentRestarted: a newer seek restarted the encoder while a segment request waited (503
// segment_restarted, retry after a second).
var (
	ErrUnauthenticated  = errors.New("unauthenticated")
	ErrTranscodeBusy    = errors.New("transcode budget exhausted")
	ErrSegmentRestarted = errors.New("segment request superseded by a newer seek")
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
	HDR        bool     `json:"hdr,omitempty"` // the display shows HDR
	Name       string   `json:"name,omitempty"`
	Type       string   `json:"type,omitempty"` // desktop, phone, tablet, tv, other
}

// PlaybackRequest is the body of startPlayback.
type PlaybackRequest struct {
	MediaFileID      string // empty = primary file
	DeviceID         string
	Capabilities     *PlaybackCapabilities // nil = H.264/AAC with MSE (a browser of the M1a client)
	AudioTrack       *int                  // stream index; nil = the file's default track
	SubtitleTrack    int64                 // track id; 0 = none
	SubtitleDelivery string                // auto, text, burn; empty = auto
	Quality          string                // auto, 1080p, …; empty = auto
	PositionMs       *int64                // start position; nil = the stored resume position
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
}

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
