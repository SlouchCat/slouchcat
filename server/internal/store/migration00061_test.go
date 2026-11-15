package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00061 (concept 22 stage 1, ADR 0033) rebuilds profiles without the CHECK on language
// and on max_rating, on the frozen state before it (sprachgeruest.sql: three profiles, one with
// PIN, offline PIN, limits and its own libraries, with progress, watchlist, downloads, a download
// rule, a device and an activity event). Every row that points at a profile survives, the
// foreign keys hold, visible_items, the indexes and the trigger are back, a profile takes ja and
// an age like 13, and every other CHECK of the table still refuses what it refused before.
func TestMigration00061ProfilesOpenLanguage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "sprachgeruest.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 61 AND is_applied = 1`: 1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                  0,
		`SELECT count(*) FROM profiles`:                                                  3,
		`SELECT count(*) FROM profiles WHERE id = 3 AND name = 'Kind' AND max_rating = 6 AND language = 'de'
			AND accent = 'teal' AND all_libraries = 0 AND is_default = 0 AND pin_hash IS NOT NULL
			AND max_bitrate_kbps = 4000 AND max_height = 720 AND max_streams = 1 AND watch_limit_minutes = 60
			AND pin_salt = '0123456789abcdef0123456789abcdef' AND json_valid(offline_pin) AND metadata_language = 'en'
			AND audio_languages = '["de","en"]' AND subtitle_mode = 'always' AND subtitle_languages = '["de"]'`: 1,
		`SELECT count(*) FROM profiles WHERE id = 1 AND is_default = 1 AND metadata_language = 'es'`:                   1,
		`SELECT count(*) FROM progress WHERE profile_id = 3`:                                                           1,
		`SELECT count(*) FROM watchlist WHERE profile_id = 3`:                                                          1,
		`SELECT count(*) FROM downloads WHERE profile_id IN (1, 3)`:                                                    2,
		`SELECT count(*) FROM download_rules WHERE profile_id = 3`:                                                     1,
		`SELECT count(*) FROM devices WHERE profile_id = 3`:                                                            1,
		`SELECT count(*) FROM profile_libraries WHERE profile_id = 3`:                                                  1,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('profiles_account', 'profiles_primary')`: 2,
		`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'activity_events_profile_deleted'`:       1,
		`SELECT count(*) FROM sqlite_master WHERE sql LIKE '%profiles_new%'`:                                           0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'profiles' AND sql LIKE '%language IN%'`:   0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'profiles' AND sql LIKE '%max_rating IN%'`: 0,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	// The view works: the main profile sees titles, «Kind» only those of library 1 up to age 6.
	if count(`SELECT count(*) FROM visible_items WHERE profile_id = 1`) == 0 {
		t.Error("visible_items shows the main profile nothing")
	}
	if n := count(`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id
		WHERE v.profile_id = 3 AND (i.library_id <> 1 OR COALESCE(i.effective_rating, 18) > 6)`); n != 0 {
		t.Errorf("visible_items shows «Kind» %d titles outside its library or above 6", n)
	}

	// A profile in Japanese (concept 22) is stored now and sees what the main profile sees; an
	// age like 13 (stage 3 of concept 22) is stored too.
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO profiles (account_id, name, max_rating, language, created_at, updated_at)
		VALUES (1, 'Nihongo', 18, 'ja', 1, 1)`); err != nil {
		t.Fatalf("a profile with ja was refused: %v", err)
	}
	main, ja := count(`SELECT count(*) FROM visible_items WHERE profile_id = 1`),
		count(`SELECT count(*) FROM visible_items AS v JOIN profiles AS p ON p.id = v.profile_id WHERE p.name = 'Nihongo'`)
	if ja == 0 || ja != main {
		t.Errorf("visible_items shows the ja profile %d titles, the main profile %d", ja, main)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE profiles SET max_rating = 13 WHERE name = 'Nihongo'`); err != nil {
		t.Errorf("max_rating 13 was refused: %v", err)
	}
	// Defaults stay as they were.
	if n := count(`SELECT count(*) FROM profiles WHERE name = 'Nihongo' AND audio_languages = '[]' AND subtitle_mode = 'forced'
		AND all_libraries = 1 AND is_default = 0`); n != 1 {
		t.Error("the defaults of the new profile changed")
	}
	// Every other CHECK still refuses.
	for what, insert := range map[string]string{
		"an empty name":            `(1, '', 18, 'de', 'forced', 1, 1)`,
		"an unknown subtitle rule": `(1, 'x', 18, 'de', 'sometimes', 1, 1)`,
	} {
		if _, err := db.Write.ExecContext(ctx, `INSERT INTO profiles (account_id, name, max_rating, language, subtitle_mode, created_at, updated_at) VALUES `+insert); err == nil {
			t.Errorf("%s was accepted", what)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE profiles SET is_default = 1 WHERE id = 2`); err == nil {
		t.Error("a second primary profile was accepted (profiles_primary)")
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE profiles SET max_height = 1000 WHERE id = 2`); err == nil {
		t.Error("max_height 1000 was accepted")
	}
	// The trigger is back: deleting «Kind» deletes its activity events, and the cascades still run.
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM profiles WHERE id = 3`); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM activity_events WHERE profile_id = 3`:                                     0,
		`SELECT count(*) FROM progress WHERE profile_id = 3`:                                            0,
		`SELECT count(*) FROM watchlist WHERE profile_id = 3`:                                           0,
		`SELECT count(*) FROM downloads WHERE id = '01M65SPRACHE0000000D00002A' AND profile_id IS NULL`: 1,
	} {
		if got := count(query); got != want {
			t.Errorf("after deleting «Kind»: %s = %d, want %d", query, got, want)
		}
	}
}
