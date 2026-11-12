package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract tests of the detail page of concept 17 stage 2 (contract 1.20.0) through getItem:
// nextUp of series and seasons, watchedCount and the year of seasons, the episode overviews of a
// season, CollectionRef.members and the watchlist mark on every ItemRef.

// rawDetail is getItem decoded loosely, so a test can tell an absent property from a zero one.
type rawDetail map[string]json.RawMessage

func getRawDetail(t *testing.T, h http.Handler, token, id string) rawDetail {
	t.Helper()
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+id, token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s getItem %s: status %d (%s)", token, id, rec.Code, rec.Body.String())
	}
	var d rawDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// list decodes the array property key of d into raw objects (none when absent).
func (d rawDetail) list(t *testing.T, key string) []rawDetail {
	t.Helper()
	var out []rawDetail
	if raw, ok := d[key]; ok {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	return out
}

// object decodes the object property key of d (nil when absent).
func (d rawDetail) object(t *testing.T, key string) rawDetail {
	t.Helper()
	raw, ok := d[key]
	if !ok {
		return nil
	}
	var out rawDetail
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	return out
}

func (d rawDetail) str(key string) string {
	var s string
	_ = json.Unmarshal(d[key], &s)
	return s
}

func (d rawDetail) num(key string) (int, bool) {
	raw, ok := d[key]
	if !ok {
		return 0, false
	}
	var n int
	return n, json.Unmarshal(raw, &n) == nil
}

func (d rawDetail) flag(key string) (bool, bool) {
	raw, ok := d[key]
	if !ok {
		return false, false
	}
	var b bool
	return b, json.Unmarshal(raw, &b) == nil
}

func rawIDs(refs []rawDetail) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.str("id"))
	}
	return out
}

func TestContractDetailNextUp(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	show := f.Show
	progress := func(profileID int64, itemID string, completed bool, at int64) {
		t.Helper()
		pos, done := int64(1000), 0
		if completed {
			pos, done = 5000, 1
		}
		if _, err := f.DB.Write.Exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, ?, 5000, ?, ?)
		  ON CONFLICT (profile_id, item_id) DO UPDATE SET position_ms = excluded.position_ms, completed = excluded.completed, updated_at = excluded.updated_at`,
			profileID, itemID, pos, done, at); err != nil {
			t.Fatal(err)
		}
	}
	nextUp := func(token, id string) (string, rawDetail) {
		t.Helper()
		n := getRawDetail(t, h, token, id).object(t, "nextUp")
		if n == nil {
			return "", nil
		}
		return n.str("reason"), n.object(t, "episode")
	}

	if reason, ep := nextUp("p18", show.Series.ID); reason != "start" || ep.str("id") != show.E1.ID {
		t.Errorf("P18 series without progress: nextUp %q %q, want start E1", reason, ep.str("id"))
	}
	// P12 completed E1 and started E2, which it may not see: next is E3 on the series and season 1.
	progress(f.P12.ID, show.E1.ID, true, 1)
	progress(f.P12.ID, show.E2.ID, false, 5)
	for _, id := range []string{show.Series.ID, show.Season.ID} {
		if reason, ep := nextUp("p12", id); reason != "next" || ep.str("id") != show.E3.ID {
			t.Errorf("P12 %s with a hidden started episode: nextUp %q %q, want next E3", id, reason, ep.str("id"))
		}
	}
	// P16 started E2 after E1: resume E2; the episode is a full ItemRef without the overview.
	progress(f.P16.ID, show.E1.ID, true, 1)
	progress(f.P16.ID, show.E2.ID, false, 2)
	reason, ep := nextUp("p16", show.Series.ID)
	if reason != "resume" || ep.str("id") != show.E2.ID {
		t.Errorf("P16 series: nextUp %q %q, want resume E2", reason, ep.str("id"))
	}
	if _, ok := ep.flag("inWatchlist"); !ok || ep["progress"] == nil || ep["overview"] != nil {
		t.Errorf("P16 nextUp episode = %v", ep)
	}
	// The specials page counts its own episodes only; films and episodes have no nextUp.
	if reason, ep := nextUp("p16", show.Specials.ID); reason != "start" || ep.str("id") != show.Special.ID {
		t.Errorf("P16 specials: nextUp %q %q, want start P1", reason, ep.str("id"))
	}
	for _, id := range []string{show.E1.ID, f.Rated(12).ID} {
		if reason, _ := nextUp("p16", id); reason != "" {
			t.Errorf("P16 %s carries nextUp %q", id, reason)
		}
	}
	// A series the profile may not see stays 404.
	if got := getTitle(t, h, "p6", show.Series.ID); got != http.StatusNotFound {
		t.Errorf("P6 getItem of the series rated 12: %d, want 404", got)
	}
}

func TestContractDetailSeasonShelf(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	show := f.Show
	for _, p := range []int64{f.P12.ID, f.P16.ID} {
		for _, id := range []string{show.E1.ID, show.E2.ID} {
			if _, err := f.DB.Write.Exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at) VALUES (?, ?, 5000, 5000, 1, 1, 1)`, p, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Seasons of the series: watchedCount of the visible watched episodes (P12 does not see E2)
	// and the year (from the release date when the season has none: TestSeasonYearAndWatchedCount
	// in visibility, since rule 2 keeps title SQL out of this package); since contract 1.31.0 the
	// series carries the same count over all its seasons (concept 20 F-12).
	for _, tc := range []struct {
		token   string
		watched int
	}{{"p12", 1}, {"p16", 2}, {"p18", 0}} {
		d := getRawDetail(t, h, tc.token, show.Series.ID)
		if n, ok := d.num("watchedCount"); !ok || n != tc.watched {
			t.Errorf("%s series watchedCount = %d (set %v), want %d", tc.token, n, ok, tc.watched)
		}
		seasons := d.list(t, "seasons")
		if len(seasons) != 2 || seasons[0].str("id") != show.Season.ID {
			t.Fatalf("%s seasons = %v", tc.token, rawIDs(seasons))
		}
		if n, ok := seasons[0].num("watchedCount"); !ok || n != tc.watched {
			t.Errorf("%s season 1 watchedCount = %d (set %v), want %d", tc.token, n, ok, tc.watched)
		}
		if _, ok := seasons[0].num("year"); !ok {
			t.Errorf("%s season 1 without a year", tc.token)
		}
		for _, s := range seasons {
			if _, ok := s.flag("inWatchlist"); !ok || s["overview"] != nil {
				t.Errorf("%s season ref %s: inWatchlist set %v, overview %s", tc.token, s.str("id"), ok, s["overview"])
			}
		}
	}
	// Season detail: its own watchedCount and overview, the overviews of its episodes.
	d := getRawDetail(t, h, "p16", show.Season.ID)
	if n, ok := d.num("watchedCount"); !ok || n != 2 || d.str("overview") != "Handlung SN" {
		t.Errorf("P16 season detail: watchedCount %d (set %v), overview %q", n, ok, d.str("overview"))
	}
	episodes := d.list(t, "episodes")
	if want := []string{show.E1.ID, show.E2.ID, show.E3.ID}; !slices.Equal(rawIDs(episodes), want) {
		t.Fatalf("P16 episodes = %v, want %v", rawIDs(episodes), want)
	}
	for i, e := range episodes {
		if want := "Handlung " + []string{"E1", "E2", "E3"}[i]; e.str("overview") != want {
			t.Errorf("P16 episode %d overview = %q, want %q", i, e.str("overview"), want)
		}
		if _, ok := e.num("watchedCount"); ok {
			t.Errorf("P16 episode %d carries watchedCount", i)
		}
	}
	// No other list carries the overview: the extras of the series, the series of the season.
	for _, ref := range append(getRawDetail(t, h, "p16", show.Series.ID).list(t, "extras"), d.object(t, "series")) {
		if ref["overview"] != nil {
			t.Errorf("%s carries an overview outside the season's episodes", ref.str("id"))
		}
	}
}

func TestContractDetailCollectionMembers(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	for _, tc := range []struct {
		token  string
		want   []string
		marked string // the member on the profile's watchlist (vistest.AddDiscover)
	}{
		{"p12", []string{f.Rated(6).ID, f.Rated(12).ID}, ""},
		{"p18", []string{f.Rated(6).ID, f.Rated(12).ID, f.Rated(18).ID}, f.Rated(18).ID},
	} {
		d := getRawDetail(t, h, tc.token, f.Rated(6).ID)
		var mixed rawDetail
		for _, c := range d.list(t, "collections") {
			if c.str("id") == vistest.MixedCollectionID {
				mixed = c
			}
		}
		if mixed == nil {
			t.Fatalf("%s: no mixed collection on the detail", tc.token)
		}
		members := mixed.list(t, "members")
		if !slices.Equal(rawIDs(members), tc.want) {
			t.Errorf("%s members = %v, want %v", tc.token, rawIDs(members), tc.want)
		}
		for _, m := range members {
			if on, ok := m.flag("inWatchlist"); !ok || on != (m.str("id") == tc.marked) {
				t.Errorf("%s member %s inWatchlist = %v (set %v)", tc.token, m.str("id"), on, ok)
			}
		}
	}
}

// subjectImageMatches checks the rule of concept 17 stage 3 (m-4, ADR 0026) for one object of an
// answer: its image is set exactly when the image endpoint answers 200 for the profile, and then
// its url answers 200 as well.
func subjectImageMatches(t *testing.T, h http.Handler, token, what, endpoint string, obj rawDetail) {
	t.Helper()
	img := obj.object(t, "image")
	code := send(t, h, http.MethodGet, BasePath+endpoint, token, "", nil).Code
	if (img != nil) != (code == http.StatusOK) {
		t.Errorf("%s %s: image %v, endpoint %s status %d", token, what, img, endpoint, code)
		return
	}
	if img == nil {
		return
	}
	if w, _ := img.num("width"); w < 1 {
		t.Errorf("%s %s: image %v without width", token, what, img)
	}
	if got := send(t, h, http.MethodGet, img.str("url"), token, "", nil).Code; got != http.StatusOK {
		t.Errorf("%s %s: image url %s status %d", token, what, img.str("url"), got)
	}
}

// The portrait on the cast of a title and on the person page, and the TMDB poster on the
// collections of a title and on the collection page, follow the endpoints for every fixture
// profile (P18 sees the poster, P12 not); without rows the fields are absent.
func TestContractDetailImages(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	tokens := []string{"p0", "p6", "p12", "p16", "p18", "px", "pg", "pg18"}
	personPath := "/people/1/image"
	collPath := "/collections/" + vistest.MixedCollectionID + "/image"
	images := map[string]int{} // place → answers with an image
	check := func(stage string) {
		for _, token := range tokens {
			p := fixtureProfile(f, token)
			if f.VisibleTo(p, f.Rated(6)) {
				d := getRawDetail(t, h, token, f.Rated(6).ID)
				for _, c := range d.list(t, "cast") {
					if n, _ := c.num("personId"); n == int(f.Person.ID) {
						subjectImageMatches(t, h, token, stage+" cast", personPath, c)
						images[stage+" cast"] += len(c.object(t, "image"))
					}
				}
				for _, c := range d.list(t, "collections") {
					if c.str("id") == vistest.MixedCollectionID {
						subjectImageMatches(t, h, token, stage+" detail collection", collPath, c)
						images[stage+" detail collection"] += len(c.object(t, "image"))
					}
				}
			}
			if rec := send(t, h, http.MethodGet, BasePath+"/people/1", token, "", nil); rec.Code == http.StatusOK {
				var person rawDetail
				decode(t, rec, &person)
				subjectImageMatches(t, h, token, stage+" person", personPath, person)
				images[stage+" person"] += len(person.object(t, "image"))
			}
			for _, id := range []string{vistest.MixedCollectionID, vistest.ManualCollectionID} {
				rec := send(t, h, http.MethodGet, BasePath+"/collections/"+id, token, "", nil)
				if rec.Code != http.StatusOK {
					continue
				}
				var c rawDetail
				decode(t, rec, &c)
				subjectImageMatches(t, h, token, stage+" collection "+id, "/collections/"+id+"/image", c)
				images[stage+" collection "+id] += len(c.object(t, "image"))
			}
		}
	}
	check("with")
	for _, place := range []string{"cast", "detail collection", "person", "collection " + vistest.MixedCollectionID} {
		if images["with "+place] == 0 {
			t.Errorf("no profile got an image on the %s", place)
		}
	}
	if images["with collection "+vistest.ManualCollectionID] != 0 {
		t.Errorf("the manual collection carries an image")
	}
	if _, err := f.DB.Write.Exec(`DELETE FROM person_images; DELETE FROM collection_images`); err != nil {
		t.Fatal(err)
	}
	check("without")
	for place, n := range images {
		if strings.HasPrefix(place, "without ") && n != 0 {
			t.Errorf("%s: %d answers with an image, want none", place, n)
		}
	}
}

// Contract 1.22.0: ItemRef.watched on series and podcasts, in the detail and in the library list,
// by the rule of the library status watched (TestSeriesWatched and TestPodcastWatched in
// visibility hold the cases); every other kind leaves it out.
func TestContractSeriesAndPodcastWatched(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	show, a := f.Show, f.Audio
	// P12 completes every episode it may see (not E2, rated 16) and every podcast episode rated
	// up to 12.
	for _, id := range []string{show.E1.ID, show.E3.ID, show.Special.ID, a.Episodes[0].ID, a.Episodes[1].ID, a.Episodes[2].ID, a.EnclosureOnly.ID} {
		if _, err := f.DB.Write.Exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at) VALUES (?, ?, 5000, 5000, 1, 1, 1)`, f.P12.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	listed := func(token string, libraryID int64, id string) rawDetail {
		t.Helper()
		rec := send(t, h, http.MethodGet, fmt.Sprintf("%s/libraries/%d/items?limit=100", BasePath, libraryID), token, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s listLibraryItems %d: %d %s", token, libraryID, rec.Code, rec.Body.String())
		}
		var page rawDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		for _, it := range page.list(t, "items") {
			if it.str("id") == id {
				return it
			}
			if _, ok := it["watched"]; ok && it.str("kind") != "series" && it.str("kind") != "podcast" {
				t.Errorf("%s: %s %s carries watched", token, it.str("kind"), it.str("id"))
			}
		}
		t.Fatalf("%s: %s missing from library %d", token, id, libraryID)
		return nil
	}
	for _, tc := range []struct {
		token string
		want  bool
	}{{"p12", true}, {"p18", false}} {
		for _, title := range []struct {
			id  string
			lib int64
		}{{show.Series.ID, fixtureLibrary}, {a.Podcast.ID, a.PodcastLibraryID}} {
			if got, ok := getRawDetail(t, h, tc.token, title.id).flag("watched"); !ok || got != tc.want {
				t.Errorf("%s getItem %s: watched %v (set %v), want %v", tc.token, title.id, got, ok, tc.want)
			}
			if got, ok := listed(tc.token, title.lib, title.id).flag("watched"); !ok || got != tc.want {
				t.Errorf("%s list %s: watched %v (set %v), want %v", tc.token, title.id, got, ok, tc.want)
			}
		}
	}
	for _, id := range []string{show.Season.ID, show.E1.ID, f.Rated(12).ID} {
		if _, ok := getRawDetail(t, h, "p12", id)["watched"]; ok {
			t.Errorf("getItem %s carries watched", id)
		}
	}
}
