package visibility

import (
	"context"
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

// BenchmarkContinueWatching measures ListContinueWatching (the row "Weiterschauen", concept 15
// P-1) for a profile with 10 000 progress rows: 100 series with one season of 100 episodes each,
// one playable file per episode (docs/perf/feinschliff.md, review L-1). Episodes 1 to 99 are completed one
// hour apart, episode 100 carries a row as well. Two profiles:
//   - mixed (P16): the newest row of series s lies s × 1.5 days back; even series have episode 100
//     started (the entry is that episode), odd series have it at position 0, so their newest row is
//     the completed episode 99 and the entry is the next episode inside the 60-day window only;
//   - completed (P18): every series ends on the completed episode 99 inside the window (s × 0.5
//     days back), episode 100 at position 0: every series walks next_episodes, the worst case.
//
// It reports p50 and p95 of the query alone and of Reader.ContinueWatching (query plus summaries)
// and logs EXPLAIN QUERY PLAN of the query. Run:
//
//	go test -run '^$' -bench ContinueWatching -benchtime 200x -v ./internal/visibility/
func BenchmarkContinueWatching(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	r := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return continueNow }))
	start := time.Now()
	const series, perSeries = 100, 100
	hour := time.Hour.Milliseconds()
	for _, query := range []string{
		fmt.Sprintf(`WITH RECURSIVE n(s) AS (SELECT 1 UNION ALL SELECT s + 1 FROM n WHERE s < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9BENCHS%%016d', s), 'series', NULL, %d, 'Serie ' || s, 'serie ' || s, '[]', 12, 'matched', 1, 1, 1 FROM n`,
			series, f.LibraryID()),
		fmt.Sprintf(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, season_number, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9BENCHN%%016d', CAST(substr(id, 11) AS INTEGER)), 'season', id, %d, 'Staffel 1', 'staffel 1', '[]', 1, 'matched', 1, 1, 1
		 FROM items WHERE id LIKE '01J9BENCHS%%'`, f.LibraryID()),
		fmt.Sprintf(`WITH RECURSIVE n(e) AS (SELECT 1 UNION ALL SELECT e + 1 FROM n WHERE e < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, season_number, episode_number,
		   metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9BENCHE%%010d%%06d', CAST(substr(s.id, 11) AS INTEGER), n.e), 'episode', s.id, %d, 'Folge ' || n.e, 'folge ' || n.e,
		   '[]', 12, 1, n.e, 'matched', 1, 1, 1
		 FROM items AS s, n WHERE s.id LIKE '01J9BENCHN%%'`, perSeries, f.LibraryID()),
		fmt.Sprintf(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec,
		   width, height, duration_ms, created_at, updated_at)
		 SELECT '01J9BENCHF' || substr(id, 11), id, %d, 'bench/' || id || '.mp4', 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, %d, 1, 1
		 FROM items WHERE id LIKE '01J9BENCHE%%'`, f.RootID(), vistest.DurationMs),
	} {
		benchExec(b, f, query)
	}
	// The episode number is the last six digits of its id, the series number the ten before them.
	progress := func(p model.Profile, newestOf string, startedLast string) {
		benchExec(b, f, `DELETE FROM progress WHERE profile_id = ?`, p.ID)
		benchExec(b, f, fmt.Sprintf(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at)
		 SELECT ?, e.id,
		   CASE WHEN e.episode_number < %[1]d THEN %[2]d WHEN %[3]s THEN %[2]d / 10 ELSE 0 END, %[2]d,
		   e.episode_number < %[1]d,
		   (%[4]s) - (%[1]d - e.episode_number) * %[5]d
		 FROM items AS e WHERE e.id LIKE '01J9BENCHE%%'`,
			perSeries, vistest.DurationMs, startedLast, newestOf, hour), p.ID)
	}
	const seriesNo = `CAST(substr(e.id, 11, 10) AS INTEGER)`
	day := (24 * time.Hour).Milliseconds()
	now := continueNow.UnixMilli()
	progress(f.P16, fmt.Sprintf(`%d - %s * %d`, now, seriesNo, day*3/2), seriesNo+` % 2 = 0`)
	progress(f.P18, fmt.Sprintf(`%d - %s * %d`, now, seriesNo, day/2), `0`)
	b.Logf("fixture: %d series × %d episodes, 2 × %d progress rows in %v",
		series, perSeries, series*perSeries, time.Since(start).Round(time.Millisecond))
	logQueryPlan(b, f, f.P16)

	// query/…: ListContinueWatching alone; reader/…: Reader.ContinueWatching with the summaries.
	for _, tc := range []struct {
		name      string
		p         model.Profile
		queryOnly bool
	}{{"query/mixed", f.P16, true}, {"query/completed", f.P18, true}, {"reader/mixed", f.P16, false}, {"reader/completed", f.P18, false}} {
		b.Run(tc.name, func(b *testing.B) {
			var lat []time.Duration
			var n int
			for b.Loop() {
				t0 := time.Now()
				var err error
				if tc.queryOnly {
					var rows []scopedq.Item
					rows, err = r.q.ListContinueWatching(ctx, scopedq.ListContinueWatchingParams{
						ProfileID: tc.p.ID, LimitCount: MaxRowItems, Since: continueNow.Add(-NextEpisodeWindow).UnixMilli(),
					})
					n = len(rows)
				} else {
					var got []model.ItemSummary
					got, err = r.ContinueWatching(ctx, Profile(tc.p), MaxRowItems)
					n = len(got)
				}
				if err != nil {
					b.Fatal(err)
				}
				lat = append(lat, time.Since(t0))
			}
			if n != MaxRowItems {
				b.Fatalf("entries = %d, want %d", n, MaxRowItems)
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(lat[(len(lat)*95+99)/100-1].Microseconds())/1000, "p95-ms")
		})
	}
}

func benchExec(b *testing.B, f *vistest.Fixture, query string, args ...any) {
	b.Helper()
	if _, err := f.DB.Write.ExecContext(context.Background(), query, args...); err != nil {
		b.Fatalf("%s: %v", query, err)
	}
}

// logQueryPlan logs EXPLAIN QUERY PLAN of ListContinueWatching, read from the generated query so
// the plan is that of the code under test.
func logQueryPlan(b *testing.B, f *vistest.Fixture, p model.Profile) {
	b.Helper()
	src, err := os.ReadFile("../store/scopedq/items.sql.go")
	if err != nil {
		b.Fatal(err)
	}
	_, query, found := strings.Cut(string(src), "const listContinueWatching = `")
	query, _, closed := strings.Cut(query, "`")
	if !found || !closed {
		b.Fatal("listContinueWatching not found")
	}
	rows, err := f.DB.Read.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query,
		p.ID, MaxRowItems, continueNow.Add(-NextEpisodeWindow).UnixMilli())
	if err != nil {
		b.Fatal(err)
	}
	defer rows.Close()
	depth := map[int]int{}
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			b.Fatal(err)
		}
		depth[id] = depth[parent] + 1
		fmt.Fprintf(&plan, "\n%s%s", strings.Repeat("  ", depth[id]-1), detail)
	}
	if err := rows.Err(); err != nil {
		b.Fatal(err)
	}
	b.Logf("EXPLAIN QUERY PLAN ListContinueWatching:%s", plan.String())
}
