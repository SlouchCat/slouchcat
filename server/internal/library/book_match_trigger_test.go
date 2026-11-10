package library

import (
	"fmt"
	"path/filepath"
	"testing"
)

// A book is handed to the match against its folder's sources once all its parts are probed
// (concept 19 D6.3 point 6, PD-10): not before, not with the automatic match off, not when it is
// matched already, and not for a folder without a source besides the files.
func TestFinishAudiobookQueuesBookMatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.createAudiobooks(t.TempDir()).ID
	part := func(n int, id string, probed bool) {
		probe := any(nil)
		if probed {
			probe = "{}"
		}
		e.exec(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, probe, duration_ms, created_at, updated_at)
		        VALUES (?, ?, (SELECT id FROM library_roots WHERE library_id = ?), ?, 1, 1, 'm4b', ?, 60000, 1, 1)`,
			fmt.Sprintf("01J9F22E%018d", n), id, lib, fmt.Sprintf("b%d/%d.m4b", n, n), probe)
	}
	queued := func(id string) bool {
		return e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND state = 'queued'`, JobMetadataMatch, "match:"+id) > 0
	}
	finish := func(id string) {
		if err := e.svc.finishAudiobook(e.ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	waiting := e.book(lib, 1, "", "A", 0)
	part(1, waiting, false)
	finish(waiting)
	if queued(waiting) {
		t.Error("match queued before the probe")
	}
	e.exec(`UPDATE media_files SET probe = '{}' WHERE item_id = ?`, waiting)
	finish(waiting)
	if !queued(waiting) {
		t.Error("match not queued once every part is probed")
	}

	matched := e.book(lib, 2, "", "A", 0)
	part(2, matched, true)
	e.exec(`UPDATE items SET book_match_state = 'matched' WHERE id = ?`, matched)
	finish(matched)
	if queued(matched) {
		t.Error("match queued for a matched book")
	}

	failed := e.book(lib, 3, "", "A", 0)
	part(3, failed, true)
	e.exec(`UPDATE items SET book_match_state = 'error' WHERE id = ?`, failed)
	finish(failed)
	if !queued(failed) {
		t.Error("match not queued again for a book in error")
	}

	e.exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, 'off', 1)`, settingBookAutoMatch)
	off := e.book(lib, 4, "", "A", 0)
	part(4, off, true)
	finish(off)
	if queued(off) {
		t.Error("match queued with the automatic match off")
	}

	e.exec(`DELETE FROM settings WHERE key = ?`, settingBookAutoMatch)
	e.exec(`UPDATE library_roots SET providers = '["local"]' WHERE library_id = ?`, lib)
	local := e.book(lib, 5, "", "A", 0)
	part(5, local, true)
	finish(local)
	if queued(local) {
		t.Error("match queued for a folder without a book source")
	}
}

// A book a scan finds carries the source list of its folder (concept 19 D6.2), like the titles of
// video folders, so the editor offers the folder's sources for it.
func TestNewAudiobookTakesTheFolderSources(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	src := filepath.Join(audiobookMedia(t), filepath.FromSlash(m4bFolder), m4bTitle+".m4b")
	if err := copyFile(src, filepath.Join(dir, "Autorin", "Buch", "buch.m4b")); err != nil {
		t.Fatal(err)
	}
	lib := e.createAudiobooks(dir)
	e.scan(lib.ID)
	want := e.str(`SELECT providers FROM library_roots WHERE library_id = ?`, lib.ID)
	if want != `["local","audible","dnb","applebooks"]` {
		t.Fatalf("folder list = %s, want the default of audiobook folders", want)
	}
	if got := e.str(`SELECT providers FROM items WHERE kind = 'audiobook' AND library_id = ?`, lib.ID); got != want {
		t.Errorf("book providers = %s, want %s", got, want)
	}
}
