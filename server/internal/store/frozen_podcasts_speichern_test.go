package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 19 part A stage 1 (migration 00047;
// testdata/db/podcasts-speichern.sql) carries a podcast library on «Speichern» with «Neue + letzte
// 5», three at once and its folder chosen, a podcast with an own scope «Alle Folgen» and a stored
// episode, a new subscription on «Nur Link» with an own N and a pending folder, and a failed
// download. TestFrozenStatesMigrate replays it like every state; this test checks that each of
// those is really in it.
func TestFrozenPodcastsSpeichernStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "podcasts-speichern.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a podcast library that stores":          `SELECT count(*) FROM libraries WHERE kind = 'podcasts' AND podcast_mode = 'save' AND podcast_scope = 'last_n' AND podcast_last_n = 5 AND podcast_parallel = 3 AND podcast_root_id IS NOT NULL`,
		"a library at the defaults":              `SELECT count(*) FROM libraries WHERE podcast_mode = 'link' AND podcast_scope = 'new' AND podcast_last_n = 10 AND podcast_parallel = 2`,
		"a podcast with an own scope":            `SELECT count(*) FROM podcast_settings WHERE mode IS NULL AND scope = 'all'`,
		"a podcast on «Nur Link» with an own N":  `SELECT count(*) FROM podcast_settings WHERE mode = 'link' AND last_n IS NOT NULL`,
		"a pending folder":                       `SELECT count(*) FROM podcast_settings WHERE pending_root_id IS NOT NULL AND pending_folder IS NOT NULL`,
		"a stored episode in the podcast folder": `SELECT count(*) FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id JOIN items AS i ON i.id = m.item_id WHERE r.content = 'podcast' AND i.enclosure_url IS NOT NULL`,
		"a failed download with its code":        `SELECT count(*) FROM podcast_download_failures WHERE error = 'enclosure_unavailable' AND attempts = 2`,
		"migration 00047 recorded as applied":    `SELECT count(*) FROM goose_db_version WHERE version_id = 47 AND is_applied = 1`,
		"foreign keys of the frozen state hold":  `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen podcasts-speichern state has no %s", what)
		}
	}
}
