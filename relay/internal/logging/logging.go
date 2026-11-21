// Package logging is the only logger of the two relay services: log/slog behind a handler that
// redacts secrets and the addresses of callers (concept 27, rules D-5 and D-6; CLAUDE.md rule 10).
//
// It is a small copy of server/internal/logging, which a second Go module cannot import (Go keeps
// internal packages to their module). Redaction happens in three ways:
//
//   - Attributes whose key names a secret or an address (see sensitiveKeys) get the value
//     "[redacted]", at any depth of nested groups and for attributes added with Logger.With.
//
//   - The capability parts of the services' URL paths -- the meeting point id in /v1/pair/<id>
//     and the sealed blob in /v1/push/<version>.<blob> -- are masked inside every string value,
//     inside error messages and inside the log message itself.
//
//   - IPv4 and IPv6 addresses are masked inside every string value: an error of net/http names
//     the address it dialled or the one that called (D-5 keeps callers out of logs).
//
// A new key name that can carry a secret belongs in sensitiveKeys and in the test.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// Redacted replaces every secret value.
const Redacted = "[redacted]"

// sensitiveKeys are attribute key suffixes that carry secrets or caller addresses. Keys are
// compared after lower-casing and removing '-', '_' and '.', so "fcmToken", "fcm_token" and
// "X-Forwarded-For" all match.
//
// Secrets (D-6): the Firebase service account and its private key (credentials, privatekey), the
// sealing keys (sealingkey, key), the Google token (fcmtoken, token), the pickup secret and its
// hash (pickup, pickuphash), the OAuth access token (accesstoken, authorization), the VAPID key
// of a server (vapidkey), the sealed push address (endpoint, blob) and the meeting point packet
// (packet). Addresses (D-5): ip, addr, remoteaddr, clientip, caller, forwardedfor, host.
var sensitiveKeys = []string{
	"password", "secret", "token", "authorization", "apikey", "credentials", "privatekey",
	"sealingkey", "key", "fcmtoken", "accesstoken", "assertion", "pickup", "pickuphash",
	"vapidkey", "endpoint", "blob", "packet", "id",
	"ip", "addr", "remoteaddr", "clientip", "caller", "forwardedfor", "host",
}

// New returns a logger that writes to w in the given format ("json" or "text") and level, with
// redaction applied.
func New(w io.Writer, format string, level slog.Leveler) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{Level: level}
	switch format {
	case "json", "":
		return slog.New(NewRedactingHandler(slog.NewJSONHandler(w, opts))), nil
	case "text":
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

var (
	// pairPath matches the meeting point id in /v1/pair/<id>.
	pairPath = regexp.MustCompile(`(?i)(/v1/pair/)[A-Za-z0-9_-]+`)
	// pushPath matches the sealed blob in /v1/push/<version>.<blob>.
	pushPath = regexp.MustCompile(`(?i)(/v1/push/)[A-Za-z0-9_.-]+`)
	// ipv4 matches a dotted quad.
	ipv4 = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// ipv6 matches the candidates for an IPv6 address: hex digits with at least two colons,
	// optionally in brackets, with a zone or an embedded IPv4 tail. A candidate is masked only
	// when it parses as an address, so a time of day such as 10:00:00 stays readable.
	ipv6 = regexp.MustCompile(`\[?[0-9A-Fa-f]*:[0-9A-Fa-f]*:[0-9A-Fa-f:.]*(?:%[0-9A-Za-z]+)?\]?`)
)

// RedactString masks the capability parts of the services' paths and every IP address in s.
func RedactString(s string) string {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "/v1/pair/") {
		s = pairPath.ReplaceAllString(s, "${1}"+Redacted)
	}
	if strings.Contains(lower, "/v1/push/") {
		s = pushPath.ReplaceAllString(s, "${1}"+Redacted)
	}
	if strings.Count(s, ":") >= 2 {
		s = ipv6.ReplaceAllStringFunc(s, func(m string) string {
			if _, err := netip.ParseAddr(strings.Trim(m, "[]")); err == nil {
				return Redacted
			}
			return m
		})
	}
	if strings.Contains(s, ".") {
		s = ipv4.ReplaceAllStringFunc(s, func(m string) string {
			if _, err := netip.ParseAddr(m); err == nil {
				return Redacted
			}
			return m
		})
	}
	return s
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

// redactAny covers the values that commonly embed URLs and addresses: errors (net/http includes
// the request URL and the dialled address in its errors) and *url.URL. Other values are logged
// as their redacted string form, so a struct cannot smuggle a secret past the handler.
func redactAny(v slog.Value) slog.Value {
	switch x := v.Any().(type) {
	case error:
		msg := x.Error()
		if clean := RedactString(msg); clean != msg {
			return slog.AnyValue(errors.New(clean))
		}
		return v
	case *url.URL:
		return slog.StringValue(RedactString(x.String()))
	case fmt.Stringer:
		return slog.StringValue(RedactString(x.String()))
	}
	return slog.StringValue(RedactString(fmt.Sprint(v.Any())))
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(key))
	for _, s := range sensitiveKeys {
		if strings.HasSuffix(k, s) {
			return true
		}
	}
	return false
}
