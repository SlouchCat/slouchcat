package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/fcmfake"
)

// gatedFake puts a gate in front of the fake's token endpoint and counts its requests. arrived is
// closed when the first token request comes in.
func gatedFake(t *testing.T, gate <-chan struct{}) (f *fcmfake.Fake, c *Client, calls *atomic.Int32, arrived <-chan struct{}) {
	t.Helper()
	f, err := fcmfake.New("proj")
	if err != nil {
		t.Fatal(err)
	}
	calls = &atomic.Int32{}
	first := make(chan struct{})
	var once sync.Once
	inner := f.Handler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			// Read the body first: only then does the server notice a client that hangs up.
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			calls.Add(1)
			once.Do(func() { close(first) })
			select {
			case <-gate:
			case <-r.Context().Done():
				return
			}
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	sa, _ := f.ServiceAccount(srv.URL + "/token")
	var parsed ServiceAccount
	_ = json.Unmarshal(sa, &parsed)
	c, err = New(parsed, srv.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return f, c, calls, first
}

// Twenty sends at once while the token endpoint is slow: one token request, and a caller that
// gives up leaves at once without cancelling the request for the others (code review major 1).
func TestConcurrentSendsShareOneTokenRequest(t *testing.T) {
	gate := make(chan struct{})
	f, c, calls, arrived := gatedFake(t, gate)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- c.Send(context.Background(), Message{Token: "dev", Data: map[string]string{"wp": "A"}})
		}()
	}
	<-arrived
	quitter, quit := context.WithCancel(context.Background())
	quitDone := make(chan error, 1)
	go func() { quitDone <- c.Send(quitter, Message{Token: "dev", Data: map[string]string{"wp": "q"}}) }()
	quit()
	var fe *Error
	if err := <-quitDone; !errors.As(err, &fe) {
		t.Fatalf("the caller that gave up: %v", err)
	}
	close(gate)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("send: %v", err)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("%d token requests, want 1", n)
	}
	if n := len(f.Messages("dev")); n != 20 {
		t.Fatalf("%d messages, want 20", n)
	}
}

// A token endpoint that hangs: the request ends after TokenTimeout, every waiting send fails with
// an Error, and for FailureMemo no new request is made.
func TestHangingTokenEndpoint(t *testing.T) {
	gate := make(chan struct{}) // never opened
	_, c, calls, _ := gatedFake(t, gate)
	c.TokenTimeout = 100 * time.Millisecond
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var fe *Error
			if err := c.Send(context.Background(), Message{Token: "dev"}); !errors.As(err, &fe) {
				t.Errorf("send: %v", err)
			}
		}()
	}
	wg.Wait()
	start := time.Now()
	var fe *Error
	if err := c.Send(context.Background(), Message{Token: "dev"}); !errors.As(err, &fe) {
		t.Fatalf("send within the failure memo: %v", err)
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("send within the failure memo took %s; it should fail at once", d)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("%d token requests, want 1", n)
	}
}
