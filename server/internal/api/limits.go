package api

import "net/http"

// MaxBodyBytes caps request bodies (JSON endpoints take a few hundred bytes; the largest is the
// setup wizard), so a client cannot make the server buffer or decode megabytes.
const MaxBodyBytes = 64 << 10

// MaxBatchBodyBytes caps the bodies of the offline batch operations, which carry lists the
// contract bounds: up to 500 offline events of about 300 bytes (uploadOfflineEvents), 2000 held
// downloads (syncOffline) or 2000 download ids (checkOfflineRevocations).
const MaxBatchBodyBytes = 1 << 20

// batchBodies are the routes ("METHOD path") that get MaxBatchBodyBytes.
var batchBodies = map[string]bool{
	http.MethodPost + " " + BasePath + "/offline/events":      true,
	http.MethodPost + " " + BasePath + "/offline/sync":        true,
	http.MethodPost + " " + BasePath + "/offline/revocations": true,
}

// MaxBody wraps every request body in http.MaxBytesReader with limit n, or MaxBatchBodyBytes when
// that is larger for the offline batch routes: reading past it fails with *http.MaxBytesError,
// which the JSON decoding of the strict server reports as 400.
func MaxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				limit := n
				if batchBodies[r.Method+" "+r.URL.Path] {
					limit = max(n, MaxBatchBodyBytes)
				}
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
