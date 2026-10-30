// Package admin implements the first-start wizard and the library administration of the main
// account (plan 3.1 admin, M1a "minimaler Assistent"). It validates the whole wizard input before
// it creates anything, checks the TMDB key against TMDB and stores it encrypted through package
// secrets. Library work itself (path checks, file system detection, scans) is delegated to package
// library through the Libraries interface.
package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

const (
	// SettingTMDBKey is the settings key of the encrypted TMDB key.
	SettingTMDBKey = "tmdb_api_key"
	// EnvTMDBKey overrides the stored TMDB key (plan 3.8 point 3).
	EnvTMDBKey = "SLOUCHCAT_TMDB_API_KEY"

	// Optional provider keys of M3 (design 3.1): the environment wins; the encrypted settings are
	// only read when present (the settings page that stores them follows in M8).
	SettingTVDBKey    = "tvdb_api_key"
	SettingTVDBPin    = "tvdb_pin"
	SettingYouTubeKey = "youtube_api_key"
	EnvTVDBKey        = "SLOUCHCAT_TVDB_API_KEY"
	EnvTVDBPin        = "SLOUCHCAT_TVDB_PIN"
	EnvYouTubeKey     = "SLOUCHCAT_YOUTUBE_API_KEY"

	maxLibraryName = 80
	maxLibraryPath = 4096
	maxTMDBKeyLen  = 1024
)

// Enqueuer is the job queue (package jobs).
type Enqueuer interface {
	Enqueue(ctx context.Context, j jobs.NewJob) (int64, error)
}

// jobMetadataMatch is the job of package metadata that enriches one item; payload {"itemId": …}.
const jobMetadataMatch = "metadata.match"

// jobTraktRefresh is the schedule type of package metadata that fetches the Trakt lists; its
// interval is spotlight_config.interval_hours (one value, two doors, concept 09 4.3).
const jobTraktRefresh = "trakt.refresh"

// Schedules is the scheduler of package jobs: the settings pages write their interval and switch
// into the schedule row as well (SetSpotlightConfig, the update check of UpdateSettings).
type Schedules interface {
	UpdateSchedule(ctx context.Context, typ string, u jobs.ScheduleUpdate) (jobs.Schedule, error)
}

// Libraries is implemented by package library.
type Libraries interface {
	ValidatePath(ctx context.Context, path string) error // model.ErrLibraryPathInvalid when not absolute, missing, not a directory or not readable
	// ValidateNewRoot adds the rule of E14 to ValidatePath: a folder that is about to become a
	// library root must lie below a browsable root (library.ErrNotBrowsable, which wraps
	// model.ErrLibraryPathInvalid) unless it is a root already.
	ValidateNewRoot(ctx context.Context, path string) error
	Create(ctx context.Context, req model.LibraryRequest) (model.Library, error) // validates, detects the file system, inserts library + root, queues the first scan
	List(ctx context.Context) ([]model.Library, error)
	Get(ctx context.Context, id int64) (model.Library, error) // model.ErrNotFound
	QueueScan(ctx context.Context, id int64) (model.Library, error)
	AcceptLoss(ctx context.Context, id int64) (model.Library, error)
	// ValidateRoots applies the folder rules of a request without writing anything (the wizard
	// checks the first library before it creates the account); *model.ValidationError with the
	// index of the folder that broke a rule, field names prefixed with field.
	ValidateRoots(ctx context.Context, kind string, roots []model.LibraryRootRequest, field string) error
	// AddRoots adds further folders to a library in one transaction (idea 02, E8; F-13);
	// model.ErrNotFound, *model.ValidationError with the index of the folder that broke a rule.
	AddRoots(ctx context.Context, libraryID int64, roots []model.LibraryRootRequest) (model.Library, error)
	// UpdateRoot changes the content or the provider list of a folder (ADR 0015); a content
	// change repeats the path like a removal. model.ErrNotFound, *model.ValidationError,
	// model.ErrConfirmMismatch, model.ErrScanRunning.
	UpdateRoot(ctx context.Context, libraryID, rootID int64, upd model.LibraryRootUpdate) (model.Library, error)
	// RemoveRoot removes a folder and soft-deletes the titles that live only there (E12).
	// confirmPath must repeat the stored path; model.ErrNotFound, model.ErrConfirmMismatch,
	// model.ErrScanRunning.
	RemoveRoot(ctx context.Context, libraryID, rootID int64, confirmPath string) (model.Library, error)
}

// LibraryMaintenance is implemented by package library (M3): cancelling a scan and removing
// files from the library (B-9 cleanup).
type LibraryMaintenance interface {
	// CancelScan cancels the queued or running scan job; model.ErrNotFound for an unknown library.
	CancelScan(ctx context.Context, libraryID int64) (model.Library, error)
	// IgnoreFiles removes the media files, records them in ignored_files with the reason and
	// soft-deletes titles left without a file; unknown ids are skipped.
	IgnoreFiles(ctx context.Context, mediaFileIDs []string, reason string) (files, itemsDeleted int, err error)
}

// Metadata is implemented by package metadata (M3): provider search, manual match, refresh and
// trailers.
type Metadata interface {
	// Search asks a provider; errors *model.ValidationError, model.ErrProviderNotConfigured,
	// model.ErrTMDBKeyInvalid, model.ErrTMDBUnreachable.
	Search(ctx context.Context, provider, kind, query string, year *int) ([]model.MetadataCandidate, error)
	// AssignMatch stores the provider id, locks the provider ids and queues the match
	// (model.ErrNotFound, *model.ValidationError).
	AssignMatch(ctx context.Context, itemID, provider, providerID string) error
	// Refresh queues the match and the probes of the title (model.ErrNotFound).
	Refresh(ctx context.Context, itemID string) error
	// AddManualTrailer stores a manual trailer (model.ErrNotFound, *model.ValidationError).
	AddManualTrailer(ctx context.Context, itemID, lang, youtubeID string) (model.Trailer, error)
	// DeleteTrailer removes a trailer of the title (model.ErrNotFound).
	DeleteTrailer(ctx context.Context, itemID string, trailerID int64) error
	// ListTrailersSystem lists every trailer of a title, unfiltered.
	ListTrailersSystem(ctx context.Context, itemID string) ([]model.Trailer, error)
}

// ErrNotConfigured: a maintenance operation needs a collaborator that was not wired
// (WithLibraryMaintenance, WithMetadata); the API answers 500.
var ErrNotConfigured = errors.New("admin: collaborator not configured")

// TMDBChecker is implemented by package metadata.
type TMDBChecker interface {
	CheckKey(ctx context.Context, key string) error // nil, model.ErrTMDBKeyInvalid or model.ErrTMDBUnreachable
}

// Service implements api.AdminService and is the KeySource of package metadata.
type Service struct {
	db        *sql.DB // writer, for the rating override transaction
	write     *appq.Queries
	read      *appq.Queries
	sys       *systemq.Queries // unfiltered reads of the administration views (reader pool)
	auth      *auth.Service
	vis       *visibility.Reader // system-scope read of the rating override
	queue     Enqueuer           // metadata refresh after a cleared override; nil = none
	schedules Schedules          // nil until WithSchedules
	libraries Libraries
	maint     LibraryMaintenance // nil until WithLibraryMaintenance
	meta      Metadata           // nil until WithMetadata
	streams   Streams            // nil until WithStreams
	podcasts  Podcasts           // nil until WithPodcasts
	trakt     Trakt              // nil until WithTrakt
	ops       opsState           // M8: system state, jobs, errors, notification settings, update check (dashboard.go)
	backups   backupState        // directory and version from WithBackups, restart from WithRestart
	keyring   *secrets.Keyring
	tmdb      TMDBChecker
	getenv    func(string) string
	log       *slog.Logger
	now       func() time.Time
	anilist   bool // AniList switched on (SLOUCHCAT_ANILIST, WithAniList); true until set

	browse        browseState   // zero until WithBrowse (idea 02)
	announcements Announcements // nil until WithAnnouncements (M8)
	webhooks      Integrations  // nil until WithIntegrations (M9)
}

// New returns the service. getenv is normally os.Getenv.
func New(db *store.DB, auth *auth.Service, libraries Libraries, keyring *secrets.Keyring, tmdb TMDBChecker,
	getenv func(string) string, log *slog.Logger,
) *Service {
	return &Service{
		db: db.Write, write: appq.New(db.Write), read: appq.New(db.Read), sys: systemq.New(db.Read), auth: auth, vis: visibility.New(db, log),
		libraries: libraries,
		keyring:   keyring, tmdb: tmdb, getenv: getenv, log: log, now: time.Now, anilist: true,
	}
}

// WithAniList tells the service whether the AniList filler is switched on (config.AniList), for
// listMetadataProviders, and returns the receiver.
func (s *Service) WithAniList(on bool) *Service {
	s.anilist = on
	return s
}

// WithQueue sets the job queue used to refresh the metadata of a title whose rating override was
// cleared, and returns the receiver.
func (s *Service) WithQueue(q Enqueuer) *Service {
	s.queue = q
	return s
}

// WithSchedules sets the job scheduler the settings pages write through and returns the receiver.
func (s *Service) WithSchedules(sched Schedules) *Service {
	s.schedules = sched
	return s
}

// syncSchedule writes a changed setting into the schedule row of typ (concept 09, "one value,
// two doors"). A missing row (the server starts, the scheduler has not run yet) is no error.
func (s *Service) syncSchedule(ctx context.Context, typ string, u jobs.ScheduleUpdate) {
	if s.schedules == nil {
		return
	}
	if _, err := s.schedules.UpdateSchedule(ctx, typ, u); err != nil && !errors.Is(err, model.ErrNotFound) {
		s.log.WarnContext(ctx, "sync schedule with the setting", "job_type", typ, "err", err)
	}
}

// WithLibraryMaintenance sets the scan cancellation and file removal of package library, and
// returns the receiver.
func (s *Service) WithLibraryMaintenance(m LibraryMaintenance) *Service {
	s.maint = m
	return s
}

// WithMetadata sets the provider search, manual match, refresh and trailers of package metadata,
// and returns the receiver.
func (s *Service) WithMetadata(m Metadata) *Service {
	s.meta = m
	return s
}

// RunSetup runs the first-start wizard. Order: refuse when an account exists; validate every
// field, the library path and the TMDB key without side effects; create the owner (which re-checks
// "no account" inside its transaction, so a concurrent second wizard gets model.ErrSetupDone
// before it changed anything); store the key encrypted; create the library (queues the first
// scan); start the session. Every call counts against the client's attempt limit. Password and
// key never appear in logs or errors.
func (s *Service) RunSetup(ctx context.Context, req model.SetupRequest, userAgent string) (model.Session, string, error) {
	if err := s.auth.AllowAttempt(ctx); err != nil {
		return model.Session{}, "", err
	}
	required, err := s.auth.SetupRequired(ctx)
	if err != nil {
		return model.Session{}, "", err
	}
	if !required {
		return model.Session{}, "", model.ErrSetupDone
	}

	_, verr := auth.ValidateOwner(req.Email, req.Password)
	if req.Language != "" && req.Language != "de" && req.Language != "en" {
		verr = verr.Add("language", "invalid_value")
	}
	verr = validateLibrary(verr, "library.", req.Library)
	key := strings.TrimSpace(req.TMDBKey)
	if len(key) > maxTMDBKeyLen {
		verr = verr.Add("tmdbApiKey", "too_long")
	}
	if verr != nil {
		return model.Session{}, "", verr
	}
	if err := s.libraries.ValidateRoots(ctx, req.Library.Kind, req.Library.Roots, "library.roots"); err != nil {
		return model.Session{}, "", err
	}
	if key != "" {
		if err := s.tmdb.CheckKey(ctx, key); err != nil {
			return model.Session{}, "", err
		}
	}

	account, profile, err := s.auth.CreateOwner(ctx, req.Email, req.Password, req.Language)
	if err != nil {
		return model.Session{}, "", err
	}
	if key != "" {
		if err := s.storeTMDBKey(ctx, key); err != nil {
			return model.Session{}, "", err
		}
	}
	lib, err := s.libraries.Create(ctx, req.Library)
	if err != nil {
		// The account exists now; the owner can add the library on the libraries page.
		s.log.ErrorContext(ctx, "setup: create first library", "account_id", account.ID, "err", err)
		return model.Session{}, "", fmt.Errorf("create first library: %w", err)
	}
	session, token, err := s.auth.StartSession(ctx, account, profile, userAgent)
	if err != nil {
		return model.Session{}, "", err
	}
	s.log.InfoContext(ctx, "setup completed", "account_id", account.ID, "library_id", lib.ID, "tmdb_key_stored", key != "")
	return session, token, nil
}

// validateLibrary checks name, kind and the shape of the folder list; prefix names the fields
// ("library."). The folder rules themselves (family, provider list, path, overlap) are applied
// by package library, which answers with the index of the folder that broke one.
func validateLibrary(verr *model.ValidationError, prefix string, req model.LibraryRequest) *model.ValidationError {
	name := strings.TrimSpace(req.Name)
	switch {
	case name == "":
		verr = verr.Add(prefix+"name", "required")
	case utf8.RuneCountInString(name) > maxLibraryName:
		verr = verr.Add(prefix+"name", "too_long")
	}
	switch {
	case req.Kind == "":
		verr = verr.Add(prefix+"kind", "required")
	case !model.ValidLibraryKind(req.Kind):
		verr = verr.Add(prefix+"kind", "invalid_value")
	}
	if req.Kind != "" && req.Kind != model.LibraryPodcasts && len(req.Roots) == 0 {
		verr = verr.Add(prefix+"roots", model.CodeRootsRequired)
	}
	verr = validateRootShapes(verr, prefix+"roots", req.Roots)
	if req.DefaultRating != nil {
		switch {
		case req.Kind != "" && !model.AudioLibraryKind(req.Kind):
			verr = verr.Add(prefix+"defaultRating", "not_allowed")
		case !validRating(req.DefaultRating):
			verr = verr.Add(prefix+"defaultRating", "invalid_value")
		}
	}
	return verr
}

// validateRootShapes checks what a folder request must carry before the rules run: a path of
// sane length, a content and a provider list. field names the list ("roots", "library.roots").
func validateRootShapes(verr *model.ValidationError, field string, roots []model.LibraryRootRequest) *model.ValidationError {
	for i, r := range roots {
		name := fmt.Sprintf("%s[%d].", field, i)
		switch {
		case strings.TrimSpace(r.Path) == "":
			verr = verr.Add(name+"path", "required")
		case len(r.Path) > maxLibraryPath:
			verr = verr.Add(name+"path", "too_long")
		}
		switch {
		case r.Content == "":
			verr = verr.Add(name+"content", "required")
		case !model.ValidRootContent(r.Content):
			verr = verr.Add(name+"content", "invalid_value")
		}
		if len(r.Providers) == 0 {
			verr = verr.Add(name+"providers", "required")
		}
	}
	return verr
}

func (s *Service) storeTMDBKey(ctx context.Context, key string) error {
	enc, err := s.keyring.Encrypt(key)
	if err != nil {
		return fmt.Errorf("encrypt tmdb key: %w", err)
	}
	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingTMDBKey, Value: enc, Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("store tmdb key: %w", err)
	}
	return nil
}

// TMDBKey returns the TMDB key: the environment variable wins, otherwise the decrypted setting;
// "" when none is configured.
func (s *Service) TMDBKey(ctx context.Context) (string, error) {
	return s.secretKey(ctx, EnvTMDBKey, SettingTMDBKey)
}

// TVDBKey returns the TVDB key and the optional subscriber PIN from one source: both from the
// environment when the key is set there, else both from their encrypted settings, so a PIN never
// pairs with the key of the other source; "" when not configured.
func (s *Service) TVDBKey(ctx context.Context) (key, pin string, err error) {
	if key := strings.TrimSpace(s.getenv(EnvTVDBKey)); key != "" {
		return key, strings.TrimSpace(s.getenv(EnvTVDBPin)), nil
	}
	if key, err = s.secretKey(ctx, "", SettingTVDBKey); err != nil {
		return "", "", err
	}
	if pin, err = s.secretKey(ctx, "", SettingTVDBPin); err != nil {
		return "", "", err
	}
	return key, pin, nil
}

// YouTubeKey returns the YouTube Data API key from the environment or else from its encrypted
// setting; "" when not configured.
func (s *Service) YouTubeKey(ctx context.Context) (string, error) {
	return s.secretKey(ctx, EnvYouTubeKey, SettingYouTubeKey)
}

// secretKey reads a key: the environment variable wins (env "" reads only the setting), otherwise
// the decrypted setting; "" when neither is set. Values never reach the log or an error.
func (s *Service) secretKey(ctx context.Context, env, setting string) (string, error) {
	if key := strings.TrimSpace(s.getenv(env)); env != "" && key != "" {
		return key, nil
	}
	enc, err := s.read.GetSetting(ctx, setting)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read setting %s: %w", setting, err)
	}
	key, err := s.keyring.Decrypt(enc)
	if err != nil {
		return "", fmt.Errorf("setting %s: %w", setting, err)
	}
	return key, nil
}

// ListLibraries returns all libraries with their scan state.
func (s *Service) ListLibraries(ctx context.Context) ([]model.Library, error) {
	return s.libraries.List(ctx)
}

// GetLibrary returns one library or model.ErrNotFound.
func (s *Service) GetLibrary(ctx context.Context, id int64) (model.Library, error) {
	return s.libraries.Get(ctx, id)
}

// CreateLibrary validates the request and creates the library, which queues its first scan.
func (s *Service) CreateLibrary(ctx context.Context, req model.LibraryRequest) (model.Library, error) {
	if verr := validateLibrary(nil, "", req); verr != nil {
		return model.Library{}, verr
	}
	return s.libraries.Create(ctx, req)
}

// ScanLibrary queues a full scan. A podcast library fetches every subscription of the library
// (M6) and, when it has folders, scans them as well (ADR 0015).
func (s *Service) ScanLibrary(ctx context.Context, id int64) (model.Library, error) {
	if lib, err := s.read.GetLibrary(ctx, id); err == nil && lib.Kind == model.LibraryPodcasts {
		if s.podcasts == nil {
			return model.Library{}, ErrNotConfigured
		}
		if _, err := s.podcasts.RefreshLibrary(ctx, id); err != nil {
			return model.Library{}, err
		}
		if roots, err := s.read.ListLibraryRoots(ctx, id); err == nil && len(roots) > 0 {
			return s.libraries.QueueScan(ctx, id)
		}
		return s.libraries.Get(ctx, id)
	}
	return s.libraries.QueueScan(ctx, id)
}

// AcceptLibraryLoss lifts the loss protection for the next scan and queues it.
func (s *Service) AcceptLibraryLoss(ctx context.Context, id int64) (model.Library, error) {
	return s.libraries.AcceptLoss(ctx, id)
}

// AddLibraryRoots adds further folders to a library in one go (idea 02, E8; F-13). The folder
// rules, the overlap check and the browsable-root rule live in package library; this only refuses
// what is not a folder request at all.
func (s *Service) AddLibraryRoots(ctx context.Context, libraryID int64, roots []model.LibraryRootRequest) (model.Library, error) {
	var verr *model.ValidationError
	if len(roots) == 0 {
		verr = verr.Add("roots", "required")
	}
	if verr = validateRootShapes(verr, "roots", roots); verr != nil {
		return model.Library{}, verr
	}
	return s.libraries.AddRoots(ctx, libraryID, roots)
}

// UpdateLibraryRoot changes the content or the provider list of a folder (ADR 0015). The rules
// and the typed confirmation of a content change live in package library.
func (s *Service) UpdateLibraryRoot(ctx context.Context, libraryID, rootID int64, upd model.LibraryRootUpdate) (model.Library, error) {
	var verr *model.ValidationError
	if upd.Content == nil && upd.Providers == nil {
		verr = verr.Add("body", "required")
	}
	if upd.Content != nil && !model.ValidRootContent(*upd.Content) {
		verr = verr.Add("content", "invalid_value")
	}
	if upd.Providers != nil && len(upd.Providers) == 0 {
		verr = verr.Add("providers", "required")
	}
	if len(upd.ConfirmPath) > maxLibraryPath {
		verr = verr.Add("confirmPath", "too_long")
	}
	if verr != nil {
		return model.Library{}, verr
	}
	return s.libraries.UpdateRoot(ctx, libraryID, rootID, upd)
}

// ListMetadataProviders describes the four providers to the administration
// (listMetadataProviders, ADR 0015): whether each one is configured, which roles it plays and in
// the lists of which root contents it may stand. The interface builds its checkbox list from
// this and shows "no key" and "no German" from it, instead of a table of its own.
func (s *Service) ListMetadataProviders(ctx context.Context) ([]model.MetadataProviderInfo, error) {
	tmdbKey, err := s.TMDBKey(ctx)
	if err != nil {
		return nil, err
	}
	tvdbKey, _, err := s.TVDBKey(ctx)
	if err != nil {
		return nil, err
	}
	infos := []model.MetadataProviderInfo{
		{ID: model.ProviderTMDB, Configured: tmdbKey != "", Roles: []string{"identify", "fill"}, Languages: []string{"de", "en"}},
		{ID: model.ProviderTVDB, Configured: tvdbKey != "", Roles: []string{"identify", "fill"}, Languages: []string{"de", "en"}},
		{ID: model.ProviderAniList, Configured: s.anilist, Roles: []string{"fill"}, Languages: []string{"en"}},
		{ID: model.ProviderLocal, Configured: true, Roles: []string{"identify"}, Languages: []string{}},
	}
	for i := range infos {
		infos[i].Contents = []string{}
		for _, content := range []string{model.RootMovie, model.RootSeries, model.RootAudiobook, model.RootPodcast} {
			if slices.Contains(model.ProvidersForContent(content), infos[i].ID) {
				infos[i].Contents = append(infos[i].Contents, content)
			}
		}
	}
	return infos, nil
}

// RemoveLibraryRoot removes a folder from a library and lets the titles that live only there
// leave it (idea 02, E12). confirmPath is compared against the stored row inside package
// library, never here: a confirmation checked against another value from the same request would
// confirm nothing.
func (s *Service) RemoveLibraryRoot(ctx context.Context, libraryID, rootID int64, confirmPath string) (model.Library, error) {
	var verr *model.ValidationError
	switch {
	case strings.TrimSpace(confirmPath) == "":
		verr = verr.Add("confirmPath", "required")
	case len(strings.TrimSpace(confirmPath)) > maxLibraryPath:
		verr = verr.Add("confirmPath", "too_long")
	}
	if verr != nil {
		return model.Library{}, verr
	}
	return s.libraries.RemoveRoot(ctx, libraryID, rootID, confirmPath)
}

// queueRefresh queues a metadata match for the title, which fills in the rating again now that the
// override and its lock are gone. A failure is logged only: the override is cleared either way.
func (s *Service) queueRefresh(ctx context.Context, itemID string) {
	if s.queue == nil {
		return
	}
	payload, err := json.Marshal(map[string]string{"itemId": itemID})
	if err == nil {
		_, err = s.queue.Enqueue(ctx, jobs.NewJob{Type: jobMetadataMatch, Priority: 4, IdempotencyKey: "match:" + itemID, Payload: payload})
	}
	if err != nil {
		s.log.WarnContext(ctx, "queue metadata refresh", "item_id", itemID, "err", err)
	}
}

// ratingLockField is the entry of items.locked_fields that keeps the metadata job from
// overwriting an override (the job locks by column name).
const ratingLockField = "own_rating"

// SetItemRating overrides the age rating of a title (M2, plan 10.2 "Freigabe-Änderung"): own_rating
// and rating_source "manual" are set and own_rating is added to locked_fields, so enrichment keeps
// the value. The triggers recompute the effective rating of the title and its descendants in the
// same statement. A nil rating drops the override: own_rating and rating_source become NULL and the
// lock is removed; the title counts as unrated (18) until the next metadata match restores the
// rating. The result is the unfiltered administration view. model.ErrNotFound for an unknown title.
func (s *Service) SetItemRating(ctx context.Context, itemID string, rating *int) (model.ItemRating, error) {
	if !validRating(rating) {
		return model.ItemRating{}, (&model.ValidationError{}).Add("rating", "invalid_value")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ItemRating{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	ok, refresh, err := s.applyRating(ctx, systemq.New(tx), itemID, rating)
	if err != nil {
		return model.ItemRating{}, err
	}
	if !ok {
		return model.ItemRating{}, model.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return model.ItemRating{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "item rating overridden", "item_id", itemID, "rating_set", rating != nil)
	if refresh {
		s.queueRefresh(ctx, itemID)
	}
	return s.vis.SystemItemRating(ctx, visibility.System("admin rating override"), itemID)
}

// maxRatingBatch is the size of one transaction of the mass rating (design 4.2) and of the
// maintenance cleanup (plan 3.6: batches of at most 500).
const maxRatingBatch = 500

// maxRatingItems caps one mass rating request (contract ItemsRatingRequest).
const maxRatingItems = 5000

// SetItemsRating assigns (or, with nil, clears) the rating of many titles with the semantics of
// SetItemRating per title, in transactions of at most 500 titles (B-9, AN-1). Unknown and deleted
// ids are skipped and not counted; duplicates count once. The caller re-checks open streams once.
func (s *Service) SetItemsRating(ctx context.Context, itemIDs []string, rating *int) (int, error) {
	var verr *model.ValidationError
	switch {
	case len(itemIDs) == 0:
		verr = verr.Add("itemIds", "required")
	case len(itemIDs) > maxRatingItems:
		verr = verr.Add("itemIds", "too_long")
	}
	if !validRating(rating) {
		verr = verr.Add("rating", "invalid_value")
	}
	if verr != nil {
		return 0, verr
	}
	ids := slices.Clone(itemIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	// AN-2: one title of an audio library rated below its default rejects the whole request.
	for _, id := range ids {
		item, err := s.sys.GetItemSystem(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("get item: %w", err)
		}
		if err := s.checkRatingFloor(ctx, item, rating); err != nil {
			return 0, err
		}
	}
	var updated []string
	var refresh []string
	for chunk := range slices.Chunk(ids, maxRatingBatch) {
		done, again, err := s.rateBatch(ctx, chunk, rating)
		if err != nil {
			return len(updated), err
		}
		updated, refresh = append(updated, done...), append(refresh, again...)
	}
	s.log.InfoContext(ctx, "item ratings overridden", "requested", len(ids), "updated", len(updated), "rating_set", rating != nil)
	for _, id := range refresh {
		s.queueRefresh(ctx, id)
	}
	return len(updated), nil
}

// rateBatch applies the rating to one batch in one transaction and returns the changed ids and
// those whose metadata is matched again (a cleared override outside audio libraries).
func (s *Service) rateBatch(ctx context.Context, ids []string, rating *int) (done, refresh []string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	for _, id := range ids {
		ok, again, err := s.applyRating(ctx, q, id, rating)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			done = append(done, id)
		}
		if again {
			refresh = append(refresh, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}
	return done, refresh, nil
}

func validRating(rating *int) bool {
	return rating == nil || slices.Contains([]int{0, 6, 12, 16, 18}, *rating)
}

// applyRating writes the override (or its removal) of one title inside the caller's transaction;
// ok is false for an unknown or deleted title. In an audio library (M6, AN-2) a rating below the
// library default is model.ErrRatingBelowDefault, and a removed override restores the library
// default (FSK 18 from the feed or tags for an explicit title) instead of a metadata match; refresh
// reports whether the caller queues that match.
func (s *Service) applyRating(ctx context.Context, q *systemq.Queries, itemID string, rating *int) (ok, refresh bool, err error) {
	item, err := q.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt.Valid) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("get item: %w", err)
	}
	if err := s.checkRatingFloor(ctx, item, rating); err != nil {
		return false, false, err
	}
	lib, err := s.read.GetLibrary(ctx, item.LibraryID)
	if err != nil {
		return false, false, fmt.Errorf("get library of %s: %w", itemID, err)
	}
	var locked []string
	if err := json.Unmarshal([]byte(item.LockedFields), &locked); err != nil {
		locked = nil
	}
	locked = slices.DeleteFunc(locked, func(f string) bool { return f == ratingLockField })
	params := systemq.UpdateItemRatingOverrideParams{ID: itemID, Now: s.now().UnixMilli()}
	switch {
	case rating != nil:
		locked = append(locked, ratingLockField)
		params.OwnRating = sql.NullInt64{Int64: int64(*rating), Valid: true}
		params.RatingSource = sql.NullString{String: "manual", Valid: true}
	case lib.DefaultRating.Valid && item.Explicit == 1:
		params.OwnRating = sql.NullInt64{Int64: 18, Valid: true}
		params.RatingSource = sql.NullString{String: explicitSource(item.Kind), Valid: true}
	case lib.DefaultRating.Valid && item.Kind != "podcast_episode":
		params.OwnRating = lib.DefaultRating
		params.RatingSource = sql.NullString{String: "library", Valid: true}
	}
	raw, err := json.Marshal(locked)
	if err != nil {
		return false, false, fmt.Errorf("locked fields: %w", err)
	}
	if locked == nil {
		raw = []byte("[]")
	}
	params.LockedFields = string(raw)
	if err := q.UpdateItemRatingOverride(ctx, params); err != nil {
		return false, false, fmt.Errorf("update rating: %w", err)
	}
	return true, rating == nil && !lib.DefaultRating.Valid, nil
}

// explicitSource is the rating source of an explicit title: the feed for podcasts and episodes,
// metadata.json or the tags for audiobooks.
func explicitSource(kind string) string {
	if kind == "audiobook" {
		return "tags"
	}
	return "feed"
}
