package visibility

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// BenchmarkSeriesNextUp measures ListSeriesNextUpEpisodes (next up of the detail page, concept 17
// section 3.3) on a series of 13 seasons with 20 episodes each (260 episodes, one playable file
// each), next to 99 other series of the same shape (26 000 episodes in the library), so the early
// filter on the one series is what is measured. The profile P18 has completed the first 130
// episodes of the series one hour apart. It reports p50 and p95 of the query alone (series and one
// season) and of Reader.Item on the series (detail with next up), and logs EXPLAIN QUERY PLAN.
// The same fixture measures the series in lists (contract 1.22.0, ItemRef.watched): the child
// counts of all 100 series in one ListVisibleChildCounts and the library page sorted by title
// (100 series with their summaries), for P18 and for P16, who has completed all 26 000 episodes
// (the worst case: every series is read to its last episode).
// Run:
//
//	go test -run '^$' -bench SeriesNextUp -benchtime 200x -v ./internal/visibility/
func BenchmarkSeriesNextUp(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	r := New(f.DB, vistest.Logger())
	start := time.Now()
	const series, seasons, perSeason = 100, 13, 20
	for _, query := range []string{
		fmt.Sprintf(`WITH RECURSIVE n(s) AS (SELECT 1 UNION ALL SELECT s + 1 FROM n WHERE s < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9NEXTUPS%%015d', s), 'series', NULL, %d, 'Serie ' || s, 'serie ' || s, '[]', 12, 'matched', 1, 1, 1 FROM n`,
			series, f.LibraryID()),
		fmt.Sprintf(`WITH RECURSIVE n(k) AS (SELECT 1 UNION ALL SELECT k + 1 FROM n WHERE k < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, season_number, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9NEXTUPN%%010d%%05d', CAST(substr(s.id, 12) AS INTEGER), n.k), 'season', s.id, %d, 'Staffel ' || n.k, 'staffel ' || n.k,
		   '[]', n.k, 'matched', 1, 1, 1
		 FROM items AS s, n WHERE s.id LIKE '01J9NEXTUPS%%'`, seasons, f.LibraryID()),
		fmt.Sprintf(`WITH RECURSIVE n(e) AS (SELECT 1 UNION ALL SELECT e + 1 FROM n WHERE e < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, episode_number,
		   metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9NEXTUPE%%05d%%05d%%05d', CAST(substr(s.id, 12, 10) AS INTEGER), s.season_number, n.e), 'episode', s.id, %d, 'Folge ' || n.e, 'folge ' || n.e,
		   '[]', 12, n.e, 'matched', 1, 1, 1
		 FROM items AS s, n WHERE s.id LIKE '01J9NEXTUPN%%'`, perSeason, f.LibraryID()),
		fmt.Sprintf(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec,
		   width, height, duration_ms, created_at, updated_at)
		 SELECT 'F' || substr(e.id, 2), e.id, %d, 'bench/' || e.id || '.mp4', 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, %d, 1, 1
		 FROM items AS e JOIN items AS s ON s.id = e.parent_id WHERE s.id LIKE '01J9NEXTUPN%%'`, f.RootID(), vistest.DurationMs),
	} {
		benchExec(b, f, query)
	}
	seriesID := fmt.Sprintf("01J9NEXTUPS%015d", 1)
	seasonID := fmt.Sprintf("01J9NEXTUPN%010d%05d", 1, 7)
	wantNext := fmt.Sprintf("01J9NEXTUPE%05d%05d%05d", 1, 7, 11) // after S07E10, the newest completed
	hour := time.Hour.Milliseconds()
	// The first 130 episodes of series 1 (seasons 1 to 6 and half of 7) completed, an hour apart.
	benchExec(b, f, fmt.Sprintf(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at)
	 SELECT ?, e.id, %[1]d, %[1]d, 1, (s.season_number * 100 + e.episode_number) * %[2]d
	 FROM items AS e JOIN items AS s ON s.id = e.parent_id
	 WHERE s.parent_id = ? AND (s.season_number - 1) * %[3]d + e.episode_number <= 130`, vistest.DurationMs, hour, perSeason),
		f.P18.ID, seriesID)
	benchExec(b, f, fmt.Sprintf(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at)
	 SELECT ?, id, %[1]d, %[1]d, 1, 1 FROM items WHERE id LIKE '01J9NEXTUPE%%'`, vistest.DurationMs), f.P16.ID)
	b.Logf("fixture: %d series × %d seasons × %d episodes in %v", series, seasons, perSeason, time.Since(start).Round(time.Millisecond))
	logNextUpPlan(b, f, f.P18.ID, seriesID)

	allSeries := make([]string, 0, series)
	for s := 1; s <= series; s++ {
		allSeries = append(allSeries, fmt.Sprintf("01J9NEXTUPS%015d", s))
	}
	scope := Profile(f.P18)
	for _, tc := range []struct {
		name string
		run  func() (string, error)
	}{
		{"query/series", func() (string, error) {
			rows, err := r.q.ListSeriesNextUpEpisodes(ctx, scopedq.ListSeriesNextUpEpisodesParams{ProfileID: f.P18.ID, SeriesID: sql.NullString{String: seriesID, Valid: true}})
			return fmt.Sprint(len(rows)), err
		}},
		{"query/season", func() (string, error) {
			rows, err := r.q.ListSeriesNextUpEpisodes(ctx, scopedq.ListSeriesNextUpEpisodesParams{ProfileID: f.P18.ID, SeriesID: sql.NullString{String: seriesID, Valid: true}, SeasonID: seasonID})
			return fmt.Sprint(len(rows)), err
		}},
		{"query/child-counts-100-series", func() (string, error) {
			rows, err := r.q.ListVisibleChildCounts(ctx, scopedq.ListVisibleChildCountsParams{ProfileID: f.P18.ID, ItemIds: allSeries})
			return fmt.Sprint(len(rows)), err
		}},
		{"page/LibraryItems-series-title", func() (string, error) {
			p, err := r.LibraryItems(ctx, scope, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortTitle, MaxRating: -1, Limit: MaxLibraryPage})
			return fmt.Sprint(len(p.Items)), err
		}},
		{"query/child-counts-100-series-all-watched", func() (string, error) {
			rows, err := r.q.ListVisibleChildCounts(ctx, scopedq.ListVisibleChildCountsParams{ProfileID: f.P16.ID, ItemIds: allSeries})
			return fmt.Sprint(len(rows)), err
		}},
		{"page/LibraryItems-series-title-all-watched", func() (string, error) {
			p, err := r.LibraryItems(ctx, Profile(f.P16), f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortTitle, MaxRating: -1, Limit: MaxLibraryPage})
			return fmt.Sprint(len(p.Items)), err
		}},
		{"reader/Item-series", func() (string, error) {
			d, err := r.Item(ctx, scope, seriesID)
			if err != nil || d.NextUp == nil {
				return "", err
			}
			return d.NextUp.Reason + " " + d.NextUp.Episode.ID, nil
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var lat []time.Duration
			var got string
			for b.Loop() {
				t0 := time.Now()
				var err error
				if got, err = tc.run(); err != nil {
					b.Fatal(err)
				}
				lat = append(lat, time.Since(t0))
			}
			want := map[string]string{"query/series": "260", "query/season": "20", "query/child-counts-100-series": "100",
				"page/LibraryItems-series-title": "100", "query/child-counts-100-series-all-watched": "100",
				"page/LibraryItems-series-title-all-watched": "100", "reader/Item-series": NextUpNext + " " + wantNext}[tc.name]
			if got != want {
				b.Fatalf("got %q, want %q", got, want)
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(lat[(len(lat)*95+99)/100-1].Microseconds())/1000, "p95-ms")
		})
	}
}

// logNextUpPlan logs EXPLAIN QUERY PLAN of ListSeriesNextUpEpisodes for a whole series, read from
// the generated query so the plan is that of the code under test.
func logNextUpPlan(b *testing.B, f *vistest.Fixture, profileID int64, seriesID string) {
	b.Helper()
	src, err := os.ReadFile("../store/scopedq/items.sql.go")
	if err != nil {
		b.Fatal(err)
	}
	_, query, found := strings.Cut(string(src), "const listSeriesNextUpEpisodes = `")
	query, _, closed := strings.Cut(query, "`")
	if !found || !closed {
		b.Fatal("listSeriesNextUpEpisodes not found")
	}
	rows, err := f.DB.Read.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, profileID, seriesID, "")
	if err != nil {
		b.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			b.Fatal(err)
		}
		fmt.Fprintf(&plan, "\n  %s", detail)
	}
	if err := rows.Err(); err != nil {
		b.Fatal(err)
	}
	b.Logf("EXPLAIN QUERY PLAN ListSeriesNextUpEpisodes:%s", plan.String())
}
