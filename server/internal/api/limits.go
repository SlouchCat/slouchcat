package api

import "net/http"

// MaxBodyBytes caps request bodies (JSON endpoints take a few hundred bytes; the largest is the
// setup wizard), so a client cannot make the server buffer or decode megabytes.
const MaxBodyBytes = 64 << 10

// MaxBody wraps every request body in http.MaxBytesReader with limit n: reading past it fails
// with *http.MaxBytesError, which the JSON decoding of the strict server reports as 400.
func MaxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}
