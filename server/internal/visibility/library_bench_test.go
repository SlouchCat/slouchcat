package visibility

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// BenchmarkLibraryRows measures the start page rows of concept 15 P-7 for a library of 10 000
// films with one playable file each (docs/perf/feinschliff.md): LibraryTiles («Meine Medien») and
// LibraryNewest («Neu in …»), next to what the start page and the library page already read
// (VisibleLibraries for the side navigation, Recent, LibraryItems sorted by added). Run:
//
//	go test -run '^$' -bench LibraryRows -benchtime 200x -v ./internal/visibility/
func BenchmarkLibraryRows(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	r := New(f.DB, vistest.Logger())
	const films = 10_000
	start := time.Now()
	for _, query := range []string{
		fmt.Sprintf(`WITH RECURSIVE n(s) AS (SELECT 1 UNION ALL SELECT s + 1 FROM n WHERE s < %d)
		 INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, genres, own_rating, metadata_state, added_at, created_at, updated_at)
		 SELECT printf('01J9BENCHM%%016d', s), 'movie', NULL, %d, 'Film ' || s, 'film ' || s, '[]', 12, 'matched', 1000 + s * 1000, 1, 1 FROM n`,
			films, f.LibraryID()),
		fmt.Sprintf(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec,
		   width, height, duration_ms, created_at, updated_at)
		 SELECT '01J9BENCHF' || substr(id, 11), id, %d, 'bench/' || id || '.mp4', 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, %d, 1, 1
		 FROM items WHERE id LIKE '01J9BENCHM%%'`, f.RootID(), vistest.DurationMs),
	} {
		benchExec(b, f, query)
	}
	b.Logf("fixture: %d films in %v", films, time.Since(start).Round(time.Millisecond))
	scope := Profile(f.P18)
	for _, tc := range []struct {
		name string
		run  func() (int, error)
	}{
		{"sidenav/VisibleLibraries", func() (int, error) { l, err := r.VisibleLibraries(ctx, scope); return len(l), err }},
		{"row/Recent", func() (int, error) { l, err := r.Recent(ctx, scope, MaxRowItems); return len(l), err }},
		{"row/LibraryTiles", func() (int, error) { l, err := r.LibraryTiles(ctx, scope); return len(l), err }},
		{"row/LibraryNewest", func() (int, error) {
			l, err := r.LibraryNewest(ctx, scope, f.LibraryID(), MaxRowItems)
			return len(l), err
		}},
		{"page/LibraryItems-added", func() (int, error) {
			p, err := r.LibraryItems(ctx, scope, f.LibraryID(), model.LibraryQuery{Sort: model.LibrarySortAdded, MaxRating: -1, Limit: MaxRowItems})
			return len(p.Items), err
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var lat []time.Duration
			for b.Loop() {
				t0 := time.Now()
				n, err := tc.run()
				if err != nil || n == 0 {
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
