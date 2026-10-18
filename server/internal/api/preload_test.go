package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
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

const appShell = "<!doctype html><title>Slouchcat</title>"

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

	t.Run("start page with session", func(t *testing.T) {
		rec := getWithToken(h, "/", "p18")
		if rec.Code != http.StatusOK || rec.Body.String() != appShell {
			t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
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

	for name, tc := range map[string]struct{ path, token string }{
		"without session":       {"/", ""},
		"unknown session":       {"/", "nope"},
		"other app route":       {"/items/" + first.ID, "p18"},
		"index.html (precache)": {"/index.html", "p18"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := getWithToken(h, tc.path, tc.token)
			if rec.Code != http.StatusOK || rec.Body.String() != appShell {
				t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
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
		if rec.Code != http.StatusOK || rec.Body.String() != appShell {
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
