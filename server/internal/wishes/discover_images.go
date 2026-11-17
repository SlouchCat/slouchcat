package wishes

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The image of a genre tile of «Entdecken» (getWishGenreBackdrop, concept 25 build step 5.4). The
// TMDB path comes only from the genre row cached for the profile (wish_discover_pages), never from
// the request, so a profile can make the server fetch nothing but the backdrops Seerr named.

// genreBackdrops is the part of metadata.Service that serves the backdrops; the Titles of the tests
// may leave it out (then no tile has an image).
type genreBackdrops interface {
	OpenGenreBackdrop(ctx context.Context, src string) (model.StreamSource, error)
}

// GenreBackdrop opens the backdrop of the genre tile genreID of films or series (media movie,
// show). model.ErrNotFound without the right, with the pages off, for a genre the row does not
// list or without backdrop; ErrDiscoverUnreachable when Seerr does not answer and nothing is
// cached; model.ErrTMDBUnreachable when TMDB did not deliver the image.
func (s *Service) GenreBackdrop(ctx context.Context, p model.Profile, media string, genreID int) (model.StreamSource, error) {
	a, err := s.discoverAccess(ctx, p)
	if err != nil {
		return model.StreamSource{}, err
	}
	if media != model.WishMediaMovie && media != model.WishMediaShow {
		return model.StreamSource{}, model.ErrNotFound
	}
	open, ok := s.titles.(genreBackdrops)
	if !ok {
		return model.StreamSource{}, model.ErrNotFound
	}
	tiles, err := s.genreTiles(ctx, a, media == model.WishMediaShow)
	if err != nil {
		return model.StreamSource{}, err
	}
	for _, g := range tiles.genres {
		if g.ID == genreID && g.Backdrop != "" {
			return open.OpenGenreBackdrop(ctx, g.Backdrop)
		}
	}
	return model.StreamSource{}, model.ErrNotFound
}
