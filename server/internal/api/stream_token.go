package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Stream-token authorisation (plan 5.4): the media stream and the HLS endpoints accept no cookie;
// they are authorised by the query parameter st, which names a playback session. Native players
// (Safari, later TV clients) fetch playlists and segments without the session cookie, and the web
// app uses the same path.

type grantKey struct{}

// grantFrom returns the playback grant the streamAuth middleware resolved.
func grantFrom(ctx context.Context) (model.PlaybackGrant, bool) {
	g, ok := ctx.Value(grantKey{}).(model.PlaybackGrant)
	return g, ok
}

// streamAuth resolves the stream token of a token route: a missing, forged, revoked or expired
// token answers 401 unauthenticated. The browser session behind the token goes into the context
// like a cookie session, so the handlers read titles with its profile; the visibility of the title
// is re-checked by the playback service on every request (404 when no longer visible).
func (p problems) streamAuth(pb PlaybackService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if pb == nil {
				p.write(w, r, http.StatusUnauthorized, "unauthenticated", "")
				return
			}
			g, err := pb.Authorize(r.Context(), r.URL.Query().Get("st"))
			if err != nil {
				p.mapError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), requestKey{}, r)
			ctx = context.WithValue(ctx, sessionKey{}, g.Session)
			ctx = context.WithValue(ctx, grantKey{}, g)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// securityHeaders sets Referrer-Policy: no-referrer on every response, so a stream token in a page
// or media URL never leaves in a Referer header (plan 5.4).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
