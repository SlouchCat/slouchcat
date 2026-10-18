// Package webassets serves the built web app that `task build` copies into dist/.
//
// Hashed build output under /assets/ is cached forever; every other path that is not a file is a
// client-side route and receives index.html, which must always be revalidated so a new release
// reaches the browser.
package webassets

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var dist embed.FS

const (
	cacheImmutable = "public, max-age=31536000, immutable"
	cacheRevalid   = "no-cache"
	notBuiltText   = "Slouchcat server is running; the web app has not been built.\n"
)

// Handler serves the embedded web app.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // the embed directive guarantees dist exists
	}
	return NewHandler(sub)
}

// NewHandler serves the web app from fsys, whose root holds index.html and assets/.
func NewHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" && serveFile(w, r, fsys, name) {
			return
		}
		// A folder with its own index.html is a self-contained page (for example the offline
		// spike under /spike/), not a client-side route of the app.
		if name != "" && strings.HasSuffix(r.URL.Path, "/") && serveFile(w, r, fsys, name+"/index.html") {
			return
		}
		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r) // a missing hashed asset is never an app route
			return
		}
		if !serveFile(w, r, fsys, "index.html") {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", cacheRevalid)
			_, _ = io.WriteString(w, notBuiltText)
		}
	})
}

// serveFile writes the regular file name from fsys and reports whether it existed.
func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "unseekable asset", http.StatusInternalServerError)
		return true
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", cacheImmutable)
	} else {
		w.Header().Set("Cache-Control", cacheRevalid)
	}
	// Embedded files have no modification time, so ServeContent sends no Last-Modified.
	http.ServeContent(w, r, name, time.Time{}, rs)
	return true
}
