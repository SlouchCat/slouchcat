package playback

import (
	"context"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func sumPlayed(t *testing.T, f *vistest.Fixture, profileID int64) int64 {
	t.Helper()
	var ms int64
	if err := f.DB.Read.QueryRow(`SELECT COALESCE(sum(played_ms), 0) FROM progress_events WHERE profile_id = ?`, profileID).Scan(&ms); err != nil {
		t.Fatal(err)
	}
	return ms
}

// K-5: an exact duplicate of an event (a retry) is stored once; the played time of an event counts
// at most the server time since the previous event of the playback (or its start) plus 10 %.
func TestReportDedupesAndBoundsPlayedTime(t *testing.T) {
	f := vistest.New(t)
	clock := &manualClock{now: time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)}
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTokenKey(testKey))
	ctx := context.Background()
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 30})
	m := f.Rated(6)
	info, err := s.Start(ctx, browserSession(t, f, kid), m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	event := func(seq, played int64) model.ProgressEvent {
		return model.ProgressEvent{PlaybackSessionID: info.PlaybackSessionID, PositionMs: seq * 10_000, DurationMs: vistest.DurationMs,
			PlayedMs: played, Seq: seq, State: "playing", ClientTime: clock.Now()}
	}

	clock.Advance(10 * time.Second)
	first := event(1, 10_000)
	for range 3 { // the client retries the same event
		if _, err := s.Report(ctx, kid, m.ID, first); err != nil {
			t.Fatal(err)
		}
	}
	if n := countEvents(t, f, kid.ID, m.ID); n != 1 {
		t.Errorf("events after three identical reports = %d, want 1", n)
	}
	if ms := sumPlayed(t, f, kid.ID); ms != 10_000 {
		t.Errorf("played after the duplicates = %d ms", ms)
	}

	// One event claiming an hour, 10 s after the previous one: 11 s count.
	clock.Advance(10 * time.Second)
	p, err := s.Report(ctx, kid, m.ID, event(2, 3_600_000))
	if err != nil {
		t.Fatal(err)
	}
	if ms := sumPlayed(t, f, kid.ID); ms != 21_000 {
		t.Errorf("played after an over-long event = %d ms, want 21000", ms)
	}
	if p.WatchTime == nil || p.WatchTime.RemainingMinutes != 30 {
		t.Errorf("watch time = %+v", p.WatchTime)
	}
	// The first event of a playback is bounded by the start of its playback session.
	other, err := s.Start(ctx, browserSession(t, f, kid), f.Rated(12).ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(5 * time.Second)
	if _, err := s.Report(ctx, kid, f.Rated(12).ID, model.ProgressEvent{PlaybackSessionID: other.PlaybackSessionID, PositionMs: 5000,
		DurationMs: vistest.DurationMs, PlayedMs: 60_000, Seq: 1, State: "paused", ClientTime: clock.Now()}); err != nil {
		t.Fatal(err)
	}
	if ms := sumPlayed(t, f, kid.ID); ms != 26_500 {
		t.Errorf("played after the first event of a playback = %d ms, want 26500", ms)
	}
}

// K-5 backstop: the sweep ends the running playback of a profile whose watch time is used up,
// also when no progress event of that playback reached the limit.
func TestSweepEndsPlaybackOverWatchLimit(t *testing.T) {
	f := vistest.New(t)
	clock := &manualClock{now: time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)}
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTokenKey(testKey))
	ctx := context.Background()
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 1})
	m := f.Rated(6)
	info, err := s.Start(ctx, browserSession(t, f, kid), m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	free, err := s.Start(ctx, browserSession(t, f, f.P18), m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	s.sweep(ctx)
	if by := stoppedBy(t, f, info.PlaybackSessionID); by != "" {
		t.Fatalf("ended below the limit: %q", by)
	}
	// Played time of another client of the profile (no revocation through Report).
	if _, err := f.DB.Write.Exec(`INSERT INTO progress_events (profile_id, item_id, position_ms, duration_ms, played_ms, seq, state, client_time, server_time)
		VALUES (?, ?, 0, 1000, 60000, 1, 'playing', ?, ?)`, kid.ID, m.ID, clock.Now().UnixMilli(), clock.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Second)
	s.sweep(ctx)
	if by := stoppedBy(t, f, info.PlaybackSessionID); by != model.StoppedByLimit {
		t.Errorf("stopped_by after the sweep = %q", by)
	}
	if by := stoppedBy(t, f, free.PlaybackSessionID); by != "" {
		t.Errorf("profile without limit ended: %q", by)
	}
}

// The report sums one query per range and assigns quarter hours to the calendar days of the
// server's time zone.
func TestWatchReportDayBoundaries(t *testing.T) {
	f := vistest.New(t)
	clock := &manualClock{now: time.Date(2026, 10, 21, 12, 0, 0, 0, zurich)}
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTokenKey(testKey))
	ctx := context.Background()
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 30})
	m := f.Rated(6)
	for _, ev := range []struct {
		at     time.Time
		played int64
	}{
		{time.Date(2026, 10, 19, 23, 59, 0, 0, zurich), 120_000},
		{time.Date(2026, 10, 20, 0, 0, 0, 0, zurich), 180_000},
		{time.Date(2026, 10, 20, 23, 44, 0, 0, zurich), 60_000},
		{time.Date(2026, 10, 21, 11, 59, 0, 0, zurich), 240_000},
		{time.Date(2026, 10, 18, 23, 59, 0, 0, zurich), 600_000}, // before the range
	} {
		if _, err := f.DB.Write.Exec(`INSERT INTO progress_events (profile_id, item_id, position_ms, duration_ms, played_ms, seq, state, client_time, server_time)
			VALUES (?, ?, 0, 1000, ?, 1, 'playing', ?, ?)`, kid.ID, m.ID, ev.played, ev.at.UnixMilli(), ev.at.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := s.WatchReport(ctx, browserSession(t, f, f.P18), kid.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.WatchDay{{Date: "2026-10-19", Minutes: 2}, {Date: "2026-10-20", Minutes: 4}, {Date: "2026-10-21", Minutes: 4}}
	if len(rep.Days) != len(want) {
		t.Fatalf("days = %+v", rep.Days)
	}
	for i := range want {
		if rep.Days[i] != want[i] {
			t.Errorf("day %d = %+v, want %+v", i, rep.Days[i], want[i])
		}
	}
}
