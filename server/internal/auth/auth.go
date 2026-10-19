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
	roleOwner        = "owner"
)

// Service implements api.AuthService and creates the owner account for the setup wizard.
type Service struct {
	db     *sql.DB // writer, for transactions
	write  *appq.Queries
	read   *appq.Queries
	log    *slog.Logger
	now    func() time.Time
	params argonParams

	attempts attemptLimiter

	dummyOnce sync.Once
	dummyHash string
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

// CreateOwner creates the main account and its default profile in one transaction. Because M1a
// has exactly one owner, it returns model.ErrSetupDone when any account exists; the check runs
// inside the write transaction, so two concurrent wizards cannot both succeed.
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
	acc, err := q.InsertAccount(ctx, appq.InsertAccountParams{Email: email, PasswordHash: hash, Role: roleOwner, Now: now})
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
	return toAccount(acc), toProfile(prof), nil
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
	prof, err := s.read.GetDefaultProfile(ctx, acc.ID)
	if err != nil {
		return model.Session{}, "", fmt.Errorf("default profile of account %d: %w", acc.ID, err)
	}
	if _, err := s.DeleteExpiredSessions(ctx); err != nil {
		s.log.WarnContext(ctx, "delete expired sessions", "err", err)
	}
	return s.StartSession(ctx, toAccount(acc), toProfile(prof), userAgent)
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

// StartSession creates a session for account and profile and returns it with the plain token.
func (s *Service) StartSession(ctx context.Context, account model.Account, profile model.Profile, userAgent string) (model.Session, string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return model.Session{}, "", fmt.Errorf("session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	expires := now.Add(SessionLifetime)
	ua := sql.NullString{String: truncate(userAgent, maxUserAgentLen), Valid: userAgent != ""}
	row, err := s.write.InsertSession(ctx, appq.InsertSessionParams{
		TokenHash: hashToken(token), AccountID: account.ID, ProfileID: profile.ID, UserAgent: ua,
		Now: now.UnixMilli(), ExpiresAt: expires.UnixMilli(),
	})
	if err != nil {
		return model.Session{}, "", fmt.Errorf("insert session: %w", err)
	}
	return model.Session{ID: row.ID, Account: account, Profile: profile, ExpiresAt: time.UnixMilli(row.ExpiresAt)}, token, nil
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
	acc, err := s.read.GetAccount(ctx, row.AccountID)
	if err != nil {
		return model.Session{}, false, notFoundOr(err, "get account")
	}
	prof, err := s.read.GetProfile(ctx, row.ProfileID)
	if err != nil {
		return model.Session{}, false, notFoundOr(err, "get profile")
	}
	expires := time.UnixMilli(row.ExpiresAt)
	if now.Sub(time.UnixMilli(row.LastSeenAt)) >= touchInterval {
		next := now.Add(SessionLifetime)
		if err := s.write.TouchSession(ctx, appq.TouchSessionParams{ID: row.ID, Now: now.UnixMilli(), ExpiresAt: next.UnixMilli()}); err != nil {
			// The session stays valid until its old expiry; a failed touch must not log the user out.
			s.log.WarnContext(ctx, "touch session", "session_id", row.ID, "err", err)
		} else {
			expires, extended = next, true
		}
	}
	return model.Session{ID: row.ID, Account: toAccount(acc), Profile: toProfile(prof), ExpiresAt: expires}, extended, nil
}

// Logout revokes the session; revoking an already revoked or unknown session is not an error.
func (s *Service) Logout(ctx context.Context, sessionID int64) error {
	if _, err := s.write.RevokeSession(ctx, appq.RevokeSessionParams{ID: sessionID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}}); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes expired sessions and sessions revoked more than a day ago.
func (s *Service) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	now := s.now()
	n, err := s.write.DeleteExpiredSessions(ctx, appq.DeleteExpiredSessionsParams{
		Now: now.UnixMilli(), Before: sql.NullInt64{Int64: now.Add(-revokedRetention).UnixMilli(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return n, nil
}

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

func toAccount(a appq.Account) model.Account {
	return model.Account{ID: a.ID, Email: a.Email, Role: a.Role}
}

func toProfile(p appq.Profile) model.Profile {
	return model.Profile{
		ID: p.ID, AccountID: p.AccountID, Name: p.Name, MaxRating: int(p.MaxRating), Language: p.Language,
		AllLibraries: p.AllLibraries == 1, Accent: p.Accent.String,
	}
}
