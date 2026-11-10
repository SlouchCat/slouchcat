package api

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// contractBooks is the mock of every book source in the contract fixture (the paths of
// metadata.BookSourceURLsFromBase); a path without route answers 404. Tests never ask a real
// source.
type contractBooks struct {
	srv    *httptest.Server
	mu     sync.Mutex
	routes map[string]string
	status map[string]int
}

func newContractBooks(t *testing.T) *contractBooks {
	t.Helper()
	b := &contractBooks{routes: map[string]string{}, status: map[string]int{}}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		body, ok := b.routes[r.URL.Path]
		code := b.status[r.URL.Path]
		b.mu.Unlock()
		switch {
		case code != 0:
			http.Error(w, "refused", code)
		case !ok:
			http.NotFound(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *contractBooks) route(path, body string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.routes[path] = body
}

func (b *contractBooks) fail(path string, code int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status[path] = code
}

const (
	contractAudibleSearch = `{"products":[
	 {"asin":"B0TEST0003","title":"Der Testband","authors":[{"name":"Erika Muster"}],"runtime_length_min":240},
	 {"asin":"B0TEST0001","title":"Der Testband","authors":[{"name":"Erika Muster"}],"narrators":[{"name":"Max Sprecher"}],
	  "publisher_name":"Testverlag Hören","runtime_length_min":612}]}`
	contractAudibleProduct = `{"product":{"asin":"B0TEST0001","title":"Der Testband","authors":[{"name":"Erika Muster"}],
	 "narrators":[{"name":"Max Sprecher"}],"publisher_name":"Testverlag Hören","runtime_length_min":612,"language":"german"}}`
	contractAudnexus = `{"asin":"B0TEST0001","title":"Der Testband","authors":[{"name":"Erika Muster"}],
	 "narrators":[{"name":"Max Sprecher"}],"publisherName":"Testverlag Hören","releaseDate":"2019-03-28T00:00:00.000Z",
	 "language":"german","runtimeLengthMin":612,"image":"https://images.example.test/B0TEST0001.jpg",
	 "summary":"<p>Ein erfundener Satz.</p>","genres":[{"name":"Krimi","type":"genre"}],"isbn":"9783000000001"}`
)

// Contract 1.26.0 (concept 19 stage D3, PD-9, PD-15): the search at a book source, the comparison
// and the match in the editor, the refetch, AdminItem.bookMatch and durationMs, and the page
// «Hörbücher: Quellen». The matcher and the writes are package metadata's (book_match_test.go).
func TestBookSourcesOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h, books := pf.h, pf.books
	book := pf.f.Audio.Books[0].ID
	pf.f.SetBookForMatch(t, book, int64(615)*60_000, "Datei Sprecherin", `["narrator"]`)
	books.route("/audible/de/1.0/catalog/products", contractAudibleSearch)
	books.route("/audible/de/1.0/catalog/products/B0TEST0001", contractAudibleProduct)
	books.route("/audnexus/books/B0TEST0001", contractAudnexus)
	search := BasePath + "/admin/metadata/search?kind=audiobook"

	// Validation and the errors of the sources.
	for query, want := range map[string]string{
		"&provider=audible":                                "query required",
		"&provider=tmdb&query=X":                           "provider provider_not_searchable",
		"&provider=audible&asin=B0SHORT9":                  "asin invalid_value",
		"&provider=dnb&isbn=978-3":                         "isbn invalid_value",
		"&provider=googlebooks&query=X":                    "provider provider_not_configured",
		"&provider=audible&query=X&itemId=" + fixtureFilm6: "itemId invalid_value",
		"&provider=openlibrary&query=Der+Testband":         "provider provider_unreachable",
	} {
		if query == "&provider=openlibrary&query=Der+Testband" {
			books.fail("/openlibrary/search.json", http.StatusInternalServerError)
		}
		if got := fieldProblem(t, send(t, h, http.MethodGet, search+query, "p18", "", nil)); got != want {
			t.Errorf("searchMetadata%s: %s, want %s", query, got, want)
		}
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/admin/metadata/search?kind=movie&provider=tmdb", "p18", "", nil); fieldProblem(t, rec) != "query required" {
		t.Errorf("movie without query: %s", rec.Body.String())
	}

	// PD-9: by ASIN, and by title with the matching length first.
	var cs []gen.MetadataCandidate
	rec := send(t, h, http.MethodGet, search+"&provider=audible&asin=b0test0001&itemId="+book, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &cs)
	if len(cs) != 1 || cs[0].ProviderId != "B0TEST0001" || cs[0].DurationMatches == nil || !*cs[0].DurationMatches ||
		cs[0].Score == nil || deref(cs[0].DurationMs) != 612*60_000 || deref(cs[0].Region) != gen.AudibleRegionDe {
		t.Fatalf("search by ASIN = %s", rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, search+"&provider=audible&query=Der+Testband&author=Erika+Muster&itemId="+book, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &cs)
	if len(cs) != 2 || cs[0].ProviderId != "B0TEST0001" || !*cs[0].DurationMatches || *cs[1].DurationMatches || *cs[0].Score <= *cs[1].Score {
		t.Errorf("search by title = %s", rec.Body.String())
	}

	// The comparison: every field, the locked narrator, nothing written.
	preview := BasePath + "/admin/items/" + book + "/match/preview?provider=audible&providerId=B0TEST0001"
	rec = send(t, h, http.MethodGet, preview, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var mp gen.MatchPreview
	decode(t, rec, &mp)
	if len(mp.Fields) != 14 || mp.Title != "Der Testband" || mp.DurationMatches == nil || !*mp.DurationMatches ||
		deref(mp.CoverUrl) != "https://images.example.test/B0TEST0001.jpg" {
		t.Fatalf("preview = %s", rec.Body.String())
	}
	byField := map[gen.BookMatchField]gen.MatchPreviewField{}
	for _, f := range mp.Fields {
		byField[f.Field] = f
	}
	if f := byField[gen.BookMatchFieldNarrator]; !f.Locked || f.Current != "Datei Sprecherin" || f.Proposed != "Max Sprecher" || !f.Differs {
		t.Errorf("narrator %+v", f)
	}
	if f := byField[gen.BookMatchFieldPublisher]; f.Locked || f.Current != "" || f.Proposed != "Testverlag Hören" || !f.Differs {
		t.Errorf("publisher %+v", f)
	}
	if f := byField[gen.BookMatchFieldGenres]; f.Proposed != "Krimi" {
		t.Errorf("genres %+v", f)
	}
	for _, tc := range []struct{ path, want string }{
		{BasePath + "/admin/items/" + fixtureFilm6 + "/match/preview?provider=audible&providerId=B0TEST0001", "kind invalid_value"},
		{BasePath + "/admin/items/" + book + "/match/preview?provider=audible&providerId=B0TEST0009", "providerId not_found"},
		{BasePath + "/admin/items/" + book + "/match/preview?provider=tmdb&providerId=603", "provider provider_not_searchable"},
	} {
		if got := fieldProblem(t, send(t, h, http.MethodGet, tc.path, "p18", "", nil)); got != tc.want {
			t.Errorf("getMatchPreview %s: %s, want %s", tc.path, got, tc.want)
		}
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/admin/items/"+unknownItemID+"/match/preview?provider=audible&providerId=B0TEST0001", "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown title: %d", rec.Code)
	}

	// The refetch needs a match.
	refresh := BasePath + "/admin/items/" + book + "/refresh"
	if got := fieldProblem(t, send(t, h, http.MethodPost, refresh, "p18", `{"fromProvider":true}`, nil)); got != "fromProvider not_matched" {
		t.Errorf("refetch without match: %s", got)
	}

	// PD-9: the match writes only the ticked fields that are not locked, and locks them; an empty
	// list confirms the match by hand without taking a field.
	match := BasePath + "/admin/items/" + book + "/match"
	rec = send(t, h, http.MethodPut, match, "p18", `{"provider":"audible","providerId":"B0TEST0001","fields":[]}`, nil)
	wantStatus(t, rec, http.StatusAccepted)
	var confirmed gen.AdminItem
	decode(t, rec, &confirmed)
	if bm := confirmed.BookMatch; bm == nil || bm.State != gen.AdminBookMatchStateMatched || !bm.Manual || confirmed.Metadata == nil ||
		deref(confirmed.Metadata.Publisher) != "" || slices.Contains(confirmed.LockedFields, "publisher") {
		t.Errorf("empty fields: %s", rec.Body.String())
	}
	rec = send(t, h, http.MethodPut, match, "p18", `{"provider":"audible","providerId":"B0TEST0001","fields":["publisher","narrator","cover"]}`, nil)
	wantStatus(t, rec, http.StatusAccepted)
	var it gen.AdminItem
	decode(t, rec, &it)
	m := it.Metadata
	if m == nil || deref(m.Publisher) != "Testverlag Hören" || deref(m.Narrator) != "Datei Sprecherin" || deref(m.Isbn) == "9783000000001" {
		t.Fatalf("after the match: %+v", m)
	}
	if !slices.Contains(it.LockedFields, "publisher") || !slices.Contains(it.LockedFields, "poster") || slices.Contains(it.LockedFields, "isbn") {
		t.Errorf("locks %v", it.LockedFields)
	}
	bm := it.BookMatch
	if bm == nil || bm.State != gen.AdminBookMatchStateMatched || !bm.Manual || deref(bm.ProviderId) != "B0TEST0001" ||
		bm.Provider == nil || *bm.Provider != gen.MetadataProviderAudible || bm.MatchedAt == nil {
		t.Errorf("bookMatch %+v", bm)
	}
	if deref(it.DurationMs) != 615*60_000 {
		t.Errorf("durationMs %v", it.DurationMs)
	}
	if got := fieldProblem(t, send(t, h, http.MethodPut, BasePath+"/admin/items/"+fixtureFilm6+"/match", "p18",
		`{"provider":"audible","providerId":"B0TEST0001"}`, nil)); got != "provider invalid_value" {
		t.Errorf("book match of a film: %s", got)
	}
	// The refetch of a matched book is queued (its writes: TestBookRefetch; the fixture's
	// metadata stand-in keeps the locks of unlockAll).
	wantStatus(t, send(t, h, http.MethodPost, refresh, "p18", `{"fromProvider":true,"unlockAll":true}`, nil), http.StatusAccepted)
	if rec := send(t, h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil); strings.Contains(rec.Body.String(), "bookMatch") {
		t.Errorf("a film with bookMatch: %s", rec.Body.String())
	}

	// The page «Hörbücher: Quellen».
	page := BasePath + "/admin/metadata/audiobooks"
	rec = send(t, h, http.MethodGet, page, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var src gen.AudiobookSources
	decode(t, rec, &src)
	if src.AudibleRegion != gen.AudibleRegionDe || src.AutoMatch != gen.AudiobookAutoMatchAll || src.TolerancePercent != 3 || !src.NormalizeAuthors ||
		len(src.Sources) != 5 || src.Sources[4].Id != gen.MetadataProviderGooglebooks || src.Sources[4].Configured ||
		src.GoogleBooksKey.Source != gen.SettingSourceNone || len(src.Folders) == 0 || src.Stats.Matched != 1 ||
		src.Stats.Total != src.Stats.Matched+src.Stats.Review+src.Stats.None+src.Stats.Error+src.Stats.Pending {
		t.Errorf("getAudiobookSources = %s", rec.Body.String())
	}
	for body, want := range map[string]string{
		`{"tolerancePercent":11}`:                           "tolerancePercent invalid_value",
		`{"audnexusUrl":"ftp://audnexus.example.test"}`:     "audnexusUrl invalid_value",
		`{"googleBooksKey":"k","clearGoogleBooksKey":true}`: "googleBooksKey invalid_value",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPatch, page, "p18", body, nil)); got != want {
			t.Errorf("updateAudiobookSources %s: %s, want %s", body, got, want)
		}
	}
	rec = send(t, h, http.MethodPatch, page, "p18", `{"audibleRegion":"com","audnexusUrl":"http://192.168.1.50:3000/","tolerancePercent":5,
		"autoMatch":"new","normalizeSeries":false,"googleBooksKey":"test-google-books-key-0001"}`, nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &src)
	if src.AudibleRegion != gen.AudibleRegionCom || src.AudnexusUrl != "http://192.168.1.50:3000" || src.TolerancePercent != 5 ||
		src.AutoMatch != gen.AudiobookAutoMatchNew || src.NormalizeSeries || src.GoogleBooksKey.Source != gen.SettingSourceStored ||
		deref(src.GoogleBooksKey.Hint) != "0001" || !src.Sources[4].Configured {
		t.Errorf("after the change = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "test-google-books-key") {
		t.Error("the Google Books key in an answer")
	}
	// An empty Audnexus address is the default again.
	rec = send(t, h, http.MethodPatch, page, "p18", `{"audnexusUrl":""}`, nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &src)
	if src.AudnexusUrl != "https://api.audnex.us" {
		t.Errorf("audnexusUrl after the reset %q", src.AudnexusUrl)
	}
	rec = send(t, h, http.MethodPatch, page, "p18", `{"clearGoogleBooksKey":true}`, nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &src)
	if src.GoogleBooksKey.Source != gen.SettingSourceNone {
		t.Errorf("after clearing the key: %+v", src.GoogleBooksKey)
	}

	// Match-all and the list «Prüfen».
	rec = send(t, h, http.MethodPost, page+"/match-all", "p18", "", nil)
	wantStatus(t, rec, http.StatusAccepted)
	var all gen.AudiobookMatchAllResponse
	decode(t, rec, &all)
	if all.Queued < 0 {
		t.Errorf("match-all %+v", all)
	}
	other := pf.f.Audio.Books[1].ID
	pf.f.SetBookReview(t, other, "dnb", "3000000002", `{"provider":"dnb","providerId":"3000000002","title":"Der Testband","score":0.3}`)
	rec = send(t, h, http.MethodGet, page+"/review", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var review []gen.BookReviewEntry
	decode(t, rec, &review)
	if len(review) != 1 || review[0].ItemId != other || review[0].State != gen.BookReviewEntryStateReview || len(review[0].Candidates) != 1 ||
		review[0].Candidates[0].Provider != gen.MetadataProviderDnb || review[0].MatchedAt == nil {
		t.Errorf("listAudiobookReview = %s", rec.Body.String())
	}
}
