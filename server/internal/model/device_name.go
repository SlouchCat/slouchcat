package model

import (
	"strings"
	"unicode"
)

// CleanDeviceName returns a device name as the server keeps it (concept 27, security review L-4 of
// stage 1): control characters (Cc) and format characters (Cf: bidi overrides and isolates,
// zero-width characters, the soft hyphen) are removed, the rest is trimmed. A name is the device's
// own claim and shows in «Meine Geräte», the administration and the message device_linked, so it
// must not reorder or hide its text. limit > 0 cuts the result to that many characters.
func CleanDeviceName(name string, limit int) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, name)
	clean = strings.TrimSpace(clean)
	if r := []rune(clean); limit > 0 && len(r) > limit {
		clean = strings.TrimSpace(string(r[:limit]))
	}
	return clean
}
