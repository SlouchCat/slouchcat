package library

import (
	"mime"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// File names and comparison keys of stored podcast episodes (ADR 0029, decisions 5 and 6; they
// replace section 6.3 of concept 19). A stored episode is named like Audiobookshelf names its
// downloads (sanitizeFilename in server/utils/fileUtils.js, read on 11 November 2026; reference
// implementation abs_sanitize in docs/perf/19-podcasts/m1.py), so both tools may write into the
// same folder for a while. The key compares a feed episode with the files of its folders whatever
// either tool did to the title.

var (
	absIllegal  = regexp.MustCompile(`[/?<>\\:*|"]`)
	absControl  = regexp.MustCompile(`[\x00-\x1f\x80-\x9f]`)
	absDots     = regexp.MustCompile(`^\.+$`)
	absReserved = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\..*)?$`)
	absTrailing = regexp.MustCompile(`[. ]+$`)
	// absUUIDSuffix is the suffix Audiobookshelf puts on a name that is taken (" (<uuid>)"); it does
	// not count for the key.
	absUUIDSuffix = regexp.MustCompile(` \([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\)$`)
)

// Bounds of a stored episode's name (decision 5): 255 bytes in UTF-16 like Audiobookshelf and at
// most 250 bytes in UTF-8 as well, for file systems with 255 bytes per name.
const (
	maxNameUTF16 = 255
	maxNameUTF8  = 250
)

// absSanitize is Audiobookshelf's sanitizeFilename with the colon replacement " - ": NFC, the
// first colon becomes " - ", the characters / ? < > \ : * | " and control characters go, a name
// of dots only and a reserved Windows name become empty, dots and spaces at the end go, runs of
// white space become one space, and a name over 255 UTF-16 bytes loses characters at the end of
// its stem (then trimmed). The extension is what follows the last dot, as in Node's path.extname.
func absSanitize(name string) string {
	s := norm.NFC.String(name)
	s = strings.Replace(s, ":", " - ", 1)
	s = absIllegal.ReplaceAllString(s, "")
	s = absControl.ReplaceAllString(s, "")
	s = absDots.ReplaceAllString(s, "")
	s = strings.NewReplacer("\n", "", "\r", "").Replace(s)
	s = absReserved.ReplaceAllString(s, "")
	s = absTrailing.ReplaceAllString(s, "")
	s = collapseSpace(s)
	stem, ext := splitExt(s)
	if utf16Len(stem)+utf16Len(ext) > maxNameUTF16 {
		s = strings.TrimFunc(cutRunes(stem, func(b string) bool { return utf16Len(b) > maxNameUTF16-utf16Len(ext) }), jsSpace) + ext
	}
	return s
}

// podcastFileName is the name of a stored episode (decision 5): Audiobookshelf's rule on
// "<title>.<ext>", at most maxNameUTF8 bytes in UTF-8, without leading dots and spaces (the
// scanner passes over a name with a leading dot). A stem without a letter or a digit becomes
// "Folge <YYYY-MM-DD>".
func podcastFileName(title, ext string, published time.Time) string {
	stem := strings.TrimSuffix(absSanitize(title+"."+ext), "."+ext)
	stem = strings.TrimLeftFunc(stem, func(r rune) bool { return r == '.' || jsSpace(r) })
	if podcastKey(stem) == "" {
		stem = "Folge " + published.UTC().Format(time.DateOnly)
	}
	return fitName(stem, "."+ext)
}

// podcastNameVariant is the n-th name of a stored episode whose name is taken (decision 5): 0 the
// name itself, 1 the publication date in front, from 2 on also " (n)" at the stem.
func podcastNameVariant(name string, published time.Time, n int) string {
	if n == 0 {
		return name
	}
	stem, ext := splitExt(name)
	stem = published.UTC().Format(time.DateOnly) + " " + stem
	tail := ext
	if n > 1 {
		tail = " (" + strconv.Itoa(n) + ")" + ext
	}
	return fitName(stem, tail)
}

// fitName shortens stem at a character boundary until stem+tail fits both bounds.
func fitName(stem, tail string) string {
	too := func(b string) bool {
		return utf16Len(b)+utf16Len(tail) > maxNameUTF16 || len(b)+len(tail) > maxNameUTF8
	}
	if too(stem) {
		stem = strings.TrimRightFunc(cutRunes(stem, too), jsSpace)
	}
	return stem + tail
}

// cutRunes returns the longest prefix of s, in whole characters, for which over is false.
func cutRunes(s string, over func(string) bool) string {
	end := 0
	for i, r := range s {
		next := i + utf8.RuneLen(r)
		if over(s[:next]) {
			break
		}
		end = next
	}
	return s[:end]
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += 2 * utf16.RuneLen(r)
	}
	return n
}

// jsSpace is the white space of JavaScript's \s and trim(): Unicode spaces and the byte order mark.
func jsSpace(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' }

// collapseSpace turns every run of white space into one space, like JavaScript's replace(/\s+/g, ' ').
func collapseSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if jsSpace(r) {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

// splitExt splits a name at its last dot like Node's path.extname: a name whose only dot leads
// has no extension.
func splitExt(name string) (stem, ext string) {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 {
		return name, ""
	}
	return name[:i], name[i:]
}

// podcastKey is the comparison key of section 8 (decision 6): NFC, lower case, letters and digits
// only.
func podcastKey(s string) string {
	s = strings.ToLower(norm.NFC.String(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// podcastFileKey is the key of a file in a podcast folder: its stem, without Audiobookshelf's
// " (<uuid>)" suffix.
func podcastFileKey(rel string) string {
	stem, _ := splitExt(path.Base(rel))
	return podcastKey(absUUIDSuffix.ReplaceAllString(stem, ""))
}

// absAudioExts are Audiobookshelf's audio types (globals.SupportedAudioTypes): the extension it
// gives a download from the enclosure URL, else mp3.
var absAudioExts = map[string]bool{"m4b": true, "mp3": true, "m4a": true, "flac": true, "opus": true, "ogg": true, "oga": true,
	"mp4": true, "aac": true, "wma": true, "aiff": true, "aif": true, "wav": true, "webm": true, "webma": true, "mka": true,
	"awb": true, "caf": true, "mpeg": true, "mpg": true}

// podcastEpisodeKeys are the keys of a feed episode (decision 6): of its title and of the stem of
// the name Audiobookshelf would give it (which differs when the name was shortened). Empty keys are
// left out.
func podcastEpisodeKeys(title, enclosureURL string) []string {
	ext := urlExt(enclosureURL)
	if !absAudioExts[ext] {
		ext = "mp3"
	}
	stem, _ := splitExt(absSanitize(strings.TrimSpace(title) + "." + ext))
	var out []string
	for _, k := range []string{podcastKey(title), podcastKey(stem)} {
		if k != "" && (len(out) == 0 || out[0] != k) {
			out = append(out, k)
		}
	}
	return out
}

// urlExt is the lower-case extension of the path of a URL ("" without one).
func urlExt(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))
}

// typeExts are the scanned audio extensions of the media types an enclosure answers with.
var typeExts = map[string]string{
	"audio/mpeg": "mp3", "audio/mp3": "mp3", "audio/mpeg3": "mp3", "audio/x-mpeg": "mp3",
	"audio/mp4": "m4a", "audio/x-m4a": "m4a", "audio/m4a": "m4a", "audio/x-m4b": "m4b",
	"audio/ogg": "ogg", "audio/vorbis": "ogg", "audio/opus": "opus", "audio/flac": "flac", "audio/x-flac": "flac",
}

// probeExts are the scanned audio extensions of the formats ffprobe names.
var probeExts = map[string]string{"mp3": "mp3", "mov,mp4,m4a,3gp,3g2,mj2": "m4a", "ogg": "ogg", "flac": "flac"}

// podcastExt is the extension of a stored episode (decision 5): from the path of the enclosure URL
// when it is a scanned audio extension, else from the media type of the answer, else from the
// format of the probe (an Ogg file with an Opus stream is opus); "" when none fits, and then the
// episode is not stored (O-3).
func podcastExt(enclosureURL, contentType, probeFormat, audioCodec string) string {
	if ext := urlExt(enclosureURL); audioExts[ext] {
		return ext
	}
	if mt, _, err := mime.ParseMediaType(contentType); err == nil && typeExts[mt] != "" {
		return typeExts[mt]
	}
	ext := probeExts[probeFormat]
	if ext == "ogg" && audioCodec == "opus" {
		ext = "opus"
	}
	return ext
}

// podcastFormatKnown reports whether the planner may load an enclosure (O-3): false when its URL
// or its media type in the feed names a format the scanner does not read (video, AAC streams, WAV
// and the like); an enclosure without either is loaded, and the download decides by the probe.
func podcastFormatKnown(enclosureURL, enclosureType string) bool {
	if ext := urlExt(enclosureURL); ext != "" {
		if audioExts[ext] {
			return true
		}
		if absAudioExts[ext] || ext == "m4v" || ext == "mov" {
			return false
		}
	}
	mt, _, err := mime.ParseMediaType(enclosureType)
	switch {
	case err != nil || mt == "" || mt == "application/octet-stream":
		return true
	case typeExts[mt] != "":
		return true
	}
	return false
}
