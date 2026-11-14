package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after migration 00059 (testdata/db/meine-medien.sql) carries «Meine Medien» of
// concept 21 stage 5 as an administrator left it: radio moved to the top, a library hidden only in
// the side navigation, one only on the start page, one in both places, collections shown on the
// start page too, and a library created after that, appended at the end and visible (O-20).
// TestFrozenStatesMigrate replays it like every state; this test checks that each of those is
// really in it.
func TestFrozenMeineMedienStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "meine-medien.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"radio on top":                          `SELECT count(*) FROM my_media WHERE id = 'radio' AND position = 0`,
		"a library only on the start page":      `SELECT count(*) FROM my_media WHERE key = 'library' AND hide_sidebar = 1 AND hide_home = 0`,
		"a library only in the side navigation": `SELECT count(*) FROM my_media WHERE key = 'library' AND hide_sidebar = 0 AND hide_home = 1`,
		"a library hidden in both places":       `SELECT count(*) FROM my_media WHERE key = 'library' AND hide_sidebar = 1 AND hide_home = 1`,
		"collections on the start page":         `SELECT count(*) FROM my_media WHERE id = 'collections' AND hide_home = 0`,
		"picks hidden on the start page":        `SELECT count(*) FROM my_media WHERE id = 'picks' AND hide_home = 1`,
		"a new library last and visible": `SELECT count(*) FROM my_media AS m WHERE m.key = 'library' AND m.hide_sidebar = 0 AND m.hide_home = 0
			AND m.position = (SELECT max(position) FROM my_media) AND m.library_id = (SELECT max(id) FROM libraries)`,
		"every library with its entry":          `SELECT count(*) = (SELECT count(*) FROM libraries) FROM my_media WHERE key = 'library'`,
		"migration 00059 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 59 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen meine-medien state has no %s", what)
		}
	}
}
