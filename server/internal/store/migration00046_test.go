package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00046 (1.26.0, concept 19 stage D3) on the frozen state of migration 00045
// (hoerbuch-reihen.sql) with two audiobook folders: the folder with the default list ["local"]
// and its book get the fillers of D6.2, a folder with a list of its own keeps it. The books start
// without match state, the state and the candidate rank are checked, candidates go with their book,
// and the search finds a book by its file author and by its unified name.
func TestMigration00046BookSources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "hoerbuch-reihen.sql")+`
INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at, content, providers) VALUES
  (3, 3, '/media/frozen/Hoerbuecher', 'ext4', 'inotify', 1, 'audiobook', '["local"]'),
  (4, 3, '/media/frozen/Eigene', 'ext4', 'inotify', 1, 'audiobook', '["local","dnb"]');
UPDATE items SET providers = '["local","dnb"]' WHERE id = '01M6585TZB00KEN0BLE1ST1FT2';
INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at) VALUES
  ('01M6585TZB00KDE0F1LE00000A', '01M6585TZB00KDE0BLE1ST1FT1', 3, 'Frozen Autorin/Frozen Hörbuch/01.mp3', 1, 1, 'mp3', 1, 1),
  ('01M6585TZB00KEN0F1LE00000A', '01M6585TZB00KEN0BLE1ST1FT2', 4, 'File Author/Frozen Audiobook/01.mp3', 1, 1, 'mp3', 1, 1);
`)
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
	const fillers = `["local","audible","dnb","applebooks"]`
	for query, want := range map[string]string{
		`SELECT providers FROM library_roots WHERE id = 3`:                                                                      fillers,
		`SELECT providers FROM library_roots WHERE id = 4`:                                                                      `["local","dnb"]`,
		`SELECT providers FROM items WHERE id = '01M6585TZB00KDE0BLE1ST1FT1'`:                                                   fillers,
		`SELECT providers FROM items WHERE id = '01M6585TZB00KEN0BLE1ST1FT2'`:                                                   `["local","dnb"]`,
		`SELECT count(*) FROM pragma_foreign_key_check`:                                                                         "0",
		`SELECT count(*) FROM items WHERE book_match_state IS NOT NULL OR book_match_manual <> 0 OR author_display IS NOT NULL`: "0",
	} {
		if got := text(query); got != want {
			t.Errorf("%s = %s, want %s", query, got, want)
		}
	}
	for _, stmt := range []string{
		`UPDATE items SET book_match_state = 'unknown' WHERE id = '01M6585TZB00KDE0BLE1ST1FT1'`,
		`INSERT INTO book_match_candidates (item_id, rank, provider, provider_id, score, candidate) VALUES ('01M6585TZB00KDE0BLE1ST1FT1', 4, 'dnb', '1', 0.5, '{}')`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err == nil {
			t.Errorf("accepted: %s", stmt)
		}
	}
	for _, stmt := range []string{
		`UPDATE items SET book_match_state = 'review', author_display = 'Frozen Autorin' WHERE id = '01M6585TZB00KDE0BLE1ST1FT1'`,
		`INSERT INTO book_match_candidates (item_id, rank, provider, provider_id, score, candidate) VALUES ('01M6585TZB00KDE0BLE1ST1FT1', 1, 'dnb', '1', 0.5, '{}')`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	// The search finds the book by the author of the files and by the unified name.
	for _, match := range []string{`"File Author"`, `"Frozen Autorin"`} {
		if got := text(`SELECT count(*) FROM search_index WHERE search_index MATCH '` + match + `' AND item_id = '01M6585TZB00KDE0BLE1ST1FT1'`); got != "1" {
			t.Errorf("search %s finds the book %s times, want 1", match, got)
		}
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO search_index (search_index) VALUES ('integrity-check')`); err != nil {
		t.Errorf("search index after the rebuild: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM items WHERE id = '01M6585TZB00KDE0BLE1ST1FT1'`); err != nil {
		t.Fatal(err)
	}
	if got := text(`SELECT count(*) FROM book_match_candidates`); got != "0" {
		t.Errorf("candidates after deleting their book: %s, want 0", got)
	}
}
