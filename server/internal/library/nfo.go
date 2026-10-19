package library

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NFO kinds: the root element of the file.
const (
	NFOMovie   = "movie"
	NFOTVShow  = "tvshow"
	NFOEpisode = "episodedetails"
)

// NFO holds the fields of a Kodi/Jellyfin/Radarr/Sonarr NFO that the scanner uses (B-2): a
// movie.nfo or <stem>.nfo (<movie>), a tvshow.nfo in the series folder (<tvshow>) or the
// <stem>.nfo of an episode (<episodedetails>).
type NFO struct {
	Kind          string // NFOMovie, NFOTVShow, NFOEpisode; "" when no known root element was found
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
	TVDBID        int64
	IMDBID        string
	Genres        []string
	Season        *int   // episodedetails
	Episode       *int   // episodedetails
	Aired         string // episodedetails: YYYY-MM-DD
}

// maxNFOSize bounds the NFO files the scanner reads; real ones are a few KiB.
const maxNFOSize = 1 << 20

var errNotUTF8 = errors.New("only UTF-8 NFO files are decoded")

// ParseNFO reads an NFO with the root element <movie>, <tvshow> or <episodedetails> (NFO.Kind);
// of a multi-episode file's NFO only the first <episodedetails> is read. It never fails: junk, a
// bare URL (the "URL NFO" that only points to a database page), a missing root element or a
// non-UTF-8 charset yield the fields read so far, possibly an empty NFO. A UTF-8 byte order mark
// is allowed.
func ParseNFO(data []byte) NFO {
	var n NFO
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	start := -1
	for _, root := range []string{NFOMovie, NFOTVShow, NFOEpisode} {
		if i := bytes.Index(data, []byte("<"+root)); i >= 0 && (start < 0 || i < start) {
			start = i
		}
	}
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
		depth   int    // 1 inside the root element
		field   string // direct child of the root element being read
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
			if depth == 1 {
				switch t.Name.Local {
				case NFOMovie, NFOTVShow, NFOEpisode:
					n.Kind = t.Name.Local
				default:
					return n
				}
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
	case "aired":
		if isDate(v) {
			n.Aired = v
		}
	case "season":
		if s, err := strconv.Atoi(v); err == nil && s >= 0 {
			n.Season = &s
		}
	case "episode":
		if e, err := strconv.Atoi(v); err == nil && e >= 0 {
			n.Episode = &e
		}
	case "tvdbid":
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			n.TVDBID = id
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

// withUniqueIDs lets <uniqueid> override the legacy <tmdbid>/<tvdbid>/<id> fields (Kodi v18+).
// A <tvshow> or <episodedetails> without <year> takes the year of its first air date.
func (n NFO) withUniqueIDs(uniques map[string]string) NFO {
	if n.Year == 0 && n.Kind != NFOMovie {
		for _, d := range []string{n.Premiered, n.Aired} {
			if len(d) >= 4 {
				if y, ok := yearOf(d[:4]); ok {
					n.Year = y
					break
				}
			}
		}
	}
	if v, ok := uniques["tvdb"]; ok {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			n.TVDBID = id
		}
	}
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

// isDate reports a YYYY-MM-DD date.
func isDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
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
