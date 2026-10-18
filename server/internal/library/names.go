package library

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// ParsedName is what the movie name parser reads from a file path (Plex, Jellyfin, Radarr and
// scene naming).
type ParsedName struct {
	Title        string
	Year         *int
	TMDBID       int64
	IMDBID       string
	VersionLabel string // resolution plus edition, for example "1080p" or "2160p Director's Cut"
	Extra        bool   // trailer, featurette, … (B-7, M3); never a title of its own
	Sample       bool   // scene sample; never a title
}

var videoExts = map[string]bool{
	"mkv": true, "mp4": true, "m4v": true, "mov": true, "avi": true, "wmv": true, "webm": true,
	"ts": true, "m2ts": true, "mpg": true, "mpeg": true,
}

// IsVideo reports whether name has one of the video extensions the scanner picks up.
func IsVideo(name string) bool {
	return videoExts[extOf(name)]
}

// extOf returns the lower-cased extension of name without the dot.
func extOf(name string) string {
	return strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
}

var ignoredNames = map[string]bool{
	"@eadir": true, "#recycle": true, "#snapshot": true, "lost+found": true, "$recycle.bin": true,
	"system volume information": true, "bdmv": true, "video_ts": true, "audio_ts": true, "certificate": true,
}

// Ignored reports whether a directory entry is skipped by the scanner and the watcher: hidden
// entries, NAS metadata folders (Synology @eaDir, #recycle) and disc structures (BDMV, VIDEO_TS,
// not supported in V1).
func Ignored(name string) bool {
	return strings.HasPrefix(name, ".") || ignoredNames[strings.ToLower(name)]
}

// Extras folders inside a movie folder (Plex/Jellyfin local extras).
var extrasDirs = map[string]bool{
	"trailers": true, "featurettes": true, "deleted scenes": true, "behind the scenes": true,
	"interviews": true, "scenes": true, "shorts": true, "extras": true, "others": true, "other": true,
}

var extraSuffixes = []string{
	"-trailer", "-featurette", "-short", "-deleted", "-deletedscene", "-behindthescenes", "-interview",
	"-scene", "-other", "-extra",
}

var (
	idRe      = regexp.MustCompile(`(?i)[\[{(]\s*(tmdb|tmdbid|imdb|imdbid|tvdb|tvdbid)\s*[-=:]\s*([a-z0-9]+)\s*[\]})]`)
	editionRe = regexp.MustCompile(`(?i)\{\s*edition\s*[-=:]\s*([^}]*)\}`)
	resRe     = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(480p|576p|720p|1080p|1080i|2160p|4k|uhd)(?:$|[^a-z0-9])`)
	parenYear = regexp.MustCompile(`\(((?:19|20)\d{2})\)`)
	imdbRe    = regexp.MustCompile(`^tt\d{5,10}$`)
	partRe    = regexp.MustCompile(`(?i)^(cd|disc|disk|dvd|part|pt)[ ._-]?\d{1,2}$`)
	spacesRe  = regexp.MustCompile(`\s+`)
	// Audio channel tokens such as "5.1", "7.1", "DD5.1", "DDP5.1".
	channelsRe = regexp.MustCompile(`^(dd|ddp|dts|aac|ac3|eac3)?[257]\.[01]$`)
)

// Tokens that end the title of a scene or Radarr name when no year marks the end.
var qualityTokens = map[string]bool{
	"480p": true, "576p": true, "720p": true, "1080p": true, "1080i": true, "2160p": true, "4k": true,
	"uhd": true, "bluray": true, "blu-ray": true, "bdrip": true, "brrip": true, "bdremux": true,
	"web-dl": true, "webdl": true, "webrip": true, "web": true, "hdtv": true, "dvdrip": true, "hdrip": true,
	"x264": true, "x265": true, "hevc": true, "h264": true, "h265": true, "h.264": true, "h.265": true,
	"avc": true, "remux": true, "proper": true, "repack": true, "dts": true, "dts-hd": true, "truehd": true,
	"ac3": true, "eac3": true, "aac": true, "ddp": true, "dd": true, "atmos": true, "10bit": true,
	"hdr": true, "hdr10": true, "dv": true, "sdr": true, "german": true, "dl": true, "multi": true,
	"dubbed": true, "ac3d": true, "internal": true, "xvid": true, "divx": true,
}

// nameParts is the result of parsing one path segment (folder name or file stem).
type nameParts struct {
	title      string
	year       *int
	tmdb       int64
	imdb       string
	edition    string
	resolution string
}

func (p nameParts) hasIDs() bool { return p.tmdb != 0 || p.imdb != "" }

// ParseMoviePath parses the slash-separated path of a video file relative to its library root.
//
// The title comes from the movie folder ("one folder per movie", Radarr/Plex) when the file sits
// in a folder below the root, else from the file name. A folder without year and ids yields to a
// file name that has them (category folders such as "Movies/Title.2019.1080p.mkv"). Provider ids
// are merged from folder and file name (folder first); the version label comes from the file
// name, falling back to the folder.
func ParseMoviePath(rel string) ParsedName {
	segs := strings.Split(strings.Trim(rel, "/"), "/")
	file := segs[len(segs)-1]
	dirs := segs[:len(segs)-1]
	stem := strings.TrimSuffix(file, path.Ext(file))

	var res ParsedName
	for i, d := range dirs {
		ld := strings.ToLower(d)
		// A top-level "Shorts" or "Extras" folder is a category of the library, not local extras.
		if i >= 1 && extrasDirs[ld] {
			res.Extra = true
		}
		if ld == "sample" || ld == "samples" {
			res.Sample = true
		}
	}
	ls := strings.ToLower(strings.TrimSpace(stem))
	for _, suf := range extraSuffixes {
		if strings.HasSuffix(ls, suf) || strings.HasSuffix(ls, strings.Replace(suf, "-", " - ", 1)) || ls == suf[1:] {
			res.Extra = true
		}
	}
	if ls == "sample" || strings.HasSuffix(ls, "-sample") || strings.HasSuffix(ls, ".sample") ||
		strings.HasPrefix(ls, "sample.") || strings.HasPrefix(ls, "sample-") {
		res.Sample = true
	}

	fileP := parseName(stem)
	var folderP *nameParts
	for i := len(dirs) - 1; i >= 0; i-- {
		if specialDir(dirs[i]) {
			continue
		}
		p := parseName(dirs[i])
		folderP = &p
		break
	}

	src := fileP
	switch {
	case folderP != nil && (folderP.year != nil || folderP.hasIDs()):
		src = *folderP
	case fileP.year != nil || fileP.hasIDs():
	case folderP != nil:
		src = *folderP
	}
	res.Title, res.Year = src.title, src.year

	res.TMDBID, res.IMDBID = fileP.tmdb, fileP.imdb
	if folderP != nil {
		if folderP.tmdb != 0 {
			res.TMDBID = folderP.tmdb
		}
		if folderP.imdb != "" {
			res.IMDBID = folderP.imdb
		}
	}

	resolution, edition := fileP.resolution, fileP.edition
	if folderP != nil {
		if resolution == "" {
			resolution = folderP.resolution
		}
		if edition == "" {
			edition = folderP.edition
		}
	}
	res.VersionLabel = strings.TrimSpace(resolution + " " + edition)
	return res
}

// specialDir reports folders that never name the movie: extras, samples and part folders.
func specialDir(d string) bool {
	ld := strings.ToLower(d)
	return extrasDirs[ld] || ld == "sample" || ld == "samples" || partRe.MatchString(ld)
}

// parseName parses one folder name or file stem.
func parseName(s string) nameParts {
	var p nameParts
	// Dots and underscores separate words only in names without spaces (scene style); a spaced
	// name keeps its dots ("S.W.A.T. (2003)", "Mr. & Mrs. Smith (2005)").
	spaced := strings.Contains(strings.TrimSpace(s), " ")
	if m := resRe.FindStringSubmatch(s); m != nil {
		p.resolution = strings.ToLower(m[1])
		if p.resolution == "4k" || p.resolution == "uhd" {
			p.resolution = "2160p"
		}
	}
	if m := editionRe.FindStringSubmatch(s); m != nil {
		p.edition = strings.TrimSpace(m[1])
	}
	s = editionRe.ReplaceAllString(s, " ")
	for _, m := range idRe.FindAllStringSubmatch(s, -1) {
		v := strings.ToLower(m[2])
		switch strings.ToLower(m[1]) {
		case "tmdb", "tmdbid":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 && p.tmdb == 0 {
				p.tmdb = n
			}
		case "imdb", "imdbid":
			if imdbRe.MatchString(v) && p.imdb == "" {
				p.imdb = v
			}
		}
	}
	s = idRe.ReplaceAllString(s, " ")

	// "Title (Year) …": everything before the first parenthesised year is the title.
	for _, loc := range parenYear.FindAllStringSubmatchIndex(s, -1) {
		prefix := s[:loc[0]]
		if !spaced {
			prefix = strings.NewReplacer(".", " ", "_", " ").Replace(prefix)
		}
		if cleanTitle(prefix) == "" {
			continue
		}
		y, _ := strconv.Atoi(s[loc[2]:loc[3]])
		p.title, p.year = cleanTitle(prefix), &y
		return p
	}

	w := strings.TrimSpace(s)
	if spaced {
		w = strings.ReplaceAll(w, "_", " ")
	} else {
		w = strings.NewReplacer(".", " ", "_", " ").Replace(w)
	}
	tokens := strings.Fields(w)
	yearIdx, stopIdx := -1, len(tokens)
	for i, tok := range tokens {
		bare := strings.Trim(tok, "()[]{}")
		if i >= 1 && yearIdx < 0 {
			if _, ok := yearOf(bare); ok {
				yearIdx = i
				// "Blade.Runner.2049.2017": a second year right after the first ends the title.
				if i+1 < len(tokens) {
					if _, ok := yearOf(strings.Trim(tokens[i+1], "()[]{}")); ok {
						yearIdx = i + 1
					}
				}
			}
		}
		if i >= 1 && stopIdx == len(tokens) && isStopToken(tok) {
			stopIdx = i
		}
	}
	end := stopIdx
	if yearIdx >= 0 {
		y, _ := yearOf(strings.Trim(tokens[yearIdx], "()[]{}"))
		p.year = &y
		end = min(end, yearIdx)
	}
	p.title = cleanTitle(strings.Join(tokens[:end], " "))
	if p.title == "" {
		p.title = cleanTitle(w)
	}
	return p
}

func isStopToken(tok string) bool {
	if strings.HasPrefix(tok, "[") || strings.HasPrefix(tok, "{") {
		return true
	}
	lt := strings.ToLower(strings.Trim(tok, "()[]{}-"))
	if qualityTokens[lt] || partRe.MatchString(lt) {
		return true
	}
	// Group suffix glued to a codec: "x264-GROUP", "H.265-GROUP".
	if before, _, ok := strings.Cut(lt, "-"); ok && qualityTokens[before] {
		return true
	}
	return channelsRe.MatchString(lt)
}

func yearOf(s string) (int, bool) {
	if len(s) != 4 {
		return 0, false
	}
	y, err := strconv.Atoi(s)
	if err != nil || y < 1900 || y > 2099 {
		return 0, false
	}
	return y, true
}

// cleanTitle collapses white space and trims separators left over at the ends.
func cleanTitle(s string) string {
	s = spacesRe.ReplaceAllString(s, " ")
	return strings.Trim(s, " -–,:;")
}

var articles = []string{"the ", "a ", "an ", "der ", "die ", "das ", "ein ", "eine "}

// SortTitle lower-cases title and drops a leading English or German article.
func SortTitle(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, a := range articles {
		if strings.HasPrefix(t, a) && len(t) > len(a) {
			return strings.TrimSpace(t[len(a):])
		}
	}
	return t
}
