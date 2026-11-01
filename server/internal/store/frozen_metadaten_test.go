package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen state after concept 10 step 3 (ADR 0018, migrations 00028 to 00030;
// testdata/db/metadaten.sql) carries the data the concept adds: a third text language in the
// server's list and on a text row with rank 2, a rating country other than DE, a poster minimum
// width, a profile with its own metadata language, a library deviation with a text list and an
// image part, a decided image source per title and kind in item_images (TMDB rows carried over
// by migration 00029, a chosen local one), an image file next to the media in item_local_images,
// and the indexes of the rank and the decided sources. TestFrozenStatesMigrate replays it like
// every state; this test checks that each of those is really in it.
func TestFrozenMetadatenStateCoversTheConcept(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "metadaten.sql"))
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
		"server text list with a third language": `SELECT count(*) FROM settings WHERE key = 'metadata.text_langs' AND value LIKE '%"es"%'`,
		"rating country other than DE":           `SELECT count(*) FROM settings WHERE key = 'metadata.rating_country' AND value <> 'DE'`,
		"poster minimum width":                   `SELECT count(*) FROM settings WHERE key = 'metadata.image_min_width.poster'`,
		"text row of the third language, rank 2": `SELECT count(*) FROM item_texts WHERE lang = 'es' AND rank = 2`,
		"text rows with rank 0 and 1":            `SELECT count(*) FROM item_texts WHERE rank IN (0, 1)`,
		"profile with its own metadata language": `SELECT count(*) FROM profiles WHERE metadata_language IS NOT NULL AND metadata_language <> ''`,
		"library deviation with texts and images": `SELECT count(*) FROM library_metadata WHERE text_langs IS NOT NULL AND images IS NOT NULL
			AND json_valid(images) AND json_extract(images, '$.kinds[0]') = 'logo'`,
		"TMDB image sources carried over by 00029": `SELECT count(*) FROM item_images WHERE provider = 'tmdb' AND kind IN ('poster', 'backdrop') AND chosen = 0`,
		"chosen local poster":                      `SELECT count(*) FROM item_images WHERE provider = 'local' AND chosen = 1 AND source LIKE 'file:%'`,
		"image file next to the media":             `SELECT count(*) FROM item_local_images WHERE kind = 'poster' AND width >= 1000 AND root_id IN (SELECT id FROM library_roots)`,
		"locked poster of that title":              `SELECT count(*) FROM items WHERE locked_fields LIKE '%"poster"%' AND id IN (SELECT item_id FROM item_local_images)`,
		"index of the text rank":                   `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'item_texts_rank'`,
		"logo kind allowed on images":              `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'images' AND sql LIKE '%logo%'`,
		"job types of the concept's background":    `SELECT count(*) FROM job_types WHERE type IN ('metadata.match', 'image.fetch')`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen metadata state has no %s", what)
		}
	}
}
