// Package push sends Web Push messages (RFC 8030) with message encryption (RFC 8291, aes128gcm)
// and VAPID (RFC 8292) for the push channel (M8, A-6), and creates VAPID key pairs. Encryption and
// the ES256 token come from github.com/SherClockHolmes/webpush-go; this package adds the checks of
// a subscription, the mapping of the push service's answers and the network rules. It has no
// database and logs nothing.
//
// Endpoint and keys of a subscription are capability secrets: no returned error carries the
// endpoint URL. Outside development the endpoint must be https and the client refuses to connect to
// loopback, private or link-local addresses, so a subscription cannot make the server call into its
// own network; redirects are not followed.
package push

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
)

// ErrSubscriptionGone is the answer 404 or 410 of the push service: the browser dropped the
// subscription, the server deletes it.
var ErrSubscriptionGone = errors.New("push subscription gone")

// Timeout bounds one request to a push service.
const Timeout = 30 * time.Second

// Subscription is the browser's PushSubscription: endpoint, p256dh and auth (base64url).
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// VAPID is the server's key pair (base64url: the uncompressed P-256 point and the private scalar)
// with the contact the push services see in the token (an https URL or an e-mail address).
type VAPID struct {
	PublicKey  string
	PrivateKey string
	Subscriber string
}

// Sender sends with one VAPID pair. AllowPrivate (development mode, tests) accepts http endpoints
// and private addresses.
type Sender struct {
	VAPID        VAPID
	AllowPrivate bool
	HTTP         *http.Client // nil = the package client for AllowPrivate
}

// GenerateVAPID returns a new VAPID key pair (base64url).
func GenerateVAPID() (pub, priv string, err error) {
	priv, pub, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("generate VAPID keys: %w", err)
	}
	return pub, priv, nil
}

// CheckEndpoint validates the endpoint of a subscription: an absolute https URL with a host (http
// too when allowInsecure). The error names the rule, not the URL.
func CheckEndpoint(endpoint string, allowInsecure bool) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil {
		return errors.New("not an absolute URL")
	}
	if u.Scheme != "https" && (!allowInsecure || u.Scheme != "http") {
		return errors.New("not an https URL")
	}
	return nil
}

// CheckKeys validates the keys of a subscription: p256dh a point on P-256 (65 bytes, uncompressed),
// auth 16 bytes, both base64url (padding optional).
func CheckKeys(p256dh, auth string) (badP256dh, badAuth bool) {
	if b, err := decodeKey(p256dh); err != nil {
		badP256dh = true
	} else if _, err := ecdh.P256().NewPublicKey(b); err != nil {
		badP256dh = true
	}
	if b, err := decodeKey(auth); err != nil || len(b) != 16 {
		badAuth = true
	}
	return badP256dh, badAuth
}

func decodeKey(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// Send encrypts payload for the subscription and hands it to its push service, which keeps it for
// ttl while the browser is offline. 404 and 410 return ErrSubscriptionGone.
func (s Sender) Send(ctx context.Context, sub Subscription, payload []byte, ttl time.Duration) error {
	if err := CheckEndpoint(sub.Endpoint, s.AllowPrivate); err != nil {
		return fmt.Errorf("push endpoint: %w", err)
	}
	client := s.HTTP
	if client == nil {
		client = defaultClient(s.AllowPrivate)
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient: client, Subscriber: s.VAPID.Subscriber, TTL: int(ttl / time.Second), Urgency: webpush.UrgencyNormal,
		VAPIDPublicKey: s.VAPID.PublicKey, VAPIDPrivateKey: s.VAPID.PrivateKey,
	})
	if err != nil {
		return fmt.Errorf("push: %w", withoutURL(err))
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
	}()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrSubscriptionGone
	default:
		return fmt.Errorf("push service answered %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
}

// withoutURL drops the request URL (the endpoint) from a transport error.
func withoutURL(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return uerr.Err
	}
	return err
}

var (
	guardedClient = sync.OnceValue(func() *http.Client { return newClient(false) })
	openClient    = sync.OnceValue(func() *http.Client { return newClient(true) })
)

func defaultClient(allowPrivate bool) *http.Client {
	if allowPrivate {
		return openClient()
	}
	return guardedClient()
}

// newClient returns the client of the push requests: no proxy, no redirects and, unless
// allowPrivate, no connection to a loopback, private, link-local or unspecified address (the guard
// of package netguard, checked on the resolved address, so a DNS name cannot point it inward).
func newClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: Timeout}
	if !allowPrivate {
		dialer.Control = netguard.RefuseLocal
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext, TLSHandshakeTimeout: 10 * time.Second, MaxIdleConns: 10,
			IdleConnTimeout: 90 * time.Second, ForceAttemptHTTP2: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
