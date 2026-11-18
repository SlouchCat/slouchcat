// Package seerr is the HTTP client of Seerr (formerly Jellyseerr; concept 25, architecture 2.2).
// It has no database and no business rules: it sends the calls of the groups connection, users,
// quota, settings, requests, titles, discover and search, and maps the answers to narrow types and
// the error classes of errors.go.
//
// Rules of the client:
//
//   - Only the address the administrator entered, paths always below /api/v1, only http and https,
//     no user info in the address. Addresses in the household's network are allowed on purpose:
//     the client dials without netguard.RefuseLocal, because Seerr usually runs on the LAN and only
//     the administrator sets the address (architecture 15.3).
//   - No redirect is followed; a 3xx answer is ErrRedirect. The key in X-Api-Key and a password in
//     the body never travel to another address.
//   - 10 s per call, at most 5 calls per second per client, every call bound to its context.
//   - Every call sends X-Api-Key. Whether a call also sends X-API-User is chosen by the Session it
//     runs on: AsAdmin sends none (Seerr then acts as user 1, the owner with ADMIN), AsUser(id)
//     sends X-API-User: id (Seerr then applies that user's rights, quota and auto-approval;
//     appendix D 2.1, 3.3). There is no third way to make a call.
//   - Nothing secret in an error or a log line: the log has method, path without query, status,
//     duration and the session kind; never a header or a body. Errors name the method and path
//     and Seerr's message, from which the key and a password of the body are removed.
//   - No retries; the job queue retries.
package seerr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// Limits (architecture 2.2).
const (
	// Timeout bounds every call, the answer included.
	Timeout = 10 * time.Second
	// Rate and Burst limit the calls per client.
	Rate  = 5
	Burst = 5
	// MaxResponseBytes is the largest answer the client reads; a larger one is ErrServer.
	MaxResponseBytes = 4 << 20

	apiPrefix = "/api/v1"
	userAgent = "Slouchcat"
)

// Client talks to one Seerr instance. It is safe for concurrent use.
type Client struct {
	base    string // scheme, host and optional path prefix, without trailing "/"
	key     Secret
	http    *http.Client
	limiter *ratelimit.Limiter
	log     *slog.Logger
	agent   string
	observe func(context.Context, Call)
}

// Call describes one finished call for an observer (WithObserver): method, path without query,
// status (0 without an answer), duration and the error. Never a header or a body.
type Call struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
	Err      error
}

// Option configures New.
type Option func(*Client)

// WithTimeout replaces the time-out per call (tests); values <= 0 keep Timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.http.Timeout = d
		}
	}
}

// WithLimiter replaces the limiter (default 5 calls per second, bursts of 5).
func WithLimiter(l *ratelimit.Limiter) Option {
	return func(c *Client) {
		if l != nil {
			c.limiter = l
		}
	}
}

// WithLogger sets the logger of the debug line per call (default: none). Pass the redacting
// logger of package logging.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		if l != nil {
			c.log = l
		}
	}
}

// WithObserver sets a function that sees every finished call (package wishes writes them to its
// call log). It runs on the caller's goroutine after the answer was read.
func WithObserver(fn func(context.Context, Call)) Option {
	return func(c *Client) { c.observe = fn }
}

// WithUserAgent sets the User-Agent ("Slouchcat/<version>"); "" keeps "Slouchcat".
func WithUserAgent(agent string) Option {
	return func(c *Client) {
		if agent != "" {
			c.agent = agent
		}
	}
}

// New returns a client of the Seerr instance at baseURL (for example "http://192.168.1.20:5055" or
// "https://seerr.example.org/seerr") that authenticates with apiKey. Errors (ErrInvalid) never
// quote the address, which could carry credentials.
func New(baseURL, apiKey string, opts ...Option) (*Client, error) {
	base, err := cleanBase(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("%w: api key missing", ErrInvalid)
	}
	dialer := &net.Dialer{Timeout: Timeout, KeepAlive: 30 * time.Second} // no netguard.RefuseLocal: LAN allowed
	c := &Client{
		base: base, key: Secret(apiKey), limiter: ratelimit.New("seerr", Rate, Burst),
		log: slog.New(slog.DiscardHandler), agent: userAgent,
		http: &http.Client{
			Timeout: Timeout,
			// Never follow a redirect: the answer is returned and classified as ErrRedirect.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				Proxy:                 nil, // no proxy from the environment: the key goes only to the entered address
				DialContext:           dialer.DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          8,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   Timeout,
				ResponseHeaderTimeout: Timeout,
			},
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// NormalizeAddress checks an address of Seerr as New does and returns it without a trailing "/"
// and "/api/v1": http or https, a host, no user info, no query or fragment. Its error (ErrInvalid)
// never quotes the address, which could carry credentials.
func NormalizeAddress(raw string) (string, error) { return cleanBase(raw) }

// cleanBase checks the address the administrator entered: http or https, a host, no user info,
// no query or fragment. A trailing "/" and a trailing "/api/v1" are dropped.
func cleanBase(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil { // the parse error quotes the address
		return "", fmt.Errorf("%w: address does not parse", ErrInvalid)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return "", fmt.Errorf("%w: address scheme must be http or https", ErrInvalid)
	case u.Host == "" || u.Hostname() == "":
		return "", fmt.Errorf("%w: address has no host", ErrInvalid)
	case u.User != nil:
		return "", fmt.Errorf("%w: address must not carry a user or password", ErrInvalid)
	case u.RawQuery != "" || u.Fragment != "" || u.ForceQuery:
		return "", fmt.Errorf("%w: address must not carry a query or fragment", ErrInvalid)
	}
	p := strings.TrimRight(u.EscapedPath(), "/")
	p = strings.TrimRight(strings.TrimSuffix(p, apiPrefix), "/")
	return u.Scheme + "://" + u.Host + p, nil
}

// String and GoString name the instance by scheme and host only: the client holds the key.
func (c *Client) String() string   { return "seerr.Client(" + c.Site() + ")" }
func (c *Client) GoString() string { return c.String() }

// Site returns the scheme and host of the instance, for messages.
func (c *Client) Site() string { return netguard.Site(c.base) }

// Status returns GET /status, which Seerr answers without authentication. It still sends the key
// as every call does.
func (c *Client) Status(ctx context.Context) (Status, error) {
	var out Status
	err := c.AsAdmin().call(ctx, http.MethodGet, "/status", nil, nil, &out)
	return out, err
}

// Session is the identity calls run as. Make it with AsAdmin or AsUser; the zero Session refuses
// every call.
type Session struct {
	c    *Client
	user int // 0: administrator (no X-API-User)
}

// AsAdmin returns the session of administration calls: no X-API-User, Seerr acts as user 1.
func (c *Client) AsAdmin() Session { return Session{c: c} }

// AsUser returns the session of calls on behalf of the Seerr user id: every call sends
// X-API-User: id. An id below 1 refuses every call with ErrInvalid.
func (c *Client) AsUser(id int) Session {
	if id < 1 {
		return Session{c: c, user: -1}
	}
	return Session{c: c, user: id}
}

// UserID returns the user id of a session on behalf of a user and true, or 0 and false for the
// administrator session.
func (s Session) UserID() (int, bool) { return s.user, s.user > 0 }

func (s Session) kind() string {
	if s.user > 0 {
		return "user"
	}
	return "admin"
}

// call sends method path?query with the JSON body in (nil: none) and decodes a 2xx answer into
// out (nil: ignore it). secrets are values of the body that must not appear in an error, beside
// the key. It returns the status of a 2xx answer.
func (s Session) call(ctx context.Context, method, path string, query url.Values, in, out any, secrets ...string) error {
	_, err := s.send(ctx, method, path, query, in, out, secrets...)
	return err
}

func (s Session) send(ctx context.Context, method, path string, query url.Values, in, out any, secrets ...string) (int, error) {
	op := method + " " + path
	if s.c == nil || s.user < 0 {
		return 0, &Error{Op: op, classes: []error{ErrInvalid}, cause: errors.New("no session or invalid user id")}
	}
	c := s.c
	secrets = append(secrets, c.key.Reveal())
	start := time.Now()
	status, err := s.exchange(ctx, op, method, path, query, in, out, secrets)
	attrs := []slog.Attr{
		slog.String("method", method), slog.String("path", path), slog.Int("status", status),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()), slog.String("as", s.kind()),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	c.log.LogAttrs(ctx, slog.LevelDebug, "seerr call", attrs...)
	if c.observe != nil {
		c.observe(ctx, Call{Method: method, Path: path, Status: status, Duration: time.Since(start), Err: err})
	}
	return status, err
}

func (s Session) exchange(ctx context.Context, op, method, path string, query url.Values, in, out any, secrets []string) (int, error) {
	c := s.c
	u := c.base + apiPrefix + path
	if len(query) > 0 {
		// Spaces as %20, not "+": Seerr's search reads the query text as it arrives.
		u += "?" + strings.ReplaceAll(query.Encode(), "+", "%20")
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, &Error{Op: op, classes: []error{ErrInvalid}, cause: errors.New("body does not encode")}
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return 0, &Error{Op: op, classes: []error{ErrInvalid}, cause: errors.New("request does not build")}
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", c.agent)
	req.Header.Set("X-Api-Key", c.key.Reveal())
	if s.user > 0 {
		req.Header.Set("X-API-User", strconv.Itoa(s.user))
	}
	resp, err := c.limiter.Do(ctx, c.http, req)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return 0, &Error{Op: op, cause: ctx.Err()}
		}
		return 0, &Error{Op: op, classes: []error{ErrUnreachable}, cause: transportCause(err)}
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return resp.StatusCode, &Error{Op: op, Status: resp.StatusCode, classes: []error{ErrUnreachable}, cause: transportCause(err)}
	}
	if len(data) > MaxResponseBytes {
		return resp.StatusCode, &Error{Op: op, Status: resp.StatusCode, classes: []error{ErrServer}, cause: errors.New("answer too large")}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, &Error{
			Op: op, Status: resp.StatusCode, Message: cleanMessage(seerrMessage(data), secrets),
			classes: classify(resp.StatusCode, seerrMessage(data), s.user > 0),
		}
	}
	if out != nil && resp.StatusCode != http.StatusNoContent && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, &Error{Op: op, Status: resp.StatusCode, classes: []error{ErrServer}, cause: errors.New("answer is not the expected JSON")}
		}
	}
	return resp.StatusCode, nil
}

// transportCause keeps the reason of a transport error without its URL; errors.Is still finds
// context.DeadlineExceeded in a time-out.
func transportCause(err error) error {
	err = netguard.Clean(err)
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err
	}
	return err
}

// seerrMessage reads Seerr's error body {"message": "…"} (index.ts Z. 282-285); the authentication
// middleware may answer {"error": "…"}, a plain text body is taken as is.
func seerrMessage(data []byte) string {
	var body struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil {
		if body.Message != "" {
			return body.Message
		}
		return body.Error
	}
	if t := strings.TrimSpace(string(data)); !strings.HasPrefix(t, "<") {
		return t
	}
	return ""
}

func pathID(id int) string { return strconv.Itoa(id) }
