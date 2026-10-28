package admin

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

var statsNow = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

// ago is a server time before statsNow in Unix milliseconds.
func ago(d time.Duration) int64 { return statsNow.Add(-d).UnixMilli() }

const day = 24 * time.Hour

// seedStats writes the fixture of design 3.6: three profiles of two accounts, two devices, four
// titles, six playback sessions and progress events inside and outside the periods (with an
// offline event uploaded late, events without a playback session, without a device and of a
// deleted device).
func seedStats(t *testing.T, db *sql.DB) {
	t.Helper()
	exec(t, db,
		`INSERT INTO accounts (id, email, password_hash, role, created_at, updated_at) VALUES
		   (1, 'owner@example.test', 'x', 'admin', 1, 1), (2, 'guest@example.test', 'x', 'user', 1, 1)`,
		`INSERT INTO profiles (id, account_id, name, max_rating, is_default, created_at, updated_at) VALUES
		   (1, 1, 'Marco', 18, 1, 1, 1), (2, 1, 'Kind', 6, 0, 1, 1), (3, 2, 'Gast', 12, 1, 1, 1)`,
		`INSERT INTO devices (id, created_at, last_seen_at, type) VALUES
		   ('tv000000000000000000000000000000', 1, 1, 'tv'), ('phone000000000000000000000000000', 1, 1, 'phone')`,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'Filme', 'movies', 1, 1)`,
		`INSERT INTO items (id, kind, library_id, title, sort_title, year, added_at, created_at, updated_at) VALUES
		   ('01J9000000000000000000ST01', 'movie', 1, 'parsed one', 'a', 2001, 1, 1, 1),
		   ('01J9000000000000000000ST02', 'movie', 1, 'parsed two', 'b', NULL, 1, 1, 1),
		   ('01J9000000000000000000ST03', 'episode', 1, 'Parsed Three', 'c', 2019, 1, 1, 1),
		   ('01J9000000000000000000ST04', 'movie', 1, 'parsed four', 'd', 1999, 1, 1, 1)`,
		`INSERT INTO item_texts (item_id, lang, title) VALUES
		   ('01J9000000000000000000ST01', 'de', 'Eins'), ('01J9000000000000000000ST01', 'en', 'One'),
		   ('01J9000000000000000000ST02', 'en', 'Two'), ('01J9000000000000000000ST04', 'de', 'Vier')`,
		`INSERT INTO sessions (id, token_hash, account_id, profile_id, created_at, last_seen_at, expires_at) VALUES
		   (1, 'h1', 1, 1, 1, 1, 1), (2, 'h2', 2, 3, 1, 1, 1)`,
	)
	for i, s := range []struct {
		mode     string
		hardware int
		started  int64
	}{
		{"direct", 0, ago(1 * day)}, {"direct", 0, ago(3 * day)}, {"remux", 0, ago(10 * day)},
		{"transcode", 1, ago(2 * day)}, {"transcode", 1, ago(20 * day)}, {"transcode", 0, ago(40 * day)},
	} {
		if _, err := db.Exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode,
			hardware, started_at, last_activity_at, expires_at) VALUES (?, 1, 1, 1, '01J9000000000000000000ST01', 'f', ?, ?, ?, ?, ?)`,
			fmt.Sprintf("01J9PS00000000000000000%03d", i+1), s.mode, s.hardware, s.started, s.started, s.started); err != nil {
			t.Fatal(err)
		}
	}
	const tv, phone, gone = "tv000000000000000000000000000000", "phone000000000000000000000000000", "gone0000000000000000000000000000"
	for _, e := range []struct {
		profile  int64
		item     string
		device   any
		played   int64
		session  any
		server   int64
		clientMs int64
	}{
		// Session 1 (Marco on the TV): three events of one play.
		{1, "ST01", tv, 600_000, "01J9PS00000000000000000001", ago(1 * day), ago(1 * day)},
		{1, "ST01", tv, 300_000, "01J9PS00000000000000000001", ago(1*day - time.Minute), ago(1*day - time.Minute)},
		{1, "ST01", tv, 0, "01J9PS00000000000000000001", ago(1*day - 2*time.Minute), ago(1*day - 2*time.Minute)},
		// Session 2 (Kind on the phone) and an offline event uploaded yesterday, played 60 days ago.
		{2, "ST02", phone, 1_200_000, "01J9PS00000000000000000002", ago(3 * day), ago(3 * day)},
		{2, "ST02", phone, 400_000, nil, ago(1 * day), ago(60 * day)},
		// Gast: events without a playback session count as plays of their own; no device.
		{3, "ST03", nil, 200_000, nil, ago(5 * day), ago(5 * day)},
		{3, "ST03", nil, 100_000, nil, ago(6 * day), ago(6 * day)},
		// A device deleted since: unknown type.
		{3, "ST04", gone, 50_000, "01J9PS00000000000000000003", ago(10 * day), ago(10 * day)},
		// Exactly 30 days ago: inside the 30-day period.
		{1, "ST04", tv, 70_000, "01J9PS00000000000000000005", ago(30 * day), ago(30 * day)},
		// Only in the longer periods.
		{1, "ST02", tv, 900_000, "01J9PS00000000000000000006", ago(40 * day), ago(40 * day)},
		{2, "ST03", phone, 800_000, nil, ago(100 * day), ago(100 * day)},
		{3, "ST01", nil, 5_000_000, nil, ago(400 * day), ago(400 * day)},
		// A title watched for no time at all.
		{2, "ST04", phone, 0, nil, ago(2 * day), ago(2 * day)},
	} {
		if _, err := db.Exec(`INSERT INTO progress_events (profile_id, item_id, device_id, position_ms, duration_ms, played_ms, seq,
			state, client_time, server_time, playback_session_id) VALUES (?, ?, ?, 0, 7200000, ?, 1, 'playing', ?, ?, ?)`,
			e.profile, "01J9000000000000000000"+e.item, e.device, e.played, e.clientMs, e.server, e.session); err != nil {
			t.Fatal(err)
		}
	}
}

// expectedStats computes the statistics from the raw rows in Go, without the queries of
// admin_stats.sql.
func expectedStats(t *testing.T, db *sql.DB, days int) model.Stats {
	t.Helper()
	from, to := statsNow.Add(-time.Duration(days)*day).UnixMilli(), statsNow.UnixMilli()
	type event struct {
		id, profile, played, server int64
		item                        string
		device, session             sql.NullString
	}
	var events []event
	scanAll(t, db, "SELECT id, profile_id, item_id, device_id, played_ms, playback_session_id, server_time FROM progress_events",
		func(rows *sql.Rows) error {
			var e event
			if err := rows.Scan(&e.id, &e.profile, &e.item, &e.device, &e.played, &e.session, &e.server); err != nil {
				return err
			}
			if e.server >= from && e.server <= to {
				events = append(events, e)
			}
			return nil
		})
	lookup := func(query string, args ...any) (string, bool) {
		var v sql.NullString
		err := db.QueryRow(query, args...).Scan(&v)
		if errors.Is(err, sql.ErrNoRows) || err == nil && !v.Valid {
			return "", false
		}
		if err != nil {
			t.Fatal(err)
		}
		return v.String, true
	}
	play := func(e event) string {
		if e.session.Valid {
			return e.session.String
		}
		return fmt.Sprint("event ", e.id)
	}

	out := model.Stats{Days: days, TopTitles: []model.StatTitle{}, WatchTime: []model.StatProfile{}, Devices: []model.StatDevice{}}
	type agg struct {
		watched int64
		plays   map[string]bool
		devices map[string]bool
	}
	group := func(key func(event) string) map[string]*agg {
		m := map[string]*agg{}
		for _, e := range events {
			k := key(e)
			if m[k] == nil {
				m[k] = &agg{plays: map[string]bool{}, devices: map[string]bool{}}
			}
			m[k].watched += e.played
			m[k].plays[play(e)] = true
			if e.device.Valid {
				m[k].devices[e.device.String] = true
			}
		}
		return m
	}
	for item, a := range group(func(e event) string { return e.item }) {
		if a.watched == 0 {
			continue
		}
		title, ok := lookup("SELECT title FROM item_texts WHERE item_id = ? AND lang = 'de'", item)
		if !ok {
			if title, ok = lookup("SELECT title FROM item_texts WHERE item_id = ? AND lang = 'en'", item); !ok {
				title, _ = lookup("SELECT title FROM items WHERE id = ?", item)
			}
		}
		kind, _ := lookup("SELECT kind FROM items WHERE id = ?", item)
		st := model.StatTitle{ItemID: item, Title: title, Kind: kind, Plays: len(a.plays), WatchedMs: a.watched}
		if y, ok := lookup("SELECT year FROM items WHERE id = ?", item); ok {
			var year int
			_, _ = fmt.Sscan(y, &year)
			st.Year = &year
		}
		out.TopTitles = append(out.TopTitles, st)
	}
	slices.SortFunc(out.TopTitles, func(a, b model.StatTitle) int {
		return cmp.Or(cmp.Compare(b.WatchedMs, a.WatchedMs), cmp.Compare(a.ItemID, b.ItemID))
	})
	out.TopTitles = out.TopTitles[:min(len(out.TopTitles), 20)]

	for profile, a := range group(func(e event) string { return fmt.Sprint(e.profile) }) {
		var p model.StatProfile
		_, _ = fmt.Sscan(profile, &p.ProfileID)
		p.ProfileName, _ = lookup("SELECT name FROM profiles WHERE id = ?", p.ProfileID)
		p.AccountEmail, _ = lookup("SELECT a.email FROM accounts a, profiles p WHERE p.id = ? AND a.id = p.account_id", p.ProfileID)
		p.WatchedMs, p.Plays = a.watched, len(a.plays)
		out.WatchTime = append(out.WatchTime, p)
	}
	slices.SortFunc(out.WatchTime, func(a, b model.StatProfile) int {
		return cmp.Or(cmp.Compare(b.WatchedMs, a.WatchedMs), cmp.Compare(a.ProfileID, b.ProfileID))
	})

	for typ, a := range group(func(e event) string {
		if typ, ok := lookup("SELECT type FROM devices WHERE id = ?", e.device.String); e.device.Valid && ok {
			return typ
		}
		return "unknown"
	}) {
		out.Devices = append(out.Devices, model.StatDevice{Type: typ, Count: len(a.devices), WatchedMs: a.watched})
	}
	slices.SortFunc(out.Devices, func(a, b model.StatDevice) int {
		return cmp.Or(cmp.Compare(b.WatchedMs, a.WatchedMs), cmp.Compare(a.Type, b.Type))
	})

	hardware := 0
	scanAll(t, db, "SELECT mode, hardware, started_at FROM playback_sessions", func(rows *sql.Rows) error {
		var mode string
		var hw int
		var started int64
		if err := rows.Scan(&mode, &hw, &started); err != nil {
			return err
		}
		if started < from || started > to {
			return nil
		}
		switch mode {
		case "direct":
			out.Playback.Direct++
		case "remux":
			out.Playback.Remux++
		case "transcode":
			out.Playback.Transcode++
			hardware += hw
		}
		return nil
	})
	if all := out.Playback.Direct + out.Playback.Remux + out.Playback.Transcode; all > 0 {
		out.Playback.TranscodeShare = float64(out.Playback.Transcode) / float64(all)
	}
	if out.Playback.Transcode > 0 {
		out.Playback.HardwareShare = float64(hardware) / float64(out.Playback.Transcode)
	}
	return out
}

// scanAll calls row for every row of query.
func scanAll(t *testing.T, db *sql.DB, query string, row func(*sql.Rows) error) {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := row(rows); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// A-8: the statistics of every period match the raw rows, field by field.
func TestStatsMatchRawData(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.svc.now = func() time.Time { return statsNow }
	seedStats(t, st.db.Write)

	for _, days := range []int{7, 30, 90, 365} {
		got, err := st.svc.Stats(ctx, days)
		if err != nil {
			t.Fatal(err)
		}
		want := expectedStats(t, st.db.Read, days)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("days %d:\n got %+v\nwant %+v", days, got, want)
		}
		t.Logf("days %d: %d titles, %d profiles, playback %+v, %d device types", days, len(got.TopTitles), len(got.WatchTime), got.Playback, len(got.Devices))
	}

	// Hand-checked values of the 30-day period, so a mistake shared by the queries and
	// expectedStats would still show.
	got, err := st.svc.Stats(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if p := got.Playback; p.Direct != 2 || p.Remux != 1 || p.Transcode != 2 || p.TranscodeShare != 0.4 || p.HardwareShare != 1 {
		t.Errorf("playback %+v, want 2/1/2, share 0.4, hardware 1", p)
	}
	titles := make([]string, 0, len(got.TopTitles))
	for _, tt := range got.TopTitles {
		titles = append(titles, fmt.Sprintf("%s %d %d", tt.Title, tt.WatchedMs, tt.Plays))
	}
	if want := []string{"Two 1600000 2", "Eins 900000 1", "Parsed Three 300000 2", "Vier 120000 3"}; !slices.Equal(titles, want) {
		t.Errorf("top titles %v, want %v", titles, want)
	}
	devices := make([]string, 0, len(got.Devices))
	for _, d := range got.Devices {
		devices = append(devices, fmt.Sprintf("%s %d %d", d.Type, d.Count, d.WatchedMs))
	}
	if want := []string{"phone 1 1600000", "tv 1 970000", "unknown 1 350000"}; !slices.Equal(devices, want) {
		t.Errorf("devices %v, want %v", devices, want)
	}
	if got.TopTitles[0].Year != nil || got.TopTitles[1].Year == nil || *got.TopTitles[1].Year != 2001 || got.TopTitles[2].Kind != "episode" {
		t.Errorf("year or kind wrong: %+v", got.TopTitles)
	}
}

func TestStatsEmptyAndInvalid(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	got, err := st.svc.Stats(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	want := model.Stats{Days: 7, TopTitles: []model.StatTitle{}, WatchTime: []model.StatProfile{}, Devices: []model.StatDevice{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("empty database: %+v, want zeros and empty lists", got)
	}
	for _, days := range []int{0, 1, 8, 31, 366, -7} {
		var verr *model.ValidationError
		if _, err := st.svc.Stats(ctx, days); !errors.As(err, &verr) || verr.Fields[0].Field != "days" {
			t.Errorf("days %d: err = %v, want a validation error on days", days, err)
		}
	}
}

// Review I-3: ended playback sessions stay for the statistics. The fixture's browser sessions
// expired long ago and its playback sessions ended 1 to 40 days ago; the session cleanup of package
// auth keeps a browser session while a playback session refers to it (the foreign key would delete
// them with it), so the playback block of 90 days still counts all six. A browser session without
// playback sessions goes, and the kept one goes with its last playback session (package playback
// deletes those 400 days after their end).
func TestStatsKeepEndedPlaybackSessions(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.svc.now = func() time.Time { return statsNow }
	seedStats(t, st.db.Write)
	before, err := st.svc.Stats(ctx, 90)
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.New(st.db, slog.New(slog.DiscardHandler)).WithClock(func() time.Time { return statsNow })
	if _, err := sessions.DeleteExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := st.svc.Stats(ctx, 90)
	if err != nil {
		t.Fatal(err)
	}
	if p := after.Playback; p != before.Playback || p.Direct != 2 || p.Remux != 1 || p.Transcode != 3 {
		t.Errorf("playback after the session cleanup %+v, before %+v; want 2/1/3", p, before.Playback)
	}
	left := func() []int64 {
		t.Helper()
		var ids []int64
		scanAll(t, st.db.Read, "SELECT id FROM sessions ORDER BY id", func(rows *sql.Rows) error {
			var id int64
			err := rows.Scan(&id)
			ids = append(ids, id)
			return err
		})
		return ids
	}
	if got := left(); !slices.Equal(got, []int64{1}) {
		t.Errorf("browser sessions after the cleanup %v, want [1] (the one with playback sessions)", got)
	}
	exec(t, st.db.Write, "DELETE FROM playback_sessions")
	if _, err := sessions.DeleteExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}
	if got := left(); len(got) != 0 {
		t.Errorf("browser sessions after their playback sessions went %v, want none", got)
	}
}
