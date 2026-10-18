package library

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// NFO holds the fields of a Kodi/Jellyfin/Radarr movie.nfo that the scanner uses (B-2).
type NFO struct {
	Title         string
	OriginalTitle string
	Year          int
	Plot          string
	Tagline       string
	Runtime       int // minutes
	Premiered     string
	MPAA          string
	Certification string
	TMDBID        int64
	IMDBID        string
	Genres        []string
}

// maxNFOSize bounds the NFO files the scanner reads; real ones are a few KiB.
const maxNFOSize = 1 << 20

var errNotUTF8 = errors.New("only UTF-8 NFO files are decoded")

// ParseNFO reads a movie NFO. It never fails: junk, a bare URL (the "URL NFO" that only points to
// a database page), a missing <movie> element or a non-UTF-8 charset yield the fields read so far,
// possibly an empty NFO. A UTF-8 byte order mark is allowed.
func ParseNFO(data []byte) NFO {
	var n NFO
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	start := bytes.Index(data, []byte("<movie"))
	if start < 0 {
		return n
	}
	// Keep the XML declaration for its encoding check; drop anything else before <movie>.
	body := data[start:]
	if decl := bytes.Index(data, []byte("<?xml")); decl >= 0 && decl < start {
		if end := bytes.Index(data[decl:], []byte("?>")); end >= 0 && decl+end < start {
			body = append(append([]byte{}, data[decl:decl+end+2]...), body...)
		}
	}

	d := xml.NewDecoder(bytes.NewReader(body))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(charset) {
		case "utf-8", "utf8", "us-ascii", "ascii":
			return input, nil
		}
		return nil, errNotUTF8
	}

	var (
		depth   int    // 1 inside <movie>
		field   string // direct child of <movie> being read
		attrs   []xml.Attr
		text    strings.Builder
		uniques = map[string]string{}
	)
	for {
		tok, err := d.Token()
		if err != nil {
			break // io.EOF or junk: keep what was read
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 && t.Name.Local != "movie" {
				return n
			}
			if depth == 2 {
				field, attrs = strings.ToLower(t.Name.Local), t.Attr
				text.Reset()
			}
		case xml.CharData:
			if depth == 2 {
				text.Write(t)
			}
		case xml.EndElement:
			if depth == 2 {
				n.set(field, strings.TrimSpace(text.String()), attrs, uniques)
				field = ""
			}
			depth--
			if depth == 0 {
				return n.withUniqueIDs(uniques)
			}
		}
	}
	return n.withUniqueIDs(uniques)
}

func (n *NFO) set(field, v string, attrs []xml.Attr, uniques map[string]string) {
	if v == "" {
		return
	}
	switch field {
	case "title":
		n.Title = v
	case "originaltitle":
		n.OriginalTitle = v
	case "year":
		if y, ok := yearOf(v); ok {
			n.Year = y
		}
	case "plot":
		n.Plot = v
	case "tagline":
		n.Tagline = v
	case "runtime":
		if m, err := strconv.Atoi(strings.Fields(v)[0]); err == nil && m > 0 {
			n.Runtime = m
		}
	case "premiered", "releasedate":
		if n.Premiered == "" {
			n.Premiered = v
		}
	case "mpaa":
		n.MPAA = v
	case "certification":
		n.Certification = v
	case "tmdbid":
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			n.TMDBID = id
		}
	case "imdbid":
		if imdbRe.MatchString(v) {
			n.IMDBID = v
		}
	case "id":
		if imdbRe.MatchString(v) && n.IMDBID == "" {
			n.IMDBID = v
		}
	case "uniqueid":
		typ := "imdb"
		for _, a := range attrs {
			if strings.EqualFold(a.Name.Local, "type") {
				typ = strings.ToLower(strings.TrimSpace(a.Value))
			}
		}
		uniques[typ] = v
	case "genre":
		for g := range strings.SplitSeq(v, " / ") {
			if g = strings.TrimSpace(g); g != "" {
				n.Genres = append(n.Genres, g)
			}
		}
	}
}

// withUniqueIDs lets <uniqueid> override the legacy <tmdbid>/<id> fields (Kodi v18+).
func (n NFO) withUniqueIDs(uniques map[string]string) NFO {
	if v, ok := uniques["tmdb"]; ok {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			n.TMDBID = id
		}
	}
	if v, ok := uniques["imdb"]; ok && imdbRe.MatchString(v) {
		n.IMDBID = v
	}
	return n
}

var (
	// "FSK 16", "FSK-16", "FSK16", "DE:16", "Germany:FSK 16", "ab 16", "FSK ab 16 Jahren", "DE-16".
	germanRatingRe = regexp.MustCompile(`^(?:(?:de|deu|ger|germany|deutschland)\s*[:/-]\s*)?(?:fsk\s*[-:]?\s*)?(?:ab\s*)?(\d{1,2})(?:\s*(?:jahren|j\.?))?$`)
	germanMarkerRe = regexp.MustCompile(`^(de|deu|ger|germany|deutschland)\s*[:/-]|^fsk|^ab\s*\d`)
)

// GermanRating maps the German (FSK) certification of the NFO to an own rating (0, 6, 12, 16,
// 18). Other countries' certifications return nil; their mapping is the job of package metadata.
func (n NFO) GermanRating() *int {
	for _, src := range []string{n.Certification, n.MPAA} {
		for part := range strings.SplitSeq(src, "/") {
			if r := germanRating(part); r != nil {
				return r
			}
		}
	}
	return nil
}

func germanRating(s string) *int {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "fsk 0", "fsk0", "fsk-0", "o.al.", "ohne altersbeschränkung", "fsk ohne altersbeschränkung":
		r := 0
		return &r
	}
	if !germanMarkerRe.MatchString(s) {
		return nil
	}
	m := germanRatingRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	r, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	switch r {
	case 0, 6, 12, 16, 18:
		return &r
	}
	return nil
}
