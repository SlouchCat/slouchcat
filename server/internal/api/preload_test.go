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

// emptyCatalog is a catalog whose profiles have no stage.
type emptyCatalog struct{ CatalogService }

func (emptyCatalog) FirstHero(context.Context, model.Profile) (model.ItemSummary, bool, error) {
	return model.ItemSummary{}, false, model.ErrNotFound
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

// homeAnswer is the part of GET /home the preload tests read.
type homeAnswer struct {
	HeroRow *string `json:"heroRow"`
	Rows    []struct {
		Key   string `json:"key"`
		Items []struct {
			ID         string `json:"id"`
			HasTrailer *bool  `json:"hasTrailer"`
			Images     struct {
				Backdrop      struct{ URL string } `json:"backdrop"`
				BackdropSmall struct{ URL string } `json:"backdropSmall"`
			} `json:"images"`
		} `json:"items"`
	} `json:"rows"`
}

func getHome(t *testing.T, h http.Handler, token string) homeAnswer {
	t.Helper()
	var page homeAnswer
	rec := getWithToken(h, BasePath+"/home", token)
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("home: status %d, err %v, body %s", rec.Code, err, rec.Body.String())
	}
	return page
}

// Plan 3.5 and step 4 (P-10): the shell of GET / preloads the first image of the stage whenever
// there is one, and paints it ahead only when the stage is the first row of the start page.
func TestStartPagePreloadHeaders(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	log := vistest.Logger()
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	// The trending movies are the fixture films 1000..1004: five spotlight hits for P18, the
	// default min_hits, so spotlight is the stage and the first row (migration 00021's order).
	for i := range 5 {
		exec(`INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, time.Now().UnixMilli())
	}
	session := model.Session{ID: 1, Account: f.Account, Profile: f.P18, ExpiresAt: time.Now().Add(time.Hour)}
	auth := preloadAuth{sessions: map[string]model.Session{"p18": session, "extend": session}}
	h := NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: log, Web: shellHandler(),
		Auth: auth, Catalog: discover.New(f.DB, f.CacheDir, log), Playback: playback.New(f.DB, log),
	})

	// The header names exactly the URLs the start page's stage uses for its first title.
	page := getHome(t, h, "p18")
	if page.HeroRow == nil || *page.HeroRow != "spotlight" || len(page.Rows) == 0 || page.Rows[0].Key != "spotlight" {
		t.Fatalf("home: hero row %v, rows %+v; want spotlight first", page.HeroRow, page.Rows)
	}
	first := page.Rows[0].Items[0]
	small, large := first.Images.BackdropSmall.URL, first.Images.Backdrop.URL
	if small == "" || large == "" {
		t.Fatalf("first stage title %s without both backdrops", first.ID)
	}
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

	// Plan 3.5: a stage further down keeps the preload header but gets no prepaint, which would
	// sit at the top of the page.
	t.Run("stage further down: preload only", func(t *testing.T) {
		exec(`UPDATE home_rows SET position = CASE key WHEN 'spotlight' THEN 3 WHEN 'recent' THEN 0 ELSE position END`)
		defer exec(`UPDATE home_rows SET position = CASE key WHEN 'spotlight' THEN 0 WHEN 'recent' THEN 3 ELSE position END`)
		page := getHome(t, h, "p18")
		if page.HeroRow == nil || *page.HeroRow != "spotlight" || page.Rows[0].Key == "spotlight" {
			t.Fatalf("home: hero row %v, first row %s; want spotlight further down", page.HeroRow, page.Rows[0].Key)
		}
		rec := getWithToken(h, "/", "p18")
		if rec.Code != http.StatusOK || rec.Body.String() != shellWith("") {
			t.Fatalf("status %d, body %q, want the shell without prepaint", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Values("Link"); !slices.Equal(got, want) {
			t.Errorf("Link = %q\nwant   %q", got, want)
		}
	})

	// Step 4 DoD "FirstHero im leeren Fall": without a stage there is neither a prepaint nor a
	// hero preload, even though picks is filled (picks is never the stage).
	t.Run("no stage: data only", func(t *testing.T) {
		exec(`UPDATE home_rows SET hidden = 1 WHERE key = 'spotlight'`)
		defer exec(`UPDATE home_rows SET hidden = 0 WHERE key = 'spotlight'`)
		page := getHome(t, h, "p18")
		if page.HeroRow != nil || len(page.Rows) == 0 || page.Rows[0].Key != "picks" {
			t.Fatalf("home: hero row %v, rows %+v; want no stage and picks first", page.HeroRow, page.Rows)
		}
		rec := getWithToken(h, "/", "p18")
		if rec.Code != http.StatusOK || rec.Body.String() != shellWith("") {
			t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Values("Link"); !slices.Equal(got, want[:2]) {
			t.Errorf("Link = %q, want only the data preloads", got)
		}
	})

	t.Run("catalog without a stage: data only", func(t *testing.T) {
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

// Plan 3.6 (D3) and 5.4 through GET /home: ItemRef.hasTrailer is present in the rows spotlight and
// picks only, true for a title with a stored trailer; every other row leaves it out.
func TestHomeHasTrailerOverHTTP(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	log := vistest.Logger()
	ctx := context.Background()
	for i := range 5 {
		if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, fetched_at) VALUES ('trending', 'movie', ?, ?, ?)`, i+1, 1000+i, time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	session := model.Session{ID: 1, Account: f.Account, Profile: f.P18, ExpiresAt: time.Now().Add(time.Hour)}
	h := NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: log, Web: shellHandler(),
		Auth: preloadAuth{sessions: map[string]model.Session{"p18": session}}, Catalog: discover.New(f.DB, f.CacheDir, log), Playback: playback.New(f.DB, log),
	})
	before := getHome(t, h, "p18")
	if len(before.Rows) < 2 || before.Rows[0].Key != "spotlight" || before.Rows[1].Key != "picks" {
		t.Fatalf("rows %+v, want spotlight and picks first", before.Rows)
	}
	withTrailer := before.Rows[0].Items[0].ID
	if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at) VALUES (?, 'en', 'FlagTrailer', 'Trailer', 'tmdb', 1, 1)`, withTrailer); err != nil {
		t.Fatal(err)
	}
	page := getHome(t, h, "p18")
	others := 0
	for _, row := range page.Rows {
		flagged := row.Key == "spotlight" || row.Key == "picks"
		for _, it := range row.Items {
			switch {
			case !flagged:
				others++
				if it.HasTrailer != nil {
					t.Errorf("row %s, %s: hasTrailer = %v, want absent", row.Key, it.ID, *it.HasTrailer)
				}
			case it.HasTrailer == nil:
				t.Errorf("row %s, %s: hasTrailer absent", row.Key, it.ID)
			case *it.HasTrailer != (it.ID == withTrailer):
				t.Errorf("row %s, %s: hasTrailer = %v, want %v", row.Key, it.ID, *it.HasTrailer, it.ID == withTrailer)
			}
		}
	}
	if others == 0 {
		t.Errorf("rows %+v: want titles outside spotlight and picks", page.Rows)
	}
}

func TestHeroLinkFallsBackToSingleImage(t *testing.T) {
	t.Parallel()
	img := func(kind, size string) *model.Image {
		return &model.Image{Kind: kind, Size: size, ContentHash: "h"}
	}
	cases := []struct {
		name string
		item model.ItemSummary
		want string
	}{
		{"large backdrop only", model.ItemSummary{ID: "a", Backdrop: img("backdrop", "w1280"), Poster: img("poster", "w500")},
			"</api/v1/items/a/images/backdrop/w1280?v=h&p=7>; rel=preload; as=image; fetchpriority=high"},
		{"small backdrop only", model.ItemSummary{ID: "a", BackdropSmall: img("backdrop", "w780")},
			"</api/v1/items/a/images/backdrop/w780?v=h&p=7>; rel=preload; as=image; fetchpriority=high"},
		{"poster only", model.ItemSummary{ID: "a", Poster: img("poster", "w500")},
			"</api/v1/items/a/images/poster/w500?v=h&p=7>; rel=preload; as=image; fetchpriority=high"},
		{"no image", model.ItemSummary{ID: "a"}, ""},
	}
	for _, tc := range cases {
		if got := heroLink(tc.item, 7); got != tc.want {
			t.Errorf("%s: heroLink = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestHeroPrepaint(t *testing.T) {
	t.Parallel()
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
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w780?v=h&amp;p=7" ` +
				`srcset="/api/v1/items/a/images/backdrop/w780?v=h&amp;p=7 780w, /api/v1/items/a/images/backdrop/w1280?v=h&amp;p=7 1280w" sizes="100vw"` + tail},
		{"large backdrop only", model.ItemSummary{ID: "a", Backdrop: img("backdrop", "w1280"), Poster: img("poster", "w500")},
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w1280?v=h&amp;p=7"` + tail},
		{"small backdrop only", model.ItemSummary{ID: "a", BackdropSmall: img("backdrop", "w780")},
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w780?v=h&amp;p=7"` + tail},
		// The carousel blurs a poster; the shell does not paint it ahead.
		{"poster only", model.ItemSummary{ID: "a", Poster: img("poster", "w500")}, ""},
		{"no image", model.ItemSummary{ID: "a"}, ""},
		// Attribute values are escaped even though the URLs are built by the server.
		{"escaped", model.ItemSummary{ID: "a", Backdrop: &model.Image{Kind: "backdrop", Size: `w1280"><x&`, ContentHash: "h"}},
			`<div id="hero-prepaint" aria-hidden="true"><img src="/api/v1/items/a/images/backdrop/w1280&#34;&gt;&lt;x&amp;?v=h&amp;p=7"` + tail},
	}
	for _, tc := range cases {
		if got := heroPrepaint(tc.item, 7); got != tc.want {
			t.Errorf("%s: heroPrepaint = %q\nwant %q", tc.name, got, tc.want)
		}
	}
}
