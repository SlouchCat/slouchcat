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

// BenchmarkPodcastLibrary measures the podcasts in lists (contract 1.22.0, ItemRef.watched) on a
// podcast library of 100 podcasts with 1000 episodes each (100 000 episodes with an enclosure):
// the child counts of all 100 podcasts in one ListVisibleChildCounts and the library page sorted
// by title, for P18, who has heard nothing, and for P16, who has heard every episode (the worst
// case: every podcast is read to its last episode). Run:
//
//	go test -run '^$' -bench PodcastLibrary -benchtime 200x -v ./internal/visibility/
func BenchmarkPodcastLibrary(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	f.AddAudio(b)
	r := New(f.DB, vistest.Logger())
	lib := f.Audio.PodcastLibraryID
	start := time.Now()
	const podcasts, perPodcast = 100, 1000
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
	} {
		benchExec(b, f, query)
	}
	// Heard as the 90 % rule writes it: completed and watched_at (concept 20 section 6.3).
	benchExec(b, f, fmt.Sprintf(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
	 SELECT ?, id, %[1]d, %[1]d, 1, 1, 1 FROM items WHERE id LIKE '01J9PODBENE%%'`, vistest.DurationMs), f.P16.ID)
	b.Logf("fixture: %d podcasts × %d episodes in %v", podcasts, perPodcast, time.Since(start).Round(time.Millisecond))
	ids := make([]string, 0, podcasts)
	for p := 1; p <= podcasts; p++ {
		ids = append(ids, fmt.Sprintf("01J9PODBENP%015d", p))
	}
	for _, tc := range []struct {
		name string
		run  func() (int, error)
	}{
		{"query/child-counts-100-podcasts", func() (int, error) {
			rows, err := r.q.ListVisibleChildCounts(ctx, scopedq.ListVisibleChildCountsParams{ProfileID: f.P18.ID, ItemIds: ids})
			return len(rows), err
		}},
		{"page/LibraryItems-podcasts-title", func() (int, error) {
			p, err := r.LibraryItems(ctx, Profile(f.P18), lib, model.LibraryQuery{Sort: model.LibrarySortTitle, MaxRating: -1, Limit: MaxLibraryPage})
			return len(p.Items), err
		}},
		{"query/child-counts-100-podcasts-all-heard", func() (int, error) {
			rows, err := r.q.ListVisibleChildCounts(ctx, scopedq.ListVisibleChildCountsParams{ProfileID: f.P16.ID, ItemIds: ids})
			return len(rows), err
		}},
		{"page/LibraryItems-podcasts-title-all-heard", func() (int, error) {
			p, err := r.LibraryItems(ctx, Profile(f.P16), lib, model.LibraryQuery{Sort: model.LibrarySortTitle, MaxRating: -1, Limit: MaxLibraryPage})
			return len(p.Items), err
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var lat []time.Duration
			for b.Loop() {
				t0 := time.Now()
				n, err := tc.run()
				if err != nil || n < podcasts {
					b.Fatalf("%d entries, %v", n, err)
				}
				lat = append(lat, time.Since(t0))
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(lat[(len(lat)*95+99)/100-1].Microseconds())/1000, "p95-ms")
		})
	}
}
