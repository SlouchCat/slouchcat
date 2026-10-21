// Package version parses and compares server versions as semantic versions (semver 2.0.0): the
// restore refuses a backup of a newer server (A-4) and the update check compares the running
// version with the release manifest (A-5). A version is "X.Y.Z" or "vX.Y.Z" with an optional
// pre-release ("-rc.1") and build metadata ("+abc", ignored when comparing). What `git describe
// --dirty` appends to a tag ("-3-gabc1234", "-dirty") is build metadata as well: a self-built
// image of a release compares equal to it, so its backups restore and it sees no update to its
// own release. Development builds ("dev", a bare commit hash) do not parse; callers decide what
// that means.
package version

import (
	"regexp"
	"strconv"
	"strings"
)

// describeSuffix is the part `git describe --dirty` adds to the tag: the commits since the tag
// with the abbreviated hash, and -dirty for local changes.
var describeSuffix = regexp.MustCompile(`(?:-[0-9]+-g[0-9a-f]{4,40})?(?:-dirty)?$`)

// Semver is a parsed version.
type Semver struct {
	Major, Minor, Patch uint64
	Pre                 []string // pre-release identifiers; empty for a release
}

// Parse reads a version; ok is false when s is not a semantic version.
func Parse(s string) (Semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		if !validIdentifiers(s[i+1:], false) {
			return Semver{}, false
		}
		s = s[:i]
	}
	s = s[:describeSuffix.FindStringIndex(s)[0]]
	var pre []string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		if !validIdentifiers(s[i+1:], true) {
			return Semver{}, false
		}
		pre = strings.Split(s[i+1:], ".")
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Semver{}, false
	}
	var nums [3]uint64
	for i, p := range parts {
		n, ok := numeric(p)
		if !ok {
			return Semver{}, false
		}
		nums[i] = n
	}
	return Semver{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre}, true
}

// Compare returns -1, 0 or 1 as a is older than, equal to or newer than b; ok is false when either
// does not parse.
func Compare(a, b string) (c int, ok bool) {
	va, okA := Parse(a)
	vb, okB := Parse(b)
	if !okA || !okB {
		return 0, false
	}
	return va.Compare(vb), true
}

// Compare returns -1, 0 or 1 as v is older than, equal to or newer than o (semver precedence: a
// pre-release is older than its release).
func (v Semver) Compare(o Semver) int {
	for _, d := range [3][2]uint64{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch}} {
		if d[0] != d[1] {
			return cmp(d[0] < d[1])
		}
	}
	switch {
	case len(v.Pre) == 0 && len(o.Pre) == 0:
		return 0
	case len(v.Pre) == 0:
		return 1
	case len(o.Pre) == 0:
		return -1
	}
	for i := range min(len(v.Pre), len(o.Pre)) {
		a, b := v.Pre[i], o.Pre[i]
		if a == b {
			continue
		}
		na, aNum := numeric(a)
		nb, bNum := numeric(b)
		switch {
		case aNum && bNum:
			return cmp(na < nb)
		case aNum: // numeric identifiers sort before alphanumeric ones
			return -1
		case bNum:
			return 1
		default:
			return cmp(a < b)
		}
	}
	switch {
	case len(v.Pre) < len(o.Pre):
		return -1
	case len(v.Pre) > len(o.Pre):
		return 1
	}
	return 0
}

func cmp(less bool) int {
	if less {
		return -1
	}
	return 1
}

// numeric parses a numeric identifier without leading zeros.
func numeric(s string) (uint64, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

// validIdentifiers checks dot-separated identifiers of [0-9A-Za-z-]; pre-release numbers must not
// have leading zeros.
func validIdentifiers(s string, pre bool) bool {
	for _, id := range strings.Split(s, ".") {
		if id == "" {
			return false
		}
		digits := true
		for _, r := range id {
			switch {
			case r >= '0' && r <= '9':
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '-':
				digits = false
			default:
				return false
			}
		}
		if pre && digits && len(id) > 1 && id[0] == '0' {
			return false
		}
	}
	return true
}
