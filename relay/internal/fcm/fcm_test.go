package fcm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/fcmfake"
)

func newFake(t *testing.T) (*fcmfake.Fake, *httptest.Server, *Client) {
	t.Helper()
	f, err := fcmfake.New("proj")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(f.Handler())
	t.Cleanup(srv.Close)
	sa, err := f.ServiceAccount(srv.URL + "/token")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "firebase.json")
	if err := os.WriteFile(path, sa, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path, srv.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return f, srv, c
}

func TestSendAndTokenCache(t *testing.T) {
	f, _, c := newFake(t)
	ctx := context.Background()
	for range 3 {
		if err := c.Send(ctx, Message{Token: "dev", Data: map[string]string{"wp": "AAA"}, TTL: time.Hour, High: true}); err != nil {
			t.Fatal(err)
		}
	}
	msgs := f.Messages("dev")
	if len(msgs) != 3 || msgs[0].Data["wp"] != "AAA" || msgs[0].Priority != "HIGH" || msgs[0].TTL != "3600s" {
		t.Fatalf("messages = %+v", msgs)
	}
	if n := f.TokenRequests(); n != 1 {
		t.Fatalf("%d token requests for three messages, want 1", n)
	}
	// Renewed before it runs out.
	c.now = func() time.Time { return time.Now().Add(56 * time.Minute) }
	if err := c.Send(ctx, Message{Token: "dev", Data: map[string]string{"wp": "B"}}); err != nil {
		t.Fatal(err)
	}
	if n := f.TokenRequests(); n != 2 {
		t.Fatalf("%d token requests after 56 minutes, want 2", n)
	}
}

func TestErrors(t *testing.T) {
	_, _, c := newFake(t)
	ctx := context.Background()
	if err := c.Send(ctx, Message{Token: "unregistered-1", Data: map[string]string{"wp": "A"}}); !errors.Is(err, ErrUnregistered) {
		t.Fatalf("unregistered: %v", err)
	}
	for _, tok := range []string{"invalid-1", "fail-1", "busy-1"} {
		err := c.Send(ctx, Message{Token: tok, Data: map[string]string{"wp": "A"}})
		var fe *Error
		if !errors.As(err, &fe) || errors.Is(err, ErrUnregistered) {
			t.Fatalf("%s: %v", tok, err)
		}
		if tok == "busy-1" && fe.RetryAfter != "7" {
			t.Fatalf("Retry-After %q not passed on", fe.RetryAfter)
		}
	}
}

func TestTokenEndpointFailure(t *testing.T) {
	f, _, _ := newFake(t)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer bad.Close()
	sa, _ := f.ServiceAccount(bad.URL)
	var parsed ServiceAccount
	_ = json.Unmarshal(sa, &parsed)
	c, err := New(parsed, bad.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Send(context.Background(), Message{Token: "dev", Data: map[string]string{"wp": "A"}})
	var fe *Error
	if !errors.As(err, &fe) {
		t.Fatalf("token endpoint failure: %v", err)
	}
	// An unreachable Google is an Error too, never ErrUnregistered.
	c2, _ := New(parsed, "http://127.0.0.1:1", "http://127.0.0.1:1/token", &http.Client{Timeout: time.Second})
	if err := c2.Send(context.Background(), Message{Token: "dev"}); !errors.As(err, &fe) {
		t.Fatalf("unreachable: %v", err)
	}
}

func TestLoadRejects(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"not json":   "nope",
		"no key":     `{"project_id":"p","client_email":"e"}`,
		"key no pem": `{"project_id":"p","client_email":"e","private_key":"abc"}`,
	} {
		path := filepath.Join(dir, "f.json")
		_ = os.WriteFile(path, []byte(content), 0o600)
		if _, err := Load(path, "", "", nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Load(filepath.Join(dir, "missing"), "", "", nil); err == nil {
		t.Error("missing file accepted")
	}
}
