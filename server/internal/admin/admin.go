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

// TMDBChecker is implemented by package metadata.
type TMDBChecker interface {
	CheckKey(ctx context.Context, key string) error // nil, model.ErrTMDBKeyInvalid or model.ErrTMDBUnreachable
}

// Service implements api.AdminService and is the KeySource of package metadata.
type Service struct {
	db        *sql.DB // writer, for the rating override transaction
	write     *appq.Queries
	read      *appq.Queries
	auth      *auth.Service
	vis       *visibility.Reader // system-scope read of the rating override
	queue     Enqueuer           // metadata refresh after a cleared override; nil = none
	libraries Libraries
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
		db: db.Write, write: appq.New(db.Write), read: appq.New(db.Read), auth: auth, vis: visibility.New(db, log), libraries: libraries,
		keyring: keyring, tmdb: tmdb, getenv: getenv, log: log, now: time.Now,
	}
}

// WithQueue sets the job queue used to refresh the metadata of a title whose rating override was
// cleared, and returns the receiver.
func (s *Service) WithQueue(q Enqueuer) *Service {
	s.queue = q
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
	if key := strings.TrimSpace(s.getenv(EnvTMDBKey)); key != "" {
		return key, nil
	}
	enc, err := s.read.GetSetting(ctx, SettingTMDBKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read tmdb key setting: %w", err)
	}
	key, err := s.keyring.Decrypt(enc)
	if err != nil {
		return "", fmt.Errorf("tmdb key setting: %w", err)
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
	if rating != nil && !slices.Contains([]int{0, 6, 12, 16, 18}, *rating) {
		return model.ItemRating{}, (&model.ValidationError{}).Add("rating", "invalid_value")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ItemRating{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	item, err := q.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt.Valid) {
		return model.ItemRating{}, model.ErrNotFound
	}
	if err != nil {
		return model.ItemRating{}, fmt.Errorf("get item: %w", err)
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
		return model.ItemRating{}, fmt.Errorf("locked fields: %w", err)
	}
	if locked == nil {
		raw = []byte("[]")
	}
	params.LockedFields = string(raw)
	if err := q.UpdateItemRatingOverride(ctx, params); err != nil {
		return model.ItemRating{}, fmt.Errorf("update rating: %w", err)
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
