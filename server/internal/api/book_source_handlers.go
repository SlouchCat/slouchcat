package api

import (
	"context"
	"net/http"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Book sources (contract 1.26.0, concept 19 stage D3, D6): the match preview of an audiobook and
// the page «Hörbücher: Quellen». Administration views with ids, never an ItemRef.

// bookProviders are the providers that search and match audiobooks.
var bookProviders = []string{model.ProviderAudible, model.ProviderDNB, model.ProviderAppleBooks,
	model.ProviderOpenLibrary, model.ProviderGoogleBooks}

func isBookProvider(p string) bool { return slices.Contains(bookProviders, p) }

func (e serviceProblem) VisitGetMatchPreviewResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetAudiobookSourcesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateAudiobookSourcesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitMatchAllAudiobooksResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListAudiobookReviewResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// regionParam is an optional Audible region ("" = the setting).
func regionParam(r *gen.AudibleRegion) string {
	if r == nil {
		return ""
	}
	return string(*r)
}

func optRegion(r string) *gen.AudibleRegion {
	if r == "" {
		return nil
	}
	v := gen.AudibleRegion(r)
	return &v
}

func optStrings(v []string) *[]string {
	if len(v) == 0 {
		return nil
	}
	return &v
}

func optFloat32(v *float64) *float32 {
	if v == nil {
		return nil
	}
	f := float32(*v)
	return &f
}

func optInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// toMetadataCandidate maps a candidate of a provider, with the fields of a book source.
func toMetadataCandidate(c model.MetadataCandidate) gen.MetadataCandidate {
	return gen.MetadataCandidate{
		Provider: gen.MetadataProvider(c.Provider), ProviderId: c.ProviderID, Title: c.Title, OriginalTitle: optString(c.OriginalTitle),
		Year: c.Year, Overview: optString(c.Overview), PosterUrl: optString(c.PosterURL),
		Subtitle: optString(c.Subtitle), Authors: optStrings(c.Authors), Narrators: optStrings(c.Narrators),
		Publisher: optString(c.Publisher), SeriesName: optString(c.SeriesName), SeriesIndex: optFloat32(c.SeriesIndex),
		Language: optString(c.Language), DurationMs: optInt64(c.DurationMs), Asin: optString(c.ASIN), Isbn: optString(c.ISBN),
		CoverUrl: optString(c.CoverURL), Region: optRegion(c.Region), DurationMatches: c.DurationMatches, Score: optFloat32(c.Score),
	}
}

// toAdminBookMatch maps the match of an audiobook (nil for none).
func toAdminBookMatch(m *model.AdminBookMatch) *gen.AdminBookMatch {
	if m == nil {
		return nil
	}
	out := &gen.AdminBookMatch{State: gen.AdminBookMatchState(m.State), Manual: m.Manual, ProviderId: optString(m.ProviderID),
		Region: optRegion(m.Region), MatchedAt: m.MatchedAt, Error: optString(m.Error)}
	if m.Provider != "" {
		p := gen.MetadataProvider(m.Provider)
		out.Provider = &p
	}
	return out
}

func (s *server) GetMatchPreview(ctx context.Context, req gen.GetMatchPreviewRequestObject) (gen.GetMatchPreviewResponseObject, error) {
	p := req.Params
	m, err := s.opts.Admin.GetMatchPreview(ctx, req.Id, string(p.Provider), p.ProviderId, regionParam(p.Region))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetMatchPreview200JSONResponse{Provider: gen.MetadataProvider(m.Provider), ProviderId: m.ProviderID, Title: m.Title,
		DurationMs: optInt64(m.DurationMs), DurationMatches: m.DurationMatches, CoverUrl: optString(m.CoverURL),
		Fields: make([]gen.MatchPreviewField, 0, len(m.Fields))}
	for _, f := range m.Fields {
		out.Fields = append(out.Fields, gen.MatchPreviewField{Field: gen.BookMatchField(f.Field), Current: f.Current, Proposed: f.Proposed,
			Locked: f.Locked, Differs: f.Differs})
	}
	return out, nil
}

func toAudiobookSources(a model.AudiobookSources) gen.AudiobookSources {
	st := a.Settings
	out := gen.AudiobookSources{AudibleRegion: gen.AudibleRegion(st.AudibleRegion), AudnexusUrl: st.AudnexusURL,
		GoogleBooksKey: toKeyState(a.GoogleBooksKey), AutoMatch: gen.AudiobookAutoMatch(st.AutoMatch),
		TolerancePercent: st.TolerancePercent, NormalizeAuthors: st.NormalizeAuthors, NormalizeSeries: st.NormalizeSeries,
		Folders: make([]gen.AudiobookFolder, 0, len(a.Folders)), Sources: make([]gen.AudiobookSourceState, 0, len(a.Sources)),
		Stats: gen.AudiobookMatchStats{Total: a.Stats.Total, Matched: a.Stats.Matched, Review: a.Stats.Review, None: a.Stats.None,
			Error: a.Stats.Error, Pending: a.Stats.Pending},
		Running: a.Running}
	for _, f := range a.Folders {
		out.Folders = append(out.Folders, gen.AudiobookFolder{LibraryId: f.LibraryID, LibraryName: f.LibraryName, RootId: f.RootID,
			Path: f.Path, Providers: providerEnums(f.Providers)})
	}
	for _, src := range a.Sources {
		out.Sources = append(out.Sources, gen.AudiobookSourceState{Id: gen.MetadataProvider(src.ID), Configured: src.Configured,
			PausedUntil: src.PausedUntil})
	}
	return out
}

func (s *server) GetAudiobookSources(ctx context.Context, _ gen.GetAudiobookSourcesRequestObject) (gen.GetAudiobookSourcesResponseObject, error) {
	a, err := s.opts.Admin.AudiobookSources(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetAudiobookSources200JSONResponse(toAudiobookSources(a)), nil
}

func (s *server) UpdateAudiobookSources(ctx context.Context, req gen.UpdateAudiobookSourcesRequestObject) (gen.UpdateAudiobookSourcesResponseObject, error) {
	var upd model.AudiobookSourcesUpdate
	if b := req.Body; b != nil {
		if b.AudibleRegion != nil {
			r := string(*b.AudibleRegion)
			upd.AudibleRegion = &r
		}
		if b.AutoMatch != nil {
			m := string(*b.AutoMatch)
			upd.AutoMatch = &m
		}
		upd.AudnexusURL, upd.TolerancePercent = b.AudnexusUrl, b.TolerancePercent
		upd.NormalizeAuthors, upd.NormalizeSeries = b.NormalizeAuthors, b.NormalizeSeries
		upd.GoogleBooksKey, upd.ClearGoogleBooksKey = b.GoogleBooksKey, b.ClearGoogleBooksKey != nil && *b.ClearGoogleBooksKey
	}
	a, err := s.opts.Admin.UpdateAudiobookSources(ctx, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateAudiobookSources200JSONResponse(toAudiobookSources(a)), nil
}

func (s *server) MatchAllAudiobooks(ctx context.Context, _ gen.MatchAllAudiobooksRequestObject) (gen.MatchAllAudiobooksResponseObject, error) {
	n, err := s.opts.Admin.MatchAllAudiobooks(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.MatchAllAudiobooks202JSONResponse{Queued: n}, nil
}

func (s *server) ListAudiobookReview(ctx context.Context, _ gen.ListAudiobookReviewRequestObject) (gen.ListAudiobookReviewResponseObject, error) {
	list, err := s.opts.Admin.ListAudiobookReview(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListAudiobookReview200JSONResponse, 0, len(list))
	for _, e := range list {
		g := gen.BookReviewEntry{ItemId: e.ItemID, Title: e.Title, Author: e.Author, LibraryId: e.LibraryID, DurationMs: e.DurationMs,
			State: gen.BookReviewEntryState(e.State), Error: optString(e.Error), MatchedAt: e.MatchedAt,
			Candidates: make([]gen.MetadataCandidate, 0, len(e.Candidates))}
		for _, c := range e.Candidates {
			g.Candidates = append(g.Candidates, toMetadataCandidate(c))
		}
		out = append(out, g)
	}
	return out, nil
}
