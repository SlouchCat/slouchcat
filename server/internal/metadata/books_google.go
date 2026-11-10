package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Google Books (concept 19 D6.1, Marco's decision of 10 November 2026): only with a key
// (SLOUCHCAT_GOOGLEBOOKS_API_KEY or the setting googlebooks_api_key); without one the source is
// not configured and never asked (without a key Google answers 429). The key travels in the query
// (key=…), so no address of this source is logged or put into an error (logRequest logs host and
// path, fetch cleans the URL of a client error). volumes?q=intitle:…+inauthor:… or isbn:…; the
// detail /volumes/<id>. It names title, subtitle, authors, publisher, date, overview, ISBN,
// language, categories and a cover; no length.

var googleIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{6,32}$`)

type googleBooksProvider struct {
	bookSource
	base string
	key  func(ctx context.Context) (string, error) // nil = no key
}

func (p *googleBooksProvider) Name() string      { return ProviderGoogleBooks }
func (p *googleBooksProvider) HasDuration() bool { return false }

// Configured reports whether a key is set.
func (p *googleBooksProvider) Configured(ctx context.Context) bool {
	k, err := p.apiKey(ctx)
	return err == nil && k != ""
}

func (p *googleBooksProvider) apiKey(ctx context.Context) (string, error) {
	if p.key == nil {
		return "", nil
	}
	k, err := p.key(ctx)
	if err != nil {
		return "", fmt.Errorf("read google books key: %w", err)
	}
	return strings.TrimSpace(k), nil
}

type googleVolume struct {
	ID         string `json:"id"`
	VolumeInfo struct {
		Title               string   `json:"title"`
		Subtitle            string   `json:"subtitle"`
		Authors             []string `json:"authors"`
		Publisher           string   `json:"publisher"`
		PublishedDate       string   `json:"publishedDate"`
		Description         string   `json:"description"`
		Language            string   `json:"language"`
		Categories          []string `json:"categories"`
		IndustryIdentifiers []struct {
			Type       string `json:"type"`
			Identifier string `json:"identifier"`
		} `json:"industryIdentifiers"`
		ImageLinks map[string]string `json:"imageLinks"`
	} `json:"volumeInfo"`
}

// SearchBooks searches by ISBN, else by title and author; errNotConfigured without a key.
func (p *googleBooksProvider) SearchBooks(ctx context.Context, q BookQuery) ([]BookCandidate, error) {
	var term string
	switch isbn := normISBN(q.ISBN); {
	case isbn != "":
		term = "isbn:" + isbn
	case strings.TrimSpace(q.Title) != "":
		term = "intitle:" + strings.TrimSpace(q.Title)
		if a := strings.TrimSpace(q.Author); a != "" {
			term += " inauthor:" + a
		}
	default:
		return nil, nil
	}
	key, err := p.apiKey(ctx)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, errNotConfigured
	}
	return p.cached(bookCacheKey(p.name, "", BookQuery{Title: q.Title, Author: q.Author, ISBN: q.ISBN}), func() ([]BookCandidate, error) {
		body, status, err := p.fetch(ctx, p.http, p.limiter, queryURL(p.base, "/volumes",
			url.Values{"q": {term}, "maxResults": {strconv.Itoa(bookResults)}, "key": {key}}), bookJSON)
		if err != nil || status == http.StatusNotFound {
			return nil, err
		}
		var out struct {
			Items []googleVolume `json:"items"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, fmt.Errorf("%w: googlebooks decode: %w", model.ErrTMDBUnreachable, err)
		}
		cs := make([]BookCandidate, 0, len(out.Items))
		for _, v := range out.Items {
			if c := v.candidate(); c.ID != "" && c.Title != "" {
				cs = append(cs, c)
			}
		}
		return cs, nil
	})
}

// Book fetches one volume; errNotConfigured without a key.
func (p *googleBooksProvider) Book(ctx context.Context, id, _ string) (BookRecord, error) {
	id = strings.TrimSpace(id)
	if !googleIDRe.MatchString(id) {
		return BookRecord{}, fmt.Errorf("googlebooks %q: %w", id, ErrNotFound)
	}
	key, err := p.apiKey(ctx)
	if err != nil {
		return BookRecord{}, err
	}
	if key == "" {
		return BookRecord{}, errNotConfigured
	}
	body, status, err := p.fetch(ctx, p.http, p.limiter, queryURL(p.base, "/volumes/"+id, url.Values{"key": {key}}), bookJSON)
	if err != nil {
		return BookRecord{}, err
	}
	if status == http.StatusNotFound {
		return BookRecord{}, fmt.Errorf("googlebooks %s: %w", id, ErrNotFound)
	}
	var v googleVolume
	if err := json.Unmarshal(body, &v); err != nil {
		return BookRecord{}, fmt.Errorf("%w: googlebooks decode: %w", model.ErrTMDBUnreachable, err)
	}
	if v.ID == "" || v.VolumeInfo.Title == "" {
		return BookRecord{}, fmt.Errorf("googlebooks %s: %w", id, ErrNotFound)
	}
	rec := BookRecord{BookCandidate: v.candidate(), Overview: bookText(v.VolumeInfo.Description)}
	for _, c := range v.VolumeInfo.Categories {
		if c = strings.TrimSpace(c); c != "" {
			rec.Genres = append(rec.Genres, c)
		}
	}
	if rec.CoverURL != "" {
		rec.Covers = []ImageCandidate{{Kind: "poster", Source: rec.CoverURL, Default: true}}
	}
	return rec, nil
}

// googleImageSizes are the image links of a volume, largest first.
var googleImageSizes = []string{"extraLarge", "large", "medium", "small", "thumbnail", "smallThumbnail"}

func (v googleVolume) candidate() BookCandidate {
	vi := v.VolumeInfo
	c := BookCandidate{
		Provider: ProviderGoogleBooks, ID: v.ID, Title: strings.TrimSpace(vi.Title), Subtitle: strings.TrimSpace(vi.Subtitle),
		Authors: vi.Authors, Publisher: strings.TrimSpace(vi.Publisher), Year: bookYear(vi.PublishedDate),
		Language: normalizeOriginalLanguage(vi.Language),
	}
	var isbns []string
	for _, id := range vi.IndustryIdentifiers {
		if strings.HasPrefix(id.Type, "ISBN") {
			isbns = append(isbns, id.Identifier)
		}
	}
	c.ISBN = pickISBN(isbns)
	for _, size := range googleImageSizes {
		if u := vi.ImageLinks[size]; u != "" {
			// Google names its images with http and the page curl; the same image comes over https.
			u = strings.Replace(u, "http://", "https://", 1)
			c.CoverURL = strings.Replace(u, "&edge=curl", "", 1)
			break
		}
	}
	return c
}
