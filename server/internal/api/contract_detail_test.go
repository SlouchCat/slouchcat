package api

import (
	"encoding/json"
	"net/http"
	"slices"
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
			if _, err := f.DB.Write.Exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 5000, 5000, 1, 1)`, p, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Seasons of the series: watchedCount of the visible completed episodes (P12 does not see E2)
	// and the year (from the release date when the season has none: TestSeasonYearAndWatchedCount
	// in visibility, since rule 2 keeps title SQL out of this package); the series itself has no
	// watchedCount.
	for _, tc := range []struct {
		token   string
		watched int
	}{{"p12", 1}, {"p16", 2}, {"p18", 0}} {
		d := getRawDetail(t, h, tc.token, show.Series.ID)
		if _, ok := d.num("watchedCount"); ok {
			t.Errorf("%s series carries watchedCount", tc.token)
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
