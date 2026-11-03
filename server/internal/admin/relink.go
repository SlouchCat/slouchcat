package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The maintenance action «Verwaiste Benutzerdaten umhängen» (ADR 0023 point 3, idea 04 D1): it
// lists the soft-deleted films and series that have a live copy, with the user data left on
// them, and on confirmation merges the copies into them with the relink of package library. It
// knows no time limit and can be repeated.

// maxRelinkPairs caps the pairs of one preview and of one request.
const maxRelinkPairs = 200

// ListRelinkPairs returns the first maxRelinkPairs pairs of a soft-deleted title and its live
// copy, of one library or of all (nil), each with the user data on the deleted title and the
// name of its library, and the number of all pairs.
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
	libs, err := s.libraries.List(ctx)
	if err != nil {
		return model.RelinkPairList{}, fmt.Errorf("list libraries: %w", err)
	}
	names := make(map[int64]string, len(libs))
	for _, l := range libs {
		names[l.ID] = l.Name
	}
	out := model.RelinkPairList{Total: len(pairs), Pairs: make([]model.RelinkPair, 0, min(len(pairs), maxRelinkPairs))}
	for _, p := range pairs[:min(len(pairs), maxRelinkPairs)] {
		counts, err := s.maint.CountRelinkUserData(ctx, p.DeletedID)
		if err != nil {
			return model.RelinkPairList{}, err
		}
		out.Pairs = append(out.Pairs, model.RelinkPair{
			DeletedID: p.DeletedID, LiveID: p.LiveID, LibraryID: p.LibraryID, LibraryName: names[p.LibraryID],
			Kind: p.Kind, Title: p.Title, DeletedAt: p.DeletedAt, UserData: model.RelinkUserData(counts),
		})
	}
	return out, nil
}

// RelinkItems merges the live copies into their soft-deleted titles, one transaction per pair.
// Every pair must be listed by ListRelinkPairs, else nothing is merged (*model.ValidationError,
// pairs[i] invalid_value). A pair whose deleted title came back meanwhile (two copies of one
// title, or the pair twice) or whose titles are gone is skipped and counted. model.ErrScanRunning
// stops the run; the pairs before stay merged and a repeated request merges the rest.
func (s *Service) RelinkItems(ctx context.Context, pairs []model.RelinkPairRef) ([]model.RelinkedPair, int, error) {
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	var verr *model.ValidationError
	switch {
	case len(pairs) == 0:
		verr = verr.Add("pairs", "required")
	case len(pairs) > maxRelinkPairs:
		verr = verr.Add("pairs", "too_long")
	}
	if verr != nil {
		return nil, 0, verr
	}
	if s.maint == nil {
		return nil, 0, ErrNotConfigured
	}
	listed, err := s.maint.ListRelinkPairs(ctx, 0)
	if err != nil {
		return nil, 0, err
	}
	known := make(map[[2]string]bool, len(listed))
	for _, p := range listed {
		known[[2]string{p.DeletedID, p.LiveID}] = true
	}
	for i, p := range pairs {
		if !known[[2]string{p.DeletedID, p.LiveID}] {
			verr = verr.Add(fmt.Sprintf("pairs[%d]", i), "invalid_value")
		}
	}
	if verr != nil {
		return nil, 0, verr
	}
	done := []model.RelinkedPair{}
	skipped := 0
	for _, p := range pairs {
		res, err := s.maint.Relink(ctx, p.DeletedID, p.LiveID)
		switch {
		case errors.Is(err, library.ErrNotTwins) || errors.Is(err, model.ErrNotFound):
			skipped++
			continue
		case err != nil:
			return done, skipped, err
		}
		done = append(done, model.RelinkedPair{KeptID: res.KeptID, RemovedID: res.RemovedID, Merged: res.Merged, Moved: res.Moved})
	}
	s.log.InfoContext(ctx, "maintenance relink", "relinked", len(done), "skipped", skipped)
	return done, skipped, nil
}
