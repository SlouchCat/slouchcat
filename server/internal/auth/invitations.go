package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Invitations and guest accounts (K-6, K-7, AN-14). An invitation link carries a 256-bit token,
// shown once; the database keeps its SHA-256. A link is valid for 7 days and works once: taking it
// happens inside the transaction that creates the guest account.

// InvitationLifetime is how long an invitation link stays valid.
const InvitationLifetime = 7 * 24 * time.Hour

const maxInvitationNote = 120

// Invitation states.
const (
	InvitationOpen    = "open"
	InvitationUsed    = "used"
	InvitationExpired = "expired"
	InvitationRevoked = "revoked"
)

func invitationState(inv appq.Invitation, now time.Time) string {
	switch {
	case inv.UsedAt.Valid:
		return InvitationUsed
	case inv.RevokedAt.Valid:
		return InvitationRevoked
	case !time.UnixMilli(inv.ExpiresAt).After(now):
		return InvitationExpired
	default:
		return InvitationOpen
	}
}

// CreateInvitation creates a one-time invitation link for a guest account with the given limits
// and returns it with the plain token (the only time it is available). Errors: *ValidationError
// (maxRating, note, libraryIds unknown_library).
func (s *Service) CreateInvitation(ctx context.Context, creator model.Account, req model.InvitationRequest) (model.Invitation, error) {
	var verr *model.ValidationError
	if !slices.Contains(ratingLevels, req.MaxRating) {
		verr = verr.Add("maxRating", "invalid_value")
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxInvitationNote {
		verr = verr.Add("note", "too_long")
	}
	libs, verr, err := s.guestLibraries(ctx, s.read, req.AllLibraries, req.LibraryIDs, verr)
	if err != nil {
		return model.Invitation{}, err
	}
	if verr != nil {
		return model.Invitation{}, verr
	}
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return model.Invitation{}, fmt.Errorf("invitation token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Invitation{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	row, err := q.InsertInvitation(ctx, appq.InsertInvitationParams{
		TokenHash: hashToken(token), Note: sql.NullString{String: note, Valid: note != ""}, MaxRating: int64(req.MaxRating),
		AllLibraries: boolInt(req.AllLibraries), CreatedBy: sql.NullInt64{Int64: creator.ID, Valid: creator.ID != 0},
		Now: now.UnixMilli(), ExpiresAt: now.Add(InvitationLifetime).UnixMilli(),
	})
	if err != nil {
		return model.Invitation{}, fmt.Errorf("insert invitation: %w", err)
	}
	for _, id := range libs {
		if err := q.InsertInvitationLibrary(ctx, appq.InsertInvitationLibraryParams{InvitationID: row.ID, LibraryID: id}); err != nil {
			return model.Invitation{}, fmt.Errorf("insert invitation library: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Invitation{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "invitation created", "invitation_id", row.ID, "max_rating", req.MaxRating, "all_libraries", req.AllLibraries)
	inv := toInvitation(row, libs, "", now)
	inv.Token = token
	return inv, nil
}

// guestLibraries validates the library choice of an invitation or a guest account: with all
// libraries no rows; otherwise every id must be an existing library.
func (s *Service) guestLibraries(ctx context.Context, q *appq.Queries, all bool, ids []int64, verr *model.ValidationError) ([]int64, *model.ValidationError, error) {
	if all {
		return []int64{}, verr, nil
	}
	libs, err := q.ListLibraries(ctx)
	if err != nil {
		return nil, verr, fmt.Errorf("list libraries: %w", err)
	}
	known := map[int64]bool{}
	for _, l := range libs {
		known[l.ID] = true
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !known[id] {
			return nil, verr.Add("libraryIds", "unknown_library"), nil
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out, verr, nil
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func toInvitation(row appq.Invitation, libs []int64, usedByEmail string, now time.Time) model.Invitation {
	if libs == nil {
		libs = []int64{}
	}
	inv := model.Invitation{
		ID: row.ID, Note: row.Note.String, State: invitationState(row, now), MaxRating: int(row.MaxRating),
		AllLibraries: row.AllLibraries == 1, LibraryIDs: libs, CreatedAt: time.UnixMilli(row.CreatedAt),
		ExpiresAt: time.UnixMilli(row.ExpiresAt), UsedByEmail: usedByEmail,
	}
	if row.UsedAt.Valid {
		t := time.UnixMilli(row.UsedAt.Int64)
		inv.UsedAt = &t
	}
	return inv
}

// ListInvitations returns all invitations, newest first, with their state.
func (s *Service) ListInvitations(ctx context.Context) ([]model.Invitation, error) {
	rows, err := s.read.ListInvitations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	links, err := s.read.ListInvitationLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list invitation libraries: %w", err)
	}
	libs := map[int64][]int64{}
	for _, l := range links {
		libs[l.InvitationID] = append(libs[l.InvitationID], l.LibraryID)
	}
	now := s.now()
	out := make([]model.Invitation, 0, len(rows))
	for _, r := range rows {
		inv := appq.Invitation{
			ID: r.ID, TokenHash: r.TokenHash, Note: r.Note, MaxRating: r.MaxRating, AllLibraries: r.AllLibraries,
			CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, UsedAt: r.UsedAt, UsedBy: r.UsedBy, RevokedAt: r.RevokedAt,
		}
		out = append(out, toInvitation(inv, libs[r.ID], r.UsedByEmail.String, now))
	}
	return out, nil
}

// RevokeInvitation revokes an open invitation; a used or revoked one is left as it is (no error).
// An unknown id is model.ErrNotFound.
func (s *Service) RevokeInvitation(ctx context.Context, id int64) error {
	if _, err := s.read.GetInvitation(ctx, id); err != nil {
		return notFoundOr(err, "get invitation")
	}
	if _, err := s.write.RevokeInvitation(ctx, appq.RevokeInvitationParams{ID: id, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}}); err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	return nil
}

// openInvitation looks up an invitation by its plain token: model.ErrNotFound when unknown,
// model.ErrInvitationGone when used, expired or revoked.
func (s *Service) openInvitation(ctx context.Context, q *appq.Queries, token string) (appq.Invitation, error) {
	if len(token) != 43 {
		return appq.Invitation{}, model.ErrNotFound
	}
	inv, err := q.GetInvitationByTokenHash(ctx, hashToken(token))
	if err != nil {
		return appq.Invitation{}, notFoundOr(err, "get invitation")
	}
	if invitationState(inv, s.now()) != InvitationOpen {
		return appq.Invitation{}, model.ErrInvitationGone
	}
	return inv, nil
}

// GetInvitation returns what the invited person sees before accepting. It counts against the
// attempt limit of the client (model.ClientIP). Errors: model.ErrNotFound, model.ErrInvitationGone.
func (s *Service) GetInvitation(ctx context.Context, token string) (model.InvitationPreview, error) {
	if err := s.AllowAttempt(ctx); err != nil {
		return model.InvitationPreview{}, err
	}
	inv, err := s.openInvitation(ctx, s.read, token)
	if err != nil {
		return model.InvitationPreview{}, err
	}
	all, err := s.read.ListLibraries(ctx)
	if err != nil {
		return model.InvitationPreview{}, fmt.Errorf("list libraries: %w", err)
	}
	var allowed []int64
	if inv.AllLibraries != 1 {
		if allowed, err = s.read.ListInvitationLibraryIDs(ctx, inv.ID); err != nil {
			return model.InvitationPreview{}, fmt.Errorf("invitation libraries: %w", err)
		}
	}
	prev := model.InvitationPreview{
		Note: inv.Note.String, MaxRating: int(inv.MaxRating), AllLibraries: inv.AllLibraries == 1,
		Libraries: []model.LibraryRef{}, ExpiresAt: time.UnixMilli(inv.ExpiresAt),
	}
	for _, l := range all {
		if inv.AllLibraries == 1 || slices.Contains(allowed, l.ID) {
			prev.Libraries = append(prev.Libraries, model.LibraryRef{ID: l.ID, Name: l.Name})
		}
	}
	return prev, nil
}

// AcceptInvitation creates the guest account of an invitation with its primary profile, takes the
// link and starts a session with that profile. The link is taken inside the transaction, so of two
// concurrent acceptances one gets model.ErrInvitationGone. Errors also: model.ErrNotFound,
// *model.ValidationError (email already_exists, password, profileName, language).
func (s *Service) AcceptInvitation(ctx context.Context, in model.AcceptInvitation, userAgent string) (model.Session, string, error) {
	if err := s.AllowAttempt(ctx); err != nil {
		return model.Session{}, "", err
	}
	if _, err := s.openInvitation(ctx, s.read, in.Token); err != nil {
		return model.Session{}, "", err
	}
	email, verr := ValidateOwner(in.Email, in.Password)
	name := strings.TrimSpace(in.ProfileName)
	switch {
	case name == "":
		verr = verr.Add("profileName", "required")
	case utf8.RuneCountInString(name) > maxProfileName:
		verr = verr.Add("profileName", "too_long")
	}
	lang := in.Language
	if lang == "" {
		lang = "de"
	}
	if lang != "de" && lang != "en" {
		verr = verr.Add("language", "invalid_value")
	}
	if verr != nil {
		return model.Session{}, "", verr
	}
	release, err := acquireArgon(ctx)
	if err != nil {
		return model.Session{}, "", err
	}
	hash, err := hashPassword(in.Password, s.params)
	release()
	if err != nil {
		return model.Session{}, "", err
	}
	now := s.now().UnixMilli()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Session{}, "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	inv, err := s.openInvitation(ctx, q, in.Token)
	if err != nil {
		return model.Session{}, "", err
	}
	if _, err := q.GetAccountByEmail(ctx, email); err == nil {
		return model.Session{}, "", (&model.ValidationError{}).Add("email", "already_exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return model.Session{}, "", fmt.Errorf("get account: %w", err)
	}
	acc, err := q.InsertGuestAccount(ctx, appq.InsertGuestAccountParams{
		Email: email, PasswordHash: hash, MaxRating: inv.MaxRating, AllLibraries: inv.AllLibraries,
		InvitedBy: inv.CreatedBy, Now: now,
	})
	if err != nil {
		return model.Session{}, "", fmt.Errorf("insert guest account: %w", err)
	}
	taken, err := q.MarkInvitationUsed(ctx, appq.MarkInvitationUsedParams{ID: inv.ID, UsedBy: sql.NullInt64{Int64: acc.ID, Valid: true}, Now: sql.NullInt64{Int64: now, Valid: true}})
	if err != nil {
		return model.Session{}, "", fmt.Errorf("take invitation: %w", err)
	}
	if taken == 0 {
		return model.Session{}, "", model.ErrInvitationGone
	}
	var libs []int64
	if inv.AllLibraries != 1 {
		if libs, err = q.ListInvitationLibraryIDs(ctx, inv.ID); err != nil {
			return model.Session{}, "", fmt.Errorf("invitation libraries: %w", err)
		}
		for _, id := range libs {
			if err := q.InsertAccountLibrary(ctx, appq.InsertAccountLibraryParams{AccountID: acc.ID, LibraryID: id}); err != nil {
				return model.Session{}, "", fmt.Errorf("insert account library: %w", err)
			}
		}
	}
	prof, err := q.InsertProfile(ctx, appq.InsertProfileParams{
		AccountID: acc.ID, Name: name, MaxRating: inv.MaxRating, Language: lang, AllLibraries: 1, IsDefault: 1, Now: now,
	})
	if err != nil {
		return model.Session{}, "", fmt.Errorf("insert guest profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Session{}, "", fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "guest account created", "account_id", acc.ID, "invitation_id", inv.ID, "max_rating", inv.MaxRating)
	return s.StartSession(ctx, toAccount(acc, libs), toProfile(prof, nil), userAgent)
}
