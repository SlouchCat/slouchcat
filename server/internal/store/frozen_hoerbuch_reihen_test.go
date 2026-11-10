package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 19 stage D2 (migration 00045; testdata/db/hoerbuch-reihen.sql)
// carries a series of three books with two spellings («Bodyguard», «Bodyguard - Band»), numbers 0.5,
// 1 and 2 and a soft-deleted volume, a series with a locked name, and the settings flag of the first
// link. TestFrozenStatesMigrate replays it like every state; this test checks that each of those is
// really in it.
func TestFrozenHoerbuchReihenStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-reihen.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a series of three books":               `SELECT count(*) FROM (SELECT audio_series_id FROM items GROUP BY audio_series_id HAVING count(*) = 3 AND audio_series_id IS NOT NULL)`,
		"two spellings in one series":           `SELECT count(*) FROM (SELECT audio_series_id FROM items WHERE audio_series_id IS NOT NULL GROUP BY audio_series_id HAVING count(DISTINCT series_name) > 1)`,
		"a volume 0.5":                          `SELECT count(*) FROM items WHERE audio_series_id IS NOT NULL AND series_index = 0.5`,
		"a soft-deleted volume":                 `SELECT count(*) FROM items WHERE audio_series_id IS NOT NULL AND deleted_at IS NOT NULL`,
		"a locked series name":                  `SELECT count(*) FROM audio_series WHERE name_locked = 1`,
		"the flag of the first link":            `SELECT count(*) FROM settings WHERE key = 'library.audio_series_linked'`,
		"migration 00045 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 45 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen hoerbuch-reihen state has no %s", what)
		}
	}
}
