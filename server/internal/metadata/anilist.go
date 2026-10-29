package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// AniList (ADR 0015, F-5): a filler, never an identifier. It answers GraphQL on one fixed host
// without a key and names its own rate limit (90 requests per minute in the documentation,
// lower at times); the client stays far below it with one request per second and a burst of
// five through ratelimit.Limiter, which also honours 429 and Retry-After like TMDB and TVDB.
// The host is a constant, not a setting, so no configuration can point it into the home
// network (concept 9.3); SLOUCHCAT_ANILIST=off switches the provider off server-wide.
const (
	DefaultAniListBase = "https://graphql.anilist.co"
	// DefaultAniListImageBase is the host of AniList's cover and banner images; image.fetch
	// downloads originals from there only.
	DefaultAniListImageBase = "https://s4.anilist.co"
	anilistRate             = 1
	anilistBurst            = 5
	maxAniListBytes         = 1 << 20
)

// anilistQuery asks for the fields the fill job maps onto a title.
const anilistQuery = `query ($id: Int) { Media(id: $id, type: ANIME) { id title { romaji english native } ` +
	`description(asHtml: false) genres coverImage { extraLarge } bannerImage isAdult } }`

// aniListProvider talks to the AniList GraphQL API.
type aniListProvider struct {
	base      string
	imageBase string
	http      *http.Client
	limiter   *ratelimit.Limiter
	enabled   bool
	log       *slog.Logger
}

func newAniListProvider(base, imageBase string, hc *http.Client, enabled bool, log *slog.Logger) *aniListProvider {
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	return &aniListProvider{
		base: strings.TrimRight(base, "/"), imageBase: strings.TrimRight(imageBase, "/"), http: hc,
		limiter: ratelimit.New("anilist", anilistRate, anilistBurst), enabled: enabled, log: log,
	}
}

func (p *aniListProvider) Name() string { return model.ProviderAniList }

// Configured reports whether the provider is switched on; AniList needs no key.
func (p *aniListProvider) Configured(context.Context) bool { return p.enabled }

// AniListRecord is one anime as AniList describes it.
type AniListRecord struct {
	ID          int64
	Romaji      string
	English     string
	Native      string
	Description string // plain text, the HTML of the answer stripped
	Genres      []string
	CoverURL    string
	BannerURL   string
	IsAdult     bool
}

type anilistResponse struct {
	Data struct {
		Media *struct {
			ID    int64 `json:"id"`
			Title struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
				Native  string `json:"native"`
			} `json:"title"`
			Description string   `json:"description"`
			Genres      []string `json:"genres"`
			CoverImage  struct {
				ExtraLarge string `json:"extraLarge"`
			} `json:"coverImage"`
			BannerImage string `json:"bannerImage"`
			IsAdult     bool   `json:"isAdult"`
		} `json:"Media"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
		Status  int    `json:"status"`
	} `json:"errors"`
}

// Media fetches one anime by its AniList id. ErrNotFound when AniList does not know the id,
// errNotConfigured when the provider is off.
func (p *aniListProvider) Media(ctx context.Context, id int64) (AniListRecord, error) {
	if !p.enabled {
		return AniListRecord{}, errNotConfigured
	}
	if id <= 0 {
		return AniListRecord{}, fmt.Errorf("anilist id %d: %w", id, ErrNotFound)
	}
	body, err := json.Marshal(map[string]any{"query": anilistQuery, "variables": map[string]any{"id": id}})
	if err != nil {
		return AniListRecord{}, fmt.Errorf("anilist request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base, bytes.NewReader(body))
	if err != nil {
		return AniListRecord{}, fmt.Errorf("anilist request: %w", err)
	}
	// Do retries after a 429 and needs the body again for that.
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	logRequest(p.log, model.ProviderAniList, req)
	resp, err := p.limiter.Do(ctx, p.http, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return AniListRecord{}, ctxErr
		}
		return AniListRecord{}, fmt.Errorf("%w: anilist: %w", model.ErrTMDBUnreachable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAniListBytes+1))
	if err != nil {
		return AniListRecord{}, fmt.Errorf("%w: anilist read: %w", model.ErrTMDBUnreachable, err)
	}
	if len(raw) > maxAniListBytes {
		return AniListRecord{}, fmt.Errorf("anilist: answer larger than %d bytes", maxAniListBytes)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return AniListRecord{}, fmt.Errorf("%w: anilist %d", ErrNotFound, id)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return AniListRecord{}, fmt.Errorf("%w: anilist: status %d", model.ErrTMDBUnreachable, resp.StatusCode)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return AniListRecord{}, fmt.Errorf("anilist: status %d", resp.StatusCode)
	}
	var out anilistResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return AniListRecord{}, fmt.Errorf("%w: anilist decode: %w", model.ErrTMDBUnreachable, err)
	}
	if out.Data.Media == nil {
		for _, e := range out.Errors {
			if e.Status == http.StatusNotFound {
				return AniListRecord{}, fmt.Errorf("%w: anilist %d", ErrNotFound, id)
			}
		}
		return AniListRecord{}, fmt.Errorf("%w: anilist: no media in the answer", model.ErrTMDBUnreachable)
	}
	m := out.Data.Media
	rec := AniListRecord{
		ID: m.ID, Romaji: m.Title.Romaji, English: m.Title.English, Native: m.Title.Native,
		Description: stripHTML(m.Description), Genres: m.Genres, IsAdult: m.IsAdult,
	}
	if p.imageURL(m.CoverImage.ExtraLarge) {
		rec.CoverURL = m.CoverImage.ExtraLarge
	}
	if p.imageURL(m.BannerImage) {
		rec.BannerURL = m.BannerImage
	}
	return rec, nil
}

// imageURL reports whether u lies on AniList's image host: only those are stored as image
// paths and fetched by image.fetch (imageSource).
func (p *aniListProvider) imageURL(u string) bool {
	return u != "" && strings.HasPrefix(u, p.imageBase+"/")
}

// fields maps the record onto the fields the fill job decides (fillFields): English texts
// (title.english, else romaji; the description), the native title as original title, the
// English genres, cover and banner as poster and backdrop, isAdult as FSK 18 (K-3: raised, never
// lowered). AniList has no German texts (concept 4.4).
func (r AniListRecord) fields() providerFields {
	pf := providerFields{provider: model.ProviderAniList, values: fieldValues{}, genres: map[string][]string{}}
	pf.set("title", "en", firstNonEmpty(r.English, r.Romaji))
	pf.set("overview", "en", r.Description)
	pf.set("original_title", "", r.Native)
	pf.set("poster", "", r.CoverURL)
	pf.set("backdrop", "", r.BannerURL)
	if len(r.Genres) > 0 {
		pf.genres["en"] = r.Genres
	}
	if r.IsAdult {
		adult := 18
		pf.rating, pf.ratingSource = &adult, model.ProviderAniList
	}
	return pf
}

var (
	htmlBreak = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTag   = regexp.MustCompile(`<[^>]*>`)
)

// stripHTML turns AniList's description (HTML with <br> and <i>) into plain text.
func stripHTML(s string) string {
	s = htmlBreak.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	s = strings.Join(lines, "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}
