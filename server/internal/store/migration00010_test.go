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

// Migration 00010 recomputes the containers of the frozen M3 state with the new rule: the series
// whose episodes were all raised to 16 under the old triggers (series min_child_rating stayed at
// the seasons' inherited 12) is above 12 afterwards, and its extra is hidden for the 12 profile.
func TestMigration00010RecomputesContainers(t *testing.T) {
	const (
		series = "01M5C0000000000000000SER01"
		season = "01M5C0000000000000000SEA01"
		extra  = "01M5C0000000000000000EXT01"
		teen   = 2 // profile "Teenager", level 12
	)
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M3.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	raw, err := sql.Open("sqlite", filepath.Join(cfg, FileName))
	if err != nil {
		t.Fatal(err)
	}
	// Under the triggers of 00008 the series keeps 12 although no episode is 12 any more.
	mustExec(t, raw, `UPDATE items SET own_rating = 16 WHERE kind = 'episode' AND library_id = 2`)
	if got := minChild(t, raw, series); got != val(12) {
		t.Fatalf("before 00010: series min_child_rating = %v, want the old 12", got)
	}
	if !visible(t, raw, teen, series) || !visible(t, raw, teen, extra) {
		t.Fatal("before 00010 the series and its extra are visible for the 12 profile (the finding)")
	}
	_ = raw.Close()

	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := minChild(t, db.Read, season); got != val(16) {
		t.Errorf("season min_child_rating = %v, want 16", got)
	}
	if got := minChild(t, db.Read, series); got != val(16) {
		t.Errorf("series min_child_rating = %v, want 16", got)
	}
	if visible(t, db.Read, teen, series) || visible(t, db.Read, teen, extra) {
		t.Error("after 00010 the series and its extra must be hidden for the 12 profile")
	}
	if !visible(t, db.Read, 1, series) || !visible(t, db.Read, 1, extra) {
		t.Error("the 18 profile must still see the series and its extra")
	}
	// Every container follows the new rule.
	if got := queryInt(t, db.Read, `SELECT count(*) FROM items AS i WHERE i.kind IN ('series', 'season', 'podcast') AND (
	     i.available <> EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = i.id AND c.deleted_at IS NULL AND c.available = 1
	                            AND c.kind IN ('season', 'episode', 'podcast_episode'))
	  OR i.min_child_rating IS NOT (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
	                                                THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
	                                                ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
	                                WHERE c.parent_id = i.id AND c.deleted_at IS NULL AND c.available = 1
	                                  AND c.kind IN ('season', 'episode', 'podcast_episode')))`); got != val(0) {
		t.Errorf("%v containers differ from the new rule", got)
	}
}
