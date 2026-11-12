package visibility

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// BenchmarkWatchedList measures listWatched (the page «Gesehen», concept 20 section 15.4, M-2)
// for a profile with 5 000 watched episodes: 50 series with one season of 100 episodes each, one
// playable file per episode, every episode watched one minute apart, plus the watched films of
// the fixture (docs/perf/20-bedienung.md). Every series is a candidate of the page, so each page
// counts the visible episodes and the watched ones of its 50 series. It reports p50 and p95 of the
// query alone and of Reader.Watched (query, episode counts and the summaries with watchedCount)
// for the default order, the order by title and the filter on series. Run:
//
//	go test -run '^$' -bench WatchedList -benchtime 200x -v ./internal/visibility/
func BenchmarkWatchedList(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	r := New(f.DB, vistest.Logger())
	start := time.Now()
	const series, perSeries = 50, 100
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
		`DELETE FROM progress`,
		fmt.Sprintf(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
		 SELECT %[1]d, id, %[2]d, %[2]d, 1, 1800000000000 + rowid * 60000, 1800000000000 + rowid * 60000
		 FROM items WHERE id LIKE '01J9BENCHE%%' OR kind = 'movie'`, f.P16.ID, vistest.DurationMs),
	} {
		benchExec(b, f, query)
	}
	var episodes int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress AS p JOIN items AS i ON i.id = p.item_id
		WHERE p.profile_id = ? AND p.watched_at IS NOT NULL AND i.kind = 'episode'`, f.P16.ID).Scan(&episodes); err != nil || episodes != series*perSeries {
		b.Fatalf("watched episodes = %d (%v), want %d", episodes, err, series*perSeries)
	}
	b.Logf("fixture: %d series × %d episodes, %d watched episodes of P16 in %v", series, perSeries, episodes, time.Since(start).Round(time.Millisecond))
	scope := Profile(f.P16)
	for _, tc := range []struct {
		name      string
		q         model.WatchedQuery
		queryOnly bool
	}{
		{"query/watched", model.WatchedQuery{Limit: 50}, true},
		{"reader/watched", model.WatchedQuery{Limit: 50}, false},
		{"reader/title", model.WatchedQuery{Sort: model.WatchedSortTitle, Limit: 50}, false},
		{"reader/series", model.WatchedQuery{Content: "series", Limit: 100}, false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var lat []time.Duration
			var n, total int
			for b.Loop() {
				t0 := time.Now()
				if tc.queryOnly {
					rows, err := r.q.ListWatched(ctx, scopedq.ListWatchedParams{ProfileID: f.P16.ID, MetaLang: scope.metaLang, UiLang: scope.language,
						LimitCount: int64(tc.q.Limit)})
					if err != nil {
						b.Fatal(err)
					}
					n, total = len(rows), int(rows[0].Total)
				} else {
					page, err := r.Watched(ctx, scope, tc.q)
					if err != nil {
						b.Fatal(err)
					}
					n, total = len(page.Items), page.Total
				}
				lat = append(lat, time.Since(t0))
			}
			if n == 0 || total < series {
				b.Fatalf("entries = %d of %d, want a full page of at least %d", n, total, series)
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(lat[(len(lat)*95+99)/100-1].Microseconds())/1000, "p95-ms")
		})
	}
}
