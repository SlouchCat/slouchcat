package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// heroPicker names the first slide of the start page's hero carousel. The catalog service
// (discover.Service) implements it; NewHandler takes it from Options.Catalog by type assertion, so
// CatalogService stays as it is and a catalog without the method simply sends no preload.
type heroPicker interface {
	// FirstPick returns the first title of the picks row or model.ErrNotFound.
	FirstPick(ctx context.Context, profile model.Profile) (model.ItemSummary, error)
}

// Width descriptors of the hero srcset; they must match imageSource in web/src/ui/HeroCarousel.tsx.
const (
	heroSmallWidth = "780w"
	heroLargeWidth = "1280w"
)

// startPreloads are the API reads the start page makes first (router guard and home query). The
// web client sends reads without custom headers, so its fetch() reuses these preloads.
var startPreloads = []string{
	"<" + BasePath + "/auth/session>; rel=preload; as=fetch; crossorigin",
	"<" + BasePath + "/home>; rel=preload; as=fetch; crossorigin",
}

// startPagePreload adds Link preload headers to the app shell of the start page (P-10): the session
// and home data and the hero image. The browser fetches them while it still loads the JavaScript,
// instead of after the app has run. Only GET / with a valid session gets the headers; the image is
// the first pick of the session's profile, read through the catalog service and thus the
// visibility layer, and is left out on any error. The shell itself is always served. /index.html
// is left out on purpose: the service worker precaches it, and a stored header would preload a
// stale image later.
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
			session, token, extended, ok := resolveSession(r.Context(), auth, r)
			if ok {
				if extended {
					// Same as the API middleware: a sliding extension re-issues the cookie.
					setSessionCookie(w, r, token, session.ExpiresAt)
				}
				for _, link := range startPreloads {
					w.Header().Add("Link", link)
				}
				item, err := picker.FirstPick(r.Context(), session.Profile)
				switch {
				case err == nil:
					if link := heroLink(item); link != "" {
						w.Header().Add("Link", link)
					}
				case !errors.Is(err, model.ErrNotFound) && !errors.Is(err, context.Canceled):
					log.WarnContext(r.Context(), "hero preload skipped", "err", err)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// heroLink returns the Link header value that preloads the image the hero shows for item, chosen
// exactly like the carousel does (same URLs, srcset and sizes), or "" when the item has no image.
func heroLink(item model.ItemSummary) string {
	if item.Backdrop != nil && item.BackdropSmall != nil {
		small, large := imageURL(item.ID, *item.BackdropSmall), imageURL(item.ID, *item.Backdrop)
		return "<" + small + `>; rel=preload; as=image; imagesrcset="` + small + " " + heroSmallWidth + ", " +
			large + " " + heroLargeWidth + `"; imagesizes="100vw"; fetchpriority=high`
	}
	for _, img := range []*model.Image{item.Backdrop, item.BackdropSmall, item.Poster} {
		if img != nil {
			return "<" + imageURL(item.ID, *img) + ">; rel=preload; as=image; fetchpriority=high"
		}
	}
	return ""
}
