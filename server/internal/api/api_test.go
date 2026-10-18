package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/logging"
)

func newTestHandler(t *testing.T, logs *bytes.Buffer, web http.Handler) http.Handler {
	t.Helper()
	log, err := logging.New(logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	return NewHandler(Options{
		Version: "0.1.0+test",
		Started: started,
		Logger:  log,
		Web:     web,
		now:     func() time.Time { return started.Add(90 * time.Second) },
	})
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) gen.Problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != ContentTypeProblem {
		t.Fatalf("Content-Type = %q, want %s", ct, ContentTypeProblem)
	}
	var p gen.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGetHealth(t *testing.T) {
	var logs bytes.Buffer
	h := newTestHandler(t, &logs, http.NotFoundHandler())
	rec := do(h, http.MethodGet, "/api/v1/healthz?api_key=FAKE-health-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got gen.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if want := (gen.Health{Status: gen.Ok, Version: "0.1.0+test", UptimeSeconds: 90}); got != want {
		t.Fatalf("health = %+v, want %+v", got, want)
	}
	if rec.Header().Get(HeaderRequestID) == "" {
		t.Fatal("missing X-Request-Id")
	}
	out := logs.String()
	if strings.Contains(out, "FAKE-health-key") {
		t.Fatalf("query secret leaked into the request log: %s", out)
	}
	for _, want := range []string{`"path":"/api/v1/healthz"`, `"status":200`, `"query":"api_key=[redacted]"`, `"method":"GET"`} {
		if !strings.Contains(out, want) {
			t.Errorf("request log lacks %s: %s", want, out)
		}
	}
}

func TestProblems(t *testing.T) {
	var logs bytes.Buffer
	h := newTestHandler(t, &logs, http.NotFoundHandler())
	for _, tc := range []struct {
		method, target, code string
		status               int
	}{
		{http.MethodGet, "/api/v1/nope", "not_found", http.StatusNotFound},
		{http.MethodGet, "/api/v1", "not_found", http.StatusNotFound},
		{http.MethodPost, "/api/v1/healthz", "method_not_allowed", http.StatusMethodNotAllowed},
	} {
		rec := do(h, tc.method, tc.target)
		p := decodeProblem(t, rec)
		if rec.Code != tc.status || p.Status != tc.status || p.Code != tc.code || p.Type != "about:blank" ||
			p.Title != http.StatusText(tc.status) || p.Instance == nil || *p.Instance != tc.target {
			t.Errorf("%s %s = %d %+v", tc.method, tc.target, rec.Code, p)
		}
	}
}

func TestPanicBecomesProblem(t *testing.T) {
	var logs bytes.Buffer
	h := newTestHandler(t, &logs, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaputt")
	}))
	rec := do(h, http.MethodGet, "/some/page")
	p := decodeProblem(t, rec)
	if rec.Code != http.StatusInternalServerError || p.Code != "internal_error" || p.Detail != nil {
		t.Fatalf("got %d %+v", rec.Code, p)
	}
	out := logs.String()
	if !strings.Contains(out, "handler panicked") || !strings.Contains(out, "kaputt") || !strings.Contains(out, "goroutine") {
		t.Fatalf("panic log lacks message or stack: %s", out)
	}
	if !strings.Contains(out, `"status":500`) {
		t.Fatalf("request log lacks status 500: %s", out)
	}
}

func TestWebFallback(t *testing.T) {
	var logs bytes.Buffer
	h := newTestHandler(t, &logs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("web:" + r.URL.Path))
	}))
	for _, target := range []string{"/", "/library/42", "/api/v2/x", "/apiv1"} {
		if rec := do(h, http.MethodGet, target); rec.Body.String() != "web:"+target {
			t.Errorf("GET %s = %q, want the web handler", target, rec.Body)
		}
	}
}

func TestRequestIDPropagation(t *testing.T) {
	var logs bytes.Buffer
	h := newTestHandler(t, &logs, http.NotFoundHandler())
	for incoming, keep := range map[string]bool{"proxy-1234.abc": true, "bad id\n": false, strings.Repeat("a", 65): false} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set(HeaderRequestID, incoming)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		got := rec.Header().Get(HeaderRequestID)
		if (got == incoming) != keep || got == "" {
			t.Errorf("incoming %q -> %q, keep = %v", incoming, got, keep)
		}
	}
}
