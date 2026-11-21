package pair

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
	"github.com/slouchcat/slouchcat/relay/internal/logging"
	"github.com/slouchcat/slouchcat/relay/internal/serve"
)

// MaxPacket is the largest packet the service holds, in bytes after base64url decoding (7.1).
const MaxPacket = 4096

// pickupScheme is the scheme of the Authorization header that carries the pickup secret.
const pickupScheme = "Pickup "

// Handler serves the protocol of section 7.1 and the help page of the QR code.
type Handler struct {
	Store    *Store
	Callers  *clientip.Resolver
	Limit    *limit.Limiter // per caller key, every request but the health check
	NetLimit *limit.Limiter // per /48 or /24 around the caller (security review M-1)
	Log      *slog.Logger
	Warn     *logging.Throttle
	HelpPage *HelpPage
}

// Routes returns the mux of the service.
func (h *Handler) Routes() http.Handler {
	if h.NetLimit == nil {
		h.NetLimit = limit.New(1e9, 1<<30, 1)
	}
	if h.Warn == nil {
		h.Warn = logging.NewThrottle(h.Log, time.Minute)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", func(w http.ResponseWriter, _ *http.Request) {
		serve.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/pair", h.limited(h.create))
	mux.HandleFunc("PUT /v1/pair/{id}", h.limited(h.put))
	mux.HandleFunc("GET /v1/pair/{id}", h.limited(h.get))
	mux.HandleFunc("DELETE /v1/pair/{id}", h.limited(h.cancel))
	mux.HandleFunc("GET /tv", h.limited(h.HelpPage.ServeHTTP))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		serve.Error(w, http.StatusNotFound, "not_found")
	})
	return serve.Middleware(h.Log, mux)
}

func (h *Handler) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, network := h.Callers.Keys(r)
		if !h.Limit.Allow(caller) || !h.NetLimit.Allow(network) {
			w.Header().Set("Retry-After", "2")
			serve.Error(w, http.StatusTooManyRequests, "rate_limited")
			return
		}
		next(w, r)
	}
}

type createRequest struct {
	PickupHash string `json:"pickupHash"`
}

type createResponse struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !serve.DecodeJSON(w, r, 1024, &req) {
		return
	}
	raw, err := base64.RawURLEncoding.DecodeString(req.PickupHash)
	if err != nil || len(raw) != sha256.Size {
		serve.Error(w, http.StatusBadRequest, "bad_pickup_hash")
		return
	}
	caller, network := h.Callers.Keys(r)
	id, exp, err := h.Store.Create(caller, network, [sha256.Size]byte(raw))
	switch {
	case errors.Is(err, ErrCallerFull):
		w.Header().Set("Retry-After", "60")
		serve.Error(w, http.StatusTooManyRequests, "too_many_open")
		return
	case errors.Is(err, ErrFull):
		w.Header().Set("Retry-After", "60")
		serve.Error(w, http.StatusServiceUnavailable, "full")
		h.Warn.Warn("full", "meeting points full", "open", h.Store.Len())
		return
	case err != nil:
		serve.Error(w, http.StatusInternalServerError, "internal")
		return
	}
	serve.JSON(w, http.StatusCreated, createResponse{ID: id, ExpiresAt: exp.UTC().Truncate(time.Second)})
}

type packetBody struct {
	Packet string `json:"packet"`
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	var req packetBody
	// base64url of 4096 bytes is 5462 characters; the JSON frame adds a dozen.
	if !serve.DecodeJSON(w, r, 6<<10, &req) {
		return
	}
	packet, err := base64.RawURLEncoding.DecodeString(req.Packet)
	switch {
	case err != nil || len(packet) == 0:
		serve.Error(w, http.StatusBadRequest, "bad_packet")
		return
	case len(packet) > MaxPacket:
		serve.Error(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	}
	switch err := h.Store.Put(r.PathValue("id"), packet); {
	case errors.Is(err, ErrNotFound):
		serve.Error(w, http.StatusNotFound, "not_found")
	case errors.Is(err, ErrOccupied):
		serve.Error(w, http.StatusConflict, "occupied")
	case errors.Is(err, ErrConsumed):
		serve.Error(w, http.StatusGone, "consumed")
	case err != nil:
		serve.Error(w, http.StatusInternalServerError, "internal")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	secret, ok := pickupSecret(r)
	if !ok {
		serve.Error(w, http.StatusUnauthorized, "pickup_secret_required")
		return
	}
	packet, err := h.Store.Get(r.PathValue("id"), secret)
	switch {
	case errors.Is(err, ErrNotFound):
		serve.Error(w, http.StatusNotFound, "not_found")
	case errors.Is(err, ErrForbidden):
		serve.Error(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, ErrConsumed):
		serve.Error(w, http.StatusGone, "consumed")
	case err != nil:
		serve.Error(w, http.StatusInternalServerError, "internal")
	case packet == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		serve.JSON(w, http.StatusOK, packetBody{Packet: base64.RawURLEncoding.EncodeToString(packet)})
	}
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	secret, ok := pickupSecret(r)
	if !ok {
		serve.Error(w, http.StatusUnauthorized, "pickup_secret_required")
		return
	}
	switch err := h.Store.Delete(r.PathValue("id"), secret); {
	case errors.Is(err, ErrNotFound):
		serve.Error(w, http.StatusNotFound, "not_found")
	case errors.Is(err, ErrForbidden):
		serve.Error(w, http.StatusForbidden, "forbidden")
	case err != nil:
		serve.Error(w, http.StatusInternalServerError, "internal")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// pickupSecret reads "Authorization: Pickup <base64url of 32 bytes>".
func pickupSecret(r *http.Request) ([]byte, bool) {
	v := r.Header.Get("Authorization")
	if len(v) <= len(pickupScheme) || !strings.EqualFold(v[:len(pickupScheme)], pickupScheme) {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(v[len(pickupScheme):]))
	if err != nil || len(raw) != 32 {
		return nil, false
	}
	return raw, true
}
