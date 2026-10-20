// Package feed parses podcast feeds (M6, design 3.4, H-2): RSS 2.0 with the iTunes and the content
// namespace. It is pure (no database, no network); package metadata fetches the feed and writes the
// podcast and its episodes.
//
// Only items with an enclosure become episodes; an item without guid uses its enclosure URL, and a
// guid that appears twice keeps its first item. Dates accept RFC 1123/822 with numeric or named
// zones and ISO 8601; itunes:duration accepts seconds and h:mm:ss. Descriptions prefer
// content:encoded over description over itunes:summary and are reduced from HTML to text
// (paragraphs as blank lines, links as their text, at most MaxDescription characters); titles,
// authors, guids and URLs are bounded as well (MaxTitle, MaxURL). Atom and RSS 1.0 are not
// supported (ErrNotRSS).
package feed

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxDescription is the maximum length of a description in characters.
const MaxDescription = 4000

// Limits of the other fields a feed names (a feed may be 16 MiB): titles, authors and MIME types are
// clipped to MaxTitle characters, a guid to MaxURL; an image URL longer than MaxURL bytes is dropped,
// and so is an episode whose enclosure URL is.
const (
	MaxTitle = 500
	MaxURL   = 2048
)

// Namespaces of the podcast extensions.
const (
	nsITunes  = "http://www.itunes.com/dtds/podcast-1.0.dtd"
	nsContent = "http://purl.org/rss/1.0/modules/content/"
)

// Errors of Parse.
var (
	ErrNotRSS  = errors.New("feed: not an RSS 2.0 feed")
	ErrInvalid = errors.New("feed: invalid XML")
)

// Feed is the channel of a podcast feed.
type Feed struct {
	Title       string
	Description string
	Author      string
	Language    string
	ImageURL    string
	Explicit    bool
	Type        string // itunes:type: episodic or serial; empty when absent
	Episodes    []Episode
}

// Episode is one item of a feed with an enclosure.
type Episode struct {
	GUID        string
	Title       string
	Description string
	PublishedAt time.Time // zero when the feed gives no parsable date
	Duration    time.Duration
	Explicit    *bool // nil when the item does not say
	ImageURL    string
	Enclosure   Enclosure
	Number      int // itunes:episode, 0 when absent
	Season      int // itunes:season, 0 when absent
}

// Enclosure is the media file of an episode.
type Enclosure struct {
	URL    string
	Type   string
	Length int64
}

// node is a generic XML element; the parser selects children by namespace and local name.
type node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Text    string     `xml:",chardata"`
	Nodes   []node     `xml:",any"`
}

// Parse reads an RSS 2.0 podcast feed.
func Parse(r io.Reader) (Feed, error) {
	d := xml.NewDecoder(bufio.NewReader(r))
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charsetReader
	var root node
	if err := d.Decode(&root); err != nil {
		return Feed{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if root.XMLName.Local != "rss" || root.XMLName.Space != "" {
		return Feed{}, ErrNotRSS
	}
	ch := root.child("", "channel")
	if ch == nil {
		return Feed{}, ErrNotRSS
	}
	f := Feed{
		Title:    truncate(ch.text("", "title"), MaxTitle),
		Author:   truncate(first(ch.text(nsITunes, "author"), ch.text("", "managingEditor")), MaxTitle),
		Language: truncate(ch.text("", "language"), MaxTitle),
		ImageURL: shortURL(first(ch.attr(nsITunes, "image", "href"), ch.childText("", "image", "url"))),
		Explicit: isExplicit(ch.text(nsITunes, "explicit")),
		Type:     strings.ToLower(ch.text(nsITunes, "type")),
		Description: Text(first(ch.raw("", "description"), ch.raw(nsITunes, "summary"),
			ch.raw(nsContent, "encoded"))),
	}
	seen := map[string]bool{}
	for i := range ch.Nodes {
		it := &ch.Nodes[i]
		if !it.is("", "item") {
			continue
		}
		e, ok := episode(it)
		if !ok || seen[e.GUID] {
			continue
		}
		seen[e.GUID] = true
		f.Episodes = append(f.Episodes, e)
	}
	return f, nil
}

// episode maps one item; ok is false without an enclosure URL.
func episode(it *node) (Episode, bool) {
	enc := it.child("", "enclosure")
	if enc == nil {
		return Episode{}, false
	}
	e := Episode{Enclosure: Enclosure{URL: strings.TrimSpace(enc.attrValue("", "url")),
		Type: truncate(strings.TrimSpace(enc.attrValue("", "type")), MaxTitle)}}
	if e.Enclosure.URL == "" || len(e.Enclosure.URL) > MaxURL {
		return Episode{}, false
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(enc.attrValue("", "length")), 10, 64); err == nil && n > 0 {
		e.Enclosure.Length = n
	}
	e.GUID = truncate(first(it.text("", "guid"), e.Enclosure.URL), MaxURL)
	e.Title = truncate(first(it.text("", "title"), it.text(nsITunes, "title")), MaxTitle)
	e.Description = Text(first(it.raw(nsContent, "encoded"), it.raw("", "description"), it.raw(nsITunes, "summary")))
	e.PublishedAt = ParseDate(it.text("", "pubDate"))
	e.Duration = ParseDuration(it.text(nsITunes, "duration"))
	if v := it.text(nsITunes, "explicit"); v != "" {
		x := isExplicit(v)
		e.Explicit = &x
	}
	e.ImageURL = shortURL(it.attr(nsITunes, "image", "href"))
	e.Number, _ = strconv.Atoi(it.text(nsITunes, "episode"))
	e.Season, _ = strconv.Atoi(it.text(nsITunes, "season"))
	return e, true
}

func (n *node) is(space, local string) bool {
	return n.XMLName.Local == local && strings.EqualFold(n.XMLName.Space, space)
}

func (n *node) child(space, local string) *node {
	for i := range n.Nodes {
		if n.Nodes[i].is(space, local) {
			return &n.Nodes[i]
		}
	}
	return nil
}

// raw returns the untrimmed character data of the first matching child.
func (n *node) raw(space, local string) string {
	if c := n.child(space, local); c != nil {
		return c.Text
	}
	return ""
}

func (n *node) text(space, local string) string { return strings.TrimSpace(n.raw(space, local)) }

func (n *node) childText(space, local, sub string) string {
	if c := n.child(space, local); c != nil {
		return c.text(space, sub)
	}
	return ""
}

func (n *node) attr(space, local, name string) string {
	if c := n.child(space, local); c != nil {
		return strings.TrimSpace(c.attrValue("", name))
	}
	return ""
}

func (n *node) attrValue(space, name string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == name && strings.EqualFold(a.Name.Space, space) {
			return a.Value
		}
	}
	return ""
}

// truncate keeps at most n characters of s.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// shortURL keeps a URL of at most MaxURL bytes and drops a longer one.
func shortURL(s string) string {
	if len(s) > MaxURL {
		return ""
	}
	return s
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isExplicit(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "explicit":
		return true
	}
	return false
}

// dateLayouts are the pubDate forms seen in podcast feeds, most common first.
var dateLayouts = []string{
	time.RFC1123Z, time.RFC1123,
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04 -0700", "Mon, 2 Jan 2006 15:04 -0700", "Mon, 02 Jan 2006 15:04 MST",
	"02 Jan 2006 15:04:05 -0700", "2 Jan 2006 15:04:05 -0700", "02 Jan 2006 15:04:05 MST",
	time.RFC822Z, time.RFC822,
	time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02T15:04:05", "2006-01-02 15:04:05", time.DateOnly,
}

// usZones are the named zones RFC 822 allows besides UT and GMT. time.Parse knows only the
// abbreviations of the local location and reads any other with offset 0.
var usZones = map[string]int{"EST": -5, "EDT": -4, "CST": -6, "CDT": -5, "MST": -7, "MDT": -6, "PST": -8, "PDT": -7}

// ParseDate parses a pubDate; it returns the zero time when no layout fits.
func ParseDate(v string) time.Time {
	v = strings.Join(strings.Fields(v), " ")
	if v == "" {
		return time.Time{}
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			if name, offset := t.Zone(); offset == 0 {
				if h, ok := usZones[name]; ok {
					t = t.Add(-time.Duration(h) * time.Hour)
				}
			}
			return t.UTC()
		}
	}
	return time.Time{}
}

// ParseDuration parses itunes:duration as seconds ("3600", "3600.5") or h:mm:ss / mm:ss; anything
// else is 0.
func ParseDuration(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	parts := strings.Split(v, ":")
	if len(parts) > 3 {
		return 0
	}
	var total float64
	for i, p := range parts {
		n, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || n < 0 || (i < len(parts)-1 && n != float64(int64(n))) {
			return 0
		}
		total = total*60 + n
	}
	return time.Duration(total * float64(time.Second)).Round(time.Millisecond)
}

// blockTags end a paragraph; lineTags end a line.
var (
	blockTags = map[string]bool{"p": true, "div": true, "ul": true, "ol": true, "h1": true, "h2": true,
		"h3": true, "h4": true, "h5": true, "h6": true, "blockquote": true, "pre": true, "table": true}
	lineTags = map[string]bool{"br": true, "li": true, "tr": true}
	skipTags = map[string]bool{"script": true, "style": true}
)

// Text reduces an HTML description to plain text: tags are removed (links keep their text),
// entities decoded, paragraphs separated by a blank line, whitespace collapsed, and the result cut
// to MaxDescription characters.
func Text(s string) string {
	var b strings.Builder
	skip := ""
	for len(s) > 0 {
		lt := strings.IndexByte(s, '<')
		if lt < 0 {
			if skip == "" {
				b.WriteString(s)
			}
			break
		}
		if skip == "" {
			b.WriteString(s[:lt])
		}
		s = s[lt:]
		gt := strings.IndexByte(s, '>')
		if gt < 0 {
			if skip == "" {
				b.WriteString(s) // a lone "<" is text
			}
			break
		}
		name, closing := tagName(s[1:gt])
		s = s[gt+1:]
		switch {
		case skip != "":
			if closing && name == skip {
				skip = ""
			}
		case skipTags[name] && !closing:
			skip = name
		case blockTags[name]:
			b.WriteString("\n\n")
		case lineTags[name]:
			b.WriteString("\n")
		}
	}
	return clip(normalize(html.UnescapeString(b.String())))
}

// tagName returns the lower-case element name of a tag body ("a href=…", "/p", "br/").
func tagName(body string) (string, bool) {
	body = strings.TrimSpace(body)
	closing := strings.HasPrefix(body, "/")
	body = strings.TrimPrefix(body, "/")
	end := strings.IndexFunc(body, func(r rune) bool { return r == ' ' || r == '/' || r == '\t' || r == '\n' || r == '\r' })
	if end >= 0 {
		body = body[:end]
	}
	return strings.ToLower(body), closing
}

// normalize collapses spaces within lines, trims every line and keeps at most one blank line
// between paragraphs.
func normalize(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, l := range lines {
		l = strings.Join(strings.Fields(l), " ")
		if l == "" {
			blank = len(out) > 0
			continue
		}
		if blank {
			out = append(out, "")
			blank = false
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func clip(s string) string {
	if utf8.RuneCountInString(s) <= MaxDescription {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:MaxDescription-1])) + "…"
}

// charsetReader decodes the single-byte encodings some feeds still declare (ISO-8859-1 and the
// Windows variant, treated as Latin-1); UTF-8 needs no reader.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(charset) {
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1", "windows-1252", "cp1252", "us-ascii", "ascii":
		raw, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		var b bytes.Buffer
		for _, c := range raw {
			b.WriteRune(rune(c))
		}
		return &b, nil
	}
	return nil, fmt.Errorf("unsupported charset %q", charset)
}
