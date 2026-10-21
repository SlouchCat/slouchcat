package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// browser is the receiving side of a subscription: its ECDH key pair and auth secret.
type browser struct {
	key  *ecdh.PrivateKey
	auth []byte
}

func newBrowser(t *testing.T) browser {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return browser{key: key, auth: auth}
}

func (b browser) subscription(endpoint string) Subscription {
	return Subscription{Endpoint: endpoint, P256dh: b64(b.key.PublicKey().Bytes()), Auth: b64(b.auth)}
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// decrypt opens an aes128gcm body (RFC 8188 header, RFC 8291 keys) with the browser's keys.
func (b browser) decrypt(body []byte) ([]byte, error) {
	if len(body) < 21 {
		return nil, errors.New("short body")
	}
	salt, idLen := body[:16], int(body[20])
	if binary.BigEndian.Uint32(body[16:20]) < 18 || len(body) < 21+idLen {
		return nil, errors.New("bad header")
	}
	serverPub, err := ecdh.P256().NewPublicKey(body[21 : 21+idLen])
	if err != nil {
		return nil, err
	}
	secret, err := b.key.ECDH(serverPub)
	if err != nil {
		return nil, err
	}
	info := append(append([]byte("WebPush: info\x00"), b.key.PublicKey().Bytes()...), serverPub.Bytes()...)
	ikm, err := hkdf.Key(sha256.New, secret, b.auth, string(info), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, body[21+idLen:], nil)
	if err != nil {
		return nil, err
	}
	plain = bytes.TrimRight(plain, "\x00") // padding
	if len(plain) == 0 || plain[len(plain)-1] != 2 {
		return nil, errors.New("no last-record delimiter")
	}
	return plain[:len(plain)-1], nil
}

// checkVAPID verifies "vapid t=<JWT>, k=<key>": the ES256 signature with k, aud = origin, exp in
// the next 24 hours, sub mailto: or https:. It returns k.
func checkVAPID(header, origin string) (string, error) {
	rest, ok := strings.CutPrefix(header, "vapid ")
	if !ok {
		return "", errors.New("not vapid")
	}
	var token, key string
	for _, part := range strings.Split(rest, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			token = v
		case "k":
			key = v
		}
	}
	segs := strings.Split(token, ".")
	if len(segs) != 3 {
		return "", errors.New("not a JWT")
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), pubBytes)
	if err != nil {
		return "", err
	}
	sig, err := base64.RawURLEncoding.DecodeString(segs[2])
	if err != nil || len(sig) != 64 {
		return "", errors.New("bad signature encoding")
	}
	digest := sha256.Sum256([]byte(segs[0] + "." + segs[1]))
	if !ecdsa.Verify(pub, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return "", errors.New("signature does not verify")
	}
	raw, err := base64.RawURLEncoding.DecodeString(segs[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", err
	}
	exp := time.Unix(claims.Exp, 0)
	switch {
	case claims.Aud != origin:
		return "", fmt.Errorf("aud %q", claims.Aud)
	case exp.Before(time.Now()) || exp.After(time.Now().Add(24*time.Hour)):
		return "", fmt.Errorf("exp %v", exp)
	case !strings.HasPrefix(claims.Sub, "mailto:") && !strings.HasPrefix(claims.Sub, "https:"):
		return "", fmt.Errorf("sub %q", claims.Sub)
	}
	return key, nil
}

// pushService is an httptest push service that checks a request like a real one and keeps the
// decrypted payloads.
type pushService struct {
	srv      *httptest.Server
	browser  browser
	status   int
	mu       sync.Mutex
	payloads [][]byte
	errs     []error
	ttl      string
}

func newPushService(t *testing.T, b browser, status int) *pushService {
	t.Helper()
	p := &pushService{browser: b, status: status}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		defer p.mu.Unlock()
		p.ttl = r.Header.Get("TTL")
		fail := func(err error) {
			p.errs = append(p.errs, err)
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Encoding") != "aes128gcm" || r.Header.Get("TTL") == "" {
			fail(fmt.Errorf("method %s, encoding %q, ttl %q", r.Method, r.Header.Get("Content-Encoding"), r.Header.Get("TTL")))
			return
		}
		if _, err := checkVAPID(r.Header.Get("Authorization"), p.srv.URL); err != nil {
			fail(err)
			return
		}
		plain, err := p.browser.decrypt(body)
		if err != nil {
			fail(err)
			return
		}
		p.payloads = append(p.payloads, plain)
		w.WriteHeader(p.status)
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func newVAPID(t *testing.T) VAPID {
	t.Helper()
	pub, priv, err := GenerateVAPID()
	if err != nil {
		t.Fatal(err)
	}
	return VAPID{PublicKey: pub, PrivateKey: priv, Subscriber: "https://slouchcat.example.ch"}
}

// The push service gets an aes128gcm body that the browser's keys decrypt to the payload, a VAPID
// token that verifies with the server's public key, and the TTL.
func TestSendEncryptsAndSigns(t *testing.T) {
	b := newBrowser(t)
	svc := newPushService(t, b, http.StatusCreated)
	v := newVAPID(t)
	s := Sender{VAPID: v, AllowPrivate: true}
	payload := []byte(`{"title":"[Kind] Neue Folge","body":"S01E02","url":"/items/x"}`)
	if err := s.Send(context.Background(), b.subscription(svc.srv.URL+"/push/abc"), payload, time.Minute); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.errs) != 0 {
		t.Fatalf("push service rejected: %v", svc.errs)
	}
	if len(svc.payloads) != 1 || !bytes.Equal(svc.payloads[0], payload) {
		t.Errorf("payloads = %q", svc.payloads)
	}
	if svc.ttl != "60" {
		t.Errorf("TTL = %q", svc.ttl)
	}
	// The token's key is the server's public key.
	pub, _ := base64.RawURLEncoding.DecodeString(v.PublicKey)
	if len(pub) != 65 || pub[0] != 4 {
		t.Errorf("public key = %d bytes", len(pub))
	}
}

func TestSendAnswers(t *testing.T) {
	b := newBrowser(t)
	for _, tc := range []struct {
		status int
		gone   bool
	}{{http.StatusGone, true}, {http.StatusNotFound, true}, {http.StatusTooManyRequests, false}, {http.StatusInternalServerError, false}} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			svc := newPushService(t, b, tc.status)
			endpoint := svc.srv.URL + "/push/secret-capability-token"
			err := Sender{VAPID: newVAPID(t), AllowPrivate: true}.Send(context.Background(), b.subscription(endpoint), []byte("x"), time.Hour)
			if errors.Is(err, ErrSubscriptionGone) != tc.gone || err == nil {
				t.Fatalf("err = %v, gone %v", err, tc.gone)
			}
			if strings.Contains(err.Error(), "secret-capability-token") {
				t.Errorf("error carries the endpoint: %v", err)
			}
		})
	}
}

// A transport error names neither the endpoint nor its path.
func TestSendTransportErrorWithoutEndpoint(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	b := newBrowser(t)
	err = Sender{VAPID: newVAPID(t), AllowPrivate: true}.Send(context.Background(),
		b.subscription("http://"+addr+"/push/secret-capability-token"), []byte("x"), time.Hour)
	if err == nil || strings.Contains(err.Error(), "secret-capability-token") || strings.Contains(err.Error(), "http://") {
		t.Errorf("err = %v", err)
	}
}

// Outside development an endpoint must be https and may not point at a private or loopback address.
func TestSendGuard(t *testing.T) {
	b := newBrowser(t)
	var hits int
	var mu sync.Mutex
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	s := Sender{VAPID: newVAPID(t)}
	if err := s.Send(context.Background(), b.subscription(srv.URL+"/push/x"), []byte("x"), time.Hour); err == nil ||
		!strings.Contains(err.Error(), "not allowed") {
		t.Errorf("loopback https endpoint: %v", err)
	}
	if err := s.Send(context.Background(), b.subscription("http://push.example.ch/x"), []byte("x"), time.Hour); err == nil ||
		!strings.Contains(err.Error(), "https") {
		t.Errorf("http endpoint: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 0 {
		t.Errorf("guarded sender reached the server %d times", hits)
	}
}

func TestCheckEndpointAndKeys(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		insecure bool
		ok       bool
	}{
		{"https://fcm.googleapis.com/fcm/send/abc", false, true},
		{"http://127.0.0.1:9000/push/1", false, false},
		{"http://127.0.0.1:9000/push/1", true, true},
		{"ftp://push.example.ch/x", true, false},
		{"/relative", true, false},
		{"https://user:pw@push.example.ch/x", false, false},
	} {
		if err := CheckEndpoint(tc.endpoint, tc.insecure); (err == nil) != tc.ok {
			t.Errorf("CheckEndpoint(%q, %v) = %v", tc.endpoint, tc.insecure, err)
		}
	}
	b := newBrowser(t)
	sub := b.subscription("https://x")
	if p, a := CheckKeys(sub.P256dh, sub.Auth); p || a {
		t.Errorf("valid keys rejected: %v %v", p, a)
	}
	if p, a := CheckKeys(sub.P256dh+"==", sub.Auth+"=="); p || a {
		t.Errorf("padded keys rejected: %v %v", p, a)
	}
	if p, a := CheckKeys("BNcR", "tBHI"); !p || !a {
		t.Errorf("short keys accepted: %v %v", p, a)
	}
	notOnCurve := append([]byte{4}, bytes.Repeat([]byte{1}, 64)...)
	if p, _ := CheckKeys(b64(notOnCurve), sub.Auth); !p {
		t.Error("point off the curve accepted")
	}
}
