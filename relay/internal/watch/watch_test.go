package watch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A service goes down and comes back; the receiver gets exactly one message for each change.
func TestNotifiesDownAndUp(t *testing.T) {
	var up atomic.Bool
	up.Store(true)
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if up.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer svc.Close()
	var mu sync.Mutex
	var got []string
	var titles []string
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		titles = append(titles, r.Header.Get("Title"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer recv.Close()

	w := &Watcher{Targets: []Target{{Name: "pair", URL: svc.URL + "/v1/healthz"}}, Client: &http.Client{Timeout: 5 * time.Second},
		Notify: recv.URL, Failures: 2}
	ctx := context.Background()
	if ev := w.CheckOnce(ctx); len(ev) != 0 {
		t.Fatalf("healthy service gave %v", ev)
	}
	up.Store(false)
	if ev := w.CheckOnce(ctx); len(ev) != 0 {
		t.Fatalf("one failure is not yet down: %v", ev)
	}
	if ev := w.CheckOnce(ctx); len(ev) != 1 || !ev[0].Down || ev[0].Reason != "status 503" {
		t.Fatalf("second failure: %v", ev)
	}
	if ev := w.CheckOnce(ctx); len(ev) != 0 {
		t.Fatalf("still down notified again: %v", ev)
	}
	up.Store(true)
	if ev := w.CheckOnce(ctx); len(ev) != 1 || ev[0].Down {
		t.Fatalf("recovery: %v", ev)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || !strings.Contains(got[0], "pair is DOWN") || !strings.Contains(got[1], "pair is back up") {
		t.Fatalf("messages = %q", got)
	}
	if titles[0] != "Slouchcat relay pair down" || titles[1] != "Slouchcat relay pair up" {
		t.Fatalf("titles = %q", titles)
	}
}

// A stopped service (nothing listens) counts as down too.
func TestUnreachableIsDown(t *testing.T) {
	svc := httptest.NewServer(http.NotFoundHandler())
	addr := svc.URL
	svc.Close()
	w := &Watcher{Targets: []Target{{Name: "push", URL: addr + "/v1/healthz"}}, Client: &http.Client{Timeout: 2 * time.Second}, Failures: 1}
	if ev := w.CheckOnce(context.Background()); len(ev) != 1 || !ev[0].Down || ev[0].Reason != "no answer" {
		t.Fatalf("events = %v", ev)
	}
}
