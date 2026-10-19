package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Session cookie and CSRF header (plan 3.8 point 7, contract description).
const (
	SessionCookie     = "slouchcat_session"
	CSRFHeader        = "X-Requested-With"
	CSRFHeaderValue   = "Slouchcat"
	sessionCookieDays = 30
)

type sessionKey struct{}

type requestKey struct{}

// requestFrom returns the HTTP request stored by the authorize middleware, so strict handlers can
// read request properties (TLS, user agent) that the generated interface does not pass on.
func requestFrom(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestKey{}).(*http.Request)
	return r
}

// sessionFrom returns the session of the request, if the middleware resolved one.
func sessionFrom(ctx context.Context) (model.Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(model.Session)
	return s, ok
}

// visibilityOf maps "METHOD pattern" (pattern as chi reports it, with the base path) to the
// x-visibility marker of the contract. Every route the generated server registers is covered;
// a route without a marker fails closed (401) in the middleware.
func visibilityOf(spec *openapi3.T, base string) map[string]string {
	out := map[string]string{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			vis, _ := op.Extensions["x-visibility"].(string)
			out[method+" "+base+path] = vis
		}
	}
	return out
}

// csrfOK accepts safe methods and state-changing requests that carry the custom header. Browsers
// never add it to cross-site form posts, so together with SameSite=Lax the cookie cannot be
// replayed from another origin. Checked after routing, so unknown routes still answer 404/405.
func csrfOK(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return r.Header.Get(CSRFHeader) == CSRFHeaderValue
	}
}

// authorize runs after routing (chi knows the pattern) and enforces the x-visibility of the
// operation: public passes, scoped needs a session, admin needs the owner. It stores the session
// in the context for the handlers. The cookie is read for every operation, so a public operation
// can still see who is logged in.
func (p problems) authorize(auth AuthService, visibility map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !csrfOK(r) {
				p.write(w, r, http.StatusForbidden, "csrf_header_missing",
					"State-changing requests need the header "+CSRFHeader+": "+CSRFHeaderValue+".")
				return
			}
			ctx := context.WithValue(r.Context(), requestKey{}, r)
			session, token, extended, hasSession := resolveSession(ctx, auth, r)
			if hasSession {
				ctx = context.WithValue(ctx, sessionKey{}, session)
			}
			if extended {
				// Sliding expiry: the browser keeps the cookie as long as the server keeps the session.
				setSessionCookie(w, r, token, session.ExpiresAt)
			}
			r = r.WithContext(ctx)
			pattern := chi.RouteContext(ctx).RoutePattern()
			vis, known := visibility[r.Method+" "+pattern]
			switch {
			case known && vis == "public":
			case !known || vis == "system":
				p.log.WarnContext(ctx, "route without usable x-visibility", "method", r.Method, "pattern", pattern, "visibility", vis)
				p.write(w, r, http.StatusUnauthorized, "unauthenticated", "")
				return
			case !hasSession:
				clearSessionCookie(w, r)
				p.write(w, r, http.StatusUnauthorized, "unauthenticated", "A valid session is required.")
				return
			case vis == "admin" && !session.IsOwner():
				p.write(w, r, http.StatusForbidden, "forbidden", "Only the main account may do this.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveSession returns the session of the cookie with its token and whether Resolve extended it.
func resolveSession(ctx context.Context, auth AuthService, r *http.Request) (session model.Session, token string, extended, ok bool) {
	if auth == nil {
		return model.Session{}, "", false, false
	}
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return model.Session{}, "", false, false
	}
	s, extended, err := auth.Resolve(ctx, c.Value)
	if err != nil {
		return model.Session{}, "", false, false
	}
	return s, c.Value, extended, true
}

// clientIP is the address the login and setup attempt limit counts: the first X-Forwarded-For
// entry when it is a valid address (reverse proxy), else the host of RemoteAddr.
func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if ip, err := netip.ParseAddr(strings.TrimSpace(first)); err == nil {
			return ip.Unmap().String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// setSessionCookie writes the session cookie. Secure is set when the request arrived over TLS,
// directly or through a reverse proxy (X-Forwarded-Proto), so plain-HTTP LAN use keeps working.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure depends on the transport, plain HTTP in the LAN is a supported setup (K-8 adds HTTPS via Caddy)
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   isTLS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: see setSessionCookie
		Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isTLS(r), SameSite: http.SameSiteLaxMode,
	})
}

func isTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// sessionExpiry is the sliding lifetime of a browser session.
func sessionExpiry(now time.Time) time.Time { return now.Add(sessionCookieDays * 24 * time.Hour) }

// toSessionInfo maps a session to the contract type.
func toSessionInfo(s model.Session) gen.SessionInfo {
	return gen.SessionInfo{
		Account: gen.Account{Id: s.Account.ID, Email: types.Email(s.Account.Email), Role: gen.Owner},
		Profile: toProfile(s.Profile),
	}
}

func toProfile(p model.Profile) gen.Profile {
	out := gen.Profile{Id: p.ID, Name: p.Name, MaxRating: gen.Rating(p.MaxRating), Language: gen.Language(p.Language)}
	if p.Accent != "" {
		out.Accent = &p.Accent
	}
	return out
}

// mapError turns a service error into a problem response. Handlers call it for every error they
// do not handle themselves.
func (p problems) mapError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *model.ValidationError
	switch {
	case errors.Is(err, model.ErrNotFound):
		p.write(w, r, http.StatusNotFound, "not_found", "")
	case errors.Is(err, model.ErrInvalidCredentials):
		p.write(w, r, http.StatusUnauthorized, "invalid_credentials", "Unknown e-mail or wrong password.")
	case errors.Is(err, model.ErrTooManyAttempts):
		w.Header().Set("Retry-After", "60")
		p.write(w, r, http.StatusTooManyRequests, "too_many_attempts", "Too many attempts; wait a minute and try again.")
	case errors.Is(err, model.ErrSetupDone):
		p.write(w, r, http.StatusConflict, "setup_done", "The setup wizard already ran.")
	case errors.Is(err, model.ErrLibraryPathInvalid):
		p.write(w, r, http.StatusUnprocessableEntity, "library_path_invalid", "The library path must be an absolute, readable directory on the server.")
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		p.write(w, r, http.StatusUnprocessableEntity, "tmdb_key_invalid", "TMDB rejected the key.")
	case errors.Is(err, model.ErrUnauthenticated):
		p.write(w, r, http.StatusUnauthorized, "unauthenticated", "The stream token is missing, invalid, revoked or expired.")
	case errors.Is(err, model.ErrTranscodeBusy):
		w.Header().Set("Retry-After", "30")
		p.write(w, r, http.StatusServiceUnavailable, "transcode_busy", "The server is busy transcoding; try again later.")
	case errors.Is(err, model.ErrTMDBUnreachable):
		p.write(w, r, http.StatusUnprocessableEntity, "tmdb_unreachable", "TMDB did not answer; try again or leave the key empty.")
	case errors.As(err, &verr):
		p.writeValidation(w, r, verr)
	case errors.Is(err, context.Canceled):
		// client went away; nothing to send
	default:
		p.internal(w, r, err)
	}
}

func (p problems) writeValidation(w http.ResponseWriter, r *http.Request, verr *model.ValidationError) {
	fields := make([]gen.FieldError, 0, len(verr.Fields))
	for _, f := range verr.Fields {
		fields = append(fields, gen.FieldError{Field: f.Field, Code: f.Code})
	}
	p.writeProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "One or more fields are invalid.", fields)
}
