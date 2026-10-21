package trakt

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

const clientID = "test-client-id"

func fast() *ratelimit.Limiter { return ratelimit.New("trakt-test", 1000, 100) }

// server answers the four lists in both shapes and checks the headers of every request.
func server(t *testing.T, status *atomic.Int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	check := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("trakt-api-key") != clientID || r.Header.Get("trakt-api-version") != "2" ||
			r.Header.Get("Content-Type") != "application/json" || !strings.HasPrefix(r.Header.Get("User-Agent"), "Slouchcat") {
			t.Errorf("%s headers: %v", r.URL.Path, r.Header)
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Errorf("%s limit = %q", r.URL.Path, r.URL.RawQuery)
		}
		if s := status.Load(); s != 0 {
			w.WriteHeader(int(s))
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		return true
	}
	mux.HandleFunc("GET /movies/trending", func(w http.ResponseWriter, r *http.Request) {
		if check(w, r) {
			_, _ = w.Write([]byte(`[
				{"watchers": 50, "movie": {"title": "Eins", "year": 2020, "ids": {"trakt": 1, "slug": "eins", "tmdb": 101}}},
				{"watchers": 40, "movie": {"title": "Ohne", "year": 2021, "ids": {"trakt": 2, "slug": "ohne"}}},
				{"watchers": 30, "movie": {"title": "Drei", "year": null, "ids": {"trakt": 3, "tmdb": 103}}}]`))
		}
	})
	mux.HandleFunc("GET /shows/trending", func(w http.ResponseWriter, r *http.Request) {
		if check(w, r) {
			_, _ = w.Write([]byte(`[{"watchers": 9, "show": {"title": "Serie", "year": 2019, "ids": {"tmdb": 201, "tvdb": 7}}}]`))
		}
	})
	mux.HandleFunc("GET /movies/popular", func(w http.ResponseWriter, r *http.Request) {
		if check(w, r) {
			_, _ = w.Write([]byte(`[{"title": "Pop", "year": 2001, "ids": {"tmdb": 301}}, {"title": "Null", "ids": {"tmdb": null}}]`))
		}
	})
	mux.HandleFunc("GET /shows/popular", func(w http.ResponseWriter, r *http.Request) {
		if check(w, r) {
			_, _ = w.Write([]byte(`[{"title": " Popserie ", "year": 2010, "ids": {"tmdb": 401}}]`))
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestListShapes(t *testing.T) {
	var status atomic.Int32
	srv := server(t, &status)
	c := New(clientID, WithBaseURL(srv.URL+"/"), WithHTTPClient(srv.Client()), WithLimiter(fast()))
	ctx := context.Background()
	for _, tc := range []struct {
		list, media string
		want        []Entry
	}{
		{ListTrending, MediaMovie, []Entry{{Media: "movie", Position: 1, TMDBID: 101, Title: "Eins", Year: 2020},
			{Media: "movie", Position: 3, TMDBID: 103, Title: "Drei"}}},
		{ListTrending, MediaShow, []Entry{{Media: "show", Position: 1, TMDBID: 201, Title: "Serie", Year: 2019}}},
		{ListPopular, MediaMovie, []Entry{{Media: "movie", Position: 1, TMDBID: 301, Title: "Pop", Year: 2001}}},
		{ListPopular, MediaShow, []Entry{{Media: "show", Position: 1, TMDBID: 401, Title: "Popserie", Year: 2010}}},
	} {
		got, err := c.List(ctx, tc.list, tc.media, MaxLimit)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.list, tc.media, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s %s = %+v, want %+v", tc.list, tc.media, got, tc.want)
		}
	}
	if _, err := c.List(ctx, "anticipated", MediaMovie, 10); err == nil {
		t.Error("unknown list accepted")
	}
	if _, err := c.List(ctx, ListPopular, "person", 10); err == nil {
		t.Error("unknown media accepted")
	}
}

func TestListErrors(t *testing.T) {
	var status atomic.Int32
	srv := server(t, &status)
	c := New(clientID, WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithLimiter(fast()))
	ctx := context.Background()
	for code, want := range map[int32]error{503: ErrUnavailable, 500: ErrUnavailable, 401: ErrUnauthorized, 403: ErrUnauthorized} {
		status.Store(code)
		_, err := c.List(ctx, ListPopular, MediaMovie, MaxLimit)
		if !errors.Is(err, want) {
			t.Errorf("status %d: %v, want %v", code, err, want)
		}
		if err != nil && strings.Contains(err.Error(), clientID) {
			t.Errorf("status %d: error names the client id: %v", code, err)
		}
	}
	status.Store(404)
	if _, err := c.List(ctx, ListPopular, MediaMovie, MaxLimit); err == nil || errors.Is(err, ErrUnavailable) {
		t.Errorf("status 404: %v", err)
	}
	srv.Close()
	status.Store(0)
	if _, err := c.List(ctx, ListPopular, MediaMovie, MaxLimit); !errors.Is(err, ErrUnavailable) {
		t.Errorf("closed server: %v", err)
	}
}

// A malformed body is an unavailable list, not an empty one.
func TestListMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error": "not a list"}`))
	}))
	defer srv.Close()
	c := New(clientID, WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithLimiter(fast()))
	if _, err := c.List(context.Background(), ListTrending, MediaShow, 5); !errors.Is(err, ErrUnavailable) {
		t.Errorf("malformed body: %v", err)
	}
}

// k8 (M7 review): a redirect to another host never receives the client id; one on the same host
// is followed.
func TestRedirectStaysOnHost(t *testing.T) {
	var foreignHits atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignHits.Add(1)
		if r.Header.Get("trakt-api-key") != "" {
			t.Errorf("foreign host got the client id")
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer foreign.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /movies/popular", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/movies/popular", http.StatusFound)
	})
	mux.HandleFunc("GET /shows/popular", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/moved/shows", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /moved/shows", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("trakt-api-key") != clientID {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[{"title": "Da", "ids": {"tmdb": 5}}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(clientID, WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithLimiter(fast()))
	ctx := context.Background()
	if _, err := c.List(ctx, ListPopular, MediaMovie, MaxLimit); !errors.Is(err, ErrUnavailable) {
		t.Errorf("redirect to another host: %v, want ErrUnavailable", err)
	}
	if n := foreignHits.Load(); n != 0 {
		t.Errorf("foreign host hit %d times", n)
	}
	got, err := c.List(ctx, ListPopular, MediaShow, MaxLimit)
	if err != nil || len(got) != 1 || got[0].TMDBID != 5 {
		t.Errorf("redirect on the same host: %+v, %v", got, err)
	}
}

// n7 (M7 review): the body is drained before it is closed, so sequential requests share one
// connection even when the JSON decoder stops before the end of the body.
func TestListReusesConnection(t *testing.T) {
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"title": "Pop", "ids": {"tmdb": 301}}]` + strings.Repeat(" ", 64<<10)))
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()
	c := New(clientID, WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithLimiter(fast()))
	for range 3 {
		if _, err := c.List(context.Background(), ListPopular, MediaMovie, MaxLimit); err != nil {
			t.Fatal(err)
		}
	}
	if n := conns.Load(); n != 1 {
		t.Errorf("%d connections for three requests, want 1", n)
	}
}
