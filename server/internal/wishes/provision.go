package wishes

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Opening the Seerr user of an account (concept 25, architecture 18.2, build step 1.5a): search by
// the email; a hit is linked at once, whoever triggered it (E-8, N-90); otherwise POST /user. A
// user Slouchcat opened keeps the rights Seerr gives it (defaultPermissions, N-110), unless the
// account may not request (permissions 0). Seerr unreachable: the account waits and the job
// seerr.provision tries again. The shared user «slouchcat-gaeste» is made sure of the same way
// (N-101, N-111). A password reaches this file only in memory (build step 1.5b); never in a job
// payload, a table or a log line.

// JobProvision is the job type of the provisioning.
const JobProvision = "seerr.provision"

// Triggers of a link (data model 2.2, linked_by).
const (
	LinkedByAdmin       = "admin"        // «In Seerr eröffnen», «Alle eröffnen»
	LinkedByInvite      = "invite"       // an invitation was accepted
	LinkedByEmailChange = "email_change" // the account changed its email (N-95)
)

// The shared user (N-101).
const (
	SharedEmail    = "slouchcat-gaeste@slouchcat.invalid"
	SharedUsername = "slouchcat-gaeste"
)

const (
	provisionPriority = 5
	// linkedHintDays is how long the overview shows «Neu verknüpft» (N-93).
	linkedHintDays = 7
	// userSearchTake is the page of GET /user?q=<email>: the search also matches parts of names.
	userSearchTake = 100
)

// Intents of an account job: what the trigger asked for. Only intentOpen and intentRetry may
// search by the email and open a user; intentSync only brings a ready user in line (rights, limit,
// quota) and does nothing when the mapping is gone or changed by the time the job runs.
const (
	intentOpen  = "open"  // «In Seerr eröffnen», «Alle eröffnen», an invitation
	intentRetry = "retry" // a mapping that waits for Seerr; nothing when it no longer waits
	intentSync  = "sync"  // a ready user after a hand choice, a new limit or a new request right
)

// provisionPayload is the payload of a seerr.provision job. It never carries a password.
type provisionPayload struct {
	AccountID   int64  `json:"accountId,omitempty"`
	Intent      string `json:"intent,omitempty"`    // intentOpen, intentRetry or intentSync ("" counts as sync)
	SeerrUser   int    `json:"seerrUser,omitempty"` // intentSync: the user the trigger saw; 0 = any ready user
	LinkedBy    string `json:"linkedBy,omitempty"`
	SetBy       int64  `json:"setBy,omitempty"`
	Shared      bool   `json:"shared,omitempty"`      // make sure of the shared user
	SharedLimit bool   `json:"sharedLimit,omitempty"` // and write the default limit to it again
	CloseSeerr  int    `json:"closeSeerr,omitempty"`  // a user Slouchcat opened whose account is gone
}

// handleProvision runs one seerr.provision job. A run that cannot reach Seerr fails, so the queue
// tries again; an account that is gone or a connection that is not set up ends it quietly.
func (s *Service) handleProvision(ctx context.Context, j *jobs.Job) error {
	var p provisionPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("%s payload: %w", JobProvision, err)
	}
	var err error
	switch {
	case p.Shared:
		err = s.ensureShared(ctx, p.SharedLimit)
	case p.CloseSeerr > 0:
		err = s.closeUser(ctx, p.CloseSeerr)
	case p.AccountID > 0 && (p.Intent == intentOpen || p.Intent == intentRetry):
		err = s.provisionJob(ctx, p)
	case p.AccountID > 0:
		err = s.syncAccount(ctx, p.AccountID, p.SeerrUser)
	}
	switch {
	case errors.Is(err, model.ErrNotFound), errors.Is(err, model.ErrSeerrNotConfigured), errors.Is(err, errDisabled),
		errors.Is(err, model.ErrProvisionDisabled):
		return nil
	}
	return err
}

// queueProvision queues one provisioning job; one waiting job per account (or shared user, or
// closed user) is enough.
func (s *Service) queueProvision(ctx context.Context, p provisionPayload, prio int) error {
	if s.queue == nil {
		return errors.New("wishes: no job queue")
	}
	key := JobProvision + ":account:" + strconv.FormatInt(p.AccountID, 10)
	switch {
	case p.AccountID > 0 && p.Intent != intentOpen && p.Intent != intentRetry:
		// A sync never swallows an open job, nor one for another user (a second hand choice).
		key = JobProvision + ":sync:" + strconv.FormatInt(p.AccountID, 10) + ":" + strconv.Itoa(p.SeerrUser)
	case p.Shared:
		key = JobProvision + ":shared"
	case p.CloseSeerr > 0:
		key = JobProvision + ":close:" + strconv.Itoa(p.CloseSeerr)
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode %s: %w", JobProvision, err)
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobProvision, Priority: prio, IdempotencyKey: key, Payload: payload})
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", JobProvision, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return fmt.Errorf("raise %s: %w", JobProvision, err)
	}
	return nil
}

// provisionJob runs an open or retry job: only while «Neue Konten in Seerr eröffnen» is on and the
// connection is set up, and a retry only while the mapping still waits (an administrator may have
// undone it or chosen a user by hand since).
func (s *Service) provisionJob(ctx context.Context, p provisionPayload) error {
	if err := s.provisionAllowed(ctx); err != nil {
		return err
	}
	if p.Intent == intentRetry {
		m, err := s.read.GetAccountMapping(ctx, p.AccountID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && m.State != StateWaiting) {
			s.log.InfoContext(ctx, "seerr provisioning dropped: mapping no longer waits", "account_id", p.AccountID)
			return nil
		}
		if err != nil {
			return fmt.Errorf("mapping: %w", err)
		}
	}
	_, err := s.provision(ctx, p.AccountID, p.LinkedBy, "", p.SetBy)
	return err
}

// syncAccount runs a sync job: it brings the ready user of the account in line and never searches
// or opens a user. Nothing when the account has no ready user or, with seerrUser > 0, another one.
func (s *Service) syncAccount(ctx context.Context, accountID int64, seerrUser int) error {
	acc, err := s.read.GetProvisionAccount(ctx, accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("account: %w", err)
	}
	m, err := s.read.GetAccountMapping(ctx, accountID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mapping: %w", err)
	}
	if err != nil || m.State != StateReady || !m.SeerrUserID.Valid || (seerrUser > 0 && int(m.SeerrUserID.Int64) != seerrUser) {
		s.log.InfoContext(ctx, "seerr sync dropped: mapping gone or changed", "account_id", accountID)
		return nil
	}
	admin, err := s.provisionSession(ctx)
	if err != nil {
		return err
	}
	return s.syncReady(ctx, admin, acc, m)
}

// ProvisionAccount is «In Seerr eröffnen» (provisionSeerrAccount): it opens or links the Seerr
// user of the account at once and returns the mapping; state waiting when Seerr was not reached
// (the job tries again). A hit by the email keeps its password (password state off, F-24, N-103).
// model.ErrNotFound, model.ErrProvisionDisabled («Neue Konten in Seerr eröffnen» off),
// model.ErrSeerrNotConfigured.
func (s *Service) ProvisionAccount(ctx context.Context, accountID, by int64) (model.WishAccountMapping, error) {
	if err := s.provisionAllowed(ctx); err != nil {
		return model.WishAccountMapping{}, err
	}
	if _, err := s.provision(ctx, accountID, LinkedByAdmin, "", by); err != nil && !s.waits(err) {
		return model.WishAccountMapping{}, err
	}
	return s.accountMapping(ctx, accountID)
}

// ProvisionAll is «Alle eröffnen» (provisionSeerrAccounts): it queues the provisioning of every
// account without a ready Seerr user and returns how many of them the mirror links by the email
// and how many get a new user. Errors as ProvisionAccount.
func (s *Service) ProvisionAll(ctx context.Context, by int64) (link, create int, err error) {
	if err := s.provisionAllowed(ctx); err != nil {
		return 0, 0, err
	}
	rows, err := s.read.ListUnprovisionedAccounts(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list accounts: %w", err)
	}
	for _, r := range rows {
		_, err := s.read.FindFreeSeerrUserByEmail(ctx, nullString(r.Email))
		switch {
		case err == nil:
			link++
		case errors.Is(err, sql.ErrNoRows):
			create++
		default:
			return 0, 0, fmt.Errorf("find seerr user: %w", err)
		}
		if err := s.queueProvision(ctx, provisionPayload{AccountID: r.ID, Intent: intentOpen, LinkedBy: LinkedByAdmin, SetBy: by}, jobs.ManualPriority); err != nil {
			return 0, 0, err
		}
	}
	s.log.InfoContext(ctx, "seerr provisioning of every account queued", "link", link, "create", create)
	return link, create, nil
}

// provisionAllowed checks the switch «Neue Konten in Seerr eröffnen» and the connection.
func (s *Service) provisionAllowed(ctx context.Context) error {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Configured() {
		return model.ErrSeerrNotConfigured
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return err
	}
	if !ws.ProvisionAccounts {
		return model.ErrProvisionDisabled
	}
	return nil
}

// accountMapping returns the mapping of one account as getWishMappings shows it.
func (s *Service) accountMapping(ctx context.Context, accountID int64) (model.WishAccountMapping, error) {
	all, err := s.WishMappings(ctx)
	if err != nil {
		return model.WishAccountMapping{}, err
	}
	for _, a := range all.Accounts {
		if a.AccountID == accountID {
			return a, nil
		}
	}
	return model.WishAccountMapping{}, model.ErrNotFound
}

// waits reports whether err left the account waiting for Seerr (the job tries again).
func (s *Service) waits(err error) bool {
	var se *seerr.Error
	return errors.As(err, &se) || errors.Is(err, model.ErrInstanceChanged) || errors.Is(err, context.DeadlineExceeded)
}

// provision opens or links the Seerr user of the account. password is the account's password when
// it is in memory (an invitation, build step 1.5b), else "". It returns the state of the mapping:
// ready, waiting, or "" when nothing was to be done (an email change of an account without a user
// and without a hit in Seerr). A waiting account comes back with the error that made it wait.
func (s *Service) provision(ctx context.Context, accountID int64, linkedBy, password string, setBy int64) (string, error) {
	acc, err := s.read.GetProvisionAccount(ctx, accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", model.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("account: %w", err)
	}
	m, err := s.read.GetAccountMapping(ctx, accountID)
	hasRow := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("mapping: %w", err)
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return "", err
	}
	if linkedBy == "" {
		linkedBy = LinkedByAdmin
	}
	// An administrator's «In Seerr eröffnen» may link a privileged user; the account's own trigger
	// (an invitation, an email change, a retry of one) may not (security review H-1).
	auto := linkedBy != LinkedByAdmin
	if hasRow && m.State == StateWaiting && m.LinkedBy.Valid && linkedBy != LinkedByEmailChange {
		linkedBy = m.LinkedBy.String // an invitation stays the trigger of a later link
	}
	admin, err := s.provisionSession(ctx)
	if err != nil {
		if errors.Is(err, model.ErrInstanceChanged) && (!hasRow || m.State == StateWaiting) {
			return s.wait(ctx, accountID, linkedBy, setBy, err)
		}
		return "", err
	}
	if hasRow && m.State == StateReady {
		return StateReady, s.syncReady(ctx, admin, acc, m)
	}
	hit, err := findUser(ctx, admin, acc.Email)
	if err != nil {
		return s.wait(ctx, accountID, linkedBy, setBy, err)
	}
	if hit == nil && linkedBy == LinkedByEmailChange && (!hasRow || m.LinkedBy.String == LinkedByEmailChange) {
		// N-95: only a hit links an account that was never opened, also when an earlier email
		// change left it waiting.
		if hasRow {
			if _, err := s.write.DeleteAccountMapping(ctx, accountID); err != nil {
				return "", fmt.Errorf("drop waiting mapping: %w", err)
			}
		}
		return "", nil
	}
	if hit == nil {
		u, created, err := s.createUser(ctx, admin, acc.Email, password, ws)
		if err != nil {
			return s.wait(ctx, accountID, linkedBy, setBy, err)
		}
		if !created {
			hit = &u // 409: the user exists after all
		} else if err := s.opened(ctx, admin, acc, u, password != "" && ws.SyncPassword, ws, setBy); err != nil {
			return s.wait(ctx, accountID, linkedBy, setBy, err)
		}
	}
	if hit != nil {
		if err := s.link(ctx, admin, acc, *hit, linkedBy, auto, password, ws, setBy); err != nil {
			switch {
			case errors.Is(err, model.ErrSeerrUserTaken):
				return s.waitTaken(ctx, accountID, linkedBy, setBy, ProvisionUserTaken)
			case errors.Is(err, errUserPrivileged):
				return s.waitTaken(ctx, accountID, linkedBy, setBy, ProvisionUserPrivileged)
			}
			return s.wait(ctx, accountID, linkedBy, setBy, err)
		}
	}
	return StateReady, nil
}

// provisionSession is the administration session for the provisioning: refused while another
// instance waits for the administrator (architecture 7.4).
func (s *Service) provisionSession(ctx context.Context) (seerr.Session, error) {
	if pending, err := s.instancePending(ctx); err != nil {
		return seerr.Session{}, err
	} else if pending != nil {
		return seerr.Session{}, model.ErrInstanceChanged
	}
	return s.AdminSession(ctx)
}

// wait keeps the account waiting for Seerr with the class of err and returns err, so the job tries
// again.
func (s *Service) wait(ctx context.Context, accountID int64, linkedBy string, setBy int64, cause error) (string, error) {
	// Without cancel: a hand-over that ran out of its time still records the wait.
	if err := s.write.SetWaitingMapping(context.WithoutCancel(ctx), wishq.SetWaitingMappingParams{
		AccountID: accountID, LinkedBy: nullString(linkedBy), ProvisionError: nullString(provisionError(cause)),
		Now: s.now().UnixMilli(), SetBy: sql.NullInt64{Int64: setBy, Valid: setBy > 0},
	}); err != nil {
		return "", fmt.Errorf("waiting mapping: %w", err)
	}
	var detail string
	var se *seerr.Error
	if errors.As(cause, &se) {
		detail = se.Message
	}
	s.log.InfoContext(ctx, "seerr user of account waits", "account_id", accountID, "reason", provisionError(cause), "detail", detail)
	return StateWaiting, cause
}

// Waiting reasons of an account whose email names a Seerr user Slouchcat may not take
// (WishAccountMapping.provisionError): nothing to retry, the administrator chooses by hand.
const (
	// ProvisionUserTaken: the user belongs to another account or is the shared user.
	ProvisionUserTaken = "seerr_user_taken"
	// ProvisionUserPrivileged: the user is user 1 or holds a managing right (ADMIN, MANAGE_USERS,
	// MANAGE_SETTINGS, MANAGE_REQUESTS); an invitation or an email change never takes it over
	// (security review H-1). The administrator can still choose it by hand.
	ProvisionUserPrivileged = "seerr_user_privileged"
)

// errUserPrivileged: an automatic link met a privileged user.
var errUserPrivileged = errors.New("wishes: the seerr user of the email administers seerr")

// waitTaken keeps the account waiting because its email names a user Slouchcat may not take
// (reason ProvisionUserTaken or ProvisionUserPrivileged): nothing to retry, the administrator
// chooses by hand.
func (s *Service) waitTaken(ctx context.Context, accountID int64, linkedBy string, setBy int64, reason string) (string, error) {
	if err := s.write.SetWaitingMapping(ctx, wishq.SetWaitingMappingParams{
		AccountID: accountID, LinkedBy: nullString(linkedBy), ProvisionError: nullString(reason),
		Now: s.now().UnixMilli(), SetBy: sql.NullInt64{Int64: setBy, Valid: setBy > 0},
	}); err != nil {
		return "", fmt.Errorf("waiting mapping: %w", err)
	}
	s.log.WarnContext(ctx, "seerr user of the email may not be linked automatically", "account_id", accountID, "reason", reason)
	return StateWaiting, nil
}

// provisionError is the text of provision_error: only the class of the error, as the contract
// describes it (WishAccountMapping.provisionError); Seerr's message goes to the log.
func provisionError(err error) string {
	if errors.Is(err, model.ErrInstanceChanged) {
		return "instance_changed"
	}
	return errorClass(err)
}

// findUser searches the user with this email (any spelling) with GET /user?q=; nil without a hit.
func findUser(ctx context.Context, admin seerr.Session, email string) (*seerr.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	page, err := admin.Users(ctx, seerr.UserQuery{Q: email, Take: userSearchTake})
	if err != nil {
		return nil, err
	}
	for _, u := range page.Results {
		if strings.EqualFold(strings.TrimSpace(u.Email), email) {
			return &u, nil
		}
	}
	return nil, nil
}

// createUser opens a local user: the email in lower case, the part before the @ as username, the
// account's password when it may be synced, else a random one that is never stored (N-4). 409 is
// a hit after all (created false, the user read again).
func (s *Service) createUser(ctx context.Context, admin seerr.Session, email, password string, ws model.WishSettings) (seerr.User, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	username, _, _ := strings.Cut(email, "@")
	if password == "" || !ws.SyncPassword {
		password = randomPassword()
	}
	u, err := admin.CreateUser(ctx, seerr.NewUser{Email: email, Username: username, Password: password})
	if errors.Is(err, seerr.ErrUserExists) {
		hit, err := findUser(ctx, admin, email)
		switch {
		case err != nil:
			return seerr.User{}, false, err
		case hit == nil:
			return seerr.User{}, false, err409Lost
		}
		return *hit, false, nil
	}
	if err != nil {
		return seerr.User{}, false, err
	}
	u.Email = email // Seerr 3.5.1 answers without it (appendix I, I-12)
	return u, true, nil
}

// err409Lost: POST /user answered 409, but the search found no user with the email.
var err409Lost = errors.New("seerr reported the email as taken but no user has it")

// randomPassword returns a password nobody knows: for users opened without the account's password
// and for the shared user. It is never stored.
func randomPassword() string { return rand.Text() }

// opened records a user Slouchcat just created: the mirror, the mapping (origin created) and the
// rights (permissions 0 without the request right, N-110), then the limit.
func (s *Service) opened(ctx context.Context, admin seerr.Session, acc wishq.GetProvisionAccountRow, u seerr.User, synced bool, ws model.WishSettings, setBy int64) error {
	if acc.CanRequest != 1 {
		if err := admin.SetPermissions(ctx, u.ID, seerr.PermNone); err != nil {
			return err
		}
		u.Permissions = seerr.PermNone
	}
	state := PasswordNextLogin
	var syncedAt sql.NullInt64
	switch {
	case !ws.SyncPassword:
		state = PasswordOff
	case synced:
		state, syncedAt = PasswordSynced, sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}
	}
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		if err := s.mirror(ctx, q, u); err != nil {
			return err
		}
		return q.SetReadyMapping(ctx, wishq.SetReadyMappingParams{
			AccountID: acc.ID, SeerrUserID: sql.NullInt64{Int64: int64(u.ID), Valid: true},
			Origin: nullString(OriginCreated), PasswordState: state, PasswordSyncedAt: syncedAt,
			Now: s.now().UnixMilli(), SetBy: sql.NullInt64{Int64: setBy, Valid: setBy > 0},
		})
	})
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "seerr user opened", "account_id", acc.ID, "seerr_user_id", u.ID, "password_state", state)
	if err := s.writeLimit(ctx, admin, acc.ID, u.ID); err != nil {
		return err
	}
	return s.refreshQuota(ctx, admin, u.ID)
}

// link records an existing user as the user of the account (origin linked, N-90) with the trigger
// and the time; its rights stay as they are (N-9). The password goes to it only after an
// invitation or an email change, and never to an administrator or a user signing in through Plex,
// Jellyfin or Emby (N-91, N-103, N-6); after a link by an administrator the state is off.
// model.ErrSeerrUserTaken when the user belongs to another account or is the shared user;
// errUserPrivileged when an automatic link (auto: an invitation, an email change) meets user 1 or a user with a managing
// right (security review H-1: only an administrator links such a user, by «In Seerr eröffnen» or
// by hand).
func (s *Service) link(ctx context.Context, admin seerr.Session, acc wishq.GetProvisionAccountRow, u seerr.User, linkedBy string, auto bool, password string, ws model.WishSettings, setBy int64) error {
	if auto && privileged(int64(u.ID), u.Permissions) {
		return errUserPrivileged
	}
	now := s.now().UnixMilli()
	fallback := PasswordNextLogin
	if linkedBy == LinkedByAdmin || !ws.SyncPassword {
		fallback = PasswordOff
	}
	var state string
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		row, err := q.GetSeerrUser(ctx, int64(u.ID))
		switch {
		case err == nil && row.IsShared == 1:
			return model.ErrSeerrUserTaken
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("seerr user: %w", err)
		}
		if other, err := q.GetMappingBySeerrUser(ctx, sql.NullInt64{Int64: int64(u.ID), Valid: true}); err == nil && other.AccountID != acc.ID {
			return model.ErrSeerrUserTaken
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("mapping of seerr user: %w", err)
		}
		if err := s.mirror(ctx, q, u); err != nil {
			return err
		}
		if row, err = q.GetSeerrUser(ctx, int64(u.ID)); err != nil {
			return fmt.Errorf("seerr user: %w", err)
		}
		state = linkedPasswordState(row, fallback)
		if err := s.adoptLimit(ctx, q, acc.ID, rawQuota(u), u.Permissions.Has(seerr.PermManageUsers), ws.DefaultLimit, setBy); err != nil {
			return err
		}
		return q.SetReadyMapping(ctx, wishq.SetReadyMappingParams{
			AccountID: acc.ID, SeerrUserID: sql.NullInt64{Int64: int64(u.ID), Valid: true},
			Origin: nullString(OriginLinked), LinkedBy: nullString(linkedBy), LinkedAt: sql.NullInt64{Int64: now, Valid: true},
			PasswordState: state, Now: now, SetBy: sql.NullInt64{Int64: setBy, Valid: setBy > 0 && linkedBy == LinkedByAdmin},
		})
	})
	if err != nil {
		if strings.Contains(err.Error(), "seerr_user_taken") {
			return model.ErrSeerrUserTaken
		}
		return err
	}
	// The log names the account, the Seerr user and the trigger; never the email or the password.
	s.log.InfoContext(ctx, "seerr user linked", "account_id", acc.ID, "seerr_user_id", u.ID, "linked_by", linkedBy, "password_state", state)
	if state == PasswordNextLogin && password != "" {
		s.setPassword(ctx, admin, acc.ID, u.ID, password)
	}
	if err := s.writeLimit(ctx, admin, acc.ID, u.ID); err != nil {
		return err
	}
	return s.refreshQuota(ctx, admin, u.ID)
}

// mirror writes the user to seerr_users (its row must exist before the mapping points at it).
func (s *Service) mirror(ctx context.Context, q *wishq.Queries, u seerr.User) error {
	p, ok := upsertParams(u, s.now().UnixMilli())
	if !ok {
		return fmt.Errorf("seerr user %d has an unknown user type", u.ID)
	}
	if err := q.UpsertSeerrUser(ctx, p); err != nil {
		return fmt.Errorf("store seerr user %d: %w", u.ID, err)
	}
	return nil
}

// setPassword hands the password to the user and records the state: synced, or next_login when
// Seerr refused or was not reached. It reads the user fresh first (security review L-2): user 1, a
// managing right or a sign-in through Plex, Jellyfin or Emby keeps its password, and the state
// becomes admin or external. The password is not kept.
func (s *Service) setPassword(ctx context.Context, admin seerr.Session, accountID int64, seerrUserID int, password string) {
	state, at := PasswordSynced, sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}
	u, err := admin.User(ctx, seerrUserID)
	switch {
	case err != nil:
		s.log.WarnContext(ctx, "seerr user not read before the password", "account_id", accountID, "seerr_user_id", seerrUserID, "reason", errorClass(err))
		state, at = PasswordNextLogin, sql.NullInt64{}
	case guardedState(int64(u.ID), u.Permissions, u.UserType) != "":
		state, at = guardedState(int64(u.ID), u.Permissions, u.UserType), sql.NullInt64{}
		s.log.InfoContext(ctx, "seerr password stays in seerr", "account_id", accountID, "seerr_user_id", seerrUserID, "password_state", state)
	default:
		if err := admin.SetPassword(ctx, seerrUserID, password); err != nil {
			s.log.WarnContext(ctx, "seerr password not set", "account_id", accountID, "seerr_user_id", seerrUserID, "reason", errorClass(err))
			state, at = PasswordNextLogin, sql.NullInt64{}
		}
	}
	if _, err := s.write.SetMappingPasswordState(ctx, wishq.SetMappingPasswordStateParams{PasswordState: state, PasswordSyncedAt: at, AccountID: accountID}); err != nil {
		s.log.WarnContext(ctx, "seerr password state not stored", "account_id", accountID, "err", err)
	}
}

// syncReady brings a ready user in line with the account: the rights of a user Slouchcat opened
// follow the request right (N-110: 0 without it, the instance's defaultPermissions when it comes
// back), linked and chosen users keep theirs.
func (s *Service) syncReady(ctx context.Context, admin seerr.Session, acc wishq.GetProvisionAccountRow, m wishq.WishAccountMap) error {
	if !m.SeerrUserID.Valid {
		return nil
	}
	if err := s.syncRights(ctx, admin, acc, m); err != nil {
		return err
	}
	id := int(m.SeerrUserID.Int64)
	if !m.LimitWrittenAt.Valid {
		if err := s.writeLimit(ctx, admin, acc.ID, id); err != nil {
			return err
		}
	}
	return s.refreshQuota(ctx, admin, id)
}

// syncRights: the rights of a user Slouchcat opened follow the request right (N-110).
func (s *Service) syncRights(ctx context.Context, admin seerr.Session, acc wishq.GetProvisionAccountRow, m wishq.WishAccountMap) error {
	if m.Origin.String != OriginCreated {
		return nil
	}
	id := int(m.SeerrUserID.Int64)
	u, err := admin.User(ctx, id)
	if err != nil {
		return err
	}
	want := u.Permissions
	switch {
	case acc.CanRequest != 1 && u.Permissions != seerr.PermNone:
		want = seerr.PermNone
	case acc.CanRequest == 1 && u.Permissions == seerr.PermNone:
		ms, err := admin.MainSettings(ctx)
		if err != nil {
			return err
		}
		want = ms.DefaultPermissions
	}
	if want == u.Permissions {
		return nil
	}
	if err := admin.SetPermissions(ctx, id, want); err != nil {
		return err
	}
	if want > seerr.Permission(1<<62) {
		return fmt.Errorf("permissions of seerr user %d out of range", id)
	}
	if err := s.write.SetSeerrUserPermissions(ctx, wishq.SetSeerrUserPermissionsParams{Permissions: int64(want), SeerrID: int64(id)}); err != nil {
		return fmt.Errorf("store permissions: %w", err)
	}
	s.log.InfoContext(ctx, "seerr rights of opened user follow the request right", "account_id", acc.ID, "seerr_user_id", id, "can_request", acc.CanRequest == 1)
	return nil
}

// writeLimit writes the effective limit of the account into the four limit fields of the user
// (architecture 19.1): read settings/main and send every field back. Nothing for a user with
// MANAGE_USERS: Seerr gives it no limit.
func (s *Service) writeLimit(ctx context.Context, admin seerr.Session, accountID int64, seerrUserID int) error {
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return err
	}
	l, err := s.read.GetAccountWishLimit(ctx, accountID)
	has := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("limit: %w", err)
	}
	limit := accountLimit(l, has, ws.DefaultLimit, false).Effective
	if err := s.writeLimits(ctx, admin, seerrUserID, limit); err != nil {
		return err
	}
	if accountID > 0 {
		if err := s.write.SetMappingLimitWritten(ctx, wishq.SetMappingLimitWrittenParams{
			LimitWrittenAt: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, AccountID: accountID,
		}); err != nil {
			return fmt.Errorf("limit written: %w", err)
		}
	}
	return nil
}

// writeLimits writes the limit into settings/main of the user unless it has MANAGE_USERS.
func (s *Service) writeLimits(ctx context.Context, admin seerr.Session, seerrUserID int, limit model.WishLimits) error {
	row, err := s.read.GetSeerrUser(ctx, int64(seerrUserID))
	if err == nil && row.IsAdmin == 1 {
		return nil // «Kein Limit» (N-34)
	}
	set, err := admin.UserSettings(ctx, seerrUserID)
	if err != nil {
		return err
	}
	set.MovieQuotaLimit, set.MovieQuotaDays = &limit.Movie.Limit, &limit.Movie.Days
	set.TVQuotaLimit, set.TVQuotaDays = &limit.TV.Limit, &limit.TV.Days
	return admin.SetUserSettings(ctx, seerrUserID, set)
}

// EnsureSharedUser makes sure the shared user «slouchcat-gaeste» exists once Seerr is set up and
// «Wünsche aktiv» is on (N-101): a user of that email is taken over, otherwise a local user with a
// random password that is never stored is opened. It keeps the rights Seerr gives a new user
// (defaultPermissions, N-111) and gets the default limit. A shared user already known does
// nothing.
func (s *Service) EnsureSharedUser(ctx context.Context) error { return s.ensureShared(ctx, false) }

// ensureShared is EnsureSharedUser; rewrite writes the default limit to a known shared user again.
func (s *Service) ensureShared(ctx context.Context, rewrite bool) error {
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return err
	}
	if !ws.Enabled {
		return nil
	}
	cur, err := s.SharedUser(ctx)
	if err != nil {
		return err
	}
	if cur != nil && !cur.Gone && !rewrite {
		return nil
	}
	admin, err := s.provisionSession(ctx)
	if err != nil {
		return err
	}
	if cur != nil && !cur.Gone {
		return s.writeLimits(ctx, admin, cur.ID, ws.DefaultLimit)
	}
	u, err := findUser(ctx, admin, SharedEmail)
	if err != nil {
		return err
	}
	var user seerr.User
	takenOver := u != nil
	if u != nil {
		user = *u
	} else {
		created, isNew, err := s.createUser(ctx, admin, SharedEmail, "", model.WishSettings{})
		if err != nil {
			return err
		}
		user, takenOver = created, !isNew
	}
	if takenOver {
		// Someone may know the password of a user that existed already (security review L-4).
		if err := admin.SetPassword(ctx, user.ID, randomPassword()); err != nil {
			return err
		}
	}
	if err := s.inTx(ctx, func(q *wishq.Queries) error { return s.mirror(ctx, q, user) }); err != nil {
		return err
	}
	if err := s.MarkShared(ctx, user.ID); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "shared seerr user ready", "seerr_user_id", user.ID, "taken_over", takenOver)
	if err := s.writeLimits(ctx, admin, user.ID, ws.DefaultLimit); err != nil {
		return err
	}
	return s.refreshQuota(ctx, admin, user.ID)
}

// WishSettingsSaved is the hook of package admin after the wish settings were saved: switching
// «Wünsche aktiv» on queues the shared user (N-101).
// A new default limit is written to every account that follows it and to the shared user.
func (s *Service) WishSettingsSaved(ctx context.Context, before, after model.WishSettings) {
	if after.Enabled && !before.Enabled {
		if err := s.queueProvision(ctx, provisionPayload{Shared: true}, jobs.ManualPriority); err != nil {
			s.log.WarnContext(ctx, "shared seerr user not queued", "err", err)
		}
	}
	if after.DefaultLimit != before.DefaultLimit {
		if err := s.defaultLimitChanged(ctx); err != nil {
			s.log.WarnContext(ctx, "new default limit not queued", "err", err)
		}
	}
}

// queueAfterUsers queues, after a refresh of the users, the accounts that wait for Seerr and the
// shared user while there is none.
func (s *Service) queueAfterUsers(ctx context.Context) error {
	rows, err := s.read.ListWaitingMappings(ctx)
	if err != nil {
		return fmt.Errorf("list waiting: %w", err)
	}
	for _, r := range rows {
		if err := s.queueProvision(ctx, provisionPayload{AccountID: r.AccountID, Intent: intentRetry, LinkedBy: r.LinkedBy.String, SetBy: r.SetBy.Int64}, provisionPriority); err != nil {
			return err
		}
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return err
	}
	if cur, err := s.SharedUser(ctx); err != nil {
		return err
	} else if ws.Enabled && (cur == nil || cur.Gone) {
		return s.queueProvision(ctx, provisionPayload{Shared: true}, provisionPriority)
	}
	return nil
}

// AccountDeleting is the hook of package auth before an account is deleted (K-2, F-31): for a
// user Slouchcat opened it returns what runs after the deletion, a job that takes the user's
// rights and gives it a random password. A linked or chosen user stays as it is; the user stays
// in Seerr in every case (its requests would go with it). nil when nothing is to be done.
func (s *Service) AccountDeleting(ctx context.Context, accountID int64) func(context.Context) {
	m, err := s.read.GetAccountMapping(ctx, accountID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.log.WarnContext(ctx, "mapping of deleted account not read", "account_id", accountID, "err", err)
		}
		return nil
	}
	if m.State != StateReady || m.Origin.String != OriginCreated || !m.SeerrUserID.Valid {
		return nil
	}
	id := int(m.SeerrUserID.Int64)
	return func(ctx context.Context) {
		if err := s.queueProvision(ctx, provisionPayload{CloseSeerr: id}, provisionPriority); err != nil {
			s.log.WarnContext(ctx, "closing of seerr user not queued", "seerr_user_id", id, "err", err)
		}
	}
}

// closeUser takes the rights of a user Slouchcat opened for a deleted account and gives it a random
// password (K-2); the user stays.
func (s *Service) closeUser(ctx context.Context, seerrUserID int) error {
	admin, err := s.provisionSession(ctx)
	if err != nil {
		return err
	}
	if err := admin.SetPermissions(ctx, seerrUserID, seerr.PermNone); err != nil {
		if errors.Is(err, seerr.ErrNotFound) {
			return nil
		}
		return err
	}
	if err := admin.SetPassword(ctx, seerrUserID, randomPassword()); err != nil {
		return err
	}
	if err := s.write.SetSeerrUserPermissions(ctx, wishq.SetSeerrUserPermissionsParams{Permissions: 0, SeerrID: int64(seerrUserID)}); err != nil {
		return fmt.Errorf("store permissions: %w", err)
	}
	s.log.InfoContext(ctx, "seerr user of deleted account closed", "seerr_user_id", seerrUserID)
	return nil
}

// RecentLinks returns the hint «Neu verknüpft» (N-93): accounts linked to an existing Seerr user
// after an invitation or an email change within the last seven days. Never after a link by an
// administrator.
func (s *Service) RecentLinks(ctx context.Context) ([]model.WishLinkHint, error) {
	since := s.now().Add(-linkedHintDays * 24 * time.Hour).UnixMilli()
	rows, err := s.read.ListRecentLinks(ctx, sql.NullInt64{Int64: since, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("recent links: %w", err)
	}
	out := make([]model.WishLinkHint, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.WishLinkHint{
			AccountID: r.AccountID, AccountEmail: r.AccountEmail, SeerrUserID: int(r.SeerrUserID.Int64),
			SeerrDisplayName: r.SeerrDisplayName, LinkedBy: r.LinkedBy.String,
			LinkedAt: time.UnixMilli(r.LinkedAt.Int64).UTC(),
		})
	}
	return out, nil
}

// RequestRightChanged is the hook of package auth after an administrator changed the request right
// of an account: the rights of a user Slouchcat opened follow it (N-110) through seerr.provision.
func (s *Service) RequestRightChanged(ctx context.Context, accountID int64) {
	m, err := s.read.GetAccountMapping(ctx, accountID)
	if err != nil || m.State != StateReady || m.Origin.String != OriginCreated {
		return
	}
	if err := s.queueProvision(ctx, provisionPayload{AccountID: accountID, Intent: intentSync, SeerrUser: int(m.SeerrUserID.Int64)}, jobs.ManualPriority); err != nil {
		s.log.WarnContext(ctx, "rights of seerr user not queued", "account_id", accountID, "err", err)
	}
}

// AccountCreated is the hook of package auth after a new account was committed: an invitation
// hands its wish limit to the account (data model 2.3), before the Seerr user is opened with it.
func (s *Service) AccountCreated(ctx context.Context, accountID int64) error {
	if err := s.write.CopyInvitationLimit(ctx, wishq.CopyInvitationLimitParams{Now: s.now().UnixMilli(), AccountID: sql.NullInt64{Int64: accountID, Valid: true}}); err != nil {
		return fmt.Errorf("limit of the invitation: %w", err)
	}
	return nil
}
