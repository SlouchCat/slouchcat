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

// heroPicker names the title whose image the start page shows first: the title the stage begins
// with (HomePage.heroRow, plan 3.5; drawn once per page view, concept 08, 6) or, without a stage,
// the first title of the first row. The catalog
// service (discover.Service) implements it; NewHandler takes it from Options.Catalog by type
// assertion, so CatalogService stays as it is and a catalog without the methods simply sends no
// image preload.
type heroPicker interface {
	// FirstHero draws the title the stage begins with and returns it with its index in the stage
	// row and whether that row is the first row of the start page, or model.ErrNotFound when the
	// page has no stage.
	FirstHero(ctx context.Context, profile model.Profile) (item model.ItemSummary, start int, first bool, err error)
	// FirstRow returns the titles of the first row when the page has no stage, or
	// model.ErrNotFound when it has one or its first row holds no titles.
	FirstRow(ctx context.Context, profile model.Profile) ([]model.ItemSummary, error)
}

// Width descriptors and sizes of the hero srcset; they must match imageSource and HERO_SIZES in
// web/src/ui/HeroCarousel.tsx. From 768 px the side navigation's column (15rem, --sidenav-width in
// web/src/ui/global.css) takes its width off the stage (concept 08).
const (
	heroSmallWidth = "780w"
	heroLargeWidth = "1280w"
	heroSizes      = "(width >= 768px) calc(100vw - 15rem), 100vw"
)

// Width descriptors and sizes of the poster srcset; they must match srcSet and CARD_SIZES in
// web/src/ui/PosterCard.tsx, or the browser loads the card's image a second time. Under 768 px a
// row shows three whole cards (concept 08, N-3; Row.module.css and #poster-prepaint in
// web/src/ui/global.css).
const (
	posterSmallWidth = "200w"
	posterLargeWidth = "400w"
	posterSizes      = "(width >= 1440px) 12.5rem, (width >= 768px) 10.5rem, calc(33vw - 1.5rem)"
)

// posterPrepaintCards is the number of cards the shell paints ahead on a start page without a
// stage: the eagerly loaded cards of the first row (EAGER_POSTERS in web/src/features/catalog/
// HomePage.tsx).
const posterPrepaintCards = 6

// heroMarker is the comment in web/index.html where the start page's shell receives its first
// images: the first hero image (heroPrepaint) or, without a stage, the first cards of the first row
// (posterPrepaint). Every GET / replaces it, with nothing when there is no image.
const heroMarker = "<!--hero-->"

// heroStartMeta is the element that hands the stage's drawn start index to the client when the
// shell paints no stage image (shellHeroStart in web/src/features/catalog/queries.ts).
const heroStartMeta = "slouchcat-hero-start"

// startPreloads are the API reads the start page makes first (router guard, home query and the
// side navigation's libraries, concept 08). The web client sends reads without custom headers, so
// its fetch() reuses these preloads. With a stage, the home read carries the shell's draw
// (?start=K, concept 08, 6), the URL the client's first home query asks for; start < 0 means no
// stage.
func startPreloads(start int) []string {
	home := BasePath + "/home"
	if start >= 0 {
		home += "?start=" + strconv.Itoa(start)
	}
	return []string{
		"<" + BasePath + "/auth/session>; rel=preload; as=fetch; crossorigin",
		"<" + home + ">; rel=preload; as=fetch; crossorigin",
		"<" + BasePath + "/libraries/visible>; rel=preload; as=fetch; crossorigin",
	}
}

// startPagePreload adds Link preload headers to the app shell of the start page (P-10): the session,
// home and library data and the first image. The browser fetches them while it still loads the JavaScript,
// instead of after the app has run. Only GET / with a valid session gets the headers; the image is
// the first title of the stage of the session's profile or, without a stage, the poster of the
// first title of the first row (the first card), read through the catalog service and thus the
// visibility layer, and is left out on any error. The shell itself is always served. /index.html
// is left out on purpose: the service worker precaches it, and a stored header would preload a
// stale image later.
//
// With a stage the shell draws the title the stage begins with (FirstHero, concept 08, 6: one draw
// per page view) and hands its index K to the client twice: in the home preload (/home?start=K)
// and in the shell, as data-start on #hero-prepaint or, without that image, on a
// <meta name="slouchcat-hero-start">. The client's first home query sends start=K, so it reuses
// the preload and the band begins with the painted title.
//
// When the stage is the first row of the start page, the same request also writes its first image
// into the shell (heroPrepaint), so the browser paints it as soon as it has loaded instead of after
// the JavaScript has run. The prepaint sits at the top of the page, so a stage further down gets
// only the preload header (plan 3.5): painting it ahead would flash a picture in the wrong place.
// Without a stage the same request writes the first cards' posters into the shell instead
// (posterPrepaint), at the place of the first row's cards.
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
				item, start, first, err := firstHero(r.Context(), picker, session)
				if err != nil {
					start = -1
				}
				if session.HasProfile() {
					for _, link := range startPreloads(start) {
						w.Header().Add("Link", link)
					}
				}
				link := ""
				switch {
				case err == nil:
					link = heroLink(item, session.Profile.ID)
					if first {
						prepaint = heroPrepaint(item, start, session.Profile.ID)
					}
					if prepaint == "" {
						// No stage image ahead (a stage further down, or no backdrop): the draw
						// still reaches the client.
						prepaint = `<meta name="` + heroStartMeta + `" content="` + strconv.Itoa(start) + `">`
					}
				case errors.Is(err, model.ErrNotFound) && session.HasProfile():
					// Without a stage the first card's poster is the page's first image: it is
					// preloaded, and the first cards of the row are painted ahead.
					var items []model.ItemSummary
					if items, err = picker.FirstRow(r.Context(), session.Profile); err == nil && len(items) > 0 {
						link = posterLink(items[0], session.Profile.ID)
						prepaint = posterPrepaint(items, session.Profile.ID)
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

// firstHero draws the stage's first title of the session's profile; a session without profile has
// none.
func firstHero(ctx context.Context, picker heroPicker, session model.Session) (model.ItemSummary, int, bool, error) {
	if !session.HasProfile() {
		return model.ItemSummary{}, 0, false, model.ErrNotFound
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
	_, _ = s.ResponseWriter.Write(body) //nolint:gosec // G705: the only inserted text is heroPrepaint's or posterPrepaint's output or the start meta, built from html-escaped values and integers (profile id, start index)
}

// heroPrepaint returns the first hero image as markup for the app shell, or "" when the item has
// no backdrop (a poster is shown blurred by the carousel and is not painted ahead). It sits outside
// #root with the carousel's geometry (#hero-prepaint in web/src/ui/global.css) and the attributes
// of the carousel's active image (URLs, srcset, sizes, priority, sync decoding); the start page
// fades it out once its own image is on screen. data-start is the item's index in the stage row,
// the draw the client's first home query repeats (concept 08, 6).
func heroPrepaint(item model.ItemSummary, start int, profileID int64) string {
	var attrs string
	switch {
	case item.Backdrop != nil && item.BackdropSmall != nil:
		small, large := imageURL(item.ID, *item.BackdropSmall, profileID), imageURL(item.ID, *item.Backdrop, profileID)
		attrs = `src="` + html.EscapeString(small) + `" srcset="` +
			html.EscapeString(small+" "+heroSmallWidth+", "+large+" "+heroLargeWidth) + `" sizes="` + heroSizes + `"`
	case item.Backdrop != nil:
		attrs = `src="` + html.EscapeString(imageURL(item.ID, *item.Backdrop, profileID)) + `"`
	case item.BackdropSmall != nil:
		attrs = `src="` + html.EscapeString(imageURL(item.ID, *item.BackdropSmall, profileID)) + `"`
	default:
		return ""
	}
	return `<div id="hero-prepaint" data-start="` + strconv.Itoa(start) + `" aria-hidden="true"><img ` + attrs +
		` alt="" width="1280" height="720" fetchpriority="high" decoding="sync"></div>`
}

// posterPrepaint returns the first cards of a start page without a stage as markup for the app
// shell: up to posterPrepaintCards posters, nothing but images. It sits outside #root with the
// geometry of the first row's cards (#poster-prepaint in web/src/ui/global.css; a square cover for
// audiobooks and podcasts, like PosterCard's shape) and the attributes of their images (URLs,
// srcset, sizes; the first one with priority and sync decoding, like the preload); the start page
// fades it out once its own first poster is on screen. It stops at the first title without a
// poster: the card shows the lettered fallback there, and a gap would misplace the cards after it.
func posterPrepaint(items []model.ItemSummary, profileID int64) string {
	var b strings.Builder
	for i, item := range items[:min(len(items), posterPrepaintCards)] {
		var attrs string
		switch {
		case item.Poster != nil && item.PosterSmall != nil:
			small, large := imageURL(item.ID, *item.PosterSmall, profileID), imageURL(item.ID, *item.Poster, profileID)
			attrs = `src="` + html.EscapeString(small) + `" srcset="` +
				html.EscapeString(small+" "+posterSmallWidth+", "+large+" "+posterLargeWidth) +
				`" sizes="` + posterSizes + `"`
		case item.Poster != nil:
			attrs = `src="` + html.EscapeString(imageURL(item.ID, *item.Poster, profileID)) + `"`
		case item.PosterSmall != nil:
			attrs = `src="` + html.EscapeString(imageURL(item.ID, *item.PosterSmall, profileID)) + `"`
		}
		if attrs == "" {
			break
		}
		b.WriteString("<img ")
		if squareCover(item.Kind) {
			b.WriteString(`class="square" `)
		}
		b.WriteString(attrs + ` alt="" loading="eager"`)
		if i == 0 {
			b.WriteString(` fetchpriority="high" decoding="sync"`)
		}
		b.WriteString(">")
	}
	if b.Len() == 0 {
		return ""
	}
	return `<div id="poster-prepaint" aria-hidden="true">` + b.String() + `</div>`
}

// squareCover reports whether a card shows a square cover instead of a 2:3 poster: audiobooks and
// podcasts (isAudioKind in web/src/features/catalog/labels.ts, M6).
func squareCover(kind string) bool {
	return kind == "audiobook" || kind == "podcast" || kind == "podcast_episode"
}

// heroLink returns the Link header value that preloads the image the hero shows for item, chosen
// exactly like the carousel does (same URLs, srcset and sizes), or "" when the item has no image.
func heroLink(item model.ItemSummary, profileID int64) string {
	if item.Backdrop != nil && item.BackdropSmall != nil {
		small, large := imageURL(item.ID, *item.BackdropSmall, profileID), imageURL(item.ID, *item.Backdrop, profileID)
		return "<" + small + `>; rel=preload; as=image; imagesrcset="` + small + " " + heroSmallWidth + ", " +
			large + " " + heroLargeWidth + `"; imagesizes="` + heroSizes + `"; fetchpriority=high`
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
