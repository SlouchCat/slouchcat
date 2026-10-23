package playback

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// condenseNow is the clock of the condensation tests. The retention boundary
// (condenseNow - ProgressRetention) falls in the middle of a UTC day on purpose, so the tests show
// that only whole days are condensed.
var condenseNow = time.Date(2026, 10, 23, 11, 30, 0, 0, time.UTC)

// condenseAt is the UTC time at hour of the day d days before condenseNow.
func condenseAt(d, hour int) time.Time {
	at := condenseNow.AddDate(0, 0, -d)
	return time.Date(at.Year(), at.Month(), at.Day(), hour, 0, 0, 0, time.UTC)
}

// addEvent writes one raw progress event at a server time; device and playback session may be nil.
func addEvent(t *testing.T, f *vistest.Fixture, profileID int64, itemID string, device, session any, played int64, at time.Time) {
	t.Helper()
	if _, err := f.DB.Write.Exec(`INSERT INTO progress_events (profile_id, item_id, device_id, playback_session_id,
		position_ms, duration_ms, played_ms, seq, state, client_time, server_time)
		VALUES (?, ?, ?, ?, 0, 7200000, ?, 1, 'playing', ?, ?)`,
		profileID, itemID, device, session, played, at.UnixMilli(), at.UnixMilli()); err != nil {
		t.Fatal(err)
	}
}

// condensedDays reads progress_days as "<UTC day> <device> <played_ms> <plays>" lines.
func condensedDays(t *testing.T, f *vistest.Fixture) []string {
	t.Helper()
	rows, err := f.DB.Read.Query(`SELECT day, device_id, played_ms, plays FROM progress_days ORDER BY day, device_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var day, played, plays int64
		var device string
		if err := rows.Scan(&day, &device, &played, &plays); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s %q %d %d", time.UnixMilli(day*dayMs).UTC().Format(time.DateOnly), device, played, plays))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// rawEventDays reads the UTC days that still have raw events.
func rawEventDays(t *testing.T, f *vistest.Fixture) []string {
	t.Helper()
	rows, err := f.DB.Read.Query(`SELECT DISTINCT server_time / 86400000 FROM progress_events ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var day int64
		if err := rows.Scan(&day); err != nil {
			t.Fatal(err)
		}
		out = append(out, time.UnixMilli(day*dayMs).UTC().Format(time.DateOnly))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// seedCondense writes raw events around the 90-day boundary: three days well past it (one with two
// playback sessions and an event without one, one on a second device, one without a device) and two
// days inside the retention window, among them the day the boundary falls in.
func seedCondense(t *testing.T, f *vistest.Fixture) {
	t.Helper()
	m := f.Rated(12)
	const tv, phone = "tv000000000000000000000000000000", "phone000000000000000000000000000"
	day := func(d, hour int) time.Time { return condenseAt(d, hour) }

	// 200 days ago: one playback session with two events, a second session, and an event without one.
	addEvent(t, f, f.P18.ID, m.ID, tv, "01J9PS00000000000000000001", 600_000, day(200, 9))
	addEvent(t, f, f.P18.ID, m.ID, tv, "01J9PS00000000000000000001", 300_000, day(200, 10))
	addEvent(t, f, f.P18.ID, m.ID, tv, "01J9PS00000000000000000002", 120_000, day(200, 20))
	addEvent(t, f, f.P18.ID, m.ID, tv, nil, 60_000, day(200, 21))
	// 150 days ago: a second device of the same profile and title, and an event without a device.
	addEvent(t, f, f.P18.ID, m.ID, phone, "01J9PS00000000000000000003", 90_000, day(150, 8))
	addEvent(t, f, f.P18.ID, m.ID, nil, nil, 30_000, day(150, 9))
	// 91 days ago: the last whole day outside the retention window.
	addEvent(t, f, f.P16.ID, m.ID, tv, "01J9PS00000000000000000004", 45_000, day(91, 12))
	// 90 days ago: the day the boundary falls in, 11:30 UTC; both events stay raw.
	addEvent(t, f, f.P16.ID, m.ID, tv, "01J9PS00000000000000000005", 15_000, day(90, 2))
	addEvent(t, f, f.P16.ID, m.ID, tv, "01J9PS00000000000000000005", 25_000, day(90, 20))
	// Yesterday: well inside the window.
	addEvent(t, f, f.P16.ID, m.ID, tv, "01J9PS00000000000000000006", 35_000, day(1, 12))
}

// M10, plan 3.2: raw events older than 90 days become day rows per profile, title and device, their
// raw rows go, and everything inside the window stays untouched. Only whole UTC days are condensed,
// so the day the boundary falls in keeps both its events.
func TestCondenseProgressKeepsTheRetentionWindowRaw(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return condenseNow }), WithTokenKey(testKey))
	seedCondense(t, f)

	removed, err := s.CondenseProgress(context.Background(), condenseNow)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 7 {
		t.Errorf("removed %d raw events, want the 7 of the three days outside the window", removed)
	}
	// 2026-04-06 is 200 days before 2026-10-23, 2026-05-26 is 150 days, 2026-07-24 is 91 days.
	want := []string{
		`2026-04-06 "tv000000000000000000000000000000" 1080000 3`, // two sessions and one event without
		`2026-05-26 "" 30000 1`,                                   // the event without a device
		`2026-05-26 "phone000000000000000000000000000" 90000 1`,
		`2026-07-24 "tv000000000000000000000000000000" 45000 1`,
	}
	if got := condensedDays(t, f); !slices.Equal(got, want) {
		t.Errorf("condensed days\n got %q\nwant %q", got, want)
	}
	// The day the boundary falls in (2026-07-25) and yesterday keep their raw events.
	if got, want := rawEventDays(t, f), []string{"2026-07-25", "2026-10-22"}; !slices.Equal(got, want) {
		t.Errorf("raw days left %q, want %q", got, want)
	}
	if n := countRaw(t, f); n != 3 {
		t.Errorf("%d raw events left, want the 3 inside the window", n)
	}
}

// Plan 3.6: a second run condenses nothing twice. It finds no whole day outside the window any
// more, removes nothing and leaves every day row as it was.
func TestCondenseProgressIsIdempotent(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return condenseNow }), WithTokenKey(testKey))
	seedCondense(t, f)
	ctx := context.Background()
	if _, err := s.CondenseProgress(ctx, condenseNow); err != nil {
		t.Fatal(err)
	}
	before, raw := condensedDays(t, f), countRaw(t, f)

	removed, err := s.CondenseProgress(ctx, condenseNow)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Errorf("the second run removed %d raw events, want none", removed)
	}
	if got := condensedDays(t, f); !slices.Equal(got, before) {
		t.Errorf("the second run changed the day rows\n got %q\nwant %q", got, before)
	}
	if n := countRaw(t, f); n != raw {
		t.Errorf("the second run changed the raw events: %d, want %d", n, raw)
	}
}

// An event uploaded for an already condensed day (the offline device of the known edge case in
// migration 00019) is added to the stored day row instead of starting a second one, and its raw row
// goes with the next run.
func TestCondenseProgressAddsALateEventToItsDay(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return condenseNow }), WithTokenKey(testKey))
	seedCondense(t, f)
	ctx := context.Background()
	if _, err := s.CondenseProgress(ctx, condenseNow); err != nil {
		t.Fatal(err)
	}

	m := f.Rated(12)
	const tv = "tv000000000000000000000000000000"
	addEvent(t, f, f.P18.ID, m.ID, tv, "01J9PS00000000000000000009", 240_000, condenseAt(200, 15))
	removed, err := s.CondenseProgress(ctx, condenseNow)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("removed %d raw events, want the one late upload", removed)
	}
	days := condensedDays(t, f)
	if want := `2026-04-06 "tv000000000000000000000000000000" 1320000 4`; len(days) == 0 || days[0] != want {
		t.Errorf("day row after the late upload %q, want %q", days, want)
	}
	if len(days) != 4 {
		t.Errorf("%d day rows after the late upload, want 4", len(days))
	}
}

// Deleting the profile takes its condensed days with it (ON DELETE CASCADE, like progress_events).
func TestCondenseProgressDaysFollowTheProfile(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return condenseNow }), WithTokenKey(testKey))
	seedCondense(t, f)
	if _, err := s.CondenseProgress(context.Background(), condenseNow); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.Exec(`DELETE FROM profiles WHERE id = ?`, f.P18.ID); err != nil {
		t.Fatal(err)
	}
	for _, line := range condensedDays(t, f) {
		if line == `2026-04-06 "tv000000000000000000000000000000" 1080000 3` {
			t.Errorf("the days of the deleted profile survived: %q", line)
		}
	}
}

func countRaw(t *testing.T, f *vistest.Fixture) int64 {
	t.Helper()
	var n int64
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
