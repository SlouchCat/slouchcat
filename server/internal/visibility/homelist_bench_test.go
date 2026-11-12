package visibility

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// BenchmarkHomeList measures the lists behind the row titles (listHomeRow, concept 20 P-7, M-3:
// goal under 150 ms) on a library of 5000 films and 250 series with one season of four episodes
// each, one playable file per title (docs/perf/20-bedienung.md, section «M-3 listHomeRow»):
//   - continue: P18 has started episode 2 of every series, so the query yields 250 entries and the
//     list is cut at MaxContinueList (200); first page of 50 and the page at offset 150;
//   - recent with the filter series: the 250 series among 5000 newer films, so the walk of
//     items_top_added skips the films; first page of 50 and the last page.
//
// It reports p50 and p95 of the Reader calls (queries and summaries). Run:
//
//	go test -run '^$' -bench HomeList -benchtime 200x -v ./internal/visibility/
func BenchmarkHomeList(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	now := time.Now()
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	start := time.Now()
	const films, series, perSeries = 5000, 250, 4
	lib, root := f.LibraryID(), f.RootID()
	for _, query := range []string{
		// Series added long ago, the films after them: the filter series must walk past every film.
		fmt.Sprintf(`WITH RECURSIVE n(s) AS (SELECT 1 UNION ALL SELECT s + 1 FROM n WHERE s < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9HLISTS%%016d', s), 'series', NULL, %d, 'Serie ' || s, 'serie ' || s, '[]', 12, 'matched', 1000 + s, 1, 1 FROM n`,
			series, lib),
		fmt.Sprintf(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, season_number, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9HLISTN%%016d', CAST(substr(id, 11) AS INTEGER)), 'season', id, %d, 'Staffel 1', 'staffel 1', '[]', 1, 'matched', 1, 1, 1
		 FROM items WHERE id LIKE '01J9HLISTS%%'`, lib),
		fmt.Sprintf(`WITH RECURSIVE n(e) AS (SELECT 1 UNION ALL SELECT e + 1 FROM n WHERE e < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, season_number, episode_number,
		   metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9HLISTE%%010d%%06d', CAST(substr(s.id, 11) AS INTEGER), n.e), 'episode', s.id, %d, 'Folge ' || n.e, 'folge ' || n.e,
		   '[]', 12, 1, n.e, 'matched', 1, 1, 1
		 FROM items AS s, n WHERE s.id LIKE '01J9HLISTN%%'`, perSeries, lib),
		fmt.Sprintf(`WITH RECURSIVE n(k) AS (SELECT 1 UNION ALL SELECT k + 1 FROM n WHERE k < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9HLISTM%%016d', k), 'movie', NULL, %d, 'Film ' || k, 'film ' || k, '[]', 12, 'matched', 100000 + k, 1, 1 FROM n`,
			films, lib),
		fmt.Sprintf(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec,
		   width, height, duration_ms, created_at, updated_at)
		 SELECT '01J9HLISTF' || substr(id, 11), id, %d, 'bench/' || id || '.mp4', 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, %d, 1, 1
		 FROM items WHERE id LIKE '01J9HLISTE%%' OR id LIKE '01J9HLISTM%%'`, root, vistest.DurationMs),
		// Episode 2 of every series started, one minute apart.
		fmt.Sprintf(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at)
		 SELECT %d, id, %d, %d, 0, %d - CAST(substr(id, 11, 10) AS INTEGER) * 60000
		 FROM items WHERE id LIKE '01J9HLISTE%%' AND episode_number = 2`, f.P18.ID, vistest.DurationMs/10, vistest.DurationMs, now.UnixMilli()),
	} {
		benchExec(b, f, query)
	}
	b.Logf("fixture: %d films, %d series × %d episodes, %d started episodes in %v",
		films, series, perSeries, series, time.Since(start).Round(time.Millisecond))

	for _, tc := range []struct {
		name    string
		list    homeListPage
		content string
		offset  int
		total   int
	}{
		{"continue/first", r.ContinuePage, "", 0, MaxContinueList},
		{"continue/offset150", r.ContinuePage, "", 150, MaxContinueList},
		{"recent-series/first", r.RecentPage, "series", 0, series},
		{"recent-series/last", r.RecentPage, "series", series - 50, series},
		{"recent-all/first", r.RecentPage, "", 0, -1},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var lat []time.Duration
			var n, total int
			for b.Loop() {
				t0 := time.Now()
				items, all, err := tc.list(ctx, Profile(f.P18), tc.content, tc.offset, DefaultLibraryPage)
				if err != nil {
					b.Fatal(err)
				}
				lat = append(lat, time.Since(t0))
				n, total = len(items), all
			}
			if n != DefaultLibraryPage || (tc.total >= 0 && total != tc.total) {
				b.Fatalf("page of %d titles, total %d; want %d of %d", n, total, DefaultLibraryPage, tc.total)
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(lat[(len(lat)*95+99)/100-1].Microseconds())/1000, "p95-ms")
		})
	}
}
