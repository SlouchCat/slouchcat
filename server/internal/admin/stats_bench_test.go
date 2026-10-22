package admin

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"
)

// BenchmarkStatsMillionEvents measures Stats over one million progress events (docs/perf/M8.md:
// "Statistik-Abfrage p95 bei 1 Mio progress_events"): 3 profiles, 50 devices, 5000 titles, 1000
// playback sessions, events evenly spread over the last 400 days. It reports p50 and p95 per
// period. Run: go test -run '^$' -bench StatsMillionEvents -benchtime 40x ./internal/admin/
func BenchmarkStatsMillionEvents(b *testing.B) {
	ctx := context.Background()
	st := newStack(b, "0.9.0")
	st.svc.now = func() time.Time { return statsNow }
	span := (400 * day).Milliseconds()
	start := time.Now()
	exec(b, st.db.Write,
		`INSERT INTO accounts (id, email, password_hash, role, created_at, updated_at) VALUES (1, 'owner@example.test', 'x', 'owner', 1, 1)`,
		`INSERT INTO profiles (id, account_id, name, max_rating, is_default, created_at, updated_at) VALUES
		   (1, 1, 'Eins', 18, 1, 1, 1), (2, 1, 'Zwei', 18, 0, 1, 1), (3, 1, 'Drei', 18, 0, 1, 1)`,
		`INSERT INTO sessions (id, token_hash, account_id, profile_id, created_at, last_seen_at, expires_at) VALUES (1, 'h', 1, 1, 1, 1, 1)`,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'Filme', 'movies', 1, 1)`,
		`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 5000)
		 INSERT INTO items (id, kind, library_id, title, sort_title, year, added_at, created_at, updated_at)
		 SELECT printf('01J9BENCH%017d', i), 'movie', 1, 'Film ' || i, 'film ' || i, 1950 + i % 75, 1, 1, 1 FROM n`,
		`INSERT INTO item_texts (item_id, lang, title) SELECT id, 'de', 'Titel ' || title FROM items WHERE rowid % 2 = 0`,
		`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 50)
		 INSERT INTO devices (id, created_at, last_seen_at, type)
		 SELECT printf('%032d', i), 1, 1, CASE i % 5 WHEN 0 THEN 'desktop' WHEN 1 THEN 'phone' WHEN 2 THEN 'tablet' WHEN 3 THEN 'tv' ELSE NULL END FROM n`,
		fmt.Sprintf(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 1000)
		 INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, hardware,
		   started_at, last_activity_at, expires_at)
		 SELECT printf('01J9PS%%020d', i), 1, 1, 1 + i %% 3, printf('01J9BENCH%%017d', 1 + i %% 5000), 'f',
		   CASE i %% 3 WHEN 0 THEN 'direct' WHEN 1 THEN 'remux' ELSE 'transcode' END, i %% 2, %d - i * %d, 1, 1 FROM n`,
			statsNow.UnixMilli(), span/1000),
		fmt.Sprintf(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 1000000)
		 INSERT INTO progress_events (profile_id, item_id, device_id, position_ms, duration_ms, played_ms, seq, state,
		   client_time, server_time, playback_session_id)
		 SELECT 1 + i %% 3, printf('01J9BENCH%%017d', 1 + (i * 7919) %% 5000), CASE WHEN i %% 10 = 0 THEN NULL ELSE printf('%%032d', 1 + i %% 50) END,
		   0, 7200000, 10000 + i %% 20000, i %% 8, 'playing', 0, %d - (i * %d) / 1000000,
		   CASE WHEN i %% 7 = 0 THEN NULL ELSE printf('01J9PS%%020d', i / 8) END FROM n`,
			statsNow.UnixMilli(), span),
		`ANALYZE`,
	)
	b.Logf("fixture: 1000000 progress events in %v", time.Since(start).Round(time.Millisecond))
	for _, days := range []int{7, 30, 90, 365} {
		b.Run(fmt.Sprintf("days=%d", days), func(b *testing.B) {
			var lat []time.Duration
			for b.Loop() {
				t0 := time.Now()
				if _, err := st.svc.Stats(ctx, days); err != nil {
					b.Fatal(err)
				}
				lat = append(lat, time.Since(t0))
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(lat[(len(lat)*95+99)/100-1].Microseconds())/1000, "p95-ms")
		})
	}
}
