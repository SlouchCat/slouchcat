package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/config"
)

func TestHealthcheck(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
	}))
	defer srv.Close()
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	if code := healthcheck(config.Config{Listen: ":" + port}); code != 0 {
		t.Fatalf("healthy server: exit %d, want 0", code)
	}
	status = http.StatusServiceUnavailable
	if code := healthcheck(config.Config{Listen: "0.0.0.0:" + port}); code != 1 {
		t.Fatalf("unhealthy server: exit %d, want 1", code)
	}
	srv.Close()
	if code := healthcheck(config.Config{Listen: ":" + port}); code != 1 {
		t.Fatalf("no server: exit %d, want 1", code)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	if code := run([]string{"frobnicate"}); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

// A requested restart runs the binary again whatever the shutdown returned (review I-1: a stream
// that outlived the shutdown timeout used to end the process with 1 and strand the restore).
func TestAfterStop(t *testing.T) {
	for _, tc := range []struct {
		err       error
		restart   bool
		wantAgain bool
		wantCode  int
		situation string
	}{
		{nil, false, false, 0, "signal"},
		{context.DeadlineExceeded, false, false, 1, "signal, shutdown timed out"},
		{nil, true, true, 0, "restart"},
		{context.DeadlineExceeded, true, true, 0, "restart, shutdown timed out"},
	} {
		if again, code := afterStop(tc.err, tc.restart); again != tc.wantAgain || code != tc.wantCode {
			t.Errorf("%s: again %v code %d, want %v %d", tc.situation, again, code, tc.wantAgain, tc.wantCode)
		}
	}
}

// A request that outlives the shutdown timeout makes a plain stop fail; on the way to a restart
// its connection is cut and the stop counts as normal.
func TestShutdownCutsLingeringStreamsOnRestart(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handlerDone := make(chan struct{})
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select { // a long direct-play stream
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+ln.Addr().String()+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := shutdown(srv, 50*time.Millisecond, false, log); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("plain stop with a running stream: %v, want the deadline", err)
	}
	if err := shutdown(srv, 50*time.Millisecond, true, log); err != nil {
		t.Fatalf("stop for a restart: %v, want nil", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream still runs after the restart stop")
	}
	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Error("the client's stream ended cleanly, want a cut connection")
	}
}
