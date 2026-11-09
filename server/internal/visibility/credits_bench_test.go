package visibility

import (
	"context"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// BenchmarkListVisibleCredits measures the cast of a title with 60 roles, each person with a
// portrait row in JPEG and WebP (concept 17 stage 3, A-3: the LEFT JOIN on person_images of
// ADR 0026). Run before and after the join:
//
//	go test -run '^$' -bench ListVisibleCredits -benchtime 2000x ./internal/visibility/
func BenchmarkListVisibleCredits(b *testing.B) {
	ctx := context.Background()
	f := vistest.New(b)
	r := New(f.DB, vistest.Logger())
	m := f.Rated(6)
	benchExec(b, f, `WITH RECURSIVE n(k) AS (SELECT 1 UNION ALL SELECT k + 1 FROM n WHERE k < 60)
		INSERT INTO people (id, tmdb_id, name, profile_path, created_at) SELECT 1000 + k, 1000 + k, 'Person ' || k, '/p.jpg', 1 FROM n`)
	benchExec(b, f, `INSERT INTO credits (item_id, person_id, role, character, ord) SELECT '`+m.ID+`', id, 'cast', 'Role', id - 1000 FROM people WHERE id > 1000`)
	benchExec(b, f, `INSERT INTO person_images (person_id, format, status, source, size, rel_path, content_hash, media_type, width, height, bytes, fetched_at)
		SELECT id, f.format, 'ok', '/p.jpg', 'w185', 'people/' || id || '/w185.' || f.ext, 'hash', 'image/' || f.format, 185, 278, 9000, 1
		FROM people, (SELECT 'jpeg' AS format, 'jpg' AS ext UNION ALL SELECT 'webp', 'webp') AS f WHERE id > 1000`)
	pid := f.P18.ID
	b.ResetTimer()
	for b.Loop() {
		rows, err := r.q.ListVisibleCredits(ctx, scopedq.ListVisibleCreditsParams{ProfileID: pid, ItemID: m.ID})
		if err != nil || len(rows) != 60 {
			b.Fatalf("%d credits, %v; want 60", len(rows), err)
		}
	}
}
