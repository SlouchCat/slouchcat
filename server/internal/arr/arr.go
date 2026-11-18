// Package arr is the HTTP client of Radarr and Sonarr (API v3; concept 25, architecture 2.6). It
// has no database and no business rules. It knows exactly two things (rule 11, appendix H N-81):
// the profile tag (list tags, create a tag, find a title, attach a tag to a title) and reading the
// download queue and the file state of a title for the download progress.
//
// Rules of the client:
//
//   - Only the calls of calls.go: GET and POST /api/v3/tag, GET /api/v3/movie?tmdbId= and
//     PUT /api/v3/movie/editor (Radarr), GET /api/v3/series?tvdbId= and PUT /api/v3/series/editor
//     (Sonarr), GET /api/v3/queue. The editor is always sent with applyTags "add": the client never
//     removes or replaces a tag. No title is added, deleted or searched, no queue entry is
//     changed, no command is sent and /api/v3/notification is never called (N-22, N-26, N-80).
//   - Only http and https, no user info in the address, paths always below /api/v3. Addresses in
//     the household's network are allowed on purpose: the client dials without
//     netguard.RefuseLocal, because Radarr and Sonarr run on the LAN (architecture 15.3). Which host
//     may receive the key is decided by the caller (host confirmation, architecture 20.2).
//   - No redirect is followed; a 3xx answer is ErrRedirect. The key never travels to another
//     address.
//   - 10 s per call, at most 2 calls per second per client, at most 4 MiB per answer, every call
//     bound to its context.
//   - Nothing secret in an error or a log line: the log has the service, method, path without
//     query, status and duration; never a header or a body. Errors name the method and path and
//     the service's message, from which the key is removed.
//   - No retries; the job queue retries.
package arr

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
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// Limits (architecture 2.6).
const (
	// Timeout bounds every call, the answer included.
	Timeout = 10 * time.Second
	// Rate and Burst limit the calls per client, that is per service.
	Rate  = 2
	Burst = 2
	// MaxResponseBytes is the largest answer the client reads; a larger one is ErrServer.
	MaxResponseBytes = 4 << 20

	apiPrefix = "/api/v3"
	userAgent = "Slouchcat"
)

// Kind is the service a client talks to.
type Kind string

// The two services.
const (
	Radarr Kind = "radarr"
	Sonarr Kind = "sonarr"
)

// Client talks to one Radarr or one Sonarr instance. It is safe for concurrent use.
type Client struct {
	kind    Kind
	base    string // scheme, host and optional path prefix, without trailing "/"
	key     Secret
	http    *http.Client
	limiter *ratelimit.Limiter
	log     *slog.Logger
	agent   string
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

// WithLimiter replaces the limiter (default 2 calls per second, bursts of 2).
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

// WithUserAgent sets the User-Agent ("Slouchcat/<version>"); "" keeps "Slouchcat".
func WithUserAgent(agent string) Option {
	return func(c *Client) {
		if agent != "" {
			c.agent = agent
		}
	}
}

// New returns a client of the Radarr or Sonarr instance (kind) at baseURL (for example
// "http://192.168.1.60:7878" or "https://media.example.org/sonarr") that authenticates with
// apiKey. Errors (ErrInvalid) never quote the address, which could carry credentials.
func New(kind Kind, baseURL string, apiKey Secret, opts ...Option) (*Client, error) {
	if kind != Radarr && kind != Sonarr {
		return nil, fmt.Errorf("%w: unknown service kind", ErrInvalid)
	}
	base, err := cleanBase(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey.Reveal()) == "" {
		return nil, fmt.Errorf("%w: api key missing", ErrInvalid)
	}
	dialer := &net.Dialer{Timeout: Timeout, KeepAlive: 30 * time.Second} // no netguard.RefuseLocal: LAN allowed
	c := &Client{
		kind: kind, base: base, key: apiKey, limiter: ratelimit.New(string(kind), Rate, Burst),
		log: slog.New(slog.DiscardHandler), agent: userAgent,
		http: &http.Client{
			Timeout: Timeout,
			// Never follow a redirect: the answer is returned and classified as ErrRedirect.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				Proxy:                 nil, // no proxy from the environment: the key goes only to the given address
				DialContext:           dialer.DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          4,
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

// cleanBase checks the address: http or https, a host, no user info, no query or fragment. A
// trailing "/" and a trailing "/api/v3" are dropped.
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

// Kind returns the service of the client.
func (c *Client) Kind() Kind { return c.kind }

// String and GoString name the service and instance by scheme and host only: the client holds the
// key.
func (c *Client) String() string   { return "arr.Client(" + string(c.kind) + " " + c.Site() + ")" }
func (c *Client) GoString() string { return c.String() }

// Site returns the scheme and host of the instance, for messages and the host confirmation.
func (c *Client) Site() string { return netguard.Site(c.base) }

// call sends method path?query with the JSON body in (nil: none) and decodes a 2xx answer into out
// (nil: ignore it).
func (c *Client) call(ctx context.Context, method, path string, query url.Values, in, out any) error {
	op := method + " " + apiPrefix + path
	start := time.Now()
	status, err := c.exchange(ctx, op, method, path, query, in, out)
	attrs := []slog.Attr{
		slog.String("service", string(c.kind)), slog.String("method", method), slog.String("path", apiPrefix+path),
		slog.Int("status", status), slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	c.log.LogAttrs(ctx, slog.LevelDebug, "arr call", attrs...)
	return err
}

func (c *Client) exchange(ctx context.Context, op, method, path string, query url.Values, in, out any) (int, error) {
	u := c.base + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, c.fail(op, 0, "", []error{ErrInvalid}, errors.New("body does not encode"))
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return 0, c.fail(op, 0, "", []error{ErrInvalid}, errors.New("request does not build"))
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", c.agent)
	req.Header.Set("X-Api-Key", c.key.Reveal())
	resp, err := c.limiter.Do(ctx, c.http, req)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return 0, c.fail(op, 0, "", nil, ctx.Err())
		}
		return 0, c.fail(op, 0, "", []error{ErrUnreachable}, transportCause(err))
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return resp.StatusCode, c.fail(op, resp.StatusCode, "", []error{ErrUnreachable}, transportCause(err))
	}
	if len(data) > MaxResponseBytes {
		return resp.StatusCode, c.fail(op, resp.StatusCode, "", []error{ErrServer}, errors.New("answer too large"))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := cleanMessage(arrMessage(data), c.key.Reveal())
		return resp.StatusCode, c.fail(op, resp.StatusCode, msg, classify(resp.StatusCode), nil)
	}
	if out != nil && resp.StatusCode != http.StatusNoContent && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, c.fail(op, resp.StatusCode, "", []error{ErrServer}, errors.New("answer is not the expected JSON"))
		}
	}
	return resp.StatusCode, nil
}

func (c *Client) fail(op string, status int, msg string, classes []error, cause error) *Error {
	return &Error{Service: c.kind, Op: op, Status: status, Message: msg, classes: classes, cause: cause}
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
