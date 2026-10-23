package admin

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
)

// A-8 and plan 3.2: condensing the raw events older than 90 days into progress_days must not move
// a single number of the statistics. The fixture of design 3.6 carries events 100 and 400 days
// back, so the condensation has work to do in every period; the statistics of all four periods are
// read before and after and must match field by field.
func TestStatsSurviveCondensing(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.svc.now = func() time.Time { return statsNow }
	seedStats(t, st.db.Write)
	// 120 days back, without a device and without a playback session: the condensed row stores the
	// empty device id, which the statistics have to map back into the unknown bucket.
	old := ago(120 * day)
	if _, err := st.db.Write.Exec(`INSERT INTO progress_events (profile_id, item_id, device_id, position_ms, duration_ms,
		played_ms, seq, state, client_time, server_time)
		VALUES (1, '01J9000000000000000000ST01', NULL, 0, 7200000, 123000, 1, 'playing', ?, ?)`, old, old); err != nil {
		t.Fatal(err)
	}

	periods := []int{7, 30, 90, 365}
	before := make(map[int]model.Stats, len(periods))
	for _, days := range periods {
		got, err := st.svc.Stats(ctx, days)
		if err != nil {
			t.Fatal(err)
		}
		before[days] = got
	}

	pb := playback.New(st.db, slog.New(slog.DiscardHandler), playback.WithClock(func() time.Time { return statsNow }))
	removed, err := pb.CondenseProgress(ctx, statsNow)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Errorf("condensed %d raw events, want the 3 older than 90 days", removed)
	}
	// Nothing older than the day the retention boundary falls in is left raw, and the day rows are there.
	boundary := statsNow.Add(-playback.ProgressRetention).UnixMilli() / 86_400_000 * 86_400_000
	var stillRaw, days int
	if err := st.db.Read.QueryRow(`SELECT count(*) FROM progress_events WHERE server_time < ?`, boundary).Scan(&stillRaw); err != nil {
		t.Fatal(err)
	}
	if err := st.db.Read.QueryRow(`SELECT count(*) FROM progress_days`).Scan(&days); err != nil {
		t.Fatal(err)
	}
	if stillRaw != 0 || days != 3 {
		t.Errorf("%d raw events older than the boundary and %d day rows, want 0 and 3", stillRaw, days)
	}

	for _, d := range periods {
		got, err := st.svc.Stats(ctx, d)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, before[d]) {
			t.Errorf("days %d after condensing:\n got %+v\nwant %+v", d, got, before[d])
		}
	}
	t.Logf("condensed %d raw events into %d day rows; all four periods unchanged", removed, days)
}

// The merge of two titles (A-6) takes the condensed days with it: the target keeps its own days and
// gains the source's, so the statistics of the merged title are the sum of both.
func TestMergeItemsTakesTheCondensedDays(t *testing.T) {
	m := newMaintFixture(t)
	target, source := m.f.Rated(16), m.f.Rated(18)
	now := time.Now()
	// Both titles get an event on the same old day, device and profile, so their day rows collide.
	old := now.Add(-200 * 24 * time.Hour).UnixMilli()
	m.exec(t, `INSERT INTO devices (id, created_at, last_seen_at, type) VALUES ('tv000000000000000000000000000000', 1, 1, 'tv')`)
	for _, item := range []string{source.ID, target.ID} {
		m.exec(t, `INSERT INTO progress_events (profile_id, item_id, device_id, position_ms, duration_ms, played_ms,
			seq, state, client_time, server_time)
			VALUES (?, ?, 'tv000000000000000000000000000000', 0, 7200000, 120000, 1, 'playing', ?, ?)`,
			m.f.P18.ID, item, old, old)
	}
	pb := playback.New(m.f.DB, slog.New(slog.DiscardHandler), playback.WithClock(func() time.Time { return now }))
	if _, err := pb.CondenseProgress(m.ctx, now); err != nil {
		t.Fatal(err)
	}

	if _, err := m.svc.MergeItems(m.ctx, target.ID, []string{source.ID}); err != nil {
		t.Fatal(err)
	}
	var rows, played, plays int64
	if err := m.f.DB.Read.QueryRow(`SELECT count(*), COALESCE(sum(played_ms), 0), COALESCE(sum(plays), 0)
		FROM progress_days WHERE item_id = ?`, target.ID).Scan(&rows, &played, &plays); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || played != 240_000 || plays != 2 {
		t.Errorf("merged day rows %d, played %d, plays %d; want 1 row with 240000 ms and 2 plays", rows, played, plays)
	}
	var left int64
	if err := m.f.DB.Read.QueryRow(`SELECT count(*) FROM progress_days WHERE item_id = ?`, source.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d day rows of the merged-away title left", left)
	}
}
