package api

import (
	"net/http"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// MaxBodyBytes caps request bodies (JSON endpoints take a few hundred bytes; the largest is the
// setup wizard), so a client cannot make the server buffer or decode megabytes.
const MaxBodyBytes = 64 << 10

// MaxBatchBodyBytes caps the bodies of the offline batch operations, which carry lists the
// contract bounds: up to 500 offline events of about 300 bytes (uploadOfflineEvents), 2000 held
// downloads (syncOffline) or 2000 download ids (checkOfflineRevocations).
const MaxBatchBodyBytes = 1 << 20

// MaxRestoreBodyBytes caps the upload of a backup to restore (restoreBackupUpload, A-4): the
// archive streams to the administration, which unpacks at most as much.
const MaxRestoreBodyBytes = model.MaxBackupBytes + 1<<20 // the archive plus the multipart framing and the passphrase

// MaxImageUploadBodyBytes caps the upload of an image for a title (uploadItemImage, concept 15
// 7.5): the image plus the multipart framing; the handler refuses an image above
// model.MaxImageUploadBytes itself.
const MaxImageUploadBodyBytes = model.MaxImageUploadBytes + 1<<20

// largeBodies are the routes ("METHOD path") with a larger limit than MaxBodyBytes. A path
// segment in braces ("{id}") is a parameter and matches any one non-empty segment.
var largeBodies = map[string]int64{
	http.MethodPost + " " + BasePath + "/offline/events":        MaxBatchBodyBytes,
	http.MethodPost + " " + BasePath + "/offline/sync":          MaxBatchBodyBytes,
	http.MethodPost + " " + BasePath + "/offline/revocations":   MaxBatchBodyBytes,
	http.MethodPost + " " + BasePath + "/admin/backups/restore": MaxRestoreBodyBytes,
	http.MethodPut + " " + BasePath + itemImageUploadPath:       MaxImageUploadBodyBytes,
	http.MethodPut + " " + BasePath + radioLogoUploadPath:       MaxImageUploadBodyBytes,
}

// bodyLimit returns the larger limit of the route ("METHOD path") from largeBodies, 0 when it has
// none: an exact entry first, else every entry with parameters whose segments match.
func bodyLimit(route string) int64 {
	if n, ok := largeBodies[route]; ok {
		return n
	}
	var out int64
	for pattern, n := range largeBodies {
		if strings.Contains(pattern, "{") && routeMatches(pattern, route) {
			out = max(out, n)
		}
	}
	return out
}

// routeMatches reports whether route has the segments of pattern, a "{…}" segment standing for
// any one non-empty segment.
func routeMatches(pattern, route string) bool {
	ps, rs := strings.Split(pattern, "/"), strings.Split(route, "/")
	if len(ps) != len(rs) {
		return false
	}
	for i, p := range ps {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			if rs[i] == "" {
				return false
			}
			continue
		}
		if p != rs[i] {
			return false
		}
	}
	return true
}

// MaxBody wraps every request body in http.MaxBytesReader with limit n, or the larger limit of the
// route in largeBodies (the offline batch routes, the backup upload, the image upload): reading
// past it fails with *http.MaxBytesError, which the JSON decoding of the strict server reports as
// 400.
func MaxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max(n, bodyLimit(r.Method+" "+r.URL.Path)))
			}
			next.ServeHTTP(w, r)
		})
	}
}
