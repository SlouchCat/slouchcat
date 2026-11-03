package admin

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The maintenance action «Verwaiste Benutzerdaten umhängen» (ADR 0023 point 3, idea 04 D1): it
// lists the soft-deleted films and series that have a live copy, with the user data left on
// them, and on confirmation merges the copies into them with the relink of package library. It
// knows no time limit and no freshness rule (decision 21): the administrator sees both titles and
// decides. It can be repeated.

// maxRelinkPairs caps the pairs of one preview and of one request.
const maxRelinkPairs = 200

// itemIDShape is the form of an item id (a ULID, schema ItemID of the contract).
var itemIDShape = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// ListRelinkPairs returns the first maxRelinkPairs pairs of a soft-deleted title and its live
// copy, of one library or of all (nil), ordered by library name, title of the copy and its id,
// each with the user data on the deleted title and the name of its library, and the number of all
// pairs. The user data of the listed pairs is counted in one statement.
func (s *Service) ListRelinkPairs(ctx context.Context, libraryID *int64) (model.RelinkPairList, error) {
	if s.maint == nil {
		return model.RelinkPairList{}, ErrNotConfigured
	}
	var lib int64
	if libraryID != nil {
		lib = *libraryID
	}
	pairs, err := s.maint.ListRelinkPairs(ctx, lib)
	if err != nil {
		return model.RelinkPairList{}, err
	}
	names, err := s.libraryNames(ctx)
	if err != nil {
		return model.RelinkPairList{}, err
	}
	slices.SortStableFunc(pairs, func(a, b library.RelinkPair) int {
		return cmp.Or(cmp.Compare(names[a.LibraryID], names[b.LibraryID]), cmp.Compare(a.LibraryID, b.LibraryID),
			cmp.Compare(a.Title, b.Title), cmp.Compare(a.LiveID, b.LiveID))
	})
	shown := pairs[:min(len(pairs), maxRelinkPairs)]
	ids := make([]string, 0, len(shown))
	for _, p := range shown {
		ids = append(ids, p.DeletedID)
	}
	counts, err := s.maint.CountRelinkUserData(ctx, ids)
	if err != nil {
		return model.RelinkPairList{}, err
	}
	out := model.RelinkPairList{Total: len(pairs), Pairs: make([]model.RelinkPair, 0, len(shown))}
	for _, p := range shown {
		out.Pairs = append(out.Pairs, model.RelinkPair{
			DeletedID: p.DeletedID, LiveID: p.LiveID, LibraryID: p.LibraryID, LibraryName: names[p.LibraryID],
			Kind: p.Kind, Title: p.Title, Year: p.Year, DeletedTitle: p.DeletedTitle, DeletedYear: p.DeletedYear,
			DeletedAt: p.DeletedAt, UserData: model.RelinkUserData(counts[p.DeletedID]),
		})
	}
	return out, nil
}

func (s *Service) libraryNames(ctx context.Context) (map[int64]string, error) {
	libs, err := s.libraries.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	names := make(map[int64]string, len(libs))
	for _, l := range libs {
		names[l.ID] = l.Name
	}
	return names, nil
}

// RelinkItems merges the live copies into their soft-deleted titles, one transaction per pair; a
// pair named twice counts once. A malformed request moves nothing (*model.ValidationError): no
// pair, more than maxRelinkPairs, or pairs[i] invalid_value for an id that is no ULID. A pair that
// is no pair (any more) is skipped and counted: an id is unknown or names neither film nor series
// (the copy merged meanwhile, by library.relink or an earlier request), the deleted title came back
// (two copies of one title, the first merged) or the titles changed. A running scan stops the run:
// with pairs merged before, the outcome carries
// StoppedBy "scan_running" and a repeated request merges the rest; with none, the error is a
// *model.ScanRunningError naming the library.
func (s *Service) RelinkItems(ctx context.Context, pairs []model.RelinkPairRef) (model.RelinkOutcome, error) {
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	var verr *model.ValidationError
	switch {
	case len(pairs) == 0:
		verr = verr.Add("pairs", "required")
	case len(pairs) > maxRelinkPairs:
		verr = verr.Add("pairs", "too_long")
	default:
		for i, p := range pairs {
			if !itemIDShape.MatchString(p.DeletedID) || !itemIDShape.MatchString(p.LiveID) {
				verr = verr.Add(fmt.Sprintf("pairs[%d]", i), "invalid_value")
			}
		}
	}
	if verr != nil {
		return model.RelinkOutcome{}, verr
	}
	if s.maint == nil {
		return model.RelinkOutcome{}, ErrNotConfigured
	}
	type pending struct {
		ref     model.RelinkPairRef
		library int64
	}
	out := model.RelinkOutcome{Relinked: []model.RelinkedPair{}}
	seen := make(map[model.RelinkPairRef]bool, len(pairs))
	run := make([]pending, 0, len(pairs))
	for _, p := range pairs {
		if seen[p] {
			continue
		}
		seen[p] = true
		lib, err := s.maint.CheckRelinkPair(ctx, p.DeletedID, p.LiveID)
		switch {
		case errors.Is(err, model.ErrNotFound) || errors.Is(err, library.ErrNotTwins):
			out.Skipped++
		case err != nil:
			return model.RelinkOutcome{}, err
		default:
			run = append(run, pending{ref: p, library: lib})
		}
	}
	for _, p := range run {
		res, err := s.maint.Relink(ctx, p.ref.DeletedID, p.ref.LiveID)
		if errors.Is(err, model.ErrScanRunning) {
			if len(out.Relinked) > 0 {
				out.StoppedBy = "scan_running"
				break
			}
			names, nerr := s.libraryNames(ctx)
			if nerr != nil {
				return model.RelinkOutcome{}, errors.Join(err, nerr)
			}
			return model.RelinkOutcome{}, &model.ScanRunningError{Library: names[p.library]}
		}
		switch {
		case errors.Is(err, library.ErrNotTwins) || errors.Is(err, model.ErrNotFound):
			out.Skipped++
			continue
		case err != nil:
			return out, err
		}
		out.Relinked = append(out.Relinked, model.RelinkedPair{KeptID: res.KeptID, RemovedID: res.RemovedID, Merged: res.Merged, Moved: res.Moved})
	}
	s.log.InfoContext(ctx, "maintenance relink", "relinked", len(out.Relinked), "skipped", out.Skipped, "stopped_by", out.StoppedBy)
	return out, nil
}
