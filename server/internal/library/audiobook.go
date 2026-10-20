package library

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Audiobooks (M6 design 3.1): pure logic of the audiobook scanner without database access. Files
// group into books per directory: every .m4b file is a book of its own; all other audio files of a
// directory form one book whose parts play in order (track tag, then the file name compared with
// numbers as numbers). Audio files directly in the root are books of their own. The metadata of a
// book comes from Audiobookshelf's metadata.json in the book folder, then the tags of the first
// part, then the folder names Author/[Series/][NN - ]Title.

var audioExts = map[string]bool{"m4b": true, "m4a": true, "mp3": true, "flac": true, "ogg": true, "opus": true}

// IsAudio reports whether name has one of the audio extensions the audiobook scanner picks up.
func IsAudio(name string) bool {
	return audioExts[extOf(name)]
}

// audioFolderKey names the book of an audio file in a root: the directory for the parts of a
// folder book, the file itself for an .m4b file and for a file directly in the root.
func audioFolderKey(rootID int64, rel string) string {
	dir := path.Dir(rel)
	if dir == "." || extOf(rel) == "m4b" {
		return titleFolderKey(rootID, rel)
	}
	return titleFolderKey(rootID, dir)
}

// bookMeta is the metadata of an audiobook from one source; empty fields are unknown.
type bookMeta struct {
	title, subtitle  string
	author, narrator string
	series           string
	volume           *float64
	year             int
	description      string
	lang             string // de or en (metadata.json), empty when unknown
	genres           []string
	explicit         bool
	cover            string // rel path of the folder image below the root
}

// mergeBook combines sources field by field; the first source with a value wins.
func mergeBook(layers ...bookMeta) bookMeta {
	var m bookMeta
	first := func(get func(bookMeta) string) string {
		for _, l := range layers {
			if v := get(l); v != "" {
				return v
			}
		}
		return ""
	}
	m.title = first(func(l bookMeta) string { return l.title })
	m.subtitle = first(func(l bookMeta) string { return l.subtitle })
	m.author = first(func(l bookMeta) string { return l.author })
	m.narrator = first(func(l bookMeta) string { return l.narrator })
	m.series = first(func(l bookMeta) string { return l.series })
	m.description = first(func(l bookMeta) string { return l.description })
	m.lang = first(func(l bookMeta) string { return l.lang })
	m.cover = first(func(l bookMeta) string { return l.cover })
	for _, l := range layers {
		if m.volume == nil && l.volume != nil {
			m.volume = l.volume
		}
		if m.year == 0 {
			m.year = l.year
		}
		if len(m.genres) == 0 {
			m.genres = l.genres
		}
		m.explicit = m.explicit || l.explicit
	}
	if m.lang == "" {
		m.lang = "de"
	}
	return m
}

// volumePrefix is the volume of a book in its folder name: "05 - Title", "1.5 - Title".
var volumePrefix = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*-\s*(.+)$`)

// folderMeta reads title, volume, author and series from the path of a book (its folder, or the
// file without extension for a book that does not own its folder): Author/Series/[NN - ]Title,
// Author/[NN - ]Title, "Author - Title" or only the title.
func folderMeta(bookPath string) bookMeta {
	comps := strings.Split(bookPath, "/")
	last := strings.TrimSpace(comps[len(comps)-1])
	var m bookMeta
	if v := volumePrefix.FindStringSubmatch(last); v != nil {
		if f, err := strconv.ParseFloat(v[1], 64); err == nil {
			m.volume = &f
		}
		last = strings.TrimSpace(v[2])
	}
	m.title = last
	switch {
	case len(comps) >= 3:
		m.author, m.series = comps[len(comps)-3], comps[len(comps)-2]
	case len(comps) == 2:
		m.author = comps[0]
	case m.volume == nil:
		if a, t, ok := strings.Cut(last, " - "); ok && strings.TrimSpace(a) != "" && strings.TrimSpace(t) != "" {
			m.author, m.title = strings.TrimSpace(a), strings.TrimSpace(t)
		}
	}
	return m
}

// tagMeta reads the container tags of the first part (transcode.Format.Tags). In a book of several
// parts the title tag names the part, so only the album names the book.
func tagMeta(tags map[string]string, multi bool) bookMeta {
	firstTag := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(tags[k]); v != "" {
				return v
			}
		}
		return ""
	}
	m := bookMeta{
		author:      firstTag("artist", "album_artist"),
		narrator:    firstTag("composer"),
		series:      firstTag("show", "series", "mvnm"),
		description: firstTag("description", "comment"),
	}
	if multi {
		m.title = firstTag("album")
	} else {
		m.title = firstTag("title", "album")
	}
	if v := firstTag("episode_id", "series-part", "mvin"); v != "" {
		if f, err := strconv.ParseFloat(strings.TrimPrefix(v, "#"), 64); err == nil && f >= 0 {
			m.volume = &f
		}
	}
	m.year = yearIn(firstTag("date", "year"))
	if g := firstTag("genre"); g != "" {
		m.genres = []string{g}
	}
	return m
}

// yearIn returns the year at the start of s ("2024", "2024-05-01"), 0 when there is none.
func yearIn(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil || y < 1000 || y > 9999 {
		return 0
	}
	return y
}

// absMeta is Audiobookshelf's metadata.json: the current flat schema and the older one with the
// fields below "metadata". Lists hold strings or objects with a name (and a sequence for series).
type absMeta struct {
	Title         string            `json:"title"`
	Subtitle      string            `json:"subtitle"`
	Authors       []json.RawMessage `json:"authors"`
	Narrators     []json.RawMessage `json:"narrators"`
	Series        []json.RawMessage `json:"series"`
	Description   string            `json:"description"`
	PublishedYear json.RawMessage   `json:"publishedYear"`
	Genres        []json.RawMessage `json:"genres"`
	Language      string            `json:"language"`
	Explicit      json.RawMessage   `json:"explicit"`
	Metadata      *absMeta          `json:"metadata"`
}

// maxMetadataJSON caps the metadata.json that is read.
const maxMetadataJSON = 1 << 20

// jsonMeta parses metadata.json; ok is false for a file that does not parse.
func jsonMeta(data []byte) (bookMeta, bool) {
	var raw absMeta
	if err := json.Unmarshal(data, &raw); err != nil {
		return bookMeta{}, false
	}
	if raw.Title == "" && raw.Metadata != nil {
		raw = *raw.Metadata
	}
	m := bookMeta{
		title: strings.TrimSpace(raw.Title), subtitle: strings.TrimSpace(raw.Subtitle),
		description: strings.TrimSpace(raw.Description), lang: language(raw.Language),
		author: joinNames(raw.Authors), narrator: joinNames(raw.Narrators),
	}
	for _, g := range raw.Genres {
		if n, _ := entryName(g); n != "" {
			m.genres = append(m.genres, n)
		}
	}
	if len(raw.Series) > 0 {
		name, seq := entryName(raw.Series[0])
		if i := strings.LastIndex(name, " #"); i >= 0 && seq == "" {
			name, seq = name[:i], name[i+2:]
		}
		m.series = strings.TrimSpace(name)
		if f, err := strconv.ParseFloat(strings.TrimSpace(seq), 64); err == nil && f >= 0 {
			m.volume = &f
		}
	}
	var year string
	if err := json.Unmarshal(raw.PublishedYear, &year); err != nil {
		year = string(raw.PublishedYear)
	}
	m.year = yearIn(year)
	var explicit any
	if json.Unmarshal(raw.Explicit, &explicit) == nil {
		switch v := explicit.(type) {
		case bool:
			m.explicit = v
		case string:
			m.explicit = strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
		}
	}
	return m, true
}

// entryName returns the name (and series sequence) of a metadata.json list entry: a string, or an
// object with name and sequence.
func entryName(raw json.RawMessage) (name, sequence string) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s), ""
	}
	var o struct {
		Name     string          `json:"name"`
		Sequence json.RawMessage `json:"sequence"`
	}
	if json.Unmarshal(raw, &o) != nil {
		return "", ""
	}
	var seq string
	if json.Unmarshal(o.Sequence, &seq) != nil && len(o.Sequence) > 0 && string(o.Sequence) != "null" {
		seq = string(o.Sequence)
	}
	return strings.TrimSpace(o.Name), seq
}

func joinNames(list []json.RawMessage) string {
	var names []string
	for _, e := range list {
		if n, _ := entryName(e); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}

// language maps the language of metadata.json to the text languages (de, en); "" for any other.
func language(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "de", "deu", "ger", "german", "deutsch":
		return "de"
	case "en", "eng", "english", "englisch":
		return "en"
	}
	return ""
}

// coverNames are the folder images of a book, in order of preference.
var coverNames = []string{"cover.jpg", "cover.jpeg", "cover.png", "folder.jpg", "folder.jpeg", "folder.png",
	"poster.jpg", "poster.jpeg", "poster.png"}

// bookFiles reads what the folder of a book holds besides its parts. A book owns its folder when
// it is made of the non-.m4b audio files of a directory below the root, or is the only audio file
// there; then metadata.json (meta) and the folder image (folder.cover) count, and the folder names
// are read from the directory, else from the file name. rel is the path of a part. Unreadable
// files count as absent.
func bookFiles(r *os.Root, rel string) (meta, folder bookMeta) {
	dir := path.Dir(rel)
	owns := false
	var names []string
	if dir != "." {
		if entries, err := fs.ReadDir(r.FS(), dir); err == nil {
			audio := 0
			for _, e := range entries {
				if e.IsDir() || Ignored(e.Name()) {
					continue
				}
				names = append(names, e.Name())
				if IsAudio(e.Name()) {
					audio++
				}
			}
			owns = extOf(rel) != "m4b" || audio == 1
		}
	}
	bookPath := strings.TrimSuffix(rel, path.Ext(rel))
	if owns {
		bookPath = dir
	}
	folder = folderMeta(bookPath)
	if !owns {
		return bookMeta{}, folder
	}
	for _, want := range coverNames {
		if i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, want) }); i >= 0 {
			folder.cover = path.Join(dir, names[i])
			break
		}
	}
	if i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, "metadata.json") }); i >= 0 {
		name := path.Join(dir, names[i])
		if fi, err := r.Stat(name); err == nil && fi.Mode().IsRegular() && fi.Size() <= maxMetadataJSON {
			if data, err := r.ReadFile(name); err == nil {
				if m, ok := jsonMeta(data); ok {
					return m, folder
				}
			}
		}
	}
	return bookMeta{}, folder
}

// bookPart is a present part of a book with its probe (nil when it is not probed or the probe
// failed).
type bookPart struct {
	id         string
	rel        string
	durationMs int64
	probe      *transcode.Probe
}

// orderParts sorts the parts of a book: by disc and track tag when every part has a distinct
// track number, else by file name with numbers compared as numbers.
func orderParts(parts []bookPart) {
	type pos struct{ disc, track int }
	positions := map[string]pos{}
	seen := map[pos]bool{}
	byTrack := true
	for _, p := range parts {
		if p.probe == nil {
			byTrack = false
			break
		}
		ps := pos{disc: leadingInt(p.probe.Format.Tags["disc"]), track: leadingInt(p.probe.Format.Tags["track"])}
		if ps.track <= 0 || seen[ps] {
			byTrack = false
			break
		}
		seen[ps] = true
		positions[p.id] = ps
	}
	slices.SortStableFunc(parts, func(a, b bookPart) int {
		if byTrack {
			pa, pb := positions[a.id], positions[b.id]
			if c := cmp.Or(cmp.Compare(pa.disc, pb.disc), cmp.Compare(pa.track, pb.track)); c != 0 {
				return c
			}
		}
		return cmp.Or(naturalCompare(a.rel, b.rel), strings.Compare(a.id, b.id))
	})
}

// leadingInt parses the number at the start of a tag such as "3/10"; 0 when there is none.
func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0
	}
	return n
}

// naturalCompare compares two paths case-insensitively with runs of digits compared as numbers,
// so "2 - B.mp3" sorts before "10 - A.mp3".
func naturalCompare(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		da, db := digitRun(a), digitRun(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if c := cmp.Or(cmp.Compare(len(na), len(nb)), strings.Compare(na, nb)); c != 0 {
				return c
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return cmp.Compare(a[0], b[0])
		}
		a, b = a[1:], b[1:]
	}
	return cmp.Compare(len(a), len(b))
}

func digitRun(s string) int {
	n := 0
	for n < len(s) && unicode.IsDigit(rune(s[n])) {
		n++
	}
	return n
}

// chapterRow is one row of the chapters table.
type chapterRow struct {
	title       string
	startMs     int64
	endMs       int64
	mediaFileID string
	fileStartMs int64
}

// partNumberPrefix is the number before the title in a part's file name ("01 - Kapitel 1").
var partNumberPrefix = regexp.MustCompile(`^\d+\s*[-._]?\s*`)

// bookChapters computes the book-wide chapters of the ordered, probed parts of a book: the chapter
// marks of a part with at least two (an empty title becomes "Kapitel n"); one chapter for a part
// without marks, titled by its title tag, else its file name without number prefix and extension,
// else "Teil n"; one chapter over a book of one file without marks, titled with the book title.
// Starts count from the start of the book (the durations of the parts before).
func bookChapters(parts []bookPart, bookTitle string) []chapterRow {
	var rows []chapterRow
	var offset int64
	for i, p := range parts {
		dur := p.durationMs
		if p.probe != nil && len(p.probe.Chapters) >= 2 {
			for _, c := range p.probe.Chapters {
				start := min(max(c.Start.Milliseconds(), 0), dur)
				end := min(max(c.End.Milliseconds(), start), dur)
				title := strings.TrimSpace(c.Title)
				if title == "" {
					title = "Kapitel " + strconv.Itoa(len(rows)+1)
				}
				rows = append(rows, chapterRow{title: title, startMs: offset + start, endMs: offset + end, mediaFileID: p.id, fileStartMs: start})
			}
		} else {
			title := bookTitle
			if len(parts) > 1 || title == "" {
				title = partTitle(p, i)
			}
			rows = append(rows, chapterRow{title: title, startMs: offset, endMs: offset + dur, mediaFileID: p.id})
		}
		offset += dur
	}
	return rows
}

// partTitle names the chapter of a part without chapter marks.
func partTitle(p bookPart, i int) string {
	if p.probe != nil {
		if t := strings.TrimSpace(p.probe.Format.Tags["title"]); t != "" {
			return t
		}
	}
	stem := strings.TrimSuffix(path.Base(p.rel), path.Ext(p.rel))
	if t := strings.TrimSpace(partNumberPrefix.ReplaceAllString(stem, "")); t != "" {
		return t
	}
	return fmt.Sprintf("Teil %d", i+1)
}
