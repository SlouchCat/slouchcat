package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M7 curation operations of the main user (plan M7, design sections 2 and 3.2): Cat's picks,
// collections, the row order and the Trakt state. Every answer names titles by id and plain title,
// never as ItemRef and never with an image URL (the administration is unfiltered).

func (e serviceProblem) VisitListTopPicksResponse(w http.ResponseWriter) error     { return e.visit(w) }
func (e serviceProblem) VisitCreateTopPicksResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitReorderTopPicksResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitUpdateTopPickResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitDeleteTopPickResponse(w http.ResponseWriter) error    { return e.visit(w) }
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
func (e serviceProblem) VisitRefreshTraktResponse(w http.ResponseWriter) error   { return e.visit(w) }

func (s *server) ListTopPicks(ctx context.Context, _ gen.ListTopPicksRequestObject) (gen.ListTopPicksResponseObject, error) {
	picks, err := s.opts.Admin.ListTopPicks(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListTopPicks200JSONResponse, 0, len(picks))
	for _, p := range picks {
		out = append(out, toAdminPick(p))
	}
	return out, nil
}

func (s *server) CreateTopPicks(ctx context.Context, req gen.CreateTopPicksRequestObject) (gen.CreateTopPicksResponseObject, error) {
	b := deref(req.Body)
	picks, err := s.opts.Admin.CreateTopPicks(ctx, model.PickRequest{
		ItemID: b.ItemId, ProfileIDs: deref(b.ProfileIds), Note: deref(b.Note), StartsAt: b.StartsAt, EndsAt: b.EndsAt,
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.CreateTopPicks201JSONResponse, 0, len(picks))
	for _, p := range picks {
		out = append(out, toAdminPick(p))
	}
	return out, nil
}

func (s *server) ReorderTopPicks(ctx context.Context, req gen.ReorderTopPicksRequestObject) (gen.ReorderTopPicksResponseObject, error) {
	if err := s.opts.Admin.ReorderTopPicks(ctx, deref(req.Body).Ids); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ReorderTopPicks204Response{}, nil
}

func (s *server) UpdateTopPick(ctx context.Context, req gen.UpdateTopPickRequestObject) (gen.UpdateTopPickResponseObject, error) {
	b := deref(req.Body)
	nulls, _ := ctx.Value(pickNullsKey{}).(map[string]bool)
	upd := model.PickUpdate{
		Note: b.Note, ClearNote: nulls["note"],
		StartsAt: b.StartsAt, ClearStartsAt: nulls["startsAt"],
		EndsAt: b.EndsAt, ClearEndsAt: nulls["endsAt"],
	}
	p, err := s.opts.Admin.UpdateTopPick(ctx, req.Id, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateTopPick200JSONResponse(toAdminPick(p)), nil
}

func (s *server) DeleteTopPick(ctx context.Context, req gen.DeleteTopPickRequestObject) (gen.DeleteTopPickResponseObject, error) {
	if err := s.opts.Admin.DeleteTopPick(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteTopPick204Response{}, nil
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
			in = append(in, model.HomeRowSetting{Key: string(r.Key), Position: r.Position, Hidden: r.Hidden})
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
		LastError: optString(st.LastError), Trending: st.Trending, Popular: st.Popular,
	}), nil
}

func (s *server) RefreshTrakt(ctx context.Context, _ gen.RefreshTraktRequestObject) (gen.RefreshTraktResponseObject, error) {
	if err := s.opts.Admin.RefreshTrakt(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RefreshTrakt202Response{}, nil
}

func toAdminPick(p model.AdminPick) gen.AdminPick {
	return gen.AdminPick{
		Id: p.ID, ItemId: p.ItemID, Kind: gen.ItemKind(p.Kind), Title: p.Title, Year: p.Year,
		ProfileId: p.ProfileID, ProfileName: optString(p.ProfileName), Position: p.Position, Note: optString(p.Note),
		StartsAt: utc(p.StartsAt), EndsAt: utc(p.EndsAt), CreatedAt: p.CreatedAt.UTC(),
	}
}

func toAdminCollection(c model.AdminCollection) gen.AdminCollection {
	items := make([]gen.AdminCollectionItem, 0, len(c.Items))
	for _, it := range c.Items {
		items = append(items, gen.AdminCollectionItem{
			ItemId: it.ItemID, Kind: gen.ItemKind(it.Kind), Title: it.Title, Year: it.Year, Position: it.Position,
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
		out = append(out, gen.HomeRowSetting{Key: gen.HomeRowKey(r.Key), Position: r.Position, Hidden: r.Hidden})
	}
	return out
}

type pickNullsKey struct{}

// pickUpdateNulls records which top-level fields of a PATCH /admin/picks/{id} body are JSON null:
// the generated PickUpdate cannot tell null (clear the field) from absent (keep it). The body is
// handed on unchanged; a read error (such as a body over the size limit) reaches the decoder of
// the strict handler after the bytes read so far.
func pickUpdateNulls(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.Body == nil || !strings.HasPrefix(r.URL.Path, BasePath+"/admin/picks/") {
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
				nulls[k] = string(bytes.TrimSpace(v)) == "null"
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pickNullsKey{}, nulls)))
	})
}

// errReader returns its error on every read.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
