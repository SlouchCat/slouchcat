package playback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

// watchedRow is the stored progress of a title as "position/duration/completed/updated/watched"
// ("-" for a NULL watched_at, "none" without a row).
func watchedRow(t *testing.T, f *vistest.Fixture, profileID int64, itemID string) string {
	t.Helper()
	var out string
	err := f.DB.Read.QueryRow(`SELECT position_ms || '/' || duration_ms || '/' || completed || '/' || updated_at || '/' || COALESCE(watched_at, '-')
		FROM progress WHERE profile_id = ? AND item_id = ?`, profileID, itemID).Scan(&out)
	if errors.Is(err, sql.ErrNoRows) {
		return "none"
	}
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Concept 20 section 6.4, a single title: true finishes it at the end of its duration (a new row
// takes the duration of visibility.WatchTargets, TestWatchTargets) with watched_at and updated_at
// now; false clears watched_at, a finished title starts again at 0 while a running
// position stays, and updated_at stays. No activity event (O-2); an invisible title is 404.
func TestSetWatchedSingleTitle(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	now := time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)
	var events []model.ActivityEvent
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now })).
		WithActivity(func(_ context.Context, e model.ActivityEvent) { events = append(events, e) })
	ctx := context.Background()
	set := func(p model.Profile, id string, watched bool) {
		t.Helper()
		if err := s.SetWatched(ctx, p, id, watched); err != nil {
			t.Fatal(err)
		}
	}
	at := now.UnixMilli()
	m := f.Rated(12)
	set(f.P18, m.ID, true)
	if got, want := watchedRow(t, f, f.P18.ID, m.ID), fmt.Sprintf("%d/%d/1/%d/%d", vistest.DurationMs, vistest.DurationMs, at, at); got != want {
		t.Errorf("film watched = %s, want %s", got, want)
	}
	now = now.Add(time.Hour)
	set(f.P18, m.ID, false)
	if got, want := watchedRow(t, f, f.P18.ID, m.ID), fmt.Sprintf("0/%d/0/%d/-", vistest.DurationMs, at); got != want {
		t.Errorf("film unwatched = %s, want %s (position 0, updated_at kept)", got, want)
	}
	// A second run: finished once, started again; removing the check mark keeps the position.
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE progress SET position_ms = 600000, completed = 0, updated_at = 5, watched_at = 3 WHERE profile_id = ? AND item_id = ?`, f.P18.ID, m.ID)
	set(f.P18, m.ID, false)
	if got, want := watchedRow(t, f, f.P18.ID, m.ID), fmt.Sprintf("600000/%d/0/5/-", vistest.DurationMs); got != want {
		t.Errorf("second run unwatched = %s, want %s", got, want)
	}
	// Setting it again moves the position to the end of the stored duration.
	set(f.P18, m.ID, true)
	if got, want := watchedRow(t, f, f.P18.ID, m.ID), fmt.Sprintf("%d/%d/1/%d/%d", vistest.DurationMs, vistest.DurationMs, now.UnixMilli(), now.UnixMilli()); got != want {
		t.Errorf("second run watched = %s, want %s", got, want)
	}
	// Removing a check mark that was never set changes nothing.
	set(f.P18, f.Rated(0).ID, false)
	if got := watchedRow(t, f, f.P18.ID, f.Rated(0).ID); got != "none" {
		t.Errorf("unwatched without a row = %s, want none", got)
	}
	if err := s.SetWatched(ctx, f.P6, m.ID, true); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible title: %v, want ErrNotFound", err)
	}
	if got := watchedRow(t, f, f.P6.ID, m.ID); got != "none" {
		t.Errorf("invisible title stored %s", got)
	}
	if len(events) != 0 {
		t.Errorf("the check mark by hand wrote activity events: %+v", events)
	}
}

// Concept 20 section 6.4: the check mark of a series writes every visible episode below it, the
// specials too, and leaves the episode the profile may not see alone: P12 marks the series, E2
// (rated 16) gets no row; removing it again clears the same episodes. A profile that may not see
// the series gets 404 and writes nothing; extras below the series stay untouched.
func TestSetWatchedSeriesSkipsHiddenEpisodes(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	now := time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	show := f.Show
	if err := s.SetWatched(ctx, f.P12, show.Series.ID, true); err != nil {
		t.Fatal(err)
	}
	// updated_at and watched_at are staggered (TestSetWatchedStaggersEpisodes in package offline),
	// so only the fixed fields are compared here.
	fields := func(m vistest.Movie) string {
		t.Helper()
		parts := strings.Split(watchedRow(t, f, f.P12.ID, m.ID), "/")
		if len(parts) != 5 {
			return strings.Join(parts, "/")
		}
		// position/completed/watched_at with updated_at cut off its end ("" when both are equal)
		return parts[0] + "/" + parts[2] + "/" + strings.TrimSuffix(parts[4], parts[3])
	}
	for _, m := range []vistest.Movie{show.Special, show.E1, show.E3} {
		if got, want := fields(m), fmt.Sprintf("%d/1/", vistest.DurationMs); got != want {
			t.Errorf("P12 %s after the series = %s, want %s", m.Label, got, want)
		}
	}
	for _, m := range []vistest.Movie{show.E2, show.Extra, show.Season, show.Series} {
		if got := watchedRow(t, f, f.P12.ID, m.ID); got != "none" {
			t.Errorf("P12 %s after the series = %s, want untouched", m.Label, got)
		}
	}
	if err := s.SetWatched(ctx, f.P12, show.E2.ID, true); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P12 marks the hidden E2 directly: %v, want ErrNotFound", err)
	}
	if err := s.SetWatched(ctx, f.P12, show.Series.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, m := range []vistest.Movie{show.Special, show.E1, show.E3} {
		if got := fields(m); got != "0/0/-" {
			t.Errorf("P12 %s after removing = %s, want 0/0/- (position 0, not completed, no watched_at)", m.Label, got)
		}
	}
	if err := s.SetWatched(ctx, f.P6, show.Season.ID, true); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 marks the season rated 12: %v, want ErrNotFound", err)
	}
	for _, m := range show.All() {
		if got := watchedRow(t, f, f.P6.ID, m.ID); got != "none" {
			t.Errorf("P6 %s = %s, want untouched", m.Label, got)
		}
	}
}
