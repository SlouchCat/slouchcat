package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// Audible (concept 19 D6.1, D6.3, ADR 0031): the search runs against the catalog API of a
// marketplace (api.audible.<region>/1.0/catalog/products, no key), the details come from Audnexus
// (<audnexus>/books/<ASIN>?region=…), which adds ISBN and genres. An ASIN belongs to one
// marketplace: the catalog answers a foreign ASIN with a product that has no fields, Audnexus with
// 404 REGION_UNAVAILABLE. Both then try the region of the installation, audible.com and
// audible.co.uk, and Audnexus last without region (measurement D0, section 3).

// audibleResponseGroups are the catalog fields a candidate needs.
const audibleResponseGroups = "product_desc,product_attrs,contributors,series,media"

// audnexusRegions maps the Audible marketplaces onto the region codes of Audnexus.
var audnexusRegions = map[string]string{
	"de": "de", "com": "us", "co.uk": "uk", "fr": "fr", "it": "it", "es": "es",
	"ca": "ca", "com.au": "au", "in": "in", "co.jp": "jp",
}

var asinRe = regexp.MustCompile(`^[A-Z0-9]{10}$`)

// audibleProvider searches the catalog and fetches Audnexus. region and audnexus are read per call
// (the settings audible_region and audnexus_url; set by the service).
type audibleProvider struct {
	bookSource
	base            func(region string) string
	audnexusLimiter *ratelimit.Limiter
	audnexusClients netguard.Clients
	region          func(ctx context.Context) string
	audnexus        func(ctx context.Context) string
}

// newAudnexusClients are the clients of the administrator's Audnexus address: the guarded one
// refuses local addresses, the other is used only when the address's own host is local. A
// redirect has to stay on the host of the address, so a local Audnexus cannot lead the server to
// another address in the household's network.
func newAudnexusClients() netguard.Clients {
	return netguard.NewClients(http.Client{Timeout: httpTimeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > maxBookRedirects || req.URL.Host != via[0].URL.Host {
			return errors.New("audnexus redirects to another host")
		}
		return nil
	}}, 0)
}

func (p *audibleProvider) Name() string                    { return ProviderAudible }
func (p *audibleProvider) Configured(context.Context) bool { return true }
func (p *audibleProvider) HasDuration() bool               { return true }

// regions returns the region of the query or of the installation, then the extra regions.
func (p *audibleProvider) regions(ctx context.Context, first string, extra ...string) ([]string, error) {
	if first == "" {
		first = p.region(ctx)
	}
	if !slices.Contains(AudibleRegions, first) {
		return nil, fmt.Errorf("%w: %q", errBookRegion, first)
	}
	out := []string{first}
	for _, r := range extra {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out, nil
}

// SearchBooks finds books by ASIN (with the region switch: the first region that knows it) or by
// title and author (English books also on audible.com and audible.co.uk; candidates of several
// regions are merged by ASIN). A query with an ISBN only finds nothing here.
func (p *audibleProvider) SearchBooks(ctx context.Context, q BookQuery) ([]BookCandidate, error) {
	asin := strings.ToUpper(strings.TrimSpace(q.ASIN))
	if asin != "" {
		if !asinRe.MatchString(asin) {
			return nil, nil
		}
		regions, err := p.regions(ctx, q.Region, "com", "co.uk")
		if err != nil {
			return nil, err
		}
		return p.cached(bookCacheKey(p.name, regions[0], BookQuery{ASIN: asin}), func() ([]BookCandidate, error) {
			for _, r := range regions {
				c, ok, err := p.product(ctx, r, asin)
				if err != nil {
					return nil, err
				}
				if ok {
					return []BookCandidate{c}, nil
				}
			}
			return nil, nil
		})
	}
	if strings.TrimSpace(q.Title) == "" {
		return nil, nil
	}
	var extra []string
	if q.Lang == "en" {
		extra = []string{"com", "co.uk"}
	}
	regions, err := p.regions(ctx, q.Region, extra...)
	if err != nil {
		return nil, err
	}
	return p.cached(bookCacheKey(p.name, regions[0], BookQuery{Title: q.Title, Author: q.Author, Lang: q.Lang}), func() ([]BookCandidate, error) {
		var out []BookCandidate
		for _, r := range regions {
			cs, err := p.search(ctx, r, q.Title, q.Author)
			if err != nil {
				return nil, err
			}
			for _, c := range cs {
				if !slices.ContainsFunc(out, func(o BookCandidate) bool { return o.ID == c.ID }) {
					out = append(out, c)
				}
			}
		}
		return out, nil
	})
}

type audiblePerson struct {
	Name string `json:"name"`
}

type audibleProduct struct {
	ASIN             string          `json:"asin"`
	Title            string          `json:"title"`
	Subtitle         string          `json:"subtitle"`
	Authors          []audiblePerson `json:"authors"`
	Narrators        []audiblePerson `json:"narrators"`
	PublisherName    string          `json:"publisher_name"`
	ReleaseDate      string          `json:"release_date"`
	Language         string          `json:"language"`
	RuntimeLengthMin int64           `json:"runtime_length_min"`
	Series           []struct {
		Title    string `json:"title"`
		Sequence string `json:"sequence"`
	} `json:"series"`
	ProductImages map[string]string `json:"product_images"`
}

func (p *audibleProvider) catalogURL(region, path string, q url.Values) string {
	q.Set("response_groups", audibleResponseGroups)
	return queryURL(strings.TrimRight(p.base(region), "/"), "/1.0/catalog/products"+path, q)
}

// search asks the catalog of one region by title and author.
func (p *audibleProvider) search(ctx context.Context, region, title, author string) ([]BookCandidate, error) {
	q := url.Values{"title": {title}, "num_results": {strconv.Itoa(bookResults)}, "products_sort_by": {"Relevance"}}
	if author != "" {
		q.Set("author", author)
	}
	body, status, err := p.fetch(ctx, p.http, p.limiter, p.catalogURL(region, "", q), bookJSON)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	var out struct {
		Products []audibleProduct `json:"products"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: audible decode: %w", model.ErrTMDBUnreachable, err)
	}
	var cs []BookCandidate
	for _, pr := range out.Products {
		if pr.ASIN != "" && pr.Title != "" {
			cs = append(cs, pr.candidate(region))
		}
	}
	return cs, nil
}

// product asks the catalog of one region for an ASIN; ok is false when the region does not know
// it (a 404, or the product without fields that the catalog gives for a foreign ASIN).
func (p *audibleProvider) product(ctx context.Context, region, asin string) (BookCandidate, bool, error) {
	body, status, err := p.fetch(ctx, p.http, p.limiter, p.catalogURL(region, "/"+asin, url.Values{}), bookJSON)
	if err != nil || status == http.StatusNotFound || len(bytes.TrimSpace(body)) == 0 {
		return BookCandidate{}, false, err
	}
	var out struct {
		Product audibleProduct `json:"product"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return BookCandidate{}, false, fmt.Errorf("%w: audible decode: %w", model.ErrTMDBUnreachable, err)
	}
	if out.Product.Title == "" {
		return BookCandidate{}, false, nil
	}
	return out.Product.candidate(region), true, nil
}

func (pr audibleProduct) candidate(region string) BookCandidate {
	c := BookCandidate{
		Provider: ProviderAudible, ID: pr.ASIN, Region: region, ASIN: pr.ASIN,
		Title: strings.TrimSpace(pr.Title), Subtitle: strings.TrimSpace(pr.Subtitle),
		Authors: audibleNames(pr.Authors), Narrators: audibleNames(pr.Narrators),
		Publisher: strings.TrimSpace(pr.PublisherName), Year: bookYear(pr.ReleaseDate),
		Language: normalizeOriginalLanguage(pr.Language), DurationMs: pr.RuntimeLengthMin * 60_000,
		CoverURL: largestImage(pr.ProductImages),
	}
	if len(pr.Series) > 0 {
		c.SeriesName = strings.TrimSpace(pr.Series[0].Title)
		c.SeriesIndex = seriesIndex(pr.Series[0].Sequence)
	}
	return c
}

func audibleNames(ps []audiblePerson) []string {
	var out []string
	for _, p := range ps {
		if n := strings.TrimSpace(p.Name); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// largestImage returns the image of the largest size key ("500", "1024") of product_images.
func largestImage(images map[string]string) string {
	best, src := -1, ""
	for k, v := range images {
		if n, err := strconv.Atoi(k); err == nil && n > best && v != "" {
			best, src = n, v
		}
	}
	return src
}

type audnexusBook struct {
	ASIN             string          `json:"asin"`
	Title            string          `json:"title"`
	Subtitle         string          `json:"subtitle"`
	Authors          []audiblePerson `json:"authors"`
	Narrators        []audiblePerson `json:"narrators"`
	PublisherName    string          `json:"publisherName"`
	ReleaseDate      string          `json:"releaseDate"`
	Language         string          `json:"language"`
	RuntimeLengthMin int64           `json:"runtimeLengthMin"`
	SeriesPrimary    *struct {
		Name     string `json:"name"`
		Position string `json:"position"`
	} `json:"seriesPrimary"`
	Image   string `json:"image"`
	Summary string `json:"summary"`
	Genres  []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"genres"`
	ISBN   string `json:"isbn"`
	Region string `json:"region"`
}

// Book fetches the details of an ASIN from Audnexus: in region (or the installation's), then
// audible.com and audible.co.uk, then without region, as long as Audnexus answers
// REGION_UNAVAILABLE. Region of the record is the marketplace that answered ("" without region).
func (p *audibleProvider) Book(ctx context.Context, id, region string) (BookRecord, error) {
	asin := strings.ToUpper(strings.TrimSpace(id))
	if !asinRe.MatchString(asin) {
		return BookRecord{}, fmt.Errorf("audible %q: %w", id, ErrNotFound)
	}
	regions, err := p.regions(ctx, region, "com", "co.uk")
	if err != nil {
		return BookRecord{}, err
	}
	base := strings.TrimRight(p.audnexus(ctx), "/")
	hc := p.audnexusClients.For(ctx, base, "admin")
	for _, r := range append(regions, "") {
		q := url.Values{}
		if r != "" {
			q.Set("region", audnexusRegions[r])
		}
		body, status, err := p.fetch(ctx, hc, p.audnexusLimiter, queryURL(base, "/books/"+asin, q), bookJSON)
		if err != nil {
			return BookRecord{}, err
		}
		if status == http.StatusNotFound {
			if bytes.Contains(body, []byte("REGION_UNAVAILABLE")) {
				continue
			}
			return BookRecord{}, fmt.Errorf("audnexus %s: %w", asin, ErrNotFound)
		}
		var b audnexusBook
		if err := json.Unmarshal(body, &b); err != nil {
			return BookRecord{}, fmt.Errorf("%w: audnexus decode: %w", model.ErrTMDBUnreachable, err)
		}
		if b.ASIN == "" || b.Title == "" {
			return BookRecord{}, fmt.Errorf("audnexus %s: %w", asin, ErrNotFound)
		}
		return b.record(r), nil
	}
	return BookRecord{}, fmt.Errorf("audnexus %s: %w", asin, ErrNotFound)
}

func (b audnexusBook) record(region string) BookRecord {
	rec := BookRecord{BookCandidate: BookCandidate{
		Provider: ProviderAudible, ID: b.ASIN, Region: region, ASIN: b.ASIN,
		Title: strings.TrimSpace(b.Title), Subtitle: strings.TrimSpace(b.Subtitle),
		Authors: audibleNames(b.Authors), Narrators: audibleNames(b.Narrators),
		Publisher: strings.TrimSpace(b.PublisherName), Year: bookYear(b.ReleaseDate),
		Language: normalizeOriginalLanguage(b.Language), DurationMs: b.RuntimeLengthMin * 60_000,
		ISBN: normISBN(b.ISBN), CoverURL: b.Image,
	}, Overview: bookText(b.Summary)}
	if s := b.SeriesPrimary; s != nil {
		rec.SeriesName = strings.TrimSpace(s.Name)
		rec.SeriesIndex = seriesIndex(s.Position)
	}
	for _, g := range b.Genres {
		if g.Type == "genre" && strings.TrimSpace(g.Name) != "" {
			rec.Genres = append(rec.Genres, strings.TrimSpace(g.Name))
		}
	}
	if b.Image != "" {
		rec.Covers = []ImageCandidate{{Kind: "poster", Source: b.Image, Default: true}}
	}
	return rec
}
