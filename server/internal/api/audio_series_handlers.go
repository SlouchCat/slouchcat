package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Audiobook series and collections (contract 1.25.0, concept 19 stage D2): the row «Reihen und
// Sammlungen» of a library, the page of a series and «Reihe bearbeiten». The catalog reads through
// the visibility layer; a library or series the profile does not see answers 404, never 403.

func (e serviceProblem) VisitListAudioGroupsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetAudioSeriesResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitUpdateAudioSeriesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// ListAudioGroups checks the filter bounds like ListLibraryItems; the catalog checks the status.
func (s *server) ListAudioGroups(ctx context.Context, req gen.ListAudioGroupsRequestObject) (gen.ListAudioGroupsResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	if verr := libraryFilterProblems(p.Genre, p.YearFrom, p.YearTo, p.MaxRating); verr != nil {
		return s.problem(ctx, verr), nil
	}
	q := model.LibraryQuery{Genre: deref(p.Genre), YearFrom: deref(p.YearFrom), YearTo: deref(p.YearTo), MaxRating: -1}
	if p.MaxRating != nil {
		q.MaxRating = int(*p.MaxRating)
	}
	for _, st := range deref(p.Status) {
		q.Status = append(q.Status, string(st))
	}
	list, err := s.opts.Catalog.AudioGroups(ctx, session.Profile, req.Id, q)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	pid := session.Profile.ID
	out := gen.AudioGroupList{Groups: make([]gen.AudioGroup, 0, len(list.Groups)), SeriesCount: list.SeriesCount, CollectionCount: list.CollectionCount}
	for _, g := range list.Groups {
		out.Groups = append(out.Groups, gen.AudioGroup{Id: g.ID, Kind: gen.AudioGroupKind(g.Kind), Name: g.Name, Author: optString(g.Author),
			Count: g.Count, Matching: g.Matching, Completed: g.Completed, Cover: optItemRef(pid, g.Cover)})
	}
	return gen.ListAudioGroups200JSONResponse(out), nil
}

func (s *server) GetAudioSeries(ctx context.Context, req gen.GetAudioSeriesRequestObject) (gen.GetAudioSeriesResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	a, err := s.opts.Catalog.AudioSeries(ctx, session.Profile, req.Id, req.SeriesId)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	pid := session.Profile.ID
	locked := a.NameLocked
	out := gen.AudioSeries{Id: a.ID, LibraryId: a.LibraryID, Name: a.Name, NameLocked: &locked, Author: optString(a.Author),
		Narrator: optString(a.Narrator), DurationMs: a.DurationMs, Cover: optItemRef(pid, a.Cover), Next: optItemRef(pid, a.Next),
		Volumes: make([]gen.AudioSeriesVolume, 0, len(a.Volumes)), Spellings: make([]gen.AudioSeriesSpelling, 0, len(a.Spellings))}
	for _, v := range a.Volumes {
		vol := gen.AudioSeriesVolume{Item: toItemRef(pid, v.Item), Narrator: optString(v.Narrator)}
		if v.SeriesIndex != nil {
			idx := float32(*v.SeriesIndex)
			vol.SeriesIndex = &idx
		}
		out.Volumes = append(out.Volumes, vol)
	}
	for _, sp := range a.Spellings {
		out.Spellings = append(out.Spellings, gen.AudioSeriesSpelling{Name: sp.Name, Count: sp.Count})
	}
	return gen.GetAudioSeries200JSONResponse(out), nil
}

// UpdateAudioSeries is «Reihe bearbeiten»; the middleware lets only an administrator with the
// primary profile through, whose profile decides the volumes «In alle Bände schreiben» reaches.
func (s *server) UpdateAudioSeries(ctx context.Context, req gen.UpdateAudioSeriesRequestObject) (gen.UpdateAudioSeriesResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return nil, errNoSession
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("name", "required")), nil
	}
	a, err := s.opts.Admin.UpdateAudioSeries(ctx, session.Profile.ID, req.SeriesId, req.Body.Name, deref(req.Body.WriteToVolumes))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateAudioSeries200JSONResponse(gen.AdminAudioSeries{Id: a.ID, LibraryId: a.LibraryID, Name: a.Name,
		NameLocked: a.NameLocked, VolumeCount: a.VolumeCount}), nil
}
