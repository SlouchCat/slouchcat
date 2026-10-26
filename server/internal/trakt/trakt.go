// Package trakt reads the public Trakt lists (plan M7, AN-3, E-7, I-7; M11 plan 5.6): the seven
// list kinds of the spotlight row, from trending to a user's public list. It is a pure HTTP client
// without database access; package metadata stores the lists and package visibility intersects them
// with the library at read time.
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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// DefaultBaseURL is the Trakt API.
const DefaultBaseURL = "https://api.trakt.tv"

// Kind keys (spotlight_config.list_key of migration 00021; the CHECK there is the source of truth).
const (
	KindTrending         = "trending"
	KindPopular          = "popular"
	KindWatchedWeekly    = "watched_weekly"
	KindCollectedMonthly = "collected_monthly"
	KindAnticipated      = "anticipated"
	KindBoxOffice        = "boxoffice"
	KindURL              = "url"
)

// Media values of spotlight_config.media; MediaMovie and MediaShow also mark an Entry.
const (
	MediaMovie = "movie"
	MediaShow  = "show"
	MediaBoth  = "both"
)

// Limits.
const (
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
	// ErrInvalidList: the kind, media or address of a ListRef do not fit (plan 5.6).
	ErrInvalidList = errors.New("trakt: invalid list")
	// ErrWatchlist: the address names a Trakt watchlist, whose import is a non-goal (plan 5.7).
	ErrWatchlist = errors.New("trakt: watchlist addresses are not accepted")
)

// Kind describes one list kind for the catalogue the administration shows (listKinds of
// getSpotlightConfig, plan 5.6).
type Kind struct {
	Key          string
	Media        []string // allowed values of spotlight_config.media for this kind
	MediaFilters bool     // true: media is a post-filter on fetched entries (the user list); false: media selects the path
	NeedsURL     bool     // true only for KindURL
}

// kindPaths is one row of the table of plan 5.6: the kind and its path per medium. A path kind
// with only a movie path (boxoffice) allows the medium movie only; KindURL has no fixed path.
type kindPaths struct {
	Kind
	movie, show string
}

var allMedia = []string{MediaMovie, MediaShow, MediaBoth}

// kinds is the table of plan 5.6 in its order. Period segments (weekly, monthly) are part of the
// path.
var kinds = []kindPaths{
	{Kind{Key: KindTrending, Media: allMedia}, "/movies/trending", "/shows/trending"},
	{Kind{Key: KindPopular, Media: allMedia}, "/movies/popular", "/shows/popular"},
	{Kind{Key: KindWatchedWeekly, Media: allMedia}, "/movies/watched/weekly", "/shows/watched/weekly"},
	{Kind{Key: KindCollectedMonthly, Media: allMedia}, "/movies/collected/monthly", "/shows/collected/monthly"},
	{Kind{Key: KindAnticipated, Media: allMedia}, "/movies/anticipated", "/shows/anticipated"},
	{Kind{Key: KindBoxOffice, Media: []string{MediaMovie}}, "/movies/boxoffice", ""},
	{Kind{Key: KindURL, Media: allMedia, MediaFilters: true, NeedsURL: true}, "", ""},
}

// Kinds returns the seven kinds in the order of the plan's table 5.6 (trending, popular,
// watched_weekly, collected_monthly, anticipated, boxoffice, url).
func Kinds() []Kind {
	out := make([]Kind, len(kinds))
	for i, k := range kinds {
		out[i] = k.Kind
		out[i].Media = slices.Clone(k.Media)
	}
	return out
}

// KindOf returns the kind of key or false.
func KindOf(key string) (Kind, bool) {
	k, ok := kindRow(key)
	if !ok {
		return Kind{}, false
	}
	k.Media = slices.Clone(k.Media)
	return k.Kind, true
}

func kindRow(key string) (kindPaths, bool) {
	i := slices.IndexFunc(kinds, func(k kindPaths) bool { return k.Key == key })
	if i < 0 {
		return kindPaths{}, false
	}
	return kinds[i], true
}

// ListRef names a configured list: Kind key, Media (movie, show, both) and for KindURL the
// user list address as the administration stored it.
type ListRef struct {
	Kind, Media, URL string
}

// userListLabel stands for the path of a user list in errors: the path names a Trakt user, and an
// error may reach the log (rule 10, plan 8.4).
const userListLabel = "/users/…/lists/…/items"

// traktHosts are the hosts a user list address may name.
var traktHosts = []string{"trakt.tv", "www.trakt.tv", "api.trakt.tv"}

// ParseListURL accepts the public list address forms https://trakt.tv/users/{user}/lists/{slug}
// (optionally with a trailing "/" or "/items", host trakt.tv, www.trakt.tv or api.trakt.tv, http
// or https) and returns user and slug. A path under /users/{user}/watchlist (any suffix) returns
// ErrWatchlist; anything else returns ErrInvalidList (wrapped with a short reason, never echoing
// the whole address). Both parts are returned unescaped and must be path-escaped when used.
func ParseListURL(raw string) (user, slug string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil { // the parse error quotes the address
		return "", "", fmt.Errorf("%w: not an address", ErrInvalidList)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", "", fmt.Errorf("%w: scheme is not http or https", ErrInvalidList)
	}
	if u.User != nil || !slices.Contains(traktHosts, strings.ToLower(u.Host)) {
		return "", "", fmt.Errorf("%w: host is not trakt.tv", ErrInvalidList)
	}
	// Split the escaped path, so an escaped "/" in a name stays inside its segment.
	seg := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	if len(seg) > 0 && seg[len(seg)-1] == "" {
		seg = seg[:len(seg)-1]
	}
	for i, s := range seg {
		if seg[i], err = url.PathUnescape(s); err != nil || seg[i] == "" {
			return "", "", fmt.Errorf("%w: malformed path", ErrInvalidList)
		}
	}
	if len(seg) < 3 || !strings.EqualFold(seg[0], "users") {
		return "", "", fmt.Errorf("%w: not a user list", ErrInvalidList)
	}
	if strings.EqualFold(seg[2], "watchlist") {
		return "", "", ErrWatchlist
	}
	if !strings.EqualFold(seg[2], "lists") || len(seg) < 4 || len(seg) > 5 ||
		(len(seg) == 5 && !strings.EqualFold(seg[4], "items")) {
		return "", "", fmt.Errorf("%w: not a user list", ErrInvalidList)
	}
	return seg[1], seg[3], nil
}

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

// element is one entry of any list response: popular is a flat array of titles; trending, watched,
// collected, anticipated and boxoffice wrap the title in movie or show next to fields that do not
// matter here; a user list adds type, which also names seasons, episodes and persons.
type element struct {
	title
	Type  string `json:"type"`
	Movie *title `json:"movie"`
	Show  *title `json:"show"`
}

// pick returns the title of e and its medium; flat is the medium of a flat title. A user list
// entry (with type) counts as movie or show only by its type: a season or episode also carries the
// show it belongs to.
func (e element) pick(flat string) (title, string, bool) {
	switch {
	case e.Type != "":
		if e.Type == MediaMovie && e.Movie != nil {
			return *e.Movie, MediaMovie, true
		}
		if e.Type == MediaShow && e.Show != nil {
			return *e.Show, MediaShow, true
		}
		return title{}, "", false
	case e.Movie != nil:
		return *e.Movie, MediaMovie, true
	case e.Show != nil:
		return *e.Show, MediaShow, true
	default:
		return e.title, flat, true
	}
}

// Fetch returns up to limit entries (at most MaxLimit) per request of the list in Trakt's order.
// For path kinds with Media "both" it makes two requests (movies, then shows) and returns the
// movies first; for KindURL it makes one request (/users/{user}/lists/{slug}/items) and keeps only
// the entries whose own media matches Media (both keeps movies and shows; seasons, episodes and
// persons are skipped). Errors: ErrUnavailable, ErrUnauthorized, ErrInvalidList (kind, media or
// address do not fit), ErrWatchlist, a context error, or a plain error for an unexpected status.
// Errors of KindURL never name the user or the address.
func (c *Client) Fetch(ctx context.Context, ref ListRef, limit int) ([]Entry, error) {
	k, ok := kindRow(ref.Kind)
	if !ok {
		return nil, fmt.Errorf("%w: unknown kind %q", ErrInvalidList, ref.Kind)
	}
	if !slices.Contains(k.Media, ref.Media) {
		return nil, fmt.Errorf("%w: kind %s does not offer media %q", ErrInvalidList, k.Key, ref.Media)
	}
	limit = max(1, min(limit, MaxLimit))
	if k.NeedsURL {
		user, slug, err := ParseListURL(ref.URL)
		if err != nil {
			return nil, err
		}
		path := "/users/" + url.PathEscape(user) + "/lists/" + url.PathEscape(slug) + "/items"
		want := []string{ref.Media}
		if ref.Media == MediaBoth {
			want = []string{MediaMovie, MediaShow}
		}
		return c.list(ctx, path, userListLabel, "", want, limit)
	}
	var out []Entry
	for _, m := range []struct{ media, path string }{{MediaMovie, k.movie}, {MediaShow, k.show}} {
		if ref.Media != m.media && ref.Media != MediaBoth {
			continue
		}
		e, err := c.list(ctx, m.path, m.path, m.media, []string{m.media}, limit)
		if err != nil {
			return nil, err
		}
		out = append(out, e...)
	}
	return out, nil
}

// list requests path with ?limit=limit and returns the entries whose medium is in want; flat is
// the medium of a flat title. label stands for the path in every error.
func (c *Client) list(ctx context.Context, path, label, flat string, want []string, limit int) ([]Entry, error) {
	u := c.base + path + "?" + url.Values{"limit": {strconv.Itoa(limit)}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("trakt request %s: %w", label, cleanErr(err))
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
		return nil, fmt.Errorf("%w: %s: %w", ErrUnavailable, label, cleanErr(err))
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxJSONBytes)) // drain, so the connection is reused
		_ = resp.Body.Close()
	}()
	if err := statusError(resp, label); err != nil {
		return nil, err
	}
	var raw []element
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONBytes)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: decode %s: %w", ErrUnavailable, label, err)
	}
	out := make([]Entry, 0, min(len(raw), limit))
	for i, r := range raw {
		if i >= limit {
			break
		}
		t, media, ok := r.pick(flat)
		if !ok || !slices.Contains(want, media) || t.IDs.TMDB == nil || *t.IDs.TMDB <= 0 {
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
