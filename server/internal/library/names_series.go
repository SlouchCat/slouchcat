package library

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// ParsedEpisode is what ParseSeriesPath reads about the series tree from a path: the series, and
// for an episode its numbers and title. For an extra only the series fields, SeriesDir,
// SeasonDir and Season (when SeasonDir is set) are meaningful.
type ParsedEpisode struct {
	SeriesTitle string
	SeriesYear  *int
	TVDBID      int64
	TMDBID      int64
	IMDBID      string
	Season      int    // 0 = specials; the year for a daily show
	Episode     int    // MMDD for a daily show
	EpisodeEnd  int    // last episode of a file with several episodes, 0 for a single episode
	Title       string // episode title from the name, "" when the name has none
	Date        string // YYYY-MM-DD of a daily show, "" otherwise
	SeriesDir   string // series folder relative to the root, "" for a file directly in the root
	SeasonDir   string // season folder relative to the root, "" when there is none
}

var (
	// Season folders: "Season 01", "Staffel 2", "Saison 3", "Season 2024" (daily shows), "S01",
	// "Specials" (season 0).
	seasonDirRe  = regexp.MustCompile(`(?i)^(?:season|staffel|saison|temporada|series)[ ._-]*(\d{1,4})(?:[ ._-].*)?$`)
	seasonShort  = regexp.MustCompile(`(?i)^s(\d{1,2})$`)
	specialsDirs = map[string]bool{"specials": true, "special": true, "extras specials": true}

	// Episode markers, tried in this order (design 2.1); group 1 is the marker itself. sxeRe
	// matches "S01E02", "s1e2", "S01 E02", "S01.E02"; episodeTail reads a multi-episode tail.
	sxeRe    = regexp.MustCompile(`(?i)(?:^|[^\pL\pN])(s(\d{1,2})[ ._]?e(\d{1,3}))`)
	wordsRe  = regexp.MustCompile(`(?i)(?:^|[^\pL\pN])(season[ ._-]*(\d{1,2})[ ._-]*episode[ ._-]*(\d{1,3}))(?:[^\pL\pN]|$)`)
	xRe      = regexp.MustCompile(`(?i)(?:^|[^\pL\pN])((\d{1,2})x(\d{2,3})(?:[-x](\d{2,3}))?)(?:[^\pL\pN]|$)`)
	dateRe   = regexp.MustCompile(`(?:^|[^0-9])(((?:19|20)\d{2})[-._ ](\d{2})[-._ ](\d{2}))(?:[^0-9]|$)`)
	eOnlyRe  = regexp.MustCompile(`(?i)(?:^|[^\pL\pN])((?:e|ep[ ._]?|episode[ ._-]?)(\d{2,3}))(?:[^\pL\pN]|$)`)
	tailEpRe = regexp.MustCompile(`(?i)^(?:-?e|-)(\d{1,3})`)
)

// seasonDir returns the season number of a season folder name.
func seasonDir(d string) (int, bool) {
	t := strings.TrimSpace(d)
	if specialsDirs[strings.ToLower(t)] {
		return 0, true
	}
	if m := seasonDirRe.FindStringSubmatch(t); m != nil {
		n, err := strconv.Atoi(m[1])
		return n, err == nil
	}
	if m := seasonShort.FindStringSubmatch(t); m != nil {
		n, err := strconv.Atoi(m[1])
		return n, err == nil
	}
	return 0, false
}

// marker is an episode marker found in a name: the numbers and where it starts and ends.
type marker struct {
	season, episode, end int
	hasSeason            bool
	date                 string
	start, stop          int // byte offsets of the marker in the name
}

// findMarker looks for an episode marker in a file stem or folder name.
func findMarker(s string) (marker, bool) {
	num := func(loc []int, g int) int {
		if loc[2*g] < 0 {
			return 0
		}
		n, _ := strconv.Atoi(s[loc[2*g]:loc[2*g+1]])
		return n
	}
	if loc := sxeRe.FindStringSubmatchIndex(s); loc != nil {
		m := marker{season: num(loc, 2), episode: num(loc, 3), hasSeason: true, start: loc[2]}
		m.end, m.stop = episodeTail(s, loc[3], m.episode)
		return m, true
	}
	if loc := wordsRe.FindStringSubmatchIndex(s); loc != nil {
		return marker{season: num(loc, 2), episode: num(loc, 3), hasSeason: true, start: loc[2], stop: loc[3]}, true
	}
	if loc := xRe.FindStringSubmatchIndex(s); loc != nil {
		m := marker{season: num(loc, 2), episode: num(loc, 3), hasSeason: true, start: loc[2], stop: loc[3]}
		if e := num(loc, 4); e > m.episode {
			m.end = e
		}
		return m, true
	}
	if loc := dateRe.FindStringSubmatchIndex(s); loc != nil {
		mo, d := num(loc, 3), num(loc, 4)
		if mo >= 1 && mo <= 12 && d >= 1 && d <= 31 {
			return marker{
				season: num(loc, 2), episode: mo*100 + d, hasSeason: true,
				date:  s[loc[4]:loc[5]] + "-" + s[loc[6]:loc[7]] + "-" + s[loc[8]:loc[9]],
				start: loc[2], stop: loc[3],
			}, true
		}
	}
	if loc := eOnlyRe.FindStringSubmatchIndex(s); loc != nil {
		return marker{episode: num(loc, 2), start: loc[2], stop: loc[3]}, true
	}
	return marker{}, false
}

// episodeTail reads the rest of a multi-episode marker after "S01E01": "-E02", "E02", "-02"
// (repeated). It returns the last episode (0 when there is none) and the end offset.
func episodeTail(s string, pos, first int) (int, int) {
	end := 0
	for {
		loc := tailEpRe.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			return end, pos
		}
		next := pos + loc[1]
		// "-1080p", "-02x": digits must end the token.
		if next < len(s) && isAlnum(s[next]) {
			return end, pos
		}
		e, _ := strconv.Atoi(s[pos+loc[2] : pos+loc[3]])
		if e <= max(first, end) {
			return end, pos
		}
		end, pos = e, next
	}
}

func isAlnum(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// ParseSeriesPath parses the slash-separated path of a video file in a series library, relative
// to the root: <series folder>/[<season folder>/]<file> (Sonarr, Plex, Jellyfin and scene
// naming, design 2.1).
//
// The series folder is the deepest folder that is no season, extras, sample or release folder (a
// folder whose name holds an episode marker); without one, the series title comes from the file
// name up to the episode marker. Episode markers: S01E02 with multi-episode tails (S01E01-E02,
// S01E01E02, S01E01-02), 1x02, "Season 1 Episode 2", the date form YYYY-MM-DD (season = year,
// episode = MMDD) and E02 alone (season from the season folder, else 1). A file without a marker
// in its name or its release folder is no episode; it is still parsed as an extra when it sits
// in an extras folder or carries an extra suffix. Extras keep the season of their season folder;
// without one they belong to the series.
func ParseSeriesPath(rel string) ParsedName {
	segs := strings.Split(strings.Trim(rel, "/"), "/")
	file := segs[len(segs)-1]
	dirs := segs[:len(segs)-1]
	stem := strings.TrimSuffix(file, path.Ext(file))

	var res ParsedName
	res.Sample = isSampleStem(stem)
	seasonIdx, seriesIdx, season := -1, -1, 0
	var dirMarker *marker
	var markerDir string
	for i := len(dirs) - 1; i >= 0; i-- {
		d := dirs[i]
		if kind, ok := extrasDirs[strings.ToLower(d)]; ok {
			if !res.Extra {
				res.Extra, res.ExtraKind = true, kind
			}
			continue
		}
		if isSampleDir(d) {
			res.Sample = true
			continue
		}
		if seasonIdx < 0 {
			if n, ok := seasonDir(d); ok {
				seasonIdx, season = i, n
				continue
			}
		}
		if m, ok := findMarker(d); ok && seasonIdx < 0 {
			if dirMarker == nil {
				dirMarker, markerDir = &m, d
			}
			continue
		}
		seriesIdx = i
		break
	}
	if kind, _, ok := extraSuffix(stem); ok {
		res.Extra = true
		if res.ExtraKind == "" {
			res.ExtraKind = kind
		}
	}
	if res.Extra {
		res.ExtraTitle = extraTitle(stem)
	}

	ep := &ParsedEpisode{}
	if seasonIdx >= 0 {
		ep.SeasonDir = strings.Join(dirs[:seasonIdx+1], "/")
		ep.Season = season
	}
	if seriesIdx >= 0 {
		ep.SeriesDir = strings.Join(dirs[:seriesIdx+1], "/")
	}
	fileP := parseName(stem)

	m, found := findMarker(stem)
	markerSrc := stem
	if !found && dirMarker != nil {
		m, found, markerSrc = *dirMarker, true, markerDir
	}
	if found && !res.Extra {
		if m.hasSeason {
			ep.Season = m.season
		} else if seasonIdx < 0 {
			ep.Season = 1
		}
		ep.Episode, ep.EpisodeEnd, ep.Date = m.episode, m.end, m.date
		if markerSrc == stem {
			ep.Title = episodeTitle(stem[m.stop:])
		}
	}

	// The series: folder name, else the name before the marker.
	var seriesP nameParts
	switch {
	case seriesIdx >= 0:
		seriesP = parseName(dirs[seriesIdx])
	case found:
		seriesP = parseName(markerSrc[:m.start])
	default:
		if !res.Extra {
			return res // no episode and no series: skipped by the scanner
		}
		seriesP = fileP
	}
	ep.SeriesTitle, ep.SeriesYear = seriesP.title, seriesP.year
	ep.TVDBID, ep.TMDBID, ep.IMDBID = seriesP.tvdb, seriesP.tmdb, seriesP.imdb
	if ep.TVDBID == 0 {
		ep.TVDBID = fileP.tvdb
	}
	if ep.TMDBID == 0 {
		ep.TMDBID = fileP.tmdb
	}
	if ep.IMDBID == "" {
		ep.IMDBID = fileP.imdb
	}
	if ep.SeriesTitle == "" {
		return ParsedName{Sample: res.Sample, Extra: res.Extra, ExtraKind: res.ExtraKind, ExtraTitle: res.ExtraTitle}
	}
	if res.Extra && seriesIdx < 0 {
		// An extra needs a series folder to find its parent.
		return res
	}
	if !found && !res.Extra {
		return res
	}
	res.Series = ep
	res.VersionLabel = strings.TrimSpace(fileP.resolution + " " + fileP.edition)
	return res
}

// episodeTitle reads the episode title after the marker: up to the first quality token, bracket
// or provider id ("- Pilot - 1080p" → "Pilot", ".Pilot.720p.HDTV" → "Pilot").
func episodeTitle(rest string) string {
	rest = editionRe.ReplaceAllString(rest, " ")
	rest = idRe.ReplaceAllString(rest, " ")
	if !strings.Contains(strings.TrimSpace(rest), " ") {
		rest = strings.NewReplacer(".", " ", "_", " ").Replace(rest)
	} else {
		rest = strings.ReplaceAll(rest, "_", " ")
	}
	tokens := strings.Fields(rest)
	end := len(tokens)
	for i, tok := range tokens {
		if isStopToken(tok) || resRe.MatchString(tok) {
			end = i
			break
		}
	}
	return cleanTitle(strings.Join(tokens[:end], " "))
}
