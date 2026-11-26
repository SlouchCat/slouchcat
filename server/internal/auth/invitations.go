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
// (maxRating, note, libraryIds unknown_library, canRequest age_restricted).
func (s *Service) CreateInvitation(ctx context.Context, creator model.Account, req model.InvitationRequest) (model.Invitation, error) {
	rs, err := s.RatingSystem(ctx)
	if err != nil {
		return model.Invitation{}, err
	}
	var verr *model.ValidationError
	if !rs.HasAge(req.MaxRating) {
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
	if !req.AllLibraries && len(libs) == 0 && !hasField(verr, "libraryIds") {
		verr = verr.Add("libraryIds", "required")
	}
	wl, verr := invitationWishLimit(req.WishLimit, verr)
	if verr != nil {
		return model.Invitation{}, verr
	}
	// «Neue Konten dürfen anfordern», on by default, and only without an age limit (question 36,
	// ADR 0045); a right sent for an invitation with an age limit is refused.
	canRequest := true
	if req.CanRequest != nil {
		canRequest = *req.CanRequest
	} else if v, err := s.read.GetSetting(ctx, settingNewAccountsCanRequest); err == nil {
		canRequest = v != "0"
	} else if !errors.Is(err, sql.ErrNoRows) {
		return model.Invitation{}, fmt.Errorf("read %s: %w", settingNewAccountsCanRequest, err)
	}
	if req.MaxRating < model.MaxRatingAge {
		if req.CanRequest != nil && *req.CanRequest {
			return model.Invitation{}, (&model.ValidationError{}).Add("canRequest", model.WishReasonAgeRestricted)
		}
		canRequest = false
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
		CanRequest: boolInt(canRequest), WishLimitMode: wl.Mode,
		WishMovieLimit: limitPart(wl.Movie, true), WishMovieDays: limitPart(wl.Movie, false),
		WishTvLimit: limitPart(wl.TV, true), WishTvDays: limitPart(wl.TV, false),
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

// settingNewAccountsCanRequest is the setting of package admin «Neue Konten dürfen anfordern».
const settingNewAccountsCanRequest = "wishes.new_accounts_can_request"

// invitationWishLimit checks the wish limit of an invitation (concept 25, data model 2.3): mode
// default (also for nil), custom with both halves (limit 0 to 100, days 1 to 90) or unlimited.
func invitationWishLimit(in *model.InvitationWishLimit, verr *model.ValidationError) (model.InvitationWishLimit, *model.ValidationError) {
	if in == nil {
		return model.InvitationWishLimit{Mode: model.WishLimitDefault}, verr
	}
	check := func(field string, l *model.WishLimit) {
		switch {
		case l == nil:
			verr = verr.Add(field, "required")
		case l.Limit < 0 || l.Limit > 100:
			verr = verr.Add(field+".limit", "out_of_range")
		case l.Days < 1 || l.Days > 90:
			verr = verr.Add(field+".days", "out_of_range")
		}
	}
	switch in.Mode {
	case model.WishLimitCustom:
		check("wishLimit.movie", in.Movie)
		check("wishLimit.tv", in.TV)
		return *in, verr
	case model.WishLimitDefault, model.WishLimitUnlimited:
		if in.Movie != nil || in.TV != nil {
			verr = verr.Add("wishLimit", "invalid_value")
		}
		return model.InvitationWishLimit{Mode: in.Mode}, verr
	}
	return model.InvitationWishLimit{Mode: model.WishLimitDefault}, verr.Add("wishLimit.mode", "invalid_value")
}

func limitPart(l *model.WishLimit, limit bool) sql.NullInt64 {
	switch {
	case l == nil:
		return sql.NullInt64{}
	case limit:
		return sql.NullInt64{Int64: int64(l.Limit), Valid: true}
	}
	return sql.NullInt64{Int64: int64(l.Days), Valid: true}
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
		CanRequest: row.CanRequest == 1, WishLimit: model.InvitationWishLimit{Mode: row.WishLimitMode},
	}
	if row.WishLimitMode == model.WishLimitCustom {
		inv.WishLimit.Movie = &model.WishLimit{Limit: int(row.WishMovieLimit.Int64), Days: int(row.WishMovieDays.Int64)}
		inv.WishLimit.TV = &model.WishLimit{Limit: int(row.WishTvLimit.Int64), Days: int(row.WishTvDays.Int64)}
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
		lang = model.DefaultUILanguage
	}
	if !model.IsUILanguage(lang) {
		verr = verr.Add("language", "invalid_value")
	}
	if verr != nil {
		return model.Session{}, "", verr
	}
	release, err := s.acquireArgon(ctx)
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
	// By design (accepted): a valid link lets its holder learn whether an e-mail address already has
	// an account (422 email already_exists). The link is secret, one-time and rate limited, and the
	// guest needs the answer to pick another address.
	if _, err := q.GetAccountByEmail(ctx, email); err == nil {
		return model.Session{}, "", (&model.ValidationError{}).Add("email", "already_exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return model.Session{}, "", fmt.Errorf("get account: %w", err)
	}
	acc, err := q.InsertInvitedAccount(ctx, appq.InsertInvitedAccountParams{
		Email: email, PasswordHash: hash, MaxRating: inv.MaxRating, AllLibraries: inv.AllLibraries,
		CanRequest: inv.CanRequest, InvitedBy: inv.CreatedBy, Now: now,
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
	audio, mode, subs := prefColumns(initialPrefs(lang))
	prof, err := q.InsertProfile(ctx, appq.InsertProfileParams{
		AccountID: acc.ID, Name: name, MaxRating: inv.MaxRating, Language: lang, AllLibraries: 1, IsDefault: 1,
		AudioLanguages: audio, SubtitleMode: mode, SubtitleLanguages: subs, Now: now,
	})
	if err != nil {
		return model.Session{}, "", fmt.Errorf("insert guest profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Session{}, "", fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "guest account created", "account_id", acc.ID, "invitation_id", inv.ID, "max_rating", inv.MaxRating)
	s.created(ctx, acc.ID)
	if s.invitationAccepted != nil && inv.CreatedBy.Valid {
		if err := s.invitationAccepted(ctx, inv.CreatedBy.Int64, email); err != nil {
			s.log.WarnContext(ctx, "invitation accepted hook failed", "invitation_id", inv.ID, "err", err)
		}
	}
	s.seen(ctx, acc.ID, email, in.Password, PasswordMomentInvitation)
	account := toAccount(acc, libs)
	return s.StartSession(ctx, account, toProfile(prof, nil, account.Limits), userAgent)
}
