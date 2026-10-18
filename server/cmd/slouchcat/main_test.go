package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

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
