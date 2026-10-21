package admin

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// HomeRows returns the eight start page rows in display order with their hidden flag (E-9).
func (s *Service) HomeRows(ctx context.Context) ([]model.HomeRowSetting, error) {
	return homeRows(ctx, s.read)
}

// SetHomeRows stores the order and visibility of the start page rows: rows must name every row
// key exactly once (else 422 rows invalid_value); the slice order is the display order, positions
// become 0..7 in one transaction. The start page reads the order on every request, so there is
// nothing to invalidate.
func (s *Service) SetHomeRows(ctx context.Context, rows []model.HomeRowSetting) ([]model.HomeRowSetting, error) {
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, r.Key)
	}
	slices.Sort(keys)
	want := slices.Clone(model.HomeRowKeys)
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		return nil, (&model.ValidationError{}).Add("rows", "invalid_value")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	now := s.now().UnixMilli()
	for i, r := range rows {
		hidden := int64(0)
		if r.Hidden {
			hidden = 1
		}
		n, err := q.SetHomeRow(ctx, appq.SetHomeRowParams{Key: r.Key, Position: int64(i), Hidden: hidden, Now: now})
		if err != nil {
			return nil, fmt.Errorf("set home row %s: %w", r.Key, err)
		}
		if n != 1 {
			return nil, fmt.Errorf("home row %s missing from home_rows", r.Key)
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

func homeRows(ctx context.Context, q *appq.Queries) ([]model.HomeRowSetting, error) {
	rows, err := q.ListHomeRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list home rows: %w", err)
	}
	out := make([]model.HomeRowSetting, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.HomeRowSetting{Key: r.Key, Position: int(r.Position), Hidden: r.Hidden == 1})
	}
	return out, nil
}
