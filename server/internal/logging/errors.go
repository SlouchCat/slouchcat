package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ErrorRingSize is how many error records the dashboard shows (A-3, design 3.1).
const ErrorRingSize = 50

// ErrorEntry is one error record of the server log as the ring keeps it. Message and Attrs are
// already redacted: the ring sits behind the RedactingHandler.
type ErrorEntry struct {
	At      time.Time
	Source  string // attribute component or job_type, else the logger's first group, else "server"
	Message string
	Attrs   map[string]string // every attribute as text; keys of groups are joined with "."
	JobID   int64             // attribute job_id; 0 = none
	JobType string            // attribute job_type
}

// ErrorRing keeps the newest records of level Error for the admin dashboard (A-3). Wrap puts it
// between the RedactingHandler and the output handler, so it only ever sees redacted records.
type ErrorRing struct {
	mu      sync.Mutex
	entries []ErrorEntry // ring buffer; next is the slot of the next record
	next    int
	full    bool
}

// NewErrorRing returns a ring that keeps the last size error records.
func NewErrorRing(size int) *ErrorRing {
	return &ErrorRing{entries: make([]ErrorEntry, max(size, 1))}
}

// NewWithErrors is New with an ErrorRing of ErrorRingSize between the redaction and the output;
// main hands the ring to the admin dashboard.
func NewWithErrors(w io.Writer, format Format, level slog.Leveler) (*slog.Logger, *ErrorRing, error) {
	opts := &slog.HandlerOptions{Level: level}
	var out slog.Handler
	switch format {
	case FormatJSON:
		out = slog.NewJSONHandler(w, opts)
	case FormatText:
		out = slog.NewTextHandler(w, opts)
	default:
		return nil, nil, fmt.Errorf("unknown log format %q", format)
	}
	ring := NewErrorRing(ErrorRingSize)
	return slog.New(NewRedactingHandler(ring.Wrap(out))), ring, nil
}

// Wrap returns a handler that records every Error record in the ring and passes all records on
// to inner.
func (r *ErrorRing) Wrap(inner slog.Handler) slog.Handler {
	return &ringHandler{ring: r, inner: inner}
}

// Entries returns copies of the kept records, newest first.
func (r *ErrorRing) Entries() []ErrorEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := r.next
	if r.full {
		n = len(r.entries)
	}
	out := make([]ErrorEntry, 0, n)
	for i := range n {
		e := r.entries[(r.next-1-i+len(r.entries))%len(r.entries)]
		attrs := make(map[string]string, len(e.Attrs))
		for k, v := range e.Attrs {
			attrs[k] = v
		}
		e.Attrs = attrs
		out = append(out, e)
	}
	return out
}

func (r *ErrorRing) add(e ErrorEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.next] = e
	r.next = (r.next + 1) % len(r.entries)
	if r.next == 0 {
		r.full = true
	}
}

// ringHandler is the slog.Handler of an ErrorRing; attrs are those of Logger.With, already
// prefixed with their groups.
type ringHandler struct {
	ring   *ErrorRing
	inner  slog.Handler
	attrs  []slog.Attr
	groups []string
}

func (h *ringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ringHandler) Handle(ctx context.Context, rec slog.Record) error {
	if rec.Level >= slog.LevelError {
		h.ring.add(h.entry(rec))
	}
	return h.inner.Handle(ctx, rec)
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &ringHandler{ring: h.ring, inner: h.inner.WithAttrs(attrs), groups: h.groups}
	next.attrs = append(append([]slog.Attr{}, h.attrs...), prefixed(h.groups, attrs)...)
	return next
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &ringHandler{ring: h.ring, inner: h.inner.WithGroup(name), attrs: h.attrs,
		groups: append(append([]string{}, h.groups...), name)}
}

// entry turns a record into an ErrorEntry with the attributes of the handler and the record.
func (h *ringHandler) entry(rec slog.Record) ErrorEntry {
	e := ErrorEntry{At: rec.Time, Message: rec.Message, Attrs: map[string]string{}}
	all := append([]slog.Attr{}, h.attrs...)
	rec.Attrs(func(a slog.Attr) bool {
		all = append(all, prefixed(h.groups, []slog.Attr{a})...)
		return true
	})
	for _, a := range all {
		flatten(e.Attrs, "", a)
	}
	for _, a := range all {
		switch a.Key {
		case "job_id":
			if v := a.Value.Resolve(); v.Kind() == slog.KindInt64 {
				e.JobID = v.Int64()
			}
		case "job_type":
			e.JobType = a.Value.Resolve().String()
		}
	}
	switch {
	case e.Attrs["component"] != "":
		e.Source = e.Attrs["component"]
	case e.JobType != "":
		e.Source = e.JobType
	case len(h.groups) > 0:
		e.Source = h.groups[0]
	default:
		e.Source = "server"
	}
	return e
}

// prefixed nests attrs into the open groups, so their keys read "group.key" once flattened.
func prefixed(groups []string, attrs []slog.Attr) []slog.Attr {
	if len(groups) == 0 {
		return attrs
	}
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		a.Key = strings.Join(groups, ".") + "." + a.Key
		out[i] = a
	}
	return out
}

// flatten writes a as text into m; the members of a group get the key "group.member".
func flatten(m map[string]string, prefix string, a slog.Attr) {
	v := a.Value.Resolve()
	key := a.Key
	switch {
	case prefix == "":
	case key == "":
		key = prefix // an inline group
	default:
		key = prefix + "." + key
	}
	if v.Kind() == slog.KindGroup {
		for _, member := range v.Group() {
			flatten(m, key, member)
		}
		return
	}
	if key == "" {
		return
	}
	m[key] = v.String()
}
