package playback

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"
	"time"
	_ "time/tzdata" // Europe/Zurich for the DST case of OfflineWatchTime

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

const offlineDevice = "dddddddddddddddddddddddddddddddd"

// offlineID is a valid ULID for playback and download ids of the tests.
func offlineID(n int) string { return fmt.Sprintf("01JB%022d", n) }

// offlineSession is a browser session of p on the test device.
func offlineSession(t *testing.T, f *vistest.Fixture, p model.Profile) model.Session {
	t.Helper()
	s := f.Session(t, p)
	s.DeviceID = offlineDevice
	return s
}

// offlineEvent is a playing event of profile p for movie m at device time at.
func offlineEvent(p model.Profile, m vistest.Movie, playback string, seq int64, at time.Time, positionMs, playedMs int64) model.OfflineEvent {
	return model.OfflineEvent{
		ProfileID: p.ID, ItemID: m.ID, PlaybackID: playback, MediaFileID: m.FileID, PositionMs: positionMs,
		DurationMs: vistest.DurationMs, PlayedMs: playedMs, Seq: seq, State: "playing", ClientTime: at,
	}
}

// playbackRun is n events of 10 s each from device time start (the first at start + 10 s), with
// the position advancing from fromMs.
func playbackRun(p model.Profile, m vistest.Movie, playback string, start time.Time, fromMs int64, n int) []model.OfflineEvent {
	out := make([]model.OfflineEvent, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, offlineEvent(p, m, playback, int64(i), start.Add(time.Duration(i)*10*time.Second), fromMs+int64(i)*10_000, 10_000))
	}
	return out
}

// storedEvent is a row of progress_events.
type storedEvent struct {
	seq, serverTime, clientTime, played, offline int64
	uploadedAt                                   sql.NullInt64
	device, playback, download, mediaFile        sql.NullString
}

func storedEvents(t *testing.T, f *vistest.Fixture, profileID int64, itemID string) []storedEvent {
	t.Helper()
	rows, err := f.DB.Read.Query(`SELECT seq, server_time, client_time, played_ms, offline, uploaded_at, device_id, playback_session_id,
		download_id, media_file_id FROM progress_events WHERE profile_id = ? AND item_id = ? ORDER BY server_time, seq`, profileID, itemID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []storedEvent
	for rows.Next() {
		var e storedEvent
		if err := rows.Scan(&e.seq, &e.serverTime, &e.clientTime, &e.played, &e.offline, &e.uploadedAt, &e.device, &e.playback, &e.download, &e.mediaFile); err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func progressOf(t *testing.T, f *vistest.Fixture, profileID int64, itemID string) appq.Progress {
	t.Helper()
	p, err := appq.New(f.DB.Read).GetProgress(context.Background(), appq.GetProgressParams{ProfileID: profileID, ItemID: itemID})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newOfflineService(t *testing.T, at time.Time) (*vistest.Fixture, *Service, *manualClock) {
	t.Helper()
	f := vistest.New(t)
	clock := &manualClock{now: at}
	return f, New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTokenKey(testKey)), clock
}

// O-5: the device clock runs 2 h behind; the server shifts every event by the offset of sentAt,
// caps a time after its own clock at now and stores the events as offline events of the session's
// device, with the playback id, the download and the device's own clock time.
func TestReportOfflineCorrectsDeviceClock(t *testing.T) {
	now := time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)
	f, s, _ := newOfflineService(t, now)
	ctx := context.Background()
	m := f.Rated(12)
	device := now.Add(-2 * time.Hour)
	pb, dl := offlineID(1), offlineID(900)
	events := []model.OfflineEvent{
		offlineEvent(f.P18, m, pb, 1, device.Add(-50*time.Minute), 600_000, 10_000),
		offlineEvent(f.P18, m, pb, 2, device.Add(-50*time.Minute+10*time.Second), 610_000, 10_000),
		offlineEvent(f.P18, m, pb, 3, device.Add(5*time.Minute), 620_000, 10_000), // after sentAt: capped at now
	}
	for i := range events {
		events[i].DownloadID = dl
	}
	res, err := s.ReportOffline(ctx, offlineSession(t, f, f.P18), device, events)
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted != 3 || res.Duplicates != 0 || res.Rejected != 0 || !slices.Equal(res.Results, []string{"accepted", "accepted", "accepted"}) {
		t.Fatalf("result = %+v", res)
	}
	got := storedEvents(t, f, f.P18.ID, m.ID)
	wantServer := []time.Time{now.Add(-50 * time.Minute), now.Add(-50*time.Minute + 10*time.Second), now}
	if len(got) != 3 {
		t.Fatalf("stored %d events", len(got))
	}
	for i, e := range got {
		if e.seq != int64(i+1) || e.serverTime != wantServer[i].UnixMilli() || e.clientTime != events[i].ClientTime.UnixMilli() {
			t.Errorf("event %d: seq %d, server %v, client %v", i, e.seq, time.UnixMilli(e.serverTime).In(zurich), time.UnixMilli(e.clientTime).In(zurich))
		}
		if e.offline != 1 || e.uploadedAt.Int64 != now.UnixMilli() || e.device.String != offlineDevice || e.playback.String != pb ||
			e.download.String != dl || e.mediaFile.String != m.FileID {
			t.Errorf("event %d: %+v", i, e)
		}
	}
	if p := progressOf(t, f, f.P18.ID, m.ID); p.PositionMs != 620_000 || p.UpdatedAt != now.UnixMilli() || p.Completed != 0 {
		t.Errorf("progress = %+v", p)
	}
}

// Events are processed in the order of their corrected times, results come in the order of the
// request; the played time counts at most 11 s for the first event of a playback and 110 % of the
// time since the previous one; an event uploaded again (also within one batch) is a duplicate.
func TestReportOfflineOrderDuplicatesAndPlayedBound(t *testing.T) {
	now := time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)
	f, s, clock := newOfflineService(t, now)
	ctx := context.Background()
	m := f.Rated(16)
	session := offlineSession(t, f, f.P18)
	pb := offlineID(2)
	t0 := now.Add(-time.Hour)
	batch := []model.OfflineEvent{
		offlineEvent(f.P16, m, pb, 3, t0.Add(30*time.Second), 30_000, 5_000),
		offlineEvent(f.P16, m, pb, 1, t0, 10_000, 3_600_000),                  // claims an hour: 11 s
		offlineEvent(f.P16, m, pb, 2, t0.Add(20*time.Second), 20_000, 60_000), // 20 s after seq 1: 22 s
		offlineEvent(f.P16, m, pb, 1, t0, 10_000, 3_600_000),                  // the same event twice
	}
	res, err := s.ReportOffline(ctx, session, now, batch)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"accepted", "accepted", "accepted", "duplicate"}; !slices.Equal(res.Results, want) || res.Accepted != 3 || res.Duplicates != 1 {
		t.Fatalf("result = %+v, want %v", res, want)
	}
	got := storedEvents(t, f, f.P16.ID, m.ID)
	if len(got) != 3 {
		t.Fatalf("stored %d events", len(got))
	}
	for i, want := range []int64{11_000, 22_000, 5_000} {
		if got[i].seq != int64(i+1) || got[i].played != want {
			t.Errorf("seq %d played %d ms, want %d", got[i].seq, got[i].played, want)
		}
	}
	// The newest event wins although it came first in the request.
	if p := progressOf(t, f, f.P16.ID, m.ID); p.PositionMs != 30_000 || p.UpdatedAt != t0.Add(30*time.Second).UnixMilli() {
		t.Errorf("progress = %+v", p)
	}

	// The same batch again (a retry after a lost answer): nothing new is stored or counted.
	clock.Advance(time.Minute)
	res, err = s.ReportOffline(ctx, session, clock.Now(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted != 0 || res.Duplicates != 4 || res.Rejected != 0 {
		t.Errorf("result of the retry = %+v", res)
	}
	if ms := sumPlayed(t, f, f.P16.ID); ms != 38_000 {
		t.Errorf("played after the retry = %d ms, want 38000", ms)
	}
}

// Events of a profile outside the session's account, of a title the profile may not see and with
// invalid fields are rejected and not stored; a guest uploads only for its own profiles. An empty
// or oversized batch and a missing sentAt are validation errors.
func TestReportOfflineRejects(t *testing.T) {
	now := time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)
	f, s, _ := newOfflineService(t, now)
	f.AddGuest(t)
	ctx := context.Background()
	at := now.Add(-time.Minute)
	film6, film16 := f.Rated(6), f.Rated(16)
	valid := offlineEvent(f.P6, film6, offlineID(3), 1, at, 10_000, 10_000)
	invalid := func(change func(*model.OfflineEvent)) model.OfflineEvent {
		ev := valid
		ev.Seq = 100
		change(&ev)
		return ev
	}
	batch := []model.OfflineEvent{
		valid,
		offlineEvent(f.P6, film16, offlineID(4), 1, at, 10_000, 10_000), // FSK 16 for P6
		offlineEvent(f.PG, film6, offlineID(5), 1, at, 10_000, 10_000),  // the guest's profile
		offlineEvent(model.Profile{ID: 99_999}, film6, offlineID(6), 1, at, 10_000, 10_000),
		invalid(func(ev *model.OfflineEvent) { ev.ItemID = "01J0000000000000000000XXXX" }),
		invalid(func(ev *model.OfflineEvent) { ev.PlaybackID = "" }),
		invalid(func(ev *model.OfflineEvent) { ev.PlaybackID = "not-a-ulid" }),
		invalid(func(ev *model.OfflineEvent) { ev.DownloadID = "not-a-ulid" }),
		invalid(func(ev *model.OfflineEvent) { ev.DurationMs = 0 }),
		invalid(func(ev *model.OfflineEvent) { ev.PositionMs = -1 }),
		invalid(func(ev *model.OfflineEvent) { ev.PlayedMs = -1 }),
		invalid(func(ev *model.OfflineEvent) { ev.Seq = -1 }),
		invalid(func(ev *model.OfflineEvent) { ev.State = "stopped" }),
		invalid(func(ev *model.OfflineEvent) { ev.ClientTime = time.Time{} }),
		invalid(func(ev *model.OfflineEvent) { ev.MediaFileID = string(make([]byte, 65)) }),
	}
	res, err := s.ReportOffline(ctx, offlineSession(t, f, f.P18), now, batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted != 1 || res.Rejected != len(batch)-1 || res.Results[0] != "accepted" {
		t.Fatalf("result = %+v", res)
	}
	for i, r := range res.Results[1:] {
		if r != "rejected" {
			t.Errorf("event %d: %s, want rejected", i+1, r)
		}
	}
	if got := storedEvents(t, f, f.P6.ID, film6.ID); len(got) != 1 || got[0].seq != 1 {
		t.Errorf("stored events of P6 = %+v", got)
	}
	for _, c := range []struct {
		p model.Profile
		m vistest.Movie
	}{{f.P6, film16}, {f.PG, film6}} {
		if got := storedEvents(t, f, c.p.ID, c.m.ID); len(got) != 0 {
			t.Errorf("rejected events of profile %s stored: %+v", c.p.Name, got)
		}
	}

	// The guest: its own profiles count, the owner's are rejected; FSK 12 is its ceiling.
	guest := offlineSession(t, f, f.PG)
	res, err = s.ReportOffline(ctx, guest, now, []model.OfflineEvent{
		offlineEvent(f.PG18, film6, offlineID(7), 1, at, 10_000, 10_000),
		offlineEvent(f.P18, film6, offlineID(8), 1, at, 10_000, 10_000),
		offlineEvent(f.PG18, film16, offlineID(9), 1, at, 10_000, 10_000),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Results, []string{"accepted", "rejected", "rejected"}) {
		t.Errorf("guest results = %v", res.Results)
	}
	if got := storedEvents(t, f, f.P18.ID, film6.ID); len(got) != 0 {
		t.Errorf("the guest stored events for the owner: %+v", got)
	}

	tooMany := make([]model.OfflineEvent, MaxOfflineEvents+1)
	for i := range tooMany {
		tooMany[i] = offlineEvent(f.P6, film6, offlineID(10), int64(i), at, 0, 0)
	}
	for name, tc := range map[string]struct {
		sentAt      time.Time
		events      []model.OfflineEvent
		field, code string
	}{
		"empty":    {now, nil, "events", "required"},
		"too many": {now, tooMany, "events", "too_long"},
		"sentAt":   {time.Time{}, []model.OfflineEvent{valid}, "sentAt", "required"},
	} {
		if _, err := s.ReportOffline(ctx, offlineSession(t, f, f.P18), tc.sentAt, tc.events); !hasField(err, tc.field, tc.code) {
			t.Errorf("%s: %v, want %s %s", name, err, tc.field, tc.code)
		}
	}
	if n := countEvents(t, f, f.P6.ID, film6.ID); n != 1 {
		t.Errorf("validation errors stored events: %d", n)
	}
}

// AN-10: per profile and title the event with the latest (corrected) time wins over the stored
// progress, online or offline; an older offline event only books watch time; "completed" stays
// until a newer event below 90 % comes.
func TestReportOfflineMergesByTime(t *testing.T) {
	t18 := time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)
	f, s, clock := newOfflineService(t, t18)
	ctx := context.Background()
	m := f.Rated(12)
	minute := time.Minute.Milliseconds()
	if _, err := s.Report(ctx, f.P12, m.ID, model.ProgressEvent{PositionMs: 50 * minute, DurationMs: vistest.DurationMs, PlayedMs: 0,
		Seq: 1, State: "playing", ClientTime: t18}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Hour) // the device comes back at 19:00 with a correct clock
	session := offlineSession(t, f, f.P18)
	upload := func(seq int64, at time.Time, positionMs int64) {
		t.Helper()
		res, err := s.ReportOffline(ctx, session, clock.Now(), []model.OfflineEvent{offlineEvent(f.P12, m, offlineID(int(seq)), 1, at, positionMs, 10_000)})
		if err != nil || res.Accepted != 1 {
			t.Fatalf("upload %d: %+v, %v", seq, res, err)
		}
	}
	check := func(label string, positionMs int64, completed bool, updated time.Time) {
		t.Helper()
		p := progressOf(t, f, f.P12.ID, m.ID)
		if p.PositionMs != positionMs || (p.Completed == 1) != completed || p.UpdatedAt != updated.UnixMilli() {
			t.Errorf("%s: progress = %+v, want position %d, completed %v, updated %v", label, p, positionMs, completed, updated)
		}
	}

	upload(1, t18.Add(-30*time.Minute), 10*minute) // older than the online event
	check("older offline event", 50*minute, false, t18)
	if ms := sumPlayed(t, f, f.P12.ID); ms != 10_000 {
		t.Errorf("the older offline event booked %d ms, want 10000", ms)
	}
	upload(2, t18.Add(30*time.Minute), 20*minute) // newer: wins
	check("newer offline event", 20*minute, false, t18.Add(30*time.Minute))
	upload(3, t18.Add(40*time.Minute), 115*minute) // newer and beyond 90 %: completed
	check("completed", 115*minute, true, t18.Add(40*time.Minute))
	upload(4, t18.Add(35*time.Minute), 30*minute) // older and below 90 %: completed stays
	check("older event below 90 %", 115*minute, true, t18.Add(40*time.Minute))
	upload(5, t18.Add(50*time.Minute), 30*minute) // newer and below 90 %: open again
	check("newer event below 90 %", 30*minute, false, t18.Add(50*time.Minute))
	if ms := sumPlayed(t, f, f.P12.ID); ms != 50_000 {
		t.Errorf("played = %d ms, want 50000 (every event counts)", ms)
	}

	// An online event after the upload wins again: its server time is later than every
	// corrected time.
	clock.Advance(time.Minute)
	if _, err := s.Report(ctx, f.P12, m.ID, model.ProgressEvent{PositionMs: 60 * minute, DurationMs: vistest.DurationMs,
		Seq: 2, State: "paused", ClientTime: clock.Now()}); err != nil {
		t.Fatal(err)
	}
	check("online event after the upload", 60*minute, false, clock.Now())
}

// AN-18 and K-5: offline watch time is booked on the day of the corrected time, so 45 minutes of
// yesterday leave today's limit alone, and 31 minutes of today end the running stream of the
// profile (stopped_by limit). The watch-time report shows both days, including the overrun.
func TestReportOfflineBooksWatchTime(t *testing.T) {
	now := time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)
	f, s, _ := newOfflineService(t, now)
	ctx := context.Background()
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 30})
	m := f.Rated(6)
	session := offlineSession(t, f, kid)
	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	behind := -2 * time.Hour // the device clock
	sentAt := now.Add(behind)

	yesterday := time.Date(2026, 10, 19, 20, 0, 0, 0, zurich).Add(behind)
	res, err := s.ReportOffline(ctx, session, sentAt, playbackRun(kid, m, offlineID(20), yesterday, 0, 270))
	if err != nil || res.Accepted != 270 {
		t.Fatalf("yesterday: %+v, %v", res, err)
	}
	if by := stoppedBy(t, f, info.PlaybackSessionID); by != "" {
		t.Fatalf("yesterday's 45 minutes ended today's stream: %q", by)
	}
	if wt, err := s.WatchTime(ctx, kid, now); err != nil || wt.RemainingMinutes != 30 {
		t.Fatalf("watch time today after yesterday's upload = %+v, %v", wt, err)
	}

	today := time.Date(2026, 10, 20, 16, 0, 0, 0, zurich).Add(behind)
	res, err = s.ReportOffline(ctx, session, sentAt, playbackRun(kid, m, offlineID(21), today, 0, 186))
	if err != nil || res.Accepted != 186 {
		t.Fatalf("today: %+v, %v", res, err)
	}
	if by := stoppedBy(t, f, info.PlaybackSessionID); by != model.StoppedByLimit {
		t.Errorf("stopped_by after crossing the limit = %q, want limit", by)
	}
	if wt, err := s.WatchTime(ctx, kid, now); err != nil || *wt != (model.WatchTime{LimitMinutes: 30, UsedMinutes: 31}) {
		t.Errorf("watch time today = %+v, %v", wt, err)
	}
	rep, err := s.WatchReport(ctx, browserSession(t, f, f.P18), kid.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []model.WatchDay{{Date: "2026-10-19", Minutes: 45}, {Date: "2026-10-20", Minutes: 31}}; !slices.Equal(rep.Days, want) {
		t.Errorf("report = %+v, want %+v", rep.Days, want)
	}
}

// AN-18: the device takes the remaining minutes of today and the next midnight of the server's
// time zone along; after midnight the full limit applies again. A profile without limit gets nil.
func TestOfflineWatchTime(t *testing.T) {
	now := time.Date(2026, 10, 20, 23, 59, 30, 0, zurich)
	f, s, clock := newOfflineService(t, now)
	ctx := context.Background()
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 30})
	if wt, err := s.OfflineWatchTime(ctx, f.P18, now); err != nil || wt != nil {
		t.Fatalf("profile without limit: %+v, %v", wt, err)
	}
	if _, err := f.DB.Write.Exec(`INSERT INTO progress_events (profile_id, item_id, position_ms, duration_ms, played_ms, seq, state, client_time, server_time)
		VALUES (?, ?, 0, 1000, 600000, 1, 'playing', ?, ?)`, kid.ID, f.Rated(6).ID, now.Add(-time.Hour).UnixMilli(), now.Add(-time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	wt, err := s.OfflineWatchTime(ctx, kid, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if wt.LimitMinutes != 30 || wt.RemainingMinutes != 20 || !wt.ResetsAt.Equal(time.Date(2026, 10, 21, 0, 0, 0, 0, zurich)) {
		t.Errorf("before midnight = %+v", wt)
	}
	clock.Advance(time.Minute)
	wt, err = s.OfflineWatchTime(ctx, kid, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if wt.RemainingMinutes != 30 || !wt.ResetsAt.Equal(time.Date(2026, 10, 22, 0, 0, 0, 0, zurich)) {
		t.Errorf("after midnight = %+v", wt)
	}

	// The day the clocks go back has 25 hours: the reset is still the next local midnight.
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	wt, err = s.OfflineWatchTime(ctx, kid, time.Date(2026, 10, 25, 1, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 26, 0, 0, 0, 0, loc); !wt.ResetsAt.Equal(want) || wt.ResetsAt.Sub(time.Date(2026, 10, 25, 0, 0, 0, 0, loc)) != 25*time.Hour {
		t.Errorf("reset on the DST day = %v, want %v", wt.ResetsAt, want)
	}
}
