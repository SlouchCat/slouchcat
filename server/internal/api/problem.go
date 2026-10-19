package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/logging"
)

// ContentTypeProblem is the media type of RFC 9457 problem details.
const ContentTypeProblem = "application/problem+json"

// problems writes problem responses and logs server-side failures.
type problems struct {
	log *slog.Logger
	dev bool
}

// write sends a problem with a stable machine-readable code.
func (p problems) write(w http.ResponseWriter, r *http.Request, status int, code, detail string) {
	p.writeProblem(w, r, status, code, detail, nil, nil)
}

// writeProblem sends a problem with optional field errors and, for pin_locked, the remaining lock
// in seconds (retryAfterSeconds).
func (p problems) writeProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, fields []gen.FieldError, retryAfter *int) {
	instance := logging.RedactString(r.URL.Path) // an invitation token in the path stays out of the body
	body := gen.Problem{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Code:     code,
		Instance: &instance,
	}
	if detail != "" {
		body.Detail = &detail
	}
	if len(fields) > 0 {
		body.Errors = &fields
	}
	body.RetryAfterSeconds = retryAfter
	w.Header().Set("Content-Type", ContentTypeProblem)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		p.log.Debug("write problem response", "err", err)
	}
}

func (p problems) badRequest(w http.ResponseWriter, r *http.Request, err error) {
	p.write(w, r, http.StatusBadRequest, "bad_request", err.Error())
}

func (p problems) internal(w http.ResponseWriter, r *http.Request, err error) {
	p.log.ErrorContext(r.Context(), "request failed", "path", logging.RedactString(r.URL.Path), "request_id", requestIDFrom(r.Context()), "err", err)
	p.write(w, r, http.StatusInternalServerError, "internal_error", "")
}

// recoverer turns a panic in a handler into a 500 problem and logs the stack.
func (p problems) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler { //nolint:errorlint // sentinel value of panic, not a wrapped error
				panic(rec)
			}
			p.log.ErrorContext(r.Context(), "handler panicked", "path", logging.RedactString(r.URL.Path),
				"request_id", requestIDFrom(r.Context()), "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
			detail := ""
			if p.dev {
				detail = fmt.Sprint(rec)
			}
			p.write(w, r, http.StatusInternalServerError, "internal_error", detail)
		}()
		next.ServeHTTP(w, r)
	})
}
