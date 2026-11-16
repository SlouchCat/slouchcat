package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00065 (concept 24 section 3.5, ADR 0036) on the frozen state before it
// (ordner-metadaten.sql): the deviation of library 2 (Serien) goes onto both its folders, that of
// library 1 (text languages only) onto its film folder, the audiobook and podcast folders stay like
// the default, library_metadata goes; every title learns its home folder by the oldest present
// file, a series by the files of its episodes (the missing older file of the intro show does not
// count), seasons, episodes and extras inherit, a feed-only podcast has none. (The test of
// migration 00030, which created library_metadata, went with the table.)
func TestMigration00065RootMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql"))
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
	const (
		testShow     = "01M6585TXFV7FMJPYSCG96AQRK"
		introShow    = "01M6585TX6FV1CMGBDPS84K004"
		downloadShow = "01M6585TWQDRBNT5QQW1HXW2JH"
		extrasFilm   = "01M6585TXRHCKT35BV8XKA9H60"
		podcast      = "01M6585TZB00P0DCAST000000A"
		feedPodcast  = "01M6585TZB00P0DCAST000000B"
	)
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 65 AND is_applied = 1`:                    1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                     0,
		`SELECT count(*) FROM sqlite_master WHERE name = 'library_metadata'`:                                0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'items_root'`:                   1,
		`SELECT count(*) FROM library_roots WHERE id = 1 AND text_langs = '["fr","en"]' AND images IS NULL`: 1,
		`SELECT count(*) FROM library_roots WHERE id IN (2, 5) AND text_langs = '["ja","en"]'
			AND json_extract(images, '$.langs.poster') = '["","ja"]' AND json_extract(images, '$.kinds') = '["logo"]'`: 2,
		`SELECT count(*) FROM library_roots WHERE id IN (3, 4) AND text_langs IS NULL AND images IS NULL`: 2,
		// Films and their extras: the film folder.
		`SELECT count(*) FROM items WHERE kind = 'movie' AND (root_id IS NOT 1)`:                               0,
		`SELECT count(*) FROM items WHERE parent_id = '` + extrasFilm + `' AND kind = 'extra' AND root_id = 1`: 5,
		// The test show lies in both folders; its oldest present file is in folder 5.
		`SELECT count(*) FROM items WHERE id = '` + testShow + `' AND root_id = 5`: 1,
		`SELECT count(*) FROM items WHERE (parent_id = '` + testShow + `' OR parent_id IN (SELECT id FROM items WHERE parent_id = '` + testShow + `'))
			AND root_id IS NOT 5`: 0,
		`SELECT count(*) FROM items WHERE parent_id IN (SELECT id FROM items WHERE parent_id = '` + testShow + `') AND kind = 'episode'`: 5,
		// The intro show has an older but missing file in folder 5: its home stays folder 2.
		`SELECT count(*) FROM items WHERE id IN ('` + introShow + `', '` + downloadShow + `') AND root_id = 2`:                       2,
		`SELECT count(*) FROM items WHERE parent_id IN (SELECT id FROM items WHERE parent_id = '` + introShow + `') AND root_id = 2`: 5,
		// The podcast from its episode's file, every episode inherits (the feed one too); the feed-only
		// podcast and its episode have none, nor have the audiobooks without files.
		`SELECT count(*) FROM items WHERE (id = '` + podcast + `' OR parent_id = '` + podcast + `') AND root_id = 4`:             3,
		`SELECT count(*) FROM items WHERE (id = '` + feedPodcast + `' OR parent_id = '` + feedPodcast + `') AND root_id IS NULL`: 2,
		`SELECT count(*) FROM items WHERE kind = 'audiobook' AND root_id IS NOT NULL`:                                            0,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	// The JSON checks hold, and removing a folder clears the home folder of its titles.
	if _, err := db.Write.ExecContext(ctx, `UPDATE library_roots SET text_langs = 'not json' WHERE id = 1`); err == nil {
		t.Error("invalid JSON in text_langs should fail the CHECK")
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE library_roots SET images = '{' WHERE id = 1`); err == nil {
		t.Error("invalid JSON in images should fail the CHECK")
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM library_roots WHERE id = 5`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM items WHERE id = '` + testShow + `' AND root_id IS NULL`); got != 1 {
		t.Errorf("home folder of the test show after its folder went: %d rows with NULL, want 1", got)
	}
}

// Files of the same age (ADR 0036): the path decides. With every file at the same created_at, the
// test show takes whichever of its two series folders has the path that comes first.
func TestMigration00065TieBreakByPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range []struct {
		behind int64 // the folder whose path is moved behind the other
		want   int64
	}{{5, 2}, {2, 5}} {
		cfg := t.TempDir()
		path := cfg + "/" + FileName
		replay(t, path, frozenState(t, "ordner-metadaten.sql"))
		raw := openRaw(t, path)
		if _, err := raw.Exec(`UPDATE media_files SET created_at = 1`); err != nil {
			t.Fatal(err)
		}
		if _, err := raw.Exec(`UPDATE library_roots SET path = '/zz/frozen/behind' WHERE id = ?`, c.behind); err != nil {
			t.Fatal(err)
		}
		if err := raw.Close(); err != nil {
			t.Fatal(err)
		}
		db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		if err != nil {
			t.Fatal(err)
		}
		var root int64
		err = db.Read.QueryRowContext(ctx, `SELECT coalesce(root_id, 0) FROM items WHERE id = '01M6585TXFV7FMJPYSCG96AQRK'`).Scan(&root)
		_ = db.Close()
		if err != nil {
			t.Fatal(err)
		}
		if root != c.want {
			t.Errorf("folder %d behind: test show with files of the same age = folder %d, want %d", c.behind, root, c.want)
		}
	}
}
