package store

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// The frozen state after concept 14 step 3a (migration 00033; testdata/db/wiedergabestart.sql)
// carries the data the step adds: keyframes_source in all four values and NULL, an empty v1 blob
// (only the version byte) with source NULL, a pre-migration blob the backfill turned into 'scan',
// a job that may run during a pause and a schedule with a one-off date. TestFrozenStatesMigrate
// replays it like every state; this test checks that each of those is really in it and that the
// constraints and the index of the migration came along.
func TestFrozenWiedergabestartStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "wiedergabestart.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"index from Matroska cues":                  `SELECT count(*) FROM media_files WHERE keyframes_source = 'cues' AND length(keyframes) > 1`,
		"index from MP4 tables":                     `SELECT count(*) FROM media_files WHERE keyframes_source = 'mp4' AND length(keyframes) > 1`,
		"index from the full read":                  `SELECT count(*) FROM media_files WHERE keyframes_source = 'scan' AND length(keyframes) > 1`,
		"file with no possible index":               `SELECT count(*) FROM media_files WHERE keyframes_source = 'none' AND keyframes IS NULL`,
		"file with no index yet":                    `SELECT count(*) FROM media_files WHERE keyframes_source IS NULL AND keyframes IS NULL AND video_codec IN ('h264', 'hevc')`,
		"empty v1 blob without a source":            `SELECT count(*) FROM media_files WHERE keyframes_source IS NULL AND length(keyframes) = 1`,
		"no source on a blob with entries":          `SELECT count(*) = 0 FROM media_files WHERE keyframes_source IS NULL AND length(keyframes) > 1`,
		"no source other than the four":             `SELECT count(*) = 0 FROM media_files WHERE keyframes_source NOT IN ('cues', 'mp4', 'scan', 'none')`,
		"cues only on Matroska files":               `SELECT count(*) = 0 FROM media_files WHERE keyframes_source = 'cues' AND container NOT IN ('mkv', 'webm')`,
		"MP4 tables only on MP4 files":              `SELECT count(*) = 0 FROM media_files WHERE keyframes_source = 'mp4' AND container NOT IN ('mp4', 'm4v', 'mov')`,
		"run that may start during a pause":         `SELECT count(*) FROM jobs WHERE override_pause = 1`,
		"run that does not":                         `SELECT count(*) FROM jobs WHERE override_pause = 0`,
		"schedule with a one-off date":              `SELECT count(*) FROM job_schedules WHERE once_at > 0 AND enabled = 0`,
		"schedule without a one-off date":           `SELECT count(*) FROM job_schedules WHERE once_at IS NULL`,
		"partial index of the keyframe sources":     `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'media_files_keyframes' AND tbl_name = 'media_files' AND sql LIKE '%WHERE missing_since IS NULL%'`,
		"migration 00033 recorded as applied":       `SELECT count(*) FROM goose_db_version WHERE version_id = 33 AND is_applied = 1`,
		"foreign keys of the frozen state hold":     `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
		"constraint on keyframes_source in schema":  `SELECT count(*) FROM sqlite_master WHERE name = 'media_files' AND sql LIKE '%keyframes_source%CHECK%'`,
		"keyframes_source filled for a probed file": `SELECT count(*) FROM media_files WHERE probed_at IS NOT NULL AND keyframes_source IS NOT NULL`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen fast playback start state has no %s", what)
		}
	}
	ctx := context.Background()
	for _, bad := range []string{
		`UPDATE media_files SET keyframes_source = 'bogus'`,
		`UPDATE media_files SET keyframes_source = ''`,
		`UPDATE jobs SET override_pause = 2`,
		`UPDATE jobs SET override_pause = NULL`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: accepted, want a constraint error", bad)
		}
	}
}

// Migration 00033 marks every keyframe blob with at least one entry as 'scan' (until then only the
// full read wrote the index), leaves a blob with only the version byte and a file without a blob
// at NULL, keeps every blob byte for byte, and gives the new columns their defaults. The frozen
// abspielseite state (migration 00032) has no source column; three files get a blob first.
func TestMigration00033KeyframeSourceBackfill(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "abspielseite.sql"))
	raw := openRaw(t, path)
	full := string([]byte{1, 0, 0xd0, 0x0f, 0xd0, 0x0f})
	empty := string([]byte{1})
	var ids []string
	rows, err := raw.Query(`SELECT id FROM media_files ORDER BY id LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() // the loop below reads to the end, which closes rows already
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Fatalf("got %d media files, want 3", len(ids))
	}
	for i, blob := range []any{[]byte(full), []byte(empty), nil} {
		if _, err := raw.Exec(`UPDATE media_files SET keyframes = ? WHERE id = ?`, blob, ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	var jobsBefore, schedulesBefore int
	if err := raw.QueryRow(`SELECT (SELECT count(*) FROM jobs), (SELECT count(*) FROM job_schedules)`).Scan(&jobsBefore, &schedulesBefore); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for i, want := range []struct {
		source any
		blob   string
	}{{"scan", full}, {nil, empty}, {nil, ""}} {
		var source any
		var blob []byte
		if err := db.Read.QueryRow(`SELECT keyframes_source, keyframes FROM media_files WHERE id = ?`, ids[i]).Scan(&source, &blob); err != nil {
			t.Fatal(err)
		}
		if source != want.source || string(blob) != want.blob {
			t.Errorf("file %d: source %v, blob % x; want source %v, blob % x", i, source, blob, want.source, []byte(want.blob))
		}
	}
	var bad int
	if err := db.Read.QueryRow(`SELECT count(*) FROM media_files WHERE (coalesce(length(keyframes), 0) > 1) IS NOT (keyframes_source IS 'scan')`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Errorf("%d files where the source does not match the blob", bad)
	}
	var jobs, schedules, paused, once int
	if err := db.Read.QueryRow(`SELECT (SELECT count(*) FROM jobs), (SELECT count(*) FROM job_schedules),
		(SELECT count(*) FROM jobs WHERE override_pause <> 0), (SELECT count(*) FROM job_schedules WHERE once_at IS NOT NULL)`).Scan(&jobs, &schedules, &paused, &once); err != nil {
		t.Fatal(err)
	}
	if jobs != jobsBefore || schedules != schedulesBefore || paused != 0 || once != 0 {
		t.Errorf("jobs %d (was %d), schedules %d (was %d), override_pause set on %d, once_at set on %d; want nothing lost and defaults", jobs, jobsBefore, schedules, schedulesBefore, paused, once)
	}
	var sqlText string
	if err := db.Read.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'media_files_keyframes'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"missing_since IS NULL", "'h264'", "'hevc'", "'av1'", "'vp9'"} {
		if !strings.Contains(sqlText, part) {
			t.Errorf("index media_files_keyframes %q lacks %s", sqlText, part)
		}
	}
}
