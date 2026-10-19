package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// Migration 00006 rebuilds images for the image pipeline: rows stored by M1a survive as format
// 'jpeg' without an accent, the new unique key allows both formats of a size, and the checks
// reject unknown formats and malformed accents.
func TestImagesMigrationKeepsM1aRows(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M1a.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	path := filepath.Join(cfg, FileName)
	replay(t, path, string(script))

	// An M1a image row, written with the M1a schema before the server migrates.
	dsn := (&url.URL{Scheme: "file", Path: path}).String()
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	var itemID string
	if err := raw.QueryRow(`SELECT id FROM items ORDER BY id LIMIT 1`).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO images (item_id, kind, size, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
	                       VALUES (?, 'poster', 'w500', 'https://image.tmdb.org/t/p/w500/a.jpg', 'images/x/poster-w500.jpg', 'h', 'image/jpeg', 500, 750, 1, 1)`, itemID); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "images-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	defer db.Close()
	var size, format string
	var accent sql.NullString
	if err := db.Read.QueryRow(`SELECT size, format, accent FROM images WHERE item_id = ?`, itemID).Scan(&size, &format, &accent); err != nil {
		t.Fatal(err)
	}
	if size != "w500" || format != "jpeg" || accent.Valid {
		t.Fatalf("migrated row: size %s, format %s, accent %v", size, format, accent)
	}

	insert := func(size, format string, accent any) error {
		_, err := db.Write.Exec(`INSERT INTO images (item_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, accent, created_at)
		                         VALUES (?, 'poster', ?, ?, 's', 'r', 'h', 'image/webp', 1, 1, 1, ?, 1)`, itemID, size, format, accent)
		return err
	}
	if err := insert("w400", "webp", "#a1b2c3"); err != nil {
		t.Fatalf("webp row: %v", err)
	}
	if err := insert("w400", "jpeg", "#a1b2c3"); err != nil {
		t.Fatalf("jpeg row next to webp: %v", err)
	}
	for name, err := range map[string]error{
		"duplicate":    insert("w400", "webp", nil),
		"gif":          insert("w200", "gif", nil),
		"upper accent": insert("w800", "webp", "#A1B2C3"),
		"short accent": insert("w800", "jpeg", "#abc"),
	} {
		if err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
