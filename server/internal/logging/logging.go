// Package logging provides the only logger Slouchcat uses: log/slog behind a handler that redacts
// secrets (plan section 3.8, point 4; CLAUDE.md rule 10).
//
// Redaction happens in two ways:
//
//   - Attributes whose key names a secret (see sensitiveKeys) get the value "[redacted]", at any
//     depth of nested groups and for attributes added with Logger.With.
//   - Query parameters that carry secrets (api_key, st) are masked inside every string value,
//     inside error messages and inside the log message itself, so a logged URL keeps its shape.
//
// A new key name that can carry a secret belongs in sensitiveKeys and in the test.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
)

// Redacted replaces every secret value.
const Redacted = "[redacted]"

// Format selects the output encoding of the logger.
type Format string

// Supported formats.
const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// sensitiveKeys are attribute key suffixes that carry secrets. Keys are compared after lower-casing
// and removing '-' and '_', so "apiKey", "api_key", "X-Api-Key" and "tmdb_api_key" all match.
var sensitiveKeys = []string{"password", "pin", "token", "apikey", "authorization", "secret", "pinhash", "tokenhash", "passwordhash"}

// sensitiveQuery matches the value of the query parameters api_key and st.
var sensitiveQuery = regexp.MustCompile(`(?i)((?:^|[?&;])(?:api_key|st)=)[^&;#\s"']*`)

// New returns a logger that writes to w in the given format and level, with redaction applied.
func New(w io.Writer, format Format, level slog.Leveler) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{Level: level}
	switch format {
	case FormatJSON:
		return slog.New(NewRedactingHandler(slog.NewJSONHandler(w, opts))), nil
	case FormatText:
		return slog.New(NewRedactingHandler(slog.NewTextHandler(w, opts))), nil
	default:
		return nil, fmt.Errorf("unknown log format %q", format)
	}
}

// RedactingHandler wraps another slog.Handler and removes secrets before records reach it.
type RedactingHandler struct {
	inner slog.Handler
}

// NewRedactingHandler wraps inner.
func NewRedactingHandler(inner slog.Handler) *RedactingHandler {
	return &RedactingHandler{inner: inner}
}

// Enabled reports whether the wrapped handler handles records at level.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts the message and all attributes of r and passes the copy on.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, RedactString(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

// WithAttrs redacts attrs once and adds them to the wrapped handler.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		clean[i] = redactAttr(a)
	}
	return &RedactingHandler{inner: h.inner.WithAttrs(clean)}
}

// WithGroup opens a group on the wrapped handler.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{inner: h.inner.WithGroup(name)}
}

// RedactString masks the values of secret query parameters in s.
func RedactString(s string) string {
	if !strings.Contains(s, "=") {
		return s
	}
	return sensitiveQuery.ReplaceAllString(s, "${1}"+Redacted)
}

func redactAttr(a slog.Attr) slog.Attr {
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, RedactString(v.String()))
	case slog.KindGroup:
		members := v.Group()
		clean := make([]slog.Attr, len(members))
		for i, m := range members {
			clean[i] = redactAttr(m)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(clean...)}
	case slog.KindAny:
		return slog.Attr{Key: a.Key, Value: redactAny(v)}
	default:
		return slog.Attr{Key: a.Key, Value: v}
	}
}

// redactAny covers the values that commonly embed URLs: errors (net/http includes the request
// URL in its errors) and *url.URL. Other values pass unchanged.
func redactAny(v slog.Value) slog.Value {
	switch x := v.Any().(type) {
	case error:
		msg := x.Error()
		if clean := RedactString(msg); clean != msg {
			return slog.AnyValue(errors.New(clean))
		}
	case *url.URL:
		return slog.StringValue(RedactString(x.String()))
	}
	return v
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(key))
	for _, s := range sensitiveKeys {
		if strings.HasSuffix(k, s) {
			return true
		}
	}
	return false
}
