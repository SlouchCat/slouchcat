// Package watch is the reachability check of the two relay services (concept 27 section 7.5,
// "Überwachung"): it asks /v1/healthz of each service at an interval and sends a message to a
// configurable address when a service goes down and when it is back. It is meant to run outside
// the network of the services, so it does not fail together with them.
package watch

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Target is one service.
type Target struct {
	Name string // "pair", "push"
	URL  string // health address, https://pair.example.test/v1/healthz
}

// Watcher checks targets and notifies on changes.
type Watcher struct {
	Targets  []Target
	Client   *http.Client
	Notify   string // URL that receives a POST with a text/plain body per change
	Failures int    // consecutive failed checks before "down" (at least 1)
	Log      *slog.Logger

	mu    sync.Mutex
	state map[string]*targetState
}

type targetState struct {
	down     bool
	failures int
}

// Event is one change of state.
type Event struct {
	Target Target
	Down   bool
	Reason string
}

// Text is the message body of e.
func (e Event) Text() string {
	if e.Down {
		return fmt.Sprintf("Slouchcat relay %s is DOWN (%s): %s", e.Target.Name, e.Target.URL, e.Reason)
	}
	return fmt.Sprintf("Slouchcat relay %s is back up (%s)", e.Target.Name, e.Target.URL)
}

// CheckOnce checks every target once and returns the changes, after notifying them.
func (w *Watcher) CheckOnce(ctx context.Context) []Event {
	w.mu.Lock()
	if w.state == nil {
		w.state = map[string]*targetState{}
	}
	w.mu.Unlock()
	var events []Event
	for _, t := range w.Targets {
		reason := w.probe(ctx, t)
		w.mu.Lock()
		st := w.state[t.Name]
		if st == nil {
			st = &targetState{}
			w.state[t.Name] = st
		}
		switch {
		case reason == "" && st.down:
			st.down, st.failures = false, 0
			events = append(events, Event{Target: t})
		case reason == "":
			st.failures = 0
		default:
			st.failures++
			if !st.down && st.failures >= max(1, w.Failures) {
				st.down = true
				events = append(events, Event{Target: t, Down: true, Reason: reason})
			}
		}
		w.mu.Unlock()
	}
	for _, e := range events {
		w.send(ctx, e)
	}
	return events
}

// Run checks every interval until ctx ends.
func (w *Watcher) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		w.CheckOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (w *Watcher) probe(ctx context.Context, t Target) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return err.Error()
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		return "no answer"
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("status %d", resp.StatusCode)
	}
	return ""
}

func (w *Watcher) send(ctx context.Context, e Event) {
	if w.Log != nil {
		w.Log.Warn("state changed", "service", e.Target.Name, "down", e.Down, "reason", e.Reason)
	}
	if w.Notify == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Notify, strings.NewReader(e.Text()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	title := "Slouchcat relay " + e.Target.Name + " up"
	if e.Down {
		title = "Slouchcat relay " + e.Target.Name + " down"
	}
	req.Header.Set("Title", title)
	resp, err := w.Client.Do(req)
	if err != nil {
		if w.Log != nil {
			w.Log.Error("notification not sent", "err", err)
		}
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 && w.Log != nil {
		w.Log.Error("notification refused", "status", resp.StatusCode)
	}
}
