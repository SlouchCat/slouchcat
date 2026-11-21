package serve

import (
	"bytes"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/logging"
)

func TestEnv(t *testing.T) {
	t.Setenv("RELAY_TEST_INT", "12")
	t.Setenv("RELAY_TEST_BAD", "-1")
	t.Setenv("RELAY_TEST_DUR", "90s")
	if n, err := EnvInt("RELAY_TEST_INT", 1); err != nil || n != 12 {
		t.Fatalf("EnvInt = %d, %v", n, err)
	}
	if _, err := EnvInt("RELAY_TEST_BAD", 1); err == nil {
		t.Fatal("negative accepted")
	}
	if n, _ := EnvInt("RELAY_TEST_UNSET", 7); n != 7 {
		t.Fatalf("default = %d", n)
	}
	if d, err := EnvDuration("RELAY_TEST_DUR", time.Second); err != nil || d != 90*time.Second {
		t.Fatalf("EnvDuration = %s, %v", d, err)
	}
	if _, err := EnvFloat("RELAY_TEST_BAD", 1); err == nil {
		t.Fatal("negative float accepted")
	}
}

// The request log names the route pattern, never the path or the caller (D-5).
func TestMiddlewareLogsNoPathOrAddress(t *testing.T) {
	var buf bytes.Buffer
	log, _ := logging.New(&buf, "json", slog.LevelDebug)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /v1/pair/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) { panic("boom") })
	srv := httptest.NewServer(Middleware(log, mux))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/pair/secretmeetingpoint", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("standard headers missing")
	}
	resp, _ = http.Get(srv.URL + "/panic")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("panic answered %d", resp.StatusCode)
	}
	out := buf.String()
	if !strings.Contains(out, `"route":"PUT /v1/pair/{id}"`) || !strings.Contains(out, `"status":204`) {
		t.Fatalf("request log incomplete:\n%s", out)
	}
	for _, s := range []string{"secretmeetingpoint", "203.0.113.50", "127.0.0.1"} {
		if strings.Contains(out, s) {
			t.Fatalf("log contains %q:\n%s", s, out)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/healthz" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ok.Close()
	if code := Healthcheck(strings.TrimPrefix(ok.URL, "http://")); code != 0 {
		t.Fatalf("healthy: exit %d", code)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer bad.Close()
	if code := Healthcheck(strings.TrimPrefix(bad.URL, "http://")); code != 1 {
		t.Fatalf("unhealthy: exit %d", code)
	}
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(ok.URL, "http://"))
	if code := Healthcheck(":" + port); code != 0 {
		t.Fatalf(":port form: exit %d", code)
	}
}
