package playback

import (
	"context"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 20 section 6.3: an event that meets the 90 % rule sets watched_at to its time; a later
// event below 90 % reopens the title for continue watching (completed 0, its position) but leaves
// watched_at, and finishing again moves it on. The answer of Report carries it.
func TestReportKeepsWatchedAt(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	now := time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	m := f.Rated(12)
	report := func(pos int64) model.Progress {
		t.Helper()
		p, err := s.Report(ctx, f.P18, m.ID, model.ProgressEvent{PositionMs: pos, DurationMs: vistest.DurationMs, PlayedMs: 10_000,
			Seq: 1, State: "playing", ClientTime: now})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := report(vistest.DurationMs / 2); p.WatchedAt != nil || progressOf(t, f, f.P18.ID, m.ID).WatchedAt.Valid {
		t.Errorf("50 %% event: watchedAt %v, want none", p.WatchedAt)
	}
	finished := now
	if p := report(vistest.DurationMs * 95 / 100); p.WatchedAt == nil || !p.WatchedAt.Equal(finished) {
		t.Errorf("95 %% event: watchedAt %v, want %v", p.WatchedAt, finished)
	}
	now = now.Add(time.Hour)
	p := report(vistest.DurationMs / 10)
	stored := progressOf(t, f, f.P18.ID, m.ID)
	if p.Completed || stored.Completed != 0 || stored.PositionMs != vistest.DurationMs/10 {
		t.Errorf("10 %% after finishing: %+v, want reopened at 10 %%", stored)
	}
	if p.WatchedAt == nil || !p.WatchedAt.Equal(finished) || stored.WatchedAt.Int64 != finished.UnixMilli() {
		t.Errorf("10 %% after finishing: watchedAt %v / %v, want it to stay at %v", p.WatchedAt, stored.WatchedAt, finished)
	}
	now = now.Add(time.Hour)
	if p := report(vistest.DurationMs); p.WatchedAt == nil || !p.WatchedAt.Equal(now) || progressOf(t, f, f.P18.ID, m.ID).WatchedAt.Int64 != now.UnixMilli() {
		t.Errorf("finished again: watchedAt %v, want %v", p.WatchedAt, now)
	}
}

// The late online event (the isLate branch: an older seq of the same playback) leaves position and
// completed alone, but beyond 90 % it still sets watched_at (concept 20 section 6.3).
func TestReportLateEventSetsWatchedAt(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	now := time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	m := f.Rated(12)
	t0 := now.Add(-time.Minute)
	report := func(seq, pos int64, client time.Time) model.Progress {
		t.Helper()
		p, err := s.Report(ctx, f.P18, m.ID, model.ProgressEvent{MediaFileID: m.FileID, DeviceID: "tv", PositionMs: pos,
			DurationMs: vistest.DurationMs, PlayedMs: 10_000, Seq: seq, State: "paused", ClientTime: client})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	report(5, vistest.DurationMs/2, t0.Add(50*time.Second))
	now = now.Add(time.Second)
	p := report(3, vistest.DurationMs*95/100, t0.Add(30*time.Second))
	stored := progressOf(t, f, f.P18.ID, m.ID)
	if stored.PositionMs != vistest.DurationMs/2 || stored.Completed != 0 || p.PositionMs != vistest.DurationMs/2 {
		t.Errorf("late event moved the progress: %+v", stored)
	}
	if !stored.WatchedAt.Valid || stored.WatchedAt.Int64 != now.UnixMilli() || p.WatchedAt == nil || !p.WatchedAt.Equal(now) {
		t.Errorf("late event beyond 90 %%: watched_at %v (answer %v), want %v", stored.WatchedAt, p.WatchedAt, now)
	}
}

// The offline event older than the stored progress only books watch time, but beyond 90 % it sets
// watched_at to its corrected time; a newer one beyond 90 % sets it too, and an older one below
// 90 % leaves it (concept 20 section 6.3).
func TestReportOfflineSetsWatchedAt(t *testing.T) {
	t.Parallel()
	t18 := time.Date(2026, 11, 12, 18, 0, 0, 0, zurich)
	f, s, clock := newOfflineService(t, t18)
	ctx := context.Background()
	m := f.Rated(12)
	minute := time.Minute.Milliseconds()
	if _, err := s.Report(ctx, f.P12, m.ID, model.ProgressEvent{PositionMs: 10 * minute, DurationMs: vistest.DurationMs,
		Seq: 1, State: "playing", ClientTime: t18}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Hour)
	session := offlineSession(t, f, f.P18)
	upload := func(n int, at time.Time, positionMs int64) {
		t.Helper()
		res, err := s.ReportOffline(ctx, session, clock.Now(), []model.OfflineEvent{offlineEvent(f.P12, m, offlineID(n), 1, at, positionMs, 10_000)})
		if err != nil || res.Accepted != 1 {
			t.Fatalf("upload %d: %+v, %v", n, res, err)
		}
	}
	finished := t18.Add(-30 * time.Minute)
	upload(1, finished, 115*minute) // older than the online event, beyond 90 %
	p := progressOf(t, f, f.P12.ID, m.ID)
	if p.PositionMs != 10*minute || p.Completed != 0 || p.UpdatedAt != t18.UnixMilli() {
		t.Errorf("older offline event moved the progress: %+v", p)
	}
	if p.WatchedAt.Int64 != finished.UnixMilli() {
		t.Errorf("older offline event beyond 90 %%: watched_at %v, want %d", p.WatchedAt, finished.UnixMilli())
	}
	upload(2, t18.Add(-40*time.Minute), 30*minute) // older still, below 90 %
	if p := progressOf(t, f, f.P12.ID, m.ID); p.WatchedAt.Int64 != finished.UnixMilli() {
		t.Errorf("older event below 90 %% changed watched_at to %v", p.WatchedAt)
	}
	upload(3, t18.Add(-50*time.Minute), 116*minute) // beyond 90 %, but before the stored date
	if p := progressOf(t, f, f.P12.ID, m.ID); p.WatchedAt.Int64 != finished.UnixMilli() {
		t.Errorf("an earlier finish moved watched_at back to %v", p.WatchedAt)
	}
	newer := t18.Add(30 * time.Minute)
	upload(4, newer, 118*minute) // newer, beyond 90 %: replaces the progress
	if p := progressOf(t, f, f.P12.ID, m.ID); p.Completed != 1 || p.WatchedAt.Int64 != newer.UnixMilli() {
		t.Errorf("newer offline event beyond 90 %%: %+v, want completed and watched_at %d", p, newer.UnixMilli())
	}
}
