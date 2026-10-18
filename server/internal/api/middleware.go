package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// HeaderRequestID carries the request ID in both directions.
const HeaderRequestID = "X-Request-Id"

type requestIDKey struct{}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// requestID accepts a well-formed incoming X-Request-Id (from a reverse proxy) or creates one,
// stores it in the context and echoes it in the response.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !validRequestID.MatchString(id) {
			var b [12]byte
			_, _ = rand.Read(b[:]) // crypto/rand.Read never fails
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// requestLog logs one line per request. The query string is a separate attribute, so the
// redacting handler masks secret parameters (api_key, st); it is never part of "path".
func requestLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
				slog.String("request_id", requestIDFrom(r.Context())),
			}
			if r.URL.RawQuery != "" {
				attrs = append(attrs, slog.String("query", r.URL.Query().Encode()))
			}
			log.LogAttrs(r.Context(), slog.LevelInfo, "http request", attrs...)
		})
	}
}
