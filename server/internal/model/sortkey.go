package model

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Sort keys of the shown title per language (concept 08, ADR 0016, item_texts.sort_key). The
// library page sorts by this key and derives its letter bar from its first character;
// items.sort_title stays the key of the file name for the scanner, seasons and episodes.

// sortArticles are the articles stripped from the front of a title, per language.
var sortArticles = map[string][]string{
	"de": {"der ", "die ", "das ", "eine ", "ein "},
	"en": {"the ", "an ", "a "},
}

// SortKey returns the sort key of title in lang: white space collapsed, the article of the
// language stripped (der, die, das, ein, eine; the, a, an), lower-cased, diacritics folded (Ä
// to a, É to e, ß to ss). A title that is nothing but an article keeps it. An unknown language
// strips no article.
func SortKey(lang, title string) string {
	t := strings.Join(strings.Fields(title), " ")
	t = strings.ToLower(foldDiacritics(t))
	for _, a := range sortArticles[lang] {
		if strings.HasPrefix(t, a) && len(t) > len(a) {
			t = strings.TrimSpace(t[len(a):])
			break
		}
	}
	return t
}

// SortLetter returns the letter of the letter bar for a sort key: its first character upper-cased
// when it is an ASCII letter, else "#" (digits, punctuation, other scripts, empty).
func SortLetter(sortKey string) string {
	if sortKey == "" {
		return "#"
	}
	c := sortKey[0]
	switch {
	case c >= 'a' && c <= 'z':
		return string(c - 'a' + 'A')
	case c >= 'A' && c <= 'Z':
		return string(c)
	}
	return "#"
}

// foldDiacritics decomposes s (NFD), drops the combining marks and maps the letters without a
// decomposition (ß, ø, æ, œ, ł, đ) by hand.
func foldDiacritics(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		switch r {
		case 'ß':
			b.WriteString("ss")
		case 'ø':
			b.WriteByte('o')
		case 'Ø':
			b.WriteByte('O')
		case 'æ':
			b.WriteString("ae")
		case 'Æ':
			b.WriteString("AE")
		case 'œ':
			b.WriteString("oe")
		case 'Œ':
			b.WriteString("OE")
		case 'ł':
			b.WriteByte('l')
		case 'Ł':
			b.WriteByte('L')
		case 'đ':
			b.WriteByte('d')
		case 'Đ':
			b.WriteByte('D')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
