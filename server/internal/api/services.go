package api

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Services the HTTP layer depends on. Each is implemented by one package and wired in
// cmd/slouchcat. Handlers never query the database themselves.

// AuthService resolves and creates sessions (package auth).
type AuthService interface {
	// SetupRequired reports whether no account exists yet.
	SetupRequired(ctx context.Context) (bool, error)
	// Login checks e-mail and password and creates a session. It returns the plain token exactly
	// once; the store keeps only its hash. Unknown e-mail and wrong password both return
	// model.ErrInvalidCredentials; a client over the attempt limit (model.ClientIP of ctx) gets
	// model.ErrTooManyAttempts.
	Login(ctx context.Context, email, password, userAgent string) (model.Session, string, error)
	// Resolve returns the session of a plain token or model.ErrNotFound. It may extend the
	// session (sliding expiry) and then reports extended, so the cookie is re-issued.
	Resolve(ctx context.Context, token string) (session model.Session, extended bool, err error)
	// Logout revokes the session.
	Logout(ctx context.Context, sessionID int64) error
}

// AdminService implements the wizard and library administration (package admin).
type AdminService interface {
	// RunSetup creates the owner account, its default profile and the first library, stores the
	// TMDB key encrypted after checking it, queues the first scan and returns a session with its
	// plain token. Errors: model.ErrSetupDone, *model.ValidationError, model.ErrLibraryPathInvalid,
	// model.ErrTMDBKeyInvalid, model.ErrTMDBUnreachable.
	RunSetup(ctx context.Context, req model.SetupRequest, userAgent string) (model.Session, string, error)
	ListLibraries(ctx context.Context) ([]model.Library, error)
	GetLibrary(ctx context.Context, id int64) (model.Library, error)
	// CreateLibrary validates the path, detects the file system and queues the first scan.
	CreateLibrary(ctx context.Context, req model.LibraryRequest) (model.Library, error)
	// ScanLibrary queues a full scan (idempotent while one is active).
	ScanLibrary(ctx context.Context, id int64) (model.Library, error)
	// AcceptLibraryLoss lifts the loss protection for the next scan and queues it.
	AcceptLibraryLoss(ctx context.Context, id int64) (model.Library, error)
}

// CatalogService answers the start page and the detail page through the visibility layer
// (package discover).
type CatalogService interface {
	Home(ctx context.Context, profile model.Profile) ([]model.HomeRow, error)
	// Item returns the detail or model.ErrNotFound when the item is unknown or not visible.
	Item(ctx context.Context, profile model.Profile, itemID string) (model.ItemDetail, error)
	// Image opens a stored image of a visible item (WebP when preferWebP and stored, else JPEG) or
	// returns model.ErrNotFound.
	Image(ctx context.Context, profile model.Profile, itemID, kind, size string, preferWebP bool) (model.StreamSource, error)
}

// PlaybackService decides playback and records progress (package playback).
type PlaybackService interface {
	// Start returns the playback description; mediaFileID may be empty for the primary file.
	Start(ctx context.Context, profile model.Profile, itemID, mediaFileID, deviceID string) (model.PlaybackInfo, error)
	// Open opens the media file for direct play through the paths package.
	Open(ctx context.Context, profile model.Profile, itemID, mediaFileID string) (model.StreamSource, error)
	// Report stores a progress event and returns the merged progress.
	Report(ctx context.Context, profile model.Profile, itemID string, ev model.ProgressEvent) (model.Progress, error)
}
