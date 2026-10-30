package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Library views of concept 08 (ADR 0016): the side navigation, the library page with its filters,
// sort orders, status and facets, and the page of Cat's recommendations.

func libraryFixture(t *testing.T) *vistest.Fixture {
	t.Helper()
	f := vistest.New(t)
	f.AddEdgeCases(t)
	f.AddSeries(t)
	f.AddAudio(t)
	f.AddDiscover(t)
	return f
}

// topLevel reports whether a fixture title is a top-level title of its library.
func topLevel(m vistest.Movie) bool {
	return m.Kind == "" || m.Kind == "series" || m.Kind == "audiobook" || m.Kind == "podcast"
}

// libraryOracle returns the ids of the top-level titles of a library that p may see, by the
// fixture's own rules.
func libraryOracle(f *vistest.Fixture, p model.Profile, libraryID int64) []string {
	var out []string
	for _, m := range f.All() {
		if m.LibraryID == libraryID && topLevel(m) && f.VisibleTo(p, m) {
			out = append(out, m.ID)
		}
	}
	slices.Sort(out)
	return out
}

// libraryItems runs q (limit 100 unless set; MaxRating 0 means no rating filter here, the test of
// maxRating 0 calls the reader itself) and fails on an error.
func libraryItems(t *testing.T, r *Reader, p model.Profile, libraryID int64, q model.LibraryQuery) model.LibraryPage {
	t.Helper()
	if q.Limit == 0 {
		q.Limit = MaxLibraryPage
	}
	if q.MaxRating == 0 {
		q.MaxRating = -1
	}
	page, err := r.LibraryItems(context.Background(), Profile(p), libraryID, q)
	if err != nil {
		t.Fatalf("library %d for %s with %+v: %v", libraryID, p.Name, q, err)
	}
	return page
}

func sorted(v []string) []string {
	out := slices.Clone(v)
	slices.Sort(out)
	return out
}

func TestVisibleLibraries(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	books, pods := f.Audio.AudiobookLibraryID, f.Audio.PodcastLibraryID
	for _, tc := range []struct {
		p    model.Profile
		libs []int64
	}{
		{f.P18, []int64{f.LibraryID(), f.OtherLibraryID(), books, pods}},
		{f.P0, []int64{f.LibraryID(), f.OtherLibraryID(), books, pods}},
		{f.PX, []int64{f.LibraryID()}},
	} {
		libs, err := r.VisibleLibraries(ctx, Profile(tc.p))
		if err != nil {
			t.Fatal(err)
		}
		var got []int64
		for _, l := range libs {
			got = append(got, l.ID)
			if want := len(libraryOracle(f, tc.p, l.ID)); l.ItemCount != want {
				t.Errorf("%s library %d: itemCount %d, want %d top-level titles", tc.p.Name, l.ID, l.ItemCount, want)
			}
			want := map[int64][]string{f.LibraryID(): {"movie"}, f.OtherLibraryID(): {"movie"}, books: {"audiobook"}, pods: {"podcast"}}[l.ID]
			if !slices.Equal(l.Contents, want) {
				t.Errorf("%s library %d: contents %v, want %v", tc.p.Name, l.ID, l.Contents, want)
			}
		}
		if !slices.Equal(got, tc.libs) {
			t.Errorf("%s libraries = %v, want %v", tc.p.Name, got, tc.libs)
		}
	}
	// The series counts once, its seasons, episodes and extras never.
	libs, err := r.VisibleLibraries(ctx, Profile(f.P18))
	if err != nil {
		t.Fatal(err)
	}
	if libs[0].ItemCount != 8 || libs[0].Name != "Filme" || libs[0].Kind != model.LibraryVideo {
		t.Errorf("P18 library 1 = %+v, want Filme (video) with six films, the filler and the series", libs[0])
	}
	if _, err := r.VisibleLibraries(ctx, System("test")); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: %v, want ErrUnscoped", err)
	}
}

// K-3: every profile sees exactly its titles of the library, maxRating only lowers the ceiling
// and the ratings facet stays at or below it.
func TestLibraryItemsCeiling(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	for _, p := range []model.Profile{f.P0, f.P6, f.P12, f.P16, f.P18, f.PX} {
		page := libraryItems(t, r, p, f.LibraryID(), model.LibraryQuery{})
		want := libraryOracle(f, p, f.LibraryID())
		if got := sorted(ids(page.Items)); !slices.Equal(got, want) || page.Total != len(want) {
			t.Errorf("%s library 1 = %v (total %d), want %v", p.Name, got, page.Total, want)
		}
		clamped := libraryItems(t, r, p, f.LibraryID(), model.LibraryQuery{MaxRating: 18})
		if !slices.Equal(ids(clamped.Items), ids(page.Items)) && p.MaxRating < 18 {
			t.Errorf("%s maxRating 18 = %v, want the same as without %v", p.Name, ids(clamped.Items), ids(page.Items))
		}
		for _, rt := range page.Facets.Ratings {
			if rt > p.MaxRating {
				t.Errorf("%s ratings facet %v above the ceiling %d", p.Name, page.Facets.Ratings, p.MaxRating)
			}
		}
		if len(page.Facets.Genres) != 1 || page.Facets.Genres[0].Count != len(want) {
			t.Errorf("%s genres facet = %+v, want one genre on %d titles", p.Name, page.Facets.Genres, len(want))
		}
	}
	if got := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{}).Facets.Ratings; !slices.Equal(got, []int{0, 6, 12, 16, 18}) {
		t.Errorf("P18 ratings facet = %v, want 0 6 12 16 18 (the unrated film has none)", got)
	}
	if got := libraryItems(t, r, f.P6, f.LibraryID(), model.LibraryQuery{}).Facets.Ratings; !slices.Equal(got, []int{0, 6}) {
		t.Errorf("P6 ratings facet = %v, want 0 6", got)
	}
	// maxRating 6 for P18: the known ratings up to 6, never the unrated film.
	page := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{MaxRating: 6})
	if want := sorted([]string{f.Rated(0).ID, f.Rated(6).ID}); !slices.Equal(sorted(ids(page.Items)), want) {
		t.Errorf("P18 maxRating 6 = %v, want %v", ids(page.Items), want)
	}
	// maxRating 0 is a filter too (0 is a valid ceiling, -1 is none).
	zero, err := r.LibraryItems(context.Background(), Profile(f.P18), f.LibraryID(), model.LibraryQuery{MaxRating: 0})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(zero.Items), []string{f.Rated(0).ID}) {
		t.Errorf("P18 maxRating 0 = %v, want the film rated 0", ids(zero.Items))
	}
}

// A library without a visible title is 404 like an unknown or a not permitted one.
func TestLibraryItemsNotFound(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	exec(t, f, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (99, 'Leer', 'video', 1, 1)`)
	for _, tc := range []struct {
		p   model.Profile
		lib int64
	}{
		{f.PX, f.OtherLibraryID()}, // not permitted
		{f.P18, 99},                // permitted, empty
		{f.P18, 12345},             // unknown
	} {
		if _, err := r.LibraryItems(ctx, Profile(tc.p), tc.lib, model.LibraryQuery{MaxRating: -1}); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s library %d: %v, want ErrNotFound", tc.p.Name, tc.lib, err)
		}
	}
	// A library whose only titles lie above the ceiling does not exist for the profile.
	exec(t, f, `UPDATE items SET own_rating = 18 WHERE library_id = ? AND kind = 'movie'`, f.OtherLibraryID())
	if _, err := r.LibraryItems(ctx, Profile(f.P0), f.OtherLibraryID(), model.LibraryQuery{MaxRating: -1}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P0 library X above its ceiling: %v, want ErrNotFound", err)
	}
}

func TestLibraryItemsValidation(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	for _, tc := range []struct {
		lib         int64
		q           model.LibraryQuery
		field, code string
	}{
		{f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortAdded, Letter: "A"}, "letter", "letter_requires_title_sort"},
		{f.LibraryID(), model.LibraryQuery{Letter: "a"}, "letter", "invalid_value"},
		{f.LibraryID(), model.LibraryQuery{Letter: "AB"}, "letter", "invalid_value"},
		{f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortAuthor}, "sort", "invalid_value"},
		{f.LibraryID(), model.LibraryQuery{Sort: "rating"}, "sort", "invalid_value"},
		{f.LibraryID(), model.LibraryQuery{Order: "up"}, "order", "invalid_value"},
		{f.LibraryID(), model.LibraryQuery{Status: []string{"seen"}}, "status", "invalid_value"},
		{f.LibraryID(), model.LibraryQuery{Content: model.RootSeries}, "content", "invalid_value"},
		{f.Audio.PodcastLibraryID, model.LibraryQuery{Sort: model.LibrarySortAuthor}, "sort", "invalid_value"},
	} {
		tc.q.MaxRating = -1
		_, err := r.LibraryItems(ctx, Profile(f.P18), tc.lib, tc.q)
		var verr *model.ValidationError
		if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != tc.field || verr.Fields[0].Code != tc.code {
			t.Errorf("library %d %+v: %v, want %s %s", tc.lib, tc.q, err, tc.field, tc.code)
		}
	}
	// Valid: author in the audiobook library, the podcast content of a feed library, # with title.
	libraryItems(t, r, f.P18, f.Audio.AudiobookLibraryID, model.LibraryQuery{Sort: model.LibrarySortAuthor})
	libraryItems(t, r, f.P18, f.Audio.PodcastLibraryID, model.LibraryQuery{Content: model.RootPodcast})
	libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Letter: "#", Content: model.RootMovie})
}

// The letter bar: the sort key of the profile language, then the English one, then the file
// name's sort title; # for anything but a-z.
func TestLibraryItemsLetters(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	exec(t, f, `UPDATE item_texts SET sort_key = '1984' WHERE item_id = ? AND lang = 'de'`, f.Rated(0).ID)
	exec(t, f, `UPDATE item_texts SET sort_key = 'aaa' WHERE item_id = ? AND lang = 'en'`, f.Rated(6).ID)

	page := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{})
	if !slices.Equal(page.Facets.Letters, []string{"A", "M", "S", "#"}) {
		t.Errorf("P18 letters = %v, want A M S #", page.Facets.Letters)
	}
	// Title order: "1984" < "aaa" (English fallback of the 6) < "movie …" < "series …".
	if got := ids(page.Items); got[0] != f.Rated(0).ID || got[1] != f.Rated(6).ID || got[len(got)-1] != f.Show.Series.ID {
		t.Errorf("P18 title order = %v", got)
	}
	desc := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Order: "desc"})
	if got := ids(desc.Items); got[0] != f.Show.Series.ID || got[len(got)-1] != f.Rated(0).ID {
		t.Errorf("P18 title order desc = %v", got)
	}
	hash := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Letter: "#"})
	if !slices.Equal(ids(hash.Items), []string{f.Rated(0).ID}) || hash.Total != 1 {
		t.Errorf("P18 letter # = %v (total %d), want the film rated 0", ids(hash.Items), hash.Total)
	}
	if !slices.Equal(hash.Facets.Letters, page.Facets.Letters) {
		t.Errorf("P18 letters with the letter filter = %v, want them without it %v", hash.Facets.Letters, page.Facets.Letters)
	}
	if s := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Letter: "S"}); !slices.Equal(ids(s.Items), []string{f.Show.Series.ID}) {
		t.Errorf("P18 letter S = %v, want the series", ids(s.Items))
	}
	// P6 reads English: the 0 sorts by its file name, the 6 by its English key.
	p6 := libraryItems(t, r, f.P6, f.LibraryID(), model.LibraryQuery{})
	if !slices.Equal(p6.Facets.Letters, []string{"A", "M"}) || ids(p6.Items)[0] != f.Rated(6).ID {
		t.Errorf("P6 letters = %v, order %v", p6.Facets.Letters, ids(p6.Items))
	}
	// The facet follows the other filters: films only, no series letter.
	films := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Content: model.RootMovie})
	if !slices.Equal(films.Facets.Letters, []string{"A", "M", "#"}) || slices.Contains(ids(films.Items), f.Show.Series.ID) {
		t.Errorf("P18 films letters = %v, items %v", films.Facets.Letters, ids(films.Items))
	}
}

// Sort by vote, runtime and release date: unknown values last in either direction, ties by title.
func TestLibraryItemsSortNullsLast(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	exec(t, f, `UPDATE items SET vote_average = 8.0 WHERE id = ?`, f.Rated(6).ID)
	exec(t, f, `UPDATE items SET vote_average = 6.5 WHERE id = ?`, f.Rated(12).ID)
	exec(t, f, `UPDATE items SET release_date = NULL, runtime_minutes = NULL WHERE id = ?`, f.Rated(16).ID)

	vote := ids(libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortVote}).Items)
	if vote[0] != f.Rated(6).ID || vote[1] != f.Rated(12).ID {
		t.Errorf("vote desc = %v, want 6 (8.0), 12 (6.5) first", vote)
	}
	asc := ids(libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortVote, Order: "asc"}).Items)
	if asc[0] != f.Rated(12).ID || asc[1] != f.Rated(6).ID {
		t.Errorf("vote asc = %v, want 12, 6 first and the unknown after them", asc)
	}
	if rest := asc[2:]; slices.Contains(rest, f.Rated(6).ID) || slices.Contains(rest, f.Rated(12).ID) || len(rest) != len(vote)-2 {
		t.Errorf("vote asc rest = %v", rest)
	}
	for _, sort := range []string{model.LibrarySortReleased, model.LibrarySortRuntime} {
		for _, order := range []string{"asc", "desc"} {
			got := ids(libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Sort: sort, Order: order}).Items)
			if got[len(got)-1] != f.Rated(16).ID {
				t.Errorf("%s %s = %v, want the film rated 16 without a value last", sort, order, got)
			}
		}
	}
	released := ids(libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortReleased}).Items)
	if released[0] != f.Show.Series.ID { // released 2030-05-01 by the fixture's numbering
		t.Errorf("released desc = %v, want the series (2030) first", released)
	}
	age := ids(libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortAge}).Items)
	if age[0] != f.Rated(0).ID || age[len(age)-1] != f.Unrated().ID {
		t.Errorf("age asc = %v, want the film rated 0 first and the unrated one last", age)
	}
	page := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortVote, Offset: 1, Limit: 2})
	if !slices.Equal(ids(page.Items), vote[1:3]) || page.Total != len(vote) {
		t.Errorf("vote page 1+2 = %v (total %d), want %v of %d", ids(page.Items), page.Total, vote[1:3], len(vote))
	}
}

// Status per profile (concept 08, 4.2): films by their own progress, the series by its visible
// episodes only; exclusive; series progress completed for watched.
func TestLibraryItemsStatus(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	s := f.Show
	status := func(p model.Profile, st ...string) []string {
		return sorted(ids(libraryItems(t, r, p, f.LibraryID(), model.LibraryQuery{Status: st}).Items))
	}
	all := libraryOracle(f, f.P18, f.LibraryID())
	// P18 started the film rated 12 (fixture), nothing of the series yet.
	if got := status(f.P18, model.LibraryStatusStarted); !slices.Equal(got, []string{f.Rated(12).ID}) {
		t.Errorf("P18 started = %v, want the film rated 12", got)
	}
	if got := status(f.P18, model.LibraryStatusUnwatched, model.LibraryStatusStarted, model.LibraryStatusWatched); !slices.Equal(got, all) {
		t.Errorf("P18 all three = %v, want every title %v", got, all)
	}
	// E1 completed: the series is started for P18, P12 and P16.
	for _, p := range []model.Profile{f.P18, f.P16, f.P12} {
		exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 0, 100, 1, 9000)`, p.ID, s.E1.ID)
	}
	if got := status(f.P16, model.LibraryStatusStarted); !slices.Equal(got, []string{s.Series.ID}) {
		t.Errorf("P16 started = %v, want the series", got)
	}
	// E3 and the special completed too: P12 sees no E2, so all its visible episodes are done;
	// P16 sees E2, which is open.
	for _, p := range []model.Profile{f.P16, f.P12} {
		for _, e := range []vistest.Movie{s.E3, s.Special} {
			exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 0, 100, 1, 9500)`, p.ID, e.ID)
		}
	}
	if got := status(f.P12, model.LibraryStatusWatched); !slices.Equal(got, []string{s.Series.ID}) {
		t.Errorf("P12 watched = %v, want the series (E2 is above its ceiling)", got)
	}
	if got := status(f.P16, model.LibraryStatusWatched); len(got) != 0 {
		t.Errorf("P16 watched = %v, want none (E2 open)", got)
	}
	if got := status(f.P16, model.LibraryStatusStarted); !slices.Equal(got, []string{s.Series.ID}) {
		t.Errorf("P16 started = %v, want the series", got)
	}
	// Exclusive: the series is in exactly one status for P12.
	if got := status(f.P12, model.LibraryStatusUnwatched, model.LibraryStatusStarted); slices.Contains(got, s.Series.ID) {
		t.Errorf("P12 unwatched or started = %v, contains the watched series", got)
	}
	// The series carries its status as progress.
	for _, tc := range []struct {
		p         model.Profile
		completed bool
	}{{f.P12, true}, {f.P16, false}} {
		page := libraryItems(t, r, tc.p, f.LibraryID(), model.LibraryQuery{Letter: "S"})
		if len(page.Items) != 1 || page.Items[0].Progress == nil || page.Items[0].Progress.Completed != tc.completed ||
			page.Items[0].Progress.UpdatedAt.UnixMilli() != 9500 {
			t.Errorf("%s series = %+v, want progress completed %v at 9500", tc.p.Name, page.Items, tc.completed)
		}
	}
	if page := libraryItems(t, r, f.P6, f.LibraryID(), model.LibraryQuery{}); slices.ContainsFunc(page.Items, func(it model.ItemSummary) bool {
		return it.Kind == "series"
	}) {
		t.Errorf("P6 sees the series rated 12")
	}
	// Played: the series (9000) before the film rated 12 (5000), never played after them.
	played := ids(libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortPlayed}).Items)
	if played[0] != s.Series.ID || played[1] != f.Rated(12).ID {
		t.Errorf("P18 played = %v, want the series and the film rated 12 first", played)
	}
}

// The podcast is watched by its visible episodes like a series; audiobooks by their own progress.
func TestLibraryItemsAudioStatus(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	book := f.Audio.Books[0]
	exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 10, 100, 0, 7000)`, f.P0.ID, book.ID)
	got := ids(libraryItems(t, r, f.P0, f.Audio.AudiobookLibraryID, model.LibraryQuery{Status: []string{model.LibraryStatusStarted}}).Items)
	if !slices.Equal(got, []string{book.ID}) {
		t.Errorf("P0 started books = %v, want %s", got, book.Label)
	}
	pods := f.Audio.PodcastLibraryID
	for _, e := range f.Audio.Episodes {
		if f.VisibleTo(f.P0, e) {
			exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 0, 100, 1, 7000)`, f.P0.ID, e.ID)
		}
	}
	started := ids(libraryItems(t, r, f.P0, pods, model.LibraryQuery{Status: []string{model.LibraryStatusStarted, model.LibraryStatusWatched}}).Items)
	if !slices.Equal(started, []string{f.Audio.Podcast.ID}) {
		t.Errorf("P0 started or watched podcasts = %v, want the podcast", started)
	}
}

// listPicks: the row's list without its cut, total after the deduplication, paged.
func TestPicksPage(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	for _, p := range []model.Profile{f.P0, f.P12, f.P18} {
		row, err := r.Picks(ctx, Profile(p), MaxRowItems)
		if err != nil {
			t.Fatal(err)
		}
		page, total, err := r.PicksPage(ctx, Profile(p), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(page), ids(row)) || total != len(row) {
			t.Errorf("%s picks page = %v (total %d), want the row %v", p.Name, ids(page), total, ids(row))
		}
		for i := range page {
			if page[i].Note != row[i].Note {
				t.Errorf("%s pick %s note %q, want %q", p.Name, page[i].ID, page[i].Note, row[i].Note)
			}
		}
		if len(row) > 1 {
			tail, total2, err := r.PicksPage(ctx, Profile(p), 1, 1)
			if err != nil || total2 != total || !slices.Equal(ids(tail), ids(row)[1:2]) {
				t.Errorf("%s picks 1+1 = %v (total %d, %v), want %v", p.Name, ids(tail), total2, err, ids(row)[1:2])
			}
		}
		if beyond, _, err := r.PicksPage(ctx, Profile(p), 500, 10); err != nil || len(beyond) != 0 {
			t.Errorf("%s picks beyond the end = %v, %v", p.Name, ids(beyond), err)
		}
	}
	// Without the cut of 20: 25 global picks of the film rated 0 in library 1 deduplicate to one,
	// so add distinct titles instead.
	f.AddSpotlightFilms(t, 25)
	for k := 1; k <= 25; k++ {
		exec(t, f, `INSERT INTO top_picks (item_id, position, created_at) VALUES (printf('01J9SPOTLIGHT00000000%05d', ?), ?, 1)`, k, 100+k)
	}
	row, err := r.Picks(ctx, Profile(f.P0), MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	page, total, err := r.PicksPage(ctx, Profile(f.P0), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(row) != MaxRowItems || total != 26 || len(page) != 26 || !slices.Equal(ids(page)[:MaxRowItems], ids(row)) {
		t.Errorf("P0 picks: row %d, page %d of %d, want 20 and 26 of 26 in the same order", len(row), len(page), total)
	}
}
