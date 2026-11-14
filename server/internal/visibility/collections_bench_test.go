package visibility

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// BenchmarkAllCollections measures the page «Sammlungen» with 333 collections of three films each
// (CT 190 on 14 November 2026: 333 collections), every film watched or started by the profile
// (concept 21 P-10: years, runtime and progress per collection in one answer, so the page filters
// and sorts in the client). It reports the size of the answer as JSON of the model. Run:
//
//	go test -run '^$' -bench AllCollections -benchtime 200x ./internal/visibility/
func BenchmarkAllCollections(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	f.AddSeries(b)
	f.AddDiscover(b)
	r := New(f.DB, vistest.Logger())
	films := []string{f.Rated(0).ID, f.Rated(6).ID, f.Rated(12).ID, f.Rated(16).ID, f.Rated(18).ID}
	benchExec(b, f, `WITH RECURSIVE n(k) AS (SELECT 1 UNION ALL SELECT k + 1 FROM n WHERE k < 333)
		INSERT INTO collections (id, source, tmdb_id, name, created_at, updated_at)
		SELECT printf('01J90000000000000000B%05d', k), 'tmdb', 500000 + k, 'Collection ' || k, 1, 1 FROM n`)
	for i := range 3 {
		benchExec(b, f, `INSERT INTO collection_items (collection_id, item_id, position, created_at)
			SELECT id, ?1, ?2, 1 FROM collections WHERE id LIKE '01J90000000000000000B%'`, films[i], i)
	}
	for i := range 3 {
		benchExec(b, f, `INSERT OR REPLACE INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at)
			VALUES (?, ?, ?, 10000, ?, 1, ?)`, f.P18.ID, films[i], []int{10000, 3000, 0}[i], []int{1, 0, 0}[i], []any{int64(1), nil, nil}[i])
	}
	scope := Profile(f.P18)
	b.ResetTimer()
	var size int
	for b.Loop() {
		refs, err := r.AllCollections(ctx, scope)
		if err != nil || len(refs) < 333 {
			b.Fatalf("%d collections, %v; want 333 and more", len(refs), err)
		}
		if size == 0 {
			out, _ := json.Marshal(refs)
			size = len(out)
		}
	}
	b.ReportMetric(float64(size), "bytes/answer")
}
