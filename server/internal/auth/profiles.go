package auth

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Profile management (K-2, M2 design section 3). Only the account's primary profile manages
// profiles; the checks here are the authority, the API maps model.ErrPrimaryProfileRequired to 403.

// MaxProfiles is the number of profiles one account may have.
const MaxProfiles = 10

// Accents are the accent colour tokens of the design system (packages/design-tokens
// tokens/color.json); the API schema Accent lists the same names.
var Accents = []string{"ember", "coral", "amber", "lime", "teal", "sky", "violet", "rose"}

var ratingLevels = []int{0, 6, 12, 16, 18}

func requirePrimary(session model.Session) error {
	if !session.IsPrimary() {
		return model.ErrPrimaryProfileRequired
	}
	return nil
}

// ListAccountLibraries returns the libraries the account may see, ordered by name: all of them for
// an account with all libraries, otherwise its own. It needs no profile.
func (s *Service) ListAccountLibraries(ctx context.Context, account model.Account) ([]model.LibraryRef, error) {
	rows, err := s.read.ListLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	out := make([]model.LibraryRef, 0, len(rows))
	for _, l := range rows {
		if account.AllLibraries || slices.Contains(account.LibraryIDs, l.ID) {
			out = append(out, model.LibraryRef{ID: l.ID, Name: l.Name, Kind: l.Kind})
		}
	}
	slices.SortStableFunc(out, func(a, b model.LibraryRef) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

// ListMyProfiles returns the profiles of the account for the picker, primary profile first. It
// works without an active profile.
func (s *Service) ListMyProfiles(ctx context.Context, accountID int64) ([]model.Profile, error) {
	return s.profilesOf(ctx, s.read, accountID)
}

// ListProfiles returns the profiles of the account with their settings; the primary profile must
// be active.
func (s *Service) ListProfiles(ctx context.Context, session model.Session) ([]model.Profile, error) {
	if err := requirePrimary(session); err != nil {
		return nil, err
	}
	return s.profilesOf(ctx, s.read, session.Account.ID)
}

func (s *Service) profilesOf(ctx context.Context, q *appq.Queries, accountID int64) ([]model.Profile, error) {
	rows, err := q.ListProfilesByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	libs, err := q.ListProfileLibrariesByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list profile libraries: %w", err)
	}
	acc, err := q.GetAccount(ctx, accountID)
	if err != nil {
		return nil, notFoundOr(err, "get account")
	}
	caps := AccountLimits(acc)
	byProfile := map[int64][]int64{}
	for _, l := range libs {
		byProfile[l.ProfileID] = append(byProfile[l.ProfileID], l.LibraryID)
	}
	out := make([]model.Profile, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProfile(r, byProfile[r.ID], caps))
	}
	return out, nil
}

// CreateProfile adds a profile to the account. Errors: model.ErrPrimaryProfileRequired and a
// *model.ValidationError (name, maxRating above_account_limit, language, accent, libraryIds
// unknown_library, pin, name too_many_profiles).
func (s *Service) CreateProfile(ctx context.Context, session model.Session, in model.ProfileInput) (model.Profile, error) {
	if err := requirePrimary(session); err != nil {
		return model.Profile{}, err
	}
	acc := session.Account
	p := appq.InsertProfileParams{AccountID: acc.ID, MaxRating: int64(min(18, acc.MaxRating)), Language: session.Profile.Language, AllLibraries: 1}
	verr := applyInput(acc, &p.Name, &p.MaxRating, &p.Language, &p.Accent, &p.AllLibraries, in)
	libs, verr, err := s.libraryChoice(ctx, acc, p.AllLibraries == 1, in.LibraryIDs, verr)
	if err != nil {
		return model.Profile{}, err
	}
	if in.Name == nil {
		verr = verr.Add("name", "required")
	}
	if in.PIN != nil && !ValidPIN(*in.PIN) {
		verr = verr.Add("pin", "invalid_pin")
	}
	if in.Limits != nil {
		verr = validateLimits(*in.Limits, true, verr)
		p.MaxBitrateKbps, p.MaxHeight, p.MaxStreams, p.WatchLimitMinutes = limitColumns(*in.Limits)
	}
	if p.AllLibraries == 0 && len(libs) == 0 && !hasField(verr, "libraryIds") {
		verr = verr.Add("libraryIds", "required")
	}
	if verr != nil {
		return model.Profile{}, verr
	}
	if in.PIN != nil {
		hash, err := s.hashPIN(ctx, *in.PIN)
		if err != nil {
			return model.Profile{}, err
		}
		p.PinHash = sql.NullString{String: hash, Valid: true}
	}
	p.Now = s.now().UnixMilli()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Profile{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	// The limit is checked inside the write transaction, so two concurrent creations cannot both
	// take the last slot.
	n, err := q.CountProfilesByAccount(ctx, acc.ID)
	if err != nil {
		return model.Profile{}, fmt.Errorf("count profiles: %w", err)
	}
	if n >= MaxProfiles {
		return model.Profile{}, (&model.ValidationError{}).Add("name", "too_many_profiles")
	}
	row, err := q.InsertProfile(ctx, p)
	if err != nil {
		return model.Profile{}, fmt.Errorf("insert profile: %w", err)
	}
	for _, id := range libs {
		if err := q.InsertProfileLibrary(ctx, appq.InsertProfileLibraryParams{ProfileID: row.ID, LibraryID: id}); err != nil {
			return model.Profile{}, fmt.Errorf("insert profile library: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Profile{}, fmt.Errorf("commit: %w", err)
	}
	return toProfile(row, libs, acc.Limits), nil
}

// UpdateProfile changes the sent fields of a profile of the account (model.ErrNotFound for any
// other); errors as for CreateProfile.
func (s *Service) UpdateProfile(ctx context.Context, session model.Session, id int64, in model.ProfileInput) (model.Profile, error) {
	if err := requirePrimary(session); err != nil {
		return model.Profile{}, err
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: id, AccountID: session.Account.ID})
	if err != nil {
		return model.Profile{}, notFoundOr(err, "get profile")
	}
	current, err := s.profileModel(ctx, s.read, row, session.Account)
	if err != nil {
		return model.Profile{}, err
	}
	acc := session.Account
	p := appq.UpdateProfileParams{
		ID: row.ID, Name: row.Name, MaxRating: row.MaxRating, Language: row.Language, Accent: row.Accent, AllLibraries: row.AllLibraries,
		MaxBitrateKbps: row.MaxBitrateKbps, MaxHeight: row.MaxHeight, MaxStreams: row.MaxStreams, WatchLimitMinutes: row.WatchLimitMinutes,
	}
	verr := applyInput(acc, &p.Name, &p.MaxRating, &p.Language, &p.Accent, &p.AllLibraries, in)
	if in.Limits != nil {
		verr = validateLimits(*in.Limits, true, verr)
		p.MaxBitrateKbps, p.MaxHeight, p.MaxStreams, p.WatchLimitMinutes = limitColumns(*in.Limits)
	}
	// A profile with all libraries keeps no rows; library ids that are sent are checked against
	// the libraries and the account's rights, otherwise a restricted profile keeps its rights.
	libs := current.LibraryIDs
	if p.AllLibraries == 1 || in.LibraryIDs != nil {
		if libs, verr, err = s.libraryChoice(ctx, acc, p.AllLibraries == 1, in.LibraryIDs, verr); err != nil {
			return model.Profile{}, err
		}
	}
	if p.AllLibraries == 0 && len(libs) == 0 && !hasField(verr, "libraryIds") {
		verr = verr.Add("libraryIds", "required")
	}
	if verr != nil {
		return model.Profile{}, verr
	}
	p.Now = s.now().UnixMilli()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Profile{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	if err := q.UpdateProfile(ctx, p); err != nil {
		return model.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	if err := q.DeleteProfileLibraries(ctx, row.ID); err != nil {
		return model.Profile{}, fmt.Errorf("replace profile libraries: %w", err)
	}
	for _, lib := range libs {
		if err := q.InsertProfileLibrary(ctx, appq.InsertProfileLibraryParams{ProfileID: row.ID, LibraryID: lib}); err != nil {
			return model.Profile{}, fmt.Errorf("insert profile library: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Profile{}, fmt.Errorf("commit: %w", err)
	}
	row.Name, row.MaxRating, row.Language, row.Accent, row.AllLibraries = p.Name, p.MaxRating, p.Language, p.Accent, p.AllLibraries
	row.MaxBitrateKbps, row.MaxHeight, row.MaxStreams, row.WatchLimitMinutes = p.MaxBitrateKbps, p.MaxHeight, p.MaxStreams, p.WatchLimitMinutes
	return toProfile(row, libs, acc.Limits), nil
}

// applyInput validates the scalar fields of in and writes the accepted values through the
// pointers.
func applyInput(acc model.Account, name *string, maxRating *int64, language *string,
	accent *sql.NullString, allLibraries *int64, in model.ProfileInput,
) *model.ValidationError {
	var verr *model.ValidationError
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		switch {
		case n == "":
			verr = verr.Add("name", "required")
		case utf8.RuneCountInString(n) > maxProfileName:
			verr = verr.Add("name", "too_long")
		default:
			*name = n
		}
	}
	if in.MaxRating != nil {
		switch {
		case !slices.Contains(ratingLevels, *in.MaxRating):
			verr = verr.Add("maxRating", "invalid_value")
		case *in.MaxRating > acc.MaxRating:
			verr = verr.Add("maxRating", "above_account_limit")
		default:
			*maxRating = int64(*in.MaxRating)
		}
	}
	if in.Language != nil {
		if *in.Language != "de" && *in.Language != "en" {
			verr = verr.Add("language", "invalid_value")
		} else {
			*language = *in.Language
		}
	}
	if in.Accent != nil {
		if !slices.Contains(Accents, *in.Accent) {
			verr = verr.Add("accent", "invalid_value")
		} else {
			*accent = sql.NullString{String: *in.Accent, Valid: true}
		}
	}
	if in.AllLibraries != nil {
		*allLibraries = 0
		if *in.AllLibraries {
			*allLibraries = 1
		}
	}
	return verr
}

// Bounds of the W-8 and K-5 limits (contract schemas BitrateKbps, OutputHeight, StreamCount,
// WatchMinutes and the CHECK constraints of migration 00011).
const (
	minBitrateKbps    = 200
	maxBitrateKbps    = 200_000
	maxStreamCount    = 16
	maxWatchLimitMins = 1440
)

var outputHeights = []int{360, 480, 720, 1080, 2160}

// validateLimits checks limits (zero = no limit); withWatch is false for the caps of a guest
// account, which ignore the watch-time limit. Field names are "limits.<field>".
func validateLimits(l model.PlaybackLimits, withWatch bool, verr *model.ValidationError) *model.ValidationError {
	switch {
	case l.MaxBitrateKbps < 0, l.MaxBitrateKbps > 0 && l.MaxBitrateKbps < minBitrateKbps:
		verr = verr.Add("limits.maxBitrateKbps", "too_small")
	case l.MaxBitrateKbps > maxBitrateKbps:
		verr = verr.Add("limits.maxBitrateKbps", "too_large")
	}
	if l.MaxHeight != 0 && !slices.Contains(outputHeights, l.MaxHeight) {
		verr = verr.Add("limits.maxHeight", "invalid_value")
	}
	switch {
	case l.MaxStreams < 0:
		verr = verr.Add("limits.maxStreams", "too_small")
	case l.MaxStreams > maxStreamCount:
		verr = verr.Add("limits.maxStreams", "too_large")
	}
	if withWatch {
		switch {
		case l.WatchLimitMinutes < 0:
			verr = verr.Add("limits.watchLimitMinutes", "too_small")
		case l.WatchLimitMinutes > maxWatchLimitMins:
			verr = verr.Add("limits.watchLimitMinutes", "too_large")
		}
	}
	return verr
}

// limitColumns maps limits to nullable columns (zero = NULL = no limit).
func limitColumns(l model.PlaybackLimits) (bitrate, height, streams, watch sql.NullInt64) {
	col := func(v int) sql.NullInt64 { return sql.NullInt64{Int64: int64(v), Valid: v > 0} }
	return col(l.MaxBitrateKbps), col(l.MaxHeight), col(l.MaxStreams), col(l.WatchLimitMinutes)
}

// hasField reports whether verr already has an error for field.
func hasField(verr *model.ValidationError, field string) bool {
	if verr == nil {
		return false
	}
	return slices.ContainsFunc(verr.Fields, func(f model.FieldError) bool { return f.Field == field })
}

// libraryChoice validates the library rights of a profile: every id must exist and, for a
// restricted (guest) account, lie within the account's libraries. A profile with all libraries
// keeps no rows.
func (s *Service) libraryChoice(ctx context.Context, acc model.Account, all bool, ids *[]int64, verr *model.ValidationError) ([]int64, *model.ValidationError, error) {
	if all || ids == nil {
		return []int64{}, verr, nil
	}
	libs, err := s.read.ListLibraries(ctx)
	if err != nil {
		return nil, verr, fmt.Errorf("list libraries: %w", err)
	}
	known := map[int64]bool{}
	for _, l := range libs {
		known[l.ID] = true
	}
	out := make([]int64, 0, len(*ids))
	for _, id := range *ids {
		if !known[id] || (!acc.AllLibraries && !slices.Contains(acc.LibraryIDs, id)) {
			return nil, verr.Add("libraryIds", "unknown_library"), nil
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out, verr, nil
}

// SetProfilePIN sets or replaces the PIN of a profile of the account and clears its lock counters.
func (s *Service) SetProfilePIN(ctx context.Context, session model.Session, id int64, pin string) error {
	if err := requirePrimary(session); err != nil {
		return err
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: id, AccountID: session.Account.ID})
	if err != nil {
		return notFoundOr(err, "get profile")
	}
	if !ValidPIN(pin) {
		return (&model.ValidationError{}).Add("pin", "invalid_pin")
	}
	hash, err := s.hashPIN(ctx, pin)
	if err != nil {
		return err
	}
	return s.storePIN(ctx, row.ID, sql.NullString{String: hash, Valid: true})
}

// RemoveProfilePIN removes the PIN of a profile of the account; a profile without PIN is fine.
func (s *Service) RemoveProfilePIN(ctx context.Context, session model.Session, id int64) error {
	if err := requirePrimary(session); err != nil {
		return err
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: id, AccountID: session.Account.ID})
	if err != nil {
		return notFoundOr(err, "get profile")
	}
	return s.storePIN(ctx, row.ID, sql.NullString{})
}

func (s *Service) storePIN(ctx context.Context, profileID int64, hash sql.NullString) error {
	s.pinMu.Lock()
	defer s.pinMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	if err := q.SetProfilePin(ctx, appq.SetProfilePinParams{ID: profileID, PinHash: hash, Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("set pin: %w", err)
	}
	if err := q.DeletePinAttemptsByProfile(ctx, profileID); err != nil {
		return fmt.Errorf("reset pin attempts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// DeleteProfile deletes a profile of the account with its progress. The primary profile returns
// model.ErrPrimaryProfile. end runs after the checks and before the delete (the API ends the
// profile's playback there), so a refused delete leaves running streams alone; sessions that had
// the profile active fall back to the picker.
func (s *Service) DeleteProfile(ctx context.Context, session model.Session, id int64, end func(ctx context.Context, profileID int64) error) error {
	if err := requirePrimary(session); err != nil {
		return err
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: id, AccountID: session.Account.ID})
	if err != nil {
		return notFoundOr(err, "get profile")
	}
	if row.IsDefault == 1 {
		return model.ErrPrimaryProfile
	}
	// Order matters: first the sessions drop the profile (own committed statement: their next
	// request is a session without profile), then the profile's streams end, then it is deleted.
	if _, err := s.write.ClearSessionsProfile(ctx, nullInt(row.ID)); err != nil {
		return fmt.Errorf("detach sessions: %w", err)
	}
	if end != nil {
		if err := end(ctx, row.ID); err != nil {
			return err
		}
	}
	if err := s.write.DeleteProfile(ctx, row.ID); err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	if end != nil { // a stream opened by a request that was already past its checks
		if err := end(ctx, row.ID); err != nil {
			return err
		}
	}
	s.log.InfoContext(ctx, "profile deleted", "account_id", session.Account.ID, "profile_id", row.ID)
	return nil
}
