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

// Libraries is implemented by package library.
type Libraries interface {
	ValidatePath(ctx context.Context, path string) error                         // model.ErrLibraryPathInvalid when not absolute, missing, not a directory or not readable
	Create(ctx context.Context, req model.LibraryRequest) (model.Library, error) // validates, detects the file system, inserts library + root, queues the first scan
	List(ctx context.Context) ([]model.Library, error)
	Get(ctx context.Context, id int64) (model.Library, error) // model.ErrNotFound
	QueueScan(ctx context.Context, id int64) (model.Library, error)
	AcceptLoss(ctx context.Context, id int64) (model.Library, error)
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
	libraries Libraries
	maint     LibraryMaintenance // nil until WithLibraryMaintenance
	meta      Metadata           // nil until WithMetadata
	streams   Streams            // nil until WithStreams
	keyring   *secrets.Keyring
	tmdb      TMDBChecker
	getenv    func(string) string
	log       *slog.Logger
	now       func() time.Time
}

// New returns the service. getenv is normally os.Getenv.
func New(db *store.DB, auth *auth.Service, libraries Libraries, keyring *secrets.Keyring, tmdb TMDBChecker,
	getenv func(string) string, log *slog.Logger,
) *Service {
	return &Service{
		db: db.Write, write: appq.New(db.Write), read: appq.New(db.Read), sys: systemq.New(db.Read), auth: auth, vis: visibility.New(db, log),
		libraries: libraries,
		keyring:   keyring, tmdb: tmdb, getenv: getenv, log: log, now: time.Now,
	}
}

// WithQueue sets the job queue used to refresh the metadata of a title whose rating override was
// cleared, and returns the receiver.
func (s *Service) WithQueue(q Enqueuer) *Service {
	s.queue = q
	return s
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
	if err := s.libraries.ValidatePath(ctx, req.Library.Path); err != nil {
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

// validateLibrary checks name, kind and path presence; prefix names the fields ("library.").
func validateLibrary(verr *model.ValidationError, prefix string, req model.LibraryRequest) *model.ValidationError {
	name := strings.TrimSpace(req.Name)
	switch {
	case name == "":
		verr = verr.Add(prefix+"name", "required")
	case utf8.RuneCountInString(name) > maxLibraryName:
		verr = verr.Add(prefix+"name", "too_long")
	}
	switch req.Kind {
	case "":
		verr = verr.Add(prefix+"kind", "required")
	case "movies", "series":
	default:
		verr = verr.Add(prefix+"kind", "invalid_value")
	}
	switch {
	case strings.TrimSpace(req.Path) == "":
		verr = verr.Add(prefix+"path", "required")
	case len(req.Path) > maxLibraryPath:
		verr = verr.Add(prefix+"path", "too_long")
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

// ScanLibrary queues a full scan.
func (s *Service) ScanLibrary(ctx context.Context, id int64) (model.Library, error) {
	return s.libraries.QueueScan(ctx, id)
}

// AcceptLibraryLoss lifts the loss protection for the next scan and queues it.
func (s *Service) AcceptLibraryLoss(ctx context.Context, id int64) (model.Library, error) {
	return s.libraries.AcceptLoss(ctx, id)
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
	ok, err := s.applyRating(ctx, systemq.New(tx), itemID, rating)
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
	if rating == nil {
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
	var updated []string
	for chunk := range slices.Chunk(ids, maxRatingBatch) {
		done, err := s.rateBatch(ctx, chunk, rating)
		if err != nil {
			return len(updated), err
		}
		updated = append(updated, done...)
	}
	s.log.InfoContext(ctx, "item ratings overridden", "requested", len(ids), "updated", len(updated), "rating_set", rating != nil)
	if rating == nil {
		for _, id := range updated {
			s.queueRefresh(ctx, id)
		}
	}
	return len(updated), nil
}

// rateBatch applies the rating to one batch in one transaction and returns the changed ids.
func (s *Service) rateBatch(ctx context.Context, ids []string, rating *int) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	var done []string
	for _, id := range ids {
		ok, err := s.applyRating(ctx, q, id, rating)
		if err != nil {
			return nil, err
		}
		if ok {
			done = append(done, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return done, nil
}

func validRating(rating *int) bool {
	return rating == nil || slices.Contains([]int{0, 6, 12, 16, 18}, *rating)
}

// applyRating writes the override (or its removal) of one title inside the caller's transaction;
// false for an unknown or deleted title.
func (s *Service) applyRating(ctx context.Context, q *systemq.Queries, itemID string, rating *int) (bool, error) {
	item, err := q.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt.Valid) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get item: %w", err)
	}
	var locked []string
	if err := json.Unmarshal([]byte(item.LockedFields), &locked); err != nil {
		locked = nil
	}
	locked = slices.DeleteFunc(locked, func(f string) bool { return f == ratingLockField })
	params := systemq.UpdateItemRatingOverrideParams{ID: itemID, Now: s.now().UnixMilli()}
	if rating != nil {
		locked = append(locked, ratingLockField)
		params.OwnRating = sql.NullInt64{Int64: int64(*rating), Valid: true}
		params.RatingSource = sql.NullString{String: "manual", Valid: true}
	}
	raw, err := json.Marshal(locked)
	if err != nil {
		return false, fmt.Errorf("locked fields: %w", err)
	}
	if locked == nil {
		raw = []byte("[]")
	}
	params.LockedFields = string(raw)
	if err := q.UpdateItemRatingOverride(ctx, params); err != nil {
		return false, fmt.Errorf("update rating: %w", err)
	}
	return true, nil
}
