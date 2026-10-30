package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// TVDB v4 endpoints and limits (design 3.1). TVDB is optional: without SLOUCHCAT_TVDB_API_KEY the
// provider is not configured. It delivers series only; movies come from TMDB.
const (
	DefaultTVDBBase        = "https://api4.thetvdb.com/v4"
	DefaultTVDBArtworkBase = "https://artworks.thetvdb.com"
	tvdbRate               = 5
	tvdbBurst              = 5
	tvdbMaxPages           = 20 // 500 episodes per page
	tvdbEpisodeCacheTTL    = 5 * time.Minute
	tvdbEpisodeCacheSize   = 16
)

// TVDB artwork types (v4 /artwork/types) used for series images.
const (
	tvdbArtworkPoster     = 2
	tvdbArtworkBackground = 3
)

// tvdbLangs maps the text languages to TVDB's ISO 639-2 codes.
var tvdbLangs = map[string]string{"de": "deu", "en": "eng"}

// tvdbCountries maps TVDB's ISO 3166-1 alpha-3 rating countries to the AN-8 countries.
var tvdbCountries = map[string]string{"deu": "DE", "aut": "AT", "che": "CH", "gbr": "GB", "usa": "US"}

// tvdbProvider talks to the TVDB API v4. The API key (and the optional subscriber PIN) is sent
// only in the body of POST /login; the returned bearer token is cached per key and renewed after
// a 401. Neither is ever logged or put into a URL.
type tvdbProvider struct {
	base    string
	artBase string
	http    *http.Client
	limiter *ratelimit.Limiter
	keys    KeySource
	log     *slog.Logger // set by New; nil logs nothing

	mu       sync.Mutex
	token    string
	tokenFor [32]byte // hash of the key and PIN the token belongs to
	episodes map[string]tvdbEpisodeCache
}

type tvdbEpisodeCache struct {
	at       time.Time
	episodes []tvdbEpisode
}

func newTVDBProvider(base, artBase string, hc *http.Client, keys KeySource) *tvdbProvider {
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	return &tvdbProvider{
		base: strings.TrimRight(base, "/"), artBase: strings.TrimRight(artBase, "/"), http: hc,
		limiter: ratelimit.New("tvdb", tvdbRate, tvdbBurst), keys: keys,
		episodes: map[string]tvdbEpisodeCache{},
	}
}

func (p *tvdbProvider) Name() string { return ProviderTVDB }

func (p *tvdbProvider) Configured(ctx context.Context) bool {
	key, _, err := p.keys.TVDBKey(ctx)
	return err == nil && key != ""
}

// tvdbEnvelope is the shape of every TVDB answer.
type tvdbEnvelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
	Links  struct {
		Next *string `json:"next"`
	} `json:"links"`
}

// getJSON runs an authenticated GET and decodes the data of the envelope into out. It reports
// whether the answer links a next page.
func (p *tvdbProvider) getJSON(ctx context.Context, path string, q url.Values, out any) (bool, error) {
	key, pin, err := p.keys.TVDBKey(ctx)
	if err != nil {
		return false, fmt.Errorf("read tvdb key: %w", err)
	}
	if key == "" {
		return false, errNotConfigured
	}
	u := p.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	for attempt := 0; ; attempt++ {
		token, err := p.bearer(ctx, key, pin, attempt > 0)
		if err != nil {
			return false, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return false, fmt.Errorf("tvdb request %s: %w", path, err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := p.send(ctx, req)
		if err != nil {
			return false, err
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			// The token expired: log in again once.
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			_ = resp.Body.Close()
			continue
		}
		defer resp.Body.Close()
		if err := tvdbStatusError(resp, path); err != nil {
			return false, err
		}
		env := tvdbEnvelope[json.RawMessage]{}
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONBytes)).Decode(&env); err != nil {
			return false, fmt.Errorf("%w: tvdb decode %s: %w", model.ErrTMDBUnreachable, path, err)
		}
		if len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return false, fmt.Errorf("%w: tvdb decode %s: %w", model.ErrTMDBUnreachable, path, err)
			}
		}
		return env.Links.Next != nil && *env.Links.Next != "", nil
	}
}

// bearer returns the cached token for key and PIN, logging in when there is none, the key changed
// or renew is set.
func (p *tvdbProvider) bearer(ctx context.Context, key, pin string, renew bool) (string, error) {
	sum := sha256.Sum256([]byte(key + "\x00" + pin))
	p.mu.Lock()
	if !renew && p.token != "" && p.tokenFor == sum {
		t := p.token
		p.mu.Unlock()
		return t, nil
	}
	p.mu.Unlock()

	body, err := json.Marshal(struct { //nolint:gosec // G117: the login body is where TVDB expects the key
		APIKey string `json:"apikey"`
		PIN    string `json:"pin,omitempty"`
	}{key, pin})
	if err != nil {
		return "", fmt.Errorf("tvdb login body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/login", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("tvdb login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := p.send(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := tvdbStatusError(resp, "/login"); err != nil {
		return "", err
	}
	var env tvdbEnvelope[struct {
		Token string `json:"token"`
	}]
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONBytes)).Decode(&env); err != nil {
		return "", fmt.Errorf("%w: tvdb login decode: %w", model.ErrTMDBUnreachable, err)
	}
	if env.Data.Token == "" {
		return "", fmt.Errorf("%w: tvdb login without token", model.ErrTMDBKeyInvalid)
	}
	p.mu.Lock()
	p.token, p.tokenFor = env.Data.Token, sum
	p.mu.Unlock()
	return env.Data.Token, nil
}

// send runs req through the TVDB limiter; network errors become model.ErrTMDBUnreachable (the
// contract has one code per failure kind for every provider).
func (p *tvdbProvider) send(ctx context.Context, req *http.Request) (*http.Response, error) {
	logRequest(p.log, ProviderTVDB, req)
	resp, err := p.limiter.Do(ctx, p.http, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: tvdb %s: %w", model.ErrTMDBUnreachable, req.URL.Path, err)
	}
	return resp, nil
}

// tvdbStatusError maps a non-2xx answer to an error and closes the body in that case. 401 is
// model.ErrTMDBKeyInvalid and 5xx/429 model.ErrTMDBUnreachable, each with a "tvdb" message.
func tvdbStatusError(resp *http.Response, what string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%w: tvdb %s: status 401", model.ErrTMDBKeyInvalid, what)
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: tvdb %s", ErrNotFound, what)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: tvdb %s: status %d", model.ErrTMDBUnreachable, what, resp.StatusCode)
	default:
		return fmt.Errorf("tvdb %s: status %d", what, resp.StatusCode)
	}
}

// artURL makes a TVDB image reference absolute (older records carry /banners/… paths).
func (p *tvdbProvider) artURL(ref string) string {
	switch {
	case ref == "":
		return ""
	case strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://"):
		return ref
	default:
		return p.artBase + "/" + strings.TrimLeft(ref, "/")
	}
}

func parseTVDBID(id string) (int64, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("tvdb id %q: %w", id, ErrNotFound)
	}
	return n, nil
}

// SearchMovie returns nothing: movies come from TMDB only (design 3.1).
func (p *tvdbProvider) SearchMovie(context.Context, MovieQuery) ([]Candidate, error) {
	return nil, nil
}

// Movie is not supported by TVDB in M3.
func (p *tvdbProvider) Movie(context.Context, string, []string) (MovieRecord, error) {
	return MovieRecord{}, fmt.Errorf("tvdb movie: %w", ErrNotFound)
}

type tvdbSearchResult struct {
	TVDBID       string            `json:"tvdb_id"`
	Name         string            `json:"name"`
	Year         string            `json:"year"`
	Overview     string            `json:"overview"`
	ImageURL     string            `json:"image_url"`
	Thumbnail    string            `json:"thumbnail"`
	FirstAirTime string            `json:"first_air_time"`
	Translations map[string]string `json:"translations"`
	Overviews    map[string]string `json:"overviews"`
}

// SearchSeries searches by name (GET /search?type=series). A query with a TVDB ID answers that ID
// as the only candidate; the caller loads it with Series. An IMDb ID alone is not searched.
func (p *tvdbProvider) SearchSeries(ctx context.Context, q SeriesQuery) ([]Candidate, error) {
	if q.TVDBID > 0 {
		rec, err := p.Series(ctx, strconv.FormatInt(q.TVDBID, 10), []string{"en"})
		if err != nil {
			return nil, err
		}
		c := Candidate{Provider: ProviderTVDB, ID: rec.ID, Title: rec.Texts["en"].Title, Overview: rec.Texts["en"].Overview,
			OriginalTitle: rec.OriginalTitle, PosterURL: rec.PosterPath}
		if y, ok := yearOf(rec.Date); ok {
			c.Year = &y
		}
		return []Candidate{c}, nil
	}
	if strings.TrimSpace(q.Title) == "" {
		return nil, nil
	}
	var rs []tvdbSearchResult
	if _, err := p.getJSON(ctx, "/search", url.Values{"query": {q.Title}, "type": {"series"}}, &rs); err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(rs))
	for _, r := range rs {
		if r.TVDBID == "" {
			continue
		}
		c := Candidate{Provider: ProviderTVDB, ID: r.TVDBID, Title: firstNonEmpty(r.Translations["eng"], r.Name),
			OriginalTitle: r.Name, Overview: firstNonEmpty(r.Overviews["eng"], r.Overview),
			PosterURL: p.artURL(firstNonEmpty(r.Thumbnail, r.ImageURL))}
		if y, ok := yearOf(firstNonEmpty(r.FirstAirTime, r.Year)); ok {
			c.Year = &y
		}
		out = append(out, c)
	}
	return out, nil
}

type tvdbSeries struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Image            string `json:"image"`
	FirstAired       string `json:"firstAired"`
	Overview         string `json:"overview"`
	AverageRuntime   int    `json:"averageRuntime"`
	OriginalLanguage string `json:"originalLanguage"`
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ContentRatings []struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"contentRatings"`
	RemoteIDs []struct {
		ID         string `json:"id"`
		SourceName string `json:"sourceName"`
	} `json:"remoteIds"`
	Artworks []struct {
		Image string  `json:"image"`
		Type  int     `json:"type"`
		Score float64 `json:"score"`
	} `json:"artworks"`
	Seasons []struct {
		Number int    `json:"number"`
		Image  string `json:"image"`
		Type   struct {
			Type string `json:"type"`
		} `json:"type"`
	} `json:"seasons"`
	Characters []struct {
		Name       string `json:"name"`
		PersonName string `json:"personName"`
		PeopleType string `json:"peopleType"`
		Sort       int    `json:"sort"`
	} `json:"characters"`
	Translations struct {
		Names []struct {
			Language string `json:"language"`
			Name     string `json:"name"`
		} `json:"nameTranslations"`
		Overviews []struct {
			Language string `json:"language"`
			Overview string `json:"overview"`
		} `json:"overviewTranslations"`
	} `json:"translations"`
}

// Series loads /series/{id}/extended with translations; one request covers every language.
func (p *tvdbProvider) Series(ctx context.Context, id string, langs []string) (SeriesRecord, error) {
	n, err := parseTVDBID(id)
	if err != nil {
		return SeriesRecord{}, err
	}
	var s tvdbSeries
	if _, err := p.getJSON(ctx, "/series/"+strconv.FormatInt(n, 10)+"/extended", url.Values{"meta": {"translations"}}, &s); err != nil {
		return SeriesRecord{}, err
	}
	names, overviews := map[string]string{}, map[string]string{}
	for _, t := range s.Translations.Names {
		names[t.Language] = t.Name
	}
	for _, t := range s.Translations.Overviews {
		overviews[t.Language] = t.Overview
	}
	// No VoteAverage: TVDB v4 has no 0-10 user rating; its "score" is a popularity number without
	// an upper bound, not a vote (concept 08 4.2 assumed one). TMDB and AniList fill the field.
	rec := SeriesRecord{TitleRecord: TitleRecord{
		Provider: ProviderTVDB, ID: strconv.FormatInt(s.ID, 10), TVDBID: s.ID, OriginalTitle: s.Name,
		Date: s.FirstAired, Runtime: s.AverageRuntime, Texts: map[string]Text{}, Genres: map[string][]string{},
		Certifications: map[string]string{},
	}}
	var genres []string
	for _, g := range s.Genres {
		if g.Name != "" {
			genres = append(genres, g.Name)
		}
	}
	for _, l := range langs {
		code := tvdbLangs[l]
		t := Text{Title: names[code], Overview: overviews[code]}
		if code == s.OriginalLanguage {
			t.Title, t.Overview = firstNonEmpty(t.Title, s.Name), firstNonEmpty(t.Overview, s.Overview)
		}
		rec.Texts[l] = t
		rec.Genres[l] = genres // TVDB names genres in English only
	}
	for _, r := range s.ContentRatings {
		if c, ok := tvdbCountries[strings.ToLower(r.Country)]; ok && rec.Certifications[c] == "" && r.Name != "" {
			rec.Certifications[c] = r.Name
		}
	}
	for _, r := range s.RemoteIDs {
		if strings.EqualFold(r.SourceName, "IMDB") && strings.HasPrefix(r.ID, "tt") {
			rec.IMDBID = r.ID
		}
	}
	rec.PosterPath = p.artURL(firstNonEmpty(p.bestArtwork(s, tvdbArtworkPoster), s.Image))
	rec.BackdropPath = p.artURL(p.bestArtwork(s, tvdbArtworkBackground))
	for _, season := range s.Seasons {
		if season.Type.Type == "official" {
			rec.Seasons = append(rec.Seasons, SeasonInfo{Number: season.Number, PosterPath: p.artURL(season.Image)})
		}
	}
	chars := s.Characters
	sort.SliceStable(chars, func(i, j int) bool { return chars[i].Sort < chars[j].Sort })
	for _, c := range chars {
		if c.PeopleType != "Actor" || c.PersonName == "" {
			continue
		}
		if len(rec.Credits) == maxCast {
			break
		}
		rec.Credits = append(rec.Credits, Credit{Person: Person{Name: c.PersonName}, Role: RoleCast, Character: c.Name, Order: len(rec.Credits)})
	}
	return rec, nil
}

// bestArtwork returns the image of the highest scored artwork of the type, or "".
func (p *tvdbProvider) bestArtwork(s tvdbSeries, kind int) string {
	best, score := "", -1.0
	for _, a := range s.Artworks {
		if a.Type == kind && a.Image != "" && a.Score > score {
			best, score = a.Image, a.Score
		}
	}
	return best
}

type tvdbEpisode struct {
	SeasonNumber int    `json:"seasonNumber"`
	Number       int    `json:"number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	Aired        string `json:"aired"`
	Runtime      int    `json:"runtime"`
	Image        string `json:"image"`
}

// allEpisodes returns every episode of the series in the default order with texts in the TVDB
// language code, following the pages. Results are cached briefly: a series job asks for every
// season in turn.
func (p *tvdbProvider) allEpisodes(ctx context.Context, id int64, code string) ([]tvdbEpisode, error) {
	cacheKey := strconv.FormatInt(id, 10) + "/" + code
	p.mu.Lock()
	if c, ok := p.episodes[cacheKey]; ok && time.Since(c.at) < tvdbEpisodeCacheTTL {
		p.mu.Unlock()
		return c.episodes, nil
	}
	p.mu.Unlock()
	var out []tvdbEpisode
	for page := 0; page < tvdbMaxPages; page++ {
		var data struct {
			Episodes []tvdbEpisode `json:"episodes"`
		}
		path := "/series/" + strconv.FormatInt(id, 10) + "/episodes/default/" + code
		next, err := p.getJSON(ctx, path, url.Values{"page": {strconv.Itoa(page)}}, &data)
		if err != nil {
			return nil, err
		}
		out = append(out, data.Episodes...)
		if !next || len(data.Episodes) == 0 {
			break
		}
	}
	p.mu.Lock()
	if len(p.episodes) >= tvdbEpisodeCacheSize {
		clear(p.episodes)
	}
	p.episodes[cacheKey] = tvdbEpisodeCache{at: time.Now(), episodes: out}
	p.mu.Unlock()
	return out, nil
}

// Season collects the episodes of season n from the episode list per language. TVDB keeps season
// names and posters elsewhere; the match falls back to "Staffel N"/"Season N".
func (p *tvdbProvider) Season(ctx context.Context, seriesID string, n int, langs []string) (SeasonRecord, error) {
	id, err := parseTVDBID(seriesID)
	if err != nil {
		return SeasonRecord{}, err
	}
	rec := SeasonRecord{Number: n, Texts: map[string]Text{}}
	byNumber := map[int]*EpisodeRecord{}
	var order []int
	for _, l := range langs {
		eps, err := p.allEpisodes(ctx, id, tvdbLangs[l])
		if err != nil {
			return SeasonRecord{}, err
		}
		for _, e := range eps {
			if e.SeasonNumber != n {
				continue
			}
			ep := byNumber[e.Number]
			if ep == nil {
				ep = &EpisodeRecord{Season: n, Number: e.Number, Texts: map[string]Text{}, AirDate: e.Aired,
					Runtime: e.Runtime, StillPath: p.artURL(e.Image)}
				byNumber[e.Number] = ep
				order = append(order, e.Number)
			}
			ep.Texts[l] = Text{Title: e.Name, Overview: e.Overview}
		}
	}
	if len(order) == 0 {
		return SeasonRecord{}, fmt.Errorf("tvdb season %d of %d: %w", n, id, ErrNotFound)
	}
	sort.Ints(order)
	for _, num := range order {
		rec.Episodes = append(rec.Episodes, *byNumber[num])
	}
	return rec, nil
}

// Episode returns one episode from the season.
func (p *tvdbProvider) Episode(ctx context.Context, seriesID string, s, e int, langs []string) (EpisodeRecord, error) {
	season, err := p.Season(ctx, seriesID, s, langs)
	if err != nil {
		return EpisodeRecord{}, err
	}
	for _, ep := range season.Episodes {
		if ep.Number == e {
			return ep, nil
		}
	}
	return EpisodeRecord{}, fmt.Errorf("tvdb episode S%02dE%02d of %s: %w", s, e, seriesID, ErrNotFound)
}
