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

// BenchmarkAudioGroups measures the row «Reihen und Sammlungen» (concept 19 D0: p95 under 100 ms
// at 5 000 books) for an audiobook library of 5 000 books with one file each, 1 000 series of five
// volumes, without and with a filter, and the page of one series. Run:
//
//	go test -run '^$' -bench AudioGroups -benchtime 100x -v ./internal/visibility/
func BenchmarkAudioGroups(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	f.AddAudio(b)
	r := New(f.DB, vistest.Logger())
	lib := f.Audio.AudiobookLibraryID
	const books = 5_000
	start := time.Now()
	for _, query := range []string{
		fmt.Sprintf(`WITH RECURSIVE n(s) AS (SELECT 0 UNION ALL SELECT s + 1 FROM n WHERE s < %d)
		 INSERT INTO audio_series (id, library_id, series_key, name, name_locked, created_at, updated_at)
		 SELECT printf('01J9BENCHS%%016d', s), %d, 'reihe ' || s || char(31) || 'autorin', 'Reihe ' || s, 0, 1, 1 FROM n`,
			books/5-1, lib),
		fmt.Sprintf(`WITH RECURSIVE n(s) AS (SELECT 0 UNION ALL SELECT s + 1 FROM n WHERE s < %d)
		 INSERT INTO items (id, kind, library_id, title, sort_title, genres, own_rating, metadata_state, author, narrator,
		   series_name, series_index, audio_series_id, year, duration_ms, added_at, created_at, updated_at)
		 SELECT printf('01J9BENCHB%%016d', s), 'audiobook', %d, 'Buch ' || s, 'buch ' || s, '{"de":["Krimi"],"en":["Crime"]}', 0,
		   'matched', 'Autorin', 'Sprecher', CASE WHEN s %% 2 = 0 THEN 'Reihe ' || (s / 5) ELSE 'Reihe ' || (s / 5) || ' - Band' END,
		   s %% 5 + 1, printf('01J9BENCHS%%016d', s / 5), 1990 + s %% 30, 1000, 1000 + s, 1, 1 FROM n`, books-1, lib),
		`INSERT OR IGNORE INTO item_genres (item_id, lang, name) SELECT id, 'de', 'Krimi' FROM items WHERE id LIKE '01J9BENCHB%'`,
		fmt.Sprintf(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, audio_codec, duration_ms, created_at, updated_at)
		 SELECT '01J9BENCHF' || substr(id, 11), id, (SELECT id FROM library_roots WHERE library_id = %d), 'bench/' || id || '.m4b', 1, 1,
		   'm4b', 'aac', 1000, 1, 1 FROM items WHERE id LIKE '01J9BENCHB%%'`, lib),
	} {
		benchExec(b, f, query)
	}
	b.Logf("fixture: %d books in %d series in %v", books, books/5, time.Since(start).Round(time.Millisecond))
	scope := Profile(f.P18)
	for _, tc := range []struct {
		name string
		run  func() (int, error)
	}{
		{"row/AudioGroups", func() (int, error) {
			l, err := r.AudioGroups(ctx, scope, lib, model.LibraryQuery{MaxRating: -1})
			return len(l.Groups), err
		}},
		{"row/AudioGroups-filtered", func() (int, error) {
			l, err := r.AudioGroups(ctx, scope, lib, model.LibraryQuery{Genre: "Krimi", YearFrom: 2000, YearTo: 2010, MaxRating: -1})
			return len(l.Groups), err
		}},
		{"page/AudioSeries", func() (int, error) {
			s, err := r.AudioSeries(ctx, scope, lib, "01J9BENCHS0000000000000042")
			return len(s.Volumes), err
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
