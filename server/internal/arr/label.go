package arr

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Tag labels (appendix I, I-52 and N-113): Radarr accepts only ^[a-z0-9-]+$, Sonarr more. Slouchcat
// forms every label so that both accept it.

// Slug turns s into a label after N-113: lower case, ä → ae, ö → oe, ü → ue, sharp s → ss, other
// letters with marks to the base letter, everything outside a–z and 0–9 becomes "-", runs of "-"
// collapse, no "-" at the edges. The result may be empty (only symbols, emoji, blanks).
func Slug(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	dash := false // a "-" is pending
	emit := func(t string) {
		if dash && b.Len() > 0 {
			b.WriteByte('-')
		}
		dash = false
		b.WriteString(t)
	}
	// The umlauts are spelled out before the decomposition, which would drop their marks.
	for _, r := range norm.NFD.String(umlauts.Replace(strings.ToLower(norm.NFC.String(s)))) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// a mark of the letter before it
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			emit(string(r))
		default:
			if t, ok := baseLetters[r]; ok {
				emit(t)
			} else {
				dash = true
			}
		}
	}
	return b.String()
}

var umlauts = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "ẞ", "ss")

// baseLetters are lower-case letters without a decomposition into base letter and mark.
var baseLetters = map[rune]string{
	'ø': "o", 'æ': "ae", 'œ': "oe", 'ł': "l", 'đ': "d", 'ð': "d", 'þ': "th", 'ħ': "h", 'ı': "i",
}

// ProfileLabel returns prefix + profile name as a label (architecture 20.1): Slug(prefix + name).
// It returns false when the name gives no letter or digit; the caller then uses its fallback
// (<prefix>profil-<id>).
func ProfileLabel(prefix, name string) (string, bool) {
	n := Slug(name)
	if n == "" {
		return "", false
	}
	return Slug(prefix + n), true
}

// ValidLabel checks a label, also one set by hand per profile (F-25): it must already be in the
// form Slug gives, non-empty.
func ValidLabel(label string) error {
	switch {
	case label == "":
		return errors.New("tag label is empty")
	case Slug(label) != label:
		return errors.New("tag label allows only a-z, 0-9 and single inner dashes")
	}
	return nil
}
