package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// TMDB endpoints and limits (plan section 9: at most 20 requests per second).
const (
	DefaultAPIBase   = "https://api.themoviedb.org/3"
	DefaultImageBase = "https://image.tmdb.org/t/p"
	tmdbRate         = 20
	tmdbBurst        = 20
	httpTimeout      = 15 * time.Second
	maxJSONBytes     = 4 << 20
	maxImageBytes    = 20 << 20
)

// Languages fetched for every title (B-3).
const (
	langDE = "de-DE"
	langEN = "en-US"
)

// ErrNotFound is returned when TMDB answers 404 for an ID.
var ErrNotFound = errors.New("tmdb: not found")

// Client talks to the TMDB API v3 with a v4 read access token. The token is sent only in the
// Authorization header of API requests, never in a URL and never to the image host, and it is
// never logged. A Client is safe for concurrent use; copies made with WithKey share the limiter.
type Client struct {
	apiBase   string
	imageBase string
	http      *http.Client
	limiter   *ratelimit.Limiter
	key       string
}

// NewClient returns a client for the given API and image base URLs (without trailing slash).
// hc nil means a client with a 15 s timeout.
func NewClient(apiBase, imageBase string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	return &Client{
		apiBase:   strings.TrimRight(apiBase, "/"),
		imageBase: strings.TrimRight(imageBase, "/"),
		http:      hc,
		limiter:   ratelimit.New("tmdb", tmdbRate, tmdbBurst),
	}
}

// WithKey returns a copy of c that authenticates with key.
func (c *Client) WithKey(key string) *Client {
	cp := *c
	cp.key = key
	return &cp
}

// CheckKey validates key with GET /configuration. It returns nil, model.ErrTMDBKeyInvalid
// (HTTP 401 or an empty key) or model.ErrTMDBUnreachable (network error, timeout, 5xx).
func (c *Client) CheckKey(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return model.ErrTMDBKeyInvalid
	}
	var cfg struct {
		Images struct {
			SecureBaseURL string `json:"secure_base_url"`
		} `json:"images"`
	}
	return c.WithKey(key).getJSON(ctx, "/configuration", nil, &cfg)
}

// Genre is a TMDB genre with its localised name.
type Genre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Release is one release of a movie in a country.
type Release struct {
	Certification string `json:"certification"`
	Type          int    `json:"type"` // 1 premiere, 2 limited, 3 theatrical, 4 digital, 5 physical, 6 TV
	ReleaseDate   string `json:"release_date"`
}

// CountryReleases lists the releases of one country (ISO 3166-1).
type CountryReleases struct {
	Country      string    `json:"iso_3166_1"`
	ReleaseDates []Release `json:"release_dates"`
}

// ReleaseDates is the append_to_response=release_dates part of a movie.
type ReleaseDates struct {
	Results []CountryReleases `json:"results"`
}

// Movie is the TMDB movie detail in one language.
type Movie struct {
	ID            int64        `json:"id"`
	IMDBID        string       `json:"imdb_id"`
	Title         string       `json:"title"`
	OriginalTitle string       `json:"original_title"`
	Overview      string       `json:"overview"`
	Tagline       string       `json:"tagline"`
	ReleaseDate   string       `json:"release_date"` // YYYY-MM-DD or empty
	Runtime       int          `json:"runtime"`      // minutes, 0 when unknown
	Genres        []Genre      `json:"genres"`
	PosterPath    string       `json:"poster_path"`
	BackdropPath  string       `json:"backdrop_path"`
	ReleaseDates  ReleaseDates `json:"release_dates"`
}

// Movie returns the detail of movie id in lang (de-DE, en-US) including its release dates.
func (c *Client) Movie(ctx context.Context, id int64, lang string) (Movie, error) {
	var m Movie
	q := url.Values{"language": {lang}, "append_to_response": {"release_dates"}}
	err := c.getJSON(ctx, "/movie/"+strconv.FormatInt(id, 10), q, &m)
	return m, err
}

// FindByIMDB returns the TMDB ID of the first movie with the IMDb ID, or 0 when there is none.
func (c *Client) FindByIMDB(ctx context.Context, imdbID string) (int64, error) {
	var res struct {
		MovieResults []struct {
			ID int64 `json:"id"`
		} `json:"movie_results"`
	}
	q := url.Values{"external_source": {"imdb_id"}}
	if err := c.getJSON(ctx, "/find/"+url.PathEscape(imdbID), q, &res); err != nil {
		return 0, err
	}
	if len(res.MovieResults) == 0 {
		return 0, nil
	}
	return res.MovieResults[0].ID, nil
}

// SearchMovie searches by title and returns the TMDB ID of the best result, or 0. With a known
// year it takes the first result released in that year ±1, otherwise the first result.
func (c *Client) SearchMovie(ctx context.Context, title string, year *int) (int64, error) {
	var res struct {
		Results []struct {
			ID          int64  `json:"id"`
			ReleaseDate string `json:"release_date"`
		} `json:"results"`
	}
	q := url.Values{"query": {title}, "language": {langEN}}
	if year != nil {
		q.Set("year", strconv.Itoa(*year))
	}
	if err := c.getJSON(ctx, "/search/movie", q, &res); err != nil {
		return 0, err
	}
	for _, r := range res.Results {
		if year == nil {
			return r.ID, nil
		}
		if y, ok := yearOf(r.ReleaseDate); ok && y >= *year-1 && y <= *year+1 {
			return r.ID, nil
		}
	}
	return 0, nil
}

// DownloadImage fetches an image in size (w500, w780, w1280) at the TMDB path (/abc.jpg). The caller
// closes body. mediaType comes from the Content-Type of the answer.
func (c *Client) DownloadImage(ctx context.Context, size, path string) (body io.ReadCloser, mediaType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.imageURL(size, path), nil)
	if err != nil {
		return nil, "", fmt.Errorf("tmdb image request: %w", err)
	}
	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, "", err
	}
	if err := statusError(resp, "image "+size+path); err != nil {
		return nil, "", err
	}
	mediaType, _, _ = mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return limitedBody{io.LimitReader(resp.Body, maxImageBytes+1), resp.Body}, mediaType, nil
}

// imageURL is the public URL of an image; it carries no secret and is stored as source_url.
func (c *Client) imageURL(size, path string) string {
	return c.imageBase + "/" + size + "/" + strings.TrimLeft(path, "/")
}

type limitedBody struct {
	io.Reader
	io.Closer
}

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, out any) error {
	u := c.apiBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("tmdb request %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.key)
	resp, err := c.send(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := statusError(resp, path); err != nil {
		return err
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONBytes)).Decode(out); err != nil {
		return fmt.Errorf("%w: decode %s: %w", model.ErrTMDBUnreachable, path, err)
	}
	return nil
}

// send runs req through the shared limiter. Network errors and timeouts become
// model.ErrTMDBUnreachable; a cancelled ctx stays a context error.
func (c *Client) send(ctx context.Context, req *http.Request) (*http.Response, error) {
	resp, err := c.limiter.Do(ctx, c.http, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: %s: %w", model.ErrTMDBUnreachable, req.URL.Path, err)
	}
	return resp, nil
}

// statusError maps a non-2xx answer to an error and closes the body in that case.
func statusError(resp *http.Response, what string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%w: %s: status 401", model.ErrTMDBKeyInvalid, what)
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, what)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: %s: status %d", model.ErrTMDBUnreachable, what, resp.StatusCode)
	default:
		return fmt.Errorf("tmdb %s: status %d", what, resp.StatusCode)
	}
}

// yearOf parses the year of a YYYY-MM-DD date.
func yearOf(date string) (int, bool) {
	if len(date) < 4 {
		return 0, false
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil || y < 1800 {
		return 0, false
	}
	return y, true
}
