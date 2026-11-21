// Package push is the push forwarder ("Push-Weiterleitung", concept 27 section 7.3): the app
// registers its Google token and the VAPID key of its server and gets a push address back; the
// Slouchcat server sends Web Push to that address as it would to a browser; the forwarder checks
// the VAPID signature and hands the unchanged ciphertext to Google as a data message.
//
// Status codes towards the server (rule D-10): 201 accepted; 410 only when Google no longer knows
// the token; 503 for every failure of the forwarder itself -- a missing or wrong sealing key, an
// unknown key version, missing Firebase credentials, Google or the network failing. A Slouchcat
// server deletes a subscription on 404 or 410, so 404 is never used on the push route: one broken
// key file would otherwise delete the subscriptions of every server that is sending.
package push

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/fcm"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
	"github.com/slouchcat/slouchcat/relay/internal/logging"
	"github.com/slouchcat/slouchcat/relay/internal/seal"
	"github.com/slouchcat/slouchcat/relay/internal/serve"
	"github.com/slouchcat/slouchcat/relay/internal/vapid"
)

// maxTTLSeconds is the longest Google keeps a message: four weeks.
const maxTTLSeconds = 28 * 24 * 3600

// Sender is what the handler needs of Google's push service (the fcm client, or a fake in tests).
type Sender interface {
	Send(ctx context.Context, m fcm.Message) error
}

// Handler serves /v1/register and /v1/push/{address}.
type Handler struct {
	Keys    *seal.Keys // nil: no sealing key loaded, everything but the health check is 503
	FCM     Sender     // nil: no Firebase credentials, pushes are 503
	Origins []string   // own origins ("https://push.example.test"), first is the default
	MaxBody int        // largest Web Push body in bytes
	Callers *clientip.Resolver
	// Rate limits: register and push per caller key and per larger network (/48, /24; security
	// review M-1, M-2), push per push address after the VAPID check, and one bucket for the whole
	// service in front of Google (GlobalLimit, M-2): empty, it answers 503 like any own failure.
	// A nil limiter does not limit (tests).
	RegisterLimit    *limit.Limiter
	RegisterNetLimit *limit.Limiter
	CallerLimit      *limit.Limiter
	NetLimit         *limit.Limiter
	AddressLimit     *limit.Limiter
	GlobalLimit      *limit.Limiter
	Log              *slog.Logger
	Warn             *logging.Throttle
	Now              func() time.Time
}

func allow(l *limit.Limiter, key string) bool { return l == nil || l.Allow(key) }

// Routes returns the mux of the service.
func (h *Handler) Routes() http.Handler {
	if h.Now == nil {
		h.Now = time.Now
	}
	if h.Warn == nil {
		h.Warn = logging.NewThrottle(h.Log, time.Minute)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", h.health)
	mux.HandleFunc("POST /v1/register", h.register)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		serve.Error(w, http.StatusNotFound, "not_found")
	})
	// The push route bypasses the mux: ServeMux answers an unclean path ("/v1/push/..") with a
	// redirect that ends in a 404, and a server deletes a subscription on 404 (D-10). Whatever
	// follows /v1/push/ is handed to the handler as the address and fails there with 503.
	return serve.Middleware(h.Log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address, ok := strings.CutPrefix(r.URL.Path, "/v1/push/")
		if !ok {
			mux.ServeHTTP(w, r)
			return
		}
		r.Pattern = "POST /v1/push/{address}"
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			serve.Error(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		r.SetPathValue("address", address)
		h.push(w, r)
	}))
}

// health is 200 only when the forwarder can do its work: a sealing key and Firebase credentials
// are loaded. The watch then notices a move without the key (7.5) instead of a quiet 503.
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	var problems []string
	if h.Keys == nil {
		problems = append(problems, "no_sealing_key")
	}
	if h.FCM == nil {
		problems = append(problems, "no_firebase_credentials")
	}
	if len(problems) > 0 {
		serve.JSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "problems": problems})
		return
	}
	serve.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerRequest struct {
	FCMToken string `json:"fcmToken"`
	VAPIDKey string `json:"vapidKey"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if caller, network := h.Callers.Keys(r); !allow(h.RegisterLimit, caller) || !allow(h.RegisterNetLimit, network) {
		w.Header().Set("Retry-After", "10")
		serve.Error(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	var req registerRequest
	if !serve.DecodeJSON(w, r, 8<<10, &req) {
		return
	}
	if req.FCMToken == "" || len(req.FCMToken) > 4096 || strings.ContainsAny(req.FCMToken, " \t\r\n") {
		serve.Error(w, http.StatusBadRequest, "bad_fcm_token")
		return
	}
	if _, err := vapid.ParsePublicKey(req.VAPIDKey); err != nil {
		serve.Error(w, http.StatusBadRequest, "bad_vapid_key")
		return
	}
	sealed, err := h.Keys.Seal(seal.Content{Token: req.FCMToken, VAPIDKey: strings.TrimRight(req.VAPIDKey, "=")})
	if err != nil {
		h.Log.Error("register: cannot seal", "err", err)
		serve.Error(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	serve.JSON(w, http.StatusOK, map[string]string{"endpoint": h.originFor(r) + "/v1/push/" + sealed})
}

// originFor is the own origin the request came in on, so a registration through the probe name
// gets an address under the probe name; any other host gets the first (the fixed name).
func (h *Handler) originFor(r *http.Request) string {
	for _, o := range h.Origins {
		if u, err := url.Parse(o); err == nil && strings.EqualFold(u.Host, r.Host) {
			return o
		}
	}
	return h.Origins[0]
}

func (h *Handler) push(w http.ResponseWriter, r *http.Request) {
	if caller, network := h.Callers.Keys(r); !allow(h.CallerLimit, caller) || !allow(h.NetLimit, network) {
		w.Header().Set("Retry-After", "10")
		serve.Error(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	address := r.PathValue("address")
	content, err := h.Keys.Open(address)
	if err != nil {
		// D-10: never 404 or 410 here. The log names the kind, never the address.
		h.Warn.Warn(err.Error(), "push: cannot open the push address")
		w.Header().Set("Retry-After", "60")
		serve.Error(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	switch err := vapid.Verify(r.Header.Get("Authorization"), content.VAPIDKey, h.Origins, h.Now()); {
	case errors.Is(err, vapid.ErrAudience):
		// Signed by the right server but for another name: RELAY_PUSH_ORIGINS here lacks the name
		// the server sends to. That is our own fault and a 503 (D-10, code review minor 5).
		h.Warn.Warn("audience", "push: VAPID aud is none of RELAY_PUSH_ORIGINS; check the setting")
		w.Header().Set("Retry-After", "60")
		serve.Error(w, http.StatusServiceUnavailable, "unavailable")
		return
	case err != nil:
		serve.Error(w, http.StatusForbidden, "vapid_rejected")
		return
	}
	// Charged only now: without a valid signature nobody can use up an address's budget (L-1).
	sum := sha256.Sum256([]byte(address))
	if !allow(h.AddressLimit, base64.RawURLEncoding.EncodeToString(sum[:])) {
		w.Header().Set("Retry-After", "10")
		serve.Error(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Content-Encoding")), "aes128gcm") {
		serve.Error(w, http.StatusUnsupportedMediaType, "aes128gcm_required")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(h.MaxBody)+1))
	switch {
	case err != nil:
		serve.Error(w, http.StatusBadRequest, "bad_request")
		return
	case len(body) > h.MaxBody:
		serve.Error(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	case len(body) == 0:
		serve.Error(w, http.StatusBadRequest, "empty")
		return
	}
	// TTL is required (RFC 8030 section 5); every Slouchcat server sends it. Google keeps a message
	// at most four weeks, so larger values are cut before they can overflow; 0 means "now or
	// never" and becomes one second.
	ttl, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get("TTL")), 10, 64)
	if err != nil || ttl < 0 {
		serve.Error(w, http.StatusBadRequest, "ttl_required")
		return
	}
	ttl = max(1, min(ttl, maxTTLSeconds))
	if h.FCM == nil {
		w.Header().Set("Retry-After", "60")
		serve.Error(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	if !allow(h.GlobalLimit, "all") {
		h.Warn.Warn("global_limit", "push: service-wide limit reached; messages are refused with 503")
		w.Header().Set("Retry-After", "10")
		serve.Error(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	msg := fcm.Message{
		Token: content.Token,
		Data:  map[string]string{"wp": base64.StdEncoding.EncodeToString(body)},
		High:  strings.EqualFold(strings.TrimSpace(r.Header.Get("Urgency")), "high"),
		TTL:   time.Duration(ttl) * time.Second,
	}
	switch err := h.FCM.Send(r.Context(), msg); {
	case errors.Is(err, fcm.ErrUnregistered):
		serve.Error(w, http.StatusGone, "unregistered")
	case err != nil:
		var fe *fcm.Error
		retry := "60"
		if errors.As(err, &fe) && fe.RetryAfter != "" {
			retry = fe.RetryAfter
		}
		h.Warn.Warn("google", "push: Google did not take the message", "err", err)
		w.Header().Set("Retry-After", retry)
		serve.Error(w, http.StatusServiceUnavailable, "unavailable")
	default:
		w.Header().Set("TTL", strconv.FormatInt(int64(msg.TTL/time.Second), 10))
		w.WriteHeader(http.StatusCreated)
	}
}
