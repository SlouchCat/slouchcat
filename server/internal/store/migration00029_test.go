package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00029 (concept 10 step 2, ADR 0018) rebuilds images with the kind logo, the format png
// and a provider per row, carries the image paths of items over into item_images and creates
// item_local_images. Over the frozen startseite state (matched series with TMDB posters): every
// image row names a provider, every title with a stored path has its item_images row with the
// provider of the host and lang ”, and the new tables take the new values.
func TestMigration00029CarriesTheImagePathsIntoItemImages(t *testing.T) {
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "startseite.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	var withPath, rows int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM items WHERE deleted_at IS NULL AND tmdb_poster_path IS NOT NULL AND tmdb_poster_path <> ''`).Scan(&withPath); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_images WHERE kind = 'poster'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if withPath == 0 || rows != withPath {
		t.Errorf("item_images poster rows = %d, titles with a poster path = %d", rows, withPath)
	}
	var tmdb, other int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_images WHERE provider = 'tmdb' AND lang = '' AND chosen = 0 AND source LIKE '/%'`).Scan(&tmdb); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_images WHERE provider <> 'tmdb'`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if tmdb == 0 || other != 0 {
		t.Errorf("item_images by provider: tmdb %d, other %d (the frozen state knows TMDB paths only)", tmdb, other)
	}
	var images, withProvider int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*), count(provider) FROM images`).Scan(&images, &withProvider); err != nil {
		t.Fatal(err)
	}
	if images != withProvider {
		t.Errorf("images rows without provider: %d of %d", images-withProvider, images)
	}

	// The new values pass the rebuilt CHECKs; the old ones are still refused.
	var itemID string
	if err := db.Read.QueryRowContext(ctx, `SELECT item_id FROM item_images LIMIT 1`).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO images (item_id, kind, size, format, lang, provider, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
		VALUES (?, 'logo', 'w400', 'png', 'de', 'tmdb', '/logo.png', 'images/x/logo-w400.png', 'abc', 'image/png', 400, 150, 1000, 0)`, itemID); err != nil {
		t.Fatalf("insert a png logo row: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO images (item_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
		VALUES (?, 'banner', 'w400', 'jpeg', '/b.jpg', 'images/x/b.jpg', 'abc', 'image/jpeg', 400, 150, 1000, 0)`, itemID); err == nil {
		t.Error("a banner row should fail the kind CHECK")
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO item_images (item_id, kind, provider, lang, source, chosen, updated_at) VALUES (?, 'logo', 'local', '', 'file:Film/logo.png?10-20', 1, 0)`, itemID); err != nil {
		t.Fatalf("insert a chosen logo: %v", err)
	}
	var rootID int64
	if err := db.Read.QueryRowContext(ctx, `SELECT id FROM library_roots LIMIT 1`).Scan(&rootID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO item_local_images (item_id, kind, rel_path, root_id, width, height, size, mtime) VALUES (?, 'poster', 'Film/poster.jpg', ?, 1000, 1500, 123, 456)`, itemID, rootID); err != nil {
		t.Fatalf("insert a local image: %v", err)
	}
	var n int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM item_local_images`).Scan(&n); err != nil || n != 1 {
		t.Errorf("item_local_images rows = %d (%v), want 1 (the migration leaves it empty, the next scan fills it)", n, err)
	}
}
