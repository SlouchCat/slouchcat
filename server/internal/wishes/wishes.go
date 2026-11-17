// Package wishes is the service of the wishes through Seerr (concept 25, architecture 2.3). Stage 1
// builds its foundation: the connection test and the fingerprint of the instance (connection.go),
// the mirror of the Seerr users and the job seerr.users (users.go), the Seerr user of each account
// (Mapping, mapping.go) and the request right of a profile (Rights, rights.go). Stage 2 requests:
// creating, co-wishing and the lists (request.go), the outbox wishes.submit (submit.go), the
// withdrawal wishes.withdraw (withdraw.go), the sync wishes.sync (sync.go) and the reconcile with
// the library wishes.reconcile (reconcile.go).
//
// Rules of the package:
//
//   - Only this package reads and writes the wish tables (store/wishq, depguard rule "wishq"). It
//     never asks items, media_files or tracks and never imports store/systemq (CLAUDE.md rule 2).
//   - The address and the key come from package admin (Config); the key is never logged and never
//     stored here. Every call to Seerr is logged in seerr_log with method, path without query,
//     status, duration and the error class, never a header, a body or the host.
//   - A profile always sends as the Seerr user of its account, else as the shared user
//     «slouchcat-gaeste» (F-27, F-4, N-100, N-101); there is no user per profile.
//   - While another instance is detected (architecture 7.4) no call runs on behalf of a user
//     (UserSession refuses with model.ErrInstanceChanged) and the users job writes nothing, until
//     the administrator confirms or resets (ConfirmInstance).
package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Config is implemented by package admin: the connection and the settings of the wishes.
type Config interface {
	SeerrConfig(ctx context.Context) (model.SeerrConfig, error)
	WishSettings(ctx context.Context) (model.WishSettings, error)
}

// Enqueuer is the job queue (package jobs).
type Enqueuer interface {
	Enqueue(ctx context.Context, j jobs.NewJob) (int64, error)
	RaiseQueuedPriority(ctx context.Context, id int64, prio int) (bool, error)
}

// State keys this package keeps in settings (data model 2.12); the administrator's keys belong to
// package admin.
const (
	settingVersion     = "seerr.version"
	settingInstance    = "seerr.instance"         // the confirmed fingerprint, JSON fingerprint
	settingInstanceNew = "seerr.instance_pending" // a fingerprint that differs, waiting for confirmSeerrInstance
	settingTest        = "seerr.test"             // the last test of the saved connection, JSON testRecord (T-1)
	settingLastSuccess = "seerr.last_success"
	settingLastAttempt = "seerr.last_attempt"
	settingLastError   = "seerr.last_error" // an error class: unreachable, key_invalid, server, csrf
)

// Service is the wish service. Mapping and Rights are its two parts of stage 1; their methods are
// promoted.
type Service struct {
	*Mapping
	*Rights

	db    *store.DB
	read  *wishq.Queries
	write *wishq.Queries
	cfg   Config
	queue Enqueuer
	log   *slog.Logger
	now   func() time.Time
	opts  []seerr.Option

	lib      Library  // the library through visibility (deps.go)
	titles   Titles   // titles outside the library through metadata
	notifier Notifier // the messages; nil sends none

	bg sync.WaitGroup // hand-overs of a password in flight (password.go)

	locks   wishLocks    // one submission or withdrawal per wish at a time (submit.go)
	seasons seasonsCache // the season lists of Seerr, 1 h (request.go)

	mu       sync.Mutex
	client   *seerr.Client
	clientOf string // base and key the cached client was made for (in memory only)

	arr arrServices // access to Radarr and Sonarr, made on first use (arraccess.go)

	discover searchCache // the search of the wishes area, 10 minutes (discover.go)

	searchLimit *auth.Limiter // searchExternalTitles per profile (preview.go)
}

// Option configures New.
type Option func(*Service)

// WithClock replaces time.Now (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithClientOptions adds options to every Seerr client (tests: a short time-out).
func WithClientOptions(opts ...seerr.Option) Option {
	return func(s *Service) { s.opts = append(s.opts, opts...) }
}

// New returns the service. queue may be nil in tests that do not queue jobs.
func New(db *store.DB, cfg Config, queue Enqueuer, log *slog.Logger, opts ...Option) *Service {
	s := &Service{db: db, read: wishq.New(db.Read), write: wishq.New(db.Write), cfg: cfg, queue: queue, log: log, now: time.Now,
		searchLimit: auth.NewLimiter(searchPerMin, searchPerMin)}
	for _, o := range opts {
		o(s)
	}
	s.Mapping = &Mapping{s: s}
	s.Rights = &Rights{s: s}
	return s
}

// errNotConfigured wraps model.ErrSeerrNotConfigured.
var errDisabled = errors.New("seerr switched off")

// connection returns the saved connection and a client for it; model.ErrSeerrNotConfigured without
// address or key. A switched-off connection is returned with errDisabled when !evenDisabled.
func (s *Service) connection(ctx context.Context, evenDisabled bool) (model.SeerrConfig, *seerr.Client, error) {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return model.SeerrConfig{}, nil, err
	}
	if !cfg.Configured() {
		return cfg, nil, model.ErrSeerrNotConfigured
	}
	if !cfg.Enabled && !evenDisabled {
		return cfg, nil, errDisabled
	}
	base, err := seerr.NormalizeAddress(cfg.URL)
	if err != nil {
		return cfg, nil, model.ErrSeerrNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && s.clientOf == base+"\x00"+cfg.APIKey {
		return cfg, s.client, nil
	}
	c, err := s.newClient(base, cfg.APIKey)
	if err != nil {
		return cfg, nil, model.ErrSeerrNotConfigured
	}
	s.client, s.clientOf = c, base+"\x00"+cfg.APIKey
	return cfg, c, nil
}

// newClient makes a client that logs every call to seerr_log.
func (s *Service) newClient(base, key string) (*seerr.Client, error) {
	opts := append([]seerr.Option{seerr.WithLogger(s.log), seerr.WithObserver(s.observe)}, s.opts...)
	return seerr.New(base, key, opts...)
}

// UserSession returns the session of calls on behalf of the Seerr user id; model.ErrInstanceChanged
// while another instance waits for the administrator, model.ErrSeerrNotConfigured without a
// connection, and an error when Seerr is switched off. Every call on behalf of a user goes through
// here (architecture 7.4).
func (s *Service) UserSession(ctx context.Context, id int) (seerr.Session, error) {
	if pending, err := s.instancePending(ctx); err != nil {
		return seerr.Session{}, err
	} else if pending != nil {
		return seerr.Session{}, model.ErrInstanceChanged
	}
	_, c, err := s.connection(ctx, false)
	if err != nil {
		return seerr.Session{}, err
	}
	return c.AsUser(id), nil
}

// AdminSession returns the session of administration calls (no X-API-User) on the saved
// connection; errors as UserSession except the instance check.
func (s *Service) AdminSession(ctx context.Context) (seerr.Session, error) {
	_, c, err := s.connection(ctx, false)
	if err != nil {
		return seerr.Session{}, err
	}
	return c.AsAdmin(), nil
}

// maxLogError is the length of an error in seerr_log.
const maxLogError = 300

// observe writes one call to seerr_log. The error is its class and Seerr's cleaned message; never
// the host, a header or a body.
func (s *Service) observe(ctx context.Context, c seerr.Call) {
	var status sql.NullInt64
	if c.Status != 0 {
		status = sql.NullInt64{Int64: int64(c.Status), Valid: true}
	}
	var msg sql.NullString
	if c.Err != nil {
		text := errorClass(c.Err)
		var se *seerr.Error
		if errors.As(c.Err, &se) && se.Message != "" {
			text += ": " + se.Message
		}
		if r := []rune(text); len(r) > maxLogError {
			text = string(r[:maxLogError])
		}
		msg = sql.NullString{String: text, Valid: true}
	}
	path := c.Path
	if r := []rune(path); len(r) > 300 {
		path = string(r[:300])
	}
	err := s.write.InsertSeerrLog(context.WithoutCancel(ctx), wishq.InsertSeerrLogParams{
		At: s.now().UnixMilli(), Method: c.Method, Path: path, Status: status,
		DurationMs: max(0, c.Duration.Milliseconds()), Error: msg,
	})
	if err != nil {
		s.log.WarnContext(ctx, "seerr log not written", "err", err)
	}
}

// errorClass names the class of a Seerr error for the log and the status.
func errorClass(err error) string {
	switch {
	case errors.Is(err, seerr.ErrRedirect):
		return "redirect"
	case errors.Is(err, seerr.ErrUnreachable):
		return "unreachable"
	case errors.Is(err, seerr.ErrKeyInvalid):
		return "key_invalid"
	case errors.Is(err, seerr.ErrAccessDenied):
		return "access_denied"
	case errors.Is(err, seerr.ErrCSRF):
		return "csrf"
	case errors.Is(err, seerr.ErrQuota):
		return "quota"
	case errors.Is(err, seerr.ErrNoPermission):
		return "no_permission"
	case errors.Is(err, seerr.ErrForbidden):
		return "forbidden"
	case errors.Is(err, seerr.ErrNotFound):
		return "not_found"
	case errors.Is(err, seerr.ErrConflict):
		return "conflict"
	case errors.Is(err, seerr.ErrServer):
		return "server"
	case errors.Is(err, seerr.ErrBadRequest):
		return "bad_request"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	}
	return "failed"
}

// state reads a state setting; "" when absent.
func (s *Service) state(ctx context.Context, key string) (string, error) {
	v, err := s.read.GetWishSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	return v, nil
}

func (s *Service) setState(ctx context.Context, q *wishq.Queries, key, value string) error {
	if err := q.UpsertWishSetting(ctx, wishq.UpsertWishSettingParams{Key: key, Value: value, Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("store %s: %w", key, err)
	}
	return nil
}

func (s *Service) delState(ctx context.Context, q *wishq.Queries, key string) error {
	if err := q.DeleteWishSetting(ctx, key); err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

// stateJSON reads a JSON state setting into v; false when absent or unreadable.
func (s *Service) stateJSON(ctx context.Context, key string, v any) (bool, error) {
	raw, err := s.state(ctx, key)
	if err != nil || raw == "" {
		return false, err
	}
	if json.Unmarshal([]byte(raw), v) != nil {
		s.log.WarnContext(ctx, "unreadable seerr state", "key", key)
		return false, nil
	}
	return true, nil
}

func (s *Service) setStateJSON(ctx context.Context, q *wishq.Queries, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	return s.setState(ctx, q, key, string(b))
}

// recordOutcome keeps the state of the connection after a test or a job: the time of the attempt,
// of the last success and the class of the last error ("" after a success).
func (s *Service) recordOutcome(ctx context.Context, err error) {
	now := fmt.Sprint(s.now().UnixMilli())
	w := s.write
	errs := []error{s.setState(ctx, w, settingLastAttempt, now)}
	if err == nil {
		errs = append(errs, s.setState(ctx, w, settingLastSuccess, now), s.delState(ctx, w, settingLastError))
	} else {
		errs = append(errs, s.setState(ctx, w, settingLastError, errorClass(err)))
	}
	if e := errors.Join(errs...); e != nil {
		s.log.WarnContext(ctx, "seerr state not written", "err", e)
	}
}

// inTx runs fn in one write transaction.
func (s *Service) inTx(ctx context.Context, fn func(q *wishq.Queries) error) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(wishq.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func nullInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

func nullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	return sql.NullString{String: s, Valid: s != ""}
}

// TitlesInUse names the titles outside the library the wishes hold (metadata.WithWishTitles, wired
// in main.go; migration 00074): open those of open wishes, recent those of wishes closed since
// closedSince. titles.refresh fetches the open ones first and removes neither.
func (s *Service) TitlesInUse(ctx context.Context, closedSince time.Time) (open, recent []metadata.TitleRef, err error) {
	rows, err := s.read.ListWishTitlesInUse(ctx, sql.NullInt64{Int64: closedSince.UnixMilli(), Valid: true})
	if err != nil {
		return nil, nil, fmt.Errorf("titles of the wishes: %w", err)
	}
	seen := map[metadata.TitleRef]bool{}
	for _, r := range rows {
		ref := metadata.TitleRef{Media: r.Media, TMDBID: r.TmdbID}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		if r.Open == 1 {
			open = append(open, ref)
		} else {
			recent = append(recent, ref)
		}
	}
	return open, recent, nil
}
