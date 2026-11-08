package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
)

// Migration 00039 (concept 16 stage 2) rebuilds home_rows with the key radio on the frozen state of
// migration 00038: every row keeps id, key, library, position, hidden flag and time, «Meine Sender»
// follows hidden at the end (F-17), the trigger of 00035 still gives a new library its row behind
// it, the radio config gets both cover switches on, and the constraints of the two new tables
// refuse a rule or a cache row that does not fit.
func TestMigration00039RadioStage2(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "webradio.sql"))
	type row struct {
		id, key   string
		library   int64
		position  int
		hidden    int
		updatedAt int64
	}
	read := func(q *sql.DB) map[string]row {
		t.Helper()
		rows, err := q.QueryContext(ctx, `SELECT id, key, COALESCE(library_id, 0), position, hidden, updated_at FROM home_rows`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]row{}
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.key, &r.library, &r.position, &r.hidden, &r.updatedAt); err != nil {
				t.Fatal(err)
			}
			out[r.id] = r
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	raw := openRaw(t, path)
	before := read(raw)
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	maxPos := -1
	for _, r := range before {
		maxPos = max(maxPos, r.position)
	}
	if len(before) == 0 {
		t.Fatal("frozen state holds no home rows")
	}

	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	after := read(db.Read)
	for id, r := range after {
		if id == "radio" {
			if want := (row{id: "radio", key: "radio", position: maxPos + 1, hidden: 1}); r != want {
				t.Errorf("row radio %+v, want %+v", r, want)
			}
			continue
		}
		if old, ok := before[id]; !ok || r != old {
			t.Errorf("row %+v, want %+v carried over", r, old)
		}
	}
	if len(after) != len(before)+1 {
		t.Errorf("%d home rows after the migration, want %d", len(after), len(before)+1)
	}

	// The trigger of 00035 still adds a library's row, hidden, behind «Meine Sender».
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (77, 'Hörbücher', 'audiobooks', 42, 42)`); err != nil {
		t.Fatal(err)
	}
	var position, hidden int
	if err := db.Read.QueryRowContext(ctx, `SELECT position, hidden FROM home_rows WHERE id = 'library:77'`).Scan(&position, &hidden); err != nil ||
		position != maxPos+2 || hidden != 1 {
		t.Errorf("row of the new library at %d, hidden %d (%v); want %d, hidden", position, hidden, err, maxPos+2)
	}

	var mb, itunes int
	if err := db.Read.QueryRowContext(ctx, `SELECT cover_musicbrainz, cover_itunes FROM radio_config WHERE id = 1`).Scan(&mb, &itunes); err != nil || mb != 1 || itunes != 1 {
		t.Errorf("cover switches %d, %d (%v); want both on", mb, itunes, err)
	}

	for _, good := range []string{
		`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('80s', 'genre', 'oldies', 1)`,
		`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('80s', 'kind', NULL, 1)`,
		`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('fm', 'ignore', NULL, 1)`,
		`INSERT INTO radio_tracks (key, artist, title, state, source, image_hash, checked_at, expires_at) VALUES ('a|b', 'A', 'B', 'found', 'musicbrainz', 'abc', 1, 2)`,
		`INSERT INTO radio_tracks (key, artist, title, state, checked_at, expires_at) VALUES ('c|d', 'C', 'D', 'none', 1, 2)`,
	} {
		if _, err := db.Write.ExecContext(ctx, good); err != nil {
			t.Errorf("%s: %v", good, err)
		}
	}
	for _, bad := range []string{
		`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('80s', 'genre', 'pop', 1)`,                                 // twice
		`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('x', 'language', 'de', 1)`,                                 // unknown facet
		`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('y', 'ignore', 'pop', 1)`,                                  // ignore with a value
		`INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES ('', 'genre', 'pop', 1)`,                                    // empty tag
		`INSERT INTO radio_tracks (key, state, checked_at, expires_at) VALUES ('e|f', 'found', 1, 2)`,                                   // found without image
		`INSERT INTO radio_tracks (key, state, source, image_hash, checked_at, expires_at) VALUES ('g|h', 'none', 'itunes', 'x', 1, 2)`, // none with image
		`INSERT INTO radio_tracks (key, state, checked_at, expires_at) VALUES ('a|b', 'none', 1, 2)`,                                    // key twice
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('radio', 'radio', NULL, 99, 0, 0)`,           // twice
		`INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at) VALUES ('library:5', 'library', NULL, 99, 1, 0)`,     // library row without its library
		`UPDATE radio_config SET cover_itunes = 2 WHERE id = 1`,
	} {
		if _, err := db.Write.ExecContext(ctx, bad); err == nil {
			t.Errorf("%s: accepted, want a constraint error", bad)
		}
	}
}
