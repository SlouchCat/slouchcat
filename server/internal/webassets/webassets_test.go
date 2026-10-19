package webassets

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func get(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestBuiltApp(t *testing.T) {
	h := NewHandler(fstest.MapFS{
		"index.html":             {Data: []byte("<!doctype html><title>Slouchcat</title>")},
		"assets/index-3f2a1b.js": {Data: []byte("console.log(1)")},
		"manifest.webmanifest":   {Data: []byte("{}")},
		"spike/index.html":       {Data: []byte("<!doctype html><title>Spike</title>")},
	})
	cases := []struct {
		target, cache, body string
		status              int
	}{
		{"/", cacheRevalid, "<!doctype html><title>Slouchcat</title>", 200},
		{"/index.html", cacheRevalid, "<!doctype html><title>Slouchcat</title>", 200},
		{"/library/movies/42", cacheRevalid, "<!doctype html><title>Slouchcat</title>", 200},
		{"/assets/index-3f2a1b.js", cacheImmutable, "console.log(1)", 200},
		{"/manifest.webmanifest", cacheRevalid, "{}", 200},
		{"/assets/missing-000000.js", "", "404 page not found\n", 404},
		{"/../../etc/passwd", cacheRevalid, "<!doctype html><title>Slouchcat</title>", 200},
		{"/spike/", cacheRevalid, "<!doctype html><title>Spike</title>", 200},
		{"/spike/index.html", cacheRevalid, "<!doctype html><title>Spike</title>", 200},
		{"/spike", cacheRevalid, "<!doctype html><title>Slouchcat</title>", 200},
	}
	for _, tc := range cases {
		rec := get(t, h, http.MethodGet, tc.target)
		if rec.Code != tc.status || rec.Body.String() != tc.body || rec.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("GET %s = %d %q cache %q; want %d %q cache %q", tc.target, rec.Code, rec.Body.String(),
				rec.Header().Get("Cache-Control"), tc.status, tc.body, tc.cache)
		}
	}
	if rec := get(t, h, http.MethodPost, "/"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST / = %d, want 405", rec.Code)
	}
}

func TestAppNotBuilt(t *testing.T) {
	h := NewHandler(fstest.MapFS{".gitkeep": {}})
	for _, target := range []string{"/", "/some/route"} {
		rec := get(t, h, http.MethodGet, target)
		if rec.Code != http.StatusOK || rec.Body.String() != notBuiltText ||
			rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Errorf("GET %s = %d %q %q", target, rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
		}
	}
}

func TestEmbeddedHandler(t *testing.T) {
	// The repository embeds only dist/.gitkeep unless the web app was built into it.
	if rec := get(t, Handler(), http.MethodGet, "/"); rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
}
