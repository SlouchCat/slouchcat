package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 13 step 7 (migration 00032; testdata/db/abspielseite.sql)
// carries the data the step adds: the primary profile with the preferences the migration filled
// from its interface language (F-12), a second profile with its own lists and the rule "foreign",
// a remembered choice per series of both profiles (one with subtitles off), a series with an
// original language, a film looked up without one, an SDH subtitle track and one playback session
// each on the rungs original and 2160p. TestFrozenStatesMigrate replays it like every state; this
// test checks that each of those is really in it.
func TestFrozenAbspielseiteStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "abspielseite.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"profile filled from its language":    `SELECT count(*) FROM profiles WHERE audio_languages = json_array(language) AND subtitle_languages = json_array(language) AND subtitle_mode = 'forced'`,
		"profile with its own preferences":    `SELECT count(*) FROM profiles WHERE json_array_length(audio_languages) = 2 AND subtitle_mode = 'foreign' AND json_array_length(subtitle_languages) = 2`,
		"series choice with subtitles off":    `SELECT count(*) FROM profile_series_tracks WHERE subtitle_off = 1 AND audio_language <> '' AND subtitle_language = ''`,
		"series choice with subtitles":        `SELECT count(*) FROM profile_series_tracks WHERE subtitle_off = 0 AND subtitle_language <> ''`,
		"series choice of a real series":      `SELECT count(*) FROM profile_series_tracks AS t JOIN items AS i ON i.id = t.series_id WHERE i.kind = 'series'`,
		"series with an original language":    `SELECT count(*) FROM items WHERE kind = 'series' AND length(original_language) = 2`,
		"title looked up without one":         `SELECT count(*) FROM items WHERE original_language = ''`,
		"title not looked up yet":             `SELECT count(*) FROM items WHERE original_language IS NULL`,
		"SDH subtitle track":                  `SELECT count(*) FROM tracks WHERE kind = 'subtitle' AND is_hearing_impaired = 1`,
		"playback on the rung original":       `SELECT count(*) FROM playback_sessions WHERE quality = 'original' AND mode <> 'transcode'`,
		"playback on the rung 2160p":          `SELECT count(*) FROM playback_sessions WHERE quality = '2160p' AND height = 2160 AND hardware = 1`,
		"index of the series choices":         `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'profile_series_tracks_series'`,
		"index of the rebuilt playback table": `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'playback_sessions_profile_activity'`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen abspielseite state has no %s", what)
		}
	}
}
