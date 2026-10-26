package trakt

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

const clientID = "test-client-id"

func fast() *ratelimit.Limiter { return ratelimit.New("trakt-test", 1000, 100) }

// mixedList is a user list with every entry type Trakt knows; positions 1 to 7.
const mixedList = `[
	{"rank": 1, "id": 1, "listed_at": "2026-01-01T00:00:00.000Z", "type": "movie", "movie": {"title": "Film A", "year": 2000, "ids": {"tmdb": 11}}},
	{"rank": 2, "id": 2, "type": "show", "show": {"title": "Serie B", "year": 2001, "ids": {"tmdb": 12}}},
	{"rank": 3, "id": 3, "type": "season", "season": {"number": 1, "ids": {"tmdb": 13}}, "show": {"title": "Serie S", "ids": {"tmdb": 14}}},
	{"rank": 4, "id": 4, "type": "episode", "episode": {"season": 1, "number": 2, "ids": {"tmdb": 15}}, "show": {"title": "Serie E", "ids": {"tmdb": 16}}},
	{"rank": 5, "id": 5, "type": "person", "person": {"name": "Person P", "ids": {"tmdb": 17}}},
	{"rank": 6, "id": 6, "type": "movie", "movie": {"title": "Film F", "year": 2002, "ids": {"tmdb": 18}}},
	{"rank": 7, "id": 7, "type": "show", "show": {"title": "Serie G", "ids": {"tmdb": 19}}}]`

// bodies are the answers of the mock per escaped request path, in the response shapes of Trakt.
var bodies = map[string]string{
	"/movies/trending": `[
		{"watchers": 50, "movie": {"title": "Eins", "year": 2020, "ids": {"trakt": 1, "slug": "eins", "tmdb": 101}}},
		{"watchers": 40, "movie": {"title": "Ohne", "year": 2021, "ids": {"trakt": 2, "slug": "ohne"}}},
		{"watchers": 30, "movie": {"title": "Drei", "year": null, "ids": {"trakt": 3, "tmdb": 103}}}]`,
	"/shows/trending":                                  `[{"watchers": 9, "show": {"title": "Serie", "year": 2019, "ids": {"tmdb": 201, "tvdb": 7}}}]`,
	"/movies/popular":                                  `[{"title": "Pop", "year": 2001, "ids": {"tmdb": 301}}, {"title": "Null", "ids": {"tmdb": null}}]`,
	"/shows/popular":                                   `[{"title": " Popserie ", "year": 2010, "ids": {"tmdb": 401}}]`,
	"/movies/watched/weekly":                           `[{"watcher_count": 5, "play_count": 9, "collected_count": 2, "movie": {"title": "Gesehen", "ids": {"tmdb": 501}}}]`,
	"/shows/watched/weekly":                            `[{"watcher_count": 5, "play_count": 9, "show": {"title": "Gesehene Serie", "ids": {"tmdb": 502}}}]`,
	"/movies/collected/monthly":                        `[{"watcher_count": 1, "collected_count": 8, "movie": {"title": "Gesammelt", "ids": {"tmdb": 601}}}]`,
	"/shows/collected/monthly":                         `[{"watcher_count": 1, "collected_count": 8, "show": {"title": "Gesammelte Serie", "ids": {"tmdb": 602}}}]`,
	"/movies/anticipated":                              `[{"list_count": 300, "movie": {"title": "Erwartet", "ids": {"tmdb": 701}}}]`,
	"/shows/anticipated":                               `[{"list_count": 200, "show": {"title": "Erwartete Serie", "ids": {"tmdb": 702}}}]`,
	"/movies/boxoffice":                                `[{"revenue": 1000000, "movie": {"title": "Kino", "year": 2026, "ids": {"tmdb": 801}}}]`,
	"/users/jane/lists/mixed/items":                    mixedList,
	"/users/Jane%20Doe%2F%C3%A4/lists/my%20list/items": `[{"type": "movie", "movie": {"title": "Escaped", "ids": {"tmdb": 901}}}]`,
	"/users/secretuser/lists/secretslug/items":         mixedList,
}

// mock answers the paths in bodies (404 for any other), checks the headers and the limit of every
// request, and records the escaped paths it saw.
type mock struct {
	*httptest.Server
	status    atomic.Int32 // answer with this status instead, when not 0
	wantLimit atomic.Int32
	mu        sync.Mutex
	paths     []string
}

func newMock(t *testing.T) *mock {
	t.Helper()
	m := &mock{}
	m.wantLimit.Store(MaxLimit)
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		m.mu.Lock()
		m.paths = append(m.paths, path)
		m.mu.Unlock()
		if r.Method != http.MethodGet || r.Header.Get("trakt-api-key") != clientID || r.Header.Get("trakt-api-version") != "2" ||
			r.Header.Get("Content-Type") != "application/json" || !strings.HasPrefix(r.Header.Get("User-Agent"), "Slouchcat") {
			t.Errorf("%s %s headers: %v", r.Method, path, r.Header)
		}
		if want := strconv.Itoa(int(m.wantLimit.Load())); r.URL.RawQuery != "limit="+want {
			t.Errorf("%s query = %q, want limit=%s", path, r.URL.RawQuery, want)
		}
		if s := m.status.Load(); s != 0 {
			w.WriteHeader(int(s))
			return
		}
		body, ok := bodies[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(m.Close)
	return m
}

// seen returns and resets the recorded paths.
func (m *mock) seen() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.paths
	m.paths = nil
	return p
}

func (m *mock) client() *Client {
	return New(clientID, WithBaseURL(m.URL+"/"), WithHTTPClient(m.Client()), WithLimiter(fast()))
}

func TestKinds(t *testing.T) {
	var keys []string
	for _, k := range Kinds() {
		keys = append(keys, k.Key)
		got, ok := KindOf(k.Key)
		if !ok || got.Key != k.Key || !slices.Equal(got.Media, k.Media) || got.MediaFilters != k.MediaFilters || got.NeedsURL != k.NeedsURL {
			t.Errorf("KindOf(%q) = %+v, %v, want %+v", k.Key, got, ok, k)
		}
		if k.NeedsURL != (k.Key == KindURL) || k.MediaFilters != (k.Key == KindURL) {
			t.Errorf("%s: NeedsURL %v, MediaFilters %v", k.Key, k.NeedsURL, k.MediaFilters)
		}
		wantMedia := []string{MediaMovie, MediaShow, MediaBoth}
		if k.Key == KindBoxOffice {
			wantMedia = []string{MediaMovie}
		}
		if !slices.Equal(k.Media, wantMedia) {
			t.Errorf("%s media = %v, want %v", k.Key, k.Media, wantMedia)
		}
	}
	want := []string{KindTrending, KindPopular, KindWatchedWeekly, KindCollectedMonthly, KindAnticipated, KindBoxOffice, KindURL}
	if !slices.Equal(keys, want) {
		t.Errorf("Kinds() = %v, want %v", keys, want)
	}
	if _, ok := KindOf("watchlist"); ok {
		t.Error("KindOf accepted an unknown key")
	}
	// The catalogue is a copy: a caller cannot change the table.
	Kinds()[0].Media[0] = "person"
	if k, _ := KindOf(KindTrending); k.Media[0] != MediaMovie {
		t.Errorf("Kinds() shares the table: %v", k.Media)
	}
}

// One path test per kind (plan step 2): every kind of table 5.6 asks for its own path(s) and
// nothing else. Without it boxoffice would be forced into the old "/" + media + "s/" + list
// pattern, ask /movies/boxoffices, get a 404 and lose the row in production only.
func TestFetchPathPerKind(t *testing.T) {
	m := newMock(t)
	c := m.client()
	ctx := context.Background()
	const userList = "https://trakt.tv/users/jane/lists/mixed"
	for _, tc := range []struct {
		kind, media, url string
		paths            []string
		ids              []int64
	}{
		{KindTrending, MediaMovie, "", []string{"/movies/trending"}, []int64{101, 103}},
		{KindTrending, MediaShow, "", []string{"/shows/trending"}, []int64{201}},
		{KindTrending, MediaBoth, "", []string{"/movies/trending", "/shows/trending"}, []int64{101, 103, 201}},
		{KindPopular, MediaMovie, "", []string{"/movies/popular"}, []int64{301}},
		{KindPopular, MediaShow, "", []string{"/shows/popular"}, []int64{401}},
		{KindPopular, MediaBoth, "", []string{"/movies/popular", "/shows/popular"}, []int64{301, 401}},
		{KindWatchedWeekly, MediaMovie, "", []string{"/movies/watched/weekly"}, []int64{501}},
		{KindWatchedWeekly, MediaShow, "", []string{"/shows/watched/weekly"}, []int64{502}},
		{KindWatchedWeekly, MediaBoth, "", []string{"/movies/watched/weekly", "/shows/watched/weekly"}, []int64{501, 502}},
		{KindCollectedMonthly, MediaMovie, "", []string{"/movies/collected/monthly"}, []int64{601}},
		{KindCollectedMonthly, MediaShow, "", []string{"/shows/collected/monthly"}, []int64{602}},
		{KindCollectedMonthly, MediaBoth, "", []string{"/movies/collected/monthly", "/shows/collected/monthly"}, []int64{601, 602}},
		{KindAnticipated, MediaMovie, "", []string{"/movies/anticipated"}, []int64{701}},
		{KindAnticipated, MediaShow, "", []string{"/shows/anticipated"}, []int64{702}},
		{KindAnticipated, MediaBoth, "", []string{"/movies/anticipated", "/shows/anticipated"}, []int64{701, 702}},
		{KindBoxOffice, MediaMovie, "", []string{"/movies/boxoffice"}, []int64{801}},
		{KindURL, MediaMovie, userList, []string{"/users/jane/lists/mixed/items"}, []int64{11, 18}},
		{KindURL, MediaShow, userList, []string{"/users/jane/lists/mixed/items"}, []int64{12, 19}},
		{KindURL, MediaBoth, userList, []string{"/users/jane/lists/mixed/items"}, []int64{11, 12, 18, 19}},
	} {
		got, err := c.Fetch(ctx, ListRef{Kind: tc.kind, Media: tc.media, URL: tc.url}, MaxLimit)
		if err != nil {
			t.Errorf("%s %s: %v", tc.kind, tc.media, err)
		}
		if p := m.seen(); !slices.Equal(p, tc.paths) {
			t.Errorf("%s %s asked %v, want %v", tc.kind, tc.media, p, tc.paths)
		}
		var ids []int64
		for _, e := range got {
			ids = append(ids, e.TMDBID)
			if tc.media != MediaBoth && e.Media != tc.media {
				t.Errorf("%s %s: entry %d has media %s", tc.kind, tc.media, e.TMDBID, e.Media)
			}
		}
		if !slices.Equal(ids, tc.ids) {
			t.Errorf("%s %s = %v, want %v", tc.kind, tc.media, ids, tc.ids)
		}
	}
}

func TestFetchShapes(t *testing.T) {
	m := newMock(t)
	c := m.client()
	ctx := context.Background()
	for _, tc := range []struct {
		kind, media string
		want        []Entry
	}{
		{KindTrending, MediaMovie, []Entry{{Media: "movie", Position: 1, TMDBID: 101, Title: "Eins", Year: 2020},
			{Media: "movie", Position: 3, TMDBID: 103, Title: "Drei"}}},
		{KindTrending, MediaShow, []Entry{{Media: "show", Position: 1, TMDBID: 201, Title: "Serie", Year: 2019}}},
		{KindPopular, MediaMovie, []Entry{{Media: "movie", Position: 1, TMDBID: 301, Title: "Pop", Year: 2001}}},
		{KindPopular, MediaShow, []Entry{{Media: "show", Position: 1, TMDBID: 401, Title: "Popserie", Year: 2010}}},
		{KindPopular, MediaBoth, []Entry{{Media: "movie", Position: 1, TMDBID: 301, Title: "Pop", Year: 2001},
			{Media: "show", Position: 1, TMDBID: 401, Title: "Popserie", Year: 2010}}},
		{KindBoxOffice, MediaMovie, []Entry{{Media: "movie", Position: 1, TMDBID: 801, Title: "Kino", Year: 2026}}},
	} {
		got, err := c.Fetch(ctx, ListRef{Kind: tc.kind, Media: tc.media}, MaxLimit)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.kind, tc.media, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s %s = %+v, want %+v", tc.kind, tc.media, got, tc.want)
		}
	}
}

// The medium of a user list is a post-filter on the entries' own media (plan 5.6); positions are
// the index in the response, seasons, episodes and persons are skipped.
func TestFetchUserListFilter(t *testing.T) {
	m := newMock(t)
	c := m.client()
	ctx := context.Background()
	const addr = "https://trakt.tv/users/jane/lists/mixed/items"
	film := func(pos int, id int64, title string, year int) Entry {
		return Entry{Media: MediaMovie, Position: pos, TMDBID: id, Title: title, Year: year}
	}
	show := func(pos int, id int64, title string, year int) Entry {
		return Entry{Media: MediaShow, Position: pos, TMDBID: id, Title: title, Year: year}
	}
	for _, tc := range []struct {
		media string
		limit int
		want  []Entry
	}{
		{MediaMovie, MaxLimit, []Entry{film(1, 11, "Film A", 2000), film(6, 18, "Film F", 2002)}},
		{MediaShow, MaxLimit, []Entry{show(2, 12, "Serie B", 2001), show(7, 19, "Serie G", 0)}},
		{MediaBoth, MaxLimit, []Entry{film(1, 11, "Film A", 2000), show(2, 12, "Serie B", 2001),
			film(6, 18, "Film F", 2002), show(7, 19, "Serie G", 0)}},
		{MediaMovie, 5, []Entry{film(1, 11, "Film A", 2000)}},
	} {
		m.wantLimit.Store(int32(tc.limit))
		got, err := c.Fetch(ctx, ListRef{Kind: KindURL, Media: tc.media, URL: addr}, tc.limit)
		if err != nil {
			t.Fatalf("%s: %v", tc.media, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s limit %d = %+v, want %+v", tc.media, tc.limit, got, tc.want)
		}
	}
}

// A user name that needs escaping is escaped in the request path, not split or passed raw.
func TestFetchUserListEscapes(t *testing.T) {
	m := newMock(t)
	c := m.client()
	got, err := c.Fetch(context.Background(), ListRef{Kind: KindURL, Media: MediaMovie,
		URL: "https://trakt.tv/users/Jane%20Doe%2F%C3%A4/lists/my%20list"}, MaxLimit)
	if err != nil || len(got) != 1 || got[0].TMDBID != 901 {
		t.Errorf("escaped user: %+v, %v", got, err)
	}
	if p := m.seen(); !slices.Equal(p, []string{"/users/Jane%20Doe%2F%C3%A4/lists/my%20list/items"}) {
		t.Errorf("asked %v", p)
	}
}

// A ref that does not fit its kind is refused before any request.
func TestFetchInvalid(t *testing.T) {
	m := newMock(t)
	c := m.client()
	ctx := context.Background()
	for _, tc := range []struct {
		ref  ListRef
		want error
	}{
		{ListRef{Kind: KindBoxOffice, Media: MediaShow}, ErrInvalidList},
		{ListRef{Kind: KindBoxOffice, Media: MediaBoth}, ErrInvalidList},
		{ListRef{Kind: "boxoffices", Media: MediaMovie}, ErrInvalidList},
		{ListRef{Kind: "", Media: MediaMovie}, ErrInvalidList},
		{ListRef{Kind: KindPopular, Media: "person"}, ErrInvalidList},
		{ListRef{Kind: KindTrending, Media: ""}, ErrInvalidList},
		{ListRef{Kind: KindURL, Media: MediaMovie}, ErrInvalidList},
		{ListRef{Kind: KindURL, Media: "season", URL: "https://trakt.tv/users/jane/lists/mixed"}, ErrInvalidList},
		{ListRef{Kind: KindURL, Media: MediaMovie, URL: "https://example.com/users/jane/lists/mixed"}, ErrInvalidList},
		{ListRef{Kind: KindURL, Media: MediaBoth, URL: "https://trakt.tv/users/jane/watchlist"}, ErrWatchlist},
	} {
		if _, err := c.Fetch(ctx, tc.ref, MaxLimit); !errors.Is(err, tc.want) {
			t.Errorf("%+v: %v, want %v", tc.ref, err, tc.want)
		}
	}
	if p := m.seen(); len(p) != 0 {
		t.Errorf("invalid refs asked %v", p)
	}
}

func TestParseListURL(t *testing.T) {
	for _, tc := range []struct {
		raw, user, slug string
		want            error
	}{
		{"https://trakt.tv/users/jane/lists/mixed", "jane", "mixed", nil},
		{"https://trakt.tv/users/jane/lists/mixed/", "jane", "mixed", nil},
		{"https://trakt.tv/users/jane/lists/mixed/items", "jane", "mixed", nil},
		{"https://trakt.tv/users/jane/lists/mixed/items/", "jane", "mixed", nil},
		{"http://www.trakt.tv/users/jane/lists/mixed", "jane", "mixed", nil},
		{"https://api.trakt.tv/users/jane/lists/mixed/items", "jane", "mixed", nil},
		{"  https://Trakt.tv/users/jane.doe/lists/top-10?sort=rank,asc ", "jane.doe", "top-10", nil},
		{"https://trakt.tv/users/Jane%20Doe%2F%C3%A4/lists/my%20list", "Jane Doe/ä", "my list", nil},
		{"https://trakt.tv/users/x/watchlist", "", "", ErrWatchlist},
		{"https://trakt.tv/users/x/watchlist/", "", "", ErrWatchlist},
		{"https://trakt.tv/users/x/watchlist/movies", "", "", ErrWatchlist},
		{"https://www.trakt.tv/users/x/Watchlist?sort=rank", "", "", ErrWatchlist},
		{"", "", "", ErrInvalidList},
		{"secretname", "", "", ErrInvalidList},
		{"trakt.tv/users/secretname/lists/y", "", "", ErrInvalidList},
		{"ftp://trakt.tv/users/secretname/lists/y", "", "", ErrInvalidList},
		{"https://evil.example/users/secretname/lists/y", "", "", ErrInvalidList},
		{"https://trakt.tv.evil.example/users/secretname/lists/y", "", "", ErrInvalidList},
		{"https://trakt.tv:8443/users/secretname/lists/y", "", "", ErrInvalidList},
		{"https://secretname:pw@trakt.tv/users/secretname/lists/y", "", "", ErrInvalidList},
		{"https://trakt.tv/lists/123", "", "", ErrInvalidList},
		{"https://trakt.tv/movies/trending", "", "", ErrInvalidList},
		{"https://trakt.tv/users/secretname", "", "", ErrInvalidList},
		{"https://trakt.tv/users/secretname/lists", "", "", ErrInvalidList},
		{"https://trakt.tv/users/secretname/lists/y/comments", "", "", ErrInvalidList},
		{"https://trakt.tv/users/secretname/lists/y/items/movies", "", "", ErrInvalidList},
		{"https://trakt.tv/users//lists/y", "", "", ErrInvalidList},
		{"https://trakt.tv/users/secretname/lists//items", "", "", ErrInvalidList},
		{"https://trakt.tv/users/secretname/history", "", "", ErrInvalidList},
		{"https://trakt.tv/users/secret%zzname/lists/y", "", "", ErrInvalidList},
	} {
		user, slug, err := ParseListURL(tc.raw)
		if tc.want == nil {
			if err != nil || user != tc.user || slug != tc.slug {
				t.Errorf("%q = %q, %q, %v, want %q, %q", tc.raw, user, slug, err, tc.user, tc.slug)
			}
			continue
		}
		if !errors.Is(err, tc.want) || user != "" || slug != "" {
			t.Errorf("%q = %q, %q, %v, want %v", tc.raw, user, slug, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Errorf("%q: error echoes the address: %v", tc.raw, err)
		}
	}
}

func TestFetchErrors(t *testing.T) {
	m := newMock(t)
	c := m.client()
	ctx := context.Background()
	popular := ListRef{Kind: KindPopular, Media: MediaMovie}
	for code, want := range map[int32]error{503: ErrUnavailable, 500: ErrUnavailable, 401: ErrUnauthorized, 403: ErrUnauthorized} {
		m.status.Store(code)
		_, err := c.Fetch(ctx, popular, MaxLimit)
		if !errors.Is(err, want) {
			t.Errorf("status %d: %v, want %v", code, err, want)
		}
		if err != nil && strings.Contains(err.Error(), clientID) {
			t.Errorf("status %d: error names the client id: %v", code, err)
		}
	}
	m.status.Store(404)
	if _, err := c.Fetch(ctx, popular, MaxLimit); err == nil || errors.Is(err, ErrUnavailable) {
		t.Errorf("status 404: %v", err)
	}
	// A failing first request of "both" ends the fetch; the shows are not asked.
	m.seen()
	if _, err := c.Fetch(ctx, ListRef{Kind: KindPopular, Media: MediaBoth}, MaxLimit); err == nil {
		t.Error("status 404 for both: no error")
	}
	if p := m.seen(); !slices.Equal(p, []string{"/movies/popular"}) {
		t.Errorf("both after a failure asked %v", p)
	}
	m.Close()
	m.status.Store(0)
	if _, err := c.Fetch(ctx, popular, MaxLimit); !errors.Is(err, ErrUnavailable) {
		t.Errorf("closed server: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := c.Fetch(cancelled, popular, MaxLimit); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled context: %v", err)
	}
}

// Errors of a user list never name the user or the slug (rule 10, plan 8.4), nor the client id.
func TestFetchUserListErrorsHideAddress(t *testing.T) {
	ref := ListRef{Kind: KindURL, Media: MediaBoth, URL: "https://trakt.tv/users/secretuser/lists/secretslug"}
	check := func(what string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: no error", what)
			return
		}
		for _, s := range []string{"secretuser", "secretslug", clientID} {
			if strings.Contains(err.Error(), s) {
				t.Errorf("%s: error names %q: %v", what, s, err)
			}
		}
	}
	m := newMock(t)
	c := m.client()
	ctx := context.Background()
	for _, code := range []int32{503, 401, 404} {
		m.status.Store(code)
		_, err := c.Fetch(ctx, ref, MaxLimit)
		check("status "+strconv.Itoa(int(code)), err)
	}
	malformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error": "not a list"}`))
	}))
	defer malformed.Close()
	mc := New(clientID, WithBaseURL(malformed.URL), WithHTTPClient(malformed.Client()), WithLimiter(fast()))
	_, err := mc.Fetch(ctx, ref, MaxLimit)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("malformed body: %v", err)
	}
	check("malformed body", err)
	m.Close()
	m.status.Store(0)
	_, err = c.Fetch(ctx, ref, MaxLimit)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("closed server: %v", err)
	}
	check("closed server", err)
}

// A malformed body is an unavailable list, not an empty one.
func TestFetchMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error": "not a list"}`))
	}))
	defer srv.Close()
	c := New(clientID, WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithLimiter(fast()))
	if _, err := c.Fetch(context.Background(), ListRef{Kind: KindTrending, Media: MediaShow}, 5); !errors.Is(err, ErrUnavailable) {
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
	if _, err := c.Fetch(ctx, ListRef{Kind: KindPopular, Media: MediaMovie}, MaxLimit); !errors.Is(err, ErrUnavailable) {
		t.Errorf("redirect to another host: %v, want ErrUnavailable", err)
	}
	if n := foreignHits.Load(); n != 0 {
		t.Errorf("foreign host hit %d times", n)
	}
	got, err := c.Fetch(ctx, ListRef{Kind: KindPopular, Media: MediaShow}, MaxLimit)
	if err != nil || len(got) != 1 || got[0].TMDBID != 5 {
		t.Errorf("redirect on the same host: %+v, %v", got, err)
	}
}

// n7 (M7 review): the body is drained before it is closed, so sequential requests share one
// connection even when the JSON decoder stops before the end of the body.
func TestFetchReusesConnection(t *testing.T) {
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
		if _, err := c.Fetch(context.Background(), ListRef{Kind: KindPopular, Media: MediaMovie}, MaxLimit); err != nil {
			t.Fatal(err)
		}
	}
	if n := conns.Load(); n != 1 {
		t.Errorf("%d connections for three requests, want 1", n)
	}
}
