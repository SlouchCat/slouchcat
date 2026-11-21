// Package serve holds what both relay services need around their handlers: reading settings from
// the environment, the HTTP server with its time limits and graceful stop, the container health
// check (`-healthcheck`), JSON answers and the request log without caller addresses (D-5).
package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Env returns the variable name or def when it is unset or empty.
func Env(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

// EnvInt is Env for a positive integer.
func EnvInt(name string, def int) (int, error) {
	v := Env(name, "")
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s: want a positive integer, got %q", name, v)
	}
	return n, nil
}

// EnvFloat is Env for a positive number.
func EnvFloat(name string, def float64) (float64, error) {
	v := Env(name, "")
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("%s: want a positive number, got %q", name, v)
	}
	return f, nil
}

// EnvDuration is Env for a positive duration ("5m", "90s").
func EnvDuration(name string, def time.Duration) (time.Duration, error) {
	v := Env(name, "")
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: want a positive duration, got %q", name, v)
	}
	return d, nil
}

// LogLevel reads RELAY_LOG_LEVEL (debug, info, warn, error).
func LogLevel() slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(Env("RELAY_LOG_LEVEL", "info"))); err != nil {
		return slog.LevelInfo
	}
	return l
}

// Run serves h on addr until SIGINT or SIGTERM, then stops within ten seconds.
func Run(log *slog.Logger, addr string, h http.Handler) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    16 << 10,
		// The standard library logs TLS and protocol errors with the caller's address (D-5).
		ErrorLog: slog.NewLogLogger(slog.DiscardHandler, slog.LevelError),
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "listen", addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("stopping")
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

// Healthcheck asks the service on listen (":8080" or "host:port") for /v1/healthz and returns
// the exit code of the container health check: 0 when it answered 200.
func Healthcheck(listen string) int {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/v1/healthz", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}

// JSON writes v with status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes {"error": code} with status. The codes are part of the protocol and stable.
func Error(w http.ResponseWriter, status int, code string) {
	JSON(w, status, map[string]string{"error": code})
}

// DecodeJSON reads a JSON body of at most limit bytes into v. It answers 413 or 400 itself and
// reports false then.
func DecodeJSON(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			Error(w, http.StatusRequestEntityTooLarge, "too_large")
			return false
		}
		Error(w, http.StatusBadRequest, "bad_request")
		return false
	}
	return true
}

// statusWriter remembers the status for the request log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Middleware adds the headers every answer carries, recovers from a panic with a 500 and logs
// each request at debug level by its route pattern -- never by its path, which carries the
// meeting point id or the sealed push address, and never with the caller's address (D-5).
func Middleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("Referrer-Policy", "no-referrer")
		defer func() {
			if p := recover(); p != nil {
				log.Error("panic in handler", "route", r.Pattern, "panic", fmt.Sprint(p))
				if sw.status == 0 {
					Error(sw, http.StatusInternalServerError, "internal")
				}
			}
			log.Debug("request", "method", r.Method, "route", r.Pattern, "status", sw.status, "ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(sw, r)
	})
}
