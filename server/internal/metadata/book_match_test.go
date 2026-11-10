package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The match of audiobooks against the sources of their folder (concept 19 D6.3, PD-10 to PD-12):
// every source is the mock of bookServer, never a real one.

const (
	audibleSearchPath = "/audible/de/1.0/catalog/products"
	bookLibrary       = 7
	bookRoot          = 7
)

// bookSeed is an audiobook of a test.
type bookSeed struct {
	id, title, author, narrator, publisher, asin, isbn, lang string
	minutes                                                  int64 // 0 = no length
	unprobed                                                 bool  // its part waits for the probe
}

// addBookLibrary adds the audiobook library and its folder with the given sources.
func (e *bookEnv) addBookLibrary(t *testing.T, providers ...string) {
	t.Helper()
	raw, _ := json.Marshal(providers)
	mustExec(t, e.db.Write, `INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (?, 'Hörbücher', 'audiobooks', 0, 0, 0)`, bookLibrary)
	mustExec(t, e.db.Write, `INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at, content, providers)
	                         VALUES (?, ?, ?, 'ext4', 'inotify', 0, 'audiobook', ?)`, bookRoot, bookLibrary, t.TempDir(), string(raw))
}

func (e *bookEnv) setFolderSources(t *testing.T, providers ...string) {
	t.Helper()
	raw, _ := json.Marshal(providers)
	mustExec(t, e.db.Write, `UPDATE library_roots SET providers = ? WHERE id = ?`, string(raw), bookRoot)
}

// addBook inserts a book as the scan leaves it: the columns, a text row in its language and in en
// (with the files' provenance), and one part.
func (e *bookEnv) addBook(t *testing.T, b bookSeed) {
	t.Helper()
	if b.lang == "" {
		b.lang = "de"
	}
	var duration any
	if b.minutes > 0 {
		duration = b.minutes * 60_000
	}
	null := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, genres, author, narrator, publisher, asin, isbn,
	                         original_language, duration_ms, providers, metadata_state, added_at, created_at, updated_at)
	                         VALUES (?, 'audiobook', ?, ?, lower(?), '{"de":[],"en":[]}', ?, ?, ?, ?, ?, ?, ?, '["local"]', 'matched', 0, 0, 0)`,
		b.id, bookLibrary, b.title, b.title, null(b.author), null(b.narrator), null(b.publisher), null(b.asin), null(b.isbn), b.lang, duration)
	for _, l := range []string{b.lang, "en"} {
		mustExec(t, e.db.Write, `INSERT OR IGNORE INTO item_texts (item_id, lang, title, sort_key, rank) VALUES (?, ?, ?, lower(?), 0)`, b.id, l, b.title, b.title)
		mustExec(t, e.db.Write, `INSERT OR IGNORE INTO item_field_sources (item_id, field, lang, provider) VALUES (?, 'title', ?, 'local')`, b.id, l)
	}
	for _, f := range []string{"author", "narrator", "publisher"} {
		mustExec(t, e.db.Write, `INSERT INTO item_field_sources (item_id, field, lang, provider)
		                         SELECT id, ?, '', 'local' FROM items WHERE id = ? AND `+f+` IS NOT NULL`, f, b.id)
	}
	var probe any = `{}`
	if b.unprobed {
		probe = nil
	}
	mustExec(t, e.db.Write, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, probe, duration_ms, created_at, updated_at)
	                         VALUES (?, ?, ?, ?, 1, 1, 'mp4', ?, ?, 0, 0)`, "F"+b.id[1:], b.id, bookRoot, b.id+"/01.m4b", probe, duration)
}

func (e *bookEnv) book(t *testing.T, id string) systemq.Item {
	t.Helper()
	it, err := e.svc.read.GetItemSystem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func (e *bookEnv) bookCandidates(t *testing.T, id string) []bookCandidateJSON {
	t.Helper()
	rows, err := e.svc.read.ListBookMatchCandidates(context.Background(), []string{id})
	if err != nil {
		t.Fatal(err)
	}
	var out []bookCandidateJSON
	for _, r := range rows {
		var c bookCandidateJSON
		if err := json.Unmarshal([]byte(r.Candidate), &c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func (e *bookEnv) fieldSource(t *testing.T, id, field, lang string) string {
	t.Helper()
	var p string
	err := e.db.Read.QueryRow(`SELECT provider FROM item_field_sources WHERE item_id = ? AND field = ? AND lang = ?`, id, field, lang).Scan(&p)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *bookEnv) text(t *testing.T, id, lang string) systemq.ItemText {
	t.Helper()
	rows, err := e.svc.read.ListItemTextsByItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Lang == lang {
			return r
		}
	}
	t.Fatalf("no %s text of %s", lang, id)
	return systemq.ItemText{}
}

func (e *bookEnv) match(t *testing.T, p itemPayload) {
	t.Helper()
	if err := e.svc.matchItem(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}

func (e *bookEnv) queued(t *testing.T, key string) bool {
	t.Helper()
	var n int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// audnexusTestBook is the Audnexus answer for B0TEST0001.
const audnexusTestBook = `{"asin":"B0TEST0001","title":"Der Testband","subtitle":"Die Reihe, Teil 3",
 "authors":[{"name":"Erika Muster"}],"narrators":[{"name":"Max Sprecher"}],"publisherName":"Testverlag Hören",
 "releaseDate":"2019-03-28T00:00:00.000Z","language":"german","runtimeLengthMin":612,
 "seriesPrimary":{"name":"Die Reihe","position":"3"},"image":"%s/covers/B0TEST0001.jpg",
 "summary":"<p>Ein erfundener Satz.</p>","genres":[{"name":"Krimi","type":"genre"}],"isbn":"9783000000001","region":"de"}`

func (e *bookEnv) audnexus(asin, body string) {
	e.srv.route("/audnexus/books/"+asin, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
}

func TestBookMatchFillsEmptyFields(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.svc.relinkSeries = library.RelinkAudioSeries
	e.addBookLibrary(t, "local", "audible", "dnb", "applebooks")
	const id = "01JBOOKMATCH0000000000000A"
	// 620 minutes: B0TEST0001 (612) lies within 3 %, B0TEST0003 (240) does not.
	e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", narrator: "Datei Sprecherin", minutes: 620})
	e.srv.fixture(audibleSearchPath, "audible_search.json")
	e.audnexus("B0TEST0001", fmt.Sprintf(audnexusTestBook, e.srv.srv.URL))
	e.match(t, itemPayload{ItemID: id})

	b := e.book(t, id)
	if b.BookMatchState.String != BookStateMatched || b.BookMatchProvider.String != ProviderAudible || b.BookMatchID.String != "B0TEST0001" ||
		b.BookMatchRegion.String != "de" || b.BookMatchManual != 0 || !b.BookMatchedAt.Valid {
		t.Fatalf("match %+v %+v %+v %+v", b.BookMatchState, b.BookMatchProvider, b.BookMatchID, b.BookMatchRegion)
	}
	// Only the empty fields are filled: the narrator of the files stays.
	if b.Narrator.String != "Datei Sprecherin" || b.Publisher.String != "Testverlag Hören" || b.SeriesName.String != "Die Reihe" ||
		b.SeriesIndex.Float64 != 3 || b.Isbn.String != "9783000000001" || b.Year.Int64 != 2019 || b.Title != "Der Testband" {
		t.Errorf("fields %+v", b)
	}
	if g := bookGenres(b.Genres, "de"); !slices.Equal(g, []string{"Krimi"}) || !slices.Equal(bookGenres(b.Genres, "en"), g) {
		t.Errorf("genres %s", b.Genres)
	}
	if b.LockedFields != "[]" && b.LockedFields != "" {
		t.Errorf("locks %s", b.LockedFields)
	}
	for _, l := range []string{"de", "en"} {
		tx := e.text(t, id, l)
		if tx.Overview.String != "Ein erfundener Satz." || tx.Tagline.String != "Die Reihe, Teil 3" || tx.Title != "Der Testband" {
			t.Errorf("%s text %+v", l, tx)
		}
		if got := e.fieldSource(t, id, "overview", l); got != ProviderAudible {
			t.Errorf("source of overview:%s %q", l, got)
		}
		if got := e.fieldSource(t, id, "title", l); got != model.ProviderLocal {
			t.Errorf("source of title:%s %q", l, got)
		}
	}
	for f, want := range map[string]string{"publisher": ProviderAudible, "narrator": model.ProviderLocal, "series_name": ProviderAudible, "genres": ProviderAudible} {
		if got := e.fieldSource(t, id, f, ""); got != want {
			t.Errorf("source of %s %q, want %q", f, got, want)
		}
	}
	// The cover of the source is a candidate, not a choice; the series follows the new series name.
	var provider, source string
	var chosen int
	if err := e.db.Read.QueryRow(`SELECT provider, source, chosen FROM item_images WHERE item_id = ? AND kind = 'poster'`, id).Scan(&provider, &source, &chosen); err != nil {
		t.Fatal(err)
	}
	if provider != ProviderAudible || source != e.srv.srv.URL+"/covers/B0TEST0001.jpg" || chosen != 0 {
		t.Errorf("poster row %s %s %d", provider, source, chosen)
	}
	if !b.AudioSeriesID.Valid {
		t.Error("book not linked to its series after the match")
	}
	cands := e.bookCandidates(t, id)
	if len(cands) != 2 || cands[0].ProviderID != "B0TEST0001" || cands[0].DurationMatches == nil || !*cands[0].DurationMatches ||
		cands[1].DurationMatches == nil || *cands[1].DurationMatches || cands[0].Score <= cands[1].Score {
		t.Errorf("candidates %+v", cands)
	}
	if !e.queued(t, "image:"+id) {
		t.Error("image.fetch not queued after the match")
	}
	// DNB and Apple are not asked once Audible matched.
	if n := e.srv.count("/dnb") + e.srv.count("/apple/search"); n != 0 {
		t.Errorf("%d requests after the match", n)
	}

	// A second run, of any kind, changes nothing and asks nobody.
	before := e.srv.total()
	mustExec(t, e.db.Write, `UPDATE items SET publisher = 'Von Hand' WHERE id = ?`, id)
	e.match(t, itemPayload{ItemID: id})
	e.match(t, itemPayload{ItemID: id, Book: true})
	if n := e.srv.total(); n != before {
		t.Errorf("%d requests in the second run", n-before)
	}
	if got := e.book(t, id).Publisher.String; got != "Von Hand" {
		t.Errorf("second run wrote the publisher: %q", got)
	}
}

func TestBookMatchReviewAndSameRecording(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "audible")
	products := func(mins ...int) string {
		var ps []string
		for i, m := range mins {
			ps = append(ps, fmt.Sprintf(`{"asin":"B0TEST00%02d","title":"Der Testband","authors":[{"name":"Erika Muster"}],"runtime_length_min":%d,"publisher_name":"Verlag %d"}`, 10+i, m, i))
		}
		return `{"products":[` + strings.Join(ps, ",") + `]}`
	}
	body := products(600, 612)
	e.srv.route(audibleSearchPath, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })

	// Two editions within 3 % of 606 minutes: review with both, nothing written.
	const a = "01JBOOKREVIEW000000000000A"
	e.addBook(t, bookSeed{id: a, title: "Der Testband", author: "Erika Muster", minutes: 606})
	e.match(t, itemPayload{ItemID: a})
	if b := e.book(t, a); b.BookMatchState.String != BookStateReview || b.Publisher.Valid || b.BookMatchProvider.Valid {
		t.Errorf("two in tolerance: %+v %+v", b.BookMatchState, b.Publisher)
	}
	if c := e.bookCandidates(t, a); len(c) != 2 {
		t.Errorf("candidates %+v", c)
	}
	if n := e.srv.count("/audnexus/books/B0TEST0010") + e.srv.count("/audnexus/books/B0TEST0011"); n != 0 {
		t.Errorf("audnexus asked for a book in review: %d", n)
	}

	// The same title with lengths a minute apart counts as one recording: matched.
	body = products(612, 613)
	const b = "01JBOOKREVIEW000000000000B"
	e.addBook(t, bookSeed{id: b, title: "Der Testband", author: "Erika Muster", minutes: 610})
	e.svc.books.audible.cache = newBookCache(e.svc.now) // a fresh answer for the same query
	e.match(t, itemPayload{ItemID: b})
	if got := e.book(t, b); got.BookMatchState.String != BookStateMatched || got.Publisher.String == "" {
		t.Errorf("same recording: %+v %+v", got.BookMatchState, got.Publisher)
	}

	// No hit at all: none.
	body = `{"products":[]}`
	const c = "01JBOOKREVIEW000000000000C"
	e.addBook(t, bookSeed{id: c, title: "Ganz anders", author: "Niemand Bekannt", minutes: 100})
	e.match(t, itemPayload{ItemID: c})
	if got := e.book(t, c).BookMatchState.String; got != BookStateNone {
		t.Errorf("no hit: state %q", got)
	}
}

func TestBookMatchWithoutLengthIsReview(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "applebooks")
	e.srv.fixture("/apple/search", "apple_search.json")
	const id = "01JBOOKAPPLE00000000000000"
	e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", minutes: 612})
	e.match(t, itemPayload{ItemID: id})
	b := e.book(t, id)
	if b.BookMatchState.String != BookStateReview || b.Publisher.Valid {
		t.Fatalf("apple only: %+v %+v", b.BookMatchState, b.Publisher)
	}
	c := e.bookCandidates(t, id)
	if len(c) != 1 || c[0].Provider != ProviderAppleBooks || c[0].DurationMatches != nil {
		t.Errorf("candidates %+v", c)
	}
}

func TestBookMatchWaitsForTheProbe(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "audible")
	e.srv.fixture(audibleSearchPath, "audible_search.json")
	const id = "01JBOOKWAITS00000000000000"
	e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", unprobed: true})
	e.match(t, itemPayload{ItemID: id, Book: true})
	if n := e.srv.total(); n != 0 {
		t.Errorf("%d requests before the probe", n)
	}
	if b := e.book(t, id); b.BookMatchState.Valid {
		t.Errorf("state %q before the probe", b.BookMatchState.String)
	}
	if !e.queued(t, "image:"+id) {
		t.Error("image.fetch not queued")
	}
	// The automatic match off: a probed book is not matched by a plain match job.
	mustExec(t, e.db.Write, `UPDATE media_files SET probe = '{}' WHERE item_id = ?`, id)
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, 'off', 0)`, SettingBookAutoMatch)
	e.match(t, itemPayload{ItemID: id})
	if n := e.srv.total(); n != 0 {
		t.Errorf("%d requests with the automatic match off", n)
	}
}

func TestBookMatchBlockedSource(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "audible", "dnb")
	blocked := true
	e.srv.route(audibleSearchPath, func(w http.ResponseWriter, r *http.Request) {
		if blocked {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(bookFixture(t, "audible_search.json"))
	})
	e.srv.fixture("/dnb", "dnb_search.xml")
	e.audnexus("B0TEST0001", fmt.Sprintf(audnexusTestBook, e.srv.srv.URL))
	ids := []string{"01JBOOKBLOCK00000000000001", "01JBOOKBLOCK00000000000002", "01JBOOKBLOCK00000000000003"}
	for i, id := range ids {
		e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", minutes: 612 + int64(i)})
		e.match(t, itemPayload{ItemID: id})
	}
	if n := e.srv.count(audibleSearchPath); n != 3 {
		t.Errorf("audible asked %d times, want 3", n)
	}
	if _, paused := e.svc.bookSourcePausedUntil(context.Background(), ProviderAudible); !paused {
		t.Fatal("audible not paused after three 429")
	}
	for _, id := range ids {
		b := e.book(t, id)
		if b.BookMatchState.String != BookStateError || b.BookMatchError.String != "audible:blocked" || b.Publisher.Valid {
			t.Errorf("%s: %+v %+v %+v", id, b.BookMatchState, b.BookMatchError, b.Publisher)
		}
		// The other source still answers: its hits are the candidates.
		if c := e.bookCandidates(t, id); len(c) == 0 || c[0].Provider != ProviderDNB {
			t.Errorf("%s: candidates %+v", id, c)
		}
	}
	// While audible pauses it is not asked; a book whose only source pauses is not queued again.
	e.setFolderSources(t, "local", "audible")
	if n, err := e.svc.queueErrorBooks(context.Background(), bookLibrary); err != nil || n != 0 {
		t.Errorf("queued %d books while their source pauses (%v)", n, err)
	}
	// After the pause the next run tries again and matches.
	blocked = false
	e.clock.set(testNow.Add(bookBreakerPause + time.Minute))
	if n, err := e.svc.queueErrorBooks(context.Background(), bookLibrary); err != nil || n != 3 {
		t.Fatalf("queued %d books after the pause (%v)", n, err)
	}
	e.match(t, itemPayload{ItemID: ids[0], Book: true})
	if b := e.book(t, ids[0]); b.BookMatchState.String != BookStateMatched || b.BookMatchError.Valid {
		t.Errorf("after the pause: %+v %+v", b.BookMatchState, b.BookMatchError)
	}
}

func TestBookMatchFolderSources(t *testing.T) {
	t.Parallel()
	if got := model.DefaultProviders(model.RootAudiobook); !slices.Equal(got, []string{"local", "audible", "dnb", "applebooks"}) {
		t.Errorf("default list of an audiobook folder %v", got)
	}
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "dnb", "googlebooks") // Audible taken off the list, Google without key
	e.srv.fixture(audibleSearchPath, "audible_search.json")
	e.srv.fixture("/dnb", "dnb_search.xml")
	const id = "01JBOOKFOLDER0000000000000"
	e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", minutes: 612})
	e.match(t, itemPayload{ItemID: id})
	if n := e.srv.count(audibleSearchPath) + e.srv.count("/google/volumes"); n != 0 {
		t.Errorf("%d requests to sources off the list or without key", n)
	}
	if n := e.srv.count("/dnb"); n == 0 {
		t.Error("dnb not asked")
	}
	if got := e.book(t, id).BookMatchState.String; got != BookStateReview {
		t.Errorf("state %q", got)
	}
	// The Audnexus address in the home network is allowed, another local address is not:
	// TestAudibleAudnexusNetguard.
}

func TestBookMatchIdentifierFirst(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "audible", "dnb")
	e.srv.fixture(audibleSearchPath, "audible_search.json")
	e.srv.fixture("/dnb", "dnb_search.xml")
	// The ASIN is not sold in .de: the region switch finds it in .com.
	e.srv.fixture(audibleSearchPath+"/B0TEST0002", "audible_product_empty.json")
	e.srv.fixture("/audible/com/1.0/catalog/products/B0TEST0002", "audible_product_com.json")
	e.srv.fixture("/audnexus/books/B0TEST0002", "audnexus_book.json")
	const id = "01JBOOKASIN000000000000000"
	// The title of the files would match B0TEST0001 by title; the ASIN wins.
	e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", asin: "B0TEST0002", minutes: 1340})
	e.match(t, itemPayload{ItemID: id})
	b := e.book(t, id)
	if b.BookMatchState.String != BookStateMatched || b.BookMatchID.String != "B0TEST0002" || b.BookMatchRegion.String != "com" {
		t.Fatalf("match %+v %+v %+v", b.BookMatchState, b.BookMatchID, b.BookMatchRegion)
	}
	if n := e.srv.count(audibleSearchPath) + e.srv.count("/dnb"); n != 0 {
		t.Errorf("%d title or ISBN requests after the ASIN matched", n)
	}
	if b.Publisher.String != "Sample Audio" || b.Title != "Der Testband" {
		t.Errorf("fields %+v %q", b.Publisher, b.Title)
	}
}

func TestBookCoverFallback(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	e := newBookEnv(t)
	e.svc.runner = transcode.Runner{FFmpegPath: lookFFmpeg()}
	e.addBookLibrary(t, "local", "audible")
	cover := jpegBytes(t, 500, 500)
	e.srv.route("/covers/B0TEST0001.jpg", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(cover) })
	e.srv.fixture(audibleSearchPath, "audible_search.json")
	e.audnexus("B0TEST0001", fmt.Sprintf(audnexusTestBook, e.srv.srv.URL))
	const id = "01JBOOKCOVER00000000000000"
	e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", minutes: 612})
	e.match(t, itemPayload{ItemID: id})
	// Without a cover in its files the book takes the cover of its source.
	if err := e.svc.fetchImages(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	var src string
	if err := e.db.Read.QueryRow(`SELECT source_url FROM images WHERE item_id = ? AND kind = 'poster' AND size = 'original'`, id).Scan(&src); err != nil {
		t.Fatal(err)
	}
	if src != e.srv.srv.URL+"/covers/B0TEST0001.jpg" {
		t.Errorf("poster from %q", src)
	}
	img, err := e.svc.ItemImages(context.Background(), id)
	if err != nil || len(img) != 1 || img[0].Provider != ProviderAudible || img[0].Chosen {
		t.Errorf("title page image %+v (%v)", img, err)
	}
}

func TestBookCleanTitleAndSimilarity(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Das Känguru-Manifest: Live und ungekürzt":        "Das Känguru-Manifest",
		"Harry Potter -03- und der Gefangene von Azkaban": "Harry Potter und der Gefangene von Azkaban",
		"03 - Der Testband (ungekürzt)":                   "Der Testband",
		"Der Testband - Hörbuch":                          "Der Testband",
		"Der Testband":                                    "Der Testband",
	} {
		if got := bookCleanTitle(in); got != want {
			t.Errorf("bookCleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
	if s := bookTitleSimilarity("Frühstück mit Kängurus", "Fruehstueck mit Kaengurus"); s < 0.7 {
		t.Errorf("similarity %v", s)
	}
	if s := bookTitleSimilarity("Der Testband", "Der Testband: Die Reihe 3"); s < bookTitleThreshold {
		t.Errorf("contained title %v", s)
	}
	if bookTitleSimilarity("Der Testband", "Ein ganz anderes Buch") >= bookTitleThreshold {
		t.Error("different titles are similar")
	}
	if !bookAuthorMatches("J.R.R. Tolkien", []string{"J. R. R. Tolkien"}) || !bookAuthorMatches("Erika Muster", []string{"Muster, Erika"}) ||
		bookAuthorMatches("Erika Muster", []string{"Hans Beispiel"}) {
		t.Error("author match")
	}
}

func TestQueueFirstBookMatches(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "audible")
	ctx := context.Background()
	a, b, c := "01JBOOKFIRST00000000000001", "01JBOOKFIRST00000000000002", "01JBOOKFIRST00000000000003"
	for _, id := range []string{a, b, c} {
		e.addBook(t, bookSeed{id: id, title: "Buch " + id[len(id)-1:], minutes: 10})
	}
	mustExec(t, e.db.Write, `UPDATE items SET book_match_state = 'review' WHERE id = ?`, c)
	if n, err := e.svc.QueueFirstBookMatches(ctx); err != nil || n != 2 {
		t.Fatalf("first run queued %d (%v), want 2", n, err)
	}
	if !e.queued(t, "match:"+a) || !e.queued(t, "match:"+b) || e.queued(t, "match:"+c) {
		t.Error("wrong books queued")
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueueFirstBookMatches(ctx); err != nil || n != 0 {
		t.Errorf("second start queued %d (%v)", n, err)
	}
}

func TestBookRefetch(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local", "audible")
	e.srv.fixture(audibleSearchPath, "audible_search.json")
	e.audnexus("B0TEST0001", fmt.Sprintf(audnexusTestBook, e.srv.srv.URL))
	const id = "01JBOOKREFETCH000000000000"
	e.addBook(t, bookSeed{id: id, title: "Der Testband", author: "Erika Muster", narrator: "Datei Sprecherin", minutes: 612})
	e.match(t, itemPayload{ItemID: id})
	// The administrator locks the series name; the source renames publisher and series.
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["series_name"]' WHERE id = ?`, id)
	changed := strings.NewReplacer("Testverlag Hören", "Neuer Verlag", `"name":"Die Reihe"`, `"name":"Andere Reihe"`).
		Replace(fmt.Sprintf(audnexusTestBook, e.srv.srv.URL))
	e.audnexus("B0TEST0001", changed)
	e.match(t, itemPayload{ItemID: id, Refetch: true})
	b := e.book(t, id)
	if b.Publisher.String != "Neuer Verlag" || b.SeriesName.String != "Die Reihe" || b.Narrator.String != "Datei Sprecherin" {
		t.Errorf("refetch wrote %+v %+v %+v", b.Publisher, b.SeriesName, b.Narrator)
	}
	if b.BookMatchState.String != BookStateMatched || b.BookMatchID.String != "B0TEST0001" {
		t.Errorf("match after the refetch %+v %+v", b.BookMatchState, b.BookMatchID)
	}
}

// PD-9: the match in the editor writes only the ticked fields that are not locked, locks them (a
// text in every row), credits them to the source and chooses the cover; the errors of a source
// are field errors.
func TestApplyBookMatch(t *testing.T) {
	t.Parallel()
	e := newBookEnv(t)
	e.addBookLibrary(t, "local")
	e.audnexus("B0TEST0001", fmt.Sprintf(audnexusTestBook, e.srv.srv.URL))
	const id = "01JBOOKAPPLY00000000000000"
	e.addBook(t, bookSeed{id: id, title: "Der Testbnd", author: "Erika Muster", narrator: "Datei Sprecherin", minutes: 612})
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["narrator"]' WHERE id = ?`, id)
	ctx := context.Background()
	p, err := e.svc.BookMatchPreview(ctx, id, ProviderAudible, "B0TEST0001", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Fields) != len(BookMatchFields) || p.DurationMatches == nil || !*p.DurationMatches {
		t.Fatalf("preview %+v", p)
	}
	if err := e.svc.ApplyBookMatch(ctx, id, ProviderAudible, "B0TEST0001", "", []string{"title", "narrator", "publisher", "cover"}); err != nil {
		t.Fatal(err)
	}
	b := e.book(t, id)
	locks := lockedFields(b.LockedFields)
	if b.Title != "Der Testband" || b.Narrator.String != "Datei Sprecherin" || b.Publisher.String != "Testverlag Hören" || b.Isbn.Valid {
		t.Errorf("fields %q %+v %+v %+v", b.Title, b.Narrator, b.Publisher, b.Isbn)
	}
	for _, l := range []string{"title:de", "title:en", "publisher", "narrator", "poster"} {
		if !locks[l] {
			t.Errorf("lock %s missing in %s", l, b.LockedFields)
		}
	}
	if locks["isbn"] || locks["overview:de"] {
		t.Errorf("unticked field locked: %s", b.LockedFields)
	}
	if e.text(t, id, "en").Title != "Der Testband" || e.fieldSource(t, id, "title", "de") != ProviderAudible || e.fieldSource(t, id, "publisher", "") != ProviderAudible {
		t.Error("texts or sources of the match")
	}
	if b.BookMatchState.String != BookStateMatched || b.BookMatchManual != 1 {
		t.Errorf("match %+v manual %d", b.BookMatchState, b.BookMatchManual)
	}
	var chosen int
	if err := e.db.Read.QueryRow(`SELECT chosen FROM item_images WHERE item_id = ? AND kind = 'poster'`, id).Scan(&chosen); err != nil || chosen != 1 {
		t.Errorf("cover chosen %d (%v)", chosen, err)
	}
	// An automatic run never changes a match from the editor.
	e.match(t, itemPayload{ItemID: id, Book: true})
	if got := e.book(t, id); got.BookMatchManual != 1 || got.Publisher.String != "Testverlag Hören" {
		t.Errorf("automatic run changed the manual match")
	}
	var verr *model.ValidationError
	if err := e.svc.ApplyBookMatch(ctx, id, ProviderAudible, "B0TEST0009", "", nil); !errors.As(err, &verr) || verr.Error() == "" ||
		!strings.Contains(fmt.Sprint(verr.Fields), "not_found") {
		t.Errorf("unknown entry: %v", err)
	}
	e.srv.route("/dnb", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusBadGateway) })
	if _, err := e.svc.SearchBookSource(ctx, model.BookSearch{Provider: ProviderDNB, Query: "X"}); !errors.As(err, &verr) ||
		!strings.Contains(fmt.Sprint(verr.Fields), "provider_unreachable") {
		t.Errorf("failing source: %v", err)
	}
}
