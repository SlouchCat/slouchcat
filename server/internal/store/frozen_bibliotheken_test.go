package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen state of the library concept (ADR 0015, testdata/db/README.md) carries every kind of
// data the concept added: a video library with a film folder and a series folder, a series
// folder with TVDB as the identifier, provider lists on the titles, provenance rows of more than
// one provider, a title with an AniList id, and a podcast subscription that a folder adopted (an
// episode with a file and an enclosure). A later migration that touches those has to migrate
// all of it.
func TestFrozenBibliothekenStateCoversTheConcept(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "bibliotheken.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"video library with a film and a series folder": `SELECT count(*) FROM libraries l WHERE l.kind = 'video'
			AND EXISTS (SELECT 1 FROM library_roots r WHERE r.library_id = l.id AND r.content = 'movie')
			AND EXISTS (SELECT 1 FROM library_roots r WHERE r.library_id = l.id AND r.content = 'series')`,
		"series folder with TVDB as the identifier": `SELECT count(*) FROM library_roots WHERE content = 'series' AND providers LIKE '["tvdb",%'`,
		"film with AniList behind TMDB in its list": `SELECT count(*) FROM items WHERE kind = 'movie' AND providers = '["tmdb","anilist"]'`,
		"matched series with provenance rows":       `SELECT count(*) FROM items i WHERE i.kind = 'series' AND i.metadata_state = 'matched' AND EXISTS (SELECT 1 FROM item_field_sources f WHERE f.item_id = i.id AND f.provider = 'tmdb')`,
		"provenance rows of a filler and the union": `SELECT count(*) FROM item_field_sources WHERE provider IN ('anilist', 'union')`,
		"title with an AniList id":                  `SELECT count(*) FROM items WHERE anilist_id > 0`,
		"title looked up without an AniList entry":  `SELECT count(*) FROM items WHERE anilist_id = 0`,
		"podcast folder root":                       `SELECT count(*) FROM library_roots WHERE content = 'podcast'`,
		"subscription by hand with a feed":          `SELECT count(*) FROM items WHERE kind = 'podcast' AND feed_origin = 'admin' AND feed_url IS NOT NULL`,
		"episode with a file and an enclosure":      `SELECT count(*) FROM items e JOIN media_files m ON m.item_id = e.id WHERE e.kind = 'podcast_episode' AND e.enclosure_url IS NOT NULL`,
		"episode with a file and without enclosure": `SELECT count(*) FROM items e JOIN media_files m ON m.item_id = e.id WHERE e.kind = 'podcast_episode' AND e.enclosure_url IS NULL`,
		"scanned film library of the base state":    `SELECT count(*) FROM items WHERE kind = 'movie'`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen library state has no %s", what)
		}
	}
}
