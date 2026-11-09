package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Curation operations of the administration (plan M7, design sections 2 and 3.2): the user picks
// (concept 18 part B, ADR 0028), collections, the row order and the Trakt state. Every answer names titles by id and plain title,
// never as ItemRef and never with an image URL (the administration is unfiltered).

func (e serviceProblem) VisitListAdminUserPicksResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRemoveAdminUserPickResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetPicksSettingsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdatePicksSettingsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSearchAdminItemsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListCollectionsResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitCreateCollectionResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetAdminCollectionResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateCollectionResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDeleteCollectionResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetCollectionItemsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetHomeRowsResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitSetHomeRowsResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitGetTraktStatusResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ListAdminUserPicks(ctx context.Context, _ gen.ListAdminUserPicksRequestObject) (gen.ListAdminUserPicksResponseObject, error) {
	picks, err := s.opts.Admin.ListUserPicks(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListAdminUserPicks200JSONResponse, 0, len(picks))
	for _, p := range picks {
		day, err := time.Parse(time.DateOnly, p.PickedOn)
		if err != nil {
			return nil, fmt.Errorf("day of the user pick %s: %w", p.ItemID, err)
		}
		out = append(out, gen.AdminUserPick{
			ItemId: p.ItemID, Kind: gen.ItemKind(p.Kind), Title: p.Title, Year: p.Year, Count: p.Count,
			LastPickedOn: openapi_types.Date{Time: day}, ExpiresInDays: p.ExpiresInDays,
			PosterUrl: adminPosterURL(p.ItemID, p.PosterHash),
		})
	}
	return out, nil
}

func (s *server) RemoveAdminUserPick(ctx context.Context, req gen.RemoveAdminUserPickRequestObject) (gen.RemoveAdminUserPickResponseObject, error) {
	if err := s.opts.Admin.RemoveUserPick(ctx, req.ItemId); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RemoveAdminUserPick204Response{}, nil
}

func (s *server) GetPicksSettings(ctx context.Context, _ gen.GetPicksSettingsRequestObject) (gen.GetPicksSettingsResponseObject, error) {
	days, err := s.opts.Admin.PicksRetention(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetPicksSettings200JSONResponse{RetentionDays: days}, nil
}

func (s *server) UpdatePicksSettings(ctx context.Context, req gen.UpdatePicksSettingsRequestObject) (gen.UpdatePicksSettingsResponseObject, error) {
	days, err := s.opts.Admin.SetPicksRetention(ctx, deref(req.Body).RetentionDays)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdatePicksSettings200JSONResponse{RetentionDays: days}, nil
}

func (s *server) SearchAdminItems(ctx context.Context, req gen.SearchAdminItemsRequestObject) (gen.SearchAdminItemsResponseObject, error) {
	hits, err := s.opts.Admin.SearchItems(ctx, req.Params.Q, deref(req.Params.Limit))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.SearchAdminItems200JSONResponse, 0, len(hits))
	for _, h := range hits {
		out = append(out, gen.AdminItemHit{
			Id: h.ID, Kind: gen.ItemKind(h.Kind), Title: h.Title, Year: h.Year, LibraryId: h.LibraryID,
			ParentTitle: optString(h.ParentTitle),
		})
	}
	return out, nil
}

func (s *server) ListCollections(ctx context.Context, _ gen.ListCollectionsRequestObject) (gen.ListCollectionsResponseObject, error) {
	list, err := s.opts.Admin.ListCollections(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListCollections200JSONResponse, 0, len(list))
	for _, c := range list {
		out = append(out, toAdminCollection(c))
	}
	return out, nil
}

func (s *server) CreateCollection(ctx context.Context, req gen.CreateCollectionRequestObject) (gen.CreateCollectionResponseObject, error) {
	c, err := s.opts.Admin.CreateCollection(ctx, deref(req.Body).Name)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateCollection201JSONResponse(toAdminCollection(c)), nil
}

func (s *server) GetAdminCollection(ctx context.Context, req gen.GetAdminCollectionRequestObject) (gen.GetAdminCollectionResponseObject, error) {
	c, err := s.opts.Admin.GetCollection(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetAdminCollection200JSONResponse(toAdminCollection(c)), nil
}

func (s *server) UpdateCollection(ctx context.Context, req gen.UpdateCollectionRequestObject) (gen.UpdateCollectionResponseObject, error) {
	c, err := s.opts.Admin.UpdateCollection(ctx, req.Id, deref(req.Body).Name)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateCollection200JSONResponse(toAdminCollection(c)), nil
}

func (s *server) DeleteCollection(ctx context.Context, req gen.DeleteCollectionRequestObject) (gen.DeleteCollectionResponseObject, error) {
	if err := s.opts.Admin.DeleteCollection(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteCollection204Response{}, nil
}

func (s *server) SetCollectionItems(ctx context.Context, req gen.SetCollectionItemsRequestObject) (gen.SetCollectionItemsResponseObject, error) {
	c, err := s.opts.Admin.SetCollectionItems(ctx, req.Id, deref(req.Body).ItemIds)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetCollectionItems200JSONResponse(toAdminCollection(c)), nil
}

func (s *server) GetHomeRows(ctx context.Context, _ gen.GetHomeRowsRequestObject) (gen.GetHomeRowsResponseObject, error) {
	rows, err := s.opts.Admin.HomeRows(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetHomeRows200JSONResponse(toHomeRowSettings(rows)), nil
}

func (s *server) SetHomeRows(ctx context.Context, req gen.SetHomeRowsRequestObject) (gen.SetHomeRowsResponseObject, error) {
	var in []model.HomeRowSetting
	if req.Body != nil {
		for _, r := range *req.Body {
			in = append(in, model.HomeRowSetting{ID: r.Id, Key: string(r.Key), Position: r.Position, Hidden: r.Hidden})
		}
	}
	rows, err := s.opts.Admin.SetHomeRows(ctx, in)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetHomeRows200JSONResponse(toHomeRowSettings(rows)), nil
}

func (s *server) GetTraktStatus(ctx context.Context, _ gen.GetTraktStatusRequestObject) (gen.GetTraktStatusResponseObject, error) {
	st, err := s.opts.Admin.TraktStatus(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetTraktStatus200JSONResponse(gen.TraktStatus{
		Configured: st.Configured, LastSuccess: utc(st.LastSuccess), LastAttempt: utc(st.LastAttempt),
		LastError: optString(st.LastError), ListKey: st.ListKey, Entries: st.Entries,
	}), nil
}

func toAdminCollection(c model.AdminCollection) gen.AdminCollection {
	items := make([]gen.AdminCollectionItem, 0, len(c.Items))
	for _, it := range c.Items {
		items = append(items, gen.AdminCollectionItem{
			ItemId: it.ItemID, Kind: gen.ItemKind(it.Kind), Title: it.Title, Year: it.Year, Position: it.Position,
			PosterUrl: adminPosterURL(it.ItemID, it.PosterHash),
		})
	}
	return gen.AdminCollection{
		Id: c.ID, Source: gen.AdminCollectionSource(c.Source), Name: c.Name, NameDe: optString(c.NameDe),
		TmdbId: c.TMDBID, ItemCount: c.ItemCount, Items: items,
	}
}

func toHomeRowSettings(rows []model.HomeRowSetting) []gen.HomeRowSetting {
	out := make([]gen.HomeRowSetting, 0, len(rows))
	for _, r := range rows {
		row := gen.HomeRowSetting{Id: r.ID, Key: gen.HomeRowKey(r.Key), Position: r.Position, Hidden: r.Hidden}
		if r.Key == model.RowLibrary { // concept 15 P-7: the library and its name
			row.LibraryId, row.LibraryName = &r.LibraryID, &r.LibraryName
		}
		out = append(out, row)
	}
	return out
}

type pickNullsKey struct{}

// pickUpdateNulls records which top-level fields of a PATCH body are JSON null: the generated
// request types cannot tell null (clear the field) from absent (keep it). It serves PATCH
// /admin/spotlight/entries/{media}/{tmdbId} (SpotlightEntryOverride, M11), PATCH
// /admin/radio/config and PATCH /admin/radio/stations/{stationId} (contract 1.17.0); it was made
// for the PATCH of the former curated picks (gone with contract 1.23.0, ADR 0028), hence the
// name. Field names match without regard to case, like encoding/json matches them when it decodes
// the body ({"Note": null} clears the note as {"note": null} does). The body is handed on
// unchanged; a read error (such as a body over the size limit) reaches the decoder of the strict
// handler after the bytes read so far.
func pickUpdateNulls(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var names []string
		switch {
		case strings.HasPrefix(r.URL.Path, BasePath+"/admin/spotlight/entries/"):
			names = spotlightOverrideFields
		case r.URL.Path == BasePath+"/admin/radio/config":
			names = radioConfigNullFields
		case strings.HasPrefix(r.URL.Path, BasePath+"/admin/radio/stations/"):
			names = radioStationNullFields
		}
		if r.Method != http.MethodPatch || r.Body == nil || names == nil {
			next.ServeHTTP(w, r)
			return
		}
		raw, err := io.ReadAll(r.Body)
		var rest io.Reader = bytes.NewReader(raw)
		if err != nil {
			rest = io.MultiReader(rest, errReader{err})
		}
		r.Body = io.NopCloser(rest)
		var fields map[string]json.RawMessage
		nulls := map[string]bool{}
		if json.Unmarshal(raw, &fields) == nil {
			for k, v := range fields {
				for _, name := range names {
					if strings.EqualFold(k, name) && string(bytes.TrimSpace(v)) == "null" {
						nulls[name] = true
					}
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pickNullsKey{}, nulls)))
	})
}

// errReader returns its error on every read.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
