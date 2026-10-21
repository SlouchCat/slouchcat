package api

import (
	"net/http"

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

// largeBodies are the routes ("METHOD path") with a larger limit than MaxBodyBytes.
var largeBodies = map[string]int64{
	http.MethodPost + " " + BasePath + "/offline/events":        MaxBatchBodyBytes,
	http.MethodPost + " " + BasePath + "/offline/sync":          MaxBatchBodyBytes,
	http.MethodPost + " " + BasePath + "/offline/revocations":   MaxBatchBodyBytes,
	http.MethodPost + " " + BasePath + "/admin/backups/restore": MaxRestoreBodyBytes,
}

// MaxBody wraps every request body in http.MaxBytesReader with limit n, or the larger limit of the
// offline batch routes (MaxBatchBodyBytes) and of the backup upload (MaxRestoreBodyBytes): reading
// past it fails with *http.MaxBytesError, which the JSON decoding of the strict server reports as
// 400.
func MaxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max(n, largeBodies[r.Method+" "+r.URL.Path]))
			}
			next.ServeHTTP(w, r)
		})
	}
}
