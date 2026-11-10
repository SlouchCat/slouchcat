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

// Apple Books (concept 19 D6.1): the iTunes Search API in the Swiss store (country=ch,
// media=audiobook), the lookup by collection id. It names title (with a subtitle after ": "),
// the authors (artistName), the overview (HTML stripped), the genre, the date and the publisher
// (from the copyright line); the 100 px artwork is asked as 3000x3000 (the store delivers its
// largest up to that box). No length, no ISBN. Apple allows about 20 requests per minute.

const appleCountry = "ch"

var (
	appleIDRe      = regexp.MustCompile(`^[0-9]{1,15}$`)
	appleArtworkRe = regexp.MustCompile(`/\d+x\d+(bb)?\.(jpg|png|webp)$`)
	appleCopyright = regexp.MustCompile(`^\s*(?:℗|©|\(c\)|\(p\))?\s*(?:\d{4})?\s*`)
	appleArtists   = regexp.MustCompile(`\s*(?:&|,| und | and )\s*`)
)

type appleBooksProvider struct {
	bookSource
	base string
}

func (p *appleBooksProvider) Name() string                    { return ProviderAppleBooks }
func (p *appleBooksProvider) Configured(context.Context) bool { return true }
func (p *appleBooksProvider) HasDuration() bool               { return false }

type appleResult struct {
	WrapperType      string `json:"wrapperType"`
	CollectionID     int64  `json:"collectionId"`
	CollectionName   string `json:"collectionName"`
	ArtistName       string `json:"artistName"`
	ArtworkURL100    string `json:"artworkUrl100"`
	ReleaseDate      string `json:"releaseDate"`
	PrimaryGenreName string `json:"primaryGenreName"`
	Description      string `json:"description"`
	Copyright        string `json:"copyright"`
}

// SearchBooks searches title and author as one term; a query without title finds nothing.
func (p *appleBooksProvider) SearchBooks(ctx context.Context, q BookQuery) ([]BookCandidate, error) {
	if strings.TrimSpace(q.Title) == "" {
		return nil, nil
	}
	return p.cached(bookCacheKey(p.name, "", BookQuery{Title: q.Title, Author: q.Author}), func() ([]BookCandidate, error) {
		term := strings.TrimSpace(q.Title + " " + q.Author)
		rs, err := p.get(ctx, "/search", url.Values{"country": {appleCountry}, "media": {"audiobook"},
			"limit": {strconv.Itoa(bookResults)}, "term": {term}})
		if err != nil {
			return nil, err
		}
		out := make([]BookCandidate, 0, len(rs))
		for _, r := range rs {
			if r.CollectionID > 0 && r.CollectionName != "" {
				out = append(out, r.candidate())
			}
		}
		return out, nil
	})
}

// Book looks a collection id up.
func (p *appleBooksProvider) Book(ctx context.Context, id, _ string) (BookRecord, error) {
	id = strings.TrimSpace(id)
	if !appleIDRe.MatchString(id) {
		return BookRecord{}, fmt.Errorf("applebooks %q: %w", id, ErrNotFound)
	}
	rs, err := p.get(ctx, "/lookup", url.Values{"id": {id}, "country": {appleCountry}})
	if err != nil {
		return BookRecord{}, err
	}
	for _, r := range rs {
		if strconv.FormatInt(r.CollectionID, 10) != id || r.CollectionName == "" {
			continue
		}
		rec := BookRecord{BookCandidate: r.candidate(), Overview: bookText(r.Description)}
		if g := strings.TrimSpace(r.PrimaryGenreName); g != "" {
			rec.Genres = []string{g}
		}
		if rec.CoverURL != "" {
			rec.Covers = []ImageCandidate{{Kind: "poster", Source: rec.CoverURL, Default: true}}
		}
		return rec, nil
	}
	return BookRecord{}, fmt.Errorf("applebooks %s: %w", id, ErrNotFound)
}

func (p *appleBooksProvider) get(ctx context.Context, path string, q url.Values) ([]appleResult, error) {
	body, status, err := p.fetch(ctx, p.http, p.limiter, queryURL(p.base, path, q), bookJSON)
	if err != nil || status == http.StatusNotFound {
		return nil, err
	}
	var out struct {
		Results []appleResult `json:"results"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: applebooks decode: %w", model.ErrTMDBUnreachable, err)
	}
	return out.Results, nil
}

func (r appleResult) candidate() BookCandidate {
	title, sub, _ := strings.Cut(r.CollectionName, ": ")
	c := BookCandidate{
		Provider: ProviderAppleBooks, ID: strconv.FormatInt(r.CollectionID, 10),
		Title: strings.TrimSpace(title), Subtitle: strings.TrimSpace(sub),
		Year: bookYear(r.ReleaseDate), Publisher: strings.TrimSpace(appleCopyright.ReplaceAllString(r.Copyright, "")),
		CoverURL: appleArtwork(r.ArtworkURL100),
	}
	for _, a := range appleArtists.Split(r.ArtistName, -1) {
		if a = strings.TrimSpace(a); a != "" {
			c.Authors = append(c.Authors, a)
		}
	}
	return c
}

// appleArtwork asks the artwork as 3000x3000 ("…/100x100bb.jpg" → "…/3000x3000bb.jpg").
func appleArtwork(u string) string {
	if u == "" || !appleArtworkRe.MatchString(u) {
		return u
	}
	return appleArtworkRe.ReplaceAllString(u, "/3000x3000bb.jpg")
}
