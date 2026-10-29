package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The view visible_items applies the account's ceiling and library rights on top of the profile's
// (M2 design section 1): a guest profile that allows more than the account still sees only what the
// account allows.
func TestVisibleItemsAppliesAccountLimits(t *testing.T) {
	db := openTestDB(t)
	seedTree(t, db.Write)
	addFile(t, db.Write, "01FILE0000000000000000000A", idEp1, "Show/Season 01/E1.mkv")
	addFile(t, db.Write, "01FILE0000000000000000000C", idEp2, "Show/Season 01/E2.mkv")
	addFile(t, db.Write, "01FILE0000000000000000000B", idMovie, "Film/Film.mkv")
	// Library 2 holds a movie rated 0 that the guest account has no right to.
	mustExec(t, db.Write, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (2, 'X', 'video', 0, 0)`)
	mustExec(t, db.Write, `INSERT INTO library_roots (id, library_id, path, created_at) VALUES (2, 2, '/media/x', 0)`)
	const idOther = "01MOVIE000000000000000000B"
	mustExec(t, db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, own_rating, added_at, created_at, updated_at)
	                       VALUES (?, 'movie', 2, 'X', 'x', 0, 0, 0, 0)`, idOther)
	mustExec(t, db.Write, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at)
	                       VALUES ('01FILE0000000000000000000D', ?, 2, 'X.mkv', 1, 0, 'mkv', 0, 0)`, idOther)

	// Guest account: ceiling 12, library 1 only. Its profile PG18 allows 18 and every library.
	mustExec(t, db.Write, `INSERT INTO accounts (id, email, password_hash, role, max_rating, all_libraries, created_at, updated_at)
	                       VALUES (2, 'g@b.c', 'x', 'user', 12, 0, 0, 0)`)
	mustExec(t, db.Write, `INSERT INTO profiles (id, account_id, name, max_rating, all_libraries, is_default, created_at, updated_at)
	                       VALUES (30, 2, 'PG18', 18, 1, 1, 0, 0)`)

	const pg = 30
	for _, id := range []string{idSeries, idSeason, idEp1, idEp2, idMovie, idOther} {
		if visible(t, db.Write, pg, id) {
			t.Errorf("%s visible to the guest profile without any library right", id)
		}
	}

	mustExec(t, db.Write, `INSERT INTO account_libraries (account_id, library_id) VALUES (2, 1)`)
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{idSeries, true}, {idSeason, true}, {idEp1, true}, // 12 on the account's ceiling
		{idEp2, false},   // own rating 16 above the ceiling although PG18 allows it
		{idMovie, true},  // 6
		{idOther, false}, // library 2 is outside account_libraries although profile all_libraries = 1
	} {
		if got := visible(t, db.Write, pg, tc.id); got != tc.want {
			t.Errorf("guest profile PG18, %s: visible = %v, want %v", tc.id, got, tc.want)
		}
	}
	// The owner's P18 is unaffected: it sees everything including episode 2 and library 2.
	for _, id := range []string{idSeries, idEp2, idMovie, idOther} {
		if !visible(t, db.Write, 18, id) {
			t.Errorf("owner profile P18 must see %s", id)
		}
	}

	// Lowering the account ceiling takes effect at once and hides the series (12 > 6).
	mustExec(t, db.Write, `UPDATE accounts SET max_rating = 6 WHERE id = 2`)
	if visible(t, db.Write, pg, idSeries) || visible(t, db.Write, pg, idEp1) || !visible(t, db.Write, pg, idMovie) {
		t.Error("account ceiling 6 must hide the 12-rated series and keep the 6-rated movie")
	}

	// The profile's own library rights narrow the account's rights further.
	mustExec(t, db.Write, `UPDATE accounts SET max_rating = 18 WHERE id = 2`)
	mustExec(t, db.Write, `INSERT INTO account_libraries (account_id, library_id) VALUES (2, 2)`)
	mustExec(t, db.Write, `UPDATE profiles SET all_libraries = 0 WHERE id = ?`, pg)
	mustExec(t, db.Write, `INSERT INTO profile_libraries (profile_id, library_id) VALUES (?, 2)`, pg)
	if visible(t, db.Write, pg, idMovie) || !visible(t, db.Write, pg, idOther) {
		t.Error("profile library rights must narrow the account's rights")
	}
}

// Deleting a profile leaves the sessions that had it active without a profile (the picker shows).
func TestDeletedProfileLeavesSessionsWithoutProfile(t *testing.T) {
	db := openTestDB(t)
	seedTree(t, db.Write)
	mustExec(t, db.Write, `INSERT INTO sessions (id, token_hash, account_id, profile_id, created_at, last_seen_at, expires_at)
	                       VALUES (1, 'h', 1, 6, 0, 0, 1)`)
	mustExec(t, db.Write, `DELETE FROM profiles WHERE id = 6`)
	if got := queryInt(t, db.Write, `SELECT profile_id FROM sessions WHERE id = 1`); got.Valid {
		t.Errorf("session profile_id = %v after the profile was deleted, want NULL", got)
	}
	if queryInt(t, db.Write, `SELECT count(*) FROM sessions`).Int64 != 1 {
		t.Error("the session itself must survive")
	}
}

// Migration 00007 keeps the data of the frozen M1a and M1b states valid: the owner has the full
// ceiling and every library, keeps its sessions (profile still set, device unset) and its one
// primary profile.
func TestMigration00007KeepsFrozenData(t *testing.T) {
	for _, name := range []string{"M1a.sql", "M1b.sql"} {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", name))
			if err != nil {
				t.Fatal(err)
			}
			cfg := t.TempDir()
			replay(t, filepath.Join(cfg, FileName), string(script))
			var sessions, playbacks int
			var before sql.NullInt64
			raw, err := sql.Open("sqlite", filepath.Join(cfg, FileName))
			if err != nil {
				t.Fatal(err)
			}
			if err := raw.QueryRow(`SELECT count(*), max(profile_id) FROM sessions`).Scan(&sessions, &before); err != nil {
				t.Fatal(err)
			}
			if name == "M1b.sql" { // playback_sessions exists from migration 00005
				if err := raw.QueryRow(`SELECT count(*) FROM playback_sessions`).Scan(&playbacks); err != nil {
					t.Fatal(err)
				}
			}
			_ = raw.Close()

			db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var maxRating, allLibraries int
			if err := db.Read.QueryRow(`SELECT max_rating, all_libraries FROM accounts WHERE role = 'admin'`).Scan(&maxRating, &allLibraries); err != nil {
				t.Fatal(err)
			}
			if maxRating != 18 || allLibraries != 1 {
				t.Errorf("owner after migration: max_rating %d, all_libraries %d, want 18 and 1", maxRating, allLibraries)
			}
			var after int
			var profile sql.NullInt64
			if err := db.Read.QueryRow(`SELECT count(*), max(profile_id) FROM sessions`).Scan(&after, &profile); err != nil {
				t.Fatal(err)
			}
			if after != sessions || profile != before {
				t.Errorf("sessions after migration: %d (profile %v), before %d (profile %v)", after, profile, sessions, before)
			}
			if name == "M1b.sql" {
				// The playback sessions survive the rebuild of sessions, and their foreign key still
				// points at the (new) table: every row finds its session.
				var after, orphans int
				if err := db.Read.QueryRow(`SELECT count(*) FROM playback_sessions`).Scan(&after); err != nil {
					t.Fatal(err)
				}
				if err := db.Read.QueryRow(`SELECT count(*) FROM playback_sessions AS ps LEFT JOIN sessions AS s ON s.id = ps.session_id WHERE s.id IS NULL`).Scan(&orphans); err != nil {
					t.Fatal(err)
				}
				if after != playbacks || playbacks == 0 || orphans != 0 {
					t.Errorf("playback_sessions: %d before, %d after, %d without session", playbacks, after, orphans)
				}
				var target string
				if err := db.Read.QueryRow(`SELECT "table" FROM pragma_foreign_key_list('playback_sessions') WHERE "from" = 'session_id'`).Scan(&target); err != nil || target != "sessions" {
					t.Errorf("playback_sessions.session_id references %q (%v), want sessions", target, err)
				}
			}
			var primaries int
			if err := db.Read.QueryRow(`SELECT count(*) FROM profiles WHERE is_default = 1`).Scan(&primaries); err != nil || primaries != 1 {
				t.Errorf("primary profiles = %d (%v), want 1", primaries, err)
			}
			var visibleCount int
			if err := db.Read.QueryRow(`SELECT count(*) FROM visible_items`).Scan(&visibleCount); err != nil {
				t.Fatal(err)
			}
			if visibleCount == 0 {
				t.Error("the owner's profile sees no titles after the migration")
			}
		})
	}
}
