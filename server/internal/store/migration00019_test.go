package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// Migration 00019 (M10) adds progress_days, the condensed playback history of plan 3.2, next to the
// untouched progress_events. TestFrozenStatesMigrate already runs it over every frozen state; this
// test adds what the condensation and the statistics rely on: the raw events survive, the day rows
// have the primary key that lets a second condensation add to a day instead of failing, "no device"
// is the empty string (a WITHOUT ROWID primary key cannot hold NULL), deleting a profile takes its
// days with it, and the index the statistics read is there.
func TestMigration00019AddsTheCondensedDays(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M8.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// The frozen state's raw events are still there: the migration only adds.
	var events int
	if err := db.Read.QueryRow(`SELECT count(*) FROM progress_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events == 0 {
		t.Fatal("the frozen M8 state has no progress events to condense")
	}

	var profile int64
	var item string
	if err := db.Read.QueryRow(`SELECT profile_id, item_id FROM progress_events LIMIT 1`).Scan(&profile, &item); err != nil {
		t.Fatal(err)
	}
	const upsert = `INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays)
	                VALUES (?, ?, '', 20000, 1000, 1)
	                ON CONFLICT (profile_id, item_id, device_id, day) DO UPDATE
	                SET played_ms = progress_days.played_ms + excluded.played_ms,
	                    plays = progress_days.plays + excluded.plays`
	for range 2 { // a second condensation of the same day adds to the stored row
		if _, err := db.Write.Exec(upsert, profile, item); err != nil {
			t.Fatal(err)
		}
	}
	var rows, played, plays int64
	if err := db.Read.QueryRow(`SELECT count(*), sum(played_ms), sum(plays) FROM progress_days`).Scan(&rows, &played, &plays); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || played != 2000 || plays != 2 {
		t.Errorf("after two upserts: %d rows, %d ms, %d plays; want 1 row with 2000 ms and 2 plays", rows, played, plays)
	}

	// NULL as the device is refused: the empty string stands for "no device".
	if _, err := db.Write.Exec(`INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays)
	                            VALUES (?, ?, NULL, 20001, 1, 1)`, profile, item); err == nil {
		t.Error("progress_days accepted NULL as the device id")
	}
	// An unknown profile is refused, and deleting a profile takes its days with it.
	if _, err := db.Write.Exec(`INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays)
	                            VALUES (999999, ?, '', 20002, 1, 1)`, item); err == nil {
		t.Error("progress_days accepted an unknown profile")
	}
	if _, err := db.Write.Exec(`DELETE FROM profiles WHERE id = ?`, profile); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRow(`SELECT count(*) FROM progress_days`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Errorf("%d day rows survived the deleted profile", rows)
	}

	for _, index := range []string{"progress_days_day", "progress_events_server_time"} {
		var n int
		if err := db.Read.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&n); err != nil || n != 1 {
			t.Errorf("index %s missing after the migration (%v)", index, err)
		}
	}
}
