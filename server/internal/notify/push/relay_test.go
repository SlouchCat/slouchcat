package push

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Concept 27 7.3 (ADR 0044): an endpoint of the push relay gets a record of 2700 bytes, a payload of
// at most 2597 bytes, and only 410 deletes it; a browser endpoint keeps the old rules.

func TestIsRelay(t *testing.T) {
	for _, c := range []struct {
		endpoint, origin string
		want             bool
	}{
		{"https://push.slouchcat.com/v1/push/1.abc", DefaultRelayOrigin, true},
		{"https://PUSH.slouchcat.com/v1/push/1.abc", DefaultRelayOrigin, true},
		{"https://push.slouchcat.com:8443/v1/push/1.abc", DefaultRelayOrigin, false},
		{"http://push.slouchcat.com/v1/push/1.abc", DefaultRelayOrigin, false},
		{"https://push.slouchcat.com.example.test/v1/push/1.abc", DefaultRelayOrigin, false},
		{"https://fcm.googleapis.com/fcm/send/abc", DefaultRelayOrigin, false},
		{"https://push.slouchcat.com/v1/push/1.abc", "", false},
		{"http://127.0.0.1:8181/v1/push/1.abc", "http://127.0.0.1:8181", true},
	} {
		if got := IsRelay(c.endpoint, c.origin); got != c.want {
			t.Errorf("IsRelay(%q, %q) = %v, want %v", c.endpoint, c.origin, got, c.want)
		}
	}
}

// The relay (an httptest server in its place) gets a body of exactly 2700 bytes with the VAPID
// header, which the app's keys open to the payload; a browser endpoint still gets 4096.
func TestSendToRelay(t *testing.T) {
	b := newBrowser(t)
	svc := newPushService(t, b, http.StatusCreated)
	v := newVAPID(t)
	payload := []byte(`{"title":"Neues Gerät verbunden","body":"x","url":"/"}`)
	relay := Sender{VAPID: v, AllowPrivate: true, RelayOrigin: svc.srv.URL}
	if err := relay.Send(context.Background(), b.subscription(svc.srv.URL+"/v1/push/1.blob"), payload, time.Hour); err != nil {
		t.Fatal(err)
	}
	browser := Sender{VAPID: v, AllowPrivate: true}
	if err := browser.Send(context.Background(), b.subscription(svc.srv.URL+"/v1/push/1.blob"), payload, time.Hour); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.errs) != 0 {
		t.Fatalf("relay rejected: %v", svc.errs)
	}
	if len(svc.sizes) != 2 || svc.sizes[0] != RelayRecordSize || svc.sizes[1] != 4096 {
		t.Fatalf("body sizes %v, want [%d 4096]", svc.sizes, RelayRecordSize)
	}
	if !bytes.Equal(svc.payloads[0], payload) {
		t.Errorf("payload %q", svc.payloads[0])
	}
}

// The largest payload the relay takes still fits the record; one byte more is refused before
// anything is sent.
func TestRelayPayloadLimit(t *testing.T) {
	b := newBrowser(t)
	svc := newPushService(t, b, http.StatusCreated)
	s := Sender{VAPID: newVAPID(t), AllowPrivate: true, RelayOrigin: svc.srv.URL}
	sub := b.subscription(svc.srv.URL + "/v1/push/1.blob")
	if err := s.Send(context.Background(), sub, bytes.Repeat([]byte("a"), RelayMaxPayload), time.Hour); err != nil {
		t.Fatalf("payload of %d bytes: %v", RelayMaxPayload, err)
	}
	err := s.Send(context.Background(), sub, bytes.Repeat([]byte("a"), RelayMaxPayload+1), time.Hour)
	if err == nil || errors.Is(err, ErrSubscriptionGone) {
		t.Fatalf("payload of %d bytes: %v, want an error", RelayMaxPayload+1, err)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.sizes) != 1 || svc.sizes[0] != RelayRecordSize {
		t.Errorf("bodies %v, want one of %d", svc.sizes, RelayRecordSize)
	}
}

// For the relay only 410 is gone; 404 and every other answer is a failure to retry (security
// review of the services, ADR 0044 decision 8). A browser endpoint keeps 404 as gone.
func TestRelayAnswers(t *testing.T) {
	b := newBrowser(t)
	for _, tc := range []struct {
		status int
		gone   bool
	}{{http.StatusGone, true}, {http.StatusNotFound, false}, {http.StatusServiceUnavailable, false}, {http.StatusForbidden, false},
		{http.StatusRequestEntityTooLarge, false}} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			svc := newPushService(t, b, tc.status)
			endpoint := svc.srv.URL + "/v1/push/1.secret-blob"
			err := Sender{VAPID: newVAPID(t), AllowPrivate: true, RelayOrigin: svc.srv.URL}.Send(context.Background(), b.subscription(endpoint), []byte("x"), time.Hour)
			if err == nil || errors.Is(err, ErrSubscriptionGone) != tc.gone {
				t.Fatalf("err = %v, gone %v", err, tc.gone)
			}
			if strings.Contains(err.Error(), "secret-blob") {
				t.Errorf("error carries the endpoint: %v", err)
			}
		})
	}
}
