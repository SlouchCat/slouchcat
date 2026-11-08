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
	// AvailableMetadataLanguages are the metadata languages a profile may choose (ADR 0018).
	AvailableMetadataLanguages(ctx context.Context) ([]string, error)
	// DeleteProfile deletes a non-primary profile; end runs after the checks, before the delete.
	DeleteProfile(ctx context.Context, session model.Session, id int64, end func(ctx context.Context, profileID int64) error) error
	SetProfilePIN(ctx context.Context, session model.Session, id int64, pin string) error
	RemoveProfilePIN(ctx context.Context, session model.Session, id int64) error
	// SetOfflinePIN stores the PBKDF2 verifier of the separate offline PIN (AN-19, 6 to 12 digits,
	// *model.ValidationError otherwise); RemoveOfflinePIN clears it. model.ErrNotFound for a profile
	// of another account.
	SetOfflinePIN(ctx context.Context, session model.Session, id int64, pin string) error
	RemoveOfflinePIN(ctx context.Context, session model.Session, id int64) error
	// GetPlaybackPrefs and SetPlaybackPrefs read and replace the language and subtitle preferences
	// of the session's active profile (concept 13, F-13; *model.ValidationError on a bad value);
	// ResetSeriesTracks forgets the languages the profile chose per series.
	GetPlaybackPrefs(ctx context.Context, session model.Session) (model.PlaybackPrefs, error)
	SetPlaybackPrefs(ctx context.Context, session model.Session, prefs model.PlaybackPrefs) (model.PlaybackPrefs, error)
	ResetSeriesTracks(ctx context.Context, session model.Session) error
	// UpdateAccount changes the e-mail or the password of the session's account after checking the
	// current password (M8, A-2) and returns the session with the changed account; a new password
	// revokes every other session of the account. Errors: model.ErrInvalidCredentials,
	// model.ErrEmailTaken, *model.ValidationError, model.ErrTooManyAttempts.
	UpdateAccount(ctx context.Context, session model.Session, upd model.AccountUpdate) (model.Session, error)
}

// AccessService is the invitation and account management of the administration plus the public
// invitation links (package auth).
type AccessService interface {
	// GetInvitation is the preview of an open link (model.ErrNotFound, model.ErrInvitationGone,
	// model.ErrTooManyAttempts for model.ClientIP of ctx).
	GetInvitation(ctx context.Context, token string) (model.InvitationPreview, error)
	// AcceptInvitation creates the account and starts its session (plain token returned once).
	AcceptInvitation(ctx context.Context, in model.AcceptInvitation, userAgent string) (model.Session, string, error)
	CreateInvitation(ctx context.Context, creator model.Account, req model.InvitationRequest) (model.Invitation, error)
	ListInvitations(ctx context.Context) ([]model.Invitation, error)
	RevokeInvitation(ctx context.Context, id int64) error
	// UpdateAccountAccess changes ceiling, libraries and W-8 caps of an account that administers
	// nothing. Errors: model.ErrNotFound, model.ErrAdminLimitsFixed, *model.ValidationError.
	UpdateAccountAccess(ctx context.Context, id int64, in model.AccountAccessUpdate) error
	// DeleteAccount revokes and deletes an account; end runs after the checks, before the delete.
	// Errors: model.ErrNotFound, model.ErrOwnerProtected, model.ErrLastAdmin.
	DeleteAccount(ctx context.Context, accountID int64, end func(ctx context.Context, accountID int64) error) error
	// SetAccountRole appoints an administrator or takes the role away (ADR 0014). Errors:
	// model.ErrNotFound, model.ErrOwnerProtected, model.ErrLastAdmin, *model.ValidationError.
	// Who may call it is the visibility level owner, decided in the middleware.
	SetAccountRole(ctx context.Context, id int64, role string) error
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
	// AddLibraryRoots adds folders to a library in one transaction (idea 02, E8; F-13). Errors:
	// model.ErrNotFound, *model.ValidationError with the index of the folder that broke a rule.
	AddLibraryRoots(ctx context.Context, libraryID int64, roots []model.LibraryRootRequest) (model.Library, error)
	// UpdateLibraryRoot changes the content, the provider list (ADR 0015) or the path (ADR 0023)
	// of a folder. Errors:
	// model.ErrNotFound, *model.ValidationError, model.ErrConfirmMismatch, model.ErrScanRunning.
	UpdateLibraryRoot(ctx context.Context, libraryID, rootID int64, upd model.LibraryRootUpdate) (model.Library, error)
	// ListMetadataProviders describes the four providers (ADR 0015).
	ListMetadataProviders(ctx context.Context) ([]model.MetadataProviderInfo, error)
	// Metadata settings of concept 10 (ADR 0018): server settings, a library's deviation, apply.
	MetadataSettings(ctx context.Context) (model.MetadataSettings, error)
	UpdateMetadataSettings(ctx context.Context, m model.MetadataSettings) (model.MetadataSettings, error)
	LibraryMetadata(ctx context.Context, libraryID int64) (model.LibraryMetadata, error)
	UpdateLibraryMetadata(ctx context.Context, libraryID int64, textLangs *[]string, images *model.LibraryImages) (model.LibraryMetadata, error)
	ApplyMetadata(ctx context.Context, req model.ApplyMetadataRequest) (model.ApplyMetadataResult, error)
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
	RefreshItem(ctx context.Context, itemID string, unlockAll bool) (model.AdminItem, error)
	// CreateItemTrailer stores a manual trailer from a YouTube id or URL.
	CreateItemTrailer(ctx context.Context, itemID, lang, youtubeID, url string) (model.Trailer, error)
	// DeleteItemTrailer removes a trailer of a title.
	DeleteItemTrailer(ctx context.Context, itemID string, trailerID int64) error
	// The image choice per title (concept 10 6.4, 7.3).
	ItemImageCandidates(ctx context.Context, itemID, kind string, refresh bool) (model.ImageCandidateList, error)
	SetItemImage(ctx context.Context, itemID, kind, provider, source string) (model.AdminItem, error)
	ClearItemImage(ctx context.Context, itemID, kind string) (model.AdminItem, error)
	// UploadItemImage stores an uploaded image for a kind and locks the kind (concept 15 7.5;
	// model.ErrNotFound, *model.ValidationError).
	UploadItemImage(ctx context.Context, itemID, kind string, data []byte) (model.AdminItem, error)
	SetItemOriginalLanguage(ctx context.Context, itemID string, lang *string) (model.AdminItem, error)
	// UpdateItemMetadata writes, locks and unlocks the editor's fields of a title in the server's
	// metadata language (concept 15 7.5): model.ErrNotFound, *model.ValidationError.
	UpdateItemMetadata(ctx context.Context, itemID string, edit model.ItemMetadataEdit) (model.AdminItem, error)
	// ListAdminGenres returns the server's metadata language and every genre of the live films and
	// series in it (concept 15 7.5).
	ListAdminGenres(ctx context.Context) (string, []string, error)
	ItemImageCandidatePreview(ctx context.Context, itemID, source string) (model.StreamSource, error)
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
	// ListRelinkPairs returns the soft-deleted titles with a live copy and their user data
	// (ADR 0023); RelinkItems merges the copies into them (*model.ValidationError for a malformed
	// request, *model.ScanRunningError when a scan stopped the run before any merge) and returns
	// the merged pairs, the skipped count and why the run stopped early.
	ListRelinkPairs(ctx context.Context, libraryID *int64) (model.RelinkPairList, error)
	RelinkItems(ctx context.Context, pairs []model.RelinkPairRef) (model.RelinkOutcome, error)

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
	// ForgetDashboardCounts drops the kept counts of the admin sidebar after a change on the
	// accounts page (concept 11); the next dashboard recomputes them.
	ForgetDashboardCounts()
	// RetryJob queues a failed job again (model.ErrNotFound, model.ErrJobNotFailed); CancelJob
	// cancels a queued or running one (model.ErrNotFound for an unknown or ended job).
	RetryJob(ctx context.Context, id int64) (model.JobSummary, error)
	CancelJob(ctx context.Context, id int64) error
	// The page "Hintergrundjobs" (concept 09): model.ErrNotFound for an unknown type,
	// model.ErrJobTypeNotRunnable, model.ErrJobTypePaused, model.ErrJobTypeNotScheduled,
	// *model.ValidationError, and the services' own refusals of a manual run
	// (model.ErrProviderNotConfigured, model.ErrRefreshTooSoon, model.ErrUpdateCheckDisabled).
	ListJobTypes(ctx context.Context) (model.JobOverview, error)
	ListJobRuns(ctx context.Context, typ, state string, limit int) (model.JobRunPage, error)
	RunJobType(ctx context.Context, typ string) ([]model.JobRun, error)
	PauseJobType(ctx context.Context, typ string) (model.JobTypeInfo, error)
	ResumeJobType(ctx context.Context, typ string) (model.JobTypeInfo, error)
	UpdateJobSchedule(ctx context.Context, typ string, upd model.JobScheduleUpdate) (model.JobTypeInfo, error)
	// UpdateJobTypeWindow: model.ErrNotFound, model.ErrJobTypeNotWindowable, *model.ValidationError
	// (concept 15, 4.3).
	UpdateJobTypeWindow(ctx context.Context, typ string, upd model.JobTypeWindowUpdate) (model.JobTypeInfo, error)
	CancelJobRuns(ctx context.Context, typ string, all bool) (int, error)
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
	// ItemImage opens a stored image of a title system-wide, without a profile check
	// (getAdminItemImage, M11 step 5b): WebP when preferWebP and stored, else JPEG.
	// model.ErrNotFound for an unknown or deleted title, a missing image or a row without file.
	ItemImage(ctx context.Context, itemID, kind, size string, preferWebP bool) (model.StreamSource, error)
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
	// RestartServer schedules a restart of the server (concept 11) and records it for the account:
	// model.ErrBackupBusy while a backup or restore runs, model.ErrRestartPending while a restart
	// or a restore is scheduled. The restores answer model.ErrRestartPending as well.
	RestartServer(ctx context.Context, accountID int64) (model.RestartAccepted, error)
	// Update check (A-5); CheckForUpdate: model.ErrUpdateCheckDisabled. UpdateAvailable is the
	// header notice of SessionInfo for the main account, answered from memory; nil = none.
	UpdateState(ctx context.Context) (model.UpdateState, error)
	CheckForUpdate(ctx context.Context) (model.UpdateState, error)
	UpdateAvailable(ctx context.Context) *model.UpdateNotice
	// InstallUpdate asks the updater companion to install the newest release (ADR 0024):
	// model.ErrUpdaterUnavailable, model.ErrUpdateRunning, *model.UpdateRefusedError.
	// InstallStatus reads its last install from state.json (idle without one).
	InstallUpdate(ctx context.Context) (model.UpdateInstall, error)
	InstallStatus(ctx context.Context) (model.UpdateInstall, error)
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
	// Home returns the start page; start repeats an earlier draw of the stage's first title when it
	// lies in the stage row (concept 08, F-2), nil or outside draws again.
	Home(ctx context.Context, profile model.Profile, start *int) (model.HomePage, error)
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

	// Library views (concept 08, ADR 0016). VisibleLibraries lists the libraries with a visible
	// top-level title; LibraryItems answers one page of a library with its facets
	// (model.ErrNotFound for a library without a visible title, *model.ValidationError for an
	// invalid query); PicksPage pages all of Cat's recommendations with their total.
	VisibleLibraries(ctx context.Context, profile model.Profile) ([]model.LibraryNav, error)
	LibraryItems(ctx context.Context, profile model.Profile, libraryID int64, q model.LibraryQuery) (model.LibraryPage, error)
	PicksPage(ctx context.Context, profile model.Profile, offset, limit int) ([]model.ItemSummary, int, error)
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
	// ChapterImage opens the image of chapter idx of a media file of a visible title (concept 13
	// stage 3); model.ErrNotFound for anything that is not visible, not a ULID of a file of the
	// title, or has no image.
	// version is the v of the image URL (library.ChapterImagesVersion); the ETag of the result
	// names the version the image belongs to.
	ChapterImage(ctx context.Context, profile model.Profile, itemID, mediaFileID string, idx int, version string) (model.StreamSource, error)
	// State reports from memory whether a playback session of the browser session still runs
	// (concept 13); model.ErrNotFound for any other and for one the server does not know.
	State(session model.Session, playbackID string) (model.PlaybackState, error)
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

// RadioService is the webradio of concept 16 stage 1 (package radio, contract 1.17.0, ADR 0025).
// Every method that takes the session acts for its active profile. A station the profile may not
// reach (hidden, or of a country the setting excludes) answers model.ErrNotFound, never a 403.
// Methods that read the catalog answer model.ErrRadioCatalogLoading before the first sync.
type RadioService interface {
	// ListStations and Facets read the in-memory index: never hidden or excluded stations, broken
	// ones only while the setting hide_broken is off. *model.ValidationError for a bad value.
	ListStations(ctx context.Context, session model.Session, q model.RadioQuery) (model.RadioStationPage, error)
	Facets(ctx context.Context, session model.Session, q model.RadioQuery) (model.RadioFacets, error)
	// Station returns one station, broken and missing ones included.
	Station(ctx context.Context, session model.Session, stationID int64) (model.RadioStation, error)
	// Favorites returns the profile's favourites in its order, without hidden and excluded ones.
	Favorites(ctx context.Context, session model.Session) ([]model.RadioStation, error)
	// SetFavoriteOrder stores the complete new order; another set of ids is a
	// *model.ValidationError on field stationIds.
	SetFavoriteOrder(ctx context.Context, session model.Session, stationIDs []int64) error
	AddFavorite(ctx context.Context, session model.Session, stationID int64) error
	RemoveFavorite(ctx context.Context, session model.Session, stationID int64) error
	// Listen starts or joins the station's relay and opens a listen session. Errors:
	// model.ErrNotFound, *model.RadioCapacityError, model.ErrRadioUnreachable,
	// model.ErrRadioNotAudio, *model.WatchLimitError.
	Listen(ctx context.Context, session model.Session, stationID int64) (model.RadioListen, error)
	// Heartbeat keeps a listen session of the profile open. Errors: model.ErrNotFound (unknown or
	// another profile's session), *model.WatchLimitError (the session ended at the limit),
	// model.ErrRadioUnreachable (the relay ended; the handler answers 410).
	Heartbeat(ctx context.Context, session model.Session, listenID string, playing bool) (model.RadioHeartbeat, error)
	// EndListen ends a listen session of the profile (idempotent; model.ErrNotFound for an unknown
	// or another profile's session).
	EndListen(ctx context.Context, session model.Session, listenID string) error
	// Now returns the title timeline of the station's running relay after since (zero = all).
	Now(ctx context.Context, session model.Session, stationID int64, since time.Time) (model.RadioNow, error)
	Preferences(ctx context.Context, session model.Session) (model.RadioPreferences, error)
	// SetPreferences replaces the preferences (*model.ValidationError for a bad value).
	SetPreferences(ctx context.Context, session model.Session, p model.RadioPreferences) (model.RadioPreferences, error)

	// AuthorizeLive resolves the listen token of a live request; a forged token, a token of
	// another listen session, an ended or expired session answer model.ErrUnauthenticated. Every
	// call slides the session's expiry.
	AuthorizeLive(ctx context.Context, listenID, token string) (model.RadioGrant, error)
	// Playlist returns the live playlist; skip is the _HLS_skip query value ("" = full list).
	Playlist(ctx context.Context, g model.RadioGrant, skip string) ([]byte, error)
	// Init returns the fMP4 initialisation segment of a relay generation (model.ErrNotFound
	// outside the window).
	Init(ctx context.Context, g model.RadioGrant, gen int) ([]byte, error)
	// Segment returns one fMP4 segment (model.ErrNotFound outside the window) and a channel that
	// closes when the listen session is revoked, so the handler cuts a running write.
	Segment(ctx context.Context, g model.RadioGrant, n int64) ([]byte, <-chan struct{}, error)
	// Image opens a station image (kind logo; size w96, w256 or w512) as WebP or PNG. admin
	// false applies the viewer rules (hidden or excluded → model.ErrNotFound); no logo is
	// model.ErrNotFound as well.
	Image(ctx context.Context, stationID int64, kind, size string, webp, admin bool) (model.StreamSource, error)

	// Administration (x-visibility admin).
	Config(ctx context.Context) (model.RadioConfig, error)
	// UpdateConfig changes the settings (*model.ValidationError for a value out of range).
	UpdateConfig(ctx context.Context, upd model.RadioConfigUpdate) (model.RadioConfig, error)
	CatalogState(ctx context.Context) (model.RadioCatalogState, error)
	// SyncCatalog queues the catalog sync with manual priority (or names the queued or running
	// one) and returns its job id.
	SyncCatalog(ctx context.Context) (int64, error)
	AdminStations(ctx context.Context, q model.AdminRadioQuery) (model.AdminRadioStationPage, error)
	AdminStation(ctx context.Context, stationID int64) (model.AdminRadioStation, error)
	// UpdateStation applies a patch; a set field locks itself. Errors: model.ErrNotFound,
	// *model.ValidationError.
	UpdateStation(ctx context.Context, stationID int64, patch model.AdminRadioStationPatch) (model.AdminRadioStation, error)
	// Probe checks a stream of the station (streamID 0 = the best ranked) within 10 s. Errors:
	// model.ErrNotFound, model.ErrRadioUnreachable, model.ErrRadioNotAudio.
	Probe(ctx context.Context, stationID, streamID int64) (model.RadioProbe, error)
	// SetLogo stores an uploaded logo (at most model.MaxImageUploadBytes, already limited by the
	// handler) and locks the field logo. Errors: model.ErrNotFound, *model.ValidationError on
	// field file (unsupported_format, too_many_pixels).
	SetLogo(ctx context.Context, stationID int64, r io.Reader) (model.AdminRadioStation, error)
	// DeleteLogo removes an uploaded logo and lifts the lock, so the logo chain runs again.
	DeleteLogo(ctx context.Context, stationID int64) (model.AdminRadioStation, error)
	Relays(ctx context.Context) (model.RadioRelays, error)
	// StopListener ends a listen session (ended_by admin); model.ErrNotFound for an unknown or
	// ended one.
	StopListener(ctx context.Context, listenID string) error

	// Session hooks: the login session ended (by model.StoppedByLogout or model.StoppedByRevoked
	// for a profile switch), a profile's sessions ended (by model.StoppedBy*), an account was
	// revoked. Each ends the listen sessions concerned; their next live request answers 401.
	EndSession(ctx context.Context, sessionID int64, by string) error
	EndProfile(ctx context.Context, profileID int64, by string) error
	EndAccount(ctx context.Context, accountID int64) error
}
