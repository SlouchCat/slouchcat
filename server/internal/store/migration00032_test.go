package store

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// playbackSessionColumns are the columns of playback_sessions before and after migration 00032.
const playbackSessionColumns = `id, session_id, account_id, profile_id, item_id, media_file_id, device_id, mode, backend, reason,
	started_at, last_activity_at, expires_at, revoked_at, quality, width, height, bitrate, hardware, audio_track,
	subtitle_track, subtitle_delivery, capabilities, position_ms, stopped_by`

func playbackSessionRows(t *testing.T, db *sql.DB) [][]any {
	t.Helper()
	rows, err := db.Query(`SELECT ` + playbackSessionColumns + ` FROM playback_sessions ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out [][]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// Migration 00032 (concept 13 stage 2, 11.9) adds the playback preferences of a profile, filled
// from the interface language (F-12), the table of the languages chosen per series, the original
// language of a title and the SDH mark of a track, and rebuilds playback_sessions so its quality
// takes original and 2160p (blocker B-1). The frozen M4 state carries a finished playback session
// with every column of M4 filled (quality 720p, AC3, a text subtitle, capabilities, stopped_by)
// and the profile "Kind" with all four limits; the test adds an English profile, so both
// interface languages are carried over. A column forgotten in the INSERT ... SELECT of the rebuild
// would vanish without a sound, so every row is compared column by column.
func TestMigration00032PlaybackPrefs(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "M4.sql"))
	raw := openRaw(t, path)
	if _, err := raw.Exec(`INSERT INTO profiles (account_id, name, max_rating, language, all_libraries, is_default, created_at, updated_at)
		SELECT id, 'English', 18, 'en', 1, 0, 1, 1 FROM accounts ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	before := playbackSessionRows(t, raw)
	if len(before) == 0 {
		t.Fatal("the frozen M4 state should carry a playback session")
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	// Every profile starts with its interface language in both lists and the rule "forced" (F-3, F-12).
	rows, err := db.Read.QueryContext(ctx, `SELECT language, audio_languages, subtitle_mode, subtitle_languages FROM profiles ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var langs []string
	for rows.Next() {
		var lang, audio, mode, subs string
		if err := rows.Scan(&lang, &audio, &mode, &subs); err != nil {
			t.Fatal(err)
		}
		if want := `["` + lang + `"]`; audio != want || subs != want || mode != "forced" {
			t.Errorf("profile with language %s: audio %s, mode %s, subtitles %s; want %s, forced, %s", lang, audio, mode, subs, want, want)
		}
		langs = append(langs, lang)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(langs, "de") || !slices.Contains(langs, "en") {
		t.Errorf("profiles %v, want both interface languages", langs)
	}
	for _, bad := range []string{
		`UPDATE profiles SET subtitle_mode = 'sometimes'`,
		`UPDATE profiles SET audio_languages = 'de'`,
		`UPDATE profiles SET subtitle_languages = NULL`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: accepted, want a constraint error", bad)
		}
	}

	// playback_sessions keeps every row column by column, its four indexes and its references.
	if after := playbackSessionRows(t, db.Read); !reflect.DeepEqual(after, before) {
		t.Errorf("playback_sessions after the rebuild:\n%v\nwant\n%v", after, before)
	}
	for _, index := range []string{"playback_sessions_session", "playback_sessions_expires", "playback_sessions_profile_activity", "playback_sessions_device"} {
		var n int
		if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ? AND tbl_name = 'playback_sessions'`, index).Scan(&n); err != nil || n != 1 {
			t.Errorf("index %s: n=%d err=%v", index, n, err)
		}
	}
	var leftover int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name = 'playback_sessions_new'`).Scan(&leftover); err != nil || leftover != 0 {
		t.Errorf("playback_sessions_new left behind: %d, %v", leftover, err)
	}
	// The two new rungs fit the CHECK; an unknown one still does not.
	insert := `INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at, quality)
		SELECT ?, session_id, account_id, profile_id, item_id, media_file_id, 'remux', 1, 1, 1, ? FROM playback_sessions ORDER BY id LIMIT 1`
	for i, q := range []string{"original", "2160p"} {
		if _, err := db.Write.ExecContext(ctx, insert, "01M5CB8WTWR64JW4BB80PVQQ0"+string(rune('A'+i)), q); err != nil {
			t.Errorf("quality %s: %v", q, err)
		}
	}
	for _, q := range []string{"4k", "1440p", ""} {
		if _, err := db.Write.ExecContext(ctx, insert, "01M5CB8WTWR64JW4BB80PVQQ0Z", q); err == nil || !strings.Contains(err.Error(), "CHECK") {
			t.Errorf("quality %q: %v, want a CHECK error", q, err)
		}
	}

	// The original language is not looked up yet (NULL), no track is marked as SDH.
	var looked, sdh int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM items WHERE original_language IS NOT NULL`).Scan(&looked); err != nil || looked != 0 {
		t.Errorf("items with an original language after the migration: %d, %v", looked, err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM tracks WHERE is_hearing_impaired <> 0`).Scan(&sdh); err != nil || sdh != 0 {
		t.Errorf("tracks marked as SDH after the migration: %d, %v", sdh, err)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE tracks SET is_hearing_impaired = 2`); err == nil {
		t.Error("is_hearing_impaired 2: accepted, want a CHECK error")
	}

	// profile_series_tracks: one row per profile and series, gone with the profile or the title.
	q := appq.New(db.Write)
	var profileID, otherProfile int64
	var seriesID, otherItem string
	if err := db.Read.QueryRowContext(ctx, `SELECT min(id), max(id) FROM profiles`).Scan(&profileID, &otherProfile); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM items WHERE kind = 'series' ORDER BY id LIMIT 1`).Scan(&seriesID); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM items WHERE kind = 'movie' ORDER BY id LIMIT 1`).Scan(&otherItem); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertSeriesTracks(ctx, appq.UpsertSeriesTracksParams{ProfileID: profileID, SeriesID: seriesID, AudioLanguage: "en", Now: 1}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertSeriesTracks(ctx, appq.UpsertSeriesTracksParams{ProfileID: profileID, SeriesID: seriesID, SubtitleLanguage: "de", SubtitleOff: 0, Now: 2}); err != nil {
		t.Fatal(err)
	}
	row, err := q.GetSeriesTracks(ctx, appq.GetSeriesTracksParams{ProfileID: profileID, SeriesID: seriesID})
	if err != nil || row.AudioLanguage != "" || row.SubtitleLanguage != "de" || row.UpdatedAt != 2 {
		t.Errorf("the second choice replaces the first: %+v, %v", row, err)
	}
	if err := q.UpsertSeriesTracks(ctx, appq.UpsertSeriesTracksParams{ProfileID: profileID, SeriesID: seriesID, SubtitleOff: 2, Now: 3}); err == nil {
		t.Error("subtitle_off 2: accepted, want a CHECK error")
	}
	for _, p := range []appq.UpsertSeriesTracksParams{
		{ProfileID: otherProfile, SeriesID: seriesID, AudioLanguage: "ja", SubtitleOff: 1, Now: 1},
		{ProfileID: profileID, SeriesID: otherItem, AudioLanguage: "fr", Now: 1},
	} {
		if err := q.UpsertSeriesTracks(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.UpsertSeriesTracks(ctx, appq.UpsertSeriesTracksParams{ProfileID: 999999, SeriesID: seriesID, Now: 1}); err == nil {
		t.Error("a row for an unknown profile: accepted, want a foreign key error")
	}
	count := func() (n int) {
		t.Helper()
		if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM profile_series_tracks`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, otherItem); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 2 {
		t.Errorf("rows after deleting a title: %d, want 2", n)
	}
	if n, err := q.DeleteSeriesTracksByProfile(ctx, profileID); err != nil || n != 1 {
		t.Errorf("reset of one profile: %d, %v", n, err)
	}
	if _, err := q.GetSeriesTracks(ctx, appq.GetSeriesTracksParams{ProfileID: profileID, SeriesID: seriesID}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("row of the reset profile: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, otherProfile); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 0 {
		t.Errorf("rows after deleting the profile: %d, want 0", n)
	}
}
