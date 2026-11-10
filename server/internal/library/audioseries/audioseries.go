// Package audioseries normalises the series names and authors of audiobooks (concept 19 stage D2,
// D4.1): the series key that groups the books of a library into one series and the displayed
// spelling of a series name without its suffix. The scanner, the editor and «Reihe bearbeiten»
// compute keys through library.RelinkAudioSeries; the visibility layer names a series from the
// spellings of the volumes a profile may see. Both use this package, so a key and a name always
// follow the same list of suffixes (suffixes.txt, embedded). It knows nothing else of the server.
package audioseries

import (
	_ "embed"
	"slices"
	"strings"
	"sync"
	"unicode"
)

//go:embed suffixes.txt
var suffixList string

// Names normalises series names with one list of suffixes. The zero value strips no suffix.
type Names struct {
	trailing [][]string // whole words at the end of a name
	cutFrom  [][]string // words from whose first occurrence on the name is cut («gesprochen von …»)
}

// Default returns the Names of the embedded list.
var Default = sync.OnceValue(func() Names { return Parse(suffixList) })

// Parse reads a list in the format of suffixes.txt.
func Parse(list string) Names {
	var n Names
	for line := range strings.Lines(list) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cut := false
		for _, ellipsis := range []string{"…", "..."} {
			if s, ok := strings.CutSuffix(line, ellipsis); ok {
				line, cut = s, true
			}
		}
		words := make([]string, 0, 2)
		for _, w := range splitWords(line) {
			words = append(words, w.fold)
		}
		switch {
		case len(words) == 0:
		case cut:
			n.cutFrom = append(n.cutFrom, words)
		default:
			n.trailing = append(n.trailing, words)
		}
	}
	return n
}

// separator reports whether r separates the words of a series name: white space, hyphens and
// dashes, brackets and the dot, comma and colon that end a word.
func separator(r rune) bool {
	if unicode.IsSpace(r) || unicode.Is(unicode.Pd, r) {
		return true
	}
	return strings.ContainsRune("()[]{}.,:;/|", r)
}

// word is a word of a name: its case-folded form and its byte offset in the name.
type word struct {
	fold  string
	start int
}

func splitWords(s string) []word {
	var out []word
	start := -1
	for i, r := range s {
		switch {
		case separator(r) && start >= 0:
			out = append(out, word{fold: strings.ToLower(s[start:i]), start: start})
			start = -1
		case !separator(r) && start < 0:
			start = i
		}
	}
	if start >= 0 {
		out = append(out, word{fold: strings.ToLower(s[start:]), start: start})
	}
	return out
}

// strip returns the words of a name without its suffixes, repeatedly («Reihe - Band» goes
// entirely). A name that would lose every word keeps the words it had before that step.
func (n Names) strip(words []word) []word {
	for {
		before := len(words)
		for _, p := range n.cutFrom {
			for i := 1; i+len(p) <= len(words); i++ { // never at the first word: the name stays
				if foldsEqual(words[i:i+len(p)], p) {
					words = words[:i]
					break
				}
			}
		}
		for _, p := range n.trailing {
			if k := len(words) - len(p); k > 0 && foldsEqual(words[k:], p) {
				words = words[:k]
			}
		}
		if len(words) == before {
			return words
		}
	}
}

func foldsEqual(words []word, p []string) bool {
	for i, w := range words {
		if w.fold != p[i] {
			return false
		}
	}
	return true
}

// articles are the leading articles the key leaves out, only as a whole first word.
var articles = []string{"der", "die", "das", "ein", "eine", "the", "a", "an", "le", "la", "les"}

// Name returns the key part of a series name: case-folded, white space collapsed, hyphens,
// dashes and brackets as spaces, the suffixes and a leading article left out (a name that is
// nothing but an article keeps it). "" for a name without a word.
func (n Names) Name(series string) string {
	words := n.strip(splitWords(series))
	if len(words) > 1 && slices.Contains(articles, words[0].fold) {
		words = words[1:]
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, w.fold)
	}
	return strings.Join(out, " ")
}

// Display returns the spelling of a series name without its suffixes, in the original
// capitalisation and with its white space collapsed: «Bodyguard - Band» is «Bodyguard». A name
// that is nothing but a suffix stays as it is.
func (n Names) Display(series string) string {
	all := splitWords(series)
	words := n.strip(all)
	if len(words) == 0 {
		return strings.Join(strings.Fields(series), " ")
	}
	s := series
	if len(words) < len(all) {
		s = series[:all[len(words)].start]
	}
	s = strings.TrimRightFunc(s, separator)
	return strings.Join(strings.Fields(s), " ")
}

// Author returns the normalised first author of an author field: the part before the first «,»,
// «;», «&», « und », « and » or « / », case-folded, dots and hyphens as spaces, white space
// collapsed. So «J.R.R. Tolkien», «J R R Tolkien» and «J. R. R. Tolkien» are one author, and so
// are «Marc-Uwe Kling» and «Marc Uwe Kling».
func Author(author string) string {
	a := strings.ToLower(author)
	for _, sep := range []string{",", ";", "&", " und ", " and ", " / "} {
		if i := strings.Index(a, sep); i >= 0 {
			a = a[:i]
		}
	}
	a = strings.Map(func(r rune) rune {
		if r == '.' || unicode.Is(unicode.Pd, r) {
			return ' '
		}
		return r
	}, a)
	return strings.Join(strings.Fields(a), " ")
}

// keySeparator joins the name and the author of a key; it appears in neither.
const keySeparator = "\x1f"

// Key returns the series key of a book (D4.1): Name of the series plus the Author, per library.
// "" for a book without series name.
func (n Names) Key(series, author string) string {
	name := n.Name(series)
	if name == "" {
		return ""
	}
	return name + keySeparator + Author(author)
}

// MostCommon returns the value that occurs most often among values ("" for none); a tie goes to
// the value that sorts first, so the choice does not depend on the order of the rows.
func MostCommon(values []string) string {
	counts := map[string]int{}
	best := ""
	for _, v := range values {
		counts[v]++
	}
	for v, c := range counts {
		if v != "" && (best == "" || c > counts[best] || (c == counts[best] && v < best)) {
			best = v
		}
	}
	return best
}
