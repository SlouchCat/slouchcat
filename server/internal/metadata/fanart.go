package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// Fanart.tv as the last image source (concept 10 6.2, F-4, ADR 0018): logos, posters and
// backgrounds of films (by TMDB id) and series (by TVDB id), each with language, likes and size
// (API v3.2). The keys go as headers (api-key, client-key), never into the address, so no log
// line carries them (O-4, checked against the official client @fanart-tv/api 1.2.0 on
// 31 October 2026: query or header; project keys see new images after 7 days, personal keys
// after 2; no quota, 429 with Retry-After when overused). Without a project key the source is
// "not configured" and never asked.

// DefaultFanartBase is the Fanart.tv API with width and height of every image (v3.2);
// DefaultFanartAssets the host of its images (the only host an image of the source may come from).
const (
	DefaultFanartBase   = "https://webservice.fanart.tv/v3.2"
	DefaultFanartAssets = "https://assets.fanart.tv"
)

const (
	fanartRate  = 2 // requests per second; Fanart.tv names no quota, this stays polite
	fanartBurst = 4
	// fanartNoText is the language Fanart.tv gives an image without text.
	fanartNoText = "00"
)

// fanartKinds maps the Fanart.tv fields to the image kinds, the HD variants first (a field listed
// earlier wins between two images of the same language and likes, sortCandidates is stable).
var fanartKinds = map[string][]struct{ field, kind string }{
	"movie": {
		{"hdmovielogo", "logo"}, {"movielogo", "logo"},
		{"movieposter", "poster"},
		{"moviebackground", "backdrop"}, {"movie4kbackground", "backdrop"},
	},
	"series": {
		{"hdtvlogo", "logo"}, {"clearlogo", "logo"},
		{"tvposter", "poster"},
		{"showbackground", "backdrop"}, {"show4kbackground", "backdrop"},
	},
}

type fanartImage struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Lang   string `json:"lang"`
	Likes  string `json:"likes"`
	Width  string `json:"width"`
	Height string `json:"height"`
	Season string `json:"season"`
}

// fanartProvider asks Fanart.tv with the keys of the KeySource, read per call.
type fanartProvider struct {
	base    string
	assets  string
	http    *http.Client
	limiter *ratelimit.Limiter
	keys    KeySource
	enabled func(ctx context.Context) bool // metadata.fanart_enabled; nil = off
}

func newFanartProvider(base, assets string, hc *http.Client, keys KeySource) *fanartProvider {
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	return &fanartProvider{base: strings.TrimRight(base, "/"), assets: strings.TrimRight(assets, "/"), http: hc, limiter: ratelimit.New("fanart", fanartRate, fanartBurst), keys: keys}
}

// assetURL reports whether an address points to the asset host of Fanart.tv (step 6).
func (p *fanartProvider) assetURL(src string) bool {
	return strings.HasPrefix(src, p.assets+"/")
}

// Configured reports whether a project key is set.
func (p *fanartProvider) Configured(ctx context.Context) bool {
	key, _, err := p.keys.FanartKeys(ctx)
	return err == nil && key != ""
}

// Active reports whether Fanart.tv is switched on and has a key: only then is it asked.
func (p *fanartProvider) Active(ctx context.Context) bool {
	return p.enabled != nil && p.enabled(ctx) && p.Configured(ctx)
}

// Images returns the candidates of a film (TMDB id) or series (TVDB id); a title Fanart.tv does
// not know has none (ErrNotFound is not an error here).
func (p *fanartProvider) Images(ctx context.Context, itemKind string, id int64) ([]ImageCandidate, error) {
	fields, ok := fanartKinds[itemKind]
	if !ok || id <= 0 {
		return nil, nil
	}
	path := "/movies/" + strconv.FormatInt(id, 10)
	if itemKind == "series" {
		path = "/tv/" + strconv.FormatInt(id, 10)
	}
	var raw map[string]json.RawMessage
	if err := p.getJSON(ctx, path, &raw); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var out []ImageCandidate
	for _, f := range fields {
		var list []fanartImage
		if err := json.Unmarshal(raw[f.field], &list); err != nil {
			continue // a field that is missing or not a list
		}
		for _, im := range list {
			if !p.assetURL(im.URL) || im.Season != "" && im.Season != "all" && f.kind == "poster" {
				continue
			}
			lang := strings.ToLower(strings.TrimSpace(im.Lang))
			if lang == fanartNoText {
				lang = ""
			}
			w, _ := strconv.Atoi(im.Width)
			h, _ := strconv.Atoi(im.Height)
			likes, _ := strconv.Atoi(im.Likes)
			out = append(out, ImageCandidate{Kind: f.kind, Lang: lang, Width: w, Height: h,
				Votes: float64(likes), VoteCount: likes, Source: im.URL})
		}
	}
	return out, nil
}

// fanartImages returns the Fanart.tv candidates of a film (TMDB id) or series (TVDB id) when the
// source is switched on and has a key; nil otherwise. A failing Fanart.tv never fails a match or a
// fill: it is the last source, the others decide without it (logged).
func (s *Service) fanartImages(ctx context.Context, itemID, itemKind string, id int64) []ImageCandidate {
	if id <= 0 || !s.fanart.Active(ctx) {
		return nil
	}
	imgs, err := s.fanart.Images(ctx, itemKind, id)
	if err != nil {
		if ctx.Err() == nil {
			s.log.Warn("fanart.tv images skipped", "item_id", itemID, "err", err)
		}
		return nil
	}
	return imgs
}

// fanartPreview is the 200 px preview of an asset (the CDN serves /preview/ next to /fanart/).
func fanartPreview(url string) string {
	return strings.Replace(url, "/fanart/", "/preview/", 1)
}

func (p *fanartProvider) getJSON(ctx context.Context, path string, out any) error {
	key, client, err := p.keys.FanartKeys(ctx)
	if err != nil {
		return fmt.Errorf("read fanart key: %w", err)
	}
	if key == "" {
		return errNotConfigured
	}
	if err := p.limiter.Wait(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base+path, nil)
	if err != nil {
		return fmt.Errorf("fanart request %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", key)
	if client != "" {
		req.Header.Set("client-key", client)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: fanart %s: %w", model.ErrTMDBUnreachable, path, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: fanart %s: status %d", errFanartKeyRefused, path, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%w: fanart %s: status %d", model.ErrTMDBUnreachable, path, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONBytes)).Decode(out); err != nil {
		return fmt.Errorf("%w: fanart decode %s: %w", model.ErrTMDBUnreachable, path, err)
	}
	return nil
}
