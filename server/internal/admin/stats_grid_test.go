package admin

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Concept 12 (contract 1.8.0, section 11.4): the watch time per quarter hour of the week.

// StatsWatchTimeByQuarter sums the played time per UTC quarter hour of the raw events in
// [from_ms, to_ms): the first and the last millisecond of a quarter hour land in the same row, the
// next millisecond in the next one, and nothing outside the period counts.
func TestStatsWatchTimeByQuarterQuery(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	seedStats(t, st.db.Write) // the rows the events refer to; its own events lie before the period
	start := time.Date(2026, 10, 21, 13, 0, 0, 0, time.UTC)
	for _, e := range []struct {
		at     time.Duration
		played int64
	}{
		{-time.Millisecond, 1},                   // before the period
		{0, 100},                                 // quarter 0
		{15*time.Minute - time.Millisecond, 200}, // last millisecond of quarter 0
		{15 * time.Minute, 400},                  // quarter 1
		{time.Hour - time.Millisecond, 800},      // quarter 3
		{time.Hour, 1600},                        // the end of the period is open
	} {
		at := start.Add(e.at).UnixMilli()
		if _, err := st.db.Write.Exec(`INSERT INTO progress_events (profile_id, item_id, position_ms, duration_ms, played_ms, seq,
			state, client_time, server_time) VALUES (1, '01J9000000000000000000ST01', 0, 7200000, ?, 1, 'playing', ?, ?)`, e.played, at, at); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := st.svc.sys.StatsWatchTimeByQuarter(ctx, systemq.StatsWatchTimeByQuarterParams{
		FromMs: start.UnixMilli(), ToMs: start.Add(time.Hour).UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	q0 := start.UnixMilli() / quarterMs
	want := []systemq.StatsWatchTimeByQuarterRow{{Quarter: q0, WatchedMs: 300}, {Quarter: q0 + 1, WatchedMs: 400}, {Quarter: q0 + 3, WatchedMs: 800}}
	if !slices.Equal(rows, want) {
		t.Errorf("quarters %+v, want %+v", rows, want)
	}
}

// weekGrid maps each UTC quarter hour to its local quarter hour of the week (Monday 00:00 = 0):
// Europe/Zurich across both switches of 2026 (29 March: 02:00 to 02:59 does not exist; 25 October:
// 02:00 to 02:59 happens twice and counts into the same slots) and Asia/Kolkata (UTC+05:30).
func TestWeekGridZones(t *testing.T) {
	kolkata := mustZone("Asia/Kolkata")
	quarter := func(utc string) int64 {
		at, err := time.Parse(time.RFC3339, utc)
		if err != nil {
			t.Fatal(err)
		}
		return at.UnixMilli() / quarterMs
	}
	slot := func(weekday, hour, minute int) int { return weekday*96 + hour*4 + minute/15 } // weekday 0 = Monday
	const sunday, monday, tuesday = 6, 0, 1
	for _, c := range []struct {
		name string
		loc  *time.Location
		utc  []string
		want map[int]int64 // slot → watched ms, every other slot 0
	}{
		{"Zurich before the spring switch", zurich, []string{"2026-03-29T00:45:00Z"}, map[int]int64{slot(sunday, 1, 45): 1}},
		{"Zurich after the spring switch", zurich, []string{"2026-03-29T01:00:00Z"}, map[int]int64{slot(sunday, 3, 0): 1}},
		{"Zurich autumn: 02:30 twice", zurich, []string{"2026-10-25T00:30:00Z", "2026-10-25T01:30:00Z"}, map[int]int64{slot(sunday, 2, 30): 2}},
		{"Zurich Monday 00:00 is the first value", zurich, []string{"2026-10-25T23:00:00Z"}, map[int]int64{0: 1}},
		{"Zurich Sunday 23:45 is the last value", zurich, []string{"2026-10-25T22:45:00Z"}, map[int]int64{model.StatsQuarterHours - 1: 1}},
		{"Zurich summer time", zurich, []string{"2026-10-20T12:00:00Z"}, map[int]int64{slot(tuesday, 14, 0): 1}},
		{"Kolkata early Monday", kolkata, []string{"2026-11-02T00:15:00Z"}, map[int]int64{slot(monday, 5, 45): 1}},
		{"Kolkata Monday 23:45", kolkata, []string{"2026-11-02T18:15:00Z"}, map[int]int64{slot(monday, 23, 45): 1}},
		{"Kolkata Tuesday 00:00", kolkata, []string{"2026-11-02T18:30:00Z"}, map[int]int64{slot(tuesday, 0, 0): 1}},
	} {
		rows := make([]systemq.StatsWatchTimeByQuarterRow, 0, len(c.utc))
		for _, u := range c.utc {
			rows = append(rows, systemq.StatsWatchTimeByQuarterRow{Quarter: quarter(u), WatchedMs: 1})
		}
		got := weekGrid(rows, c.loc)
		if len(got) != model.StatsQuarterHours {
			t.Fatalf("%s: %d values, want %d", c.name, len(got), model.StatsQuarterHours)
		}
		for i, v := range got {
			if v != c.want[i] {
				t.Errorf("%s: slot %d (day %d %02d:%02d) = %d, want %d", c.name, i, i/96, i%96/4, i%4*15, v, c.want[i])
			}
		}
	}
	// No event of the spring switch lands in the hour that does not exist.
	var spring []systemq.StatsWatchTimeByQuarterRow
	for q := quarter("2026-03-28T22:00:00Z"); q < quarter("2026-03-29T04:00:00Z"); q++ {
		spring = append(spring, systemq.StatsWatchTimeByQuarterRow{Quarter: q, WatchedMs: 1})
	}
	got := weekGrid(spring, zurich)
	for m := 0; m < 60; m += 15 {
		if v := got[slot(sunday, 2, m)]; v != 0 {
			t.Errorf("Sunday 02:%02d on 29 March = %d, want 0", m, v)
		}
	}
	if got[slot(sunday, 1, 45)] != 1 || got[slot(sunday, 3, 0)] != 1 {
		t.Errorf("around the spring gap: 01:45 = %d, 03:00 = %d, want 1 and 1", got[slot(sunday, 1, 45)], got[slot(sunday, 3, 0)])
	}
}

// The window of the grid is min(days, 90) in whole weeks: a year and 90 days count the last 84 days
// (the fixture's events 100 and 400 days back stay out), 30 days the last 28 (the events 30 and 40
// days back stay out), a week the week. The zone pinned with WithLocation decides the slot and names
// itself.
func TestStatsTimeWindow(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.svc.now = func() time.Time { return statsNow }
	seedStats(t, st.db.Write)
	sum := func(grid []int64) (n int64) {
		for _, v := range grid {
			n += v
		}
		return n
	}
	// statsNow - 1 day is Tuesday, 20 October 2026, 12:00 UTC: 14:00 in Zurich, 17:30 in Kolkata,
	// with 600000 + 300000 + 0 (one play in that quarter hour) and the offline upload of 400000.
	for _, c := range []struct {
		loc           *time.Location
		days, window  int
		sum           int64
		slot          int
		slotWatchedMs int64
	}{
		{zurich, 365, 84, 3_820_000, 96 + 14*4, 1_300_000},
		{zurich, 90, 84, 3_820_000, 96 + 14*4, 1_300_000},
		{zurich, 30, 28, 2_850_000, 96 + 14*4, 1_300_000},
		{mustZone("Asia/Kolkata"), 7, 7, 2_800_000, 96 + 17*4 + 2, 1_300_000},
	} {
		st.svc.WithLocation(c.loc)
		got, err := st.svc.Stats(ctx, c.days)
		if err != nil {
			t.Fatal(err)
		}
		if got.TimeWindowDays != c.window || got.TimeZone != c.loc.String() || len(got.WatchTimeByQuarterHour) != model.StatsQuarterHours ||
			sum(got.WatchTimeByQuarterHour) != c.sum || got.WatchTimeByQuarterHour[c.slot] != c.slotWatchedMs {
			t.Errorf("%s, %d days: window %d, zone %q, %d values summing to %d, slot %d = %d; want %d, %d, slot %d",
				c.loc, c.days, got.TimeWindowDays, got.TimeZone, len(got.WatchTimeByQuarterHour), sum(got.WatchTimeByQuarterHour),
				c.slot, got.WatchTimeByQuarterHour[c.slot], c.window, c.sum, c.slotWatchedMs)
		}
	}
}

// ResolveZone names the zone of the server once by its IANA name and loads the grid's location from
// that name, so both agree: TZ wins (with or without the leading ":" of POSIX, also as an absolute
// path into a zoneinfo tree), else the target of the /etc/localtime link (also below posix/ or
// right/); an invalid TZ, a missing link or a copied zone file (no name) is UTC.
func TestResolveZone(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "usr", "share", "zoneinfo")
	for _, zone := range []string{"Europe/Zurich", "posix/Asia/Kolkata", "right/America/New_York"} {
		if err := os.MkdirAll(filepath.Join(tree, filepath.Dir(zone)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tree, zone), []byte("TZif"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := func(name, target string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	zurichLink := link("localtime-zurich", filepath.Join(tree, "Europe/Zurich"))
	posixLink := link("localtime-posix", filepath.Join(tree, "posix/Asia/Kolkata"))
	rightLink := link("localtime-right", filepath.Join(tree, "right/America/New_York"))
	copied := filepath.Join(dir, "localtime-copy")
	if err := os.WriteFile(copied, []byte("TZif"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, tz, localtime, want string
	}{
		{"TZ by name", "Europe/Zurich", posixLink, "Europe/Zurich"},
		{"TZ with POSIX colon", ":Europe/Zurich", posixLink, "Europe/Zurich"},
		{"TZ as absolute path", "/usr/share/zoneinfo/Asia/Kolkata", zurichLink, "Asia/Kolkata"},
		{"TZ as absolute posix path", ":/usr/share/zoneinfo/posix/Asia/Kolkata", zurichLink, "Asia/Kolkata"},
		{"TZ path outside zoneinfo", "/etc/localtime", zurichLink, "UTC"},
		{"invalid TZ", "Mars/Olympus", zurichLink, "UTC"},
		{"TZ UTC", "UTC", zurichLink, "UTC"},
		{"empty TZ, localtime link", "", zurichLink, "Europe/Zurich"},
		{"empty TZ, link below posix/", "", posixLink, "Asia/Kolkata"},
		{"empty TZ, link below right/", "", rightLink, "America/New_York"},
		{"empty TZ, copied zone file", "", copied, "UTC"},
		{"empty TZ, no localtime", "", filepath.Join(dir, "missing"), "UTC"},
	} {
		getenv := func(key string) string {
			if key == "TZ" {
				return c.tz
			}
			return ""
		}
		loc, name := resolveZone(getenv, c.localtime)
		if name != c.want || loc.String() != c.want {
			t.Errorf("%s: zone %q named %q, want %q for both", c.name, loc, name, c.want)
		}
	}
	// The pinned zone names the statistics.
	st := newStack(t, "0.9.0")
	st.svc.WithLocation(zurich)
	if got, err := st.svc.Stats(context.Background(), 7); err != nil || got.TimeZone != "Europe/Zurich" {
		t.Errorf("pinned zone: %q, %v", got.TimeZone, err)
	}
}

func TestGridWindowDays(t *testing.T) {
	for days, want := range map[int]int{1: 1, 6: 6, 7: 7, 13: 7, 14: 14, 30: 28, 84: 84, 90: 84, 365: 84} {
		if got := gridWindowDays(days); got != want {
			t.Errorf("gridWindowDays(%d) = %d, want %d", days, got, want)
		}
	}
}
