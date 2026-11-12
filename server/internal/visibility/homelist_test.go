package visibility

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// homeListPage is the signature of RecentPage, ReleasedPage and ContinuePage.
type homeListPage func(ctx context.Context, s Scope, content string, offset, limit int) ([]model.ItemSummary, int, error)

// Concept 20 P-7 (8.3): the lists behind the row titles hold the titles of the row in its order,
// only without the cut to MaxRowItems; content keeps the titles whose top level has that kind (the
// next episode of a series counts as series); offset and limit page the list and total counts it
// after the filter, for every profile only what it may see.
func TestHomeListsAreTheRowsWithoutTheCut(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	f.AddAudio(t)
	f.AddSpotlightFilms(t, 25) // films rated 0 for everybody, added first: the lists outgrow the rows
	known := map[string]vistest.Movie{}
	for _, m := range f.All() {
		known[m.ID] = m
	}
	ctx := context.Background()
	now := time.Now()
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	// Started titles of P18: two episodes of the series (E3 newer), the film rated 12 and a podcast
	// episode; the row «Weiterschauen» holds the newer episode once for the series.
	day := (24 * time.Hour).Milliseconds()
	for i, id := range []string{f.Show.E1.ID, f.Show.E3.ID, f.Rated(12).ID, f.Audio.Episodes[0].ID} {
		exec(t, f, `INSERT OR REPLACE INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, ?, 0, ?)`,
			f.P18.ID, id, vistest.DurationMs, now.UnixMilli()-int64(4-i)*day)
	}
	// Release dates in the last twelve months for the films, so the list released has titles.
	for i, m := range []vistest.Movie{f.Rated(0), f.Rated(6), f.Rated(12), f.Rated(18)} {
		exec(t, f, `UPDATE items SET release_date = ? WHERE id = ?`, now.AddDate(0, -1-i, 0).Format(time.DateOnly), m.ID)
	}
	exec(t, f, `UPDATE items SET release_date = ? WHERE id = ?`, now.AddDate(0, -2, 0).Format(time.DateOnly), f.Show.E1.ID)
	rowOf := map[string]func(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error){
		"recent": r.Recent, "released": r.Released, "continue": r.ContinueWatching,
	}
	for name, list := range map[string]homeListPage{"recent": r.RecentPage, "released": r.ReleasedPage, "continue": r.ContinuePage} {
		for _, p := range []model.Profile{f.P0, f.P6, f.P12, f.P18} {
			all, total, err := list(ctx, Profile(p), "", 0, MaxLibraryPage)
			if err != nil {
				t.Fatal(err)
			}
			row, err := rowOf[name](ctx, Profile(p), MaxRowItems)
			if err != nil {
				t.Fatal(err)
			}
			if total != len(all) || !slices.Equal(ids(all[:min(len(all), MaxRowItems)]), ids(row)) {
				t.Errorf("%s for %s = %v (total %d), want the row %v and more", name, p.Name, ids(all), total, ids(row))
			}
			if name == "recent" && len(all) <= MaxRowItems {
				t.Errorf("%s for %s holds %d titles, the fixture should have more than the row's %d", name, p.Name, len(all), MaxRowItems)
			}
			for _, it := range all {
				if m, ok := known[it.ID]; ok && !f.VisibleTo(p, m) {
					t.Errorf("%s for %s holds the invisible %s", name, p.Name, it.ID)
				}
			}
			if len(all) > 1 {
				page, n, err := list(ctx, Profile(p), "", 1, 1)
				if err != nil || n != total || !slices.Equal(ids(page), ids(all[1:2])) {
					t.Errorf("%s for %s page (1, 1) = %v (%d, %v), want %v of %d", name, p.Name, ids(page), n, err, ids(all[1:2]), total)
				}
			}
			if beyond, n, err := list(ctx, Profile(p), "", total+5, 10); err != nil || len(beyond) != 0 || n != total {
				t.Errorf("%s for %s beyond the end = %v (%d, %v)", name, p.Name, ids(beyond), n, err)
			}
			for _, content := range []string{"movie", "series", "audiobook", "podcast"} {
				var want []string
				for _, it := range all {
					if topKind(it.Kind) == content {
						want = append(want, it.ID)
					}
				}
				got, n, err := list(ctx, Profile(p), content, 0, MaxLibraryPage)
				if err != nil || !slices.Equal(ids(got), want) || n != len(want) {
					t.Errorf("%s for %s with %s = %v (%d, %v), want %v", name, p.Name, content, ids(got), n, err, want)
				}
			}
		}
	}
	// «Weiterschauen» of P18 with «Serien»: the next episode of the series only.
	got, n, err := r.ContinuePage(ctx, Profile(f.P18), "series", 0, 10)
	if err != nil || n != 1 || got[0].ID != f.Show.E3.ID {
		t.Errorf("continue of P18 with series = %v (%d, %v), want E3", ids(got), n, err)
	}
	if got, _, err = r.ContinuePage(ctx, Profile(f.P18), "podcast", 0, 10); err != nil || !slices.Equal(ids(got), []string{f.Audio.Episodes[0].ID}) {
		t.Errorf("continue of P18 with podcast = %v (%v), want the podcast episode", ids(got), err)
	}
}

// Concept 20 P-7 (8.3): the spotlight list is the whole list after the pinned entries took their
// place, without the cut to item_count; content filters after the placing, and the hits (the
// condition of the row with min_hits) count before the filter. Disabled, there is no row.
func TestSpotlightPageWithoutTheCut(t *testing.T) {
	t.Parallel()
	f := discoverFixture(t)
	ctx := context.Background()
	fetched := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return fetched.Add(time.Hour) }))
	all := spotlightFilms(t, f, 25, fetched)
	exec(t, f, `UPDATE spotlight_config SET item_count = 5`)
	override(t, f, 25, false, 2, "angeheftet")
	override(t, f, 3, true, 0, "")
	want := append(films(all, 1, 25), films(all, 2)...)
	for k := 4; k <= 24; k++ {
		want = append(want, all[k-1])
	}
	got, total, shown, err := r.SpotlightPage(ctx, Profile(f.P0), "", 0, MaxLibraryPage)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(got), want) || total != 24 || !shown {
		t.Errorf("spotlight page = %v (total %d, shown %v), want %v of 24", ids(got), total, shown, want)
	}
	if got[1].Note != "angeheftet" {
		t.Errorf("pinned note = %q", got[1].Note)
	}
	if page, total, _, err := r.SpotlightPage(ctx, Profile(f.P0), "", 20, 10); err != nil || !slices.Equal(ids(page), want[20:]) || total != 24 {
		t.Errorf("spotlight page (20, 10) = %v (%d, %v), want %v", ids(page), total, err, want[20:])
	}
	if page, total, shown, err := r.SpotlightPage(ctx, Profile(f.P0), "series", 0, 10); err != nil || len(page) != 0 || total != 0 || !shown {
		t.Errorf("spotlight page with series = %v (total %d, shown %v, %v), want none, the row shown", ids(page), total, shown, err)
	}
	exec(t, f, `UPDATE spotlight_config SET enabled = 0`)
	if page, total, shown, err := r.SpotlightPage(ctx, Profile(f.P0), "", 0, 10); err != nil || len(page) != 0 || total != 0 || shown {
		t.Errorf("disabled spotlight page = %v (total %d, shown %v, %v), want nothing", ids(page), total, shown, err)
	}
	// Below min_hits (nine hits of ten) the row and the list are gone.
	exec(t, f, `UPDATE spotlight_config SET enabled = 1, min_hits = 10`)
	exec(t, f, `DELETE FROM spotlight_entries WHERE position > 9`)
	if page, _, shown, err := r.SpotlightPage(ctx, Profile(f.P0), "", 0, 10); err != nil || len(page) != 0 || shown {
		t.Errorf("spotlight page below min_hits = %v (shown %v, %v), want nothing", ids(page), shown, err)
	}
}

// Concept 20 P-7 (8.3): the filter content must not defeat the partial index items_top_added; the
// planner still walks it in its order, with and without a filter.
func TestHomeListRecentUsesTheIndex(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	src, err := os.ReadFile("../store/scopedq/items.sql.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"listRecentlyAddedPage", "countRecentlyAdded"} {
		_, query, found := strings.Cut(string(src), "const "+name+" = `")
		query, _, closed := strings.Cut(query, "`")
		if !found || !closed {
			t.Fatalf("%s not found", name)
		}
		for _, content := range []string{"", "series"} {
			args := []any{f.P18.ID, content}
			if name == "listRecentlyAddedPage" {
				args = append(args, 0, 50)
			}
			text := strings.Join(queryPlan(t, f.DB.Read, query, args), "\n")
			if !strings.Contains(text, "items_top_added") || strings.Contains(text, "TEMP B-TREE") {
				t.Errorf("%s with content %q does not walk items_top_added in order:\n%s", name, content, text)
			}
		}
	}
}

// queryPlan returns the detail lines of EXPLAIN QUERY PLAN for query with args.
func queryPlan(t *testing.T, db *sql.DB, query string, args []any) []string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return plan
}
