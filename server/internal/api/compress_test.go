package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/slouchcat/slouchcat/server/internal/webassets"
)

func gunzip(t *testing.T, body []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Web assets and JSON answers are gzipped for clients that accept it; images and media streams
// stay as they are (P-10).
func TestCompression(t *testing.T) {
	js := []byte(strings.Repeat("export const slouchcat = 'start page';\n", 200))
	web := webassets.NewHandler(fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><title>Slouchcat</title>")},
		"assets/index.js": {Data: js},
	})
	pf := newProfileFixtureWithWeb(t, web)
	h, m := pf.h, pf.f.Rated(6)
	gz := map[string]string{"Accept-Encoding": "gzip, deflate, br"}

	rec := send(t, h, http.MethodGet, "/assets/index.js", "", "", gz)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("asset: status %d, Content-Encoding %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
	if v := rec.Header().Get("Vary"); v != "Accept-Encoding" {
		t.Errorf("asset Vary = %q", v)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control = %q", cc)
	}
	if rec.Header().Get("Content-Length") != "" || !bytes.Equal(gunzip(t, rec.Body.Bytes()), js) {
		t.Errorf("asset body does not decompress to the file (Content-Length %q)", rec.Header().Get("Content-Length"))
	}
	// A range of the identity body is not applied to the compressed answer.
	ranged := send(t, h, http.MethodGet, "/assets/index.js", "", "", map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-9"})
	if ranged.Code != http.StatusOK || !bytes.Equal(gunzip(t, ranged.Body.Bytes()), js) {
		t.Errorf("ranged asset: status %d", ranged.Code)
	}
	// Without Accept-Encoding: identity, still marked as varying.
	plain := send(t, h, http.MethodGet, "/assets/index.js", "", "", nil)
	if plain.Header().Get("Content-Encoding") != "" || !bytes.Equal(plain.Body.Bytes(), js) || plain.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("identity asset: headers %v", plain.Header())
	}
	if page := send(t, h, http.MethodGet, "/items/x", "", "", gz); page.Header().Get("Content-Encoding") != "gzip" || page.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("app route: headers %v", page.Header())
	}

	// JSON API answers, including problems.
	if home := send(t, h, http.MethodGet, BasePath+"/home", "p6", "", gz); home.Code != http.StatusOK || home.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("home: status %d, Content-Encoding %q", home.Code, home.Header().Get("Content-Encoding"))
	}
	if miss := send(t, h, http.MethodGet, BasePath+"/items/"+pf.f.Rated(12).ID, "p6", "", gz); miss.Code != http.StatusNotFound || miss.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("hidden item: status %d, Content-Encoding %q", miss.Code, miss.Header().Get("Content-Encoding"))
	}

	// Binary routes are never compressed.
	_, st := pf.mint(t, "p6", m, "direct")
	stream := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/stream?st="+url.QueryEscape(st), "", "", map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-99"})
	if stream.Code != http.StatusPartialContent || stream.Header().Get("Content-Encoding") != "" || !bytes.Equal(stream.Body.Bytes(), m.Content[:100]) {
		t.Errorf("stream: status %d, Content-Encoding %q", stream.Code, stream.Header().Get("Content-Encoding"))
	}
	img := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/images/backdrop/w780", "p6", "", gz)
	if img.Code != http.StatusOK || img.Header().Get("Content-Encoding") != "" || img.Header().Get("Content-Length") == "" {
		t.Errorf("image: status %d, headers %v", img.Code, img.Header())
	}
}
