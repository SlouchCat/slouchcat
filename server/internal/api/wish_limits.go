package api

import (
	"strconv"
	"time"

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
//
// Each call also takes a token of the account (security review L-1): a household has up to ten
// profiles (auth.MaxProfiles), and a new profile would otherwise start with a full bucket. Ten
// profiles rarely browse at the same moment; the account bucket lets four of them browse at the
// full pace of one profile (or all ten at 40 % of it), so it holds four times the bucket of one
// profile and refills four times as fast.
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
	// wishAccountFactor: the bucket of an account against that of one profile.
	wishAccountFactor = 4
)

// wishBucket is the bucket of one group per profile and per account.
type wishBucket struct {
	profile, account *auth.Limiter
}

func newWishBucket(perMin, burst float64) wishBucket {
	return wishBucket{profile: auth.NewLimiter(perMin, burst), account: auth.NewLimiter(perMin*wishAccountFactor, burst*wishAccountFactor)}
}

// allow takes a token of the profile and one of its account; model.ErrTooManyAttempts when either
// bucket is empty.
func (l wishBucket) allow(p model.Profile, now time.Time) error {
	if err := l.account.Allow(strconv.FormatInt(p.AccountID, 10), now); err != nil {
		return err
	}
	return l.profile.Allow(strconv.FormatInt(p.ID, 10), now)
}

// wishLimiters are the buckets of the operations on titles outside the library.
type wishLimiters struct {
	titles, images, pages, search wishBucket
}

func newWishLimiters() wishLimiters {
	return wishLimiters{
		titles: newWishBucket(wishTitlesPerMin, wishTitlesBurst),
		images: newWishBucket(wishImagesPerMin, wishImagesBurst),
		pages:  newWishBucket(wishPagesPerMin, wishPagesBurst),
		search: newWishBucket(wishSearchPerMin, wishSearchBurst),
	}
}

// wishAllow takes a token of the profile and its account from l; model.ErrTooManyAttempts (429)
// when one is empty.
func (s *server) wishAllow(l wishBucket, p model.Profile) error {
	return l.allow(p, s.opts.now())
}
