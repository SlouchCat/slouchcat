package push

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/fcm"
	"github.com/slouchcat/slouchcat/relay/internal/fcmfake"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
	"github.com/slouchcat/slouchcat/relay/internal/logging"
	"github.com/slouchcat/slouchcat/relay/internal/seal"
	"github.com/slouchcat/slouchcat/relay/internal/vapid"
)

type env struct {
	t     *testing.T
	fake  *fcmfake.Fake
	h     *Handler
	srv   *httptest.Server
	vapid *ecdsa.PrivateKey
	logs  *bytes.Buffer
}

func newEnv(t *testing.T, keys *seal.Keys, withFCM bool) *env {
	t.Helper()
	f, err := fcmfake.New("proj")
	if err != nil {
		t.Fatal(err)
	}
	fsrv := httptest.NewServer(f.Handler())
	t.Cleanup(fsrv.Close)
	callers, _ := clientip.ParseTrusted("127.0.0.1")
	var logs bytes.Buffer
	log, _ := logging.New(&logs, "json", slog.LevelDebug)
	h := &Handler{
		Keys: keys, MaxBody: 3000, Callers: callers, Log: log,
		RegisterLimit: limit.New(100, 100, 100), CallerLimit: limit.New(100, 100, 100), AddressLimit: limit.New(100, 100, 100),
	}
	if withFCM {
		sa, _ := f.ServiceAccount(fsrv.URL + "/token")
		var parsed fcm.ServiceAccount
		_ = json.Unmarshal(sa, &parsed)
		c, err := fcm.New(parsed, fsrv.URL, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		h.FCM = c
	}
	srv := httptest.NewServer(nil)
	h.Origins = []string{"https://push.example.test", srv.URL}
	srv.Config.Handler = h.Routes()
	t.Cleanup(srv.Close)
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return &env{t: t, fake: f, h: h, srv: srv, vapid: k, logs: &logs}
}

func newKeys(t *testing.T, current string, versions map[string]string) *seal.Keys {
	t.Helper()
	k, err := seal.Parse(seal.File{Current: current, Keys: versions})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func (e *env) register(token string) (int, string) {
	pub, _ := vapid.PublicKeyString(&e.vapid.PublicKey)
	b, _ := json.Marshal(map[string]string{"fcmToken": token, "vapidKey": pub})
	resp, err := http.Post(e.srv.URL+"/v1/register", "application/json", bytes.NewReader(b))
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var r struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return resp.StatusCode, r.Endpoint
}

func (e *env) push(endpoint string, body []byte, hdr map[string]string) int {
	u, _ := url.Parse(endpoint)
	auth, _ := vapid.Authorization(e.vapid, vapid.Claims{Aud: e.srv.URL, Exp: time.Now().Add(time.Hour).Unix()})
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+u.Path, bytes.NewReader(body))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", "600")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestForwardsUnchangedBody(t *testing.T) {
	e := newEnv(t, newKeys(t, "1", map[string]string{"1": seal.NewKey()}), true)
	status, endpoint := e.register("dev-1")
	if status != http.StatusOK || !strings.HasPrefix(endpoint, e.srv.URL+"/v1/push/1.") {
		t.Fatalf("register: %d %q", status, endpoint)
	}
	body := []byte("ciphertext \x00\x01\x02 of the server")
	if s := e.push(endpoint, body, map[string]string{"Urgency": "high"}); s != http.StatusCreated {
		t.Fatalf("push: %d", s)
	}
	msgs := e.fake.Messages("dev-1")
	if len(msgs) != 1 {
		t.Fatalf("%d messages", len(msgs))
	}
	raw, _ := base64.StdEncoding.DecodeString(msgs[0].Data["wp"])
	if !bytes.Equal(raw, body) || msgs[0].Priority != "HIGH" || msgs[0].TTL != "600s" || len(msgs[0].Data) != 1 {
		t.Fatalf("message = %+v", msgs[0])
	}
	// A registration that does not come in on an own name gets the fixed name.
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/v1/register", strings.NewReader(`{"fcmToken":"x","vapidKey":"`+mustPub(e.vapid)+`"}`))
	req.Host = "other.example.test"
	resp, _ := http.DefaultClient.Do(req)
	var r struct{ Endpoint string }
	_ = json.NewDecoder(resp.Body).Decode(&r)
	_ = resp.Body.Close()
	if !strings.HasPrefix(r.Endpoint, "https://push.example.test/v1/push/") {
		t.Fatalf("endpoint for a foreign host = %q", r.Endpoint)
	}
	// Neither token nor address reach the log.
	if logs := e.logs.String(); strings.Contains(logs, "dev-1") || strings.Contains(logs, strings.TrimPrefix(endpoint, e.srv.URL+"/v1/push/")) || strings.Contains(logs, "127.0.0.1") {
		t.Fatalf("log leaks:\n%s", logs)
	}
}

func mustPub(k *ecdsa.PrivateKey) string {
	s, _ := vapid.PublicKeyString(&k.PublicKey)
	return s
}

// D-10: 410 only when Google no longer knows the token; every own failure is 503.
func TestGoneOnlyForUnknownToken(t *testing.T) {
	key := seal.NewKey()
	e := newEnv(t, newKeys(t, "1", map[string]string{"1": key}), true)
	_, gone := e.register("unregistered-abc")
	if s := e.push(gone, []byte("x"), nil); s != http.StatusGone {
		t.Fatalf("unregistered token: %d, want 410", s)
	}
	for _, tok := range []string{"invalid-abc", "fail-abc", "busy-abc"} {
		_, ep := e.register(tok)
		if s := e.push(ep, []byte("x"), nil); s != http.StatusServiceUnavailable {
			t.Fatalf("%s: %d, want 503", tok, s)
		}
	}
	_, good := e.register("dev-2")

	// The same address at instances that cannot open it: 503, never 404 or 410.
	for name, keys := range map[string]*seal.Keys{
		"wrong sealing key":   newKeys(t, "1", map[string]string{"1": seal.NewKey()}),
		"unknown version":     newKeys(t, "2", map[string]string{"2": key}),
		"no sealing key file": nil,
	} {
		other := newEnv(t, keys, true)
		if s := other.push(good, []byte("x"), nil); s != http.StatusServiceUnavailable {
			t.Errorf("%s: %d, want 503", name, s)
		}
		if n := len(other.fake.Messages("")); n != 0 {
			t.Errorf("%s: %d messages reached Google", name, n)
		}
	}
	noKey := newEnv(t, nil, true)
	if s, _ := noKey.register("dev-3"); s != http.StatusServiceUnavailable {
		t.Errorf("register without a key: %d, want 503", s)
	}
	resp, _ := http.Get(noKey.srv.URL + "/v1/healthz")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("health without a key: %d, want 503", resp.StatusCode)
	}
	noFCM := newEnv(t, newKeys(t, "1", map[string]string{"1": key}), false)
	noFCM.vapid = e.vapid
	if s := noFCM.push(good, []byte("x"), nil); s != http.StatusServiceUnavailable {
		t.Errorf("without Firebase credentials: %d, want 503", s)
	}
	for _, garbage := range []string{"x", "1.", "1.AAAA", "nodot", "%2e%2e"} {
		if s := e.push(e.srv.URL+"/v1/push/"+garbage, []byte("x"), nil); s == http.StatusNotFound || s == http.StatusGone {
			t.Errorf("garbage address %q: %d (a server would delete the subscription)", garbage, s)
		}
	}
}

func TestVAPIDAndInput(t *testing.T) {
	e := newEnv(t, newKeys(t, "1", map[string]string{"1": seal.NewKey()}), true)
	_, ep := e.register("dev-4")
	u, _ := url.Parse(ep)
	send := func(auth string, hdr map[string]string, body []byte) int {
		req, _ := http.NewRequest(http.MethodPost, e.srv.URL+u.Path, bytes.NewReader(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		req.Header.Set("Content-Encoding", "aes128gcm")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	sign := func(k *ecdsa.PrivateKey, aud string, exp time.Duration) string {
		a, _ := vapid.Authorization(k, vapid.Claims{Aud: aud, Exp: time.Now().Add(exp).Unix()})
		return a
	}
	for name, c := range map[string]struct {
		auth string
		hdr  map[string]string
		body []byte
		want int
	}{
		"no signature":       {"", nil, []byte("x"), http.StatusForbidden},
		"another server":     {sign(other, e.srv.URL, time.Hour), nil, []byte("x"), http.StatusForbidden},
		"foreign aud":        {sign(e.vapid, "https://elsewhere.example.test", time.Hour), nil, []byte("x"), http.StatusForbidden},
		"expired":            {sign(e.vapid, e.srv.URL, -time.Minute), nil, []byte("x"), http.StatusForbidden},
		"fixed name as aud":  {sign(e.vapid, "https://push.example.test", time.Hour), nil, []byte("x"), http.StatusCreated},
		"too large":          {sign(e.vapid, e.srv.URL, time.Hour), nil, make([]byte, 3001), http.StatusRequestEntityTooLarge},
		"largest allowed":    {sign(e.vapid, e.srv.URL, time.Hour), nil, make([]byte, 3000), http.StatusCreated},
		"empty":              {sign(e.vapid, e.srv.URL, time.Hour), nil, nil, http.StatusBadRequest},
		"old content coding": {sign(e.vapid, e.srv.URL, time.Hour), map[string]string{"Content-Encoding": "aesgcm"}, []byte("x"), http.StatusUnsupportedMediaType},
	} {
		if s := send(c.auth, c.hdr, c.body); s != c.want {
			t.Errorf("%s: %d, want %d", name, s, c.want)
		}
	}
	if resp, _ := http.Post(e.srv.URL+"/v1/register", "application/json", strings.NewReader(`{"fcmToken":"x","vapidKey":"not-a-key"}`)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("register with a bad VAPID key: %d", resp.StatusCode)
	}
	if resp, _ := http.Post(e.srv.URL+"/v1/register", "application/json", strings.NewReader(`{"fcmToken":"","vapidKey":"`+mustPub(e.vapid)+`"}`)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("register without a token: %d", resp.StatusCode)
	}
}

func TestRateLimits(t *testing.T) {
	e := newEnv(t, newKeys(t, "1", map[string]string{"1": seal.NewKey()}), true)
	e.h.AddressLimit = limit.New(0.001, 2, 100)
	_, a := e.register("dev-5")
	_, b := e.register("dev-6")
	for range 2 {
		if s := e.push(a, []byte("x"), nil); s != http.StatusCreated {
			t.Fatalf("push: %d", s)
		}
	}
	if s := e.push(a, []byte("x"), nil); s != http.StatusTooManyRequests {
		t.Fatalf("third push to one address: %d, want 429", s)
	}
	if s := e.push(b, []byte("x"), nil); s != http.StatusCreated {
		t.Fatalf("another address: %d", s)
	}
	e.h.CallerLimit = limit.New(0.001, 1, 100)
	if s := e.push(b, []byte("x"), map[string]string{"X-Forwarded-For": "203.0.113.1"}); s != http.StatusCreated {
		t.Fatalf("first push of a caller: %d", s)
	}
	if s := e.push(b, []byte("x"), map[string]string{"X-Forwarded-For": "203.0.113.1"}); s != http.StatusTooManyRequests {
		t.Fatalf("second push of a caller: %d, want 429", s)
	}
	e.h.RegisterLimit = limit.New(0.001, 1, 100)
	e.register("dev-7")
	if s, _ := e.register("dev-8"); s != http.StatusTooManyRequests {
		t.Fatalf("second registration: %d, want 429", s)
	}
}
