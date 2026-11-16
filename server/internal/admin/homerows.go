package admin

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// HomeRows returns every start page row in display order with its hidden flag (E-9): the eight
// fixed rows, «Meine Medien» and one row «Neu in …» per library (concept 15 P-7). The row of a
// library that is being deleted (deleting_at, library.delete) is left out, like ListLibraries leaves
// the library out; it keeps its stored place and shows there again when the deletion is undone.
func (s *Service) HomeRows(ctx context.Context) ([]model.HomeRowSetting, error) {
	return homeRows(ctx, s.read)
}

// SetHomeRows stores the order and visibility of the start page rows by id (concept 15 P-7). The
// slice order is the display order; an empty slice is 422 rows invalid_value. A stored row the
// slice leaves out (a library added after the page was opened, or the row of a library that is
// being deleted) keeps its place in the stored order and its hidden flag. The merge covers every
// stored row, the one of a library that is being deleted too, so a slice that still names it is
// valid: the slice decides only its place, its hidden flag stays as stored, so a page read before
// the deletion cannot show a stored-hidden row. The returned list leaves it out like HomeRows. An
// unknown id, an id twice or a key that does not match the stored row is 422 rows invalid_value;
// more than model.MaxVisibleHomeRows rows not hidden is 422 rows too_many_visible (F-12). The row
// of a library that is being deleted does not count toward that limit, whatever its hidden flag,
// because the administration does not show it. The mark is per library and several libraries can
// be deleted at once, so when deletions are undone (the job fails and clears deleting_at), each row
// shows again at its stored place and the stored rows can hold more visible rows than the limit,
// one per library whose deletion was undone and whose row was visible; the start page never shows
// more than its MaxRows, and the next save with too many visible rows is refused as before.
// Positions become 0..n-1 over all stored rows in one transaction. The start page reads the order
// on every request, so there is nothing to invalidate.
func (s *Service) SetHomeRows(ctx context.Context, rows []model.HomeRowSetting) ([]model.HomeRowSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	stored, deleting, err := listHomeRows(ctx, q)
	if err != nil {
		return nil, err
	}
	order, err := mergeHomeRows(stored, rows, deleting)
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
// first, and keeps its flag. A row whose id is in deleting takes only its place from rows; it keeps
// its stored hidden flag and does not count toward the visible limit.
func mergeHomeRows(stored, rows []model.HomeRowSetting, deleting map[string]bool) ([]model.HomeRowSetting, error) {
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
		if !deleting[r.ID] {
			row.Hidden = r.Hidden
		}
		order = append(order, row)
	}
	for i, r := range stored { // stored is in display order
		if !given[r.ID] {
			order = slices.Insert(order, min(i, len(order)), r)
		}
	}
	visible := 0
	for _, r := range order {
		if !r.Hidden && !deleting[r.ID] {
			visible++
		}
	}
	if visible > model.MaxVisibleHomeRows {
		return nil, (&model.ValidationError{}).Add("rows", "too_many_visible")
	}
	return order, nil
}

// homeRows returns the stored rows without the one of a library that is being deleted.
func homeRows(ctx context.Context, q *appq.Queries) ([]model.HomeRowSetting, error) {
	all, deleting, err := listHomeRows(ctx, q)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(r model.HomeRowSetting) bool { return deleting[r.ID] }), nil
}

// listHomeRows returns every stored row in display order and the ids of the rows whose library is
// being deleted.
func listHomeRows(ctx context.Context, q *appq.Queries) ([]model.HomeRowSetting, map[string]bool, error) {
	rows, err := q.ListHomeRows(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list home rows: %w", err)
	}
	out := make([]model.HomeRowSetting, 0, len(rows))
	deleting := map[string]bool{}
	for _, r := range rows {
		out = append(out, model.HomeRowSetting{ID: r.ID, Key: r.Key, LibraryID: r.LibraryID.Int64, LibraryName: r.LibraryName,
			Position: int(r.Position), Hidden: r.Hidden == 1})
		if r.LibraryDeletingAt.Valid {
			deleting[r.ID] = true
		}
	}
	return out, deleting, nil
}
