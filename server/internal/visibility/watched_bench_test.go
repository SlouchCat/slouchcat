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

// BenchmarkWatchedCounts measures the two counts behind the half check mark on a podcast library
// of 100 podcasts with 1000 episodes each (docs/perf/20-bedienung.md): ListVisibleEpisodeCounts
// (episodeCount of listWatched) for 1, 20 and 100 podcasts, and ListVisibleWatchedCounts
// (ItemRef.watchedCount) for a home-like row of 1 to 20 podcasts while P16 has heard 20 000
// episodes, 200 of every podcast. Run:
//
//	go test -run '^$' -bench WatchedCounts -benchtime 200x -v ./internal/visibility/
func BenchmarkWatchedCounts(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	f.AddAudio(b)
	r := New(f.DB, vistest.Logger())
	lib := f.Audio.PodcastLibraryID
	start := time.Now()
	const podcasts, perPodcast, heard = 100, 1000, 200
	for _, query := range []string{
		fmt.Sprintf(`WITH RECURSIVE n(s) AS (SELECT 1 UNION ALL SELECT s + 1 FROM n WHERE s < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9PODBENP%%015d', s), 'podcast', NULL, %d, 'Podcast ' || s, 'podcast ' || s, '[]', 0, 'matched', 1, 1, 1 FROM n`,
			podcasts, lib),
		fmt.Sprintf(`WITH RECURSIVE n(e) AS (SELECT 1 UNION ALL SELECT e + 1 FROM n WHERE e < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, enclosure_url, published_at,
		   metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9PODBENE%%05d%%010d', CAST(substr(p.id, 12) AS INTEGER), n.e), 'podcast_episode', p.id, %d,
		   'Folge ' || n.e, 'folge ' || n.e, '[]', 0, 'https://example.invalid/' || n.e || '.mp3', n.e,
		   'matched', 1, 1, 1
		 FROM items AS p, n WHERE p.id LIKE '01J9PODBENP%%'`, perPodcast, lib),
		`DELETE FROM progress`,
	} {
		benchExec(b, f, query)
	}
	benchExec(b, f, fmt.Sprintf(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
	 SELECT ?, id, %[1]d, %[1]d, 1, 1800000000000 + rowid, 1800000000000 + rowid FROM items
	 WHERE id LIKE '01J9PODBENE%%' AND CAST(substr(id, 17) AS INTEGER) <= %[2]d`, vistest.DurationMs, heard), f.P16.ID)
	var watched int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress WHERE profile_id = ? AND watched_at IS NOT NULL`, f.P16.ID).Scan(&watched); err != nil ||
		watched != podcasts*heard {
		b.Fatalf("watched episodes = %d (%v), want %d", watched, err, podcasts*heard)
	}
	b.Logf("fixture: %d podcasts × %d episodes, %d heard by P16 in %v", podcasts, perPodcast, watched, time.Since(start).Round(time.Millisecond))
	ids := make([]string, 0, podcasts)
	for p := 1; p <= podcasts; p++ {
		ids = append(ids, fmt.Sprintf("01J9PODBENP%015d", p))
	}
	type run struct {
		name string
		want int
		run  func() ([]int64, error)
	}
	var runs []run
	for _, n := range []int{1, 20, 100} {
		runs = append(runs, run{fmt.Sprintf("episode-counts-%d", n), perPodcast, func() ([]int64, error) {
			rows, err := r.q.ListVisibleEpisodeCounts(ctx, scopedq.ListVisibleEpisodeCountsParams{ProfileID: f.P16.ID, ItemIds: ids[:n]})
			out := make([]int64, 0, len(rows))
			for _, row := range rows {
				out = append(out, row.EpisodeCount)
			}
			return out, err
		}})
	}
	for _, n := range []int{1, 5, 20} {
		runs = append(runs, run{fmt.Sprintf("watched-counts-%d", n), heard, func() ([]int64, error) {
			rows, err := r.q.ListVisibleWatchedCounts(ctx, scopedq.ListVisibleWatchedCountsParams{ProfileID: f.P16.ID, ItemIds: ids[:n]})
			out := make([]int64, 0, len(rows))
			for _, row := range rows {
				out = append(out, row.WatchedCount)
			}
			return out, err
		}})
	}
	for _, tc := range runs {
		b.Run(tc.name, func(b *testing.B) {
			var lat []time.Duration
			for b.Loop() {
				t0 := time.Now()
				counts, err := tc.run()
				if err != nil || len(counts) == 0 || slices.ContainsFunc(counts, func(c int64) bool { return c != int64(tc.want) }) {
					b.Fatalf("counts %v, %v, want %d each", counts, err, tc.want)
				}
				lat = append(lat, time.Since(t0))
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(lat[(len(lat)*95+99)/100-1].Microseconds())/1000, "p95-ms")
		})
	}
}
