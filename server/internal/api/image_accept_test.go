package api

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func TestAcceptsWebP(t *testing.T) {
	for _, c := range []struct {
		accept []string
		want   bool
	}{
		{nil, false},
		{[]string{"image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8"}, true},                                                             // Chromium
		{[]string{"image/avif,image/webp,*/*"}, true},                                                                                                    // Firefox
		{[]string{"image/webp,image/avif,image/jxl,image/heic,image/heic-sequence,video/*;q=0.8,image/png,image/svg+xml,image/*;q=0.8,*/*;q=0.5"}, true}, // Safari
		{[]string{"image/png,image/*;q=0.8,*/*;q=0.5"}, false},
		{[]string{"*/*"}, false},
		{[]string{"image/*"}, false},
		{[]string{"IMAGE/WEBP"}, true},
		{[]string{"image/webp;q=0"}, false},
		{[]string{"image/webp; q=0.0"}, false},
		{[]string{"image/webp;q=0.1"}, true},
		{[]string{"text/html", "image/webp"}, true},
		{[]string{"image/webpx"}, false},
	} {
		if got := acceptsWebP(c.accept); got != c.want {
			t.Errorf("acceptsWebP(%q) = %v, want %v", c.accept, got, c.want)
		}
	}
}

// The image endpoint answers WebP when the request names it, JPEG otherwise, always with
// Vary: Accept, and every contract size of a kind is served.
func TestImageNegotiation(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := f.Rated(6)
	base := BasePath + "/items/" + m.ID + "/images/poster/w400"
	jpg, err := os.ReadFile(filepath.Join(f.CacheDir, filepath.FromSlash(m.PosterRel)))
	if err != nil {
		t.Fatal(err)
	}
	webp, err := os.ReadFile(filepath.Join(f.CacheDir, filepath.FromSlash(m.PosterWebPRel)))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		accept, wantType string
		want             []byte
	}{
		{"image/avif,image/webp,image/apng,image/*,*/*;q=0.8", "image/webp", webp},
		{"image/png,image/*;q=0.8,*/*;q=0.5", "image/jpeg", jpg},
		{"", "image/jpeg", jpg},
	} {
		var header map[string]string
		if c.accept != "" {
			header = map[string]string{"Accept": c.accept}
		}
		rec := send(t, h, http.MethodGet, base, "p6", "", header)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != c.wantType || !bytes.Equal(rec.Body.Bytes(), c.want) {
			t.Errorf("Accept %q: status %d, type %q", c.accept, rec.Code, rec.Header().Get("Content-Type"))
		}
		if v := rec.Header().Values("Vary"); !containsToken(v, "Accept") {
			t.Errorf("Accept %q: Vary = %q, want Accept", c.accept, v)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
			t.Errorf("Accept %q: Cache-Control = %q", c.accept, cc)
		}
	}
	for _, path := range []string{"poster/w200", "poster/w800", "backdrop/w780", "backdrop/w1280", "backdrop/w1920"} {
		rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/images/"+path, "p6", "", map[string]string{"Accept": "image/webp"})
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/webp" {
			t.Errorf("%s: status %d, type %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	// The detail exposes every size and the accent colour.
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, "p6", "", nil)
	var d struct {
		Accent string                          `json:"accent"`
		Images map[string]struct{ URL string } `json:"images"`
	}
	decode(t, rec, &d)
	if d.Accent != vistest.ItemAccent {
		t.Errorf("accent = %q, want %q", d.Accent, vistest.ItemAccent)
	}
	for name, size := range map[string]string{"posterSmall": "w200", "poster": "w400", "posterLarge": "w800", "backdropSmall": "w780", "backdrop": "w1280", "backdropLarge": "w1920"} {
		kind := "poster"
		if name[:4] == "back" {
			kind = "backdrop"
		}
		want := BasePath + "/items/" + m.ID + "/images/" + kind + "/" + size + "?v="
		if u := d.Images[name].URL; len(u) <= len(want) || u[:len(want)] != want {
			t.Errorf("%s url = %q, want prefix %q", name, u, want)
		}
	}
}

func containsToken(values []string, token string) bool {
	for _, v := range values {
		for _, p := range bytes.Split([]byte(v), []byte(",")) {
			if string(bytes.TrimSpace(p)) == token {
				return true
			}
		}
	}
	return false
}
