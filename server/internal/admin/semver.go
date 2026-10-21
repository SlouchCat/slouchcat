package admin

import (
	"strconv"
	"strings"
)

// Semantic versions of the update check (A-5, design 3.5): "X.Y.Z" or "vX.Y.Z" with an optional
// pre-release ("-rc.1") and build metadata ("+abc1234", ignored when comparing).

type semver struct {
	major, minor, patch int
	pre                 []string // pre-release identifiers; empty = a release
}

// parseSemver parses v; ok is false for anything else, such as "dev".
func parseSemver(v string) (semver, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "+")
	core, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, ok := numericID(p)
		if !ok {
			return semver{}, false
		}
		nums[i] = n
	}
	out := semver{major: nums[0], minor: nums[1], patch: nums[2]}
	if hasPre {
		out.pre = strings.Split(pre, ".")
		for _, id := range out.pre {
			if id == "" || strings.Trim(id, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-") != "" {
				return semver{}, false
			}
		}
	}
	return out, true
}

// numericID parses a numeric identifier without a leading zero.
func numericID(s string) (int, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// compare returns -1, 0 or 1 by semver precedence.
func (a semver) compare(b semver) int {
	for _, d := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if d[0] != d[1] {
			return cmpInt(d[0], d[1])
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1 // a release ranks above its pre-releases
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, xNum := numericID(x)
		yn, yNum := numericID(y)
		switch {
		case xNum && yNum:
			if xn != yn {
				return cmpInt(xn, yn)
			}
		case xNum:
			return -1 // numeric identifiers rank below alphanumeric ones
		case yNum:
			return 1
		case x != y:
			return strings.Compare(x, y)
		}
	}
	return cmpInt(len(a.pre), len(b.pre))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// newerVersion reports whether latest is newer than current. A current version that does not parse
// (a development build "dev") counts as older than every release; an unparsable latest is never
// newer.
func newerVersion(latest, current string) bool {
	l, ok := parseSemver(latest)
	if !ok {
		return false
	}
	c, ok := parseSemver(current)
	return !ok || l.compare(c) > 0
}
