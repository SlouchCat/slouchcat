package store

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// Migration 00055 (concept 20 sections 9 and 10, stage 5) on the frozen state of stage 3
// (merkliste.sql, after 00054, the newest one before it). The state grants no library to a profile, so the test
// adds three grants first: a valid one, one of a library that no longer exists and one of a
// profile that no longer exists. Only the valid one survives; the table refers to libraries now,
// so deleting a library takes its grants along. items.removed_at exists, comes only with
// deleted_at, and the new indexes are there; the view still reads the grants.
func TestMigration00055LibraryAndTitleDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "merkliste.sql"))
	raw := openRaw(t, path)
	if _, err := raw.ExecContext(ctx, `UPDATE profiles SET all_libraries = 0 WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO profile_libraries (profile_id, library_id) VALUES (2, 1), (2, 99), (99, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	text := func(query string) string {
		t.Helper()
		var s string
		if err := db.Read.QueryRowContext(ctx, query).Scan(&s); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return s
	}
	for query, want := range map[string]string{
		`SELECT group_concat(profile_id || '/' || library_id, ',') FROM profile_libraries`:                                                  "2/1",
		`SELECT "table" || '.' || "from" || ' ' || on_delete FROM pragma_foreign_key_list('profile_libraries') WHERE "table" = 'libraries'`: "libraries.library_id CASCADE",
		`SELECT count(*) > 0 FROM visible_items WHERE profile_id = 2`:                                                                       "1",
		`SELECT count(*) FROM visible_items AS v JOIN items AS i ON i.id = v.item_id WHERE v.profile_id = 2 AND i.library_id <> 1`:          "0",
		`SELECT count(*) FROM items WHERE removed_at IS NOT NULL`:                                                                           "0",
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                                     "0",
		`SELECT count(*) FROM goose_db_version WHERE version_id = 55 AND is_applied = 1`:                                                    "1",
	} {
		if got := text(query); got != want {
			t.Errorf("%s = %s, want %s", query, got, want)
		}
	}
	for _, index := range []string{
		"profile_libraries_library", "items_removed", "progress_item", "progress_events_item", "progress_days_item",
		"playback_sessions_item", "downloads_media_file", "notifications_parent", "item_local_images_root",
	} {
		if n := text(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = '` + index + `'`); n != "1" {
			t.Errorf("index %s missing", index)
		}
	}
	if ddl := text(`SELECT sql FROM sqlite_master WHERE name = 'items_removed'`); !strings.Contains(ddl, "WHERE removed_at IS NOT NULL") {
		t.Errorf("items_removed = %q, want partial on removed_at IS NOT NULL", ddl)
	}
	// removed_at only with deleted_at; both together are accepted.
	film := "01M6585TXK7NNK68XFTPTBESVF"
	if _, err := db.Write.ExecContext(ctx, `UPDATE items SET removed_at = 1 WHERE id = ?`, film); err == nil {
		t.Error("removed_at without deleted_at was accepted")
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE items SET removed_at = 2, deleted_at = 2 WHERE id = ?`, film); err != nil {
		t.Errorf("removed_at with deleted_at: %v", err)
	}
	// The grant goes with its library.
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM libraries WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if n := text(`SELECT count(*) FROM profile_libraries`); n != "0" {
		t.Errorf("grants after deleting the library = %s, want 0", n)
	}
}
