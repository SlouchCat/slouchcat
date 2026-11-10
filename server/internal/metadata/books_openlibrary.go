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

// Open Library (concept 19 D6.1): search.json by title and author or by ISBN; the detail asks the
// same search for the work key (q=key:/works/OL…W), so it maps the same fields. It names title,
// subtitle, authors, ISBN, publisher, the first year, the language and a cover
// (covers.openlibrary.org/b/id/<cover_i>-L.jpg); no length, no overview.

const openLibraryFields = "key,title,subtitle,author_name,isbn,publisher,first_publish_year,language,cover_i"

var openLibraryIDRe = regexp.MustCompile(`^OL[0-9]{1,12}W$`)

type openLibraryProvider struct {
	bookSource
	base   string
	covers string
}

func (p *openLibraryProvider) Name() string                    { return ProviderOpenLibrary }
func (p *openLibraryProvider) Configured(context.Context) bool { return true }
func (p *openLibraryProvider) HasDuration() bool               { return false }

type openLibraryDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	Subtitle         string   `json:"subtitle"`
	AuthorName       []string `json:"author_name"`
	ISBN             []string `json:"isbn"`
	Publisher        []string `json:"publisher"`
	FirstPublishYear int      `json:"first_publish_year"`
	Language         []string `json:"language"`
	CoverI           int64    `json:"cover_i"`
}

// SearchBooks searches by ISBN, else by title and author.
func (p *openLibraryProvider) SearchBooks(ctx context.Context, q BookQuery) ([]BookCandidate, error) {
	v := url.Values{}
	switch isbn := normISBN(q.ISBN); {
	case isbn != "":
		v.Set("isbn", isbn)
	case strings.TrimSpace(q.Title) != "":
		v.Set("title", q.Title)
		if a := strings.TrimSpace(q.Author); a != "" {
			v.Set("author", a)
		}
	default:
		return nil, nil
	}
	return p.cached(bookCacheKey(p.name, "", BookQuery{Title: q.Title, Author: q.Author, ISBN: q.ISBN}), func() ([]BookCandidate, error) {
		docs, err := p.search(ctx, v)
		if err != nil {
			return nil, err
		}
		out := make([]BookCandidate, 0, len(docs))
		for _, d := range docs {
			if c := p.candidate(d); c.ID != "" && c.Title != "" {
				out = append(out, c)
			}
		}
		return out, nil
	})
}

// Book fetches a work by its key ("OL123W" or "/works/OL123W").
func (p *openLibraryProvider) Book(ctx context.Context, id, _ string) (BookRecord, error) {
	key := strings.TrimPrefix(strings.TrimSpace(id), "/works/")
	if !openLibraryIDRe.MatchString(key) {
		return BookRecord{}, fmt.Errorf("openlibrary %q: %w", id, ErrNotFound)
	}
	docs, err := p.search(ctx, url.Values{"q": {"key:/works/" + key}})
	if err != nil {
		return BookRecord{}, err
	}
	for _, d := range docs {
		if c := p.candidate(d); c.ID == key {
			rec := BookRecord{BookCandidate: c}
			if c.CoverURL != "" {
				rec.Covers = []ImageCandidate{{Kind: "poster", Source: c.CoverURL, Default: true}}
			}
			return rec, nil
		}
	}
	return BookRecord{}, fmt.Errorf("openlibrary %s: %w", key, ErrNotFound)
}

func (p *openLibraryProvider) search(ctx context.Context, v url.Values) ([]openLibraryDoc, error) {
	v.Set("limit", strconv.Itoa(bookResults))
	v.Set("fields", openLibraryFields)
	body, status, err := p.fetch(ctx, p.http, p.limiter, queryURL(p.base, "/search.json", v), bookJSON)
	if err != nil || status == http.StatusNotFound {
		return nil, err
	}
	var out struct {
		Docs []openLibraryDoc `json:"docs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: openlibrary decode: %w", model.ErrTMDBUnreachable, err)
	}
	return out.Docs, nil
}

func (p *openLibraryProvider) candidate(d openLibraryDoc) BookCandidate {
	c := BookCandidate{
		Provider: ProviderOpenLibrary, ID: strings.TrimPrefix(d.Key, "/works/"),
		Title: strings.TrimSpace(d.Title), Subtitle: strings.TrimSpace(d.Subtitle),
		Authors: d.AuthorName, ISBN: pickISBN(d.ISBN),
	}
	if len(d.Publisher) > 0 {
		c.Publisher = strings.TrimSpace(d.Publisher[0])
	}
	if d.FirstPublishYear > 0 {
		y := d.FirstPublishYear
		c.Year = &y
	}
	if len(d.Language) > 0 {
		c.Language = normalizeOriginalLanguage(d.Language[0])
	}
	if d.CoverI > 0 {
		c.CoverURL = p.covers + "/b/id/" + strconv.FormatInt(d.CoverI, 10) + "-L.jpg"
	}
	return c
}
