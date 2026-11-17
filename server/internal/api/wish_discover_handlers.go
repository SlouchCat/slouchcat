package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/wishes"
)

// Wishes through Seerr, stage 5 (concept 25 build step 5.4, contract 1.41.0): the pages
// «Entdecken», «Filme», «Serien» and «Suche» of the wishes area. Every entry passed visibility in
// package wishes; an ItemRef appears only for a library title the profile sees (rule 2), and images
// go through Slouchcat's own routes (never TMDB's addresses).

// genreBackdropPath is the image of a genre tile (binary, CLAUDE.md rule 6).
const genreBackdropPath = "/wishes/genres/{media}/{genreId}/backdrop"

// genreBackdropSize is the nominal size of a genre tile image (TMDB's w780 of a 16:9 backdrop).
const genreBackdropWidth, genreBackdropHeight = 780, 439

// WishDiscoverService is the part of package wishes for stage 5 (part of WishesService).
type WishDiscoverService interface {
	DiscoverVisible(ctx context.Context, p model.Profile) (bool, error)
	Discover(ctx context.Context, p model.Profile) ([]wishes.DiscoverRow, error)
	DiscoverRowPage(ctx context.Context, p model.Profile, row string, page, genre int) (wishes.DiscoverPage, error)
	DiscoverTitles(ctx context.Context, p model.Profile, q wishes.TitlesQuery) (wishes.DiscoverPage, error)
	DiscoverGenres(ctx context.Context, p model.Profile, media string) ([]wishes.DiscoverGenre, error)
	SearchWishTitles(ctx context.Context, p model.Profile, q string, page int) (wishes.DiscoverPage, error)
	GenreBackdrop(ctx context.Context, p model.Profile, media string, genreID int) (model.StreamSource, error)
}

func (e serviceProblem) VisitGetWishDiscoverResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListWishDiscoverRowResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListWishTitlesResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitListWishGenresResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitSearchWishTitlesResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) GetWishDiscover(ctx context.Context, _ gen.GetWishDiscoverRequestObject) (gen.GetWishDiscoverResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.wishAllow(s.limits.pages, session.Profile); err != nil {
		return s.problem(ctx, err), nil
	}
	rows, err := svc.Discover(ctx, session.Profile)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetWishDiscover200JSONResponse{Rows: make([]gen.DiscoverRow, 0, len(rows))}
	for _, r := range rows {
		g := gen.DiscoverRow{Key: gen.DiscoverRowKey(r.Key), Stale: r.Stale, FetchedAt: optTime(r.FetchedAt),
			Genres: make([]gen.DiscoverGenreTile, 0, len(r.Genres))}
		if g.Entries, err = s.discoverEntries(ctx, svc, session.Profile, r.Entries); err != nil {
			return s.problem(ctx, err), nil
		}
		media := model.WishMediaMovie
		if r.Key == wishes.RowShowGenres {
			media = model.WishMediaShow
		}
		for _, gr := range r.Genres {
			tile := gen.DiscoverGenreTile{Id: gr.ID, Name: gr.Name}
			if gr.Backdrop != "" {
				tile.Backdrop = &gen.ImageRef{Url: BasePath + "/wishes/genres/" + media + "/" + strconv.Itoa(gr.ID) + "/backdrop?p=" +
					strconv.FormatInt(session.Profile.ID, 10), Width: genreBackdropWidth, Height: genreBackdropHeight}
			}
			g.Genres = append(g.Genres, tile)
		}
		if r.NextPage > 0 {
			n := r.NextPage
			g.NextPage = &n
		}
		out.Rows = append(out.Rows, g)
	}
	return out, nil
}

func (s *server) ListWishDiscoverRow(ctx context.Context, req gen.ListWishDiscoverRowRequestObject) (gen.ListWishDiscoverRowResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.wishAllow(s.limits.pages, session.Profile); err != nil {
		return s.problem(ctx, err), nil
	}
	page, err := svc.DiscoverRowPage(ctx, session.Profile, string(req.Row), deref(req.Params.Page), deref(req.Params.Genre))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out, err := s.discoverPage(ctx, svc, session.Profile, page)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListWishDiscoverRow200JSONResponse(out), nil
}

func (s *server) ListWishTitles(ctx context.Context, req gen.ListWishTitlesRequestObject) (gen.ListWishTitlesResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	q := wishes.TitlesQuery{Media: string(req.Media), Genre: deref(req.Params.Genre), HideAvailable: deref(req.Params.HideAvailable),
		Page: deref(req.Params.Page)}
	if req.Params.Sort != nil {
		q.Sort = string(*req.Params.Sort)
	}
	if err := s.wishAllow(s.limits.pages, session.Profile); err != nil {
		return s.problem(ctx, err), nil
	}
	page, err := svc.DiscoverTitles(ctx, session.Profile, q)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out, err := s.discoverPage(ctx, svc, session.Profile, page)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ListWishTitles200JSONResponse(out), nil
}

func (s *server) ListWishGenres(ctx context.Context, req gen.ListWishGenresRequestObject) (gen.ListWishGenresResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.wishAllow(s.limits.pages, session.Profile); err != nil {
		return s.problem(ctx, err), nil
	}
	list, err := svc.DiscoverGenres(ctx, session.Profile, string(req.Media))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListWishGenres200JSONResponse{}
	out.Items = make([]struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}, 0, len(list))
	for _, g := range list {
		out.Items = append(out.Items, struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		}{Id: g.ID, Name: g.Name})
	}
	return out, nil
}

func (s *server) SearchWishTitles(ctx context.Context, req gen.SearchWishTitlesRequestObject) (gen.SearchWishTitlesResponseObject, error) {
	session, svc, ok, err := s.wishSession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.wishAllow(s.limits.search, session.Profile); err != nil {
		return s.problem(ctx, err), nil
	}
	page, err := svc.SearchWishTitles(ctx, session.Profile, req.Params.Q, deref(req.Params.Page))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out, err := s.discoverPage(ctx, svc, session.Profile, page)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SearchWishTitles200JSONResponse(out), nil
}

func (s *server) discoverPage(ctx context.Context, svc WishesService, p model.Profile, page wishes.DiscoverPage) (gen.DiscoverPage, error) {
	entries, err := s.discoverEntries(ctx, svc, p, page.Entries)
	if err != nil {
		return gen.DiscoverPage{}, err
	}
	out := gen.DiscoverPage{Entries: entries, Page: max(page.Page, 1), Stale: page.Stale, FetchedAt: page.FetchedAt.UTC()}
	if page.TotalPages > 0 {
		n := page.TotalPages
		out.TotalPages = &n
	}
	return out, nil
}

// discoverEntries maps the tiles: the title as an ExternalTitleRef with the images of the stored
// kinds, and the ItemRef of a library title the profile sees (the service left out the others;
// one that turns invisible meanwhile is left out here too).
func (s *server) discoverEntries(ctx context.Context, svc WishesService, p model.Profile, list []wishes.DiscoverEntry) ([]gen.DiscoverEntry, error) {
	out := make([]gen.DiscoverEntry, 0, len(list))
	for _, e := range list {
		card := model.ExternalTitleCard{Media: e.Media, TMDBID: e.TMDBID, Title: e.Title, Year: e.Year, MinAge: e.MinAge,
			ImageKinds: e.ImageKinds, Images: e.Images, ProfileID: p.ID, HasTrailer: e.HasTrailer, CanRequest: e.CanRequest}
		if e.Wish != nil {
			card.Wish = &model.WishMark{ID: e.Wish.ID, State: e.Wish.State}
		}
		g := gen.DiscoverEntry{Title: toExternalTitleRef(card), SeerrRequested: e.SeerrRequested}
		if e.ItemID != "" {
			sum, err := svc.ItemSummary(ctx, p, e.ItemID)
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			item := toItemRef(p.ID, sum)
			g.Item = &item
		}
		out = append(out, g)
	}
	return out, nil
}

// getWishGenreBackdrop serves the image of a genre tile (binary endpoint, CLAUDE.md rule 6); the
// TMDB path comes from the cached genre row of the profile (package wishes), never the request.
func (s *server) getWishGenreBackdrop(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFrom(r.Context())
	svc := s.opts.Wishes
	if !ok || svc == nil {
		s.p.mapError(w, r, model.ErrNotImplemented)
		return
	}
	media, raw := chi.URLParam(r, "media"), chi.URLParam(r, "genreId")
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 || strconv.Itoa(id) != raw || (media != model.WishMediaMovie && media != model.WishMediaShow) {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	if err := s.wishAllow(s.limits.images, session.Profile); err != nil {
		s.p.mapError(w, r, err)
		return
	}
	src, err := svc.GenreBackdrop(r.Context(), session.Profile, media, id)
	s.serveExternalImage(w, r, src, err)
}
