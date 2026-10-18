// Package model holds the plain data types that cross package boundaries between the services
// (auth, admin, visibility, discover, playback, library, metadata) and the HTTP layer (api). It has
// no dependencies, so every package can import it without cycles. Shapes follow api/openapi.yaml;
// the api package maps them to the generated types.
package model

import (
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
)

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
	ID    int64
	Email string
	Role  string // owner, guest
}

// Profile is a viewing profile; its MaxRating and library rights drive the visibility layer.
type Profile struct {
	ID           int64
	AccountID    int64
	Name         string
	MaxRating    int    // 0, 6, 12, 16, 18
	Language     string // de, en
	AllLibraries bool
	Accent       string
}

// Session is a resolved browser session.
type Session struct {
	ID        int64
	Account   Account
	Profile   Profile
	ExpiresAt time.Time
}

// IsOwner reports whether the session belongs to the main account.
func (s Session) IsOwner() bool { return s.Account.Role == "owner" }

// Image is a poster or backdrop reference as the client uses it.
type Image struct {
	Kind        string // poster, backdrop
	Size        string // w500, w780, w1280
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
	Rating         *int // effective FSK rating, nil when unknown
	Poster         *Image
	Backdrop       *Image
	BackdropSmall  *Image // backdrop in w780, nil when that size is not stored
	Progress       *Progress
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
}

// ItemDetail is a title with everything the detail page shows.
type ItemDetail struct {
	ItemSummary
	OriginalTitle string
	Tagline       string
	Overview      string
	ReleaseDate   string // YYYY-MM-DD or empty
	Genres        []string
	TMDBID        *int64
	MediaFiles    []MediaFile
}

// HomeRow is one row of the start page.
type HomeRow struct {
	Key   string // picks, continue, recent
	Items []ItemSummary
}

// PlaybackInfo is the answer of startPlayback.
type PlaybackInfo struct {
	Mode             string // direct
	Item             ItemSummary
	MediaFile        MediaFile
	StreamURL        string
	ResumePositionMs int64
}

// ProgressEvent is one raw event from a client.
type ProgressEvent struct {
	MediaFileID string
	DeviceID    string
	PositionMs  int64
	DurationMs  int64
	PlayedMs    int64
	Seq         int64
	State       string // playing, paused, ended
	ClientTime  time.Time
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
