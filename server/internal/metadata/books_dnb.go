package metadata

import (
	"bytes"
	"cmp"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The Deutsche Nationalbibliothek (concept 19 D6.1, CC0): SRU searchRetrieve with MARC21-xml,
// by title and the author's last name (tit=… and per=…), by ISBN (num=…) or by its record id
// (idn=…). It names title and subtitle (245 $a/$b), authors and narrators (100/700 with the
// relator code nrt or the function «Sprecher»), publisher and year (264, else 260), ISBN (020),
// language (041), series (830, else 490) and seldom a length (306 hhmmss); no cover, no overview.

const marcNS = "http://www.loc.gov/MARC21/slim"

var dnbIDRe = regexp.MustCompile(`^[0-9]{8,10}[0-9X]?$`)

type dnbProvider struct {
	bookSource
	base string
}

func (p *dnbProvider) Name() string                    { return ProviderDNB }
func (p *dnbProvider) Configured(context.Context) bool { return true }
func (p *dnbProvider) HasDuration() bool               { return false }

// SearchBooks finds books by ISBN, else by title and the author's last name. An ASIN finds
// nothing here.
func (p *dnbProvider) SearchBooks(ctx context.Context, q BookQuery) ([]BookCandidate, error) {
	var cql string
	switch isbn := normISBN(q.ISBN); {
	case isbn != "":
		cql = "num=" + isbn
	case strings.TrimSpace(q.Title) != "":
		cql = `tit="` + cqlValue(q.Title) + `"`
		if last := lastName(q.Author); last != "" {
			cql += ` and per="` + cqlValue(last) + `"`
		}
	default:
		return nil, nil
	}
	return p.cached(bookCacheKey(p.name, "", BookQuery{Title: q.Title, Author: q.Author, ISBN: q.ISBN}), func() ([]BookCandidate, error) {
		recs, err := p.records(ctx, cql)
		if err != nil {
			return nil, err
		}
		out := make([]BookCandidate, 0, len(recs))
		for _, r := range recs {
			if c := r.candidate(); c.ID != "" && c.Title != "" {
				out = append(out, c)
			}
		}
		return out, nil
	})
}

// Book fetches one record by its IDN.
func (p *dnbProvider) Book(ctx context.Context, id, _ string) (BookRecord, error) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if !dnbIDRe.MatchString(id) {
		return BookRecord{}, fmt.Errorf("dnb %q: %w", id, ErrNotFound)
	}
	recs, err := p.records(ctx, "idn="+id)
	if err != nil {
		return BookRecord{}, err
	}
	for _, r := range recs {
		if c := r.candidate(); c.ID == id {
			return BookRecord{BookCandidate: c}, nil
		}
	}
	return BookRecord{}, fmt.Errorf("dnb %s: %w", id, ErrNotFound)
}

// cqlValue removes the quotes and backslashes a CQL term cannot carry inside quotes.
func cqlValue(s string) string {
	return strings.Join(strings.Fields(strings.NewReplacer(`"`, " ", `\`, " ").Replace(s)), " ")
}

// lastName returns the last name of an author: the part before a comma ("Tolkien, J. R. R."),
// else the last word ("J. R. R. Tolkien"), without trailing punctuation.
func lastName(author string) string {
	author = strings.TrimSpace(author)
	if before, _, ok := strings.Cut(author, ","); ok {
		author = before
		return strings.Trim(strings.TrimSpace(author), ".;:")
	}
	f := strings.Fields(author)
	if len(f) == 0 {
		return ""
	}
	return strings.Trim(f[len(f)-1], ".;:")
}

func (p *dnbProvider) records(ctx context.Context, cql string) ([]marcRecord, error) {
	q := url.Values{"version": {"1.1"}, "operation": {"searchRetrieve"}, "recordSchema": {"MARC21-xml"},
		"maximumRecords": {strconv.Itoa(bookResults)}, "query": {cql}}
	body, status, err := p.fetch(ctx, p.http, p.limiter, queryURL(p.base, "", q), bookXML)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	recs, err := parseMARC(body)
	if err != nil {
		return nil, fmt.Errorf("%w: dnb decode: %w", model.ErrTMDBUnreachable, err)
	}
	return recs, nil
}

type marcSubfield struct {
	Code  string `xml:"code,attr"`
	Value string `xml:",chardata"`
}

type marcField struct {
	Tag  string         `xml:"tag,attr"`
	Ind2 string         `xml:"ind2,attr"`
	Subs []marcSubfield `xml:"subfield"`
}

type marcRecord struct {
	Controls []struct {
		Tag   string `xml:"tag,attr"`
		Value string `xml:",chardata"`
	} `xml:"controlfield"`
	Fields []marcField `xml:"datafield"`
}

// parseMARC reads every MARC21 record of an SRU answer. An SRU diagnostic (a query DNB refuses)
// is an error.
func parseMARC(body []byte) ([]marcRecord, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	var out []marcRecord
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch {
		case se.Name.Local == "record" && se.Name.Space == marcNS:
			var r marcRecord
			if err := dec.DecodeElement(&r, &se); err != nil {
				return nil, err
			}
			out = append(out, r)
		case se.Name.Local == "diagnostic":
			var d struct {
				Message string `xml:"message"`
				Details string `xml:"details"`
			}
			if err := dec.DecodeElement(&d, &se); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("sru diagnostic: %s %s", strings.TrimSpace(d.Message), strings.TrimSpace(d.Details))
		}
	}
}

func (r marcRecord) control(tag string) string {
	for _, c := range r.Controls {
		if c.Tag == tag {
			return strings.TrimSpace(c.Value)
		}
	}
	return ""
}

func (r marcRecord) fields(tag string) []marcField {
	var out []marcField
	for _, f := range r.Fields {
		if f.Tag == tag {
			out = append(out, f)
		}
	}
	return out
}

func (f marcField) sub(code string) string {
	for _, s := range f.Subs {
		if s.Code == code {
			return marcText(s.Value)
		}
	}
	return ""
}

func (f marcField) subs(code string) []string {
	var out []string
	for _, s := range f.Subs {
		if s.Code == code {
			out = append(out, marcText(s.Value))
		}
	}
	return out
}

// marcText drops the non-sorting marks of DNB (U+0098, U+009C around «Der»), the ISBD
// punctuation at the end and outer brackets.
func marcText(s string) string {
	s = strings.NewReplacer("\u0098", "", "\u009c", "").Replace(s)
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, " /:;,=")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// marcName turns "Tolkien, J. R. R." into "J. R. R. Tolkien".
func marcName(s string) string {
	last, first, ok := strings.Cut(s, ",")
	if !ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func (r marcRecord) candidate() BookCandidate {
	c := BookCandidate{Provider: ProviderDNB, ID: r.control("001")}
	if f := r.fields("245"); len(f) > 0 {
		c.Title, c.Subtitle = f[0].sub("a"), f[0].sub("b")
	}
	for _, tag := range []string{"100", "700"} {
		for _, f := range r.fields(tag) {
			name := marcName(f.sub("a"))
			if name == "" {
				continue
			}
			switch dnbRole(f) {
			case "author":
				c.Authors = append(c.Authors, name)
			case "narrator":
				c.Narrators = append(c.Narrators, name)
			}
		}
	}
	// The publication statement (264 with second indicator 1) first, then the other 264 (copyright,
	// manufacture) and 260 for what is still missing.
	pub := slices.SortedStableFunc(slices.Values(append(r.fields("264"), r.fields("260")...)), func(a, b marcField) int {
		return cmp.Compare(publicationRank(a), publicationRank(b))
	})
	for _, f := range pub {
		if c.Publisher == "" {
			c.Publisher = f.sub("b")
		}
		if c.Year == nil {
			c.Year = bookYear(f.sub("c"))
		}
	}
	var isbns []string
	for _, f := range r.fields("020") {
		isbns = append(isbns, f.subs("a")...)
		isbns = append(isbns, f.subs("9")...)
	}
	c.ISBN = pickISBN(isbns)
	if f := r.fields("041"); len(f) > 0 {
		c.Language = normalizeOriginalLanguage(f[0].sub("a"))
	}
	series := r.fields("830")
	if len(series) == 0 {
		series = r.fields("490")
	}
	if len(series) > 0 {
		c.SeriesName, c.SeriesIndex = series[0].sub("a"), seriesIndex(series[0].sub("v"))
	}
	for _, f := range r.fields("306") {
		if ms := hhmmss(f.sub("a")); ms > 0 {
			c.DurationMs = ms
			break
		}
	}
	return c
}

// publicationRank orders the publication fields: 264 publication, 260, other 264.
func publicationRank(f marcField) int {
	switch {
	case f.Tag == "264" && f.Ind2 == "1":
		return 0
	case f.Tag == "260":
		return 1
	}
	return 2
}

// dnbRole sorts a person of field 100 or 700: narrator with the relator code nrt or the function
// «Sprecher»/«Erzähler», author with aut, «Verfasser» or no function at all; "" for any other role
// (translator, editor, composer).
func dnbRole(f marcField) string {
	codes, funcs := f.subs("4"), f.subs("e")
	if len(codes) == 0 && len(funcs) == 0 {
		return "author"
	}
	for _, c := range codes {
		switch strings.ToLower(c) {
		case "nrt":
			return "narrator"
		case "aut":
			return "author"
		}
	}
	for _, e := range funcs {
		e = strings.ToLower(e)
		switch {
		case strings.Contains(e, "sprecher"), strings.Contains(e, "erzähler"):
			return "narrator"
		case strings.Contains(e, "verfasser"):
			return "author"
		}
	}
	return ""
}

// hhmmss reads a MARC 306 length "011530" (1 h 15 min 30 s) as milliseconds; 0 when it is none.
func hhmmss(s string) int64 {
	if len(s) != 6 {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	h, m, sec := n/10000, n/100%100, n%100
	if m >= 60 || sec >= 60 {
		return 0
	}
	return int64(h*3600+m*60+sec) * 1000
}
