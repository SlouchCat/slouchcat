package api

import (
	"strconv"
	"strings"
)

// acceptsWebP reports whether the Accept header values list image/webp with a non-zero quality.
// Wildcards (image/*, */*) do not count: only browsers that name WebP get it.
func acceptsWebP(values []string) bool {
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			mediaType, params, _ := strings.Cut(part, ";")
			if !strings.EqualFold(strings.TrimSpace(mediaType), "image/webp") {
				continue
			}
			for p := range strings.SplitSeq(params, ";") {
				k, val, _ := strings.Cut(p, "=")
				if strings.EqualFold(strings.TrimSpace(k), "q") {
					if q, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil && q <= 0 {
						return false
					}
				}
			}
			return true
		}
	}
	return false
}
