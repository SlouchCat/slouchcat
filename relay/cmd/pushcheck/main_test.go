package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/fcm"
	"github.com/slouchcat/slouchcat/relay/internal/fcmfake"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
	"github.com/slouchcat/slouchcat/relay/internal/push"
	"github.com/slouchcat/slouchcat/relay/internal/seal"
)

func forwarder(t *testing.T, keys *seal.Keys, fakeURL string, f *fcmfake.Fake) *httptest.Server {
	t.Helper()
	callers, _ := clientip.ParseTrusted("")
	h := &push.Handler{Keys: keys, MaxBody: 2700, Callers: callers, Log: slog.New(slog.DiscardHandler),
		RegisterLimit: limit.New(100, 100, 100), CallerLimit: limit.New(100, 100, 100), AddressLimit: limit.New(100, 100, 100)}
	sa, _ := f.ServiceAccount(fakeURL + "/token")
	var parsed fcm.ServiceAccount
	_ = json.Unmarshal(sa, &parsed)
	c, err := fcm.New(parsed, fakeURL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	h.FCM = c
	srv := httptest.NewServer(nil)
	h.Origins = []string{"https://push.example.test", srv.URL}
	srv.Config.Handler = h.Routes()
	t.Cleanup(srv.Close)
	return srv
}

// pushcheck against the real forwarder and the fake: everything passes, and the move rehearsal
// (seal at the old instance, push to the new one) passes with the copied key and gives 503
// without it.
func TestPushcheck(t *testing.T) {
	f, _ := fcmfake.New("proj")
	fake := httptest.NewServer(f.Handler())
	defer fake.Close()
	key := seal.NewKey()
	keys, _ := seal.Parse(seal.File{Current: "1", Keys: map[string]string{"1": key}})
	old := forwarder(t, keys, fake.URL, f)
	client := &http.Client{Timeout: 10 * time.Second}

	var out bytes.Buffer
	n, err := Run(client, client, client, Options{URL: old.URL, FCMFake: fake.URL, MaxBody: 2700}, &out)
	if err != nil || n < 10 {
		t.Fatalf("%d checks, %v\n%s", n, err, out.String())
	}

	copied, _ := seal.Parse(seal.File{Current: "1", Keys: map[string]string{"1": key}})
	moved := forwarder(t, copied, fake.URL, f)
	out.Reset()
	if n, err := Run(client, client, client, Options{URL: moved.URL, RegisterURL: old.URL, FCMFake: fake.URL, MaxBody: 2700}, &out); err != nil {
		t.Fatalf("with the copied key, after %d checks: %v\n%s", n, err, out.String())
	}

	without, _ := seal.Parse(seal.File{Current: "1", Keys: map[string]string{"1": seal.NewKey()}})
	bare := forwarder(t, without, fake.URL, f)
	out.Reset()
	if n, err := Run(client, client, client, Options{URL: bare.URL, RegisterURL: old.URL, FCMFake: fake.URL, ExpectUnavailable: true}, &out); err != nil {
		t.Fatalf("without the key, after %d checks: %v\n%s", n, err, out.String())
	}
	// And the other way round: expecting 503 from a good instance fails.
	if _, err := Run(client, client, client, Options{URL: old.URL, FCMFake: fake.URL, ExpectUnavailable: true}, &out); err == nil {
		t.Fatal("-expect-unavailable passed against an instance that has the key")
	}
}
