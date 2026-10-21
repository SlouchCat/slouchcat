// Package trakt reads the public Trakt lists (plan M7, AN-3, E-7, I-7): trending and popular
// movies and shows. It is a pure HTTP client without database access; package metadata stores the
// lists and package visibility intersects them with the library at read time.
//
// Trakt is reached with the server's client id only (header trakt-api-key); there is no Trakt
// account per profile (I-7). The client id is a secret: it is sent in the header only and never
// written to an error or the log.
package trakt

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// DefaultBaseURL is the Trakt API.
const DefaultBaseURL = "https://api.trakt.tv"

// Lists, media and limits.
const (
	ListTrending = "trending"
	ListPopular  = "popular"
	MediaMovie   = "movie"
	MediaShow    = "show"

	// MaxLimit is the number of entries a list request asks for (AN-3: the top 100).
	MaxLimit = 100

	rate         = 1 // requests per second
	burst        = 2
	httpTimeout  = 15 * time.Second
	maxJSONBytes = 4 << 20
	userAgent    = "Slouchcat"
)

var (
	// ErrUnavailable: Trakt could not be reached, answered 5xx, or still 429 after the backoff of
	// the limiter, or sent a body that is no list.
	ErrUnavailable = errors.New("trakt unavailable")
	// ErrUnauthorized: Trakt refused the client id (401 or 403).
	ErrUnauthorized = errors.New("trakt refused the client id")
)

// Entry is one title of a list. Position is the rank in the Trakt list (1-based); entries without
// a TMDB id are skipped, so positions can have gaps.
type Entry struct {
	Media    string // movie, show
	Position int
	TMDBID   int64
	Title    string
	Year     int
}

// Client talks to the Trakt API v2. It is safe for concurrent use.
type Client struct {
	base     string
	clientID string
	http     *http.Client
	limiter  *ratelimit.Limiter
	agent    string
}

// Option configures New.
type Option func(*Client)

// WithBaseURL replaces the API base URL (mocks in development and tests); "" keeps the default.
func WithBaseURL(base string) Option {
	return func(c *Client) {
		if base = strings.TrimRight(strings.TrimSpace(base), "/"); base != "" {
			c.base = base
		}
	}
}

// WithHTTPClient replaces the HTTP client (default: 15 s timeout). The client is copied: its
// redirect policy is replaced by one that refuses a change of host or scheme (sameOrigin).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.http = hc
		}
	}
}

// WithLimiter replaces the request limiter (default: 1 request per second, bursts of 2).
func WithLimiter(l *ratelimit.Limiter) Option {
	return func(c *Client) {
		if l != nil {
			c.limiter = l
		}
	}
}

// WithUserAgent sets the User-Agent ("Slouchcat/<version>"); "" keeps "Slouchcat".
func WithUserAgent(agent string) Option {
	return func(c *Client) { c.agent = cmp.Or(agent, c.agent) }
}

// New returns a client that authenticates with clientID.
func New(clientID string, opts ...Option) *Client {
	c := &Client{
		base: DefaultBaseURL, clientID: clientID, http: &http.Client{Timeout: httpTimeout},
		limiter: ratelimit.New("trakt", rate, burst), agent: userAgent,
	}
	for _, o := range opts {
		o(c)
	}
	hc := *c.http
	hc.CheckRedirect = sameOrigin
	c.http = &hc
	return c
}

// errCrossOriginRedirect refuses a redirect to another host or scheme.
var errCrossOriginRedirect = errors.New("redirect to another origin refused")

// sameOrigin follows a redirect only to the scheme and host of the first request: net/http drops
// only Authorization, Cookie and WWW-Authenticate on a redirect to another host, so the client id
// in trakt-api-key would go along.
func sameOrigin(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if first := via[0].URL; req.URL.Scheme != first.Scheme || req.URL.Host != first.Host {
		return errCrossOriginRedirect
	}
	return nil
}

// ids are the ids of a title; only tmdb is used.
type ids struct {
	TMDB *int64 `json:"tmdb"`
}

type title struct {
	Title string `json:"title"`
	Year  *int   `json:"year"`
	IDs   ids    `json:"ids"`
}

// trendingEntry is the shape of the trending lists: {watchers, movie|show}.
type trendingEntry struct {
	Watchers int    `json:"watchers"`
	Movie    *title `json:"movie"`
	Show     *title `json:"show"`
}

// List returns up to limit entries (at most MaxLimit) of a list ("trending", "popular") of a
// medium ("movie", "show") in Trakt's order. Errors: ErrUnavailable, ErrUnauthorized, a context
// error, or a plain error for an unexpected status.
func (c *Client) List(ctx context.Context, list, media string, limit int) ([]Entry, error) {
	if list != ListTrending && list != ListPopular {
		return nil, fmt.Errorf("trakt: unknown list %q", list)
	}
	if media != MediaMovie && media != MediaShow {
		return nil, fmt.Errorf("trakt: unknown media %q", media)
	}
	limit = max(1, min(limit, MaxLimit))
	path := "/" + media + "s/" + list
	u := c.base + path + "?" + url.Values{"limit": {strconv.Itoa(limit)}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("trakt request %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", c.clientID)
	req.Header.Set("User-Agent", c.agent)
	resp, err := c.limiter.Do(ctx, c.http, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: %s: %w", ErrUnavailable, path, cleanErr(err))
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxJSONBytes)) // drain, so the connection is reused
		_ = resp.Body.Close()
	}()
	if err := statusError(resp, path); err != nil {
		return nil, err
	}
	body := io.LimitReader(resp.Body, maxJSONBytes)
	var titles []title
	if list == ListTrending {
		var raw []trendingEntry
		if err := json.NewDecoder(body).Decode(&raw); err != nil {
			return nil, fmt.Errorf("%w: decode %s: %w", ErrUnavailable, path, err)
		}
		for _, r := range raw {
			t := r.Movie
			if media == MediaShow {
				t = r.Show
			}
			if t == nil {
				t = &title{}
			}
			titles = append(titles, *t)
		}
	} else if err := json.NewDecoder(body).Decode(&titles); err != nil {
		return nil, fmt.Errorf("%w: decode %s: %w", ErrUnavailable, path, err)
	}
	out := make([]Entry, 0, len(titles))
	for i, t := range titles {
		if i >= limit {
			break
		}
		if t.IDs.TMDB == nil || *t.IDs.TMDB <= 0 {
			continue
		}
		e := Entry{Media: media, Position: i + 1, TMDBID: *t.IDs.TMDB, Title: strings.TrimSpace(t.Title)}
		if t.Year != nil {
			e.Year = *t.Year
		}
		out = append(out, e)
	}
	return out, nil
}

// statusError maps a non-2xx answer to an error; the body is drained by the caller's Close.
func statusError(resp *http.Response, path string) error {
	switch s := resp.StatusCode; {
	case s >= 200 && s < 300:
		return nil
	case s == http.StatusUnauthorized || s == http.StatusForbidden:
		return fmt.Errorf("%w: %s: status %d", ErrUnauthorized, path, s)
	case s == http.StatusTooManyRequests || s >= 500:
		return fmt.Errorf("%w: %s: status %d", ErrUnavailable, path, s)
	default:
		return fmt.Errorf("trakt %s: status %d", path, s)
	}
}

// cleanErr drops the URL from a transport error (*url.Error): it names the host and path only
// through the caller's message, never the query.
func cleanErr(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err
	}
	return err
}
