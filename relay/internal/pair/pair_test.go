package pair

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func hashOf(secret []byte) [sha256.Size]byte { return sha256.Sum256(secret) }

func TestStoreLifecycle(t *testing.T) {
	c := &clock{t: time.Unix(1_000_000, 0)}
	s := NewStore(5*time.Minute, 10, 10)
	s.SetClock(c.now)
	secret := bytes.Repeat([]byte{7}, 32)
	id, exp, err := s.Create("tv", hashOf(secret))
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := base64.RawURLEncoding.DecodeString(id); err != nil || len(raw) != 16 {
		t.Fatalf("id %q is not 16 bytes base64url", id)
	}
	if exp.Sub(c.t) != 5*time.Minute {
		t.Fatalf("expires after %s, want 5m", exp.Sub(c.t))
	}
	if p, err := s.Get(id, secret); p != nil || err != nil {
		t.Fatalf("empty Get = %v, %v", p, err)
	}
	if _, err := s.Get(id, bytes.Repeat([]byte{8}, 32)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong secret: %v", err)
	}
	if err := s.Put(id, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(id, []byte("two")); !errors.Is(err, ErrOccupied) {
		t.Fatalf("second Put: %v", err)
	}
	if p, err := s.Get(id, secret); err != nil || string(p) != "one" {
		t.Fatalf("Get = %q, %v", p, err)
	}
	if _, err := s.Get(id, secret); !errors.Is(err, ErrConsumed) {
		t.Fatalf("second Get: %v", err)
	}
	if err := s.Put(id, []byte("three")); !errors.Is(err, ErrConsumed) {
		t.Fatalf("Put after pick-up: %v", err)
	}
	// Consumed is remembered until the five minutes are over, then the id is unknown.
	c.t = c.t.Add(5*time.Minute - time.Second)
	if err := s.Put(id, []byte("x")); !errors.Is(err, ErrConsumed) {
		t.Fatalf("just before expiry: %v", err)
	}
	c.t = c.t.Add(time.Second)
	if err := s.Put(id, []byte("x")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after expiry: %v", err)
	}
	if s.Len() != 0 {
		t.Fatalf("Len = %d after expiry", s.Len())
	}
}

func TestStoreExpiresUnpickedPacket(t *testing.T) {
	c := &clock{t: time.Unix(1_000_000, 0)}
	s := NewStore(5*time.Minute, 10, 10)
	s.SetClock(c.now)
	secret := bytes.Repeat([]byte{1}, 32)
	id, _, _ := s.Create("tv", hashOf(secret))
	_ = s.Put(id, []byte("p"))
	c.t = c.t.Add(5 * time.Minute)
	s.Expire()
	if s.Len() != 0 {
		t.Fatal("an expired packet stayed in memory")
	}
	if _, err := s.Get(id, secret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after expiry: %v", err)
	}
}

func TestStoreCancel(t *testing.T) {
	s := NewStore(time.Minute, 10, 10)
	secret := bytes.Repeat([]byte{3}, 32)
	id, _, _ := s.Create("tv", hashOf(secret))
	if err := s.Delete(id, bytes.Repeat([]byte{4}, 32)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete with a wrong secret: %v", err)
	}
	if err := s.Delete(id, secret); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(id, []byte("p")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Put after cancel: %v", err)
	}
}

func TestStoreLimits(t *testing.T) {
	s := NewStore(time.Minute, 3, 2)
	h := hashOf([]byte("x"))
	for range 2 {
		if _, _, err := s.Create("a", h); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.Create("a", h); !errors.Is(err, ErrCallerFull) {
		t.Fatalf("third for one caller: %v", err)
	}
	if _, _, err := s.Create("b", h); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Create("c", h); !errors.Is(err, ErrFull) {
		t.Fatalf("over the total: %v", err)
	}
}

func newServer(t *testing.T, rate float64, burst int, trusted string) (*httptest.Server, *Store) {
	t.Helper()
	callers, err := clientip.ParseTrusted(trusted)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(5*time.Minute, 100, 3)
	h := &Handler{Store: store, Callers: callers, Limit: limit.New(rate, burst, 1000),
		Log: slog.New(slog.DiscardHandler), HelpPage: NewHelpPage()}
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	return srv, store
}

func call(t *testing.T, method, url, auth string, body any, header map[string]string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rd)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestHandlerSizesAndInput(t *testing.T) {
	srv, _ := newServer(t, 100, 100, "")
	secret := bytes.Repeat([]byte{9}, 32)
	sum := sha256.Sum256(secret)
	resp, body := call(t, "POST", srv.URL+"/v1/pair", "", map[string]string{"pickupHash": base64.RawURLEncoding.EncodeToString(sum[:])}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var o struct {
		ID        string    `json:"id"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	_ = json.Unmarshal(body, &o)
	url := srv.URL + "/v1/pair/" + o.ID
	for _, c := range []struct {
		name string
		body any
		want int
	}{
		{"exactly 4096 bytes", map[string]string{"packet": base64.RawURLEncoding.EncodeToString(make([]byte, 4096))}, http.StatusNoContent},
		{"4097 bytes", map[string]string{"packet": base64.RawURLEncoding.EncodeToString(make([]byte, 4097))}, http.StatusRequestEntityTooLarge},
		{"body far over the limit", map[string]string{"packet": strings.Repeat("A", 20000)}, http.StatusRequestEntityTooLarge},
		{"not base64url", map[string]string{"packet": "***"}, http.StatusBadRequest},
		{"unknown field", map[string]string{"packet": "AAAA", "extra": "x"}, http.StatusBadRequest},
	} {
		if resp, b := call(t, "PUT", url, "", c.body, nil); resp.StatusCode != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, resp.StatusCode, b, c.want)
		}
	}
	if resp, _ := call(t, "POST", srv.URL+"/v1/pair", "", map[string]string{"pickupHash": "short"}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("short pickup hash: %d", resp.StatusCode)
	}
	if resp, _ := call(t, "GET", url, "Bearer x", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("other scheme: %d", resp.StatusCode)
	}
	if resp, _ := call(t, "GET", url, "Pickup "+base64.RawURLEncoding.EncodeToString(secret), nil, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("pick-up of the 4096-byte packet: %d", resp.StatusCode)
	}
	if resp, _ := call(t, "GET", srv.URL+"/v1/healthz", "", nil, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: %d", resp.StatusCode)
	}
	if resp, _ := call(t, "GET", srv.URL+"/elsewhere", "", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown path: %d", resp.StatusCode)
	}
}

func TestHandlerRateLimitPerCaller(t *testing.T) {
	// The test client at 127.0.0.1 plays the trusted proxy: two callers behind it get a bucket
	// each (D-9), and the health check is never limited.
	srv, _ := newServer(t, 0.001, 3, "127.0.0.1")
	get := func(xff string) int {
		resp, _ := call(t, "GET", srv.URL+"/tv", "", nil, map[string]string{"X-Forwarded-For": xff})
		return resp.StatusCode
	}
	for i := range 3 {
		if s := get("203.0.113.1"); s != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, s)
		}
	}
	if s := get("203.0.113.1"); s != http.StatusTooManyRequests {
		t.Fatalf("fourth request: %d, want 429", s)
	}
	if s := get("203.0.113.2"); s != http.StatusOK {
		t.Fatalf("another caller behind the same proxy: %d", s)
	}
	if s := get("2001:db8:5:5::1"); s != http.StatusOK {
		t.Fatalf("IPv6 caller: %d", s)
	}
	for range 3 {
		get("2001:db8:5:5::2")
	}
	if s := get("2001:db8:5:5:ffff::3"); s != http.StatusTooManyRequests {
		t.Fatalf("same /64: %d, want 429 (D-2)", s)
	}
	if resp, _ := call(t, "GET", srv.URL+"/v1/healthz", "", nil, map[string]string{"X-Forwarded-For": "203.0.113.1"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("health check limited: %d", resp.StatusCode)
	}
}

func TestHandlerOpenPerCaller(t *testing.T) {
	srv, _ := newServer(t, 100, 100, "")
	sum := sha256.Sum256([]byte("x"))
	body := map[string]string{"pickupHash": base64.RawURLEncoding.EncodeToString(sum[:])}
	for range 3 {
		if resp, _ := call(t, "POST", srv.URL+"/v1/pair", "", body, nil); resp.StatusCode != http.StatusCreated {
			t.Fatalf("create: %d", resp.StatusCode)
		}
	}
	if resp, _ := call(t, "POST", srv.URL+"/v1/pair", "", body, nil); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("fourth open meeting point of one caller: %d, want 429", resp.StatusCode)
	}
}

func TestHelpPage(t *testing.T) {
	srv, _ := newServer(t, 100, 100, "")
	for lang, want := range map[string]string{
		"de-CH,de;q=0.9,en;q=0.8": "Öffne die Slouchcat-App und wähle «Fernseher verbinden».",
		"en-US,en;q=0.9,de;q=0.5": "Open the Slouchcat app and choose “Connect TV”.",
		"fr-FR":                   "Open the Slouchcat app",
		"fr;q=1, de;q=0.3":        "Öffne die Slouchcat-App",
		"":                        "Open the Slouchcat app",
	} {
		resp, b := call(t, "GET", srv.URL+"/tv", "", nil, map[string]string{"Accept-Language": lang})
		page := string(b)
		if resp.StatusCode != http.StatusOK || !strings.Contains(page, want) {
			t.Errorf("%q: %d, text %q missing", lang, resp.StatusCode, want)
		}
		if strings.Contains(strings.ToLower(page), "<script") || strings.Contains(page, "http") {
			t.Errorf("%q: the page has a script or loads something", lang)
		}
		csp := resp.Header.Get("Content-Security-Policy")
		m := regexp.MustCompile(`style-src 'sha256-([^']+)'`).FindStringSubmatch(csp)
		if !strings.HasPrefix(csp, "default-src 'none'") || m == nil || strings.Contains(csp, "script-src") {
			t.Fatalf("CSP %q", csp)
		}
		style := regexp.MustCompile(`(?s)<style>(.*)</style>`).FindStringSubmatch(page)
		sum := sha256.Sum256([]byte(style[1]))
		if base64.StdEncoding.EncodeToString(sum[:]) != m[1] {
			t.Error("the style hash in the CSP does not match the style of the page")
		}
		for k, v := range map[string]string{"Referrer-Policy": "no-referrer", "X-Content-Type-Options": "nosniff", "Vary": "Accept-Language"} {
			if resp.Header.Get(k) != v {
				t.Errorf("%s = %q, want %q", k, resp.Header.Get(k), v)
			}
		}
	}
}
