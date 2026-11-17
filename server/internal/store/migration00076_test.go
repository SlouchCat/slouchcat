package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00076 (concept 25 step 4.6b, 08-datenmodell-api.md section 2.14) on the newest frozen
// state (ordner-metadaten.sql): wish_profile_tags and wish_tags come empty with their indexes, the
// checks hold (label form, unique names, states, set_at with set) and the rows go with their wish
// resp. profile.
func TestMigration00076WishTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 76 AND is_applied = 1`:                              1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                               0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('wish_profile_tags', 'wish_tags')`:      2,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('wish_tags_open', 'wish_tags_profile')`: 2,
		`SELECT count(*) FROM wish_profile_tags`:                                                                      0,
		`SELECT count(*) FROM wish_tags`:                                                                              0,
		`SELECT count(*) FROM pragma_table_info('wish_tags') WHERE name IN ('wish_id', 'profile_id', 'service', 'tag_name', 'arr_tag_id', 'arr_title_id', 'state', 'attempts', 'last_error', 'set_at')`: 10,
		`SELECT count(*) FROM profiles`: 4,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	exec := func(query string) error {
		_, err := db.Write.ExecContext(ctx, query)
		return err
	}
	for _, q := range []string{
		`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (1, 'movie', 603, 'The Matrix', 'slouchcat', 'processing', 1, 1)`,
		`INSERT INTO wishes (id, media, tmdb_id, tvdb_id, title, origin, state, created_at, updated_at) VALUES (2, 'show', 1399, 121361, 'Game of Thrones', 'slouchcat', 'processing', 1, 1)`,
		`INSERT INTO wish_profile_tags (profile_id, name, custom, set_at) SELECT id, 'sc-noah', 0, 1 FROM profiles ORDER BY id LIMIT 1`,
		`INSERT INTO wish_tags (wish_id, profile_id, service, tag_name) SELECT 1, id, 'radarr', 'sc-noah' FROM profiles ORDER BY id LIMIT 1`,
		`INSERT INTO wish_tags (wish_id, profile_id, service, tag_name, arr_tag_id, arr_title_id, state, set_at) SELECT 2, id, 'sonarr', 'sc-noah', 3, 7, 'set', 5 FROM profiles ORDER BY id LIMIT 1`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	const p1 = `(SELECT id FROM profiles ORDER BY id LIMIT 1)`
	const p2 = `(SELECT id FROM profiles ORDER BY id LIMIT 1 OFFSET 1)`
	for name, q := range map[string]string{
		"upper case name":      `INSERT INTO wish_profile_tags (profile_id, name, set_at) VALUES (` + p2 + `, 'sc-Noah', 1)`,
		"blank in name":        `INSERT INTO wish_profile_tags (profile_id, name, set_at) VALUES (` + p2 + `, 'sc noah', 1)`,
		"empty name":           `INSERT INTO wish_profile_tags (profile_id, name, set_at) VALUES (` + p2 + `, '', 1)`,
		"name too long":        `INSERT INTO wish_profile_tags (profile_id, name, set_at) VALUES (` + p2 + `, printf('%.65c', 'a'), 1)`,
		"name taken":           `INSERT INTO wish_profile_tags (profile_id, name, set_at) VALUES (` + p2 + `, 'sc-noah', 1)`,
		"custom 2":             `INSERT INTO wish_profile_tags (profile_id, name, custom, set_at) VALUES (` + p2 + `, 'sc-x', 2, 1)`,
		"unknown profile":      `INSERT INTO wish_profile_tags (profile_id, name, set_at) VALUES (9999, 'sc-x', 1)`,
		"second row":           `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name) VALUES (1, ` + p1 + `, 'radarr', 'sc-noah')`,
		"unknown service":      `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name) VALUES (1, ` + p2 + `, 'lidarr', 'sc-x')`,
		"unknown state":        `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name, state) VALUES (1, ` + p2 + `, 'radarr', 'sc-x', 'removed')`,
		"set without set_at":   `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name, state) VALUES (1, ` + p2 + `, 'radarr', 'sc-x', 'set')`,
		"set_at while pending": `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name, set_at) VALUES (1, ` + p2 + `, 'radarr', 'sc-x', 1)`,
		"error with address":   `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name, state, last_error) VALUES (1, ` + p2 + `, 'radarr', 'sc-x', 'failed', 'http://radarr:7878')`,
		"tag name with blank":  `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name) VALUES (1, ` + p2 + `, 'radarr', 'sc x')`,
		"unknown wish":         `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name) VALUES (99, ` + p2 + `, 'radarr', 'sc-x')`,
		"title id 0":           `INSERT INTO wish_tags (wish_id, profile_id, service, tag_name, arr_title_id) VALUES (1, ` + p2 + `, 'radarr', 'sc-x', 0)`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: insert succeeded, want a constraint error", name)
		}
	}
	if got := count(`SELECT count(*) FROM wish_tags WHERE wish_id = 1 AND state = 'pending' AND attempts = 0 AND set_at IS NULL`); got != 1 {
		t.Errorf("defaults of a new row: %d rows, want 1", got)
	}
	if err := exec(`DELETE FROM wishes WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM wish_tags WHERE wish_id = 1`); got != 0 {
		t.Errorf("tag rows after their wish went: %d, want 0", got)
	}
	if err := exec(`DELETE FROM profiles WHERE id = ` + p1); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM wish_tags`) + count(`SELECT count(*) FROM wish_profile_tags`); got != 0 {
		t.Errorf("tag rows after their profile went: %d, want 0", got)
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
