package radio

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
)

// Grouping of catalog entries into stations (concept 16 section 5.3): the entries of one
// broadcaster (the same program in several bitrates or formats) share a group key of the folded
// name without bitrate and format suffixes, the country and the host of the homepage. The key is
// fixed when the station is created; an alias never moves to another station, and two stations
// never merge (R-2).

var (
	// groupSuffixBracket removes a bracketed bitrate or format: «(aac)», «[64]», «(128 kbps)».
	groupSuffixBracket = regexp.MustCompile(`[\(\[]\s*(\d{2,3}\s*(k|kb|kbps|kbit|kbit/s|kb/s)?|aac\+?|he-aac|mp3|ogg|opus|flac|hd|hq|lq|low|high|mobile)\s*[\)\]]`)
	// groupSuffixWord removes a free-standing bitrate or format word: «128k», «64 kbps», «mp3», «hd».
	groupSuffixWord = regexp.MustCompile(`(^|[\s\-_|/])(\d{2,3}\s?(k|kb|kbps|kbit|kbit/s|kb/s)|aac\+?|he-aac|mp3|ogg|opus|flac|hd|hq|lq|mobile)($|[\s\-_|/])`)
	groupSpace      = regexp.MustCompile(`[\s\-_|/:.,]+`)
)

// foldGroupName folds a station name for its group key: model.SortKey without bitrate and format
// suffixes and with punctuation collapsed («SRF 3 (AAC) 128k» and «SRF 3 - MP3» → «srf 3»).
func foldGroupName(name string) string {
	s := rules.Fold(name)
	s = groupSuffixBracket.ReplaceAllString(s, " ")
	// The word pattern consumes its separators, so adjacent suffixes need a second pass.
	for range 3 {
		next := groupSuffixWord.ReplaceAllString(s, " ")
		if next == s {
			break
		}
		s = next
	}
	return strings.TrimSpace(groupSpace.ReplaceAllString(s, " "))
}

// hostOf returns the lower-cased host of rawURL without «www.»; "" when it has none.
func hostOf(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// groupKey returns the group key of a catalog entry: fold(name)|country|host, where host is the
// homepage's host, else (m-7) the host of url_resolved, else of url. Without any host, or without
// a name that survives the folding, the key is uuid:<stationuuid>, so the entry never groups.
func groupKey(e *rbStation) string {
	name := foldGroupName(e.Name)
	host := hostOf(e.Homepage)
	if host == "" {
		host = hostOf(e.URLResolved)
	}
	if host == "" {
		host = hostOf(e.URL)
	}
	if name == "" || host == "" {
		return "uuid:" + e.StationUUID
	}
	return name + "|" + strings.ToUpper(strings.TrimSpace(e.CountryCode)) + "|" + host
}
