package admin

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// MyMedia returns every entry of «Meine Medien» in display order with its hiding in the side
// navigation and on the start page (concept 21 P-11, O-21) and what it holds for every profile
// alike: a library its top-level titles that are not deleted, collections those with a member that
// is not an audiobook, picks the recommended titles within the retention. The count of radio (the
// stations in the offer) is the radio service's; it stays 0 here.
func (s *Service) MyMedia(ctx context.Context) ([]model.MyMediaSetting, error) {
	return s.myMedia(ctx, s.read, s.sys)
}

// SetMyMedia stores the order and hiding of «Meine Medien» by id (concept 21 P-11). The slice
// order is the display order; an empty slice, an unknown id, an id twice or a key that does not
// match the stored entry is 422 rows invalid_value. A stored entry the slice leaves out (a library
// added after the page was opened) keeps its place in the stored order and its flags. Positions
// become 0..n-1 in one transaction. The side navigation and the start page read the order on every
// request, so there is nothing to invalidate.
func (s *Service) SetMyMedia(ctx context.Context, entries []model.MyMediaSetting) ([]model.MyMediaSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	stored, err := listMyMedia(ctx, q)
	if err != nil {
		return nil, err
	}
	order, err := mergeMyMedia(stored, entries)
	if err != nil {
		return nil, err
	}
	now := s.now().UnixMilli()
	for i, e := range order {
		n, err := q.SetMyMediaEntry(ctx, appq.SetMyMediaEntryParams{ID: e.ID, Position: int64(i),
			HideSidebar: flag(e.HideSidebar), HideHome: flag(e.HideHome), Now: now})
		if err != nil {
			return nil, fmt.Errorf("set my media entry %s: %w", e.ID, err)
		}
		if n != 1 {
			return nil, fmt.Errorf("my media entry %s missing from my_media", e.ID)
		}
	}
	out, err := s.myMedia(ctx, q, systemq.New(tx))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "my media set")
	return out, nil
}

// mergeMyMedia returns the stored entries in the order of entries, with their flags from entries.
// A stored entry that entries leaves out goes back to its index in the stored order, earliest
// index first, and keeps its flags.
func mergeMyMedia(stored, entries []model.MyMediaSetting) ([]model.MyMediaSetting, error) {
	invalid := (&model.ValidationError{}).Add("rows", "invalid_value")
	if len(entries) == 0 {
		return nil, invalid
	}
	byID := make(map[string]model.MyMediaSetting, len(stored))
	for _, e := range stored {
		byID[e.ID] = e
	}
	given := make(map[string]bool, len(entries))
	order := make([]model.MyMediaSetting, 0, len(stored))
	for _, e := range entries {
		entry, ok := byID[e.ID]
		if !ok || given[e.ID] || entry.Key != e.Key {
			return nil, invalid
		}
		given[e.ID] = true
		entry.HideSidebar, entry.HideHome = e.HideSidebar, e.HideHome
		order = append(order, entry)
	}
	for i, e := range stored { // stored is in display order
		if !given[e.ID] {
			order = slices.Insert(order, min(i, len(order)), e)
		}
	}
	return order, nil
}

func (s *Service) myMedia(ctx context.Context, q *appq.Queries, sys *systemq.Queries) ([]model.MyMediaSetting, error) {
	out, err := listMyMedia(ctx, q)
	if err != nil {
		return nil, err
	}
	titles, err := sys.AdminLibraryTitleCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("library title counts: %w", err)
	}
	byLibrary := make(map[int64]int, len(titles))
	for _, t := range titles {
		byLibrary[t.LibraryID] = int(t.Titles)
	}
	collections, err := sys.AdminCollectionCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("collection count: %w", err)
	}
	picks, err := sys.AdminUserPickCount(ctx, s.now().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("user pick count: %w", err)
	}
	for i := range out {
		switch out[i].Key {
		case model.MyMediaLibrary:
			out[i].Count = byLibrary[out[i].LibraryID]
		case model.MyMediaCollections:
			out[i].Count = int(collections)
		case model.MyMediaPicks:
			out[i].Count = int(picks)
		}
	}
	return out, nil
}

func listMyMedia(ctx context.Context, q *appq.Queries) ([]model.MyMediaSetting, error) {
	rows, err := q.ListMyMedia(ctx)
	if err != nil {
		return nil, fmt.Errorf("list my media: %w", err)
	}
	out := make([]model.MyMediaSetting, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.MyMediaSetting{ID: r.ID, Key: r.Key, LibraryID: r.LibraryID.Int64, LibraryName: r.LibraryName,
			LibraryKind: r.LibraryKind, Position: int(r.Position), HideSidebar: r.HideSidebar == 1, HideHome: r.HideHome == 1})
	}
	return out, nil
}

func flag(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
