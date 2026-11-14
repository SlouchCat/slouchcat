package visibility

import (
	"context"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 21 P-10: a collection and its entry in the lists carry the years and the summed runtime
// of the visible members and the progress of the profile; the next part is the first visible
// member that is not finished, the first one when every member is. Progress on a member the
// profile may not see and progress of another profile never count.
func TestCollectionFactsAndNext(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	m6, m12, m18 := f.Rated(6).ID, f.Rated(12).ID, f.Rated(18).ID
	year := func(id string) int {
		t.Helper()
		var y int
		if err := f.DB.Read.QueryRowContext(ctx, `SELECT year FROM items WHERE id = ?`, id).Scan(&y); err != nil {
			t.Fatal(err)
		}
		return y
	}
	progress := func(p model.Profile, id string, position int64, completed, watched bool) {
		t.Helper()
		var at any
		if watched {
			at = int64(5000)
		}
		done := 0
		if completed {
			done = 1
		}
		exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
			VALUES (?, ?, ?, 10000, ?, 5000, ?)
			ON CONFLICT (profile_id, item_id) DO UPDATE SET position_ms = excluded.position_ms,
			    completed = excluded.completed, watched_at = excluded.watched_at`, p.ID, id, position, done, at)
	}
	check := func(step string, p model.Profile, seen int, started bool, next string, part int) {
		t.Helper()
		c, err := r.Collection(ctx, Profile(p), vistest.MixedCollectionID, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		want := model.CollectionFacts{FirstYear: year(m6), LastYear: year(m12), RuntimeMinutes: 240, SeenCount: seen, Started: started}
		if p.ID == f.P18.ID {
			want.LastYear, want.RuntimeMinutes = year(m18), 360
		}
		if c.CollectionFacts != want {
			t.Errorf("%s: %s facts = %+v, want %+v", step, p.Name, c.CollectionFacts, want)
		}
		if c.Next == nil || c.Next.ID != next || c.NextPart != part {
			t.Errorf("%s: %s next = %v part %d, want %s part %d", step, p.Name, c.Next, c.NextPart, next, part)
		}
		refs, err := r.AllCollections(ctx, Profile(p))
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range refs {
			if ref.ID == vistest.MixedCollectionID && ref.CollectionFacts != want {
				t.Errorf("%s: %s list entry facts = %+v, want %+v", step, p.Name, ref.CollectionFacts, want)
			}
		}
	}
	check("nothing watched", f.P12, 0, false, m6, 1)
	// P12 finishes part 1 and starts part 2; the film rated 18 it may not see and the progress of
	// P18 do not count for it.
	progress(f.P12, m6, 10000, true, true)
	progress(f.P12, m12, 3000, false, false)
	progress(f.P12, m18, 10000, true, true)
	progress(f.P18, m12, 10000, true, true)
	check("part 1 finished, part 2 started", f.P12, 1, true, m12, 2)
	// A second run of part 1 (watched once, running again) still counts as seen; part 1 is open
	// again and comes next.
	progress(f.P12, m6, 2000, false, true)
	check("part 1 again", f.P12, 1, true, m6, 1)
	// Every visible part finished: the first one comes next (play again).
	progress(f.P12, m6, 10000, true, true)
	progress(f.P12, m12, 10000, true, true)
	check("all finished", f.P12, 2, true, m6, 1)
	// P18 sees the film rated 18 open behind two finished parts of its own.
	progress(f.P18, m6, 10000, true, true)
	check("P18 two of three", f.P18, 2, true, m18, 3)
}

// A series in a collection counts as seen when every episode the profile may see is watched
// (concept 20 F-12): P12 sees E1, E3 and the special, P18 also E2 (rated 16). It counts 0 minutes
// and starts the collection with one episode watched.
func TestCollectionFactsOfASeriesMember(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	const id = "01J9000000000000000000CMSR"
	exec(t, f, `INSERT INTO collections (id, source, name, created_at, updated_at) VALUES (?, 'manual', 'Mixed kinds', 1, 1)`, id)
	exec(t, f, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES (?, ?, 0, 1), (?, ?, 1, 1)`,
		id, f.Show.Series.ID, id, f.Rated(6).ID)
	facts := func(p model.Profile) model.Collection {
		t.Helper()
		c, err := r.Collection(ctx, Profile(p), id, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := facts(f.P12); c.SeenCount != 0 || c.Started || c.RuntimeMinutes != 120 || c.Next == nil || c.Next.ID != f.Show.Series.ID {
		t.Errorf("P12 before: %+v next %v, want nothing seen, 120 minutes, the series next", c.CollectionFacts, c.Next)
	}
	exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
		VALUES (?, ?, 10000, 10000, 1, 5000, 5000)`, f.P12.ID, f.Show.E1.ID)
	if c := facts(f.P12); c.SeenCount != 0 || !c.Started {
		t.Errorf("P12 with E1 watched: %+v, want started and nothing seen", c.CollectionFacts)
	}
	for _, e := range []string{f.Show.E3.ID, f.Show.Special.ID} {
		exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
			VALUES (?, ?, 10000, 10000, 1, 5000, 5000), (?, ?, 10000, 10000, 1, 5000, 5000)`, f.P12.ID, e, f.P18.ID, e)
	}
	exec(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
		VALUES (?, ?, 10000, 10000, 1, 5000, 5000)`, f.P18.ID, f.Show.E1.ID)
	if c := facts(f.P12); c.SeenCount != 1 || c.Next == nil || c.Next.ID != f.Rated(6).ID || c.NextPart != 2 {
		t.Errorf("P12 with every visible episode watched: %+v next %v part %d, want the series seen, the film next", c.CollectionFacts, c.Next, c.NextPart)
	}
	if c := facts(f.P18); c.SeenCount != 0 || !c.Started || c.Next == nil || c.Next.ID != f.Show.Series.ID {
		t.Errorf("P18 with E2 open: %+v next %v, want the series open and next", c.CollectionFacts, c.Next)
	}
}

// Concept 21 O-18: the fetched description comes before the older text, with its language; the
// English one in place of the main language is a fallback.
func TestCollectionDescription(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	r := New(f.DB, vistest.Logger())
	c, err := r.Collection(ctx, Profile(f.P12), vistest.MixedCollectionID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	older := c.Overview
	if c.OverviewLang != "" || c.OverviewFallback {
		t.Errorf("before a fetch: lang %q fallback %v, want none", c.OverviewLang, c.OverviewFallback)
	}
	for _, tc := range []struct {
		desc, lang, asked string
		fallback          bool
	}{
		{"Die Reihe.", "de", "de", false},
		{"The series.", "en", "de", true},
		{"The series.", "en", "en", false},
	} {
		exec(t, f, `UPDATE collections SET description = ?, description_lang = ?, details_lang = ?, details_fetched_at = 1 WHERE id = ?`,
			tc.desc, tc.lang, tc.asked, vistest.MixedCollectionID)
		c, err := r.Collection(ctx, Profile(f.P12), vistest.MixedCollectionID, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if c.Overview != tc.desc || c.OverviewLang != tc.lang || c.OverviewFallback != tc.fallback {
			t.Errorf("%s asked in %s: %q %q %v", tc.lang, tc.asked, c.Overview, c.OverviewLang, c.OverviewFallback)
		}
	}
	// A fetch without a text (TMDB has none) leaves the older one.
	exec(t, f, `UPDATE collections SET description = NULL, description_lang = NULL WHERE id = ?`, vistest.MixedCollectionID)
	if c, err := r.Collection(ctx, Profile(f.P12), vistest.MixedCollectionID, 0, 0); err != nil || c.Overview != older || c.OverviewLang != "" {
		t.Errorf("without a description: %q %q (%v), want the older text %q", c.Overview, c.OverviewLang, err, older)
	}
}
