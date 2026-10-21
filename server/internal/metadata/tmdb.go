package metadata

import (
	"context"
	"encoding/json"
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
	Credits       Credits      `json:"credits"`
	// BelongsToCollection is the TMDB collection of the film (E-6), nil when it has none.
	BelongsToCollection *MovieCollection `json:"belongs_to_collection"`
}

// MovieCollection is the belongs_to_collection part of a TMDB movie in one language.
type MovieCollection struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Overview   string `json:"overview"`
	PosterPath string `json:"poster_path"`
}

// CastMember is one entry of the cast of a TMDB credits list.
type CastMember struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
}

// CrewMember is one entry of the crew of a TMDB credits list.
type CrewMember struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

// Credits is the append_to_response=credits part of a movie or series.
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// Movie returns the detail of movie id in lang (de-DE, en-US) including its release dates and
// credits.
func (c *Client) Movie(ctx context.Context, id int64, lang string) (Movie, error) {
	var m Movie
	q := url.Values{"language": {lang}, "append_to_response": {"release_dates,credits"}}
	err := c.getJSON(ctx, "/movie/"+strconv.FormatInt(id, 10), q, &m)
	return m, err
}

// TV is the TMDB series detail in one language with content ratings, external IDs and credits.
type TV struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	OriginalName   string  `json:"original_name"`
	Overview       string  `json:"overview"`
	Tagline        string  `json:"tagline"`
	FirstAirDate   string  `json:"first_air_date"`
	EpisodeRunTime []int   `json:"episode_run_time"`
	Genres         []Genre `json:"genres"`
	PosterPath     string  `json:"poster_path"`
	BackdropPath   string  `json:"backdrop_path"`
	Seasons        []struct {
		SeasonNumber int    `json:"season_number"`
		EpisodeCount int    `json:"episode_count"`
		PosterPath   string `json:"poster_path"`
	} `json:"seasons"`
	CreatedBy []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		ProfilePath string `json:"profile_path"`
	} `json:"created_by"`
	ContentRatings struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Rating  string `json:"rating"`
		} `json:"results"`
	} `json:"content_ratings"`
	ExternalIDs struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int64  `json:"tvdb_id"`
	} `json:"external_ids"`
	Credits Credits `json:"credits"`
}

// TV returns the detail of series id in lang.
func (c *Client) TV(ctx context.Context, id int64, lang string) (TV, error) {
	var tv TV
	q := url.Values{"language": {lang}, "append_to_response": {"content_ratings,external_ids,credits"}}
	err := c.getJSON(ctx, "/tv/"+strconv.FormatInt(id, 10), q, &tv)
	return tv, err
}

// TVEpisode is one episode of a TMDB season, or the detail of an episode.
type TVEpisode struct {
	SeasonNumber  int    `json:"season_number"`
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	AirDate       string `json:"air_date"`
	Runtime       int    `json:"runtime"`
	StillPath     string `json:"still_path"`
}

// TVSeason is the detail of one season of a series in one language.
type TVSeason struct {
	SeasonNumber int         `json:"season_number"`
	Name         string      `json:"name"`
	Overview     string      `json:"overview"`
	AirDate      string      `json:"air_date"`
	PosterPath   string      `json:"poster_path"`
	Episodes     []TVEpisode `json:"episodes"`
}

// TVSeason returns season n of series id in lang.
func (c *Client) TVSeason(ctx context.Context, id int64, n int, lang string) (TVSeason, error) {
	var s TVSeason
	path := "/tv/" + strconv.FormatInt(id, 10) + "/season/" + strconv.Itoa(n)
	err := c.getJSON(ctx, path, url.Values{"language": {lang}}, &s)
	return s, err
}

// TVEpisode returns episode e of season s of series id in lang.
func (c *Client) TVEpisode(ctx context.Context, id int64, s, e int, lang string) (TVEpisode, error) {
	var ep TVEpisode
	path := "/tv/" + strconv.FormatInt(id, 10) + "/season/" + strconv.Itoa(s) + "/episode/" + strconv.Itoa(e)
	err := c.getJSON(ctx, path, url.Values{"language": {lang}}, &ep)
	return ep, err
}

// SearchResult is one movie or series of a TMDB search or find answer.
type SearchResult struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`          // movie
	OriginalTitle string `json:"original_title"` // movie
	ReleaseDate   string `json:"release_date"`   // movie
	Name          string `json:"name"`           // series
	OriginalName  string `json:"original_name"`  // series
	FirstAirDate  string `json:"first_air_date"` // series
	Overview      string `json:"overview"`
	PosterPath    string `json:"poster_path"`
}

// Find looks up an external ID; source is imdb_id or tvdb_id. It returns the movie and the series
// results.
func (c *Client) Find(ctx context.Context, externalID, source string) (movies, series []SearchResult, err error) {
	var res struct {
		MovieResults []SearchResult `json:"movie_results"`
		TVResults    []SearchResult `json:"tv_results"`
	}
	q := url.Values{"external_source": {source}}
	if err := c.getJSON(ctx, "/find/"+url.PathEscape(externalID), q, &res); err != nil {
		return nil, nil, err
	}
	return res.MovieResults, res.TVResults, nil
}

// Search runs /search/movie or /search/tv (kind "movie" or "tv") in English. year filters by
// release year (movie) or first air date year (tv).
func (c *Client) Search(ctx context.Context, kind, query string, year *int) ([]SearchResult, error) {
	var res struct {
		Results []SearchResult `json:"results"`
	}
	q := url.Values{"query": {query}, "language": {langEN}}
	if year != nil {
		if kind == "tv" {
			q.Set("first_air_date_year", strconv.Itoa(*year))
		} else {
			q.Set("year", strconv.Itoa(*year))
		}
	}
	if err := c.getJSON(ctx, "/search/"+kind, q, &res); err != nil {
		return nil, err
	}
	return res.Results, nil
}

// Video is one entry of /movie/{id}/videos or /tv/{id}/videos.
type Video struct {
	Lang        string `json:"iso_639_1"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	Site        string `json:"site"`
	Type        string `json:"type"`
	Official    bool   `json:"official"`
	PublishedAt string `json:"published_at"`
}

// Videos returns the videos of a movie or series (kind "movie" or "tv") in the de, en and
// language-less video languages; lang is the request language (de-DE, en-US).
func (c *Client) Videos(ctx context.Context, kind string, id int64, lang string) ([]Video, error) {
	var res struct {
		Results []Video `json:"results"`
	}
	q := url.Values{"language": {lang}, "include_video_language": {"de,en,null"}}
	if err := c.getJSON(ctx, "/"+kind+"/"+strconv.FormatInt(id, 10)+"/videos", q, &res); err != nil {
		return nil, err
	}
	return res.Results, nil
}

// DownloadURL fetches an image (a TMDB original or a TVDB artwork) without credentials. The caller
// closes body. mediaType comes from the Content-Type of the answer.
func (c *Client) DownloadURL(ctx context.Context, rawURL string) (body io.ReadCloser, mediaType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("image request: %w", err)
	}
	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, "", err
	}
	if err := statusError(resp, "image "+req.URL.Path); err != nil {
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
