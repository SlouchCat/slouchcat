package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const (
	bookTestAgent = "Slouchcat/1.2.3"
	googleTestKey = "test-google-books-key-0001"
)

// bookServer is the mock of every audiobook source (paths of BookSourceURLsFromBase). A path
// without a route answers 404.
type bookServer struct {
	t   *testing.T
	srv *httptest.Server

	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	hits   map[string]int
	reqs   []*http.Request
}

func newBookServer(t *testing.T) *bookServer {
	t.Helper()
	b := &bookServer{t: t, routes: map[string]http.HandlerFunc{}, hits: map[string]int{}}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		b.hits[r.URL.Path]++
		b.reqs = append(b.reqs, r.Clone(context.Background()))
		h := b.routes[r.URL.Path]
		b.mu.Unlock()
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *bookServer) route(path string, h http.HandlerFunc) {
	b.mu.Lock()
	b.routes[path] = h
	b.mu.Unlock()
}

// fixture serves a file of testdata/books.
func (b *bookServer) fixture(path, name string) {
	data := bookFixture(b.t, name)
	b.route(path, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data) })
}

func (b *bookServer) count(path string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.hits[path]
}

func (b *bookServer) total() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, c := range b.hits {
		n += c
	}
	return n
}

// last returns the last request to path.
func (b *bookServer) last(path string) *http.Request {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := len(b.reqs) - 1; i >= 0; i-- {
		if b.reqs[i].URL.Path == path {
			return b.reqs[i]
		}
	}
	b.t.Fatalf("no request to %s", path)
	return nil
}

func bookFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "books", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type bookEnv struct {
	svc   *Service
	srv   *bookServer
	db    *store.DB
	clock *testClock
	logs  *bytes.Buffer
	key   *string
}

// newBookEnv returns a service whose audiobook sources ask a mock server, with fast limiters (the
// limiter test keeps the real ones) and a Google Books key that is "" until the test sets it.
func newBookEnv(t *testing.T) *bookEnv {
	t.Helper()
	e := newBookEnvWith(t, nil)
	fastBookLimits(e.svc)
	return e
}

func newBookEnvWith(t *testing.T, db *store.DB) *bookEnv {
	t.Helper()
	logs := &bytes.Buffer{}
	log := slog.New(logging.NewRedactingHandler(slog.NewTextHandler(&lockedWriter{w: logs}, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if db == nil {
		var err error
		db, err = storetest.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
	}
	srv := newBookServer(t)
	clock := &testClock{t: testNow}
	key := new(string)
	var keyMu sync.Mutex
	svc := New(db, jobs.NewQueue(db.Write, nil), staticKey(""), t.TempDir(), transcode.Runner{}, log,
		WithBookSourceURLs(BookSourceURLsFromBase(srv.srv.URL)), WithClock(clock.now), WithFeedUserAgent(bookTestAgent),
		WithGoogleBooksKey(func(context.Context) (string, error) {
			keyMu.Lock()
			defer keyMu.Unlock()
			return *key, nil
		}))
	t.Cleanup(func() {
		if strings.Contains(logs.String(), googleTestKey) {
			t.Errorf("google books key in logs:\n%s", logs.String())
		}
	})
	return &bookEnv{svc: svc, srv: srv, db: db, clock: clock, logs: logs, key: key}
}

// fastBookLimits replaces the polite limiters of the audiobook sources, so a test can make many
// requests without waiting.
func fastBookLimits(s *Service) {
	fast := func() *ratelimit.Limiter { return ratelimit.New("test", 1000, 1000) }
	s.books.audible.limiter, s.books.audible.audnexusLimiter = fast(), fast()
	s.books.dnb.limiter, s.books.apple.limiter = fast(), fast()
	s.books.openLibrary.limiter, s.books.google.limiter = fast(), fast()
}

func (e *bookEnv) provider(t *testing.T, name string) BookProvider {
	t.Helper()
	p, ok := e.svc.bookProvider(name)
	if !ok {
		t.Fatalf("no book provider %s", name)
	}
	return p
}

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int) *int           { return &v }

func checkCandidate(t *testing.T, got, want BookCandidate) {
	t.Helper()
	gj, _ := json.Marshal(got)
	wj, _ := json.Marshal(want)
	if !bytes.Equal(gj, wj) {
		t.Errorf("candidate\n got %s\nwant %s", gj, wj)
	}
}

func TestBookProvidersByName(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	ctx := context.Background()
	for _, name := range BookProviderNames {
		p := e.provider(t, name)
		if p.Name() != name {
			t.Errorf("%s: Name %q", name, p.Name())
		}
		if p.HasDuration() != (name == ProviderAudible) {
			t.Errorf("%s: HasDuration %v", name, p.HasDuration())
		}
		if p.Configured(ctx) != (name != ProviderGoogleBooks) {
			t.Errorf("%s: Configured %v without a Google key", name, p.Configured(ctx))
		}
	}
	if _, ok := e.svc.bookProvider(ProviderTMDB); ok {
		t.Error("tmdb is no book provider")
	}
}

func TestAudibleSearch(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	ctx := context.Background()
	const path = "/audible/de/1.0/catalog/products"
	e.srv.fixture(path, "audible_search.json")
	cs, err := e.provider(t, ProviderAudible).SearchBooks(ctx, BookQuery{Title: "Der Testband", Author: "Erika Muster"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("candidates %+v", cs)
	}
	checkCandidate(t, cs[0], BookCandidate{
		Provider: ProviderAudible, ID: "B0TEST0001", Region: "de", Title: "Der Testband", Subtitle: "Die Reihe, Teil 3",
		Authors: []string{"Erika Muster"}, Narrators: []string{"Max Sprecher"}, Publisher: "Testverlag Hören",
		SeriesName: "Die Reihe", SeriesIndex: ptrFloat(3), Year: ptrInt(2019), Language: "de", DurationMs: 612 * 60_000,
		ASIN: "B0TEST0001", CoverURL: "https://images.example.test/B0TEST0001._SL1024_.jpg",
	})
	if cs[1].ID != "B0TEST0003" || cs[1].DurationMs != 240*60_000 || cs[1].SeriesIndex != nil || cs[1].CoverURL != "" {
		t.Errorf("second candidate %+v", cs[1])
	}
	r := e.srv.last(path)
	q := r.URL.Query()
	if q.Get("title") != "Der Testband" || q.Get("author") != "Erika Muster" || q.Get("num_results") != "10" ||
		q.Get("products_sort_by") != "Relevance" || q.Get("response_groups") != audibleResponseGroups {
		t.Errorf("query %s", r.URL.RawQuery)
	}
	if ua := r.Header.Get("User-Agent"); ua != bookTestAgent+" (+https://github.com/SlouchCat/slouchcat)" {
		t.Errorf("User-Agent %q", ua)
	}

	// The same search, written otherwise, comes from the cache.
	if again, err := e.provider(t, ProviderAudible).SearchBooks(ctx, BookQuery{Title: "  der  TESTBAND ", Author: "erika muster"}); err != nil || len(again) != 2 {
		t.Fatalf("cached search: %d candidates (%v)", len(again), err)
	}
	if n := e.srv.count(path); n != 1 {
		t.Errorf("requests after the cached search: %d, want 1", n)
	}
	// After seven days the cache is stale.
	e.clock.set(testNow.Add(bookCacheTTL))
	if _, err := e.provider(t, ProviderAudible).SearchBooks(ctx, BookQuery{Title: "Der Testband", Author: "Erika Muster"}); err != nil {
		t.Fatal(err)
	}
	if n := e.srv.count(path); n != 2 {
		t.Errorf("requests after seven days: %d, want 2", n)
	}
}

// An English book is searched on audible.de, audible.com and audible.co.uk; a candidate found in
// two regions counts once (the first region's).
func TestAudibleSearchEnglishRegions(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	for _, r := range []string{"de", "com", "co.uk"} {
		e.srv.fixture("/audible/"+r+"/1.0/catalog/products", "audible_search.json")
	}
	cs, err := e.provider(t, ProviderAudible).SearchBooks(context.Background(), BookQuery{Title: "Der Testband", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].Region != "de" || cs[1].Region != "de" {
		t.Errorf("candidates %+v", cs)
	}
	for _, r := range []string{"de", "com", "co.uk"} {
		if n := e.srv.count("/audible/" + r + "/1.0/catalog/products"); n != 1 {
			t.Errorf("requests to %s: %d", r, n)
		}
	}
	if _, err := e.provider(t, ProviderAudible).SearchBooks(context.Background(), BookQuery{Title: "x", Region: "xx"}); !errors.Is(err, errBookRegion) {
		t.Errorf("unknown region: %v", err)
	}
}

// An ASIN of another marketplace: audible.de answers a product without fields, audible.com knows
// it; audible.co.uk is not asked.
func TestAudibleASINRegionSwitch(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.srv.fixture("/audible/de/1.0/catalog/products/B0TEST0002", "audible_product_empty.json")
	e.srv.fixture("/audible/com/1.0/catalog/products/B0TEST0002", "audible_product_com.json")
	cs, err := e.provider(t, ProviderAudible).SearchBooks(context.Background(), BookQuery{ASIN: "b0test0002", Title: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 {
		t.Fatalf("candidates %+v", cs)
	}
	checkCandidate(t, cs[0], BookCandidate{
		Provider: ProviderAudible, ID: "B0TEST0002", Region: "com", Title: "The Test Return",
		Authors: []string{"John Q. Sample"}, Narrators: []string{"Rob Reader"}, Publisher: "Sample Audio",
		SeriesName: "The Test Trilogy", SeriesIndex: ptrFloat(3), Year: ptrInt(2015), Language: "en", DurationMs: 1342 * 60_000,
		ASIN: "B0TEST0002", CoverURL: "https://images.example.test/B0TEST0002._SL500_.jpg",
	})
	if e.srv.count("/audible/de/1.0/catalog/products/B0TEST0002") != 1 || e.srv.count("/audible/co.uk/1.0/catalog/products/B0TEST0002") != 0 {
		t.Errorf("requests %v", e.srv.hits)
	}
	// An ASIN no region knows: no candidate, no error.
	cs, err = e.provider(t, ProviderAudible).SearchBooks(context.Background(), BookQuery{ASIN: "B0TEST0404"})
	if err != nil || len(cs) != 0 {
		t.Errorf("unknown ASIN: %+v (%v)", cs, err)
	}
	if e.srv.count("/audible/co.uk/1.0/catalog/products/B0TEST0404") != 1 {
		t.Errorf("co.uk not asked for an unknown ASIN")
	}
}

// Audnexus answers 404 REGION_UNAVAILABLE in the region of the installation; the details follow
// from audible.com (region us) or, when no region has it, from the request without region.
func TestAudibleBookAudnexusRegions(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	book := bookFixture(t, "audnexus_book.json")
	unavailable := bookFixture(t, "audnexus_region_unavailable.json")
	var mu sync.Mutex
	var asked []string
	answerIn := "us"
	e.srv.route("/audnexus/books/B0TEST0002", func(w http.ResponseWriter, r *http.Request) {
		region, has := r.URL.Query()["region"]
		mu.Lock()
		got := "none"
		if has {
			got = region[0]
		}
		asked = append(asked, got)
		ok := got == answerIn
		mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(unavailable)
			return
		}
		_, _ = w.Write(book)
	})
	p := e.provider(t, ProviderAudible)
	rec, err := p.Book(context.Background(), "B0TEST0002", "")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Region != "com" || !slices.Equal(asked, []string{"de", "us"}) {
		t.Errorf("region %q after %v", rec.Region, asked)
	}
	checkCandidate(t, rec.BookCandidate, BookCandidate{
		Provider: ProviderAudible, ID: "B0TEST0002", Region: "com", Title: "The Test Return", Subtitle: "Unabridged",
		Authors: []string{"John Q. Sample"}, Narrators: []string{"Rob Reader"}, Publisher: "Sample Audio",
		SeriesName: "The Test Trilogy", SeriesIndex: ptrFloat(3), Year: ptrInt(2015), Language: "en", DurationMs: 1342 * 60_000,
		ASIN: "B0TEST0002", ISBN: "9780000000002", CoverURL: "https://images.example.test/B0TEST0002.jpg",
	})
	if rec.Overview != "An invented sentence for the test.\n\nA second one & more." {
		t.Errorf("overview %q", rec.Overview)
	}
	if !slices.Equal(rec.Genres, []string{"Fantasy"}) {
		t.Errorf("genres %v", rec.Genres)
	}
	if len(rec.Covers) != 1 || rec.Covers[0].Kind != "poster" || rec.Covers[0].Source != "https://images.example.test/B0TEST0002.jpg" {
		t.Errorf("covers %+v", rec.Covers)
	}

	mu.Lock()
	asked, answerIn = nil, "none"
	mu.Unlock()
	rec, err = p.Book(context.Background(), "B0TEST0002", "de")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Region != "" || !slices.Equal(asked, []string{"de", "us", "uk", "none"}) {
		t.Errorf("without region: region %q after %v", rec.Region, asked)
	}

	// A plain 404 is an unknown ASIN; an ASIN of the wrong form asks nothing.
	if _, err := p.Book(context.Background(), "B0TEST0404", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown ASIN: %v", err)
	}
	before := e.srv.total()
	if _, err := p.Book(context.Background(), "../etc", ""); !errors.Is(err, ErrNotFound) || e.srv.total() != before {
		t.Errorf("invalid ASIN: %v", err)
	}
}

// The Audnexus address is the administrator's: a local one may be asked (the mock runs on the
// loopback address), but a redirect to another host is refused, and on behalf of a public address
// no local address is reached.
func TestAudibleAudnexusNetguard(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.srv.fixture("/audnexus/books/B0TEST0002", "audnexus_book.json")
	other := strings.Replace(e.srv.srv.URL, "127.0.0.1", "localhost", 1)
	e.srv.route("/audnexus/books/B0TEST0005", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other+"/audnexus/books/B0TEST0002", http.StatusFound)
	})
	p := e.provider(t, ProviderAudible)
	if _, err := p.Book(context.Background(), "B0TEST0002", ""); err != nil {
		t.Fatalf("local audnexus: %v", err)
	}
	if _, err := p.Book(context.Background(), "B0TEST0005", ""); err == nil || !strings.Contains(err.Error(), "another host") {
		t.Errorf("redirect to another local host: %v", err)
	}
	if n := e.srv.count("/audnexus/books/B0TEST0002"); n != 1 {
		t.Errorf("requests to the redirect target: %d, want 1 (the direct one)", n)
	}

	ctx := context.Background()
	hc := e.svc.books.audible.audnexusClients.For(ctx, "https://203.0.113.5", "admin")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.srv.srv.URL+"/audnexus/books/B0TEST0002", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := hc.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, netguard.ErrRefused) {
		t.Errorf("public audnexus address reaching a local one: %v", err)
	}
}

func TestDNBSearchAndBook(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.srv.fixture("/dnb", "dnb_search.xml")
	p := e.provider(t, ProviderDNB)
	ctx := context.Background()
	cs, err := p.SearchBooks(ctx, BookQuery{Title: `Der "Testband"`, Author: "Erika Muster"})
	if err != nil {
		t.Fatal(err)
	}
	q := e.srv.last("/dnb").URL.Query()
	if q.Get("query") != `tit="Der Testband" and per="Muster"` || q.Get("recordSchema") != "MARC21-xml" ||
		q.Get("operation") != "searchRetrieve" || q.Get("maximumRecords") != "10" {
		t.Errorf("query %v", q)
	}
	if len(cs) != 2 {
		t.Fatalf("candidates %+v", cs)
	}
	checkCandidate(t, cs[0], BookCandidate{
		Provider: ProviderDNB, ID: "1000000001", Title: "Der Testband", Subtitle: "ungekürzte Lesung",
		Authors: []string{"Erika Muster"}, Narrators: []string{"Max Sprecher"}, Publisher: "Testverlag Hören",
		SeriesName: "Die Reihe", SeriesIndex: ptrFloat(3), Year: ptrInt(2019), Language: "de",
		DurationMs: (3600 + 12*60) * 1000, ISBN: "9783000000001",
	})
	checkCandidate(t, cs[1], BookCandidate{
		Provider: ProviderDNB, ID: "100000000X", Title: "Der Testband", Authors: []string{"Erika Muster"},
		Publisher: "Testverlag Druck", Year: ptrInt(2017), ISBN: "3000000002",
	})

	if _, err := p.SearchBooks(ctx, BookQuery{ISBN: "978-3-00-000000-1", Title: "ignored"}); err != nil {
		t.Fatal(err)
	}
	if got := e.srv.last("/dnb").URL.Query().Get("query"); got != "num=9783000000001" {
		t.Errorf("isbn query %q", got)
	}
	rec, err := p.Book(ctx, "1000000001", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.srv.last("/dnb").URL.Query().Get("query"); got != "idn=1000000001" || rec.Title != "Der Testband" || rec.Covers != nil {
		t.Errorf("book %q: %+v", got, rec)
	}
	if _, err := p.Book(ctx, "1000000002", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("idn not in the answer: %v", err)
	}
	before := e.srv.total()
	if cs, err := p.SearchBooks(ctx, BookQuery{ASIN: "B0TEST0001"}); err != nil || cs != nil || e.srv.total() != before {
		t.Errorf("ASIN at DNB: %+v (%v)", cs, err)
	}
}

func TestDNBDiagnostic(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.srv.route("/dnb", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><searchRetrieveResponse xmlns="http://www.loc.gov/zing/srw/"><diagnostics>` +
			`<diagnostic xmlns="http://www.loc.gov/zing/srw/diagnostic/"><message>Query syntax error</message></diagnostic></diagnostics></searchRetrieveResponse>`))
	})
	if _, err := e.provider(t, ProviderDNB).SearchBooks(context.Background(), BookQuery{Title: "x"}); err == nil || errors.Is(err, errSourceBlocked) {
		t.Errorf("diagnostic: %v", err)
	}
}

func TestAppleBooksSearchAndBook(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.srv.fixture("/apple/search", "apple_search.json")
	e.srv.fixture("/apple/lookup", "apple_search.json")
	p := e.provider(t, ProviderAppleBooks)
	ctx := context.Background()
	cs, err := p.SearchBooks(ctx, BookQuery{Title: "Der Testband", Author: "Erika Muster"})
	if err != nil {
		t.Fatal(err)
	}
	q := e.srv.last("/apple/search").URL.Query()
	if q.Get("term") != "Der Testband Erika Muster" || q.Get("country") != "ch" || q.Get("media") != "audiobook" || q.Get("limit") != "10" {
		t.Errorf("query %v", q)
	}
	if len(cs) != 1 {
		t.Fatalf("candidates %+v", cs)
	}
	want := BookCandidate{
		Provider: ProviderAppleBooks, ID: "1000000101", Title: "Der Testband", Subtitle: "Die Reihe 3",
		Authors: []string{"Erika Muster", "Hans Beispiel"}, Publisher: "Testverlag Hören", Year: ptrInt(2019),
		CoverURL: "https://is1.example.test/image/thumb/Test/3000x3000bb.jpg",
	}
	checkCandidate(t, cs[0], want)
	rec, err := p.Book(ctx, "1000000101", "")
	if err != nil {
		t.Fatal(err)
	}
	q = e.srv.last("/apple/lookup").URL.Query()
	if q.Get("id") != "1000000101" || q.Get("country") != "ch" {
		t.Errorf("lookup %v", q)
	}
	checkCandidate(t, rec.BookCandidate, want)
	if rec.Overview != "Ein erfundener Satz.\nNoch einer & Schluss." || !slices.Equal(rec.Genres, []string{"Krimis & Thriller"}) ||
		len(rec.Covers) != 1 || rec.Covers[0].Source != want.CoverURL {
		t.Errorf("record %q %v %+v", rec.Overview, rec.Genres, rec.Covers)
	}
	if _, err := p.Book(ctx, "1000000999", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("other id: %v", err)
	}
}

func TestOpenLibrarySearchAndBook(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.srv.fixture("/openlibrary/search.json", "openlibrary_search.json")
	p := e.provider(t, ProviderOpenLibrary)
	ctx := context.Background()
	cs, err := p.SearchBooks(ctx, BookQuery{Title: "Der Testband", Author: "Erika Muster"})
	if err != nil {
		t.Fatal(err)
	}
	q := e.srv.last("/openlibrary/search.json").URL.Query()
	if q.Get("title") != "Der Testband" || q.Get("author") != "Erika Muster" || q.Get("fields") != openLibraryFields || q.Get("limit") != "10" {
		t.Errorf("query %v", q)
	}
	if len(cs) != 1 {
		t.Fatalf("candidates %+v", cs)
	}
	want := BookCandidate{
		Provider: ProviderOpenLibrary, ID: "OL1000001W", Title: "Der Testband", Subtitle: "Roman",
		Authors: []string{"Erika Muster"}, Publisher: "Testverlag Druck", Year: ptrInt(2017), Language: "de",
		ISBN: "9783000000001", CoverURL: e.srv.srv.URL + "/openlibrary/b/id/4242-L.jpg",
	}
	checkCandidate(t, cs[0], want)
	if _, err := p.SearchBooks(ctx, BookQuery{ISBN: "3-00-000000-2"}); err != nil {
		t.Fatal(err)
	}
	if got := e.srv.last("/openlibrary/search.json").URL.Query().Get("isbn"); got != "3000000002" {
		t.Errorf("isbn %q", got)
	}
	rec, err := p.Book(ctx, "/works/OL1000001W", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.srv.last("/openlibrary/search.json").URL.Query().Get("q"); got != "key:/works/OL1000001W" {
		t.Errorf("book query %q", got)
	}
	checkCandidate(t, rec.BookCandidate, want)
	if len(rec.Covers) != 1 || rec.Covers[0].Source != want.CoverURL {
		t.Errorf("covers %+v", rec.Covers)
	}
}

// Without a key Google Books is not configured and asks nothing; with one the key goes into the
// query only, never into a log line or an error.
func TestGoogleBooks(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	vols := bookFixture(t, "google_volumes.json")
	e.srv.route("/google/volumes", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vols) })
	e.srv.route("/google/volumes/TESTvol001", func(w http.ResponseWriter, _ *http.Request) {
		var all struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(vols, &all)
		_, _ = w.Write(all.Items[0])
	})
	p := e.provider(t, ProviderGoogleBooks)
	ctx := context.Background()
	if _, err := p.SearchBooks(ctx, BookQuery{Title: "Der Testband"}); !errors.Is(err, errNotConfigured) {
		t.Errorf("search without key: %v", err)
	}
	if _, err := p.Book(ctx, "TESTvol001", ""); !errors.Is(err, errNotConfigured) {
		t.Errorf("book without key: %v", err)
	}
	if n := e.srv.total(); n != 0 {
		t.Fatalf("requests without key: %d", n)
	}

	*e.key = googleTestKey
	if !p.Configured(ctx) {
		t.Fatal("not configured with a key")
	}
	cs, err := p.SearchBooks(ctx, BookQuery{Title: "Der Testband", Author: "Erika Muster"})
	if err != nil {
		t.Fatal(err)
	}
	q := e.srv.last("/google/volumes").URL.Query()
	if q.Get("q") != "intitle:Der Testband inauthor:Erika Muster" || q.Get("key") != googleTestKey || q.Get("maxResults") != "10" {
		t.Errorf("query %v", q)
	}
	want := BookCandidate{
		Provider: ProviderGoogleBooks, ID: "TESTvol001", Title: "Der Testband", Subtitle: "Roman",
		Authors: []string{"Erika Muster"}, Publisher: "Testverlag Druck", Year: ptrInt(2017), Language: "de",
		ISBN: "9783000000001", CoverURL: "https://books.example.test/c?id=TESTvol001&zoom=1",
	}
	if len(cs) != 1 {
		t.Fatalf("candidates %+v", cs)
	}
	checkCandidate(t, cs[0], want)
	if _, err := p.SearchBooks(ctx, BookQuery{ISBN: "9783000000001"}); err != nil {
		t.Fatal(err)
	}
	if got := e.srv.last("/google/volumes").URL.Query().Get("q"); got != "isbn:9783000000001" {
		t.Errorf("isbn query %q", got)
	}
	rec, err := p.Book(ctx, "TESTvol001", "")
	if err != nil {
		t.Fatal(err)
	}
	checkCandidate(t, rec.BookCandidate, want)
	if rec.Overview != "Ein erfundener Satz." || !slices.Equal(rec.Genres, []string{"Fiction"}) || len(rec.Covers) != 1 {
		t.Errorf("record %q %v %+v", rec.Overview, rec.Genres, rec.Covers)
	}
	if !strings.Contains(e.logs.String(), "/google/volumes") {
		t.Errorf("request not logged:\n%s", e.logs.String())
	}

	// A failing client names scheme and host only, not the key of the query.
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	e.svc.books.google.base = closed.URL + "/google"
	_, err = p.Book(ctx, "TESTvol001", "")
	if err == nil || strings.Contains(err.Error(), googleTestKey) {
		t.Errorf("client error %v", err)
	}
	e.srv.route("/google/volumes", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	e.svc.books.google.base = e.srv.srv.URL + "/google"
	if _, err := p.SearchBooks(ctx, BookQuery{Title: "Anders"}); err == nil || strings.Contains(err.Error(), googleTestKey) {
		t.Errorf("status error %v", err)
	}
}

// Three 429 in a row pause the source for six hours: no request reaches it then, the pause is
// stored and read by a new service on the same database, and it ends after six hours.
func TestBookBreakerPausesAfterThreeBlocked(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	const path = "/apple/search"
	e.srv.route(path, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	p := e.provider(t, ProviderAppleBooks)
	ctx := context.Background()
	for i := range 3 {
		if _, err := p.SearchBooks(ctx, BookQuery{Title: "Buch " + strconv.Itoa(i)}); !errors.Is(err, errSourceBlocked) {
			t.Fatalf("answer %d: %v", i, err)
		}
	}
	if _, err := p.SearchBooks(ctx, BookQuery{Title: "Buch 4"}); !errors.Is(err, errSourcePaused) {
		t.Fatalf("after three 429: %v", err)
	}
	if n := e.srv.count(path); n != 3 {
		t.Errorf("requests: %d, want 3", n)
	}
	until := testNow.Add(bookBreakerPause)
	var stored string
	if err := e.db.Read.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, SettingBookPausedPrefix+ProviderAppleBooks).Scan(&stored); err != nil ||
		stored != strconv.FormatInt(until.UnixMilli(), 10) {
		t.Errorf("stored pause %q (%v), want %d", stored, err, until.UnixMilli())
	}
	if got, ok := e.svc.bookSourcePausedUntil(ctx, ProviderAppleBooks); !ok || !got.Equal(until) {
		t.Errorf("paused until %v %v", got, ok)
	}
	if _, ok := e.svc.bookSourcePausedUntil(ctx, ProviderDNB); ok {
		t.Error("dnb pauses as well")
	}

	// A restart reads the pause: the new service asks nothing.
	e2 := newBookEnvWith(t, e.db)
	fastBookLimits(e2.svc)
	e2.srv.fixture(path, "apple_search.json")
	p2 := e2.provider(t, ProviderAppleBooks)
	if _, err := p2.SearchBooks(ctx, BookQuery{Title: "Der Testband"}); !errors.Is(err, errSourcePaused) {
		t.Errorf("after the restart: %v", err)
	}
	if n := e2.srv.total(); n != 0 {
		t.Errorf("requests during the pause: %d", n)
	}
	e2.clock.set(until)
	if cs, err := p2.SearchBooks(ctx, BookQuery{Title: "Der Testband"}); err != nil || len(cs) != 1 {
		t.Errorf("after the pause: %+v (%v)", cs, err)
	}
}

// A good answer resets the count; 404 neither counts nor resets; HTML instead of JSON or XML
// counts.
func TestBookBreakerCounts(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	ctx := context.Background()
	var mu sync.Mutex
	status := http.StatusTooManyRequests
	data := bookFixture(t, "openlibrary_search.json")
	e.srv.route("/openlibrary/search.json", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		s := status
		mu.Unlock()
		if s != http.StatusOK {
			w.WriteHeader(s)
			return
		}
		_, _ = w.Write(data)
	})
	set := func(s int) {
		mu.Lock()
		status = s
		mu.Unlock()
	}
	p := e.provider(t, ProviderOpenLibrary)
	n := 0
	search := func() error {
		n++
		_, err := p.SearchBooks(ctx, BookQuery{Title: "Buch " + strconv.Itoa(n)})
		return err
	}
	for _, step := range []struct {
		status int
		want   error
	}{
		{http.StatusTooManyRequests, errSourceBlocked}, {http.StatusForbidden, errSourceBlocked},
		{http.StatusNotFound, nil}, // neither counts nor resets
		{http.StatusOK, nil},       // resets
		{http.StatusTooManyRequests, errSourceBlocked}, {http.StatusTooManyRequests, errSourceBlocked},
		{http.StatusOK, nil},
	} {
		set(step.status)
		if err := search(); !errors.Is(err, step.want) {
			t.Fatalf("status %d: %v, want %v", step.status, err, step.want)
		}
	}
	if _, ok := e.svc.bookSourcePausedUntil(ctx, ProviderOpenLibrary); ok {
		t.Fatal("paused although a good answer came between")
	}
	// 2 blocked + 404 + 1 blocked = three in a row.
	set(http.StatusTooManyRequests)
	_ = search()
	_ = search()
	set(http.StatusNotFound)
	_ = search()
	set(http.StatusTooManyRequests)
	_ = search()
	if err := search(); !errors.Is(err, errSourcePaused) {
		t.Errorf("after 429, 429, 404, 429: %v", err)
	}

	// HTML: a captcha page with 200 at a JSON source and at the XML source.
	html := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Robot check</body></html>"))
	}
	e.srv.route("/dnb", html)
	dnb := e.provider(t, ProviderDNB)
	for i := range 3 {
		if _, err := dnb.SearchBooks(ctx, BookQuery{Title: "Buch " + strconv.Itoa(i)}); !errors.Is(err, errSourceBlocked) {
			t.Fatalf("dnb html %d: %v", i, err)
		}
	}
	if _, err := dnb.SearchBooks(ctx, BookQuery{Title: "Buch 9"}); !errors.Is(err, errSourcePaused) {
		t.Errorf("dnb after three HTML pages: %v", err)
	}
	if !htmlAnswer("application/json", []byte("  <html>"), bookJSON) || htmlAnswer("application/xml", []byte(`<?xml version="1.0"?><x/>`), bookXML) ||
		htmlAnswer("application/json", []byte(`{"a":1}`), bookJSON) {
		t.Error("htmlAnswer")
	}
}

// Every source waits for its own limiter: a request takes the one token of the bucket, and the
// rates are one per second (Apple one per three seconds).
func TestBookLimiter(t *testing.T) {
	t.Parallel()
	e := newBookEnvWith(t, nil)
	e.srv.fixture("/dnb", "dnb_search.xml")
	e.srv.fixture("/apple/search", "apple_search.json")
	ctx := context.Background()
	if _, err := e.provider(t, ProviderDNB).SearchBooks(ctx, BookQuery{Title: "Der Testband"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.provider(t, ProviderAppleBooks).SearchBooks(ctx, BookQuery{Title: "Der Testband"}); err != nil {
		t.Fatal(err)
	}
	if e.svc.books.dnb.limiter.Allow() || e.svc.books.apple.limiter.Allow() {
		t.Error("a request left a token in the bucket")
	}
	if !e.svc.books.openLibrary.limiter.Allow() {
		t.Error("open library has no token before its first request")
	}

	clock := &testClock{t: testNow}
	for _, c := range []struct {
		name string
		rate float64
		wait time.Duration
	}{{"dnb", bookRate, time.Second}, {"apple", appleBooksRate, 3 * time.Second}} {
		l := ratelimit.New(c.name, c.rate, 1, ratelimit.WithClock(clock.now))
		start := clock.now()
		if !l.Allow() || l.Allow() {
			t.Fatalf("%s: burst", c.name)
		}
		clock.set(start.Add(c.wait - 100*time.Millisecond))
		if l.Allow() {
			t.Errorf("%s: token before %v", c.name, c.wait)
		}
		clock.set(start.Add(c.wait + 10*time.Millisecond)) // the float rate of Apple sums up to just under one token at 3 s
		if !l.Allow() {
			t.Errorf("%s: no token after %v", c.name, c.wait)
		}
		clock.set(testNow)
	}
}

func TestBookCacheBounds(t *testing.T) {
	t.Parallel()
	clock := &testClock{t: testNow}
	c := newBookCache(clock.now)
	c.max = 2
	c.put("a", []BookCandidate{{ID: "1"}})
	clock.set(testNow.Add(time.Minute))
	c.put("b", nil)
	clock.set(testNow.Add(2 * time.Minute))
	c.put("c", []BookCandidate{{ID: "3"}})
	if _, ok := c.get("a"); ok {
		t.Error("oldest entry kept beyond the limit")
	}
	if got, ok := c.get("b"); !ok || len(got) != 0 {
		t.Errorf("empty result not cached: %v %v", got, ok)
	}
	if got, ok := c.get("c"); !ok || got[0].ID != "3" {
		t.Errorf("c: %v %v", got, ok)
	}
	clock.set(testNow.Add(time.Minute + bookCacheTTL))
	if _, ok := c.get("b"); ok {
		t.Error("entry older than seven days")
	}
	if _, ok := c.get("c"); !ok {
		t.Error("younger entry gone")
	}
	if key := bookCacheKey("audible", "de", BookQuery{Title: " A  b ", ISBN: "3-00-000000-2"}); key != bookCacheKey("audible", "de", BookQuery{Title: "a B", ISBN: "3000000002"}) {
		t.Errorf("keys differ: %q", key)
	}
}

func TestBookSourceURLsFromBase(t *testing.T) {
	t.Parallel()
	u := BookSourceURLsFromBase("http://127.0.0.1:9999/mock/")
	if u.Audible("co.uk") != "http://127.0.0.1:9999/mock/audible/co.uk" || u.Audnexus != "http://127.0.0.1:9999/mock/audnexus" ||
		u.DNB != "http://127.0.0.1:9999/mock/dnb" || u.Apple != "http://127.0.0.1:9999/mock/apple" ||
		u.OpenLibrary != "http://127.0.0.1:9999/mock/openlibrary" || u.Google != "http://127.0.0.1:9999/mock/google" {
		t.Errorf("urls %+v", u)
	}
	if d := DefaultBookSourceURLs(); d.Audible("de") != "https://api.audible.de" || d.Audnexus != "https://api.audnex.us" {
		t.Errorf("defaults %+v", d)
	}
	if got := (url.Values{"a": {"1"}}); queryURL("https://x", "/p", got) != "https://x/p?a=1" {
		t.Error("queryURL")
	}
}
