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

// ProfileService is the profile picker, the PIN check and the profile management of the signed-in
// account (package auth). Methods that take the session act for its account and require the
// primary profile to be active (model.ErrPrimaryProfileRequired otherwise).
type ProfileService interface {
	// ListMyProfiles returns the account's profiles for the picker; works without an active profile.
	ListMyProfiles(ctx context.Context, accountID int64) ([]model.Profile, error)
	// SelectProfile activates a profile in place and returns the updated session. Errors:
	// model.ErrNotFound (not a profile of the account), model.ErrPinRequired, model.ErrPinInvalid,
	// *model.PinLockedError, model.ErrTooManyAttempts, *model.ValidationError. The device comes
	// from model.DeviceID(ctx).
	SelectProfile(ctx context.Context, session model.Session, profileID int64, pin string) (model.Session, error)
	// ListAccountLibraries returns the libraries the account may see (all for the main account), by name.
	ListAccountLibraries(ctx context.Context, account model.Account) ([]model.LibraryRef, error)
	ListProfiles(ctx context.Context, session model.Session) ([]model.Profile, error)
	CreateProfile(ctx context.Context, session model.Session, in model.ProfileInput) (model.Profile, error)
	UpdateProfile(ctx context.Context, session model.Session, id int64, in model.ProfileInput) (model.Profile, error)
	// DeleteProfile deletes a non-primary profile; end runs after the checks, before the delete.
	DeleteProfile(ctx context.Context, session model.Session, id int64, end func(ctx context.Context, profileID int64) error) error
	SetProfilePIN(ctx context.Context, session model.Session, id int64, pin string) error
	RemoveProfilePIN(ctx context.Context, session model.Session, id int64) error
}

// AccessService is the invitation and guest management of the main account plus the public
// invitation links (package auth).
type AccessService interface {
	// GetInvitation is the preview of an open link (model.ErrNotFound, model.ErrInvitationGone,
	// model.ErrTooManyAttempts for model.ClientIP of ctx).
	GetInvitation(ctx context.Context, token string) (model.InvitationPreview, error)
	// AcceptInvitation creates the guest account and starts its session (plain token returned once).
	AcceptInvitation(ctx context.Context, in model.AcceptInvitation, userAgent string) (model.Session, string, error)
	CreateInvitation(ctx context.Context, creator model.Account, req model.InvitationRequest) (model.Invitation, error)
	ListInvitations(ctx context.Context) ([]model.Invitation, error)
	RevokeInvitation(ctx context.Context, id int64) error
	ListGuests(ctx context.Context) ([]model.Guest, error)
	UpdateGuest(ctx context.Context, id int64, in model.GuestUpdate) (model.Guest, error)
	// RevokeAccount revokes and deletes a guest account; end runs after the checks, before the delete.
	RevokeAccount(ctx context.Context, accountID int64, end func(ctx context.Context, accountID int64) error) error
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
	// SetItemRating overrides (or, with nil, clears) the age rating of a title and returns its
	// ratings; model.ErrNotFound for an unknown title.
	SetItemRating(ctx context.Context, itemID string, rating *int) (model.ItemRating, error)

	// M3 (design 4.2). Every method answers model.ErrNotFound for an unknown or deleted title and
	// *model.ValidationError for invalid input.

	// CancelLibraryScan cancels the queued or running scan of a library.
	CancelLibraryScan(ctx context.Context, libraryID int64) (model.Library, error)
	// GetAdminItem returns the unfiltered administration view of a title.
	GetAdminItem(ctx context.Context, itemID string) (model.AdminItem, error)
	// SetItemMatch assigns a film or series to a provider entry (B-6) and queues the match.
	SetItemMatch(ctx context.Context, itemID, provider, providerID string) (model.AdminItem, error)
	// RefreshItem queues metadata, images, trailers and probes of a title.
	RefreshItem(ctx context.Context, itemID string) (model.AdminItem, error)
	// CreateItemTrailer stores a manual trailer from a YouTube id or URL.
	CreateItemTrailer(ctx context.Context, itemID, lang, youtubeID, url string) (model.Trailer, error)
	// DeleteItemTrailer removes a trailer of a title.
	DeleteItemTrailer(ctx context.Context, itemID string, trailerID int64) error
	// SetItemsRating applies SetItemRating to many titles; unknown ids are skipped and not counted.
	SetItemsRating(ctx context.Context, itemIDs []string, rating *int) (int, error)
	// SearchMetadata asks a provider (model.ErrProviderNotConfigured, model.ErrTMDBKeyInvalid,
	// model.ErrTMDBUnreachable).
	SearchMetadata(ctx context.Context, provider, kind, query string, year *int) ([]model.MetadataCandidate, error)
	// ListIssues returns one page of maintenance issues of a kind (limit 0 = 50).
	ListIssues(ctx context.Context, kind string, libraryID *int64, limit, offset int) (model.IssuePage, error)
	// ListDuplicates returns the duplicate groups.
	ListDuplicates(ctx context.Context, libraryID *int64) ([]model.DuplicateGroup, error)
	// MergeItems merges the sources into the target and returns the target.
	MergeItems(ctx context.Context, targetID string, sourceIDs []string) (model.AdminItem, error)
	// CleanupItems soft-deletes orphaned titles and removes media files as ignored.
	CleanupItems(ctx context.Context, itemIDs, mediaFileIDs []string) (items, files int, err error)
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
	// Trailers returns the trailers of a visible item, the profile language first, or model.ErrNotFound.
	Trailers(ctx context.Context, profile model.Profile, itemID string) ([]model.Trailer, error)
	// Person returns a person with the credits on visible titles; model.ErrNotFound when the person
	// is unknown or has no visible credit.
	Person(ctx context.Context, profile model.Profile, personID int64) (model.Person, error)
}

// PlaybackService decides playback, issues and checks stream tokens and records progress
// (package playback).
type PlaybackService interface {
	// Start decides the playback mode, creates a playback session of the browser session and
	// returns the playback description with the token URL. Errors: model.ErrNotFound,
	// *model.ValidationError, model.ErrTranscodeBusy.
	Start(ctx context.Context, session model.Session, itemID string, req model.PlaybackRequest) (model.PlaybackInfo, error)
	// Authorize resolves a stream token; model.ErrUnauthenticated when it is missing, forged,
	// revoked or expired.
	Authorize(ctx context.Context, token string) (model.PlaybackGrant, error)
	// OpenGrant opens the media file of a direct-play grant through the paths package after
	// re-checking visibility (model.ErrNotFound otherwise). When the File has a method
	// Done() <-chan struct{}, a close of that channel means the playback was revoked: the
	// handler aborts the response.
	OpenGrant(ctx context.Context, g model.PlaybackGrant) (model.StreamSource, error)
	// HLS returns the HLS output of a grant after re-checking visibility (model.ErrNotFound
	// otherwise). An output the transcoder lost (server restart) is rebuilt and starts at
	// segment; model.ErrTranscodeBusy when the budget stays exhausted.
	HLS(ctx context.Context, g model.PlaybackGrant, segment int) (model.HLSStream, error)
	// Stop ends a playback session of the browser session (model.ErrNotFound for any other).
	Stop(ctx context.Context, session model.Session, playbackID string) error
	// EndSession ends every playback session of a browser session (logout, profile switch).
	EndSession(ctx context.Context, sessionID int64) error
	// EndAccount ends every playback session of an account (guest revocation).
	EndAccount(ctx context.Context, accountID int64) error
	// EndProfile ends every playback session of a profile (profile deleted).
	EndProfile(ctx context.Context, profileID int64) error
	// RecheckVisibility ends the open direct-play streams whose title their profile may no longer
	// see; called after a change of ratings, ceilings or library rights.
	RecheckVisibility(ctx context.Context) error
	// Report stores a progress event and returns the merged progress.
	Report(ctx context.Context, profile model.Profile, itemID string, ev model.ProgressEvent) (model.Progress, error)
}
