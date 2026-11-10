package metadata

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// Audiobook sources (concept 19 D6.1, D6.5, ADR 0031): fillers of audiobook folders next to the
// local files. Audible (catalog search on api.audible.<region> plus the details of Audnexus),
// the Deutsche Nationalbibliothek (SRU with MARC21), Apple Books (iTunes Search), Open Library
// and Google Books (only with a key). Every source asks its own host through its own
// ratelimit.Limiter (one request per second; Apple one per three seconds), reads at most 5 MB,
// sends the User-Agent "Slouchcat/<version> (+https://github.com/SlouchCat/slouchcat)" and stops
// asking for six hours after three blocked answers in a row (bookBreaker, book_breaker.go).
// Search results stay seven days in memory (bookCache, book_cache.go). The fixed hosts go
// through the service's HTTP client like TMDB; the Audnexus address is entered by the
// administrator and goes through package netguard, which allows the local network only for that
// address's own host (a self-hosted Audnexus at home, like a podcast feed).

// Names of the audiobook sources (MetadataProvider of the contract, items.book_match_provider,
// the keys metadata.books.paused.<name>).
const (
	ProviderAudible     = "audible"
	ProviderDNB         = "dnb"
	ProviderAppleBooks  = "applebooks"
	ProviderOpenLibrary = "openlibrary"
	ProviderGoogleBooks = "googlebooks"
)

// BookProviderNames lists the audiobook sources in the order of the contract.
var BookProviderNames = []string{ProviderAudible, ProviderDNB, ProviderAppleBooks, ProviderOpenLibrary, ProviderGoogleBooks}

// Errors of the audiobook sources besides ErrNotFound and errNotConfigured.
var (
	// errSourcePaused: the source answered blocked three times in a row and pauses (no request).
	errSourcePaused = errors.New("audiobook source paused after blocked answers")
	// errSourceBlocked: the source answered 403, 429 or HTML instead of JSON or XML (counts for
	// the breaker).
	errSourceBlocked = errors.New("audiobook source blocked the request")
	// errBookRegion: an Audible region outside AudibleRegions.
	errBookRegion = errors.New("unknown audible region")
)

// AudibleRegions are the Audible marketplaces (the domain after api.audible.); DefaultAudibleRegion
// is the region of a new installation (Switzerland buys at audible.de).
var AudibleRegions = []string{"de", "com", "co.uk", "fr", "it", "es", "ca", "com.au", "in", "co.jp"}

const DefaultAudibleRegion = "de"

// BookQuery searches books: by ASIN (Audible) or ISBN (DNB, Open Library, Google Books) when set,
// else by title and author. Region is the Audible region ("" = the setting); Lang is the ISO
// 639-1 language of the book ("en" also searches audible.com and audible.co.uk).
type BookQuery struct {
	Title, Author, ASIN, ISBN, Region, Lang string
}

// BookCandidate is one search result of an audiobook source. ID is the source's own id (ASIN,
// DNB IDN, Apple collection id, Open Library work key "OL…W", Google volume id); Region is the
// Audible region that knows the ASIN ("" elsewhere). Language is ISO 639-1; DurationMs 0 is
// unknown; CoverURL is the source's largest cover ("" when none).
type BookCandidate struct {
	Provider, ID, Region string
	Title, Subtitle      string
	Authors, Narrators   []string
	Publisher            string
	SeriesName           string
	SeriesIndex          *float64
	Year                 *int
	Language             string
	DurationMs           int64
	ASIN, ISBN, CoverURL string
}

// BookRecord is the detail of one book: the candidate's fields, the plain-text overview, the
// genres and the covers (kind "poster"; width and height 0 when the source names none).
type BookRecord struct {
	BookCandidate
	Overview string
	Genres   []string
	Covers   []ImageCandidate
}

// BookProvider is one audiobook source. SearchBooks returns no candidates (and no error) for a
// query the source cannot answer (an ASIN at DNB, a title-less query at Apple). Book returns
// ErrNotFound for an id the source does not know; errSourcePaused, errSourceBlocked and
// errNotConfigured as their names say.
type BookProvider interface {
	Name() string
	Configured(ctx context.Context) bool
	// HasDuration reports whether the source delivers lengths (only Audible); a source without
	// them never matches on its own.
	HasDuration() bool
	SearchBooks(ctx context.Context, q BookQuery) ([]BookCandidate, error)
	Book(ctx context.Context, id, region string) (BookRecord, error)
}

// Default addresses of the sources.
const (
	DefaultAudnexusURL       = "https://api.audnex.us"
	DefaultDNBURL            = "https://services.dnb.de/sru/dnb"
	DefaultAppleBooksURL     = "https://itunes.apple.com"
	DefaultOpenLibraryURL    = "https://openlibrary.org"
	DefaultOpenLibraryCovers = "https://covers.openlibrary.org"
	DefaultGoogleBooksURL    = "https://www.googleapis.com/books/v1"
)

// BookSourceURLs are the base addresses of the audiobook sources, without trailing slash.
// Audible maps a region onto the base of its catalog API ("https://api.audible.de"; the client
// appends /1.0/catalog/products); Audnexus is the default of the administrator's setting.
type BookSourceURLs struct {
	Audible           func(region string) string
	Audnexus          string
	DNB               string
	Apple             string
	OpenLibrary       string
	OpenLibraryCovers string
	Google            string
}

// DefaultBookSourceURLs are the public hosts of the sources.
func DefaultBookSourceURLs() BookSourceURLs {
	return BookSourceURLs{
		Audible:  func(region string) string { return "https://api.audible." + region },
		Audnexus: DefaultAudnexusURL, DNB: DefaultDNBURL, Apple: DefaultAppleBooksURL,
		OpenLibrary: DefaultOpenLibraryURL, OpenLibraryCovers: DefaultOpenLibraryCovers, Google: DefaultGoogleBooksURL,
	}
}

// BookSourceURLsFromBase points every source at one mock server (tests and the dev-only
// SLOUCHCAT_TEST_BOOKS_BASE_URL): <base>/audible/<region>/1.0/catalog/products…,
// <base>/audnexus/books/…, <base>/dnb?…, <base>/apple/search…, <base>/openlibrary/search.json…
// (covers at <base>/openlibrary/b/id/…), <base>/google/volumes….
func BookSourceURLsFromBase(base string) BookSourceURLs {
	base = strings.TrimRight(base, "/")
	return BookSourceURLs{
		Audible:  func(region string) string { return base + "/audible/" + region },
		Audnexus: base + "/audnexus", DNB: base + "/dnb", Apple: base + "/apple",
		OpenLibrary: base + "/openlibrary", OpenLibraryCovers: base + "/openlibrary", Google: base + "/google",
	}
}

// WithBookSourceURLs replaces the addresses of the audiobook sources (tests; the dev-only test
// base through BookSourceURLsFromBase). A field left empty keeps its default.
func WithBookSourceURLs(u BookSourceURLs) Option {
	return func(o *options) { o.bookURLs = &u }
}

// WithTestBookSources points every audiobook source at the mock at base (BookSourceURLsFromBase)
// and lifts their limits, so a mock is not asked at Apple's one request per three seconds: the
// dev-only SLOUCHCAT_TEST_BOOKS_BASE_URL of the e2e suite. An empty base changes nothing; the
// public hosts always keep their limits.
func WithTestBookSources(base string) Option {
	return func(o *options) {
		if base = strings.TrimSpace(base); base != "" {
			u := BookSourceURLsFromBase(base)
			o.bookURLs, o.bookUnlimited = &u, true
		}
	}
}

// WithGoogleBooksKey sets the function that returns the Google Books key per call
// (SLOUCHCAT_GOOGLEBOOKS_API_KEY or the encrypted setting googlebooks_api_key); "" without error
// means no key: the source is then not configured and never asked.
func WithGoogleBooksKey(key func(ctx context.Context) (string, error)) Option {
	return func(o *options) { o.googleBooksKey = key }
}

// Limits of the sources (concept 19 D6.5: Audible, DNB and Open Library one per second,
// Audnexus 60 per minute, Apple 20 per minute, Google Books one per second).
const (
	bookRate         = 1.0
	appleBooksRate   = 1.0 / 3
	maxBookBytes     = 5 << 20
	bookResults      = 10
	bookUserAgentURL = " (+https://github.com/SlouchCat/slouchcat)"
	maxBookRedirects = 5
	// testBookRate is the rate (and burst) of every source at the mock of WithTestBookSources.
	testBookRate = 1000
)

// bookFormat is what a source answers: JSON or XML. HTML instead of it counts as blocked.
type bookFormat int

const (
	bookJSON bookFormat = iota
	bookXML
)

// bookSource is the request side every source shares: its name, the client of its fixed host,
// its limiter, its breaker and the User-Agent.
type bookSource struct {
	name    string
	http    *http.Client
	limiter *ratelimit.Limiter
	breaker *bookBreaker
	cache   *bookCache
	agent   string
	log     *slog.Logger
}

// fetch asks rawURL with hc after the breaker and the limiter allowed it. It returns the body of
// a 2xx answer (after checking it is no HTML page) and of a 404 (status 404, err nil, for
// Audnexus' REGION_UNAVAILABLE); 403, 429 and HTML count for the breaker (errSourceBlocked) as
// answers of the host of lim, any other status is an error. Errors carry scheme and host of the address only (netguard.Clean),
// because the Google Books key travels in the query.
func (b *bookSource) fetch(ctx context.Context, hc *http.Client, lim *ratelimit.Limiter, rawURL string, format bookFormat) ([]byte, int, error) {
	if err := b.breaker.allow(ctx); err != nil {
		return nil, 0, err
	}
	if err := lim.Wait(ctx); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("%s request: %w", b.name, netguard.Clean(err))
	}
	req.Header.Set("User-Agent", b.agent)
	if format == bookXML {
		req.Header.Set("Accept", "application/xml")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	logRequest(b.log, b.name, req)
	resp, err := hc.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, 0, ctxErr
		}
		return nil, 0, fmt.Errorf("%w: %s: %w", model.ErrTMDBUnreachable, b.name, netguard.Clean(err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBookBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s read: %w", model.ErrTMDBUnreachable, b.name, err)
	}
	if len(body) > maxBookBytes {
		return nil, 0, fmt.Errorf("%s: answer larger than %d bytes", b.name, maxBookBytes)
	}
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		b.breaker.blocked(ctx, lim)
		return nil, resp.StatusCode, fmt.Errorf("%w: %s: status %d", errSourceBlocked, b.name, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		return body, resp.StatusCode, nil
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, resp.StatusCode, fmt.Errorf("%w: %s: status %d", model.ErrTMDBUnreachable, b.name, resp.StatusCode)
	}
	if htmlAnswer(resp.Header.Get("Content-Type"), body, format) {
		b.breaker.blocked(ctx, lim)
		return nil, resp.StatusCode, fmt.Errorf("%w: %s: HTML instead of the answer", errSourceBlocked, b.name)
	}
	b.breaker.ok(lim)
	return body, resp.StatusCode, nil
}

// htmlAnswer reports whether a 2xx answer is an HTML page (a block or captcha page) instead of
// JSON or XML: declared as text/html, or a JSON answer starting with "<", or an XML answer
// starting with an HTML doctype or element.
func htmlAnswer(contentType string, body []byte, format bookFormat) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "text/html") {
		return true
	}
	head := bytes.TrimSpace(body)
	if format == bookJSON {
		return len(head) > 0 && head[0] == '<'
	}
	lower := strings.ToLower(string(head[:min(len(head), 64)]))
	return strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html")
}

// cached returns the cached candidates of key or runs search and caches its result (also an
// empty one); an error is not cached.
func (b *bookSource) cached(key string, search func() ([]BookCandidate, error)) ([]BookCandidate, error) {
	if c, ok := b.cache.get(key); ok {
		return c, nil
	}
	c, err := search()
	if err != nil {
		return nil, err
	}
	b.cache.put(key, c)
	return c, nil
}

// bookSources are the five sources of the service; byName reaches them by provider name.
type bookSources struct {
	audible     *audibleProvider
	dnb         *dnbProvider
	apple       *appleBooksProvider
	openLibrary *openLibraryProvider
	google      *googleBooksProvider
	byName      map[string]BookProvider
}

// newBookSources builds the sources of s from the options: the clients, one limiter and one
// breaker per source (Audible and Audnexus share the breaker of "audible", which counts each host
// on its own), one search cache.
func newBookSources(s *Service, o options) bookSources {
	urls := DefaultBookSourceURLs()
	if u := o.bookURLs; u != nil {
		if u.Audible != nil {
			urls.Audible = u.Audible
		}
		urls.Audnexus = cmp.Or(u.Audnexus, urls.Audnexus)
		urls.DNB = cmp.Or(u.DNB, urls.DNB)
		urls.Apple = cmp.Or(u.Apple, urls.Apple)
		urls.OpenLibrary = cmp.Or(u.OpenLibrary, urls.OpenLibrary)
		urls.OpenLibraryCovers = cmp.Or(u.OpenLibraryCovers, urls.OpenLibraryCovers)
		urls.Google = cmp.Or(u.Google, urls.Google)
	}
	hc := o.http
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	agent := cmp.Or(o.feedAgent, "Slouchcat") + bookUserAgentURL
	cache := newBookCache(s.now)
	limiter := func(name string, rate float64) *ratelimit.Limiter {
		if o.bookUnlimited {
			return ratelimit.New(name, testBookRate, testBookRate)
		}
		return ratelimit.New(name, rate, 1)
	}
	source := func(name string, rate float64) bookSource {
		return bookSource{name: name, http: hc, limiter: limiter(name, rate), breaker: newBookBreaker(s, name),
			cache: cache, agent: agent, log: s.log}
	}
	audnexus := strings.TrimRight(urls.Audnexus, "/")
	bs := bookSources{
		audible: &audibleProvider{bookSource: source(ProviderAudible, bookRate), base: urls.Audible,
			audnexusLimiter: limiter("audnexus", bookRate), audnexusClients: newAudnexusClients(),
			region:   func(context.Context) string { return DefaultAudibleRegion },
			audnexus: func(context.Context) string { return audnexus }},
		dnb:   &dnbProvider{bookSource: source(ProviderDNB, bookRate), base: strings.TrimRight(urls.DNB, "/")},
		apple: &appleBooksProvider{bookSource: source(ProviderAppleBooks, appleBooksRate), base: strings.TrimRight(urls.Apple, "/")},
		openLibrary: &openLibraryProvider{bookSource: source(ProviderOpenLibrary, bookRate), base: strings.TrimRight(urls.OpenLibrary, "/"),
			covers: strings.TrimRight(urls.OpenLibraryCovers, "/")},
		google: &googleBooksProvider{bookSource: source(ProviderGoogleBooks, bookRate), base: strings.TrimRight(urls.Google, "/"),
			key: o.googleBooksKey},
	}
	bs.byName = map[string]BookProvider{
		ProviderAudible: bs.audible, ProviderDNB: bs.dnb, ProviderAppleBooks: bs.apple,
		ProviderOpenLibrary: bs.openLibrary, ProviderGoogleBooks: bs.google,
	}
	return bs
}

// bookProvider returns the audiobook source called name.
func (s *Service) bookProvider(name string) (BookProvider, bool) {
	p, ok := s.books.byName[name]
	return p, ok
}

// bookSourcePausedUntil reports until when the source called name pauses after blocked answers
// (the page «Hörbücher: Quellen»); ok is false when it does not pause.
func (s *Service) bookSourcePausedUntil(ctx context.Context, name string) (until time.Time, ok bool) {
	var b *bookBreaker
	switch name {
	case ProviderAudible:
		b = s.books.audible.breaker
	case ProviderDNB:
		b = s.books.dnb.breaker
	case ProviderAppleBooks:
		b = s.books.apple.breaker
	case ProviderOpenLibrary:
		b = s.books.openLibrary.breaker
	case ProviderGoogleBooks:
		b = s.books.google.breaker
	default:
		return time.Time{}, false
	}
	return b.pausedUntil(ctx)
}

// bookCacheKey is the key of a search in the cache: source, region and the normalised query.
func bookCacheKey(source, region string, q BookQuery) string {
	return strings.Join([]string{source, region, normQuery(q.Title), normQuery(q.Author), normQuery(q.ASIN),
		normISBN(q.ISBN), normQuery(q.Lang)}, "\x1f")
}

// normQuery lowers s and collapses its white space.
func normQuery(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

var nonISBN = regexp.MustCompile(`[^0-9X]`)

// normISBN keeps the digits and a check digit X of an ISBN ("978-3-86717-123-4" → "9783867171234");
// "" when the rest is no ISBN-10 or ISBN-13.
func normISBN(s string) string {
	s = nonISBN.ReplaceAllString(strings.ToUpper(s), "")
	if len(s) != 10 && len(s) != 13 {
		return ""
	}
	return s
}

// pickISBN returns the first ISBN-13 of the list, else the first ISBN-10 ("" when none).
func pickISBN(list []string) string {
	ten := ""
	for _, v := range list {
		switch n := normISBN(v); len(n) {
		case 13:
			return n
		case 10:
			if ten == "" {
				ten = n
			}
		}
	}
	return ten
}

var (
	yearRe   = regexp.MustCompile(`\b(1[5-9]\d\d|2\d\d\d)\b`)
	numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
)

// bookYear reads the first year of a date or a publication statement ("2019-03-28", "[2019]",
// "℗ 2019"); nil when there is none.
func bookYear(s string) *int {
	m := yearRe.FindString(s)
	if m == "" {
		return nil
	}
	y, _ := strconv.Atoi(m)
	return &y
}

// seriesIndex reads the volume number of a series position ("3", "Band 3", "1.5", "2,5"); nil when
// there is none.
func seriesIndex(s string) *float64 {
	m := numberRe.FindString(s)
	if m == "" {
		return nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
	if err != nil {
		return nil
	}
	return &f
}

var htmlParagraph = regexp.MustCompile(`(?i)</p\s*>`)

// bookText turns the HTML overview of a source into plain text: a paragraph ends with an empty
// line, the rest as stripHTML.
func bookText(s string) string {
	return stripHTML(htmlParagraph.ReplaceAllString(s, "\n\n"))
}

// queryURL joins base, path and the query parameters.
func queryURL(base, path string, q url.Values) string {
	u := base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}
