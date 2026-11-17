package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00068 (concept 25, 08-datenmodell-api.md section 2.9) on the newest frozen state
// (ordner-metadaten.sql): the three tables of titles outside the library come empty with their
// indexes, the checks hold (media, JSON, the age 0 to 18, seasons only for a show), and the texts
// and images go with their title.
func TestMigration00068ExternalTitles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "ordner-metadaten.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM goose_db_version WHERE version_id = 68 AND is_applied = 1`:                                                           1,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                                            0,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('external_titles', 'external_title_texts', 'external_title_images')`: 3,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('external_titles_tvdb', 'external_titles_imdb')`:                     2,
		`SELECT count(*) FROM external_titles`:                                                                                                     0,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	exec := func(query string) error {
		_, err := db.Write.ExecContext(ctx, query)
		return err
	}
	for _, q := range []string{
		`INSERT INTO external_titles (media, tmdb_id, imdb_id, min_age, genres, certifications, fetched_at)
			VALUES ('movie', 1061474, 'tt5950044', 12, '[{"id":878,"names":{"de":"Science-Fiction"}}]', '{"DE":"12","US":"PG-13"}', 1)`,
		`INSERT INTO external_titles (media, tmdb_id, tvdb_id, seasons, last_episode, fetched_at)
			VALUES ('show', 1399, 121361, '[{"number":1,"episodeCount":10,"airDate":"2011-04-17"}]', '{"season":8,"episode":6,"airDate":"2019-05-19"}', 1)`,
		`INSERT INTO external_title_texts (media, tmdb_id, language, title) VALUES ('movie', 1061474, 'de', 'Superman')`,
		`INSERT INTO external_title_texts (media, tmdb_id, language, title) VALUES ('movie', 1061474, 'en', 'Superman')`,
		`INSERT INTO external_title_images (media, tmdb_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, fetched_at)
			VALUES ('movie', 1061474, 'poster', 'w400', 'webp', 'https://image.tmdb.org/t/p/original/a.jpg', 'titles/movie/1061474/poster-w400.webp', 'h', 'image/webp', 400, 600, 10, 1)`,
	} {
		if err := exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for name, q := range map[string]string{
		"unknown media":       `INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('series', 1, 1)`,
		"tmdb id 0":           `INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('movie', 0, 1)`,
		"age above 18":        `INSERT INTO external_titles (media, tmdb_id, min_age, fetched_at) VALUES ('movie', 2, 19, 1)`,
		"genres not an array": `INSERT INTO external_titles (media, tmdb_id, genres, fetched_at) VALUES ('movie', 3, '{}', 1)`,
		"seasons of a movie":  `INSERT INTO external_titles (media, tmdb_id, seasons, fetched_at) VALUES ('movie', 4, '[{"number":1}]', 1)`,
		"episode of a movie":  `INSERT INTO external_titles (media, tmdb_id, last_episode, fetched_at) VALUES ('movie', 5, '{}', 1)`,
		"duplicate title":     `INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('movie', 1061474, 1)`,
		"text without title":  `INSERT INTO external_title_texts (media, tmdb_id, language, title) VALUES ('movie', 999, 'de', 'x')`,
		"unknown image kind": `INSERT INTO external_title_images (media, tmdb_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, fetched_at)
			VALUES ('movie', 1061474, 'still', 'w400', 'webp', 's', 'r', 'h', 'image/webp', 1, 1, 1, 1)`,
		"duplicate image": `INSERT INTO external_title_images (media, tmdb_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, fetched_at)
			VALUES ('movie', 1061474, 'poster', 'w400', 'webp', 's', 'r', 'h', 'image/webp', 1, 1, 1, 1)`,
	} {
		if err := exec(q); err == nil {
			t.Errorf("%s: insert succeeded, want a constraint error", name)
		}
	}
	// An unknown age is NULL; the same TMDB id may stand once as a movie and once as a show.
	if err := exec(`INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('show', 1061474, 1)`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM external_titles WHERE media = 'show' AND tmdb_id = 1061474 AND min_age IS NULL AND genres = '[]' AND certifications = '{}' AND trailers = '[]' AND seasons = '[]'`); got != 1 {
		t.Errorf("defaults of a new row: %d rows, want 1", got)
	}
	if err := exec(`DELETE FROM external_titles WHERE media = 'movie' AND tmdb_id = 1061474`); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT (SELECT count(*) FROM external_title_texts) + (SELECT count(*) FROM external_title_images)`); got != 0 {
		t.Errorf("texts and images after their title went: %d rows, want 0", got)
	}
	if got := count(`SELECT count(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Errorf("foreign_key_check after the inserts: %d rows, want 0", got)
	}
}
