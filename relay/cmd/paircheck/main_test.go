package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
	"github.com/slouchcat/slouchcat/relay/internal/pair"
)

// The check program against the real handler: every check of a move rehearsal passes.
func TestPaircheckAgainstTheService(t *testing.T) {
	callers, _ := clientip.ParseTrusted("")
	h := &pair.Handler{Store: pair.NewStore(5*time.Minute, pair.Limits{MaxOpen: 100, MaxPerCaller: 10, MaxPerNet: 50}), Callers: callers,
		Limit: limit.New(100, 100, 100), Log: slog.New(slog.DiscardHandler), HelpPage: pair.NewHelpPage()}
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()
	var out bytes.Buffer
	n, err := Run(&http.Client{Timeout: 5 * time.Second}, srv.URL, &out)
	if err != nil {
		t.Fatalf("after %d checks: %v\n%s", n, err, out.String())
	}
	if n < 19 || strings.Contains(out.String(), "FAIL") {
		t.Fatalf("%d checks:\n%s", n, out.String())
	}
}

// Against a service that hands the packet out twice, paircheck must fail.
func TestPaircheckNoticesABrokenService(t *testing.T) {
	broken := http.NewServeMux()
	broken.HandleFunc("GET /v1/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	broken.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := httptest.NewServer(broken)
	defer srv.Close()
	var out bytes.Buffer
	if _, err := Run(&http.Client{Timeout: 5 * time.Second}, srv.URL, &out); err == nil {
		t.Fatal("paircheck passed against a broken service")
	}
}
