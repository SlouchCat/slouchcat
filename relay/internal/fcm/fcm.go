// Package fcm sends data messages through Google's push service, Firebase Cloud Messaging HTTP v1
// (concept 27 section 7.3). It reads the service account of the app's Firebase project from a
// file outside the image (D-6), trades a signed assertion for an access token (OAuth 2.0 JWT bearer
// grant, RS256) and keeps that token until shortly before it expires.
//
// Both addresses are configurable, so tests and the fake in cmd/fcmfake can stand in for Google:
// the base of the send endpoint (default https://fcm.googleapis.com) and the token endpoint
// (default token_uri of the service account).
//
// Only the push forwarder imports this package; the meeting point service never does (E-21,
// enforced by depguard in relay/.golangci.yml).
package fcm

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Scope is the OAuth scope of the send endpoint.
const Scope = "https://www.googleapis.com/auth/firebase.messaging"

// ErrUnregistered means Google does not know the token any more (errorCode UNREGISTERED). It is
// the one case in which the forwarder answers 410 (D-10).
var ErrUnregistered = errors.New("fcm: token unregistered")

// Error is every other failure: of Google, of the network or of our credentials. The forwarder
// answers 503 for it; RetryAfter is passed on when Google named one.
type Error struct {
	Status     int
	Code       string
	RetryAfter string
	Err        error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return "fcm: " + e.Err.Error()
	}
	return fmt.Sprintf("fcm: status %d %s", e.Status, e.Code)
}

func (e *Error) Unwrap() error { return e.Err }

// ServiceAccount is the part of the service account file that is used.
type ServiceAccount struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// Client sends messages.
type Client struct {
	project  string
	email    string
	key      *rsa.PrivateKey
	tokenURL string
	baseURL  string
	http     *http.Client
	now      func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// Load reads the service account file. baseURL and tokenURL may be empty for Google's own.
func Load(path, baseURL, tokenURL string, hc *http.Client) (*Client, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the operator's configured credentials file
	if err != nil {
		return nil, err
	}
	var sa ServiceAccount
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, errors.New("firebase credentials: not a service account file")
	}
	return New(sa, baseURL, tokenURL, hc)
}

// New builds a client from a parsed service account.
func New(sa ServiceAccount, baseURL, tokenURL string, hc *http.Client) (*Client, error) {
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("firebase credentials: project_id, client_email or private_key missing")
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, errors.New("firebase credentials: private_key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("firebase credentials: private_key is not PKCS #8")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("firebase credentials: private_key is not RSA")
	}
	if tokenURL == "" {
		tokenURL = sa.TokenURI
	}
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token" //nolint:gosec // the address of Google's token endpoint, no credential
	}
	if baseURL == "" {
		baseURL = "https://fcm.googleapis.com"
	}
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		project: sa.ProjectID, email: sa.ClientEmail, key: key, tokenURL: tokenURL,
		baseURL: strings.TrimRight(baseURL, "/"), http: hc, now: time.Now,
	}, nil
}

// Message is one data message.
type Message struct {
	Token string
	Data  map[string]string
	TTL   time.Duration // 0: Google's default (four weeks)
	High  bool          // Android priority HIGH instead of NORMAL
}

type sendBody struct {
	Message sendMessage `json:"message"`
}

type sendMessage struct {
	Token   string            `json:"token"`
	Data    map[string]string `json:"data"`
	Android sendAndroid       `json:"android"`
}

type sendAndroid struct {
	Priority string `json:"priority"`
	TTL      string `json:"ttl,omitempty"`
}

type googleError struct {
	Error struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Details []struct {
			Type      string `json:"@type"`
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

// Send delivers m.
func (c *Client) Send(ctx context.Context, m Message) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	body := sendBody{Message: sendMessage{Token: m.Token, Data: m.Data, Android: sendAndroid{Priority: "NORMAL"}}}
	if m.High {
		body.Message.Android.Priority = "HIGH"
	}
	if m.TTL > 0 {
		body.Message.Android.TTL = strconv.FormatInt(int64(m.TTL/time.Second), 10) + "s"
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/projects/"+url.PathEscape(c.project)+"/messages:send", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return &Error{Err: errors.New("send request failed")}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	var ge googleError
	_ = json.Unmarshal(raw, &ge)
	code := ge.Error.Status
	for _, d := range ge.Error.Details {
		if d.ErrorCode != "" {
			code = d.ErrorCode
		}
	}
	// Only UNREGISTERED says the token is gone. INVALID_ARGUMENT can also mean a fault of the
	// message, which would be ours; it stays a 503 like everything else (D-10).
	if code == "UNREGISTERED" {
		return ErrUnregistered
	}
	if resp.StatusCode == http.StatusUnauthorized {
		c.dropToken()
	}
	return &Error{Status: resp.StatusCode, Code: code, RetryAfter: resp.Header.Get("Retry-After")}
}

func (c *Client) dropToken() {
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
}

// accessToken returns a cached token or fetches a new one.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Before(c.expires) {
		return c.token, nil
	}
	assertion, err := c.assertion()
	if err != nil {
		return "", &Error{Err: err}
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", &Error{Err: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", &Error{Err: errors.New("token request failed")}
	}
	defer resp.Body.Close()
	var t struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if resp.StatusCode != http.StatusOK {
		return "", &Error{Status: resp.StatusCode, Code: "TOKEN_ENDPOINT", Err: fmt.Errorf("token endpoint answered %d", resp.StatusCode)}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&t); err != nil || t.AccessToken == "" {
		return "", &Error{Err: errors.New("token endpoint sent no access token")}
	}
	if t.ExpiresIn <= 0 {
		t.ExpiresIn = 3600
	}
	c.token = t.AccessToken
	// Renew five minutes early, or at half the lifetime for a very short one.
	margin := min(5*time.Minute, time.Duration(t.ExpiresIn)*time.Second/2)
	c.expires = c.now().Add(time.Duration(t.ExpiresIn)*time.Second - margin)
	return c.token, nil
}

// assertion is the RS256 JWT of the JWT bearer grant.
func (c *Client) assertion() (string, error) {
	now := c.now()
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	cl, _ := json.Marshal(map[string]any{
		"iss": c.email, "scope": Scope, "aud": c.tokenURL,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	enc := base64.RawURLEncoding
	signing := enc.EncodeToString(h) + "." + enc.EncodeToString(cl)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signing + "." + enc.EncodeToString(sig), nil
}
