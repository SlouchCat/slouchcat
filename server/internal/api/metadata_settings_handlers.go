package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Metadata settings of concept 10 (ADR 0018): the server's settings and the apply run per folder
// (concept 24); all x-visibility admin (concept 9.1).

func (e serviceProblem) VisitGetMetadataSettingsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateMetadataSettingsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitApplyMetadataResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) GetMetadataSettings(ctx context.Context, _ gen.GetMetadataSettingsRequestObject) (gen.GetMetadataSettingsResponseObject, error) {
	st, err := s.opts.Admin.MetadataSettings(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetMetadataSettings200JSONResponse(toMetadataSettings(st)), nil
}

func (s *server) UpdateMetadataSettings(ctx context.Context, req gen.UpdateMetadataSettingsRequestObject) (gen.UpdateMetadataSettingsResponseObject, error) {
	b := deref(req.Body)
	m := model.MetadataSettings{
		TextLangs: b.TextLangs, ImageLangs: fromImageLangs(b.ImageLangs), ImageKinds: b.ImageKinds,
		ImageMinWidth: map[string]int{"poster": b.ImageMinWidth.Poster, "backdrop": b.ImageMinWidth.Backdrop},
		ImageByVote:   b.ImageByVote, LocalImages: b.LocalImages, RatingCountry: b.RatingCountry,
		TMDBIncludeAdult: b.TmdbIncludeAdult, FanartEnabled: b.FanartEnabled,
	}
	st, err := s.opts.Admin.UpdateMetadataSettings(ctx, m)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateMetadataSettings200JSONResponse(toMetadataSettings(st)), nil
}

func (s *server) ApplyMetadata(ctx context.Context, req gen.ApplyMetadataRequestObject) (gen.ApplyMetadataResponseObject, error) {
	b := deref(req.Body)
	what := make([]string, 0, len(b.What))
	for _, w := range b.What {
		what = append(what, string(w))
	}
	res, err := s.opts.Admin.ApplyMetadata(ctx, model.ApplyMetadataRequest{RootIDs: deref(b.RootIds), What: what})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ApplyMetadata202JSONResponse{JobId: res.JobID, Titles: res.Titles}, nil
}

func toImageLangs(m map[string][]string) gen.ImageLangs {
	return gen.ImageLangs{Poster: strs(m["poster"]), Backdrop: strs(m["backdrop"]), Logo: strs(m["logo"])}
}

func fromImageLangs(l gen.ImageLangs) map[string][]string {
	return map[string][]string{"poster": strs(l.Poster), "backdrop": strs(l.Backdrop), "logo": strs(l.Logo)}
}

func toMetadataSettings(st model.MetadataSettings) gen.MetadataSettings {
	kinds := make([]gen.MetadataSettingsImageKinds, 0, len(st.ImageKinds))
	for _, k := range st.ImageKinds {
		kinds = append(kinds, gen.MetadataSettingsImageKinds(k))
	}
	return gen.MetadataSettings{
		TextLangs: strs(st.TextLangs), ImageLangs: toImageLangs(st.ImageLangs), ImageKinds: kinds,
		ImageMinWidth: struct {
			Backdrop int `json:"backdrop"`
			Poster   int `json:"poster"`
		}{Backdrop: st.ImageMinWidth["backdrop"], Poster: st.ImageMinWidth["poster"]},
		ImageByVote: st.ImageByVote, LocalImages: st.LocalImages, RatingCountry: gen.MetadataSettingsRatingCountry(st.RatingCountry),
		TmdbIncludeAdult: st.TMDBIncludeAdult, FanartEnabled: st.FanartEnabled, KnownLanguages: strs(st.KnownLanguages),
		RatingSystems:  toRatingSystems(st.RatingSystems),
		AffectedTitles: st.AffectedTitles, LocalImageTitles: st.LocalImageTitles,
	}
}

// strs returns a non-nil string slice (JSON [] instead of null).
func strs(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func toRatingSystems(list []model.RatingSystem) []gen.RatingSystem {
	out := make([]gen.RatingSystem, 0, len(list))
	for _, rs := range list {
		out = append(out, toRatingSystem(rs))
	}
	return out
}
