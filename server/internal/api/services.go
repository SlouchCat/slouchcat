package api

import (
	"context"
	"io"
	"net/http"
	"time"

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
	// SetOfflinePIN stores the PBKDF2 verifier of the separate offline PIN (AN-19, 6 to 12 digits,
	// *model.ValidationError otherwise); RemoveOfflinePIN clears it. model.ErrNotFound for a profile
	// of another account.
	SetOfflinePIN(ctx context.Context, session model.Session, id int64, pin string) error
	RemoveOfflinePIN(ctx context.Context, session model.Session, id int64) error
	// UpdateAccount changes the e-mail or the password of the session's account after checking the
	// current password (M8, A-2) and returns the session with the changed account; a new password
	// revokes every other session of the account. Errors: model.ErrInvalidCredentials,
	// model.ErrEmailTaken, *model.ValidationError, model.ErrTooManyAttempts.
	UpdateAccount(ctx context.Context, session model.Session, upd model.AccountUpdate) (model.Session, error)
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
	// AddLibraryRoot adds a further folder to a library (idea 02, E8). Errors:
	// model.ErrNotFound, *model.ValidationError, model.ErrLibraryPathInvalid.
	AddLibraryRoot(ctx context.Context, libraryID int64, path string) (model.Library, error)
	// RemoveLibraryRoot removes a folder and soft-deletes the titles that live only there
	// (E12). Errors: model.ErrNotFound, *model.ValidationError, model.ErrConfirmMismatch,
	// model.ErrScanRunning.
	RemoveLibraryRoot(ctx context.Context, libraryID, rootID int64, confirmPath string) (model.Library, error)
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

	// M4 (design section 8): devices (W-7) and running streams (W-6).

	// ListDevices returns the devices with owner, capability profile and running streams.
	ListDevices(ctx context.Context) ([]model.AdminDevice, error)
	// DeleteDevice ends the device's streams, revokes its sessions and PIN counters and deletes it
	// (model.ErrNotFound for an unknown device).
	DeleteDevice(ctx context.Context, id string) error
	// ListStreams returns the running playback sessions with plain titles, newest first.
	ListStreams(ctx context.Context) ([]model.AdminStream, error)
	// StopStream ends a running playback session (model.ErrNotFound for an unknown or ended one).
	StopStream(ctx context.Context, id string) error

	// M6 (design 3.4): library settings and podcast subscriptions.

	// UpdateLibrary renames a library or sets the default rating of an audio library; titles rated
	// by the library follow (model.ErrNotFound, *model.ValidationError).
	UpdateLibrary(ctx context.Context, id int64, upd model.LibraryUpdate) (model.Library, error)
	// ListPodcasts returns every podcast subscription, newest first.
	ListPodcasts(ctx context.Context) ([]model.AdminPodcast, error)
	// CreatePodcast subscribes a podcast library to a feed after one fetch (*model.ValidationError,
	// model.ErrFeedExists, model.ErrFeedUnreachable, model.ErrFeedInvalid).
	CreatePodcast(ctx context.Context, req model.PodcastCreate) (model.AdminPodcast, error)
	// DeletePodcast soft-deletes a subscription with its episodes (model.ErrNotFound).
	DeletePodcast(ctx context.Context, id string) error
	// RefreshPodcast queues the fetch of a subscription now (model.ErrNotFound).
	RefreshPodcast(ctx context.Context, id string) (model.AdminPodcast, error)

	// Curation (M7). Picks (E-2): one global pick or one per profile of the main account;
	// *model.ValidationError for unknown items, seasons, extras, foreign profiles or a window that
	// ends before it starts; model.ErrNotFound for an unknown pick id.
	ListTopPicks(ctx context.Context) ([]model.AdminPick, error)
	CreateTopPicks(ctx context.Context, req model.PickRequest) ([]model.AdminPick, error)
	UpdateTopPick(ctx context.Context, id int64, upd model.PickUpdate) (model.AdminPick, error)
	DeleteTopPick(ctx context.Context, id int64) error
	// ReorderTopPicks stores the order; every existing pick exactly once (*model.ValidationError).
	ReorderTopPicks(ctx context.Context, ids []int64) error
	// SearchItems finds titles by name without any profile filter (admin dialogs).
	SearchItems(ctx context.Context, query string, limit int) ([]model.AdminItemHit, error)
	// Collections (E-6): TMDB collections are read-only (model.ErrNotEditable).
	ListCollections(ctx context.Context) ([]model.AdminCollection, error)
	GetCollection(ctx context.Context, id string) (model.AdminCollection, error)
	CreateCollection(ctx context.Context, name string) (model.AdminCollection, error)
	UpdateCollection(ctx context.Context, id, name string) (model.AdminCollection, error)
	DeleteCollection(ctx context.Context, id string) error
	SetCollectionItems(ctx context.Context, id string, itemIDs []string) (model.AdminCollection, error)
	// Start page rows (E-9): all eight keys exactly once (*model.ValidationError).
	HomeRows(ctx context.Context) ([]model.HomeRowSetting, error)
	SetHomeRows(ctx context.Context, rows []model.HomeRowSetting) ([]model.HomeRowSetting, error)
	// Trakt (E-7, I-7): the state of the daily lists and a refresh out of schedule
	// (model.ErrProviderNotConfigured without a client id).
	TraktStatus(ctx context.Context) (model.TraktStatus, error)
	// Spotlight (M11, ADR 0011): the configuration with the catalogue of list kinds, the fetched
	// entries with hits and overrides, one override, and a manual fetch.
	// SetSpotlightConfig: *model.ValidationError (ranges, media not offered by the kind, listUrl
	// invalid_value for a non-list or watchlist address). SetSpotlightEntryOverride:
	// model.ErrNotFound (neither entry nor override), model.ErrRankTaken, *model.ValidationError.
	// RefreshSpotlight: model.ErrProviderNotConfigured, model.ErrRefreshTooSoon.
	SpotlightConfig(ctx context.Context) (model.SpotlightConfig, error)
	SetSpotlightConfig(ctx context.Context, in model.SpotlightConfigUpdate) (model.SpotlightConfig, error)
	SpotlightEntries(ctx context.Context) (model.SpotlightEntries, error)
	SetSpotlightEntryOverride(ctx context.Context, media string, tmdbID int64, in model.SpotlightOverrideUpdate) (model.SpotlightEntry, error)
	RefreshSpotlight(ctx context.Context) error

	// M8 (design sections 1 and 3). Wizard checks (A-1): only while no account exists
	// (model.ErrSetupDone), counted against the attempt limit of model.ClientIP of ctx
	// (model.ErrTooManyAttempts); a bad path or key is described in the result, not an error.
	CheckSetupPath(ctx context.Context, path string) (model.PathCheck, error)
	CheckSetupTMDBKey(ctx context.Context, key string) (model.TMDBCheck, error)
	// The folder browser of the library form (idea 02, A-2). SetupRoots answers the wizard from
	// the list filled at start-up, with its own limit for model.ClientIP of ctx
	// (model.ErrTooManyAttempts) and only while no account exists (model.ErrSetupDone).
	// BrowsePaths: *model.ValidationError, model.ErrPathNotBrowsable, model.ErrPathUnreachable,
	// model.ErrBrowseBusy.
	SetupRoots(ctx context.Context) ([]model.BrowseRoot, error)
	BrowseRoots(ctx context.Context) ([]model.BrowseRoot, error)
	BrowsePaths(ctx context.Context, req model.BrowseRequest) (model.DirectoryListing, error)
	// SystemInfo returns version, transcode hardware, inotify limit and file systems; no secrets.
	SystemInfo(ctx context.Context) (model.SystemInfo, error)
	// Dashboard returns the admin overview (A-3).
	Dashboard(ctx context.Context) (model.Dashboard, error)
	// RetryJob queues a failed job again (model.ErrNotFound, model.ErrJobNotFailed); CancelJob
	// cancels a queued or running one (model.ErrNotFound for an unknown or ended job).
	RetryJob(ctx context.Context, id int64) (model.JobSummary, error)
	CancelJob(ctx context.Context, id int64) error
	// Settings and integrations (A-2). UpdateSettings checks everything before it stores anything:
	// *model.ValidationError, *model.EnvWinsError, model.ErrTMDBKeyInvalid, model.ErrTMDBUnreachable,
	// model.ErrTelegramTokenInvalid, model.ErrTelegramUnreachable.
	Settings(ctx context.Context) (model.AdminSettings, error)
	UpdateSettings(ctx context.Context, upd model.AdminSettingsUpdate) (model.AdminSettings, error)
	// SendTestEmail sends a test mail to to, or to the main account when empty
	// (model.ErrEmailNotConfigured, *model.SendError with model.ErrEmailSendFailed,
	// model.ErrTooManyAttempts for the client in model.ClientIP of ctx).
	SendTestEmail(ctx context.Context, to string) error
	// RotatePushKeys creates a new VAPID pair and deletes every push subscription.
	RotatePushKeys(ctx context.Context) (model.AdminSettings, error)
	// Backup and restore (A-4). Names are file names of ListBackups (model.ErrNotFound otherwise).
	// CreateBackup: model.ErrPassphraseRequired, model.ErrPassphraseTooShort. The restores stage the
	// archive and schedule the restart: model.ErrBackupInvalid, model.ErrPassphraseRequired,
	// model.ErrPassphraseWrong, model.ErrBackupNewerThanServer.
	ListBackups(ctx context.Context) ([]model.Backup, error)
	CreateBackup(ctx context.Context, req model.BackupRequest) (model.Backup, error)
	OpenBackup(ctx context.Context, name string) (model.StreamSource, error)
	DeleteBackup(ctx context.Context, name string) error
	RestoreBackupUpload(ctx context.Context, archive io.Reader, passphrase string) (model.RestoreResult, error)
	RestoreBackup(ctx context.Context, name, passphrase string) (model.RestoreResult, error)
	// Update check (A-5); CheckForUpdate: model.ErrUpdateCheckDisabled. UpdateAvailable is the
	// header notice of SessionInfo for the main account, answered from memory; nil = none.
	UpdateState(ctx context.Context) (model.UpdateState, error)
	CheckForUpdate(ctx context.Context) (model.UpdateState, error)
	UpdateAvailable(ctx context.Context) *model.UpdateNotice
	// ListAccounts returns the main account first, then the guests, newest first (A-2).
	ListAccounts(ctx context.Context) ([]model.AdminAccount, error)
	// Stats returns the statistics of the last days days: 7, 30, 90 or 365 (*model.ValidationError).
	Stats(ctx context.Context, days int) (model.Stats, error)
	// Announcements (A-7): *model.ValidationError, model.ErrNotFound, model.ErrAnnouncementPublished
	// (UpdateAnnouncement of a published one). Publishing sends it on the channels once.
	ListAnnouncements(ctx context.Context) ([]model.Announcement, error)
	CreateAnnouncement(ctx context.Context, creator model.Account, in model.AnnouncementInput) (model.Announcement, error)
	UpdateAnnouncement(ctx context.Context, id int64, title, body string) (model.Announcement, error)
	PublishAnnouncement(ctx context.Context, id int64) (model.Announcement, error)
	DeleteAnnouncement(ctx context.Context, id int64) error

	// Webhook integrations of Radarr and Sonarr (M9, I-8): *model.ValidationError and
	// model.ErrNotFound. CreateIntegration and RenewIntegrationToken return the integration with
	// its address, which exists only in that answer.
	ListIntegrations(ctx context.Context) ([]model.Integration, error)
	CreateIntegration(ctx context.Context, name, kind string) (model.Integration, error)
	RenewIntegrationToken(ctx context.Context, id int64) (model.Integration, error)
	SetIntegrationPathMap(ctx context.Context, id int64, pairs []model.PathMapping) (model.Integration, error)
	DeleteIntegration(ctx context.Context, id int64) error
	IntegrationEvents(ctx context.Context, id int64) ([]model.IntegrationEvent, error)
}

// CatalogService answers the start page and the detail page through the visibility layer
// (package discover).
type CatalogService interface {
	Home(ctx context.Context, profile model.Profile) (model.HomePage, error)
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
	// PodcastEpisodes returns one page of the visible episodes of a visible podcast, newest first,
	// and their total (M6); model.ErrNotFound for an invisible title or one that is no podcast.
	PodcastEpisodes(ctx context.Context, profile model.Profile, podcastID string, offset, limit int) ([]model.ItemSummary, int, error)

	// Discover (M7). Search over the FTS index through the visibility layer (E-4); genres of the
	// visible titles; a collection with one page of its visible members (model.ErrNotFound without
	// a visible member); the watchlist (E-5) and the favourites (E-8) of the profile. Add/Remove
	// answer model.ErrNotFound for an invisible title, model.ErrNotListable for seasons and extras,
	// model.ErrNotFavoritable for anything but series and podcasts.
	Search(ctx context.Context, profile model.Profile, q model.SearchQuery) (model.SearchResult, error)
	Genres(ctx context.Context, profile model.Profile) ([]model.GenreRef, error)
	Collection(ctx context.Context, profile model.Profile, id string, offset, limit int) (model.Collection, error)
	Watchlist(ctx context.Context, profile model.Profile, offset, limit int) ([]model.ItemSummary, int, error)
	AddToWatchlist(ctx context.Context, profile model.Profile, itemID string) error
	RemoveFromWatchlist(ctx context.Context, profile model.Profile, itemID string) error
	Favorites(ctx context.Context, profile model.Profile) ([]model.ItemSummary, error)
	AddFavorite(ctx context.Context, profile model.Profile, itemID string) error
	RemoveFavorite(ctx context.Context, profile model.Profile, itemID string) error
}

// NotifyService is the in-app inbox of a profile (package notify, M7 E-8, plan 5.5), the
// notification channels of an account and the announcements of a profile (M8). Inbox entries whose
// episode the profile may no longer see are omitted.
type NotifyService interface {
	// Notifications returns one page, newest first, with the total and the unread count.
	Notifications(ctx context.Context, profile model.Profile, offset, limit int) (list []model.Notification, total, unread int, err error)
	// MarkRead marks one entry of the profile (model.ErrNotFound for another profile's entry).
	MarkRead(ctx context.Context, profile model.Profile, id int64) error
	MarkAllRead(ctx context.Context, profile model.Profile) error

	// Channels of the session's account (M8, A-6, 5.5): email, push and telegram in this order.
	// Every method that changes them (all but Channels and PushPublicKey) answers
	// model.ErrPrimaryProfileRequired for a session whose active profile is not the primary one.
	// UpdateChannel: model.ErrNotFound (unknown channel), model.ErrChannelUnavailable (switching on a
	// channel the server lacks), model.ErrEventNotAllowed (errors on a guest account),
	// *model.ValidationError. TestChannel: model.ErrChannelUnavailable, model.ErrChannelDisabled,
	// *model.SendError with model.ErrSendFailed, model.ErrTooManyAttempts (attempt limit of the
	// client in model.ClientIP of ctx).
	Channels(ctx context.Context, session model.Session) ([]model.ChannelSetting, error)
	UpdateChannel(ctx context.Context, session model.Session, channel string, upd model.ChannelSettingUpdate) (model.ChannelSetting, error)
	TestChannel(ctx context.Context, session model.Session, channel string) error
	// CreateTelegramLink issues a new link code (model.ErrChannelUnavailable without a bot);
	// DeleteTelegramLink unlinks the chat and drops an open code.
	CreateTelegramLink(ctx context.Context, session model.Session) (model.ChannelSetting, error)
	DeleteTelegramLink(ctx context.Context, session model.Session) error
	// Web Push: the VAPID public key and the subscriptions of the session's account
	// (model.ErrPushUnavailable without a key pair; DeletePushSubscription: model.ErrNotFound).
	PushPublicKey(ctx context.Context) (string, error)
	CreatePushSubscription(ctx context.Context, session model.Session, in model.PushSubscriptionInput) (model.PushSubscription, error)
	DeletePushSubscription(ctx context.Context, session model.Session, id int64) error
	// Announcements of the profile (A-7): the published ones it has not acknowledged, newest first;
	// AckAnnouncement answers model.ErrNotFound for an unknown or unpublished one.
	Announcements(ctx context.Context, profile model.Profile) ([]model.ProfileAnnouncement, error)
	AckAnnouncement(ctx context.Context, profile model.Profile, id int64) error
}

// PlaybackService decides playback, issues and checks stream tokens and records progress
// (package playback).
type PlaybackService interface {
	// Start decides the playback mode, creates a playback session of the browser session and
	// returns the playback description with the token URL. Errors: model.ErrNotFound,
	// *model.ValidationError, model.ErrTranscodeBusy, *model.WatchLimitError (K-5),
	// *model.StreamLimitError (W-8).
	Start(ctx context.Context, session model.Session, itemID string, req model.PlaybackRequest) (model.PlaybackInfo, error)
	// Authorize resolves a stream token; model.ErrUnauthenticated when it is missing, forged,
	// revoked or expired.
	Authorize(ctx context.Context, token string) (model.PlaybackGrant, error)
	// ProxyEnclosure answers the stream of a direct-play grant without media file (a podcast episode,
	// M6) from the enclosure with Range pass-through after re-checking visibility; errors before the
	// response started: model.ErrNotFound, model.ErrEnclosureUnavailable.
	ProxyEnclosure(ctx context.Context, g model.PlaybackGrant, w http.ResponseWriter, r *http.Request) error
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
	// Report stores a progress event and returns the merged progress (with the watch time of a
	// profile with a daily limit; at the limit its playback sessions are revoked).
	Report(ctx context.Context, profile model.Profile, itemID string, ev model.ProgressEvent) (model.Progress, error)
	// Tracks returns the audio and subtitle tracks of a media file of a visible title (empty id:
	// the primary file), or model.ErrNotFound.
	Tracks(ctx context.Context, profile model.Profile, itemID, mediaFileID string) (model.TrackList, error)
	// Subtitle returns the WebVTT file of the text subtitle track a grant selected; any other
	// track, a bitmap track and an invisible title return model.ErrNotFound.
	Subtitle(ctx context.Context, g model.PlaybackGrant, trackID int64) (model.StreamSource, error)
	// WatchReport returns the minutes per day of a profile of the session's account (K-5); only
	// the primary profile (model.ErrPrimaryProfileRequired), model.ErrNotFound for another
	// account's profile, *model.ValidationError for days outside 1–90 (0 = 30).
	WatchReport(ctx context.Context, session model.Session, profileID int64, days int) (model.WatchReport, error)
	// ReportOffline stores the progress events of offline playback for profiles of the session's
	// account (O-5, AN-10) and returns one result per event in request order; events of other
	// accounts' profiles or of invisible titles are rejected, not errors. *model.ValidationError
	// for an empty or oversized batch or a missing sentAt.
	ReportOffline(ctx context.Context, session model.Session, sentAt time.Time, events []model.OfflineEvent) (model.OfflineEventResult, error)
}

// IntegrationService receives the webhooks of Radarr and Sonarr (package integrations, M9, I-8).
// The administration operations of the same service live in AdminService.
type IntegrationService interface {
	// Receive handles one webhook for the integration the token belongs to. Errors:
	// model.ErrWebhookToken (401), model.ErrWebhookRateLimited (429),
	// model.ErrWebhookPathOutside (422) and *model.ValidationError (422, an acting event without
	// a path). Every other event is accepted and reported as ignored.
	Receive(ctx context.Context, token string, p model.WebhookPayload) (model.WebhookReceipt, error)
}
