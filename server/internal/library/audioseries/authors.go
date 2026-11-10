package audioseries

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Unified author and series names (concept 19 stage D3, D6.4 «Vereinheitlichen»). Two switches on
// the page «Hörbücher: Quellen», on by default: the author names (items.author_display, computed
// by library.RecomputeAuthorDisplay) and the series names (audio_series.name and the name the
// visibility layer shows; the key strips the suffixes always).
const (
	SettingNormalizeAuthors = "metadata.books.normalize_authors"
	SettingNormalizeSeries  = "metadata.books.normalize_series"
)

// SwitchOn reports whether a switch of «Vereinheitlichen» is on: an absent setting (found false)
// is on, and so is every value but "0" and "false".
func SwitchOn(value string, found bool) bool {
	if !found {
		return true
	}
	v := strings.TrimSpace(value)
	return v != "0" && !strings.EqualFold(v, "false")
}

// Shown returns the Names that give the displayed spelling of series names: these Names when the
// series switch is on, else Names without suffixes (the most common spelling keeps «- Band»).
// Keys always use the Names with suffixes.
func (n Names) Shown(normalize bool) Names {
	if normalize {
		return n
	}
	return Names{version: n.version}
}

// authorSeparators split an author field into names, like Author takes the first one; matched
// without regard to case.
var authorSeparators = []string{",", ";", "&", " und ", " and ", " / "}

// AuthorPart is one piece of an author field: a name with the white space around it (Lead,
// Trail) and the separator that follows it (Sep, "" for the last).
type AuthorPart struct {
	Lead, Name, Trail, Sep string
}

// SplitAuthors splits an author field (composed to NFC) into its names at the separators of
// Author («,», «;», «&», « und », « and », « / »), so that joining Lead+Name+Trail+Sep of every
// part gives the field back. The first part's AuthorKey is Author of the field. A name may be
// empty («A,,B»).
func SplitAuthors(field string) []AuthorPart {
	s := norm.NFC.String(field)
	var parts []AuthorPart
	for {
		at, sep := -1, ""
		for _, candidate := range authorSeparators {
			if i := indexFold(s, candidate); i >= 0 && (at < 0 || i < at) {
				at, sep = i, s[i:i+len(candidate)]
			}
		}
		piece := s
		if at >= 0 {
			piece = s[:at]
		}
		name := strings.TrimSpace(piece)
		lead := piece[:strings.Index(piece, name)]
		parts = append(parts, AuthorPart{Lead: lead, Name: name, Trail: piece[len(lead)+len(name):], Sep: sep})
		if at < 0 {
			return parts
		}
		s = s[at+len(sep):]
	}
}

// indexFold is strings.Index for an ASCII needle, ignoring the case of ASCII letters.
func indexFold(s, needle string) int {
	for i := 0; i+len(needle) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

// AuthorKey returns the key of one author name: case-folded, dots and hyphens as spaces, white
// space collapsed (Author without the split).
func AuthorKey(name string) string {
	a := strings.Map(func(r rune) rune {
		if r == '.' || unicode.Is(unicode.Pd, r) {
			return ' '
		}
		return r
	}, strings.ToLower(norm.NFC.String(name)))
	return strings.Join(strings.Fields(a), " ")
}

// FormatInitials writes the initials of an author name with a dot and a space: «J R R Tolkien»,
// «J.R.R. Tolkien» and «J. R. R. Tolkien» are «J. R. R. Tolkien», «James S.A. Corey» is «James S.
// A. Corey». An initial is a word of single letters each followed by a dot («J.R.R.», «S.A.»), or
// a single capital letter that is not the last word («J R R Tolkien»; «Malcolm X» keeps its «X»,
// «Ortega y Gasset» its «y»). Every other word stays as it is (hyphens, «Dr.», «St.Exupéry»); white
// space is collapsed.
func FormatInitials(name string) string {
	words := strings.Fields(norm.NFC.String(name))
	out := make([]string, 0, len(words))
	for i, w := range words {
		switch letters, ok := dottedInitials(w); {
		case ok:
			for _, l := range letters {
				out = append(out, l+".")
			}
		case i < len(words)-1 && utf8.RuneCountInString(w) == 1 && unicode.IsUpper([]rune(w)[0]):
			out = append(out, w+".")
		default:
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// dottedInitials splits a word of single letters each followed by a dot («J.R.R.»; the last dot
// may miss: «J.R.R») into its letters. A word without a dot or with a longer piece is no initial.
func dottedInitials(w string) ([]string, bool) {
	if !strings.Contains(w, ".") {
		return nil, false
	}
	pieces := strings.Split(strings.TrimSuffix(w, "."), ".")
	for _, p := range pieces {
		r, size := utf8.DecodeRuneInString(p)
		if size == 0 || size != len(p) || !unicode.IsLetter(r) {
			return nil, false
		}
	}
	return pieces, true
}
