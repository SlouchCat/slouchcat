package radio

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
)

// Metadata cleanup of the titles a stream announces (concept 16 sections 8.1 to 8.3): ICY
// StreamTitle, ID3 frames of HLS packed audio and Ogg comments all end here.

// decodeMetaText returns b as text: UTF-8 when it is valid UTF-8, else Windows-1252 (the encoding
// of most older Shoutcast servers).
func decodeMetaText(b []byte) string {
	b = bytes.TrimRight(b, "\x00")
	if utf8.Valid(b) {
		return string(b)
	}
	s, err := charmap.Windows1252.NewDecoder().Bytes(b)
	if err != nil {
		return strings.ToValidUTF8(string(b), "")
	}
	return string(s)
}

// foldTitle folds text for comparisons: lower case, diacritics removed, spaces collapsed.
func foldTitle(s string) string { return model.SortKey("", s) }

// cleanTitle turns announced text into a timeline entry (At is set by the relay): spaces trimmed
// and collapsed, artist and title split at the first " - " or " – ". ok is false when the text
// announces no title: empty, only punctuation, the name of the station (icy-name or the catalog
// name), or a pattern of rules/notitle.json.
func cleanTitle(text string, stationNames ...string) (model.RadioNowEntry, bool) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || strings.IndexFunc(text, func(r rune) bool {
		return !unicode.IsPunct(r) && !unicode.IsSymbol(r) && !unicode.IsSpace(r)
	}) < 0 {
		return model.RadioNowEntry{}, false
	}
	folded := foldTitle(text)
	for _, n := range stationNames {
		if n != "" && foldTitle(n) == folded {
			return model.RadioNowEntry{}, false
		}
	}
	for _, re := range rules.NoTitlePatterns() {
		if re.MatchString(folded) {
			return model.RadioNowEntry{}, false
		}
	}
	e := model.RadioNowEntry{Text: text}
	i, sep := strings.Index(text, " - "), " - "
	if j := strings.Index(text, " – "); j >= 0 && (i < 0 || j < i) {
		i, sep = j, " – "
	}
	if i > 0 && i+len(sep) < len(text) {
		e.Artist, e.Title = strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+len(sep):])
	}
	return e, true
}

// joinArtistTitle builds the announced text of an ID3 or Ogg title from its parts.
func joinArtistTitle(artist, title string) string {
	artist, title = strings.TrimSpace(artist), strings.TrimSpace(title)
	switch {
	case artist == "":
		return title
	case title == "":
		return artist
	}
	return artist + " - " + title
}

// parseICYMeta reads StreamTitle and StreamUrl from an ICY metadata block
// (StreamTitle='…';StreamUrl='…';). The terminator is "';", so a title may contain "'".
func parseICYMeta(block []byte) (title string, hasTitle bool, streamURL string) {
	block = bytes.TrimRight(block, "\x00")
	field := func(key string) ([]byte, bool) {
		i := bytes.Index(block, []byte(key+"='"))
		if i < 0 {
			return nil, false
		}
		v := block[i+len(key)+2:]
		if j := bytes.Index(v, []byte("';")); j >= 0 {
			return v[:j], true
		}
		if j := bytes.LastIndexByte(v, '\''); j >= 0 {
			return v[:j], true
		}
		return v, true
	}
	t, ok := field("StreamTitle")
	u, _ := field("StreamUrl")
	return decodeMetaText(t), ok, decodeMetaText(u)
}
