package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M11 spotlight administration (plan 05-plan-spotlight.md 5.4, ADR 0011): the configuration with
// the catalogue of list kinds, the fetched entries with their library hits and overrides, one
// override, and a manual fetch. Titles are named by id and plain title, never as ItemRef.

func (e serviceProblem) VisitGetSpotlightConfigResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetSpotlightConfigResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetSpotlightEntriesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetSpotlightEntryOverrideResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRefreshSpotlightResponse(w http.ResponseWriter) error { return e.visit(w) }

// spotlightOverrideFields are the nullable fields of SpotlightEntryOverride, by their contract
// names; pickUpdateNulls records which of them are JSON null.
var spotlightOverrideFields = []string{"pinnedRank", "note"}

func (s *server) GetSpotlightConfig(ctx context.Context, _ gen.GetSpotlightConfigRequestObject) (gen.GetSpotlightConfigResponseObject, error) {
	cfg, err := s.opts.Admin.SpotlightConfig(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetSpotlightConfig200JSONResponse(toSpotlightConfig(cfg)), nil
}

func (s *server) SetSpotlightConfig(ctx context.Context, req gen.SetSpotlightConfigRequestObject) (gen.SetSpotlightConfigResponseObject, error) {
	b := deref(req.Body)
	cfg, err := s.opts.Admin.SetSpotlightConfig(ctx, model.SpotlightConfigUpdate{
		Enabled: b.Enabled, ListKey: string(b.ListKey), ListURL: deref(b.ListUrl), Media: string(b.Media),
		ItemCount: b.ItemCount, MinHits: b.MinHits, IntervalHours: b.IntervalHours,
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetSpotlightConfig200JSONResponse(toSpotlightConfig(cfg)), nil
}

func (s *server) GetSpotlightEntries(ctx context.Context, _ gen.GetSpotlightEntriesRequestObject) (gen.GetSpotlightEntriesResponseObject, error) {
	list, err := s.opts.Admin.SpotlightEntries(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.SpotlightEntries{
		ListKey: gen.SpotlightListKey(list.ListKey),
		Entries: make([]gen.SpotlightEntry, 0, len(list.Entries)),
		Orphans: make([]gen.SpotlightEntry, 0, len(list.Orphans)),
	}
	for _, e := range list.Entries {
		out.Entries = append(out.Entries, toSpotlightEntry(e))
	}
	for _, e := range list.Orphans {
		out.Orphans = append(out.Orphans, toSpotlightEntry(e))
	}
	return gen.GetSpotlightEntries200JSONResponse(out), nil
}

// SetSpotlightEntryOverride tells null (clear) from absent (keep) through pickUpdateNulls; a body
// without any of the three fields removes the override.
func (s *server) SetSpotlightEntryOverride(ctx context.Context, req gen.SetSpotlightEntryOverrideRequestObject) (gen.SetSpotlightEntryOverrideResponseObject, error) {
	b := deref(req.Body)
	nulls, _ := ctx.Value(pickNullsKey{}).(map[string]bool)
	upd := model.SpotlightOverrideUpdate{
		Hidden:     b.Hidden,
		PinnedRank: b.PinnedRank, ClearPinnedRank: nulls["pinnedRank"],
		Note: b.Note, ClearNote: nulls["note"],
	}
	upd.Empty = upd.Hidden == nil && upd.PinnedRank == nil && upd.Note == nil && !upd.ClearPinnedRank && !upd.ClearNote
	e, err := s.opts.Admin.SetSpotlightEntryOverride(ctx, string(req.Media), req.TmdbId, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetSpotlightEntryOverride200JSONResponse(toSpotlightEntry(e)), nil
}

func (s *server) RefreshSpotlight(ctx context.Context, _ gen.RefreshSpotlightRequestObject) (gen.RefreshSpotlightResponseObject, error) {
	if err := s.opts.Admin.RefreshSpotlight(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RefreshSpotlight202Response{}, nil
}

func toSpotlightConfig(c model.SpotlightConfig) gen.SpotlightConfig {
	kinds := make([]gen.SpotlightListKind, 0, len(c.ListKinds))
	for _, k := range c.ListKinds {
		media := make([]gen.SpotlightMedia, 0, len(k.Media))
		for _, m := range k.Media {
			media = append(media, gen.SpotlightMedia(m))
		}
		kinds = append(kinds, gen.SpotlightListKind{
			Key: gen.SpotlightListKey(k.Key), Media: media, MediaFilters: k.MediaFilters, NeedsUrl: k.NeedsURL,
		})
	}
	return gen.SpotlightConfig{
		Enabled: c.Enabled, ListKey: gen.SpotlightListKey(c.ListKey), ListUrl: optString(c.ListURL),
		Media: gen.SpotlightMedia(c.Media), ItemCount: c.ItemCount, MinHits: c.MinHits,
		IntervalHours: c.IntervalHours, ListKinds: kinds,
	}
}

func toSpotlightEntry(e model.SpotlightEntry) gen.SpotlightEntry {
	return gen.SpotlightEntry{
		Media: gen.SpotlightEntryMedia(e.Media), TmdbId: e.TmdbID, Position: e.Position,
		Title: optString(e.Title), Year: e.Year, ItemId: optString(e.ItemID), ItemTitle: optString(e.ItemTitle),
		Hidden: e.Hidden, PinnedRank: e.PinnedRank, Note: optString(e.Note), Orphaned: e.Orphaned,
		PosterUrl: adminPosterURL(e.ItemID, e.PosterHash),
	}
}
