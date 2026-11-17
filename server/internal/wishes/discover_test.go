package wishes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Build step 5.3: the read path of the wishes area and the job wishes.discover (architecture 21,
// DoD row 5.3 of the build plan).

// titleStore is external_titles as metadata keeps it: the fake library reads the ages and ids
// from it, the fake titles write it on a fetch.
type titleStore struct {
	mu     sync.Mutex
	stored map[string]metadata.ExternalTitle
	tmdb   map[string]metadata.ExternalTitle // what TMDB would answer
	calls  int                               // fetches from TMDB
}

func (s *titleStore) EnsureExternalTitle(_ context.Context, media string, tmdbID int64) (metadata.ExternalTitle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := refKey(media, tmdbID)
	if t, ok := s.stored[k]; ok {
		return t, nil
	}
	s.calls++
	t, ok := s.tmdb[k]
	if !ok {
		return metadata.ExternalTitle{}, model.ErrNotFound
	}
	s.stored[k] = t
	return t, nil
}

func (s *titleStore) fetches() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// discoverLib answers like visibility.MatchExternalTitles for one profile at a time (ceiling): a
// series TVDB first (the TVDB id of the ref, else of the stored title), then TMDB; a library title
// is never missing; outside the library the age of the stored title counts, unknown as 18.
type discoverLib struct {
	mu      sync.Mutex
	store   *titleStore
	items   map[string]string // media:tmdb -> item
	byTVDB  map[int64]string  // series by TVDB id
	hidden  map[string]bool
	ceiling int
}

func (l *discoverLib) MatchExternalTitles(_ context.Context, _ visibility.Scope, right visibility.RequestRight, refs []visibility.ExternalRef) ([]visibility.ExternalMatch, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.store.mu.Lock()
	defer l.store.mu.Unlock()
	out := make([]visibility.ExternalMatch, len(refs))
	for i, r := range refs {
		stored, has := l.store.stored[refKey(r.Media, r.TMDBID)]
		tvdb := r.TVDBID
		if tvdb == 0 && has {
			tvdb = stored.TVDBID
		}
		id := ""
		if r.Media == visibility.MediaShow && tvdb > 0 {
			id = l.byTVDB[tvdb]
		}
		if id == "" {
			id = l.items[refKey(r.Media, r.TMDBID)]
		}
		m := visibility.ExternalMatch{Ref: r, InLibrary: id != "", Stored: has}
		if id != "" && !l.hidden[id] {
			m.ItemID = id
		}
		if !m.InLibrary && right != nil && right(r.Media) {
			age := 18
			if has && stored.MinAge != nil {
				age = *stored.MinAge
			}
			m.Missing = age <= l.ceiling
			m.NeedsDetails = !has && l.ceiling < 18
		}
		out[i] = m
	}
	return out, nil
}

func (l *discoverLib) MissingTitle(ctx context.Context, s visibility.Scope, right visibility.RequestRight, ref visibility.ExternalRef) error {
	m, _ := l.MatchExternalTitles(ctx, s, right, []visibility.ExternalRef{ref})
	if !m[0].Missing {
		return model.ErrNotFound
	}
	return nil
}

func (l *discoverLib) TitleByExternalIDs(context.Context, visibility.ExternalRef) (string, bool, error) {
	return "", false, nil
}

func (l *discoverLib) Visible(context.Context, visibility.Scope, string) error { return nil }

func (l *discoverLib) VisibleSeasons(context.Context, visibility.Scope, string) (map[int]int, error) {
	return map[int]int{}, nil
}

func (l *discoverLib) setCeiling(age int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ceiling = age
}

type dfix struct {
	*rfix
	dlib   *discoverLib
	store  *titleStore
	noah12 model.Profile // the second profile of account 1, capped to 12
}

func age(n int) *int { return &n }

// newDiscoverFixture: Marco (18) and Noah (12) of account 1 send as kinder (3). Seerr knows the
// films of the request fixture (Matrix, Alien) and Game of Thrones beside the ones a test adds.
func newDiscoverFixture(t *testing.T) *dfix {
	t.Helper()
	r := newRequestFixture(t)
	r.exec(`UPDATE profiles SET max_rating = 12 WHERE id = 2`)
	store := &titleStore{stored: map[string]metadata.ExternalTitle{}, tmdb: map[string]metadata.ExternalTitle{}}
	d := &dfix{rfix: r, store: store, noah12: model.Profile{ID: 2, AccountID: 1, Language: "de", MaxRating: 12},
		dlib: &discoverLib{store: store, items: map[string]string{}, byTVDB: map[int64]string{}, hidden: map[string]bool{}, ceiling: 18}}
	d.fixture.rebuild(WithLibrary(d.dlib), WithTitles(store), WithNotifier(r.note), WithClock(r.clock.now))
	return d
}

// film adds a film to Seerr and to what TMDB answers (minAge nil = no rating).
func (d *dfix) film(tmdb int, name string, popularity float64, minAge *int, ignoresLte bool) {
	cert := ""
	if minAge != nil {
		cert = fmt.Sprint(*minAge)
	}
	d.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: tmdb, Name: name, Year: 2020, Popularity: popularity,
		GenreIDs: []int{28}, Certification: cert, IgnoresCertification: ignoresLte})
	d.store.mu.Lock()
	defer d.store.mu.Unlock()
	d.store.tmdb[refKey("movie", int64(tmdb))] = metadata.ExternalTitle{Media: "movie", TMDBID: int64(tmdb), MinAge: minAge}
}

func tmdbIDs(es []DiscoverEntry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, fmt.Sprintf("%s:%d", e.Media, e.TMDBID))
	}
	return out
}

// A second read within the hour makes no call to Seerr; after the hour it reads again; another
// age step is its own cache entry.
func TestDiscoverCachePerHourAndAgeStep(t *testing.T) {
	d := newDiscoverFixture(t)
	d.film(5000, "Twelve", 50, age(12), false)
	d.fake.ResetCalls()

	first, err := d.svc.DiscoverRowPage(d.ctx, d.marco, RowTrending, 1, 0)
	if err != nil || len(first.Entries) == 0 || first.Stale {
		t.Fatalf("first read = %+v, %v", first, err)
	}
	d.clock.advance(59 * time.Minute)
	second, err := d.svc.DiscoverRowPage(d.ctx, d.marco, RowTrending, 1, 0)
	if err != nil || !slices.Equal(tmdbIDs(second.Entries), tmdbIDs(first.Entries)) || !second.FetchedAt.Equal(first.FetchedAt) {
		t.Fatalf("second read = %+v, %v", second, err)
	}
	if n := d.fake.Calls("GET /discover/trending"); n != 1 {
		t.Errorf("trending calls within the hour = %d, want 1", n)
	}
	d.dlib.setCeiling(12)
	if _, err := d.svc.DiscoverRowPage(d.ctx, d.noah12, RowTrending, 1, 0); err != nil {
		t.Fatal(err)
	}
	if n := d.fake.Calls("GET /discover/trending"); n != 2 {
		t.Errorf("trending calls after another age step = %d, want 2", n)
	}
	if n := d.count(`SELECT count(*) FROM wish_discover_pages WHERE cache_key IN ('trending|genre=|p=1|de|18', 'trending|genre=|p=1|de|12')`); n != 2 {
		t.Errorf("cache rows = %d, want 2", n)
	}
	d.dlib.setCeiling(18)
	d.clock.advance(2 * time.Minute)
	if _, err := d.svc.DiscoverRowPage(d.ctx, d.marco, RowTrending, 1, 0); err != nil {
		t.Fatal(err)
	}
	if n := d.fake.Calls("GET /discover/trending"); n != 3 {
		t.Errorf("trending calls after the hour = %d, want 3", n)
	}
	for _, c := range d.fake.CallLog() {
		if !c.HasAPIUser || c.APIUser != "3" {
			t.Errorf("%s sent as %q, want the Seerr user 3 of the account", c.Route, c.APIUser)
		}
	}
}

// A profile aged 12 sees no hit rated 16 and none without a rating, even when Seerr returns them
// despite certificationLte; the request carries the pre-filter of the rating country (AK-181).
func TestDiscoverAgeCheckedAgain(t *testing.T) {
	d := newDiscoverFixture(t)
	d.film(5012, "Rated Twelve", 30, age(12), false)
	d.film(5016, "Rated Sixteen", 20, age(16), true)
	d.film(5099, "No Rating", 10, nil, true)
	d.dlib.setCeiling(12)
	d.fake.ResetCalls()

	page, err := d.svc.DiscoverTitles(d.ctx, d.noah12, TitlesQuery{Media: model.WishMediaMovie})
	if err != nil {
		t.Fatal(err)
	}
	if got := tmdbIDs(page.Entries); !slices.Equal(got, []string{"movie:5012"}) {
		t.Errorf("films for 12 = %v, want only the one rated 12", got)
	}
	if e := page.Entries[0]; e.MinAge == nil || *e.MinAge != 12 || !e.CanRequest || e.ItemID != "" {
		t.Errorf("entry = %+v", e)
	}
	var sent bool
	for _, c := range d.fake.CallLog() {
		if c.Route == "GET /discover/movies" {
			sent = c.Query.Get("certificationLte") == "12" && c.Query.Get("certificationCountry") == "DE" && c.Query.Get("sortBy") == "popularity.desc"
		}
	}
	if !sent {
		t.Errorf("no certificationLte 12 for DE in %+v", d.fake.CallLog())
	}
	// The same in a row of «Entdecken» and in the search.
	row, err := d.svc.DiscoverRowPage(d.ctx, d.noah12, RowPopularMovies, 1, 0)
	if err != nil || !slices.Equal(tmdbIDs(row.Entries), []string{"movie:5012"}) {
		t.Errorf("row for 12 = %v, %v", tmdbIDs(row.Entries), err)
	}
	search, err := d.svc.SearchWishTitles(d.ctx, d.noah12, "rated", 1)
	if err != nil || !slices.Equal(tmdbIDs(search.Entries), []string{"movie:5012"}) {
		t.Errorf("search for 12 = %v, %v", tmdbIDs(search.Entries), err)
	}
	// Marco (18) gets all three, unknown included; Seerr was asked without the pre-filter.
	d.dlib.setCeiling(18)
	all, err := d.svc.DiscoverTitles(d.ctx, d.marco, TitlesQuery{Media: model.WishMediaMovie, Genre: 28})
	if err != nil {
		t.Fatal(err)
	}
	if got := tmdbIDs(all.Entries); !slices.Contains(got, "movie:5016") || !slices.Contains(got, "movie:5099") {
		t.Errorf("films for 18 = %v", got)
	}
	for _, c := range d.fake.CallLog() {
		if c.Route == "GET /discover/movies" && c.Query.Get("genre") == "28" && c.Query.Has("certificationLte") {
			t.Errorf("pre-filter sent for 18: %v", c.Query)
		}
	}
}

// The library match: a film of the library the profile sees comes with its item and no request; a
// library film it may not see is missing; a series that is in the library only by its TVDB id is
// found; mediaInfo «available» of Seerr without a library title does not count as present; the
// own open wish is marked; «Vorhandene ausblenden» drops the library titles (AK-182).
func TestDiscoverLibraryMatch(t *testing.T) {
	d := newDiscoverFixture(t)
	d.film(7000, "Seerr Says Available", 5, age(0), false)
	d.fake.SetMediaStatus("movie", 7000, seerrtest.MediaAvailable, nil)
	d.fake.SetMediaStatus("movie", alien, seerrtest.MediaPending, nil)
	d.dlib.items[refKey("movie", matrix)] = "ITEM-MATRIX"
	d.dlib.items[refKey("movie", 9001)] = "ITEM-HIDDEN"
	d.dlib.hidden["ITEM-HIDDEN"] = true
	d.film(9001, "Hidden Film", 4, age(0), false)
	d.store.tmdb[refKey("movie", alien)] = metadata.ExternalTitle{Media: "movie", TMDBID: alien, MinAge: age(16)}
	// Game of Thrones: in the library under another TMDB id, matched by TVDB only.
	d.dlib.byTVDB[gotTVDB] = "ITEM-GOT"
	d.store.tmdb[refKey("show", got)] = metadata.ExternalTitle{Media: "show", TMDBID: got, TVDBID: gotTVDB}
	d.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (40, 'movie', ?, 'Alien', 'slouchcat', 'pending', 1, 1)`, alien)
	d.exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (40, 1, 1, 'owner', 1)`)

	page, err := d.svc.DiscoverRowPage(d.ctx, d.marco, RowTrending, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]DiscoverEntry{}
	for _, e := range page.Entries {
		by[fmt.Sprintf("%s:%d", e.Media, e.TMDBID)] = e
	}
	if e, ok := by[refKey("movie", matrix)]; !ok || e.ItemID != "ITEM-MATRIX" || e.CanRequest {
		t.Errorf("matrix = %+v, %v; want the library title", e, ok)
	}
	if e, ok := by[refKey("show", got)]; !ok || e.ItemID != "ITEM-GOT" {
		t.Errorf("series by TVDB = %+v, %v; want the library series", e, ok)
	}
	if _, ok := by[refKey("movie", 9001)]; ok {
		t.Error("a library film the profile may not see is listed")
	}
	if e, ok := by[refKey("movie", 7000)]; !ok || e.ItemID != "" || !e.CanRequest || e.SeerrRequested {
		t.Errorf("available in Seerr only = %+v, %v; want missing and requestable", e, ok)
	}
	if e, ok := by[refKey("movie", alien)]; !ok || e.Wish == nil || e.Wish.ID != 40 || e.CanRequest || !e.SeerrRequested {
		t.Errorf("alien = %+v, %v; want the own wish and «in Seerr angefragt»", e, ok)
	}
	hidden, err := d.svc.DiscoverTitles(d.ctx, d.marco, TitlesQuery{Media: model.WishMediaMovie, HideAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := tmdbIDs(hidden.Entries); slices.Contains(got, refKey("movie", matrix)) || !slices.Contains(got, "movie:7000") {
		t.Errorf("hide available = %v", got)
	}
}

// Seerr gone: the older entry with stale; a list never read fails with ErrDiscoverUnreachable.
func TestDiscoverStaleWhileSeerrIsGone(t *testing.T) {
	d := newDiscoverFixture(t)
	d.film(5000, "Twelve", 50, age(12), false)
	first, err := d.svc.DiscoverTitles(d.ctx, d.marco, TitlesQuery{Media: model.WishMediaMovie})
	if err != nil || first.Stale {
		t.Fatalf("first = %+v, %v", first, err)
	}
	d.clock.advance(2 * time.Hour)
	d.fake.SetFailure("*", seerrtest.Failure{Status: 500, Message: "down"})
	old, err := d.svc.DiscoverTitles(d.ctx, d.marco, TitlesQuery{Media: model.WishMediaMovie})
	if err != nil || !old.Stale || !old.FetchedAt.Equal(first.FetchedAt) || !slices.Equal(tmdbIDs(old.Entries), tmdbIDs(first.Entries)) {
		t.Errorf("stale = %+v, %v", old, err)
	}
	if _, err := d.svc.DiscoverTitles(d.ctx, d.marco, TitlesQuery{Media: model.WishMediaShow}); !errors.Is(err, ErrDiscoverUnreachable) {
		t.Errorf("never read: %v, want ErrDiscoverUnreachable", err)
	}
	if _, err := d.svc.SearchWishTitles(d.ctx, d.marco, "twelve", 1); !errors.Is(err, ErrDiscoverUnreachable) {
		t.Errorf("search: %v, want ErrDiscoverUnreachable", err)
	}
	rows, err := d.svc.Discover(d.ctx, d.marco)
	if !errors.Is(err, ErrDiscoverUnreachable) {
		t.Errorf("overview with nothing cached = %+v, %v", rows, err)
	}
}

// Not found for a profile without the request right and with the pages switched off (F-9, N-57).
func TestDiscoverNotFoundWithoutRight(t *testing.T) {
	d := newDiscoverFixture(t)
	oma := model.Profile{ID: 4, AccountID: 3, Language: "de"} // account without the right
	d.fake.ResetCalls()
	calls := map[string]func(model.Profile) error{
		"discover": func(p model.Profile) error { _, err := d.svc.Discover(d.ctx, p); return err },
		"row":      func(p model.Profile) error { _, err := d.svc.DiscoverRowPage(d.ctx, p, RowTrending, 1, 0); return err },
		"titles": func(p model.Profile) error {
			_, err := d.svc.DiscoverTitles(d.ctx, p, TitlesQuery{Media: model.WishMediaMovie})
			return err
		},
		"genres": func(p model.Profile) error {
			_, err := d.svc.DiscoverGenres(d.ctx, p, model.WishMediaMovie)
			return err
		},
		"search": func(p model.Profile) error { _, err := d.svc.SearchWishTitles(d.ctx, p, "matrix", 1); return err },
	}
	for name, call := range calls {
		if err := call(oma); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s without the right: %v, want ErrNotFound", name, err)
		}
	}
	if ok, err := d.svc.DiscoverVisible(d.ctx, oma); ok || err != nil {
		t.Errorf("visible without the right: %v, %v", ok, err)
	}
	if ok, err := d.svc.DiscoverVisible(d.ctx, d.marco); !ok || err != nil {
		t.Errorf("visible for Marco: %v, %v", ok, err)
	}
	d.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('wishes.discover_pages', '0', 1)`)
	for name, call := range calls {
		if err := call(d.marco); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s with the pages off: %v, want ErrNotFound", name, err)
		}
	}
	if n := len(d.fake.CallLog()); n != 0 {
		t.Errorf("calls to Seerr: %d, want 0", n)
	}
	// Unknown row, medium and invalid input.
	d.exec(`UPDATE settings SET value = '1' WHERE key = 'wishes.discover_pages'`)
	if _, err := d.svc.DiscoverRowPage(d.ctx, d.marco, "networks", 1, 0); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown row: %v", err)
	}
	if _, err := d.svc.DiscoverTitles(d.ctx, d.marco, TitlesQuery{Media: "music"}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown medium: %v", err)
	}
	var verr *model.ValidationError
	for name, err := range map[string]error{
		"genre row without genre": func() error { _, err := d.svc.DiscoverRowPage(d.ctx, d.marco, RowMovieGenres, 1, 0); return err }(),
		"genre on trending":       func() error { _, err := d.svc.DiscoverRowPage(d.ctx, d.marco, RowTrending, 1, 28); return err }(),
		"unknown sort": func() error {
			_, err := d.svc.DiscoverTitles(d.ctx, d.marco, TitlesQuery{Media: model.WishMediaMovie, Sort: "rating"})
			return err
		}(),
		"short search": func() error { _, err := d.svc.SearchWishTitles(d.ctx, d.marco, " m ", 1); return err }(),
		"page 501":     func() error { _, err := d.svc.DiscoverRowPage(d.ctx, d.marco, RowTrending, 501, 0); return err }(),
	} {
		if !errors.As(err, &verr) {
			t.Errorf("%s: %v, want a validation error", name, err)
		}
	}
}

// At most 20 detail fetches per page: «Entdecken» for a profile aged 12 with 45 hits nobody has
// fetched yet stays at 20, the rest is left out until fetched.
func TestDiscoverAtMostTwentyDetailsPerPage(t *testing.T) {
	d := newDiscoverFixture(t)
	for i := range 45 {
		d.film(6000+i, fmt.Sprint("Film ", i), float64(100+i), age(6), false)
	}
	d.dlib.setCeiling(12)
	rows, err := d.svc.Discover(d.ctx, d.noah12)
	if err != nil {
		t.Fatal(err)
	}
	if n := d.store.fetches(); n != detailMax {
		t.Errorf("detail fetches = %d, want %d", n, detailMax)
	}
	keys := make([]string, 0, len(rows))
	shown := 0
	for _, r := range rows {
		keys = append(keys, r.Key)
		shown += len(r.Entries)
	}
	if !slices.Equal(keys, DiscoverRows) {
		t.Errorf("rows = %v, want %v", keys, DiscoverRows)
	}
	if shown == 0 || shown > 2*detailMax {
		t.Errorf("entries shown = %d", shown)
	}
	for _, r := range rows {
		if r.Key == RowMovieGenres && (len(r.Genres) != 1 || r.Genres[0].ID != 28 || r.Genres[0].Backdrop == "") {
			t.Errorf("film genres = %+v", r.Genres)
		}
		if r.Key == RowPopularMovies && r.NextPage != 2 {
			t.Errorf("popular films next page = %d, want 2", r.NextPage)
		}
	}
	// A next page has its own budget.
	before := d.store.fetches()
	if _, err := d.svc.DiscoverRowPage(d.ctx, d.noah12, RowPopularMovies, 2, 0); err != nil {
		t.Fatal(err)
	}
	if n := d.store.fetches() - before; n < 1 || n > detailMax {
		t.Errorf("fetches of page 2 = %d", n)
	}
}

// The search by id goes as tmdb:603 without the blank the profile typed, and keeps only the film.
func TestDiscoverSearchByID(t *testing.T) {
	d := newDiscoverFixture(t)
	d.fake.AddTitle(seerrtest.Title{MediaType: "person", TMDBID: matrix, Name: "Some Person"})
	d.store.tmdb[refKey("movie", matrix)] = metadata.ExternalTitle{Media: "movie", TMDBID: matrix, MinAge: age(16)}
	d.fake.ResetCalls()
	page, err := d.svc.SearchWishTitles(d.ctx, d.marco, "TMDB: 603", 1)
	if err != nil || !slices.Equal(tmdbIDs(page.Entries), []string{refKey("movie", matrix)}) {
		t.Fatalf("search = %v, %v", tmdbIDs(page.Entries), err)
	}
	if log := d.fake.CallLog(); len(log) != 1 || log[0].Query.Get("query") != "tmdb:603" {
		t.Errorf("calls = %+v", log)
	}
	// Ten minutes in memory.
	if _, err := d.svc.SearchWishTitles(d.ctx, d.marco, "tmdb:603", 1); err != nil {
		t.Fatal(err)
	}
	if n := len(d.fake.CallLog()); n != 1 {
		t.Errorf("calls after the second search = %d, want 1", n)
	}
}

// wishes.discover: queued only while the wishes and the pages are on; it warms every row for each
// age step in use and deletes rows older than a day.
func TestDiscoverJobWarmsAgeSteps(t *testing.T) {
	d := newDiscoverFixture(t)
	d.film(5000, "Twelve", 50, age(12), false)
	d.exec(`INSERT INTO wish_discover_pages (cache_key, items, fetched_at) VALUES ('old', '[]', ?)`, d.clock.now().Add(-25*time.Hour).UnixMilli())
	if ids, err := d.svc.runDiscover(d.ctx, nil, discoverPriority); err != nil || len(ids) != 1 || d.queue.count(JobDiscover) != 1 {
		t.Fatalf("run = %v, %v", ids, err)
	}
	if err := d.svc.handleDiscover(d.ctx, &jobs.Job{Type: JobDiscover}); err != nil {
		t.Fatal(err)
	}
	if n := d.count(`SELECT count(*) FROM wish_discover_pages WHERE cache_key = 'old'`); n != 0 {
		t.Errorf("old row kept")
	}
	// Marco 18, Noah 12 and the guest profile 3 (18, same step as Marco): two steps, seven rows each.
	for _, step := range []string{"18", "12"} {
		if n := d.count(`SELECT count(*) FROM wish_discover_pages WHERE cache_key LIKE '%|de|' || ?`, step); n != len(DiscoverRows) {
			t.Errorf("rows of step %s = %d, want %d", step, n, len(DiscoverRows))
		}
	}
	d.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('wishes.discover_pages', '0', 1)`)
	if ids, err := d.svc.runDiscover(d.ctx, nil, discoverPriority); err != nil || len(ids) != 0 {
		t.Errorf("run with the pages off = %v, %v", ids, err)
	}
}
