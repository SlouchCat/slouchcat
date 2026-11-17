package api

import (
	"strconv"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Limits for titles outside the library (concept 25, security finding of stages 2 and 5): every
// profile with the request right can name any TMDB id, and the server then fetches, stores and
// scales the title and its images. Each operation that may make the server ask TMDB or Seerr takes
// a token per profile from a bucket of its group and answers 429 too_many_attempts with
// Retry-After once the bucket is empty (the token bucket of the login attempts, auth.Limiter, as
// the search of stage 2 uses it; createWish has its own limit in package wishes).
//
// The numbers come from what one person browsing does. A view of «Entdecken» makes 1 page call
// and loads up to 7 rows x 20 posters plus about 40 genre tiles, so about 180 images; a scroll
// through a row adds a page call and 20 posters. A preview page is one title call (and the
// dialog one season call) and three to seven images.
const (
	// getExternalTitle and getWishSeasons: 60 a minute, 30 at once.
	wishTitlesPerMin, wishTitlesBurst = 60, 30
	// getExternalTitleImage and getWishGenreBackdrop: 600 a minute, 300 at once (one view of
	// «Entdecken» with room to scroll).
	wishImagesPerMin, wishImagesBurst = 600, 300
	// getWishDiscover, listWishDiscoverRow, listWishTitles and listWishGenres: 120 a minute, 60 at
	// once.
	wishPagesPerMin, wishPagesBurst = 120, 60
	// searchWishTitles: 30 a minute, as searchExternalTitles.
	wishSearchPerMin, wishSearchBurst = 30, 30
)

// wishLimiters are the buckets per profile of the operations on titles outside the library.
type wishLimiters struct {
	titles, images, pages, search *auth.Limiter
}

func newWishLimiters() wishLimiters {
	return wishLimiters{
		titles: auth.NewLimiter(wishTitlesPerMin, wishTitlesBurst),
		images: auth.NewLimiter(wishImagesPerMin, wishImagesBurst),
		pages:  auth.NewLimiter(wishPagesPerMin, wishPagesBurst),
		search: auth.NewLimiter(wishSearchPerMin, wishSearchBurst),
	}
}

// wishAllow takes a token of the profile from l; model.ErrTooManyAttempts (429) when it is empty.
func (s *server) wishAllow(l *auth.Limiter, p model.Profile) error {
	return l.Allow(strconv.FormatInt(p.ID, 10), s.opts.now())
}
