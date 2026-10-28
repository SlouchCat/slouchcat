// Package auth owns accounts, passwords and browser sessions (plan 3.1 auth, 3.8 points 2 and 7).
//
// Passwords are stored as argon2id PHC strings. A session token is 256 random bits, handed to the
// client exactly once (cookie); the database keeps only its SHA-256. Sessions expire 30 days after
// the last use; Resolve extends them at most once per touchInterval so a busy client does not write
// on every request.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

const (
	// SessionLifetime is the sliding expiry of a session.
	SessionLifetime = 30 * 24 * time.Hour
	// touchInterval limits how often Resolve writes last_seen_at/expires_at.
	touchInterval = 5 * time.Minute
	// revokedRetention keeps revoked sessions for a day before DeleteExpiredSessions removes them.
	revokedRetention = 24 * time.Hour

	tokenBytes       = 32
	maxEmailLen      = 254
	minPasswordLen   = 10
	maxPasswordLen   = 256
	maxProfileName   = 60
	maxUserAgentLen  = 512
	defaultMaxRating = 18
)

// Service implements api.AuthService and creates the owner account for the setup wizard.
type Service struct {
	db     *sql.DB // writer, for transactions
	write  *appq.Queries
	read   *appq.Queries
	log    *slog.Logger
	now    func() time.Time
	params argonParams

	attempts Limiter
	pinMu    sync.Mutex // serialises PIN checks (pin.go)

	dummyOnce sync.Once
	dummyHash string

	// Hooks of package notify (M8), set before the service runs; nil = none.
	accountCreated     func(ctx context.Context, accountID int64) error
	invitationAccepted func(ctx context.Context, inviterAccountID int64, guestEmail string) error
}

// New returns the service on db.
func New(db *store.DB, log *slog.Logger) *Service {
	return &Service{
		db:     db.Write,
		write:  appq.New(db.Write),
		read:   appq.New(db.Read),
		log:    log,
		now:    time.Now,
		params: defaultParams,
	}
}

// WithClock replaces the clock (tests) and returns the receiver.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// WithAccountCreated runs fn after CreateOwner or AcceptInvitation committed a new account
// (notify.EnsureDefaults: the default channels of plan 5.2) and returns the receiver. An error is
// logged; the account stays.
func (s *Service) WithAccountCreated(fn func(ctx context.Context, accountID int64) error) *Service {
	s.accountCreated = fn
	return s
}

// WithInvitationAccepted runs fn after a guest accepted an invitation, with the account that
// created it and the guest's e-mail (notify.InvitationAccepted, M8 A-6), and returns the receiver.
// An error is logged.
func (s *Service) WithInvitationAccepted(fn func(ctx context.Context, inviterAccountID int64, guestEmail string) error) *Service {
	s.invitationAccepted = fn
	return s
}

// created runs the account hook.
func (s *Service) created(ctx context.Context, accountID int64) {
	if s.accountCreated == nil {
		return
	}
	if err := s.accountCreated(ctx, accountID); err != nil {
		s.log.WarnContext(ctx, "account created hook failed", "account_id", accountID, "err", err)
	}
}

// SetupRequired reports whether no account exists yet.
func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	n, err := s.read.CountAccounts(ctx)
	if err != nil {
		return false, fmt.Errorf("count accounts: %w", err)
	}
	return n == 0, nil
}

// ValidateOwner checks the e-mail and password of a new owner account and returns the normalised
// e-mail (trimmed, lower-cased). The returned error is nil or a *model.ValidationError with the
// fields "email" and "password".
func ValidateOwner(email, password string) (string, *model.ValidationError) {
	var verr *model.ValidationError
	email = NormalizeEmail(email)
	switch {
	case email == "":
		verr = verr.Add("email", "required")
	case !validEmail(email):
		verr = verr.Add("email", "invalid_email")
	}
	n := utf8.RuneCountInString(password)
	switch {
	case password == "":
		verr = verr.Add("password", "required")
	case n < minPasswordLen:
		verr = verr.Add("password", "too_short")
	case n > maxPasswordLen:
		verr = verr.Add("password", "too_long")
	}
	return email, verr
}

// NormalizeEmail trims and lower-cases an e-mail address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if len(email) > maxEmailLen || strings.Count(email, "@") != 1 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	local, domain, _ := strings.Cut(email, "@")
	return local != "" && strings.Contains(domain, ".") &&
		!strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".") && !strings.Contains(domain, "..")
}

// CreateOwner creates the owner's account and its default profile in one transaction. The setup
// wizard runs once, so it returns model.ErrSetupDone when any account exists; the check runs
// inside the write transaction, so two concurrent wizards cannot both succeed. Every further
// account comes from an invitation, and further administrators only from an appointment by the
// owner (ADR 0014).
func (s *Service) CreateOwner(ctx context.Context, email, password, language string) (model.Account, model.Profile, error) {
	email, verr := ValidateOwner(email, password)
	if language == "" {
		language = "de"
	}
	if language != "de" && language != "en" {
		verr = verr.Add("language", "invalid_value")
	}
	if verr != nil {
		return model.Account{}, model.Profile{}, verr
	}
	release, err := acquireArgon(ctx)
	if err != nil {
		return model.Account{}, model.Profile{}, err
	}
	hash, err := hashPassword(password, s.params)
	release()
	if err != nil {
		return model.Account{}, model.Profile{}, err
	}
	now := s.now().UnixMilli()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Account{}, model.Profile{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	n, err := q.CountAccounts(ctx)
	if err != nil {
		return model.Account{}, model.Profile{}, fmt.Errorf("count accounts: %w", err)
	}
	if n > 0 {
		return model.Account{}, model.Profile{}, model.ErrSetupDone
	}
	acc, err := q.InsertAccount(ctx, appq.InsertAccountParams{
		Email: email, PasswordHash: hash, Role: model.RoleAdmin, IsOwner: 1, Now: now,
	})
	if err != nil {
		return model.Account{}, model.Profile{}, fmt.Errorf("insert account: %w", err)
	}
	prof, err := q.InsertProfile(ctx, appq.InsertProfileParams{
		AccountID: acc.ID, Name: profileName(email), MaxRating: defaultMaxRating, Language: language,
		AllLibraries: 1, IsDefault: 1, Now: now,
	})
	if err != nil {
		return model.Account{}, model.Profile{}, fmt.Errorf("insert profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Account{}, model.Profile{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "owner account created", "account_id", acc.ID, "profile_id", prof.ID)
	s.created(ctx, acc.ID)
	return toAccount(acc, nil), toProfile(prof, nil, model.PlaybackLimits{}), nil
}

// profileName is the local part of the e-mail, cut to the column limit.
func profileName(email string) string {
	local, _, _ := strings.Cut(email, "@")
	if utf8.RuneCountInString(local) > maxProfileName {
		local = string([]rune(local)[:maxProfileName])
	}
	return local
}

// Login checks the credentials and starts a session. Unknown e-mail and wrong password take the
// same time and both return model.ErrInvalidCredentials. Over-long input is refused before any
// hashing; a client over the attempt limit gets model.ErrTooManyAttempts.
func (s *Service) Login(ctx context.Context, email, password, userAgent string) (model.Session, string, error) {
	email = NormalizeEmail(email)
	var verr *model.ValidationError
	switch {
	case email == "":
		verr = verr.Add("email", "required")
	case utf8.RuneCountInString(email) > maxEmailLen:
		verr = verr.Add("email", "too_long")
	}
	switch {
	case password == "":
		verr = verr.Add("password", "required")
	case utf8.RuneCountInString(password) > maxPasswordLen:
		verr = verr.Add("password", "too_long")
	}
	if verr != nil {
		return model.Session{}, "", verr
	}
	if err := s.AllowAttempt(ctx); err != nil {
		return model.Session{}, "", err
	}
	acc, err := s.read.GetAccountByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		// Spend the same time as a real verification so the answer does not reveal whether the
		// account exists.
		if _, err := s.verify(ctx, password, ""); err != nil && ctx.Err() != nil {
			return model.Session{}, "", err
		}
		return model.Session{}, "", model.ErrInvalidCredentials
	}
	if err != nil {
		return model.Session{}, "", fmt.Errorf("get account: %w", err)
	}
	ok, err := s.verify(ctx, password, acc.PasswordHash)
	if err != nil {
		return model.Session{}, "", fmt.Errorf("account %d: %w", acc.ID, err)
	}
	if !ok {
		return model.Session{}, "", model.ErrInvalidCredentials
	}
	if _, err := s.DeleteExpiredSessions(ctx); err != nil {
		s.log.WarnContext(ctx, "delete expired sessions", "err", err)
	}
	profiles, err := s.read.ListProfilesByAccount(ctx, acc.ID)
	if err != nil {
		return model.Session{}, "", fmt.Errorf("profiles of account %d: %w", acc.ID, err)
	}
	// Exactly one profile without PIN is chosen right away; otherwise the client shows the picker.
	var active appq.Profile
	if len(profiles) == 1 && !profiles[0].PinHash.Valid {
		active = profiles[0]
	}
	account, err := s.accountModel(ctx, s.read, acc)
	if err != nil {
		return model.Session{}, "", err
	}
	var profile model.Profile
	if active.ID != 0 {
		if profile, err = s.profileModel(ctx, s.read, active, account); err != nil {
			return model.Session{}, "", err
		}
	}
	return s.StartSession(ctx, account, profile, userAgent)
}

// verify checks password against encoded (the dummy hash when encoded is empty) in an argon2id
// slot.
func (s *Service) verify(ctx context.Context, password, encoded string) (bool, error) {
	release, err := acquireArgon(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	if encoded == "" {
		encoded = s.dummy()
	}
	return verifyPassword(password, encoded)
}

// dummy returns a hash of a random password with the service's parameters, computed once.
func (s *Service) dummy() string {
	s.dummyOnce.Do(func() {
		h, err := hashPassword(rand.Text(), s.params)
		if err != nil {
			panic(err) // crypto/rand never fails on supported platforms
		}
		s.dummyHash = h
	})
	return s.dummyHash
}

// StartSession creates a session for account and returns it with the plain token. A zero profile
// starts the session without an active profile (the client shows the picker). The device of the
// request (model.DeviceID) is remembered with the session.
func (s *Service) StartSession(ctx context.Context, account model.Account, profile model.Profile, userAgent string) (model.Session, string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return model.Session{}, "", fmt.Errorf("session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	expires := now.Add(SessionLifetime)
	ua := sql.NullString{String: truncate(userAgent, maxUserAgentLen), Valid: userAgent != ""}
	device, err := s.sessionDevice(ctx, userAgent)
	if err != nil {
		return model.Session{}, "", err
	}
	row, err := s.write.InsertSession(ctx, appq.InsertSessionParams{
		TokenHash: hashToken(token), AccountID: account.ID,
		ProfileID: sql.NullInt64{Int64: profile.ID, Valid: profile.ID != 0},
		DeviceID:  sql.NullString{String: device, Valid: true},
		UserAgent: ua, Now: now.UnixMilli(), ExpiresAt: expires.UnixMilli(),
	})
	if err != nil {
		return model.Session{}, "", fmt.Errorf("insert session: %w", err)
	}
	return model.Session{ID: row.ID, Account: account, Profile: profile, DeviceID: device, ExpiresAt: time.UnixMilli(row.ExpiresAt)}, token, nil
}

// Resolve returns the session of a plain token, extending it when it was last touched more than
// touchInterval ago; extended reports whether the expiry moved, so the caller can re-issue the
// cookie. Unknown, expired and revoked tokens return model.ErrNotFound.
func (s *Service) Resolve(ctx context.Context, token string) (session model.Session, extended bool, err error) {
	if token == "" || len(token) > 2*tokenBytes {
		return model.Session{}, false, model.ErrNotFound
	}
	now := s.now()
	row, err := s.read.GetSessionByTokenHash(ctx, appq.GetSessionByTokenHashParams{TokenHash: hashToken(token), Now: now.UnixMilli()})
	if errors.Is(err, sql.ErrNoRows) {
		return model.Session{}, false, model.ErrNotFound
	}
	if err != nil {
		return model.Session{}, false, fmt.Errorf("get session: %w", err)
	}
	session, err = s.loadSession(ctx, s.read, row)
	if err != nil {
		return model.Session{}, false, err
	}
	if now.Sub(time.UnixMilli(row.LastSeenAt)) >= touchInterval {
		next := now.Add(SessionLifetime)
		if err := s.write.TouchSession(ctx, appq.TouchSessionParams{ID: row.ID, Now: now.UnixMilli(), ExpiresAt: next.UnixMilli()}); err != nil {
			// The session stays valid until its old expiry; a failed touch must not log the user out.
			s.log.WarnContext(ctx, "touch session", "session_id", row.ID, "err", err)
		} else {
			session.ExpiresAt, extended = next, true
			if err := s.write.TouchAccount(ctx, appq.TouchAccountParams{ID: row.AccountID, Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}}); err != nil {
				s.log.WarnContext(ctx, "touch account", "account_id", row.AccountID, "err", err)
			}
			if err := s.write.UpsertDevice(ctx, appq.UpsertDeviceParams{ID: session.DeviceID, Now: now.UnixMilli()}); err != nil {
				s.log.WarnContext(ctx, "touch device", "session_id", row.ID, "err", err)
			}
		}
	}
	return session, extended, nil
}

// loadSession builds the model session of a session row: account and profile with their library
// rights (loaded only for restricted accounts and profiles).
func (s *Service) loadSession(ctx context.Context, q *appq.Queries, row appq.Session) (model.Session, error) {
	acc, err := q.GetAccount(ctx, row.AccountID)
	if err != nil {
		return model.Session{}, notFoundOr(err, "get account")
	}
	account, err := s.accountModel(ctx, q, acc)
	if err != nil {
		return model.Session{}, err
	}
	session := model.Session{ID: row.ID, Account: account, DeviceID: row.DeviceID.String, ExpiresAt: time.UnixMilli(row.ExpiresAt)}
	if !row.DeviceID.Valid {
		// A session from before migration 00007 gets its device now, fixed for its lifetime.
		var err error
		if session.DeviceID, err = s.assignDevice(ctx, row.ID); err != nil {
			return model.Session{}, err
		}
	}
	if row.ProfileID.Valid {
		prof, err := q.GetProfile(ctx, row.ProfileID.Int64)
		if err != nil {
			return model.Session{}, notFoundOr(err, "get profile")
		}
		if session.Profile, err = s.profileModel(ctx, q, prof, account); err != nil {
			return model.Session{}, err
		}
	}
	return session, nil
}

// accountModel maps an account row; guests get their library rights.
func (s *Service) accountModel(ctx context.Context, q *appq.Queries, a appq.Account) (model.Account, error) {
	var libs []int64
	if a.AllLibraries != 1 {
		var err error
		if libs, err = q.ListAccountLibraries(ctx, a.ID); err != nil {
			return model.Account{}, fmt.Errorf("account libraries: %w", err)
		}
	}
	return toAccount(a, libs), nil
}

// profileModel maps a profile row of account; a profile with restricted libraries gets them.
func (s *Service) profileModel(ctx context.Context, q *appq.Queries, p appq.Profile, account model.Account) (model.Profile, error) {
	var libs []int64
	if p.AllLibraries != 1 {
		var err error
		if libs, err = q.ListProfileLibraries(ctx, p.ID); err != nil {
			return model.Profile{}, fmt.Errorf("profile libraries: %w", err)
		}
	}
	return toProfile(p, libs, account.Limits), nil
}

// Logout revokes the session and, in the same transaction, every playback session it started, so
// their stream tokens answer 401 from the next request on (plan 5.4); revoking an already revoked
// or unknown session is not an error. The API stops the HLS output through the playback service.
func (s *Service) Logout(ctx context.Context, sessionID int64) error {
	now := sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin logout: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	if _, err := q.RevokeSession(ctx, appq.RevokeSessionParams{ID: sessionID, Now: now}); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if _, err := q.RevokePlaybackSessionsBySession(ctx, appq.RevokePlaybackSessionsBySessionParams{
		SessionID: sessionID, Now: now, StoppedBy: sql.NullString{String: model.StoppedByLogout, Valid: true},
	}); err != nil {
		return fmt.Errorf("revoke playback sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit logout: %w", err)
	}
	return nil
}

// deviceRetention is how long a device (and its PIN counters) is kept without use.
const deviceRetention = 400 * 24 * time.Hour

// DeleteExpiredSessions removes expired sessions and sessions revoked more than a day ago, except
// those a playback session still refers to (package playback keeps them for the statistics), and
// devices and PIN counters unused for 400 days (the lifetime of the device cookie).
func (s *Service) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	now := s.now()
	n, err := s.write.DeleteExpiredSessions(ctx, appq.DeleteExpiredSessionsParams{
		Now: now.UnixMilli(), Before: sql.NullInt64{Int64: now.Add(-revokedRetention).UnixMilli(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	cutoff := now.Add(-deviceRetention).UnixMilli()
	if _, err := s.write.DeleteStalePinAttempts(ctx, cutoff); err != nil {
		return n, fmt.Errorf("delete stale pin attempts: %w", err)
	}
	if _, err := s.write.DeleteStaleDevices(ctx, cutoff); err != nil {
		return n, fmt.Errorf("delete stale devices: %w", err)
	}
	return n, nil
}

func nullInt(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }

func notFoundOr(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func toAccount(a appq.Account, libs []int64) model.Account {
	if libs == nil {
		libs = []int64{}
	}
	return model.Account{
		ID: a.ID, Email: a.Email, Role: a.Role, IsOwner: a.IsOwner == 1, MaxRating: int(a.MaxRating),
		AllLibraries: a.AllLibraries == 1, LibraryIDs: libs, Limits: AccountLimits(a),
	}
}

// toProfile maps a profile row; account are the caps of its account (Account.Limits), which
// bound the effective limits.
func toProfile(p appq.Profile, libs []int64, account model.PlaybackLimits) model.Profile {
	if libs == nil {
		libs = []int64{}
	}
	limits := ProfileLimits(p)
	return model.Profile{
		ID: p.ID, AccountID: p.AccountID, Name: p.Name, MaxRating: int(p.MaxRating), Language: p.Language,
		AllLibraries: p.AllLibraries == 1, LibraryIDs: libs, Accent: p.Accent.String,
		HasPin: p.PinHash.Valid, HasOfflinePin: p.OfflinePin.Valid, IsPrimary: p.IsDefault == 1,
		Limits: limits, Effective: limits.Cap(account),
	}
}

// AccountLimits are the W-8 caps of an account row (guests); the watch-time limit is a profile
// setting and stays zero.
func AccountLimits(a appq.Account) model.PlaybackLimits {
	return model.PlaybackLimits{
		MaxBitrateKbps: int(a.MaxBitrateKbps.Int64), MaxHeight: int(a.MaxHeight.Int64), MaxStreams: int(a.MaxStreams.Int64),
	}
}

// ProfileLimits are the W-8 and K-5 limits of a profile row (zero = none).
func ProfileLimits(p appq.Profile) model.PlaybackLimits {
	return model.PlaybackLimits{
		MaxBitrateKbps: int(p.MaxBitrateKbps.Int64), MaxHeight: int(p.MaxHeight.Int64), MaxStreams: int(p.MaxStreams.Int64),
		WatchLimitMinutes: int(p.WatchLimitMinutes.Int64),
	}
}
