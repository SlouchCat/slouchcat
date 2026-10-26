package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// maxPickNote caps the personal note of a pick (contract PickRequest.note).
const maxPickNote = 280

// ListTopPicks returns every pick, global ones and those of single profiles, in carousel order
// (E-2). Titles are plain text, never an ItemRef; the poster is the content hash of the image
// that getAdminItemImage serves.
func (s *Service) ListTopPicks(ctx context.Context) ([]model.AdminPick, error) {
	rows, err := s.sys.AdminListTopPicks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list top picks: %w", err)
	}
	out := make([]model.AdminPick, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAdminPick(systemq.AdminGetTopPickRow(r)))
	}
	if err := withPosters(ctx, s.sys, out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateTopPicks adds a title to Cat's recommendations: one global pick without ProfileIDs, else
// one pick per profile of the main account, at the end of the carousel. The title must exist and
// must not be a season or an extra; the time window must end after it starts.
func (s *Service) CreateTopPicks(ctx context.Context, req model.PickRequest) ([]model.AdminPick, error) {
	// The checks run inside the write transaction: a title merged away (hard-deleted) in between
	// would otherwise fail the insert on its foreign key (500 instead of 422).
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	verr, err := s.checkCurationItem(ctx, q, "itemId", req.ItemID, "season", "extra")
	if err != nil {
		return nil, err
	}
	profiles := slices.Clone(req.ProfileIDs)
	slices.Sort(profiles)
	profiles = slices.Compact(profiles)
	if len(profiles) > 0 {
		owner, err := q.AdminOwnerProfileIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("list owner profiles: %w", err)
		}
		for _, id := range profiles {
			if !slices.Contains(owner, id) {
				verr = verr.Add("profileIds", "invalid_value")
				break
			}
		}
	}
	verr = validatePick(verr, req.Note, req.StartsAt, req.EndsAt)
	if verr != nil {
		return nil, verr
	}

	pos, err := q.AdminNextTopPickPosition(ctx)
	if err != nil {
		return nil, fmt.Errorf("next pick position: %w", err)
	}
	targets := []sql.NullInt64{{}}
	if len(profiles) > 0 {
		targets = targets[:0]
		for _, id := range profiles {
			targets = append(targets, sql.NullInt64{Int64: id, Valid: true})
		}
	}
	now := s.now().UnixMilli()
	ids := make([]int64, 0, len(targets))
	for i, target := range targets {
		id, err := q.AdminInsertTopPick(ctx, systemq.AdminInsertTopPickParams{
			ItemID: req.ItemID, ProfileID: target, Position: pos + int64(i), Note: nullText(req.Note),
			StartsAt: nullMillis(req.StartsAt), EndsAt: nullMillis(req.EndsAt), Now: now,
		})
		if err != nil {
			return nil, fmt.Errorf("insert top pick: %w", err)
		}
		ids = append(ids, id)
	}
	out := make([]model.AdminPick, 0, len(ids))
	for _, id := range ids {
		row, err := q.AdminGetTopPick(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get top pick %d: %w", id, err)
		}
		out = append(out, toAdminPick(row))
	}
	if err := withPosters(ctx, q, out); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "top picks created", "item_id", req.ItemID, "picks", len(out))
	return out, nil
}

// UpdateTopPick changes note and time window of a pick: a nil field keeps its value, a Clear flag
// (JSON null) clears it. model.ErrNotFound for an unknown pick.
func (s *Service) UpdateTopPick(ctx context.Context, id int64, upd model.PickUpdate) (model.AdminPick, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.AdminPick{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	cur, err := q.AdminGetTopPick(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AdminPick{}, model.ErrNotFound
	}
	if err != nil {
		return model.AdminPick{}, fmt.Errorf("get top pick %d: %w", id, err)
	}
	pick := toAdminPick(cur)
	switch {
	case upd.ClearNote:
		pick.Note = ""
	case upd.Note != nil:
		pick.Note = *upd.Note
	}
	switch {
	case upd.ClearStartsAt:
		pick.StartsAt = nil
	case upd.StartsAt != nil:
		pick.StartsAt = upd.StartsAt
	}
	switch {
	case upd.ClearEndsAt:
		pick.EndsAt = nil
	case upd.EndsAt != nil:
		pick.EndsAt = upd.EndsAt
	}
	if verr := validatePick(nil, pick.Note, pick.StartsAt, pick.EndsAt); verr != nil {
		return model.AdminPick{}, verr
	}
	if _, err := q.AdminUpdateTopPick(ctx, systemq.AdminUpdateTopPickParams{
		ID: id, Note: nullText(pick.Note), StartsAt: nullMillis(pick.StartsAt), EndsAt: nullMillis(pick.EndsAt),
	}); err != nil {
		return model.AdminPick{}, fmt.Errorf("update top pick %d: %w", id, err)
	}
	row, err := q.AdminGetTopPick(ctx, id)
	if err != nil {
		return model.AdminPick{}, fmt.Errorf("get top pick %d: %w", id, err)
	}
	out := []model.AdminPick{toAdminPick(row)}
	if err := withPosters(ctx, q, out); err != nil {
		return model.AdminPick{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.AdminPick{}, fmt.Errorf("commit: %w", err)
	}
	return out[0], nil
}

// DeleteTopPick removes a pick; model.ErrNotFound for an unknown one.
func (s *Service) DeleteTopPick(ctx context.Context, id int64) error {
	n, err := systemq.New(s.db).AdminDeleteTopPick(ctx, id)
	if err != nil {
		return fmt.Errorf("delete top pick %d: %w", id, err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// ReorderTopPicks stores the carousel order: ids must name every existing pick exactly once
// (else 422 ids invalid_value); positions become 0..n-1 in one transaction.
func (s *Service) ReorderTopPicks(ctx context.Context, ids []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	existing, err := q.AdminListTopPickIDs(ctx)
	if err != nil {
		return fmt.Errorf("list top picks: %w", err)
	}
	given := slices.Clone(ids)
	slices.Sort(given)
	if !slices.Equal(given, existing) {
		return (&model.ValidationError{}).Add("ids", "invalid_value")
	}
	for i, id := range ids {
		if err := q.AdminSetTopPickPosition(ctx, systemq.AdminSetTopPickPositionParams{ID: id, Position: int64(i)}); err != nil {
			return fmt.Errorf("set position of pick %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// validatePick checks the note length and that the time window ends after it starts.
func validatePick(verr *model.ValidationError, note string, starts, ends *time.Time) *model.ValidationError {
	if utf8.RuneCountInString(note) > maxPickNote {
		verr = verr.Add("note", "too_long")
	}
	if starts != nil && ends != nil && !ends.After(*starts) {
		verr = verr.Add("endsAt", "invalid_value")
	}
	return verr
}

// checkCurationItem checks a title for a pick with the queries of the caller's transaction: field
// gets unknown_item for an unknown or deleted title and invalid_value for one of the refused kinds.
func (s *Service) checkCurationItem(ctx context.Context, q *systemq.Queries, field, itemID string, refused ...string) (*model.ValidationError, error) {
	if itemID == "" {
		return (&model.ValidationError{}).Add(field, "required"), nil
	}
	it, err := q.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && it.DeletedAt.Valid) {
		return (&model.ValidationError{}).Add(field, "unknown_item"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get item %s: %w", itemID, err)
	}
	if slices.Contains(refused, it.Kind) {
		return (&model.ValidationError{}).Add(field, "invalid_value"), nil
	}
	return nil, nil
}

func toAdminPick(r systemq.AdminGetTopPickRow) model.AdminPick {
	p := model.AdminPick{
		ID: r.ID, ItemID: r.ItemID, Kind: r.Kind, Title: r.Title, Year: optInt(r.Year), ProfileName: r.ProfileName,
		Position: int(r.Position), Note: r.Note.String, StartsAt: optTime(r.StartsAt), EndsAt: optTime(r.EndsAt),
		CreatedAt: time.UnixMilli(r.CreatedAt).UTC(),
	}
	if r.ProfileID.Valid {
		id := r.ProfileID.Int64
		p.ProfileID = &id
	}
	return p
}

func optInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func optTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.UnixMilli(v.Int64).UTC()
	return &t
}

func nullMillis(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixMilli(), Valid: true}
}

func nullText(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
