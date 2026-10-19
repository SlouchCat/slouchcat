package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/discover"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// preloadAuth resolves fixed tokens; the token "extend" reports a sliding extension.
type preloadAuth struct{ sessions map[string]model.Session }

func (preloadAuth) SetupRequired(context.Context) (bool, error) { return false, nil }
func (preloadAuth) Login(context.Context, string, string, string) (model.Session, string, error) {
	return model.Session{}, "", model.ErrInvalidCredentials
}
func (preloadAuth) Logout(context.Context, int64) error { return nil }
func (a preloadAuth) Resolve(_ context.Context, token string) (model.Session, bool, error) {
	if s, ok := a.sessions[token]; ok {
		return s, token == "extend", nil
	}
	return model.Session{}, false, model.ErrNotFound
}

// emptyCatalog is a catalog whose profiles have no picks.
type emptyCatalog struct{ CatalogService }

func (emptyCatalog) FirstPick(context.Context, model.Profile) (model.ItemSummary, error) {
	return model.ItemSummary{}, model.ErrNotFound
}

const appShell = `<!doctype html><title>Slouchcat</title><body><!--hero--><div id="root"></div>`

// shellWith is the app shell with the hero marker replaced by prepaint.
func shellWith(prepaint string) string { return strings.Replace(appShell, heroMarker, prepaint, 1) }

func shellHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, appShell)
	})
}

func getWithToken(h http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestStartPagePreloadHeaders(t *testing.T) {
	f := vistest.New(t)
	log := vistest.Logger()
	session := model.Session{ID: 1, Account: f.Account, Profile: f.P18, ExpiresAt: time.Now().Add(time.Hour)}
	auth := preloadAuth{sessions: map[string]model.Session{"p18": session, "extend": session}}
	h := NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: log, Web: shellHandler(),
		Auth: auth, Catalog: discover.New(f.DB, f.CacheDir, log), Playback: playback.New(f.DB, log),
	})

	// The header names exactly the URLs the start page's carousel uses for its first slide.
	var page struct {
		Rows []struct {
			Key   string `json:"key"`
			Items []struct {
				ID     string `json:"id"`
				Images struct {
					Backdrop      struct{ URL string } `json:"backdrop"`
					BackdropSmall struct{ URL string } `json:"backdropSmall"`
				} `json:"images"`
			} `json:"items"`
		} `json:"rows"`
	}
	home := getWithToken(h, BasePath+"/home", "p18")
	if err := json.Unmarshal(home.Body.Bytes(), &page); err != nil || len(page.Rows) == 0 || page.Rows[0].Key != "picks" {
		t.Fatalf("home: status %d, err %v, body %s", home.Code, err, home.Body.String())
	}
	first := page.Rows[0].Items[0]
	if first.ID != f.Rated(18).ID {
		t.Fatalf("first pick of P18 = %s, want the 18-rated movie", first.ID)
	}
	small, large := first.Images.BackdropSmall.URL, first.Images.Backdrop.URL
	want := []string{
		"</api/v1/auth/session>; rel=preload; as=fetch; crossorigin",
		"</api/v1/home>; rel=preload; as=fetch; crossorigin",
		"<" + small + `>; rel=preload; as=image; imagesrcset="` + small + " 780w, " + large +
			` 1280w"; imagesizes="100vw"; fetchpriority=high`,
	}

	// The shell paints the same image ahead of the app: outside #root, URLs HTML-escaped.
	esc := func(u string) string { return strings.ReplaceAll(u, "&", "&amp;") }
	prepaint := `<div id="hero-prepaint" aria-hidden="true"><img src="` + esc(small) + `" srcset="` +
		esc(small) + " 780w, " + esc(large) + ` 1280w" sizes="100vw" alt="" width="1280" height="720" ` +
		`fetchpriority="high" decoding="sync"></div>`

	t.Run("start page with session", func(t *testing.T) {
		rec := getWithToken(h, "/", "p18")
		if rec.Code != http.StatusOK || rec.Body.String() != shellWith(prepaint) {
			t.Fatalf("status %d, body %q\nwant %q", rec.Code, rec.Body.String(), shellWith(prepaint))
		}
		if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(shellWith(prepaint))) {
			t.Errorf("Content-Length = %q", got)
		}
		if got := rec.Header().Values("Link"); !slices.Equal(got, want) {
			t.Errorf("Link = %q\nwant   %q", got, want)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q", got)
		}
		if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Cookie") {
			t.Errorf("Vary = %v, want Cookie", rec.Header().Values("Vary"))
		}
		if c := rec.Result().Cookies(); len(c) != 0 {
			t.Errorf("cookies = %v, want none without an extension", c)
		}
	})

	t.Run("sliding extension re-issues the cookie", func(t *testing.T) {
		rec := getWithToken(h, "/", "extend")
		if got := rec.Header().Values("Link"); !slices.Equal(got, want) {
			t.Errorf("Link = %q", got)
		}
		c := rec.Result().Cookies()
		if len(c) != 1 || c[0].Name != SessionCookie || c[0].Value != "extend" {
			t.Errorf("cookies = %v", c)
		}
	})

	// The start page always drops the marker; other routes are left alone (it is a comment).
	for name, tc := range map[string]struct{ path, token, body string }{
		"without session":       {"/", "", shellWith("")},
		"unknown session":       {"/", "nope", shellWith("")},
		"other app route":       {"/items/" + first.ID, "p18", appShell},
		"index.html (precache)": {"/index.html", "p18", appShell},
	} {
		t.Run(name, func(t *testing.T) {
			rec := getWithToken(h, tc.path, tc.token)
			if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
				t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "hero-prepaint") {
				t.Errorf("body carries the hero image: %q", rec.Body.String())
			}
			if got := rec.Header().Get("Link"); got != "" {
				t.Errorf("Link = %q, want none", got)
			}
		})
	}

	t.Run("profile without picks: data only", func(t *testing.T) {
		h := NewHandler(Options{
			Version: "test", Started: time.Now(), Logger: log, Web: shellHandler(),
			Auth: auth, Catalog: emptyCatalog{},
		})
		rec := getWithToken(h, "/", "p18")
		if rec.Code != http.StatusOK || rec.Body.String() != shellWith("") {
			t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Values("Link"); !slices.Equal(got, want[:2]) {
			t.Errorf("Link = %q, want only the data preloads", got)
		}
	})
}

func TestHeroLinkFallsBackToSingleImage(t *testing.T) {
	img := func(kind, size string) *model.Image {
		return &model.Image{Kind: kind, Size: size, ContentHash: "h"}
	}
	cases := []struct {
		name string
		item model.ItemSummary
		want string
	}{
		{"large backdrop only", model.ItemSummary{ID: "a", Backdrop: img("backdrop", "w1280"), Poster: img("poster", "w500")},
			"</api/v1/items/a/images/backdrop/w1280?v=h>; rel=preload; as=image; fetchpriority=high"},
		{"small backdrop only", model.ItemSummary{ID: "a", BackdropSmall: img("backdrop", "w780")},
			"</api/v1/items/a/images/backdrop/w780?v=h>; rel=preload; as=image; fetchpriority=high"},
		{"poster only", model.ItemSummary{ID: "a", Poster: img("poster", "w500")},
			"</api/v1/items/a/images/poster/w500?v=h>; rel=preload; as=image; fetchpriority=high"},
		{"no image", model.ItemSummary{ID: "a"}, ""},
	}
	for _, tc := range cases {
		if got := heroLink(tc.item); got != tc.want {
			t.Errorf("%s: heroLink = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestHeroPrepaint(t *testing.T) {
	img := func(kind, size string) *model.Image {
		return &model.Image{Kind: kind, Size: size, ContentHash: "h"}
	}
	const tail = ` alt="" width="1280" height="720" fetchpriority="high" decoding="sync"></div>`
	cases := []struct {
		name string
		item model.ItemSummary
		want string
	}{
		{"both backdrops", model.ItemSummary{ID: "a", Backdrop: img("backdrop", "w1280"), BackdropSmall: img("backdrop", "w780")},
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w780?v=h" ` +
				`srcset="/api/v1/items/a/images/backdrop/w780?v=h 780w, /api/v1/items/a/images/backdrop/w1280?v=h 1280w" sizes="100vw"` + tail},
		{"large backdrop only", model.ItemSummary{ID: "a", Backdrop: img("backdrop", "w1280"), Poster: img("poster", "w500")},
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w1280?v=h"` + tail},
		{"small backdrop only", model.ItemSummary{ID: "a", BackdropSmall: img("backdrop", "w780")},
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w780?v=h"` + tail},
		// The carousel blurs a poster; the shell does not paint it ahead.
		{"poster only", model.ItemSummary{ID: "a", Poster: img("poster", "w500")}, ""},
		{"no image", model.ItemSummary{ID: "a"}, ""},
		// Attribute values are escaped even though the URLs are built by the server.
		{"escaped", model.ItemSummary{ID: "a", Backdrop: &model.Image{Kind: "backdrop", Size: `w1280"><x&`, ContentHash: "h"}},
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w1280&#34;&gt;&lt;x&amp;?v=h"` + tail},
	}
	for _, tc := range cases {
		if got := heroPrepaint(tc.item); got != tc.want {
			t.Errorf("%s: heroPrepaint = %q\nwant %q", tc.name, got, tc.want)
		}
	}
}
