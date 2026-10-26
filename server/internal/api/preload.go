package api

import (
	"bytes"
	"context"
	"errors"
	"html"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// heroPicker names the title whose image the start page shows first: the first title of the stage
// (HomePage.heroRow, plan 3.5) or, without a stage, the first title of the first row. The catalog
// service (discover.Service) implements it; NewHandler takes it from Options.Catalog by type
// assertion, so CatalogService stays as it is and a catalog without the methods simply sends no
// image preload.
type heroPicker interface {
	// FirstHero returns the first title of the stage row and whether that row is the first row of
	// the start page, or model.ErrNotFound when the page has no stage.
	FirstHero(ctx context.Context, profile model.Profile) (item model.ItemSummary, first bool, err error)
	// FirstPoster returns the first title of the first row when the page has no stage, or
	// model.ErrNotFound when it has one or its first row holds no titles.
	FirstPoster(ctx context.Context, profile model.Profile) (model.ItemSummary, error)
}

// Width descriptors of the hero srcset; they must match imageSource in web/src/ui/HeroCarousel.tsx.
const (
	heroSmallWidth = "780w"
	heroLargeWidth = "1280w"
)

// Width descriptors and sizes of the poster srcset; they must match srcSet and CARD_SIZES in
// web/src/ui/PosterCard.tsx, or the browser loads the card's image a second time.
const (
	posterSmallWidth = "200w"
	posterLargeWidth = "400w"
	posterSizes      = "(width >= 1440px) 12.5rem, (width >= 768px) 10.5rem, 8.5rem"
)

// heroMarker is the comment in web/index.html where the start page's shell receives the first hero
// image (heroPrepaint); every GET / replaces it, with nothing when there is no image.
const heroMarker = "<!--hero-->"

// startPreloads are the API reads the start page makes first (router guard and home query). The
// web client sends reads without custom headers, so its fetch() reuses these preloads.
var startPreloads = []string{
	"<" + BasePath + "/auth/session>; rel=preload; as=fetch; crossorigin",
	"<" + BasePath + "/home>; rel=preload; as=fetch; crossorigin",
}

// startPagePreload adds Link preload headers to the app shell of the start page (P-10): the session
// and home data and the first image. The browser fetches them while it still loads the JavaScript,
// instead of after the app has run. Only GET / with a valid session gets the headers; the image is
// the first title of the stage of the session's profile or, without a stage, the poster of the
// first title of the first row (the first card), read through the catalog service and thus the
// visibility layer, and is left out on any error. The shell itself is always served. /index.html
// is left out on purpose: the service worker precaches it, and a stored header would preload a
// stale image later.
//
// When the stage is the first row of the start page, the same request also writes its first image
// into the shell (heroPrepaint), so the browser paints it as soon as it has loaded instead of after
// the JavaScript has run. The prepaint sits at the top of the page, so a stage further down gets
// only the preload header (plan 3.5): painting it ahead would flash a picture in the wrong place.
//
// The shell is sent with Cache-Control: no-cache (webassets), so the header is computed per
// request; Vary: Cookie keeps shared caches from handing it to another viewer.
func startPagePreload(auth AuthService, picker heroPicker, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if auth == nil || picker == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/" {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Add("Vary", "Cookie")
			prepaint := ""
			session, token, extended, ok := resolveSession(r.Context(), auth, r)
			if ok {
				if extended {
					// Same as the API middleware: a sliding extension re-issues the cookie.
					setSessionCookie(w, r, token, session.ExpiresAt)
				}
				if session.HasProfile() {
					for _, link := range startPreloads {
						w.Header().Add("Link", link)
					}
				}
				item, first, err := firstHero(r.Context(), picker, session)
				link := ""
				switch {
				case err == nil:
					link = heroLink(item, session.Profile.ID)
					if first {
						prepaint = heroPrepaint(item, session.Profile.ID)
					}
				case errors.Is(err, model.ErrNotFound) && session.HasProfile():
					// Without a stage the first card's poster is the page's first image; the app
					// draws the row, so it gets the header only and no prepaint.
					if item, err = picker.FirstPoster(r.Context(), session.Profile); err == nil {
						link = posterLink(item, session.Profile.ID)
					}
				}
				if link != "" {
					w.Header().Add("Link", link)
				}
				if err != nil && !errors.Is(err, model.ErrNotFound) && !errors.Is(err, context.Canceled) {
					log.WarnContext(r.Context(), "image preload skipped", "err", err)
				}
			}
			shell := &shellWriter{ResponseWriter: w}
			next.ServeHTTP(shell, r)
			shell.flush(prepaint)
		})
	}
}

// firstHero is the first stage title of the session's profile; a session without profile has none.
func firstHero(ctx context.Context, picker heroPicker, session model.Session) (model.ItemSummary, bool, error) {
	if !session.HasProfile() {
		return model.ItemSummary{}, false, model.ErrNotFound
	}
	return picker.FirstHero(ctx, session.Profile)
}

// shellWriter holds back the app shell so startPagePreload can fill in the hero marker. The shell
// is a small file; anything other than a 200 HTML answer passes through unchanged.
type shellWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (s *shellWriter) WriteHeader(status int) {
	if s.status == 0 {
		s.status = status
	}
}

func (s *shellWriter) Write(p []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.body.Write(p)
}

func (s *shellWriter) flush(prepaint string) {
	body := s.body.Bytes()
	if s.status == 0 {
		s.status = http.StatusOK
	}
	if s.status == http.StatusOK && strings.HasPrefix(s.Header().Get("Content-Type"), "text/html") {
		body = bytes.Replace(body, []byte(heroMarker), []byte(prepaint), 1)
		s.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	s.ResponseWriter.WriteHeader(s.status)
	_, _ = s.ResponseWriter.Write(body) //nolint:gosec // G705: the only inserted text is heroPrepaint's output, built from html-escaped values and an integer profile id
}

// heroPrepaint returns the first hero image as markup for the app shell, or "" when the item has
// no backdrop (a poster is shown blurred by the carousel and is not painted ahead). It sits outside
// #root with the carousel's geometry (#hero-prepaint in web/src/ui/global.css) and the attributes
// of the carousel's active image (URLs, srcset, sizes, priority, sync decoding); the start page
// fades it out once its own image is on screen.
func heroPrepaint(item model.ItemSummary, profileID int64) string {
	var attrs string
	switch {
	case item.Backdrop != nil && item.BackdropSmall != nil:
		small, large := imageURL(item.ID, *item.BackdropSmall, profileID), imageURL(item.ID, *item.Backdrop, profileID)
		attrs = `src="` + html.EscapeString(small) + `" srcset="` +
			html.EscapeString(small+" "+heroSmallWidth+", "+large+" "+heroLargeWidth) + `" sizes="100vw"`
	case item.Backdrop != nil:
		attrs = `src="` + html.EscapeString(imageURL(item.ID, *item.Backdrop, profileID)) + `"`
	case item.BackdropSmall != nil:
		attrs = `src="` + html.EscapeString(imageURL(item.ID, *item.BackdropSmall, profileID)) + `"`
	default:
		return ""
	}
	return `<div id="hero-prepaint" aria-hidden="true"><img ` + attrs +
		` alt="" width="1280" height="720" fetchpriority="high" decoding="sync"></div>`
}

// heroLink returns the Link header value that preloads the image the hero shows for item, chosen
// exactly like the carousel does (same URLs, srcset and sizes), or "" when the item has no image.
func heroLink(item model.ItemSummary, profileID int64) string {
	if item.Backdrop != nil && item.BackdropSmall != nil {
		small, large := imageURL(item.ID, *item.BackdropSmall, profileID), imageURL(item.ID, *item.Backdrop, profileID)
		return "<" + small + `>; rel=preload; as=image; imagesrcset="` + small + " " + heroSmallWidth + ", " +
			large + " " + heroLargeWidth + `"; imagesizes="100vw"; fetchpriority=high`
	}
	for _, img := range []*model.Image{item.Backdrop, item.BackdropSmall, item.Poster} {
		if img != nil {
			return "<" + imageURL(item.ID, *img, profileID) + ">; rel=preload; as=image; fetchpriority=high"
		}
	}
	return ""
}

// posterLink returns the Link header value that preloads the poster a PosterCard shows for item
// (same URLs, srcset and sizes), or "" when the item has no poster.
func posterLink(item model.ItemSummary, profileID int64) string {
	if item.Poster != nil && item.PosterSmall != nil {
		small, large := imageURL(item.ID, *item.PosterSmall, profileID), imageURL(item.ID, *item.Poster, profileID)
		return "<" + small + `>; rel=preload; as=image; imagesrcset="` + small + " " + posterSmallWidth + ", " +
			large + " " + posterLargeWidth + `"; imagesizes="` + posterSizes + `"; fetchpriority=high`
	}
	for _, img := range []*model.Image{item.Poster, item.PosterSmall} {
		if img != nil {
			return "<" + imageURL(item.ID, *img, profileID) + ">; rel=preload; as=image; fetchpriority=high"
		}
	}
	return ""
}
