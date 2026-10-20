package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
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
	DeviceCookie      = "slouchcat_device"
	CSRFHeader        = "X-Requested-With"
	CSRFHeaderValue   = "Slouchcat"
	sessionCookieDays = 30
	deviceCookieDays  = 400
)

var deviceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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
// operation: public passes; account needs a session (profile optional); scoped needs a session with
// an active profile (403 profile_required without); admin needs the owner with the primary profile
// active (403 forbidden otherwise). It stores the session in the context for the handlers. The
// cookie is read for every operation, so a public operation can still see who is logged in. A
// request without a valid device cookie gets one (PIN locks count per profile and device).
func (p problems) authorize(auth AuthService, visibility map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !csrfOK(r) {
				p.write(w, r, http.StatusForbidden, "csrf_header_missing",
					"State-changing requests need the header "+CSRFHeader+": "+CSRFHeaderValue+".")
				return
			}
			ctx := context.WithValue(r.Context(), requestKey{}, r)
			ctx = model.WithDeviceID(ctx, ensureDeviceCookie(w, r))
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
			case !known || vis == "system" || !validVisibility[vis]:
				p.log.WarnContext(ctx, "route without usable x-visibility", "method", r.Method, "pattern", pattern, "visibility", vis)
				p.write(w, r, http.StatusUnauthorized, "unauthenticated", "")
				return
			case !hasSession:
				clearSessionCookie(w, r)
				p.write(w, r, http.StatusUnauthorized, "unauthenticated", "A valid session is required.")
				return
			case vis == "admin" && (!session.IsOwner() || !session.IsPrimary()):
				p.write(w, r, http.StatusForbidden, "forbidden", "Only the main account with its primary profile may do this.")
				return
			case vis == "scoped" && !session.HasProfile():
				p.write(w, r, http.StatusForbidden, "profile_required", "Choose a profile first.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// validVisibility lists the x-visibility values the middleware enforces; any other value fails
// closed.
var validVisibility = map[string]bool{"public": true, "account": true, "scoped": true, "admin": true}

// ensureDeviceCookie returns the device id of the request and sets the cookie when the request has
// none (or a malformed one): 128 random bits as 32 hex characters.
func ensureDeviceCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(DeviceCookie); err == nil && deviceIDPattern.MatchString(c.Value) {
		return c.Value
	}
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails
	id := hex.EncodeToString(b[:])
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure depends on the transport, see setSessionCookie
		Name:     DeviceCookie,
		Value:    id,
		Path:     "/",
		MaxAge:   deviceCookieDays * 24 * 3600,
		HttpOnly: true,
		Secure:   isTLS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return id
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

// clientIP is the address the attempt limits count. Without trusted proxies it is the host of
// RemoteAddr and X-Forwarded-For is ignored (anyone could set it). When RemoteAddr is a trusted
// proxy, the address is the last X-Forwarded-For entry that is not itself a trusted proxy (the
// client as the nearest proxy saw it), or the first entry when every hop is trusted.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote, err := netip.ParseAddr(host)
	if err != nil || len(trusted) == 0 || !inPrefixes(trusted, remote.Unmap()) {
		return host
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return remote.Unmap().String() // a malformed chain is not believed
		}
		if ip = ip.Unmap(); !inPrefixes(trusted, ip) || i == 0 {
			return ip.String()
		}
	}
	return remote.Unmap().String()
}

func inPrefixes(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// clientAddr is clientIP of the request in ctx with the configured trusted proxies.
func (s *server) clientAddr(ctx context.Context) string {
	return clientIP(requestFrom(ctx), s.opts.TrustedProxies)
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

// toSessionInfo maps a session to the contract type; the profile is absent until one is chosen.
func toSessionInfo(s model.Session) gen.SessionInfo {
	info := gen.SessionInfo{Account: toAccount(s.Account)}
	if s.HasProfile() {
		p := toProfile(s.Profile)
		info.Profile = &p
	}
	return info
}

func toAccount(a model.Account) gen.Account {
	role := gen.AccountRoleOwner
	if a.Role == "guest" {
		role = gen.AccountRoleGuest
	}
	return gen.Account{
		Id: a.ID, Email: types.Email(a.Email), Role: role, MaxRating: gen.Rating(a.MaxRating),
		AllLibraries: a.AllLibraries, LibraryIds: nonNil(a.LibraryIDs),
	}
}

func toProfile(p model.Profile) gen.Profile {
	out := gen.Profile{
		Id: p.ID, Name: p.Name, MaxRating: gen.Rating(p.MaxRating), Language: gen.Language(p.Language),
		AllLibraries: p.AllLibraries, LibraryIds: nonNil(p.LibraryIDs), HasPin: p.HasPin, IsPrimary: p.IsPrimary,
	}
	if p.Accent != "" {
		a := gen.Accent(p.Accent)
		out.Accent = &a
	}
	return out
}

func toProfileSummary(p model.Profile) gen.ProfileSummary {
	out := gen.ProfileSummary{Id: p.ID, Name: p.Name, HasPin: p.HasPin, IsPrimary: p.IsPrimary}
	if p.Accent != "" {
		a := gen.Accent(p.Accent)
		out.Accent = &a
	}
	return out
}

func nonNil(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
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
	case errors.Is(err, model.ErrPinLocked):
		var locked *model.PinLockedError
		secs := 1
		if errors.As(err, &locked) {
			secs = max(1, int((locked.RetryAfter+time.Second-1)/time.Second))
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		p.writeProblem(w, r, http.StatusTooManyRequests, "pin_locked", "Too many wrong PINs; the profile is locked on this device.", nil, &secs)
	case errors.Is(err, model.ErrPinRequired):
		p.write(w, r, http.StatusForbidden, "pin_required", "This profile is protected by a PIN.")
	case errors.Is(err, model.ErrPinInvalid):
		p.write(w, r, http.StatusForbidden, "pin_invalid", "The PIN is wrong.")
	case errors.Is(err, model.ErrPrimaryProfileRequired):
		p.write(w, r, http.StatusForbidden, "primary_profile_required", "Only the account's primary profile may manage profiles.")
	case errors.Is(err, model.ErrPrimaryProfile):
		p.write(w, r, http.StatusConflict, "primary_profile", "The primary profile of an account cannot be deleted.")
	case errors.Is(err, model.ErrInvitationGone):
		p.write(w, r, http.StatusGone, "invitation_gone", "The invitation link was used, has expired or was revoked.")
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
	case errors.Is(err, model.ErrSegmentRestarted):
		w.Header().Set("Retry-After", "1")
		p.write(w, r, http.StatusServiceUnavailable, "segment_restarted", "A newer seek restarted the encoder; retry the segment.")
	case errors.Is(err, model.ErrProviderNotConfigured):
		p.write(w, r, http.StatusUnprocessableEntity, "provider_not_configured", "The metadata provider has no key configured.")
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
	p.writeProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "One or more fields are invalid.", fields, nil)
}
