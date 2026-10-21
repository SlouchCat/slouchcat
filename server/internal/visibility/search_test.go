package visibility

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func TestMatchExpression(t *testing.T) {
	for in, want := range map[string]string{
		"Fi":                   `"Fi"*`,
		"  Herr der  Ringe ":   `"Herr"* "der"* "Ringe"*`,
		`Star-Wars: "Episode"`: `"Star"* "Wars"* "Episode"*`,
		"Ärger 2":              `"Ärger"*`,
		"-- !":                 "",
		// k6: FTS5 syntax stays literal, one-character words go when a longer one is there.
		"a OR b":    `"OR"*`,
		"text:foo":  `"text"* "foo"*`,
		"NEAR(a b)": `"NEAR"*`,
		"^x":        `"x"*`,
		"a b":       `"a"* "b"*`,
		"Fi x":      `"Fi"*`,
		"eins zwei drei vier fünf sechs sieben acht neun zehn": `"eins"* "zwei"* "drei"* "vier"* "fünf"* "sechs"* "sieben"* "acht"*`,
		"a b c d e f g h i j": `"a"* "b"* "c"* "d"* "e"* "f"* "g"* "h"*`,
	} {
		if got := MatchExpression(in); got != want {
			t.Errorf("MatchExpression(%q) = %q, want %q", in, got, want)
		}
	}
}

func searchFixture(t *testing.T) *vistest.Fixture {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddAudio(t)
	f.AddDiscover(t)
	return f
}

// search runs q for p (Limit 100, no rating filter unless set) and fails on an error or on a hit
// the profile may not see.
func search(t *testing.T, r *Reader, f *vistest.Fixture, p model.Profile, q model.SearchQuery) model.SearchResult {
	t.Helper()
	if q.Limit == 0 {
		q.Limit = MaxSearchPage
	}
	res, err := r.Search(context.Background(), Profile(p), q)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range res.Items {
		m, ok := lookup(f, it.ID)
		if !ok || !f.VisibleTo(p, m) || it.Kind == "season" || it.Kind == "extra" {
			t.Errorf("%s search %+v: hit %s (%s) must not appear", p.Name, q, it.ID, it.Kind)
		}
	}
	return res
}

func lookup(f *vistest.Fixture, id string) (vistest.Movie, bool) {
	for _, m := range f.All() {
		if m.ID == id {
			return m, true
		}
	}
	return vistest.Movie{}, false
}

func textQuery(q string) model.SearchQuery { return model.SearchQuery{Text: q, MaxRating: -1} }

// E-4: two characters find titles, people and collections, each only through visible titles.
func TestSearchTitlesPeopleCollections(t *testing.T) {
	f := searchFixture(t)
	r := New(f.DB, vistest.Logger())
	for _, tc := range []struct {
		p          model.Profile
		level      vistest.Movie
		above      *vistest.Movie
		person     bool
		collection int // visible members of the mixed collection, 0 = absent
	}{
		{f.P0, f.Rated(0), ptrTo(f.Rated(6)), false, 0},
		{f.P6, f.Rated(6), ptrTo(f.Rated(12)), true, 1},
		{f.P12, f.Rated(12), ptrTo(f.Rated(16)), true, 2},
		{f.P16, f.Rated(16), ptrTo(f.Rated(18)), true, 2},
		{f.P18, f.Rated(18), nil, true, 3},
	} {
		res := search(t, r, f, tc.p, textQuery("Fi")) // "Film NN", the German titles
		got := ids(res.Items)
		if !slices.Contains(got, tc.level.ID) || (tc.above != nil && slices.Contains(got, tc.above.ID)) {
			t.Errorf("%s \"Fi\" = %v, want the film rated %s and not the next level", tc.p.Name, got, tc.level.Label)
		}
		if res.Total != len(res.Items) || len(res.People) != 0 || len(res.Collections) != 0 {
			t.Errorf("%s \"Fi\": total %d of %d, people %v, collections %v", tc.p.Name, res.Total, len(res.Items), res.People, res.Collections)
		}

		res = search(t, r, f, tc.p, textQuery("Pat"))
		if want := tc.person; want != (len(res.People) == 1 && res.People[0] == model.PersonRef{ID: f.Person.ID, Name: f.Person.Name}) {
			t.Errorf("%s \"Pat\" people = %+v, want the person %v", tc.p.Name, res.People, want)
		}
		if len(res.Items) != 0 {
			t.Errorf("%s \"Pat\" titles = %v, want none (people are no title hits)", tc.p.Name, ids(res.Items))
		}

		res = search(t, r, f, tc.p, textQuery("Samm"))
		switch {
		case tc.collection == 0 && len(res.Collections) != 0:
			t.Errorf("%s \"Samm\" collections = %+v, want none", tc.p.Name, res.Collections)
		case tc.collection > 0 && (len(res.Collections) != 1 || res.Collections[0].ID != vistest.MixedCollectionID ||
			res.Collections[0].Count != tc.collection || res.Collections[0].Cover == nil):
			t.Errorf("%s \"Samm\" collections = %+v, want the mixed collection with %d members", tc.p.Name, res.Collections, tc.collection)
		}
	}
	// The manual collection holds only the film rated 18.
	if res := search(t, r, f, f.P16, textQuery("Late")); len(res.Collections) != 0 {
		t.Errorf("P16 \"Late\" = %+v, want no collection", res.Collections)
	}
	if res := search(t, r, f, f.P18, textQuery("late ni")); len(res.Collections) != 1 || res.Collections[0].ID != vistest.ManualCollectionID {
		t.Errorf("P18 \"late ni\" = %+v, want the manual collection", res.Collections)
	}
	// Every word must match, as a prefix; English titles, original titles and authors count too.
	if res := search(t, r, f, f.P18, textQuery("movie 12")); !slices.Equal(ids(res.Items), []string{f.Rated(12).ID}) {
		t.Errorf("P18 \"movie 12\" = %v, want the film rated 12", ids(res.Items))
	}
	if res := search(t, r, f, f.P18, textQuery("Origin 16")); !slices.Equal(ids(res.Items), []string{f.Rated(16).ID}) {
		t.Errorf("P18 \"Origin 16\" = %v, want the film rated 16", ids(res.Items))
	}
	if res := search(t, r, f, f.P6, textQuery("Autorin")); len(res.Items) == 0 {
		t.Errorf("P6 \"Autorin\" finds no audiobook")
	}
	// Diacritics are folded on both sides; the index follows the texts in the same transaction.
	exec(t, f, `UPDATE item_texts SET title = 'Ärger im Paradies' WHERE item_id = ? AND lang = 'de'`, f.Rated(6).ID)
	if res := search(t, r, f, f.P6, textQuery("arg par")); !slices.Equal(ids(res.Items), []string{f.Rated(6).ID}) {
		t.Errorf("P6 \"arg par\" = %v, want the film rated 6", ids(res.Items))
	}
	// Less than two characters searches nothing.
	if res := search(t, r, f, f.P18, textQuery("F")); len(res.Items)+len(res.People)+len(res.Collections) != 0 || res.Total != 0 {
		t.Errorf("P18 \"F\" = %+v, want nothing", res)
	}
}

func ptrTo(m vistest.Movie) *vistest.Movie { return &m }

// E-4: each filter alone and combined; with a text and without one (browse).
func TestSearchFilters(t *testing.T) {
	f := searchFixture(t)
	r := New(f.DB, vistest.Logger())
	exec(t, f, `UPDATE items SET genres = '{"de":["Abenteuer","Drama"],"en":["Adventure","Drama"]}' WHERE id IN (?, ?)`, f.Rated(12).ID, f.Rated(18).ID)
	films := func(ratings ...int) []string {
		var out []string
		for _, n := range ratings {
			out = append(out, f.Rated(n).ID)
		}
		return out
	}
	kinds := func(res model.SearchResult) []string {
		var out []string
		for _, it := range res.Items {
			if !slices.Contains(out, it.Kind) {
				out = append(out, it.Kind)
			}
		}
		slices.Sort(out)
		return out
	}

	q := textQuery("Fi")
	q.Genre = "Drama"
	if res := search(t, r, f, f.P18, q); !slices.Equal(ids(res.Items), films(12, 18)) {
		t.Errorf("genre Drama = %v", ids(res.Items))
	}
	q = textQuery("Fi")
	q.Genre = "Drama" // the genre name of an English profile is English
	if res := search(t, r, f, f.P6, q); len(res.Items) != 0 {
		t.Errorf("P6 genre Drama = %v, want none", ids(res.Items))
	}
	q = textQuery("Fi")
	q.YearFrom, q.YearTo = 2012, 2013
	if res := search(t, r, f, f.P18, q); !slices.Equal(ids(res.Items), films(12, 16)) {
		t.Errorf("years 2012-2013 = %v", ids(res.Items))
	}
	q = textQuery("Fi")
	q.YearFrom = 2014
	if res := search(t, r, f, f.P18, q); !slices.Contains(ids(res.Items), f.Rated(18).ID) || slices.Contains(ids(res.Items), f.Rated(16).ID) {
		t.Errorf("from 2014 = %v", ids(res.Items))
	}
	q = textQuery("Fi")
	q.YearTo = 2010
	if res := search(t, r, f, f.P18, q); !slices.Equal(ids(res.Items), films(0)) {
		t.Errorf("to 2010 = %v", ids(res.Items))
	}
	all := ids(search(t, r, f, f.P18, textQuery("Fi")).Items)
	q = textQuery("Fi")
	q.MaxRating = 18 // an unknown rating never passes
	if res := ids(search(t, r, f, f.P18, q).Items); !slices.Contains(all, f.Unrated().ID) || slices.Contains(res, f.Unrated().ID) || !slices.Contains(res, f.Rated(18).ID) {
		t.Errorf("max 18 = %v, all = %v", res, all)
	}
	q = textQuery("Fi")
	q.MaxRating = 0
	for _, it := range search(t, r, f, f.P18, q).Items {
		if it.Rating == nil || *it.Rating != 0 {
			t.Errorf("max 0: hit %s rated %v", it.ID, it.Rating)
		}
	}
	for _, tc := range []struct {
		types []string
		want  []string
	}{
		{[]string{"movie"}, []string{"movie"}},
		{[]string{"series"}, []string{"episode", "series"}},
		{[]string{"audiobook"}, []string{"audiobook"}},
		{[]string{"podcast"}, []string{"podcast", "podcast_episode"}},
		{[]string{"movie", "audiobook"}, []string{"audiobook", "movie"}},
	} {
		q = textQuery("Fi")
		q.MediaTypes = tc.types
		if got := kinds(search(t, r, f, f.P18, q)); !slices.Equal(got, tc.want) {
			t.Errorf("media types %v = kinds %v, want %v", tc.types, got, tc.want)
		}
	}
	// Local: a present file, for a series or podcast one of an episode; the episode with only its
	// enclosure is not local.
	q = textQuery("Fi")
	q.Availability = []string{"local"}
	local := ids(search(t, r, f, f.P18, q).Items)
	if !slices.Contains(local, f.Show.Series.ID) || !slices.Contains(local, f.Rated(6).ID) || !slices.Contains(local, f.Audio.Podcast.ID) ||
		slices.Contains(local, f.Audio.EnclosureOnly.ID) || !slices.Contains(all, f.Audio.EnclosureOnly.ID) {
		t.Errorf("local = %v", local)
	}
	exec(t, f, `UPDATE media_files SET missing_since = 5 WHERE item_id = ? AND id = ?`, f.Rated(16).ID, f.Rated(16).FileID)
	if slices.Contains(ids(search(t, r, f, f.P18, q).Items), f.Rated(16).ID) {
		t.Errorf("local contains the film whose only file is missing")
	}
	// Download: ready on this device and allowed for the profile.
	device := "0123456789abcdef0123456789abcdef"
	exec(t, f, `INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, 1)`, device)
	exec(t, f, `INSERT INTO downloads (id, account_id, device_id, item_id, media_file_id, quality, audio_stream, state, estimated_bytes,
	            allowed_profile_ids, created_at, updated_at) VALUES
	            ('01J9000000000000000000DL06', ?, ?, ?, ?, 'low', 1, 'ready', 1, ?, 1, 1),
	            ('01J9000000000000000000DL12', ?, ?, ?, ?, 'low', 1, 'preparing', 1, ?, 1, 1),
	            ('01J9000000000000000000DL00', ?, ?, ?, ?, 'low', 1, 'ready', 1, ?, 1, 1)`,
		f.Account.ID, device, f.Rated(6).ID, f.Rated(6).FileID, "["+strconv.FormatInt(f.P6.ID, 10)+","+strconv.FormatInt(f.P18.ID, 10)+"]",
		f.Account.ID, device, f.Rated(12).ID, f.Rated(12).FileID, "["+strconv.FormatInt(f.P18.ID, 10)+"]",
		f.Account.ID, device, f.Rated(0).ID, f.Rated(0).FileID, "["+strconv.FormatInt(f.P6.ID, 10)+"]")
	q = textQuery("Fi")
	q.Availability, q.DeviceID = []string{"download"}, device
	if res := search(t, r, f, f.P18, q); !slices.Equal(ids(res.Items), films(6)) {
		t.Errorf("P18 download = %v, want the film rated 6 (12 is not ready, 0 not allowed)", ids(res.Items))
	}
	if res := search(t, r, f, f.P6, q); !slices.Equal(ids(res.Items), films(0, 6)) {
		t.Errorf("P6 download = %v", ids(res.Items))
	}
	q.DeviceID = "fedcba9876543210fedcba9876543210"
	if res := search(t, r, f, f.P18, q); len(res.Items) != 0 {
		t.Errorf("download on another device = %v", ids(res.Items))
	}
	q.Availability, q.DeviceID = []string{"download", "local"}, device
	if res := ids(search(t, r, f, f.P18, q).Items); !slices.Contains(res, f.Rated(12).ID) || slices.Contains(res, f.Rated(16).ID) {
		t.Errorf("download or local = %v", res)
	}

	// Combined: genre, media type, rating and years narrow each other.
	q = textQuery("Fi")
	q.Genre, q.MediaTypes, q.MaxRating, q.YearFrom = "Abenteuer", []string{"movie"}, 12, 2011
	if res := search(t, r, f, f.P18, q); !slices.Equal(ids(res.Items), films(6, 12)) || res.Total != 2 {
		t.Errorf("combined = %v (%d)", ids(res.Items), res.Total)
	}
	q.Genre = "Drama"
	if res := search(t, r, f, f.P18, q); !slices.Equal(ids(res.Items), films(12)) {
		t.Errorf("combined with Drama = %v", ids(res.Items))
	}
}

// Without a text the filters browse the visible titles by title, without people and collections.
func TestSearchBrowseByFilters(t *testing.T) {
	f := searchFixture(t)
	r := New(f.DB, vistest.Logger())
	q := model.SearchQuery{Genre: "Adventure", MaxRating: -1, Limit: MaxSearchPage}
	res := search(t, r, f, f.P6, q)
	if len(res.Items) == 0 || res.Total != len(res.Items) || len(res.People) != 0 || len(res.Collections) != 0 {
		t.Fatalf("P6 browse Adventure = %+v", res)
	}
	for _, m := range f.All() {
		present := slices.Contains(ids(res.Items), m.ID)
		if m.Kind != "season" && m.Kind != "extra" && f.VisibleTo(f.P6, m) != present {
			t.Errorf("P6 browse Adventure: %s present %v, visible %v", m.Label, present, f.VisibleTo(f.P6, m))
		}
	}
	titles := make([]string, 0, len(res.Items))
	for _, it := range res.Items {
		titles = append(titles, it.ID)
	}
	q.MediaTypes = []string{"movie"}
	if got := ids(search(t, r, f, f.P6, q).Items); !slices.Equal(got, []string{f.Rated(0).ID, f.Rated(6).ID}) {
		t.Errorf("P6 browse Adventure films = %v (by sort title)", got)
	}
	// Paging keeps the order.
	q.MediaTypes = nil
	q.Offset, q.Limit = 1, 2
	page := search(t, r, f, f.P6, q)
	if !slices.Equal(ids(page.Items), titles[1:3]) || page.Total != len(titles) {
		t.Errorf("P6 browse page = %v (%d), want %v of %d", ids(page.Items), page.Total, titles[1:3], len(titles))
	}
	// Neither text nor filter: nothing.
	if res := search(t, r, f, f.P18, model.SearchQuery{MaxRating: -1}); len(res.Items) != 0 || res.Total != 0 {
		t.Errorf("empty query = %+v", res)
	}
	// A profile scope is required.
	if _, err := r.Search(context.Background(), System("test"), q); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: err = %v", err)
	}
}

func TestAdminSearch(t *testing.T) {
	f := searchFixture(t)
	f.AddEdgeCases(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	if _, err := r.AdminSearch(ctx, Profile(f.P18), "Fi", 10); !errors.Is(err, ErrNotSystem) {
		t.Errorf("profile scope: err = %v", err)
	}
	hits, err := r.AdminSearch(ctx, System("test"), "Fi", 50)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]model.AdminItemHit{}
	for _, h := range hits {
		byID[h.ID] = h
		if h.Kind == "season" || h.Kind == "extra" {
			t.Errorf("admin hit %s of kind %s", h.ID, h.Kind)
		}
	}
	other := edge(f, func(m vistest.Movie) bool { return m.OtherLibrary })
	for _, m := range []vistest.Movie{f.Rated(18), f.Unrated(), other, edge(f, func(m vistest.Movie) bool { return m.Missing })} {
		if _, ok := byID[m.ID]; !ok {
			t.Errorf("admin search lacks %s (no visibility applies)", m.Label)
		}
	}
	if _, ok := byID[edge(f, func(m vistest.Movie) bool { return m.Deleted }).ID]; ok {
		t.Errorf("admin search contains the deleted film")
	}
	if h := byID[f.Rated(12).ID]; h.Title != "Film 12" || h.Year == nil || *h.Year != 2012 || h.LibraryID != f.LibraryID() || h.ParentTitle != "" {
		t.Errorf("film hit = %+v", h)
	}
	if h := byID[other.ID]; h.LibraryID != f.OtherLibraryID() {
		t.Errorf("library X hit = %+v", h)
	}
	if h := byID[f.Show.E1.ID]; h.Kind != "episode" || h.ParentTitle != "Film SS" {
		t.Errorf("episode hit = %+v, want the series title", h)
	}
	if h := byID[f.Audio.Episodes[0].ID]; h.Kind != "podcast_episode" || h.ParentTitle != "Film PC" {
		t.Errorf("podcast episode hit = %+v, want the podcast title", h)
	}
	if more, err := r.AdminSearch(ctx, System("test"), "Fi", 500); err != nil || len(more) != len(hits) || len(hits) > MaxAdminHits {
		t.Errorf("limit 500: %d hits, err %v, want the %d of limit 50", len(more), err, len(hits))
	}
	if few, err := r.AdminSearch(ctx, System("test"), "Fi", 3); err != nil || len(few) != 3 || few[0].ID != hits[0].ID {
		t.Errorf("limit 3: %v, err %v", few, err)
	}
	if hits, err := r.AdminSearch(ctx, System("test"), "F", 10); err != nil || len(hits) != 0 {
		t.Errorf("one character: %v, %v", hits, err)
	}
}

func edge(f *vistest.Fixture, pick func(vistest.Movie) bool) vistest.Movie {
	for _, m := range f.Movies {
		if pick(m) {
			return m
		}
	}
	panic("edge case missing")
}

// n1 (M7 review): the filter local counts a series or podcast as present only through files of
// episodes the profile may see. The podcast stays visible to P12 through episodes it can stream
// from their enclosure; its only present file belongs to the explicit episode (FSK 18).
func TestSearchLocalCountsVisibleEpisodesOnly(t *testing.T) {
	f := searchFixture(t)
	r := New(f.DB, vistest.Logger())
	for _, m := range f.Audio.Episodes {
		exec(t, f, `UPDATE media_files SET missing_since = 5 WHERE item_id = ?`, m.ID)
	}
	q := textQuery("Fi")
	q.Availability = []string{"local"}
	if got := ids(search(t, r, f, f.P18, q).Items); !slices.Contains(got, f.Audio.Podcast.ID) {
		t.Errorf("P18 local = %v, want the podcast through its explicit episode", got)
	}
	if got := ids(search(t, r, f, f.P12, q).Items); slices.Contains(got, f.Audio.Podcast.ID) {
		t.Errorf("P12 local = %v, want no podcast: its only present file belongs to an episode rated 18", got)
	}
	if got := ids(search(t, r, f, f.P12, textQuery("Fi")).Items); !slices.Contains(got, f.Audio.Podcast.ID) {
		t.Errorf("P12 without filter = %v, want the podcast", got)
	}
}

// n5 (M7 review): collectionsByID keeps the order of the ids it gets (the search rank).
func TestCollectionsByIDKeepOrder(t *testing.T) {
	f := searchFixture(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	for _, order := range [][]string{
		{vistest.MixedCollectionID, vistest.ManualCollectionID},
		{vistest.ManualCollectionID, vistest.MixedCollectionID},
	} {
		refs, err := r.collectionsByID(ctx, Profile(f.P18), f.P18.ID, order)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, 0, len(refs))
		for _, c := range refs {
			got = append(got, c.ID)
		}
		if !slices.Equal(got, order) {
			t.Errorf("collections = %v, want %v", got, order)
		}
	}
}
