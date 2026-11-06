package admin

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// HomeRows returns every start page row in display order with its hidden flag (E-9): the eight
// fixed rows, «Meine Medien» and one row «Neu in …» per library (concept 15 P-7).
func (s *Service) HomeRows(ctx context.Context) ([]model.HomeRowSetting, error) {
	return homeRows(ctx, s.read)
}

// SetHomeRows stores the order and visibility of the start page rows by id (concept 15 P-7). The
// slice order is the display order; an empty slice is 422 rows invalid_value. A stored row the slice leaves out (a library added after the
// page was opened) keeps its place in the stored order and its hidden flag. An unknown id, an id
// twice or a key that does not match the stored row is 422 rows invalid_value; more than
// model.MaxVisibleHomeRows rows not hidden is 422 rows too_many_visible (F-12). Positions become
// 0..n-1 in one transaction. The start page reads the order on every request, so there is nothing
// to invalidate.
func (s *Service) SetHomeRows(ctx context.Context, rows []model.HomeRowSetting) ([]model.HomeRowSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	stored, err := homeRows(ctx, q)
	if err != nil {
		return nil, err
	}
	order, err := mergeHomeRows(stored, rows)
	if err != nil {
		return nil, err
	}
	now := s.now().UnixMilli()
	for i, r := range order {
		hidden := int64(0)
		if r.Hidden {
			hidden = 1
		}
		n, err := q.SetHomeRow(ctx, appq.SetHomeRowParams{ID: r.ID, Position: int64(i), Hidden: hidden, Now: now})
		if err != nil {
			return nil, fmt.Errorf("set home row %s: %w", r.ID, err)
		}
		if n != 1 {
			return nil, fmt.Errorf("home row %s missing from home_rows", r.ID)
		}
	}
	out, err := homeRows(ctx, q)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "home rows set")
	return out, nil
}

// mergeHomeRows returns the stored rows in the order of rows, with their hidden flags from rows.
// A stored row that rows leaves out goes back to its index in the stored order, earliest index
// first, and keeps its flag.
func mergeHomeRows(stored, rows []model.HomeRowSetting) ([]model.HomeRowSetting, error) {
	invalid := (&model.ValidationError{}).Add("rows", "invalid_value")
	if len(rows) == 0 {
		return nil, invalid
	}
	byID := make(map[string]model.HomeRowSetting, len(stored))
	for _, r := range stored {
		byID[r.ID] = r
	}
	given := make(map[string]bool, len(rows))
	order := make([]model.HomeRowSetting, 0, len(stored))
	for _, r := range rows {
		row, ok := byID[r.ID]
		if !ok || given[r.ID] || row.Key != r.Key {
			return nil, invalid
		}
		given[r.ID] = true
		row.Hidden = r.Hidden
		order = append(order, row)
	}
	for i, r := range stored { // stored is in display order
		if !given[r.ID] {
			order = slices.Insert(order, min(i, len(order)), r)
		}
	}
	visible := 0
	for _, r := range order {
		if !r.Hidden {
			visible++
		}
	}
	if visible > model.MaxVisibleHomeRows {
		return nil, (&model.ValidationError{}).Add("rows", "too_many_visible")
	}
	return order, nil
}

func homeRows(ctx context.Context, q *appq.Queries) ([]model.HomeRowSetting, error) {
	rows, err := q.ListHomeRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list home rows: %w", err)
	}
	out := make([]model.HomeRowSetting, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.HomeRowSetting{ID: r.ID, Key: r.Key, LibraryID: r.LibraryID.Int64, LibraryName: r.LibraryName,
			Position: int(r.Position), Hidden: r.Hidden == 1})
	}
	return out, nil
}
